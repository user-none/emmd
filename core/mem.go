package core

import (
	"hash/crc32"

	"github.com/user-none/go-chip-m68k"
	"github.com/user-none/go-chip-sn76489"
)

const (
	mainRAMSize = 0x10000  // 64KB main 68K RAM
	z80RAMSize  = 0x2000   // 8KB Z80 RAM
	maxROMSize  = 0x400000 // 4MB max ROM
)

// GenesisBus implements m68k.Bus with the full Genesis memory map.
//
// Address map (M68K view, 24-bit):
//
//	0x000000-0x3FFFFF  ROM (up to 4MB, read-only)
//	0x200000-0x3FFFFF  SRAM (when enabled via $A130F1, overlays ROM)
//	0xA00000-0xA0FFFF  Z80 address space (0xA00000-0xA01FFF = 8KB Z80 RAM)
//	0xA10000-0xA1001F  I/O registers
//	0xA11100-0xA11101  Z80 bus request
//	0xA11200-0xA11201  Z80 reset
//	0xA130F1           SRAM control register
//	0xC00000-0xC00003  VDP data port
//	0xC00004-0xC00007  VDP control port
//	0xC00008-0xC0000F  VDP HV counter, PSG, debug
//	0xC00011           PSG write port
//	0xE00000-0xFFFFFF  68K main RAM (64KB physical, mirrored every $10000)
type GenesisBus struct {
	rom    []byte
	ram    [mainRAMSize]byte
	z80RAM [z80RAMSize]byte
	romCRC uint32
	vdp    *VDP
	io     *IO
	psg    *sn76489.SN76489
	ym2612 *YM2612

	// SRAM fields
	sram         []byte // Battery-backed SRAM
	sramStart    uint32 // SRAM start address from ROM header
	sramEnd      uint32 // SRAM end address from ROM header
	sramEnabled  bool   // SRAM mapped into address space (vs ROM)
	sramWritable bool   // SRAM is writable (vs read-only)

	z80BusRequested bool
	z80Reset        bool
	z80PendingReset bool // Set when Z80 reset transitions from asserted to deasserted

	// CPU reference for instruction-aware bus behavior (e.g., TAS write suppression)
	cpu *m68k.CPU
}

// NewGenesisBus creates a new GenesisBus with the given ROM, VDP, IO, PSG, and YM2612.
func NewGenesisBus(rom []byte, vdp *VDP, io *IO, psg *sn76489.SN76489, ym2612 *YM2612) *GenesisBus {
	if len(rom) > maxROMSize {
		rom = rom[:maxROMSize]
	}

	bus := &GenesisBus{
		rom:    rom,
		romCRC: crc32.ChecksumIEEE(rom),
		vdp:    vdp,
		io:     io,
		psg:    psg,
		ym2612: ym2612,
	}
	bus.parseSRAMHeader()
	return bus
}

// SetCPU sets the CPU reference for instruction-aware bus behavior.
// Called after CPU creation due to circular construction dependency.
func (b *GenesisBus) SetCPU(cpu *m68k.CPU) {
	b.cpu = cpu
}

// isTASWriteBack returns true if the current 68K instruction is TAS with a
// memory operand. On Genesis hardware, the TAS read-modify-write bus cycle
// does not complete the write-back phase because the VDP bus arbiter does
// not support RMW cycles.
func (b *GenesisBus) isTASWriteBack() bool {
	if b.cpu == nil {
		return false
	}
	ir := b.cpu.Registers().IR
	// TAS opcode: 0100 1010 11MM MRRR (0x4AC0-0x4AFF)
	// Mode 000 = data register (write goes to register, not bus)
	return ir&0xFFC0 == 0x4AC0 && ir&0x0038 != 0
}

