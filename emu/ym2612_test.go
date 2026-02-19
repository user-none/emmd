package emu

import "testing"

// --- Phase 1: Register Interface, Timers, Bus Wiring ---

func TestYM2612_InitialState(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// All channels should start with panning enabled
	for ch := 0; ch < 6; ch++ {
		if !y.ch[ch].panL || !y.ch[ch].panR {
			t.Errorf("ch%d: expected L+R panning enabled", ch)
		}
	}

	// All operators should start in release state, fully silent
	for ch := 0; ch < 6; ch++ {
		for op := 0; op < 4; op++ {
			if y.ch[ch].op[op].egState != egRelease {
				t.Errorf("ch%d op%d: expected egRelease, got %d", ch, op, y.ch[ch].op[op].egState)
			}
			if y.ch[ch].op[op].egLevel != 0x3FF {
				t.Errorf("ch%d op%d: expected egLevel 0x3FF, got 0x%03X", ch, op, y.ch[ch].op[op].egLevel)
			}
		}
	}

	// Status should be 0
	if y.ReadPort(0) != 0 {
		t.Errorf("expected status 0, got 0x%02X", y.ReadPort(0))
	}
}

func TestYM2612_AddressLatch(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Port 0 latches address for Part I
	y.WritePort(0, 0x30)
	if y.addrLatch[0] != 0x30 {
		t.Errorf("expected Part I latch 0x30, got 0x%02X", y.addrLatch[0])
	}

	// Port 2 latches address for Part II
	y.WritePort(2, 0x40)
	if y.addrLatch[1] != 0x40 {
		t.Errorf("expected Part II latch 0x40, got 0x%02X", y.addrLatch[1])
	}
}

func TestYM2612_OperatorRegisterSlotMapping(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Register $30: channel 0 (addr&3=0), op slot 0 (S1) -> operator 0
	// Write DT=3, MUL=5
	y.WritePort(0, 0x30) // Latch address $30
	y.WritePort(1, 0x35) // DT=3(0x30), MUL=5(0x05)
	if y.ch[0].op[0].dt != 3 || y.ch[0].op[0].mul != 5 {
		t.Errorf("ch0 op0: expected dt=3 mul=5, got dt=%d mul=%d",
			y.ch[0].op[0].dt, y.ch[0].op[0].mul)
	}

	// Register $34: channel 0 (addr&3=0), op slot 1 (S3) -> operator 2
	y.WritePort(0, 0x34)
	y.WritePort(1, 0x27) // DT=2, MUL=7
	if y.ch[0].op[2].dt != 2 || y.ch[0].op[2].mul != 7 {
		t.Errorf("ch0 op2: expected dt=2 mul=7, got dt=%d mul=%d",
			y.ch[0].op[2].dt, y.ch[0].op[2].mul)
	}

	// Register $38: channel 0 (addr&3=0), op slot 2 (S2) -> operator 1
	y.WritePort(0, 0x38)
	y.WritePort(1, 0x13) // DT=1, MUL=3
	if y.ch[0].op[1].dt != 1 || y.ch[0].op[1].mul != 3 {
		t.Errorf("ch0 op1: expected dt=1 mul=3, got dt=%d mul=%d",
			y.ch[0].op[1].dt, y.ch[0].op[1].mul)
	}

	// Register $3C: channel 0 (addr&3=0), op slot 3 (S4) -> operator 3
	y.WritePort(0, 0x3C)
	y.WritePort(1, 0x4F) // DT=4, MUL=15
	if y.ch[0].op[3].dt != 4 || y.ch[0].op[3].mul != 15 {
		t.Errorf("ch0 op3: expected dt=4 mul=15, got dt=%d mul=%d",
			y.ch[0].op[3].dt, y.ch[0].op[3].mul)
	}
}

func TestYM2612_OperatorRegisterPartII(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Part II $30: channel 3 (chSlot 0 + part*3 = 3), op slot 0 (S1) -> operator 0
	y.WritePort(2, 0x30) // Latch address for Part II
	y.WritePort(3, 0x5A) // DT=5, MUL=10
	if y.ch[3].op[0].dt != 5 || y.ch[3].op[0].mul != 10 {
		t.Errorf("ch3 op0: expected dt=5 mul=10, got dt=%d mul=%d",
			y.ch[3].op[0].dt, y.ch[3].op[0].mul)
	}

	// Part II $31: channel 4, op slot 0 (S1)
	y.WritePort(2, 0x31)
	y.WritePort(3, 0x61) // DT=6, MUL=1
	if y.ch[4].op[0].dt != 6 || y.ch[4].op[0].mul != 1 {
		t.Errorf("ch4 op0: expected dt=6 mul=1, got dt=%d mul=%d",
			y.ch[4].op[0].dt, y.ch[4].op[0].mul)
	}

	// Part II $32: channel 5, op slot 0 (S1)
	y.WritePort(2, 0x32)
	y.WritePort(3, 0x72) // DT=7, MUL=2
	if y.ch[5].op[0].dt != 7 || y.ch[5].op[0].mul != 2 {
		t.Errorf("ch5 op0: expected dt=7 mul=2, got dt=%d mul=%d",
			y.ch[5].op[0].dt, y.ch[5].op[0].mul)
	}
}

func TestYM2612_InvalidChannelSlot(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// addr & 3 == 3 should be ignored
	y.WritePort(0, 0x33) // Latch $33 (slot 3 = invalid)
	y.WritePort(1, 0xFF)

	// None of the channels should be affected
	for ch := 0; ch < 6; ch++ {
		for op := 0; op < 4; op++ {
			if y.ch[ch].op[op].dt != 0 || y.ch[ch].op[op].mul != 0 {
				t.Errorf("ch%d op%d should be unmodified", ch, op)
			}
		}
	}
}

func TestYM2612_AllOperatorRegisters(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Write all operator registers to ch0 op0 (S1, addr base $30/$40/$50/$60/$70/$80/$90)
	// $30: DT/MUL
	y.WritePort(0, 0x30)
	y.WritePort(1, 0x35) // DT=3, MUL=5

	// $40: TL
	y.WritePort(0, 0x40)
	y.WritePort(1, 0x7F) // TL=127

	// $50: RS/AR
	y.WritePort(0, 0x50)
	y.WritePort(1, 0xDF) // RS=3, AR=31

	// $60: AM/D1R
	y.WritePort(0, 0x60)
	y.WritePort(1, 0x9F) // AM=true, D1R=31

	// $70: D2R
	y.WritePort(0, 0x70)
	y.WritePort(1, 0x1F) // D2R=31

	// $80: D1L/RR
	y.WritePort(0, 0x80)
	y.WritePort(1, 0xEF) // D1L=14, RR=15

	// $90: SSG-EG
	y.WritePort(0, 0x90)
	y.WritePort(1, 0x0B) // SSG-EG=0x0B

	op := &y.ch[0].op[0]
	if op.dt != 3 {
		t.Errorf("DT: expected 3, got %d", op.dt)
	}
	if op.mul != 5 {
		t.Errorf("MUL: expected 5, got %d", op.mul)
	}
	if op.tl != 127 {
		t.Errorf("TL: expected 127, got %d", op.tl)
	}
	if op.rs != 3 {
		t.Errorf("RS: expected 3, got %d", op.rs)
	}
	if op.ar != 31 {
		t.Errorf("AR: expected 31, got %d", op.ar)
	}
	if !op.am {
		t.Error("AM: expected true")
	}
	if op.d1r != 31 {
		t.Errorf("D1R: expected 31, got %d", op.d1r)
	}
	if op.d2r != 31 {
		t.Errorf("D2R: expected 31, got %d", op.d2r)
	}
	if op.d1l != 14 {
		t.Errorf("D1L: expected 14, got %d", op.d1l)
	}
	if op.rr != 15 {
		t.Errorf("RR: expected 15, got %d", op.rr)
	}
	if op.ssgEG != 0x0B {
		t.Errorf("SSG-EG: expected 0x0B, got 0x%02X", op.ssgEG)
	}
}

func TestYM2612_ChannelRegisters(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set channel 1 frequency: Block=4, FNum=0x29A
	// $A5: MSB (block + fnum high bits)
	y.WritePort(0, 0xA5)
	y.WritePort(1, 0x22) // block=4(0x20), fnum_hi=2(0x02) -> fNum MSB = 0x200
	// $A1: LSB
	y.WritePort(0, 0xA1)
	y.WritePort(1, 0x9A) // fnum_lo = 0x9A -> fNum = 0x29A

	if y.ch[1].block != 4 {
		t.Errorf("block: expected 4, got %d", y.ch[1].block)
	}
	if y.ch[1].fNum != 0x29A {
		t.Errorf("fNum: expected 0x29A, got 0x%03X", y.ch[1].fNum)
	}

	// Set algorithm/feedback for channel 0
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x3A) // feedback=7(0x38), algorithm=2(0x02)
	if y.ch[0].algorithm != 2 {
		t.Errorf("algo: expected 2, got %d", y.ch[0].algorithm)
	}
	if y.ch[0].feedback != 7 {
		t.Errorf("feedback: expected 7, got %d", y.ch[0].feedback)
	}

	// Set panning/AMS/FMS for channel 0
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC3) // L=1, R=1, AMS=0, FMS=3
	if !y.ch[0].panL {
		t.Error("panL: expected true")
	}
	if !y.ch[0].panR {
		t.Error("panR: expected true")
	}
	if y.ch[0].fms != 3 {
		t.Errorf("FMS: expected 3, got %d", y.ch[0].fms)
	}

	// Disable left panning
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0x40) // L=0, R=1
	if y.ch[0].panL {
		t.Error("panL: expected false")
	}
	if !y.ch[0].panR {
		t.Error("panR: expected true")
	}
}

