package emu

import (
	"testing"

	"github.com/user-none/go-chip-sn76489"
)

func TestIO_ReadRegister_VersionUSA(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)

	val := io.ReadRegister(0, 0xA10001)
	if val != 0x80 {
		t.Errorf("expected 0x80 (overseas NTSC), got 0x%02X", val)
	}
}

func TestIO_ReadRegister_VersionEurope(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3546893, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7600489, 48000)
	io := NewIO(vdp, psg, ym, ConsoleEurope)

	val := io.ReadRegister(0, 0xA10001)
	if val != 0xC0 {
		t.Errorf("expected 0xC0 (overseas PAL), got 0x%02X", val)
	}
}

func TestIO_ReadRegister_VersionJapan(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleJapan)

	val := io.ReadRegister(0, 0xA10001)
	if val != 0x00 {
		t.Errorf("expected 0x00 (domestic NTSC), got 0x%02X", val)
	}
}

func TestIO_ReadRegister_ControllerData(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)

	val := io.ReadRegister(0, 0xA10003)
	if val != 0xFF {
		t.Errorf("expected 0xFF (no buttons), got 0x%02X", val)
	}

	val = io.ReadRegister(0, 0xA10005)
	if val != 0xFF {
		t.Errorf("expected 0xFF (no buttons), got 0x%02X", val)
	}
}

func TestIO_ReadRegister_Unknown(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)

	val := io.ReadRegister(0, 0xA10007)
	if val != 0x00 {
		t.Errorf("expected 0x00 for unknown register, got 0x%02X", val)
	}
}

func TestIO_WriteRegister_DataStored(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)

	// Writing to data register should store the value
	io.WriteRegister(0, 0xA10003, 0x40)
	// With ctrl=0 (all input), data register bits are masked out;
	// verify no panic and port still reads from peripheral
	val := io.ReadRegister(0, 0xA10003)
	if val != 0xFF {
		t.Errorf("expected 0xFF (ctrl=0, no buttons), got 0x%02X", val)
	}
}

func TestIO_Port1_TH1_NoButtons(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)

	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output
	io.WriteRegister(0, 0xA10003, 0x40) // data: TH=1

	val := io.ReadRegister(0, 0xA10003)
	if val != 0xFF {
		t.Errorf("TH=1 no buttons: expected 0xFF, got 0x%02X", val)
	}
}

func TestIO_Port1_TH0_NoButtons(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)

	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output
	io.WriteRegister(0, 0xA10003, 0x00) // data: TH=0

	// TH=0: bits 7=1, 6=0(TH), 5=Start(1), 4=A(1), 3=0, 2=0, 1=Down(1), 0=Up(1)
	// peripheral = 0xB3, ctrl has bit 6 as output (0 from data), rest input
	// result = (0x00 & 0x40) | (0xB3 & ^0x40) = 0x00 | (0xB3 & 0xBF) = 0xB3
	val := io.ReadRegister(0, 0xA10003)
	if val != 0xB3 {
		t.Errorf("TH=0 no buttons: expected 0xB3, got 0x%02X", val)
	}
}

func TestIO_Port1_TH1_UpPressed(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)

	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output
	io.WriteRegister(0, 0xA10003, 0x40) // data: TH=1
	io.InputP1.Set(true, false, false, false, false, false, false, false, false, false, false, false)

	val := io.ReadRegister(0, 0xA10003)
	if val != 0xFE {
		t.Errorf("TH=1 Up pressed: expected 0xFE, got 0x%02X", val)
	}
}

func TestIO_Port1_TH0_StartAPressed(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)

	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output
	io.WriteRegister(0, 0xA10003, 0x00) // data: TH=0
	io.InputP1.Set(false, false, false, false, true, false, false, true, false, false, false, false)

	// TH=0, Start+A pressed: peripheral = 0x83
	// result = (0x00 & 0x40) | (0x83 & 0xBF) = 0x83
	val := io.ReadRegister(0, 0xA10003)
	if val != 0x83 {
		t.Errorf("TH=0 Start+A pressed: expected 0x83, got 0x%02X", val)
	}
}

