package dmgo


import "fmt"

// Information about the game cartridge
type CartInfo struct {
	Title            string // Title is the game title (11 or 16 characters)
	ManufacturerCode string // ManufacturerCode is an optional 4-character code for the manufacturer
	CGBFlag          byte   // CGBFlag describes the compatibility with CGB (Color Game Boy) and DMG (Game Boy)
	NewLicenseeCode  string // NewLicenseeCode is used to indicate the publisher of the game
	SGBFlag          byte   // SGBFlag indicates support for Super Game Boy
	CartridgeType    byte   // CartridgeType indicates the type of cartridge (MBC-type, accessories, etc.)
	ROMSizeCode      byte   // ROMSizeCode indicates the size of the ROM in the cartridge
	RAMSizeCode      byte   // RAMSizeCode indicates the size of the RAM in the cartridge
	DestinationCode  byte   // DestinationCode shows if the game is meant for Japan or not
	OldLicenseeCode  byte   // OldLicenseeCode is the pre-SGB way to indicate the publisher. If it is 0x33, the NewLicenseeCode is used instead. SGB will not function if the old code is not 0x33.
	MaskRomVersion   byte   // MaskRomVersion is the version of the game cartridge. Usually 0x00.
	HeaderChecksum   byte   // HeaderChecksum is a checksum of the header which must be correct for the game to run
}

// GetRAMSize decodes the RAM size code into the actual size
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

// GetROMSize decodes the ROM size code into an actual size
func (ci *CartInfo) GetROMSize() uint {
	codeSizeMap := map[byte]uint{
		0x00: 32 * 1024,   // no banking
		0x01: 64 * 1024,   // 4 banks
		0x02: 128 * 1024,  // 8 banks
		0x03: 256 * 1024,  // 16 banks
		0x04: 512 * 1024,  // 32 banks
		0x05: 1024 * 1024, // 64 banks (only 63 used by MBC1)
		0x06: 2048 * 1024, // 128 banks (only 125 used by MBC1)
		0x07: 4096 * 1024, // 256 banks
		0x08: 8192 * 1024, // 512 banks
		0x52: 1152 * 1024, // 72 banks
		0x53: 1280 * 1024, // 80 banks
		0x54: 1536 * 1024, // 96 banks
	}
	if size, ok := codeSizeMap[ci.ROMSizeCode]; ok {
		return size
	}
	panic(fmt.Sprintf("unknown ROM size code 0x%02x", ci.RAMSizeCode))
}

func (ci *CartInfo) cgbOnly() bool     { return ci.CGBFlag == 0xc0 }
func (ci *CartInfo) cgbOptional() bool { return ci.CGBFlag == 0x80 }

// ParseCartInfo parses a dmg cart header
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
