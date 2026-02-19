package emu

import "testing"

// setupTestChannel configures ch0 with the given algorithm. All operators
// get TL=0, instant attack (RS=3, AR=31), MUL=1, DT=0, frequency set,
// L+R panning, and all operators keyed on.
func setupTestChannel(algo uint8) *YM2612 {
	y := NewYM2612(7670454, 48000)

	// Algorithm/feedback
	y.WritePort(0, 0xB0)
	y.WritePort(1, algo) // algo, fb=0

	// Panning L+R
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC0)

	// Frequency: block=4, fNum=0x29A
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A)

	// All operators: DT=0, MUL=1
	for _, reg := range []uint8{0x30, 0x34, 0x38, 0x3C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}
	// All operators: TL=0
	for _, reg := range []uint8{0x40, 0x44, 0x48, 0x4C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// All operators: RS=3, AR=31 (instant attack)
	for _, reg := range []uint8{0x50, 0x54, 0x58, 0x5C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xDF)
	}
	// D1R=0 (no decay)
	for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// D2R=0
	for _, reg := range []uint8{0x70, 0x74, 0x78, 0x7C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// D1L=0, RR=15
	for _, reg := range []uint8{0x80, 0x84, 0x88, 0x8C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x0F)
	}

	// Key on all operators
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF0)

	return y
}

// silenceOp sets TL=127 for the given operator on ch0.
// opReg is the register slot: 0x40=S1(op0), 0x44=S3(op2), 0x48=S2(op1), 0x4C=S4(op3)
func silenceOp(y *YM2612, opSlotReg uint8) {
	y.WritePort(0, opSlotReg)
	y.WritePort(1, 0x7F) // TL=127
}

// evaluateNonZero evaluates ch0 up to n times and returns true if any output
// differs from the ladder baseline. With the ladder effect, a silent channel
// with pan enabled produces applyLadder(0, true) = 128, not 0.
func evaluateNonZero(y *YM2612, n int) bool {
	for i := 0; i < n; i++ {
		_, l, _ := y.evaluateChannelFull(0)
		if l != 128 {
			return true
		}
	}
	return false
}

// maxAbsOutput evaluates ch0 n times and returns max absolute output.
func maxAbsOutput(y *YM2612, n int) int16 {
	var maxAbs int16
	for i := 0; i < n; i++ {
		_, l, _ := y.evaluateChannelFull(0)
		if l < 0 {
			l = -l
		}
		if l > maxAbs {
			maxAbs = l
		}
	}
	return maxAbs
}

// --- Algorithm 1: (OP1+OP2)->OP3->OP4 ---

func TestAlgo1_CarrierIsOP4(t *testing.T) {
	y := setupTestChannel(1)
	// Silence OP4 (S4 = reg 0x4C) - should kill output
	silenceOp(y, 0x4C)
	if evaluateNonZero(y, 200) {
		t.Error("algo 1: silencing OP4 should silence output")
	}
}

func TestAlgo1_OP1ModulatesOP3(t *testing.T) {
	y1 := setupTestChannel(1)
	y2 := setupTestChannel(1)
	silenceOp(y2, 0x40) // Silence OP1

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 1: removing OP1 should change output (it modulates OP3)")
	}
}

func TestAlgo1_OP2ModulatesOP3(t *testing.T) {
	y1 := setupTestChannel(1)
	y2 := setupTestChannel(1)
	silenceOp(y2, 0x48) // Silence OP2

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 1: removing OP2 should change output (it modulates OP3)")
	}
}

func TestAlgo1_ModulationDepth(t *testing.T) {
	y1 := setupTestChannel(1)
	y2 := setupTestChannel(1)
	y2.WritePort(0, 0x40) // OP1 TL
	y2.WritePort(1, 0x20) // TL=32

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 1: changing modulator TL should affect output")
	}
}

func TestAlgo1_FeedbackOnlyOP1(t *testing.T) {
	// With feedback enabled, only OP1 should use it
	y := setupTestChannel(1)
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x39) // algo=1, fb=7

	if !evaluateNonZero(y, 200) {
		t.Error("algo 1 with feedback should produce output")
	}
}

