package emu

import (
	"testing"

	"github.com/user-none/go-chip-z80"
)

func TestZ80_Creation(t *testing.T) {
	mem := makeTestZ80Memory()
	cpu := z80.New(mem)

	if cpu == nil {
		t.Fatal("z80.New returned nil")
	}
}

func TestZ80_StepReturnsCycles(t *testing.T) {
	mem := makeTestZ80Memory()
	cpu := z80.New(mem)

	// PC starts at 0, Z80 RAM is all zeros.
	// Opcode 0x00 = NOP, which takes 4 T-states.
	cycles := cpu.Step()
	if cycles != 4 {
		t.Errorf("NOP should return 4 cycles, got %d", cycles)
	}
}

func TestZ80_CycleAccumulation(t *testing.T) {
	mem := makeTestZ80Memory()
	cpu := z80.New(mem)

	// Execute several NOPs and verify total cycles
	totalCycles := 0
	for i := 0; i < 10; i++ {
		totalCycles += cpu.Step()
	}

	if totalCycles != 40 {
		t.Errorf("10 NOPs should be 40 cycles, got %d", totalCycles)
	}
}

func TestZ80_GetPC(t *testing.T) {
	mem := makeTestZ80Memory()
	cpu := z80.New(mem)

	if cpu.Registers().PC != 0 {
		t.Errorf("initial PC should be 0, got 0x%04X", cpu.Registers().PC)
	}

	// Execute NOP, PC should advance by 1
	cpu.Step()
	if cpu.Registers().PC != 1 {
		t.Errorf("PC after NOP should be 1, got 0x%04X", cpu.Registers().PC)
	}
}

func TestZ80_GetIFF1(t *testing.T) {
	mem := makeTestZ80Memory()
	cpu := z80.New(mem)

	// IFF1 starts false
	if cpu.Registers().IFF1 {
		t.Error("IFF1 should start false")
	}
}

func TestZ80_GetIM(t *testing.T) {
	mem := makeTestZ80Memory()
	cpu := z80.New(mem)

	// IM starts at 0
	if cpu.Registers().IM != 0 {
		t.Errorf("IM should start at 0, got %d", cpu.Registers().IM)
	}
}

func TestZ80_InterruptBehavior(t *testing.T) {
	mem := makeTestZ80Memory()
	cpu := z80.New(mem)

	// Assert INT line with IM1 data
	cpu.INT(true, 0xFF)

	// With IFF1=false (default after reset), INT should not be serviced.
	// Step should execute NOP at PC=0 normally.
	cpu.Step()
	if cpu.Registers().PC != 1 {
		t.Errorf("INT with IFF1=false should not be serviced, PC expected 1, got 0x%04X", cpu.Registers().PC)
	}

	// Clear INT
	cpu.INT(false, 0)
}

func TestZ80_HaltBurnsCycles(t *testing.T) {
	mem := makeTestZ80Memory()
	cpu := z80.New(mem)

	// Write HALT (0x76) at address 0
	mem.Write(0x0000, 0x76)

	// Execute HALT
	cycles := cpu.Step()
	if cycles != 4 {
		t.Errorf("HALT should return 4 cycles, got %d", cycles)
	}

	// After HALT, stepping should return 4 (burning NOP cycles)
	cycles = cpu.Step()
	if cycles != 4 {
		t.Errorf("halted step should return 4 cycles, got %d", cycles)
	}

	// CPU should be halted
	if !cpu.Halted() {
		t.Error("CPU should be halted after executing HALT")
	}
}

func TestZ80_LDImmediate(t *testing.T) {
	mem := makeTestZ80Memory()
	cpu := z80.New(mem)

	// LD A, 0x42 = opcode 0x3E, immediate 0x42
	mem.Write(0x0000, 0x3E)
	mem.Write(0x0001, 0x42)

	cycles := cpu.Step()
	if cycles != 7 {
		t.Errorf("LD A,n should return 7 cycles, got %d", cycles)
	}

	// Verify A register was loaded (A is high byte of AF)
	a := uint8(cpu.Registers().AF >> 8)
	if a != 0x42 {
		t.Errorf("A register should be 0x42, got 0x%02X", a)
	}
}
