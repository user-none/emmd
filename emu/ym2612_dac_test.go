package emu

import "testing"

// --- DAC value tests ---

func TestDAC_Value0x00(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80) // DAC enable
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x00) // DAC sample

	mono, _, _ := y.evaluateChannelFull(5)
	expected := int16((0x00 - 128) << 6) // -128 << 6 = -8192
	if mono != expected {
		t.Errorf("DAC 0x00: expected %d, got %d", expected, mono)
	}
}

func TestDAC_Value0x01(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x01)

	mono, _, _ := y.evaluateChannelFull(5)
	expected := int16((0x01 - 128) << 6)
	if mono != expected {
		t.Errorf("DAC 0x01: expected %d, got %d", expected, mono)
	}
}

func TestDAC_Value0x40(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x40)

	mono, _, _ := y.evaluateChannelFull(5)
	expected := int16((0x40 - 128) << 6) // -64 << 6 = -4096
	if mono != expected {
		t.Errorf("DAC 0x40: expected %d, got %d", expected, mono)
	}
}

func TestDAC_Value0x80Center(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x80)

	mono, _, _ := y.evaluateChannelFull(5)
	if mono != 0 {
		t.Errorf("DAC 0x80 (center): expected 0, got %d", mono)
	}
}

func TestDAC_Value0xC0(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xC0)

	mono, _, _ := y.evaluateChannelFull(5)
	expected := int16((0xC0 - 128) << 6) // 64 << 6 = 4096
	if mono != expected {
		t.Errorf("DAC 0xC0: expected %d, got %d", expected, mono)
	}
}

func TestDAC_Value0xFF(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xFF)

	mono, _, _ := y.evaluateChannelFull(5)
	expected := int16((0xFF - 128) << 6) // 127 << 6 = 8128
	if mono != expected {
		t.Errorf("DAC 0xFF: expected %d, got %d", expected, mono)
	}
}

// --- DAC monotonic ---

func TestDAC_Monotonic(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80) // DAC enable

	var prev int16 = -32768
	for val := 0; val <= 255; val++ {
		y.WritePort(0, 0x2A)
		y.WritePort(1, uint8(val))
		mono, _, _ := y.evaluateChannelFull(5)
		if mono < prev {
			t.Errorf("DAC not monotonic at 0x%02X: prev=%d, cur=%d", val, prev, mono)
			break
		}
		prev = mono
	}
}

// --- DAC initial sample ---

func TestDAC_InitialSample0x80(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	if y.dacSample != 0x80 {
		t.Errorf("initial DAC sample: expected 0x80, got 0x%02X", y.dacSample)
	}
	// Enable DAC without writing a sample - should output 0 (center)
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	mono, _, _ := y.evaluateChannelFull(5)
	if mono != 0 {
		t.Errorf("initial DAC output: expected 0 (center), got %d", mono)
	}
}

// --- DAC panning ---

func TestDAC_PanLeftOnly(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80) // DAC enable
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xC0) // Non-center value

	// Set ch5 panning: L only
	y.WritePort(2, 0xB6)
	y.WritePort(3, 0x80) // L=1, R=0

	_, l, r := y.evaluateChannelFull(5)
	// With ladder effect, enabled pan gets signal+128, disabled pan gets +128
	// DAC 0xC0: dacOut=4096, so l=4224, r=128 (muted positive offset)
	if l == 128 {
		t.Error("DAC L-only: left should carry signal, not just ladder offset")
	}
	if r != 128 {
		t.Errorf("DAC L-only: right should be muted ladder offset 128, got %d", r)
	}
}

func TestDAC_PanRightOnly(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xC0)

	y.WritePort(2, 0xB6)
	y.WritePort(3, 0x40) // L=0, R=1

	_, l, r := y.evaluateChannelFull(5)
	if l != 128 {
		t.Errorf("DAC R-only: left should be muted ladder offset 128, got %d", l)
	}
	if r == 128 {
		t.Error("DAC R-only: right should carry signal, not just ladder offset")
	}
}

