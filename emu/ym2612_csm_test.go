package emu

import "testing"

// --- CSM Mode Register Tests ---

func TestCSM_ModeRegisterValues(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Normal mode: bits 7-6 = 0b00
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x00)
	if y.ch3Mode != ch3ModeNormal {
		t.Errorf("0x00: expected ch3ModeNormal (%d), got %d", ch3ModeNormal, y.ch3Mode)
	}

	// Special mode: bits 7-6 = 0b01
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)
	if y.ch3Mode != ch3ModeSpecial {
		t.Errorf("0x40: expected ch3ModeSpecial (%d), got %d", ch3ModeSpecial, y.ch3Mode)
	}

	// CSM mode: bits 7-6 = 0b10
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x80)
	if y.ch3Mode != ch3ModeCSM {
		t.Errorf("0x80: expected ch3ModeCSM (%d), got %d", ch3ModeCSM, y.ch3Mode)
	}

	// Bits 7-6 = 0b11 maps to mode value 3 (treated as >0, per-op freqs)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0xC0)
	if y.ch3Mode != 3 {
		t.Errorf("0xC0: expected mode 3, got %d", y.ch3Mode)
	}
}

func TestCSM_PerOperatorFrequencies(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set CSM mode
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x80)

	// Set MUL=1 for ch2 op2 (slot 0 maps to op2/OP3): reg $36
	y.WritePort(0, 0x36)
	y.WritePort(1, 0x01)

	// Write ch3 slot 0 per-operator frequency
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x22) // block=4, fNum_hi=2
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x50) // fNum_lo=0x50

	// Verify the per-op frequency was used
	kc := computeKeyCode(0x250, 4)
	expectedInc := computePhaseIncrement(0x250, 4, kc, 0, 1)
	if y.ch[2].op[2].phaseInc != expectedInc {
		t.Errorf("CSM per-op freq: phaseInc got %d, want %d",
			y.ch[2].op[2].phaseInc, expectedInc)
	}
}

// setupCSMTimerA configures CSM mode with Timer A loaded at the given period.
// Returns the YM2612 instance.
func setupCSMTimerA(period uint16) *YM2612 {
	y := NewYM2612(7670454, 48000)

	// Set up ch3 ops with fast attack (AR=31) so CSM key-on is observable
	for i := 0; i < 4; i++ {
		opSlot := [4]uint8{0x32, 0x36, 0x3A, 0x3E}[i] // ch2 op slots
		// AR=31, RS=0
		y.WritePort(0, opSlot-0x32+0x52) // $5x register for RS/AR
		y.WritePort(1, 0x1F)             // AR=31
	}

	// Set Timer A period
	y.WritePort(0, 0x24)
	y.WritePort(1, uint8(period>>2))
	y.WritePort(0, 0x25)
	y.WritePort(1, uint8(period&0x03))

	// Enable CSM mode + load Timer A (bit 0)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x80|0x01) // CSM mode + timer A load

	return y
}

// stepNative advances the YM2612 by one native sample clock (144 M68K cycles).
func stepNative(y *YM2612) {
	y.GenerateSamples(144)
}

// stepsUntilTimerAOverflow steps the timer until Timer A overflows.
// Returns the number of steps taken. Stops after maxSteps to prevent infinite loop.
func stepsUntilTimerAOverflow(y *YM2612, maxSteps int) int {
	for i := 0; i < maxSteps; i++ {
		prevCounter := y.timerA.counter
		stepNative(y)
		// Overflow detected when counter resets to 0 from a non-zero value
		if y.timerA.counter == 0 && prevCounter != 0 {
			return i + 1
		}
		// Also detect overflow on first tick if period=1023
		if y.timerA.counter == 0 && y.csmKeyOn {
			return i + 1
		}
	}
	return maxSteps
}

