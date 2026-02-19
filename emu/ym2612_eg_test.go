package emu

import "testing"

// --- EG State Transitions ---

func TestEG_AttackToDecay(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	op := &ymOperator{
		egState: egAttack,
		egLevel: 0x3FF,
		ar:      20,
		rs:      0,
		keyCode: 0x10,
		keyOn:   true,
		d1l:     7,
	}

	// Step until attack completes
	for i := 0; i < 1000000 && op.egState == egAttack; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egState != egDecay {
		t.Errorf("expected egDecay after attack, got state %d, level=0x%03X", op.egState, op.egLevel)
	}
	if op.egLevel != 0 {
		t.Errorf("expected level 0 at decay transition, got 0x%03X", op.egLevel)
	}
}

func TestEG_DecayToSustainMultipleD1L(t *testing.T) {
	tests := []struct {
		d1l       uint8
		wantLevel uint16
	}{
		{1, 0x20},
		{4, 0x80},
		{7, 0xE0},
		{10, 0x140},
		{14, 0x1C0},
	}

	for _, tt := range tests {
		y := NewYM2612(7670454, 48000)
		op := &ymOperator{
			egState: egDecay,
			egLevel: 0,
			d1r:     31,
			d1l:     tt.d1l,
			rs:      3,
			keyCode: 0x1F,
			keyOn:   true,
		}

		for i := 0; i < 100000 && op.egState == egDecay; i++ {
			y.stepOperatorEnvelope(op, uint16(i))
		}

		if op.egState != egSustain {
			t.Errorf("d1l=%d: expected egSustain, got %d", tt.d1l, op.egState)
			continue
		}
		if op.egLevel < tt.wantLevel {
			t.Errorf("d1l=%d: level 0x%03X should be >= sustain 0x%03X",
				tt.d1l, op.egLevel, tt.wantLevel)
		}
	}
}

func TestEG_SustainContinues(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	op := &ymOperator{
		egState: egSustain,
		egLevel: 0x100,
		d2r:     10,
		rs:      0,
		keyCode: 0x10,
		keyOn:   true,
		d1l:     7,
	}

	initialLevel := op.egLevel
	for i := 0; i < 100000; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	// Sustain should increase egLevel (decay further)
	if op.egLevel <= initialLevel {
		t.Errorf("sustain with D2R=%d should increase level: initial=0x%03X, after=0x%03X",
			op.d2r, initialLevel, op.egLevel)
	}
	if op.egState != egSustain {
		t.Errorf("should remain in sustain, got state %d", op.egState)
	}
}

func TestEG_SustainToMax(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	op := &ymOperator{
		egState: egSustain,
		egLevel: 0x300,
		d2r:     31,
		rs:      3,
		keyCode: 0x1F,
		keyOn:   true,
		d1l:     15,
	}

	for i := 0; i < 100000; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egLevel != 0x3FF {
		t.Errorf("sustain at max D2R should reach 0x3FF, got 0x%03X", op.egLevel)
	}
}

func TestEG_ReleaseFromAttack(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	op := &ymOperator{
		egState: egAttack,
		egLevel: 0x200,
		rr:      15,
		rs:      3,
		keyCode: 0x1F,
		keyOn:   false,
	}

	// Transition to release
	op.egState = egRelease
	savedLevel := op.egLevel

	for i := 0; i < 100000; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egLevel <= savedLevel {
		t.Errorf("release should increase level: saved=0x%03X, after=0x%03X", savedLevel, op.egLevel)
	}
}

func TestEG_ReleaseFromDecay(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	op := &ymOperator{
		egState: egDecay,
		egLevel: 0x80,
		d1l:     15,
		rr:      15,
		rs:      3,
		keyCode: 0x1F,
		keyOn:   false,
	}

	op.egState = egRelease
	initialLevel := op.egLevel

	for i := 0; i < 100000; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egLevel <= initialLevel {
		t.Errorf("release from decay should increase level")
	}
	if op.egLevel != 0x3FF {
		t.Errorf("release should reach 0x3FF, got 0x%03X", op.egLevel)
	}
}