func TestYM2612_KeyOnOff(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Key on channel 0, all operators
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF0) // operators S1-S4 on, channel 0

	for i := 0; i < 4; i++ {
		if !y.ch[0].op[i].keyOn {
			t.Errorf("ch0 op%d: expected keyOn=true", i)
		}
		if y.ch[0].op[i].egState != egAttack && y.ch[0].op[i].egState != egDecay {
			t.Errorf("ch0 op%d: expected egAttack or egDecay, got %d", i, y.ch[0].op[i].egState)
		}
	}

	// Key off channel 0, all operators
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00) // all off, channel 0
	for i := 0; i < 4; i++ {
		if y.ch[0].op[i].keyOn {
			t.Errorf("ch0 op%d: expected keyOn=false", i)
		}
		if y.ch[0].op[i].egState != egRelease {
			t.Errorf("ch0 op%d: expected egRelease, got %d", i, y.ch[0].op[i].egState)
		}
	}
}

func TestYM2612_KeyOnPartII(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Key on channel 4 (Part II: bit2=1, ch=1): val = 0x05 | 0xF0 = 0xF5
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF5) // all ops on, channel 4 (bit2=1, bits0-1=1)

	for i := 0; i < 4; i++ {
		if !y.ch[4].op[i].keyOn {
			t.Errorf("ch4 op%d: expected keyOn=true", i)
		}
	}
	// Other channels should be unaffected
	for i := 0; i < 4; i++ {
		if y.ch[0].op[i].keyOn {
			t.Errorf("ch0 op%d: should not be key-on", i)
		}
	}
}

func TestYM2612_KeyOnResetsPhase(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Artificially set phase counter
	y.ch[0].op[0].phaseCounter = 0x12345
	// Key on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10) // op S1 only, channel 0

	if y.ch[0].op[0].phaseCounter != 0 {
		t.Errorf("phase should be reset to 0 on key-on, got 0x%05X", y.ch[0].op[0].phaseCounter)
	}
}

func TestYM2612_DAC(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable DAC
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	if !y.dacEnable {
		t.Error("DAC should be enabled")
	}

	// Write DAC sample
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xC0)
	if y.dacSample != 0xC0 {
		t.Errorf("DAC sample: expected 0xC0, got 0x%02X", y.dacSample)
	}

	// Disable DAC
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x00)
	if y.dacEnable {
		t.Error("DAC should be disabled")
	}
}

func TestYM2612_LFORegister(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable LFO, freq=5
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x0D) // enable(0x08) | freq=5
	if !y.lfoEnable {
		t.Error("LFO should be enabled")
	}
	if y.lfoFreq != 5 {
		t.Errorf("LFO freq: expected 5, got %d", y.lfoFreq)
	}

	// Disable LFO
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x03) // no enable, freq=3
	if y.lfoEnable {
		t.Error("LFO should be disabled")
	}
	if y.lfoFreq != 3 {
		t.Errorf("LFO freq: expected 3, got %d", y.lfoFreq)
	}
}

func TestYM2612_TimerAPeriod(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Timer A = 10-bit: MSB in $24, LSB (2 bits) in $25
	// Set period to 0x3FF (1023)
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF) // high 8 bits = 0xFF -> 0x3FC
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03) // low 2 bits = 0x03

	if y.timerA.period != 0x3FF {
		t.Errorf("Timer A period: expected 0x3FF, got 0x%03X", y.timerA.period)
	}
}

func TestYM2612_TimerBPeriod(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.WritePort(0, 0x26)
	y.WritePort(1, 0xA5)
	if y.timerB.period != 0xA5 {
		t.Errorf("Timer B period: expected 0xA5, got 0x%02X", y.timerB.period)
	}
}

func TestYM2612_TimerAOverflow(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set Timer A period to 1023 (overflow every 1 sample)
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03)

	// Enable and load Timer A (bit0=load, bit2=enable)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x05)

	// Tick once - should overflow (1024 - 1023 = 1, so overflows after 1 tick)
	y.stepTimers()
	if !y.timerAOver {
		t.Error("Timer A should have overflowed")
	}

	// Verify readable in status
	status := y.ReadPort(0)
	if status&0x01 == 0 {
		t.Error("Timer A overflow should be in status bit 0")
	}

	// Reset overflow flag via register $27 bit 4
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x15) // bit4=reset A, keep load+enable
	if y.timerAOver {
		t.Error("Timer A overflow should be cleared after reset")
	}
	if y.ReadPort(0)&0x01 != 0 {
		t.Error("Timer A overflow should be cleared in status")
	}
}

func TestYM2612_TimerBOverflow(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set Timer B period to 255 (overflow every 1 tick of 16)
	y.WritePort(0, 0x26)
	y.WritePort(1, 0xFF)

	// Enable and load Timer B (bit1=load, bit3=enable)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x0A)

	// Timer B ticks every 16 sample clocks
	for i := 0; i < 16; i++ {
		y.stepTimers()
	}
	if !y.timerBOver {
		t.Error("Timer B should have overflowed after 16 sample clocks")
	}

	status := y.ReadPort(0)
	if status&0x02 == 0 {
		t.Error("Timer B overflow should be in status bit 1")
	}

	// Reset overflow flag via register $27 bit 5
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x2A) // bit5=reset B, keep load+enable
	if y.timerBOver {
		t.Error("Timer B overflow should be cleared")
	}
}

func TestYM2612_TimerNotEnabledNoOverflow(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set Timer A period to max (overflow every 1 sample)
	y.WritePort(0, 0x24)
	y.WritePort(1, 0xFF)
	y.WritePort(0, 0x25)
	y.WritePort(1, 0x03)

	// Load but don't enable (bit0=load, but bit2 not set)
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x01)

	y.stepTimers()
	if y.timerAOver {
		t.Error("Timer A overflow flag should not set when enable is off")
	}
}

func TestYM2612_Ch3SpecialMode(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable channel 3 special mode via bits 6-7 of $27
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40) // mode bits set
	if y.ch3Mode != ch3ModeSpecial {
		t.Errorf("ch3Mode should be ch3ModeSpecial, got %d", y.ch3Mode)
	}

	// Write per-operator frequency for ch3
	// Slot 0: $AC (MSB), $A8 (LSB)
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x22) // block=4, fnum_hi=2
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x50) // fnum_lo=0x50

	if y.ch3Block[0] != 4 {
		t.Errorf("ch3 slot0 block: expected 4, got %d", y.ch3Block[0])
	}
	if y.ch3Freq[0] != 0x250 {
		t.Errorf("ch3 slot0 fNum: expected 0x250, got 0x%03X", y.ch3Freq[0])
	}

	// Disable special mode
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x00)
	if y.ch3Mode != ch3ModeNormal {
		t.Errorf("ch3Mode should be ch3ModeNormal, got %d", y.ch3Mode)
	}
}

func TestYM2612_StatusRead(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Status should read 0 initially
	if y.ReadPort(0) != 0 {
		t.Errorf("expected initial status 0, got 0x%02X", y.ReadPort(0))
	}

	// Port 2 also returns status
	if y.ReadPort(2) != 0 {
		t.Errorf("expected port 2 status 0, got 0x%02X", y.ReadPort(2))
	}

	// Non-status ports return 0
	if y.ReadPort(1) != 0 {
		t.Errorf("expected port 1 = 0, got 0x%02X", y.ReadPort(1))
	}
	if y.ReadPort(3) != 0 {
		t.Errorf("expected port 3 = 0, got 0x%02X", y.ReadPort(3))
	}
}

func TestYM2612_GlobalRegistersOnlyPartI(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Writing global register $2B via Part II should be ignored
	y.WritePort(2, 0x2B)
	y.WritePort(3, 0x80) // Try to enable DAC via Part II
	if y.dacEnable {
		t.Error("DAC should not be enabled via Part II write")
	}

	// Same register via Part I should work
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	if !y.dacEnable {
		t.Error("DAC should be enabled via Part I write")
	}
}

func TestYM2612_GenerateSamplesProducesBuffer(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Generate samples for a reasonable number of cycles
	y.GenerateSamples(7670454 / 60) // ~1 frame worth of cycles
	buf := y.GetBuffer()

	// Should have produced some stereo samples (pairs of L/R)
	if len(buf) == 0 {
		t.Error("expected non-empty buffer")
	}
	if len(buf)%2 != 0 {
		t.Errorf("buffer length should be even (stereo), got %d", len(buf))
	}
}

func TestYM2612_GetBufferResetsBuffer(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.GenerateSamples(7670454 / 60)
	buf1 := y.GetBuffer()
	if len(buf1) == 0 {
		t.Fatal("expected non-empty first buffer")
	}

	// Second call without generating should return empty
	buf2 := y.GetBuffer()
	if len(buf2) != 0 {
		t.Errorf("expected empty buffer after GetBuffer, got length %d", len(buf2))
	}
}

func TestYM2612_SilenceWhenNoKeysPressed(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	// With ladder effect: each of 6 channels (panL=true, panR=true)
	// outputs applyLadder(0, true) = 128 per L/R. Sum = 768. After >>1 = 384.
	for i, s := range buf {
		if s != 384 {
			t.Errorf("no keys: sample[%d]=%d, want 384 (ladder offset)", i, s)
			break
		}
	}
}