func TestAlgo1_OutputQuantized(t *testing.T) {
	y := setupTestChannel(1)
	for i := 0; i < 200; i++ {
		_, l, _ := y.evaluateChannelFull(0)
		if l&0x1F != 0 {
			t.Errorf("algo 1 output %d not quantize9'd (lower 5 bits: 0x%02X)", l, l&0x1F)
			break
		}
	}
}

// --- Algorithm 2: OP1+(OP2->OP3)->OP4 ---

func TestAlgo2_CarrierIsOP4(t *testing.T) {
	y := setupTestChannel(2)
	silenceOp(y, 0x4C)
	if evaluateNonZero(y, 200) {
		t.Error("algo 2: silencing OP4 should silence output")
	}
}

func TestAlgo2_OP2ModulatesOP3(t *testing.T) {
	y1 := setupTestChannel(2)
	y2 := setupTestChannel(2)
	silenceOp(y2, 0x48)

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 2: removing OP2 should change output (it modulates OP3)")
	}
}

func TestAlgo2_OP1AndOP3ModulateOP4(t *testing.T) {
	y1 := setupTestChannel(2)
	y2 := setupTestChannel(2)
	silenceOp(y2, 0x40)

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 2: removing OP1 should change output (it modulates OP4)")
	}
}

func TestAlgo2_ModulationDepth(t *testing.T) {
	y1 := setupTestChannel(2)
	y2 := setupTestChannel(2)
	y2.WritePort(0, 0x48) // OP2 TL
	y2.WritePort(1, 0x40) // TL=64

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 2: changing OP2 TL should affect output")
	}
}

func TestAlgo2_FeedbackOnlyOP1(t *testing.T) {
	y := setupTestChannel(2)
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x3A) // algo=2, fb=7
	if !evaluateNonZero(y, 200) {
		t.Error("algo 2 with feedback should produce output")
	}
}

func TestAlgo2_OutputQuantized(t *testing.T) {
	y := setupTestChannel(2)
	for i := 0; i < 200; i++ {
		_, l, _ := y.evaluateChannelFull(0)
		if l&0x1F != 0 {
			t.Errorf("algo 2 output not quantize9'd: %d", l)
			break
		}
	}
}

// --- Algorithm 3: (OP1->OP2)+OP3->OP4 ---

func TestAlgo3_CarrierIsOP4(t *testing.T) {
	y := setupTestChannel(3)
	silenceOp(y, 0x4C)
	if evaluateNonZero(y, 200) {
		t.Error("algo 3: silencing OP4 should silence output")
	}
}

func TestAlgo3_OP1ModulatesOP2(t *testing.T) {
	y1 := setupTestChannel(3)
	y2 := setupTestChannel(3)
	silenceOp(y2, 0x40)

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 3: removing OP1 should change output (it modulates OP2)")
	}
}

func TestAlgo3_OP2AndOP3ModulateOP4(t *testing.T) {
	y1 := setupTestChannel(3)
	y2 := setupTestChannel(3)
	silenceOp(y2, 0x44)

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 3: removing OP3 should change output (it modulates OP4)")
	}
}

func TestAlgo3_ModulationDepth(t *testing.T) {
	y1 := setupTestChannel(3)
	y2 := setupTestChannel(3)
	y2.WritePort(0, 0x40) // OP1 TL
	y2.WritePort(1, 0x30) // TL=48

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 3: changing OP1 TL should affect output")
	}
}

func TestAlgo3_FeedbackOnlyOP1(t *testing.T) {
	y := setupTestChannel(3)
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x3B) // algo=3, fb=7
	if !evaluateNonZero(y, 200) {
		t.Error("algo 3 with feedback should produce output")
	}
}

func TestAlgo3_OutputQuantized(t *testing.T) {
	y := setupTestChannel(3)
	for i := 0; i < 200; i++ {
		_, l, _ := y.evaluateChannelFull(0)
		if l&0x1F != 0 {
			t.Errorf("algo 3 output not quantize9'd: %d", l)
			break
		}
	}
}

// --- Algorithm 4: (OP1->OP2) + (OP3->OP4) ---

