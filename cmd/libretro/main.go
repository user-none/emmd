package main

import (
	libretro "github.com/user-none/eblitui/libretro"
	"github.com/user-none/emmd/adapter"
)

func init() {
	libretro.RegisterFactory(&adapter.Factory{}, []libretro.RetropadMapping{
		{RetroID: libretro.JoypadY, BitID: 4},       // A
		{RetroID: libretro.JoypadB, BitID: 5},       // B
		{RetroID: libretro.JoypadA, BitID: 6},       // C
		{RetroID: libretro.JoypadStart, BitID: 7},   // Start
		{RetroID: libretro.JoypadX, BitID: 8},       // X
		{RetroID: libretro.JoypadL, BitID: 9},       // Y
		{RetroID: libretro.JoypadR, BitID: 10},      // Z
		{RetroID: libretro.JoypadSelect, BitID: 11}, // Mode
	})
}

func main() {}