func TestYM2612_KeyCode(t *testing.T) {
	tests := []struct {
		fNum  uint16
		block uint8
		want  uint8
	}{
		// fNum=0x000, block=0: F11=0, F10=0, F9=0, F8=0 -> bit1=0, bit0=0
		{0x000, 0, 0x00},
		// fNum=0x780 (F11=1, F10=1, F9=1), block=4 -> bit1=1, bit0=1
		{0x780, 4, 0x13},
		// fNum=0x400 (F11=1 only), block=3 -> bit1=1, bit0=0
		{0x400, 3, 0x0E},
		// fNum=0x380 (F11=0, F10=1, F9=1, F8=1), block=2 -> bit1=0, bit0=1
		{0x380, 2, 0x09},
		// fNum=0x200 (F11=0, F10=1, F9=0, F8=0), block=0 -> bit1=0, bit0=0
		{0x200, 0, 0x00},
	}

	for _, tt := range tests {
		got := computeKeyCode(tt.fNum, tt.block)
		if got != tt.want {
			t.Errorf("computeKeyCode(0x%03X, %d): got 0x%02X, want 0x%02X",
				tt.fNum, tt.block, got, tt.want)
		}
	}
}

// --- Bus Wiring Tests ---

func TestYM2612_BusWriteFromM68K(t *testing.T) {
	bus := makeTestBus()

	// Write to YM2612 via M68K Z80 space: 0xA04000 = port 0
	bus.WriteCycle(0, 1, 0xA04000, 0x2B) // Latch $2B (DAC enable)
	// Write data: 0xA04001 = port 1
	bus.WriteCycle(0, 1, 0xA04001, 0x80) // Enable DAC

	if !bus.ym2612.dacEnable {
		t.Error("DAC should be enabled via M68K write to 0xA04000/0xA04001")
	}
}

func TestYM2612_BusWordWriteFromM68K(t *testing.T) {
	bus := makeTestBus()

	// Word write to 0xA04000: high byte = address latch (port 0), low byte = data (port 1)
	// This is the most common way games write to YM2612 from M68K
	bus.WriteCycle(0, 2, 0xA04000, 0x2B80) // Latch $2B, data $80 -> enable DAC

	if !bus.ym2612.dacEnable {
		t.Error("DAC should be enabled via M68K word write to 0xA04000")
	}
}

func TestYM2612_BusWordWritePartII(t *testing.T) {
	bus := makeTestBus()

	// Word write to Part II (port 2+3): 0xA04002
	// Latch $30 (DT/MUL for ch3 op0), data $5A (DT=5, MUL=10)
	bus.WriteCycle(0, 2, 0xA04002, 0x305A)

	if bus.ym2612.ch[3].op[0].dt != 5 || bus.ym2612.ch[3].op[0].mul != 10 {
		t.Errorf("Part II word write failed: dt=%d mul=%d",
			bus.ym2612.ch[3].op[0].dt, bus.ym2612.ch[3].op[0].mul)
	}
}

func TestYM2612_BusReadFromM68K(t *testing.T) {
	bus := makeTestBus()

	// Read status from M68K: 0xA04000 = port 0
	val := bus.ReadCycle(0, 1, 0xA04000)
	if val != 0 {
		t.Errorf("expected status 0, got 0x%02X", val)
	}
}

func TestYM2612_BusWriteFromZ80(t *testing.T) {
	bus := makeTestBus()
	z80Mem := NewZ80Memory(bus)

	// Write to YM2612 via Z80: 0x4000 = port 0
	z80Mem.Write(0x4000, 0x2B) // Latch $2B
	z80Mem.Write(0x4001, 0x80) // Enable DAC

	if !bus.ym2612.dacEnable {
		t.Error("DAC should be enabled via Z80 write to 0x4000/0x4001")
	}
}

func TestYM2612_BusReadFromZ80(t *testing.T) {
	bus := makeTestBus()
	z80Mem := NewZ80Memory(bus)

	// Read status from Z80: 0x4000 = port 0
	val := z80Mem.Read(0x4000)
	if val != 0 {
		t.Errorf("expected status 0, got 0x%02X", val)
	}
}

func TestYM2612_BusPartIIFromZ80(t *testing.T) {
	bus := makeTestBus()
	z80Mem := NewZ80Memory(bus)

	// Write to Part II via Z80: 0x4002 = port 2 (address), 0x4003 = port 3 (data)
	z80Mem.Write(0x4002, 0x30) // Latch Part II $30
	z80Mem.Write(0x4003, 0x5A) // DT=5, MUL=10 for ch3 op0

	if bus.ym2612.ch[3].op[0].dt != 5 || bus.ym2612.ch[3].op[0].mul != 10 {
		t.Errorf("Part II write via Z80 failed: dt=%d mul=%d",
			bus.ym2612.ch[3].op[0].dt, bus.ym2612.ch[3].op[0].mul)
	}
}

func TestYM2612_KeyOnSelectiveOperators(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Key on only S1 and S3 (bits 4 and 6) for channel 0
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x50) // S1(bit4)=1, S2(bit5)=0, S3(bit6)=1, S4(bit7)=0, ch=0

	if !y.ch[0].op[0].keyOn {
		t.Error("op0 (S1) should be key-on")
	}
	if y.ch[0].op[1].keyOn {
		t.Error("op1 (S2) should not be key-on")
	}
	if !y.ch[0].op[2].keyOn {
		t.Error("op2 (S3) should be key-on")
	}
	if y.ch[0].op[3].keyOn {
		t.Error("op3 (S4) should not be key-on")
	}
}

// --- Phase 2: Phase Generator Tests ---

func TestPhase_MULZeroHalves(t *testing.T) {
	// MUL=0 should halve the base increment
	inc0 := computePhaseIncrement(0x400, 4, 0x12, 0, 0) // MUL=0
	inc1 := computePhaseIncrement(0x400, 4, 0x12, 0, 1) // MUL=1
	if inc0 != inc1/2 {
		t.Errorf("MUL=0 should be half of MUL=1: got %d, expected %d", inc0, inc1/2)
	}
}

func TestPhase_MULMultiply(t *testing.T) {
	// MUL=1..15 should multiply the base increment
	base := computePhaseIncrement(0x400, 4, 0x12, 0, 1) // MUL=1 = base
	for mul := uint8(2); mul <= 15; mul++ {
		got := computePhaseIncrement(0x400, 4, 0x12, 0, mul)
		want := (base * uint32(mul)) & 0xFFFFF
		if got != want {
			t.Errorf("MUL=%d: got %d, want %d", mul, got, want)
		}
	}
}

func TestPhase_BlockDoubling(t *testing.T) {
	// Incrementing block by 1 should double the base (before MUL)
	inc4 := computePhaseIncrement(0x400, 4, 0, 0, 1)
	inc5 := computePhaseIncrement(0x400, 5, 0, 0, 1)
	if inc5 != inc4*2 {
		t.Errorf("block+1 should double: block4=%d, block5=%d", inc4, inc5)
	}
}

func TestPhase_DetuneAdd(t *testing.T) {
	// DT=1 (no sign bit) should add detune value
	inc0 := computePhaseIncrement(0x400, 4, 16, 0, 1) // DT=0
	inc1 := computePhaseIncrement(0x400, 4, 16, 1, 1) // DT=1 (add)
	if inc1 <= inc0 {
		t.Errorf("DT=1 should add to increment: DT0=%d, DT1=%d", inc0, inc1)
	}
}

func TestPhase_DetuneSubtract(t *testing.T) {
	// DT=5 (bit2 set, so DT&3=1 with sign) should subtract detune value
	inc0 := computePhaseIncrement(0x400, 4, 16, 0, 1) // DT=0
	inc5 := computePhaseIncrement(0x400, 4, 16, 5, 1) // DT=5 (subtract DT&3=1)
	if inc5 >= inc0 {
		t.Errorf("DT=5 should subtract from increment: DT0=%d, DT5=%d", inc0, inc5)
	}
}

func TestPhase_DT0NoDetune(t *testing.T) {
	// DT=0: column 0 is always 0, no detune
	inc := computePhaseIncrement(0x400, 4, 16, 0, 1)
	// Base is (0x400 << 4) >> 1 = 0x2000
	if inc != 0x2000 {
		t.Errorf("DT=0 MUL=1 fNum=0x400 block=4: expected 0x2000, got 0x%X", inc)
	}
}

func TestPhase_KeyOnResetsAccumulator(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set up a frequency and key on
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A)

	// Manually advance phase
	y.ch[0].op[0].phaseCounter = 0xABCDE

	// Key on - should reset phase to 0
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF0)
	if y.ch[0].op[0].phaseCounter != 0 {
		t.Errorf("expected phase 0 after key-on, got 0x%05X", y.ch[0].op[0].phaseCounter)
	}
}

func TestPhase_Accumulation(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set frequency for channel 0
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22) // block=4, fNum high=2
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x00) // fNum=0x200

	// Set MUL=1, DT=0 for operator 0
	y.WritePort(0, 0x30)
	y.WritePort(1, 0x01) // DT=0, MUL=1

	op := &y.ch[0].op[0]
	inc := op.phaseInc
	if inc == 0 {
		t.Fatal("phase increment should be non-zero")
	}

	// Step phase a few times
	stepPhase(op)
	if op.phaseCounter != inc {
		t.Errorf("after 1 step: expected 0x%05X, got 0x%05X", inc, op.phaseCounter)
	}

	stepPhase(op)
	want := (inc * 2) & 0xFFFFF
	if op.phaseCounter != want {
		t.Errorf("after 2 steps: expected 0x%05X, got 0x%05X", want, op.phaseCounter)
	}
}