func TestIO_Port1_AllButtons(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)

	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output
	io.InputP1.Set(true, true, true, true, true, true, true, true, false, false, false, false)

	// TH=1: all buttons pressed -> peripheral = 0xC0
	// result = (0x40 & 0x40) | (0xC0 & 0xBF) = 0x40 | 0x80 = 0xC0
	io.WriteRegister(0, 0xA10003, 0x40) // TH=1
	val := io.ReadRegister(0, 0xA10003)
	if val != 0xC0 {
		t.Errorf("TH=1 all buttons: expected 0xC0, got 0x%02X", val)
	}

	// TH=0: all buttons pressed -> peripheral = 0x80
	// result = (0x00 & 0x40) | (0x80 & 0xBF) = 0x80
	io.WriteRegister(0, 0xA10003, 0x00) // TH=0
	val = io.ReadRegister(0, 0xA10003)
	if val != 0x80 {
		t.Errorf("TH=0 all buttons: expected 0x80, got 0x%02X", val)
	}
}

func TestIO_CtrlRegister_ReadWrite(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)

	// Port 1 ctrl
	io.WriteRegister(0, 0xA10009, 0x40)
	val := io.ReadRegister(0, 0xA10009)
	if val != 0x40 {
		t.Errorf("p1Ctrl: expected 0x40, got 0x%02X", val)
	}

	// Port 2 ctrl
	io.WriteRegister(0, 0xA1000B, 0x7F)
	val = io.ReadRegister(0, 0xA1000B)
	if val != 0x7F {
		t.Errorf("p2Ctrl: expected 0x7F, got 0x%02X", val)
	}
}

func TestIO_Port2_NoController(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)

	// Default: P2 disconnected, ctrl=0, all input, peripheral=0xFF -> 0xFF
	val := io.ReadRegister(0, 0xA10005)
	if val != 0xFF {
		t.Errorf("port 2 default: expected 0xFF, got 0x%02X", val)
	}

	// With TH as output, TH=0: no controller so peripheral stays 0xFF
	// result = (0x00 & 0x40) | (0xFF & 0xBF) = 0xBF
	io.WriteRegister(0, 0xA1000B, 0x40)
	io.WriteRegister(0, 0xA10005, 0x00)
	val = io.ReadRegister(0, 0xA10005)
	if val != 0xBF {
		t.Errorf("port 2 TH=0: expected 0xBF, got 0x%02X", val)
	}

	// With TH as output, TH=1
	// result = (0x40 & 0x40) | (0xFF & 0xBF) = 0x40 | 0xBF = 0xFF
	io.WriteRegister(0, 0xA10005, 0x40)
	val = io.ReadRegister(0, 0xA10005)
	if val != 0xFF {
		t.Errorf("port 2 TH=1: expected 0xFF, got 0x%02X", val)
	}
}

func TestIO_Port1_DefaultState(t *testing.T) {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	io := NewIO(vdp, psg, ym, ConsoleUSA)

	// Default state: ctrl=0 (all input), data=0
	// TH pulled high (ctrl bit 6=0), no buttons -> peripheral=0xFF
	// result = (0x00 & 0x00) | (0xFF & 0xFF) = 0xFF
	val := io.ReadRegister(0, 0xA10003)
	if val != 0xFF {
		t.Errorf("default state: expected 0xFF, got 0x%02X", val)
	}
}

// --- 6-button controller tests ---

// newTestIO creates a test IO instance with NTSC region.
func newTestIO() *IO {
	vdp := NewVDP(false)
	psg := sn76489.New(3579545, 48000, psgBufferSize, sn76489.Sega)
	ym := NewYM2612(7670454, 48000)
	return NewIO(vdp, psg, ym, ConsoleUSA)
}

// thCycle performs a TH transition on port 1 by writing the data register.
// Returns the value read from port 1 after the write.
func thCycle(io *IO, cycle uint64, thHigh bool) byte {
	var data byte
	if thHigh {
		data = 0x40
	}
	io.WriteRegister(cycle, 0xA10003, data)
	return io.ReadRegister(cycle, 0xA10003)
}

