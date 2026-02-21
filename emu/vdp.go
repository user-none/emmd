package emu

import "image"

const (
	ScreenWidth         = 320
	DefaultScreenHeight = 224
	MaxScreenHeight     = 480
)

// BusReader provides word-level read access to the 68K bus for DMA transfers.
type BusReader interface {
	ReadWord(addr uint32) uint16
}

// cramChange records a CRAM write that occurred during active display.
type cramChange struct {
	pixelX int   // pixel position (0-319)
	addr   uint8 // CRAM byte address (always even, 0-126)
	hi     uint8 // high byte
	lo     uint8 // low byte
}

// vsramChange records a VSRAM write that occurred during active display.
type vsramChange struct {
	pixelX int   // pixel position when write occurred
	addr   int   // VSRAM byte address (even, 0-78)
	hi     uint8 // high byte (bits 9:8)
	lo     uint8 // low byte (bits 7:0)
}

// VDP is the Genesis Video Display Processor.
type VDP struct {
	vram  [0x10000]uint8 // 64KB VRAM
	cram  [128]uint8     // 64 color entries x 2 bytes (9-bit color)
	vsram [80]uint8      // 40 scroll entries x 2 bytes

	regs [24]uint8 // VDP registers

	// Control port state machine
	writePending bool
	code         uint8  // CD5-CD0 (6 bits)
	address      uint16 // 16-bit VRAM/CRAM/VSRAM address
	readBuffer   uint16 // Pre-fetch buffer for data reads

	// Status
	vIntPending      bool   // Bit 7: V-int occurred
	spriteOverflow   bool   // Bit 6: too many sprites on a scanline
	spriteCollision  bool   // Bit 5: two sprite pixels overlap
	vBlank           bool   // Bit 3: in VBlank
	hBlank           bool   // Bit 2: in HBlank
	dmaEndCycle      uint64 // Cycle at which current DMA completes (0 = idle)
	dmaStallCycles   int    // 68K cycles the CPU is stalled by 68K->VDP DMA (0 = none)
	assertedIntLevel uint8  // Interrupt level asserted by register write (0 = none)

	// Counters
	vCounter     uint16
	hCounter     uint8
	currentLine  int    // Linear scanline index within the frame (set by StartScanline)
	hvLatched    bool   // HV counter latch active
	hvLatchValue uint16 // Latched HV counter value

	// Interrupt tracking
	hIntCounter    int  // Reloaded from reg 10
	dmaFillPending bool // Waiting for data port write to trigger fill

	// Interlace
	oddField bool // Toggles each frame for interlace modes

	// Region
	isPAL bool

	// Bus reference for DMA 68K transfers
	bus BusReader

	// Framebuffer
	framebuffer *image.RGBA

	// Scanline rendering line buffers (pre-allocated, reused each scanline)
	lineBufB   [320]layerPixel
	lineBufA   [320]layerPixel
	lineBufSpr [320]layerPixel

	// Mid-scanline CRAM change tracking
	cramSnapshot        [128]uint8   // CRAM state at start of scanline (before M68K)
	cramChanges         []cramChange // CRAM writes during this scanline
	scanlineStartCycle  uint64       // CPU cycle at start of current scanline
	scanlineTotalCycles int          // M68K cycles budgeted for this scanline

	// Mid-scanline VSRAM change tracking
	vsramSnapshot [80]uint8     // VSRAM state at start of scanline (before M68K)
	vsramChanges  []vsramChange // VSRAM writes during this scanline
}

// NewVDP creates a new VDP.
func NewVDP(isPAL bool) *VDP {
	return &VDP{
		isPAL:       isPAL,
		framebuffer: image.NewRGBA(image.Rect(0, 0, ScreenWidth, MaxScreenHeight)),
	}
}

// SetBus sets the bus reader for DMA transfers.
// Called after GenesisBus is created due to circular construction dependency.
func (v *VDP) SetBus(bus BusReader) {
	v.bus = bus
}

