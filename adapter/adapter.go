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
		AspectRatio:     320.0 / 224.0,
		SampleRate:      48000,
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
		RDBName:       "Sega - Mega Drive - Genesis",
		ThumbnailRepo: "Sega_-_Mega_Drive_-_Genesis",
		DataDirName:   "emmd",
		ConsoleID:     1,
		CoreName:      emu.Name,
		CoreVersion:   emu.Version,
		SerializeSize: emu.SerializeSize(),
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
