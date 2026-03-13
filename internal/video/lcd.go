package video

import (
	"sort"

	"github.com/sugoto/gameboy-emu/internal/util"
)

type CPUBridge interface {
	IsCGBMode() bool
	SetVBlankIRQ()
	SetLCDStatIRQ()
	RunHblankDMA()
}

type LCD struct {
	framebuffer [160 * 144 * 4]byte

	FlipRequested bool

	PastFirstFrame bool

	VideoRAM       [0x4000]byte
	HighBankActive bool

	CGBMode                       bool
	BGPaletteRAM                  [64]byte
	BGPaletteRAMIndex             byte
	BGPaletteRAMAutoIncrement     bool
	SpritePaletteRAM              [64]byte
	SpritePaletteRAMIndex         byte
	SpritePaletteRAMAutoIncrement bool

	OAM            [160]byte
	OAMForScanline []oamEntry

	BGMask         [160]bool
	BGPriorityMask [160]bool
	SpriteMask     [160]bool

	ScrollY byte
	ScrollX byte
	WindowY byte
	WindowX byte

	PassedWindowY bool

	BackgroundPaletteReg byte
	ObjectPalette0Reg    byte
	ObjectPalette1Reg    byte

	HBlankInterrupt bool
	VBlankInterrupt bool
	OAMInterrupt    bool
	LYCInterrupt    bool

	LWY    byte
	LYReg  byte
	LYCReg byte

	InVBlank     bool
	InHBlank     bool
	AccessingOAM bool
	ReadingData  bool

	DisplayOn                   bool
	UseUpperWindowTileMap       bool
	DisplayWindow               bool
	UseLowerBGAndWindowTileData bool
	UseUpperBGTileMap           bool
	BigSprites                  bool
	DisplaySprites              bool
	BGWindowMasterEnable        bool
	BGWindowPrioritiesActive    bool

	CyclesSinceLYInc       uint
	CyclesSinceVBlankStart uint

	StatIRQSignal bool
}

func (lcd *LCD) Framebuffer() []byte { return lcd.framebuffer[:] }

func (lcd *LCD) WriteBGPaletteRAMIndexReg(val byte) {
	lcd.BGPaletteRAMIndex = val & 0x3f
	lcd.BGPaletteRAMAutoIncrement = val&0x80 != 0
}
func (lcd *LCD) ReadBGPaletteRAMIndexReg() byte {
	out := lcd.BGPaletteRAMIndex
	if lcd.BGPaletteRAMAutoIncrement {
		out |= 0x80
	}
	return out
}

func (lcd *LCD) WriteBGPaletteRAMDataReg(val byte) {
	if !lcd.DisplayOn || !lcd.ReadingData {
		lcd.BGPaletteRAM[lcd.BGPaletteRAMIndex] = val
	}
	if lcd.BGPaletteRAMAutoIncrement {
		lcd.BGPaletteRAMIndex = (lcd.BGPaletteRAMIndex + 1) & 0x3f
	}
}
func (lcd *LCD) ReadBGPaletteRAMDataReg() byte {
	if !lcd.DisplayOn || !lcd.ReadingData {
		return lcd.BGPaletteRAM[lcd.BGPaletteRAMIndex]
	}
	return 0xff
}

func (lcd *LCD) WriteSpritePaletteRAMIndexReg(val byte) {
	lcd.SpritePaletteRAMIndex = val & 0x3f
	lcd.SpritePaletteRAMAutoIncrement = val&0x80 != 0
}
func (lcd *LCD) ReadSpritePaletteRAMIndexReg() byte {
	out := lcd.SpritePaletteRAMIndex
	if lcd.SpritePaletteRAMAutoIncrement {
		out |= 0x80
	}
	return out
}

func (lcd *LCD) WriteSpritePaletteRAMDataReg(val byte) {
	if !lcd.DisplayOn || !lcd.ReadingData {
		lcd.SpritePaletteRAM[lcd.SpritePaletteRAMIndex] = val
	}
	if lcd.SpritePaletteRAMAutoIncrement {
		lcd.SpritePaletteRAMIndex = (lcd.SpritePaletteRAMIndex + 1) & 0x3f
	}
}
func (lcd *LCD) ReadSpritePaletteRAMDataReg() byte {
	if !lcd.ReadingData {
		return lcd.SpritePaletteRAM[lcd.SpritePaletteRAMIndex]
	}
	return 0xff
}

