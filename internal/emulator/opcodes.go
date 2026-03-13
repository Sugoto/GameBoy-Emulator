package emulator

import "fmt"

func (cs *cpuState) setOp8(reg *uint8, val uint8, flags uint16) {
	*reg = val
	cs.setFlags(flags)
}

func (cs *cpuState) setALUOp(val uint8, flags uint16) {
	cs.A = val
	cs.setFlags(flags)
}

func setOpA(cs *cpuState, val uint8) { cs.A = val }
func setOpB(cs *cpuState, val uint8) { cs.B = val }
func setOpC(cs *cpuState, val uint8) { cs.C = val }
func setOpD(cs *cpuState, val uint8) { cs.D = val }
func setOpE(cs *cpuState, val uint8) { cs.E = val }
func setOpL(cs *cpuState, val uint8) { cs.L = val }
func setOpH(cs *cpuState, val uint8) { cs.H = val }

func (cs *cpuState) setOp16(cycles uint, setFn func(uint16), val uint16, flags uint16) {
	cs.runCycles(cycles)
	setFn(val)
	cs.setFlags(flags)
}

func (cs *cpuState) jmpRel8(test bool, relAddr int8) {
	if test {
		cs.runCycles(4)
		cs.PC = uint16(int(cs.PC) + int(relAddr))
	}
}
func (cs *cpuState) jmpAbs16(test bool, addr uint16) {
	if test {
		cs.runCycles(4)
		cs.PC = addr
	}
}

func (cs *cpuState) jmpCall(test bool, addr uint16) {
	if test {
		cs.pushOp16(cs.PC)
		cs.PC = addr
	}
}
func (cs *cpuState) jmpRet(test bool) {
	cs.runCycles(4)
	if test {
		cs.popOp16(cs.setPC)
		cs.runCycles(4)
	}
}

func zFlag(val uint8) uint16 {
	if val == 0 {
		return 0x1000
	}
	return 0x0000
}

func hFlagAdd(val, addend uint8) uint16 {

	if int(val&0x0f)+int(addend&0x0f) >= 0x10 {
		return 0x10
	}
	return 0x00
}

func hFlagAdc(val, addend, fReg uint8) uint16 {
	carry := (fReg >> 4) & 0x01

	if int(carry)+int(val&0x0f)+int(addend&0x0f) >= 0x10 {
		return 0x10
	}
	return 0x00
}

func hFlagAdd16(val, addend uint16) uint16 {

	if int(val&0x0fff)+int(addend&0x0fff) >= 0x1000 {
		return 0x10
	}
	return 0x00
}

func hFlagSub(val, subtrahend uint8) uint16 {
	if int(val&0xf)-int(subtrahend&0xf) < 0 {
		return 0x10
	}
	return 0x00
}

func hFlagSbc(val, subtrahend, fReg uint8) uint16 {
	carry := (fReg >> 4) & 0x01
	if int(val&0xf)-int(subtrahend&0xf)-int(carry) < 0 {
		return 0x10
	}
	return 0x00
}

func cFlagAdd(val, addend uint8) uint16 {
	if int(val)+int(addend) > 0xff {
		return 0x1
	}
	return 0x0
}

func cFlagAdc(val, addend, fReg uint8) uint16 {
	carry := (fReg >> 4) & 0x01
	if int(carry)+int(val)+int(addend) > 0xff {
		return 0x1
	}
	return 0x0
}

func cFlagAdd16(val, addend uint16) uint16 {
	if int(val)+int(addend) > 0xffff {
		return 0x1
	}
	return 0x0
}

func cFlagSub(val, subtrahend uint8) uint16 {
	if int(val)-int(subtrahend) < 0 {
		return 0x1
	}
	return 0x0
}
func cFlagSbc(val, subtrahend, fReg uint8) uint16 {
	carry := (fReg >> 4) & 0x01
	if int(val)-int(subtrahend)-int(carry) < 0 {
		return 0x1
	}
	return 0x0
}

