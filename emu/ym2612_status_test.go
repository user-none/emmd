package emu

import "testing"

// advanceNativeSamples generates enough M68K cycles to advance n native samples.
// Each native sample requires 144 M68K cycles.
func advanceNativeSamples(y *YM2612, n int) {
	y.GenerateSamples(n * 144)
	y.GetBuffer() // Drain buffer to avoid accumulation
}

// --- Busy Flag Tests ---

func TestYM2612_BusyFlag_SetOnDataWrite(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Write address latch then data (port 1 = data write)
	y.WritePort(0, 0x2A) // Address latch: DAC data register
	y.WritePort(1, 0x80) // Data write: should set busy

	status := y.ReadPort(0)
	if status&0x80 == 0 {
		t.Errorf("expected busy flag (bit 7) set after data write, got 0x%02X", status)
	}
}

func TestYM2612_BusyFlag_SetOnPort3DataWrite(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Write via Part II (port 2/3)
	y.WritePort(2, 0x30) // Address latch Part II
	y.WritePort(3, 0x00) // Data write Part II: should set busy

	status := y.ReadPort(0)
	if status&0x80 == 0 {
		t.Errorf("expected busy flag set after port 3 data write, got 0x%02X", status)
	}
}

func TestYM2612_BusyFlag_NotSetOnAddressWrite(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Address latch only (port 0) - should NOT set busy
	y.WritePort(0, 0x2A)

	status := y.ReadPort(0)
	if status&0x80 != 0 {
		t.Errorf("expected no busy flag after address latch write, got 0x%02X", status)
	}
}

func TestYM2612_BusyFlag_NotSetOnPort2AddressWrite(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Address latch only (port 2) - should NOT set busy
	y.WritePort(2, 0x30)

	status := y.ReadPort(0)
	if status&0x80 != 0 {
		t.Errorf("expected no busy flag after port 2 address write, got 0x%02X", status)
	}
}

func TestYM2612_BusyFlag_ClearsAfterDuration(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set busy
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x80)

	// Advance past busy duration (2 native samples)
	advanceNativeSamples(y, busyDuration)

	status := y.ReadPort(0)
	if status&0x80 != 0 {
		t.Errorf("expected busy flag cleared after %d native samples, got 0x%02X", busyDuration, status)
	}
}

func TestYM2612_BusyFlag_StillSetDuringDuration(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set busy
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x80)

	// Advance only 1 native sample (busy duration is 2)
	advanceNativeSamples(y, 1)

	status := y.ReadPort(0)
	if status&0x80 == 0 {
		t.Errorf("expected busy flag still set after 1 native sample, got 0x%02X", status)
	}
}

func TestYM2612_BusyFlag_MultipleWritesExtend(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// First write sets busy
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x80)

	// Advance 1 sample
	advanceNativeSamples(y, 1)

	// Second write should extend busy window
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x40)

	// Advance 1 more sample - should still be busy (second write reset the window)
	advanceNativeSamples(y, 1)

	status := y.ReadPort(0)
	if status&0x80 == 0 {
		t.Errorf("expected busy flag still set after second write, got 0x%02X", status)
	}

	// Advance past the second write's busy window
	advanceNativeSamples(y, busyDuration)

	status = y.ReadPort(0)
	if status&0x80 != 0 {
		t.Errorf("expected busy flag cleared after full duration, got 0x%02X", status)
	}
}

// --- Status Port Caching Tests ---

func TestYM2612_StatusCache_Port1ReturnsLastPort0(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set up Timer A to overflow
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF) // Timer A MSB = max (short period)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03) // Timer A LSB
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x05) // Load Timer A + enable flag

	// Advance to overflow
	advanceNativeSamples(y, 2000)

	// Read port 0 to get status with timer A flag and cache it
	// Need to clear busy first (from the writes above)
	status0 := y.ReadPort(0)
	if status0&0x01 == 0 {
		t.Fatalf("expected Timer A overflow flag set, got 0x%02X", status0)
	}

	// Port 1 should return cached status (same value, minus any timing changes)
	status1 := y.ReadPort(1)
	if status1 != status0 {
		t.Errorf("expected port 1 to return cached status 0x%02X, got 0x%02X", status0, status1)
	}
}

