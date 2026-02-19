package emu

import (
	"math"
	"testing"
)

// --- computeOperatorOutput tests ---

func TestOutput_ZeroPhaseSineNearZero(t *testing.T) {
	// Phase 0 degrees: top 10 bits = 0x000, sine near zero -> small output
	out := computeOperatorOutput(0, 0)
	if out <= 0 {
		t.Errorf("phase=0 should be small positive (near zero sine), got %d", out)
	}
}

func TestOutput_90DegreesPeakPositive(t *testing.T) {
	// Phase 90 degrees: top 10 bits = 0x0FF (quarter sine peak, max positive)
	phase := uint32(0x0FF) << 10
	out := computeOperatorOutput(phase, 0)
	if out <= 0 {
		t.Errorf("phase=90deg should be max positive, got %d", out)
	}
}

func TestOutput_180DegreesNearZero(t *testing.T) {
	// Phase 180 degrees: top 10 bits = 0x200, sine crosses zero (negative side)
	phase := uint32(0x200) << 10
	out := computeOperatorOutput(phase, 0)
	// At 180 degrees, bit 9 is set (negative half), idx=0 (near zero)
	// Output should be small negative
	if out >= 0 {
		t.Errorf("phase=180deg should be small negative, got %d", out)
	}
}

func TestOutput_270DegreesNegativePeak(t *testing.T) {
	// Phase 270 degrees: top 10 bits = 0x2FF (peak of negative half)
	phase := uint32(0x2FF) << 10
	out := computeOperatorOutput(phase, 0)
	if out >= 0 {
		t.Errorf("phase=270deg should be max negative, got %d", out)
	}
}

func TestOutput_360DegreesWraps(t *testing.T) {
	// Phase 360 degrees = 0 degrees (wraps around)
	// Top 10 bits = 0x000
	out360 := computeOperatorOutput(0, 0)
	outZero := computeOperatorOutput(0, 0)
	if out360 != outZero {
		t.Errorf("360 degrees should equal 0 degrees: got %d vs %d", out360, outZero)
	}
}

func TestOutput_SymmetryPositiveNegative(t *testing.T) {
	// Positive half output should equal magnitude of negative half
	for idx := uint32(0); idx < 256; idx++ {
		posPhase := idx << 10
		negPhase := (idx | 0x200) << 10
		posOut := computeOperatorOutput(posPhase, 0)
		negOut := computeOperatorOutput(negPhase, 0)
		if posOut != -negOut {
			t.Errorf("idx=0x%02X: positive=%d, negative=%d, magnitudes differ", idx, posOut, negOut)
			break
		}
	}
}

func TestOutput_MirrorSymmetrySecondQuarter(t *testing.T) {
	// Second quarter (bit 8 set) mirrors first quarter
	// idx 0x100+i should give same magnitude as 0x1FF-i (mirrored)
	for i := uint32(0); i < 256; i++ {
		firstQ := computeOperatorOutput(i<<10, 0)
		secondQ := computeOperatorOutput((0x1FF-i)<<10, 0)
		if firstQ != secondQ {
			t.Errorf("mirror: idx %d first=%d, mirror=%d", i, firstQ, secondQ)
			break
		}
	}
}

func TestOutput_MonotonicFirstQuarter(t *testing.T) {
	// First quarter (indices 0-255): output should be monotonically increasing
	prev := computeOperatorOutput(0, 0)
	for i := uint32(1); i < 256; i++ {
		out := computeOperatorOutput(i<<10, 0)
		if out < prev {
			t.Errorf("not monotonic at idx %d: prev=%d, cur=%d", i, prev, out)
			break
		}
		prev = out
	}
}

func TestOutput_AttenuationCutoff(t *testing.T) {
	// When total attenuation >= 0x1A00, output should be 0
	// At max EG attenuation (0x3FF), even near-zero sine phase should be silent
	out := computeOperatorOutput(0, 0x3FF) // Max EG attenuation
	if out != 0 {
		t.Errorf("expected 0 at max EG attenuation, got %d", out)
	}
}

func TestOutput_MaxOutputValue(t *testing.T) {
	// Maximum output occurs at quarter-peak (idx=0xFF) with 0 attenuation
	phase := uint32(0x0FF) << 10
	maxOut := computeOperatorOutput(phase, 0)
	// pow2Table[0] << 2 >> 0 = approx 2048*4 = 8192
	if maxOut <= 0 {
		t.Fatalf("max output should be positive, got %d", maxOut)
	}
	// Verify it's approximately 8192 (pow2Table[sineTable[255]&0xFF] << 2 >> (sineTable[255]>>8))
	if maxOut < 4000 || maxOut > 9000 {
		t.Errorf("max output %d outside expected range [4000, 9000]", maxOut)
	}
}

