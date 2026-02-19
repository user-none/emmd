package emu

import "testing"

// --- Detune Table tests ---

func TestPhaseGen_DetuneColumn0AllZeros(t *testing.T) {
	for kc := 0; kc < 32; kc++ {
		if detuneTable[kc][0] != 0 {
			t.Errorf("detuneTable[%d][0]: expected 0, got %d", kc, detuneTable[kc][0])
		}
	}
}

func TestPhaseGen_DetuneSpotCheckKC0(t *testing.T) {
	expected := [4]uint32{0, 0, 1, 2}
	for col := 0; col < 4; col++ {
		if detuneTable[0][col] != expected[col] {
			t.Errorf("detuneTable[0][%d]: got %d, want %d", col, detuneTable[0][col], expected[col])
		}
	}
}

func TestPhaseGen_DetuneSpotCheckKC15(t *testing.T) {
	expected := [4]uint32{0, 2, 5, 7}
	for col := 0; col < 4; col++ {
		if detuneTable[15][col] != expected[col] {
			t.Errorf("detuneTable[15][%d]: got %d, want %d", col, detuneTable[15][col], expected[col])
		}
	}
}

func TestPhaseGen_DetuneSpotCheckKC31(t *testing.T) {
	expected := [4]uint32{0, 8, 16, 22}
	for col := 0; col < 4; col++ {
		if detuneTable[31][col] != expected[col] {
			t.Errorf("detuneTable[31][%d]: got %d, want %d", col, detuneTable[31][col], expected[col])
		}
	}
}

func TestPhaseGen_DetuneMonotonicity(t *testing.T) {
	// Each column should be monotonically non-decreasing with keyCode
	for col := 1; col < 4; col++ {
		for kc := 1; kc < 32; kc++ {
			if detuneTable[kc][col] < detuneTable[kc-1][col] {
				t.Errorf("detuneTable col %d not monotonic at kc=%d: %d < %d",
					col, kc, detuneTable[kc][col], detuneTable[kc-1][col])
				break
			}
		}
	}
}

func TestPhaseGen_DetuneColumnOrdering(t *testing.T) {
	// For each keyCode, column values should be non-decreasing: col0 <= col1 <= col2 <= col3
	for kc := 0; kc < 32; kc++ {
		for col := 1; col < 4; col++ {
			if detuneTable[kc][col] < detuneTable[kc][col-1] {
				t.Errorf("detuneTable[%d]: col %d (%d) < col %d (%d)",
					kc, col, detuneTable[kc][col], col-1, detuneTable[kc][col-1])
				break
			}
		}
	}
}

// --- DT mirroring tests ---

func TestPhaseGen_DT4EqualsDT0(t *testing.T) {
	// DT=4 (bit2 set, &3=0) should subtract column 0 (always 0) = same as DT=0
	fNum := uint16(0x400)
	block := uint8(4)
	kc := computeKeyCode(fNum, block)
	inc0 := computePhaseIncrement(fNum, block, kc, 0, 1) // DT=0
	inc4 := computePhaseIncrement(fNum, block, kc, 4, 1) // DT=4 (subtract col 0 = 0)
	if inc0 != inc4 {
		t.Errorf("DT=4 should equal DT=0: DT0=%d, DT4=%d", inc0, inc4)
	}
}

func TestPhaseGen_DT5MirrorsDT1(t *testing.T) {
	// DT=5 subtracts same delta as DT=1 adds
	fNum := uint16(0x400)
	block := uint8(4)
	kc := computeKeyCode(fNum, block)
	inc0 := computePhaseIncrement(fNum, block, kc, 0, 1)
	inc1 := computePhaseIncrement(fNum, block, kc, 1, 1) // Adds delta
	inc5 := computePhaseIncrement(fNum, block, kc, 5, 1) // Subtracts same delta
	delta := inc1 - inc0
	if inc0-inc5 != delta {
		t.Errorf("DT=5 should mirror DT=1: DT0=%d, DT1=%d (delta=%d), DT5=%d (delta=%d)",
			inc0, inc1, delta, inc5, inc0-inc5)
	}
}

// --- Detune underflow tests ---

