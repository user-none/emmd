package emu

import "math"

// sineTable is a quarter-sine log table: 256 entries of -log2(sin((2i+1)/512 * pi/2))
// in 4.8 fixed-point (12-bit values). Used for operator output computation.
var sineTable [256]uint16

// pow2Table is a power-of-2 table: 256 entries of 2^(1-(i+1)/256) scaled to 11-bit.
// Used to convert log-domain attenuation back to linear amplitude.
var pow2Table [256]uint16

func init() {
	// Build quarter-sine log table
	for i := 0; i < 256; i++ {
		// sin((2*i+1) / 512 * pi/2)
		angle := float64(2*i+1) / 512.0 * math.Pi / 2.0
		s := math.Sin(angle)
		// -log2(sin) in 4.8 fixed-point
		logVal := -math.Log2(s) * 256.0
		sineTable[i] = uint16(math.Round(logVal))
	}

	// Build power-of-2 table
	for i := 0; i < 256; i++ {
		// 2^(1 - (i+1)/256) scaled to fit 11 bits (0-2047)
		val := math.Pow(2.0, 1.0-float64(i+1)/256.0) * 1024.0
		pow2Table[i] = uint16(math.Round(val))
	}
}

// computeOperatorOutput computes the signed 14-bit output of an operator
// given its phase (with modulation) and envelope attenuation.
func computeOperatorOutput(phase uint32, egLevel uint16) int16 {
	// Extract 10-bit sine table index from phase
	// Top 10 bits of 20-bit phase = bits 19-10
	phaseIdx := (phase >> 10) & 0x3FF

	// Bit 9 = sign (negative half of sine)
	sign := phaseIdx & 0x200
	// Bit 8 = mirror (second quarter)
	mirror := phaseIdx & 0x100
	// Bits 7-0 = table index
	idx := phaseIdx & 0xFF

	// Mirror for second quarter
	if mirror != 0 {
		idx = 0xFF - idx
	}

	// Look up log-sine attenuation (4.8 fixed-point, ~12 bits)
	sineAtten := uint32(sineTable[idx])

	// Total attenuation = sine_atten + (envelope_atten << 2)
	// Envelope is 10-bit, shift to 4.8 format
	totalAtten := sineAtten + (uint32(egLevel) << 2)

	// Convert from log domain to linear
	// Integer part = totalAtten >> 8, fractional part = totalAtten & 0xFF
	// Max totalAtten is ~6229 (sineTable max ~2137 + egLevel max 0x3FF << 2),
	// giving intPart up to ~24. The pow2 table produces 13-bit values after
	// << 2, so right-shifting by >= 13 naturally yields 0 - no explicit
	// threshold check is needed.
	intPart := totalAtten >> 8
	fracPart := totalAtten & 0xFF

	// Linear value from pow2 table, shifted by integer part
	linear := uint32(pow2Table[fracPart]) << 2
	linear >>= intPart

	// Apply sign
	if sign != 0 {
		return -int16(linear)
	}
	return int16(linear)
}

// evaluateChannelFull computes one sample of output for a channel using FM algorithms.
// Returns (mono, left, right) where mono is the raw algorithm output before panning.
func (y *YM2612) evaluateChannelFull(chIdx int) (int16, int16, int16) {
	ch := &y.ch[chIdx]

	// DAC mode: channel 5 (index 5) replaced by DAC when enabled
	if chIdx == 5 && y.dacEnable {
		dacOut := (int16(y.dacSample) - 128) << 6
		l := applyLadder(dacOut, ch.panL)
		r := applyLadder(dacOut, ch.panR)
		return dacOut, l, r
	}

	// Step phase for all operators, applying PM if active.
	// On real hardware, PM modifies the F-number proportionally (not a flat
	// offset), then the phase increment is recomputed from the modulated
	// F-number with the original block, keycode, detune, and multiplier.
	if ch.fms != 0 && y.lfoEnable {
		for i := range ch.op {
			op := &ch.op[i]
			fNum := ch.fNum
			block := ch.block

			// Ch3 special mode: per-operator frequency
			if y.ch3Mode != ch3ModeNormal && chIdx == 2 {
				slot := ch3SlotMap(i)
				if slot >= 0 {
					fNum = y.ch3Freq[slot]
					block = y.ch3Block[slot]
				}
			}

			pmDelta := y.lfoPMFnumDelta(ch.fms, fNum)
			modFnum12 := uint32(int32(fNum)<<1+pmDelta) & 0xFFF
			inc := computePMPhaseIncrement(modFnum12, block, op.keyCode, op.dt, op.mul)
			op.phaseCounter = (op.phaseCounter + inc) & 0xFFFFF
		}
	} else {
		for i := range ch.op {
			op := &ch.op[i]
			op.phaseCounter = (op.phaseCounter + op.phaseInc) & 0xFFFFF
		}
	}

	// Compute AM attenuation for this channel
	amAtten := y.lfoAMAttenuation(ch.ams)

	// Compute operator outputs using the selected algorithm
	var out int16
	switch ch.algorithm {
	case 0:
		out = y.evalAlgo0(ch, amAtten)
	case 1:
		out = y.evalAlgo1(ch, amAtten)
	case 2:
		out = y.evalAlgo2(ch, amAtten)
	case 3:
		out = y.evalAlgo3(ch, amAtten)
	case 4:
		out = y.evalAlgo4(ch, amAtten)
	case 5:
		out = y.evalAlgo5(ch, amAtten)
	case 6:
		out = y.evalAlgo6(ch, amAtten)
	case 7:
		out = y.evalAlgo7(ch, amAtten)
	}

	// Apply panning with ladder effect. Note: even disabled pan outputs
	// produce a non-zero residual offset (see applyLadder).
	l := applyLadder(out, ch.panL)
	r := applyLadder(out, ch.panR)
	return out, l, r
}

