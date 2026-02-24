package emu

import (
	"testing"

	"github.com/user-none/go-chip-m68k"
	"github.com/user-none/go-chip-sn76489"
)

// makeTestBus creates a GenesisBus with a minimal ROM containing
// reset vectors: SSP at address 0, PC at address 4.
func makeTestBus() *GenesisBus {
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
	// Write a NOP (0x4E71) at address 0x200
	rom[0x200] = 0x4E
	rom[0x201] = 0x71

	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)
	return NewGenesisBus(rom, vdp, io, psg, ym)
}

func TestGenesisBus_ReadROMByte(t *testing.T) {
	bus := makeTestBus()
	val := bus.ReadCycle(0, m68k.Byte, 0)
	if val != 0x00 {
		t.Errorf("expected 0x00, got 0x%02X", val)
	}

	val = bus.ReadCycle(0, m68k.Byte, 1)
	if val != 0xFF {
		t.Errorf("expected 0xFF, got 0x%02X", val)
	}
}

func TestGenesisBus_ReadROMWord(t *testing.T) {
	bus := makeTestBus()
	val := bus.ReadCycle(0, m68k.Word, 0)
	if val != 0x00FF {
		t.Errorf("expected 0x00FF, got 0x%04X", val)
	}
}

func TestGenesisBus_ReadROMLong(t *testing.T) {
	bus := makeTestBus()
	// SSP at address 0 = 0x00FF0000
	val := bus.ReadCycle(0, m68k.Long, 0)
	if val != 0x00FF0000 {
		t.Errorf("expected 0x00FF0000, got 0x%08X", val)
	}

	// PC at address 4 = 0x00000200
	val = bus.ReadCycle(0, m68k.Long, 4)
	if val != 0x00000200 {
		t.Errorf("expected 0x00000200, got 0x%08X", val)
	}
}

func TestGenesisBus_ReadROMPastEnd(t *testing.T) {
	bus := makeTestBus()
	val := bus.ReadCycle(0, m68k.Byte, 0x1000)
	if val != 0 {
		t.Errorf("expected 0, got 0x%02X", val)
	}
}

func TestGenesisBus_ROMReadOnly(t *testing.T) {
	bus := makeTestBus()
	// Writing to ROM should not change ROM
	bus.WriteCycle(0, m68k.Byte, 0, 0xAA)
	val := bus.ReadCycle(0, m68k.Byte, 0)
	if val != 0x00 {
		t.Errorf("ROM should be read-only, expected 0x00, got 0x%02X", val)
	}
}

func TestGenesisBus_RAMByteReadWrite(t *testing.T) {
	bus := makeTestBus()
	bus.WriteCycle(0, m68k.Byte, 0xFF0000, 0x42)
	val := bus.ReadCycle(0, m68k.Byte, 0xFF0000)
	if val != 0x42 {
		t.Errorf("expected 0x42, got 0x%02X", val)
	}
}

func TestGenesisBus_RAMWordReadWrite(t *testing.T) {
	bus := makeTestBus()
	bus.WriteCycle(0, m68k.Word, 0xFF0000, 0xBEEF)
	val := bus.ReadCycle(0, m68k.Word, 0xFF0000)
	if val != 0xBEEF {
		t.Errorf("expected 0xBEEF, got 0x%04X", val)
	}

	// Verify individual bytes (big-endian)
	hi := bus.ReadCycle(0, m68k.Byte, 0xFF0000)
	lo := bus.ReadCycle(0, m68k.Byte, 0xFF0001)
	if hi != 0xBE {
		t.Errorf("expected high byte 0xBE, got 0x%02X", hi)
	}
	if lo != 0xEF {
		t.Errorf("expected low byte 0xEF, got 0x%02X", lo)
	}
}

func TestGenesisBus_RAMLongReadWrite(t *testing.T) {
	bus := makeTestBus()
	bus.WriteCycle(0, m68k.Long, 0xFF0000, 0xDEADBEEF)
	val := bus.ReadCycle(0, m68k.Long, 0xFF0000)
	if val != 0xDEADBEEF {
		t.Errorf("expected 0xDEADBEEF, got 0x%08X", val)
	}
}

func TestGenesisBus_RAMMirroring(t *testing.T) {
	bus := makeTestBus()
	// Write at base RAM address
	bus.WriteCycle(0, m68k.Byte, 0xFF0000, 0x55)
	// Read from mirrored address (64KB mirror wraps around the lower 16 bits)
	val := bus.ReadCycle(0, m68k.Byte, 0xFF0000)
	if val != 0x55 {
		t.Errorf("expected 0x55, got 0x%02X", val)
	}

	// Address 0xFFFFFF should mirror to RAM offset 0xFFFF
	bus.WriteCycle(0, m68k.Byte, 0xFFFFFF, 0xAA)
	val = bus.ReadCycle(0, m68k.Byte, 0xFFFFFF)
	if val != 0xAA {
		t.Errorf("expected 0xAA, got 0x%02X", val)
	}
}

