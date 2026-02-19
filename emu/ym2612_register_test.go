package emu

import "testing"

// --- Invalid address range tests ---

func TestRegister_WriteBelowAddr20Ignored(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Snapshot relevant state
	dacBefore := y.dacSample
	lfoBefore := y.lfoEnable

	// Write to addresses below $20 (invalid range)
	for _, addr := range []uint8{0x00, 0x10, 0x1F} {
		y.WritePort(0, addr)
		y.WritePort(1, 0xFF)
	}

	if y.dacSample != dacBefore {
		t.Error("write below $20 should not change dacSample")
	}
	if y.lfoEnable != lfoBefore {
		t.Error("write below $20 should not change lfoEnable")
	}
}

func TestRegister_UnhandledGlobalRegsIgnored(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Snapshot state
	dacEn := y.dacEnable
	dacSamp := y.dacSample
	lfoEn := y.lfoEnable
	ch3sp := y.ch3Mode

	// These global registers are not handled by the switch ($20, $21, $23, $29, $2C-$2F)
	unhandled := []uint8{0x20, 0x21, 0x23, 0x29, 0x2C, 0x2D, 0x2E, 0x2F}
	for _, addr := range unhandled {
		y.WritePort(0, addr)
		y.WritePort(1, 0xFF)
	}

	if y.dacEnable != dacEn {
		t.Error("unhandled global reg changed dacEnable")
	}
	if y.dacSample != dacSamp {
		t.Error("unhandled global reg changed dacSample")
	}
	if y.lfoEnable != lfoEn {
		t.Error("unhandled global reg changed lfoEnable")
	}
	if y.ch3Mode != ch3sp {
		t.Error("unhandled global reg changed ch3Mode")
	}
}

func TestRegister_ChannelRegInvalidSlot(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Channel registers with addr&3==3 are invalid
	// Save state of all channels
	var fNums [6]uint16
	for i := range y.ch {
		fNums[i] = y.ch[i].fNum
	}

	// Write to invalid slot addresses ($A3, $A7, $AB, $AF, $B3, $B7)
	invalidAddrs := []uint8{0xA3, 0xA7, 0xAB, 0xAF, 0xB3, 0xB7}
	for _, addr := range invalidAddrs {
		y.WritePort(0, addr)
		y.WritePort(1, 0xFF)
	}

	// Verify no channel state changed
	for i := range y.ch {
		if y.ch[i].fNum != fNums[i] {
			t.Errorf("invalid slot addr changed ch%d fNum: got 0x%03X, was 0x%03X",
				i, y.ch[i].fNum, fNums[i])
		}
	}
}

// --- Frequency register latching tests ---

func TestRegister_FreqMSBLatchNoPhaseUpdate(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set MUL=1 for ch0 op0
	y.WritePort(0, 0x30)
	y.WritePort(1, 0x01)

	oldInc := y.ch[0].op[0].phaseInc

	// Write MSB only ($A4) - should latch but NOT trigger phase update
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22) // block=4, fNum_hi=2

	if y.ch[0].op[0].phaseInc != oldInc {
		t.Errorf("MSB-only write should not update phaseInc: old=%d, new=%d",
			oldInc, y.ch[0].op[0].phaseInc)
	}
}

func TestRegister_FreqLSBTriggersPhaseUpdate(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set MUL=1 for ch0 op0
	y.WritePort(0, 0x30)
	y.WritePort(1, 0x01)

	oldInc := y.ch[0].op[0].phaseInc

	// Write MSB then LSB
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A)

	if y.ch[0].op[0].phaseInc == oldInc {
		t.Error("MSB+LSB write should update phaseInc")
	}
}

func TestRegister_FreqDoubleMSBBeforeLSB(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set MUL=1 for ch0 op0
	y.WritePort(0, 0x30)
	y.WritePort(1, 0x01)

	// First MSB: block=2, fNum_hi=1
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x11)

	// Second MSB: block=5, fNum_hi=4
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x2C)

	// LSB
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x00)

	// Second MSB value should be used
	if y.ch[0].block != 5 {
		t.Errorf("double MSB: block should be 5 (second write), got %d", y.ch[0].block)
	}
	if y.ch[0].fNum&0x700 != 0x400 {
		t.Errorf("double MSB: fNum hi should be 4, got 0x%03X", y.ch[0].fNum)
	}
}

func TestRegister_FreqPartIILatching(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set MUL=1 for ch3 op0 (Part II ch0)
	y.WritePort(2, 0x30)
	y.WritePort(3, 0x01)

	// Write freq via Part II: ch3 (chSlot=0 in Part II)
	y.WritePort(2, 0xA4)
	y.WritePort(3, 0x22) // block=4, fNum_hi=2

	// Verify MSB latched but no phase update yet
	oldInc := y.ch[3].op[0].phaseInc

	y.WritePort(2, 0xA0)
	y.WritePort(3, 0x9A) // LSB triggers update

	if y.ch[3].fNum != 0x29A {
		t.Errorf("Part II freq: fNum got 0x%03X, want 0x29A", y.ch[3].fNum)
	}
	if y.ch[3].block != 4 {
		t.Errorf("Part II freq: block got %d, want 4", y.ch[3].block)
	}
	if y.ch[3].op[0].phaseInc == oldInc {
		t.Error("Part II LSB should trigger phase update")
	}
}

