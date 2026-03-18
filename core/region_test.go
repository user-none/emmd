package core

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

func TestDetectVideoStandard_JUE(t *testing.T) {
	if got := DetectVideoStandard(makeROM("JUE")); got != VideoNTSC {
		t.Errorf("JUE: got %v, want NTSC", got)
	}
}

func TestDetectVideoStandard_U(t *testing.T) {
	if got := DetectVideoStandard(makeROM("U")); got != VideoNTSC {
		t.Errorf("U: got %v, want NTSC", got)
	}
}

func TestDetectVideoStandard_E(t *testing.T) {
	if got := DetectVideoStandard(makeROM("E")); got != VideoPAL {
		t.Errorf("E: got %v, want PAL", got)
	}
}

func TestDetectVideoStandard_J(t *testing.T) {
	if got := DetectVideoStandard(makeROM("J")); got != VideoNTSC {
		t.Errorf("J: got %v, want NTSC", got)
	}
}

func TestDetectVideoStandard_UE(t *testing.T) {
	if got := DetectVideoStandard(makeROM("UE")); got != VideoNTSC {
		t.Errorf("UE: got %v, want NTSC (prefer NTSC for multi-region)", got)
	}
}

func TestDetectVideoStandard_Empty(t *testing.T) {
	// Region field filled with spaces (no region characters)
	if got := DetectVideoStandard(makeROM("")); got != VideoNTSC {
		t.Errorf("empty: got %v, want NTSC (default)", got)
	}
}

func TestDetectVideoStandard_ROMTooShort(t *testing.T) {
	rom := make([]byte, 0x100)
	if got := DetectVideoStandard(rom); got != VideoNTSC {
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

// --- Hex digit format DetectVideoStandard tests ---

func TestDetectVideoStandard_Hex8_PAL(t *testing.T) {
	if got := DetectVideoStandard(makeROM("8")); got != VideoPAL {
		t.Errorf("hex 8: got %v, want PAL", got)
	}
}

func TestDetectVideoStandard_Hex9_NTSC(t *testing.T) {
	if got := DetectVideoStandard(makeROM("9")); got != VideoNTSC {
		t.Errorf("hex 9: got %v, want NTSC", got)
	}
}

func TestDetectVideoStandard_HexF_NTSC(t *testing.T) {
	if got := DetectVideoStandard(makeROM("F")); got != VideoNTSC {
		t.Errorf("hex F: got %v, want NTSC", got)
	}
}
