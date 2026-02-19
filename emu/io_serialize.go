package emu

import (
	"encoding/binary"
	"errors"
)

const (
	ioSerializeVersion = 1
	// IOSerializeSize is the total bytes needed for IO serialization.
	// version(1) + p1Data(1) + p1Ctrl(1) + p2Data(1) + p2Ctrl(1) +
	// p1THState(1) + p1LastTHHigh(1) + p1LastCycle(8) +
	// p2THState(1) + p2LastTHHigh(1) + p2LastCycle(8) +
	// InputP1.Connected(1) + InputP1.SixButton(1) +
	// InputP2.Connected(1) + InputP2.SixButton(1)
	IOSerializeSize = 29
)

// Serialize writes IO state to buf. buf must be at least IOSerializeSize bytes.
func (io *IO) Serialize(buf []byte) error {
	if len(buf) < IOSerializeSize {
		return errors.New("IO serialize buffer too small")
	}

	offset := 0

	// Version
	buf[offset] = ioSerializeVersion
	offset++

	// Port data/control registers
	buf[offset] = io.p1Data
	offset++
	buf[offset] = io.p1Ctrl
	offset++
	buf[offset] = io.p2Data
	offset++
	buf[offset] = io.p2Ctrl
	offset++

	// Player 1 six-button state machine
	buf[offset] = io.p1THState
	offset++
	buf[offset] = boolByte(io.p1LastTHHigh)
	offset++
	binary.LittleEndian.PutUint64(buf[offset:], io.p1LastCycle)
	offset += 8

	// Player 2 six-button state machine
	buf[offset] = io.p2THState
	offset++
	buf[offset] = boolByte(io.p2LastTHHigh)
	offset++
	binary.LittleEndian.PutUint64(buf[offset:], io.p2LastCycle)
	offset += 8

	// Input configuration
	buf[offset] = boolByte(io.InputP1.Connected)
	offset++
	buf[offset] = boolByte(io.InputP1.SixButton)
	offset++
	buf[offset] = boolByte(io.InputP2.Connected)
	offset++
	buf[offset] = boolByte(io.InputP2.SixButton)
	offset++

	return nil
}

// Deserialize reads IO state from buf. buf must be at least IOSerializeSize bytes.
func (io *IO) Deserialize(buf []byte) error {
	if len(buf) < IOSerializeSize {
		return errors.New("IO deserialize buffer too small")
	}

	offset := 0

	// Version
	version := buf[offset]
	offset++
	if version > ioSerializeVersion {
		return errors.New("unsupported IO state version")
	}

	// Port data/control registers
	io.p1Data = buf[offset]
	offset++
	io.p1Ctrl = buf[offset]
	offset++
	io.p2Data = buf[offset]
	offset++
	io.p2Ctrl = buf[offset]
	offset++

	// Player 1 six-button state machine
	io.p1THState = buf[offset]
	offset++
	io.p1LastTHHigh = buf[offset] != 0
	offset++
	io.p1LastCycle = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8

	// Player 2 six-button state machine
	io.p2THState = buf[offset]
	offset++
	io.p2LastTHHigh = buf[offset] != 0
	offset++
	io.p2LastCycle = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8

	// Input configuration
	io.InputP1.Connected = buf[offset] != 0
	offset++
	io.InputP1.SixButton = buf[offset] != 0
	offset++
	io.InputP2.Connected = buf[offset] != 0
	offset++
	io.InputP2.SixButton = buf[offset] != 0
	offset++

	return nil
}