// parseSRAMHeader reads the ROM header at $1B0-$1BB for SRAM metadata.
func (b *GenesisBus) parseSRAMHeader() {
	if len(b.rom) < 0x1BC {
		return
	}
	// Check for "RA" signature at $1B0
	if b.rom[0x1B0] != 'R' || b.rom[0x1B1] != 'A' {
		return
	}
	start := uint32(b.rom[0x1B4])<<24 | uint32(b.rom[0x1B5])<<16 |
		uint32(b.rom[0x1B6])<<8 | uint32(b.rom[0x1B7])
	end := uint32(b.rom[0x1B8])<<24 | uint32(b.rom[0x1B9])<<16 |
		uint32(b.rom[0x1BA])<<8 | uint32(b.rom[0x1BB])

	// Validate address range is within SRAM region
	if start < 0x200000 || end < start || end > 0x3FFFFF {
		return
	}

	size := end - start + 1
	b.sram = make([]byte, size)
	b.sramStart = start
	b.sramEnd = end
}

// sramReadByte returns the SRAM byte value, or 0 if out of range.
func (b *GenesisBus) sramReadByte(addr uint32) byte {
	offset := addr - b.sramStart
	if offset < uint32(len(b.sram)) {
		return b.sram[offset]
	}
	return 0
}

// Read8 implements m68k.Bus.
func (b *GenesisBus) Read8(addr uint32) uint8 {
	addr &= 0xFFFFFF
	switch {
	case addr < 0x400000:
		if b.sramEnabled && b.sram != nil && addr >= b.sramStart && addr <= b.sramEnd {
			return b.sramReadByte(addr)
		}
		return b.readROM8(addr)
	case addr >= 0xA00000 && addr <= 0xA0FFFF:
		return b.readZ808(addr)
	case addr >= 0xA10000 && addr <= 0xA1001F:
		return b.io.ReadRegister(b.cpu.Cycles(), addr)
	case addr >= 0xA11100 && addr <= 0xA11101:
		if b.z80BusRequested {
			return 0x00
		}
		return 0x01
	case addr >= 0xA11200 && addr <= 0xA11201:
		return 0x00
	case addr >= 0xC00000 && addr <= 0xDFFFFF:
		port := addr & 0x1F
		switch {
		case port <= 0x03:
			val := b.vdp.ReadData()
			if addr&1 == 0 {
				return uint8(val >> 8)
			}
			return uint8(val)
		case port <= 0x07:
			val := b.vdp.ReadControl(b.cpu.Cycles())
			if addr&1 == 0 {
				return uint8(val >> 8)
			}
			return uint8(val)
		case port <= 0x0F:
			val := b.vdp.ReadHVCounterAtCycle(b.cpu.Cycles())
			if addr&1 == 0 {
				return uint8(val >> 8)
			}
			return uint8(val)
		default:
			return 0
		}
	case addr >= 0xA130F0 && addr <= 0xA130FF:
		if addr == 0xA130F1 {
			var val uint8
			if b.sramEnabled {
				val |= 0x01
			}
			if b.sramWritable {
				val |= 0x02
			}
			return val
		}
		return 0
	case addr >= 0xE00000:
		return b.ram[addr&0xFFFF]
	default:
		return 0
	}
}

// Read16 implements m68k.Bus.
func (b *GenesisBus) Read16(addr uint32) uint16 {
	addr &= 0xFFFFFF
	switch {
	case addr < 0x400000:
		if b.sramEnabled && b.sram != nil && addr >= b.sramStart && addr <= b.sramEnd {
			return uint16(b.sramReadByte(addr))<<8 | uint16(b.sramReadByte(addr+1))
		}
		return b.readROM16(addr)
	case addr >= 0xA00000 && addr <= 0xA0FFFF:
		return b.readZ8016(addr)
	case addr >= 0xA10000 && addr <= 0xA1001F:
		cycle := b.cpu.Cycles()
		return uint16(b.io.ReadRegister(cycle, addr))<<8 | uint16(b.io.ReadRegister(cycle, addr+1))
	case addr >= 0xA11100 && addr <= 0xA11101:
		if b.z80BusRequested {
			return 0x0000
		}
		return 0x0100
	case addr >= 0xA11200 && addr <= 0xA11201:
		return 0x0000
	case addr >= 0xC00000 && addr <= 0xDFFFFF:
		port := addr & 0x1F
		switch {
		case port <= 0x03:
			return b.vdp.ReadData()
		case port <= 0x07:
			return b.vdp.ReadControl(b.cpu.Cycles())
		case port <= 0x0F:
			return b.vdp.ReadHVCounterAtCycle(b.cpu.Cycles())
		default:
			return 0
		}
	case addr >= 0xA130F0 && addr <= 0xA130FF:
		if addr == 0xA130F1 {
			var val uint8
			if b.sramEnabled {
				val |= 0x01
			}
			if b.sramWritable {
				val |= 0x02
			}
			return uint16(val)
		}
		return 0
	case addr >= 0xE00000:
		idx := addr & 0xFFFF
		return uint16(b.ram[idx])<<8 | uint16(b.ram[(idx+1)&0xFFFF])
	default:
		return 0
	}
}