func TestIO_SixButton_States0to3_BackwardCompat(t *testing.T) {
	io := newTestIO()
	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output

	// No buttons pressed. Cycle through states 0-3 (two TH toggles)
	// which should be identical to 3-button behavior.
	var cycle uint64 = 1000

	// State 0 (TH=1): C, B, Right, Left, Down, Up - all released = 0x3F
	val := thCycle(io, cycle, true)
	// result = (0x40 & 0x40) | (0xFF & 0xBF) = 0x40 | 0xBF = 0xFF
	if val != 0xFF {
		t.Errorf("state 0 (TH=1): expected 0xFF, got 0x%02X", val)
	}

	cycle += 100
	// State 1 (TH=0): Start, A, 0, 0, Down, Up - all released
	val = thCycle(io, cycle, false)
	// peripheral = 0xB3 (bits 7,6 high, 5=Start(1), 4=A(1), 3=0, 2=0, 1=Down(1), 0=Up(1))
	// result = (0x00 & 0x40) | (0xB3 & 0xBF) = 0xB3
	if val != 0xB3 {
		t.Errorf("state 1 (TH=0): expected 0xB3, got 0x%02X", val)
	}

	cycle += 100
	// State 2 (TH=1): repeat of state 0
	val = thCycle(io, cycle, true)
	if val != 0xFF {
		t.Errorf("state 2 (TH=1): expected 0xFF, got 0x%02X", val)
	}

	cycle += 100
	// State 3 (TH=0): repeat of state 1
	val = thCycle(io, cycle, false)
	if val != 0xB3 {
		t.Errorf("state 3 (TH=0): expected 0xB3, got 0x%02X", val)
	}
}

func TestIO_SixButton_State5_Detection(t *testing.T) {
	io := newTestIO()
	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output

	var cycle uint64 = 1000

	// Advance through states 0-4
	thCycle(io, cycle, true) // state 0 (TH=1, initial was low/default)
	cycle += 100
	thCycle(io, cycle, false) // state 1
	cycle += 100
	thCycle(io, cycle, true) // state 2
	cycle += 100
	thCycle(io, cycle, false) // state 3
	cycle += 100
	thCycle(io, cycle, true) // state 4

	cycle += 100
	// State 5 (TH=0): Start, A, 0, 0, 0, 0 - bits 3-0 all zero for 6-button detection
	val := thCycle(io, cycle, false)
	// peripheral = 0xF0 (bits 7,6 high, 5=Start(1), 4=A(1), 3-0=0)
	// Wait - bits 7,6 = 0xC0. Then bits 5,4 = Start(released=1), A(released=1) = 0x30
	// bits 3-0 = 0x00. So peripheral = 0xC0 | 0x30 | 0x00 = 0xF0
	// result = (0x00 & 0x40) | (0xF0 & 0xBF) = 0xB0
	if val != 0xB0 {
		t.Errorf("state 5 (TH=0 detection): expected 0xB0, got 0x%02X", val)
	}
}

func TestIO_SixButton_State6_ExtraButtons(t *testing.T) {
	io := newTestIO()
	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output
	// Press X and Z only
	io.InputP1.Set(false, false, false, false, false, false, false, false, true, false, true, false)

	var cycle uint64 = 1000

	// Advance through states 0-5
	thCycle(io, cycle, true) // state 0
	cycle += 100
	thCycle(io, cycle, false) // state 1
	cycle += 100
	thCycle(io, cycle, true) // state 2
	cycle += 100
	thCycle(io, cycle, false) // state 3
	cycle += 100
	thCycle(io, cycle, true) // state 4
	cycle += 100
	thCycle(io, cycle, false) // state 5

	cycle += 100
	// State 6 (TH=1): C, B, Mode, X, Y, Z
	// X pressed (bit 2=0), Z pressed (bit 0=0), rest released
	// peripheral = 0xC0 | 0x3F = 0xFF, then clear X(bit2) and Z(bit0)
	// peripheral = 0xFF &^ 0x04 &^ 0x01 = 0xFA
	val := thCycle(io, cycle, true)
	// result = (0x40 & 0x40) | (0xFA & 0xBF) = 0x40 | 0xBA = 0xFA
	if val != 0xFA {
		t.Errorf("state 6 (TH=1 extra buttons X+Z): expected 0xFA, got 0x%02X", val)
	}
}

func TestIO_SixButton_State7_EndMarker(t *testing.T) {
	io := newTestIO()
	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output

	var cycle uint64 = 1000

	// Advance through states 0-6
	thCycle(io, cycle, true) // state 0
	cycle += 100
	thCycle(io, cycle, false) // state 1
	cycle += 100
	thCycle(io, cycle, true) // state 2
	cycle += 100
	thCycle(io, cycle, false) // state 3
	cycle += 100
	thCycle(io, cycle, true) // state 4
	cycle += 100
	thCycle(io, cycle, false) // state 5
	cycle += 100
	thCycle(io, cycle, true) // state 6

	cycle += 100
	// State 7 (TH=0): Start, A, 1, 1, 1, 1 - bits 3-0 all one
	val := thCycle(io, cycle, false)
	// peripheral = 0xC0 | 0x3F = 0xFF (bits 5,4 = Start(1),A(1), bits 3-0 = 1111)
	// result = (0x00 & 0x40) | (0xFF & 0xBF) = 0xBF
	if val != 0xBF {
		t.Errorf("state 7 (TH=0 end marker): expected 0xBF, got 0x%02X", val)
	}
}

