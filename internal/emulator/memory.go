package emulator

import "fmt"

type mem struct {
	cart []byte

	CartRAM               []byte
	InternalRAM           [0x8000]byte
	InternalRAMBankNumber uint16
	HighInternalRAM       [0x7f]byte
	mbc                   mbc

	DMASource     uint16
	DMASourceReg  uint16
	DMADest       uint16
	DMADestReg    uint16
	DMALength     uint16
	DMAHblankMode bool
	DMAInProgress bool
}

func (mem *mem) mbcRead(addr uint16) byte {
	return mem.mbc.Read(mem, addr)
}
func (mem *mem) mbcWrite(addr uint16, val byte) {
	mem.mbc.Write(mem, addr, val)
}

func (cs *cpuState) writeDMASourceHigh(val byte) {
	cs.Mem.DMASourceReg = (cs.Mem.DMASourceReg &^ 0xff00) | (uint16(val) << 8)
}
func (cs *cpuState) readDMASourceHigh() byte {
	return byte(cs.Mem.DMASourceReg >> 8)
}

func (cs *cpuState) writeDMASourceLow(val byte) {
	cs.Mem.DMASourceReg = (cs.Mem.DMASourceReg &^ 0xff) | uint16(val)
}
func (cs *cpuState) readDMASourceLow() byte {
	return byte(cs.Mem.DMASourceReg)
}

func (cs *cpuState) writeDMADestHigh(val byte) {
	val = (val &^ 0xe0) | 0x80
	cs.Mem.DMADestReg = (cs.Mem.DMADestReg &^ 0xff00) | (uint16(val) << 8)
}
func (cs *cpuState) readDMADestHigh() byte {
	return byte(cs.Mem.DMADestReg >> 8)
}

func (cs *cpuState) writeDMADestLow(val byte) {
	cs.Mem.DMADestReg = (cs.Mem.DMADestReg &^ 0xff) | uint16(val)
}
func (cs *cpuState) readDMADestLow() byte {
	return byte(cs.Mem.DMADestReg)
}

func (cs *cpuState) writeDMAControlReg(val byte) {
	cs.Mem.DMALength = (uint16(val&0x7f) + 1) << 4
	cs.Mem.DMAHblankMode = val&0x80 != 0
	if cs.Mem.DMAInProgress && (val&0x80 == 0) {
		cs.Mem.DMAHblankMode = false
		cs.Mem.DMAInProgress = false
		return
	}
	cs.Mem.DMAInProgress = true
	cs.Mem.DMASource = (cs.Mem.DMASourceReg & 0xfff0)
	cs.Mem.DMADest = (cs.Mem.DMADestReg & 0x1ff0) | 0x8000
	if !cs.Mem.DMAHblankMode {
		for cs.Mem.DMAInProgress {
			cs.runDMACycle()
		}
	}
}
func (cs *cpuState) readDMAControlReg() byte {
	out := byte((cs.Mem.DMALength>>4)-1) & 0x7f
	if !cs.Mem.DMAInProgress {
		out |= 0x80
	}
	return out
}

func (cs *cpuState) runDMACycle() {
	cs.write(cs.Mem.DMADest, cs.read(cs.Mem.DMASource))
	cs.write(cs.Mem.DMADest+1, cs.read(cs.Mem.DMASource+1))
	if cs.CGBMode {
		cs.runCycles(8)
	} else {
		cs.runCycles(4)
	}
	cs.Mem.DMASource += 2
	cs.Mem.DMADest += 2
	cs.Mem.DMALength -= 2
	if cs.Mem.DMALength == 0 {
		cs.Mem.DMAInProgress = false
	}
}
func (cs *cpuState) runHblankDMA() {
	if cs.Mem.DMAInProgress && cs.Mem.DMAHblankMode {
		for i := 0; cs.Mem.DMAInProgress && i < 8; i++ {
			cs.runDMACycle()
		}
	}
}