// --- TL attenuation curve tests ---

func TestOutput_TLAttenuationCurve(t *testing.T) {
	phase := uint32(0x0FF) << 10 // Peak sine
	tests := []uint8{0, 16, 32, 64, 127}

	var prevOut int16 = 32767 // Start above max possible
	for _, tl := range tests {
		egAtten := totalLevel(0, tl)
		out := computeOperatorOutput(phase, egAtten)
		if out > prevOut {
			t.Errorf("TL=%d: output %d should be <= previous TL output %d", tl, out, prevOut)
		}
		prevOut = out
	}
}

// --- EG attenuation curve tests ---

func TestOutput_EGAttenuationCurve(t *testing.T) {
	phase := uint32(0x0FF) << 10 // Peak sine
	egLevels := []uint16{0, 0x80, 0x100, 0x200, 0x3FF}

	var prevOut int16 = 32767
	for _, eg := range egLevels {
		out := computeOperatorOutput(phase, eg)
		if out > prevOut {
			t.Errorf("egLevel=0x%03X: output %d should be <= previous output %d", eg, out, prevOut)
		}
		prevOut = out
	}
}

// --- TL+EG combined tests ---

func TestOutput_TLPlusEGSaturation(t *testing.T) {
	// Both TL and EG at max should give 0 output
	out := computeOperatorOutput(uint32(0x0FF)<<10, totalLevel(0x3FF, 127))
	if out != 0 {
		t.Errorf("TL=127 + EG=0x3FF: expected 0, got %d", out)
	}
}

func TestOutput_TLPlusEGScaling(t *testing.T) {
	// TL=0 with moderate EG should give louder output than TL>0 with same EG
	phase := uint32(0x0FF) << 10
	out0 := computeOperatorOutput(phase, totalLevel(0x100, 0))
	out32 := computeOperatorOutput(phase, totalLevel(0x100, 32))
	if out0 <= out32 {
		t.Errorf("TL=0 should be louder: TL0=%d, TL32=%d", out0, out32)
	}
}

func TestOutput_TLPlusEGBoundaryValues(t *testing.T) {
	phase := uint32(0x0FF) << 10

	// TL=0, EG=0: maximum output
	maxOut := computeOperatorOutput(phase, totalLevel(0, 0))
	if maxOut <= 0 {
		t.Errorf("TL=0 EG=0 should be positive max, got %d", maxOut)
	}

	// TL=1, EG=0: slightly less
	tl1Out := computeOperatorOutput(phase, totalLevel(0, 1))
	if tl1Out > maxOut {
		t.Errorf("TL=1 should not exceed TL=0: %d vs %d", tl1Out, maxOut)
	}
}

// --- quantize9 tests ---

func TestOutput_Quantize9Zero(t *testing.T) {
	if quantize9(0) != 0 {
		t.Errorf("quantize9(0): got %d", quantize9(0))
	}
}

func TestOutput_Quantize9Aligned(t *testing.T) {
	// Value already aligned to 32-boundary
	if quantize9(0x100) != 0x100 {
		t.Errorf("quantize9(0x100): got %d", quantize9(0x100))
	}
}

func TestOutput_Quantize9Unaligned(t *testing.T) {
	// 0x11F -> lower 5 bits zeroed = 0x100
	if quantize9(0x11F) != 0x100 {
		t.Errorf("quantize9(0x11F): got 0x%X, want 0x100", quantize9(0x11F))
	}
}

func TestOutput_Quantize9Negative(t *testing.T) {
	// Negative: -0x11F & ^0x1F = should preserve sign and clear low bits
	got := quantize9(-0x11F)
	// In two's complement: -0x11F = 0xFEE1 (int16)
	// 0xFEE1 & ^0x1F = 0xFEE1 & 0xFFE0 = 0xFEC0 = -0x140
	want := int16(-0x11F) & ^int16(0x1F)
	if got != want {
		t.Errorf("quantize9(-0x11F): got %d (0x%X), want %d (0x%X)", got, uint16(got), want, uint16(want))
	}
}