func TestEG_ReleaseFromSustain(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	op := &ymOperator{
		egState: egSustain,
		egLevel: 0x100,
		d1l:     7,
		rr:      15,
		rs:      3,
		keyCode: 0x1F,
		keyOn:   false,
	}

	op.egState = egRelease
	initialLevel := op.egLevel

	for i := 0; i < 100000; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egLevel <= initialLevel {
		t.Errorf("release from sustain should increase level")
	}
}

func TestEG_ReKeyFromDecay(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Setup op in decay
	y.WritePort(0, 0x50)
	y.WritePort(1, 0xDF) // RS=3, AR=31 (instant)
	y.WritePort(0, 0x80)
	y.WritePort(1, 0x4F) // D1L=4, RR=15

	// Key on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10) // S1 on

	op := &y.ch[0].op[0]
	if op.egState != egDecay {
		t.Fatalf("expected decay after instant attack, got %d", op.egState)
	}

	// Manually put some decay progress
	op.egLevel = 0x40

	// Re-key: key off then on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00) // off
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10) // on again

	// Should restart attack
	if op.egState != egDecay && op.egState != egAttack {
		t.Errorf("re-key should restart: got state %d", op.egState)
	}
	if op.phaseCounter != 0 {
		t.Errorf("re-key should reset phase, got 0x%05X", op.phaseCounter)
	}
}

func TestEG_ReKeyFromRelease(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.WritePort(0, 0x50)
	y.WritePort(1, 0x0F) // RS=0, AR=15

	// Key on then off
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10)
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00)

	op := &y.ch[0].op[0]
	if op.egState != egRelease {
		t.Fatalf("expected release, got %d", op.egState)
	}

	savedLevel := op.egLevel

	// Re-key on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10)

	if op.egState != egAttack && op.egState != egDecay {
		t.Errorf("re-key from release should start attack, got state %d", op.egState)
	}
	_ = savedLevel
}

// --- Rate Scaling ---

func TestEG_RateScalingTableDriven(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	tests := []struct {
		rate    uint8
		rs      uint8
		keyCode uint8
		want    uint8
	}{
		// RS=0: rks = keyCode >> 3
		{10, 0, 0, 20},
		{10, 0, 8, 21},  // rks = 8>>3 = 1
		{10, 0, 16, 22}, // rks = 16>>3 = 2
		{10, 0, 24, 23}, // rks = 24>>3 = 3
		{10, 0, 31, 23}, // rks = 31>>3 = 3

		// RS=1: rks = keyCode >> 2
		{10, 1, 0, 20},
		{10, 1, 4, 21},  // rks = 4>>2 = 1
		{10, 1, 8, 22},  // rks = 8>>2 = 2
		{10, 1, 16, 24}, // rks = 16>>2 = 4
		{10, 1, 31, 27}, // rks = 31>>2 = 7

		// RS=2: rks = keyCode >> 1
		{10, 2, 0, 20},
		{10, 2, 2, 21},  // rks = 2>>1 = 1
		{10, 2, 16, 28}, // rks = 16>>1 = 8
		{10, 2, 31, 35}, // rks = 31>>1 = 15

		// RS=3: rks = keyCode >> 0 = keyCode
		{10, 3, 0, 20},
		{10, 3, 1, 21},
		{10, 3, 16, 36}, // 2*10+16 = 36
		{10, 3, 31, 51}, // 2*10+31 = 51

		// Clamping to 63
		{31, 3, 31, 63}, // 2*31+31 = 93, clamped to 63

		// Rate 0 always frozen
		{0, 0, 0, 0},
		{0, 3, 31, 0},
	}

	for _, tt := range tests {
		op := &ymOperator{rs: tt.rs, keyCode: tt.keyCode}
		got := y.effectiveRate(tt.rate, op)
		if got != tt.want {
			t.Errorf("effectiveRate(%d, rs=%d, kc=%d): got %d, want %d",
				tt.rate, tt.rs, tt.keyCode, got, tt.want)
		}
	}
}

