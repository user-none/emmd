package emu

import "github.com/user-none/go-chip-sn76489"

// Input holds the state of a Genesis controller port.
type Input struct {
	Connected             bool // true if a controller is plugged in
	SixButton             bool // true = 6-button controller, false = 3-button
	up, down, left, right bool
	btnA, btnB, btnC      bool
	btnX, btnY, btnZ      bool
	btnMode               bool
	start                 bool
}

// Set sets controller state.
func (inp *Input) Set(up, down, left, right, btnA, btnB, btnC, start, btnX, btnY, btnZ, btnMode bool) {
	inp.up = up
	inp.down = down
	inp.left = left
	inp.right = right
	inp.btnA = btnA
	inp.btnB = btnB
	inp.btnC = btnC
	inp.start = start
	inp.btnX = btnX
	inp.btnY = btnY
	inp.btnZ = btnZ
	inp.btnMode = btnMode
}

// sixButtonTimeoutCycles is the number of M68K cycles (~1.5ms at 7.67MHz)
// after which the 6-button state counter resets to 0. The real hardware uses
// an RC circuit, so this is approximate. The same value works for both
// NTSC and PAL since the difference is negligible (~100 cycles).
const sixButtonTimeoutCycles uint64 = 11506

// IO is a Genesis I/O controller wrapping the hardware components
// needed for bus access.
type IO struct {
	InputP1 Input
	InputP2 Input
	Region  Region
	vdp     *VDP
	psg     *sn76489.SN76489
	ym2612  *YM2612

	p1Data byte // Port 1 data register (output values)
	p1Ctrl byte // Port 1 ctrl register (1=output, 0=input)
	p2Data byte // Port 2 data register
	p2Ctrl byte // Port 2 ctrl register

	// 6-button controller state machine (only active when SixButton is true)
	p1THState    uint8  // Current state counter (0-7)
	p1LastTHHigh bool   // Previous TH value for edge detection
	p1LastCycle  uint64 // M68K cycle of last TH transition

	p2THState    uint8  // P2 state counter (0-7)
	p2LastTHHigh bool   // P2 previous TH value for edge detection
	p2LastCycle  uint64 // P2 M68K cycle of last TH transition
}

// NewIO creates a new I/O controller.
func NewIO(vdp *VDP, psg *sn76489.SN76489, ym2612 *YM2612, region Region) *IO {
	return &IO{
		InputP1:      Input{Connected: true, SixButton: true},
		InputP2:      Input{SixButton: true},
		Region:       region,
		vdp:          vdp,
		psg:          psg,
		ym2612:       ym2612,
		p1LastTHHigh: true, // TH pulled high at power-on
		p2LastTHHigh: true, // TH pulled high at power-on
	}
}

// ReadRegister reads an I/O register by address.
// cycle is the current M68K cycle count, used for 6-button timeout detection.
func (io *IO) ReadRegister(cycle uint64, addr uint32) byte {
	switch addr {
	case 0xA10001:
		// Version register: bit 7 = overseas, bit 6 = PAL, bits 3-0 = hardware version
		var val byte = 0x80 // overseas
		if io.Region == RegionPAL {
			val |= 0x40
		}
		return val
	case 0xA10003:
		return io.readPort1(cycle)
	case 0xA10005:
		return io.readPort2(cycle)
	case 0xA10009:
		return io.p1Ctrl
	case 0xA1000B:
		return io.p2Ctrl
	default:
		return 0x00
	}
}

// WriteRegister writes an I/O register by address.
// cycle is the current M68K cycle count, used for 6-button TH edge detection.
func (io *IO) WriteRegister(cycle uint64, addr uint32, val byte) {
	switch addr {
	case 0xA10003:
		io.p1Data = val

		// 6-button state machine: track TH edges to advance state counter
		if io.InputP1.SixButton && io.InputP1.Connected {
			var newTH bool
			if io.p1Ctrl&0x40 != 0 {
				newTH = val&0x40 != 0
			} else {
				newTH = true // TH pulled high when configured as input
			}

			if newTH != io.p1LastTHHigh {
				// Check timeout: reset counter and TH tracking if enough
				// time has elapsed. Resetting p1LastTHHigh to true
				// (idle/pulled-high) ensures the first TH=1 write after
				// timeout is not an edge, keeping state 0 aligned with
				// TH=1 reads.
				if cycle > 0 && io.p1LastCycle > 0 && cycle-io.p1LastCycle >= sixButtonTimeoutCycles {
					io.p1THState = 0
					io.p1LastTHHigh = true
				}
				// Re-check after potential reset: only advance if still an edge
				if newTH != io.p1LastTHHigh {
					io.p1THState = (io.p1THState + 1) & 0x07
					io.p1LastTHHigh = newTH
					io.p1LastCycle = cycle
				}
			}
		}
	case 0xA10005:
		io.p2Data = val

		// 6-button state machine: track TH edges to advance state counter
		if io.InputP2.SixButton && io.InputP2.Connected {
			var newTH bool
			if io.p2Ctrl&0x40 != 0 {
				newTH = val&0x40 != 0
			} else {
				newTH = true // TH pulled high when configured as input
			}

			if newTH != io.p2LastTHHigh {
				if cycle > 0 && io.p2LastCycle > 0 && cycle-io.p2LastCycle >= sixButtonTimeoutCycles {
					io.p2THState = 0
					io.p2LastTHHigh = true
				}
				if newTH != io.p2LastTHHigh {
					io.p2THState = (io.p2THState + 1) & 0x07
					io.p2LastTHHigh = newTH
					io.p2LastCycle = cycle
				}
			}
		}
	case 0xA10009:
		io.p1Ctrl = val
	case 0xA1000B:
		io.p2Ctrl = val
	}
}

