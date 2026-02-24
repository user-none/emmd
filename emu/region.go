package emu

import emucore "github.com/user-none/eblitui/api"

// Region is an alias for emucore.Region so internal code compiles unchanged.
type Region = emucore.Region

const (
	RegionNTSC = emucore.RegionNTSC
	RegionPAL  = emucore.RegionPAL
)

// RegionTiming holds timing constants for a specific region.
// The Genesis has two CPUs with different clock rates.
type RegionTiming struct {
	M68KClockHz int // Motorola 68000 clock frequency
	Z80ClockHz  int // Z80 sound CPU clock frequency
	Scanlines   int // Total scanlines per frame
	FPS         int // Frames per second
}

// NTSC timing: M68K 7.670454 MHz, Z80 3.579545 MHz, 262 scanlines, 60 Hz
var NTSCTiming = RegionTiming{
	M68KClockHz: 7670454,
	Z80ClockHz:  3579545,
	Scanlines:   262,
	FPS:         60,
}

// PAL timing: M68K 7.600489 MHz, Z80 3.546893 MHz, 313 scanlines, 50 Hz
var PALTiming = RegionTiming{
	M68KClockHz: 7600489,
	Z80ClockHz:  3546893,
	Scanlines:   313,
	FPS:         50,
}

// GetTimingForRegion returns the appropriate timing constants
func GetTimingForRegion(r Region) RegionTiming {
	if r == RegionPAL {
		return PALTiming
	}
	return NTSCTiming
}

// ConsoleRegion represents the hardware region identity of the console.
// This determines the version register value ($A10001) which games use
// for region lockout checks. It is separate from the display timing
// region (NTSC/PAL).
type ConsoleRegion int

const (
	ConsoleJapan  ConsoleRegion = iota // Domestic, NTSC
	ConsoleUSA                         // Overseas, NTSC
	ConsoleEurope                      // Overseas, PAL
)

// DetectConsoleRegion inspects the ROM header region field at offset $1F0-$1F2
// and returns the console region. The field uses either character format
// ('J', 'U', 'E') or hex digit format ('0'-'9', 'A'-'F') where bits encode
// regions: bit 0 = Japan, bit 2 = Americas, bit 3 = Europe.
// Character format is checked first. If no character codes are found, the
// first non-space byte is parsed as a hex digit.
// For multi-region ROMs, priority is U > J > E.
// Returns ConsoleUSA for unknown or missing region data.
func DetectConsoleRegion(rom []byte) ConsoleRegion {
	if len(rom) < 0x200 {
		return ConsoleUSA
	}

	hasJ := false
	hasU := false
	hasE := false

	// Phase 1: scan first 3 bytes for character format codes.
	for _, b := range rom[0x1F0:0x1F3] {
		switch b {
		case 'J':
			hasJ = true
		case 'U':
			hasU = true
		case 'E':
			hasE = true
		}
	}

	// Phase 2: if no character codes found, try hex digit format.
	if !hasJ && !hasU && !hasE {
		for _, b := range rom[0x1F0:0x1F3] {
			if b == ' ' {
				continue
			}
			var val int
			if b >= '0' && b <= '9' {
				val = int(b - '0')
			} else if b >= 'A' && b <= 'F' {
				val = int(b-'A') + 10
			} else {
				break
			}
			if val&0x1 != 0 {
				hasJ = true
			}
			if val&0x4 != 0 {
				hasU = true
			}
			if val&0x8 != 0 {
				hasE = true
			}
			break
		}
	}

	if hasU {
		return ConsoleUSA
	}
	if hasJ {
		return ConsoleJapan
	}
	if hasE {
		return ConsoleEurope
	}
	return ConsoleUSA
}

// DetectRegion inspects the ROM header region field at offset $1F0-$1F2
// and returns the display timing region. ConsoleEurope maps to PAL;
// ConsoleJapan and ConsoleUSA map to NTSC.
func DetectRegion(rom []byte) Region {
	if DetectConsoleRegion(rom) == ConsoleEurope {
		return RegionPAL
	}
	return RegionNTSC
}

// DefaultRegion returns the default region (NTSC).
func DefaultRegion() Region {
	return RegionNTSC
}