func TestIO_SixButton_StateWraps(t *testing.T) {
	io := newTestIO()
	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output

	var cycle uint64 = 1000

	// Advance through all 8 states (0-7)
	thCycle(io, cycle, true) // state 0
	cycle += 100
	thCycle(io, cycle, false) // state 1
	cycle += 100
	thCycle(io, cycle, true) // state 2
	cycle += 100
	thCycle(io, cycle, false) // state 3
	cycle += 100
	thCycle(io, cycle, true) // state 4
	cycle += 100
	thCycle(io, cycle, false) // state 5
	cycle += 100
	thCycle(io, cycle, true) // state 6
	cycle += 100
	thCycle(io, cycle, false) // state 7

	// Next transition wraps to state 0
	cycle += 100
	val := thCycle(io, cycle, true) // state 0 again (TH=1)
	// State 0 (TH=1): standard C, B, Right, Left, Down, Up - no buttons
	// result = 0xFF
	if val != 0xFF {
		t.Errorf("state 0 after wrap (TH=1): expected 0xFF, got 0x%02X", val)
	}
}

func TestIO_SixButton_Timeout(t *testing.T) {
	io := newTestIO()
	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output

	var cycle uint64 = 1000

	// Advance to state 4
	thCycle(io, cycle, true) // state 0
	cycle += 100
	thCycle(io, cycle, false) // state 1
	cycle += 100
	thCycle(io, cycle, true) // state 2
	cycle += 100
	thCycle(io, cycle, false) // state 3
	cycle += 100
	thCycle(io, cycle, true) // state 4

	// Wait for timeout (>= 11506 cycles)
	cycle += sixButtonTimeoutCycles

	// Next write should reset counter to 0 first, then advance to 1
	val := thCycle(io, cycle, false)
	// After reset to 0, advance to state 1 (TH=0): Start, A, 0, 0, Down, Up
	// peripheral = 0xB3
	// result = (0x00 & 0x40) | (0xB3 & 0xBF) = 0xB3
	if val != 0xB3 {
		t.Errorf("after timeout: expected state 1 (0xB3), got 0x%02X", val)
	}
}

func TestIO_SixButton_NoTimeoutWithinWindow(t *testing.T) {
	io := newTestIO()
	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output

	var cycle uint64 = 1000

	// Advance to state 4
	thCycle(io, cycle, true) // state 0
	cycle += 100
	thCycle(io, cycle, false) // state 1
	cycle += 100
	thCycle(io, cycle, true) // state 2
	cycle += 100
	thCycle(io, cycle, false) // state 3
	cycle += 100
	thCycle(io, cycle, true) // state 4

	// Wait just under the timeout
	cycle += sixButtonTimeoutCycles - 1

	// Should NOT reset - continues to state 5
	val := thCycle(io, cycle, false)
	// State 5 (TH=0): detection, bits 3-0 all zero
	// peripheral = 0xF0 -> result with TH output = 0xB0
	if val != 0xB0 {
		t.Errorf("within window: expected state 5 (0xB0), got 0x%02X", val)
	}
}

func TestIO_SixButton_AllExtraButtonsPressed(t *testing.T) {
	io := newTestIO()
	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output
	// Press all extra buttons: X, Y, Z, Mode
	io.InputP1.Set(false, false, false, false, false, false, false, false, true, true, true, true)

	var cycle uint64 = 1000

	// Advance to state 6
	thCycle(io, cycle, true) // state 0
	cycle += 100
	thCycle(io, cycle, false) // state 1
	cycle += 100
	thCycle(io, cycle, true) // state 2
	cycle += 100
	thCycle(io, cycle, false) // state 3
	cycle += 100
	thCycle(io, cycle, true) // state 4
	cycle += 100
	thCycle(io, cycle, false) // state 5

	cycle += 100
	// State 6 (TH=1): C, B, Mode, X, Y, Z - all extra pressed
	// peripheral = 0xC0 | 0x30 (B,C released) | 0x00 (Mode,X,Y,Z all pressed)
	// = 0xF0
	val := thCycle(io, cycle, true)
	// result = (0x40 & 0x40) | (0xF0 & 0xBF) = 0x40 | 0xB0 = 0xF0
	if val != 0xF0 {
		t.Errorf("state 6 all extra pressed: expected 0xF0, got 0x%02X", val)
	}
}