// readPort1 reads Player 1 controller data, combining output pins from the
// data register with input pins from the controller peripheral.
//
// The 6-button controller uses an 8-state machine driven by TH transitions:
//
//	State 0,2,4 (TH=1): C, B, Right, Left, Down, Up
//	State 1,3   (TH=0): Start, A, 0, 0, Down, Up
//	State 5     (TH=0): Start, A, 0, 0, 0, 0  (detection: bits 3-0 all zero)
//	State 6     (TH=1): C, B, Mode, X, Y, Z    (extra buttons)
//	State 7     (TH=0): Start, A, 1, 1, 1, 1   (end marker)
func (io *IO) readPort1(cycle uint64) byte {
	// No controller: all peripheral pins float high
	if !io.InputP1.Connected {
		return (io.p1Data & io.p1Ctrl) | (0xFF & ^io.p1Ctrl)
	}

	// Determine TH: if output (ctrl bit 6=1), use data register; else pulled high
	var th bool
	if io.p1Ctrl&0x40 != 0 {
		th = io.p1Data&0x40 != 0
	} else {
		th = true
	}

	// Controller response (active low: 0=pressed, 1=released)
	var peripheral byte = 0xC0 // Bits 7,6 pulled high

	if !io.InputP1.SixButton {
		// 3-button controller: no state machine, just TH level
		if th {
			// TH=1: C, B, Right, Left, Down, Up
			peripheral |= 0x3F
			if io.InputP1.up {
				peripheral &^= 0x01
			}
			if io.InputP1.down {
				peripheral &^= 0x02
			}
			if io.InputP1.left {
				peripheral &^= 0x04
			}
			if io.InputP1.right {
				peripheral &^= 0x08
			}
			if io.InputP1.btnB {
				peripheral &^= 0x10
			}
			if io.InputP1.btnC {
				peripheral &^= 0x20
			}
		} else {
			// TH=0: Start, A, 0, 0, Down, Up
			peripheral |= 0x33
			if io.InputP1.up {
				peripheral &^= 0x01
			}
			if io.InputP1.down {
				peripheral &^= 0x02
			}
			if io.InputP1.btnA {
				peripheral &^= 0x10
			}
			if io.InputP1.start {
				peripheral &^= 0x20
			}
		}
		return (io.p1Data & io.p1Ctrl) | (peripheral & ^io.p1Ctrl)
	}

	// 6-button controller: state-machine driven
	// Check timeout: reset state if too much time has elapsed since last TH edge
	if cycle > 0 && io.p1LastCycle > 0 && cycle-io.p1LastCycle >= sixButtonTimeoutCycles {
		io.p1THState = 0
		io.p1LastTHHigh = true
	}

	switch io.p1THState {
	case 0, 2, 4:
		// TH=1: C, B, Right, Left, Down, Up
		peripheral |= 0x3F
		if io.InputP1.up {
			peripheral &^= 0x01
		}
		if io.InputP1.down {
			peripheral &^= 0x02
		}
		if io.InputP1.left {
			peripheral &^= 0x04
		}
		if io.InputP1.right {
			peripheral &^= 0x08
		}
		if io.InputP1.btnB {
			peripheral &^= 0x10
		}
		if io.InputP1.btnC {
			peripheral &^= 0x20
		}
	case 1, 3:
		// TH=0: Start, A, 0, 0, Down, Up
		peripheral |= 0x33
		if io.InputP1.up {
			peripheral &^= 0x01
		}
		if io.InputP1.down {
			peripheral &^= 0x02
		}
		if io.InputP1.btnA {
			peripheral &^= 0x10
		}
		if io.InputP1.start {
			peripheral &^= 0x20
		}
	case 5:
		// TH=0, detection: Start, A, 0, 0, 0, 0 (bits 3-0 all zero)
		peripheral |= 0x30
		if io.InputP1.btnA {
			peripheral &^= 0x10
		}
		if io.InputP1.start {
			peripheral &^= 0x20
		}
	case 6:
		// TH=1, extra buttons: C, B, Mode, X, Y, Z
		peripheral |= 0x3F
		if io.InputP1.btnZ {
			peripheral &^= 0x01
		}
		if io.InputP1.btnY {
			peripheral &^= 0x02
		}
		if io.InputP1.btnX {
			peripheral &^= 0x04
		}
		if io.InputP1.btnMode {
			peripheral &^= 0x08
		}
		if io.InputP1.btnB {
			peripheral &^= 0x10
		}
		if io.InputP1.btnC {
			peripheral &^= 0x20
		}
	case 7:
		// TH=0, end marker: Start, A, 1, 1, 1, 1 (bits 3-0 all one)
		peripheral |= 0x3F
		if io.InputP1.btnA {
			peripheral &^= 0x10
		}
		if io.InputP1.start {
			peripheral &^= 0x20
		}
	}

	return (io.p1Data & io.p1Ctrl) | (peripheral & ^io.p1Ctrl)
}

