package emu

import (
	"encoding/binary"
	"hash/crc32"
	"testing"

	"github.com/user-none/go-chip-m68k"
)

// createTestEmulator creates an Emulator with a minimal valid Genesis
// ROM for testing serialization. The ROM contains a valid vector table (SSP
// at address 0, PC at address 4) and a NOP at the entry point.
func createTestEmulator() *Emulator {
	rom := make([]byte, 1024)
	// SSP = 0x00FF0000 (big-endian at address 0)
	rom[0] = 0x00
	rom[1] = 0xFF
	rom[2] = 0x00
	rom[3] = 0x00
	// PC = 0x00000200 (big-endian at address 4)
	rom[4] = 0x00
	rom[5] = 0x00
	rom[6] = 0x02
	rom[7] = 0x00
	// NOP (0x4E71) at entry point 0x200
	rom[0x200] = 0x4E
	rom[0x201] = 0x71
	// Another NOP so stepping twice doesn't run off
	rom[0x202] = 0x4E
	rom[0x203] = 0x71

	base, err := NewEmulator(rom, RegionNTSC)
	if err != nil {
		panic("createTestEmulator: " + err.Error())
	}
	return &base
}

func TestSerializeSize(t *testing.T) {
	size1 := SerializeSize()
	size2 := SerializeSize()

	if size1 != size2 {
		t.Errorf("SerializeSize not consistent: %d vs %d", size1, size2)
	}

	if size1 < stateHeaderSize {
		t.Errorf("SerializeSize too small: %d < %d (header)", size1, stateHeaderSize)
	}
}

func TestSerializeDeserializeRoundTrip(t *testing.T) {
	base := createTestEmulator()

	// Run a few M68K steps to change CPU state
	for i := 0; i < 10; i++ {
		base.m68k.Step()
	}

	// Write recognizable values to main RAM via bus
	base.bus.WriteCycle(0, m68k.Byte, 0xFF0000, 0xAB)
	base.bus.WriteCycle(0, m68k.Byte, 0xFF0001, 0xCD)

	// Write to VDP registers (register 0 = 0x14)
	base.vdp.WriteControl(0, 0x8014)

	// Serialize
	state, err := base.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Corrupt emulator state
	base.bus.WriteCycle(0, m68k.Byte, 0xFF0000, 0xFF)
	base.bus.WriteCycle(0, m68k.Byte, 0xFF0001, 0xFF)
	base.vdp.WriteControl(0, 0x8000) // register 0 = 0x00

	// Deserialize
	err = base.Deserialize(state)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	// Verify RAM was restored
	val := base.bus.ReadCycle(0, m68k.Byte, 0xFF0000)
	if val != 0xAB {
		t.Errorf("RAM[0xFF0000]: expected 0xAB, got 0x%02X", val)
	}
	val = base.bus.ReadCycle(0, m68k.Byte, 0xFF0001)
	if val != 0xCD {
		t.Errorf("RAM[0xFF0001]: expected 0xCD, got 0x%02X", val)
	}

	// Verify VDP register was restored
	if base.vdp.regs[0] != 0x14 {
		t.Errorf("VDP Register 0: expected 0x14, got 0x%02X", base.vdp.regs[0])
	}
}

func TestVerifyState_ValidState(t *testing.T) {
	base := createTestEmulator()

	state, err := base.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	err = base.VerifyState(state)
	if err != nil {
		t.Errorf("VerifyState should pass for valid state: %v", err)
	}
}

func TestVerifyState_InvalidMagic(t *testing.T) {
	base := createTestEmulator()

	state, err := base.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Corrupt magic bytes
	state[0] = 'X'

	err = base.VerifyState(state)
	if err == nil {
		t.Error("VerifyState should reject invalid magic bytes")
	}
}

func TestVerifyState_UnsupportedVersion(t *testing.T) {
	base := createTestEmulator()

	state, err := base.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Set a future version number
	binary.LittleEndian.PutUint16(state[12:14], 9999)

	err = base.VerifyState(state)
	if err == nil {
		t.Error("VerifyState should reject unsupported version")
	}
}