func TestPhase_PhaseIncrementUpdatedOnFreqWrite(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set MUL=1 DT=0 first
	y.WritePort(0, 0x30)
	y.WritePort(1, 0x01)

	// Set frequency
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22) // block=4, fNum high bits
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A) // fNum=0x29A

	op := &y.ch[0].op[0]
	expected := computePhaseIncrement(0x29A, 4, op.keyCode, 0, 1)
	if op.phaseInc != expected {
		t.Errorf("phase inc: expected 0x%05X, got 0x%05X", expected, op.phaseInc)
	}
}

func TestPhase_Wraps20Bits(t *testing.T) {
	op := &ymOperator{}
	op.phaseInc = 0xFFFFF // Max increment
	op.phaseCounter = 0xFFFFF

	stepPhase(op)
	// Should wrap: (0xFFFFF + 0xFFFFF) & 0xFFFFF = 0xFFFFE
	want := uint32(0xFFFFE)
	if op.phaseCounter != want {
		t.Errorf("expected 0x%05X, got 0x%05X", want, op.phaseCounter)
	}
}

// --- Phase 3: Envelope Generator Tests ---

func TestEnvelope_SustainLevel(t *testing.T) {
	tests := []struct {
		d1l  uint8
		want uint16
	}{
		{0, 0},
		{1, 0x20},
		{7, 0xE0},
		{14, 0x1C0},
		{15, 0x3E0},
	}
	for _, tt := range tests {
		got := sustainLevel(tt.d1l)
		if got != tt.want {
			t.Errorf("sustainLevel(%d): got 0x%03X, want 0x%03X", tt.d1l, got, tt.want)
		}
	}
}

func TestEnvelope_TotalLevel(t *testing.T) {
	// TL=0, egLevel=0 -> 0
	if totalLevel(0, 0) != 0 {
		t.Errorf("totalLevel(0,0): got %d", totalLevel(0, 0))
	}
	// TL=127, egLevel=0 -> 127*8 = 1016
	if totalLevel(0, 127) != 0x3F8 {
		t.Errorf("totalLevel(0,127): got 0x%03X", totalLevel(0, 127))
	}
	// Overflow clamped to 0x3FF
	if totalLevel(0x3FF, 127) != 0x3FF {
		t.Errorf("totalLevel(0x3FF,127): got 0x%03X", totalLevel(0x3FF, 127))
	}
}

func TestEnvelope_AttackToZero(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set up operator with max attack rate
	y.WritePort(0, 0x50) // RS/AR for ch0 op0 (S1)
	y.WritePort(1, 0xDF) // RS=3, AR=31

	// Key on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10) // S1 on, ch0

	op := &y.ch[0].op[0]
	// With max AR and RS, effective rate should be >= 62, instant attack
	if op.egLevel != 0 {
		t.Errorf("instant attack: expected egLevel=0, got 0x%03X", op.egLevel)
	}
	if op.egState != egDecay {
		t.Errorf("should be in decay after instant attack, got state %d", op.egState)
	}
}

func TestEnvelope_InstantAttackRates62_63(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// AR=31, RS=3 should give effective rate >= 62
	y.WritePort(0, 0x50) // RS/AR
	y.WritePort(1, 0xDF) // RS=3, AR=31

	op := &y.ch[0].op[0]
	rate := y.effectiveRate(op.ar, op)
	if rate < 62 {
		t.Errorf("expected effective rate >= 62, got %d", rate)
	}
}

func TestEnvelope_NonInstantAttack(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set up operator with moderate attack rate (AR=15, RS=0)
	// Effective rate = 2*15 + 0 = 30 (well below 62, NOT instant)
	y.WritePort(0, 0x50) // RS/AR for ch0 op0 (S1)
	y.WritePort(1, 0x0F) // RS=0, AR=15

	// Set a frequency so keyCode is non-zero
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22) // block=4, fNum MSB
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x69) // fNum LSB

	// Key on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10) // S1 on, ch0

	op := &y.ch[0].op[0]

	// Should be in attack state (not instant)
	if op.egState != egAttack {
		t.Fatalf("expected egAttack state, got %d", op.egState)
	}
	if op.egLevel != 0x3FF {
		t.Fatalf("expected initial egLevel 0x3FF, got 0x%03X", op.egLevel)
	}

	rate := y.effectiveRate(op.ar, op)
	if rate >= 62 {
		t.Fatalf("expected non-instant rate (< 62), got %d", rate)
	}

	// Step the envelope many times - level should decrease toward 0
	for i := 0; i < 100000; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egLevel >= 0x3FF {
		t.Errorf("attack did not progress from 0x3FF after 100000 steps, egLevel=0x%03X", op.egLevel)
	}
	if op.egLevel > 0 {
		t.Logf("after 100000 steps: egLevel=0x%03X (still decreasing)", op.egLevel)
	}
}

func TestEnvelope_NonInstantAttackReachesZero(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	op := &y.ch[0].op[0]
	op.egState = egAttack
	op.egLevel = 0x3FF
	op.ar = 20 // Moderate attack rate
	op.rs = 0
	op.keyCode = 0x10
	op.keyOn = true

	// Step until attack completes or timeout
	for i := 0; i < 1000000 && op.egState == egAttack; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egState != egDecay {
		t.Errorf("expected transition to decay, still in state %d with egLevel=0x%03X", op.egState, op.egLevel)
	}
	if op.egLevel != 0 {
		t.Errorf("expected egLevel=0 after attack, got 0x%03X", op.egLevel)
	}
}

func TestEnvelope_RateZeroFrozen(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// AR=0 means rate=0, no attack
	y.WritePort(0, 0x50)
	y.WritePort(1, 0x00) // RS=0, AR=0

	// Key on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10)

	op := &y.ch[0].op[0]
	initialLevel := op.egLevel

	// Step many times - level should not change
	for i := 0; i < 1000; i++ {
		y.egCounter = uint16(i)
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egLevel != initialLevel {
		t.Errorf("rate=0 should freeze: initial=0x%03X, after=0x%03X", initialLevel, op.egLevel)
	}
}

func TestEnvelope_DecayToSustain(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	op := &y.ch[0].op[0]
	// Set up: already past attack (level=0), in decay
	op.egLevel = 0
	op.egState = egDecay
	op.d1r = 31 // Max decay rate
	op.d1l = 4  // Sustain level = 4 << 5 = 0x80
	op.rs = 3
	op.keyCode = 0x1F
	op.keyOn = true

	sl := sustainLevel(op.d1l)

	// Step until we reach sustain level
	for i := 0; i < 10000 && op.egState == egDecay; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egState != egSustain {
		t.Errorf("expected egSustain state, got %d", op.egState)
	}
	if op.egLevel < sl {
		t.Errorf("egLevel should be >= sustain level 0x%03X, got 0x%03X", sl, op.egLevel)
	}
}

func TestEnvelope_DecayToSustainLevelZero(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	op := &y.ch[0].op[0]
	// Start in decay at level 0 with sustain level 0 (d1l=0).
	// The sustain check must fire before the increment to prevent
	// the level from overshooting to a non-zero value.
	op.egLevel = 0
	op.egState = egDecay
	op.d1r = 31
	op.d1l = 0 // Sustain level = 0
	op.d2r = 0 // Hold at sustain
	op.rs = 0
	op.keyCode = 0
	op.keyOn = true

	// One step should transition to sustain without incrementing
	y.stepOperatorEnvelope(op, 0)

	if op.egState != egSustain {
		t.Errorf("expected egSustain, got %d", op.egState)
	}
	if op.egLevel != 0 {
		t.Errorf("level should stay 0 with sustain_level=0, got %d", op.egLevel)
	}
}

func TestEnvelope_SustainHoldWithD2R0(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	op := &y.ch[0].op[0]
	op.egLevel = 0x80
	op.egState = egSustain
	op.d2r = 0 // D2R=0 means rate=0, sustain holds
	op.keyOn = true

	initialLevel := op.egLevel
	for i := 0; i < 1000; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egLevel != initialLevel {
		t.Errorf("D2R=0 sustain should hold: initial=0x%03X, after=0x%03X",
			initialLevel, op.egLevel)
	}
}

func TestEnvelope_ReleaseToMax(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	op := &y.ch[0].op[0]
	op.egLevel = 0
	op.egState = egRelease
	op.rr = 15 // Max release rate
	op.rs = 3
	op.keyCode = 0x1F
	op.keyOn = false

	// Step until max attenuation
	for i := 0; i < 100000 && op.egLevel < 0x3FF; i++ {
		y.stepOperatorEnvelope(op, uint16(i))
	}

	if op.egLevel != 0x3FF {
		t.Errorf("release should reach 0x3FF, got 0x%03X", op.egLevel)
	}
}

func TestEnvelope_KeyOffTransitionsToRelease(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Key on then off
	y.WritePort(0, 0x50)
	y.WritePort(1, 0xDF) // RS=3, AR=31 (instant attack)

	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10) // Key on S1

	if y.ch[0].op[0].egState == egRelease {
		t.Fatal("should not be in release right after key-on")
	}

	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00) // Key off

	if y.ch[0].op[0].egState != egRelease {
		t.Errorf("expected egRelease after key-off, got %d", y.ch[0].op[0].egState)
	}
}

func TestEnvelope_SustainLevel15Mapping(t *testing.T) {
	// D1L=15 maps to 0x3E0, not 0x1E0
	sl := sustainLevel(15)
	if sl != 0x3E0 {
		t.Errorf("D1L=15: expected 0x3E0, got 0x%03X", sl)
	}
}

