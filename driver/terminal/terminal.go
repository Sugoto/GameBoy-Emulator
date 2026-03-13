package terminal

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const (
	outWidth   = 160
	charRows   = 72
	targetFPS  = 30
	FrameDelay = time.Second / targetFPS
)

type Display struct {
	pix       []byte
	keyState  [256]bool
	oldTermio *unix.Termios
}

func New() *Display {
	return &Display{}
}

func (d *Display) Run(renderWidth, renderHeight int, title string, loop func()) {
	d.pix = make([]byte, renderWidth*renderHeight*4)
	d.setupTerminal(title)
	defer d.restoreTerminal()

	go d.readInput()
	loop()
}

func (d *Display) SetPixels(src []byte) {
	copy(d.pix, src)
}

func (d *Display) IsKeyDown(c rune) bool {
	if c >= 0 && c < 256 {
		return d.keyState[byte(c)]
	}
	return false
}

func (d *Display) CopyKeyState(dest []bool) {
	copy(dest, d.keyState[:])
}

func (d *Display) Render() {
	var sb strings.Builder

	for row := 0; row < charRows; row++ {
		topY := row * 2
		botY := row*2 + 1
		fmt.Fprintf(&sb, "\033[%d;1H", row+1)
		for col := 0; col < outWidth; col++ {
			tr, tg, tb := d.getPixel(col, topY)
			br, bg, bb := d.getPixel(col, botY)
			fmt.Fprintf(&sb, "\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▀",
				tr, tg, tb, br, bg, bb)
		}
		sb.WriteString("\033[0m")
	}
	os.Stdout.WriteString(sb.String())
}

func (d *Display) getPixel(x, y int) (byte, byte, byte) {
	if y >= 144 || x >= 160 {
		return 0, 0, 0
	}
	idx := (y*160 + x) * 4
	if idx+2 >= len(d.pix) {
		return 0, 0, 0
	}
	return d.pix[idx], d.pix[idx+1], d.pix[idx+2]
}

func (d *Display) setupTerminal(title string) {
	oldState, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TIOCGETA)
	if err == nil {
		d.oldTermio = oldState
		raw := *oldState
		raw.Lflag &^= unix.ECHO | unix.ICANON
		raw.Cc[unix.VMIN] = 0
		raw.Cc[unix.VTIME] = 0
		unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TIOCSETA, &raw)
	}

	fmt.Print("\033[?1049h")
	fmt.Print("\033[?25l")
	fmt.Print("\033[2J")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		d.restoreTerminal()
		os.Exit(0)
	}()
}

func (d *Display) restoreTerminal() {
	fmt.Print("\033[?25h")
	fmt.Print("\033[?1049l")
	if d.oldTermio != nil {
		unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TIOCSETA, d.oldTermio)
	}
}

func (d *Display) readInput() {
	buf := make([]byte, 1)
	for {
		n, _ := os.Stdin.Read(buf)
		if n == 0 {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		ch := buf[0]

		for i := range d.keyState {
			d.keyState[i] = false
		}
		d.keyState[ch] = true

		if ch == 'q' {
			d.restoreTerminal()
			os.Exit(0)
		}

		time.Sleep(80 * time.Millisecond)
		for i := range d.keyState {
			d.keyState[i] = false
		}
	}
}
