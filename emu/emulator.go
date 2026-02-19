package emu

import (
	"github.com/user-none/go-chip-m68k"
	"github.com/user-none/go-chip-sn76489"
	"github.com/user-none/go-chip-z80"
)

// EmulatorBase contains fields shared by all platform implementations
type EmulatorBase struct {
	m68k   *m68k.CPU
	z80    *z80.CPU
	z80Mem *Z80Memory
	bus    *GenesisBus
	vdp    *VDP
	psg    *sn76489.SN76489
	ym2612 *YM2612
	io     *IO

	m68kCyclesPerFrame    int
	m68kCyclesPerScanline int
	z80CyclesPerScanline  int

	// Region timing
	region    Region
	timing    RegionTiming
	scanlines int

	// Z80 V-blank interrupt pending delivery. Set at V-blank start,
	// cleared when the Z80 acknowledges the interrupt (IFF1 transitions
	// true->false). This keeps INT asserted until the Z80 is ready to
	// take it, regardless of bus-hold or DI state, while preventing
	// double-firing after the handler re-enables interrupts.
	z80IntPending bool

	// Pre-allocated audio buffer for external consumption
	audioBuffer []int16

	// Low-pass filter state (Model 1 RC filter, persists across frames)
	filterPrevL float64
	filterPrevR float64
}

// InitEmulatorBase creates and initializes the shared emulator components.
func InitEmulatorBase(rom []byte, region Region) (EmulatorBase, error) {
	vdp := NewVDP(region == RegionPAL)
	timing := GetTimingForRegion(region)

	ym2612 := NewYM2612(timing.M68KClockHz, sampleRate)
	psg := sn76489.New(timing.Z80ClockHz, sampleRate, psgBufferSize, sn76489.Sega)
	psg.SetGain(psgGain)
	io := NewIO(vdp, psg, ym2612, region)

	bus := NewGenesisBus(rom, vdp, io, psg, ym2612)
	vdp.SetBus(bus)

	cpu := m68k.New(bus)

	z80Mem := NewZ80Memory(bus)
	z80CPU := z80.New(z80Mem)

	m68kCyclesPerFrame := timing.M68KClockHz / timing.FPS
	m68kCyclesPerScanline := m68kCyclesPerFrame / timing.Scanlines
	z80CyclesPerScanline := (timing.Z80ClockHz / timing.FPS) / timing.Scanlines

	return EmulatorBase{
		m68k:                  cpu,
		z80:                   z80CPU,
		z80Mem:                z80Mem,
		bus:                   bus,
		vdp:                   vdp,
		psg:                   psg,
		ym2612:                ym2612,
		io:                    io,
		m68kCyclesPerFrame:    m68kCyclesPerFrame,
		m68kCyclesPerScanline: m68kCyclesPerScanline,
		z80CyclesPerScanline:  z80CyclesPerScanline,
		region:                region,
		timing:                timing,
		scanlines:             timing.Scanlines,
		audioBuffer:           make([]int16, 0, 2048),
	}, nil
}

// RunFrame executes one frame of emulation.
func (e *EmulatorBase) RunFrame() {
	e.audioBuffer = e.audioBuffer[:0]
	e.psg.ResetBuffer()

	activeHeight := e.vdp.ActiveHeight()

	for i := 0; i < e.scanlines; i++ {
		// Clear HBlank at the start of each scanline
		e.vdp.SetHBlank(false)

		// Update VDP scanline state and check for interrupts
		vInt, hInt := e.vdp.StartScanline(i)
		if vInt {
			e.m68k.RequestInterrupt(6, nil)
		}
		if hInt {
			e.m68k.RequestInterrupt(4, nil)
		}

		// Z80 V-blank interrupt: independent of VDP V-int enable.
		// On real hardware the Z80 INT is tied to the VDP V-blank output.
		// Mark as pending at V-blank start; INT stays asserted until the
		// Z80 acknowledges it during execution below.
		if i == activeHeight {
			e.z80IntPending = true
			e.z80.INT(true, 0xFF)
		}

		// Initialize VDP scanline cycle tracking before M68K runs
		e.vdp.BeginScanline(e.m68k.Cycles(), e.m68kCyclesPerScanline)

		// Run M68K for this scanline using budget-based execution
		budget := e.m68kCyclesPerScanline
		for budget > 0 {
			consumed := e.m68k.StepCycles(budget)
			if consumed == 0 {
				break // CPU halted (double bus fault)
			}
			budget -= consumed
			if stall := e.vdp.DMAStallCycles(); stall > 0 {
				e.m68k.AddCycles(uint64(stall))
				budget -= stall
			}
			// Check for VDP register-triggered interrupts (e.g., enabling
			// V-int while V-int is pending asserts the interrupt line).
			if level := e.vdp.TakeAssertedInterrupt(); level > 0 {
				e.m68k.RequestInterrupt(level, nil)
			}
		}
		scanlineCycles := e.m68kCyclesPerScanline - budget

		// Update H counter based on where we ended up in the scanline
		e.vdp.UpdateHCounter(scanlineCycles, e.m68kCyclesPerScanline)

		// Enter HBlank at end of active display portion
		e.vdp.SetHBlank(true)

		// Handle Z80 reset transition (reset deasserted = Z80 can start)
		if e.bus.z80PendingReset {
			e.z80.Reset()
			e.bus.z80PendingReset = false
		}

		// Run Z80 when not in reset and bus is not requested by the 68K.
		// On real hardware, the Z80 is paused while the 68K holds the bus.
		// This is critical: the 68K often deasserts Z80 reset before releasing
		// the bus, so the Z80 must not start executing until the bus is free.
		if e.bus.z80Reset && !e.bus.z80BusRequested {
			budget := e.z80CyclesPerScanline
			for budget > 0 {
				// While INT is pending, check each step for acknowledgment
				// by watching for IFF1 to transition from true to false.
				var prevIFF1 bool
				if e.z80IntPending {
					prevIFF1 = e.z80.Registers().IFF1
				}

				consumed := e.z80.StepCycles(budget)
				if consumed == 0 {
					break // Z80 halted
				}
				budget -= consumed

				if e.z80IntPending && prevIFF1 && !e.z80.Registers().IFF1 {
					e.z80IntPending = false
					e.z80.INT(false, 0xFF)
				}
			}
		}

		// Render active scanlines
		if i < activeHeight {
			e.vdp.RenderScanline(i)
		}

		// Generate audio for this scanline
		e.ym2612.GenerateSamples(e.m68kCyclesPerScanline)
		e.psg.Run(e.z80CyclesPerScanline)
	}

	e.mixAudio()
}