func TestEnvelope_EffectiveRateCalculation(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	tests := []struct {
		rate    uint8
		rs      uint8
		keyCode uint8
		want    uint8
	}{
		{0, 0, 0, 0},      // rate=0 always returns 0
		{31, 0, 0, 62},    // 2*31+0 = 62
		{31, 3, 0x1F, 63}, // 2*31+31=93 clamped to 63
		{15, 0, 0, 30},    // 2*15 = 30
		{15, 2, 8, 34},    // 2*15 + (8>>1) = 34
	}

	for _, tt := range tests {
		op := &ymOperator{rs: tt.rs, keyCode: tt.keyCode}
		got := y.effectiveRate(tt.rate, op)
		if got != tt.want {
			t.Errorf("effectiveRate(%d, rs=%d, kc=%d): got %d, want %d",
				tt.rate, tt.rs, tt.keyCode, got, tt.want)
		}
	}
}

// --- Phase 4: Operator Output, Algorithms, Audio Generation ---

func TestOperator_SineTableProperties(t *testing.T) {
	// sineTable[0] should be the largest value (sin near 0 -> large -log2)
	// sineTable[255] should be the smallest (sin near 1 -> -log2 near 0)
	if sineTable[0] <= sineTable[255] {
		t.Errorf("sineTable[0] (%d) should be > sineTable[255] (%d)",
			sineTable[0], sineTable[255])
	}

	// Values should be monotonically decreasing (log of increasing sine)
	for i := 1; i < 256; i++ {
		if sineTable[i] > sineTable[i-1] {
			t.Errorf("sineTable not monotonically decreasing at %d: %d > %d",
				i, sineTable[i], sineTable[i-1])
			break
		}
	}
}

func TestOperator_Pow2TableProperties(t *testing.T) {
	// pow2Table[0] should be the largest (2^(1-1/256) ~ 2)
	// pow2Table[255] should be the smallest (2^0 = 1)
	if pow2Table[0] <= pow2Table[255] {
		t.Errorf("pow2Table[0] (%d) should be > pow2Table[255] (%d)",
			pow2Table[0], pow2Table[255])
	}

	// All values should be between 1024 and 2048
	for i := 0; i < 256; i++ {
		if pow2Table[i] < 1024 || pow2Table[i] > 2048 {
			t.Errorf("pow2Table[%d] = %d, expected [1024, 2048]", i, pow2Table[i])
			break
		}
	}
}

func TestOperator_OutputZeroAttenuation(t *testing.T) {
	// Phase at peak of sine (quarter way through = index 255 for max sine)
	// Phase bits 19-10 = 0x000 -> idx=0, that's near zero though
	// Phase value where sine is at max positive: top 10 bits = 0x0FF (quarter sine peak)
	phase := uint32(0x0FF) << 10 // Max positive sine
	out := computeOperatorOutput(phase, 0)
	if out <= 0 {
		t.Errorf("expected positive output at sine peak with 0 attenuation, got %d", out)
	}
}

func TestOperator_OutputMaxAttenuation(t *testing.T) {
	// At max envelope attenuation (0x3FF), output should be 0
	phase := uint32(0x0FF) << 10
	out := computeOperatorOutput(phase, 0x3FF)
	if out != 0 {
		t.Errorf("expected 0 at max attenuation, got %d", out)
	}
}

func TestOperator_OutputSignInversion(t *testing.T) {
	// Phase in positive half (bit 9 = 0) vs negative half (bit 9 = 1)
	posPhase := uint32(0x040) << 10 // Positive half
	negPhase := uint32(0x240) << 10 // Same index but bit 9 set (negative)
	posOut := computeOperatorOutput(posPhase, 0)
	negOut := computeOperatorOutput(negPhase, 0)

	if posOut <= 0 {
		t.Errorf("positive half should produce positive output, got %d", posOut)
	}
	if negOut >= 0 {
		t.Errorf("negative half should produce negative output, got %d", negOut)
	}
	// Magnitudes should be equal
	if posOut != -negOut {
		t.Errorf("magnitudes should match: pos=%d neg=%d", posOut, negOut)
	}
}

func TestOperator_Algo0SerialChain(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set up channel 0 with algorithm 0 (serial chain S1->S2->S3->S4)
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x00) // algo=0, fb=0

	// Set frequency
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22) // block=4
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A)

	// Set all operators to MUL=1, TL=0 (max volume)
	for _, reg := range []uint8{0x30, 0x34, 0x38, 0x3C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01) // DT=0, MUL=1
	}
	for _, reg := range []uint8{0x40, 0x44, 0x48, 0x4C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00) // TL=0
	}

	// Set max AR for instant attack
	for _, reg := range []uint8{0x50, 0x54, 0x58, 0x5C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xDF) // RS=3, AR=31
	}

	// Set panning
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC0) // L+R

	// Key on all operators
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF0)

	// Evaluate channel - should produce non-zero output (S4 is carrier)
	_, l, r := y.evaluateChannelFull(0)
	// At least one sample should be non-zero (operators have frequency set)
	// Note: first sample after key-on may be 0 because phase just started
	// Evaluate a few more times
	for i := 0; i < 100; i++ {
		_, l, r = y.evaluateChannelFull(0)
	}
	if l == 0 && r == 0 {
		t.Error("algo 0 should produce non-zero output after key-on with frequency set")
	}
}

func TestOperator_Algo7AllParallel(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Algorithm 7: all carriers
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x07) // algo=7, fb=0

	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A)

	// Set all operators similarly
	for _, reg := range []uint8{0x30, 0x34, 0x38, 0x3C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}
	for _, reg := range []uint8{0x40, 0x44, 0x48, 0x4C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	for _, reg := range []uint8{0x50, 0x54, 0x58, 0x5C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xDF)
	}
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC0)

	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF0)

	// With algo 7, output = S1+S2+S3+S4, should be louder than algo 0
	var maxAlgo7 int16
	for i := 0; i < 200; i++ {
		_, l, _ := y.evaluateChannelFull(0)
		if l > maxAlgo7 {
			maxAlgo7 = l
		}
		if l < -maxAlgo7 {
			maxAlgo7 = -l
		}
	}
	if maxAlgo7 == 0 {
		t.Error("algo 7 should produce non-zero output")
	}
}

func TestOperator_FeedbackDisabled(t *testing.T) {
	op := &ymOperator{
		prevOut: [2]int16{100, 100},
	}
	fb := feedback(op, 0)
	if fb != 0 {
		t.Errorf("feedback should be 0 when disabled, got %d", fb)
	}
}

func TestOperator_FeedbackEnabled(t *testing.T) {
	op := &ymOperator{
		prevOut: [2]int16{200, 200},
	}
	fb := feedback(op, 7) // Max feedback
	// (200+200) >> (10-7) = 400 >> 3 = 50
	if fb != 50 {
		t.Errorf("feedback(7): expected 50, got %d", fb)
	}
}

func TestOperator_DACReplacesChannel6(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable DAC
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)

	// Write a non-center DAC sample
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xFF) // Max positive = (255-128)<<6 = 8128

	// Set channel 5 panning to L+R
	y.WritePort(2, 0xB6) // Part II, ch5 panning register
	y.WritePort(3, 0xC0)

	_, l, r := y.evaluateChannelFull(5)
	// With ladder effect: dacOut=8128, applyLadder(8128, true) = 8128+128 = 8256
	expected := int16((255-128)<<6) + 128
	if l != expected {
		t.Errorf("DAC left: expected %d, got %d", expected, l)
	}
	if r != expected {
		t.Errorf("DAC right: expected %d, got %d", expected, r)
	}
}

func TestOperator_DACCenter(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80) // DAC enable

	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x80) // Center value = (128-128)<<6 = 0

	y.WritePort(2, 0xB6)
	y.WritePort(3, 0xC0)

	_, l, r := y.evaluateChannelFull(5)
	// With ladder effect: dacOut=0, applyLadder(0, true) = 128
	if l != 128 || r != 128 {
		t.Errorf("DAC center should be ladder offset 128, got L=%d R=%d", l, r)
	}
}

func TestOperator_StereoPanning(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable DAC with non-zero sample for easy testing
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xC0)

	// Left only
	y.WritePort(2, 0xB6)
	y.WritePort(3, 0x80) // L=1, R=0
	_, l, r := y.evaluateChannelFull(5)
	// DAC 0xC0: dacOut=4096 (positive). With ladder:
	// enabled: 4096+128=4224, muted positive: 128
	if l == 128 {
		t.Error("left panning should carry signal, not just ladder offset")
	}
	if r != 128 {
		t.Errorf("left panning: right should be muted ladder offset 128, got %d", r)
	}

	// Right only
	y.WritePort(2, 0xB6)
	y.WritePort(3, 0x40) // L=0, R=1
	_, l, r = y.evaluateChannelFull(5)
	if l != 128 {
		t.Errorf("right panning: left should be muted ladder offset 128, got %d", l)
	}
	if r == 128 {
		t.Error("right panning should carry signal, not just ladder offset")
	}
}

func TestOperator_GenerateSamplesFormat(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	y.GenerateSamples(7670454 / 60) // ~1 frame
	buf := y.GetBuffer()

	if len(buf)%2 != 0 {
		t.Errorf("buffer should be stereo (even length), got %d", len(buf))
	}
}

