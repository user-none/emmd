package emu

import (
	"testing"

	"github.com/user-none/go-chip-m68k"
)

func makeTestZ80Memory() *Z80Memory {
	bus := makeTestBus()
	return NewZ80Memory(bus)
}

func TestZ80Memory_RAMReadWrite(t *testing.T) {
	mem := makeTestZ80Memory()

	mem.Write(0x0000, 0x42)
	if got := mem.Read(0x0000); got != 0x42 {
		t.Errorf("expected 0x42, got 0x%02X", got)
	}

	mem.Write(0x1FFF, 0xAB)
	if got := mem.Read(0x1FFF); got != 0xAB {
		t.Errorf("expected 0xAB, got 0x%02X", got)
	}
}

func TestZ80Memory_RAMMirror(t *testing.T) {
	mem := makeTestZ80Memory()

	// Write to base RAM
	mem.Write(0x0100, 0x55)
	// Read from mirror (0x2000-0x3FFF mirrors 0x0000-0x1FFF)
	if got := mem.Read(0x2100); got != 0x55 {
		t.Errorf("mirror read: expected 0x55, got 0x%02X", got)
	}

	// Write to mirror
	mem.Write(0x2200, 0x77)
	// Read from base
	if got := mem.Read(0x0200); got != 0x77 {
		t.Errorf("base read after mirror write: expected 0x77, got 0x%02X", got)
	}
}

func TestZ80Memory_YM2612Read(t *testing.T) {
	mem := makeTestZ80Memory()

	// YM2612 range (0x4000-0x5FFF) returns 0 on read
	if got := mem.Read(0x4000); got != 0 {
		t.Errorf("YM2612 read at 0x4000: expected 0, got 0x%02X", got)
	}
	if got := mem.Read(0x5FFF); got != 0 {
		t.Errorf("YM2612 read at 0x5FFF: expected 0, got 0x%02X", got)
	}
}

func TestZ80Memory_YM2612Write(t *testing.T) {
	mem := makeTestZ80Memory()

	// YM2612 writes should not panic or affect RAM
	mem.Write(0x4000, 0xFF)
	mem.Write(0x5FFF, 0xFF)

	// Verify RAM wasn't affected
	if got := mem.Read(0x0000); got != 0 {
		t.Errorf("RAM should be unaffected by YM2612 write, got 0x%02X", got)
	}
}

func TestZ80Memory_UnusedRange(t *testing.T) {
	mem := makeTestZ80Memory()

	// 0x6001-0x7EFF returns 0xFF
	if got := mem.Read(0x6001); got != 0xFF {
		t.Errorf("unused range read at 0x6001: expected 0xFF, got 0x%02X", got)
	}
	if got := mem.Read(0x7EFF); got != 0xFF {
		t.Errorf("unused range read at 0x7EFF: expected 0xFF, got 0x%02X", got)
	}
}

func TestZ80Memory_PSGWrite(t *testing.T) {
	mem := makeTestZ80Memory()

	// Z80 writes to $7F11 (VDP PSG port) should reach the PSG.
	// PSG volumes initialize to 0x0F (silent). Write a volume command
	// for channel 0: 0x90 | volume. 0x95 sets ch0 volume to 5.
	mem.Write(0x7F11, 0x95)

	if got := mem.bus.psg.GetVolume(0); got != 5 {
		t.Errorf("PSG volume after Z80 write to $7F11: expected 5, got %d", got)
	}
}

func TestZ80Memory_BankRegisterRead(t *testing.T) {
	mem := makeTestZ80Memory()

	// Bank register at 0x6000 returns 0xFF on read (part of unused range)
	if got := mem.Read(0x6000); got != 0xFF {
		t.Errorf("bank register read: expected 0xFF, got 0x%02X", got)
	}
}