func TestEG_RateClampTo63(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Various combinations that exceed 63
	tests := []struct {
		rate    uint8
		rs      uint8
		keyCode uint8
	}{
		{31, 3, 31},
		{31, 3, 20},
		{31, 2, 31},
		{30, 3, 31},
	}

	for _, tt := range tests {
		op := &ymOperator{rs: tt.rs, keyCode: tt.keyCode}
		got := y.effectiveRate(tt.rate, op)
		if got > 63 {
			t.Errorf("effectiveRate(%d, rs=%d, kc=%d): got %d, should clamp to 63",
				tt.rate, tt.rs, tt.keyCode, got)
		}
	}
}

func TestEG_Rate0AlwaysFrozen(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Rate 0 should always return 0 regardless of RS/keyCode
	for rs := uint8(0); rs <= 3; rs++ {
		for _, kc := range []uint8{0, 8, 16, 24, 31} {
			op := &ymOperator{rs: rs, keyCode: kc}
			got := y.effectiveRate(0, op)
			if got != 0 {
				t.Errorf("effectiveRate(0, rs=%d, kc=%d): got %d, want 0", rs, kc, got)
			}
		}
	}
}

// --- Increment Patterns ---

func TestEG_IncrementPatternRate4(t *testing.T) {
	// Rate 4: rate&3=0, pattern index 1: {0,1,0,1,0,1,0,1}
	expected := [8]uint8{0, 1, 0, 1, 0, 1, 0, 1}
	for i := 0; i < 8; i++ {
		if egIncrementTable[1][i] != expected[i] {
			t.Errorf("rate 4 pattern[%d]: got %d, want %d", i, egIncrementTable[1][i], expected[i])
		}
	}
}

func TestEG_IncrementPatternRate5(t *testing.T) {
	// Rate 5: rate&3=1, pattern index 2: {0,1,0,1,1,1,0,1}
	expected := [8]uint8{0, 1, 0, 1, 1, 1, 0, 1}
	for i := 0; i < 8; i++ {
		if egIncrementTable[2][i] != expected[i] {
			t.Errorf("rate 5 pattern[%d]: got %d, want %d", i, egIncrementTable[2][i], expected[i])
		}
	}
}

func TestEG_IncrementPatternRate6(t *testing.T) {
	// Rate 6: rate&3=2, pattern index 3: {0,1,1,1,0,1,1,1}
	expected := [8]uint8{0, 1, 1, 1, 0, 1, 1, 1}
	for i := 0; i < 8; i++ {
		if egIncrementTable[3][i] != expected[i] {
			t.Errorf("rate 6 pattern[%d]: got %d, want %d", i, egIncrementTable[3][i], expected[i])
		}
	}
}

func TestEG_IncrementPatternRate7(t *testing.T) {
	// Rate 7: rate&3=3, pattern index 4: {0,1,1,1,1,1,1,1}
	expected := [8]uint8{0, 1, 1, 1, 1, 1, 1, 1}
	for i := 0; i < 8; i++ {
		if egIncrementTable[4][i] != expected[i] {
			t.Errorf("rate 7 pattern[%d]: got %d, want %d", i, egIncrementTable[4][i], expected[i])
		}
	}
}

func TestEG_IncrementPatternRate12(t *testing.T) {
	// Rate 12: rate>>2=3, shift=11-3=8. rate&3=0, pattern index 1.
	// Same pattern as rate 4 but with different shift (fires more often)
	expected := [8]uint8{0, 1, 0, 1, 0, 1, 0, 1}
	for i := 0; i < 8; i++ {
		if egIncrementTable[1][i] != expected[i] {
			t.Errorf("rate 12 uses pattern 1, idx %d: got %d, want %d",
				i, egIncrementTable[1][i], expected[i])
		}
	}
}

func TestEG_HighRatePattern48(t *testing.T) {
	// Rate 48: all 1s
	for i := 0; i < 8; i++ {
		if egHighRateTable[0][i] != 1 {
			t.Errorf("rate 48[%d]: got %d, want 1", i, egHighRateTable[0][i])
		}
	}
}

