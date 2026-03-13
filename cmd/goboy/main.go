package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/sugoto/gameboy-emu/internal/emulator"
	"github.com/sugoto/gameboy-emu/driver/ebiten"
)

const (
	windowScale    = 4
	screenWidth    = 160
	screenHeight   = 144
	metricsAddr    = "127.0.0.1:12345"
	saveInterval   = 5 * time.Second
	inputPollRate  = 8 * time.Millisecond
	inputSkipTicks = 100
)

type app struct {
	emu          emulator.Emulator
	audio        *ebiten.AudioBuffer
	frameTimer   ebiten.FrameTimer
	romPath      string
	savePath     string
	snapPrefix   string
	snapMode     rune
	lastInput    emulator.Input
	lastSaveRAM  []byte
	lastSaveTime time.Time
	lastPollTime time.Time
	pollCounter  int
	frames       int
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: goboy <rom-file>")
	}
	romPath := os.Args[1]

	rom, err := loadROM(romPath)
	if err != nil {
		log.Fatal(err)
	}

	devMode := fileExists("devmode")
	emu, title := createEmulator(rom, devMode)

	a := &app{
		emu:        emu,
		romPath:    romPath,
		savePath:   romPath + ".sav",
		snapPrefix: romPath + ".snapshot",
	}
	a.restoreSave()

	ready := make(chan *app, 1)
	go serveMetrics(metricsAddr, ready)

	ebiten.InitDisplayLoop(ebiten.InitDisplayLoopOptions{
		WindowTitle:  title,
		RenderWidth:  screenWidth,
		RenderHeight: screenHeight,
		WindowWidth:  screenWidth * windowScale,
		WindowHeight: screenHeight * windowScale,
		InitCallback: func(win *ebiten.WindowState) {
			buf, err := ebiten.OpenAudioBuffer(ebiten.OpenAudioBufferOptions{
				OutputBufDuration: 25 * time.Millisecond,
				SamplesPerSecond:  44100,
				BitsPerSample:     16,
				ChannelCount:      2,
			})
			if err != nil {
				log.Fatal(err)
			}
			a.audio = buf
			a.frameTimer = ebiten.MakeFrameTimer()
			a.lastSaveTime = time.Now()
			a.lastPollTime = time.Now()
			a.lastSaveRAM = a.emu.GetCartRAM()
			ready <- a
			a.run(win)
		},
	})
}

func createEmulator(rom []byte, devMode bool) (emulator.Emulator, string) {
	title := "GoBoy Emulator"
	if string(rom[:3]) == "GBS" {
		return emulator.NewGbsPlayer(rom, devMode), title
	}
	info := emulator.ParseCartInfo(rom)
	if devMode {
		log.Printf("title=%q type=%d ram=%d rom=%d",
			info.Title, info.CartridgeType, info.GetRAMSize(), info.GetROMSize())
	}
	return emulator.NewEmulator(rom, devMode),
		fmt.Sprintf("GoBoy Emulator - %s", info.Title)
}

func (a *app) restoreSave() {
	data, err := os.ReadFile(a.savePath)
	if err != nil {
		return
	}
	if err := a.emu.SetCartRAM(data); err != nil {
		log.Printf("save load error: %v", err)
	} else {
		log.Println("save restored")
	}
}

func (a *app) run(win *ebiten.WindowState) {
	dbgKeys := make([]bool, 256)
	audioNeeded := a.audio.GetPrevCallbackReadLen()
	var audioBuf []byte

	for {
		a.pollCounter++
		if a.pollCounter >= inputSkipTicks {
			a.pollCounter = 0
			if time.Since(a.lastPollTime) > inputPollRate {
				a.lastPollTime = time.Now()
				a.handleInput(win, dbgKeys)
			}
		}

		if a.emu.InDevMode() {
			a.emu.DbgStep()
		} else {
			a.emu.Step()
		}

		audioBuf = a.pumpAudio(audioBuf, audioNeeded)

		if a.emu.FlipRequested() {
			win.RenderMutex.Lock()
			copy(win.Pix, a.emu.Framebuffer())
			win.RenderMutex.Unlock()

			a.frameTimer.MarkRenderComplete()
			a.frames++
			a.audio.WaitForPlaybackIfAhead()
			audioNeeded = a.audio.GetPrevCallbackReadLen()
			a.frameTimer.MarkFrameComplete()

			a.autoSave()
		}
	}
}

