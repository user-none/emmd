package emu

import (
	"encoding/binary"
	"testing"
)

// makeValidationROM builds a ROM with the given system type at $100 and
// a valid checksum. The ROM is 0x400 bytes so there is data after $200
// for the checksum to cover.
func makeValidationROM(sysType string) []byte {
	rom := make([]byte, 0x400)

	// Fill system type field with spaces, then copy the string
	for i := 0x100; i < 0x110; i++ {
		rom[i] = ' '
	}
	copy(rom[0x100:0x110], []byte(sysType))

	// Put some data after $200 so checksum isn't trivially zero
	rom[0x200] = 0x01
	rom[0x201] = 0x23
	rom[0x300] = 0x45
	rom[0x301] = 0x67

	// Compute and store valid checksum
	var sum uint16
	for i := 0x200; i+1 < len(rom); i += 2 {
		sum += binary.BigEndian.Uint16(rom[i : i+2])
	}
	binary.BigEndian.PutUint16(rom[0x18E:0x190], sum)

	return rom
}

func TestValidateSystemType_MegaDrive(t *testing.T) {
	rom := makeValidationROM("SEGA MEGA DRIVE")
	if err := ValidateSystemType(rom); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateSystemType_Genesis(t *testing.T) {
	rom := makeValidationROM("SEGA GENESIS")
	if err := ValidateSystemType(rom); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateSystemType_Invalid(t *testing.T) {
	rom := makeValidationROM("NOT A GENESIS")
	if err := ValidateSystemType(rom); err == nil {
		t.Error("expected error for invalid system type, got nil")
	}
}

func TestValidateSystemType_TooShort(t *testing.T) {
	rom := make([]byte, 0x100)
	if err := ValidateSystemType(rom); err == nil {
		t.Error("expected error for short ROM, got nil")
	}
}

func TestValidateChecksum_Valid(t *testing.T) {
	rom := makeValidationROM("SEGA MEGA DRIVE")
	if err := ValidateChecksum(rom); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateChecksum_Invalid(t *testing.T) {
	rom := makeValidationROM("SEGA MEGA DRIVE")
	// Corrupt one byte in the data area
	rom[0x200] = 0xFF
	if err := ValidateChecksum(rom); err == nil {
		t.Error("expected error for bad checksum, got nil")
	}
}

func TestValidateChecksum_TooShort(t *testing.T) {
	rom := make([]byte, 0x100)
	if err := ValidateChecksum(rom); err == nil {
		t.Error("expected error for short ROM, got nil")
	}
}

func TestValidateChecksum_OddLength(t *testing.T) {
	// Create a ROM with an odd number of bytes after $200
	rom := make([]byte, 0x203)
	for i := 0x100; i < 0x110; i++ {
		rom[i] = ' '
	}
	copy(rom[0x100:0x110], []byte("SEGA MEGA DRIVE"))

	rom[0x200] = 0xAA
	rom[0x201] = 0xBB
	rom[0x202] = 0xCC // odd trailing byte: treated as 0xCC00

	var sum uint16
	sum += binary.BigEndian.Uint16(rom[0x200:0x202]) // 0xAABB
	sum += uint16(rom[0x202]) << 8                   // 0xCC00
	binary.BigEndian.PutUint16(rom[0x18E:0x190], sum)

	if err := ValidateChecksum(rom); err != nil {
		t.Errorf("expected nil for odd-length ROM, got %v", err)
	}
}