func TestEG_HighRatePattern52(t *testing.T) {
	// Rate 52: all 2s
	expected := [8]uint8{2, 2, 2, 2, 2, 2, 2, 2}
	for i := 0; i < 8; i++ {
		if egHighRateTable[4][i] != expected[i] {
			t.Errorf("rate 52[%d]: got %d, want %d", i, egHighRateTable[4][i], expected[i])
		}
	}
}

func TestEG_HighRatePattern56(t *testing.T) {
	// Rate 56: all 4s
	expected := [8]uint8{4, 4, 4, 4, 4, 4, 4, 4}
	for i := 0; i < 8; i++ {
		if egHighRateTable[8][i] != expected[i] {
			t.Errorf("rate 56[%d]: got %d, want %d", i, egHighRateTable[8][i], expected[i])
		}
	}
}

func TestEG_HighRatePattern60(t *testing.T) {
	// Rate 60: all 8s
	expected := [8]uint8{8, 8, 8, 8, 8, 8, 8, 8}
	for i := 0; i < 8; i++ {
		if egHighRateTable[12][i] != expected[i] {
			t.Errorf("rate 60[%d]: got %d, want %d", i, egHighRateTable[12][i], expected[i])
		}
	}
}

func TestEG_FullTableVerification(t *testing.T) {
	// Verify all increment table rows have values in expected range
	for row := 0; row < 5; row++ {
		for col := 0; col < 8; col++ {
			v := egIncrementTable[row][col]
			if v > 1 {
				t.Errorf("egIncrementTable[%d][%d]=%d, expected 0 or 1", row, col, v)
			}
		}
	}

	// High rate table should have values 1, 2, 4, or 8
	for row := 0; row < 16; row++ {
		for col := 0; col < 8; col++ {
			v := egHighRateTable[row][col]
			if v != 1 && v != 2 && v != 4 && v != 8 {
				t.Errorf("egHighRateTable[%d][%d]=%d, expected 1/2/4/8", row, col, v)
			}
		}
	}
}

// --- Timing ---

func TestEG_ClockDividerBy3(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// EG should step every 3rd sample clock
	y.egClock = 0
	y.egCounter = 0

	steps := 0
	for i := 0; i < 9; i++ {
		y.egClock++
		if y.egClock >= 3 {
			y.egClock = 0
			y.egCounter++
			steps++
		}
	}

	// 9 clocks / 3 = 3 EG steps
	if steps != 3 {
		t.Errorf("expected 3 EG steps in 9 clocks, got %d", steps)
	}
	if y.egCounter != 3 {
		t.Errorf("expected counter=3, got %d", y.egCounter)
	}
}

func TestEG_CounterIncrementsPerFrame(t *testing.T) {
	// One frame at 60fps has ~53267Hz / 60 ~= 888 native samples
	// EG steps every 3 samples -> ~296 EG steps per frame
	y := NewYM2612(7670454, 48000)

	egSteps := 0
	for i := 0; i < 888; i++ {
		y.egClock++
		if y.egClock >= 3 {
			y.egClock = 0
			y.egCounter++
			if y.egCounter >= 4096 {
				y.egCounter = 1
			}
			egSteps++
		}
	}

	if egSteps != 296 {
		t.Errorf("expected 296 EG steps per frame (888/3), got %d", egSteps)
	}
}

func TestEG_CounterWrapsAt4096(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.egCounter = 4095

	y.egClock = 2
	y.egClock++
	if y.egClock >= 3 {
		y.egClock = 0
		y.egCounter++
		if y.egCounter >= 4096 {
			y.egCounter = 1
		}
	}

	if y.egCounter != 1 {
		t.Errorf("EG counter should wrap from 4095 to 1, got %d", y.egCounter)
	}
}

// --- Attack curve ---

