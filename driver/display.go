package driver

import "time"

type Config struct {
	WindowTitle  string
	RenderWidth  int
	RenderHeight int
	WindowWidth  int
	WindowHeight int
}

type AudioConfig struct {
	OutputBufDuration time.Duration
	SamplesPerSecond  int
	BitsPerSample     int
	ChannelCount      int
}

type Display interface {
	Run(cfg Config, loop func(Display))

	SetPixels(pix []byte)

	IsKeyDown(c rune) bool
	CopyKeyState(dest []bool)

	LockInput()
	UnlockInput()

	LockRender()
	UnlockRender()
}

type Audio interface {
	Write(data []byte) (int, error)
	WaitForPlaybackIfAhead()
	GetPrevCallbackReadLen() int
	Close() error
}

type FrameTimer interface {
	MarkRenderComplete()
	MarkFrameComplete()
}
