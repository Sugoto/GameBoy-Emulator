package dmgo

import (
	"fmt"
)

// Defines the parameters of the GameBoy's CPU
type cpuState struct {
	PC                     uint16 // Program Counter: points to the next instruction to be executed
	SP                     uint16 // Stack Pointer: points to the top of the stack
	A, F, B, C, D, E, H, L byte   // CPU registers
	Mem                    mem    // Memory interface

	Title          string // Game title
	HeaderChecksum byte   // Checksum of the game header

	LCD lcd // LCD interface
	APU apu // Audio Processing Unit interface

	InHaltMode bool // Flag indicating if the CPU is in halt mode
	InStopMode bool // Flag indicating if the CPU is in stop mode

	OAMDMAActive bool   // Flag indicating if OAM DMA transfer is active
	OAMDMAIndex  uint16 // Index for OAM DMA transfer
	OAMDMASource uint16 // Source address for OAM DMA transfer

	CGBMode            bool // Flag indicating if the Game Boy is in Color Game Boy mode
	FastMode           bool // Flag indicating if the CPU is in fast mode
	SpeedSwitchPrepped bool // Flag indicating if a speed switch has been prepared

	IRDataReadEnable bool // Flag indicating if IR data read is enabled
	IRSendDataEnable bool // Flag indicating if IR data send is enabled

	InterruptMasterEnable bool // Flag indicating if interrupts are enabled
	MasterEnableRequested bool // Flag indicating if an interrupt enable has been requested

	VBlankInterruptEnabled  bool    // Flag indicating if VBlank interrupt is enabled
	LCDStatInterruptEnabled bool    // Flag indicating if LCD status interrupt is enabled
	TimerInterruptEnabled   bool    // Flag indicating if timer interrupt is enabled
	SerialInterruptEnabled  bool    // Flag indicating if serial interrupt is enabled
	JoypadInterruptEnabled  bool    // Flag indicating if joypad interrupt is enabled
	DummyEnableBits         [3]bool // Unused bits

	VBlankIRQ  bool // Flag indicating if a VBlank interrupt request has occurred
	LCDStatIRQ bool // Flag indicating if an LCD status interrupt request has occurred
	TimerIRQ   bool // Flag indicating if a timer interrupt request has occurred
	SerialIRQ  bool // Flag indicating if a serial interrupt request has occurred
	JoypadIRQ  bool // Flag indicating if a joypad interrupt request has occurred

	SerialTransferData            byte   // Data to be transferred via the serial port
	SerialTransferStartFlag       bool   // Flag indicating if a serial transfer has been started
	SerialTransferClockIsInternal bool   // Flag indicating if the serial transfer clock is internal
	SerialFastMode                bool   // Flag indicating if the serial transfer is in fast mode
	SerialClock                   uint16 // Clock for the serial transfer
	SerialBitsTransferred         byte   // Number of bits transferred via the serial port

	TimerOn           bool   // Flag indicating if the timer is on
	TimerLag          int    // Lag for the timer
	TimerModuloReg    byte   // Timer modulo register
	TimerCounterReg   byte   // Timer counter register
	TimerFreqSelector byte   // Timer frequency selector
	TimerDivCycles    uint16 // Divider register cycles

	Joypad Joypad // Joypad interface

	Steps  uint // Number of steps executed by the CPU
	Cycles uint // Number of cycles executed by the CPU

	devMode  bool     // Flag indicating if the emulator is in developer mode
	debugger debugger // Debugger interface
}

func (cs *cpuState) SetDevMode(b bool) { cs.devMode = b }
func (cs *cpuState) InDevMode() bool   { return cs.devMode }

