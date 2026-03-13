package main

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddr = "127.0.0.1:12345"
	historySize = 60
)

var sparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

type monitor struct {
	addr      string
	history   []float64
	startTime time.Time
	samples   int
}

func main() {
	addr := defaultAddr
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	m := &monitor{
		addr:      addr,
		history:   make([]float64, 0, historySize),
		startTime: time.Now(),
	}

	clearScreen()
	fmt.Printf("\033[1;36m GoBoy Monitor\033[0m\n")
	fmt.Printf(" Connecting to %s...\n", addr)

	for {
		if err := m.run(); err != nil {
			fmt.Printf("\r\033[K \033[31m disconnected: %v\033[0m\n", err)
			fmt.Printf(" Reconnecting in 2s...\n")
			time.Sleep(2 * time.Second)
			m.history = m.history[:0]
			m.samples = 0
			m.startTime = time.Now()
		}
	}
}

func (m *monitor) run() error {
	conn, err := net.DialTimeout("tcp", m.addr, 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	clearScreen()
	m.printHeader()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		fps, err := parseFPS(scanner.Text())
		if err != nil {
			continue
		}
		m.record(fps)
		m.render()
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return fmt.Errorf("connection closed")
}

func parseFPS(line string) (float64, error) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "FPS:") {
		return 0, fmt.Errorf("unexpected format")
	}
	return strconv.ParseFloat(strings.TrimSpace(line[4:]), 64)
}

func (m *monitor) record(fps float64) {
	m.samples++
	m.history = append(m.history, fps)
	if len(m.history) > historySize {
		m.history = m.history[1:]
	}
}

func (m *monitor) render() {
	moveCursor(1, 1)
	m.printHeader()

	cur := m.history[len(m.history)-1]
	mn, mx, avg := m.stats()
	uptime := time.Since(m.startTime).Truncate(time.Second)

	fpsColor := "\033[32m"
	if cur < 30 {
		fpsColor = "\033[31m"
	} else if cur < 55 {
		fpsColor = "\033[33m"
	}

	fmt.Printf("\033[K  Status    \033[32m● connected\033[0m     Uptime  %s\n", uptime)
	fmt.Printf("\033[K  Samples   %-20d\n", m.samples)
	fmt.Println("\033[K")
	fmt.Printf("\033[K  FPS       %s%-8.1f\033[0m\n", fpsColor, cur)
	fmt.Printf("\033[K  Min       %-8.1f  Max  %-8.1f  Avg  %-8.1f\n", mn, mx, avg)
	fmt.Println("\033[K")
	fmt.Printf("\033[K  %s\n", m.sparkline())
	fmt.Printf("\033[K  \033[2m%s%s\033[0m\n", "oldest", padRight("newest", len(m.history)-6))
	fmt.Println("\033[K")

	target := 59.73
	pct := (avg / target) * 100
	bar := renderBar(pct, 40)
	fmt.Printf("\033[K  Target    59.73 FPS (GB native)\n")
	fmt.Printf("\033[K  %s %.0f%%\n", bar, pct)
}

func (m *monitor) printHeader() {
	fmt.Printf("\033[K\033[1;36m GoBoy Monitor\033[0m  \033[2m%s\033[0m\n", m.addr)
	fmt.Printf("\033[K %s\n", strings.Repeat("─", 50))
}

func (m *monitor) stats() (min, max, avg float64) {
	if len(m.history) == 0 {
		return 0, 0, 0
	}
	min = math.MaxFloat64
	var sum float64
	for _, v := range m.history {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	avg = sum / float64(len(m.history))
	return
}

func (m *monitor) sparkline() string {
	if len(m.history) == 0 {
		return ""
	}
	mn, mx, _ := m.stats()
	span := mx - mn
	if span < 1 {
		span = 1
	}
	var sb strings.Builder
	for _, v := range m.history {
		idx := int((v - mn) / span * float64(len(sparkChars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkChars) {
			idx = len(sparkChars) - 1
		}
		sb.WriteRune(sparkChars[idx])
	}
	return sb.String()
}

func renderBar(pct float64, width int) string {
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	filled := int(pct / 100 * float64(width))
	color := "\033[32m"
	if pct < 50 {
		color = "\033[31m"
	} else if pct < 90 {
		color = "\033[33m"
	}
	return fmt.Sprintf("  %s%s\033[2m%s\033[0m",
		color,
		strings.Repeat("█", filled),
		strings.Repeat("░", width-filled))
}

func padRight(s string, n int) string {
	if n <= 0 {
		return s
	}
	return strings.Repeat(" ", n) + s
}

func clearScreen()              { fmt.Print("\033[2J") }
func moveCursor(row, col int)   { fmt.Printf("\033[%d;%dH", row, col) }

func init() {
	log.SetFlags(0)
	log.SetPrefix("goboy-monitor: ")
}
