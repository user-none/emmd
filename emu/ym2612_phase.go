package emu

// detuneTable is a 32x4 table of phase increment deltas indexed by [keyCode][DT&3].
// Column 0 = DT value 0 (no detune), columns 1-3 = increasing detune.
// DT bit 2 controls sign (0 = add, 1 = subtract).
// Values from Yamaha OPN application manual / Nemesis hardware research.
var detuneTable = [32][4]uint32{
	{0, 0, 1, 2},   // KC 0
	{0, 0, 1, 2},   // KC 1
	{0, 0, 1, 2},   // KC 2
	{0, 0, 1, 2},   // KC 3
	{0, 1, 2, 2},   // KC 4
	{0, 1, 2, 3},   // KC 5
	{0, 1, 2, 3},   // KC 6
	{0, 1, 2, 3},   // KC 7
	{0, 1, 2, 4},   // KC 8
	{0, 1, 3, 4},   // KC 9
	{0, 1, 3, 4},   // KC 10
	{0, 1, 3, 5},   // KC 11
	{0, 2, 4, 5},   // KC 12
	{0, 2, 4, 6},   // KC 13
	{0, 2, 4, 6},   // KC 14
	{0, 2, 5, 7},   // KC 15
	{0, 2, 5, 8},   // KC 16
	{0, 3, 6, 8},   // KC 17
	{0, 3, 6, 9},   // KC 18
	{0, 3, 7, 10},  // KC 19
	{0, 4, 8, 11},  // KC 20
	{0, 4, 8, 12},  // KC 21
	{0, 4, 9, 13},  // KC 22
	{0, 5, 10, 14}, // KC 23
	{0, 5, 11, 16}, // KC 24
	{0, 6, 12, 17}, // KC 25
	{0, 6, 13, 19}, // KC 26
	{0, 7, 14, 20}, // KC 27
	{0, 8, 16, 22}, // KC 28
	{0, 8, 16, 22}, // KC 29
	{0, 8, 16, 22}, // KC 30
	{0, 8, 16, 22}, // KC 31
}

// computePhaseIncrement calculates the 20-bit phase increment for an operator.
// fNum: 11-bit F-number, block: 3-bit octave, dt: 3-bit detune, mul: 4-bit multiplier.
func computePhaseIncrement(fNum uint16, block, keyCode, dt, mul uint8) uint32 {
	// Base increment: (fNum << block) >> 1 (17-bit result)
	base := (uint32(fNum) << uint(block)) >> 1

	// Apply detune
	dtLow := dt & 0x03
	delta := detuneTable[keyCode&0x1F][dtLow]
	if dt&0x04 != 0 {
		// Negative detune: underflow wraps intentionally (GEMS compatibility)
		base -= delta
	} else {
		base += delta
	}
	// Mask to 17 bits
	base &= 0x1FFFF

	// Apply multiplier
	var result uint32
	if mul == 0 {
		result = base >> 1 // MUL=0 means half
	} else {
		result = base * uint32(mul)
	}

	// Mask to 20 bits
	return result & 0xFFFFF
}

// ch3SlotMap maps operator index to ch3 special mode slot register index.
// Op 0 (OP1) -> slot 1 ($A9/$AD), Op 1 (OP2) -> slot 2 ($AA/$AE),
// Op 2 (OP3) -> slot 0 ($A8/$AC), Op 3 (OP4) -> uses channel freq (returns -1).
func ch3SlotMap(opIdx int) int {
	switch opIdx {
	case 0:
		return 1
	case 1:
		return 2
	case 2:
		return 0
	default:
		return -1 // OP4 uses channel frequency
	}
}

// ch3SlotToOp is the reverse of ch3SlotMap: maps slot register index to operator index.
// Slot 0 ($A8/$AC) -> Op 2 (OP3), Slot 1 ($A9/$AD) -> Op 0 (OP1),
// Slot 2 ($AA/$AE) -> Op 1 (OP2), Slot 3 -> -1 (invalid).
func ch3SlotToOp(slot int) int {
	switch slot {
	case 0:
		return 2
	case 1:
		return 0
	case 2:
		return 1
	default:
		return -1
	}
}

// computePMPhaseIncrement computes phase increment from a PM-modulated F-number.
// modFnum12 is the 12-bit modulated F-number ((fNum << 1) + pmDelta) & 0xFFF.
// block, keyCode, dt, and mul are the operator's original (unmodified) values.
func computePMPhaseIncrement(modFnum12 uint32, block, keyCode, dt, mul uint8) uint32 {
	// Base: (modFnum12 << block) >> 2
	// This is equivalent to (fNum << block) >> 1 when modFnum12 = fNum << 1.
	base := (modFnum12 << uint(block)) >> 2

	// Apply detune using original keyCode
	dtLow := dt & 0x03
	delta := detuneTable[keyCode&0x1F][dtLow]
	if dt&0x04 != 0 {
		// Negative detune: underflow wraps intentionally (GEMS compatibility)
		base -= delta
	} else {
		base += delta
	}
	base &= 0x1FFFF

	// Apply multiplier
	var result uint32
	if mul == 0 {
		result = base >> 1
	} else {
		result = base * uint32(mul)
	}
	return result & 0xFFFFF
}

// stepPhase advances an operator's phase accumulator by its increment.
func stepPhase(op *ymOperator) {
	op.phaseCounter = (op.phaseCounter + op.phaseInc) & 0xFFFFF
}