func (cs *cpuState) pushOp16(val uint16) {
	cs.runCycles(4)

	cs.cpuWrite(cs.SP-1, byte(val>>8))
	cs.cpuWrite(cs.SP-2, byte(val))
	cs.SP -= 2
}
func (cs *cpuState) popOp16(setFn func(val uint16)) {
	setFn(cs.cpuRead16(cs.SP))
	cs.SP += 2
}

func (cs *cpuState) incOpReg(reg *byte) {
	val := *reg
	*reg = val + 1
	cs.setFlags(zFlag(val+1) | hFlagAdd(val, 1) | 0x0002)
}
func (cs *cpuState) incOpHL() {
	val := cs.cpuRead(cs.getHL())
	cs.cpuWrite(cs.getHL(), val+1)
	cs.setFlags(zFlag(val+1) | hFlagAdd(val, 1) | 0x0002)
}

func (cs *cpuState) decOpReg(reg *byte) {
	val := *reg
	*reg = val - 1
	cs.setFlags(zFlag(val-1) | hFlagSub(val, 1) | 0x0102)
}
func (cs *cpuState) decOpHL() {
	val := cs.cpuRead(cs.getHL())
	cs.cpuWrite(cs.getHL(), val-1)
	cs.setFlags(zFlag(val-1) | hFlagSub(val, 1) | 0x0102)
}

func (cs *cpuState) daaOp() {

	newCarryFlag := uint16(0)
	if cs.getSubFlag() {
		diff := byte(0)
		if cs.getHalfCarryFlag() {
			diff += 0x06
		}
		if cs.getCarryFlag() {
			newCarryFlag = 0x0001
			diff += 0x60
		}
		cs.A -= diff
	} else {
		diff := byte(0)
		if cs.A&0x0f > 0x09 || cs.getHalfCarryFlag() {
			diff += 0x06
		}
		if cs.A > 0x99 || cs.getCarryFlag() {
			newCarryFlag = 0x0001
			diff += 0x60
		}
		cs.A += diff
	}

	cs.setFlags(zFlag(cs.A) | 0x0200 | newCarryFlag)
}

func (cs *cpuState) ifToString() string {
	out := []byte("     ")
	if cs.VBlankIRQ {
		out[0] = 'V'
	}
	if cs.LCDStatIRQ {
		out[1] = 'L'
	}
	if cs.TimerIRQ {
		out[2] = 'T'
	}
	if cs.SerialIRQ {
		out[3] = 'S'
	}
	if cs.JoypadIRQ {
		out[4] = 'J'
	}
	return string(out)
}
func (cs *cpuState) ieToString() string {
	out := []byte("     ")
	if cs.VBlankInterruptEnabled {
		out[0] = 'V'
	}
	if cs.LCDStatInterruptEnabled {
		out[1] = 'L'
	}
	if cs.TimerInterruptEnabled {
		out[2] = 'T'
	}
	if cs.SerialInterruptEnabled {
		out[3] = 'S'
	}
	if cs.JoypadInterruptEnabled {
		out[4] = 'J'
	}
	return string(out)
}
func (cs *cpuState) imeToString() string {
	if cs.InterruptMasterEnable {
		return "1"
	}
	return "0"
}
func (cs *cpuState) DebugStatusLine() string {

	return fmt.Sprintf("Step:%08d, ", cs.Steps) +
		fmt.Sprintf("Cycles:%08d, ", cs.Cycles) +
		fmt.Sprintf("(*PC)[0:2]:%02x%02x%02x, ", cs.read(cs.PC), cs.read(cs.PC+1), cs.read(cs.PC+2)) +
		fmt.Sprintf("(*SP):%04x, ", cs.read16(cs.SP)) +
		fmt.Sprintf("[PC:%04x ", cs.PC) +
		fmt.Sprintf("SP:%04x ", cs.SP) +
		fmt.Sprintf("AF:%04x ", cs.getAF()) +
		fmt.Sprintf("BC:%04x ", cs.getBC()) +
		fmt.Sprintf("DE:%04x ", cs.getDE()) +
		fmt.Sprintf("HL:%04x ", cs.getHL()) +
		fmt.Sprintf("IME:%v ", cs.imeToString()) +
		fmt.Sprintf("IE:%v ", cs.ieToString()) +
		fmt.Sprintf("IF:%v ", cs.ifToString()) +
		fmt.Sprintf("LY:%02x ", cs.LCD.LYReg) +
		fmt.Sprintf("LYC:%02x ", cs.LCD.LYCReg) +
		fmt.Sprintf("LC:%02x ", cs.LCD.ReadControlReg()) +
		fmt.Sprintf("LS:%02x ", cs.LCD.ReadStatusReg()) +
		fmt.Sprintf("ROM:%d]", cs.Mem.mbc.GetROMBankNumber())
}