func TestDAC_PanBoth(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xC0)

	y.WritePort(2, 0xB6)
	y.WritePort(3, 0xC0) // L=1, R=1

	_, l, r := y.evaluateChannelFull(5)
	if l == 0 {
		t.Error("DAC L+R: left should be non-zero")
	}
	if r == 0 {
		t.Error("DAC L+R: right should be non-zero")
	}
	if l != r {
		t.Errorf("DAC L+R: left and right should be equal: L=%d, R=%d", l, r)
	}
}

func TestDAC_PanNeither(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xC0)

	y.WritePort(2, 0xB6)
	y.WritePort(3, 0x00) // L=0, R=0

	_, l, r := y.evaluateChannelFull(5)
	// With ladder effect, muted pan with positive sample produces +128
	if l != 128 {
		t.Errorf("DAC no pan: left should be muted ladder offset 128, got %d", l)
	}
	if r != 128 {
		t.Errorf("DAC no pan: right should be muted ladder offset 128, got %d", r)
	}
}

// --- FM panning tests ---

func TestFM_PanLeftOnly(t *testing.T) {
	y := setupTestChannel(7)
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0x80) // L only
	for i := 0; i < 100; i++ {
		_, l, r := y.evaluateChannelFull(0)
		// With ladder, muted pan produces +128 (sample >= 0) or -128 (sample < 0)
		lHasSignal := l != 128 && l != -128
		rIsMuted := r == 128 || r == -128
		if lHasSignal && !rIsMuted {
			t.Errorf("FM L-only: right should be muted ladder offset, got %d", r)
			break
		}
	}
}

func TestFM_PanRightOnly(t *testing.T) {
	y := setupTestChannel(7)
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0x40) // R only
	for i := 0; i < 100; i++ {
		_, l, r := y.evaluateChannelFull(0)
		rHasSignal := r != 128 && r != -128
		lIsMuted := l == 128 || l == -128
		if rHasSignal && !lIsMuted {
			t.Errorf("FM R-only: left should be muted ladder offset, got %d", l)
			break
		}
	}
}

func TestFM_PanBoth(t *testing.T) {
	y := setupTestChannel(7)
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC0) // L+R
	for i := 0; i < 100; i++ {
		_, l, r := y.evaluateChannelFull(0)
		if l != r {
			t.Errorf("FM L+R: L and R should be equal: L=%d, R=%d", l, r)
			break
		}
	}
}

func TestFM_PanNeither(t *testing.T) {
	y := setupTestChannel(7)
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0x00) // Neither
	for i := 0; i < 100; i++ {
		_, l, r := y.evaluateChannelFull(0)
		// With ladder, muted pan produces +128 or -128
		lIsMuted := l == 128 || l == -128
		rIsMuted := r == 128 || r == -128
		if !lIsMuted || !rIsMuted {
			t.Errorf("FM no pan: expected muted ladder offset, got L=%d R=%d", l, r)
			break
		}
	}
}

func TestFM_PanPartIIChannel(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set up ch3 (Part II, index 3) with algo 7, frequency, key on
	y.WritePort(2, 0xB0) // Part II ch3 algo
	y.WritePort(3, 0x07) // algo=7
	y.WritePort(2, 0xB4) // Part II ch3 panning
	y.WritePort(3, 0x80) // L only

	y.WritePort(2, 0xA4)
	y.WritePort(3, 0x22) // block=4
	y.WritePort(2, 0xA0)
	y.WritePort(3, 0x9A) // fNum

	// Set op TL=0, instant attack for ch3
	for _, reg := range []uint8{0x30, 0x34, 0x38, 0x3C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x01)
	}
	for _, reg := range []uint8{0x40, 0x44, 0x48, 0x4C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x00)
	}
	for _, reg := range []uint8{0x50, 0x54, 0x58, 0x5C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0xDF)
	}

	// Key on ch3
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF4) // all ops on, ch3 (Part II, bit2=1, ch=0)

	foundSignal := false
	for i := 0; i < 200; i++ {
		_, l, r := y.evaluateChannelFull(3)
		// With ladder, muted offset is +128 or -128. Actual signal differs.
		if l != 128 && l != -128 {
			foundSignal = true
			rIsMuted := r == 128 || r == -128
			if !rIsMuted {
				t.Errorf("Part II ch3 L-only: right should be muted ladder offset, got %d", r)
			}
			break
		}
	}
	if !foundSignal {
		t.Error("Part II ch3 should produce output")
	}
}