// DMAStallCycles returns and clears the 68K stall cycles from a
// 68K->VDP DMA. Called by the emulator loop after each StepCycles.
func (v *VDP) DMAStallCycles() int {
	n := v.dmaStallCycles
	v.dmaStallCycles = 0
	return n
}

// TakeAssertedInterrupt returns and clears any interrupt level asserted
// by a VDP register write (e.g., enabling V-int while V-int is pending).
// When a V-int (level 6) is taken, vIntPending is cleared to match the
// real hardware IACK (interrupt acknowledge) cycle behavior.
func (v *VDP) TakeAssertedInterrupt() uint8 {
	level := v.assertedIntLevel
	v.assertedIntLevel = 0
	if level == 6 {
		v.vIntPending = false
	}
	return level
}

// AcknowledgeVInt clears vIntPending, matching the behavior of the real
// hardware IACK (interrupt acknowledge) cycle. On the Genesis, the 68K's
// interrupt acknowledge clears the VDP's V-int pending flag so that
// re-enabling V-int inside the handler does not immediately re-trigger.
func (v *VDP) AcknowledgeVInt() {
	v.vIntPending = false
}

// --- Register helpers ---

func (v *VDP) displayEnabled() bool {
	return v.regs[1]&0x40 != 0
}

func (v *VDP) vIntEnabled() bool {
	return v.regs[1]&0x20 != 0
}

func (v *VDP) dmaEnabled() bool {
	return v.regs[1]&0x10 != 0
}

func (v *VDP) v30Mode() bool {
	return v.regs[1]&0x08 != 0
}

func (v *VDP) hIntEnabled() bool {
	return v.regs[0]&0x10 != 0
}

func (v *VDP) h40Mode() bool {
	return v.regs[12]&0x01 != 0
}

func (v *VDP) autoIncrement() uint16 {
	return uint16(v.regs[15])
}

// ActiveHeight returns the current active display height based on V30 mode.
func (v *VDP) ActiveHeight() int {
	if v.v30Mode() {
		return 240
	}
	return 224
}

// ActiveWidth returns the current active display width based on H40/H32 mode.
func (v *VDP) ActiveWidth() int {
	return v.activeWidth()
}

func (v *VDP) leftColumnBlank() bool {
	return v.regs[0]&0x20 != 0
}

func (v *VDP) activeWidth() int {
	if v.h40Mode() {
		return 320
	}
	return 256
}

// BeginScanline snapshots CRAM/VSRAM and resets change tracking for mid-scanline writes.
// Called at the start of each scanline before M68K runs.
func (v *VDP) BeginScanline(startCycle uint64, totalCycles int) {
	v.cramSnapshot = v.cram
	v.cramChanges = v.cramChanges[:0]
	v.vsramSnapshot = v.vsram
	v.vsramChanges = v.vsramChanges[:0]
	v.scanlineStartCycle = startCycle
	v.scanlineTotalCycles = totalCycles
}

// cycleToPixel maps a CPU cycle to a pixel position within the current scanline.
func (v *VDP) cycleToPixel(cycle uint64) int {
	if cycle <= v.scanlineStartCycle {
		return 0
	}
	relative := int(cycle - v.scanlineStartCycle)
	// Active display is ~73% of scanline cycles
	activeEnd := (v.scanlineTotalCycles * 73) / 100
	width := v.activeWidth()
	if relative >= activeEnd {
		return width - 1
	}
	return (relative * width) / activeEnd
}

// isHBlankAtCycle returns true if the given CPU cycle falls in the HBlank
// portion of the current scanline (the last ~27% of scanline cycles).
func (v *VDP) isHBlankAtCycle(cycle uint64) bool {
	if v.scanlineTotalCycles == 0 {
		return v.hBlank
	}
	relative := int(cycle - v.scanlineStartCycle)
	if relative < 0 {
		return false
	}
	activeBoundary := (v.scanlineTotalCycles * 73) / 100
	return relative >= activeBoundary
}