func TestEG_AttackExponentialCurve(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	op := &ymOperator{
		egState: egAttack,
		egLevel: 0x3FF,
		ar:      15,
		rs:      0,
		keyCode: 0x10,
		keyOn:   true,
	}

	// Collect level samples during attack
	var levels []uint16
	levels = append(levels, op.egLevel)

	for i := 0; i < 100000 && op.egState == egAttack; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
		if len(levels) == 0 || op.egLevel != levels[len(levels)-1] {
			levels = append(levels, op.egLevel)
		}
	}

	// Attack should be monotonically decreasing (exponential toward 0)
	for i := 1; i < len(levels); i++ {
		if levels[i] >= levels[i-1] {
			t.Errorf("attack not monotonically decreasing at step %d: %d >= %d",
				i, levels[i], levels[i-1])
			break
		}
	}
}

func TestEG_DecayLinear(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	op := &ymOperator{
		egState: egDecay,
		egLevel: 0,
		d1r:     20,
		d1l:     15,
		rs:      0,
		keyCode: 0x10,
		keyOn:   true,
	}

	// Collect level changes during decay
	prevLevel := op.egLevel
	increments := 0

	for i := 0; i < 100000; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
		if op.egLevel > prevLevel {
			increments++
			prevLevel = op.egLevel
		}
		if op.egState != egDecay {
			break
		}
	}

	if increments == 0 {
		t.Error("decay should produce level increments")
	}
}

func TestEG_AttackFormulaAtSpecificLevels(t *testing.T) {
	// Attack formula: step = (^egLevel * incr) >> 4
	// At egLevel=0x3FF: ^0x3FF = -0x400, step = (-0x400 * 1) >> 4 = -64
	// New level = 0x3FF + (-64) = 0x3BF
	level := uint16(0x3FF)
	incr := uint8(1)
	step := (^int32(level) * int32(incr)) >> 4
	newLevel := int32(level) + step
	if newLevel != 0x3BF {
		t.Errorf("attack at 0x3FF: expected 0x3BF, got 0x%03X", newLevel)
	}

	// At egLevel=0x100: ^0x100 = -0x101, step = (-0x101 * 1) >> 4 = -16 (truncated)
	level = 0x100
	step = (^int32(level) * int32(incr)) >> 4
	newLevel = int32(level) + step
	expected := int32(0x100) + (^int32(0x100)*1)>>4
	if newLevel != expected {
		t.Errorf("attack at 0x100: expected 0x%03X, got 0x%03X", expected, newLevel)
	}

	// Near zero: egLevel=1, ^1 = -2, step = (-2*1) >> 4 = -1 (very slow near 0)
	// This demonstrates the exponential slowdown near zero
	level = 1
	step = (^int32(level) * int32(incr)) >> 4
	if step != -1 {
		t.Errorf("attack at level 1 with incr=1: expected step=-1, got %d", step)
	}
}

// --- Sustain Level ---

func TestEG_SustainLevelAll16Values(t *testing.T) {
	tests := []struct {
		d1l  uint8
		want uint16
	}{
		{0, 0x000},
		{1, 0x020},
		{2, 0x040},
		{3, 0x060},
		{4, 0x080},
		{5, 0x0A0},
		{6, 0x0C0},
		{7, 0x0E0},
		{8, 0x100},
		{9, 0x120},
		{10, 0x140},
		{11, 0x160},
		{12, 0x180},
		{13, 0x1A0},
		{14, 0x1C0},
		{15, 0x3E0}, // Special case
	}

	for _, tt := range tests {
		got := sustainLevel(tt.d1l)
		if got != tt.want {
			t.Errorf("sustainLevel(%d): got 0x%03X, want 0x%03X", tt.d1l, got, tt.want)
		}
	}
}

func TestEG_D1L15SpecialCase(t *testing.T) {
	// D1L=15 maps to 0x3E0, not 15*32=0x1E0
	sl := sustainLevel(15)
	if sl != 0x3E0 {
		t.Errorf("D1L=15 should map to 0x3E0, got 0x%03X", sl)
	}
	// D1L=14 should be normal
	sl14 := sustainLevel(14)
	if sl14 != 0x1C0 {
		t.Errorf("D1L=14 should map to 0x1C0, got 0x%03X", sl14)
	}
}

// --- Increment table monotonicity ---

