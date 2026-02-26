package adapter

import (
	emucore "github.com/user-none/eblitui/api"
	"github.com/user-none/emmd/emu"
)

// Compile-time interface check.
var _ emucore.CoreFactory = (*Factory)(nil)

// Factory implements emucore.CoreFactory for the Genesis emulator.
type Factory struct{}

// SystemInfo returns system metadata for UI configuration.
func (f *Factory) SystemInfo() emucore.SystemInfo {
	return emucore.SystemInfo{
		Name:            "emmd",
		ConsoleName:     "Sega Genesis",
		Extensions:      []string{".md", ".bin", ".gen"},
		ScreenWidth:     emu.ScreenWidth,
		MaxScreenHeight: emu.MaxScreenHeight,
		// NTSC pixel aspect ratio for H40 mode (32:35).
		// The Genesis master clock is 53.693175 MHz. In H40, the pixel
		// clock is master/8 and 320 active pixels span 2560 master clocks
		// (47.68 us). BT.601 defines 720 active samples at 13.5 MHz
		// (53.33 us) for a 4:3 NTSC display. The PAR is the ratio of
		// these active times scaled by the sample counts:
		// PAR = (2560/53693175) / (720/13500000) * (720/320)
		//     = 32/35
		// Both H32 (256px) and H40 (320px) share the same 2560 master
		// clock active time. H32 pixels are stretched to 320px in the
		// framebuffer, so the same PAR applies to both modes.
		// The PAL master clock (53.203424 MHz) differs by <1%, producing
		// a negligible PAR difference, so this value is used for both.
		PixelAspectRatio: 32.0 / 35.0,
		SampleRate:       48000,
		Buttons: []emucore.Button{
			{Name: "A", ID: 4, DefaultKey: "J", DefaultPad: "X"},
			{Name: "B", ID: 5, DefaultKey: "K", DefaultPad: "A"},
			{Name: "C", ID: 6, DefaultKey: "L", DefaultPad: "B"},
			{Name: "X", ID: 8, DefaultKey: "U", DefaultPad: "L1"},
			{Name: "Y", ID: 9, DefaultKey: "I", DefaultPad: "Y"},
			{Name: "Z", ID: 10, DefaultKey: "O", DefaultPad: "R1"},
			{Name: "Start", ID: 7, DefaultKey: "Enter", DefaultPad: "Start"},
		},
		Players: 2,
		CoreOptions: []emucore.CoreOption{
			{
				Key:         "six_button",
				Label:       "6-Button Controller",
				Description: "Enable 6-button controller mode",
				Type:        emucore.CoreOptionBool,
				Default:     "false",
				Category:    emucore.CoreOptionCategoryInput,
			},
		},
		RDBName:         "Sega - Mega Drive - Genesis",
		ThumbnailRepo:   "Sega_-_Mega_Drive_-_Genesis",
		DataDirName:     "emmd",
		ConsoleID:       1,
		CoreName:        emu.Name,
		CoreVersion:     emu.Version,
		SerializeSize:   emu.SerializeSize(),
		BigEndianMemory: true,
	}
}

// CreateEmulator creates a new emulator instance with the given ROM and region.
func (f *Factory) CreateEmulator(rom []byte, region emucore.Region) (emucore.Emulator, error) {
	e, err := emu.NewEmulator(rom, region)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// DetectRegion auto-detects the region from ROM header data.
// The bool return is false since emmd uses header-based detection,
// not a ROM database lookup.
func (f *Factory) DetectRegion(rom []byte) (emucore.Region, bool) {
	return emu.DetectRegion(rom), false
}
