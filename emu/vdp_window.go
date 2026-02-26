package emu

// windowBounds returns the X range [startX, endX) that is window for a given scanline.
// Returns (0, 0) for no window, (0, width) for all window.
// reg 17: H boundary (bit 7 = right side, bits 4:0 = boundary in 16px units)
// reg 18: V boundary (bit 7 = bottom side, bits 4:0 = boundary in 8px units)
func (v *VDP) windowBounds(line, width int) (startX, endX int) {
	hReg := v.regs[17]
	hRight := hReg&0x80 != 0
	hBoundary := int(hReg&0x1F) * 16

	vReg := v.regs[18]
	vBottom := vReg&0x80 != 0
	vBoundary := int(vReg&0x1F) * 8

	// If both boundaries are 0, no window
	if hBoundary == 0 && vBoundary == 0 {
		return 0, 0
	}

	hActive := hBoundary != 0
	vActive := vBoundary != 0

	// Determine if this line is inside the V constraint
	var inV bool
	if vActive {
		if vBottom {
			inV = line >= vBoundary
		} else {
			inV = line < vBoundary
		}
	}

	// If only V is active, it determines everything
	if !hActive {
		if inV {
			return 0, width
		}
		return 0, 0
	}

	// Compute H range
	var hStart, hEnd int
	if hRight {
		hStart = hBoundary
		hEnd = width
	} else {
		hStart = 0
		hEnd = hBoundary
	}
	// Clamp to active width
	if hStart > width {
		hStart = width
	}
	if hEnd > width {
		hEnd = width
	}

	// If only H is active, use the H range directly
	if !vActive {
		return hStart, hEnd
	}

	// Both active: window covers the union (H OR V).
	// If V matches, entire line is window.
	if inV {
		return 0, width
	}
	// V doesn't match, only the H range is window.
	return hStart, hEnd
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
