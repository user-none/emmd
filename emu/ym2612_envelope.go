package emu

// SSG-EG bit masks and boundary constant.
const (
	ssgEnable    = 0x08  // SSG-EG enable bit
	ssgAttack    = 0x04  // SSG-EG attack/invert bit
	ssgAlternate = 0x02  // SSG-EG alternate bit
	ssgHold      = 0x01  // SSG-EG hold bit
	ssgCenter    = 0x200 // SSG-EG boundary (10-bit scale)
)

// egIncrementTable defines the attenuation increment patterns for rates 4-47.
// For rates 4-47, the shift value (11 - rate>>2) controls how often updates
// occur, and rate&3 selects one of 4 base patterns that control how much to
// increment each update. Row 0 is unused (frozen rates return early).
// For rates >= 48, see egHighRateTable.
// Values from Nemesis hardware analysis (SpritesMind YM2612 research).
var egIncrementTable = [5][8]uint8{
	{0, 0, 0, 0, 0, 0, 0, 0}, // row 0: unused (rate 0 returns early)
	{0, 1, 0, 1, 0, 1, 0, 1}, // pattern 0: rate&3 == 0 (avg 4/8)
	{0, 1, 0, 1, 1, 1, 0, 1}, // pattern 1: rate&3 == 1 (avg 5/8)
	{0, 1, 1, 1, 0, 1, 1, 1}, // pattern 2: rate&3 == 2 (avg 6/8)
	{0, 1, 1, 1, 1, 1, 1, 1}, // pattern 3: rate&3 == 3 (avg 7/8)
}

// egHighRateTable defines per-rate increment patterns for rates 48-63.
// On real hardware, rates >= 48 have shift=0 (update every tick) and each
// individual rate has its own pattern. Values go up to 8 for the highest
// rates, verified against hardware tests (Nemesis hardware analysis).
var egHighRateTable = [16][8]uint8{
	{1, 1, 1, 1, 1, 1, 1, 1}, // rate 48
	{1, 1, 1, 2, 1, 1, 1, 2}, // rate 49
	{1, 2, 1, 2, 1, 2, 1, 2}, // rate 50
	{1, 2, 2, 2, 1, 2, 2, 2}, // rate 51
	{2, 2, 2, 2, 2, 2, 2, 2}, // rate 52
	{2, 2, 2, 4, 2, 2, 2, 4}, // rate 53
	{2, 4, 2, 4, 2, 4, 2, 4}, // rate 54
	{2, 4, 4, 4, 2, 4, 4, 4}, // rate 55
	{4, 4, 4, 4, 4, 4, 4, 4}, // rate 56
	{4, 4, 4, 8, 4, 4, 4, 8}, // rate 57
	{4, 8, 4, 8, 4, 8, 4, 8}, // rate 58
	{4, 8, 8, 8, 4, 8, 8, 8}, // rate 59
	{8, 8, 8, 8, 8, 8, 8, 8}, // rate 60
	{8, 8, 8, 8, 8, 8, 8, 8}, // rate 61
	{8, 8, 8, 8, 8, 8, 8, 8}, // rate 62
	{8, 8, 8, 8, 8, 8, 8, 8}, // rate 63
}

// stepEnvelopesFull advances the envelope for all operators.
func (y *YM2612) stepEnvelopesFull() {
	counter := y.egCounter

	for ch := 0; ch < 6; ch++ {
		for op := 0; op < 4; op++ {
			o := &y.ch[ch].op[op]
			y.stepOperatorEnvelope(o, counter)
		}
	}
}