// feedback computes the self-feedback modulation for operator 0.
// Returns the phase modulation value (added to phase before sine lookup).
func feedback(op *ymOperator, fbLevel uint8) int32 {
	if fbLevel == 0 {
		return 0
	}
	return (int32(op.prevOut[0]) + int32(op.prevOut[1])) >> (10 - uint(fbLevel))
}

// opOut computes an operator's output with optional phase modulation and stores history.
// modulation is in the 10-bit phase index domain. computeOperatorOutput
// extracts the index via (phase >> 10) & 0x3FF, so we shift left by 10
// to place the modulation into the 20-bit phase counter.
// Callers must pre-scale: feedback values are used directly, operator-to-operator
// modulation is shifted right by 1 (see ym2612_reference.md Section 8,
// "Phase Modulation").
// amAtten is the LFO AM attenuation (only applied if operator has AM enabled).
func opOut(op *ymOperator, modulation int32, tl uint8, amAtten uint16) int16 {
	egLevel := op.egLevel
	if op.ssgEG&ssgEnable != 0 {
		egLevel = ssgEGProcess(op)
	}
	phase := op.phaseCounter + uint32(modulation<<10)
	egAtten := totalLevel(egLevel, tl)
	// Apply AM modulation if enabled
	if op.am {
		egAtten += amAtten
		if egAtten > 0x3FF {
			egAtten = 0x3FF
		}
	}
	out := computeOperatorOutput(phase, egAtten)

	// Update output history for feedback
	op.prevOut[1] = op.prevOut[0]
	op.prevOut[0] = out
	return out
}

// clampAccum clamps the 9-bit DAC accumulator to +8160/-8176 (+0x1FE0/-0x1FF0),
// the YM2612's internal signed 14-bit output range after 9-bit DAC quantization.
func clampAccum(v int32) int32 {
	if v > 0x1FE0 {
		return 0x1FE0
	}
	if v < -0x1FF0 {
		return -0x1FF0
	}
	return v
}

// applyLadder applies the YM2612 DAC ladder effect (crossover distortion).
// The chip's resistor-ladder DAC has a voltage gap at the zero crossing.
// Even when a channel's pan is disabled ("muted"), the DAC still produces a
// small residual DC offset (+128 or -128) - this is not a bug but matches
// real hardware behavior confirmed by die-shot analysis (ym2612_reference.md S11).
// Values are in 14-bit scale (9-bit reference values <<5).
func applyLadder(sample int16, panEnabled bool) int16 {
	if !panEnabled {
		if sample >= 0 {
			return 128
		}
		return -128
	}
	if sample >= 0 {
		return sample + 128
	}
	return sample - 96
}

// quantize9 applies the YM2612's 9-bit internal DAC quantization by
// masking off the lower 5 bits of the operator output.
func quantize9(v int16) int16 {
	return v & ^int16(0x1F)
}

// Algorithm 0: OP1->OP2->OP3->OP4 (serial chain)
// All operators modulate the next. OP4 is the only carrier.
func (y *YM2612) evalAlgo0(ch *ymChannel, amAtten uint16) int16 {
	fb := feedback(&ch.op[0], ch.feedback)
	s1 := opOut(&ch.op[0], fb, ch.op[0].tl, amAtten)
	s2 := opOut(&ch.op[1], int32(s1)>>1, ch.op[1].tl, amAtten)
	s3 := opOut(&ch.op[2], int32(s2)>>1, ch.op[2].tl, amAtten)
	s4 := opOut(&ch.op[3], int32(s3)>>1, ch.op[3].tl, amAtten)
	return quantize9(s4)
}

// Algorithm 1: (OP1+OP2)->OP3->OP4
// OP1 and OP2 both modulate OP3. OP4 is carrier.
func (y *YM2612) evalAlgo1(ch *ymChannel, amAtten uint16) int16 {
	fb := feedback(&ch.op[0], ch.feedback)
	s1 := opOut(&ch.op[0], fb, ch.op[0].tl, amAtten)
	s2 := opOut(&ch.op[1], 0, ch.op[1].tl, amAtten)
	mod := (int32(s1) + int32(s2)) >> 1
	s3 := opOut(&ch.op[2], mod, ch.op[2].tl, amAtten)
	s4 := opOut(&ch.op[3], int32(s3)>>1, ch.op[3].tl, amAtten)
	return quantize9(s4)
}