func (cs *cpuState) runSerialCycle() {
	if !cs.SerialTransferStartFlag {
		cs.SerialBitsTransferred = 0
		cs.SerialClock = 0
		return
	}
	if !cs.SerialTransferClockIsInternal {
		// no real link cable, so wait forever
		// (hopefully til game times out transfer)
		return
	}
	cs.SerialClock++
	if cs.SerialClock == 512 { // 8192Hz
		cs.SerialClock = 0
		cs.SerialTransferData <<= 1
		// emulate a disconnected cable
		cs.SerialTransferData |= 0x01
		cs.SerialBitsTransferred++
		if cs.SerialBitsTransferred == 8 {
			cs.SerialBitsTransferred = 0
			cs.SerialClock = 0
			cs.SerialIRQ = true
		}
	}
}

// NOTE: timer is more complicated than this.
// See TCAGBD
func (cs *cpuState) runTimerCycle() {

	cs.TimerDivCycles++

	if !cs.TimerOn {
		return
	}
	if cs.TimerLag > 0 {
		cs.TimerLag--
		if cs.TimerLag == 0 && cs.TimerCounterReg == 0 {
			cs.TimerCounterReg = cs.TimerModuloReg
			cs.TimerIRQ = true
		}
	}

	cycleCount := [...]uint16{
		1024, 16, 64, 256,
	}[cs.TimerFreqSelector]
	if cs.TimerDivCycles&(cycleCount-1) == 0 {
		cs.TimerCounterReg++
		if cs.TimerCounterReg == 0 {
			cs.TimerLag = 4
		}
	}
}

func (cs *cpuState) readTimerControlReg() byte {
	return 0xf8 | boolBit(cs.TimerOn, 2) | cs.TimerFreqSelector
}
func (cs *cpuState) writeTimerControlReg(val byte) {
	cs.TimerOn = val&0x04 != 0
	cs.TimerFreqSelector = val & 0x03
}

func (cs *cpuState) readSerialControlReg() byte {
	return byteFromBools(
		cs.SerialTransferStartFlag,
		true,
		true,
		true,
		true,
		true,
		cs.SerialFastMode,
		cs.SerialTransferClockIsInternal,
	)
}
func (cs *cpuState) writeSerialControlReg(val byte) {
	cs.SerialTransferStartFlag = val&0x80 != 0
	cs.SerialTransferClockIsInternal = val&0x01 != 0
	if cs.CGBMode {
		cs.SerialFastMode = val&0x02 != 0
	}
}

// Joypad represents the buttons on a gameboy
type Joypad struct {
	Sel      bool
	Start    bool
	Up       bool
	Down     bool
	Left     bool
	Right    bool
	A        bool
	B        bool
	readMask byte
}

func (jp *Joypad) writeJoypadReg(val byte) {
	jp.readMask = (val >> 4) & 0x03
}
func (jp *Joypad) readJoypadReg() byte {
	val := 0xc0 | (jp.readMask << 4) | 0x0f
	if jp.readMask&0x01 == 0 {
		val &^= boolBit(jp.Down, 3)
		val &^= boolBit(jp.Up, 2)
		val &^= boolBit(jp.Left, 1)
		val &^= boolBit(jp.Right, 0)
	}
	if jp.readMask&0x02 == 0 {
		val &^= boolBit(jp.Start, 3)
		val &^= boolBit(jp.Sel, 2)
		val &^= boolBit(jp.B, 1)
		val &^= boolBit(jp.A, 0)
	}
	return val
}

func (cs *cpuState) updateJoypad(newJP Joypad) {
	lastVal := cs.Joypad.readJoypadReg() & 0x0f

	mask := cs.Joypad.readMask
	cs.Joypad = newJP
	cs.Joypad.readMask = mask

	newVal := cs.Joypad.readJoypadReg() & 0x0f
	// this is correct behavior. it only triggers irq
	// if it goes from no-buttons-pressed to any-pressed.
	if lastVal == 0x0f && newVal < lastVal {
		cs.JoypadIRQ = true
	}
}

func (cs *cpuState) writeIRPortReg(val byte) {
	cs.IRDataReadEnable = val&0xc0 == 0xc0
	cs.IRSendDataEnable = val&0x01 == 0x01
}
func (cs *cpuState) readIRPortReg() byte {
	out := byte(0)
	if cs.IRDataReadEnable {
		out |= 0xc2 // no data received
	}
	if cs.IRSendDataEnable {
		out |= 0x01
	}
	return out
}

