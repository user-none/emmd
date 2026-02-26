package emu

// decodeTilePixel returns the color index (0-15) for a pixel within a tile.
// tileAddr is the VRAM base address of the tile (tileIndex * tileSize).
// px, py are pixel coordinates within the tile (px: 0-7, py: 0-7 normal or 0-15 interlace).
// hFlip and vFlip apply horizontal/vertical mirroring.
func (v *VDP) decodeTilePixel(tileAddr uint16, px, py int, hFlip, vFlip bool) uint8 {
	if vFlip {
		py = v.tileRows() - 1 - py
	}
	if hFlip {
		px = 7 - px
	}

	// Each row is 4 bytes. Each byte holds 2 pixels (high nibble = left, low nibble = right).
	rowAddr := (tileAddr + uint16(py*4)) & 0xFFFF
	byteOffset := uint16(px >> 1)
	b := v.vram[(rowAddr+byteOffset)&0xFFFF]

	if px&1 == 0 {
		return (b >> 4) & 0x0F // left pixel = high nibble
	}
	return b & 0x0F // right pixel = low nibble
}

// nametableSize returns the nametable dimensions in cells from register 16.
func (v *VDP) nametableSize() (hCells, vCells int) {
	hBits := v.regs[16] & 0x03
	vBits := (v.regs[16] >> 4) & 0x03

	switch hBits {
	case 0:
		hCells = 32
	case 1:
		hCells = 64
	case 3:
		hCells = 128
	default:
		hCells = 32 // invalid, treat as 32
	}

	switch vBits {
	case 0:
		vCells = 32
	case 1:
		vCells = 64
	case 3:
		vCells = 128
	default:
		vCells = 32 // invalid, treat as 32
	}

	return
}

// planeANametable returns the base VRAM address for Plane A's nametable.
func (v *VDP) planeANametable() uint16 {
	return uint16(v.regs[2]&0x38) << 10
}

// planeBNametable returns the base VRAM address for Plane B's nametable.
func (v *VDP) planeBNametable() uint16 {
	return uint16(v.regs[4]&0x07) << 13
}

// hScrollTableBase returns the VRAM base address of the H-scroll table.
func (v *VDP) hScrollTableBase() uint16 {
	return uint16(v.regs[13]&0x3F) << 10
}

// hScrollValues returns the H-scroll values for Plane A and Plane B for the given line.
func (v *VDP) hScrollValues(line int) (hScrollA, hScrollB int) {
	mode := v.regs[11] & 0x03
	base := v.hScrollTableBase()

	var offset uint16
	switch mode {
	case 0x00: // Full screen scroll
		offset = 0
	case 0x01, 0x02: // Per-cell (every 8 lines)
		offset = uint16(line&^7) * 4
	case 0x03: // Per-line
		offset = uint16(line) * 4
	}

	addr := (base + offset) & 0xFFFF
	// Plane A: first word
	hScrollA = int(int16(uint16(v.vram[addr])<<8 | uint16(v.vram[(addr+1)&0xFFFF])))
	// Plane B: second word
	addr2 := (addr + 2) & 0xFFFF
	hScrollB = int(int16(uint16(v.vram[addr2])<<8 | uint16(v.vram[(addr2+1)&0xFFFF])))

	// Only low 10 bits are significant (sign-extended from 10-bit)
	hScrollA = (hScrollA&0x3FF ^ 0x200) - 0x200
	hScrollB = (hScrollB&0x3FF ^ 0x200) - 0x200

	return
}

// vScrollValue returns the V-scroll value for the given plane and screen X column.
// planeB: false = Plane A, true = Plane B.
// Uses snapshot + change replay for mid-scanline accuracy when BeginScanline has been called.
func (v *VDP) vScrollValue(screenX int, planeB bool) int {
	vsMode := (v.regs[11] >> 2) & 0x01

	var addr int
	if vsMode == 0 {
		if planeB {
			addr = 2
		} else {
			addr = 0
		}
	} else {
		col := (screenX / 16) * 4
		if planeB {
			col += 2
		}
		addr = col
	}

	if addr+1 >= len(v.vsram) {
		return 0
	}

	// Fast path: no mid-scanline changes
	if len(v.vsramChanges) == 0 {
		return int(uint16(v.vsram[addr])<<8 | uint16(v.vsram[addr+1]))
	}

	// Full-screen mode: VDP latches at scanline start
	if vsMode == 0 {
		return int(uint16(v.vsramSnapshot[addr])<<8 | uint16(v.vsramSnapshot[addr+1]))
	}

	// Per-2-cell: replay changes up to column start pixel
	columnPixelX := (screenX / 16) * 16
	hi := v.vsramSnapshot[addr]
	lo := v.vsramSnapshot[addr+1]
	for _, c := range v.vsramChanges {
		if c.pixelX > columnPixelX {
			break
		}
		if c.addr == addr {
			hi = c.hi
			lo = c.lo
		}
	}
	return int(uint16(hi)<<8 | uint16(lo))
}
