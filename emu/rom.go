package emu

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// ValidateSystemType checks that the ROM contains a recognized Genesis system
// type string at offset $100-$10F.
func ValidateSystemType(rom []byte) error {
	if len(rom) < 0x110 {
		return fmt.Errorf("ROM too short to contain system type header (%d bytes)", len(rom))
	}

	sysType := strings.TrimRight(string(rom[0x100:0x110]), " ")
	switch sysType {
	case "SEGA MEGA DRIVE", "SEGA GENESIS":
		return nil
	default:
		return fmt.Errorf("unrecognized system type: %q", sysType)
	}
}

// ValidateChecksum verifies the ROM header checksum at offset $18E-$18F.
// The checksum is the 16-bit sum of all big-endian words from $200 to end of ROM.
func ValidateChecksum(rom []byte) error {
	if len(rom) < 0x200 {
		return fmt.Errorf("ROM too short to validate checksum (%d bytes)", len(rom))
	}

	expected := binary.BigEndian.Uint16(rom[0x18E:0x190])

	var computed uint16
	data := rom[0x200:]
	// Sum complete 16-bit words
	for i := 0; i+1 < len(data); i += 2 {
		computed += binary.BigEndian.Uint16(data[i : i+2])
	}
	// Odd trailing byte treated as high byte with low byte = 0
	if len(data)%2 != 0 {
		computed += uint16(data[len(data)-1]) << 8
	}

	if computed != expected {
		return fmt.Errorf("checksum mismatch: header=%04X computed=%04X", expected, computed)
	}
	return nil
}