// --- operatorOrder tests ---

func TestRegister_OperatorOrderDirect(t *testing.T) {
	expected := [4]int{0, 2, 1, 3}
	if operatorOrder != expected {
		t.Errorf("operatorOrder: got %v, want %v", operatorOrder, expected)
	}
}

func TestRegister_OperatorOrderRoundTrip(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Write unique DT/MUL values to all 4 slots for ch0
	// Slot 0 ($30): DT=1, MUL=1 -> val=0x11
	// Slot 1 ($34): DT=2, MUL=3 -> val=0x23
	// Slot 2 ($38): DT=3, MUL=5 -> val=0x35
	// Slot 3 ($3C): DT=4, MUL=7 -> val=0x47
	slotVals := []struct {
		reg uint8
		dt  uint8
		mul uint8
	}{
		{0x30, 1, 1},
		{0x34, 2, 3},
		{0x38, 3, 5},
		{0x3C, 4, 7},
	}
	for _, sv := range slotVals {
		y.WritePort(0, sv.reg)
		y.WritePort(1, (sv.dt<<4)|sv.mul)
	}

	// Verify via operatorOrder mapping
	for slot, sv := range slotVals {
		opIdx := operatorOrder[slot]
		op := &y.ch[0].op[opIdx]
		if op.dt != sv.dt {
			t.Errorf("slot %d (reg $%02X) -> op[%d]: dt got %d, want %d",
				slot, sv.reg, opIdx, op.dt, sv.dt)
		}
		if op.mul != sv.mul {
			t.Errorf("slot %d (reg $%02X) -> op[%d]: mul got %d, want %d",
				slot, sv.reg, opIdx, op.mul, sv.mul)
		}
	}
}

// --- Register side effect tests ---

func TestRegister_DTWriteTriggersPhaseUpdate(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set frequency first
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A)

	oldInc := y.ch[0].op[0].phaseInc

	// Write DT/MUL (register $30)
	y.WritePort(0, 0x30)
	y.WritePort(1, 0x12) // DT=1, MUL=2

	if y.ch[0].op[0].phaseInc == oldInc {
		t.Error("DT/MUL write should trigger phaseInc update")
	}
}

func TestRegister_TLWriteNoPhaseUpdate(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set frequency and DT/MUL first
	y.WritePort(0, 0x30)
	y.WritePort(1, 0x01)
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A)

	oldInc := y.ch[0].op[0].phaseInc

	// Write TL (register $40)
	y.WritePort(0, 0x40)
	y.WritePort(1, 0x20) // TL=32

	if y.ch[0].op[0].phaseInc != oldInc {
		t.Errorf("TL write should not change phaseInc: old=%d, new=%d",
			oldInc, y.ch[0].op[0].phaseInc)
	}
}

func TestRegister_AMFlagSetClearViaRegister(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Register $60 = AM/D1R for slot 0, ch0
	// Set AM=1 (bit 7) with D1R=5
	y.WritePort(0, 0x60)
	y.WritePort(1, 0x85) // AM=1, D1R=5

	if !y.ch[0].op[0].am {
		t.Error("AM should be true after writing 0x85 to $60")
	}
	if y.ch[0].op[0].d1r != 5 {
		t.Errorf("D1R: got %d, want 5", y.ch[0].op[0].d1r)
	}

	// Clear AM
	y.WritePort(0, 0x60)
	y.WritePort(1, 0x05) // AM=0, D1R=5

	if y.ch[0].op[0].am {
		t.Error("AM should be false after writing 0x05 to $60")
	}
}

func TestRegister_D2RExtractionFrom70(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Write $70 with val=0x1F -> D2R=31
	y.WritePort(0, 0x70)
	y.WritePort(1, 0x1F)

	if y.ch[0].op[0].d2r != 31 {
		t.Errorf("D2R: got %d, want 31", y.ch[0].op[0].d2r)
	}

	// Write val=0x00 -> D2R=0
	y.WritePort(0, 0x70)
	y.WritePort(1, 0x00)

	if y.ch[0].op[0].d2r != 0 {
		t.Errorf("D2R: got %d, want 0", y.ch[0].op[0].d2r)
	}
}

func TestRegister_NewYM2612NativeClockCalc(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	want := 7670454 / 144
	if y.nativeClock != want {
		t.Errorf("nativeClock: got %d, want %d", y.nativeClock, want)
	}
}

func TestRegister_NewYM2612DACInitial(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	if y.dacSample != 0x80 {
		t.Errorf("initial dacSample: got 0x%02X, want 0x80", y.dacSample)
	}
}