func (cs *cpuState) writeInterruptEnableReg(val byte) {
	boolsFromByte(val,
		&cs.DummyEnableBits[2],
		&cs.DummyEnableBits[1],
		&cs.DummyEnableBits[0],
		&cs.JoypadInterruptEnabled,
		&cs.SerialInterruptEnabled,
		&cs.TimerInterruptEnabled,
		&cs.LCDStatInterruptEnabled,
		&cs.VBlankInterruptEnabled,
	)
}
func (cs *cpuState) readInterruptEnableReg() byte {
	return byteFromBools(
		cs.DummyEnableBits[2],
		cs.DummyEnableBits[1],
		cs.DummyEnableBits[0],
		cs.JoypadInterruptEnabled,
		cs.SerialInterruptEnabled,
		cs.TimerInterruptEnabled,
		cs.LCDStatInterruptEnabled,
		cs.VBlankInterruptEnabled,
	)
}

func (cs *cpuState) writeInterruptFlagReg(val byte) {
	boolsFromByte(val,
		nil, nil, nil,
		&cs.JoypadIRQ,
		&cs.SerialIRQ,
		&cs.TimerIRQ,
		&cs.LCDStatIRQ,
		&cs.VBlankIRQ,
	)
}
func (cs *cpuState) readInterruptFlagReg() byte {
	return byteFromBools(
		true, true, true,
		cs.JoypadIRQ,
		cs.SerialIRQ,
		cs.TimerIRQ,
		cs.LCDStatIRQ,
		cs.VBlankIRQ,
	)
}

func newState(cart []byte, devMode bool) *cpuState {
	cartInfo := ParseCartInfo(cart)
	state := cpuState{
		Title:          cartInfo.Title,
		HeaderChecksum: cartInfo.HeaderChecksum,
		Mem: mem{
			cart:                  cart,
			CartRAM:               make([]byte, cartInfo.GetRAMSize()),
			InternalRAMBankNumber: 1,
			mbc:                   makeMBC(cartInfo),
		},
		CGBMode: cartInfo.cgbOptional() || cartInfo.cgbOnly(),
		devMode: devMode,
	}
	state.init()
	return &state
}

func (cs *cpuState) init() {
	if cs.CGBMode {
		cs.setAF(0x1180)
		cs.setBC(0x0000)
		cs.setDE(0xff56)
		cs.setHL(0x000d)
	} else {
		cs.setAF(0x01b0)
		cs.setBC(0x0013)
		cs.setDE(0x00d8)
		cs.setHL(0x014d)
	}
	cs.setSP(0xfffe)
	cs.setPC(0x0100)

	cs.TimerDivCycles = 0xabcc

	cs.LCD.init(cs)
	cs.APU.init()

	cs.Mem.mbc.Init(&cs.Mem)

	cs.initIORegs()

	cs.APU.Sounds[0].RestartRequested = false
	cs.APU.Sounds[1].RestartRequested = false
	cs.APU.Sounds[2].RestartRequested = false
	cs.APU.Sounds[3].RestartRequested = false

	cs.initVRAM()
	cs.VBlankIRQ = true
}

func (cs *cpuState) initIORegs() {
	cs.write(0xff10, 0x80)
	cs.write(0xff11, 0xbf)
	cs.write(0xff12, 0xf3)
	cs.write(0xff14, 0xbf)
	cs.write(0xff17, 0x3f)
	cs.write(0xff19, 0xbf)
	cs.write(0xff1a, 0x7f)
	cs.write(0xff1b, 0xff)
	cs.write(0xff1c, 0x9f)
	cs.write(0xff1e, 0xbf)
	cs.write(0xff20, 0xff)
	cs.write(0xff23, 0xbf)
	cs.write(0xff24, 0x77)
	cs.write(0xff25, 0xf3)
	cs.write(0xff26, 0xf1)

	cs.write(0xff40, 0x91)
	cs.write(0xff47, 0xfc)
	cs.write(0xff48, 0xff)
	cs.write(0xff49, 0xff)
}

