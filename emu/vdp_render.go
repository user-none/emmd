package emu

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// layerPixel holds one layer's result for one pixel position.
type layerPixel struct {
	colorIndex uint8 // 0 = transparent
	palette    uint8 // palette line 0-3
	priority   bool  // tile/sprite priority bit
}

// cramColor converts a CRAM color index (0-63) to R, G, B values.
// CRAM stores big-endian words. Format: 0000BBB0 GGG0RRR0
// High byte cram[i*2]: Blue in bits 3:1
// Low byte cram[i*2+1]: Green in bits 7:5, Red in bits 3:1
func (v *VDP) cramColor(index uint8) (r, g, b uint8) {
	idx := int(index&0x3F) * 2
	hi := v.cram[idx]
	lo := v.cram[idx+1]

	blue := (hi >> 1) & 0x07
	green := (lo >> 5) & 0x07
	red := (lo >> 1) & 0x07

	r = (red << 5) | (red << 2) | (red >> 1)
	g = (green << 5) | (green << 2) | (green >> 1)
	b = (blue << 5) | (blue << 2) | (blue >> 1)
	return
}

// cramColorShadow returns half-brightness color values.
func (v *VDP) cramColorShadow(index uint8) (r, g, b uint8) {
	nr, ng, nb := v.cramColor(index)
	return nr >> 1, ng >> 1, nb >> 1
}

// cramColorHighlight returns 1.5x brightness color values (clamped to 255).
func (v *VDP) cramColorHighlight(index uint8) (r, g, b uint8) {
	nr, ng, nb := v.cramColor(index)
	ri := uint16(nr) + 128
	gi := uint16(ng) + 128
	bi := uint16(nb) + 128
	if ri > 255 {
		ri = 255
	}
	if gi > 255 {
		gi = 255
	}
	if bi > 255 {
		bi = 255
	}
	return uint8(ri), uint8(gi), uint8(bi)
}

// fillBackdrop fills a scanline in the framebuffer with the backdrop color.
func (v *VDP) fillBackdrop(line int) {
	pal, idx := v.backdropColor()
	r, g, b := v.cramColor(pal*16 + idx)

	pix := v.framebuffer.Pix
	stride := v.framebuffer.Stride
	offset := line * stride

	for x := 0; x < ScreenWidth; x++ {
		p := offset + x*4
		pix[p] = r
		pix[p+1] = g
		pix[p+2] = b
		pix[p+3] = 0xFF
	}
}

// fillH32Backdrop fills remaining pixels beyond the active width with backdrop color.
func (v *VDP) fillH32Backdrop(line int) {
	width := v.activeWidth()
	if width >= ScreenWidth {
		return
	}

	pal, idx := v.backdropColor()
	pix := v.framebuffer.Pix
	stride := v.framebuffer.Stride
	offset := line * stride

	if v.shadowHighlightMode() {
		br, bg, bb := v.cramColorShadow(pal*16 + idx)
		for x := width; x < ScreenWidth; x++ {
			p := offset + x*4
			pix[p] = br
			pix[p+1] = bg
			pix[p+2] = bb
			pix[p+3] = 0xFF
		}
	} else {
		br, bg, bb := v.cramColor(pal*16 + idx)
		for x := width; x < ScreenWidth; x++ {
			p := offset + x*4
			pix[p] = br
			pix[p+1] = bg
			pix[p+2] = bb
			pix[p+3] = 0xFF
		}
	}
}

// stretchScanline stretches srcWidth pixels to fill ScreenWidth in the framebuffer.
// Works right-to-left so source pixels are not overwritten before being read.
func (v *VDP) stretchScanline(line, srcWidth int) {
	pix := v.framebuffer.Pix
	stride := v.framebuffer.Stride
	offset := line * stride

	for dx := ScreenWidth - 1; dx >= 0; dx-- {
		sx := dx * srcWidth / ScreenWidth
		sp := offset + sx*4
		dp := offset + dx*4
		pix[dp] = pix[sp]
		pix[dp+1] = pix[sp+1]
		pix[dp+2] = pix[sp+2]
		pix[dp+3] = pix[sp+3]
	}
}

// brightness levels for shadow/highlight mode
const (
	brightnessShadow    = 0
	brightnessNormal    = 1
	brightnessHighlight = 2
)