func TestZ80Memory_BankRegisterWrite(t *testing.T) {
	mem := makeTestZ80Memory()

	// The bank register is a 9-bit shift register.
	// Each write to 0x6000 shifts in bit 0 of the value.
	// After 9 writes, the register contains the full 9-bit bank address.
	//
	// Writing bits for bank address 0x1FF (all 1s):
	// Bit 0 of value shifts into bit 8 of register, previous bits shift right.
	for i := 0; i < 9; i++ {
		mem.Write(0x6000, 0x01) // Write bit 1 nine times
	}

	// bankRegister should be 0x1FF
	if mem.bankRegister != 0x1FF {
		t.Errorf("bank register after 9 writes of 1: expected 0x1FF, got 0x%03X", mem.bankRegister)
	}

	// Now write 9 zeros to clear it
	for i := 0; i < 9; i++ {
		mem.Write(0x6000, 0x00)
	}

	if mem.bankRegister != 0x000 {
		t.Errorf("bank register after 9 writes of 0: expected 0x000, got 0x%03X", mem.bankRegister)
	}
}

func TestZ80Memory_BankRegisterSpecificValue(t *testing.T) {
	mem := makeTestZ80Memory()

	// Write bank address 0x100 (bit 8 set, rest 0)
	// We need to write bit 8 first (it will be shifted to position 0 by subsequent writes),
	// then 8 zeros. Actually, the first write goes to bit 8, second shifts that to bit 7
	// and puts new bit at bit 8, etc.
	//
	// To get 0x100 (binary: 1_0000_0000), we write 1 then 8 zeros:
	// Write 1: register = 0x100
	// Write 0: register = 0x080
	// Write 0: register = 0x040
	// ... this doesn't work as expected.
	//
	// The shift register works as: reg = (reg >> 1) | (bit << 8)
	// To load a specific value, write LSB first, MSB last.
	// For 0x100 = binary 1_0000_0000:
	//   Write 0 (bit 0): reg = 0x000
	//   Write 0: reg = 0x000
	//   Write 0: reg = 0x000
	//   Write 0: reg = 0x000
	//   Write 0: reg = 0x000
	//   Write 0: reg = 0x000
	//   Write 0: reg = 0x000
	//   Write 0: reg = 0x000
	//   Write 1 (bit 8): reg = 0x100

	for i := 0; i < 8; i++ {
		mem.Write(0x6000, 0x00)
	}
	mem.Write(0x6000, 0x01)

	if mem.bankRegister != 0x100 {
		t.Errorf("bank register: expected 0x100, got 0x%03X", mem.bankRegister)
	}
}

func TestZ80Memory_BankWindowReadsROM(t *testing.T) {
	mem := makeTestZ80Memory()

	// With bank register = 0, the bank window (0x8000-0xFFFF) maps to
	// M68K addresses 0x000000-0x007FFF which is ROM space.
	// Our test ROM has 0x00 at offset 0 and 0xFF at offset 1.

	if got := mem.Read(0x8000); got != 0x00 {
		t.Errorf("bank window ROM read at 0x8000: expected 0x00, got 0x%02X", got)
	}
	if got := mem.Read(0x8001); got != 0xFF {
		t.Errorf("bank window ROM read at 0x8001: expected 0xFF, got 0x%02X", got)
	}
}

func TestZ80Memory_VDPStatusRead(t *testing.T) {
	mem := makeTestZ80Memory()

	// A fresh NTSC VDP returns 0x7600 from ReadControl:
	//   0x7400 (fixed bits 15:10 = 011101) | 0x0200 (FIFO empty)
	// Even address ($7F04) returns high byte, odd ($7F05) returns low byte.
	hi := mem.Read(0x7F04)
	if hi != 0x76 {
		t.Errorf("VDP status high byte at $7F04: expected 0x76, got 0x%02X", hi)
	}

	// Reading status clears writePending and flags, but result is the same
	// for a fresh VDP with no pending state.
	lo := mem.Read(0x7F05)
	if lo != 0x00 {
		t.Errorf("VDP status low byte at $7F05: expected 0x00, got 0x%02X", lo)
	}
}

func TestZ80Memory_VDPDataWrite(t *testing.T) {
	mem := makeTestZ80Memory()

	// Set up a VRAM write at address 0x0000 via direct VDP control
	// (as a 68K would). word1=0x4000, word2=0x0000 -> VRAM write at addr 0.
	mem.bus.vdp.WriteControl(0, 0x4000)
	mem.bus.vdp.WriteControl(0, 0x0000)

	// Write data byte 0xAB via Z80 data port.
	// Byte is duplicated: 0xAB -> 0xABAB written as a 16-bit word.
	mem.Write(0x7F00, 0xAB)

	if got := mem.bus.vdp.vram[0]; got != 0xAB {
		t.Errorf("VRAM[0] after Z80 data write: expected 0xAB, got 0x%02X", got)
	}
	if got := mem.bus.vdp.vram[1]; got != 0xAB {
		t.Errorf("VRAM[1] after Z80 data write: expected 0xAB, got 0x%02X", got)
	}
}

