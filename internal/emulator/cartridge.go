package emulator

import "fmt"

type CartInfo struct {
	Title            string
	ManufacturerCode string
	CGBFlag          byte
	NewLicenseeCode  string
	SGBFlag          byte
	CartridgeType    byte
	ROMSizeCode      byte
	RAMSizeCode      byte
	DestinationCode  byte
	OldLicenseeCode  byte
	MaskRomVersion   byte
	HeaderChecksum   byte
}

func (ci *CartInfo) GetRAMSize() uint {
	if ci.CartridgeType == 5 || ci.CartridgeType == 6 {
		return 512
	}
	codeSizeMap := map[byte]uint{
		0x00: 0,
		0x01: 2 * 1024,
		0x02: 8 * 1024,
		0x03: 32 * 1024,
		0x04: 128 * 1024,
		0x05: 64 * 1024,
	}
	if size, ok := codeSizeMap[ci.RAMSizeCode]; ok {
		return size
	}
	panic(fmt.Sprintf("unknown RAM size code 0x%02x", ci.RAMSizeCode))
}

func (ci *CartInfo) GetROMSize() uint {
	codeSizeMap := map[byte]uint{
		0x00: 32 * 1024,
		0x01: 64 * 1024,
		0x02: 128 * 1024,
		0x03: 256 * 1024,
		0x04: 512 * 1024,
		0x05: 1024 * 1024,
		0x06: 2048 * 1024,
		0x07: 4096 * 1024,
		0x08: 8192 * 1024,
		0x52: 1152 * 1024,
		0x53: 1280 * 1024,
		0x54: 1536 * 1024,
	}
	if size, ok := codeSizeMap[ci.ROMSizeCode]; ok {
		return size
	}
	panic(fmt.Sprintf("unknown ROM size code 0x%02x", ci.ROMSizeCode))
}

func (ci *CartInfo) cgbOnly() bool     { return ci.CGBFlag == 0xc0 }
func (ci *CartInfo) cgbOptional() bool { return ci.CGBFlag == 0x80 }

func ParseCartInfo(cartBytes []byte) *CartInfo {
	cart := CartInfo{}

	cart.CGBFlag = cartBytes[0x143]
	if cart.CGBFlag >= 0x80 {
		cart.Title = string(cartBytes[0x134:0x13f])
		cart.ManufacturerCode = string(cartBytes[0x13f:0x143])
	} else {
		cart.Title = string(cartBytes[0x134:0x144])
	}
	cart.Title = stripZeroes(cart.Title)
	cart.SGBFlag = cartBytes[0x146]
	cart.CartridgeType = cartBytes[0x147]
	cart.ROMSizeCode = cartBytes[0x148]
	cart.RAMSizeCode = cartBytes[0x149]
	cart.DestinationCode = cartBytes[0x14a]
	cart.OldLicenseeCode = cartBytes[0x14b]
	if cart.OldLicenseeCode == 0x33 {
		cart.NewLicenseeCode = string(cartBytes[0x144:0x146])
	}
	cart.MaskRomVersion = cartBytes[0x14c]
	cart.HeaderChecksum = cartBytes[0x14d]

	return &cart
}

func stripZeroes(s string) string {
	cursor := len(s)
	for cursor > 0 && s[cursor-1] == '\x00' {
		cursor--
	}
	return s[:cursor]
}