func TestCSM_KeyOnAtTimerAOverflow(t *testing.T) {
	// Use period=1023 so overflow happens on every tick
	y := setupCSMTimerA(1023)

	// All ch3 ops should start in release state
	for i := 0; i < 4; i++ {
		if y.ch[2].op[i].egState != egRelease {
			t.Fatalf("op%d: expected egRelease before overflow, got %d", i, y.ch[2].op[i].egState)
		}
	}

	// Step once - Timer A should overflow (period=1023: overflows at count >= 1)
	stepNative(y)

	// All ch3 ops should be in attack (or decay if instant attack)
	for i := 0; i < 4; i++ {
		op := &y.ch[2].op[i]
		if op.egState != egAttack && op.egState != egDecay {
			t.Errorf("op%d: expected egAttack or egDecay after CSM key-on, got %d", i, op.egState)
		}
	}

	// Phase counters should have been reset (to 0, then advanced by one step)
	// Since we just did one native sample, phaseCounter should be small
	// (equal to one step of phaseInc)
	if !y.csmKeyOn {
		t.Error("csmKeyOn should be true after overflow")
	}
}

func TestCSM_KeyOffOnNextTick(t *testing.T) {
	// Use period=1022 so we have time between overflows
	y := setupCSMTimerA(1022)

	// Step until first overflow
	stepsUntilTimerAOverflow(y, 10)

	if !y.csmKeyOn {
		t.Fatal("csmKeyOn should be true after first overflow")
	}

	// All ops should be in attack/decay (CSM key-on fired)
	for i := 0; i < 4; i++ {
		op := &y.ch[2].op[i]
		if op.egState != egAttack && op.egState != egDecay {
			t.Fatalf("op%d: expected attack/decay after CSM key-on, got %d", i, op.egState)
		}
	}

	// Step one more tick - CSM key-off should fire
	stepNative(y)

	if y.csmKeyOn {
		t.Error("csmKeyOn should be false after key-off tick")
	}

	// Ops not held by $28 should be in release
	for i := 0; i < 4; i++ {
		op := &y.ch[2].op[i]
		if op.keyOn {
			t.Fatalf("op%d: keyOn should be false (no $28 key-on)", i)
		}
		if op.egState != egRelease {
			t.Errorf("op%d: expected egRelease after CSM key-off, got %d", i, op.egState)
		}
	}
}

func TestCSM_KeyOffRespectsReg28KeyOn(t *testing.T) {
	y := setupCSMTimerA(1022)

	// Key-on op0 and op1 of ch2 via register $28
	// ch2 = channel value 2, op bits: bit4=S1(op0), bit5=S2(op1)
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x32) // ch2, S1+S2 on (bits 4,5)

	// Verify $28 key-on set
	if !y.ch[2].op[0].keyOn || !y.ch[2].op[1].keyOn {
		t.Fatal("op0,op1 should have keyOn from $28")
	}

	// Step until Timer A overflow
	stepsUntilTimerAOverflow(y, 10)

	// All 4 ops should be in attack/decay (CSM key-on)
	for i := 0; i < 4; i++ {
		op := &y.ch[2].op[i]
		if op.egState != egAttack && op.egState != egDecay {
			t.Fatalf("op%d: expected attack/decay after CSM key-on, got %d", i, op.egState)
		}
	}

	// Step one more tick - CSM key-off fires
	stepNative(y)

	// op0 and op1 are held by $28 key-on, should NOT enter release
	if y.ch[2].op[0].egState == egRelease {
		t.Error("op0: held by $28, should not be in release")
	}
	if y.ch[2].op[1].egState == egRelease {
		t.Error("op1: held by $28, should not be in release")
	}

	// op2 and op3 are NOT held by $28, should enter release
	if y.ch[2].op[2].egState != egRelease {
		t.Errorf("op2: not held by $28, expected release, got %d", y.ch[2].op[2].egState)
	}
	if y.ch[2].op[3].egState != egRelease {
		t.Errorf("op3: not held by $28, expected release, got %d", y.ch[2].op[3].egState)
	}
}

func TestCSM_NoFireInSpecialMode(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set Timer A period=1023 (overflows every tick)
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF) // period MSB = 1023>>2 = 255
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03) // period LSB = 3

	// Special mode (0x40) + timer A load + timer A enable
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40|0x05) // special mode + load + enable

	// Step past overflow
	stepNative(y)

	// csmKeyOn should NOT be set
	if y.csmKeyOn {
		t.Error("csmKeyOn should not fire in special mode")
	}

	// Ch3 ops should still be in release
	for i := 0; i < 4; i++ {
		if y.ch[2].op[i].egState != egRelease {
			t.Errorf("op%d: should remain in release, got %d", i, y.ch[2].op[i].egState)
		}
	}
}