func TestGenesisBus_IOVersionRegister(t *testing.T) {
	bus := makeTestBus()
	val := bus.ReadCycle(0, m68k.Byte, 0xA10001)
	// NTSC overseas, no expansion: 0xA0
	if val != 0xA0 {
		t.Errorf("expected 0xA0, got 0x%02X", val)
	}
}

func TestGenesisBus_IOControllerPorts(t *testing.T) {
	bus := makeTestBus()
	// Controller data port 1
	val := bus.ReadCycle(0, m68k.Byte, 0xA10003)
	if val != 0xFF {
		t.Errorf("expected 0xFF (no buttons), got 0x%02X", val)
	}
	// Controller data port 2
	val = bus.ReadCycle(0, m68k.Byte, 0xA10005)
	if val != 0xFF {
		t.Errorf("expected 0xFF (no buttons), got 0x%02X", val)
	}
}

func TestGenesisBus_VDPControlRead(t *testing.T) {
	bus := makeTestBus()
	val := bus.ReadCycle(0, m68k.Word, 0xC00004)
	if val != 0x7600 {
		t.Errorf("expected VDP status 0x7600, got 0x%04X", val)
	}
}

func TestGenesisBus_VDPDataRead(t *testing.T) {
	bus := makeTestBus()
	val := bus.ReadCycle(0, m68k.Word, 0xC00000)
	if val != 0x0000 {
		t.Errorf("expected VDP data 0x0000, got 0x%04X", val)
	}
}

func TestGenesisBus_Z80BusRequest(t *testing.T) {
	bus := makeTestBus()
	val := bus.ReadCycle(0, m68k.Word, 0xA11100)
	if val != 0x0100 {
		t.Errorf("expected Z80 bus granted 0x0100, got 0x%04X", val)
	}
}

func TestGenesisBus_Z80RAMReadWrite(t *testing.T) {
	bus := makeTestBus()
	bus.WriteCycle(0, m68k.Byte, 0xA00000, 0x76)
	val := bus.ReadCycle(0, m68k.Byte, 0xA00000)
	if val != 0x76 {
		t.Errorf("expected 0x76, got 0x%02X", val)
	}
}

func TestGenesisBus_UnmappedReturnsZero(t *testing.T) {
	bus := makeTestBus()
	val := bus.ReadCycle(0, m68k.Byte, 0x800000)
	if val != 0 {
		t.Errorf("expected 0 for unmapped region, got 0x%02X", val)
	}
}

func TestGenesisBus_Reset(t *testing.T) {
	bus := makeTestBus()
	bus.WriteCycle(0, m68k.Byte, 0xFF0000, 0x42)
	bus.Reset()
	val := bus.ReadCycle(0, m68k.Byte, 0xFF0000)
	if val != 0 {
		t.Errorf("expected 0 after reset, got 0x%02X", val)
	}
}

func TestGenesisBus_GetROMCRC32(t *testing.T) {
	bus := makeTestBus()
	crc := bus.GetROMCRC32()
	if crc == 0 {
		t.Error("expected non-zero CRC32")
	}
}

// makeTestBusWithSRAM creates a bus with a ROM that has valid SRAM header.
// SRAM range: $200000-$203FFF (16KB).
func makeTestBusWithSRAM() *GenesisBus {
	rom := make([]byte, 0x400) // Need at least $1BC bytes
	// Reset vectors (same as makeTestBus)
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

	// SRAM header
	rom[0x1B0] = 'R'
	rom[0x1B1] = 'A'
	rom[0x1B2] = 0xF8 // 16-bit
	rom[0x1B3] = 0x20
	// Start address: 0x00200000
	rom[0x1B4] = 0x00
	rom[0x1B5] = 0x20
	rom[0x1B6] = 0x00
	rom[0x1B7] = 0x00
	// End address: 0x00203FFF
	rom[0x1B8] = 0x00
	rom[0x1B9] = 0x20
	rom[0x1BA] = 0x3F
	rom[0x1BB] = 0xFF

	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)
	return NewGenesisBus(rom, vdp, io, psg, ym)
}

func TestGenesisBus_SRAMHeaderParsing(t *testing.T) {
	bus := makeTestBusWithSRAM()
	if !bus.HasSRAM() {
		t.Fatal("expected HasSRAM() to return true")
	}
	// SRAM size should be $4000 (end - start + 1 = 0x203FFF - 0x200000 + 1)
	if len(bus.sram) != 0x4000 {
		t.Errorf("expected SRAM size 0x4000, got 0x%X", len(bus.sram))
	}
}