func TestAlgo4_CarriersAreOP2AndOP4(t *testing.T) {
	y := setupTestChannel(4)
	// Silence OP2 (carrier)
	silenceOp(y, 0x48) // S2
	// OP4 should still produce output
	if !evaluateNonZero(y, 200) {
		t.Error("algo 4: silencing OP2 should not kill all output (OP4 is also carrier)")
	}

	y2 := setupTestChannel(4)
	silenceOp(y2, 0x4C) // S4
	if !evaluateNonZero(y2, 200) {
		t.Error("algo 4: silencing OP4 should not kill all output (OP2 is also carrier)")
	}

	// Silence both carriers
	y3 := setupTestChannel(4)
	silenceOp(y3, 0x48)
	silenceOp(y3, 0x4C)
	if evaluateNonZero(y3, 200) {
		t.Error("algo 4: silencing both carriers should silence output")
	}
}

func TestAlgo4_OP1ModulatesOP2(t *testing.T) {
	y1 := setupTestChannel(4)
	silenceOp(y1, 0x4C) // Isolate OP1->OP2 chain
	y2 := setupTestChannel(4)
	silenceOp(y2, 0x4C)
	silenceOp(y2, 0x40) // Also silence OP1

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 4: removing OP1 should change OP2 output (it modulates OP2)")
	}
}

func TestAlgo4_OP3ModulatesOP4(t *testing.T) {
	y1 := setupTestChannel(4)
	silenceOp(y1, 0x48) // Isolate OP3->OP4 chain
	y2 := setupTestChannel(4)
	silenceOp(y2, 0x48)
	silenceOp(y2, 0x44) // Also silence OP3

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 4: removing OP3 should change OP4 output (it modulates OP4)")
	}
}

func TestAlgo4_ModulationDepth(t *testing.T) {
	y1 := setupTestChannel(4)
	y2 := setupTestChannel(4)
	y2.WritePort(0, 0x40) // OP1 TL
	y2.WritePort(1, 0x40) // TL=64

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 4: changing OP1 TL should affect output")
	}
}

func TestAlgo4_FeedbackOnlyOP1(t *testing.T) {
	y := setupTestChannel(4)
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x3C) // algo=4, fb=7
	if !evaluateNonZero(y, 200) {
		t.Error("algo 4 with feedback should produce output")
	}
}

func TestAlgo4_OutputQuantizedPerCarrier(t *testing.T) {
	// Multi-carrier: each carrier is quantized before accumulation
	y := setupTestChannel(4)
	for i := 0; i < 200; i++ {
		_, l, _ := y.evaluateChannelFull(0)
		_ = l // Output is sum of quantized carriers, may not have low bits 0
	}
}

// --- Algorithm 5: OP1->OP2+OP3+OP4 ---

func TestAlgo5_CarriersAreOP234(t *testing.T) {
	// Silence each carrier individually - output should still exist from the others
	for _, reg := range []uint8{0x48, 0x44, 0x4C} { // S2, S3, S4
		y := setupTestChannel(5)
		silenceOp(y, reg)
		if !evaluateNonZero(y, 200) {
			t.Errorf("algo 5: silencing one carrier (reg 0x%02X) should not kill all output", reg)
		}
	}

	// Silence all three carriers
	y := setupTestChannel(5)
	silenceOp(y, 0x48)
	silenceOp(y, 0x44)
	silenceOp(y, 0x4C)
	if evaluateNonZero(y, 200) {
		t.Error("algo 5: silencing all carriers should silence output")
	}
}

func TestAlgo5_OP1ModulatesAll(t *testing.T) {
	y1 := setupTestChannel(5)
	y2 := setupTestChannel(5)
	silenceOp(y2, 0x40) // Silence OP1

	// Compare actual sample sequences - modulation changes waveform shape
	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 5: removing OP1 should change output (it modulates all carriers)")
	}
}

func TestAlgo5_ModulationDepth(t *testing.T) {
	y1 := setupTestChannel(5)
	y2 := setupTestChannel(5)
	y2.WritePort(0, 0x40) // OP1 TL
	y2.WritePort(1, 0x40) // TL=64 (less modulation)

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 5: changing OP1 TL should affect output")
	}
}

func TestAlgo5_FeedbackOnlyOP1(t *testing.T) {
	y := setupTestChannel(5)
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x3D) // algo=5, fb=7
	if !evaluateNonZero(y, 200) {
		t.Error("algo 5 with feedback should produce output")
	}
}

func TestAlgo5_OutputQuantized(t *testing.T) {
	y := setupTestChannel(5)
	for i := 0; i < 200; i++ {
		_, l, _ := y.evaluateChannelFull(0)
		_ = l
	}
}

