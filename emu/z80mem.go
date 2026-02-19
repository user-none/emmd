package emu

import "github.com/user-none/go-chip-m68k"

// Z80Memory implements z80.Bus for the Genesis Z80 address space.
//
// Genesis Z80 memory map (16-bit):
//
//	0x0000-0x1FFF  Z80 RAM (8KB)
//	0x2000-0x3FFF  Z80 RAM mirror
//	0x4000-0x5FFF  YM2612 ports
//	0x6000         Bank register (write-only, bit-by-bit)
//	0x6001-0x7EFF  Unused (reads return 0xFF)
//	0x7F00-0x7F1F  VDP ports (data, control, HV counter, PSG)
//	0x7F20-0x7FFF  Reserved
//	0x8000-0xFFFF  M68K bank window (32KB via bank register)
type Z80Memory struct {
	bus          *GenesisBus
	bankRegister uint16 // 9-bit shift register for M68K bank address
}

// NewZ80Memory creates a Z80Memory connected to the given GenesisBus.
func NewZ80Memory(bus *GenesisBus) *Z80Memory {
	return &Z80Memory{bus: bus}
}

// Fetch reads an opcode byte during an M1 cycle. On the Genesis there is
// no M1-specific behavior, so this delegates to Read.
func (m *Z80Memory) Fetch(addr uint16) uint8 {
	return m.Read(addr)
}

// Read reads a byte from the Genesis Z80 address space.
func (m *Z80Memory) Read(addr uint16) uint8 {
	switch {
	case addr < 0x4000:
		// Z80 RAM (0x0000-0x1FFF) and mirror (0x2000-0x3FFF)
		return m.bus.z80RAM[addr&0x1FFF]
	case addr < 0x6000:
		// YM2612 ports
		return m.bus.ym2612.ReadPort(uint8((addr - 0x4000) & 0x03))
	case addr >= 0x7F00 && addr < 0x7F20:
		// VDP ports (0x7F00-0x7F1F): same layout as 68K $C00000-$C0001F.
		// Even address returns high byte, odd returns low byte.
		port := addr & 0x1F
		switch {
		case port <= 0x03: // VDP data port
			val := m.bus.vdp.ReadData()
			if addr&1 == 0 {
				return uint8(val >> 8)
			}
			return uint8(val)
		case port <= 0x07: // VDP control/status port
			val := m.bus.vdp.ReadControl(0)
			if addr&1 == 0 {
				return uint8(val >> 8)
			}
			return uint8(val)
		case port <= 0x0F: // HV counter (read-only)
			val := m.bus.vdp.ReadHVCounter()
			if addr&1 == 0 {
				return uint8(val >> 8)
			}
			return uint8(val)
		default: // PSG ($10-$17) and debug ($18-$1F) are write-only
			return 0xFF
		}
	case addr < 0x8000:
		// Bank register (0x6000), unused (0x6001-0x7EFF), reserved (0x7F20-0x7FFF)
		return 0xFF
	default:
		// M68K bank window (0x8000-0xFFFF)
		m68kAddr := (uint32(m.bankRegister) << 15) | uint32(addr&0x7FFF)
		val := m.bus.ReadCycle(0, m68k.Byte, m68kAddr)
		return uint8(val)
	}
}

// Write writes a byte to the Genesis Z80 address space.
func (m *Z80Memory) Write(addr uint16, val uint8) {
	switch {
	case addr < 0x4000:
		// Z80 RAM (0x0000-0x1FFF) and mirror (0x2000-0x3FFF)
		m.bus.z80RAM[addr&0x1FFF] = val
	case addr < 0x6000:
		// YM2612 ports
		m.bus.ym2612.WritePort(uint8((addr-0x4000)&0x03), val)
	case addr == 0x6000:
		// Bank register: shift in bit 0, 9 bits total
		m.bankRegister = (m.bankRegister >> 1) | (uint16(val&1) << 8)
	case addr >= 0x7F00 && addr < 0x7F20:
		// VDP ports (0x7F00-0x7F1F): same layout as 68K $C00000-$C0001F.
		// Z80 byte is duplicated across both halves of the 16-bit word.
		port := addr & 0x1F
		word := uint16(val)<<8 | uint16(val)
		switch {
		case port <= 0x03: // VDP data port
			m.bus.vdp.WriteData(0, word)
		case port <= 0x07: // VDP control port
			m.bus.vdp.WriteControl(0, word)
		case port >= 0x10 && port < 0x18: // PSG write port
			m.bus.psg.Write(val)
		}
	case addr < 0x8000:
		// Unused (0x6001-0x7EFF) and reserved (0x7F20-0x7FFF): ignore writes
	default:
		// M68K bank window (0x8000-0xFFFF)
		m68kAddr := (uint32(m.bankRegister) << 15) | uint32(addr&0x7FFF)
		m.bus.WriteCycle(0, m68k.Byte, m68kAddr, uint32(val))
	}
}

// In reads from an I/O port. The Genesis Z80 has no I/O ports;
// all peripherals are memory-mapped. Returns 0xFF for all ports.
func (m *Z80Memory) In(port uint16) uint8 {
	return 0xFF
}

// Out writes to an I/O port. No-op on the Genesis Z80.
func (m *Z80Memory) Out(port uint16, val uint8) {}
