package emu

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
//	0xFF0000-0xFFFFFF  68K main RAM (64KB, mirrored)
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

// Read implements m68k.Bus.
func (b *GenesisBus) Read(s m68k.Size, addr uint32) uint32 {
	return b.ReadCycle(0, s, addr)
}

// ReadCycle implements m68k.CycleBus.
func (b *GenesisBus) ReadCycle(cycle uint64, s m68k.Size, addr uint32) uint32 {
	addr &= 0xFFFFFF // 24-bit address bus

	switch {
	case addr < 0x400000:
		if b.sramEnabled && b.sram != nil && addr >= b.sramStart && addr <= b.sramEnd {
			return b.readSRAM(s, addr)
		}
		return b.readROM(s, addr)
	case addr >= 0xA00000 && addr <= 0xA0FFFF:
		return b.readZ80(s, addr)
	case addr >= 0xA10000 && addr <= 0xA1001F:
		return b.readIO(cycle, s, addr)
	case addr >= 0xA11100 && addr <= 0xA11101:
		// Z80 bus request: bit 0 of high byte = 0 means bus granted to 68K
		if b.z80BusRequested {
			// Bus requested: grant immediately (bit 0 = 0)
			return b.readSized(s, 0x00, 0x00)
		}
		// Bus not requested (bit 0 = 1)
		return b.readSized(s, 0x01, 0x00)
	case addr >= 0xA11200 && addr <= 0xA11201:
		// Z80 reset
		return b.readSized(s, 0x00, 0x00)
	case addr >= 0xC00000 && addr <= 0xDFFFFF:
		// VDP is mirrored every 32 bytes in this range.
		// 68K byte reads: even addr -> high byte, odd addr -> low byte.
		port := addr & 0x1F
		switch {
		case port <= 0x03: // Data port
			switch s {
			case m68k.Long:
				hi := uint32(b.vdp.ReadData())
				lo := uint32(b.vdp.ReadData())
				return hi<<16 | lo
			case m68k.Byte:
				val := b.vdp.ReadData()
				if addr&1 == 0 {
					return uint32(val >> 8)
				}
				return uint32(val & 0xFF)
			default:
				return uint32(b.vdp.ReadData())
			}
		case port <= 0x07: // Control/status port
			switch s {
			case m68k.Long:
				hi := uint32(b.vdp.ReadControl(cycle))
				lo := uint32(b.vdp.ReadControl(cycle))
				return hi<<16 | lo
			case m68k.Byte:
				val := b.vdp.ReadControl(cycle)
				if addr&1 == 0 {
					return uint32(val >> 8)
				}
				return uint32(val & 0xFF)
			default:
				return uint32(b.vdp.ReadControl(cycle))
			}
		case port <= 0x0F: // HV counter
			switch s {
			case m68k.Byte:
				val := b.vdp.ReadHVCounterAtCycle(cycle)
				if addr&1 == 0 {
					return uint32(val >> 8)
				}
				return uint32(val & 0xFF)
			default:
				return uint32(b.vdp.ReadHVCounterAtCycle(cycle))
			}
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
	case addr >= 0xFF0000:
		return b.readRAM(s, addr)
	default:
		return 0
	}
}

// Write implements m68k.Bus.
func (b *GenesisBus) Write(s m68k.Size, addr uint32, value uint32) {
	b.WriteCycle(0, s, addr, value)
}

// WriteCycle implements m68k.CycleBus.
func (b *GenesisBus) WriteCycle(cycle uint64, s m68k.Size, addr uint32, value uint32) {
	// Genesis hardware: TAS memory write-back fails because the VDP bus
	// arbiter does not support read-modify-write cycles. Suppress the write.
	if b.isTASWriteBack() {
		return
	}

	addr &= 0xFFFFFF // 24-bit address bus

	switch {
	case addr < 0x400000:
		if b.sramWritable && b.sram != nil && addr >= b.sramStart && addr <= b.sramEnd {
			b.writeSRAM(s, addr, value)
		}
		// Otherwise: ROM, read-only, ignore writes
	case addr >= 0xA00000 && addr <= 0xA0FFFF:
		b.writeZ80(s, addr, value)
	case addr >= 0xA10000 && addr <= 0xA1001F:
		b.writeIO(cycle, s, addr, value)
	case addr >= 0xA11100 && addr <= 0xA11101:
		// Z80 bus request: bit 0 of high byte (0xA11100) controls request
		if s == m68k.Byte {
			if addr == 0xA11100 {
				b.z80BusRequested = value&0x01 != 0
			}
		} else {
			b.z80BusRequested = value&0x0100 != 0
		}
	case addr >= 0xA11200 && addr <= 0xA11201:
		// Z80 reset: writing 0x0000 asserts reset, 0x0100 deasserts
		var newReset bool
		if s == m68k.Byte {
			if addr == 0xA11200 {
				newReset = value&0x01 != 0
			} else {
				newReset = b.z80Reset // low byte write doesn't affect reset
			}
		} else {
			newReset = value&0x0100 != 0
		}
		if !b.z80Reset && newReset {
			b.z80PendingReset = true
		}
		b.z80Reset = newReset
	case addr >= 0xC00000 && addr <= 0xDFFFFF:
		// VDP is mirrored every 32 bytes in this range
		port := addr & 0x1F
		switch {
		case port <= 0x03: // Data port
			if s == m68k.Long {
				b.vdp.WriteData(cycle, uint16(value>>16))
				b.vdp.WriteData(cycle, uint16(value))
			} else {
				b.vdp.WriteData(cycle, uint16(value))
			}
		case port <= 0x07: // Control port
			if s == m68k.Long {
				b.vdp.WriteControl(cycle, uint16(value>>16))
				b.vdp.WriteControl(cycle, uint16(value))
			} else {
				b.vdp.WriteControl(cycle, uint16(value))
			}
		case port >= 0x10 && port < 0x18:
			// PSG write port ($C00011, but responds to $10-$17 range)
			b.psg.Write(byte(value))
		}
	case addr >= 0xA130F0 && addr <= 0xA130FF:
		if addr == 0xA130F1 {
			var v uint8
			if s == m68k.Byte {
				v = uint8(value)
			} else {
				v = uint8(value) // Low byte for word/long writes to odd address
			}
			b.sramEnabled = v&0x01 != 0
			b.sramWritable = v&0x02 != 0
		}
	case addr >= 0xFF0000:
		b.writeRAM(s, addr, value)
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

// readROM reads from ROM with big-endian byte order.
func (b *GenesisBus) readROM(s m68k.Size, addr uint32) uint32 {
	romLen := uint32(len(b.rom))
	switch s {
	case m68k.Byte:
		if addr < romLen {
			return uint32(b.rom[addr])
		}
	case m68k.Word:
		if addr+1 < romLen {
			return uint32(b.rom[addr])<<8 | uint32(b.rom[addr+1])
		}
	case m68k.Long:
		if addr+3 < romLen {
			return uint32(b.rom[addr])<<24 | uint32(b.rom[addr+1])<<16 |
				uint32(b.rom[addr+2])<<8 | uint32(b.rom[addr+3])
		}
	}
	return 0
}

// readRAM reads from main RAM (64KB, mirrored) with big-endian byte order.
func (b *GenesisBus) readRAM(s m68k.Size, addr uint32) uint32 {
	idx := addr & 0xFFFF
	switch s {
	case m68k.Byte:
		return uint32(b.ram[idx])
	case m68k.Word:
		return uint32(b.ram[idx])<<8 | uint32(b.ram[(idx+1)&0xFFFF])
	case m68k.Long:
		return uint32(b.ram[idx])<<24 | uint32(b.ram[(idx+1)&0xFFFF])<<16 |
			uint32(b.ram[(idx+2)&0xFFFF])<<8 | uint32(b.ram[(idx+3)&0xFFFF])
	}
	return 0
}

// writeRAM writes to main RAM (64KB, mirrored) with big-endian byte order.
func (b *GenesisBus) writeRAM(s m68k.Size, addr uint32, value uint32) {
	idx := addr & 0xFFFF
	switch s {
	case m68k.Byte:
		b.ram[idx] = byte(value)
	case m68k.Word:
		b.ram[idx] = byte(value >> 8)
		b.ram[(idx+1)&0xFFFF] = byte(value)
	case m68k.Long:
		b.ram[idx] = byte(value >> 24)
		b.ram[(idx+1)&0xFFFF] = byte(value >> 16)
		b.ram[(idx+2)&0xFFFF] = byte(value >> 8)
		b.ram[(idx+3)&0xFFFF] = byte(value)
	}
}

// readSRAM reads from battery-backed SRAM with big-endian byte order.
func (b *GenesisBus) readSRAM(s m68k.Size, addr uint32) uint32 {
	offset := addr - b.sramStart
	sramLen := uint32(len(b.sram))
	switch s {
	case m68k.Byte:
		if offset < sramLen {
			return uint32(b.sram[offset])
		}
	case m68k.Word:
		if offset+1 < sramLen {
			return uint32(b.sram[offset])<<8 | uint32(b.sram[offset+1])
		} else if offset < sramLen {
			return uint32(b.sram[offset]) << 8
		}
	case m68k.Long:
		var val uint32
		for i := uint32(0); i < 4; i++ {
			if offset+i < sramLen {
				val |= uint32(b.sram[offset+i]) << (24 - i*8)
			}
		}
		return val
	}
	return 0
}

// writeSRAM writes to battery-backed SRAM with big-endian byte order.
func (b *GenesisBus) writeSRAM(s m68k.Size, addr uint32, value uint32) {
	offset := addr - b.sramStart
	sramLen := uint32(len(b.sram))
	switch s {
	case m68k.Byte:
		if offset < sramLen {
			b.sram[offset] = byte(value)
		}
	case m68k.Word:
		if offset < sramLen {
			b.sram[offset] = byte(value >> 8)
		}
		if offset+1 < sramLen {
			b.sram[offset+1] = byte(value)
		}
	case m68k.Long:
		for i := uint32(0); i < 4; i++ {
			if offset+i < sramLen {
				b.sram[offset+i] = byte(value >> (24 - i*8))
			}
		}
	}
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

// readZ80 reads from Z80 address space.
func (b *GenesisBus) readZ80(s m68k.Size, addr uint32) uint32 {
	offset := addr - 0xA00000
	if offset < z80RAMSize {
		switch s {
		case m68k.Byte:
			return uint32(b.z80RAM[offset])
		case m68k.Word:
			if offset+1 < z80RAMSize {
				return uint32(b.z80RAM[offset])<<8 | uint32(b.z80RAM[offset+1])
			}
			return uint32(b.z80RAM[offset]) << 8
		case m68k.Long:
			var val uint32
			for i := uint32(0); i < 4; i++ {
				if offset+i < z80RAMSize {
					val |= uint32(b.z80RAM[offset+i]) << (24 - i*8)
				}
			}
			return val
		}
	} else if offset >= 0x4000 && offset < 0x6000 {
		// YM2612 ports
		port := uint8(offset & 0x03)
		switch s {
		case m68k.Byte:
			return uint32(b.ym2612.ReadPort(port))
		case m68k.Word:
			hi := uint32(b.ym2612.ReadPort(port))
			lo := uint32(b.ym2612.ReadPort(port | 1))
			return hi<<8 | lo
		case m68k.Long:
			b0 := uint32(b.ym2612.ReadPort(port))
			b1 := uint32(b.ym2612.ReadPort(port | 1))
			b2 := uint32(b.ym2612.ReadPort(port | 2))
			b3 := uint32(b.ym2612.ReadPort(port | 3))
			return b0<<24 | b1<<16 | b2<<8 | b3
		}
	}
	return 0
}

// writeZ80 writes to Z80 address space.
func (b *GenesisBus) writeZ80(s m68k.Size, addr uint32, value uint32) {
	offset := addr - 0xA00000
	if offset < z80RAMSize {
		switch s {
		case m68k.Byte:
			b.z80RAM[offset] = byte(value)
		case m68k.Word:
			b.z80RAM[offset] = byte(value >> 8)
			if offset+1 < z80RAMSize {
				b.z80RAM[offset+1] = byte(value)
			}
		case m68k.Long:
			for i := uint32(0); i < 4; i++ {
				if offset+i < z80RAMSize {
					b.z80RAM[offset+i] = byte(value >> (24 - i*8))
				}
			}
		}
	} else if offset >= 0x4000 && offset < 0x6000 {
		// YM2612 ports - games commonly use word writes to set address+data
		// in one operation (high byte = address latch, low byte = data)
		port := uint8(offset & 0x03)
		switch s {
		case m68k.Byte:
			b.ym2612.WritePort(port, byte(value))
		case m68k.Word:
			b.ym2612.WritePort(port, byte(value>>8))
			b.ym2612.WritePort(port|1, byte(value))
		case m68k.Long:
			b.ym2612.WritePort(port, byte(value>>24))
			b.ym2612.WritePort(port|1, byte(value>>16))
			b.ym2612.WritePort(port|2, byte(value>>8))
			b.ym2612.WritePort(port|3, byte(value))
		}
	}
}

// readIO reads from I/O register space. For word/long reads, the value
// is built from consecutive byte registers.
func (b *GenesisBus) readIO(cycle uint64, s m68k.Size, addr uint32) uint32 {
	switch s {
	case m68k.Byte:
		return uint32(b.io.ReadRegister(cycle, addr))
	case m68k.Word:
		return uint32(b.io.ReadRegister(cycle, addr))<<8 | uint32(b.io.ReadRegister(cycle, addr+1))
	case m68k.Long:
		return uint32(b.io.ReadRegister(cycle, addr))<<24 | uint32(b.io.ReadRegister(cycle, addr+1))<<16 |
			uint32(b.io.ReadRegister(cycle, addr+2))<<8 | uint32(b.io.ReadRegister(cycle, addr+3))
	}
	return 0
}

// writeIO writes to I/O register space.
func (b *GenesisBus) writeIO(cycle uint64, s m68k.Size, addr uint32, value uint32) {
	switch s {
	case m68k.Byte:
		b.io.WriteRegister(cycle, addr, byte(value))
	case m68k.Word:
		b.io.WriteRegister(cycle, addr, byte(value>>8))
		b.io.WriteRegister(cycle, addr+1, byte(value))
	case m68k.Long:
		b.io.WriteRegister(cycle, addr, byte(value>>24))
		b.io.WriteRegister(cycle, addr+1, byte(value>>16))
		b.io.WriteRegister(cycle, addr+2, byte(value>>8))
		b.io.WriteRegister(cycle, addr+3, byte(value))
	}
}

// ReadWord reads a 16-bit word from the bus at the given address.
// Used by the VDP for DMA 68K transfers.
func (b *GenesisBus) ReadWord(addr uint32) uint16 {
	val := b.ReadCycle(0, m68k.Word, addr)
	return uint16(val)
}

// readSized returns a 2-byte value as the appropriate size.
func (b *GenesisBus) readSized(s m68k.Size, hi, lo byte) uint32 {
	switch s {
	case m68k.Byte:
		return uint32(hi)
	case m68k.Word:
		return uint32(hi)<<8 | uint32(lo)
	case m68k.Long:
		return uint32(hi)<<24 | uint32(lo)<<16
	}
	return 0
}
