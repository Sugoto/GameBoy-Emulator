package main

import (
	"github.com/sugoto/gameboy-emu"
	// "github.com/theinternetftw/dmgo/profiling"
	"github.com/theinternetftw/glimmer"

	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

// main is the entry point of the program.
func main() {

	// defer profiling.Start().Stop()

	assert(len(os.Args) == 2, "usage: ./dmgo ROM_FILENAME")
	cartFilename := os.Args[1]

	var cartBytes []byte
	var err error
	if strings.HasSuffix(cartFilename, ".zip") {
		cartBytes = readZipFileOrDie(cartFilename)
	} else {
		cartBytes, err = ioutil.ReadFile(cartFilename)
		dieIf(err)
	}

	assert(len(cartBytes) > 3, "cannot parse, file is too small")

	// TODO: config file instead
	devMode := fileExists("devmode")

	var emu dmgo.Emulator
	windowTitle := "SuGOto-GameBoy Emulator"

	fileMagic := string(cartBytes[:3])
	if fileMagic == "GBS" {
		// nsf(e) file
		emu = dmgo.NewGbsPlayer(cartBytes, devMode)
	} else {
		// rom file
		cartInfo := dmgo.ParseCartInfo(cartBytes)
		if devMode {
			fmt.Printf("Game title: %q\n", cartInfo.Title)
			fmt.Printf("Cart type: %d\n", cartInfo.CartridgeType)
			fmt.Printf("Cart RAM size: %d\n", cartInfo.GetRAMSize())
			fmt.Printf("Cart ROM size: %d\n", cartInfo.GetROMSize())
		}

		emu = dmgo.NewEmulator(cartBytes, devMode)
		windowTitle = fmt.Sprintf("SuGOto-GameBoy Emulator - %q", cartInfo.Title)
	}

	snapshotPrefix := cartFilename + ".snapshot"
	saveFilename := cartFilename + ".sav"

	if saveFile, err := ioutil.ReadFile(saveFilename); err == nil {
		err = emu.SetCartRAM(saveFile)
		if err != nil {
			fmt.Println("error loading savefile,", err)
		} else {
			fmt.Println("loaded save!")
		}
	}

	glimmer.InitDisplayLoop(glimmer.InitDisplayLoopOptions{
		WindowTitle: windowTitle,
		RenderWidth: 160, RenderHeight: 144,
		WindowWidth: 160 * 4, WindowHeight: 144 * 4,
		InitCallback: func(sharedState *glimmer.WindowState) {

			audio, audioErr := glimmer.OpenAudioBuffer(glimmer.OpenAudioBufferOptions{
				OutputBufDuration: 25 * time.Millisecond,
				SamplesPerSecond:  44100,
				BitsPerSample:     16,
				ChannelCount:      2,
			})
			dieIf(audioErr)

			session := sessionState{
				snapshotPrefix:    snapshotPrefix,
				saveFilename:      saveFilename,
				frameTimer:        glimmer.MakeFrameTimer(),
				lastSaveTime:      time.Now(),
				lastInputPollTime: time.Now(),
				audio:             audio,
				emu:               emu,
			}

			runEmu(&session, sharedState)
		},
	})
}

// sessionState represents the state of the emulator session.
type sessionState struct {
	snapshotMode           rune
	snapshotPrefix         string
	saveFilename           string
	audio                  *glimmer.AudioBuffer
	latestInput            dmgo.Input
	frameTimer             glimmer.FrameTimer
	lastSaveTime           time.Time
	lastInputPollTime      time.Time
	ticksSincePollingInput int
	lastSaveRAM            []byte
	emu                    dmgo.Emulator
	currentNumFrames       int
}

// runEmu runs the emulator session.
func runEmu(session *sessionState, window *glimmer.WindowState) {

	dbgKeyState := make([]bool, 256)

	var audioChunkBuf []byte
	audioToGen := session.audio.GetPrevCallbackReadLen()

	session.lastSaveRAM = session.emu.GetCartRAM()

	for {
		session.ticksSincePollingInput++
		if session.ticksSincePollingInput == 100 {
			session.ticksSincePollingInput = 0
			now := time.Now()

			inputDiff := now.Sub(session.lastInputPollTime)

			if inputDiff > 8*time.Millisecond {
				session.lastInputPollTime = now

				window.InputMutex.Lock()
				var numDown rune
				{
					bDown := window.CharIsDown('b')
					session.latestInput = dmgo.Input{
						Joypad: dmgo.Joypad{
							Up:    window.CharIsDown('w'), // WASD
							Down:  window.CharIsDown('s'),
							Left:  window.CharIsDown('a'),
							Right: window.CharIsDown('d'),
							Sel:   bDown || window.CharIsDown('x'),  // X
							Start: bDown || window.CharIsDown('\n'), // Enter
							A:     bDown || window.CharIsDown(' '),  // Space
							B:     bDown || window.CharIsDown('\b'), // Backspace
						},
					}

					numDown = 'x'
					for r := '0'; r <= '9'; r++ {
						if window.CharIsDown(r) {
							numDown = r
							break
						}
					}
					if window.CharIsDown('m') {
						session.snapshotMode = 'm'
					} else if window.CharIsDown('l') {
						session.snapshotMode = 'l'
					}

					window.CopyKeyCharArray(dbgKeyState)
				}
				window.InputMutex.Unlock()

				if numDown > '0' && numDown <= '9' {
					snapFilename := session.snapshotPrefix + string(numDown)
					if session.snapshotMode == 'm' {
						session.snapshotMode = 'x'
						snapshot := session.emu.MakeSnapshot()
						ioutil.WriteFile(snapFilename, snapshot, os.FileMode(0644))
					} else if session.snapshotMode == 'l' {
						session.snapshotMode = 'x'
						snapBytes, err := ioutil.ReadFile(snapFilename)
						if err != nil {
							fmt.Println("failed to load snapshot:", err)
							continue
						}
						newEmu, err := session.emu.LoadSnapshot(snapBytes)
						if err != nil {
							fmt.Println("failed to load snapshot:", err)
							continue
						}
						session.emu = newEmu
					}
				}
				session.emu.UpdateInput(session.latestInput)
				session.emu.UpdateDbgKeyState(dbgKeyState)
			}
		}

		if session.emu.InDevMode() {
			session.emu.DbgStep()
		} else {
			session.emu.Step()
		}
		bufInfo := session.emu.GetSoundBufferInfo()
		if bufInfo.IsValid && bufInfo.UsedSize >= audioToGen {
			if cap(audioChunkBuf) < audioToGen {
				audioChunkBuf = make([]byte, audioToGen)
			}
			session.audio.Write(session.emu.ReadSoundBuffer(audioChunkBuf[:audioToGen]))
		}

		if session.emu.FlipRequested() {
			window.RenderMutex.Lock()
			copy(window.Pix, session.emu.Framebuffer())
			window.RenderMutex.Unlock()

			session.frameTimer.MarkRenderComplete()

			session.currentNumFrames++

			session.audio.WaitForPlaybackIfAhead()

			audioToGen = session.audio.GetPrevCallbackReadLen()

			session.frameTimer.MarkFrameComplete()

			if time.Since(session.lastSaveTime) > 5*time.Second {
				ram := session.emu.GetCartRAM()
				if len(ram) > 0 && !bytes.Equal(ram, session.lastSaveRAM) {
					ioutil.WriteFile(session.saveFilename, ram, os.FileMode(0644))
					session.lastSaveTime = time.Now()
					session.lastSaveRAM = ram
				}
			}
		}
	}
}

// assert checks if a condition is true, otherwise prints an error message and exits the program.
func assert(test bool, msg string) {
	if !test {
		fmt.Println(msg)
		os.Exit(1)
	}
}

// dieIf checks if an error occurred, otherwise prints the error message and exits the program.
func dieIf(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// readZipFileOrDie reads the contents of a zip file or exits the program if an error occurs.
func readZipFileOrDie(filename string) []byte {
	zipReader, err := zip.OpenReader(filename)
	dieIf(err)

	f := zipReader.File[0]
	fmt.Printf("unzipping first file found: %q\n", f.FileHeader.Name)
	cartReader, err := f.Open()
	dieIf(err)
	cartBytes, err := ioutil.ReadAll(cartReader)
	dieIf(err)

	cartReader.Close()
	zipReader.Close()
	return cartBytes
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