func TestCSM_NoFireInNormalMode(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set Timer A period=1023
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03)

	// Normal mode (0x00) + timer A load + timer A enable
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x05) // normal mode + load + enable

	stepNative(y)

	if y.csmKeyOn {
		t.Error("csmKeyOn should not fire in normal mode")
	}

	for i := 0; i < 4; i++ {
		if y.ch[2].op[i].egState != egRelease {
			t.Errorf("op%d: should remain in release, got %d", i, y.ch[2].op[i].egState)
		}
	}
}

func TestCSM_FiresWithoutTimerAEnable(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set Timer A period=1023
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03)

	// CSM mode + timer A load, but timerAEnable=false (bit 2 not set)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x80|0x01) // CSM + load only

	// Set AR=31 for observable key-on
	for i := 0; i < 4; i++ {
		arReg := [4]uint8{0x52, 0x56, 0x5A, 0x5E}[i]
		y.WritePort(0, arReg)
		y.WritePort(1, 0x1F)
	}

	stepNative(y)

	// CSM should still fire even without timerAEnable
	if !y.csmKeyOn {
		t.Error("CSM should fire even when timerAEnable is false")
	}

	// But the overflow flag should NOT be set (timerAEnable gates the flag)
	if y.timerAOver {
		t.Error("timerAOver should not be set when timerAEnable is false")
	}
}

func TestCSM_RapidRetriggering(t *testing.T) {
	// With period=1023, timer overflows every tick, so CSM fires every tick
	y := setupCSMTimerA(1023)

	// Step once: first overflow + key-on
	stepNative(y)
	if !y.csmKeyOn {
		t.Fatal("csmKeyOn should be true after first tick")
	}

	// Step again: key-off fires, then counter increments and overflows again,
	// so key-on fires again
	stepNative(y)
	if !y.csmKeyOn {
		t.Error("csmKeyOn should be true again after rapid retrigger")
	}

	// All ops should be in attack/decay (retriggered)
	for i := 0; i < 4; i++ {
		op := &y.ch[2].op[i]
		if op.egState != egAttack && op.egState != egDecay {
			t.Errorf("op%d: expected attack/decay after retrigger, got %d", i, op.egState)
		}
	}
}

func TestCSM_OnlyAffectsChannel3(t *testing.T) {
	y := setupCSMTimerA(1023)

	// Set AR=31 for ch0 and ch4 operators too
	for _, chBase := range []uint8{0x50, 0x51} {
		for opSlot := uint8(0); opSlot < 4; opSlot++ {
			y.WritePort(0, chBase+opSlot*4)
			y.WritePort(1, 0x1F)
		}
	}
	// Part II ch3 (index 3)
	for opSlot := uint8(0); opSlot < 4; opSlot++ {
		y.WritePort(2, 0x50+opSlot*4)
		y.WritePort(1, 0x1F)
	}

	// Step past overflow
	stepNative(y)

	// Ch2 (ch3 in 1-indexed) should have CSM key-on
	for i := 0; i < 4; i++ {
		op := &y.ch[2].op[i]
		if op.egState != egAttack && op.egState != egDecay {
			t.Errorf("ch2 op%d: expected attack/decay, got %d", i, op.egState)
		}
	}

	// Other channels should NOT be affected
	otherChannels := []int{0, 1, 3, 4, 5}
	for _, ch := range otherChannels {
		for i := 0; i < 4; i++ {
			if y.ch[ch].op[i].egState != egRelease {
				t.Errorf("ch%d op%d: should still be in release, got %d",
					ch, i, y.ch[ch].op[i].egState)
			}
		}
	}
}

func TestCSM_ModeChangeAllowsPendingKeyOff(t *testing.T) {
	y := setupCSMTimerA(1023)

	// Step once: CSM fires key-on
	stepNative(y)
	if !y.csmKeyOn {
		t.Fatal("csmKeyOn should be true")
	}

	// Switch to special mode (away from CSM)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40|0x01) // special mode + timer A load

	// Step: key-off should still fire for the pending csmKeyOn
	stepNative(y)

	if y.csmKeyOn {
		t.Error("csmKeyOn should be cleared after key-off")
	}

	// Ops should be in release (key-off fired)
	for i := 0; i < 4; i++ {
		op := &y.ch[2].op[i]
		if op.egState != egRelease {
			t.Errorf("op%d: expected release after mode change + key-off, got %d", i, op.egState)
		}
	}
}