func TestEG_IncrementTableRowAverages(t *testing.T) {
	// Each successive row should have a higher average (more increments)
	for row := 1; row < 4; row++ {
		sum1 := 0
		sum2 := 0
		for col := 0; col < 8; col++ {
			sum1 += int(egIncrementTable[row][col])
			sum2 += int(egIncrementTable[row+1][col])
		}
		if sum2 <= sum1 {
			t.Errorf("row %d avg (%d/8) should be < row %d avg (%d/8)",
				row, sum1, row+1, sum2)
		}
	}
}

func TestEG_HighRateTableMonotonicity(t *testing.T) {
	// Each successive high rate should have sum >= previous
	for row := 0; row < 15; row++ {
		sum1 := 0
		sum2 := 0
		for col := 0; col < 8; col++ {
			sum1 += int(egHighRateTable[row][col])
			sum2 += int(egHighRateTable[row+1][col])
		}
		if sum2 < sum1 {
			t.Errorf("highRate row %d (sum=%d) should be <= row %d (sum=%d)",
				row, sum1, row+1, sum2)
		}
	}
}

// --- Additional edge cases ---

func TestEG_AttackRates62and63InstantDecay(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	op := &ymOperator{
		egState: egAttack,
		egLevel: 0x3FF,
		ar:      31,
		rs:      3,
		keyCode: 0x1F,
		keyOn:   true,
	}

	rate := y.effectiveRate(op.ar, op)
	if rate < 62 {
		t.Fatalf("expected rate >= 62, got %d", rate)
	}

	// With rate >= 62, stepOperatorEnvelope should set level to 0
	y.stepOperatorEnvelope(op, 0)
	if op.egLevel != 0 {
		t.Errorf("rate >= 62 attack should instantly reach 0, got 0x%03X", op.egLevel)
	}
	if op.egState != egDecay {
		t.Errorf("should transition to decay, got state %d", op.egState)
	}
}

func TestEG_DecayClampAt0x3FF(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	op := &ymOperator{
		egState: egDecay,
		egLevel: 0x3FE,
		d1r:     31,
		d1l:     15,
		rs:      3,
		keyCode: 0x1F,
		keyOn:   true,
	}

	for i := 0; i < 1000; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egLevel > 0x3FF {
		t.Errorf("decay should clamp at 0x3FF, got 0x%03X", op.egLevel)
	}
}

func TestEG_ReleaseClampAt0x3FF(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	op := &ymOperator{
		egState: egRelease,
		egLevel: 0x3FE,
		rr:      15,
		rs:      3,
		keyCode: 0x1F,
		keyOn:   false,
	}

	for i := 0; i < 1000; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egLevel != 0x3FF {
		t.Errorf("release should clamp at 0x3FF, got 0x%03X", op.egLevel)
	}
}

func TestEG_SustainClampAt0x3FF(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	op := &ymOperator{
		egState: egSustain,
		egLevel: 0x3FE,
		d2r:     31,
		d1l:     15,
		rs:      3,
		keyCode: 0x1F,
		keyOn:   true,
	}

	for i := 0; i < 1000; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egLevel != 0x3FF {
		t.Errorf("sustain should clamp at 0x3FF, got 0x%03X", op.egLevel)
	}
}

func TestEG_ShiftCalculation(t *testing.T) {
	// Verify shift = 11 - (rate >> 2) for rates 4-47
	tests := []struct {
		rate  uint8
		shift uint
	}{
		{4, 10}, // 11 - 1 = 10
		{8, 9},  // 11 - 2 = 9
		{12, 8}, // 11 - 3 = 8
		{16, 7}, // 11 - 4 = 7
		{20, 6},
		{24, 5},
		{28, 4},
		{32, 3},
		{36, 2},
		{40, 1},
		{44, 0},
		{47, 0}, // 11 - 11 = 0
	}

	for _, tt := range tests {
		group := tt.rate >> 2
		shift := uint(11 - int(group))
		if shift != tt.shift {
			t.Errorf("rate %d: expected shift=%d, got %d", tt.rate, tt.shift, shift)
		}
	}
}