func (v *VDP) backdropColor() (palette uint8, index uint8) {
	return (v.regs[7] >> 4) & 0x03, v.regs[7] & 0x0F
}

func (v *VDP) hvCounterLatchEnabled() bool {
	return v.regs[0]&0x02 != 0
}

func (v *VDP) shadowHighlightMode() bool {
	return v.regs[12]&0x08 != 0
}

// interlaceMode returns the interlace mode from reg 12 bits 2:1.
// 0 = no interlace, 1 = interlace normal, 2 = invalid, 3 = interlace double-res.
func (v *VDP) interlaceMode() int {
	return int((v.regs[12] >> 1) & 0x03)
}

// interlaceDoubleRes returns true if interlace mode 2 (double resolution) is active.
func (v *VDP) interlaceDoubleRes() bool {
	return v.interlaceMode() == 3
}

// tileSize returns the tile size in bytes (32 for normal, 64 for interlace double-res).
func (v *VDP) tileSize() uint16 {
	if v.interlaceDoubleRes() {
		return 64
	}
	return 32
}

// tileRows returns the number of pixel rows per tile (8 for normal, 16 for interlace double-res).
func (v *VDP) tileRows() int {
	if v.interlaceDoubleRes() {
		return 16
	}
	return 8
}

// RenderHeight returns the framebuffer render height. Double in interlace mode 2.
func (v *VDP) RenderHeight() int {
	if v.interlaceDoubleRes() {
		return v.ActiveHeight() * 2
	}
	return v.ActiveHeight()
}

// --- Control port ---

// WriteControl writes to the VDP control port.
func (v *VDP) WriteControl(cycle uint64, val uint16) {
	// Register writes (bits 15:14 = 10) are ALWAYS detected, even when
	// writePending is true. Per Genesis hardware, a register write cancels
	// any pending two-word command.
	if val&0xC000 == 0x8000 {
		reg := (val >> 8) & 0x1F
		data := uint8(val & 0xFF)
		v.writeRegister(uint8(reg), data)
		v.code = (v.code & 0x3C) | (uint8(val>>14) & 0x03)
		v.writePending = false
		return
	}

	if !v.writePending {
		// First word of two-word command
		v.writePending = true
		v.code = (v.code & 0x3C) | (uint8(val>>14) & 0x03)
		v.address = (v.address & 0xC000) | (val & 0x3FFF)
		return
	}

	// Second word of two-word command
	v.writePending = false
	v.code = (v.code & 0x03) | (uint8(val>>2) & 0x3C)
	v.address = (v.address & 0x3FFF) | ((val & 0x03) << 14)

	// Check for DMA trigger: CD5=1 and DMA enabled
	if v.code&0x20 != 0 && v.dmaEnabled() {
		v.executeDMA(cycle)
		return
	}

	// If this sets up a read command, pre-fetch
	if v.code&0x01 == 0 {
		v.prefetch()
	}
}

// ReadControl returns the VDP status register.
// Reading the status register clears writePending and vIntPending.
func (v *VDP) ReadControl(cycle uint64) uint16 {
	// Bits 15:10 read as fixed value 011101
	var status uint16 = 0x7400

	// Bit 9: FIFO empty (always 1, no FIFO emulation)
	status |= 1 << 9

	// Bit 7: V-int pending
	if v.vIntPending {
		status |= 1 << 7
	}

	// Bit 6: Sprite overflow (too many sprites on a scanline)
	if v.spriteOverflow {
		status |= 1 << 6
	}

	// Bit 5: Sprite collision (two non-transparent sprite pixels overlap)
	if v.spriteCollision {
		status |= 1 << 5
	}

	// Bit 4: Odd frame (interlace modes only)
	if v.oddField && v.interlaceMode() != 0 {
		status |= 1 << 4
	}

	// Bit 3: VBlank
	if v.vBlank {
		status |= 1 << 3
	}

	// Bit 2: HBlank - cycle-aware for 68K, stored flag for Z80
	if v.hBlank || v.isHBlankAtCycle(cycle) {
		status |= 1 << 2
	}

	// Bit 1: DMA busy (active until dmaEndCycle)
	if cycle > 0 && cycle < v.dmaEndCycle {
		status |= 1 << 1
	}

	// Bit 0: PAL flag
	if v.isPAL {
		status |= 1
	}

	// Reading status clears writePending, vIntPending, and sprite flags
	v.writePending = false
	v.vIntPending = false
	v.spriteOverflow = false
	v.spriteCollision = false

	return status
}