func TestPhaseGen_DetuneUnderflowWraps(t *testing.T) {
	// With fNum=1, block=0: base = (1 << 0) >> 1 = 0.
	// Subtracting any non-zero detune should wrap via 17-bit mask (not clamp to 0).
	// This is intentional per ym2612_reference.md and is needed for GEMS compatibility.
	fNum := uint16(1)
	block := uint8(0)
	kc := computeKeyCode(fNum, block)

	// DT=5 (bit2=1, &3=1) subtracts detuneTable[kc][1].
	// At kc=0, detuneTable[0][1]=0 so try a keyCode with non-zero delta.
	// Use fNum=0x80, block=2 -> kc with non-zero detune column 1.
	fNum = 0x080
	block = 2
	kc = computeKeyCode(fNum, block)
	// base = (0x80 << 2) >> 1 = 0x100
	// delta = detuneTable[kc][1]
	delta := detuneTable[kc&0x1F][1]
	if delta == 0 {
		t.Skip("delta is 0 for this keyCode, cannot test underflow")
	}

	// Use fNum=1, block=0 with same kc to force base < delta.
	// We need to construct a case where base is small. Use fNum=2, block=0
	// so base = (2 << 0) >> 1 = 1. With kc from a higher block for larger delta.
	// Directly test: fNum=2, block=0, but pretend kc=12 (where delta[1]=2).
	base := (uint32(2) << 0) >> 1  // base = 1
	detDelta := detuneTable[12][1] // delta = 2
	if base >= detDelta {
		t.Fatal("test setup: need base < delta for underflow test")
	}

	// With wrapping: result = (1 - 2) & 0x1FFFF = 0x1FFFF
	inc := computePhaseIncrement(2, 0, 12, 5, 1) // DT=5 (&3=1, bit2=1 = subtract)
	if inc == 0 {
		t.Error("detune underflow should wrap to non-zero value, got 0 (clamped)")
	}
}

func TestPhaseGen_PMDetuneUnderflowWraps(t *testing.T) {
	// Same underflow behavior should apply in the PM path.
	modFnum12 := uint32(2) << 1 // small modulated fnum
	// block=0, kc=12 (detune delta=2 in column 1), DT=5, MUL=1
	inc := computePMPhaseIncrement(modFnum12, 0, 12, 5, 1)
	if inc == 0 {
		t.Error("PM detune underflow should wrap to non-zero value, got 0 (clamped)")
	}
}

// --- Block/freq boundary tests ---

func TestPhaseGen_MinFrequency(t *testing.T) {
	// fNum=1, block=0: base = (1 << 0) >> 1 = 0 (integer division)
	// Need at least fNum=2 or block=1 for non-zero with MUL=1
	kc := computeKeyCode(1, 0)
	inc := computePhaseIncrement(1, 0, kc, 0, 1)
	if inc != 0 {
		t.Errorf("fNum=1, block=0, MUL=1 should be 0 (integer truncation), got %d", inc)
	}

	// fNum=2, block=0 is the smallest non-zero
	kc2 := computeKeyCode(2, 0)
	inc2 := computePhaseIncrement(2, 0, kc2, 0, 1)
	if inc2 == 0 {
		t.Error("fNum=2, block=0, MUL=1 should produce non-zero increment")
	}
}

func TestPhaseGen_MaxFrequency(t *testing.T) {
	// fNum=0x7FF, block=7, MUL=15: largest increment
	kc := computeKeyCode(0x7FF, 7)
	inc := computePhaseIncrement(0x7FF, 7, kc, 0, 15)
	if inc == 0 {
		t.Error("max frequency should produce non-zero increment")
	}
	// Should be masked to 20 bits
	if inc > 0xFFFFF {
		t.Errorf("increment should be 20-bit, got 0x%X", inc)
	}
}

func TestPhaseGen_FNumZero(t *testing.T) {
	// fNum=0: base increment is 0
	kc := computeKeyCode(0, 4)
	inc := computePhaseIncrement(0, 4, kc, 0, 1)
	if inc != 0 {
		t.Errorf("fNum=0 should give 0 increment, got %d", inc)
	}
}

func TestPhaseGen_AllBlocksDoubling(t *testing.T) {
	// Each block should double the increment
	fNum := uint16(0x400)
	for block := uint8(0); block < 7; block++ {
		kc1 := computeKeyCode(fNum, block)
		kc2 := computeKeyCode(fNum, block+1)
		inc1 := computePhaseIncrement(fNum, block, kc1, 0, 1)
		inc2 := computePhaseIncrement(fNum, block+1, kc2, 0, 1)
		// Note: detune may differ slightly due to different keyCode,
		// so use DT=0 where detune is always 0
		if inc2 != inc1*2 {
			t.Errorf("block %d->%d: expected doubling (%d -> %d), got %d",
				block, block+1, inc1, inc1*2, inc2)
		}
	}
}