// Read32 implements m68k.Bus.
func (b *GenesisBus) Read32(addr uint32) uint32 {
	addr &= 0xFFFFFF
	switch {
	case addr < 0x400000:
		if b.sramEnabled && b.sram != nil && addr >= b.sramStart && addr <= b.sramEnd {
			return uint32(b.sramReadByte(addr))<<24 | uint32(b.sramReadByte(addr+1))<<16 |
				uint32(b.sramReadByte(addr+2))<<8 | uint32(b.sramReadByte(addr+3))
		}
		return b.readROM32(addr)
	case addr >= 0xA00000 && addr <= 0xA0FFFF:
		return b.readZ8032(addr)
	case addr >= 0xA10000 && addr <= 0xA1001F:
		cycle := b.cpu.Cycles()
		return uint32(b.io.ReadRegister(cycle, addr))<<24 | uint32(b.io.ReadRegister(cycle, addr+1))<<16 |
			uint32(b.io.ReadRegister(cycle, addr+2))<<8 | uint32(b.io.ReadRegister(cycle, addr+3))
	case addr >= 0xA11100 && addr <= 0xA11101:
		if b.z80BusRequested {
			return 0x00000000
		}
		return 0x01000000
	case addr >= 0xA11200 && addr <= 0xA11201:
		return 0x00000000
	case addr >= 0xC00000 && addr <= 0xDFFFFF:
		port := addr & 0x1F
		switch {
		case port <= 0x03:
			hi := uint32(b.vdp.ReadData())
			lo := uint32(b.vdp.ReadData())
			return hi<<16 | lo
		case port <= 0x07:
			cycle := b.cpu.Cycles()
			hi := uint32(b.vdp.ReadControl(cycle))
			lo := uint32(b.vdp.ReadControl(cycle))
			return hi<<16 | lo
		case port <= 0x0F:
			return uint32(b.vdp.ReadHVCounterAtCycle(b.cpu.Cycles()))
		default:
			return 0
		}
	case addr >= 0xA130F0 && addr <= 0xA130FF:
		if addr == 0xA130F1 {
			var val uint8
			if b.sramEnabled {
				val |= 0x01
			}
			if b.sramWritable {
				val |= 0x02
			}
			return uint32(val)
		}
		return 0
	case addr >= 0xE00000:
		idx := addr & 0xFFFF
		return uint32(b.ram[idx])<<24 | uint32(b.ram[(idx+1)&0xFFFF])<<16 |
			uint32(b.ram[(idx+2)&0xFFFF])<<8 | uint32(b.ram[(idx+3)&0xFFFF])
	default:
		return 0
	}
}