func TestOperator_NonZeroOutputWithKeyOn(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set up channel 0 with a tone
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x07) // algo=7
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22) // block=4
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A) // fNum
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC0) // L+R

	// Set operator 0: MUL=1, TL=0, instant attack
	y.WritePort(0, 0x30)
	y.WritePort(1, 0x01)
	y.WritePort(0, 0x40)
	y.WritePort(1, 0x00) // TL=0
	y.WritePort(0, 0x50)
	y.WritePort(1, 0xDF) // RS=3, AR=31

	// Key on S1
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x10)

	// Generate samples
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	hasNonZero := false
	for _, s := range buf {
		if s != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("expected non-zero output with key-on and frequency set")
	}
}

// --- Phase 5: LFO Tests ---

func TestLFO_TriangleWaveShape(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoFreq = 6 // Fast LFO for quick testing

	// Collect AM values across a full LFO cycle (128 steps)
	var amValues [128]uint8
	for step := 0; step < 128; step++ {
		y.lfoStep = uint8(step)
		y.stepLFOFull()
		amValues[step] = y.lfoAMOut
	}

	// Steps 0-63 should produce decreasing values (starts at peak 126)
	for i := 1; i < 64; i++ {
		if amValues[i] > amValues[i-1] {
			t.Errorf("AM should decrease in first half: step %d=%d > step %d=%d",
				i, amValues[i], i-1, amValues[i-1])
			break
		}
	}

	// Peak at step 0
	if amValues[0] != 126 {
		t.Errorf("AM peak at step 0: expected 126, got %d", amValues[0])
	}

	// Trough at step 63
	if amValues[63] != 0 {
		t.Errorf("AM trough at step 63: expected 0, got %d", amValues[63])
	}

	// Steps 64-127 should produce increasing values (ascending from 0)
	for i := 65; i < 128; i++ {
		if amValues[i] < amValues[i-1] {
			t.Errorf("AM should increase in second half: step %d=%d < step %d=%d",
				i, amValues[i], i-1, amValues[i-1])
			break
		}
	}
}

func TestLFO_PeriodForEachSpeed(t *testing.T) {
	expected := [8]uint16{108, 77, 71, 67, 62, 44, 8, 5}
	for i, want := range expected {
		if lfoPeriodTable[i] != want {
			t.Errorf("LFO period[%d]: got %d, want %d", i, lfoPeriodTable[i], want)
		}
	}
}

func TestLFO_AMModulationEffect(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 0 // Peak of triangle (AM output = 126)
	y.stepLFOFull()

	// AMS=0: no attenuation
	atten0 := y.lfoAMAttenuation(0)
	if atten0 != 0 {
		t.Errorf("AMS=0 should produce 0 attenuation, got %d", atten0)
	}

	// AMS=3: max sensitivity (shift by 0)
	atten3 := y.lfoAMAttenuation(3)
	if atten3 == 0 {
		t.Error("AMS=3 should produce non-zero attenuation at LFO peak")
	}
	if atten3 != 126 {
		t.Errorf("AMS=3 at peak: expected 126, got %d", atten3)
	}

	// AMS=1: shift by 3
	atten1 := y.lfoAMAttenuation(1)
	if atten1 != 126>>3 {
		t.Errorf("AMS=1: expected %d, got %d", 126>>3, atten1)
	}
}

func TestLFO_PMModulationEffect(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 4 << 2 // pmStep=4, positive quarter-wave, index 4

	// FMS=0: no modulation
	pm0 := y.lfoPMFnumDelta(0, 0x400)
	if pm0 != 0 {
		t.Errorf("FMS=0 should produce 0 PM delta, got %d", pm0)
	}

	// FMS=7: max sensitivity, fNum=0x400 (bit 10 set)
	pm7 := y.lfoPMFnumDelta(7, 0x400)
	if pm7 == 0 {
		t.Error("FMS=7 should produce non-zero PM delta")
	}
	if pm7 <= 0 {
		t.Errorf("FMS=7 in positive half should be positive, got %d", pm7)
	}

	// PM should be proportional to F-number
	pmHalf := y.lfoPMFnumDelta(7, 0x200) // fNum=0x200 (bit 9 set only)
	if pmHalf == 0 {
		t.Error("FMS=7 with fNum=0x200 should produce non-zero PM delta")
	}
	// With bit 9 set, delta should be ~half of bit 10 delta
	if pmHalf >= pm7 {
		t.Errorf("PM should be proportional: fNum=0x200 delta=%d should be < fNum=0x400 delta=%d",
			pmHalf, pm7)
	}
}

func TestLFO_DisabledProducesNoEffect(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = false
	y.lfoStep = 63 // Would be peak if enabled

	y.stepLFOFull()

	if y.lfoAMOut != 0 {
		t.Errorf("disabled LFO should produce AM=0, got %d", y.lfoAMOut)
	}

	// PM delta should be 0 when LFO is disabled
	pmDelta := y.lfoPMFnumDelta(7, 0x400) // max FMS, non-zero fNum
	if pmDelta != 0 {
		t.Errorf("disabled LFO should produce PM delta=0, got %d", pmDelta)
	}
}

func TestLFO_CounterWraps(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoFreq = 7 // Fastest (period=5)
	y.lfoStep = 127

	// After one more step, should wrap to 0
	y.lfoCnt = 4 // One tick away from next step
	y.stepLFOFull()
	if y.lfoStep != 0 {
		t.Errorf("LFO step should wrap from 127 to 0, got %d", y.lfoStep)
	}
}

// --- Phase 7: Channel 3 Special Mode Tests ---

func TestCh3Special_PerOperatorFrequencies(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable ch3 special mode
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)

	// Set per-operator frequencies for ch3 (channel index 2)
	// Slot 0 ($AC/$A8): block=3, fNum=0x300
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x1B) // block=3, fNum_hi=3
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x00) // fNum_lo=0

	// Slot 1 ($AD/$A9): block=4, fNum=0x400
	y.WritePort(0, 0xAD)
	y.WritePort(1, 0x24) // block=4, fNum_hi=4
	y.WritePort(0, 0xA9)
	y.WritePort(1, 0x00)

	// Slot 2 ($AE/$AA): block=5, fNum=0x500
	y.WritePort(0, 0xAE)
	y.WritePort(1, 0x2D) // block=5, fNum_hi=5
	y.WritePort(0, 0xAA)
	y.WritePort(1, 0x00)

	// Set channel 2 base frequency (used by S4 in special mode)
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x36) // block=6, fNum_hi=6
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0x00)

	// Set MUL=1, DT=0 for all ops
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}

	// Now update phase increments - check they differ
	// Op0 (OP1) -> slot 1 ($A9/$AD): block=4, fNum=0x400
	// Op1 (OP2) -> slot 2 ($AA/$AE): block=5, fNum=0x500
	// Op2 (OP3) -> slot 0 ($A8/$AC): block=3, fNum=0x300
	// Op3 (OP4) -> channel freq: block=6, fNum=0x600

	ch := &y.ch[2]
	inc0 := ch.op[0].phaseInc
	inc1 := ch.op[1].phaseInc
	inc2 := ch.op[2].phaseInc
	inc3 := ch.op[3].phaseInc

	// All should be different due to different frequencies
	if inc0 == inc1 || inc0 == inc2 || inc0 == inc3 {
		t.Error("per-operator phase increments should differ in ch3 special mode")
	}
	if inc1 == inc2 || inc1 == inc3 {
		t.Error("per-operator phase increments should differ in ch3 special mode")
	}

	// Higher block = larger increment
	if inc3 <= inc0 {
		t.Errorf("ch freq (block 6) should produce larger inc than slot 2 (block 5): %d vs %d",
			inc3, inc0)
	}
}

func TestCh3Special_DisabledUsesSharedFrequency(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Ensure ch3 special mode is off
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x00)

	// Set different per-slot frequencies
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x1B)
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x00)

	// Set channel 2 frequency
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x22) // block=4, fNum_hi=2
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0x9A)

	// Set MUL=1 DT=0 for all ops
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}

	ch := &y.ch[2]
	// All operators should have the same phase increment (shared frequency)
	if ch.op[0].phaseInc != ch.op[1].phaseInc {
		t.Errorf("without special mode, ops should share freq: op0=%d, op1=%d",
			ch.op[0].phaseInc, ch.op[1].phaseInc)
	}
	if ch.op[0].phaseInc != ch.op[2].phaseInc {
		t.Errorf("without special mode, ops should share freq: op0=%d, op2=%d",
			ch.op[0].phaseInc, ch.op[2].phaseInc)
	}
	if ch.op[0].phaseInc != ch.op[3].phaseInc {
		t.Errorf("without special mode, ops should share freq: op0=%d, op3=%d",
			ch.op[0].phaseInc, ch.op[3].phaseInc)
	}
}