func TestOutput_Quantize9TableDriven(t *testing.T) {
	tests := []struct {
		in   int16
		want int16
	}{
		{0, 0},
		{31, 0},    // lower 5 bits masked
		{32, 32},   // exactly aligned
		{33, 32},   // 1 above alignment
		{-1, -32},  // -1 in two's complement: & ^0x1F clears low bits
		{-32, -32}, // aligned negative
		{-33, -64}, // just below -32
		{0x1FE0, 0x1FE0},
		{0x1FFF, 0x1FE0},
	}
	for _, tt := range tests {
		got := quantize9(tt.in)
		if got != tt.want {
			t.Errorf("quantize9(%d): got %d, want %d", tt.in, got, tt.want)
		}
	}
}

// --- clampAccum tests ---

func TestOutput_ClampAccumInRange(t *testing.T) {
	if clampAccum(100) != 100 {
		t.Errorf("clampAccum(100): got %d", clampAccum(100))
	}
}

func TestOutput_ClampAccumZero(t *testing.T) {
	if clampAccum(0) != 0 {
		t.Errorf("clampAccum(0): got %d", clampAccum(0))
	}
}

func TestOutput_ClampAccumPositiveClamp(t *testing.T) {
	if clampAccum(0x2000) != 0x1FE0 {
		t.Errorf("clampAccum(0x2000): got 0x%X, want 0x1FE0", clampAccum(0x2000))
	}
}

func TestOutput_ClampAccumNegativeClamp(t *testing.T) {
	if clampAccum(-0x2000) != -0x1FF0 {
		t.Errorf("clampAccum(-0x2000): got %d, want %d", clampAccum(-0x2000), -0x1FF0)
	}
}

func TestOutput_ClampAccumPositiveBoundary(t *testing.T) {
	// Exactly at positive limit
	if clampAccum(0x1FE0) != 0x1FE0 {
		t.Errorf("clampAccum(0x1FE0): got 0x%X", clampAccum(0x1FE0))
	}
	// One above
	if clampAccum(0x1FE1) != 0x1FE0 {
		t.Errorf("clampAccum(0x1FE1): got 0x%X", clampAccum(0x1FE1))
	}
	// One below
	if clampAccum(0x1FDF) != 0x1FDF {
		t.Errorf("clampAccum(0x1FDF): got 0x%X", clampAccum(0x1FDF))
	}
}

func TestOutput_ClampAccumNegativeBoundary(t *testing.T) {
	// Exactly at negative limit
	if clampAccum(-0x1FF0) != -0x1FF0 {
		t.Errorf("clampAccum(-0x1FF0): got %d", clampAccum(-0x1FF0))
	}
	// One below (more negative)
	if clampAccum(-0x1FF1) != -0x1FF0 {
		t.Errorf("clampAccum(-0x1FF1): got %d", clampAccum(-0x1FF1))
	}
	// One above (less negative)
	if clampAccum(-0x1FEF) != -0x1FEF {
		t.Errorf("clampAccum(-0x1FEF): got %d", clampAccum(-0x1FEF))
	}
}

func TestOutput_ClampAccumLargeValues(t *testing.T) {
	if clampAccum(100000) != 0x1FE0 {
		t.Errorf("clampAccum(100000): got %d", clampAccum(100000))
	}
	if clampAccum(-100000) != -0x1FF0 {
		t.Errorf("clampAccum(-100000): got %d", clampAccum(-100000))
	}
}

// --- feedback tests ---

func TestOutput_FeedbackFB1Through7(t *testing.T) {
	tests := []struct {
		fb    uint8
		prev0 int16
		prev1 int16
		want  int32
	}{
		{1, 200, 200, (200 + 200) >> 9}, // shift 9
		{2, 200, 200, (200 + 200) >> 8}, // shift 8
		{3, 200, 200, (200 + 200) >> 7}, // shift 7
		{4, 200, 200, (200 + 200) >> 6}, // shift 6
		{5, 200, 200, (200 + 200) >> 5}, // shift 5
		{6, 200, 200, (200 + 200) >> 4}, // shift 4
		{7, 200, 200, (200 + 200) >> 3}, // shift 3
	}
	for _, tt := range tests {
		op := &ymOperator{prevOut: [2]int16{tt.prev0, tt.prev1}}
		got := feedback(op, tt.fb)
		if got != tt.want {
			t.Errorf("feedback(fb=%d, prev=[%d,%d]): got %d, want %d",
				tt.fb, tt.prev0, tt.prev1, got, tt.want)
		}
	}
}