// Nametable/sprite attribute entry bit fields.
// Format: P PAL[1:0] VF HF TILE[10:0]
const (
	entryPriority = 0x8000 // bit 15: priority flag
	entryPalShift = 13     // bits 14:13: palette line (shift count)
	entryPalMask  = 0x03   // palette mask after shifting
	entryVFlip    = 0x1000 // bit 12: vertical flip
	entryHFlip    = 0x0800 // bit 11: horizontal flip
	entryTileMask = 0x07FF // bits 10:0: tile index
)

// renderMergedRange renders Plane B, Plane A/Window, and composites into the
// framebuffer in a single pass for normal (non-SH) mode. Sprites are read from
// lineBufSpr which must already be rendered. Takes the source scanline, the
// framebuffer line, and the X range to render.
func (v *VDP) renderMergedRange(line, fbLine, startX, endX int) {
	// Backdrop
	bdPal, bdIdx := v.backdropColor()
	leftBlank := v.leftColumnBlank()

	// Framebuffer
	pix := v.framebuffer.Pix
	stride := v.framebuffer.Stride
	offset := fbLine * stride

	// Plane B invariants
	hCellsB, vCellsB := v.nametableSize()
	ntBaseB := v.planeBNametable()
	_, hScrollB := v.hScrollValues(line)
	tileRows := v.tileRows()
	tileSz := v.tileSize()
	ntWidthPxB := hCellsB * 8
	ntHeightPxB := vCellsB * tileRows
	hMaskB := ntWidthPxB - 1
	vMaskB := ntHeightPxB - 1
	tileRowMask := tileRows - 1
	tileRowShift := uint(3)
	if tileRows == 16 {
		tileRowShift = 4
	}

	// Plane A invariants
	hCellsA, vCellsA := v.nametableSize()
	ntBaseA := v.planeANametable()
	hScrollA, _ := v.hScrollValues(line)
	ntWidthPxA := hCellsA * 8
	ntHeightPxA := vCellsA * tileRows
	hMaskA := ntWidthPxA - 1
	vMaskA := ntHeightPxA - 1

	// Window bounds
	winStartX, winEndX := v.windowBounds(line, endX)

	// Window invariants
	winNtBase := v.windowNametableBase()
	winNtWidth := v.windowNametableWidth()

	// V-scroll: hoist out of per-pixel loop.
	// vsMode 0 = full-screen (constant for entire line)
	// vsMode 1 = per-2-cell (changes every 16 pixels)
	vsMode := (v.regs[11] >> 2) & 0x01
	vScrollA := v.vScrollValue(startX, false)
	vScrollB := v.vScrollValue(startX, true)
	prevVSCol := startX >> 4

	for x := startX; x < endX; x++ {
		// Update vscroll at 2-cell column boundaries (per-2-cell mode)
		if vsMode != 0 {
			if col := x >> 4; col != prevVSCol {
				vScrollA = v.vScrollValue(x, false)
				vScrollB = v.vScrollValue(x, true)
				prevVSCol = col
			}
		}

		// --- Plane B pixel ---
		vramXB := (x - hScrollB) & hMaskB
		vramYB := (line + vScrollB) & vMaskB
		cellXB := vramXB >> 3
		cellYB := vramYB >> tileRowShift
		pixXB := vramXB & 7
		pixYB := vramYB & tileRowMask

		ntAddrB := (ntBaseB + uint16(cellYB*hCellsB+cellXB)*2) & 0xFFFF
		entryB := uint16(v.vram[ntAddrB])<<8 | uint16(v.vram[(ntAddrB+1)&0xFFFF])
		bPri := entryB&entryPriority != 0
		bPal := uint8((entryB >> entryPalShift) & entryPalMask)
		bColorIdx := v.decodeTilePixel(
			(entryB&entryTileMask)*tileSz, pixXB, pixYB,
			entryB&entryHFlip != 0, entryB&entryVFlip != 0,
		)

		// --- Plane A / Window pixel ---
		var aPri bool
		var aPal, aColorIdx uint8

		if x >= winStartX && x < winEndX {
			// Window pixel (no scrolling)
			cellXW := x >> 3
			pixXW := x & 7
			cellYW := line >> tileRowShift
			pixYW := line & tileRowMask

			ntAddrW := (winNtBase + uint16(cellYW*winNtWidth+cellXW)*2) & 0xFFFF
			entryW := uint16(v.vram[ntAddrW])<<8 | uint16(v.vram[(ntAddrW+1)&0xFFFF])
			aPri = entryW&entryPriority != 0
			aPal = uint8((entryW >> entryPalShift) & entryPalMask)
			aColorIdx = v.decodeTilePixel(
				(entryW&entryTileMask)*tileSz, pixXW, pixYW,
				entryW&entryHFlip != 0, entryW&entryVFlip != 0,
			)
		} else {
			// Plane A pixel
			vramXA := (x - hScrollA) & hMaskA
			vramYA := (line + vScrollA) & vMaskA
			cellXA := vramXA >> 3
			cellYA := vramYA >> tileRowShift
			pixXA := vramXA & 7
			pixYA := vramYA & tileRowMask

			ntAddrA := (ntBaseA + uint16(cellYA*hCellsA+cellXA)*2) & 0xFFFF
			entryA := uint16(v.vram[ntAddrA])<<8 | uint16(v.vram[(ntAddrA+1)&0xFFFF])
			aPri = entryA&entryPriority != 0
			aPal = uint8((entryA >> entryPalShift) & entryPalMask)
			aColorIdx = v.decodeTilePixel(
				(entryA&entryTileMask)*tileSz, pixXA, pixYA,
				entryA&entryHFlip != 0, entryA&entryVFlip != 0,
			)
		}

		// --- Sprite pixel ---
		spr := v.lineBufSpr[x]

		// --- Priority resolution ---
		// Priority order (highest to lowest):
		// 1. High-priority sprite (non-transparent)
		// 2. High-priority Plane A/Window (non-transparent)
		// 3. High-priority Plane B (non-transparent)
		// 4. Low-priority sprite (non-transparent)
		// 5. Low-priority Plane A/Window (non-transparent)
		// 6. Low-priority Plane B (non-transparent)
		// 7. Backdrop
		var cpal, cidx uint8
		switch {
		case spr.priority && spr.colorIndex != 0:
			cpal, cidx = spr.palette, spr.colorIndex
		case aPri && aColorIdx != 0:
			cpal, cidx = aPal, aColorIdx
		case bPri && bColorIdx != 0:
			cpal, cidx = bPal, bColorIdx
		case !spr.priority && spr.colorIndex != 0:
			cpal, cidx = spr.palette, spr.colorIndex
		case !aPri && aColorIdx != 0:
			cpal, cidx = aPal, aColorIdx
		case !bPri && bColorIdx != 0:
			cpal, cidx = bPal, bColorIdx
		default:
			cpal, cidx = bdPal, bdIdx
		}

		r, g, bv := v.cramColor(cpal*16 + cidx)

		if leftBlank && x < 8 {
			r, g, bv = v.cramColor(bdPal*16 + bdIdx)
		}

		p := offset + x*4
		pix[p] = r
		pix[p+1] = g
		pix[p+2] = bv
		pix[p+3] = 0xFF
	}
}