func addOpA(cs *cpuState, val byte) {
	cs.setALUOp(cs.A+val, (zFlag(cs.A+val) | hFlagAdd(cs.A, val) | cFlagAdd(cs.A, val)))
}
func adcOpA(cs *cpuState, val byte) {
	carry := (cs.F >> 4) & 0x01
	cs.setALUOp(cs.A+val+carry, (zFlag(cs.A+val+carry) | hFlagAdc(cs.A, val, cs.F) | cFlagAdc(cs.A, val, cs.F)))
}
func subOpA(cs *cpuState, val byte) {
	cs.setALUOp(cs.A-val, (zFlag(cs.A-val) | 0x100 | hFlagSub(cs.A, val) | cFlagSub(cs.A, val)))
}
func sbcOpA(cs *cpuState, val byte) {
	carry := (cs.F >> 4) & 0x01
	cs.setALUOp(cs.A-val-carry, (zFlag(cs.A-val-carry) | 0x100 | hFlagSbc(cs.A, val, cs.F) | cFlagSbc(cs.A, val, cs.F)))
}
func andOpA(cs *cpuState, val byte) {
	cs.setALUOp(cs.A&val, (zFlag(cs.A&val) | 0x010))
}
func xorOpA(cs *cpuState, val byte) {
	cs.setALUOp(cs.A^val, zFlag(cs.A^val))
}
func orOpA(cs *cpuState, val byte) {
	cs.setALUOp(cs.A|val, zFlag(cs.A|val))
}
func cpOp(cs *cpuState, val byte) {
	cs.setFlags(zFlag(cs.A-val) | hFlagSub(cs.A, val) | cFlagSub(cs.A, val) | 0x0100)
}

func (cs *cpuState) callOp(callAddr uint16) {
	cs.pushOp16(cs.PC)
	cs.PC = callAddr
}

func (cs *cpuState) getRegFromOpBits(opBits byte) *byte {
	switch opBits {
	case 0:
		return &cs.B
	case 1:
		return &cs.C
	case 2:
		return &cs.D
	case 3:
		return &cs.E
	case 4:
		return &cs.H
	case 5:
		return &cs.L
	case 6:
		return nil
	case 7:
		return &cs.A
	}
	panic("getRegFromOpBits: unknown bits passed")
}

func (cs *cpuState) getValFromOpBits(opcode byte) byte {
	if reg := cs.getRegFromOpBits(opcode & 0x07); reg != nil {
		return *reg
	}
	return cs.cpuRead(cs.getHL())
}

var isSimpleOp = []bool{
	false, false, false, false, false, false, false, false,
	true, true, true, true, true, true, false, true,
	true, true, true, true, true, true, true, true,
	false, false, false, false, false, false, false, false,
}

var simpleOpFnTable = []func(*cpuState, byte){
	nil, nil, nil, nil, nil, nil, nil, nil,
	setOpB, setOpC, setOpD, setOpE, setOpH, setOpL, nil, setOpA,
	addOpA, adcOpA, subOpA, sbcOpA, andOpA, xorOpA, orOpA, cpOp,
}

func (cs *cpuState) cpuRead(addr uint16) byte {
	cs.runCycles(4)
	return cs.read(addr)
}

func (cs *cpuState) cpuWrite(addr uint16, val byte) {
	cs.runCycles(4)
	cs.write(addr, val)
}