// writeRegister writes a value to a VDP register with bounds checking.
func (v *VDP) writeRegister(reg uint8, data uint8) {
	if reg >= 24 {
		return
	}
	oldVal := v.regs[reg]

	// When reg 0 bit 1 transitions from set to clear, release HV latch
	if reg == 0 && oldVal&0x02 != 0 && data&0x02 == 0 {
		v.hvLatched = false
	}
	v.regs[reg] = data

	// When V-int enable (reg 1 bit 5) transitions 0->1 while V-int is
	// pending, the VDP immediately asserts the interrupt line. On real
	// hardware, INT is the logical AND of vIntPending and vIntEnabled,
	// so enabling V-int with a pending interrupt causes immediate assertion.
	if reg == 1 && oldVal&0x20 == 0 && data&0x20 != 0 && v.vIntPending {
		v.assertedIntLevel = 6
	}
}

// --- Data port ---

// WriteData writes to the VDP data port.
func (v *VDP) WriteData(cycle uint64, val uint16) {
	v.writePending = false

	// If a DMA fill is pending, the initial write goes through normal
	// routing below, then the fill loop runs afterward.
	startFill := v.dmaFillPending

	target := v.code & 0x0F
	switch {
	case target == 0x01: // VRAM write
		addr := v.address & 0xFFFF
		if addr&1 == 0 {
			// Even address: normal write
			v.vram[addr] = uint8(val >> 8)
			v.vram[(addr+1)&0xFFFF] = uint8(val)
		} else {
			// Odd address: byte-swap, write to word-aligned address
			wordAddr := addr & 0xFFFE
			v.vram[wordAddr] = uint8(val)
			v.vram[(wordAddr+1)&0xFFFF] = uint8(val >> 8)
		}
	case target == 0x03: // CRAM write
		addr := v.address & 0x7F // 128 bytes (mask to 7 bits)
		hi := uint8(val>>8) & 0x0E
		lo := uint8(val) & 0xEE
		v.cram[addr&0x7E] = hi
		v.cram[(addr&0x7E)+1] = lo
		// Log the change with pixel position for mid-scanline rendering
		v.cramChanges = append(v.cramChanges, cramChange{
			pixelX: v.cycleToPixel(cycle),
			addr:   uint8(addr & 0x7E),
			hi:     hi,
			lo:     lo,
		})
	case target == 0x05: // VSRAM write
		addr := v.address & 0x7F // 80 bytes
		if addr < 80 {
			hi := uint8(val>>8) & 0x03
			lo := uint8(val)
			v.vsram[addr&0x7E] = hi
			if (addr&0x7E)+1 < 80 {
				v.vsram[(addr&0x7E)+1] = lo
			}
			v.vsramChanges = append(v.vsramChanges, vsramChange{
				pixelX: v.cycleToPixel(cycle),
				addr:   int(addr & 0x7E),
				hi:     hi,
				lo:     lo,
			})
		}
	default:
	}

	v.address += v.autoIncrement()

	if startFill {
		v.executeDMAFill(cycle, val)
	}
}

// ReadData reads from the VDP data port.
// Returns the pre-fetched value, then fetches the next value.
func (v *VDP) ReadData() uint16 {
	v.writePending = false

	result := v.readBuffer
	v.prefetch()
	return result
}