func TestZ80Memory_VDPDataRead(t *testing.T) {
	mem := makeTestZ80Memory()

	// Pre-populate VRAM with a known value
	mem.bus.vdp.vram[0] = 0xDE
	mem.bus.vdp.vram[1] = 0xAD

	// Set up a VRAM read at address 0x0000 via the control port.
	// Two-word command: word1 = 0x0000 (CD1:0=00, addr=0x0000)
	//                   word2 = 0x0000 (CD5:2=0000, addr bits 15:14=00)
	mem.Write(0x7F04, 0x00) // 0x0000
	mem.Write(0x7F04, 0x00) // 0x0000

	// Read data port. ReadData returns the pre-fetched value.
	// Even address ($7F00) returns high byte.
	hi := mem.Read(0x7F00)
	if hi != 0xDE {
		t.Errorf("VDP data read high byte at $7F00: expected 0xDE, got 0x%02X", hi)
	}
}

func TestZ80Memory_HVCounterRead(t *testing.T) {
	mem := makeTestZ80Memory()

	// ReadHVCounter returns formatHVCounter(hCounter).
	// With fresh VDP: vCounter=0, hCounter=0, non-interlace.
	// Format: V7:V0 | H8:H1 = 0x0000 for both zero.
	hi := mem.Read(0x7F08)
	lo := mem.Read(0x7F09)

	// With zero counters, both bytes should be 0.
	if hi != 0x00 {
		t.Errorf("HV counter high byte at $7F08: expected 0x00, got 0x%02X", hi)
	}
	if lo != 0x00 {
		t.Errorf("HV counter low byte at $7F09: expected 0x00, got 0x%02X", lo)
	}
}

func TestZ80Memory_VDPPSGReadOnly(t *testing.T) {
	mem := makeTestZ80Memory()

	// PSG range ($7F10-$7F17) is write-only; reads return 0xFF.
	if got := mem.Read(0x7F10); got != 0xFF {
		t.Errorf("PSG read at $7F10: expected 0xFF, got 0x%02X", got)
	}
	if got := mem.Read(0x7F11); got != 0xFF {
		t.Errorf("PSG read at $7F11: expected 0xFF, got 0x%02X", got)
	}
}

func TestZ80Memory_VDPReservedRange(t *testing.T) {
	mem := makeTestZ80Memory()

	// $7F20-$7FFF is reserved; reads return 0xFF, writes ignored.
	if got := mem.Read(0x7F20); got != 0xFF {
		t.Errorf("reserved read at $7F20: expected 0xFF, got 0x%02X", got)
	}
	if got := mem.Read(0x7FFF); got != 0xFF {
		t.Errorf("reserved read at $7FFF: expected 0xFF, got 0x%02X", got)
	}

	// Writes to reserved range should not panic
	mem.Write(0x7F20, 0xFF)
	mem.Write(0x7FFF, 0xFF)
}

func TestZ80Memory_BankWindowWritesToRAM(t *testing.T) {
	mem := makeTestZ80Memory()

	// Set bank register to point to M68K RAM area (0xFF0000-0xFFFFFF)
	// Bank register = 0xFF0000 >> 15 = 0x1FE
	// Binary 0x1FE = 1_1111_1110, write LSB first
	bits := []byte{0, 1, 1, 1, 1, 1, 1, 1, 1}
	for _, b := range bits {
		mem.Write(0x6000, b)
	}

	if mem.bankRegister != 0x1FE {
		t.Fatalf("bank register setup: expected 0x1FE, got 0x%03X", mem.bankRegister)
	}

	// Write via bank window to M68K RAM
	mem.Write(0x8000, 0xAA)

	// Verify by reading M68K RAM directly via the bus
	val := mem.bus.ReadCycle(0, m68k.Byte, 0xFF0000)
	if uint8(val) != 0xAA {
		t.Errorf("bank window write to M68K RAM: expected 0xAA, got 0x%02X", val)
	}
}