func (cs *cpuState) cpuRead16(addr uint16) uint16 {
	lsb := cs.cpuRead(addr)
	msb := cs.cpuRead(addr + 1)
	return (uint16(msb) << 8) | uint16(lsb)
}

func (cs *cpuState) cpuWrite16(addr uint16, val uint16) {
	cs.cpuWrite(addr, byte(val))
	cs.cpuWrite(addr+1, byte(val>>8))
}

func (cs *cpuState) cpuReadAndIncPC() byte {
	val := cs.cpuRead(cs.PC)
	cs.PC++
	return val
}

func (cs *cpuState) cpuReadAndIncPC16() uint16 {
	lsb := cs.cpuRead(cs.PC)
	cs.PC++
	msb := cs.cpuRead(cs.PC)
	cs.PC++
	return (uint16(msb) << 8) | uint16(lsb)
}

func (cs *cpuState) stepOpcode() {

	opcode := cs.read(cs.PC)
	cs.PC++

	sel := opcode >> 3
	if isSimpleOp[sel] {
		val := cs.getValFromOpBits(opcode)
		simpleOpFnTable[sel](cs, val)
		cs.runCycles(4)
		return
	}

	switch opcode {

	case 0x00:

	case 0x01:
		cs.setBC(cs.cpuReadAndIncPC16())
	case 0x02:
		cs.cpuWrite(cs.getBC(), cs.A)
	case 0x03:
		cs.runCycles(4)
		cs.setBC(cs.getBC() + 1)
	case 0x04:
		cs.incOpReg(&cs.B)
	case 0x05:
		cs.decOpReg(&cs.B)
	case 0x06:
		cs.B = cs.cpuReadAndIncPC()
	case 0x07:
		cs.rlcaOp()

	case 0x08:
		cs.cpuWrite16(cs.cpuReadAndIncPC16(), cs.SP)
	case 0x09:
		v1, v2 := cs.getHL(), cs.getBC()
		cs.setOp16(4, cs.setHL, v1+v2, (0x2000 | hFlagAdd16(v1, v2) | cFlagAdd16(v1, v2)))
	case 0x0a:
		cs.A = cs.cpuRead(cs.getBC())
	case 0x0b:
		cs.runCycles(4)
		cs.setBC(cs.getBC() - 1)
	case 0x0c:
		cs.incOpReg(&cs.C)
	case 0x0d:
		cs.decOpReg(&cs.C)
	case 0x0e:
		cs.C = cs.cpuReadAndIncPC()
	case 0x0f:
		cs.rrcaOp()

	case 0x10:
		cs.InStopMode = true
	case 0x11:
		cs.setDE(cs.cpuReadAndIncPC16())
	case 0x12:
		cs.cpuWrite(cs.getDE(), cs.A)
	case 0x13:
		cs.runCycles(4)
		cs.setDE(cs.getDE() + 1)
	case 0x14:
		cs.incOpReg(&cs.D)
	case 0x15:
		cs.decOpReg(&cs.D)
	case 0x16:
		cs.D = cs.cpuReadAndIncPC()
	case 0x17:
		cs.rlaOp()

	case 0x18:
		cs.jmpRel8(true, int8(cs.cpuReadAndIncPC()))
	case 0x19:
		v1, v2 := cs.getHL(), cs.getDE()
		cs.setOp16(4, cs.setHL, v1+v2, (0x2000 | hFlagAdd16(v1, v2) | cFlagAdd16(v1, v2)))
	case 0x1a:
		cs.A = cs.cpuRead(cs.getDE())
	case 0x1b:
		cs.runCycles(4)
		cs.setDE(cs.getDE() - 1)
	case 0x1c:
		cs.incOpReg(&cs.E)
	case 0x1d:
		cs.decOpReg(&cs.E)
	case 0x1e:
		cs.E = cs.cpuReadAndIncPC()
	case 0x1f:
		cs.rraOp()

	case 0x20:
		cs.jmpRel8(!cs.getZeroFlag(), int8(cs.cpuReadAndIncPC()))
	case 0x21:
		cs.setHL(cs.cpuReadAndIncPC16())
	case 0x22:
		cs.cpuWrite(cs.getHL(), cs.A)
		cs.setHL(cs.getHL() + 1)
	case 0x23:
		cs.runCycles(4)
		cs.setHL(cs.getHL() + 1)
	case 0x24:
		cs.incOpReg(&cs.H)
	case 0x25:
		cs.decOpReg(&cs.H)
	case 0x26:
		cs.H = cs.cpuReadAndIncPC()
	case 0x27:
		cs.daaOp()

	case 0x28:
		cs.jmpRel8(cs.getZeroFlag(), int8(cs.cpuReadAndIncPC()))
	case 0x29:
		v1, v2 := cs.getHL(), cs.getHL()
		cs.setOp16(4, cs.setHL, v1+v2, (0x2000 | hFlagAdd16(v1, v2) | cFlagAdd16(v1, v2)))
	case 0x2a:
		cs.A = cs.cpuRead(cs.getHL())
		cs.setHL(cs.getHL() + 1)
	case 0x2b:
		cs.runCycles(4)
		cs.setHL(cs.getHL() - 1)
	case 0x2c:
		cs.incOpReg(&cs.L)
	case 0x2d:
		cs.decOpReg(&cs.L)
	case 0x2e:
		cs.L = cs.cpuReadAndIncPC()
	case 0x2f:
		cs.setOp8(&cs.A, ^cs.A, 0x2112)

	case 0x30:
		cs.jmpRel8(!cs.getCarryFlag(), int8(cs.cpuReadAndIncPC()))
	case 0x31:
		cs.SP = cs.cpuReadAndIncPC16()
	case 0x32:
		cs.cpuWrite(cs.getHL(), cs.A)
		cs.setHL(cs.getHL() - 1)
	case 0x33:
		cs.runCycles(4)
		cs.SP++
	case 0x34:
		cs.incOpHL()
	case 0x35:
		cs.decOpHL()
	case 0x36:
		cs.cpuWrite(cs.getHL(), cs.cpuReadAndIncPC())
	case 0x37:
		cs.setFlags(0x2001)

	case 0x38:
		cs.jmpRel8(cs.getCarryFlag(), int8(cs.cpuReadAndIncPC()))
	case 0x39:
		v1, v2 := cs.getHL(), cs.SP
		cs.setOp16(4, cs.setHL, v1+v2, (0x2000 | hFlagAdd16(v1, v2) | cFlagAdd16(v1, v2)))
	case 0x3a:
		cs.A = cs.cpuRead(cs.getHL())
		cs.setHL(cs.getHL() - 1)
	case 0x3b:
		cs.runCycles(4)
		cs.SP--
	case 0x3c:
		cs.incOpReg(&cs.A)
	case 0x3d:
		cs.decOpReg(&cs.A)
	case 0x3e:
		cs.A = cs.cpuReadAndIncPC()
	case 0x3f:
		carry := uint16((cs.F>>4)&0x01) ^ 0x01
		cs.setFlags(0x2000 | carry)

	case 0x70:
		cs.cpuWrite(cs.getHL(), cs.B)
	case 0x71:
		cs.cpuWrite(cs.getHL(), cs.C)
	case 0x72:
		cs.cpuWrite(cs.getHL(), cs.D)
	case 0x73:
		cs.cpuWrite(cs.getHL(), cs.E)
	case 0x74:
		cs.cpuWrite(cs.getHL(), cs.H)
	case 0x75:
		cs.cpuWrite(cs.getHL(), cs.L)
	case 0x76:
		cs.InHaltMode = true
	case 0x77:
		cs.cpuWrite(cs.getHL(), cs.A)

	case 0xc0:
		cs.jmpRet(!cs.getZeroFlag())
	case 0xc1:
		cs.popOp16(cs.setBC)
	case 0xc2:
		cs.jmpAbs16(!cs.getZeroFlag(), cs.cpuReadAndIncPC16())
	case 0xc3:
		cs.runCycles(4)
		cs.PC = cs.cpuReadAndIncPC16()
	case 0xc4:
		cs.jmpCall(!cs.getZeroFlag(), cs.cpuReadAndIncPC16())
	case 0xc5:
		cs.pushOp16(cs.getBC())
	case 0xc6:
		addOpA(cs, cs.cpuReadAndIncPC())
	case 0xc7:
		cs.callOp(0x0000)

	case 0xc8:
		cs.jmpRet(cs.getZeroFlag())
	case 0xc9:
		cs.popOp16(cs.setPC)
		cs.runCycles(4)
	case 0xca:
		cs.jmpAbs16(cs.getZeroFlag(), cs.cpuReadAndIncPC16())
	case 0xcb:
		cs.stepExtendedOpcode()
	case 0xcc:
		cs.jmpCall(cs.getZeroFlag(), cs.cpuReadAndIncPC16())
	case 0xcd:
		cs.callOp(cs.cpuReadAndIncPC16())
	case 0xce:
		adcOpA(cs, cs.cpuReadAndIncPC())
	case 0xcf:
		cs.callOp(0x0008)

	case 0xd0:
		cs.jmpRet(!cs.getCarryFlag())
	case 0xd1:
		cs.popOp16(cs.setDE)
	case 0xd2:
		cs.jmpAbs16(!cs.getCarryFlag(), cs.cpuReadAndIncPC16())
	case 0xd3:
		cs.illegalOpcode(opcode)
	case 0xd4:
		cs.jmpCall(!cs.getCarryFlag(), cs.cpuReadAndIncPC16())
	case 0xd5:
		cs.pushOp16(cs.getDE())
	case 0xd6:
		subOpA(cs, cs.cpuReadAndIncPC())
	case 0xd7:
		cs.callOp(0x0010)

	case 0xd8:
		cs.jmpRet(cs.getCarryFlag())
	case 0xd9:
		cs.popOp16(cs.setPC)
		cs.runCycles(4)
		cs.MasterEnableRequested = true
	case 0xda:
		cs.jmpAbs16(cs.getCarryFlag(), cs.cpuReadAndIncPC16())
	case 0xdb:
		cs.illegalOpcode(opcode)
	case 0xdc:
		cs.jmpCall(cs.getCarryFlag(), cs.cpuReadAndIncPC16())
	case 0xdd:
		cs.illegalOpcode(opcode)
	case 0xde:
		sbcOpA(cs, cs.cpuReadAndIncPC())
	case 0xdf:
		cs.callOp(0x0018)

	case 0xe0:
		val := cs.cpuReadAndIncPC()
		cs.cpuWrite(0xff00+uint16(val), cs.A)
	case 0xe1:
		cs.popOp16(cs.setHL)
	case 0xe2:
		val := cs.C
		cs.cpuWrite(0xff00+uint16(val), cs.A)
	case 0xe3:
		cs.illegalOpcode(opcode)
	case 0xe4:
		cs.illegalOpcode(opcode)
	case 0xe5:
		cs.pushOp16(cs.getHL())
	case 0xe6:
		andOpA(cs, cs.cpuReadAndIncPC())
	case 0xe7:
		cs.callOp(0x0020)

	case 0xe8:
		v1, v2 := cs.SP, uint16(int8(cs.cpuReadAndIncPC()))
		cs.setOp16(8, cs.setSP, v1+v2, (hFlagAdd(byte(v1), byte(v2)) | cFlagAdd(byte(v1), byte(v2))))
	case 0xe9:
		cs.PC = cs.getHL()
	case 0xea:
		cs.cpuWrite(cs.cpuReadAndIncPC16(), cs.A)
	case 0xeb:
		cs.illegalOpcode(opcode)
	case 0xec:
		cs.illegalOpcode(opcode)
	case 0xed:
		cs.illegalOpcode(opcode)
	case 0xee:
		xorOpA(cs, cs.cpuReadAndIncPC())
	case 0xef:
		cs.callOp(0x0028)

	case 0xf0:
		val := cs.cpuReadAndIncPC()
		cs.A = cs.cpuRead(0xff00 + uint16(val))
	case 0xf1:
		cs.popOp16(cs.setAF)
	case 0xf2:
		val := cs.C
		cs.A = cs.cpuRead(0xff00 + uint16(val))
	case 0xf3:
		cs.InterruptMasterEnable = false
	case 0xf4:
		cs.illegalOpcode(opcode)
	case 0xf5:
		cs.pushOp16(cs.getAF())
	case 0xf6:
		orOpA(cs, cs.cpuReadAndIncPC())
	case 0xf7:
		cs.callOp(0x0030)

	case 0xf8:
		v1, v2 := cs.SP, uint16(int8(cs.cpuReadAndIncPC()))
		cs.setOp16(4, cs.setHL, v1+v2, (hFlagAdd(byte(v1), byte(v2)) | cFlagAdd(byte(v1), byte(v2))))
	case 0xf9:
		cs.runCycles(4)
		cs.SP = cs.getHL()
	case 0xfa:
		cs.A = cs.cpuRead(cs.cpuReadAndIncPC16())
	case 0xfb:
		cs.MasterEnableRequested = true
	case 0xfc:
		cs.illegalOpcode(opcode)
	case 0xfd:
		cs.illegalOpcode(opcode)
	case 0xfe:
		cpOp(cs, cs.cpuReadAndIncPC())
	case 0xff:
		cs.callOp(0x0038)

	default:
		cs.stepErr(fmt.Sprintf("Unknown Opcode: 0x%02x\r\n", opcode))
	}

	cs.runCycles(4)
}

