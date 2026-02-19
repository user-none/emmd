package emu

import "testing"

// makeROM builds a 0x200-byte ROM with the given region string at offset 0x1F0.
func makeROM(region string) []byte {
	rom := make([]byte, 0x200)
	// Fill region field with spaces (default padding)
	for i := 0x1F0; i < 0x200; i++ {
		rom[i] = ' '
	}
	copy(rom[0x1F0:], []byte(region))
	return rom
}

func TestDetectRegion_JUE(t *testing.T) {
	if got := DetectRegion(makeROM("JUE")); got != RegionNTSC {
		t.Errorf("JUE: got %v, want NTSC", got)
	}
}

func TestDetectRegion_U(t *testing.T) {
	if got := DetectRegion(makeROM("U")); got != RegionNTSC {
		t.Errorf("U: got %v, want NTSC", got)
	}
}

func TestDetectRegion_E(t *testing.T) {
	if got := DetectRegion(makeROM("E")); got != RegionPAL {
		t.Errorf("E: got %v, want PAL", got)
	}
}

func TestDetectRegion_J(t *testing.T) {
	if got := DetectRegion(makeROM("J")); got != RegionNTSC {
		t.Errorf("J: got %v, want NTSC", got)
	}
}

func TestDetectRegion_UE(t *testing.T) {
	if got := DetectRegion(makeROM("UE")); got != RegionNTSC {
		t.Errorf("UE: got %v, want NTSC (prefer NTSC for multi-region)", got)
	}
}

func TestDetectRegion_Empty(t *testing.T) {
	// Region field filled with spaces (no region characters)
	if got := DetectRegion(makeROM("")); got != RegionNTSC {
		t.Errorf("empty: got %v, want NTSC (default)", got)
	}
}

func TestDetectRegion_ROMTooShort(t *testing.T) {
	rom := make([]byte, 0x100)
	if got := DetectRegion(rom); got != RegionNTSC {
		t.Errorf("short ROM: got %v, want NTSC (default)", got)
	}
}
