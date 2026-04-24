package core

import "testing"

func TestCycleDispenserNTSCM68KTotalPerSecond(t *testing.T) {
	d := newCycleDispenser(NTSCTiming.M68KClockHz, NTSCTiming.FPS, NTSCTiming.Scanlines)
	total := 0
	calls := NTSCTiming.FPS * NTSCTiming.Scanlines
	for i := 0; i < calls; i++ {
		total += d.Next()
	}
	if total != NTSCTiming.M68KClockHz {
		t.Fatalf("NTSC M68K: sum of %d calls = %d, want %d", calls, total, NTSCTiming.M68KClockHz)
	}
	if d.accum != 0 {
		t.Fatalf("NTSC M68K: residue after exact period = %d, want 0", d.accum)
	}
}

func TestCycleDispenserNTSCZ80TotalPerSecond(t *testing.T) {
	d := newCycleDispenser(NTSCTiming.Z80ClockHz, NTSCTiming.FPS, NTSCTiming.Scanlines)
	total := 0
	calls := NTSCTiming.FPS * NTSCTiming.Scanlines
	for i := 0; i < calls; i++ {
		total += d.Next()
	}
	if total != NTSCTiming.Z80ClockHz {
		t.Fatalf("NTSC Z80: sum of %d calls = %d, want %d", calls, total, NTSCTiming.Z80ClockHz)
	}
}

func TestCycleDispenserPALM68KTotalPerSecond(t *testing.T) {
	d := newCycleDispenser(PALTiming.M68KClockHz, PALTiming.FPS, PALTiming.Scanlines)
	total := 0
	calls := PALTiming.FPS * PALTiming.Scanlines
	for i := 0; i < calls; i++ {
		total += d.Next()
	}
	if total != PALTiming.M68KClockHz {
		t.Fatalf("PAL M68K: sum of %d calls = %d, want %d", calls, total, PALTiming.M68KClockHz)
	}
}

func TestCycleDispenserPALZ80TotalPerSecond(t *testing.T) {
	d := newCycleDispenser(PALTiming.Z80ClockHz, PALTiming.FPS, PALTiming.Scanlines)
	total := 0
	calls := PALTiming.FPS * PALTiming.Scanlines
	for i := 0; i < calls; i++ {
		total += d.Next()
	}
	if total != PALTiming.Z80ClockHz {
		t.Fatalf("PAL Z80: sum of %d calls = %d, want %d", calls, total, PALTiming.Z80ClockHz)
	}
}

func TestCycleDispenserPerCallValuesAreAdjacentIntegers(t *testing.T) {
	// For NTSC M68K, true cycles/scanline = 7670454 / (60*262) = 487.94...
	// Every call must be exactly floor() or ceil() of that — 487 or 488.
	d := newCycleDispenser(NTSCTiming.M68KClockHz, NTSCTiming.FPS, NTSCTiming.Scanlines)
	divisor := NTSCTiming.FPS * NTSCTiming.Scanlines
	floor := NTSCTiming.M68KClockHz / divisor
	ceil := floor + 1
	sawFloor, sawCeil := false, false
	for i := 0; i < divisor*3; i++ {
		n := d.Next()
		if n != floor && n != ceil {
			t.Fatalf("call %d produced %d; want %d or %d", i, n, floor, ceil)
		}
		if n == floor {
			sawFloor = true
		}
		if n == ceil {
			sawCeil = true
		}
	}
	if !sawFloor || !sawCeil {
		t.Fatalf("expected both %d and %d to appear; sawFloor=%v sawCeil=%v", floor, ceil, sawFloor, sawCeil)
	}
}

func TestCycleDispenserResidueBounded(t *testing.T) {
	d := newCycleDispenser(NTSCTiming.M68KClockHz, NTSCTiming.FPS, NTSCTiming.Scanlines)
	divisor := NTSCTiming.FPS * NTSCTiming.Scanlines
	for i := 0; i < divisor*10; i++ {
		d.Next()
		if d.accum < 0 || d.accum >= divisor {
			t.Fatalf("call %d: accum=%d out of [0, %d)", i, d.accum, divisor)
		}
	}
}

func TestCycleDispenserNoLongTermDrift(t *testing.T) {
	// Over 10 seconds of NTSC emulation the dispensed total must equal
	// 10 * clockHz exactly.
	d := newCycleDispenser(NTSCTiming.M68KClockHz, NTSCTiming.FPS, NTSCTiming.Scanlines)
	seconds := 10
	calls := seconds * NTSCTiming.FPS * NTSCTiming.Scanlines
	total := 0
	for i := 0; i < calls; i++ {
		total += d.Next()
	}
	want := seconds * NTSCTiming.M68KClockHz
	if total != want {
		t.Fatalf("10-second total = %d, want %d (drift = %d)", total, want, total-want)
	}
}