func TestPhaseGen_MUL0Half(t *testing.T) {
	fNum := uint16(0x400)
	block := uint8(4)
	kc := computeKeyCode(fNum, block)
	inc0 := computePhaseIncrement(fNum, block, kc, 0, 0)
	inc1 := computePhaseIncrement(fNum, block, kc, 0, 1)
	if inc0 != inc1/2 {
		t.Errorf("MUL=0 should be half of MUL=1: %d vs %d/2=%d", inc0, inc1, inc1/2)
	}
}

func TestPhaseGen_MUL15(t *testing.T) {
	fNum := uint16(0x200)
	block := uint8(3)
	kc := computeKeyCode(fNum, block)
	inc1 := computePhaseIncrement(fNum, block, kc, 0, 1)
	inc15 := computePhaseIncrement(fNum, block, kc, 0, 15)
	want := (inc1 * 15) & 0xFFFFF
	if inc15 != want {
		t.Errorf("MUL=15: got %d, want %d", inc15, want)
	}
}

func TestPhaseGen_20BitMask(t *testing.T) {
	// Large fNum * large MUL * large block should still be 20-bit
	kc := computeKeyCode(0x7FF, 7)
	inc := computePhaseIncrement(0x7FF, 7, kc, 0, 15)
	if inc&^uint32(0xFFFFF) != 0 {
		t.Errorf("increment should be masked to 20 bits, got 0x%X", inc)
	}
}

// --- PM Phase tests ---

func TestPhaseGen_PMEquivalenceAllBlocks(t *testing.T) {
	// Verify PM phase matches normal phase at all blocks when no PM delta
	fNum := uint16(0x29A)
	for block := uint8(0); block < 8; block++ {
		kc := computeKeyCode(fNum, block)
		for mul := uint8(0); mul <= 15; mul++ {
			normal := computePhaseIncrement(fNum, block, kc, 0, mul)
			modFnum12 := uint32(fNum) << 1
			pm := computePMPhaseIncrement(modFnum12, block, kc, 0, mul)
			if normal != pm {
				t.Errorf("block=%d mul=%d: normal=0x%05X, pm=0x%05X", block, mul, normal, pm)
			}
		}
	}
}

func TestPhaseGen_PMPositiveDelta(t *testing.T) {
	fNum := uint16(0x400)
	block := uint8(4)
	kc := computeKeyCode(fNum, block)

	normal := computePhaseIncrement(fNum, block, kc, 0, 1)
	// Add a positive delta
	modFnum12 := uint32(fNum)<<1 + 10
	pm := computePMPhaseIncrement(uint32(modFnum12)&0xFFF, block, kc, 0, 1)

	if pm <= normal {
		t.Errorf("positive PM delta should increase increment: normal=%d, pm=%d", normal, pm)
	}
}

func TestPhaseGen_PMNegativeDelta(t *testing.T) {
	fNum := uint16(0x400)
	block := uint8(4)
	kc := computeKeyCode(fNum, block)

	normal := computePhaseIncrement(fNum, block, kc, 0, 1)
	// Subtract from fNum
	modFnum12 := (uint32(fNum)<<1 - 10) & 0xFFF
	pm := computePMPhaseIncrement(modFnum12, block, kc, 0, 1)

	if pm >= normal {
		t.Errorf("negative PM delta should decrease increment: normal=%d, pm=%d", normal, pm)
	}
}

func TestPhaseGen_PMMaxModulation(t *testing.T) {
	// Max PM delta with FMS=7, fNum=0x7FF should not overflow
	fNum := uint16(0x7FF)
	block := uint8(7)
	kc := computeKeyCode(fNum, block)

	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 4 << 2 // Non-zero PM step

	delta := y.lfoPMFnumDelta(7, fNum)
	modFnum12 := uint32(int32(fNum)<<1+delta) & 0xFFF
	pm := computePMPhaseIncrement(modFnum12, block, kc, 0, 1)

	// Should be valid 20-bit value
	if pm > 0xFFFFF {
		t.Errorf("PM increment should be 20-bit, got 0x%X", pm)
	}
}

// --- Ch3 special mode tests ---