var lastOAMWarningCycles uint
var lastOAMWarningLine byte

func (lcd *LCD) WriteOAM(addr uint16, val byte) {
	if !lcd.DisplayOn || (!lcd.AccessingOAM && !lcd.ReadingData) {
		lcd.OAM[addr] = val
	} else {
		if lcd.CyclesSinceLYInc != lastOAMWarningCycles || lcd.LYReg != lastOAMWarningLine {
			lastOAMWarningCycles = lcd.CyclesSinceLYInc
			lastOAMWarningLine = lcd.LYReg
		}
	}
}
func (lcd *LCD) ReadOAM(addr uint16) byte {
	if !lcd.DisplayOn || (!lcd.AccessingOAM && !lcd.ReadingData) {
		return lcd.OAM[addr]
	}
	return 0xff
}

func (lcd *LCD) Init(cgbMode bool) {
	lcd.CGBMode = cgbMode
	lcd.BGWindowPrioritiesActive = !lcd.CGBMode
	lcd.BGWindowMasterEnable = lcd.CGBMode
	lcd.AccessingOAM = true
}

func (lcd *LCD) WriteVideoRAM(addr uint16, val byte) {
	if !lcd.DisplayOn || !lcd.ReadingData {
		if lcd.HighBankActive {
			addr += 0x2000
		}
		lcd.VideoRAM[addr] = val
	}
}
func (lcd *LCD) ReadVideoRAM(addr uint16) byte {
	if !lcd.DisplayOn || !lcd.ReadingData {
		if lcd.HighBankActive {
			addr += 0x2000
		}
		return lcd.VideoRAM[addr]
	}
	return 0xff
}

func (lcd *LCD) WriteBankReg(val byte) {
	lcd.HighBankActive = val&0x01 != 0
}
func (lcd *LCD) ReadBankReg() byte {
	if lcd.HighBankActive {
		return 0x01
	}
	return 0x00
}

func (lcd *LCD) updateStatIRQ(bridge CPUBridge) {
	lastSignal := lcd.StatIRQSignal
	lcd.StatIRQSignal = (lcd.LYCInterrupt && lcd.LYReg == lcd.LYCReg) ||
		(lcd.HBlankInterrupt && lcd.InHBlank) ||
		(lcd.OAMInterrupt && lcd.AccessingOAM) ||
		((lcd.VBlankInterrupt || lcd.OAMInterrupt) && lcd.InVBlank)
	if !lastSignal && lcd.StatIRQSignal {
		bridge.SetLCDStatIRQ()
	}
}

func (lcd *LCD) startHBlankAndDoRender(bridge CPUBridge) {
	lcd.ReadingData = false
	lcd.InHBlank = true
	lcd.renderScanline()
	lcd.updateStatIRQ(bridge)

	bridge.RunHblankDMA()
}

func (lcd *LCD) startReadData() {
	lcd.parseOAMForScanline(lcd.LYReg)
	lcd.AccessingOAM = false
	lcd.ReadingData = true
}

func (lcd *LCD) handleHBlankEnd(bridge CPUBridge) {
	lcd.CyclesSinceLYInc = 0
	lcd.InHBlank = false
	lcd.LYReg++
	if lcd.isWindowVisible() {
		lcd.LWY++
	}

	if lcd.LYReg == 144 && !lcd.InVBlank {
		lcd.InVBlank = true
		bridge.SetVBlankIRQ()

		if lcd.PastFirstFrame {
			lcd.FlipRequested = true
		} else {
			lcd.PastFirstFrame = true
		}
	}
}

func (lcd *LCD) handleVBlank(bridge CPUBridge) {
	lcd.CyclesSinceVBlankStart += 4
	if lcd.CyclesSinceVBlankStart == 456*10 {
		lcd.LYReg = 0
		lcd.PassedWindowY = false
		lcd.InVBlank = false
		lcd.CyclesSinceLYInc = 0
		lcd.CyclesSinceVBlankStart = 0
	}
	lcd.updateStatIRQ(bridge)
}

func (lcd *LCD) startAccessingOAM() {
	lcd.AccessingOAM = true
	if lcd.LYReg == lcd.WindowY {
		lcd.PassedWindowY = true
		lcd.LWY = lcd.LYReg - lcd.WindowY
	}
}