func (cs *cpuState) read(addr uint16) byte {
	var val byte
	switch {

	case addr < 0x8000:
		val = cs.Mem.mbcRead(addr)

	case addr >= 0x8000 && addr < 0xa000:
		val = cs.LCD.ReadVideoRAM(addr - 0x8000)

	case addr >= 0xa000 && addr < 0xc000:
		val = cs.Mem.mbcRead(addr)

	case addr >= 0xc000 && addr < 0xfe00:
		ramAddr := (addr - 0xc000) & 0x1fff
		if ramAddr >= 0x1000 {
			ramAddr = (ramAddr - 0x1000) + 0x1000*cs.Mem.InternalRAMBankNumber
		}
		val = cs.Mem.InternalRAM[ramAddr]

	case addr >= 0xfe00 && addr < 0xfea0:
		val = cs.LCD.ReadOAM(addr - 0xfe00)

	case addr >= 0xfea0 && addr < 0xff00:
		val = 0xff

	case addr == 0xff00:
		val = cs.Joypad.readJoypadReg()
	case addr == 0xff01:
		val = cs.SerialTransferData
	case addr == 0xff02:
		val = cs.readSerialControlReg()

	case addr == 0xff03:
		val = 0xff

	case addr == 0xff04:
		val = byte(cs.TimerDivCycles >> 8)
	case addr == 0xff05:
		val = cs.TimerCounterReg
	case addr == 0xff06:
		val = cs.TimerModuloReg
	case addr == 0xff07:
		val = cs.readTimerControlReg()

	case addr >= 0xff08 && addr < 0xff0f:
		val = 0xff

	case addr == 0xff0f:
		val = cs.readInterruptFlagReg()

	case addr == 0xff10:
		val = cs.APU.Sounds[0].ReadSweepReg()
	case addr == 0xff11:
		val = cs.APU.Sounds[0].ReadLenDutyReg()
	case addr == 0xff12:
		val = cs.APU.Sounds[0].ReadSoundEnvReg()
	case addr == 0xff13:
		val = cs.APU.Sounds[0].ReadFreqLowReg()
	case addr == 0xff14:
		val = cs.APU.Sounds[0].ReadFreqHighReg()

	case addr == 0xff15:
		val = 0xff
	case addr == 0xff16:
		val = cs.APU.Sounds[1].ReadLenDutyReg()
	case addr == 0xff17:
		val = cs.APU.Sounds[1].ReadSoundEnvReg()
	case addr == 0xff18:
		val = cs.APU.Sounds[1].ReadFreqLowReg()
	case addr == 0xff19:
		val = cs.APU.Sounds[1].ReadFreqHighReg()

	case addr == 0xff1a:
		val = boolBit(cs.APU.Sounds[2].On, 7) | 0x7f
	case addr == 0xff1b:
		val = cs.APU.Sounds[2].ReadLengthDataReg()
	case addr == 0xff1c:
		val = cs.APU.Sounds[2].ReadWaveOutLvlReg()
	case addr == 0xff1d:
		val = cs.APU.Sounds[2].ReadFreqLowReg()
	case addr == 0xff1e:
		val = cs.APU.Sounds[2].ReadFreqHighReg()

	case addr == 0xff1f:
		val = 0xff
	case addr == 0xff20:
		val = cs.APU.Sounds[3].ReadLengthDataReg()
	case addr == 0xff21:
		val = cs.APU.Sounds[3].ReadSoundEnvReg()
	case addr == 0xff22:
		val = cs.APU.Sounds[3].ReadPolyCounterReg()
	case addr == 0xff23:
		val = cs.APU.Sounds[3].ReadFreqHighReg()

	case addr == 0xff24:
		val = cs.APU.ReadVolumeReg()
	case addr == 0xff25:
		val = cs.APU.ReadSpeakerSelectReg()
	case addr == 0xff26:
		val = cs.APU.ReadSoundOnOffReg()

	case addr >= 0xff27 && addr < 0xff30:
		val = 0xff

	case addr >= 0xff30 && addr < 0xff40:
		val = cs.APU.Sounds[2].WavePatternRAM[addr-0xff30]

	case addr == 0xff40:
		val = cs.LCD.ReadControlReg()
	case addr == 0xff41:
		val = cs.LCD.ReadStatusReg()
	case addr == 0xff42:
		val = cs.LCD.ScrollY
	case addr == 0xff43:
		val = cs.LCD.ScrollX
	case addr == 0xff44:
		val = cs.LCD.LYReg
	case addr == 0xff45:
		val = cs.LCD.LYCReg

	case addr == 0xff46:
		val = 0xff

	case addr == 0xff47:
		val = cs.LCD.BackgroundPaletteReg
	case addr == 0xff48:
		val = cs.LCD.ObjectPalette0Reg
	case addr == 0xff49:
		val = cs.LCD.ObjectPalette1Reg
	case addr == 0xff4a:
		val = cs.LCD.WindowY
	case addr == 0xff4b:
		val = cs.LCD.WindowX

	case addr == 0xff4c:
		val = 0xff

	case addr == 0xff4d:
		if cs.CGBMode {
			val = cs.readSpeedSwitchReg()
		}

	case addr == 0xff4e:
		val = 0xff

	case addr == 0xff4f:
		if cs.CGBMode {
			val = cs.LCD.ReadBankReg()
		} else {
			val = 0xff
		}

	case addr == 0xff50:
		val = 0xff

	case addr == 0xff51:
		if cs.CGBMode {
			val = cs.readDMASourceHigh()
		}
	case addr == 0xff52:
		if cs.CGBMode {
			val = cs.readDMASourceLow()
		}
	case addr == 0xff53:
		if cs.CGBMode {
			val = cs.readDMADestHigh()
		}
	case addr == 0xff54:
		if cs.CGBMode {
			val = cs.readDMADestLow()
		}
	case addr == 0xff55:
		if cs.CGBMode {
			val = cs.readDMAControlReg()
		}

	case addr == 0xff56:
		if cs.CGBMode {
			val = cs.readIRPortReg()
		}

	case addr >= 0xff57 && addr < 0xff68:
		val = 0xff

	case addr == 0xff68:
		if cs.CGBMode {
			val = cs.LCD.ReadBGPaletteRAMIndexReg()
		} else {
			val = 0xff
		}
	case addr == 0xff69:
		if cs.CGBMode {
			val = cs.LCD.ReadBGPaletteRAMDataReg()
		} else {
			val = 0xff
		}
	case addr == 0xff6a:
		if cs.CGBMode {
			val = cs.LCD.ReadSpritePaletteRAMIndexReg()
		} else {
			val = 0xff
		}
	case addr == 0xff6b:
		if cs.CGBMode {
			val = cs.LCD.ReadSpritePaletteRAMDataReg()
		} else {
			val = 0xff
		}

	case addr >= 0xff6c && addr < 0xff70:
		val = 0xff

	case addr == 0xff70:
		if cs.CGBMode {
			val = byte(cs.Mem.InternalRAMBankNumber)
		}

	case addr >= 0xff71 && addr < 0xff80:
		val = 0xff

	case addr >= 0xff80 && addr < 0xffff:
		val = cs.Mem.HighInternalRAM[addr-0xff80]
	case addr == 0xffff:
		val = cs.readInterruptEnableReg()

	default:
		cs.stepErr(fmt.Sprintf("not implemented: read at %x\n", addr))
	}
	return val
}

