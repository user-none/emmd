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

// compositeScanline composites all layer buffers into the framebuffer for one scanline.
// When CRAM changes occurred mid-scanline, it renders in segments with the correct
// CRAM state for each pixel range.
func (v *VDP) compositeScanline(line int) {
	if len(v.cramChanges) == 0 {
		// Fast path: no mid-scanline CRAM changes
		if v.shadowHighlightMode() {
			v.compositeScanlineSHRange(line, 0, v.activeWidth())
		} else {
			v.compositeScanlineRange(line, 0, v.activeWidth())
		}
		v.fillH32Backdrop(line)
		return
	}

	// Rewind CRAM to pre-M68K state for this scanline
	savedCRAM := v.cram
	v.cram = v.cramSnapshot

	width := v.activeWidth()
	sh := v.shadowHighlightMode()
	startX := 0

	for _, c := range v.cramChanges {
		if c.pixelX > startX && startX < width {
			endX := c.pixelX
			if endX > width {
				endX = width
			}
			if sh {
				v.compositeScanlineSHRange(line, startX, endX)
			} else {
				v.compositeScanlineRange(line, startX, endX)
			}
			startX = endX
		}
		// Apply this CRAM change
		v.cram[c.addr] = c.hi
		v.cram[c.addr+1] = c.lo
	}

	// Render remaining pixels
	if startX < width {
		if sh {
			v.compositeScanlineSHRange(line, startX, width)
		} else {
			v.compositeScanlineRange(line, startX, width)
		}
	}

	v.fillH32Backdrop(line)

	// Restore final CRAM state (matches what M68K left it as)
	v.cram = savedCRAM
}

// compositeScanlineRange composites pixels from startX to endX (exclusive) using current CRAM.
func (v *VDP) compositeScanlineRange(line, startX, endX int) {
	pal, idx := v.backdropColor()
	pix := v.framebuffer.Pix
	stride := v.framebuffer.Stride
	offset := line * stride

	for x := startX; x < endX; x++ {
		spr := v.lineBufSpr[x]
		a := v.lineBufA[x]
		b := v.lineBufB[x]

		var cpal, cidx uint8

		// Priority resolution (highest to lowest):
		// 1. High-priority sprite (non-transparent)
		// 2. High-priority Plane A/Window (non-transparent)
		// 3. High-priority Plane B (non-transparent)
		// 4. Low-priority sprite (non-transparent)
		// 5. Low-priority Plane A/Window (non-transparent)
		// 6. Low-priority Plane B (non-transparent)
		// 7. Backdrop
		switch {
		case spr.priority && spr.colorIndex != 0:
			cpal, cidx = spr.palette, spr.colorIndex
		case a.priority && a.colorIndex != 0:
			cpal, cidx = a.palette, a.colorIndex
		case b.priority && b.colorIndex != 0:
			cpal, cidx = b.palette, b.colorIndex
		case !spr.priority && spr.colorIndex != 0:
			cpal, cidx = spr.palette, spr.colorIndex
		case !a.priority && a.colorIndex != 0:
			cpal, cidx = a.palette, a.colorIndex
		case !b.priority && b.colorIndex != 0:
			cpal, cidx = b.palette, b.colorIndex
		default:
			cpal, cidx = pal, idx
		}

		r, g, bv := v.cramColor(cpal*16 + cidx)

		// Left column blank: first 8 pixels replaced with backdrop
		if v.leftColumnBlank() && x < 8 {
			r, g, bv = v.cramColor(pal*16 + idx)
		}

		p := offset + x*4
		pix[p] = r
		pix[p+1] = g
		pix[p+2] = bv
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

// compositeScanlineSHRange composites pixels from startX to endX with shadow/highlight mode.
func (v *VDP) compositeScanlineSHRange(line, startX, endX int) {
	pal, idx := v.backdropColor()
	pix := v.framebuffer.Pix
	stride := v.framebuffer.Stride
	offset := line * stride

	for x := startX; x < endX; x++ {
		spr := v.lineBufSpr[x]
		a := v.lineBufA[x]
		b := v.lineBufB[x]

		// Default brightness is shadow (everything starts shadowed)
		brightness := brightnessShadow

		// Determine the visible color using standard priority resolution,
		// but also track brightness modifications from sprite operators.
		var cpal, cidx uint8
		sprIsOperator := false

		// Check if sprite is a palette 3 special operator (low-priority only)
		if !spr.priority && spr.colorIndex != 0 && spr.palette == 3 {
			if spr.colorIndex == 14 || spr.colorIndex == 15 {
				sprIsOperator = true
			}
		}

		// Standard priority resolution (same order as normal compositing)
		switch {
		case spr.priority && spr.colorIndex != 0:
			// High-priority sprite: always normal brightness
			cpal, cidx = spr.palette, spr.colorIndex
			brightness = brightnessNormal
		case a.priority && a.colorIndex != 0:
			// High-priority plane A: normal brightness
			cpal, cidx = a.palette, a.colorIndex
			brightness = brightnessNormal
		case b.priority && b.colorIndex != 0:
			// High-priority plane B: normal brightness
			cpal, cidx = b.palette, b.colorIndex
			brightness = brightnessNormal
		case !spr.priority && spr.colorIndex != 0:
			if sprIsOperator {
				// Operator sprite: doesn't display itself, modifies underlying pixel
				// Fall through to find the underlying plane pixel
				switch {
				case a.colorIndex != 0:
					cpal, cidx = a.palette, a.colorIndex
				case b.colorIndex != 0:
					cpal, cidx = b.palette, b.colorIndex
				default:
					cpal, cidx = pal, idx
				}
				// Apply operator brightness adjustment
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
				// Other low-priority sprites: remain at shadow
				cpal, cidx = spr.palette, spr.colorIndex
			}
		case a.colorIndex != 0:
			// Low-priority plane A: remains at shadow
			cpal, cidx = a.palette, a.colorIndex
		case b.colorIndex != 0:
			// Low-priority plane B: remains at shadow
			cpal, cidx = b.palette, b.colorIndex
		default:
			cpal, cidx = pal, idx
		}

		// Apply brightness to get final RGB
		var r, g, bv uint8
		switch brightness {
		case brightnessShadow:
			r, g, bv = v.cramColorShadow(cpal*16 + cidx)
		case brightnessNormal:
			r, g, bv = v.cramColor(cpal*16 + cidx)
		case brightnessHighlight:
			r, g, bv = v.cramColorHighlight(cpal*16 + cidx)
		}

		// Left column blank: first 8 pixels replaced with backdrop
		if v.leftColumnBlank() && x < 8 {
			r, g, bv = v.cramColorShadow(pal*16 + idx)
		}

		p := offset + x*4
		pix[p] = r
		pix[p+1] = g
		pix[p+2] = bv
		pix[p+3] = 0xFF
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

	width := v.activeWidth()

	// Clear line buffers
	for i := 0; i < width; i++ {
		v.lineBufB[i] = layerPixel{}
		v.lineBufA[i] = layerPixel{}
		v.lineBufSpr[i] = layerPixel{}
	}

	// Render layers
	v.renderPlaneB(line)
	v.renderPlaneAAndWindow(line)
	v.renderSprites(line)

	// Composite all layers into framebuffer
	v.compositeScanline(fbLine)

	// In H32 mode, stretch 256 active pixels to fill the 320-pixel framebuffer.
	// On real hardware, H32's slower pixel clock makes each pixel physically wider,
	// filling the same CRT width as H40's 320 pixels.
	if width < ScreenWidth {
		v.stretchScanline(fbLine, width)
	}
}
