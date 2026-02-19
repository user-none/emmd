package emu

import (
	"encoding/binary"
	"errors"
)

const (
	vdpSerializeVersion = 1
	// VDPSerializeSize is the total bytes needed for VDP serialization.
	// version(1) + vram(65536) + cram(128) + vsram(80) + regs(24) +
	// writePending(1) + code(1) + address(2) + readBuffer(2) +
	// vIntPending(1) + spriteOverflow(1) + spriteCollision(1) + vBlank(1) + hBlank(1) +
	// dmaEndCycle(8) + dmaStallCycles(4) + assertedIntLevel(1) +
	// vCounter(2) + hCounter(1) + currentLine(4) +
	// hvLatched(1) + hvLatchValue(2) +
	// hIntCounter(4) + dmaFillPending(1) +
	// oddField(1) + isPAL(1)
	VDPSerializeSize = 65810
)

// Serialize writes VDP state to buf. buf must be at least VDPSerializeSize bytes.
func (v *VDP) Serialize(buf []byte) error {
	if len(buf) < VDPSerializeSize {
		return errors.New("VDP serialize buffer too small")
	}

	offset := 0

	// Version
	buf[offset] = vdpSerializeVersion
	offset++

	// VRAM (64KB)
	copy(buf[offset:], v.vram[:])
	offset += len(v.vram)

	// CRAM (128 bytes)
	copy(buf[offset:], v.cram[:])
	offset += len(v.cram)

	// VSRAM (80 bytes)
	copy(buf[offset:], v.vsram[:])
	offset += len(v.vsram)

	// Registers (24 bytes)
	copy(buf[offset:], v.regs[:])
	offset += len(v.regs)

	// Control port state
	buf[offset] = boolByte(v.writePending)
	offset++
	buf[offset] = v.code
	offset++
	binary.LittleEndian.PutUint16(buf[offset:], v.address)
	offset += 2
	binary.LittleEndian.PutUint16(buf[offset:], v.readBuffer)
	offset += 2

	// Status
	buf[offset] = boolByte(v.vIntPending)
	offset++
	buf[offset] = boolByte(v.spriteOverflow)
	offset++
	buf[offset] = boolByte(v.spriteCollision)
	offset++
	buf[offset] = boolByte(v.vBlank)
	offset++
	buf[offset] = boolByte(v.hBlank)
	offset++
	binary.LittleEndian.PutUint64(buf[offset:], v.dmaEndCycle)
	offset += 8
	binary.LittleEndian.PutUint32(buf[offset:], uint32(int32(v.dmaStallCycles)))
	offset += 4
	buf[offset] = v.assertedIntLevel
	offset++

	// Counters
	binary.LittleEndian.PutUint16(buf[offset:], v.vCounter)
	offset += 2
	buf[offset] = v.hCounter
	offset++
	binary.LittleEndian.PutUint32(buf[offset:], uint32(int32(v.currentLine)))
	offset += 4
	buf[offset] = boolByte(v.hvLatched)
	offset++
	binary.LittleEndian.PutUint16(buf[offset:], v.hvLatchValue)
	offset += 2

	// Interrupt tracking
	binary.LittleEndian.PutUint32(buf[offset:], uint32(int32(v.hIntCounter)))
	offset += 4
	buf[offset] = boolByte(v.dmaFillPending)
	offset++

	// Interlace / Region
	buf[offset] = boolByte(v.oddField)
	offset++
	buf[offset] = boolByte(v.isPAL)
	offset++

	return nil
}

// Deserialize reads VDP state from buf. buf must be at least VDPSerializeSize bytes.
func (v *VDP) Deserialize(buf []byte) error {
	if len(buf) < VDPSerializeSize {
		return errors.New("VDP deserialize buffer too small")
	}

	offset := 0

	// Version
	version := buf[offset]
	offset++
	if version > vdpSerializeVersion {
		return errors.New("unsupported VDP state version")
	}

	// VRAM (64KB)
	copy(v.vram[:], buf[offset:offset+len(v.vram)])
	offset += len(v.vram)

	// CRAM (128 bytes)
	copy(v.cram[:], buf[offset:offset+len(v.cram)])
	offset += len(v.cram)

	// VSRAM (80 bytes)
	copy(v.vsram[:], buf[offset:offset+len(v.vsram)])
	offset += len(v.vsram)

	// Registers (24 bytes)
	copy(v.regs[:], buf[offset:offset+len(v.regs)])
	offset += len(v.regs)

	// Control port state
	v.writePending = buf[offset] != 0
	offset++
	v.code = buf[offset]
	offset++
	v.address = binary.LittleEndian.Uint16(buf[offset:])
	offset += 2
	v.readBuffer = binary.LittleEndian.Uint16(buf[offset:])
	offset += 2

	// Status
	v.vIntPending = buf[offset] != 0
	offset++
	v.spriteOverflow = buf[offset] != 0
	offset++
	v.spriteCollision = buf[offset] != 0
	offset++
	v.vBlank = buf[offset] != 0
	offset++
	v.hBlank = buf[offset] != 0
	offset++
	v.dmaEndCycle = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8
	v.dmaStallCycles = int(int32(binary.LittleEndian.Uint32(buf[offset:])))
	offset += 4
	v.assertedIntLevel = buf[offset]
	offset++

	// Counters
	v.vCounter = binary.LittleEndian.Uint16(buf[offset:])
	offset += 2
	v.hCounter = buf[offset]
	offset++
	v.currentLine = int(int32(binary.LittleEndian.Uint32(buf[offset:])))
	offset += 4
	v.hvLatched = buf[offset] != 0
	offset++
	v.hvLatchValue = binary.LittleEndian.Uint16(buf[offset:])
	offset += 2

	// Interrupt tracking
	v.hIntCounter = int(int32(binary.LittleEndian.Uint32(buf[offset:])))
	offset += 4
	v.dmaFillPending = buf[offset] != 0
	offset++

	// Interlace / Region
	v.oddField = buf[offset] != 0
	offset++
	v.isPAL = buf[offset] != 0
	offset++

	return nil
}