func TestIO_SixButton_NoExtraButtonsPressed(t *testing.T) {
	io := newTestIO()
	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output
	// No extra buttons pressed
	io.InputP1.Set(false, false, false, false, false, false, false, false, false, false, false, false)

	var cycle uint64 = 1000

	// Advance to state 6
	thCycle(io, cycle, true) // state 0
	cycle += 100
	thCycle(io, cycle, false) // state 1
	cycle += 100
	thCycle(io, cycle, true) // state 2
	cycle += 100
	thCycle(io, cycle, false) // state 3
	cycle += 100
	thCycle(io, cycle, true) // state 4
	cycle += 100
	thCycle(io, cycle, false) // state 5

	cycle += 100
	// State 6 (TH=1): C, B, Mode, X, Y, Z - none pressed
	// peripheral = 0xC0 | 0x3F = 0xFF
	val := thCycle(io, cycle, true)
	// result = (0x40 & 0x40) | (0xFF & 0xBF) = 0x40 | 0xBF = 0xFF
	if val != 0xFF {
		t.Errorf("state 6 no extra buttons: expected 0xFF, got 0x%02X", val)
	}
}

func TestIO_SixButton_TimeoutResetsAcrossFrames(t *testing.T) {
	io := newTestIO()
	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output
	io.InputP1.Set(true, false, false, false, true, false, false, true, false, false, false, false)

	// Simulate a typical 3-button read across multiple frames.
	// Each frame: write TH=1, read (expect state 0), write TH=0, read (expect state 1).
	// Between frames the timeout elapses, resetting the counter.
	var cycle uint64 = 1000
	const frameCycles uint64 = 128000 // ~16.6ms at 7.67MHz NTSC

	for frame := 0; frame < 5; frame++ {
		// TH=1 read: state 0 - C, B, Right, Left, Down, Up
		// Up pressed (bit 0=0): peripheral = 0xFE
		// result = (0x40 & 0x40) | (0xFE & 0xBF) = 0x40 | 0xBE = 0xFE
		val := thCycle(io, cycle, true)
		if val != 0xFE {
			t.Errorf("frame %d TH=1: expected 0xFE, got 0x%02X", frame, val)
		}

		cycle += 100
		// TH=0 read: state 1 - Start, A, 0, 0, Down, Up
		// Up pressed (bit0=0), A pressed (bit4=0), Start pressed (bit5=0)
		// peripheral = 0xC0 | 0x33 = 0xF3, &^0x01=0xF2, &^0x10=0xE2, &^0x20=0xC2
		// result = (0x00 & 0x40) | (0xC2 & 0xBF) = 0x82
		val = thCycle(io, cycle, false)
		if val != 0x82 {
			t.Errorf("frame %d TH=0: expected 0x82, got 0x%02X", frame, val)
		}

		// Advance to next frame (well past timeout)
		cycle += frameCycles
	}
}