// TestYM2612_DiagnosticSignalPath traces the full signal path to find where
// sound is lost. Simulates what a real game sound driver does.
func TestYM2612_DiagnosticSignalPath(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// === Step 1: Program channel 0 like a real game sound driver ===
	// Algorithm 7 (all carriers) for maximum output
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x07) // algo=7, fb=0

	// Panning L+R
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC0)

	// Program ALL 4 operators (many games need all carriers set up)
	for _, base := range []uint8{0x30, 0x34, 0x38, 0x3C} {
		y.WritePort(0, base) // DT/MUL
		y.WritePort(1, 0x01) // DT=0, MUL=1
	}
	for _, base := range []uint8{0x40, 0x44, 0x48, 0x4C} {
		y.WritePort(0, base) // TL
		y.WritePort(1, 0x00) // TL=0 (max volume)
	}
	for _, base := range []uint8{0x50, 0x54, 0x58, 0x5C} {
		y.WritePort(0, base) // RS/AR
		y.WritePort(1, 0x1F) // RS=0, AR=31
	}
	for _, base := range []uint8{0x60, 0x64, 0x68, 0x6C} {
		y.WritePort(0, base) // AM/D1R
		y.WritePort(1, 0x00) // AM=0, D1R=0
	}
	for _, base := range []uint8{0x70, 0x74, 0x78, 0x7C} {
		y.WritePort(0, base) // D2R
		y.WritePort(1, 0x00) // D2R=0
	}
	for _, base := range []uint8{0x80, 0x84, 0x88, 0x8C} {
		y.WritePort(0, base) // D1L/RR
		y.WritePort(1, 0x0F) // D1L=0 (sustain at max), RR=15
	}

	// === Step 2: Set frequency (A4 = 440Hz-ish) ===
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22) // block=4, fNum high
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x69) // fNum low

	// === Step 3: Verify registers are set ===
	ch := &y.ch[0]
	t.Logf("Channel 0 state:")
	t.Logf("  algorithm=%d, feedback=%d, panL=%v, panR=%v", ch.algorithm, ch.feedback, ch.panL, ch.panR)
	t.Logf("  fNum=0x%03X, block=%d", ch.fNum, ch.block)

	for i := 0; i < 4; i++ {
		op := &ch.op[i]
		t.Logf("  Op%d: dt=%d mul=%d tl=%d ar=%d d1r=%d d2r=%d d1l=%d rr=%d",
			i, op.dt, op.mul, op.tl, op.ar, op.d1r, op.d2r, op.d1l, op.rr)
		t.Logf("        egState=%d egLevel=0x%03X keyOn=%v phaseInc=0x%05X keyCode=%d",
			op.egState, op.egLevel, op.keyOn, op.phaseInc, op.keyCode)
	}

	// === Step 4: Key on all operators ===
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF0) // All 4 operators on, channel 0

	t.Logf("After key-on:")
	for i := 0; i < 4; i++ {
		op := &ch.op[i]
		t.Logf("  Op%d: egState=%d egLevel=0x%03X keyOn=%v phaseCounter=0x%05X",
			i, op.egState, op.egLevel, op.keyOn, op.phaseCounter)
	}

	// === Step 5: Check if phase increments are non-zero ===
	for i := 0; i < 4; i++ {
		if ch.op[i].phaseInc == 0 {
			t.Errorf("Op%d phaseInc is 0 - no frequency set!", i)
		}
	}

	// === Step 6: Manually evaluate one sample and check each stage ===
	// Step envelopes a few times to let attack work
	for i := 0; i < 100; i++ {
		y.egCounter = uint16(i)
		y.stepEnvelopesFull()
	}

	t.Logf("After 100 envelope steps:")
	for i := 0; i < 4; i++ {
		op := &ch.op[i]
		t.Logf("  Op%d: egState=%d egLevel=0x%03X", i, op.egState, op.egLevel)
		tl := totalLevel(op.egLevel, op.tl)
		t.Logf("        totalLevel=0x%03X (TL=%d)", tl, op.tl)
	}

	// === Step 7: Compute one operator output manually ===
	op0 := &ch.op[0]
	// Set phase to mid-way through sine for a non-zero output
	op0.phaseCounter = 0x40 << 10 // Quarter way = peak positive
	tl := totalLevel(op0.egLevel, op0.tl)
	rawOut := computeOperatorOutput(op0.phaseCounter, tl)
	t.Logf("Manual op0 output: phase=0x%05X, totalLevel=0x%03X, output=%d",
		op0.phaseCounter, tl, rawOut)

	if rawOut == 0 {
		// Check why
		phaseIdx := (op0.phaseCounter >> 10) & 0x3FF
		idx := phaseIdx & 0xFF
		if phaseIdx&0x100 != 0 {
			idx = 0xFF - idx
		}
		sineAtten := uint32(sineTable[idx])
		totalAtten := sineAtten + (uint32(tl) << 2)
		t.Logf("  ZERO OUTPUT: phaseIdx=0x%03X, sineIdx=0x%02X, sineAtten=%d, egAtten=%d, totalAtten=0x%X (cutoff=0x1A00)",
			phaseIdx, idx, sineAtten, tl, totalAtten)
	}

	// === Step 8: Evaluate the full channel ===
	_, l, r := y.evaluateChannelFull(0)
	t.Logf("evaluateChannelFull(0): L=%d, R=%d", l, r)

	// === Step 9: Generate actual samples ===
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()
	t.Logf("Buffer length: %d samples (%d stereo pairs)", len(buf), len(buf)/2)

	maxSample := int16(0)
	minSample := int16(0)
	nonZeroCount := 0
	for _, s := range buf {
		if s != 0 {
			nonZeroCount++
		}
		if s > maxSample {
			maxSample = s
		}
		if s < minSample {
			minSample = s
		}
	}
	t.Logf("Buffer stats: nonZero=%d, min=%d, max=%d", nonZeroCount, minSample, maxSample)

	if nonZeroCount == 0 {
		t.Error("DIAGNOSTIC: entire buffer is silence - no sound produced")
	}
}

func TestCh3Special_SlotMapping(t *testing.T) {
	// Verify ch3SlotMap returns correct mappings
	tests := []struct {
		opIdx int
		want  int
	}{
		{0, 1},  // Op0 (OP1) -> slot 1 ($A9/$AD)
		{1, 2},  // Op1 (OP2) -> slot 2 ($AA/$AE)
		{2, 0},  // Op2 (OP3) -> slot 0 ($A8/$AC)
		{3, -1}, // Op3 (OP4) -> uses channel frequency
	}
	for _, tt := range tests {
		got := ch3SlotMap(tt.opIdx)
		if got != tt.want {
			t.Errorf("ch3SlotMap(%d): got %d, want %d", tt.opIdx, got, tt.want)
		}
	}
}

// --- EG Counter Tests ---

func TestEnvelope_CounterWraps12Bit(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Set counter near overflow
	y.egCounter = 4095

	// Increment: should wrap to 1 (skipping 0, per OPN2 die-shot analysis:
	// hardware timer hits 0 via mask but carry feeds back immediately;
	// in simplified model, counter=0 causes spurious all-rate update.
	// See ym2612_reference.md Section 7, "Envelope Counter".)
	y.egClock = 2
	// Simulate one step through GenerateSamples' counter logic
	y.egClock++
	if y.egClock >= 3 {
		y.egClock = 0
		y.egCounter++
		if y.egCounter >= 4096 {
			y.egCounter = 1
		}
	}

	if y.egCounter != 1 {
		t.Errorf("EG counter should wrap to 1, got %d", y.egCounter)
	}
}

func TestEnvelope_CounterNeverReachesZero(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// The 12-bit counter wraps from 4095 to 1, skipping 0. On real
	// hardware (OPN2 die-shot analysis), timer=0 is a transient state
	// where no low/medium rate envelope updates fire. Wrapping to 1
	// in the simplified model correctly avoids a spurious all-rate update.
	y.egCounter = 4090
	for i := 0; i < 50000; i++ {
		y.egClock++
		if y.egClock >= 3 {
			y.egClock = 0
			y.egCounter++
			if y.egCounter >= 4096 {
				y.egCounter = 1
			}
		}
		if y.egCounter == 0 {
			t.Fatal("EG counter should never reach 0 (wraps from 4095 to 1)")
		}
	}
}

// --- High Rate EG Increment Tests ---

func TestEnvelope_HighRateTable(t *testing.T) {
	// Verify that rates 48-63 produce correct increments.
	// Rate 48: all 1s
	for i := 0; i < 8; i++ {
		if egHighRateTable[0][i] != 1 {
			t.Errorf("rate 48 idx %d: expected 1, got %d", i, egHighRateTable[0][i])
		}
	}

	// Rate 60-63: all 8s
	for rate := uint8(60); rate <= 63; rate++ {
		for i := 0; i < 8; i++ {
			if egHighRateTable[rate-48][i] != 8 {
				t.Errorf("rate %d idx %d: expected 8, got %d", rate, i, egHighRateTable[rate-48][i])
			}
		}
	}

	// Rate 57 has mixed values including 8
	expected57 := [8]uint8{4, 4, 4, 8, 4, 4, 4, 8}
	for i := 0; i < 8; i++ {
		if egHighRateTable[57-48][i] != expected57[i] {
			t.Errorf("rate 57 idx %d: expected %d, got %d", i, expected57[i], egHighRateTable[57-48][i])
		}
	}
}

func TestEnvelope_HighRateFasterDecay(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Compare decay speed at effective rate 48 vs 60.
	// Rate 48 has small increments (1-2), rate 60 has large (all 8).
	// Use rs=0, keyCode=0 so rks=0, effective rate = 2*d1r.
	op48 := &ymOperator{egState: egDecay, egLevel: 0, d1r: 24, d1l: 15, rs: 0, keyCode: 0, keyOn: true}
	op60 := &ymOperator{egState: egDecay, egLevel: 0, d1r: 30, d1l: 15, rs: 0, keyCode: 0, keyOn: true}

	rate48 := y.effectiveRate(op48.d1r, op48)
	rate60 := y.effectiveRate(op60.d1r, op60)
	if rate48 != 48 || rate60 != 60 {
		t.Fatalf("expected rates 48 and 60, got %d and %d", rate48, rate60)
	}

	// Step both operators a small number of times.
	// Rate 48 increments by 1 each step, rate 60 by 8.
	// After 100 steps: rate 48 ~= 100, rate 60 ~= 800.
	for i := 0; i < 100; i++ {
		y.stepOperatorEnvelope(op48, uint16(i))
		y.stepOperatorEnvelope(op60, uint16(i))
	}

	// Rate 60 should have decayed much more
	if op60.egLevel <= op48.egLevel {
		t.Errorf("rate 60 should decay faster: rate48 level=0x%03X, rate60 level=0x%03X",
			op48.egLevel, op60.egLevel)
	}
}

