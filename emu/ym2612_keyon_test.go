package emu

import "testing"

// --- Rapid toggle tests ---

func TestKeyOn_RapidToggle(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set instant attack
	y.WritePort(0, 0x50)
	y.WritePort(1, 0xDF)

	// Key on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10) // S1 on
	if !y.ch[0].op[0].keyOn {
		t.Fatal("should be key-on")
	}

	// Immediately key off
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00)
	if y.ch[0].op[0].keyOn {
		t.Error("should be key-off")
	}
	if y.ch[0].op[0].egState != egRelease {
		t.Errorf("should be in release, got %d", y.ch[0].op[0].egState)
	}
}

func TestKeyOn_ToggleBack(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.WritePort(0, 0x50)
	y.WritePort(1, 0xDF)

	// Key on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10)

	// Key off
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00)

	// Key on again
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10)

	if !y.ch[0].op[0].keyOn {
		t.Error("should be key-on after toggle back")
	}
	if y.ch[0].op[0].phaseCounter != 0 {
		t.Errorf("phase should reset on re-key, got 0x%05X", y.ch[0].op[0].phaseCounter)
	}
}

// --- Selective operator tests ---

func TestKeyOn_SelectiveOP1Only(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10) // S1 only

	if !y.ch[0].op[0].keyOn {
		t.Error("op0 should be on")
	}
	for i := 1; i < 4; i++ {
		if y.ch[0].op[i].keyOn {
			t.Errorf("op%d should be off", i)
		}
	}
}

func TestKeyOn_SelectiveOP2Only(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x20) // S2 only

	if !y.ch[0].op[1].keyOn {
		t.Error("op1 should be on")
	}
	if y.ch[0].op[0].keyOn {
		t.Error("op0 should be off")
	}
}

func TestKeyOn_SelectiveOP3Only(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x40) // S3 only

	if !y.ch[0].op[2].keyOn {
		t.Error("op2 should be on")
	}
}

func TestKeyOn_SelectiveOP4Only(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x80) // S4 only

	if !y.ch[0].op[3].keyOn {
		t.Error("op3 should be on")
	}
}

// --- Re-key during different states ---

func TestKeyOn_ReKeyDuringAttack(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set slow attack
	y.WritePort(0, 0x50)
	y.WritePort(1, 0x0A) // RS=0, AR=10

	// Key on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10)

	op := &y.ch[0].op[0]
	if op.egState != egAttack {
		t.Fatalf("should be in attack, got %d", op.egState)
	}

	// Re-key: off then on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00)
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10)

	if !op.keyOn {
		t.Error("should be key-on")
	}
	if op.phaseCounter != 0 {
		t.Error("phase should reset")
	}
}

func TestKeyOn_ReKeyDuringDecay(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Instant attack, then decay
	y.WritePort(0, 0x50)
	y.WritePort(1, 0xDF) // instant attack
	y.WritePort(0, 0x60)
	y.WritePort(1, 0x1F) // D1R=31
	y.WritePort(0, 0x80)
	y.WritePort(1, 0xEF) // D1L=14

	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10)

	op := &y.ch[0].op[0]
	if op.egState != egDecay {
		t.Fatalf("should be in decay, got %d", op.egState)
	}

	// Step a bit into decay
	for i := 0; i < 100; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	// Re-key
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00)
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10)

	if op.egState != egDecay && op.egState != egAttack {
		t.Errorf("re-key should restart, got state %d", op.egState)
	}
}

func TestKeyOn_ReKeyDuringSustain(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	op := &y.ch[0].op[0]
	op.egState = egSustain
	op.egLevel = 0x100
	op.keyOn = true

	// Key off then on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00)

	if op.egState != egRelease {
		t.Fatalf("should be in release after key-off, got %d", op.egState)
	}

	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10)

	if !op.keyOn {
		t.Error("should be key-on")
	}
	if op.egState != egAttack && op.egState != egDecay {
		t.Errorf("should restart in attack or decay, got %d", op.egState)
	}
}

// --- Key-off preserves EG level ---

func TestKeyOn_KeyOffPreservesLevel(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.WritePort(0, 0x50)
	y.WritePort(1, 0xDF) // instant attack

	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10) // key on

	op := &y.ch[0].op[0]
	// Set a specific level
	op.egLevel = 0x100

	// Key off
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00)

	// Level should be preserved at key-off
	if op.egLevel != 0x100 {
		t.Errorf("key-off should preserve egLevel: expected 0x100, got 0x%03X", op.egLevel)
	}
}

// --- Channel encoding tests ---

func TestKeyOn_Channel3Encoding(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Channel 3 = Part II, channel index 3. Encoding: bit2=1, low bits=0 -> val=0x04
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF4) // All ops on, ch3 (bit2=1, ch=0)

	for i := 0; i < 4; i++ {
		if !y.ch[3].op[i].keyOn {
			t.Errorf("ch3 op%d should be key-on", i)
		}
	}
	// Other channels should be unaffected
	for i := 0; i < 4; i++ {
		if y.ch[0].op[i].keyOn {
			t.Errorf("ch0 op%d should not be key-on", i)
		}
	}
}

func TestKeyOn_Channel6Encoding(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Channel 5 = Part II, channel index 5. Encoding: bit2=1, low bits=2 -> val=0x06
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF6) // All ops on, ch5

	for i := 0; i < 4; i++ {
		if !y.ch[5].op[i].keyOn {
			t.Errorf("ch5 op%d should be key-on", i)
		}
	}
}

// --- Invalid channel tests ---

func TestKeyOn_InvalidChannel3Ignored(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Channel slot 3 (val & 3 == 3) is invalid
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF3) // Invalid: low bits = 3

	// No channel should be affected
	for ch := 0; ch < 6; ch++ {
		for op := 0; op < 4; op++ {
			if y.ch[ch].op[op].keyOn {
				t.Errorf("ch%d op%d should not be key-on (invalid channel)", ch, op)
			}
		}
	}
}

func TestKeyOn_InvalidChannel7Ignored(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Channel slot 7 (val & 3 == 3, bit2=1) is invalid
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF7) // Invalid: bit2=1, low bits = 3

	for ch := 0; ch < 6; ch++ {
		for op := 0; op < 4; op++ {
			if y.ch[ch].op[op].keyOn {
				t.Errorf("ch%d op%d should not be key-on (invalid channel 7)", ch, op)
			}
		}
	}
}

// --- Already on/off ---

func TestKeyOn_AlreadyOnNoEffect(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.WritePort(0, 0x50)
	y.WritePort(1, 0xDF) // instant attack

	// Key on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10)

	op := &y.ch[0].op[0]
	op.egLevel = 0x50 // Some decay progress
	op.egState = egDecay

	// Key on again while already on - should NOT reset
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10)

	if op.egLevel != 0x50 {
		t.Errorf("key-on while already on should not reset: level was 0x50, got 0x%03X", op.egLevel)
	}
}

func TestKeyOn_AlreadyOffNoEffect(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	op := &y.ch[0].op[0]
	// Already in release with keyOn=false
	op.egState = egRelease
	op.egLevel = 0x200
	op.keyOn = false

	// Key off again
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00)

	// Should not change anything
	if op.egState != egRelease {
		t.Errorf("key-off while already off should keep release, got %d", op.egState)
	}
	if op.egLevel != 0x200 {
		t.Errorf("level should not change, got 0x%03X", op.egLevel)
	}
}