// stepOperatorEnvelope advances one operator's envelope by one EG step.
func (y *YM2612) stepOperatorEnvelope(op *ymOperator, counter uint16) {
	// On real hardware, the sustain level check occurs before the rate
	// calculation and increment. This prevents overshooting past the
	// sustain level (critical when sustain_level=0).
	if op.egState == egDecay && op.egLevel >= sustainLevel(op.d1l) {
		op.egState = egSustain
	}

	var rate uint8
	switch op.egState {
	case egAttack:
		rate = y.effectiveRate(op.ar, op)
	case egDecay:
		rate = y.effectiveRate(op.d1r, op)
	case egSustain:
		rate = y.effectiveRate(op.d2r, op)
	case egRelease:
		// Release rate: 2*RR + 1
		rr := uint8(2*int(op.rr) + 1)
		rate = y.effectiveRate(rr, op)
	}

	if rate == 0 {
		return // Frozen
	}

	// Compute increment based on rate and counter position.
	var incr uint8
	if rate >= 48 {
		// High rates (48-63): each individual rate has its own increment
		// pattern. Shift is 0 so every counter tick triggers an update.
		updateIdx := uint8(counter & 7)
		incr = egHighRateTable[rate-48][updateIdx]
	} else {
		// Rates 4-47: shift determines how often we update (inter-group
		// scaling), and rate&3 selects the increment pattern (intra-group
		// fine tuning). This gives a smooth ~8/7 ratio between successive
		// rates across all boundaries.
		group := rate >> 2
		shift := uint(11 - int(group))
		// Update only when the lower 'shift' bits of counter are all zero.
		if shift > 0 && (counter&((1<<shift)-1)) != 0 {
			return
		}
		updateIdx := uint8((counter >> shift) & 7)
		pattern := (rate & 3) + 1 // +1 to skip unused row 0
		incr = egIncrementTable[pattern][updateIdx]
	}
	if incr == 0 {
		return
	}

	// SSG-EG: 4x decay rate below center, stop at boundary
	if op.ssgEG&ssgEnable != 0 && op.egState != egAttack {
		if op.egLevel < ssgCenter {
			incr *= 4
		} else {
			incr = 0
		}
	}

	switch op.egState {
	case egAttack:
		if rate >= 62 {
			op.egLevel = 0
		} else {
			// Exponential attack: signed complement gives negative step,
			// decreasing attenuation toward 0 (full volume).
			// ~atten is negative for positive atten, giving exponential decay.
			step := (^int32(op.egLevel) * int32(incr)) >> 4
			newLevel := int32(op.egLevel) + step
			if newLevel <= 0 {
				op.egLevel = 0
			} else {
				op.egLevel = uint16(newLevel)
			}
		}
		// Transition to decay when level reaches 0
		if op.egLevel == 0 {
			op.egState = egDecay
		}

	case egDecay:
		// Linear decay: increase attenuation
		op.egLevel += uint16(incr)
		if op.egLevel > 0x3FF {
			op.egLevel = 0x3FF
		}

	case egSustain:
		// Continue decaying (D2R rate)
		op.egLevel += uint16(incr)
		if op.egLevel > 0x3FF {
			op.egLevel = 0x3FF
		}

	case egRelease:
		// Linear release: increase attenuation
		op.egLevel += uint16(incr)
		if op.ssgEG&ssgEnable != 0 && op.egLevel >= ssgCenter {
			op.egLevel = 0x3FF
		}
		if op.egLevel > 0x3FF {
			op.egLevel = 0x3FF
		}
	}
}

// sustainLevel converts the 4-bit D1L field to a 10-bit attenuation level.
// D1L 0-14 = level << 5, D1L 15 = 0x3E0.
func sustainLevel(d1l uint8) uint16 {
	if d1l >= 15 {
		return 0x3E0
	}
	return uint16(d1l) << 5
}

// ssgEGProcess handles SSG-EG boundary detection, state transitions,
// and output inversion. Returns the effective envelope level for output.
// Called every sample from opOut() when SSG-EG is enabled.
func ssgEGProcess(op *ymOperator) uint16 {
	// During release, inversion is disabled - return raw level
	if op.egState == egRelease {
		return op.egLevel
	}

	// Boundary response when envelope reaches SSG center
	if op.egLevel >= ssgCenter {
		if op.ssgEG&ssgAlternate != 0 {
			// Alternate: toggle inversion (with hold guard)
			hold := op.ssgEG&ssgHold != 0
			attackSet := op.ssgEG&ssgAttack != 0
			if !hold || attackSet == op.ssgInverted {
				op.ssgInverted = !op.ssgInverted
			}
		} else if op.ssgEG&ssgHold == 0 {
			// Non-alternate, non-hold: reset phase
			op.phaseCounter = 0
		}

		// Restart envelope if not holding
		if op.ssgEG&ssgHold == 0 &&
			(op.egState == egDecay || op.egState == egSustain) {
			op.egState = egAttack
		}
	}

	// Apply inversion
	if op.ssgInverted {
		return (ssgCenter - op.egLevel) & 0x3FF
	}
	return op.egLevel
}

// totalLevel returns the combined envelope + TL attenuation, capped at 0x3FF.
func totalLevel(egLevel uint16, tl uint8) uint16 {
	total := egLevel + uint16(tl)<<3
	if total > 0x3FF {
		return 0x3FF
	}
	return total
}