// renderMergedSHRange renders Plane B, Plane A/Window, and composites into the
// framebuffer in a single pass with shadow/highlight mode. Sprites are read from
// lineBufSpr which must already be rendered.
func (v *VDP) renderMergedSHRange(line, fbLine, startX, endX int) {
	bdPal, bdIdx := v.backdropColor()
	leftBlank := v.leftColumnBlank()

	pix := v.framebuffer.Pix
	stride := v.framebuffer.Stride
	offset := fbLine * stride

	// Plane B invariants
	hCellsB, vCellsB := v.nametableSize()
	ntBaseB := v.planeBNametable()
	_, hScrollB := v.hScrollValues(line)
	tileRows := v.tileRows()
	tileSz := v.tileSize()
	ntWidthPxB := hCellsB * 8
	ntHeightPxB := vCellsB * tileRows
	hMaskB := ntWidthPxB - 1
	vMaskB := ntHeightPxB - 1
	tileRowMask := tileRows - 1
	tileRowShift := uint(3)
	if tileRows == 16 {
		tileRowShift = 4
	}

	// Plane A invariants
	hCellsA, vCellsA := v.nametableSize()
	ntBaseA := v.planeANametable()
	hScrollA, _ := v.hScrollValues(line)
	ntWidthPxA := hCellsA * 8
	ntHeightPxA := vCellsA * tileRows
	hMaskA := ntWidthPxA - 1
	vMaskA := ntHeightPxA - 1

	// Window bounds
	winStartX, winEndX := v.windowBounds(line, endX)

	// Window invariants
	winNtBase := v.windowNametableBase()
	winNtWidth := v.windowNametableWidth()

	// V-scroll: hoist out of per-pixel loop.
	vsMode := (v.regs[11] >> 2) & 0x01
	vScrollA := v.vScrollValue(startX, false)
	vScrollB := v.vScrollValue(startX, true)
	prevVSCol := startX >> 4

	for x := startX; x < endX; x++ {
		// Update vscroll at 2-cell column boundaries (per-2-cell mode)
		if vsMode != 0 {
			if col := x >> 4; col != prevVSCol {
				vScrollA = v.vScrollValue(x, false)
				vScrollB = v.vScrollValue(x, true)
				prevVSCol = col
			}
		}

		// --- Plane B pixel ---
		vramXB := (x - hScrollB) & hMaskB
		vramYB := (line + vScrollB) & vMaskB
		cellXB := vramXB >> 3
		cellYB := vramYB >> tileRowShift
		pixXB := vramXB & 7
		pixYB := vramYB & tileRowMask

		ntAddrB := (ntBaseB + uint16(cellYB*hCellsB+cellXB)*2) & 0xFFFF
		entryB := uint16(v.vram[ntAddrB])<<8 | uint16(v.vram[(ntAddrB+1)&0xFFFF])
		bPri := entryB&entryPriority != 0
		bPal := uint8((entryB >> entryPalShift) & entryPalMask)
		bColorIdx := v.decodeTilePixel(
			(entryB&entryTileMask)*tileSz, pixXB, pixYB,
			entryB&entryHFlip != 0, entryB&entryVFlip != 0,
		)

		// --- Plane A / Window pixel ---
		var aPri bool
		var aPal, aColorIdx uint8

		if x >= winStartX && x < winEndX {
			cellXW := x >> 3
			pixXW := x & 7
			cellYW := line >> tileRowShift
			pixYW := line & tileRowMask

			ntAddrW := (winNtBase + uint16(cellYW*winNtWidth+cellXW)*2) & 0xFFFF
			entryW := uint16(v.vram[ntAddrW])<<8 | uint16(v.vram[(ntAddrW+1)&0xFFFF])
			aPri = entryW&entryPriority != 0
			aPal = uint8((entryW >> entryPalShift) & entryPalMask)
			aColorIdx = v.decodeTilePixel(
				(entryW&entryTileMask)*tileSz, pixXW, pixYW,
				entryW&entryHFlip != 0, entryW&entryVFlip != 0,
			)
		} else {
			vramXA := (x - hScrollA) & hMaskA
			vramYA := (line + vScrollA) & vMaskA
			cellXA := vramXA >> 3
			cellYA := vramYA >> tileRowShift
			pixXA := vramXA & 7
			pixYA := vramYA & tileRowMask

			ntAddrA := (ntBaseA + uint16(cellYA*hCellsA+cellXA)*2) & 0xFFFF
			entryA := uint16(v.vram[ntAddrA])<<8 | uint16(v.vram[(ntAddrA+1)&0xFFFF])
			aPri = entryA&entryPriority != 0
			aPal = uint8((entryA >> entryPalShift) & entryPalMask)
			aColorIdx = v.decodeTilePixel(
				(entryA&entryTileMask)*tileSz, pixXA, pixYA,
				entryA&entryHFlip != 0, entryA&entryVFlip != 0,
			)
		}

		// --- Sprite pixel ---
		spr := v.lineBufSpr[x]

		// --- Shadow/Highlight priority resolution ---
		// Same priority order as normal mode. All pixels default to shadow
		// brightness. High-priority layers promote to normal brightness.
		// Palette 3 low-priority sprites with color 14 or 15 are operators:
		// they modify the underlying pixel's brightness without displaying.
		brightness := brightnessShadow
		var cpal, cidx uint8
		sprIsOperator := false

		// Palette 3 sprite operator check (color 14 = highlight, 15 = shadow)
		if !spr.priority && spr.colorIndex != 0 && spr.palette == 3 {
			if spr.colorIndex == 14 || spr.colorIndex == 15 {
				sprIsOperator = true
			}
		}

		switch {
		case spr.priority && spr.colorIndex != 0:
			// High-priority sprite: always normal brightness
			cpal, cidx = spr.palette, spr.colorIndex
			brightness = brightnessNormal
		case aPri && aColorIdx != 0:
			// High-priority Plane A/Window: normal brightness
			cpal, cidx = aPal, aColorIdx
			brightness = brightnessNormal
		case bPri && bColorIdx != 0:
			// High-priority Plane B: normal brightness
			cpal, cidx = bPal, bColorIdx
			brightness = brightnessNormal
		case !spr.priority && spr.colorIndex != 0:
			if sprIsOperator {
				// Operator: select underlying plane/backdrop pixel,
				// then apply brightness modification
				switch {
				case aColorIdx != 0:
					cpal, cidx = aPal, aColorIdx
				case bColorIdx != 0:
					cpal, cidx = bPal, bColorIdx
				default:
					cpal, cidx = bdPal, bdIdx
				}
				if spr.colorIndex == 14 {
					brightness = brightnessHighlight
				} else {
					brightness = brightnessShadow
				}
			} else if spr.palette == 3 {
				// Palette 3 non-operator: normal brightness
				cpal, cidx = spr.palette, spr.colorIndex
				brightness = brightnessNormal
			} else {
				// Other low-priority sprites: remain at shadow brightness
				cpal, cidx = spr.palette, spr.colorIndex
			}
		case aColorIdx != 0:
			// Low-priority Plane A/Window: remains at shadow brightness
			cpal, cidx = aPal, aColorIdx
		case bColorIdx != 0:
			// Low-priority Plane B: remains at shadow brightness
			cpal, cidx = bPal, bColorIdx
		default:
			cpal, cidx = bdPal, bdIdx
		}

		var r, g, bv uint8
		switch brightness {
		case brightnessShadow:
			r, g, bv = v.cramColorShadow(cpal*16 + cidx)
		case brightnessNormal:
			r, g, bv = v.cramColor(cpal*16 + cidx)
		case brightnessHighlight:
			r, g, bv = v.cramColorHighlight(cpal*16 + cidx)
		}

		if leftBlank && x < 8 {
			r, g, bv = v.cramColorShadow(bdPal*16 + bdIdx)
		}

		p := offset + x*4
		pix[p] = r
		pix[p+1] = g
		pix[p+2] = bv
		pix[p+3] = 0xFF
	}
}

