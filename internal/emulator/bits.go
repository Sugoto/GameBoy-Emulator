package emulator

import "github.com/sugoto/gameboy-emu/internal/util"

func boolBit(b bool, bNum byte) byte { return util.BoolBit(b, bNum) }
func byteFromBools(b7, b6, b5, b4, b3, b2, b1, b0 bool) byte {
	return util.ByteFromBools(b7, b6, b5, b4, b3, b2, b1, b0)
}
func boolsFromByte(val byte, b7, b6, b5, b4, b3, b2, b1, b0 *bool) {
	util.BoolsFromByte(val, b7, b6, b5, b4, b3, b2, b1, b0)
}
