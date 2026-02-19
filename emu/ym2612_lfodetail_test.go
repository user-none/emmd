package emu

import "testing"

// --- AM at specific steps ---

func TestLFODetail_AMStep0(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 0
	y.stepLFOFull()
	// step 0: output = (63 - 0) * 2 = 126 (peak)
	if y.lfoAMOut != 126 {
		t.Errorf("AM at step 0: expected 126, got %d", y.lfoAMOut)
	}
}

func TestLFODetail_AMStep32(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 32
	y.stepLFOFull()
	// step 32: output = (63 - 32) * 2 = 62
	if y.lfoAMOut != 62 {
		t.Errorf("AM at step 32: expected 62, got %d", y.lfoAMOut)
	}
}

func TestLFODetail_AMStep63(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 63
	y.stepLFOFull()
	// step 63: output = (63 - 63) * 2 = 0 (trough)
	if y.lfoAMOut != 0 {
		t.Errorf("AM at step 63: expected 0, got %d", y.lfoAMOut)
	}
}

func TestLFODetail_AMStep64(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 64
	y.stepLFOFull()
	// step 64: output = (64 - 64) * 2 = 0 (trough)
	if y.lfoAMOut != 0 {
		t.Errorf("AM at step 64: expected 0, got %d", y.lfoAMOut)
	}
}

func TestLFODetail_AMStep96(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 96
	y.stepLFOFull()
	// step 96: output = (96 - 64) * 2 = 64
	if y.lfoAMOut != 64 {
		t.Errorf("AM at step 96: expected 64, got %d", y.lfoAMOut)
	}
}

func TestLFODetail_AMStep127(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 127
	y.stepLFOFull()
	// step 127: output = (127 - 64) * 2 = 126 (peak)
	if y.lfoAMOut != 126 {
		t.Errorf("AM at step 127: expected 126, got %d", y.lfoAMOut)
	}
}

// --- AM all 128 steps table-driven ---

func TestLFODetail_AMAll128Steps(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true

	for step := uint8(0); step < 128; step++ {
		y.lfoStep = step
		y.lfoCnt = 0 // Reset counter to prevent step advancement
		y.stepLFOFull()

		var expected uint8
		if step < 64 {
			expected = (63 - step) * 2
		} else {
			expected = (step - 64) * 2
		}

		if y.lfoAMOut != expected {
			t.Errorf("AM step %d: expected %d, got %d", step, expected, y.lfoAMOut)
		}
	}
}

// --- AM symmetry ---

func TestLFODetail_AMSymmetry(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true

	// Triangle should be symmetric: step i has same value as step (127-i)
	for i := uint8(0); i < 64; i++ {
		y.lfoStep = i
		y.lfoCnt = 0
		y.stepLFOFull()
		val1 := y.lfoAMOut

		y.lfoStep = 127 - i
		y.lfoCnt = 0
		y.stepLFOFull()
		val2 := y.lfoAMOut

		if val1 != val2 {
			t.Errorf("AM not symmetric: step %d=%d, step %d=%d", i, val1, 127-i, val2)
			break
		}
	}
}

// --- AMS all values ---

func TestLFODetail_AllAMSValues(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 0 // Peak AM = 126 (step 0 is peak in descending-first triangle)
	y.stepLFOFull()

	tests := []struct {
		ams  uint8
		want uint16
	}{
		{0, 0},        // AMS 0: disabled
		{1, 126 >> 3}, // AMS 1: shift 3 = 15
		{2, 126 >> 1}, // AMS 2: shift 1 = 63
		{3, 126},      // AMS 3: shift 0 = 126
	}

	for _, tt := range tests {
		got := y.lfoAMAttenuation(tt.ams)
		if got != tt.want {
			t.Errorf("AMS=%d: got %d, want %d", tt.ams, got, tt.want)
		}
	}
}

func TestLFODetail_AMSWithSpecificAMOut(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true

	// Test with amOut=80
	y.lfoAMOut = 80

	tests := []struct {
		ams  uint8
		want uint16
	}{
		{0, 0},       // disabled
		{1, 80 >> 3}, // 10
		{2, 80 >> 1}, // 40
		{3, 80},      // 80
	}

	for _, tt := range tests {
		got := y.lfoAMAttenuation(tt.ams)
		if got != tt.want {
			t.Errorf("amOut=80 AMS=%d: got %d, want %d", tt.ams, got, tt.want)
		}
	}
}

// --- FMS all values ---