func (cs *cpuState) illegalOpcode(opcode uint8) {
	cs.stepErr(fmt.Sprintf("illegal opcode %02x", opcode))
}

func (cs *cpuState) stepExtendedOpcode() {

	extOpcode := cs.cpuReadAndIncPC()

	switch extOpcode & 0xf8 {

	case 0x00:
		cs.extSetOp(extOpcode, cs.rlcOp)
	case 0x08:
		cs.extSetOp(extOpcode, cs.rrcOp)
	case 0x10:
		cs.extSetOp(extOpcode, cs.rlOp)
	case 0x18:
		cs.extSetOp(extOpcode, cs.rrOp)
	case 0x20:
		cs.extSetOp(extOpcode, cs.slaOp)
	case 0x28:
		cs.extSetOp(extOpcode, cs.sraOp)
	case 0x30:
		cs.extSetOp(extOpcode, cs.swapOp)
	case 0x38:
		cs.extSetOp(extOpcode, cs.srlOp)

	case 0x40:
		cs.bitOp(extOpcode, 0)
	case 0x48:
		cs.bitOp(extOpcode, 1)
	case 0x50:
		cs.bitOp(extOpcode, 2)
	case 0x58:
		cs.bitOp(extOpcode, 3)
	case 0x60:
		cs.bitOp(extOpcode, 4)
	case 0x68:
		cs.bitOp(extOpcode, 5)
	case 0x70:
		cs.bitOp(extOpcode, 6)
	case 0x78:
		cs.bitOp(extOpcode, 7)

	case 0x80:
		cs.bitResOp(extOpcode, 0)
	case 0x88:
		cs.bitResOp(extOpcode, 1)
	case 0x90:
		cs.bitResOp(extOpcode, 2)
	case 0x98:
		cs.bitResOp(extOpcode, 3)
	case 0xa0:
		cs.bitResOp(extOpcode, 4)
	case 0xa8:
		cs.bitResOp(extOpcode, 5)
	case 0xb0:
		cs.bitResOp(extOpcode, 6)
	case 0xb8:
		cs.bitResOp(extOpcode, 7)

	case 0xc0:
		cs.bitSetOp(extOpcode, 0)
	case 0xc8:
		cs.bitSetOp(extOpcode, 1)
	case 0xd0:
		cs.bitSetOp(extOpcode, 2)
	case 0xd8:
		cs.bitSetOp(extOpcode, 3)
	case 0xe0:
		cs.bitSetOp(extOpcode, 4)
	case 0xe8:
		cs.bitSetOp(extOpcode, 5)
	case 0xf0:
		cs.bitSetOp(extOpcode, 6)
	case 0xf8:
		cs.bitSetOp(extOpcode, 7)
	}
}

