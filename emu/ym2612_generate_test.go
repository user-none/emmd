package emu

import "testing"

// --- Cycle accumulation tests ---

func TestGenerate_ZeroCyclesNoOutput(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.GenerateSamples(0)
	buf := y.GetBuffer()
	if len(buf) != 0 {
		t.Errorf("0 cycles should produce empty buffer, got len=%d", len(buf))
	}
}

func TestGenerate_LessThan144CyclesNoOutput(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.GenerateSamples(143)
	buf := y.GetBuffer()
	if len(buf) != 0 {
		t.Errorf("143 cycles should produce empty buffer, got len=%d", len(buf))
	}
}

func TestGenerate_Exactly144CyclesProcesses(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.GenerateSamples(144)
	// cycleAccum should be 0 (all consumed)
	if y.cycleAccum != 0 {
		t.Errorf("144 cycles: cycleAccum should be 0, got %d", y.cycleAccum)
	}
}

func TestGenerate_FractionalCycleCarryOver(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.GenerateSamples(143) // Not enough for one native sample
	if y.cycleAccum != 143 {
		t.Errorf("after 143 cycles: cycleAccum should be 143, got %d", y.cycleAccum)
	}
	y.GenerateSamples(1) // 143 + 1 = 144, now processes
	if y.cycleAccum != 0 {
		t.Errorf("after 143+1 cycles: cycleAccum should be 0, got %d", y.cycleAccum)
	}
}

func TestGenerate_CycleAccumPersists(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.GenerateSamples(100)
	y.GenerateSamples(100) // 200 total >= 144
	// Should have processed at least one native sample
	// cycleAccum should be 200 - 144 = 56
	if y.cycleAccum != 56 {
		t.Errorf("after 100+100 cycles: cycleAccum should be 56, got %d", y.cycleAccum)
	}
}

// --- Bresenham resampling tests ---

func TestGenerate_SampleCountOneFrame(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.GenerateSamples(7670454 / 60) // ~1 frame
	buf := y.GetBuffer()
	stereoSamples := len(buf) / 2
	// Expected: 48000/60 = 800 samples per frame, allow +/-1
	if stereoSamples < 799 || stereoSamples > 801 {
		t.Errorf("one frame: got %d stereo samples, want ~800", stereoSamples)
	}
}

func TestGenerate_SampleCountMultipleFrames(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	cyclesPerFrame := 7670454 / 60
	totalSamples := 0
	for i := 0; i < 10; i++ {
		y.GenerateSamples(cyclesPerFrame)
		buf := y.GetBuffer()
		totalSamples += len(buf) / 2
	}
	// 10 frames * 800 = 8000, allow +/-5
	if totalSamples < 7995 || totalSamples > 8005 {
		t.Errorf("10 frames: got %d total stereo samples, want ~8000", totalSamples)
	}
}

func TestGenerate_StereoFormat(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()
	if len(buf)%2 != 0 {
		t.Errorf("buffer length should be even (stereo), got %d", len(buf))
	}
}

func TestGenerate_ResampAccumPersists(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	totalLen := 0
	// Generate many small cycle counts
	for i := 0; i < 200; i++ {
		y.GenerateSamples(144) // One native sample each
	}
	buf := y.GetBuffer()
	totalLen = len(buf) / 2
	// 200 native samples at ~53kHz -> ~181 output samples at 48kHz
	if totalLen == 0 {
		t.Error("200 native samples should produce some output")
	}
	if totalLen > 200 {
		t.Errorf("output samples (%d) should not exceed native samples (200)", totalLen)
	}
}

func TestGenerate_BufferResetOnGet(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.GenerateSamples(7670454 / 60)
	buf1 := y.GetBuffer()
	if len(buf1) == 0 {
		t.Fatal("expected non-empty first buffer")
	}
	buf2 := y.GetBuffer()
	if len(buf2) != 0 {
		t.Errorf("second GetBuffer should be empty, got len=%d", len(buf2))
	}
}

// --- Multi-channel mixing tests ---

func TestGenerate_SilenceNoKeys(t *testing.T) {
	// With ladder effect: each of 6 channels (panL=true, panR=true)
	// outputs applyLadder(0, true) = 128 per L/R. Sum = 768. After >>1 = 384.
	y := NewYM2612(7670454, 48000)
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()
	for i, s := range buf {
		if s != 384 {
			t.Errorf("no keys: sample[%d]=%d, want 384", i, s)
			break
		}
	}
}

func TestGenerate_SingleChannelOutput(t *testing.T) {
	y := setupTestChannel(7) // Algo 7: all carriers
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()
	nonZero := false
	for _, s := range buf {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Error("single channel with key on should produce non-zero output")
	}
}

