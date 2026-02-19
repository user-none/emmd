package emu

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"math"

	"github.com/user-none/go-chip-m68k"
	"github.com/user-none/go-chip-sn76489"
	"github.com/user-none/go-chip-z80"
)

// Save state format constants
const (
	stateVersion    = 1
	stateMagic      = "eMMDSState\x00\x00"
	stateHeaderSize = 22 // magic(12) + version(2) + romCRC(4) + dataCRC(4)
)

// Fixed serialization sizes for inline components
const (
	busSerializeFixedSize     = mainRAMSize + z80RAMSize + 4 + 5 // ram + z80RAM + sramLen + flags
	z80MemSerializeSize       = 2                                // bankRegister
	emulatorBaseSerializeSize = 17                               // z80IntPending(1) + filterPrevL(8) + filterPrevR(8)
)

// boolByte converts a bool to a uint8 (0 or 1).
func boolByte(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

// SerializeSize returns the total size in bytes needed for a save state.
// SRAM length is variable per ROM, so this is a method on EmulatorBase.
func (e *EmulatorBase) SerializeSize() int {
	sramLen := len(e.bus.sram)
	return stateHeaderSize +
		m68k.SerializeSize +
		z80.SerializeSize +
		busSerializeFixedSize + sramLen +
		z80MemSerializeSize +
		VDPSerializeSize +
		YM2612SerializeSize +
		sn76489.SerializeSize +
		IOSerializeSize +
		emulatorBaseSerializeSize
}

// Serialize creates a save state and returns it as a byte slice.
func (e *EmulatorBase) Serialize() ([]byte, error) {
	size := e.SerializeSize()
	data := make([]byte, size)

	// Write header
	copy(data[0:12], stateMagic)
	binary.LittleEndian.PutUint16(data[12:14], stateVersion)
	binary.LittleEndian.PutUint32(data[14:18], e.bus.romCRC)

	offset := stateHeaderSize

	// M68K CPU
	if err := e.m68k.Serialize(data[offset:]); err != nil {
		return nil, err
	}
	offset += m68k.SerializeSize

	// Z80 CPU
	if err := e.z80.Serialize(data[offset:]); err != nil {
		return nil, err
	}
	offset += z80.SerializeSize

	// GenesisBus
	offset = e.serializeBus(data, offset)

	// Z80Memory
	offset = e.serializeZ80Mem(data, offset)

	// VDP
	if err := e.vdp.Serialize(data[offset:]); err != nil {
		return nil, err
	}
	offset += VDPSerializeSize

	// YM2612
	if err := e.ym2612.Serialize(data[offset:]); err != nil {
		return nil, err
	}
	offset += YM2612SerializeSize

	// PSG
	if err := e.psg.Serialize(data[offset:]); err != nil {
		return nil, err
	}
	offset += sn76489.SerializeSize

	// IO
	if err := e.io.Serialize(data[offset:]); err != nil {
		return nil, err
	}
	offset += IOSerializeSize

	// EmulatorBase inline state
	e.serializeBase(data, offset)

	// Calculate and write data CRC32 (over everything after header)
	dataCRC := crc32.ChecksumIEEE(data[stateHeaderSize:])
	binary.LittleEndian.PutUint32(data[18:22], dataCRC)

	return data, nil
}

// Deserialize restores emulator state from a save state byte slice.
// Region is NOT restored - the current region setting is preserved.
func (e *EmulatorBase) Deserialize(data []byte) error {
	if err := e.VerifyState(data); err != nil {
		return err
	}

	offset := stateHeaderSize

	// M68K CPU
	if err := e.m68k.Deserialize(data[offset:]); err != nil {
		return err
	}
	offset += m68k.SerializeSize

	// Z80 CPU
	if err := e.z80.Deserialize(data[offset:]); err != nil {
		return err
	}
	offset += z80.SerializeSize

	// GenesisBus
	offset = e.deserializeBus(data, offset)

	// Z80Memory
	offset = e.deserializeZ80Mem(data, offset)

	// VDP
	if err := e.vdp.Deserialize(data[offset:]); err != nil {
		return err
	}
	offset += VDPSerializeSize

	// YM2612
	if err := e.ym2612.Deserialize(data[offset:]); err != nil {
		return err
	}
	offset += YM2612SerializeSize

	// PSG
	if err := e.psg.Deserialize(data[offset:]); err != nil {
		return err
	}
	offset += sn76489.SerializeSize

	// IO
	if err := e.io.Deserialize(data[offset:]); err != nil {
		return err
	}
	offset += IOSerializeSize

	// EmulatorBase inline state
	e.deserializeBase(data, offset)

	return nil
}

// VerifyState checks if a save state is valid without loading it.
func (e *EmulatorBase) VerifyState(data []byte) error {
	expectedSize := e.SerializeSize()
	if len(data) < expectedSize {
		return errors.New("save state too short")
	}

	if string(data[0:12]) != stateMagic {
		return errors.New("invalid save state magic")
	}

	version := binary.LittleEndian.Uint16(data[12:14])
	if version > stateVersion {
		return errors.New("unsupported save state version")
	}

	romCRC := binary.LittleEndian.Uint32(data[14:18])
	if romCRC != e.bus.romCRC {
		return errors.New("save state is for a different ROM")
	}

	expectedCRC := binary.LittleEndian.Uint32(data[18:22])
	actualCRC := crc32.ChecksumIEEE(data[stateHeaderSize:])
	if expectedCRC != actualCRC {
		return errors.New("save state data is corrupted")
	}

	return nil
}

// serializeBus writes GenesisBus state to the data buffer.
func (e *EmulatorBase) serializeBus(data []byte, offset int) int {
	// Main RAM (64KB)
	copy(data[offset:], e.bus.ram[:])
	offset += mainRAMSize

	// Z80 RAM (8KB)
	copy(data[offset:], e.bus.z80RAM[:])
	offset += z80RAMSize

	// SRAM length (4 bytes) + SRAM data
	sramLen := uint32(len(e.bus.sram))
	binary.LittleEndian.PutUint32(data[offset:], sramLen)
	offset += 4
	if sramLen > 0 {
		copy(data[offset:], e.bus.sram)
		offset += int(sramLen)
	}

	// Flags
	data[offset] = boolByte(e.bus.sramEnabled)
	offset++
	data[offset] = boolByte(e.bus.sramWritable)
	offset++
	data[offset] = boolByte(e.bus.z80BusRequested)
	offset++
	data[offset] = boolByte(e.bus.z80Reset)
	offset++
	data[offset] = boolByte(e.bus.z80PendingReset)
	offset++

	return offset
}

// deserializeBus reads GenesisBus state from the data buffer.
func (e *EmulatorBase) deserializeBus(data []byte, offset int) int {
	// Main RAM (64KB)
	copy(e.bus.ram[:], data[offset:offset+mainRAMSize])
	offset += mainRAMSize

	// Z80 RAM (8KB)
	copy(e.bus.z80RAM[:], data[offset:offset+z80RAMSize])
	offset += z80RAMSize

	// SRAM length + SRAM data
	sramLen := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	if sramLen > 0 && e.bus.sram != nil {
		copy(e.bus.sram, data[offset:offset+int(sramLen)])
	}
	offset += int(sramLen)

	// Flags
	e.bus.sramEnabled = data[offset] != 0
	offset++
	e.bus.sramWritable = data[offset] != 0
	offset++
	e.bus.z80BusRequested = data[offset] != 0
	offset++
	e.bus.z80Reset = data[offset] != 0
	offset++
	e.bus.z80PendingReset = data[offset] != 0
	offset++

	return offset
}

// serializeZ80Mem writes Z80Memory state to the data buffer.
func (e *EmulatorBase) serializeZ80Mem(data []byte, offset int) int {
	binary.LittleEndian.PutUint16(data[offset:], e.z80Mem.bankRegister)
	offset += 2
	return offset
}

// deserializeZ80Mem reads Z80Memory state from the data buffer.
func (e *EmulatorBase) deserializeZ80Mem(data []byte, offset int) int {
	e.z80Mem.bankRegister = binary.LittleEndian.Uint16(data[offset:])
	offset += 2
	return offset
}

// serializeBase writes EmulatorBase inline state to the data buffer.
func (e *EmulatorBase) serializeBase(data []byte, offset int) int {
	data[offset] = boolByte(e.z80IntPending)
	offset++

	binary.LittleEndian.PutUint64(data[offset:], math.Float64bits(e.filterPrevL))
	offset += 8

	binary.LittleEndian.PutUint64(data[offset:], math.Float64bits(e.filterPrevR))
	offset += 8

	return offset
}

// deserializeBase reads EmulatorBase inline state from the data buffer.
func (e *EmulatorBase) deserializeBase(data []byte, offset int) int {
	e.z80IntPending = data[offset] != 0
	offset++

	e.filterPrevL = math.Float64frombits(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	e.filterPrevR = math.Float64frombits(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	return offset
}
