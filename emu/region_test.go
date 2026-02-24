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

// --- ConsoleRegion tests ---

func TestDetectConsoleRegion_J(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("J")); got != ConsoleJapan {
		t.Errorf("J: got %v, want ConsoleJapan", got)
	}
}

func TestDetectConsoleRegion_U(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("U")); got != ConsoleUSA {
		t.Errorf("U: got %v, want ConsoleUSA", got)
	}
}

func TestDetectConsoleRegion_E(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("E")); got != ConsoleEurope {
		t.Errorf("E: got %v, want ConsoleEurope", got)
	}
}

func TestDetectConsoleRegion_JUE(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("JUE")); got != ConsoleUSA {
		t.Errorf("JUE: got %v, want ConsoleUSA (U takes priority)", got)
	}
}

func TestDetectConsoleRegion_UE(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("UE")); got != ConsoleUSA {
		t.Errorf("UE: got %v, want ConsoleUSA", got)
	}
}

func TestDetectConsoleRegion_JE(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("JE")); got != ConsoleJapan {
		t.Errorf("JE: got %v, want ConsoleJapan (J takes priority over E)", got)
	}
}

func TestDetectConsoleRegion_Empty(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("")); got != ConsoleUSA {
		t.Errorf("empty: got %v, want ConsoleUSA (default)", got)
	}
}

func TestDetectConsoleRegion_ROMTooShort(t *testing.T) {
	rom := make([]byte, 0x100)
	if got := DetectConsoleRegion(rom); got != ConsoleUSA {
		t.Errorf("short ROM: got %v, want ConsoleUSA (default)", got)
	}
}

// --- Hex digit format ConsoleRegion tests ---

func TestDetectConsoleRegion_Hex1_Japan(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("1")); got != ConsoleJapan {
		t.Errorf("hex 1: got %v, want ConsoleJapan", got)
	}
}

func TestDetectConsoleRegion_Hex4_Americas(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("4")); got != ConsoleUSA {
		t.Errorf("hex 4: got %v, want ConsoleUSA", got)
	}
}

func TestDetectConsoleRegion_Hex5_JapanAmericas(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("5")); got != ConsoleUSA {
		t.Errorf("hex 5: got %v, want ConsoleUSA (U takes priority)", got)
	}
}

func TestDetectConsoleRegion_Hex8_Europe(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("8")); got != ConsoleEurope {
		t.Errorf("hex 8: got %v, want ConsoleEurope", got)
	}
}

func TestDetectConsoleRegion_Hex9_JapanEurope(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("9")); got != ConsoleJapan {
		t.Errorf("hex 9: got %v, want ConsoleJapan (J takes priority over E)", got)
	}
}

func TestDetectConsoleRegion_HexC_AmericasEurope(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("C")); got != ConsoleUSA {
		t.Errorf("hex C: got %v, want ConsoleUSA (U takes priority)", got)
	}
}

func TestDetectConsoleRegion_HexD_All(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("D")); got != ConsoleUSA {
		t.Errorf("hex D: got %v, want ConsoleUSA (U takes priority)", got)
	}
}

func TestDetectConsoleRegion_HexF_All(t *testing.T) {
	if got := DetectConsoleRegion(makeROM("F")); got != ConsoleUSA {
		t.Errorf("hex F: got %v, want ConsoleUSA (U takes priority)", got)
	}
}

// --- Hex digit format DetectRegion tests ---

func TestDetectRegion_Hex8_PAL(t *testing.T) {
	if got := DetectRegion(makeROM("8")); got != RegionPAL {
		t.Errorf("hex 8: got %v, want PAL", got)
	}
}

func TestDetectRegion_Hex9_NTSC(t *testing.T) {
	if got := DetectRegion(makeROM("9")); got != RegionNTSC {
		t.Errorf("hex 9: got %v, want NTSC", got)
	}
}

func TestDetectRegion_HexF_NTSC(t *testing.T) {
	if got := DetectRegion(makeROM("F")); got != RegionNTSC {
		t.Errorf("hex F: got %v, want NTSC", got)
	}
}
