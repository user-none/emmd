package emu

import (
	"encoding/binary"
	"errors"
)

const (
	ym2612SerializeVersion = 1
	// Per-operator serialization size:
	// dt(1) + mul(1) + tl(1) + rs(1) + ar(1) + d1r(1) + d2r(1) + d1l(1) + rr(1) + am(1) +
	// ssgEG(1) + ssgInverted(1) + phaseCounter(4) + phaseInc(4) +
	// egState(1) + egLevel(2) + keyOn(1) + prevOut(4) + keyCode(1) = 29
	ymOperatorSerializeSize = 29
	// Per-channel (non-operator fields):
	// fNum(2) + block(1) + algorithm(1) + feedback(1) + panL(1) + panR(1) + ams(1) + fms(1) = 9
	ymChannelSerializeSize = 9
	// Global state:
	// addrLatch(2) + dacEnable(1) + dacSample(1) + lfoEnable(1) + lfoFreq(1) +
	// timerA.period(2) + timerA.counter(2) + timerB.period(2) + timerB.counter(2) +
	// timerALoad(1) + timerBLoad(1) + timerAEnable(1) + timerBEnable(1) + timerAOver(1) + timerBOver(1) +
	// ch3Mode(1) + csmKeyOn(1) + ch3Freq(8) + ch3Block(4) +
	// egCounter(2) + egClock(1) + lfoCnt(2) + lfoStep(1) + lfoAMOut(1) +
	// timerBSubCount(1) + cycleAccum(4) + resampAccum(4) +
	// nativeSampleCount(8) + busyUntil(8) + lastStatus(1) + lastStatusSample(8) = 75
	ymGlobalSerializeSize = 75
	// YM2612SerializeSize is the total bytes needed for YM2612 serialization.
	// version(1) + 24 operators * 29 + 6 channels * 9 + global(75) = 826
	YM2612SerializeSize = 1 + 24*ymOperatorSerializeSize + 6*ymChannelSerializeSize + ymGlobalSerializeSize
)

// Serialize writes YM2612 state to buf. buf must be at least YM2612SerializeSize bytes.
func (y *YM2612) Serialize(buf []byte) error {
	if len(buf) < YM2612SerializeSize {
		return errors.New("YM2612 serialize buffer too small")
	}

	offset := 0

	// Version
	buf[offset] = ym2612SerializeVersion
	offset++

	// Operators (6 channels x 4 operators = 24)
	for ch := 0; ch < 6; ch++ {
		for op := 0; op < 4; op++ {
			offset = serializeOperator(&y.ch[ch].op[op], buf, offset)
		}
	}

	// Channel fields (non-operator)
	for ch := 0; ch < 6; ch++ {
		offset = serializeChannel(&y.ch[ch], buf, offset)
	}

	// Global state
	buf[offset] = y.addrLatch[0]
	offset++
	buf[offset] = y.addrLatch[1]
	offset++
	buf[offset] = boolByte(y.dacEnable)
	offset++
	buf[offset] = y.dacSample
	offset++
	buf[offset] = boolByte(y.lfoEnable)
	offset++
	buf[offset] = y.lfoFreq
	offset++

	// Timers
	binary.LittleEndian.PutUint16(buf[offset:], y.timerA.period)
	offset += 2
	binary.LittleEndian.PutUint16(buf[offset:], y.timerA.counter)
	offset += 2
	binary.LittleEndian.PutUint16(buf[offset:], y.timerB.period)
	offset += 2
	binary.LittleEndian.PutUint16(buf[offset:], y.timerB.counter)
	offset += 2

	// Timer flags
	buf[offset] = boolByte(y.timerALoad)
	offset++
	buf[offset] = boolByte(y.timerBLoad)
	offset++
	buf[offset] = boolByte(y.timerAEnable)
	offset++
	buf[offset] = boolByte(y.timerBEnable)
	offset++
	buf[offset] = boolByte(y.timerAOver)
	offset++
	buf[offset] = boolByte(y.timerBOver)
	offset++

	// Channel 3 mode
	buf[offset] = y.ch3Mode
	offset++
	buf[offset] = boolByte(y.csmKeyOn)
	offset++

	// Channel 3 per-operator frequencies
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint16(buf[offset:], y.ch3Freq[i])
		offset += 2
	}
	for i := 0; i < 4; i++ {
		buf[offset] = y.ch3Block[i]
		offset++
	}

	// Envelope generator
	binary.LittleEndian.PutUint16(buf[offset:], y.egCounter)
	offset += 2
	buf[offset] = y.egClock
	offset++

	// LFO
	binary.LittleEndian.PutUint16(buf[offset:], y.lfoCnt)
	offset += 2
	buf[offset] = y.lfoStep
	offset++
	buf[offset] = y.lfoAMOut
	offset++

	// Timer B sub-counter
	buf[offset] = y.timerBSubCount
	offset++

	// Timing accumulators
	binary.LittleEndian.PutUint32(buf[offset:], uint32(int32(y.cycleAccum)))
	offset += 4
	binary.LittleEndian.PutUint32(buf[offset:], uint32(int32(y.resampAccum)))
	offset += 4

	// Counters
	binary.LittleEndian.PutUint64(buf[offset:], y.nativeSampleCount)
	offset += 8
	binary.LittleEndian.PutUint64(buf[offset:], y.busyUntil)
	offset += 8

	// Status
	buf[offset] = y.lastStatus
	offset++
	binary.LittleEndian.PutUint64(buf[offset:], y.lastStatusSample)
	offset += 8

	return nil
}