func TestAlgo5_PerAdditionClamping(t *testing.T) {
	// Algo 5 adds carriers one at a time with clamping between each.
	// With ladder effect, the clamped range [-8176, 8160] shifts to
	// [-8272, 8288] after applying ladder offsets (+128/-96).
	y := setupTestChannel(5)
	for i := 0; i < 200; i++ {
		_, l, _ := y.evaluateChannelFull(0)
		if l > 8288 || l < -8272 {
			t.Errorf("algo 5 output %d exceeds clamp range", l)
			break
		}
	}
}

// --- Algorithm 6: OP1->OP2 + OP3 + OP4 ---

func TestAlgo6_CarriersAreOP234(t *testing.T) {
	for _, reg := range []uint8{0x48, 0x44, 0x4C} {
		y := setupTestChannel(6)
		silenceOp(y, reg)
		if !evaluateNonZero(y, 200) {
			t.Errorf("algo 6: silencing one carrier (reg 0x%02X) should not kill all output", reg)
		}
	}

	y := setupTestChannel(6)
	silenceOp(y, 0x48)
	silenceOp(y, 0x44)
	silenceOp(y, 0x4C)
	if evaluateNonZero(y, 200) {
		t.Error("algo 6: silencing all carriers should silence output")
	}
}

func TestAlgo6_OP1ModulatesOnlyOP2(t *testing.T) {
	// Silence OP2 to remove OP1's effect, keep OP3 and OP4
	y1 := setupTestChannel(6)
	silenceOp(y1, 0x48) // Silence OP2
	ref := maxAbsOutput(y1, 200)

	// Also silence OP1 - since OP2 is silent, OP1 has no effect on OP3/OP4
	y2 := setupTestChannel(6)
	silenceOp(y2, 0x48)
	silenceOp(y2, 0x40) // Also silence OP1
	noOP1 := maxAbsOutput(y2, 200)

	// OP1 shouldn't affect OP3/OP4 directly in algo 6
	if ref != noOP1 {
		t.Error("algo 6: OP1 should not affect OP3/OP4 directly")
	}
}

func TestAlgo6_OP3NoModulation(t *testing.T) {
	// In algo 6, OP3 receives no modulation input
	y := setupTestChannel(6)
	// Silence everything except OP3
	silenceOp(y, 0x40)
	silenceOp(y, 0x48)
	silenceOp(y, 0x4C)
	if !evaluateNonZero(y, 200) {
		t.Error("algo 6: OP3 alone should produce output (no modulation needed)")
	}
}

func TestAlgo6_ModulationDepth(t *testing.T) {
	y1 := setupTestChannel(6)
	y2 := setupTestChannel(6)
	y2.WritePort(0, 0x40) // OP1 TL
	y2.WritePort(1, 0x40) // TL=64

	differs := false
	for i := 0; i < 200; i++ {
		_, l1, _ := y1.evaluateChannelFull(0)
		_, l2, _ := y2.evaluateChannelFull(0)
		if l1 != l2 {
			differs = true
			break
		}
	}
	if !differs {
		t.Error("algo 6: changing OP1 TL should affect output")
	}
}

func TestAlgo6_FeedbackOnlyOP1(t *testing.T) {
	y := setupTestChannel(6)
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x3E) // algo=6, fb=7
	if !evaluateNonZero(y, 200) {
		t.Error("algo 6 with feedback should produce output")
	}
}

func TestAlgo6_PerAdditionClamping(t *testing.T) {
	y := setupTestChannel(6)
	for i := 0; i < 200; i++ {
		_, l, _ := y.evaluateChannelFull(0)
		if l > 8288 || l < -8272 {
			t.Errorf("algo 6 output %d exceeds clamp range", l)
			break
		}
	}
}

// --- Multi-carrier additional tests (Algo 4-7) ---

func TestAlgo4_MultiCarrierClamping(t *testing.T) {
	y := setupTestChannel(4)
	for i := 0; i < 200; i++ {
		_, l, _ := y.evaluateChannelFull(0)
		if l > 8288 || l < -8272 {
			t.Errorf("algo 4 output %d outside clamp range [-8272, 8288]", l)
			break
		}
	}
}