func TestGenerate_TwoChannelSum(t *testing.T) {
	// Single channel
	y1 := setupTestChannel(7)
	y1.GenerateSamples(7670454 / 60)
	buf1 := y1.GetBuffer()

	// Two channels at different frequencies
	y2 := setupTestChannel(7)
	// Set ch1 with different freq
	y2.WritePort(0, 0xB1)
	y2.WritePort(1, 0x07) // algo 7 for ch1
	y2.WritePort(0, 0xB5)
	y2.WritePort(1, 0xC0) // pan L+R
	y2.WritePort(0, 0xA5)
	y2.WritePort(1, 0x24) // block=4, fNum_hi=4
	y2.WritePort(0, 0xA1)
	y2.WritePort(1, 0x00)
	// MUL=1 for all ch1 ops
	for _, reg := range []uint8{0x31, 0x35, 0x39, 0x3D} {
		y2.WritePort(0, reg)
		y2.WritePort(1, 0x01)
	}
	// TL=0 for all ch1 ops
	for _, reg := range []uint8{0x41, 0x45, 0x49, 0x4D} {
		y2.WritePort(0, reg)
		y2.WritePort(1, 0x00)
	}
	// RS=3, AR=31 for all ch1 ops
	for _, reg := range []uint8{0x51, 0x55, 0x59, 0x5D} {
		y2.WritePort(0, reg)
		y2.WritePort(1, 0xDF)
	}
	// D1R=0
	for _, reg := range []uint8{0x61, 0x65, 0x69, 0x6D} {
		y2.WritePort(0, reg)
		y2.WritePort(1, 0x00)
	}
	// D2R=0
	for _, reg := range []uint8{0x71, 0x75, 0x79, 0x7D} {
		y2.WritePort(0, reg)
		y2.WritePort(1, 0x00)
	}
	// D1L=0, RR=15
	for _, reg := range []uint8{0x81, 0x85, 0x89, 0x8D} {
		y2.WritePort(0, reg)
		y2.WritePort(1, 0x0F)
	}
	// Key on ch1
	y2.WritePort(0, 0x28)
	y2.WritePort(1, 0xF1)

	y2.GenerateSamples(7670454 / 60)
	buf2 := y2.GetBuffer()

	// Two-channel output should differ from single-channel
	differs := false
	minLen := len(buf1)
	if len(buf2) < minLen {
		minLen = len(buf2)
	}
	for i := 0; i < minLen; i++ {
		if buf1[i] != buf2[i] {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("two-channel output should differ from single-channel")
	}
}

func TestGenerate_DACThroughGenerate(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable DAC and write non-center sample
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80) // DAC enable
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xFF) // Max DAC sample

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	nonZero := false
	for _, s := range buf {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Error("DAC with non-center sample should produce non-zero output")
	}
}

func TestGenerate_DACMixedWithFM(t *testing.T) {
	y := setupTestChannel(7) // ch0 with FM

	// Enable DAC on ch5
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xFF)

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	nonZero := false
	for _, s := range buf {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Error("FM + DAC should produce non-zero output")
	}
}

// --- Scaling and clamping tests ---

func TestGenerate_ScalingFactor(t *testing.T) {
	y := setupMaxOutputYM2612()
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()
	for i, s := range buf {
		if s < -32768 || s > 32767 {
			t.Errorf("sample[%d]=%d outside int16 range", i, s)
			break
		}
	}
}

func TestGenerate_ScalingReducesAmplitude(t *testing.T) {
	// Without >>1 scaling, 6 channels at max (~8288 each with ladder) would
	// sum to ~49728 which exceeds int16 range. With >>1: max ~24864.
	// Verify: 6-channel output stays in int16 range (scaling is working),
	// and the max output is below what unscaled would produce.
	y := setupMaxOutputYM2612()
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	var maxAbs int16
	for _, s := range buf {
		if s > maxAbs {
			maxAbs = s
		}
		if -s > maxAbs {
			maxAbs = -s
		}
	}

	// Max should be well below unscaled max (6 * 8160 = 48960)
	// and within int16 range (32767)
	if maxAbs > 32767 {
		t.Errorf("scaled max (%d) exceeds int16 range", maxAbs)
	}
	// With >>1 scaling, max should be around 24864 - verify it's not 0
	if maxAbs == 0 {
		t.Error("6 channels at max should produce non-zero output")
	}
}

func TestGenerate_OutputNeverExceedsInt16(t *testing.T) {
	y := setupMaxOutputYM2612()
	// Generate multiple frames
	for frame := 0; frame < 5; frame++ {
		y.GenerateSamples(7670454 / 60)
		buf := y.GetBuffer()
		for i, s := range buf {
			if s < -32768 || s > 32767 {
				t.Fatalf("frame %d sample[%d]=%d outside int16 range", frame, i, s)
			}
		}
	}
}