func TestPhaseGen_Ch3PerOpKeyCode(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable ch3 special mode
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)

	// Set different frequencies per slot
	// Slot 0 ($AC/$A8): block=3, fNum=0x300
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x1B)
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x00)

	// Slot 2 ($AE/$AA): block=5, fNum=0x500
	y.WritePort(0, 0xAE)
	y.WritePort(1, 0x2D)
	y.WritePort(0, 0xAA)
	y.WritePort(1, 0x00)

	// Set MUL=1 DT=0 for ops
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}

	// Op2 (OP3) -> slot 0: should have keyCode for block=3, fNum=0x300
	op2 := &y.ch[2].op[2]
	kc2Expected := computeKeyCode(0x300, 3)
	if op2.keyCode != kc2Expected {
		t.Errorf("ch3 op2 keyCode: got %d, want %d", op2.keyCode, kc2Expected)
	}

	// Op1 (OP2) -> slot 2: should have keyCode for block=5, fNum=0x500
	op1 := &y.ch[2].op[1]
	kc1Expected := computeKeyCode(0x500, 5)
	if op1.keyCode != kc1Expected {
		t.Errorf("ch3 op1 keyCode: got %d, want %d", op1.keyCode, kc1Expected)
	}
}

func TestPhaseGen_Ch3PhaseIncUpdateOnSlotWrite(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable ch3 special mode
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)

	// Set MUL=1 DT=0 for OP2 (slot 2 maps to OP2)
	y.WritePort(0, 0x3A) // ch2 op S3 = OP2
	y.WritePort(1, 0x01)

	oldInc := y.ch[2].op[1].phaseInc

	// Write slot 2 frequency (maps to op1/OP2)
	y.WritePort(0, 0xAE)
	y.WritePort(1, 0x2D) // block=5, fNum_hi=5
	y.WritePort(0, 0xAA)
	y.WritePort(1, 0x00) // fNum_lo=0

	newInc := y.ch[2].op[1].phaseInc
	if newInc == oldInc {
		t.Error("phase increment should update when slot frequency is written in ch3 special mode")
	}
}

func TestPhaseGen_Ch3EnableDisableTransitions(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set MUL=1 DT=0 for all ch2 ops
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}

	// Set channel 2 frequency
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x22) // block=4
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0x9A)

	// Normal mode: all ops should have same phaseInc
	inc0Normal := y.ch[2].op[0].phaseInc
	inc1Normal := y.ch[2].op[1].phaseInc
	if inc0Normal != inc1Normal {
		t.Errorf("normal mode: op0 and op1 should share freq: %d vs %d", inc0Normal, inc1Normal)
	}

	// Enable ch3 special mode
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)

	// Write different per-op frequencies
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x1B) // slot 0: block=3
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x00)

	y.WritePort(0, 0xAE)
	y.WritePort(1, 0x2D) // slot 2: block=5
	y.WritePort(0, 0xAA)
	y.WritePort(1, 0x00)

	// Ops should now have different phase increments
	if y.ch[2].op[0].phaseInc == y.ch[2].op[1].phaseInc {
		t.Error("ch3 special mode: ops should have different phase increments")
	}

	// Disable ch3 special mode - note: phaseInc won't automatically revert
	// until a frequency write triggers recomputation
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x00)

	// Rewrite channel frequency to trigger recomputation
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0x9A)

	// Now all ops should share channel frequency again
	if y.ch[2].op[0].phaseInc != y.ch[2].op[1].phaseInc {
		t.Errorf("after disable: ops should share freq again: %d vs %d",
			y.ch[2].op[0].phaseInc, y.ch[2].op[1].phaseInc)
	}
}

// --- computeKeyCode exhaustive tests ---

func TestPhaseGen_KeyCodeAll16FBitCombinations(t *testing.T) {
	// Test all 16 combinations of F11:F10:F9:F8 at block=0
	// keyCode = [block(3), F11, (F11&(F10|F9|F8)) | (!F11&F10&F9&F8)]
	tests := []struct {
		fNum uint16
		want uint8
	}{
		{0x000, 0x00}, // F11=0,F10=0,F9=0,F8=0
		{0x080, 0x00}, // F11=0,F10=0,F9=0,F8=1
		{0x100, 0x00}, // F11=0,F10=0,F9=1,F8=0
		{0x180, 0x00}, // F11=0,F10=0,F9=1,F8=1
		{0x200, 0x00}, // F11=0,F10=1,F9=0,F8=0
		{0x280, 0x00}, // F11=0,F10=1,F9=0,F8=1
		{0x300, 0x00}, // F11=0,F10=1,F9=1,F8=0
		{0x380, 0x01}, // F11=0,F10=1,F9=1,F8=1 (!F11&F10&F9&F8)
		{0x400, 0x02}, // F11=1,F10=0,F9=0,F8=0 (bit1=1, bit0=0)
		{0x480, 0x03}, // F11=1,F10=0,F9=0,F8=1
		{0x500, 0x03}, // F11=1,F10=0,F9=1,F8=0
		{0x580, 0x03}, // F11=1,F10=0,F9=1,F8=1
		{0x600, 0x03}, // F11=1,F10=1,F9=0,F8=0
		{0x680, 0x03}, // F11=1,F10=1,F9=0,F8=1
		{0x700, 0x03}, // F11=1,F10=1,F9=1,F8=0
		{0x780, 0x03}, // F11=1,F10=1,F9=1,F8=1
	}
	for _, tt := range tests {
		got := computeKeyCode(tt.fNum, 0)
		if got != tt.want {
			t.Errorf("computeKeyCode(0x%03X, 0): got 0x%02X, want 0x%02X",
				tt.fNum, got, tt.want)
		}
	}
}

