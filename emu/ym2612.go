package emu

// Envelope states for ADSR
const (
	egAttack  = 0
	egDecay   = 1
	egSustain = 2
	egRelease = 3
)

// Channel 3 mode values (register $27 bits 7-6)
const (
	ch3ModeNormal  = 0 // All operators share frequency
	ch3ModeSpecial = 1 // Per-operator frequencies
	ch3ModeCSM     = 2 // Per-operator frequencies + Timer A overflow key-on
)

// Status port timing constants
const (
	busyDuration        = 2     // ~32 internal cycles ~= 2 native samples
	statusDecayDuration = 13300 // ~250ms at ~53kHz native rate
)

// ymOperator holds decoded register state for one of four operators in a channel.
type ymOperator struct {
	// Register fields
	dt  uint8 // Detune (3-bit: bit2=sign, bits1-0=value)
	mul uint8 // Frequency multiplier (4-bit, 0=x0.5, 1-15=x1..x15)
	tl  uint8 // Total level / attenuation (7-bit, 0=max vol, 127=min)
	rs  uint8 // Rate scaling (2-bit)
	ar  uint8 // Attack rate (5-bit)
	d1r uint8 // Decay 1 rate (5-bit) (aka DR)
	d2r uint8 // Decay 2 rate (5-bit) (aka SR)
	d1l uint8 // Decay 1 level / sustain level (4-bit)
	rr  uint8 // Release rate (4-bit)
	am  bool  // AM enable (amplitude modulation from LFO)

	// SSG-EG mode (4-bit)
	ssgEG       uint8
	ssgInverted bool // SSG-EG inversion state (toggled by alternate mode)

	// Phase generator state
	phaseCounter uint32 // 20-bit phase accumulator
	phaseInc     uint32 // 20-bit phase increment (recomputed on freq change)

	// Envelope generator state
	egState uint8  // egAttack, egDecay, egSustain, egRelease
	egLevel uint16 // 10-bit attenuation (0=full vol, 0x3FF=silent)
	keyOn   bool   // Current key-on state

	// Operator output (for feedback and algorithm connections)
	prevOut [2]int16 // Previous two outputs (for feedback)
	keyCode uint8    // 5-bit key code for rate scaling
}

// ymChannel holds decoded register state for one of six FM channels.
type ymChannel struct {
	op [4]ymOperator

	// Frequency registers
	fNum  uint16 // 11-bit F-number
	block uint8  // 3-bit block (octave)

	// Channel registers
	algorithm uint8 // 3-bit algorithm (0-7)
	feedback  uint8 // 3-bit feedback level (0=disabled, 1-7)
	panL      bool  // Left output enable
	panR      bool  // Right output enable
	ams       uint8 // 2-bit AM sensitivity
	fms       uint8 // 3-bit FM sensitivity
}

// YM2612 implements the Yamaha YM2612 (OPN2) FM synthesizer.
type YM2612 struct {
	sampleRate int
	clockHz    int // M68K clock (NTSC ~7.67MHz)
	buffer     []int16

	ch [6]ymChannel

	// Address latches for Part I (ports 0/1) and Part II (ports 2/3)
	addrLatch [2]uint8

	// DAC
	dacEnable bool
	dacSample uint8 // 8-bit unsigned DAC sample

	// LFO
	lfoEnable bool
	lfoFreq   uint8 // 3-bit LFO frequency select

	// Timers
	timerA       timer
	timerB       timer
	timerALoad   bool // Timer A load/start
	timerBLoad   bool // Timer B load/start
	timerAEnable bool // Timer A overflow flag enable
	timerBEnable bool // Timer B overflow flag enable
	timerAOver   bool // Timer A overflow flag (readable in status)
	timerBOver   bool // Timer B overflow flag (readable in status)

	// Channel 3 mode (normal/special/CSM)
	ch3Mode  uint8     // Channel 3 mode: 0=normal, 1=special, 2=CSM
	csmKeyOn bool      // True while CSM-triggered key-on is active
	ch3Freq  [4]uint16 // Per-operator F-number for ch3 slots
	ch3Block [4]uint8  // Per-operator block for ch3 slots

	// Envelope generator global counter
	egCounter uint16 // 12-bit global envelope counter
	egClock   uint8  // Divider: counts 0,1,2 then wraps (step every 3 samples)

	// LFO counter
	lfoCnt   uint16 // LFO period counter
	lfoStep  uint8  // 0-127 position in LFO cycle
	lfoAMOut uint8  // Current LFO AM output value

	// Timer B sub-counter (ticks every 16 sample clocks)
	timerBSubCount uint8

	// Timing for sample generation
	cycleAccum        int    // Accumulated M68K cycles
	resampAccum       int    // Bresenham accumulator for resampling to sampleRate
	nativeClock       int    // Internal sample rate (clockHz / 144)
	nativeSampleCount uint64 // Cumulative native sample counter (for busy/status timing)

	// Busy flag
	busyUntil uint64 // Native sample count when busy flag clears

	// Status port caching (discrete YM2612: ports 1/3 return last status)
	lastStatus       uint8  // Last value returned by port 0/2 read
	lastStatusSample uint64 // Native sample count when lastStatus was set
}