func (cs *cpuState) read16(addr uint16) uint16 {
	high := uint16(cs.read(addr + 1))
	low := uint16(cs.read(addr))
	return (high << 8) | low
}

func (cs *cpuState) write(addr uint16, val byte) {
	switch {

	case addr < 0x8000:
		cs.Mem.mbcWrite(addr, val)

	case addr >= 0x8000 && addr < 0xa000:
		cs.LCD.WriteVideoRAM(addr-0x8000, val)

	case addr >= 0xa000 && addr < 0xc000:
		cs.Mem.mbcWrite(addr, val)

	case addr >= 0xc000 && addr < 0xfe00:
		ramAddr := (addr - 0xc000) & 0x1fff
		if ramAddr >= 0x1000 {
			ramAddr = (ramAddr - 0x1000) + 0x1000*cs.Mem.InternalRAMBankNumber
		}
		cs.Mem.InternalRAM[ramAddr] = val
	case addr >= 0xfe00 && addr < 0xfea0:
		cs.LCD.WriteOAM(addr-0xfe00, val)
	case addr >= 0xfea0 && addr < 0xff00:

	case addr == 0xff00:
		cs.Joypad.writeJoypadReg(val)
	case addr == 0xff01:
		cs.SerialTransferData = val
	case addr == 0xff02:
		cs.writeSerialControlReg(val)

	case addr == 0xff03:

	case addr == 0xff04:
		cs.TimerDivCycles = 0
	case addr == 0xff05:
		cs.TimerCounterReg = val
	case addr == 0xff06:
		cs.TimerModuloReg = val
	case addr == 0xff07:
		cs.writeTimerControlReg(val)

	case addr >= 0xff08 && addr < 0xff0f:

	case addr == 0xff0f:
		cs.writeInterruptFlagReg(val)

	case addr == 0xff10:
		cs.APU.Sounds[0].WriteSweepReg(val)
	case addr == 0xff11:
		cs.APU.Sounds[0].WriteLenDutyReg(val)
	case addr == 0xff12:
		cs.APU.Sounds[0].WriteSoundEnvReg(val)
	case addr == 0xff13:
		cs.APU.Sounds[0].WriteFreqLowReg(val)
	case addr == 0xff14:
		cs.APU.Sounds[0].WriteFreqHighReg(val)

	case addr == 0xff15:

	case addr == 0xff16:
		cs.APU.Sounds[1].WriteLenDutyReg(val)
	case addr == 0xff17:
		cs.APU.Sounds[1].WriteSoundEnvReg(val)
	case addr == 0xff18:
		cs.APU.Sounds[1].WriteFreqLowReg(val)
	case addr == 0xff19:
		cs.APU.Sounds[1].WriteFreqHighReg(val)

	case addr == 0xff1a:
		cs.APU.Sounds[2].WriteWaveOnOffReg(val)
	case addr == 0xff1b:
		cs.APU.Sounds[2].WriteLengthDataReg(val)
	case addr == 0xff1c:
		cs.APU.Sounds[2].WriteWaveOutLvlReg(val)
	case addr == 0xff1d:
		cs.APU.Sounds[2].WriteFreqLowReg(val)
	case addr == 0xff1e:
		cs.APU.Sounds[2].WriteFreqHighReg(val)

	case addr == 0xff1f:

	case addr == 0xff20:
		cs.APU.Sounds[3].WriteLengthDataReg(val)
	case addr == 0xff21:
		cs.APU.Sounds[3].WriteSoundEnvReg(val)
	case addr == 0xff22:
		cs.APU.Sounds[3].WritePolyCounterReg(val)
	case addr == 0xff23:
		cs.APU.Sounds[3].WriteFreqHighReg(val)

	case addr == 0xff24:
		cs.APU.WriteVolumeReg(val)
	case addr == 0xff25:
		cs.APU.WriteSpeakerSelectReg(val)
	case addr == 0xff26:
		cs.APU.WriteSoundOnOffReg(val)

	case addr >= 0xff27 && addr < 0xff30:

	case addr >= 0xff30 && addr < 0xff40:
		cs.APU.Sounds[2].WriteWavePatternValue(addr-0xff30, val)

	case addr == 0xff40:
		cs.LCD.WriteControlReg(val)
	case addr == 0xff41:
		cs.LCD.WriteStatusReg(val)
	case addr == 0xff42:
		cs.LCD.WriteScrollY(val)
	case addr == 0xff43:
		cs.LCD.WriteScrollX(val)
	case addr == 0xff44:

	case addr == 0xff45:
		cs.LCD.WriteLycReg(val)
	case addr == 0xff46:
		cs.OAMDMAIndex = 0
		cs.OAMDMAActive = true
		cs.OAMDMASource = uint16(val) << 8
	case addr == 0xff47:
		cs.LCD.WriteBackgroundPaletteReg(val)
	case addr == 0xff48:
		cs.LCD.WriteObjectPalette0Reg(val)
	case addr == 0xff49:
		cs.LCD.WriteObjectPalette1Reg(val)
	case addr == 0xff4a:
		cs.LCD.WriteWindowY(val)
	case addr == 0xff4b:
		cs.LCD.WriteWindowX(val)

	case addr == 0xff4c:

	case addr == 0xff4d:
		if cs.CGBMode {
			cs.writeSpeedSwitchReg(val)
		}

	case addr == 0xff4e:

	case addr == 0xff4f:
		if cs.CGBMode {
			cs.LCD.WriteBankReg(val)
		}

	case addr == 0xff50:

	case addr == 0xff51:
		if cs.CGBMode {
			cs.writeDMASourceHigh(val)
		}
	case addr == 0xff52:
		if cs.CGBMode {
			cs.writeDMASourceLow(val)
		}
	case addr == 0xff53:
		if cs.CGBMode {
			cs.writeDMADestHigh(val)
		}
	case addr == 0xff54:
		if cs.CGBMode {
			cs.writeDMADestLow(val)
		}
	case addr == 0xff55:
		if cs.CGBMode {
			cs.writeDMAControlReg(val)
		}

	case addr == 0xff56:
		if cs.CGBMode {
			cs.writeIRPortReg(val)
		}

	case addr >= 0xff57 && addr < 0xff68:

	case addr == 0xff68:
		if cs.CGBMode {
			cs.LCD.WriteBGPaletteRAMIndexReg(val)
		}
	case addr == 0xff69:
		if cs.CGBMode {
			cs.LCD.WriteBGPaletteRAMDataReg(val)
		}
	case addr == 0xff6a:
		if cs.CGBMode {
			cs.LCD.WriteSpritePaletteRAMIndexReg(val)
		}
	case addr == 0xff6b:
		if cs.CGBMode {
			cs.LCD.WriteSpritePaletteRAMDataReg(val)
		}

	case addr >= 0xff6c && addr < 0xff70:

	case addr == 0xff70:
		if cs.CGBMode {
			if val&0x07 == 0 {
				val = 1
			}
			cs.Mem.InternalRAMBankNumber = uint16(val) & 0x07
		}

	case addr >= 0xff71 && addr < 0xff80:

	case addr >= 0xff80 && addr < 0xffff:
		cs.Mem.HighInternalRAM[addr-0xff80] = val
	case addr == 0xffff:
		cs.writeInterruptEnableReg(val)
	default:
		cs.stepErr(fmt.Sprintf("not implemented: write(0x%04x, %v)\n", addr, val))
	}
}

func (cs *cpuState) write16(addr uint16, val uint16) {
	cs.write(addr, byte(val))
	cs.write(addr+1, byte(val>>8))
}