// Write8 implements m68k.Bus.
// Genesis hardware: TAS memory write-back fails because the VDP bus
// arbiter does not support read-modify-write cycles. Suppress the write.
func (b *GenesisBus) Write8(addr uint32, val uint8) {
	if b.isTASWriteBack() {
		return
	}

	addr &= 0xFFFFFF
	switch {
	case addr < 0x400000:
		if b.sramWritable && b.sram != nil && addr >= b.sramStart && addr <= b.sramEnd {
			offset := addr - b.sramStart
			if offset < uint32(len(b.sram)) {
				b.sram[offset] = val
			}
		}
	case addr >= 0xA00000 && addr <= 0xA0FFFF:
		b.writeZ808(addr, val)
	case addr >= 0xA10000 && addr <= 0xA1001F:
		b.io.WriteRegister(b.cpu.Cycles(), addr, val)
	case addr >= 0xA11100 && addr <= 0xA11101:
		if addr == 0xA11100 {
			b.z80BusRequested = val&0x01 != 0
		}
	case addr >= 0xA11200 && addr <= 0xA11201:
		var newReset bool
		if addr == 0xA11200 {
			newReset = val&0x01 != 0
		} else {
			newReset = b.z80Reset
		}
		if !b.z80Reset && newReset {
			b.z80PendingReset = true
		}
		b.z80Reset = newReset
	case addr >= 0xC00000 && addr <= 0xDFFFFF:
		port := addr & 0x1F
		cycle := b.cpu.Cycles()
		switch {
		case port <= 0x03:
			b.vdp.WriteData(cycle, uint16(val))
		case port <= 0x07:
			b.vdp.WriteControl(cycle, uint16(val))
		case port >= 0x10 && port < 0x18:
			b.psg.Write(val)
		}
	case addr >= 0xA130F0 && addr <= 0xA130FF:
		if addr == 0xA130F1 {
			b.sramEnabled = val&0x01 != 0
			b.sramWritable = val&0x02 != 0
		}
	case addr >= 0xE00000:
		b.ram[addr&0xFFFF] = val
	}
}

// Write16 implements m68k.Bus.
func (b *GenesisBus) Write16(addr uint32, val uint16) {
	addr &= 0xFFFFFF
	switch {
	case addr < 0x400000:
		if b.sramWritable && b.sram != nil && addr >= b.sramStart && addr <= b.sramEnd {
			offset := addr - b.sramStart
			sramLen := uint32(len(b.sram))
			if offset < sramLen {
				b.sram[offset] = byte(val >> 8)
			}
			if offset+1 < sramLen {
				b.sram[offset+1] = byte(val)
			}
		}
	case addr >= 0xA00000 && addr <= 0xA0FFFF:
		b.writeZ8016(addr, val)
	case addr >= 0xA10000 && addr <= 0xA1001F:
		cycle := b.cpu.Cycles()
		b.io.WriteRegister(cycle, addr, byte(val>>8))
		b.io.WriteRegister(cycle, addr+1, byte(val))
	case addr >= 0xA11100 && addr <= 0xA11101:
		b.z80BusRequested = val&0x0100 != 0
	case addr >= 0xA11200 && addr <= 0xA11201:
		newReset := val&0x0100 != 0
		if !b.z80Reset && newReset {
			b.z80PendingReset = true
		}
		b.z80Reset = newReset
	case addr >= 0xC00000 && addr <= 0xDFFFFF:
		port := addr & 0x1F
		cycle := b.cpu.Cycles()
		switch {
		case port <= 0x03:
			b.vdp.WriteData(cycle, val)
		case port <= 0x07:
			b.vdp.WriteControl(cycle, val)
		case port >= 0x10 && port < 0x18:
			b.psg.Write(byte(val))
		}
	case addr >= 0xA130F0 && addr <= 0xA130FF:
		if addr == 0xA130F1 {
			v := uint8(val)
			b.sramEnabled = v&0x01 != 0
			b.sramWritable = v&0x02 != 0
		}
	case addr >= 0xE00000:
		idx := addr & 0xFFFF
		b.ram[idx] = byte(val >> 8)
		b.ram[(idx+1)&0xFFFF] = byte(val)
	}
}