func (cs *cpuState) extSetOp(opcode byte,
	opFn func(val byte) (result byte, flags uint16)) {

	if reg := cs.getRegFromOpBits(opcode & 0x07); reg != nil {
		result, flags := opFn(*reg)
		cs.setOp8(reg, result, flags)
	} else {
		result, flags := opFn(cs.cpuRead(cs.getHL()))
		cs.cpuWrite(cs.getHL(), result)
		cs.setFlags(flags)
	}
}

func (cs *cpuState) swapOp(val byte) (byte, uint16) {
	result := val>>4 | (val&0x0f)<<4
	return result, zFlag(result)
}

func (cs *cpuState) rlaOp() {
	result, flags := cs.rlOp(cs.A)
	cs.setALUOp(result, flags&^0x1000)
}
func (cs *cpuState) rlOp(val byte) (byte, uint16) {
	result, carry := (val<<1)|((cs.F>>4)&0x01), (val >> 7)
	return result, (zFlag(result) | uint16(carry))
}

func (cs *cpuState) rraOp() {
	result, flags := cs.rrOp(cs.A)
	cs.setALUOp(result, flags&^0x1000)
}
func (cs *cpuState) rrOp(val byte) (byte, uint16) {
	result, carry := ((cs.F<<3)&0x80)|(val>>1), (val & 0x01)
	return result, (zFlag(result) | uint16(carry))
}