func TestFM_DefaultPanLR(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	for ch := 0; ch < 6; ch++ {
		if !y.ch[ch].panL || !y.ch[ch].panR {
			t.Errorf("ch%d: default panning should be L+R", ch)
		}
	}
}

// --- DAC edge cases ---

func TestDAC_DuringKeyOn(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set up ch5 with FM tone
	y.WritePort(2, 0xB2) // Part II ch5 (chSlot=2)
	y.WritePort(3, 0x07) // algo=7
	y.WritePort(2, 0xA6) // Part II ch5 freq MSB
	y.WritePort(3, 0x22)
	y.WritePort(2, 0xA2) // Part II ch5 freq LSB
	y.WritePort(3, 0x9A)
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x01)
	}
	for _, reg := range []uint8{0x42, 0x46, 0x4A, 0x4E} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x00)
	}
	for _, reg := range []uint8{0x52, 0x56, 0x5A, 0x5E} {
		y.WritePort(2, reg)
		y.WritePort(3, 0xDF)
	}
	y.WritePort(2, 0xB6)
	y.WritePort(3, 0xC0) // L+R

	// Key on ch5
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF6) // all ops on, ch5 (bit2=1, ch=2)

	// Enable DAC - should override FM
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xC0) // Non-center DAC

	mono, _, _ := y.evaluateChannelFull(5)
	expected := int16((0xC0 - 128) << 6)
	if mono != expected {
		t.Errorf("DAC during key-on: expected %d, got %d", expected, mono)
	}
}

func TestDAC_DisableResumesFM(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set up ch5 FM
	y.WritePort(2, 0xB2)
	y.WritePort(3, 0x07)
	y.WritePort(2, 0xA6)
	y.WritePort(3, 0x22)
	y.WritePort(2, 0xA2)
	y.WritePort(3, 0x9A)
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x01)
	}
	for _, reg := range []uint8{0x42, 0x46, 0x4A, 0x4E} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x00)
	}
	for _, reg := range []uint8{0x52, 0x56, 0x5A, 0x5E} {
		y.WritePort(2, reg)
		y.WritePort(3, 0xDF)
	}
	y.WritePort(2, 0xB6)
	y.WritePort(3, 0xC0)

	// Key on ch5
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF6)

	// Enable DAC
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x80) // Center

	// Disable DAC
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x00)

	// Should now use FM output
	foundNonZero := false
	for i := 0; i < 200; i++ {
		_, l, _ := y.evaluateChannelFull(5)
		if l != 0 {
			foundNonZero = true
			break
		}
	}
	if !foundNonZero {
		t.Error("disabling DAC should resume FM output")
	}
}

func TestDAC_ChangeWhileActive(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80) // DAC enable

	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xC0)
	mono1, _, _ := y.evaluateChannelFull(5)

	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x40) // Change sample
	mono2, _, _ := y.evaluateChannelFull(5)

	if mono1 == mono2 {
		t.Error("changing DAC sample should change output")
	}
}

func TestDAC_WriteBeforeEnable(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Write DAC sample before enabling
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xFF)

	// Verify sample is stored
	if y.dacSample != 0xFF {
		t.Errorf("DAC sample should be stored even when disabled: got 0x%02X", y.dacSample)
	}

	// Enable DAC - should use the previously written sample
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)

	mono, _, _ := y.evaluateChannelFull(5)
	expected := int16((0xFF - 128) << 6)
	if mono != expected {
		t.Errorf("DAC after enable: expected %d, got %d", expected, mono)
	}
}