// Write32 implements m68k.Bus.
func (b *GenesisBus) Write32(addr uint32, val uint32) {
	addr &= 0xFFFFFF
	switch {
	case addr < 0x400000:
		if b.sramWritable && b.sram != nil && addr >= b.sramStart && addr <= b.sramEnd {
			offset := addr - b.sramStart
			sramLen := uint32(len(b.sram))
			for i := uint32(0); i < 4; i++ {
				if offset+i < sramLen {
					b.sram[offset+i] = byte(val >> (24 - i*8))
				}
			}
		}
	case addr >= 0xA00000 && addr <= 0xA0FFFF:
		b.writeZ8032(addr, val)
	case addr >= 0xA10000 && addr <= 0xA1001F:
		cycle := b.cpu.Cycles()
		b.io.WriteRegister(cycle, addr, byte(val>>24))
		b.io.WriteRegister(cycle, addr+1, byte(val>>16))
		b.io.WriteRegister(cycle, addr+2, byte(val>>8))
		b.io.WriteRegister(cycle, addr+3, byte(val))
	case addr >= 0xA11100 && addr <= 0xA11101:
		b.z80BusRequested = val&0x0100 != 0
	case addr >= 0xA11200 && addr <= 0xA11201:
		newReset := val&0x0100 != 0
		if !b.z80Reset && newReset {
			b.z80PendingReset = true
		}
		b.z80Reset = newReset
	case addr >= 0xC00000 && addr <= 0xDFFFFF:
		port := addr & 0x1F
		cycle := b.cpu.Cycles()
		switch {
		case port <= 0x03:
			b.vdp.WriteData(cycle, uint16(val>>16))
			b.vdp.WriteData(cycle, uint16(val))
		case port <= 0x07:
			b.vdp.WriteControl(cycle, uint16(val>>16))
			b.vdp.WriteControl(cycle, uint16(val))
		case port >= 0x10 && port < 0x18:
			b.psg.Write(byte(val))
		}
	case addr >= 0xA130F0 && addr <= 0xA130FF:
		if addr == 0xA130F1 {
			v := uint8(val)
			b.sramEnabled = v&0x01 != 0
			b.sramWritable = v&0x02 != 0
		}
	case addr >= 0xE00000:
		idx := addr & 0xFFFF
		b.ram[idx] = byte(val >> 24)
		b.ram[(idx+1)&0xFFFF] = byte(val >> 16)
		b.ram[(idx+2)&0xFFFF] = byte(val >> 8)
		b.ram[(idx+3)&0xFFFF] = byte(val)
	}
}

// Reset clears RAM. Implements m68k.Bus.
func (b *GenesisBus) Reset() {
	b.ram = [mainRAMSize]byte{}
	b.z80RAM = [z80RAMSize]byte{}
}

// GetROMCRC32 returns the CRC32 of the loaded ROM.
func (b *GenesisBus) GetROMCRC32() uint32 {
	return b.romCRC
}

// readROM8 reads a byte from ROM.
func (b *GenesisBus) readROM8(addr uint32) uint8 {
	if addr < uint32(len(b.rom)) {
		return b.rom[addr]
	}
	return 0
}

// readROM16 reads a big-endian word from ROM.
func (b *GenesisBus) readROM16(addr uint32) uint16 {
	romLen := uint32(len(b.rom))
	if addr+1 < romLen {
		return uint16(b.rom[addr])<<8 | uint16(b.rom[addr+1])
	}
	return 0
}

// readROM32 reads a big-endian long from ROM.
func (b *GenesisBus) readROM32(addr uint32) uint32 {
	romLen := uint32(len(b.rom))
	if addr+3 < romLen {
		return uint32(b.rom[addr])<<24 | uint32(b.rom[addr+1])<<16 |
			uint32(b.rom[addr+2])<<8 | uint32(b.rom[addr+3])
	}
	return 0
}

// HasSRAM returns true if the ROM declares battery-backed SRAM.
func (b *GenesisBus) HasSRAM() bool {
	return b.sram != nil
}

// GetSRAM returns a copy of the SRAM contents.
func (b *GenesisBus) GetSRAM() []byte {
	if b.sram == nil {
		return nil
	}
	out := make([]byte, len(b.sram))
	copy(out, b.sram)
	return out
}