// Algorithm 2: OP1+(OP2->OP3)->OP4
// OP2 modulates OP3, OP1 and OP3 both modulate OP4. OP4 carrier.
func (y *YM2612) evalAlgo2(ch *ymChannel, amAtten uint16) int16 {
	fb := feedback(&ch.op[0], ch.feedback)
	s1 := opOut(&ch.op[0], fb, ch.op[0].tl, amAtten)
	s2 := opOut(&ch.op[1], 0, ch.op[1].tl, amAtten)
	s3 := opOut(&ch.op[2], int32(s2)>>1, ch.op[2].tl, amAtten)
	mod := (int32(s1) + int32(s3)) >> 1
	s4 := opOut(&ch.op[3], mod, ch.op[3].tl, amAtten)
	return quantize9(s4)
}

// Algorithm 3: (OP1->OP2)+OP3->OP4
// OP1 modulates OP2, OP2 and OP3 both modulate OP4. OP4 carrier.
func (y *YM2612) evalAlgo3(ch *ymChannel, amAtten uint16) int16 {
	fb := feedback(&ch.op[0], ch.feedback)
	s1 := opOut(&ch.op[0], fb, ch.op[0].tl, amAtten)
	s2 := opOut(&ch.op[1], int32(s1)>>1, ch.op[1].tl, amAtten)
	s3 := opOut(&ch.op[2], 0, ch.op[2].tl, amAtten)
	mod := (int32(s2) + int32(s3)) >> 1
	s4 := opOut(&ch.op[3], mod, ch.op[3].tl, amAtten)
	return quantize9(s4)
}

// Algorithm 4: (OP1->OP2) + (OP3->OP4)
// Two parallel chains. OP2 and OP4 are carriers.
func (y *YM2612) evalAlgo4(ch *ymChannel, amAtten uint16) int16 {
	fb := feedback(&ch.op[0], ch.feedback)
	s1 := opOut(&ch.op[0], fb, ch.op[0].tl, amAtten)
	s3 := opOut(&ch.op[2], 0, ch.op[2].tl, amAtten)
	s2 := opOut(&ch.op[1], int32(s1)>>1, ch.op[1].tl, amAtten)
	s4 := opOut(&ch.op[3], int32(s3)>>1, ch.op[3].tl, amAtten)
	out := int32(quantize9(s2)) + int32(quantize9(s4))
	return int16(clampAccum(out))
}

// Algorithm 5: OP1->OP2+OP3+OP4
// OP1 modulates OP2, OP3, and OP4. All three are carriers.
func (y *YM2612) evalAlgo5(ch *ymChannel, amAtten uint16) int16 {
	fb := feedback(&ch.op[0], ch.feedback)
	s1 := opOut(&ch.op[0], fb, ch.op[0].tl, amAtten)
	mod := int32(s1) >> 1
	s3 := opOut(&ch.op[2], mod, ch.op[2].tl, amAtten)
	s2 := opOut(&ch.op[1], mod, ch.op[1].tl, amAtten)
	s4 := opOut(&ch.op[3], mod, ch.op[3].tl, amAtten)
	out := int32(quantize9(s2))
	out = clampAccum(out + int32(quantize9(s3)))
	out = clampAccum(out + int32(quantize9(s4)))
	return int16(out)
}

// Algorithm 6: OP1->OP2 + OP3 + OP4
// OP1 modulates OP2. OP2, OP3, OP4 are carriers.
func (y *YM2612) evalAlgo6(ch *ymChannel, amAtten uint16) int16 {
	fb := feedback(&ch.op[0], ch.feedback)
	s1 := opOut(&ch.op[0], fb, ch.op[0].tl, amAtten)
	s3 := opOut(&ch.op[2], 0, ch.op[2].tl, amAtten)
	s2 := opOut(&ch.op[1], int32(s1)>>1, ch.op[1].tl, amAtten)
	s4 := opOut(&ch.op[3], 0, ch.op[3].tl, amAtten)
	out := int32(quantize9(s2))
	out = clampAccum(out + int32(quantize9(s3)))
	out = clampAccum(out + int32(quantize9(s4)))
	return int16(out)
}

// Algorithm 7: OP1 + OP2 + OP3 + OP4
// All operators are carriers, no modulation (except OP1 feedback).
func (y *YM2612) evalAlgo7(ch *ymChannel, amAtten uint16) int16 {
	fb := feedback(&ch.op[0], ch.feedback)
	s1 := opOut(&ch.op[0], fb, ch.op[0].tl, amAtten)
	s3 := opOut(&ch.op[2], 0, ch.op[2].tl, amAtten)
	s2 := opOut(&ch.op[1], 0, ch.op[1].tl, amAtten)
	s4 := opOut(&ch.op[3], 0, ch.op[3].tl, amAtten)
	out := int32(quantize9(s1))
	out = clampAccum(out + int32(quantize9(s2)))
	out = clampAccum(out + int32(quantize9(s3)))
	out = clampAccum(out + int32(quantize9(s4)))
	return int16(out)
}