// renderMergedScanline is the merged rendering entry point used by RenderScanline.
// It clears only the sprite buffer, renders sprites, then performs plane rendering
// and compositing in a single pass.
func (v *VDP) renderMergedScanline(line, fbLine int) {
	width := v.activeWidth()

	// Only clear sprite buffer (planes are computed inline)
	for i := 0; i < width; i++ {
		v.lineBufSpr[i] = layerPixel{}
	}

	// Render sprites into lineBufSpr (unchanged - sprites traverse SAT in link order)
	v.renderSprites(line)

	if len(v.cramChanges) == 0 {
		// Fast path: no mid-scanline CRAM changes
		if v.shadowHighlightMode() {
			v.renderMergedSHRange(line, fbLine, 0, width)
		} else {
			v.renderMergedRange(line, fbLine, 0, width)
		}
		v.fillH32Backdrop(fbLine)

		if width < ScreenWidth {
			v.stretchScanline(fbLine, width)
		}
		return
	}

	// Slow path: mid-scanline CRAM changes - render in segments
	savedCRAM := v.cram
	v.cram = v.cramSnapshot

	sh := v.shadowHighlightMode()
	startX := 0

	for _, c := range v.cramChanges {
		if c.pixelX > startX && startX < width {
			endX := c.pixelX
			if endX > width {
				endX = width
			}
			if sh {
				v.renderMergedSHRange(line, fbLine, startX, endX)
			} else {
				v.renderMergedRange(line, fbLine, startX, endX)
			}
			startX = endX
		}
		v.cram[c.addr] = c.hi
		v.cram[c.addr+1] = c.lo
	}

	if startX < width {
		if sh {
			v.renderMergedSHRange(line, fbLine, startX, width)
		} else {
			v.renderMergedRange(line, fbLine, startX, width)
		}
	}

	v.fillH32Backdrop(fbLine)
	v.cram = savedCRAM

	if width < ScreenWidth {
		v.stretchScanline(fbLine, width)
	}
}

// RenderScanline renders a single scanline into the framebuffer.
func (v *VDP) RenderScanline(line int) {
	// Compute framebuffer row
	fbLine := line
	if v.interlaceDoubleRes() {
		fbLine = line*2 + boolToInt(v.oddField)
	}

	if fbLine < 0 || fbLine >= MaxScreenHeight {
		return
	}

	// If display is disabled, fill with backdrop and return
	if !v.displayEnabled() {
		v.fillBackdrop(fbLine)
		return
	}

	v.renderMergedScanline(line, fbLine)
}