func TestPhaseGen_KeyCodeAllBlocks(t *testing.T) {
	// fNum=0x400 (F11=1 only), block 0-7
	// keyCode = (block<<2) | 0x02
	for block := uint8(0); block < 8; block++ {
		got := computeKeyCode(0x400, block)
		want := (block << 2) | 0x02
		if got != want {
			t.Errorf("computeKeyCode(0x400, %d): got 0x%02X, want 0x%02X",
				block, got, want)
		}
	}
}

func TestPhaseGen_KeyCodeBitBoundaries(t *testing.T) {
	// Test fNums at exact bit transitions
	tests := []struct {
		fNum uint16
		want uint8
	}{
		{0x07F, 0x00}, // just below F8
		{0x080, 0x00}, // F8 set (F11=0,F10=0,F9=0,F8=1 -> bit0=0)
		{0x0FF, 0x00}, // just below F9
		{0x100, 0x00}, // F9 set
		{0x1FF, 0x00}, // just below F10
		{0x200, 0x00}, // F10 set (F11=0 -> bit1=0, only F10 -> bit0=0)
		{0x3FF, 0x01}, // F10+F9+F8 all set -> !F11&F10&F9&F8 = 1
		{0x400, 0x02}, // F11 set (bit1=1, no others -> bit0=0)
	}
	for _, tt := range tests {
		got := computeKeyCode(tt.fNum, 0)
		if got != tt.want {
			t.Errorf("computeKeyCode(0x%03X, 0): got 0x%02X, want 0x%02X",
				tt.fNum, got, tt.want)
		}
	}
}

// --- Ch3 special S4 path tests ---

func TestPhaseGen_Ch3S4UsesChannelFreq(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable ch3 special mode
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)

	// Set MUL=1 DT=0 for all ch2 ops
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}

	// Set channel 2 frequency: block=4, fNum=0x29A
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0x9A)

	// Set ch3 slot frequencies to very different values
	// Slot 0: block=1, fNum=0x100
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x09)
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x00)
	// Slot 1: block=2, fNum=0x200
	y.WritePort(0, 0xAD)
	y.WritePort(1, 0x12)
	y.WritePort(0, 0xA9)
	y.WritePort(1, 0x00)
	// Slot 2: block=3, fNum=0x300
	y.WritePort(0, 0xAE)
	y.WritePort(1, 0x1B)
	y.WritePort(0, 0xAA)
	y.WritePort(1, 0x00)

	// Op3 (S4) uses channel freq since ch3SlotMap(3) == -1
	op3 := &y.ch[2].op[3]
	chKC := computeKeyCode(0x29A, 4)
	expectedInc := computePhaseIncrement(0x29A, 4, chKC, 0, 1)
	if op3.phaseInc != expectedInc {
		t.Errorf("ch3 op3 (S4) should use channel freq: got phaseInc=%d, want %d",
			op3.phaseInc, expectedInc)
	}
}

func TestPhaseGen_Ch3S4KeyCodeFromChannel(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable ch3 special mode
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)

	// Set MUL=1 for all ops
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}

	// Set channel frequency: block=5, fNum=0x400
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x2C)
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0x00)

	// Op3 keyCode should match channel keyCode
	expectedKC := computeKeyCode(0x400, 5)
	if y.ch[2].op[3].keyCode != expectedKC {
		t.Errorf("ch3 op3 keyCode: got %d, want %d",
			y.ch[2].op[3].keyCode, expectedKC)
	}
}

func TestPhaseGen_Ch3S4DiffersFromOtherOps(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable ch3 special mode
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)

	// Set MUL=1 DT=0 for all ops
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}

	// Set channel freq: block=5, fNum=0x400
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x2C)
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0x00)

	// Set all 3 slot freqs to block=1, fNum=0x100 (very different from channel)
	for slot := 0; slot < 3; slot++ {
		y.WritePort(0, 0xAC+uint8(slot))
		y.WritePort(1, 0x09)
		y.WritePort(0, 0xA8+uint8(slot))
		y.WritePort(1, 0x00)
	}

	// Op3 should have different phaseInc from op0, op1, op2
	op3Inc := y.ch[2].op[3].phaseInc
	for i := 0; i < 3; i++ {
		opInc := y.ch[2].op[i].phaseInc
		if opInc == op3Inc {
			t.Errorf("ch3 op%d phaseInc=%d should differ from op3=%d", i, opInc, op3Inc)
		}
	}
}