func TestYM2612_StatusCache_Port3ReturnsLastPort2(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set up Timer A to overflow
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF) // Timer A MSB
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03) // Timer A LSB
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x05) // Load Timer A + enable flag

	// Advance to overflow
	advanceNativeSamples(y, 2000)

	// Read port 2 to get and cache status
	status2 := y.ReadPort(2)
	if status2&0x01 == 0 {
		t.Fatalf("expected Timer A overflow flag set, got 0x%02X", status2)
	}

	// Port 3 should return cached status from port 2 read
	status3 := y.ReadPort(3)
	if status3 != status2 {
		t.Errorf("expected port 3 to return cached status 0x%02X, got 0x%02X", status2, status3)
	}
}

func TestYM2612_StatusCache_DecaysToZero(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set up Timer A to overflow
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x05)

	// Advance to overflow
	advanceNativeSamples(y, 2000)

	// Read port 0 to cache a non-zero status
	status0 := y.ReadPort(0)
	if status0 == 0 {
		t.Fatal("expected non-zero status from port 0")
	}

	// Advance past decay threshold
	advanceNativeSamples(y, statusDecayDuration)

	// Port 1 should now return 0 (decayed)
	status1 := y.ReadPort(1)
	if status1 != 0 {
		t.Errorf("expected port 1 to return 0 after decay, got 0x%02X", status1)
	}
}

func TestYM2612_StatusCache_Port1DoesNotUpdateCache(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Read port 0 - caches status (should be 0, no timers)
	y.ReadPort(0)

	// Set up Timer A to overflow
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x05)

	// Advance to overflow
	advanceNativeSamples(y, 2000)

	// Read port 1 - should return cached value (0 from earlier port 0 read),
	// NOT the current live status. The cache was set before timers overflowed.
	// However, the cache may have decayed since we advanced 2000 samples,
	// which is within the decay window (13300). So it should return the cached 0.
	status1 := y.ReadPort(1)
	if status1 != 0 {
		t.Errorf("expected port 1 to return cached 0 (not live timer status), got 0x%02X", status1)
	}

	// Now read port 0 to get and cache the current live status
	status0 := y.ReadPort(0)
	if status0&0x01 == 0 {
		t.Fatalf("expected Timer A flag set on port 0, got 0x%02X", status0)
	}

	// Reading port 1 again should now reflect the newly cached value
	status1 = y.ReadPort(1)
	if status1 != status0 {
		t.Errorf("expected port 1 to return newly cached 0x%02X, got 0x%02X", status0, status1)
	}
}

// --- Combined Tests ---

func TestYM2612_BusyAndTimerCombined(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set up Timer A to overflow
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x05) // This is a data write, sets busy

	// The last write was a data write - busy should be set
	// Read port 0: should show both busy (bit 7) and possibly timer flags
	status := y.ReadPort(0)
	if status&0x80 == 0 {
		t.Errorf("expected busy flag set immediately after register write, got 0x%02X", status)
	}

	// Advance past busy but not long enough for timer overflow
	advanceNativeSamples(y, busyDuration)

	status = y.ReadPort(0)
	if status&0x80 != 0 {
		t.Errorf("expected busy flag cleared, got 0x%02X", status)
	}

	// Advance to timer overflow
	advanceNativeSamples(y, 2000)

	// Read port 0: timer A should be set, busy should be clear
	status = y.ReadPort(0)
	if status&0x01 == 0 {
		t.Errorf("expected Timer A flag after overflow, got 0x%02X", status)
	}
	if status&0x80 != 0 {
		t.Errorf("expected no busy flag long after write, got 0x%02X", status)
	}

	// Port 1 should return the cached value from the port 0 read above
	status1 := y.ReadPort(1)
	if status1 != status {
		t.Errorf("expected port 1 cached status 0x%02X, got 0x%02X", status, status1)
	}
}

func TestYM2612_BusyCachedOnPort0Read(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Write register to set busy
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x80)

	// Read port 0: gets busy flag, caches it
	status0 := y.ReadPort(0)
	if status0&0x80 == 0 {
		t.Fatalf("expected busy flag on port 0, got 0x%02X", status0)
	}

	// Read port 1: should return cached value including busy bit
	status1 := y.ReadPort(1)
	if status1 != status0 {
		t.Errorf("expected port 1 to return cached 0x%02X, got 0x%02X", status0, status1)
	}
}