func (cs *cpuState) rlcaOp() {
	result, flags := cs.rlcOp(cs.A)
	cs.setALUOp(result, flags&^0x1000)
}
func (cs *cpuState) rlcOp(val byte) (byte, uint16) {
	result, carry := (val<<1)|(val>>7), val>>7
	return result, (zFlag(result) | uint16(carry))
}

func (cs *cpuState) rrcaOp() {
	result, flags := cs.rrcOp(cs.A)
	cs.setALUOp(result, flags&^0x1000)
}
func (cs *cpuState) rrcOp(val byte) (byte, uint16) {
	result, carry := (val<<7)|(val>>1), (val & 0x01)
	return result, (zFlag(result) | uint16(carry))
}

func (cs *cpuState) srlOp(val byte) (byte, uint16) {
	result, carry := val>>1, val&0x01
	return result, (zFlag(result) | uint16(carry))
}

func (cs *cpuState) slaOp(val byte) (byte, uint16) {
	result, carry := val<<1, val>>7
	return result, (zFlag(result) | uint16(carry))
}

func (cs *cpuState) sraOp(val byte) (byte, uint16) {
	result, carry := (val&0x80)|(val>>1), val&0x01
	return result, (zFlag(result) | uint16(carry))
}

func (cs *cpuState) bitOp(opcode byte, bitNum uint8) {
	val := cs.getValFromOpBits(opcode)
	cs.setFlags(zFlag(val&(1<<bitNum)) | 0x012)
}

func (cs *cpuState) bitResOp(opcode byte, bitNum uint) {
	if reg := cs.getRegFromOpBits(opcode & 0x07); reg != nil {
		*reg = *reg &^ (1 << bitNum)
	} else {
		val := cs.cpuRead(cs.getHL())
		result := val &^ (1 << bitNum)
		cs.cpuWrite(cs.getHL(), result)
	}
}

func (cs *cpuState) bitSetOp(opcode byte, bitNum uint8) {
	if reg := cs.getRegFromOpBits(opcode & 0x07); reg != nil {
		*reg = *reg | (1 << bitNum)
	} else {
		val := cs.cpuRead(cs.getHL())
		result := val | (1 << bitNum)
		cs.cpuWrite(cs.getHL(), result)
	}
}

func (cs *cpuState) stepErr(msg string) {
	fmt.Println(msg)
	fmt.Println(cs.DebugStatusLine())
	panic("stepErr()")
}