func TestPhaseGen_Ch3SlotMapReturns(t *testing.T) {
	// Direct test of ch3SlotMap and ch3SlotToOp
	slotMapTests := []struct {
		opIdx int
		want  int
	}{
		{0, 1},
		{1, 2},
		{2, 0},
		{3, -1},
	}
	for _, tt := range slotMapTests {
		got := ch3SlotMap(tt.opIdx)
		if got != tt.want {
			t.Errorf("ch3SlotMap(%d): got %d, want %d", tt.opIdx, got, tt.want)
		}
	}

	slotToOpTests := []struct {
		slot int
		want int
	}{
		{0, 2},
		{1, 0},
		{2, 1},
		{3, -1},
	}
	for _, tt := range slotToOpTests {
		got := ch3SlotToOp(tt.slot)
		if got != tt.want {
			t.Errorf("ch3SlotToOp(%d): got %d, want %d", tt.slot, got, tt.want)
		}
	}
}

// --- Ch3 MSB register latching tests ---

func TestPhaseGen_Ch3MSBWriteNoPhaseUpdate(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable ch3 special mode
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)

	// Set MUL=1 for op mapped from slot 0 (op1)
	y.WritePort(0, 0x36) // ch2 op S3 slot (maps to op2 via operatorOrder)
	y.WritePort(1, 0x01)

	oldInc := y.ch[2].op[1].phaseInc

	// Write only MSB ($AC) - should NOT trigger phaseInc update
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x2D) // block=5, fNum_hi=5

	if y.ch[2].op[1].phaseInc != oldInc {
		t.Errorf("MSB-only write should not update phaseInc: old=%d, new=%d",
			oldInc, y.ch[2].op[1].phaseInc)
	}
}

func TestPhaseGen_Ch3MSBThenLSBUpdates(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable ch3 special mode
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)

	// Set MUL=1 for OP3 (slot 0 maps to OP3/opIdx 2)
	y.WritePort(0, 0x36) // ch2 slot 1 register = OP3
	y.WritePort(1, 0x01)

	oldInc := y.ch[2].op[2].phaseInc

	// Write MSB then LSB - should trigger update on LSB write
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x2D) // block=5, fNum_hi=5
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x00) // fNum_lo=0

	newInc := y.ch[2].op[2].phaseInc
	if newInc == oldInc {
		t.Error("MSB+LSB write should update phaseInc")
	}
}

func TestPhaseGen_Ch3FreqStoredWhenDisabled(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// ch3Mode is ch3ModeNormal initially. Write slot 0 freq.
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x2D) // block=5, fNum_hi=5
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x50) // fNum_lo=0x50

	// Freq should be stored even when disabled
	if y.ch3Freq[0] != 0x550 {
		t.Errorf("ch3Freq[0] should be stored: got 0x%03X, want 0x550", y.ch3Freq[0])
	}
	if y.ch3Block[0] != 5 {
		t.Errorf("ch3Block[0] should be stored: got %d, want 5", y.ch3Block[0])
	}

	// But op phaseInc should NOT have been updated via ch3 path
	// (since ch3Mode=ch3ModeNormal, the LSB write doesn't trigger ch3-specific update)
	// Set MUL=1 for OP3 on ch2 (slot 0 maps to OP3): reg slot 1, ch2 -> $36
	y.WritePort(0, 0x36)
	y.WritePort(1, 0x01)

	// Now enable ch3 special and rewrite LSB to trigger update
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x50) // Rewrite LSB

	// Now phaseInc should reflect the stored freq
	kc := computeKeyCode(0x550, 5)
	expectedInc := computePhaseIncrement(0x550, 5, kc, 0, 1)
	if y.ch[2].op[2].phaseInc != expectedInc {
		t.Errorf("after enable+rewrite: phaseInc got %d, want %d",
			y.ch[2].op[2].phaseInc, expectedInc)
	}
}

