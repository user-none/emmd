package emu

// dmaBytesPerLine returns DMA throughput in bytes per scanline.
// Mode 0 = 68K->VDP, Mode 1 = VRAM Fill, Mode 2 = VRAM Copy.
func (v *VDP) dmaBytesPerLine(mode int, blank bool) int {
	h40 := v.h40Mode()
	switch mode {
	case 0: // 68K -> VDP
		if h40 {
			if blank {
				return 205
			}
			return 18
		}
		if blank {
			return 167
		}
		return 16
	case 1: // VRAM Fill
		if h40 {
			if blank {
				return 204
			}
			return 17
		}
		if blank {
			return 166
		}
		return 15
	case 2: // VRAM Copy
		if h40 {
			if blank {
				return 102
			}
			return 9
		}
		if blank {
			return 83
		}
		return 8
	}
	return 18
}

// totalScanlines returns the total scanlines per frame for the current region.
func (v *VDP) totalScanlines() int {
	if v.isPAL {
		return 313
	}
	return 262
}

// dmaCalcEndCycle computes the cycle at which a DMA of totalBytes finishes.
// When a DMA spans the boundary between active display and VBlank (or vice
// versa), the two regions are calculated at their respective throughput rates.
func (v *VDP) dmaCalcEndCycle(triggerCycle uint64, totalBytes int, mode int) uint64 {
	blank := v.vBlank || !v.displayEnabled()
	currentRate := v.dmaBytesPerLine(mode, blank)
	if currentRate <= 0 || v.scanlineTotalCycles <= 0 {
		return triggerCycle
	}

	scanlineCycles := v.scanlineTotalCycles
	activeHeight := v.ActiveHeight()
	total := v.totalScanlines()

	// Lines remaining at the current rate before the rate boundary.
	// When display is disabled, use blanking rate for both regions
	// since there is no active/blank transition.
	var linesAtCurrentRate int
	var otherBlank bool
	if v.vBlank {
		// In VBlank -> boundary is end of frame (line wraps to 0 = active)
		linesAtCurrentRate = total - v.currentLine
		otherBlank = !v.displayEnabled() // blank if display disabled, active otherwise
	} else {
		// In active -> boundary is VBlank start
		linesAtCurrentRate = activeHeight - v.currentLine
		otherBlank = true
	}

	bytesAtCurrentRate := linesAtCurrentRate * currentRate
	if totalBytes <= bytesAtCurrentRate {
		// Fits entirely within current region - single rate
		fullLines := totalBytes / currentRate
		remainder := totalBytes % currentRate
		duration := fullLines * scanlineCycles
		if remainder > 0 {
			duration += (remainder * scanlineCycles) / currentRate
		}
		return triggerCycle + uint64(duration)
	}

	// Spans the boundary - split into two segments
	firstDuration := linesAtCurrentRate * scanlineCycles
	remainingBytes := totalBytes - bytesAtCurrentRate

	otherRate := v.dmaBytesPerLine(mode, otherBlank)
	if otherRate <= 0 {
		otherRate = currentRate // fallback
	}
	fullLines := remainingBytes / otherRate
	remainder := remainingBytes % otherRate
	secondDuration := fullLines * scanlineCycles
	if remainder > 0 {
		secondDuration += (remainder * scanlineCycles) / otherRate
	}

	return triggerCycle + uint64(firstDuration+secondDuration)
}

// executeDMA dispatches DMA based on reg 23 bits 7:6.
func (v *VDP) executeDMA(cycle uint64) {
	if !v.dmaEnabled() {
		return
	}

	mode := v.regs[23] >> 6
	switch mode {
	case 0, 1:
		// 68K -> VDP transfer (bit 7 = 0)
		v.executeDMA68K(cycle)
	case 2:
		// VRAM fill (bits 7:6 = 10)
		v.dmaFillPending = true
	case 3:
		// VRAM copy (bits 7:6 = 11)
		v.executeDMACopy(cycle)
	}
}

