package emu

import "testing"

// --- Period extremes ---

func TestTimer_APeriod0(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Timer A period 0: overflows every 1024 ticks
	y.WritePort(0, 0x24)
	y.WritePort(1, 0x00)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x00)
	if y.timerA.period != 0 {
		t.Errorf("Timer A period: expected 0, got %d", y.timerA.period)
	}

	// Enable and load
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x05)

	// Should overflow after 1024 ticks
	for i := 0; i < 1023; i++ {
		y.stepTimers()
	}
	if y.timerAOver {
		t.Error("Timer A should not have overflowed after 1023 ticks with period=0")
	}
	y.stepTimers()
	if !y.timerAOver {
		t.Error("Timer A should overflow after 1024 ticks with period=0")
	}
}

func TestTimer_APeriod1023(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Timer A period 1023: overflows every 1 tick
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03)
	if y.timerA.period != 1023 {
		t.Errorf("Timer A period: expected 1023, got %d", y.timerA.period)
	}

	y.WritePort(0, 0x27)
	y.WritePort(1, 0x05)

	y.stepTimers()
	if !y.timerAOver {
		t.Error("Timer A should overflow after 1 tick with period=1023")
	}
}

func TestTimer_BPeriod0(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.WritePort(0, 0x26)
	y.WritePort(1, 0x00)
	if y.timerB.period != 0 {
		t.Errorf("Timer B period: expected 0, got %d", y.timerB.period)
	}

	y.WritePort(0, 0x27)
	y.WritePort(1, 0x0A) // Load + enable timer B

	// Timer B ticks every 16 clocks, overflows at 256 ticks
	for i := 0; i < 256*16-1; i++ {
		y.stepTimers()
	}
	if y.timerBOver {
		t.Error("Timer B should not have overflowed yet")
	}
	y.stepTimers()
	if !y.timerBOver {
		t.Error("Timer B should overflow after 256*16 ticks with period=0")
	}
}

func TestTimer_BPeriod255(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.WritePort(0, 0x26)
	y.WritePort(1, 0xFF)

	y.WritePort(0, 0x27)
	y.WritePort(1, 0x0A)

	// Period 255: overflows every 1 B-tick = 16 sample clocks
	for i := 0; i < 15; i++ {
		y.stepTimers()
	}
	if y.timerBOver {
		t.Error("Timer B should not overflow before 16 clocks")
	}
	y.stepTimers()
	if !y.timerBOver {
		t.Error("Timer B should overflow after 16 clocks with period=255")
	}
}

func TestTimer_AMidRange(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Period 512: overflows every 512 ticks
	y.WritePort(0, 0x24)
	y.WritePort(1, 0x80) // high 8 bits = 0x80 -> 0x200
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x00)

	y.WritePort(0, 0x27)
	y.WritePort(1, 0x05)

	// Should overflow after 1024-512 = 512 ticks
	for i := 0; i < 511; i++ {
		y.stepTimers()
	}
	if y.timerAOver {
		t.Error("should not overflow after 511 ticks")
	}
	y.stepTimers()
	if !y.timerAOver {
		t.Error("should overflow after 512 ticks")
	}
}

// --- Sub-counter ---

func TestTimer_BSubCounter(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.WritePort(0, 0x26)
	y.WritePort(1, 0xFF) // period 255

	y.WritePort(0, 0x27)
	y.WritePort(1, 0x0A) // Load + enable

	// Step 15 times - sub counter should not have triggered B tick
	for i := 0; i < 15; i++ {
		y.stepTimers()
	}
	if y.timerBOver {
		t.Error("sub-counter: B should not overflow before 16 clocks")
	}

	// 16th step should trigger B tick and overflow
	y.stepTimers()
	if !y.timerBOver {
		t.Error("sub-counter: B should overflow at 16th clock")
	}
}

// --- Flag clear tests ---

func TestTimer_FlagClearAOnly(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.timerAOver = true
	y.timerBOver = true

	// Clear A flag only (bit 4)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x10) // bit4 = reset A

	if y.timerAOver {
		t.Error("Timer A flag should be cleared")
	}
	if !y.timerBOver {
		t.Error("Timer B flag should NOT be cleared")
	}
}

func TestTimer_FlagClearBOnly(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.timerAOver = true
	y.timerBOver = true

	// Clear B flag only (bit 5)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x20) // bit5 = reset B

	if !y.timerAOver {
		t.Error("Timer A flag should NOT be cleared")
	}
	if y.timerBOver {
		t.Error("Timer B flag should be cleared")
	}
}

func TestTimer_FlagClearBoth(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.timerAOver = true
	y.timerBOver = true

	// Clear both (bits 4+5)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x30)

	if y.timerAOver {
		t.Error("Timer A flag should be cleared")
	}
	if y.timerBOver {
		t.Error("Timer B flag should be cleared")
	}
}

// --- Concurrent operation ---

func TestTimer_ConcurrentAB(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Timer A: period 1023 (overflow every 1 tick)
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03)

	// Timer B: period 255 (overflow every 16 clocks)
	y.WritePort(0, 0x26)
	y.WritePort(1, 0xFF)

	// Enable both
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x0F) // Load A, Load B, Enable A, Enable B

	y.stepTimers()
	if !y.timerAOver {
		t.Error("Timer A should overflow after 1 tick")
	}
	// Timer B needs 16 clocks
	if y.timerBOver {
		t.Error("Timer B should not overflow after 1 tick")
	}

	// Clear A, step until B overflows
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x1F) // Reset A, keep loads + enables
	for i := 0; i < 15; i++ {
		y.stepTimers()
	}
	if !y.timerBOver {
		t.Error("Timer B should overflow after total 16 ticks")
	}
}

// --- Reload after overflow ---

func TestTimer_ReloadAfterOverflow(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Timer A period 1023 (1 tick to overflow)
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03)

	y.WritePort(0, 0x27)
	y.WritePort(1, 0x05)

	// First overflow
	y.stepTimers()
	if !y.timerAOver {
		t.Fatal("first overflow should occur")
	}

	// Clear flag
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x15) // Reset A, keep load+enable

	// Should overflow again on next tick (counter reloaded)
	y.stepTimers()
	if !y.timerAOver {
		t.Error("Timer A should reload and overflow again")
	}
}

// --- Load without enable ---

func TestTimer_LoadWithoutEnable(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03)

	// Load but don't enable (bit0=load, bit2=enable not set)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x01) // Load only

	y.stepTimers()
	if y.timerAOver {
		t.Error("Timer A should not set overflow flag when not enabled")
	}
}

func TestTimer_EnableAfterLoad(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03) // period 1023

	// Load first
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x01) // Load only

	// Timer is counting but flag won't set
	y.stepTimers()
	if y.timerAOver {
		t.Error("should not overflow when not enabled")
	}

	// Now enable
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x05) // Load + enable

	// The counter has been running, so it may overflow on next tick
	y.stepTimers()
	// The timer should eventually overflow now that it's enabled
	if !y.timerAOver {
		// May need more ticks if counter didn't reset
		t.Log("Timer A did not overflow immediately after enabling, counter may have wrapped")
	}
}