func (lcd *LCD) RunCycle(bridge CPUBridge) {
	if !lcd.DisplayOn {
		return
	}

	lcd.CyclesSinceLYInc++
	if (lcd.CyclesSinceLYInc & 3) == 0 {
		switch lcd.CyclesSinceLYInc {
		case 4:
			if !lcd.InVBlank {
				lcd.startAccessingOAM()
			}
			lcd.updateStatIRQ(bridge)
		case 80:
			if lcd.AccessingOAM {
				lcd.startReadData()
			}
		case 252:
			if lcd.ReadingData {
				lcd.startHBlankAndDoRender(bridge)
			}
		case 456:
			lcd.handleHBlankEnd(bridge)
		}
		if lcd.InVBlank {
			lcd.handleVBlank(bridge)
		}
	}
}

type tileAttrs struct {
	bgPaletteNum byte
	xFlip, yFlip bool
	useHighBank  bool
	hasPriority  bool
}

func (lcd *LCD) getTilePixel(tdataAddr uint16, attr tileAttrs, tileNum, x, y byte) byte {
	if tdataAddr == 0x0800 {
		tileNum = byte(int(int8(tileNum)) + 128)
	}
	mapBitY, mapBitX := y&0x07, x&0x07
	if attr.xFlip {
		mapBitX = 7 - mapBitX
	}
	if attr.yFlip {
		mapBitY = 7 - mapBitY
	}
	if attr.useHighBank {
		tdataAddr += 0x2000
	}
	dataByteL := lcd.VideoRAM[tdataAddr+(uint16(tileNum)<<4)+(uint16(mapBitY)<<1)]
	dataByteH := lcd.VideoRAM[tdataAddr+(uint16(tileNum)<<4)+(uint16(mapBitY)<<1)+1]
	dataBitL := (dataByteL >> (7 - mapBitX)) & 0x1
	dataBitH := (dataByteH >> (7 - mapBitX)) & 0x1
	return (dataBitH << 1) | dataBitL
}
func (lcd *LCD) getTileNum(tmapAddr uint16, x, y byte) byte {
	tileNumY, tileNumX := uint16(y>>3), uint16(x>>3)
	tileNum := lcd.VideoRAM[tmapAddr+tileNumY*32+tileNumX]
	return tileNum
}
func (lcd *LCD) getTileAttrs(tmapAddr uint16, x, y byte) tileAttrs {
	if !lcd.CGBMode {
		return tileAttrs{}
	}
	attr := tileAttrs{}
	tileNumY, tileNumX := uint16(y>>3), uint16(x>>3)
	attrByte := lcd.VideoRAM[0x2000+tmapAddr+tileNumY*32+tileNumX]
	attr.bgPaletteNum = attrByte & 0x07
	util.BoolsFromByte(attrByte,
		&attr.hasPriority,
		&attr.yFlip,
		&attr.xFlip,
		nil,
		&attr.useHighBank,
		nil,
		nil,
		nil,
	)
	return attr
}

func (lcd *LCD) DumpTiles() {
	ta := tileAttrs{}
	pixData := []byte{}
	for i := 0; i < len(lcd.VideoRAM)/16; i += 16 {
		dataAddr := uint16(i / 256 * (16 * 256))
		for y := 0; y < 8; y++ {
			for j := 0; j < 16; j++ {
				for x := 0; x < 8; x++ {
					tileNum := byte(i + j)
					pix := lcd.getTilePixel(dataAddr, ta, tileNum, byte(x), byte(y))
					r, g, b := lcd.applyBGPalettes(ta, pix)
					pixData = append(pixData, []byte{r, g, b}...)
				}
			}
		}
	}
	writeTgaRGB("tiledata.tga", 16*8, len(pixData)/(16*8*3), pixData)
}

func (lcd *LCD) getBGPixel(x, y byte) (byte, tileAttrs) {
	mapAddr := lcd.getBGTileMapAddr()
	dataAddr := lcd.getBGAndWindowTileDataAddr()
	tileNum := lcd.getTileNum(mapAddr, x, y)
	ta := lcd.getTileAttrs(mapAddr, x, y)
	return lcd.getTilePixel(dataAddr, ta, tileNum, x, y), ta
}