func (a *app) handleInput(win *ebiten.WindowState, dbgKeys []bool) {
	win.InputMutex.Lock()
	defer win.InputMutex.Unlock()

	bDown := win.CharIsDown('b')
	a.lastInput = emulator.Input{
		Joypad: emulator.Joypad{
			Up: win.CharIsDown('w'), Down: win.CharIsDown('s'),
			Left: win.CharIsDown('a'), Right: win.CharIsDown('d'),
			A:     bDown || win.CharIsDown(' '),
			B:     bDown || win.CharIsDown('\b'),
			Start: bDown || win.CharIsDown('\n'),
			Sel:   bDown || win.CharIsDown('x'),
		},
	}

	slot := slotPressed(win)
	switch {
	case win.CharIsDown('m'):
		a.snapMode = 'm'
	case win.CharIsDown('l'):
		a.snapMode = 'l'
	}
	win.CopyKeyCharArray(dbgKeys)

	a.emu.UpdateInput(a.lastInput)
	a.emu.UpdateDbgKeyState(dbgKeys)

	if slot > 0 {
		a.handleSnapshot(slot)
	}
}

func slotPressed(win *ebiten.WindowState) int {
	for r := '1'; r <= '9'; r++ {
		if win.CharIsDown(r) {
			return int(r - '0')
		}
	}
	return 0
}

func (a *app) handleSnapshot(slot int) {
	path := fmt.Sprintf("%s%d", a.snapPrefix, slot)
	switch a.snapMode {
	case 'm':
		a.snapMode = 0
		os.WriteFile(path, a.emu.MakeSnapshot(), 0644)
	case 'l':
		a.snapMode = 0
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("snapshot load: %v", err)
			return
		}
		restored, err := a.emu.LoadSnapshot(data)
		if err != nil {
			log.Printf("snapshot load: %v", err)
			return
		}
		a.emu = restored
	}
}

func (a *app) pumpAudio(buf []byte, needed int) []byte {
	info := a.emu.GetSoundBufferInfo()
	if !info.IsValid || info.UsedSize < needed {
		return buf
	}
	if cap(buf) < needed {
		buf = make([]byte, needed)
	}
	a.audio.Write(a.emu.ReadSoundBuffer(buf[:needed]))
	return buf
}

func (a *app) autoSave() {
	if time.Since(a.lastSaveTime) < saveInterval {
		return
	}
	ram := a.emu.GetCartRAM()
	if len(ram) == 0 || bytes.Equal(ram, a.lastSaveRAM) {
		return
	}
	os.WriteFile(a.savePath, ram, 0644)
	a.lastSaveTime = time.Now()
	a.lastSaveRAM = ram
}

func loadROM(path string) ([]byte, error) {
	if strings.HasSuffix(path, ".zip") {
		return loadZippedROM(path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 4 {
		return nil, fmt.Errorf("file too small to be a valid ROM")
	}
	return data, nil
}

func loadZippedROM(path string) ([]byte, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	if len(zr.File) == 0 {
		return nil, fmt.Errorf("zip archive is empty")
	}
	entry := zr.File[0]
	log.Printf("extracting %q from zip", entry.Name)

	rc, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}

func serveMetrics(addr string, ready <-chan *app) {
	a := <-ready
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("metrics server: %v", err)
		return
	}
	defer ln.Close()
	log.Printf("metrics server listening on %s", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go streamFPS(conn, a)
	}
}

func streamFPS(conn net.Conn, a *app) {
	defer conn.Close()
	prev := a.frames
	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for range tick.C {
		current := a.frames
		fps := float64(current - prev)
		prev = current
		if _, err := fmt.Fprintf(conn, "FPS: %.1f\n", fps); err != nil {
			return
		}
	}
}
