package emu

import (
	"testing"

	"github.com/user-none/go-chip-m68k"
)

func TestPSG_BusWrite(t *testing.T) {
	bus := makeTestBus()
	psg := bus.psg

	// Write to PSG port 0xC00011 via the bus
	// Latch channel 0, volume = 5: 1 00 1 0101 = 0x95
	bus.WriteCycle(0, m68k.Byte, 0xC00011, 0x95)
	if psg.GetVolume(0) != 5 {
		t.Errorf("expected PSG volume[0]=5 after bus write, got %d", psg.GetVolume(0))
	}
}

func TestPSG_BusWriteWord(t *testing.T) {
	bus := makeTestBus()
	psg := bus.psg

	// Word write to 0xC00010 - PSG data is in the low byte.
	// Many games use word writes to the PSG port.
	// Latch channel 1, volume = 3: 1 01 1 0011 = 0xB3
	bus.WriteCycle(0, m68k.Word, 0xC00010, 0x00B3)
	if psg.GetVolume(1) != 3 {
		t.Errorf("expected PSG volume[1]=3 after word write, got %d", psg.GetVolume(1))
	}
}
