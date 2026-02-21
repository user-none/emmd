//go:build !libretro && !ios

package main

import (
	"flag"
	"log"

	"github.com/user-none/eblitui/standalone"
	"github.com/user-none/emmd/adapter"
)

func main() {
	romPath := flag.String("rom", "", "path to ROM file (opens UI if not provided)")
	regionFlag := flag.String("region", "auto", "region: auto, ntsc, or pal")
	sixButton := flag.Bool("six-button", true, "enable 6-button controller")
	flag.Parse()

	factory := &adapter.Factory{}

	if *romPath != "" {
		options := map[string]string{}
		if *sixButton {
			options["six_button"] = "true"
		} else {
			options["six_button"] = "false"
		}
		if err := standalone.RunDirect(factory, *romPath, *regionFlag, options); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := standalone.Run(factory); err != nil {
		log.Fatal(err)
	}
}