func TestVerifyState_CorruptData(t *testing.T) {
	base := createTestEmulator()

	state, err := base.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Corrupt state data (after header)
	if len(state) > stateHeaderSize+10 {
		state[stateHeaderSize+5] ^= 0xFF
	}

	err = base.VerifyState(state)
	if err == nil {
		t.Error("VerifyState should reject corrupted data")
	}
}

func TestVerifyState_WrongROM(t *testing.T) {
	base1 := createTestEmulator()

	state, err := base1.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Create a different ROM
	differentROM := make([]byte, 2048)
	for i := range differentROM {
		differentROM[i] = byte(i & 0xFF)
	}
	// Needs valid vector table
	differentROM[0] = 0x00
	differentROM[1] = 0xFF
	differentROM[2] = 0x00
	differentROM[3] = 0x00
	differentROM[4] = 0x00
	differentROM[5] = 0x00
	differentROM[6] = 0x02
	differentROM[7] = 0x00
	differentROM[0x200] = 0x4E
	differentROM[0x201] = 0x71

	init2, err := NewEmulator(differentROM, RegionNTSC)
	if err != nil {
		t.Fatalf("NewEmulator failed: %v", err)
	}
	base2 := &init2

	err = base2.VerifyState(state)
	if err == nil {
		t.Error("VerifyState should reject state from different ROM")
	}
}

func TestVerifyState_TooShort(t *testing.T) {
	base := createTestEmulator()

	// Create data smaller than header
	state := make([]byte, stateHeaderSize-1)

	err := base.VerifyState(state)
	if err == nil {
		t.Error("VerifyState should reject data smaller than header")
	}
}

func TestDeserialize_PreservesRegion(t *testing.T) {
	// Create ROM for both emulators (same ROM, different regions)
	rom := make([]byte, 1024)
	rom[0] = 0x00
	rom[1] = 0xFF
	rom[2] = 0x00
	rom[3] = 0x00
	rom[4] = 0x00
	rom[5] = 0x00
	rom[6] = 0x02
	rom[7] = 0x00
	rom[0x200] = 0x4E
	rom[0x201] = 0x71

	// Create NTSC emulator and serialize
	ntscInit, err := NewEmulator(rom, RegionNTSC)
	if err != nil {
		t.Fatalf("NewEmulator NTSC failed: %v", err)
	}
	baseNTSC := &ntscInit

	state, err := baseNTSC.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Create PAL emulator with same ROM
	palInit, err := NewEmulator(rom, RegionPAL)
	if err != nil {
		t.Fatalf("NewEmulator PAL failed: %v", err)
	}
	basePAL := &palInit

	if basePAL.GetRegion() != RegionPAL {
		t.Fatal("Initial region should be PAL")
	}

	// Load NTSC state into PAL emulator
	err = basePAL.Deserialize(state)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	// Region should still be PAL
	if basePAL.GetRegion() != RegionPAL {
		t.Errorf("Region should be preserved as PAL, got %v", basePAL.GetRegion())
	}
}

func TestSerialize_StateIntegrity(t *testing.T) {
	base := createTestEmulator()

	state, err := base.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Check magic bytes
	if string(state[0:12]) != stateMagic {
		t.Errorf("Magic bytes: expected %q, got %q", stateMagic, string(state[0:12]))
	}

	// Check version
	version := binary.LittleEndian.Uint16(state[12:14])
	if version != stateVersion {
		t.Errorf("Version: expected %d, got %d", stateVersion, version)
	}

	// Verify ROM CRC32 matches
	romCRC := binary.LittleEndian.Uint32(state[14:18])
	expectedROMCRC := base.bus.GetROMCRC32()
	if romCRC != expectedROMCRC {
		t.Errorf("ROM CRC32: expected 0x%08X, got 0x%08X", expectedROMCRC, romCRC)
	}

	// Verify data CRC32
	dataCRC := binary.LittleEndian.Uint32(state[18:22])
	calculatedCRC := crc32.ChecksumIEEE(state[stateHeaderSize:])
	if dataCRC != calculatedCRC {
		t.Errorf("Data CRC32: expected 0x%08X, got 0x%08X", calculatedCRC, dataCRC)
	}
}