// prefetch reads the next value into readBuffer based on current code and address.
func (v *VDP) prefetch() {
	target := v.code & 0x0F
	switch {
	case target == 0x00: // VRAM read
		addr := v.address & 0xFFFF
		v.readBuffer = uint16(v.vram[addr])<<8 | uint16(v.vram[(addr+1)&0xFFFF])
	case target == 0x04: // VSRAM read
		addr := v.address & 0x7F
		if addr < 80 {
			v.readBuffer = uint16(v.vsram[addr&0x7E])<<8 | uint16(v.vsram[(addr&0x7E)+1])
		} else {
			v.readBuffer = 0
		}
	case target == 0x08: // CRAM read
		addr := v.address & 0x7F
		v.readBuffer = uint16(v.cram[addr&0x7E])<<8 | uint16(v.cram[(addr&0x7E)+1])
	case target == 0x0C: // VRAM 8-bit read
		addr := v.address & 0xFFFF
		v.readBuffer = uint16(v.vram[addr^1])
	default:
		v.readBuffer = 0
	}

	v.address += v.autoIncrement()
}

// --- HV Counter ---

// formatHVCounter formats the V and H counter values into the 16-bit HV counter
// readout, applying interlace mode bit rearrangement.
//
// Non-interlace:    V[7:0] | H[7:0]
// Interlace normal: V[7:1]:V[8] | H[7:0]  (9-bit V = vCounter | oddField<<8)
// Interlace double: V[7:1]:V[8] | H[7:0]  (9-bit V = vCounter*2 + oddField)
func (v *VDP) formatHVCounter(h uint8) uint16 {
	var vByte uint16
	switch v.interlaceMode() {
	case 1:
		v9 := v.vCounter & 0xFF
		if v.oddField {
			v9 |= 0x100
		}
		vByte = (v9 & 0xFE) | ((v9 >> 8) & 1)
	case 3:
		v9 := (v.vCounter & 0xFF) * 2
		if v.oddField {
			v9 |= 1
		}
		vByte = (v9 & 0xFE) | ((v9 >> 8) & 1)
	default:
		vByte = v.vCounter & 0xFF
	}
	return vByte<<8 | uint16(h)
}

// ReadHVCounter returns the HV counter value.
func (v *VDP) ReadHVCounter() uint16 {
	if v.hvLatched {
		return v.hvLatchValue
	}
	return v.formatHVCounter(v.hCounter)
}

// ReadHVCounterAtCycle returns the HV counter with H computed from the given CPU cycle.
func (v *VDP) ReadHVCounterAtCycle(cycle uint64) uint16 {
	if v.hvLatched {
		return v.hvLatchValue
	}
	h := v.hCounterFromCycle(cycle)
	return v.formatHVCounter(h)
}

// hCounterRanges returns the active-end and HBlank-start H counter values
// for the current display mode.
//
//	H32: active $00-$93, HBlank $E9-$FF
//	H40: active $00-$B6, HBlank $E4-$FF
func (v *VDP) hCounterRanges() (activeEnd, hblankStart int) {
	if v.h40Mode() {
		return 0xB6, 0xE4
	}
	return 0x93, 0xE9
}

// hCounterFromCycle computes the H counter value from the CPU cycle position
// within the current scanline.
func (v *VDP) hCounterFromCycle(cycle uint64) uint8 {
	if v.scanlineTotalCycles == 0 {
		return v.hCounter
	}
	relative := int(cycle - v.scanlineStartCycle)
	activeBoundary := (v.scanlineTotalCycles * 73) / 100
	activeEnd, hblankStart := v.hCounterRanges()
	if relative < 0 {
		return 0
	}
	if relative < activeBoundary {
		return uint8((relative * activeEnd) / activeBoundary)
	}
	hblankCycles := relative - activeBoundary
	hblankTotal := v.scanlineTotalCycles - activeBoundary
	if hblankTotal > 0 {
		return uint8(hblankStart) + uint8((hblankCycles*(0xFF-hblankStart))/hblankTotal)
	}
	return uint8(hblankStart)
}

