package ebiten

import (
	"fmt"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

type AudioBuffer struct {
	SamplesPerSecond int
	BitsPerSample    int
	ChannelCount     int

	buf      []byte
	bufMutex sync.Mutex

	context *audio.Context
	player  *audio.Player

	outputBufDuration time.Duration

	ReadLenNotifier chan int
	bufUnderflows   int

	prevReadLen int
	maxWaited   time.Duration
}

func (ab *AudioBuffer) GetPrevCallbackReadLen() int {
	return ab.prevReadLen
}

func (ab *AudioBuffer) GetMaxWaited() time.Duration {
	out := ab.maxWaited
	ab.maxWaited = time.Duration(0)
	return out
}

func (ab *AudioBuffer) WaitForPlaybackIfAhead() {
	start := time.Now()
	for ab.GetUnplayedDataLen() > ab.prevReadLen {
		ab.prevReadLen = <-ab.ReadLenNotifier
	}
	audioDiff := time.Since(start)
	if audioDiff > ab.maxWaited {
		ab.maxWaited = audioDiff
	}
}

type OpenAudioBufferOptions struct {
	OutputBufDuration time.Duration
	SamplesPerSecond  int
	BitsPerSample     int
	ChannelCount      int
}

func OpenAudioBuffer(opts OpenAudioBufferOptions) (*AudioBuffer, error) {
	ab := AudioBuffer{
		SamplesPerSecond:  opts.SamplesPerSecond,
		BitsPerSample:     opts.BitsPerSample,
		ChannelCount:      opts.ChannelCount,
		outputBufDuration: opts.OutputBufDuration,
		ReadLenNotifier:   make(chan int),
	}

	if ab.BitsPerSample != 16 || ab.ChannelCount != 2 {
		return nil, fmt.Errorf("platform audio: only 16-bit stereo is supported; use oto directly for other formats")
	}
	ab.context = audio.NewContext(ab.SamplesPerSecond)
	player, err := ab.context.NewPlayer(&ab)
	if err != nil {
		return nil, err
	}
	player.SetBufferSize(ab.outputBufDuration)
	player.Play()
	ab.player = player

	ab.prevReadLen = <-ab.ReadLenNotifier

	return &ab, nil
}

func (ab *AudioBuffer) GetLatestUnderflowCount() int {
	ab.bufMutex.Lock()
	count := ab.bufUnderflows
	ab.bufUnderflows = 0
	ab.bufMutex.Unlock()
	return count
}

func (ab *AudioBuffer) GetUnplayedDataLen() int {
	ab.bufMutex.Lock()
	bLen := len(ab.buf)
	ab.bufMutex.Unlock()
	return bLen
}

func (ab *AudioBuffer) Write(data []byte) (int, error) {
	ab.bufMutex.Lock()
	ab.buf = append(ab.buf, data...)
	ab.bufMutex.Unlock()
	return len(data), nil
}

func (ab *AudioBuffer) Read(buf []byte) (int, error) {
	ab.bufMutex.Lock()
	if len(buf) > len(ab.buf) {
		ab.bufUnderflows++
	}
	n := copy(buf, ab.buf)
	ab.buf = ab.buf[n:]
	ab.bufMutex.Unlock()

	if n == 0 {
		for i := range buf {
			buf[i] = 0
		}
	} else {
		for i := n + 1; i < len(buf); i++ {
			buf[i] = buf[i-1]
		}
	}

	select {
	case ab.ReadLenNotifier <- len(buf):
	default:
	}

	return len(buf), nil
}

func (ab *AudioBuffer) Close() error {
	return ab.player.Close()
}