func TestLFODetail_AllFMSValues(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 4 << 2 // pmStep=4, positive quarter, idx=4

	fNum := uint16(0x400) // bit 10 set

	// FMS=0 should always be 0
	if y.lfoPMFnumDelta(0, fNum) != 0 {
		t.Error("FMS=0 should produce 0 PM delta")
	}

	// Each successive FMS should produce non-decreasing delta
	// (some adjacent FMS values share the same pmBaseTable entry at certain indices)
	prevDelta := int32(0)
	for fms := uint8(1); fms <= 7; fms++ {
		delta := y.lfoPMFnumDelta(fms, fNum)
		if delta < prevDelta {
			t.Errorf("FMS=%d delta (%d) should be >= FMS=%d delta (%d)",
				fms, delta, fms-1, prevDelta)
		}
		prevDelta = delta
	}

	// FMS=7 should be strictly larger than FMS=1
	delta1 := y.lfoPMFnumDelta(1, fNum)
	delta7 := y.lfoPMFnumDelta(7, fNum)
	if delta7 <= delta1 {
		t.Errorf("FMS=7 (%d) should be > FMS=1 (%d)", delta7, delta1)
	}
}

func TestLFODetail_FMSWithSpecificFNum(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 4 << 2 // pmStep=4

	// With fNum=0x400 (bit 10 set), delta = pmBaseTable[fms][4] >> (10-10) = pmBaseTable[fms][4]
	for fms := uint8(1); fms <= 7; fms++ {
		got := y.lfoPMFnumDelta(fms, 0x400)
		want := pmBaseTable[fms][4] // idx=4 when pmStep=4
		if got != want {
			t.Errorf("FMS=%d fNum=0x400: got %d, want %d", fms, got, want)
		}
	}
}

// --- PM quarter-wave ---

func TestLFODetail_PMQuarterWaveStep0(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 0 // pmStep=0, idx=0
	delta := y.lfoPMFnumDelta(7, 0x400)
	// pmBaseTable[7][0] = 0
	if delta != 0 {
		t.Errorf("PM at step 0 should be 0, got %d", delta)
	}
}

func TestLFODetail_PMQuarterWaveStep4(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 4 << 2 // pmStep=4, idx=4
	delta := y.lfoPMFnumDelta(7, 0x400)
	// pmBaseTable[7][4] = 64
	if delta != 64 {
		t.Errorf("PM at pmStep=4: expected 64, got %d", delta)
	}
}

func TestLFODetail_PMQuarterWaveStep7(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 7 << 2 // pmStep=7, idx=7
	delta := y.lfoPMFnumDelta(7, 0x400)
	// pmBaseTable[7][7] = 96
	if delta != 96 {
		t.Errorf("PM at pmStep=7: expected 96, got %d", delta)
	}
}

func TestLFODetail_PMQuarterWaveMirror(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true

	// pmStep=9 should mirror to idx=7-1=6
	y.lfoStep = 9 << 2
	delta9 := y.lfoPMFnumDelta(7, 0x400)

	// pmStep=6 has idx=6
	y.lfoStep = 6 << 2
	delta6 := y.lfoPMFnumDelta(7, 0x400)

	if delta9 != delta6 {
		t.Errorf("pmStep=9 should mirror pmStep=6: %d vs %d", delta9, delta6)
	}
}

func TestLFODetail_PMQuarterWavePeak(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true

	// pmStep=8 (0x08): idx = 8&7 = 0, but bit 3 is set so mirror: 7-0 = 7
	y.lfoStep = 8 << 2
	delta8 := y.lfoPMFnumDelta(7, 0x400)

	y.lfoStep = 7 << 2
	delta7 := y.lfoPMFnumDelta(7, 0x400)

	if delta8 != delta7 {
		t.Errorf("pmStep=8 should equal pmStep=7 (mirror peak): %d vs %d", delta8, delta7)
	}
}

func TestLFODetail_PMNegativeHalf(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true

	// pmStep=16 starts negative half (bit 4 set)
	y.lfoStep = 20 << 2 // pmStep=20, idx=4, negative
	deltaNeg := y.lfoPMFnumDelta(7, 0x400)

	y.lfoStep = 4 << 2 // pmStep=4, idx=4, positive
	deltaPos := y.lfoPMFnumDelta(7, 0x400)

	if deltaNeg >= 0 {
		t.Errorf("negative half should be negative, got %d", deltaNeg)
	}
	if deltaNeg != -deltaPos {
		t.Errorf("negative should equal -positive: %d vs %d", deltaNeg, -deltaPos)
	}
}

// --- Combined AM+PM ---

func TestLFODetail_CombinedAMPMSimultaneous(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 32 // Non-zero for both AM and PM
	y.stepLFOFull()

	// AM should be active
	am := y.lfoAMAttenuation(3)
	if am == 0 {
		t.Error("AM should be non-zero at step 32 with AMS=3")
	}

	// PM should be active
	pm := y.lfoPMFnumDelta(7, 0x400)
	if pm == 0 {
		t.Error("PM should be non-zero at step 32 with FMS=7")
	}
}