// SetSRAM loads SRAM contents (e.g. from a save file).
func (b *GenesisBus) SetSRAM(data []byte) {
	if b.sram == nil {
		return
	}
	copy(b.sram, data)
}

// readZ808 reads a byte from Z80 address space.
func (b *GenesisBus) readZ808(addr uint32) uint8 {
	offset := addr - 0xA00000
	if offset < z80RAMSize {
		return b.z80RAM[offset]
	}
	if offset >= 0x4000 && offset < 0x6000 {
		return b.ym2612.ReadPort(uint8(offset & 0x03))
	}
	return 0
}

// readZ8016 reads a big-endian word from Z80 address space.
func (b *GenesisBus) readZ8016(addr uint32) uint16 {
	offset := addr - 0xA00000
	if offset < z80RAMSize {
		hi := uint16(b.z80RAM[offset])
		var lo uint16
		if offset+1 < z80RAMSize {
			lo = uint16(b.z80RAM[offset+1])
		}
		return hi<<8 | lo
	}
	if offset >= 0x4000 && offset < 0x6000 {
		port := uint8(offset & 0x03)
		return uint16(b.ym2612.ReadPort(port))<<8 | uint16(b.ym2612.ReadPort(port|1))
	}
	return 0
}

// readZ8032 reads a big-endian long from Z80 address space.
func (b *GenesisBus) readZ8032(addr uint32) uint32 {
	offset := addr - 0xA00000
	if offset < z80RAMSize {
		var val uint32
		for i := uint32(0); i < 4; i++ {
			if offset+i < z80RAMSize {
				val |= uint32(b.z80RAM[offset+i]) << (24 - i*8)
			}
		}
		return val
	}
	if offset >= 0x4000 && offset < 0x6000 {
		port := uint8(offset & 0x03)
		return uint32(b.ym2612.ReadPort(port))<<24 | uint32(b.ym2612.ReadPort(port|1))<<16 |
			uint32(b.ym2612.ReadPort(port|2))<<8 | uint32(b.ym2612.ReadPort(port|3))
	}
	return 0
}

// writeZ808 writes a byte to Z80 address space.
func (b *GenesisBus) writeZ808(addr uint32, val uint8) {
	offset := addr - 0xA00000
	if offset < z80RAMSize {
		b.z80RAM[offset] = val
	} else if offset >= 0x4000 && offset < 0x6000 {
		b.ym2612.WritePort(uint8(offset&0x03), val)
	}
}

// writeZ8016 writes a big-endian word to Z80 address space.
func (b *GenesisBus) writeZ8016(addr uint32, val uint16) {
	offset := addr - 0xA00000
	if offset < z80RAMSize {
		b.z80RAM[offset] = byte(val >> 8)
		if offset+1 < z80RAMSize {
			b.z80RAM[offset+1] = byte(val)
		}
	} else if offset >= 0x4000 && offset < 0x6000 {
		port := uint8(offset & 0x03)
		b.ym2612.WritePort(port, byte(val>>8))
		b.ym2612.WritePort(port|1, byte(val))
	}
}

// writeZ8032 writes a big-endian long to Z80 address space.
func (b *GenesisBus) writeZ8032(addr uint32, val uint32) {
	offset := addr - 0xA00000
	if offset < z80RAMSize {
		for i := uint32(0); i < 4; i++ {
			if offset+i < z80RAMSize {
				b.z80RAM[offset+i] = byte(val >> (24 - i*8))
			}
		}
	} else if offset >= 0x4000 && offset < 0x6000 {
		port := uint8(offset & 0x03)
		b.ym2612.WritePort(port, byte(val>>24))
		b.ym2612.WritePort(port|1, byte(val>>16))
		b.ym2612.WritePort(port|2, byte(val>>8))
		b.ym2612.WritePort(port|3, byte(val))
	}
}

// ReadWord reads a 16-bit word from the bus at the given address.
// Used by the VDP for DMA 68K transfers.
func (b *GenesisBus) ReadWord(addr uint32) uint16 {
	return b.Read16(addr)
}