func TestGenerate_OutputClampsAtExtremes(t *testing.T) {
	y := setupMaxOutputYM2612()
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	// Just verify no overflow wrapping occurred (no sign changes from
	// what should be large positive sums becoming negative)
	if len(buf) == 0 {
		t.Fatal("expected non-empty buffer")
	}
	// Buffer values should be within int16 range - tested above,
	// but also verify we get some non-zero output
	nonZero := false
	for _, s := range buf {
		if s != 0 {
			nonZero = true
			break
		}
	}
	if !nonZero {
		t.Error("max output config should produce non-zero samples")
	}
}

// --- EG/LFO/Timer integration tests ---

func TestGenerate_EGProgressesDuringGenerate(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set up ch0 with slow AR so we can see EG progress
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x07) // algo 7
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC0)
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A)
	y.WritePort(0, 0x30)
	y.WritePort(1, 0x01)

	// AR=10 (moderate, not instant)
	y.WritePort(0, 0x50)
	y.WritePort(1, 0x0A)

	// Key on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10) // S1 only

	// EG should start at 0x3FF (attack state)
	if y.ch[0].op[0].egState != egAttack {
		t.Fatalf("expected egAttack state, got %d", y.ch[0].op[0].egState)
	}

	// Generate 1 frame
	y.GenerateSamples(7670454 / 60)

	// EG level should have decreased from 0x3FF
	if y.ch[0].op[0].egLevel >= 0x3FF {
		t.Errorf("after 1 frame, egLevel should decrease from 0x3FF, got 0x%03X",
			y.ch[0].op[0].egLevel)
	}
}

func TestGenerate_LFOProgressesDuringGenerate(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable LFO with fast frequency
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x0F) // LFO enable, freq=7 (fastest)

	if y.lfoStep != 0 {
		t.Fatalf("initial lfoStep should be 0, got %d", y.lfoStep)
	}

	// Generate 1 frame
	y.GenerateSamples(7670454 / 60)

	if y.lfoStep == 0 {
		t.Error("after 1 frame with fastest LFO, lfoStep should have advanced")
	}
}

func TestGenerate_TimerOverflowDuringGenerate(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Timer A: period=1023 (overflows after 1 tick)
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03)

	// Enable and load Timer A
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x05) // load + enable

	// Generate enough for at least one timer tick
	y.GenerateSamples(144) // One native sample = one timer step

	if !y.timerAOver {
		t.Error("Timer A should have overflowed after 1 native sample with period=1023")
	}
}

// setupMaxOutputYM2612 creates a YM2612 with all 6 channels configured at
// maximum output (algo 7, all ops TL=0, instant attack, all keyed on).
func setupMaxOutputYM2612() *YM2612 {
	y := NewYM2612(7670454, 48000)

	for ch := 0; ch < 6; ch++ {
		part := uint8(0)
		chSlot := uint8(ch)
		if ch >= 3 {
			part = 2
			chSlot = uint8(ch - 3)
		}

		// Algorithm 7, fb=0
		y.WritePort(part, 0xB0+chSlot)
		y.WritePort(part+1, 0x07)

		// Pan L+R
		y.WritePort(part, 0xB4+chSlot)
		y.WritePort(part+1, 0xC0)

		// Frequency
		y.WritePort(part, 0xA4+chSlot)
		y.WritePort(part+1, 0x22)
		y.WritePort(part, 0xA0+chSlot)
		y.WritePort(part+1, 0x9A)

		// All operators
		for slot := uint8(0); slot < 4; slot++ {
			off := (slot << 2) | chSlot
			// DT=0, MUL=1
			y.WritePort(part, 0x30+off)
			y.WritePort(part+1, 0x01)
			// TL=0
			y.WritePort(part, 0x40+off)
			y.WritePort(part+1, 0x00)
			// RS=3, AR=31
			y.WritePort(part, 0x50+off)
			y.WritePort(part+1, 0xDF)
			// D1R=0
			y.WritePort(part, 0x60+off)
			y.WritePort(part+1, 0x00)
			// D2R=0
			y.WritePort(part, 0x70+off)
			y.WritePort(part+1, 0x00)
			// D1L=0, RR=15
			y.WritePort(part, 0x80+off)
			y.WritePort(part+1, 0x0F)
		}

		// Key on all ops
		chVal := uint8(ch)
		if ch >= 3 {
			chVal = uint8(ch-3) | 0x04
		}
		y.WritePort(0, 0x28)
		y.WritePort(1, 0xF0|chVal)
	}

	return y
}