// NewYM2612 creates a new YM2612 FM synthesizer.
func NewYM2612(clockHz, sampleRate int) *YM2612 {
	y := &YM2612{
		sampleRate:  sampleRate,
		clockHz:     clockHz,
		nativeClock: clockHz / 144,
		buffer:      make([]int16, 0, 2048),
		dacSample:   0x80, // Center value: (0x80-128)<<6 = 0, no DC offset
	}
	// Initialize all channels with panning enabled (L+R)
	for ch := range y.ch {
		y.ch[ch].panL = true
		y.ch[ch].panR = true
		for op := range y.ch[ch].op {
			y.ch[ch].op[op].egState = egRelease
			y.ch[ch].op[op].egLevel = 0x3FF // Silent
		}
	}
	return y
}

// ReadPort reads from a YM2612 port (0-3).
// Ports 0/2: live status register (timer flags + busy bit), cached for port 1/3
// Ports 1/3: return last status read from port 0/2, decaying to 0 after ~250ms
func (y *YM2612) ReadPort(port uint8) uint8 {
	if port == 0 || port == 2 {
		var status uint8
		if y.timerAOver {
			status |= 0x01
		}
		if y.timerBOver {
			status |= 0x02
		}
		if y.nativeSampleCount < y.busyUntil {
			status |= 0x80
		}
		// Cache for port 1/3 reads
		y.lastStatus = status
		y.lastStatusSample = y.nativeSampleCount
		return status
	}
	// Ports 1/3: return cached status with decay
	if y.nativeSampleCount-y.lastStatusSample < statusDecayDuration {
		return y.lastStatus
	}
	return 0
}

// WritePort writes to a YM2612 port (0-3).
// Port 0: address latch for Part I
// Port 1: data write for Part I
// Port 2: address latch for Part II
// Port 3: data write for Part II
func (y *YM2612) WritePort(port uint8, val uint8) {
	switch port {
	case 0:
		y.addrLatch[0] = val
	case 1:
		y.writeRegister(0, y.addrLatch[0], val)
		y.busyUntil = y.nativeSampleCount + busyDuration
	case 2:
		y.addrLatch[1] = val
	case 3:
		y.writeRegister(1, y.addrLatch[1], val)
		y.busyUntil = y.nativeSampleCount + busyDuration
	}
}

// writeRegister dispatches a register write to the appropriate handler.
// part: 0 = Part I (channels 0-2), 1 = Part II (channels 3-5)
func (y *YM2612) writeRegister(part int, addr, val uint8) {
	switch {
	case addr < 0x20:
		// Invalid register range (below $20)
		return
	case addr < 0x30:
		// Global registers $20-$2F (only valid in Part I)
		if part == 0 {
			y.writeGlobalRegister(addr, val)
		}
	case addr < 0xA0:
		// Operator registers $30-$9F
		y.writeOperatorRegister(part, addr, val)
	default:
		// Channel registers $A0-$B6
		y.writeChannelRegister(part, addr, val)
	}
}