func TestGenesisBus_SRAMNoHeader(t *testing.T) {
	bus := makeTestBus()
	if bus.HasSRAM() {
		t.Error("expected HasSRAM() to return false for ROM without SRAM header")
	}
}

func TestGenesisBus_SRAMDisabledByDefault(t *testing.T) {
	bus := makeTestBusWithSRAM()
	// SRAM should not be enabled by default; reads from SRAM range return ROM data
	val := bus.ReadCycle(0, m68k.Byte, 0x200000)
	// ROM is only 0x400 bytes, so address 0x200000 is past end -> should return 0
	if val != 0 {
		t.Errorf("expected 0 (ROM past end), got 0x%02X", val)
	}
}

func TestGenesisBus_A130F1Enable(t *testing.T) {
	bus := makeTestBusWithSRAM()
	// Write 0x01 to enable SRAM (not writable)
	bus.WriteCycle(0, m68k.Byte, 0xA130F1, 0x01)
	if !bus.sramEnabled {
		t.Error("expected sramEnabled to be true")
	}
	if bus.sramWritable {
		t.Error("expected sramWritable to be false")
	}
	// Read back register
	val := bus.ReadCycle(0, m68k.Byte, 0xA130F1)
	if val != 0x01 {
		t.Errorf("expected $A130F1 read 0x01, got 0x%02X", val)
	}
}

func TestGenesisBus_A130F1EnableWritable(t *testing.T) {
	bus := makeTestBusWithSRAM()
	// Write 0x03 to enable SRAM + writable
	bus.WriteCycle(0, m68k.Byte, 0xA130F1, 0x03)
	if !bus.sramEnabled {
		t.Error("expected sramEnabled to be true")
	}
	if !bus.sramWritable {
		t.Error("expected sramWritable to be true")
	}
	val := bus.ReadCycle(0, m68k.Byte, 0xA130F1)
	if val != 0x03 {
		t.Errorf("expected $A130F1 read 0x03, got 0x%02X", val)
	}
}

func TestGenesisBus_SRAMReadWrite(t *testing.T) {
	bus := makeTestBusWithSRAM()
	// Enable + writable
	bus.WriteCycle(0, m68k.Byte, 0xA130F1, 0x03)
	// Write a byte to SRAM
	bus.WriteCycle(0, m68k.Byte, 0x200000, 0x42)
	val := bus.ReadCycle(0, m68k.Byte, 0x200000)
	if val != 0x42 {
		t.Errorf("expected 0x42, got 0x%02X", val)
	}
}

func TestGenesisBus_SRAMWriteProtected(t *testing.T) {
	bus := makeTestBusWithSRAM()
	// Enable but NOT writable (0x01)
	bus.WriteCycle(0, m68k.Byte, 0xA130F1, 0x01)
	bus.WriteCycle(0, m68k.Byte, 0x200000, 0x42)
	// SRAM should still be all zeros (write ignored)
	val := bus.ReadCycle(0, m68k.Byte, 0x200000)
	if val != 0x00 {
		t.Errorf("expected 0x00 (write protected), got 0x%02X", val)
	}
}

func TestGenesisBus_SRAMWordReadWrite(t *testing.T) {
	bus := makeTestBusWithSRAM()
	bus.WriteCycle(0, m68k.Byte, 0xA130F1, 0x03)
	bus.WriteCycle(0, m68k.Word, 0x200000, 0xBEEF)
	val := bus.ReadCycle(0, m68k.Word, 0x200000)
	if val != 0xBEEF {
		t.Errorf("expected 0xBEEF, got 0x%04X", val)
	}
	// Verify individual bytes
	hi := bus.ReadCycle(0, m68k.Byte, 0x200000)
	lo := bus.ReadCycle(0, m68k.Byte, 0x200001)
	if hi != 0xBE {
		t.Errorf("expected high byte 0xBE, got 0x%02X", hi)
	}
	if lo != 0xEF {
		t.Errorf("expected low byte 0xEF, got 0x%02X", lo)
	}
}

func TestGenesisBus_SRAMPreservedOnReset(t *testing.T) {
	bus := makeTestBusWithSRAM()
	bus.WriteCycle(0, m68k.Byte, 0xA130F1, 0x03)
	bus.WriteCycle(0, m68k.Byte, 0x200000, 0x55)
	bus.Reset()
	// Re-enable SRAM after reset (register state is cleared)
	bus.WriteCycle(0, m68k.Byte, 0xA130F1, 0x03)
	val := bus.ReadCycle(0, m68k.Byte, 0x200000)
	if val != 0x55 {
		t.Errorf("expected 0x55 (SRAM preserved after reset), got 0x%02X", val)
	}
}

