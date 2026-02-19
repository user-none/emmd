package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	emubridge "github.com/user-none/emmd/bridge/ebiten"
	"github.com/user-none/emmd/cli"
	"github.com/user-none/emmd/emu"
)

func main() {
	romPath := flag.String("rom", "", "path to ROM file (required)")
	regionFlag := flag.String("region", "auto", "region: auto, ntsc, or pal")
	sixButton := flag.Bool("6button", true, "enable 6-button controller (false for 3-button)")
	flag.Parse()

	if *romPath == "" {
		log.Fatal("ROM path is required. Usage: emmd -rom <path>")
	}

	romData, err := os.ReadFile(*romPath)
	if err != nil {
		log.Fatalf("Failed to load ROM: %v", err)
	}

	// Determine region
	var region emu.Region
	switch strings.ToLower(*regionFlag) {
	case "auto":
		region = emu.DetectRegion(romData)
	case "ntsc":
		region = emu.RegionNTSC
	case "pal":
		region = emu.RegionPAL
	default:
		log.Fatalf("Invalid region: %s (use auto, ntsc, or pal)", *regionFlag)
	}

	e, err := emubridge.NewEmulator(romData, region)
	if err != nil {
		log.Fatalf("Failed to initialize emulator: %v", err)
	}

	e.SetSixButton(*sixButton)

	// Load SRAM save file if it exists
	srmPath := strings.TrimSuffix(*romPath, filepath.Ext(*romPath)) + ".srm"
	if e.HasSRAM() {
		if data, err := os.ReadFile(srmPath); err == nil {
			e.SetSRAM(data)
		}
	}

	ebiten.SetWindowSize(emu.ScreenWidth*2, emu.DefaultScreenHeight*2)
	ebiten.SetWindowTitle(emu.Name)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSizeLimits(348, 348, -1, -1)
	ebiten.SetTPS(60)

	runner := cli.NewRunner(e)
	defer runner.Close()
	defer e.Close()

	// Save SRAM on exit
	defer func() {
		if e.HasSRAM() {
			if data := e.GetSRAM(); data != nil {
				os.WriteFile(srmPath, data, 0644)
			}
		}
	}()

	if err := ebiten.RunGame(runner); err != nil {
		log.Fatal(err)
	}
}