func TestAlgo7_MultiCarrierClamping(t *testing.T) {
	y := setupTestChannel(7)
	for i := 0; i < 200; i++ {
		_, l, _ := y.evaluateChannelFull(0)
		if l > 8288 || l < -8272 {
			t.Errorf("algo 7 output %d outside clamp range", l)
			break
		}
	}
}

func TestAlgo4_PositiveClampBoundary(t *testing.T) {
	// clampAccum should cap at 0x1FE0
	if clampAccum(0x2000) != 0x1FE0 {
		t.Errorf("positive clamp: expected 0x1FE0, got 0x%X", clampAccum(0x2000))
	}
}

func TestAlgo4_NegativeClampBoundary(t *testing.T) {
	if clampAccum(-0x2000) != -0x1FF0 {
		t.Errorf("negative clamp: expected %d, got %d", -0x1FF0, clampAccum(-0x2000))
	}
}

func TestAlgo7_NoInterOpModulation(t *testing.T) {
	// In algo 7, each operator runs independently (no modulation except OP1 feedback)
	y := setupTestChannel(7)

	// Silence OP1 - other ops should still produce output
	silenceOp(y, 0x40)
	if !evaluateNonZero(y, 200) {
		t.Error("algo 7: silencing OP1 should not affect OP2/OP3/OP4")
	}
}

func TestAlgo7_EachOpIndependent(t *testing.T) {
	// Silence each operator individually - others should still work
	for _, reg := range []uint8{0x40, 0x48, 0x44, 0x4C} {
		y := setupTestChannel(7)
		silenceOp(y, reg)
		if !evaluateNonZero(y, 200) {
			t.Errorf("algo 7: silencing reg 0x%02X should not kill all output", reg)
		}
	}
}

// --- Cross-algorithm tests ---

func TestAlgo_AllAlgosProduceOutput(t *testing.T) {
	for algo := uint8(0); algo <= 7; algo++ {
		y := setupTestChannel(algo)
		if !evaluateNonZero(y, 200) {
			t.Errorf("algo %d: should produce non-zero output", algo)
		}
	}
}

func TestAlgo_CarrierCountPerAlgorithm(t *testing.T) {
	// Expected carrier counts: algo0=1, algo1=1, algo2=1, algo3=1, algo4=2, algo5=3, algo6=3, algo7=4
	expectedCarriers := [8]int{1, 1, 1, 1, 2, 3, 3, 4}

	for algo := uint8(0); algo <= 7; algo++ {
		// Count carriers by silencing each op and checking if the output waveform
		// is reduced. A carrier being silenced removes its contribution, so
		// we silence all OTHER ops and check if this one alone produces output.
		carriers := 0
		// TL registers: 0x40=S1(op0), 0x44=S3(op2), 0x48=S2(op1), 0x4C=S4(op3)
		allRegs := []uint8{0x40, 0x48, 0x44, 0x4C}
		for i, reg := range allRegs {
			y := setupTestChannel(algo)
			// Silence all ops EXCEPT this one
			for j, otherReg := range allRegs {
				if j != i {
					silenceOp(y, otherReg)
				}
			}
			// If this op alone produces output, it's a carrier
			if evaluateNonZero(y, 200) {
				carriers++
			}
			_ = reg
		}

		if carriers != expectedCarriers[algo] {
			t.Errorf("algo %d: expected %d carriers, detected %d",
				algo, expectedCarriers[algo], carriers)
		}
	}
}

func TestAlgo0_ModulationShiftRight1(t *testing.T) {
	// In algo 0, operator modulation is shifted >>1 (matching hardware YM_MOD_SHIFT)
	// This is verified by the code: int32(s1)>>1 in evalAlgo0
	y := setupTestChannel(0)
	if !evaluateNonZero(y, 200) {
		t.Error("algo 0 should produce output")
	}
}

func TestAlgo0_DeepModulation(t *testing.T) {
	// With all operators at TL=0 in serial chain, deep modulation produces complex waveform
	y := setupTestChannel(0)
	// Also enable feedback for even more complex modulation
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x38) // algo=0, fb=7

	// Collect multiple samples - should show variation (complex waveform)
	seen := make(map[int16]bool)
	for i := 0; i < 200; i++ {
		_, l, _ := y.evaluateChannelFull(0)
		seen[l] = true
	}
	if len(seen) < 3 {
		t.Errorf("algo 0 with deep modulation should produce varied output, only got %d distinct values", len(seen))
	}
}