func (lcd *LCD) getWindowPixel(x, y byte) (byte, tileAttrs) {
	mapAddr := lcd.getWindowTileMapAddr()
	dataAddr := lcd.getBGAndWindowTileDataAddr()
	tileNum := lcd.getTileNum(mapAddr, x, y)
	ta := lcd.getTileAttrs(mapAddr, x, y)
	return lcd.getTilePixel(dataAddr, ta, tileNum, x, y), ta
}

func (lcd *LCD) getSpritePixel(e *oamEntry, x, y byte) (byte, byte, byte, bool) {
	tileX := byte(int16(x) - e.x)
	tileY := byte(int16(y) - e.y)
	if e.xFlip() {
		tileX = 7 - tileX
	}
	if e.yFlip() {
		tileY = e.height - 1 - tileY
	}
	tileNum := e.tileNum
	if e.height == 16 {
		tileNum &^= 0x01
		if tileY >= 8 {
			tileNum++
		}
	}
	ta := tileAttrs{
		useHighBank: e.cgbUseHighBank(),
	}
	rawPixel := lcd.getTilePixel(0x0000, ta, tileNum, tileX, tileY)
	if rawPixel == 0 {
		return 0, 0, 0, false
	}
	r, g, b := lcd.applySpritePalettes(e, rawPixel)
	return r, g, b, true
}

func cgbToRGB(cgbColor uint16) (byte, byte, byte) {
	r := byte(cgbColor&0x1f) << 3
	g := byte(cgbColor>>5) << 3
	b := byte(cgbColor>>10) << 3
	return r, g, b
}

func (lcd *LCD) applySpritePalettes(e *oamEntry, rawPixel byte) (byte, byte, byte) {
	if lcd.CGBMode {
		palNum := e.cgbPalNumber()
		cVal := uint16(lcd.SpritePaletteRAM[8*palNum+2*rawPixel])
		cVal |= uint16(lcd.SpritePaletteRAM[8*palNum+2*rawPixel+1]) << 8
		return cgbToRGB(cVal)
	}
	palReg := lcd.ObjectPalette0Reg
	if e.palSelector() {
		palReg = lcd.ObjectPalette1Reg
	}
	return lcd.applyCustomPalette((palReg >> (rawPixel * 2)) & 0x03)
}

var standardPalette = [][]byte{
	{0x00, 0x00, 0x00},
	{0x55, 0x55, 0x55},
	{0xaa, 0xaa, 0xaa},
	{0xff, 0xff, 0xff},
}

func (lcd *LCD) applyCustomPalette(val byte) (byte, byte, byte) {
	outVal := standardPalette[3-val]
	return outVal[0], outVal[1], outVal[2]
}

func (lcd *LCD) getBGTileMapAddr() uint16 {
	if lcd.UseUpperBGTileMap {
		return 0x1c00
	}
	return 0x1800
}

func (lcd *LCD) getWindowTileMapAddr() uint16 {
	if lcd.UseUpperWindowTileMap {
		return 0x1c00
	}
	return 0x1800
}

func (lcd *LCD) getBGAndWindowTileDataAddr() uint16 {
	if lcd.UseLowerBGAndWindowTileData {
		return 0x0000
	}
	return 0x0800
}

type oamEntry struct {
	y         int16
	x         int16
	height    byte
	tileNum   byte
	flagsByte byte
}

func (e *oamEntry) behindBG() bool    { return e.flagsByte&0x80 != 0 }
func (e *oamEntry) yFlip() bool       { return e.flagsByte&0x40 != 0 }
func (e *oamEntry) xFlip() bool       { return e.flagsByte&0x20 != 0 }
func (e *oamEntry) palSelector() bool { return e.flagsByte&0x10 != 0 }

func (e *oamEntry) cgbUseHighBank() bool { return e.flagsByte&0x08 != 0 }
func (e *oamEntry) cgbPalNumber() byte   { return e.flagsByte & 0x07 }