func TestPhaseGen_Ch3EnableUsesStoredFreq(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Write ch3 slot 2 freq while disabled (maps to op1/OP2)
	y.WritePort(0, 0xAE)
	y.WritePort(1, 0x1B) // block=3, fNum_hi=3
	y.WritePort(0, 0xAA)
	y.WritePort(1, 0x00) // fNum_lo=0

	// Enable ch3 special
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)

	// Write DT/MUL to trigger updatePhaseIncrement
	y.WritePort(0, 0x3A) // ch2 op S3 = OP2 (op1)
	y.WritePort(1, 0x01) // DT=0, MUL=1

	// op1 (OP2) should now use stored ch3 freq (slot 2: fNum=0x300, block=3)
	kc := computeKeyCode(0x300, 3)
	expectedInc := computePhaseIncrement(0x300, 3, kc, 0, 1)
	if y.ch[2].op[1].phaseInc != expectedInc {
		t.Errorf("op1 should use stored ch3 freq: got %d, want %d",
			y.ch[2].op[1].phaseInc, expectedInc)
	}
}

// --- Ch3 + PM integration tests ---

func TestPhaseGen_Ch3PMPerOpFreq(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable ch3 special mode + LFO
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x0F) // LFO enable, freq=7 (fastest)

	// Set FMS=7 for channel 2
	y.WritePort(0, 0xB6)
	y.WritePort(1, 0xC7) // pan L+R, FMS=7

	// Set MUL=1 for all ops
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}

	// Set different ch3 slot freqs
	// Slot 0: fNum=0x200, block=4
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x00)
	// Slot 1: fNum=0x400, block=4
	y.WritePort(0, 0xAD)
	y.WritePort(1, 0x24)
	y.WritePort(0, 0xA9)
	y.WritePort(1, 0x00)
	// Slot 2: fNum=0x600, block=4
	y.WritePort(0, 0xAE)
	y.WritePort(1, 0x26)
	y.WritePort(0, 0xAA)
	y.WritePort(1, 0x00)
	// Channel freq: fNum=0x300, block=4
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x23)
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0x00)

	// Advance LFO to a non-zero PM step
	y.lfoStep = 4 << 2 // PM step with non-zero delta

	// Save initial phase counters
	var initPhase [4]uint32
	for i := 0; i < 4; i++ {
		initPhase[i] = y.ch[2].op[i].phaseCounter
	}

	// Evaluate channel - should advance phases
	y.evaluateChannelFull(2)

	// All phases should have advanced
	for i := 0; i < 4; i++ {
		if y.ch[2].op[i].phaseCounter == initPhase[i] {
			t.Errorf("op%d phase should have advanced from %d", i, initPhase[i])
		}
	}
}

func TestPhaseGen_Ch3PMS4UsesChannelFreqForPM(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable ch3 special + LFO
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x0F)

	// FMS=7 for ch2
	y.WritePort(0, 0xB6)
	y.WritePort(1, 0xC7)

	// MUL=1 for all ops
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}

	// Channel freq: fNum=0x400, block=4
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x24)
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0x00)

	// Ch3 slot freqs: all fNum=0x200, block=4 (different from channel)
	for slot := 0; slot < 3; slot++ {
		y.WritePort(0, 0xAC+uint8(slot))
		y.WritePort(1, 0x22)
		y.WritePort(0, 0xA8+uint8(slot))
		y.WritePort(1, 0x00)
	}

	// Set LFO step for non-zero PM
	y.lfoStep = 4 << 2

	// Op3 (S4) should use channel fNum=0x400 for PM delta, not slot fNum
	pmDeltaCh := y.lfoPMFnumDelta(7, 0x400)
	pmDeltaSlot := y.lfoPMFnumDelta(7, 0x200)
	if pmDeltaCh == pmDeltaSlot {
		t.Skip("PM deltas happen to match for these freqs")
	}

	// Verify by checking that op3's phase progression differs from op0-2
	y.evaluateChannelFull(2)
	op3Phase := y.ch[2].op[3].phaseCounter
	op0Phase := y.ch[2].op[0].phaseCounter

	// They should differ because op3 uses channel freq and op0 uses slot freq
	if op3Phase == op0Phase {
		t.Error("op3 (channel freq PM) and op0 (slot freq PM) should produce different phases")
	}
}

func TestPhaseGen_Ch3PMDifferentDeltas(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 4 << 2 // Non-zero PM step

	// Different fNums should give different PM deltas
	fNums := []uint16{0x100, 0x200, 0x400}
	var deltas [3]int32
	for i, fn := range fNums {
		deltas[i] = y.lfoPMFnumDelta(7, fn)
	}

	// Higher fNums should give larger |delta| since PM is proportional
	for i := 1; i < 3; i++ {
		if abs32(deltas[i]) < abs32(deltas[i-1]) {
			t.Errorf("PM delta not proportional: fNum=0x%X delta=%d, fNum=0x%X delta=%d",
				fNums[i-1], deltas[i-1], fNums[i], deltas[i])
		}
	}
}