func (cs *cpuState) initVRAM() {
	nibbleLookup := []byte{
		0x00, 0x03, 0x0c, 0x0f, 0x30, 0x33, 0x3c, 0x3f,
		0xc0, 0xc3, 0xcc, 0xcf, 0xf0, 0xf3, 0xfc, 0xff,
	}

	hdrTileData := []byte{}
	for i := 0x104; i < 0x104+48; i++ {
		packed := cs.read(uint16(i))
		b1, b2 := nibbleLookup[packed>>4], nibbleLookup[packed&0x0f]
		hdrTileData = append(hdrTileData, b1, 0, b1, 0, b2, 0, b2, 0)
	}

	// append boot rom tile data
	hdrTileData = append(hdrTileData,
		0x3c, 0x00, 0x42, 0x00, 0xb9, 0x00, 0xa5, 0x00, 0xb9, 0x00, 0xa5, 0x00, 0x42, 0x00, 0x3c, 0x00,
	)

	bootTileMap := []byte{
		0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c,
		0x19, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
	}

	for i := range hdrTileData {
		cs.write(uint16(0x8010+i), hdrTileData[i])
	}
	for i := range bootTileMap {
		cs.write(uint16(0x9900+i), bootTileMap[i])
	}
}

func (cs *cpuState) runOAMDMACycle() {
	i := cs.OAMDMAIndex
	addr := cs.OAMDMASource
	cs.write(0xfe00+i, cs.read(addr+i))
	cs.OAMDMAIndex++
	if cs.OAMDMAIndex == 0xa0 {
		cs.OAMDMAActive = false
	}
}

func (cs *cpuState) runCycles(numCycles uint) {
	// Things that speed up to match fast mode
	for i := uint(0); i < numCycles; i++ {
		cs.Cycles++
		cs.runTimerCycle()
		cs.runSerialCycle()
		if cs.OAMDMAActive {
			cs.runOAMDMACycle()
		}
	}
	if cs.FastMode {
		numCycles >>= 1
	}
	// Things that don't speed up with fast mode
	for i := uint(0); i < numCycles; i++ {
		cs.APU.runCycle(cs)
		cs.LCD.runCycle(cs)
	}
}

func (cs *cpuState) readSpeedSwitchReg() byte {
	return byteFromBools(cs.FastMode,
		true, true, true,
		true, true, true,
		cs.SpeedSwitchPrepped,
	)
}
func (cs *cpuState) writeSpeedSwitchReg(val byte) {
	cs.SpeedSwitchPrepped = val&0x01 == 0x01
}
func (cs *cpuState) handleSpeedSwitching() {
	// TODO: accurate timing
	if cs.SpeedSwitchPrepped {
		cs.SpeedSwitchPrepped = false
		cs.FastMode = !cs.FastMode
	}
}

// Emulator exposes the public facing fns for an emulation session
type Emulator interface {
	Step()

	Framebuffer() []byte
	FlipRequested() bool

	UpdateInput(input Input)
	ReadSoundBuffer([]byte) []byte
	GetSoundBufferInfo() SoundBufferInfo

	GetCartRAM() []byte
	SetCartRAM([]byte) error

	MakeSnapshot() []byte
	LoadSnapshot([]byte) (Emulator, error)

	InDevMode() bool
	SetDevMode(b bool)
	UpdateDbgKeyState([]bool)
	DbgStep()
}

func (cs *cpuState) UpdateDbgKeyState(keys []bool) {
	cs.debugger.updateInput(keys)
}

func (cs *cpuState) MakeSnapshot() []byte {
	return cs.makeSnapshot()
}