func yInSprite(y byte, spriteY int16, height int) bool {
	return int16(y) >= spriteY && int16(y) < spriteY+int16(height)
}
func (lcd *LCD) parseOAMForScanline(scanline byte) {
	height := 8
	if lcd.BigSprites {
		height = 16
	}

	lcd.OAMForScanline = lcd.OAMForScanline[:0]

	for i := 0; len(lcd.OAMForScanline) < 10 && i < 40; i++ {
		addr := i * 4
		spriteY := int16(lcd.OAM[addr]) - 16
		if yInSprite(scanline, spriteY, height) {
			lcd.OAMForScanline = append(lcd.OAMForScanline, oamEntry{
				y:         spriteY,
				x:         int16(lcd.OAM[addr+1]) - 8,
				height:    byte(height),
				tileNum:   lcd.OAM[addr+2],
				flagsByte: lcd.OAM[addr+3],
			})
		}
	}

	if !lcd.CGBMode {
		sort.Stable(sortableOAM(lcd.OAMForScanline))
	}
}

type sortableOAM []oamEntry

func (s sortableOAM) Less(i, j int) bool { return s[i].x < s[j].x }
func (s sortableOAM) Len() int           { return len(s) }
func (s sortableOAM) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (lcd *LCD) isWindowVisible() bool {
	return lcd.WindowY <= 143 && lcd.WindowX <= 166
}

func (lcd *LCD) renderScanline() {
	if lcd.LYReg >= 144 {
		return
	}
	lcd.fillScanline(0)

	y := lcd.LYReg

	for i := 0; i < 160; i++ {
		lcd.BGMask[i] = false
		lcd.BGPriorityMask[i] = false
		lcd.SpriteMask[i] = false
	}

	if lcd.BGWindowMasterEnable {

		mightDrawWindow := lcd.DisplayWindow && lcd.PassedWindowY
		winStartX := int(lcd.WindowX) - 7

		bgEndX := 160
		if mightDrawWindow && winStartX < bgEndX {
			bgEndX = winStartX
		}

		bgY := y + lcd.ScrollY
		for x := 0; x < bgEndX; x++ {
			bgX := byte(x) + lcd.ScrollX
			pixel, attrs := lcd.getBGPixel(bgX, bgY)
			if pixel != 0 {
				lcd.BGMask[x] = true
			}
			if attrs.hasPriority {
				lcd.BGPriorityMask[x] = true
			}
			r, g, b := lcd.applyBGPalettes(attrs, pixel)
			lcd.setFramebufferPixel(byte(x), y, r, g, b)
		}

		if mightDrawWindow {
			x := winStartX
			if x < 0 {
				x = 0
			}
			for ; x < 160; x++ {
				pixel, attrs := lcd.getWindowPixel(byte(x-winStartX), lcd.LWY)
				if pixel != 0 {
					lcd.BGMask[x] = true
				}
				if attrs.hasPriority {
					lcd.BGPriorityMask[x] = true
				}
				r, g, b := lcd.applyBGPalettes(attrs, pixel)
				lcd.setFramebufferPixel(byte(x), y, r, g, b)
			}
		}
	}

	if lcd.DisplaySprites {
		for i := range lcd.OAMForScanline {
			e := &lcd.OAMForScanline[i]
			lcd.renderSpriteAtScanline(e, y)
		}
	}
}

func (lcd *LCD) applyBGPalettes(attrs tileAttrs, rawPixel byte) (byte, byte, byte) {
	if lcd.CGBMode {
		cVal := uint16(lcd.BGPaletteRAM[8*attrs.bgPaletteNum+2*rawPixel])
		cVal |= uint16(lcd.BGPaletteRAM[8*attrs.bgPaletteNum+2*rawPixel+1]) << 8
		return cgbToRGB(cVal)
	}
	palettedPixel := (lcd.BackgroundPaletteReg >> (rawPixel * 2)) & 0x03
	return lcd.applyCustomPalette(palettedPixel)
}

func (lcd *LCD) renderSpriteAtScanline(e *oamEntry, y byte) {
	startX := byte(0)
	if e.x > 0 {
		startX = byte(e.x)
	}
	endX := byte(e.x + 8)
	for x := startX; x < endX && x < 160; x++ {
		if !lcd.SpriteMask[x] {
			if r, g, b, a := lcd.getSpritePixel(e, x, y); a {
				lcd.SpriteMask[x] = true
				hideSprite := lcd.BGWindowPrioritiesActive && (lcd.BGPriorityMask[x] || e.behindBG()) && lcd.BGMask[x]
				if !hideSprite {
					lcd.setFramebufferPixel(x, y, r, g, b)
				}
			}
		}
	}
}