// Deserialize reads YM2612 state from buf. buf must be at least YM2612SerializeSize bytes.
func (y *YM2612) Deserialize(buf []byte) error {
	if len(buf) < YM2612SerializeSize {
		return errors.New("YM2612 deserialize buffer too small")
	}

	offset := 0

	// Version
	version := buf[offset]
	offset++
	if version > ym2612SerializeVersion {
		return errors.New("unsupported YM2612 state version")
	}

	// Operators (6 channels x 4 operators = 24)
	for ch := 0; ch < 6; ch++ {
		for op := 0; op < 4; op++ {
			offset = deserializeOperator(&y.ch[ch].op[op], buf, offset)
		}
	}

	// Channel fields (non-operator)
	for ch := 0; ch < 6; ch++ {
		offset = deserializeChannel(&y.ch[ch], buf, offset)
	}

	// Global state
	y.addrLatch[0] = buf[offset]
	offset++
	y.addrLatch[1] = buf[offset]
	offset++
	y.dacEnable = buf[offset] != 0
	offset++
	y.dacSample = buf[offset]
	offset++
	y.lfoEnable = buf[offset] != 0
	offset++
	y.lfoFreq = buf[offset]
	offset++

	// Timers
	y.timerA.period = binary.LittleEndian.Uint16(buf[offset:])
	offset += 2
	y.timerA.counter = binary.LittleEndian.Uint16(buf[offset:])
	offset += 2
	y.timerB.period = binary.LittleEndian.Uint16(buf[offset:])
	offset += 2
	y.timerB.counter = binary.LittleEndian.Uint16(buf[offset:])
	offset += 2

	// Timer flags
	y.timerALoad = buf[offset] != 0
	offset++
	y.timerBLoad = buf[offset] != 0
	offset++
	y.timerAEnable = buf[offset] != 0
	offset++
	y.timerBEnable = buf[offset] != 0
	offset++
	y.timerAOver = buf[offset] != 0
	offset++
	y.timerBOver = buf[offset] != 0
	offset++

	// Channel 3 mode
	y.ch3Mode = buf[offset]
	offset++
	y.csmKeyOn = buf[offset] != 0
	offset++

	// Channel 3 per-operator frequencies
	for i := 0; i < 4; i++ {
		y.ch3Freq[i] = binary.LittleEndian.Uint16(buf[offset:])
		offset += 2
	}
	for i := 0; i < 4; i++ {
		y.ch3Block[i] = buf[offset]
		offset++
	}

	// Envelope generator
	y.egCounter = binary.LittleEndian.Uint16(buf[offset:])
	offset += 2
	y.egClock = buf[offset]
	offset++

	// LFO
	y.lfoCnt = binary.LittleEndian.Uint16(buf[offset:])
	offset += 2
	y.lfoStep = buf[offset]
	offset++
	y.lfoAMOut = buf[offset]
	offset++

	// Timer B sub-counter
	y.timerBSubCount = buf[offset]
	offset++

	// Timing accumulators
	y.cycleAccum = int(int32(binary.LittleEndian.Uint32(buf[offset:])))
	offset += 4
	y.resampAccum = int(int32(binary.LittleEndian.Uint32(buf[offset:])))
	offset += 4

	// Counters
	y.nativeSampleCount = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8
	y.busyUntil = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8

	// Status
	y.lastStatus = buf[offset]
	offset++
	y.lastStatusSample = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8

	return nil
}