// readPort2 reads Player 2 controller data, combining output pins from the
// data register with input pins from the controller peripheral.
// Uses the same 3/6-button state machine as readPort1.
func (io *IO) readPort2(cycle uint64) byte {
	// No controller: all peripheral pins float high
	if !io.InputP2.Connected {
		return (io.p2Data & io.p2Ctrl) | (0xFF & ^io.p2Ctrl)
	}

	// Determine TH: if output (ctrl bit 6=1), use data register; else pulled high
	var th bool
	if io.p2Ctrl&0x40 != 0 {
		th = io.p2Data&0x40 != 0
	} else {
		th = true
	}

	// Controller response (active low: 0=pressed, 1=released)
	var peripheral byte = 0xC0 // Bits 7,6 pulled high

	if !io.InputP2.SixButton {
		// 3-button controller: no state machine, just TH level
		if th {
			// TH=1: C, B, Right, Left, Down, Up
			peripheral |= 0x3F
			if io.InputP2.up {
				peripheral &^= 0x01
			}
			if io.InputP2.down {
				peripheral &^= 0x02
			}
			if io.InputP2.left {
				peripheral &^= 0x04
			}
			if io.InputP2.right {
				peripheral &^= 0x08
			}
			if io.InputP2.btnB {
				peripheral &^= 0x10
			}
			if io.InputP2.btnC {
				peripheral &^= 0x20
			}
		} else {
			// TH=0: Start, A, 0, 0, Down, Up
			peripheral |= 0x33
			if io.InputP2.up {
				peripheral &^= 0x01
			}
			if io.InputP2.down {
				peripheral &^= 0x02
			}
			if io.InputP2.btnA {
				peripheral &^= 0x10
			}
			if io.InputP2.start {
				peripheral &^= 0x20
			}
		}
		return (io.p2Data & io.p2Ctrl) | (peripheral & ^io.p2Ctrl)
	}

	// 6-button controller: state-machine driven
	// Check timeout: reset state if too much time has elapsed since last TH edge
	if cycle > 0 && io.p2LastCycle > 0 && cycle-io.p2LastCycle >= sixButtonTimeoutCycles {
		io.p2THState = 0
		io.p2LastTHHigh = true
	}

	switch io.p2THState {
	case 0, 2, 4:
		// TH=1: C, B, Right, Left, Down, Up
		peripheral |= 0x3F
		if io.InputP2.up {
			peripheral &^= 0x01
		}
		if io.InputP2.down {
			peripheral &^= 0x02
		}
		if io.InputP2.left {
			peripheral &^= 0x04
		}
		if io.InputP2.right {
			peripheral &^= 0x08
		}
		if io.InputP2.btnB {
			peripheral &^= 0x10
		}
		if io.InputP2.btnC {
			peripheral &^= 0x20
		}
	case 1, 3:
		// TH=0: Start, A, 0, 0, Down, Up
		peripheral |= 0x33
		if io.InputP2.up {
			peripheral &^= 0x01
		}
		if io.InputP2.down {
			peripheral &^= 0x02
		}
		if io.InputP2.btnA {
			peripheral &^= 0x10
		}
		if io.InputP2.start {
			peripheral &^= 0x20
		}
	case 5:
		// TH=0, detection: Start, A, 0, 0, 0, 0 (bits 3-0 all zero)
		peripheral |= 0x30
		if io.InputP2.btnA {
			peripheral &^= 0x10
		}
		if io.InputP2.start {
			peripheral &^= 0x20
		}
	case 6:
		// TH=1, extra buttons: C, B, Mode, X, Y, Z
		peripheral |= 0x3F
		if io.InputP2.btnZ {
			peripheral &^= 0x01
		}
		if io.InputP2.btnY {
			peripheral &^= 0x02
		}
		if io.InputP2.btnX {
			peripheral &^= 0x04
		}
		if io.InputP2.btnMode {
			peripheral &^= 0x08
		}
		if io.InputP2.btnB {
			peripheral &^= 0x10
		}
		if io.InputP2.btnC {
			peripheral &^= 0x20
		}
	case 7:
		// TH=0, end marker: Start, A, 1, 1, 1, 1 (bits 3-0 all one)
		peripheral |= 0x3F
		if io.InputP2.btnA {
			peripheral &^= 0x10
		}
		if io.InputP2.start {
			peripheral &^= 0x20
		}
	}

	return (io.p2Data & io.p2Ctrl) | (peripheral & ^io.p2Ctrl)
}
