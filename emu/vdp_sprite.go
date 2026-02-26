package emu

// renderSprites renders all sprites that intersect the given scanline into lineBufSpr.
// Sprites are traversed in link-list order from the SAT (Sprite Attribute Table).
func (v *VDP) renderSprites(line int) {
	satMask := uint8(0x7F)
	if v.h40Mode() {
		satMask = 0x7E // H40: $400 aligned, bit 0 ignored
	}
	satBase := uint16(v.regs[5]&satMask) << 9

	h40 := v.h40Mode()
	width := v.activeWidth()
	tileRows := v.tileRows()
	tileSz := v.tileSize()
	// Power-of-2 mask and shift for tile row divmod
	tileRowMask := tileRows - 1 // 7 or 15
	tileRowShift := uint(3)     // log2(8) = 3
	if tileRows == 16 {
		tileRowShift = 4 // log2(16) = 4
	}

	// Per-line limits
	var maxSprites, maxPixels int
	var maxTotal int
	if h40 {
		maxSprites = 20
		maxPixels = 320
		maxTotal = 80
	} else {
		maxSprites = 16
		maxPixels = 256
		maxTotal = 64
	}

	spritesOnLine := 0
	pixelsOnLine := 0
	firstSpriteFound := false
	spriteIndex := 0

	for i := 0; i < maxTotal; i++ {
		// SAT entry: 8 bytes per sprite
		entryAddr := (satBase + uint16(spriteIndex)*8) & 0xFFFF

		// Byte 0-1: Y position (10 bits)
		yRaw := int(uint16(v.vram[entryAddr])<<8|uint16(v.vram[(entryAddr+1)&0xFFFF])) & 0x03FF
		yPos := yRaw - 128

		// Byte 2: size and link
		sizeLink := v.vram[(entryAddr+2)&0xFFFF]
		hSizeCells := int((sizeLink>>2)&0x03) + 1 // 1-4 cells wide
		vSizeCells := int(sizeLink&0x03) + 1      // 1-4 cells tall

		// Byte 3: link to next sprite
		link := v.vram[(entryAddr+3)&0xFFFF] & 0x7F

		spriteHeight := vSizeCells * tileRows

		// Check if this sprite intersects the current scanline
		if line >= yPos && line < yPos+spriteHeight {
			spritesOnLine++
			if spritesOnLine > maxSprites {
				v.spriteOverflow = true
				break
			}

			// Byte 4-5: attributes (priority, palette, flip, tile index)
			attrHi := v.vram[(entryAddr+4)&0xFFFF]
			attrLo := v.vram[(entryAddr+5)&0xFFFF]
			attr := uint16(attrHi)<<8 | uint16(attrLo)

			priority := attr&entryPriority != 0
			pal := uint8((attr >> entryPalShift) & entryPalMask)
			vFlip := attr&entryVFlip != 0
			hFlip := attr&entryHFlip != 0
			baseTile := attr & entryTileMask

			// Byte 6-7: X position (9 bits)
			xRaw := int(uint16(v.vram[(entryAddr+6)&0xFFFF])<<8|uint16(v.vram[(entryAddr+7)&0xFFFF])) & 0x01FF

			// X=0 masking: if xRaw is 0 and we already found a sprite on this line,
			// mask all remaining sprites
			if xRaw == 0 && firstSpriteFound {
				break
			}
			if xRaw != 0 {
				firstSpriteFound = true
			}

			xPos := xRaw - 128
			spriteWidth := hSizeCells * 8

			// Calculate which row within the sprite this scanline hits
			spriteRow := line - yPos
			if vFlip {
				spriteRow = spriteHeight - 1 - spriteRow
			}

			// Draw each pixel of the sprite
			for sx := 0; sx < spriteWidth; sx++ {
				pixelsOnLine++
				if pixelsOnLine > maxPixels {
					break
				}

				screenX := xPos + sx
				if screenX < 0 || screenX >= width {
					continue
				}

				// Column within sprite (before H-flip)
				spriteCol := sx
				if hFlip {
					spriteCol = spriteWidth - 1 - spriteCol
				}

				// Multi-cell sprites use column-major tile order.
				// Shift and mask split pixel coords into cell and offset
				// (equivalent to divmod by 8 and tileRows).
				cellCol := spriteCol >> 3            // spriteCol / 8
				cellRow := spriteRow >> tileRowShift // spriteRow / tileRows
				tileInSprite := uint16(cellCol*vSizeCells + cellRow)
				tileIndex := baseTile + tileInSprite

				pixX := spriteCol & 7           // spriteCol % 8
				pixY := spriteRow & tileRowMask // spriteRow % tileRows

				tileAddr := (tileIndex * tileSz) & 0xFFFF
				colorIdx := v.decodeTilePixel(tileAddr, pixX, pixY, false, false)

				if colorIdx == 0 {
					continue
				}

				// Collision: two non-transparent sprite pixels at the same position
				if v.lineBufSpr[screenX].colorIndex != 0 {
					v.spriteCollision = true
					continue
				}

				v.lineBufSpr[screenX] = layerPixel{
					colorIndex: colorIdx,
					palette:    pal,
					priority:   priority,
				}
			}

			if pixelsOnLine > maxPixels {
				break
			}
		}

		// Follow link
		if link == 0 {
			break
		}
		spriteIndex = int(link)
	}
}