func TestPhaseGen_PMDisabledWhenLFOOff(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set FMS=7 but LFO disabled
	y.ch[0].fms = 7
	y.lfoEnable = false
	y.lfoStep = 4 << 2

	// MUL=1 for ch0 ops
	for i := range y.ch[0].op {
		y.ch[0].op[i].mul = 1
	}

	// Set frequency
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A)

	// Record initial phaseInc values
	var normalInc [4]uint32
	for i := range y.ch[0].op {
		normalInc[i] = y.ch[0].op[i].phaseInc
	}

	// Evaluate - should use normal phaseInc path (no PM)
	y.evaluateChannelFull(0)

	// Each op should have advanced by exactly its phaseInc
	for i := range y.ch[0].op {
		expected := normalInc[i] & 0xFFFFF
		if y.ch[0].op[i].phaseCounter != expected {
			t.Errorf("op%d: with LFO off, phase should advance by phaseInc=%d, got phaseCounter=%d",
				i, normalInc[i], y.ch[0].op[i].phaseCounter)
		}
	}
}

func TestPhaseGen_PMWithNonZeroDetune(t *testing.T) {
	// PM with DT=1 should differ from PM with DT=0
	y1 := NewYM2612(7670454, 48000)
	y2 := NewYM2612(7670454, 48000)

	for _, y := range []*YM2612{y1, y2} {
		y.lfoEnable = true
		y.lfoStep = 4 << 2
		y.ch[0].fms = 7
		y.ch[0].fNum = 0x400
		y.ch[0].block = 4
		y.ch[0].panL = true
	}

	// y1: DT=0, MUL=1
	y1.ch[0].op[0].dt = 0
	y1.ch[0].op[0].mul = 1
	y1.ch[0].op[0].keyCode = computeKeyCode(0x400, 4)

	// y2: DT=1, MUL=1
	y2.ch[0].op[0].dt = 1
	y2.ch[0].op[0].mul = 1
	y2.ch[0].op[0].keyCode = computeKeyCode(0x400, 4)

	y1.evaluateChannelFull(0)
	y2.evaluateChannelFull(0)

	// Phase progression should differ due to different detune in PM path
	if y1.ch[0].op[0].phaseCounter == y2.ch[0].op[0].phaseCounter {
		t.Error("PM with DT=0 and DT=1 should produce different phase progression")
	}
}

func TestPhaseGen_Ch3DisableRevertsToChannelFreq(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set MUL=1 for all ch2 ops
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}

	// Enable ch3 special
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)

	// Set different per-op freqs
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x09) // slot 0: block=1
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x00)
	y.WritePort(0, 0xAE)
	y.WritePort(1, 0x2D) // slot 2: block=5
	y.WritePort(0, 0xAA)
	y.WritePort(1, 0x00)

	// Set channel freq
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0x9A)

	// Verify ops have different phaseInc in special mode
	if y.ch[2].op[0].phaseInc == y.ch[2].op[1].phaseInc {
		t.Error("in ch3 special mode, ops should have different phaseInc")
	}

	// Disable ch3 special
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x00)

	// Rewrite channel freq to trigger recompute
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0x9A)

	// All ops should now share the same phaseInc
	inc0 := y.ch[2].op[0].phaseInc
	for i := 1; i < 4; i++ {
		if y.ch[2].op[i].phaseInc != inc0 {
			t.Errorf("after disable: op%d phaseInc=%d should match op0=%d",
				i, y.ch[2].op[i].phaseInc, inc0)
		}
	}
}

// --- stepPhase direct tests ---

func TestPhaseGen_StepPhaseAdvances(t *testing.T) {
	op := &ymOperator{phaseCounter: 0, phaseInc: 0x100}
	stepPhase(op)
	if op.phaseCounter != 0x100 {
		t.Errorf("stepPhase: got 0x%05X, want 0x00100", op.phaseCounter)
	}
}

func TestPhaseGen_StepPhaseWraps20Bit(t *testing.T) {
	op := &ymOperator{phaseCounter: 0xFFF00, phaseInc: 0x200}
	stepPhase(op)
	want := (uint32(0xFFF00) + 0x200) & 0xFFFFF
	if op.phaseCounter != want {
		t.Errorf("stepPhase wrap: got 0x%05X, want 0x%05X", op.phaseCounter, want)
	}
}

// abs32 returns the absolute value of an int32.
func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
