# GoBoy - Game Boy Emulator

A Nintendo Game Boy (DMG) and Game Boy Color (CGB) emulator written in Go.

## Features

- **CPU:** Full SM83 instruction set (standard + CB-prefixed opcodes) running at 4.194304 MHz with cycle-accurate timing
- **PPU:** Scanline-based pixel renderer at 160×144 native resolution (~59.73 FPS), background/window/sprite layers, CGB palette support
- **APU:** 4-channel audio (2× square wave with sweep/envelope, 1× programmable wave, 1× noise via LFSR) at 44.1 kHz with DC-blocking filter
- **Memory:** Full memory map — ROM banking, 8 KB work RAM (bank-switchable on CGB), 8 KB VRAM, OAM, I/O registers, HRAM
- **Cartridge:** MBC1, MBC2, MBC3 (with RTC), and MBC5 mappers; battery-backed save RAM with auto-save
- **Extras:** Save states (snapshot slots 1–9), GBS music file playback, debugger with breakpoints and field inspection, TCP metrics server

## Requirements

- Go 1.26+
- A Game Boy ROM file (`.gb` or `.gbc`)

## Build & Run

```bash
go build -o goboy ./cmd/goboy
./goboy path/to/rom.gb
```

ZIP-compressed ROMs are also supported — the emulator will extract the first file automatically.

## Controls

| Key | Action |
|-----|--------|
| W / A / S / D | D-Pad |
| Space | A |
| Backspace | B |
| Enter | Start |
| X | Select |
| M + 1–9 | Save snapshot to slot |
| L + 1–9 | Load snapshot from slot |

## Dev Mode

Create a file named `devmode` in the working directory to enable the built-in debugger. It supports breakpoints on memory/register changes and field inspection via a text overlay.

## Performance Monitor

A companion CLI tool connects to the emulator's TCP metrics server (port 12345) and displays a live terminal dashboard with FPS, min/max/avg stats, a sparkline graph, and a target-speed progress bar.

```bash
go build -o goboy-monitor ./cmd/goboy-monitor
./goboy-monitor                  # connects to 127.0.0.1:12345
./goboy-monitor host:port        # custom address
```

Auto-reconnects if the emulator isn't running yet or restarts.

## Terminal Mode

An alternative display driver renders the Game Boy screen directly in your terminal using ANSI truecolor and Unicode half-block characters. No GUI or audio -- just the video output at ~30 FPS. Useful for SSH sessions or headless environments.

```bash
go build -o goboy-term ./cmd/goboy-term
./goboy-term path/to/rom.gb
```

Controls are the same (WASD, Space, Z, Enter, X). Press Q to quit.

## Architecture

```
cmd/goboy/               Entry point, window loop, audio, input, TCP server
cmd/goboy-monitor/       Live terminal dashboard for emulator metrics
driver/
  display.go             Display driver interface (swap backends without touching core)
  ebiten/                Ebitengine backend — windowing, audio, input, frame timing
  terminal/              Terminal backend — ANSI truecolor renderer, no GUI needed
internal/
  emulator/              Core emulator (CPU, memory, MBC, snapshots, debugger, GBS)
  video/                 PPU — scanline rendering, VRAM, OAM, palettes
  audio/                 APU — sound channels, sample generation, mixing
  util/                  Shared bit-manipulation utilities
```