// serializeOperator writes a single ymOperator to buf at the given offset.
func serializeOperator(op *ymOperator, buf []byte, offset int) int {
	buf[offset] = op.dt
	offset++
	buf[offset] = op.mul
	offset++
	buf[offset] = op.tl
	offset++
	buf[offset] = op.rs
	offset++
	buf[offset] = op.ar
	offset++
	buf[offset] = op.d1r
	offset++
	buf[offset] = op.d2r
	offset++
	buf[offset] = op.d1l
	offset++
	buf[offset] = op.rr
	offset++
	buf[offset] = boolByte(op.am)
	offset++

	buf[offset] = op.ssgEG
	offset++
	buf[offset] = boolByte(op.ssgInverted)
	offset++

	binary.LittleEndian.PutUint32(buf[offset:], op.phaseCounter)
	offset += 4
	binary.LittleEndian.PutUint32(buf[offset:], op.phaseInc)
	offset += 4

	buf[offset] = op.egState
	offset++
	binary.LittleEndian.PutUint16(buf[offset:], op.egLevel)
	offset += 2
	buf[offset] = boolByte(op.keyOn)
	offset++

	binary.LittleEndian.PutUint16(buf[offset:], uint16(op.prevOut[0]))
	offset += 2
	binary.LittleEndian.PutUint16(buf[offset:], uint16(op.prevOut[1]))
	offset += 2
	buf[offset] = op.keyCode
	offset++

	return offset
}

// deserializeOperator reads a single ymOperator from buf at the given offset.
func deserializeOperator(op *ymOperator, buf []byte, offset int) int {
	op.dt = buf[offset]
	offset++
	op.mul = buf[offset]
	offset++
	op.tl = buf[offset]
	offset++
	op.rs = buf[offset]
	offset++
	op.ar = buf[offset]
	offset++
	op.d1r = buf[offset]
	offset++
	op.d2r = buf[offset]
	offset++
	op.d1l = buf[offset]
	offset++
	op.rr = buf[offset]
	offset++
	op.am = buf[offset] != 0
	offset++

	op.ssgEG = buf[offset]
	offset++
	op.ssgInverted = buf[offset] != 0
	offset++

	op.phaseCounter = binary.LittleEndian.Uint32(buf[offset:])
	offset += 4
	op.phaseInc = binary.LittleEndian.Uint32(buf[offset:])
	offset += 4

	op.egState = buf[offset]
	offset++
	op.egLevel = binary.LittleEndian.Uint16(buf[offset:])
	offset += 2
	op.keyOn = buf[offset] != 0
	offset++

	op.prevOut[0] = int16(binary.LittleEndian.Uint16(buf[offset:]))
	offset += 2
	op.prevOut[1] = int16(binary.LittleEndian.Uint16(buf[offset:]))
	offset += 2
	op.keyCode = buf[offset]
	offset++

	return offset
}

// serializeChannel writes the non-operator fields of a ymChannel to buf.
func serializeChannel(ch *ymChannel, buf []byte, offset int) int {
	binary.LittleEndian.PutUint16(buf[offset:], ch.fNum)
	offset += 2
	buf[offset] = ch.block
	offset++
	buf[offset] = ch.algorithm
	offset++
	buf[offset] = ch.feedback
	offset++
	buf[offset] = boolByte(ch.panL)
	offset++
	buf[offset] = boolByte(ch.panR)
	offset++
	buf[offset] = ch.ams
	offset++
	buf[offset] = ch.fms
	offset++
	return offset
}

// deserializeChannel reads the non-operator fields of a ymChannel from buf.
func deserializeChannel(ch *ymChannel, buf []byte, offset int) int {
	ch.fNum = binary.LittleEndian.Uint16(buf[offset:])
	offset += 2
	ch.block = buf[offset]
	offset++
	ch.algorithm = buf[offset]
	offset++
	ch.feedback = buf[offset]
	offset++
	ch.panL = buf[offset] != 0
	offset++
	ch.panR = buf[offset] != 0
	offset++
	ch.ams = buf[offset]
	offset++
	ch.fms = buf[offset]
	offset++
	return offset
}
