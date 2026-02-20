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

// DetectRegion inspects the ROM header region field at offset $1F0-$1FF
// and returns PAL only when no NTSC region characters (J, U) are present
// and at least one PAL character (E) is found. Multi-region ROMs prefer NTSC.
func DetectRegion(rom []byte) Region {
	if len(rom) < 0x200 {
		return RegionNTSC
	}
	hasNTSC := false
	hasPAL := false
	for _, b := range rom[0x1F0:0x200] {
		switch b {
		case 'J', 'U':
			hasNTSC = true
		case 'E':
			hasPAL = true
		}
	}
	if hasPAL && !hasNTSC {
		return RegionPAL
	}
	return RegionNTSC
}

// DefaultRegion returns the default region (NTSC).
func DefaultRegion() Region {
	return RegionNTSC
}