func TestIO_ThreeButton_NoStateMachine(t *testing.T) {
	io := newTestIO()
	io.InputP1.SixButton = false
	io.WriteRegister(0, 0xA10009, 0x40) // ctrl: TH is output
	io.InputP1.Set(true, false, false, false, true, false, false, true, true, true, true, true)

	var cycle uint64 = 1000
	const frameCycles uint64 = 128000

	for frame := 0; frame < 3; frame++ {
		// TH=1: C, B, Right, Left, Down, Up - Up pressed (bit 0=0)
		// peripheral = 0xFE
		// result = (0x40 & 0x40) | (0xFE & 0xBF) = 0xFE
		val := thCycle(io, cycle, true)
		if val != 0xFE {
			t.Errorf("frame %d TH=1: expected 0xFE, got 0x%02X", frame, val)
		}

		cycle += 100
		// TH=0: Start, A, 0, 0, Down, Up - Up+Start+A pressed
		// peripheral = 0xC2
		// result = (0x00 & 0x40) | (0xC2 & 0xBF) = 0x82
		val = thCycle(io, cycle, false)
		if val != 0x82 {
			t.Errorf("frame %d TH=0: expected 0x82, got 0x%02X", frame, val)
		}

		cycle += frameCycles
	}

	// Verify extra TH toggles never reach 6-button states.
	// Toggle through what would be states 0-5 in 6-button mode.
	cycle = 500000
	thCycle(io, cycle, true) // would be state 0
	cycle += 100
	thCycle(io, cycle, false) // would be state 1
	cycle += 100
	thCycle(io, cycle, true) // would be state 2
	cycle += 100
	thCycle(io, cycle, false) // would be state 3
	cycle += 100
	thCycle(io, cycle, true) // would be state 4
	cycle += 100

	// In 6-button mode this would be state 5 (detection: bits 3-0 all zero).
	// In 3-button mode it should still be normal TH=0 with Up/Down visible.
	val := thCycle(io, cycle, false)
	// TH=0: Up pressed -> peripheral = 0xC2 (same as normal TH=0 with Up+Start+A)
	// result = 0x82
	if val != 0x82 {
		t.Errorf("extra toggles TH=0: expected 0x82 (3-button), got 0x%02X", val)
	}
}

// --- Player 2 controller tests ---

// thCycleP2 performs a TH transition on port 2 by writing the data register.
// Returns the value read from port 2 after the write.
func thCycleP2(io *IO, cycle uint64, thHigh bool) byte {
	var data byte
	if thHigh {
		data = 0x40
	}
	io.WriteRegister(cycle, 0xA10005, data)
	return io.ReadRegister(cycle, 0xA10005)
}

func TestIO_Port2_TH1_NoButtons(t *testing.T) {
	io := newTestIO()
	io.InputP2.Connected = true
	io.WriteRegister(0, 0xA1000B, 0x40) // ctrl: TH is output
	io.WriteRegister(0, 0xA10005, 0x40) // data: TH=1

	val := io.ReadRegister(0, 0xA10005)
	if val != 0xFF {
		t.Errorf("TH=1 no buttons: expected 0xFF, got 0x%02X", val)
	}
}

func TestIO_Port2_TH0_NoButtons(t *testing.T) {
	io := newTestIO()
	io.InputP2.Connected = true
	io.WriteRegister(0, 0xA1000B, 0x40) // ctrl: TH is output
	io.WriteRegister(0, 0xA10005, 0x00) // data: TH=0

	// TH=0: bits 7=1, 6=0(TH), 5=Start(1), 4=A(1), 3=0, 2=0, 1=Down(1), 0=Up(1)
	// peripheral = 0xB3, ctrl has bit 6 as output (0 from data), rest input
	// result = (0x00 & 0x40) | (0xB3 & ^0x40) = 0x00 | (0xB3 & 0xBF) = 0xB3
	val := io.ReadRegister(0, 0xA10005)
	if val != 0xB3 {
		t.Errorf("TH=0 no buttons: expected 0xB3, got 0x%02X", val)
	}
}

func TestIO_Port2_TH1_ButtonsPressed(t *testing.T) {
	io := newTestIO()
	io.InputP2.Connected = true
	io.WriteRegister(0, 0xA1000B, 0x40) // ctrl: TH is output
	io.WriteRegister(0, 0xA10005, 0x40) // data: TH=1
	io.InputP2.Set(true, false, false, false, false, false, false, false, false, false, false, false)

	val := io.ReadRegister(0, 0xA10005)
	if val != 0xFE {
		t.Errorf("TH=1 Up pressed: expected 0xFE, got 0x%02X", val)
	}
}

func TestIO_Port2_TH0_ButtonsPressed(t *testing.T) {
	io := newTestIO()
	io.InputP2.Connected = true
	io.WriteRegister(0, 0xA1000B, 0x40) // ctrl: TH is output
	io.WriteRegister(0, 0xA10005, 0x00) // data: TH=0
	io.InputP2.Set(false, false, false, false, true, false, false, true, false, false, false, false)

	// TH=0, Start+A pressed: peripheral = 0x83
	// result = (0x00 & 0x40) | (0x83 & 0xBF) = 0x83
	val := io.ReadRegister(0, 0xA10005)
	if val != 0x83 {
		t.Errorf("TH=0 Start+A pressed: expected 0x83, got 0x%02X", val)
	}
}