// writeGlobalRegister handles writes to registers $20-$2F.
func (y *YM2612) writeGlobalRegister(addr, val uint8) {
	switch addr {
	case 0x22:
		// LFO control
		y.lfoEnable = val&0x08 != 0
		y.lfoFreq = val & 0x07
		if !y.lfoEnable {
			y.lfoStep = 0
			y.lfoCnt = 0
		}
	case 0x24:
		// Timer A MSB (high 8 bits of 10-bit period)
		y.timerA.period = (y.timerA.period & 0x003) | (uint16(val) << 2)
	case 0x25:
		// Timer A LSB (low 2 bits of 10-bit period)
		y.timerA.period = (y.timerA.period & 0x3FC) | uint16(val&0x03)
	case 0x26:
		// Timer B period (8-bit)
		y.timerB.period = uint16(val)
	case 0x27:
		// Timer control / Channel 3 mode
		// Bits 7-6: channel 3 mode
		y.ch3Mode = (val >> 6) & 0x03
		// Bit 0: Timer A load
		y.timerALoad = val&0x01 != 0
		// Bit 1: Timer B load
		y.timerBLoad = val&0x02 != 0
		// Bit 2: Timer A enable (allow overflow flag to set)
		y.timerAEnable = val&0x04 != 0
		// Bit 3: Timer B enable (allow overflow flag to set)
		y.timerBEnable = val&0x08 != 0
		// Bit 4: Reset Timer A overflow flag
		if val&0x10 != 0 {
			y.timerAOver = false
		}
		// Bit 5: Reset Timer B overflow flag
		if val&0x20 != 0 {
			y.timerBOver = false
		}
	case 0x28:
		// Key on/off
		y.writeKeyOnOff(val)
	case 0x2A:
		// DAC data
		y.dacSample = val
	case 0x2B:
		// DAC enable
		y.dacEnable = val&0x80 != 0
	}
}

// operatorOrder maps register slot bits to operator index.
// Register order is S1(0), S3(1), S2(2), S4(3) but we store as 0,1,2,3 = S1,S2,S3,S4.
// So register slot 0->op0(S1), slot 1->op2(S3), slot 2->op1(S2), slot 3->op3(S4).
var operatorOrder = [4]int{0, 2, 1, 3}

// writeOperatorRegister handles writes to registers $30-$9F.
func (y *YM2612) writeOperatorRegister(part int, addr, val uint8) {
	chSlot := int(addr & 0x03)
	if chSlot == 3 {
		return // Invalid channel slot
	}
	opSlot := int((addr >> 2) & 0x03)
	opIdx := operatorOrder[opSlot]
	chIdx := chSlot + part*3

	op := &y.ch[chIdx].op[opIdx]

	regGroup := addr & 0xF0
	switch regGroup {
	case 0x30:
		// DT1/MUL
		op.dt = (val >> 4) & 0x07
		op.mul = val & 0x0F
		y.updatePhaseIncrement(chIdx, opIdx)
	case 0x40:
		// TL (Total Level)
		op.tl = val & 0x7F
	case 0x50:
		// RS/AR
		op.rs = (val >> 6) & 0x03
		op.ar = val & 0x1F
	case 0x60:
		// AM/D1R
		op.am = val&0x80 != 0
		op.d1r = val & 0x1F
	case 0x70:
		// D2R
		op.d2r = val & 0x1F
	case 0x80:
		// D1L/RR
		op.d1l = (val >> 4) & 0x0F
		op.rr = val & 0x0F
	case 0x90:
		// SSG-EG
		newVal := val & 0x0F
		if newVal&ssgEnable == 0 {
			newVal = 0
		}
		if (newVal^op.ssgEG)&ssgAttack != 0 {
			op.ssgInverted = !op.ssgInverted
		}
		op.ssgEG = newVal
	}
}