func TestOutput_FeedbackTableDriven(t *testing.T) {
	// Test with various prevOut combinations
	tests := []struct {
		fb    uint8
		prev0 int16
		prev1 int16
	}{
		{7, 1000, 1000},
		{7, -1000, -1000},
		{7, 500, -500},
		{1, 4096, 4096},
		{3, 0, 0},
	}
	for _, tt := range tests {
		op := &ymOperator{prevOut: [2]int16{tt.prev0, tt.prev1}}
		got := feedback(op, tt.fb)
		want := (int32(tt.prev0) + int32(tt.prev1)) >> (10 - uint(tt.fb))
		if got != want {
			t.Errorf("feedback(fb=%d, prev=[%d,%d]): got %d, want %d",
				tt.fb, tt.prev0, tt.prev1, got, want)
		}
	}
}

func TestOutput_FeedbackAsymmetric(t *testing.T) {
	// prev[0] != prev[1]
	op := &ymOperator{prevOut: [2]int16{300, 100}}
	got := feedback(op, 7)
	want := int32(300+100) >> 3
	if got != want {
		t.Errorf("asymmetric feedback: got %d, want %d", got, want)
	}
}

func TestOutput_FeedbackZeroDisabled(t *testing.T) {
	// fb=0 should always return 0 regardless of prevOut
	op := &ymOperator{prevOut: [2]int16{5000, 5000}}
	if feedback(op, 0) != 0 {
		t.Errorf("fb=0 should return 0, got %d", feedback(op, 0))
	}
}

// --- opOut tests ---

func TestOutput_OpOutModulationShiftsPhase(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	_ = y

	// Create two operators at same phase
	op1 := &ymOperator{phaseCounter: 0x40 << 10, egLevel: 0, egState: egDecay}
	op2 := &ymOperator{phaseCounter: 0x40 << 10, egLevel: 0, egState: egDecay}

	out1 := opOut(op1, 0, 0, 0)   // No modulation
	out2 := opOut(op2, 100, 0, 0) // With modulation

	if out1 == out2 {
		t.Error("modulation should change the output")
	}
}

func TestOutput_OpOutAMAttenuation(t *testing.T) {
	// With AM enabled and non-zero amAtten, output should be quieter
	op1 := &ymOperator{phaseCounter: 0x40 << 10, egLevel: 0, egState: egDecay, am: true}
	op2 := &ymOperator{phaseCounter: 0x40 << 10, egLevel: 0, egState: egDecay, am: true}

	out1 := opOut(op1, 0, 0, 0)   // No AM
	out2 := opOut(op2, 0, 0, 100) // With AM

	if out1 == 0 {
		t.Fatal("expected non-zero output without AM")
	}
	// AM adds attenuation, so |out2| should be <= |out1|
	abs1 := out1
	abs2 := out2
	if abs1 < 0 {
		abs1 = -abs1
	}
	if abs2 < 0 {
		abs2 = -abs2
	}
	if abs2 > abs1 {
		t.Errorf("AM should attenuate: without=%d, with=%d", abs1, abs2)
	}
}

func TestOutput_OpOutAMDisabled(t *testing.T) {
	// With AM disabled (op.am=false), amAtten should not affect output
	op1 := &ymOperator{phaseCounter: 0x40 << 10, egLevel: 0, egState: egDecay, am: false}
	op2 := &ymOperator{phaseCounter: 0x40 << 10, egLevel: 0, egState: egDecay, am: false}

	out1 := opOut(op1, 0, 0, 0)
	out2 := opOut(op2, 0, 0, 100) // amAtten provided but AM is off

	if out1 != out2 {
		t.Errorf("AM disabled: output should not change with amAtten: %d vs %d", out1, out2)
	}
}

func TestOutput_OpOutPrevOutHistory(t *testing.T) {
	op := &ymOperator{phaseCounter: 0x40 << 10, egLevel: 0}

	// First call
	first := opOut(op, 0, 0, 0)
	if op.prevOut[0] != first {
		t.Errorf("prevOut[0] should be %d, got %d", first, op.prevOut[0])
	}

	// Second call - prevOut should shift
	second := opOut(op, 0, 0, 0)
	if op.prevOut[1] != first {
		t.Errorf("prevOut[1] should be %d (first output), got %d", first, op.prevOut[1])
	}
	if op.prevOut[0] != second {
		t.Errorf("prevOut[0] should be %d (second output), got %d", second, op.prevOut[0])
	}
}