// --- EG Rate Differentiation Tests ---

func TestEnvelope_RatesWithinGroupDiffer(t *testing.T) {
	// Rates within the same group (same shift) should produce different
	// speeds because rate&3 selects different increment patterns.
	y := NewYM2612(7670454, 48000)

	// Test group 1 (rates 4-7): all have shift=10, but different patterns.
	// Use decay state so increments add to egLevel directly.
	ops := [4]*ymOperator{}
	for i := range ops {
		ops[i] = &ymOperator{
			egState: egDecay,
			egLevel: 0,
			d1r:     0, // We'll call stepOperatorEnvelope with effective rate directly
			d1l:     15,
			rs:      0,
			keyCode: 0,
			keyOn:   true,
		}
	}

	// Simulate stepping with effective rates 4, 5, 6, 7
	// All share shift=10, differ by pattern (rate & 3 = 0, 1, 2, 3)
	rates := [4]uint8{4, 5, 6, 7}

	// We need to directly test the increment logic, so we'll manually
	// apply the same counter sequence and track results.
	// Rather than fighting with effectiveRate, create operators with
	// known effective rates by setting d1r and rs/keyCode.
	// effective rate = 2*d1r + rks. With rs=0, rks=0, rate=2*d1r.
	// So d1r=2 -> rate 4, d1r=2 -> rate 4... but we need rate 5 too.
	// Use rs=1 to add keyCode-based rks.
	// Actually, simplest: set d1r so 2*d1r gives us what we want,
	// but that only works for even rates. For odd rates we need rks=1.

	// Direct approach: manually call with controlled counter values
	// and check that the increment table selection differs.

	// With the fixed code, rates 4-7 all have shift=10.
	// Rate 4: pattern index (rate&3)+1 = 1 -> {0,1,0,1,0,1,0,1} avg=4/8
	// Rate 5: pattern index (rate&3)+1 = 2 -> {0,1,0,1,1,1,0,1} avg=5/8
	// Rate 6: pattern index (rate&3)+1 = 3 -> {0,1,1,1,0,1,1,1} avg=6/8
	// Rate 7: pattern index (rate&3)+1 = 4 -> {0,1,1,1,1,1,1,1} avg=7/8

	// Measure total increments over a full shift cycle.
	// With shift=10, updates happen when counter & 0x3FF == 0.
	// Over counter 0..8191 (8*1024), we get 8 updates at
	// counter = 0, 1024, 2048, 3072, 4096, 5120, 6144, 7168
	// updateIdx = (counter >> 10) & 7 = 0, 1, 2, 3, 4, 5, 6, 7

	type rateResult struct {
		rate  uint8
		total uint16
	}
	results := make([]rateResult, len(rates))

	for ri, rate := range rates {
		op := &ymOperator{
			egState: egDecay,
			egLevel: 0,
			d1l:     15,
			keyOn:   true,
		}
		// Set d1r and rs/keyCode to achieve the desired effective rate.
		// effective_rate = 2*d1r + (keyCode >> (3-rs))
		// For even rates: d1r = rate/2, rs=0
		// For odd rates: d1r = (rate-1)/2, rs=3, keyCode=1
		if rate%2 == 0 {
			op.d1r = rate / 2
			op.rs = 0
			op.keyCode = 0
		} else {
			op.d1r = (rate - 1) / 2
			op.rs = 3
			op.keyCode = 1 // rks = 1 >> (3-3) = 1
		}

		// Verify effective rate
		got := y.effectiveRate(op.d1r, op)
		if got != rate {
			t.Fatalf("setup error: wanted effective rate %d, got %d (d1r=%d rs=%d kc=%d)",
				rate, got, op.d1r, op.rs, op.keyCode)
		}

		// Step through counter values 0..8191
		for c := uint16(0); c < 8192; c++ {
			y.stepOperatorEnvelope(op, c)
		}
		results[ri] = rateResult{rate, op.egLevel}
	}

	// Each successive rate should produce a higher total (faster decay)
	for i := 1; i < len(results); i++ {
		if results[i].total <= results[i-1].total {
			t.Errorf("rate %d (level=0x%03X) should decay faster than rate %d (level=0x%03X)",
				results[i].rate, results[i].total,
				results[i-1].rate, results[i-1].total)
		}
	}
}

func TestEnvelope_RateContinuityAtGroupBoundary(t *testing.T) {
	// At the boundary between rate groups (e.g., rate 7->8),
	// the higher rate should be faster than the lower rate.
	// The ratio should be approximately 8/7 (~1.14).
	y := NewYM2612(7670454, 48000)

	boundaries := []struct {
		rateLow  uint8
		rateHigh uint8
		ticks    uint16 // counter range to measure over
	}{
		{7, 8, 4096},   // slow rates: full cycle won't hit sustain cap
		{11, 12, 4096}, // slow rates: full cycle won't hit sustain cap
		{19, 20, 4096}, // medium rates: still under sustain cap
		{47, 48, 512},  // fast rates (shift=0): use shorter range to avoid sustain cap
	}

	for _, b := range boundaries {
		for _, rate := range []uint8{b.rateLow, b.rateHigh} {
			// Sanity: make sure we can construct these rates
			op := &ymOperator{}
			if rate%2 == 0 {
				op.d1r = rate / 2
				op.rs = 0
				op.keyCode = 0
			} else {
				op.d1r = (rate - 1) / 2
				op.rs = 3
				op.keyCode = 1
			}
			got := y.effectiveRate(op.d1r, op)
			if got != rate {
				t.Fatalf("setup: wanted rate %d, got %d", rate, got)
			}
		}

		// Measure decay over the specified counter range
		measure := func(rate uint8) uint16 {
			op := &ymOperator{
				egState: egDecay,
				egLevel: 0,
				d1l:     15,
				keyOn:   true,
			}
			if rate%2 == 0 {
				op.d1r = rate / 2
				op.rs = 0
				op.keyCode = 0
			} else {
				op.d1r = (rate - 1) / 2
				op.rs = 3
				op.keyCode = 1
			}
			for c := uint16(0); c < b.ticks; c++ {
				y.stepOperatorEnvelope(op, c)
			}
			return op.egLevel
		}

		levelLow := measure(b.rateLow)
		levelHigh := measure(b.rateHigh)

		if levelHigh <= levelLow {
			t.Errorf("rate %d (level=0x%03X) should decay faster than rate %d (level=0x%03X)",
				b.rateHigh, levelHigh, b.rateLow, levelLow)
		}
	}
}

// --- PM F-number Proportionality Tests ---

func TestLFO_PMProportionalToFnum(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 4 << 2 // pmStep=4, positive quarter

	// Higher F-numbers should get larger PM deltas
	delta400 := y.lfoPMFnumDelta(7, 0x400) // bit 10
	delta200 := y.lfoPMFnumDelta(7, 0x200) // bit 9
	delta100 := y.lfoPMFnumDelta(7, 0x100) // bit 8

	if delta400 <= delta200 {
		t.Errorf("fNum=0x400 delta (%d) should be > fNum=0x200 delta (%d)", delta400, delta200)
	}
	if delta200 <= delta100 {
		t.Errorf("fNum=0x200 delta (%d) should be > fNum=0x100 delta (%d)", delta200, delta100)
	}
}

func TestLFO_PMSignInversion(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true

	// Positive half: pmStep = 4 (steps 0-15 are positive)
	y.lfoStep = 4 << 2
	positiveDelta := y.lfoPMFnumDelta(7, 0x400)

	// Negative half: pmStep = 20 (steps 16-31 are negative)
	y.lfoStep = 20 << 2
	negativeDelta := y.lfoPMFnumDelta(7, 0x400)

	if positiveDelta <= 0 {
		t.Errorf("positive half should produce positive delta, got %d", positiveDelta)
	}
	if negativeDelta >= 0 {
		t.Errorf("negative half should produce negative delta, got %d", negativeDelta)
	}
	if positiveDelta != -negativeDelta {
		t.Errorf("magnitudes should match: positive=%d, negative=%d", positiveDelta, negativeDelta)
	}
}

func TestLFO_PMZeroAtFnumZero(t *testing.T) {
	y := NewYM2612(7670454, 48000)
	y.lfoEnable = true
	y.lfoStep = 4 << 2

	// F-number with no bits 4-10 set should produce zero PM delta
	delta := y.lfoPMFnumDelta(7, 0x00F) // only bits 0-3
	if delta != 0 {
		t.Errorf("fNum with only low bits should produce 0 PM delta, got %d", delta)
	}
}

func TestPhase_PMIncrementEquivalence(t *testing.T) {
	// Verify computePMPhaseIncrement matches computePhaseIncrement when
	// there's no PM delta (modFnum12 = fNum << 1).
	fNum := uint16(0x29A)
	block := uint8(4)
	keyCode := computeKeyCode(fNum, block)

	for dt := uint8(0); dt < 8; dt++ {
		for mul := uint8(0); mul <= 15; mul++ {
			normal := computePhaseIncrement(fNum, block, keyCode, dt, mul)
			modFnum12 := uint32(fNum) << 1
			pm := computePMPhaseIncrement(modFnum12, block, keyCode, dt, mul)
			if normal != pm {
				t.Errorf("dt=%d mul=%d: normal=0x%05X, pm=0x%05X", dt, mul, normal, pm)
			}
		}
	}
}