func TestLFODetail_AMOnlyAffectsAMEnabled(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 0 // Peak AM = 126
	y.stepLFOFull()

	// Set up two operators: one with AM enabled, one without
	op1 := &ymOperator{phaseCounter: 0x40 << 10, egLevel: 0, am: true}
	op2 := &ymOperator{phaseCounter: 0x40 << 10, egLevel: 0, am: false}

	amAtten := y.lfoAMAttenuation(3)

	out1 := opOut(op1, 0, 0, amAtten) // AM enabled
	out2 := opOut(op2, 0, 0, amAtten) // AM disabled

	// AM-enabled op should have different (quieter) output
	abs1 := out1
	abs2 := out2
	if abs1 < 0 {
		abs1 = -abs1
	}
	if abs2 < 0 {
		abs2 = -abs2
	}
	if abs1 >= abs2 {
		t.Errorf("AM-enabled op should be quieter: am_on=%d, am_off=%d", abs1, abs2)
	}
}

func TestLFODetail_PMAffectsAllOps(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 4 << 2

	// PM delta is computed per-channel, not per-operator
	fNum := uint16(0x400)
	delta := y.lfoPMFnumDelta(7, fNum)

	if delta == 0 {
		t.Error("PM should affect all operators (non-zero delta)")
	}
}

func TestLFODetail_DisableResetsAM(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 0 // Peak AM = 126
	y.stepLFOFull()

	if y.lfoAMOut == 0 {
		t.Fatal("AM should be non-zero when enabled")
	}

	// Disable LFO
	y.lfoEnable = false
	y.stepLFOFull()

	if y.lfoAMOut != 0 {
		t.Errorf("AM should be 0 when LFO disabled, got %d", y.lfoAMOut)
	}
}

func TestLFODetail_DisableResetsCounter(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable LFO via register $22 (bit 3 = enable, freq=7)
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x0F)

	// Advance LFO so step and sub-counter are non-zero
	for i := 0; i < 200; i++ {
		y.stepLFOFull()
	}
	if y.lfoStep == 0 {
		t.Fatal("lfoStep should have advanced from 0")
	}

	// Disable LFO via register $22 (bit 3 = 0)
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x00)

	if y.lfoStep != 0 {
		t.Errorf("lfoStep should be reset to 0 on disable, got %d", y.lfoStep)
	}
	if y.lfoCnt != 0 {
		t.Errorf("lfoCnt should be reset to 0 on disable, got %d", y.lfoCnt)
	}
}

// --- pmBaseTable verification ---

func TestLFODetail_PMBaseTableRow0AllZeros(t *testing.T) {
	for col := 0; col < 8; col++ {
		if pmBaseTable[0][col] != 0 {
			t.Errorf("pmBaseTable[0][%d]: expected 0, got %d", col, pmBaseTable[0][col])
		}
	}
}

func TestLFODetail_PMBaseTableMonotonicity(t *testing.T) {
	// Within each row, values should be non-decreasing
	for row := 0; row < 8; row++ {
		for col := 1; col < 8; col++ {
			if pmBaseTable[row][col] < pmBaseTable[row][col-1] {
				t.Errorf("pmBaseTable[%d] not monotonic: col %d (%d) < col %d (%d)",
					row, col, pmBaseTable[row][col], col-1, pmBaseTable[row][col-1])
				break
			}
		}
	}

	// Each row should have sum >= previous row
	for row := 1; row < 8; row++ {
		sum1 := int32(0)
		sum2 := int32(0)
		for col := 0; col < 8; col++ {
			sum1 += pmBaseTable[row-1][col]
			sum2 += pmBaseTable[row][col]
		}
		if sum2 < sum1 {
			t.Errorf("pmBaseTable row %d sum (%d) < row %d sum (%d)",
				row, sum2, row-1, sum1)
		}
	}
}

func TestLFODetail_LFOAMShiftTable(t *testing.T) {
	expected := [4]uint8{8, 3, 1, 0}
	for i, want := range expected {
		if lfoAMShift[i] != want {
			t.Errorf("lfoAMShift[%d]: got %d, want %d", i, lfoAMShift[i], want)
		}
	}
}

func TestLFODetail_LFOPeriodTable(t *testing.T) {
	expected := [8]uint16{108, 77, 71, 67, 62, 44, 8, 5}
	for i, want := range expected {
		if lfoPeriodTable[i] != want {
			t.Errorf("lfoPeriodTable[%d]: got %d, want %d", i, lfoPeriodTable[i], want)
		}
	}
}