// writeChannelRegister handles writes to registers $A0-$B6.
func (y *YM2612) writeChannelRegister(part int, addr, val uint8) {
	chSlot := int(addr & 0x03)
	if chSlot == 3 {
		return // Invalid channel slot
	}
	chIdx := chSlot + part*3

	switch {
	case addr >= 0xA0 && addr <= 0xA2:
		// F-Number LSB (low 8 bits) - Part I
		y.ch[chIdx].fNum = (y.ch[chIdx].fNum & 0x700) | uint16(val)
		y.updateChannelFrequency(chIdx)
	case addr >= 0xA4 && addr <= 0xA6:
		// F-Number MSB (high 3 bits) + Block - Part I
		// Written before the LSB register; latched until LSB write
		y.ch[chIdx].block = (val >> 3) & 0x07
		y.ch[chIdx].fNum = (y.ch[chIdx].fNum & 0x0FF) | (uint16(val&0x07) << 8)
	case addr >= 0xA8 && addr <= 0xAA:
		// Channel 3 special mode: per-operator F-Number LSB
		slot := int(addr - 0xA8)
		if part == 0 {
			y.ch3Freq[slot] = (y.ch3Freq[slot] & 0x700) | uint16(val)
			// Update phase increment for the affected operator
			if y.ch3Mode != ch3ModeNormal {
				if opIdx := ch3SlotToOp(slot); opIdx >= 0 {
					y.updatePhaseIncrement(2, opIdx)
				}
			}
		}
	case addr >= 0xAC && addr <= 0xAE:
		// Channel 3 special mode: per-operator F-Number MSB + Block
		slot := int(addr - 0xAC)
		if part == 0 {
			y.ch3Block[slot] = (val >> 3) & 0x07
			y.ch3Freq[slot] = (y.ch3Freq[slot] & 0x0FF) | (uint16(val&0x07) << 8)
		}
	case addr >= 0xB0 && addr <= 0xB2:
		// Feedback/Algorithm
		y.ch[chIdx].algorithm = val & 0x07
		y.ch[chIdx].feedback = (val >> 3) & 0x07
	case addr >= 0xB4 && addr <= 0xB6:
		// Panning/AMS/FMS
		y.ch[chIdx].panL = val&0x80 != 0
		y.ch[chIdx].panR = val&0x40 != 0
		y.ch[chIdx].ams = (val >> 4) & 0x03
		y.ch[chIdx].fms = val & 0x07
	}
}

// writeKeyOnOff handles the Key On/Off register ($28).
// val bits 0-2: channel (0-2=Part I, 4-6=Part II)
// val bits 4-7: operator enable (bit4=S1, bit5=S2, bit6=S3, bit7=S4)
func (y *YM2612) writeKeyOnOff(val uint8) {
	chLow := int(val & 0x03)
	if chLow >= 3 {
		return // Invalid
	}
	chIdx := chLow
	if val&0x04 != 0 {
		chIdx += 3 // Part II channels
	}

	ch := &y.ch[chIdx]
	for i := 0; i < 4; i++ {
		on := val&(0x10<<uint(i)) != 0
		op := &ch.op[i]
		if on && !op.keyOn {
			// Key on: reset phase, start attack
			op.keyOn = true
			op.phaseCounter = 0
			op.egState = egAttack
			// SSG-EG: initialize inversion from attack bit
			op.ssgInverted = op.ssgEG&ssgAttack != 0
			// Rate 62-63 instant attack handled in envelope step
			if y.effectiveRate(op.ar, op) >= 62 {
				op.egLevel = 0
				op.egState = egDecay
			}
		} else if !on && op.keyOn {
			// Key off: transition to release
			op.keyOn = false
			if op.ssgEG&ssgEnable != 0 && op.ssgInverted {
				op.egLevel = (ssgCenter - op.egLevel) & 0x3FF
				op.ssgInverted = false
			}
			op.egState = egRelease
		}
	}
}