func (cs *cpuState) LoadSnapshot(snapBytes []byte) (Emulator, error) {
	return cs.loadSnapshot(snapBytes)
}

// NewEmulator creates an emulation session
func NewEmulator(cart []byte, devMode bool) Emulator {
	return newState(cart, devMode)
}

// Input covers all outside info sent to the Emulator
type Input struct {
	Joypad Joypad
}

// ReadSoundBuffer returns a 44100hz * 16bit * 2ch sound buffer.
// A pre-sized buffer must be provided, which is returned resized
// if the buffer was less full than the length requested.
func (cs *cpuState) ReadSoundBuffer(toFill []byte) []byte {
	return cs.APU.readSoundBuffer(toFill)
}

// SoundBufferInfo gives info about the sound buffer. IsValid is used to
// handle Emulator impls that don't have sound, e.g. errEmu
type SoundBufferInfo struct {
	UsedSize int
	IsValid  bool
}

// GetSoundBufferLen gets the current size of the filled sound buffer.
func (cs *cpuState) GetSoundBufferInfo() SoundBufferInfo {
	return SoundBufferInfo{
		IsValid:  true,
		UsedSize: int(cs.APU.buffer.size()),
	}
}

// GetCartRAM returns the current state of external RAM
func (cs *cpuState) GetCartRAM() []byte {
	return append([]byte{}, cs.Mem.CartRAM...)
}

// SetCartRAM attempts to set the RAM, returning error if size not correct
func (cs *cpuState) SetCartRAM(ram []byte) error {
	if len(cs.Mem.CartRAM) == len(ram) {
		copy(cs.Mem.CartRAM, ram)
		return nil
	}
	// TODO: better checks if possible (e.g. real format, cart title/checksum, etc.)
	return fmt.Errorf("ram size mismatch")
}

func (cs *cpuState) UpdateInput(input Input) {
	cs.updateJoypad(input.Joypad)
}

// Framebuffer returns the current state of the lcd screen
func (cs *cpuState) Framebuffer() []byte {
	return cs.LCD.framebuffer[:]
}

// FlipRequested indicates if a draw request is pending
// and clears it before returning
func (cs *cpuState) FlipRequested() bool {
	val := cs.LCD.FlipRequested
	cs.LCD.FlipRequested = false
	return val
}

var lastSP = int(-1)

func (cs *cpuState) debugLineOnStackChange() {
	if lastSP != int(cs.SP) {
		lastSP = int(cs.SP)
		fmt.Println(cs.DebugStatusLine())
	}
}

// Step steps the emulator one instruction
func (cs *cpuState) Step() {
	cs.step()
}
func (cs *cpuState) DbgStep() {
	cs.debugger.step(cs)
}

var hitTarget = false

func (cs *cpuState) step() {
	ieAndIfFlagMatch := cs.handleInterrupts()
	if cs.InHaltMode {
		if ieAndIfFlagMatch {
			cs.runCycles(4)
			cs.InHaltMode = false
		} else {
			cs.runCycles(4)
			return
		}
	}

	// cs.debugLineOnStackChange()
	// if cs.Steps&0x2ffff == 0 {
	// if cs.PC == 0x4d19 {
	// 	hitTarget = true
	// }
	// if hitTarget {
	// 	fmt.Println(cs.DebugStatusLine())
	// }
	// fmt.Fprintln(os.Stderr, cs.DebugStatusLine())

	// TODO: correct behavior, i.e. only resume on
	// button press if not about to switch speeds.
	if cs.InStopMode {
		cs.TimerDivCycles = 0
		cs.handleSpeedSwitching()
		cs.runCycles(4)
		cs.InStopMode = false
	}

	// this is here to lag behind the request by
	// one instruction.
	if cs.MasterEnableRequested {
		cs.MasterEnableRequested = false
		cs.InterruptMasterEnable = true
	}

	cs.Steps++

	cs.stepOpcode()
}

func fatalErr(v ...interface{}) {
	fmt.Println(v...)
	panic("fatalErr()")
}