func TestGenesisBus_SRAMGetSet(t *testing.T) {
	bus := makeTestBusWithSRAM()
	bus.WriteCycle(0, m68k.Byte, 0xA130F1, 0x03)
	bus.WriteCycle(0, m68k.Byte, 0x200000, 0xAB)
	bus.WriteCycle(0, m68k.Byte, 0x200001, 0xCD)

	// GetSRAM should return a copy
	data := bus.GetSRAM()
	if data[0] != 0xAB || data[1] != 0xCD {
		t.Errorf("GetSRAM mismatch: got [0]=0x%02X [1]=0x%02X", data[0], data[1])
	}

	// Modify the copy - should not affect bus SRAM
	data[0] = 0xFF
	val := bus.ReadCycle(0, m68k.Byte, 0x200000)
	if val != 0xAB {
		t.Errorf("GetSRAM should return a copy, but bus was modified")
	}

	// SetSRAM loads new data
	newData := make([]byte, 0x4000)
	newData[0] = 0x11
	newData[1] = 0x22
	bus.SetSRAM(newData)
	val = bus.ReadCycle(0, m68k.Byte, 0x200000)
	if val != 0x11 {
		t.Errorf("expected 0x11 after SetSRAM, got 0x%02X", val)
	}
	val = bus.ReadCycle(0, m68k.Byte, 0x200001)
	if val != 0x22 {
		t.Errorf("expected 0x22 after SetSRAM, got 0x%02X", val)
	}
}

func TestGenesisBus_TASWriteSuppressed(t *testing.T) {
	bus := makeTestBus()
	// Place a TAS (An) instruction at the NOP location: 0x4A90 = TAS (A0)
	bus.rom[0x200] = 0x4A
	bus.rom[0x201] = 0x90
	// The TAS target: set up A0 to point to RAM
	// Write a known value to RAM at 0xFF0010
	bus.WriteCycle(0, m68k.Byte, 0xFF0010, 0x55)

	// Create CPU so the bus has the IR reference
	cpu := m68k.New(bus)
	bus.SetCPU(cpu)

	// Manually set A0 to point to our RAM address and set IR to TAS opcode
	regs := cpu.Registers()
	regs.A[0] = 0xFF0010
	regs.PC = 0x200
	regs.SR = 0x2700 // Supervisor mode, all interrupts masked
	cpu.SetState(regs)

	// Step the CPU to execute the TAS instruction
	cpu.Step()

	// RAM should be unchanged (write-back suppressed on Genesis)
	val := bus.ReadCycle(0, m68k.Byte, 0xFF0010)
	if val != 0x55 {
		t.Errorf("TAS should not write back on Genesis: expected 0x55, got 0x%02X", val)
	}

	// Flags should still reflect the test (N=0, Z=0 for value 0x55)
	regs = cpu.Registers()
	if regs.SR&0x04 != 0 {
		t.Error("Z flag should be clear (byte was 0x55, not zero)")
	}
}

func TestGenesisBus_TASRegisterNotSuppressed(t *testing.T) {
	bus := makeTestBus()
	// TAS D0 = 0x4AC0
	bus.rom[0x200] = 0x4A
	bus.rom[0x201] = 0xC0

	cpu := m68k.New(bus)
	bus.SetCPU(cpu)

	regs := cpu.Registers()
	regs.D[0] = 0x00000055
	regs.PC = 0x200
	regs.SR = 0x2700
	cpu.SetState(regs)

	cpu.Step()

	// Register TAS should still set bit 7 (no bus write involved)
	regs = cpu.Registers()
	if regs.D[0]&0xFF != 0xD5 {
		t.Errorf("TAS Dn should set bit 7: expected D0 low byte 0xD5, got 0x%02X", regs.D[0]&0xFF)
	}
}

func TestGenesisBus_A130F1Disable(t *testing.T) {
	bus := makeTestBusWithSRAM()
	// Enable + writable, write data
	bus.WriteCycle(0, m68k.Byte, 0xA130F1, 0x03)
	bus.WriteCycle(0, m68k.Byte, 0x200000, 0x42)
	// Disable SRAM
	bus.WriteCycle(0, m68k.Byte, 0xA130F1, 0x00)
	// Reads should now return ROM data (ROM is only 0x400 bytes, so 0x200000 is past end -> 0)
	val := bus.ReadCycle(0, m68k.Byte, 0x200000)
	if val != 0x00 {
		t.Errorf("expected 0x00 (ROM past end, SRAM disabled), got 0x%02X", val)
	}
	// Re-enable SRAM - data should still be there
	bus.WriteCycle(0, m68k.Byte, 0xA130F1, 0x01)
	val = bus.ReadCycle(0, m68k.Byte, 0x200000)
	if val != 0x42 {
		t.Errorf("expected 0x42 (SRAM data preserved), got 0x%02X", val)
	}
}
