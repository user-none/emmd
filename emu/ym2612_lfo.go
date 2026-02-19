package emu

// lfoPeriodTable maps the 3-bit LFO frequency setting to the period
// in sample clocks between LFO steps. Lower values = faster LFO.
var lfoPeriodTable = [8]uint16{108, 77, 71, 67, 62, 44, 8, 5}

// lfoAMShift maps the 2-bit AMS (AM sensitivity) to a right-shift amount.
// Applied to the 7-bit triangle AM value to scale the effect.
// AMS 0 = disabled (shift by 8, effectively 0), 1 = 1.4dB, 2 = 5.9dB, 3 = 11.8dB.
// The AM output is a 7-bit triangle (0-126). After shifting, it becomes the
// attenuation delta added to operators with AM enabled.
// Shift values: {8, 3, 1, 0} where 8 means the 7-bit value becomes 0.
var lfoAMShift = [4]uint8{8, 3, 1, 0}

// pmBaseTable defines the base PM increment for each FMS setting and LFO
// quarter-wave step. Indexed by [FMS][quarterStep]. These values correspond
// to F-number bit 10 (MSB) being set. Lower F-number bits contribute
// proportionally less (right-shifted by bit distance from bit 10).
// From Nemesis hardware analysis and OPN2 die-shot research.
var pmBaseTable = [8][8]int32{
	{0, 0, 0, 0, 0, 0, 0, 0},       // FMS 0: disabled
	{0, 0, 0, 0, 4, 4, 4, 4},       // FMS 1
	{0, 0, 0, 4, 4, 4, 8, 8},       // FMS 2
	{0, 0, 4, 4, 8, 8, 12, 12},     // FMS 3
	{0, 0, 4, 8, 8, 8, 12, 16},     // FMS 4
	{0, 0, 8, 12, 16, 16, 20, 24},  // FMS 5
	{0, 0, 16, 24, 32, 32, 40, 48}, // FMS 6
	{0, 0, 32, 48, 64, 64, 80, 96}, // FMS 7
}

// stepLFOFull advances the LFO counter and updates the AM output value.
// PM is computed on-the-fly in lfoPMFnumDelta using lfoStep directly.
func (y *YM2612) stepLFOFull() {
	if !y.lfoEnable {
		y.lfoAMOut = 0
		return
	}

	period := lfoPeriodTable[y.lfoFreq]
	y.lfoCnt++
	if y.lfoCnt >= period {
		y.lfoCnt = 0
		y.lfoStep = (y.lfoStep + 1) & 0x7F // 128 steps
	}

	// Triangle wave from step counter (see ym2612_reference.md Section 10):
	// Steps 0-63:   output = (63 - step) * 2 (126, 124, ..., 0)
	// Steps 64-127: output = (step - 64) * 2 (0, 2, ..., 126)
	var triVal uint8
	if y.lfoStep < 64 {
		triVal = (63 - y.lfoStep) * 2
	} else {
		triVal = (y.lfoStep - 64) * 2
	}

	y.lfoAMOut = triVal
}

// lfoAMAttenuation returns the AM attenuation for a given channel's AMS setting.
// Only applied to operators with AM enabled.
func (y *YM2612) lfoAMAttenuation(ams uint8) uint16 {
	if ams == 0 {
		return 0
	}
	shift := lfoAMShift[ams]
	if shift >= 8 {
		return 0
	}
	return uint16(y.lfoAMOut) >> shift
}

// lfoPMFnumDelta computes the F-number delta for PM modulation.
// On real hardware, PM is proportional to the channel's F-number (upper bits
// 4-10), making higher-frequency notes get proportionally more vibrato.
// Returns a signed delta to add to (fNum << 1) in 12-bit precision.
func (y *YM2612) lfoPMFnumDelta(fms uint8, fNum uint16) int32 {
	if fms == 0 || !y.lfoEnable {
		return 0
	}

	// 5-bit PM step from 7-bit LFO counter (0-31)
	pmStep := y.lfoStep >> 2

	// Quarter-wave index (0-7) with mirroring at second quarter
	lfoIdx := pmStep & 0x07
	if pmStep&0x08 != 0 {
		lfoIdx = 7 - lfoIdx
	}

	// Base PM value for this FMS and quarter-wave position
	baseValue := pmBaseTable[fms][lfoIdx]

	// Compute delta proportional to F-number upper bits (4-10).
	// Each bit contributes baseValue >> (10 - bit), making higher
	// frequencies get proportionally more modulation.
	var delta int32
	for bit := uint(4); bit <= 10; bit++ {
		if fNum&(1<<bit) != 0 {
			delta += baseValue >> (10 - bit)
		}
	}

	// Sign: negative in second half of LFO cycle (steps 16-31)
	if pmStep&0x10 != 0 {
		delta = -delta
	}

	return delta
}
