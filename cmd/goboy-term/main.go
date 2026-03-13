package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/sugoto/gameboy-emu/driver/terminal"
	"github.com/sugoto/gameboy-emu/internal/emulator"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: goboy-term <rom-file>")
	}

	rom, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if len(rom) < 4 {
		log.Fatal("file too small to be a valid ROM")
	}

	var emu emulator.Emulator
	title := "GoBoy"

	if strings.HasPrefix(string(rom[:3]), "GBS") {
		emu = emulator.NewGbsPlayer(rom, false)
	} else {
		info := emulator.ParseCartInfo(rom)
		emu = emulator.NewEmulator(rom, false)
		title = fmt.Sprintf("GoBoy - %s", info.Title)
	}

	if data, err := os.ReadFile(os.Args[1] + ".sav"); err == nil {
		if err := emu.SetCartRAM(data); err == nil {
			log.Println("save restored")
		}
	}

	disp := terminal.New()
	disp.Run(160, 144, title, func() {
		lastRender := time.Now()
		lastSave := time.Now()
		var lastRAM []byte

		for {
			emu.Step()

			if time.Since(lastRender) > 8*time.Millisecond {
				lastRender = time.Now()
				input := emulator.Input{
					Joypad: emulator.Joypad{
						Up:    disp.IsKeyDown('w'),
						Down:  disp.IsKeyDown('s'),
						Left:  disp.IsKeyDown('a'),
						Right: disp.IsKeyDown('d'),
						A:     disp.IsKeyDown(' '),
						B:     disp.IsKeyDown('z'),
						Start: disp.IsKeyDown('\n'),
						Sel:   disp.IsKeyDown('x'),
					},
				}
				emu.UpdateInput(input)
			}

			if emu.FlipRequested() {
				disp.SetPixels(emu.Framebuffer())
				disp.Render()

				elapsed := time.Since(lastRender)
				if remaining := terminal.FrameDelay - elapsed; remaining > 0 {
					time.Sleep(remaining)
				}

				if time.Since(lastSave) > 5*time.Second {
					ram := emu.GetCartRAM()
					if len(ram) > 0 && !bytesEqual(ram, lastRAM) {
						os.WriteFile(os.Args[1]+".sav", ram, 0644)
						lastSave = time.Now()
						lastRAM = append([]byte{}, ram...)
					}
				}
			}
		}
	})
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