// effectiveRate computes 2*rate + rks, clamped to 63. Returns 0 if rate is 0.
func (y *YM2612) effectiveRate(rate uint8, op *ymOperator) uint8 {
	if rate == 0 {
		return 0
	}
	rks := op.keyCode >> (3 - op.rs)
	r := int(2*rate) + int(rks)
	if r > 63 {
		r = 63
	}
	return uint8(r)
}

// updatePhaseIncrement recomputes an operator's phase increment.
func (y *YM2612) updatePhaseIncrement(chIdx, opIdx int) {
	ch := &y.ch[chIdx]
	op := &ch.op[opIdx]

	fNum := ch.fNum
	block := ch.block
	kc := op.keyCode

	// Channel 3 special mode: use per-operator frequency
	if y.ch3Mode != ch3ModeNormal && chIdx == 2 {
		slot := ch3SlotMap(opIdx)
		if slot >= 0 {
			fNum = y.ch3Freq[slot]
			block = y.ch3Block[slot]
			kc = computeKeyCode(fNum, block)
			op.keyCode = kc
		}
	}

	op.phaseInc = computePhaseIncrement(fNum, block, kc, op.dt, op.mul)
}

// updateChannelFrequency recomputes phase increments for all operators in a channel.
func (y *YM2612) updateChannelFrequency(chIdx int) {
	ch := &y.ch[chIdx]
	// Update key code for all operators
	kc := computeKeyCode(ch.fNum, ch.block)
	for i := range ch.op {
		ch.op[i].keyCode = kc
		y.updatePhaseIncrement(chIdx, i)
	}
}

// computeKeyCode computes the 5-bit key code from F-number and block.
// keyCode = [block(3), F11, (F11&(F10|F9|F8)) | (!F11&F10&F9&F8)]
func computeKeyCode(fNum uint16, block uint8) uint8 {
	f11 := (fNum >> 10) & 1
	f10 := (fNum >> 9) & 1
	f9 := (fNum >> 8) & 1
	f8 := (fNum >> 7) & 1

	bit1 := f11
	bit0 := (f11 & (f10 | f9 | f8)) | ((1 ^ f11) & f10 & f9 & f8)

	return (block << 2) | uint8(bit1<<1) | uint8(bit0)
}

// GenerateSamples produces audio samples for the given number of M68K cycles.
func (y *YM2612) GenerateSamples(cycles int) {
	y.cycleAccum += cycles

	for y.cycleAccum >= 144 {
		y.cycleAccum -= 144
		y.nativeSampleCount++

		// Step timers
		y.stepTimers()

		// Step LFO
		y.stepLFOFull()

		// Step envelope generator every 3rd sample clock. On real hardware,
		// the EG operates on a 3-clock sub-cycle: the global counter
		// increments once per sub-cycle and each operator is evaluated once.
		y.egClock++
		if y.egClock >= 3 {
			y.egClock = 0
			y.egCounter++
			if y.egCounter >= 4096 {
				y.egCounter = 1 // 12-bit counter wraps to 1 (skips 0)
			}
			y.stepEnvelopesFull()
		}

		// Evaluate all channels and produce one native sample
		var left, right int32
		for ch := 0; ch < 6; ch++ {
			_, l, r := y.evaluateChannelFull(ch)
			left += int32(l)
			right += int32(r)
		}

		// Scale and clamp. With ladder offsets the 6-channel sum can
		// reach +/-49,728. Halving keeps the result within int16 range
		// (max +/-24,864) and allows headroom for PSG mixing.
		left >>= 1
		right >>= 1
		left = clampInt32(left, -32768, 32767)
		right = clampInt32(right, -32768, 32767)

		// Bresenham resample from native rate (~53kHz) to output sampleRate (48kHz)
		y.resampAccum += y.sampleRate
		if y.resampAccum >= y.nativeClock {
			y.resampAccum -= y.nativeClock
			y.buffer = append(y.buffer, int16(left), int16(right))
		}
	}
}

// GetBuffer returns accumulated samples and resets the buffer.
func (y *YM2612) GetBuffer() []int16 {
	out := y.buffer
	y.buffer = y.buffer[:0]
	return out
}