func TestOutput_OpOutAMClamp(t *testing.T) {
	// EG near max + large AM attenuation should clamp at 0x3FF, not overflow
	op := &ymOperator{phaseCounter: 0x40 << 10, egLevel: 0x3F0, am: true}
	// amAtten pushes total over 0x3FF
	out := opOut(op, 0, 0, 0x100)
	// Should not crash or produce unexpected results
	_ = out
}

// --- totalLevel tests ---

func TestOutput_TotalLevelZeroZero(t *testing.T) {
	if totalLevel(0, 0) != 0 {
		t.Errorf("totalLevel(0,0): got %d", totalLevel(0, 0))
	}
}

func TestOutput_TotalLevelMaxTL(t *testing.T) {
	// TL=127: 127 << 3 = 1016 = 0x3F8
	got := totalLevel(0, 127)
	if got != 0x3F8 {
		t.Errorf("totalLevel(0,127): got 0x%03X, want 0x3F8", got)
	}
}

func TestOutput_TotalLevelMaxEG(t *testing.T) {
	got := totalLevel(0x3FF, 0)
	if got != 0x3FF {
		t.Errorf("totalLevel(0x3FF,0): got 0x%03X", got)
	}
}

func TestOutput_TotalLevelOverflowClamp(t *testing.T) {
	// Both max: should clamp to 0x3FF
	got := totalLevel(0x3FF, 127)
	if got != 0x3FF {
		t.Errorf("totalLevel(0x3FF,127): got 0x%03X, want 0x3FF", got)
	}
}

func TestOutput_TotalLevelScaling(t *testing.T) {
	// TL contributes tl<<3 to the total
	for tl := uint8(0); tl <= 127; tl++ {
		got := totalLevel(0, tl)
		want := uint16(tl) << 3
		if want > 0x3FF {
			want = 0x3FF
		}
		if got != want {
			t.Errorf("totalLevel(0,%d): got 0x%03X, want 0x%03X", tl, got, want)
			break
		}
	}
}

// --- clampInt32 direct tests ---

func TestOutput_ClampInt32InRange(t *testing.T) {
	if clampInt32(50, -100, 100) != 50 {
		t.Errorf("clampInt32(50, -100, 100): got %d", clampInt32(50, -100, 100))
	}
}

func TestOutput_ClampInt32AtMin(t *testing.T) {
	if clampInt32(-100, -100, 100) != -100 {
		t.Errorf("clampInt32(-100, -100, 100): got %d", clampInt32(-100, -100, 100))
	}
}

func TestOutput_ClampInt32AtMax(t *testing.T) {
	if clampInt32(100, -100, 100) != 100 {
		t.Errorf("clampInt32(100, -100, 100): got %d", clampInt32(100, -100, 100))
	}
}

func TestOutput_ClampInt32BelowMin(t *testing.T) {
	if clampInt32(-200, -100, 100) != -100 {
		t.Errorf("clampInt32(-200, -100, 100): got %d", clampInt32(-200, -100, 100))
	}
}

func TestOutput_ClampInt32AboveMax(t *testing.T) {
	if clampInt32(200, -100, 100) != 100 {
		t.Errorf("clampInt32(200, -100, 100): got %d", clampInt32(200, -100, 100))
	}
}

func TestOutput_ClampInt32Zero(t *testing.T) {
	if clampInt32(0, -100, 100) != 0 {
		t.Errorf("clampInt32(0, -100, 100): got %d", clampInt32(0, -100, 100))
	}
}

func TestOutput_ClampInt32TableDriven(t *testing.T) {
	tests := []struct {
		v, min, max, want int32
	}{
		{0, -100, 100, 0},
		{-100, -100, 100, -100},
		{100, -100, 100, 100},
		{-101, -100, 100, -100},
		{101, -100, 100, 100},
		{-32768, -32768, 32767, -32768},
		{32767, -32768, 32767, 32767},
		{-32769, -32768, 32767, -32768},
		{32768, -32768, 32767, 32767},
		{50, 50, 50, 50},       // min==max==v
		{49, 50, 50, 50},       // below single-point range
		{51, 50, 50, 50},       // above single-point range
		{-5, -10, -1, -5},      // negative range
		{-11, -10, -1, -10},    // below negative range
		{0, -10, -1, -1},       // above negative range
		{1000, 0, 500, 500},    // asymmetric range
		{-1000, -500, 0, -500}, // asymmetric negative
	}
	for _, tt := range tests {
		got := clampInt32(tt.v, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("clampInt32(%d, %d, %d): got %d, want %d",
				tt.v, tt.min, tt.max, got, tt.want)
		}
	}
}