// SetInput sets Player 1 controller state.
func (e *EmulatorBase) SetInput(up, down, left, right, btnA, btnB, btnC, start, btnX, btnY, btnZ, btnMode bool) {
	e.io.InputP1.Set(up, down, left, right, btnA, btnB, btnC, start, btnX, btnY, btnZ, btnMode)
}

// SetInputP2 sets Player 2 controller state.
func (e *EmulatorBase) SetInputP2(up, down, left, right, btnA, btnB, btnC, start, btnX, btnY, btnZ, btnMode bool) {
	e.io.InputP2.Set(up, down, left, right, btnA, btnB, btnC, start, btnX, btnY, btnZ, btnMode)
}

// SetP2Connected sets whether a Player 2 controller is connected.
// When disconnected, port 2 reads as an empty port (all pins high).
func (e *EmulatorBase) SetP2Connected(connected bool) {
	e.io.InputP2.Connected = connected
}

// SetSixButton enables or disables 6-button controllers on both ports.
// When disabled, controllers behave as 3-button pads and games
// will not detect 6-button support.
func (e *EmulatorBase) SetSixButton(enabled bool) {
	e.io.InputP1.SixButton = enabled
	e.io.InputP2.SixButton = enabled
}

// GetFramebuffer returns raw RGBA pixel data for current frame.
func (e *EmulatorBase) GetFramebuffer() []byte {
	return e.vdp.GetFramebuffer()
}

// GetFramebufferStride returns the stride (bytes per row) of the framebuffer.
func (e *EmulatorBase) GetFramebufferStride() int {
	return e.vdp.GetStride()
}

// GetActiveHeight returns the current active display height.
// Returns doubled height for interlace mode 2.
func (e *EmulatorBase) GetActiveHeight() int {
	return e.vdp.RenderHeight()
}

// GetRegion returns the emulator's region setting.
func (e *EmulatorBase) GetRegion() Region {
	return e.region
}

// GetTiming returns the region timing configuration.
func (e *EmulatorBase) GetTiming() RegionTiming {
	return e.timing
}

// SetRegion updates the emulator's region configuration.
func (e *EmulatorBase) SetRegion(region Region) {
	e.region = region
	e.timing = GetTimingForRegion(region)
	e.scanlines = e.timing.Scanlines
	e.m68kCyclesPerFrame = e.timing.M68KClockHz / e.timing.FPS
	e.m68kCyclesPerScanline = e.m68kCyclesPerFrame / e.timing.Scanlines
	e.z80CyclesPerScanline = (e.timing.Z80ClockHz / e.timing.FPS) / e.timing.Scanlines
}

// HasSRAM returns true if the loaded ROM declares battery-backed SRAM.
func (e *EmulatorBase) HasSRAM() bool {
	return e.bus.HasSRAM()
}

// GetSRAM returns a copy of the current SRAM contents.
func (e *EmulatorBase) GetSRAM() []byte {
	return e.bus.GetSRAM()
}

// SetSRAM loads SRAM contents from a save file.
func (e *EmulatorBase) SetSRAM(data []byte) {
	e.bus.SetSRAM(data)
}