func TestIO_Port2_SixButton_State6(t *testing.T) {
	io := newTestIO()
	io.InputP2.Connected = true
	io.WriteRegister(0, 0xA1000B, 0x40) // ctrl: TH is output
	// Press X and Z only
	io.InputP2.Set(false, false, false, false, false, false, false, false, true, false, true, false)

	var cycle uint64 = 1000

	// Advance through states 0-5
	thCycleP2(io, cycle, true) // state 0
	cycle += 100
	thCycleP2(io, cycle, false) // state 1
	cycle += 100
	thCycleP2(io, cycle, true) // state 2
	cycle += 100
	thCycleP2(io, cycle, false) // state 3
	cycle += 100
	thCycleP2(io, cycle, true) // state 4
	cycle += 100
	thCycleP2(io, cycle, false) // state 5

	cycle += 100
	// State 6 (TH=1): C, B, Mode, X, Y, Z
	// X pressed (bit 2=0), Z pressed (bit 0=0), rest released
	// peripheral = 0xFF &^ 0x04 &^ 0x01 = 0xFA
	val := thCycleP2(io, cycle, true)
	// result = (0x40 & 0x40) | (0xFA & 0xBF) = 0x40 | 0xBA = 0xFA
	if val != 0xFA {
		t.Errorf("state 6 (TH=1 extra buttons X+Z): expected 0xFA, got 0x%02X", val)
	}
}

func TestIO_Port2_SixButton_Detection(t *testing.T) {
	io := newTestIO()
	io.InputP2.Connected = true
	io.WriteRegister(0, 0xA1000B, 0x40) // ctrl: TH is output

	var cycle uint64 = 1000

	// Advance through states 0-4
	thCycleP2(io, cycle, true) // state 0
	cycle += 100
	thCycleP2(io, cycle, false) // state 1
	cycle += 100
	thCycleP2(io, cycle, true) // state 2
	cycle += 100
	thCycleP2(io, cycle, false) // state 3
	cycle += 100
	thCycleP2(io, cycle, true) // state 4

	cycle += 100
	// State 5 (TH=0): Start, A, 0, 0, 0, 0 - bits 3-0 all zero for 6-button detection
	// peripheral = 0xC0 | 0x30 = 0xF0
	// result = (0x00 & 0x40) | (0xF0 & 0xBF) = 0xB0
	val := thCycleP2(io, cycle, false)
	if val != 0xB0 {
		t.Errorf("state 5 (TH=0 detection): expected 0xB0, got 0x%02X", val)
	}
}

func TestIO_Port2_ThreeButton(t *testing.T) {
	io := newTestIO()
	io.InputP2.Connected = true
	io.InputP2.SixButton = false
	io.WriteRegister(0, 0xA1000B, 0x40) // ctrl: TH is output
	io.InputP2.Set(true, false, false, false, true, false, false, true, true, true, true, true)

	var cycle uint64 = 1000
	const frameCycles uint64 = 128000

	for frame := 0; frame < 3; frame++ {
		// TH=1: C, B, Right, Left, Down, Up - Up pressed (bit 0=0)
		// peripheral = 0xFE
		// result = (0x40 & 0x40) | (0xFE & 0xBF) = 0xFE
		val := thCycleP2(io, cycle, true)
		if val != 0xFE {
			t.Errorf("frame %d TH=1: expected 0xFE, got 0x%02X", frame, val)
		}

		cycle += 100
		// TH=0: Start, A, 0, 0, Down, Up - Up+Start+A pressed
		// peripheral = 0xC2
		// result = (0x00 & 0x40) | (0xC2 & 0xBF) = 0x82
		val = thCycleP2(io, cycle, false)
		if val != 0x82 {
			t.Errorf("frame %d TH=0: expected 0x82, got 0x%02X", frame, val)
		}

		cycle += frameCycles
	}

	// Verify extra TH toggles never reach 6-button states.
	cycle = 500000
	thCycleP2(io, cycle, true)
	cycle += 100
	thCycleP2(io, cycle, false)
	cycle += 100
	thCycleP2(io, cycle, true)
	cycle += 100
	thCycleP2(io, cycle, false)
	cycle += 100
	thCycleP2(io, cycle, true)
	cycle += 100

	// In 3-button mode: normal TH=0 with Up/Down visible
	val := thCycleP2(io, cycle, false)
	if val != 0x82 {
		t.Errorf("extra toggles TH=0: expected 0x82 (3-button), got 0x%02X", val)
	}
}
