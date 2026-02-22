package emu

// isWindowPixel returns whether the given screen position falls within the window region.
// reg 17: H boundary (bit 7 = right side, bits 4:0 = boundary in 16px units)
// reg 18: V boundary (bit 7 = bottom side, bits 4:0 = boundary in 8px units)
func (v *VDP) isWindowPixel(screenX, line int) bool {
	// Horizontal window boundary
	hReg := v.regs[17]
	hRight := hReg&0x80 != 0
	hBoundary := int(hReg&0x1F) * 16

	// Vertical window boundary
	vReg := v.regs[18]
	vBottom := vReg&0x80 != 0
	vBoundary := int(vReg&0x1F) * 8

	// If both boundaries are 0, no window
	if hBoundary == 0 && vBoundary == 0 {
		return false
	}

	// Evaluate each constraint independently
	var inH, inV bool
	hActive := hBoundary != 0
	vActive := vBoundary != 0

	if hActive {
		if hRight {
			inH = screenX >= hBoundary
		} else {
			inH = screenX < hBoundary
		}
	}

	if vActive {
		if vBottom {
			inV = line >= vBoundary
		} else {
			inV = line < vBoundary
		}
	}

	// If only one constraint is active, it solely determines the window
	if !hActive {
		return inV
	}
	if !vActive {
		return inH
	}
	// Both active: window covers the union
	return inH || inV
}

// windowNametableBase returns the VRAM base address for the Window nametable.
func (v *VDP) windowNametableBase() uint16 {
	if v.h40Mode() {
		// H40: bit 1 is masked out (must be 0 for valid addresses)
		return uint16(v.regs[3]&0x3C) << 10
	}
	return uint16(v.regs[3]&0x3E) << 10
}

// windowNametableWidth returns the width of the window nametable in cells.
func (v *VDP) windowNametableWidth() int {
	if v.h40Mode() {
		return 64
	}
	return 32
}

// getWindowPixel returns the pixel from the Window nametable at the given screen position.
// The window has no scrolling - screen position maps directly to nametable cells.
func (v *VDP) getWindowPixel(screenX, line int) layerPixel {
	ntBase := v.windowNametableBase()
	ntWidth := v.windowNametableWidth()
	tileRows := v.tileRows()
	tileSz := v.tileSize()

	// Shift and mask split pixel coords into cell index and pixel offset
	// (equivalent to divmod by 8 and tileRows). See renderPlaneB for details.
	cellX := screenX >> 3   // screenX / 8
	pixX := screenX & 7     // screenX % 8
	tileRowShift := uint(3) // log2(8) = 3
	if tileRows == 16 {
		tileRowShift = 4 // log2(16) = 4
	}
	cellY := line >> tileRowShift // line / tileRows
	pixY := line & (tileRows - 1) // line % tileRows

	ntAddr := (ntBase + uint16(cellY*ntWidth+cellX)*2) & 0xFFFF
	entryHi := v.vram[ntAddr]
	entryLo := v.vram[(ntAddr+1)&0xFFFF]
	entry := uint16(entryHi)<<8 | uint16(entryLo)

	priority := entry&0x8000 != 0
	pal := uint8((entry >> 13) & 0x03)
	vFlip := entry&0x1000 != 0
	hFlip := entry&0x0800 != 0
	tileIndex := entry & 0x07FF

	tileAddr := tileIndex * tileSz
	colorIdx := v.decodeTilePixel(tileAddr, pixX, pixY, hFlip, vFlip)

	return layerPixel{
		colorIndex: colorIdx,
		palette:    pal,
		priority:   priority,
	}
}