// executeDMA68K transfers data from the 68K bus to VDP memory.
func (v *VDP) executeDMA68K(cycle uint64) {
	if v.bus == nil {
		return
	}

	// Length from regs 19-20 (0 means 0x10000)
	length := uint32(v.regs[20])<<8 | uint32(v.regs[19])
	if length == 0 {
		length = 0x10000
	}

	v.dmaEndCycle = v.dmaCalcEndCycle(cycle, int(length)*2, 0)
	if v.dmaEndCycle > cycle {
		v.dmaStallCycles = int(v.dmaEndCycle - cycle)
	}

	// Source address from regs 21-23 (22-bit, shifted left 1)
	source := (uint32(v.regs[23]&0x7F) << 17) | (uint32(v.regs[22]) << 9) | (uint32(v.regs[21]) << 1)

	target := v.code & 0x0F
	inc := v.autoIncrement()

	// Per-word cycle offset for mid-scanline CRAM/VSRAM change tracking
	var cyclesPerWord uint64
	if v.scanlineTotalCycles > 0 {
		bytesPerLine := v.dmaBytesPerLine(0, v.vBlank || !v.displayEnabled())
		wordsPerLine := bytesPerLine / 2
		if wordsPerLine > 0 {
			cyclesPerWord = uint64(v.scanlineTotalCycles / wordsPerLine)
		}
	}
	currentCycle := cycle

	for i := uint32(0); i < length; i++ {
		word := v.bus.ReadWord(source & 0xFFFFFF)

		switch {
		case target == 0x01: // VRAM
			addr := v.address & 0xFFFF
			if addr&1 == 0 {
				v.vram[addr] = uint8(word >> 8)
				v.vram[(addr+1)&0xFFFF] = uint8(word)
			} else {
				wordAddr := addr & 0xFFFE
				v.vram[wordAddr] = uint8(word)
				v.vram[(wordAddr+1)&0xFFFF] = uint8(word >> 8)
			}
		case target == 0x03: // CRAM
			addr := v.address & 0x7F
			hi := uint8(word>>8) & 0x0E
			lo := uint8(word) & 0xEE
			v.cram[addr&0x7E] = hi
			v.cram[(addr&0x7E)+1] = lo
			v.cramChanges = append(v.cramChanges, cramChange{
				pixelX: v.cycleToPixel(currentCycle),
				addr:   uint8(addr & 0x7E),
				hi:     hi,
				lo:     lo,
			})
		case target == 0x05: // VSRAM
			addr := v.address & 0x7F
			if addr < 80 {
				hi := uint8(word>>8) & 0x03
				lo := uint8(word)
				v.vsram[addr&0x7E] = hi
				if (addr&0x7E)+1 < 80 {
					v.vsram[(addr&0x7E)+1] = lo
				}
				v.vsramChanges = append(v.vsramChanges, vsramChange{
					pixelX: v.cycleToPixel(currentCycle),
					addr:   int(addr & 0x7E),
					hi:     hi,
					lo:     lo,
				})
			}
		}

		v.address += inc
		source = (source & 0xFE0000) | ((source + 2) & 0x01FFFF)
		currentCycle += cyclesPerWord
	}

	// Update source registers
	source >>= 1
	v.regs[21] = uint8(source)
	v.regs[22] = uint8(source >> 8)
	v.regs[23] = (v.regs[23] & 0x80) | uint8(source>>16)&0x7F

	// Zero length registers
	v.regs[19] = 0
	v.regs[20] = 0
}

// executeDMAFill fills VRAM with the high byte of the written value.
// Called after the initial word has been written through normal WriteData routing.
func (v *VDP) executeDMAFill(cycle uint64, val uint16) {
	v.dmaFillPending = false

	// Length from regs 19-20 (0 means 0x10000)
	length := uint32(v.regs[20])<<8 | uint32(v.regs[19])
	if length == 0 {
		length = 0x10000
	}

	v.dmaEndCycle = v.dmaCalcEndCycle(cycle, int(length), 1)

	fillByte := uint8(val >> 8)
	inc := v.autoIncrement()

	// Fill: high byte goes to vram[addr^1] for the remaining length
	for i := uint32(0); i < length; i++ {
		fillAddr := v.address & 0xFFFF
		v.vram[fillAddr^1] = fillByte
		v.address += inc
	}

	// Zero length registers
	v.regs[19] = 0
	v.regs[20] = 0
}

// executeDMACopy copies data within VRAM.
func (v *VDP) executeDMACopy(cycle uint64) {
	// Length from regs 19-20 (0 means 0x10000)
	length := uint32(v.regs[20])<<8 | uint32(v.regs[19])
	if length == 0 {
		length = 0x10000
	}

	v.dmaEndCycle = v.dmaCalcEndCycle(cycle, int(length), 2)

	// Source address from regs 21-22 (within VRAM, not shifted)
	source := uint32(v.regs[22])<<8 | uint32(v.regs[21])

	inc := v.autoIncrement()

	// Byte-by-byte copy: source increments linearly, dest by auto-increment
	for i := uint32(0); i < length; i++ {
		srcAddr := source & 0xFFFF
		dstAddr := v.address & 0xFFFF
		v.vram[dstAddr] = v.vram[srcAddr]
		source++
		v.address += inc
	}

	// Update source registers
	v.regs[21] = uint8(source)
	v.regs[22] = uint8(source >> 8)

	// Zero length registers
	v.regs[19] = 0
	v.regs[20] = 0
}
