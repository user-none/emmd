//go:build !ios

package main

import (
	"flag"
	"log"

	"github.com/user-none/eblitui/desktop"
	"github.com/user-none/emmd/adapter"
)

func main() {
	romPath := flag.String("rom", "", "path to ROM file (opens UI if not provided)")
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
		if err := desktop.RunDirect(factory, *romPath, options, nil); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := desktop.Run(factory); err != nil {
		log.Fatal(err)
	}
}