func (lcd *LCD) getFramebufferPixel(xByte, yByte byte) (byte, byte, byte) {
	x, y := int(xByte), int(yByte)
	yIdx := y * 160 * 4
	r := lcd.framebuffer[yIdx+x*4+0]
	g := lcd.framebuffer[yIdx+x*4+1]
	b := lcd.framebuffer[yIdx+x*4+2]
	return r, g, b
}
func (lcd *LCD) setFramebufferPixel(xByte, yByte, r, g, b byte) {
	x, y := int(xByte), int(yByte)
	yIdx := y * 160 * 4
	lcd.framebuffer[yIdx+x*4+0] = r
	lcd.framebuffer[yIdx+x*4+1] = g
	lcd.framebuffer[yIdx+x*4+2] = b
	lcd.framebuffer[yIdx+x*4+3] = 0xff
}
func (lcd *LCD) fillScanline(pixel byte) {
	r, g, b := lcd.applyBGPalettes(tileAttrs{}, pixel)
	yIdx := int(lcd.LYReg) * 160 * 4
	for x := 0; x < 160; x++ {
		lcd.framebuffer[yIdx+x*4+0] = r
		lcd.framebuffer[yIdx+x*4+1] = g
		lcd.framebuffer[yIdx+x*4+2] = b
		lcd.framebuffer[yIdx+x*4+3] = 0xff
	}
}

func (lcd *LCD) WriteScrollY(val byte)              { lcd.ScrollY = val }
func (lcd *LCD) WriteScrollX(val byte)              { lcd.ScrollX = val }
func (lcd *LCD) WriteLycReg(val byte)               { lcd.LYCReg = val }
func (lcd *LCD) WriteLyReg(val byte)                { lcd.LYReg = val }
func (lcd *LCD) WriteBackgroundPaletteReg(val byte) { lcd.BackgroundPaletteReg = val }
func (lcd *LCD) WriteObjectPalette0Reg(val byte)    { lcd.ObjectPalette0Reg = val }
func (lcd *LCD) WriteObjectPalette1Reg(val byte)    { lcd.ObjectPalette1Reg = val }
func (lcd *LCD) WriteWindowY(val byte)              { lcd.WindowY = val }
func (lcd *LCD) WriteWindowX(val byte)              { lcd.WindowX = val }

func (lcd *LCD) WriteControlReg(val byte) {
	bgBit := &lcd.BGWindowMasterEnable
	if lcd.CGBMode {
		bgBit = &lcd.BGWindowPrioritiesActive
	}
	util.BoolsFromByte(val,
		&lcd.DisplayOn,
		&lcd.UseUpperWindowTileMap,
		&lcd.DisplayWindow,
		&lcd.UseLowerBGAndWindowTileData,
		&lcd.UseUpperBGTileMap,
		&lcd.BigSprites,
		&lcd.DisplaySprites,
		bgBit,
	)

	if !lcd.DisplayOn {
		lcd.PastFirstFrame = false
		lcd.LYReg = 0
	}
}
func (lcd *LCD) ReadControlReg() byte {
	bgBit := lcd.BGWindowMasterEnable
	if lcd.CGBMode {
		bgBit = lcd.BGWindowPrioritiesActive
	}
	return util.ByteFromBools(
		lcd.DisplayOn,
		lcd.UseUpperWindowTileMap,
		lcd.DisplayWindow,
		lcd.UseLowerBGAndWindowTileData,
		lcd.UseUpperBGTileMap,
		lcd.BigSprites,
		lcd.DisplaySprites,
		bgBit,
	)
}

func (lcd *LCD) WriteStatusReg(val byte) {
	util.BoolsFromByte(val,
		nil,
		&lcd.LYCInterrupt,
		&lcd.OAMInterrupt,
		&lcd.VBlankInterrupt,
		&lcd.HBlankInterrupt,
		nil,
		nil,
		nil,
	)
}
func (lcd *LCD) ReadStatusReg() byte {
	return util.ByteFromBools(
		true,
		lcd.LYCInterrupt,
		lcd.OAMInterrupt,
		lcd.VBlankInterrupt,
		lcd.HBlankInterrupt,
		lcd.DisplayOn && (lcd.LYReg == lcd.LYCReg),
		lcd.DisplayOn && (lcd.AccessingOAM || lcd.ReadingData),
		lcd.DisplayOn && (lcd.InVBlank || lcd.ReadingData),
	)
}