// UpdateHCounter approximates the H counter value based on cycle position within a scanline.
// cycleInScanline is how many M68K cycles have elapsed in the current scanline,
// totalCycles is the total M68K cycles per scanline.
func (v *VDP) UpdateHCounter(cycleInScanline, totalCycles int) {
	if totalCycles <= 0 {
		return
	}
	activeEnd, hblankStart := v.hCounterRanges()
	activeBoundary := (totalCycles * 73) / 100
	if cycleInScanline < activeBoundary {
		v.hCounter = uint8((cycleInScanline * activeEnd) / activeBoundary)
	} else {
		hblankCycles := cycleInScanline - activeBoundary
		hblankTotal := totalCycles - activeBoundary
		if hblankTotal > 0 {
			v.hCounter = uint8(hblankStart) + uint8((hblankCycles*(0xFF-hblankStart))/hblankTotal)
		} else {
			v.hCounter = uint8(hblankStart)
		}
	}
}

// LatchHVCounter captures the current HV counter value if latch mode is enabled.
func (v *VDP) LatchHVCounter() {
	if v.hvCounterLatchEnabled() {
		v.hvLatchValue = v.formatHVCounter(v.hCounter)
		v.hvLatched = true
	}
}

// vCounterValue maps a scanline number to the V counter value.
func (v *VDP) vCounterValue(line int) uint16 {
	if v.isPAL {
		if line <= 255 {
			return uint16(line)
		}
		if v.v30Mode() {
			// PAL V30 (240-line): lines 256-266 wrap, then jump to 0xD2 at line 267
			if line < 267 {
				return uint16(line & 0xFF)
			}
			return uint16(0xD2 + (line - 267))
		}
		// PAL V28 (224-line): lines 256-258 wrap, then jump to 0xCA at line 259
		if line < 259 {
			return uint16(line & 0xFF)
		}
		return uint16(0xCA + (line - 259))
	}

	// NTSC 224-line mode
	if line <= 234 {
		return uint16(line)
	}
	// Lines 235-261: V counter jumps to 0xE5-0xFF
	return uint16(0xE5 + (line - 235))
}

// --- Scanline hooks ---

// StartScanline updates VDP state for the start of a scanline.
// Returns vInt and hInt flags indicating if those interrupts should fire.
func (v *VDP) StartScanline(line int) (vInt, hInt bool) {
	v.currentLine = line
	v.vCounter = v.vCounterValue(line)

	activeHeight := v.ActiveHeight()

	if line == 0 {
		// Start of active display
		v.vBlank = false
	}

	if line == activeHeight {
		// Entering VBlank
		v.vBlank = true
		v.vIntPending = true
		v.oddField = !v.oddField
		if v.vIntEnabled() {
			vInt = true
		}
	}

	// H-int counter: on real hardware the counter is loaded from register
	// 10 at line 0 and at each VBlank line. It only decrements during
	// active display (lines 0 to activeHeight-1). When it underflows
	// below 0, H-int fires (if enabled) and the counter reloads.
	// During VBlank the counter is reloaded each line but does not
	// decrement and H-ints do not fire.
	if line == 0 || line >= activeHeight {
		v.hIntCounter = int(v.regs[10])
	}
	if line < activeHeight {
		v.hIntCounter--
		if v.hIntCounter < 0 {
			v.hIntCounter = int(v.regs[10])
			if v.hIntEnabled() {
				hInt = true
			}
		}
	}

	return vInt, hInt
}

// SetHBlank sets the HBlank flag.
func (v *VDP) SetHBlank(active bool) {
	v.hBlank = active
}

// GetFramebuffer returns the raw RGBA pixel data.
func (v *VDP) GetFramebuffer() []byte {
	return v.framebuffer.Pix
}

// GetStride returns the stride (bytes per row) of the framebuffer.
func (v *VDP) GetStride() int {
	return v.framebuffer.Stride
}