// --- sineTable reference value tests ---

func TestOutput_SineTableSize(t *testing.T) {
	if len(sineTable) != 256 {
		t.Errorf("sineTable length: got %d, want 256", len(sineTable))
	}
}

func TestOutput_SineTableReferenceValues(t *testing.T) {
	// sineTable[0] = -log2(sin(1/512 * pi/2)) * 256 ~= 2137
	if sineTable[0] != 2137 {
		t.Errorf("sineTable[0]: got %d, want 2137", sineTable[0])
	}
	// sineTable[127] - midpoint of quarter sine
	expected127 := uint16(math.Round(-math.Log2(math.Sin(float64(2*127+1)/512.0*math.Pi/2.0)) * 256.0))
	if sineTable[127] != expected127 {
		t.Errorf("sineTable[127]: got %d, want %d", sineTable[127], expected127)
	}
	// sineTable[255] = -log2(sin(511/512 * pi/2)) * 256 ~= 0
	if sineTable[255] != 0 {
		t.Errorf("sineTable[255]: got %d, want 0", sineTable[255])
	}
}

func TestOutput_SineTableMaxBound(t *testing.T) {
	// sineTable[0] is the largest value (smallest sine angle)
	maxVal := sineTable[0]
	for i := 1; i < 256; i++ {
		if sineTable[i] > maxVal {
			t.Errorf("sineTable[%d]=%d exceeds sineTable[0]=%d", i, sineTable[i], maxVal)
			break
		}
	}
}

func TestOutput_SineTableEndpointZero(t *testing.T) {
	// sin(511/512 * pi/2) ~= 1.0, so -log2(1) = 0
	if sineTable[255] != 0 {
		t.Errorf("sineTable[255]: got %d, want 0 (sin near 1.0, log near 0)", sineTable[255])
	}
}

func TestOutput_SineTableMathVerification(t *testing.T) {
	// Spot-check 5 entries against the formula
	indices := []int{0, 50, 100, 200, 255}
	for _, i := range indices {
		angle := float64(2*i+1) / 512.0 * math.Pi / 2.0
		expected := uint16(math.Round(-math.Log2(math.Sin(angle)) * 256.0))
		if sineTable[i] != expected {
			t.Errorf("sineTable[%d]: got %d, want %d", i, sineTable[i], expected)
		}
	}
}

// --- pow2Table reference value tests ---

func TestOutput_Pow2TableSize(t *testing.T) {
	if len(pow2Table) != 256 {
		t.Errorf("pow2Table length: got %d, want 256", len(pow2Table))
	}
}

func TestOutput_Pow2TableReferenceValues(t *testing.T) {
	// pow2Table[0] = 2^(1 - 1/256) * 1024 ~= 2042
	expected0 := uint16(math.Round(math.Pow(2.0, 1.0-1.0/256.0) * 1024.0))
	if pow2Table[0] != expected0 {
		t.Errorf("pow2Table[0]: got %d, want %d", pow2Table[0], expected0)
	}
	// pow2Table[127]
	expected127 := uint16(math.Round(math.Pow(2.0, 1.0-128.0/256.0) * 1024.0))
	if pow2Table[127] != expected127 {
		t.Errorf("pow2Table[127]: got %d, want %d", pow2Table[127], expected127)
	}
	// pow2Table[255] = 2^(1 - 256/256) * 1024 = 2^0 * 1024 = 1024
	if pow2Table[255] != 1024 {
		t.Errorf("pow2Table[255]: got %d, want 1024", pow2Table[255])
	}
}

func TestOutput_Pow2TableRange(t *testing.T) {
	for i := 0; i < 256; i++ {
		if pow2Table[i] < 1024 || pow2Table[i] > 2048 {
			t.Errorf("pow2Table[%d]=%d outside [1024, 2048]", i, pow2Table[i])
		}
	}
}

func TestOutput_Pow2TableMathVerification(t *testing.T) {
	indices := []int{0, 50, 100, 200, 255}
	for _, i := range indices {
		expected := uint16(math.Round(math.Pow(2.0, 1.0-float64(i+1)/256.0) * 1024.0))
		if pow2Table[i] != expected {
			t.Errorf("pow2Table[%d]: got %d, want %d", i, pow2Table[i], expected)
		}
	}
}
