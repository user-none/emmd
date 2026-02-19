package emu

import "testing"

// makeTestVDP creates a VDP configured for NTSC (isPAL=false).
func makeTestVDP() *VDP {
	return NewVDP(false)
}

// mockBusReader provides word-level reads from a map for DMA testing.
type mockBusReader struct {
	data map[uint32]uint16
}

func (m *mockBusReader) ReadWord(addr uint32) uint16 {
	return m.data[addr]
}

// --- Register tests ---

func TestVDP_RegisterWrite(t *testing.T) {
	vdp := makeTestVDP()
	// Register write: bit 15=1, bit 14=0, reg 1, value 0x14
	// Word = 0x8114 (0x80 | reg<<8 | data)
	vdp.WriteControl(0, 0x8114)
	if vdp.regs[1] != 0x14 {
		t.Errorf("expected reg[1]=0x14, got 0x%02X", vdp.regs[1])
	}
}

func TestVDP_RegisterWriteMultiple(t *testing.T) {
	vdp := makeTestVDP()
	// Write reg 0 = 0x04
	vdp.WriteControl(0, 0x8004)
	// Write reg 15 = 0x02 (auto-increment)
	vdp.WriteControl(0, 0x8F02)
	if vdp.regs[0] != 0x04 {
		t.Errorf("expected reg[0]=0x04, got 0x%02X", vdp.regs[0])
	}
	if vdp.regs[15] != 0x02 {
		t.Errorf("expected reg[15]=0x02, got 0x%02X", vdp.regs[15])
	}
}

func TestVDP_RegisterBoundsCheck(t *testing.T) {
	vdp := makeTestVDP()
	// Register 24+ should be ignored (reg = 0x1F = 31)
	vdp.WriteControl(0, 0x9F00) // reg 31
	// Should not panic; regs array is only 24 entries
}

func TestVDP_ActiveHeight224(t *testing.T) {
	vdp := makeTestVDP()
	if vdp.ActiveHeight() != 224 {
		t.Errorf("expected 224, got %d", vdp.ActiveHeight())
	}
}

func TestVDP_ActiveHeight240(t *testing.T) {
	vdp := makeTestVDP()
	// Set V30 mode: reg 1 bit 3
	vdp.WriteControl(0, 0x8108)
	if vdp.ActiveHeight() != 240 {
		t.Errorf("expected 240, got %d", vdp.ActiveHeight())
	}
}

func TestVDP_ActiveWidthH32(t *testing.T) {
	vdp := makeTestVDP()
	// Default is H32 mode (reg 12 bit 0 = 0)
	if vdp.ActiveWidth() != 256 {
		t.Errorf("expected 256, got %d", vdp.ActiveWidth())
	}
}

func TestVDP_ActiveWidthH40(t *testing.T) {
	vdp := makeTestVDP()
	// Set H40 mode: reg 12 bit 0
	vdp.WriteControl(0, 0x8C81)
	if vdp.ActiveWidth() != 320 {
		t.Errorf("expected 320, got %d", vdp.ActiveWidth())
	}
}

// --- Control port tests ---

func TestVDP_RegisterWriteDetection(t *testing.T) {
	vdp := makeTestVDP()
	// Register write (bit 15=1, bit 14=0) should NOT set writePending
	vdp.WriteControl(0, 0x8000)
	if vdp.writePending {
		t.Error("register write should not set writePending")
	}
}

func TestVDP_TwoWordCommand(t *testing.T) {
	vdp := makeTestVDP()
	// Set up VRAM write to address 0x1234
	// First word: 0x4000 | (0x1234 & 0x3FFF) = 0x5234
	vdp.WriteControl(0, 0x5234)
	if !vdp.writePending {
		t.Error("first word should set writePending")
	}
	// Second word: 0x0000 (VRAM write, upper address bits = 0)
	vdp.WriteControl(0, 0x0000)
	if vdp.writePending {
		t.Error("second word should clear writePending")
	}
	if vdp.address != 0x1234 {
		t.Errorf("expected address 0x1234, got 0x%04X", vdp.address)
	}
}

func TestVDP_StatusFixedUpperBits(t *testing.T) {
	vdp := makeTestVDP()
	status := vdp.ReadControl(0)
	upper := status & 0xFC00
	if upper != 0x7400 {
		t.Errorf("status bits 15:10 expected 0x7400 (011101), got 0x%04X", upper)
	}
}

func TestVDP_StatusReadClearsPending(t *testing.T) {
	vdp := makeTestVDP()
	// Set writePending via first word of two-word command
	vdp.WriteControl(0, 0x4000)
	if !vdp.writePending {
		t.Error("expected writePending=true")
	}
	// Reading status should clear it
	vdp.ReadControl(0)
	if vdp.writePending {
		t.Error("status read should clear writePending")
	}
}

func TestVDP_VRAMWriteSetup(t *testing.T) {
	vdp := makeTestVDP()
	// VRAM write: code = 0x01
	// First word: CD1:CD0 = 01 -> top bits = 01 -> 0x4000 | addr_lo
	vdp.WriteControl(0, 0x4000)
	// Second word: CD5:CD4:CD3:CD2 = 0000, upper addr = 0
	vdp.WriteControl(0, 0x0000)
	if vdp.code&0x0F != 0x01 {
		t.Errorf("expected code low nibble = 0x01, got 0x%02X", vdp.code&0x0F)
	}
}

// --- Data port tests ---

func TestVDP_VRAMWriteRead(t *testing.T) {
	vdp := makeTestVDP()
	// Set auto-increment to 2
	vdp.WriteControl(0, 0x8F02)

	// Set up VRAM write at address 0x0000
	vdp.WriteControl(0, 0x4000)
	vdp.WriteControl(0, 0x0000)

	// Write data
	vdp.WriteData(0, 0xABCD)

	// Set up VRAM read at address 0x0000
	vdp.WriteControl(0, 0x0000)
	vdp.WriteControl(0, 0x0000)

	// ReadData returns the pre-fetched value
	val := vdp.ReadData()
	if val != 0xABCD {
		t.Errorf("expected 0xABCD, got 0x%04X", val)
	}
}

func TestVDP_AutoIncrement(t *testing.T) {
	vdp := makeTestVDP()
	// Set auto-increment to 2
	vdp.WriteControl(0, 0x8F02)

	// Write to VRAM at address 0x0000
	vdp.WriteControl(0, 0x4000)
	vdp.WriteControl(0, 0x0000)

	vdp.WriteData(0, 0x1111)
	vdp.WriteData(0, 0x2222)

	// Read back: set up VRAM read at 0x0000
	vdp.WriteControl(0, 0x0000)
	vdp.WriteControl(0, 0x0000)

	val1 := vdp.ReadData()
	val2 := vdp.ReadData()
	if val1 != 0x1111 {
		t.Errorf("expected first word 0x1111, got 0x%04X", val1)
	}
	if val2 != 0x2222 {
		t.Errorf("expected second word 0x2222, got 0x%04X", val2)
	}
}

func TestVDP_CRAMWriteRead(t *testing.T) {
	vdp := makeTestVDP()
	vdp.WriteControl(0, 0x8F02)

	// CRAM write: code = 0x03
	// First word: CD1:CD0=11 -> 0xC000 | addr
	vdp.WriteControl(0, 0xC000)
	// Second word: CD5:CD4:CD3:CD2 = 0000
	vdp.WriteControl(0, 0x0000)

	vdp.WriteData(0, 0x0EEE)

	// CRAM read: code = 0x08
	// First word: CD1:CD0=00 -> 0x0000 | addr
	vdp.WriteControl(0, 0x0000)
	// Second word: CD5:CD4:CD3:CD2 = 0010 -> val = (0x08 >> 2) << 2 = bits 4:2 -> 0x0020
	vdp.WriteControl(0, 0x0020)

	val := vdp.ReadData()
	if val != 0x0EEE {
		t.Errorf("expected 0x0EEE, got 0x%04X", val)
	}
}

func TestVDP_CRAMWriteRead_UnusedBitsMasked(t *testing.T) {
	vdp := makeTestVDP()
	vdp.WriteControl(0, 0x8F02)

	// CRAM write at address 0
	vdp.WriteControl(0, 0xC000)
	vdp.WriteControl(0, 0x0000)

	// Write 0xFFFF - all bits set, including unused ones.
	// CRAM format: high byte 0000_BBB0, low byte GGG0_RRR0
	// Only 9 color bits should be stored: hi & 0x0E, lo & 0xEE
	vdp.WriteData(0, 0xFFFF)

	// Read back via CRAM read
	vdp.WriteControl(0, 0x0000)
	vdp.WriteControl(0, 0x0020) // CRAM read (CD=0x08)

	val := vdp.ReadData()
	// Expected: 0x0EEE (unused bits stripped)
	if val != 0x0EEE {
		t.Errorf("CRAM unused bits should read as 0: expected 0x0EEE, got 0x%04X", val)
	}
}

func TestVDP_VSRAMWriteRead(t *testing.T) {
	vdp := makeTestVDP()
	vdp.WriteControl(0, 0x8F02)

	// VSRAM write: code = 0x05
	// First word: CD1:CD0=01 -> 0x4000 | addr
	vdp.WriteControl(0, 0x4000)
	// Second word: CD2=1 at bit 4 -> 0x0010
	vdp.WriteControl(0, 0x0010)

	vdp.WriteData(0, 0x0100)

	// VSRAM read: code = 0x04
	// First word: CD1:CD0=00 -> 0x0000
	vdp.WriteControl(0, 0x0000)
	// Second word: CD2=1 at bit 4 -> 0x0010
	vdp.WriteControl(0, 0x0010)

	val := vdp.ReadData()
	if val != 0x0100 {
		t.Errorf("expected 0x0100, got 0x%04X", val)
	}
}

func TestVDP_VSRAMWriteRead_MaskedTo10Bits(t *testing.T) {
	vdp := makeTestVDP()
	vdp.WriteControl(0, 0x8F02)

	// VSRAM write: code = 0x05 (CD1:CD0=01 in first word, CD2=1 in second word bit 4)
	vdp.WriteControl(0, 0x4000)
	vdp.WriteControl(0, 0x0010)

	// Write 0xFFFF - only 10 bits should be retained (0x03FF)
	vdp.WriteData(0, 0xFFFF)

	// VSRAM read: code = 0x04 (CD1:CD0=00 in first word, CD2=1 in second word bit 4)
	vdp.WriteControl(0, 0x0000)
	vdp.WriteControl(0, 0x0010)

	val := vdp.ReadData()
	if val != 0x03FF {
		t.Errorf("VSRAM should mask to 10 bits: expected 0x03FF, got 0x%04X", val)
	}
}

func TestVDP_PrefetchOnReadSetup(t *testing.T) {
	vdp := makeTestVDP()
	vdp.WriteControl(0, 0x8F02)

	// Write known data to VRAM
	vdp.WriteControl(0, 0x4000)
	vdp.WriteControl(0, 0x0000)
	vdp.WriteData(0, 0xDEAD)

	// Set up VRAM read at 0x0000 - should pre-fetch into readBuffer
	vdp.WriteControl(0, 0x0000)
	vdp.WriteControl(0, 0x0000)

	// The readBuffer should contain the pre-fetched value
	if vdp.readBuffer != 0xDEAD {
		t.Errorf("expected readBuffer=0xDEAD, got 0x%04X", vdp.readBuffer)
	}
}

// --- Status register tests ---

func TestVDP_FIFOEmptyFlag(t *testing.T) {
	vdp := makeTestVDP()
	status := vdp.ReadControl(0)
	if status&(1<<9) == 0 {
		t.Error("FIFO empty flag (bit 9) should be set")
	}
}

func TestVDP_PALBit(t *testing.T) {
	vdpNTSC := NewVDP(false)
	vdpPAL := NewVDP(true)

	statusNTSC := vdpNTSC.ReadControl(0)
	if statusNTSC&1 != 0 {
		t.Error("NTSC VDP should have PAL bit clear")
	}

	statusPAL := vdpPAL.ReadControl(0)
	if statusPAL&1 != 1 {
		t.Error("PAL VDP should have PAL bit set")
	}
}

func TestVDP_VBlankFlag(t *testing.T) {
	vdp := makeTestVDP()
	// Initially vBlank is false
	status := vdp.ReadControl(0)
	if status&(1<<3) != 0 {
		t.Error("VBlank should be clear initially")
	}

	// Trigger VBlank by starting scanline at activeHeight
	vdp.StartScanline(0) // clear vBlank
	vdp.StartScanline(224)
	status = vdp.ReadControl(0)
	if status&(1<<3) == 0 {
		t.Error("VBlank should be set after entering VBlank scanline")
	}
}

func TestVDP_VIntClearedOnStatusRead(t *testing.T) {
	vdp := makeTestVDP()
	// Enable V-int
	vdp.WriteControl(0, 0x8120)

	vdp.StartScanline(0)
	vdp.StartScanline(224) // triggers vIntPending

	// First read should have V-int pending
	status := vdp.ReadControl(0)
	if status&(1<<7) == 0 {
		t.Error("V-int pending should be set")
	}

	// Second read should have it cleared
	status = vdp.ReadControl(0)
	if status&(1<<7) != 0 {
		t.Error("V-int pending should be cleared after status read")
	}
}

// --- HV counter tests ---

func TestVDP_HVCounter_BasicVCounter(t *testing.T) {
	vdp := makeTestVDP()
	vdp.StartScanline(10)
	hv := vdp.ReadHVCounter()
	vCount := hv >> 8
	if vCount != 10 {
		t.Errorf("expected V counter = 10, got %d", vCount)
	}
}

func TestVDP_HVCounter_NTSCJump(t *testing.T) {
	vdp := makeTestVDP()
	// Line 234 should be V counter 234
	vdp.StartScanline(234)
	hv := vdp.ReadHVCounter()
	if hv>>8 != 234 {
		t.Errorf("expected V counter 234 at line 234, got %d", hv>>8)
	}

	// Line 235 should jump to 0xE5
	vdp.StartScanline(235)
	hv = vdp.ReadHVCounter()
	if hv>>8 != 0xE5 {
		t.Errorf("expected V counter 0xE5 at line 235, got 0x%02X", hv>>8)
	}

	// Line 261 should be 0xFF
	vdp.StartScanline(261)
	hv = vdp.ReadHVCounter()
	if hv>>8 != 0xFF {
		t.Errorf("expected V counter 0xFF at line 261, got 0x%02X", hv>>8)
	}
}

// --- DMA tests ---

func TestVDP_DMA68KToVRAM(t *testing.T) {
	vdp := makeTestVDP()
	bus := &mockBusReader{data: map[uint32]uint16{
		0x000000: 0x1234,
		0x000002: 0x5678,
		0x000004: 0x9ABC,
	}}
	vdp.SetBus(bus)

	// Enable DMA (reg 1 bit 4)
	vdp.WriteControl(0, 0x8114)
	// Auto-increment = 2
	vdp.WriteControl(0, 0x8F02)
	// DMA length = 3 (reg 19 = 3, reg 20 = 0)
	vdp.WriteControl(0, 0x9303)
	vdp.WriteControl(0, 0x9400)
	// DMA source = 0x000000 >> 1 = 0x000000 (reg 21=0, reg 22=0, reg 23=0)
	vdp.WriteControl(0, 0x9500)
	vdp.WriteControl(0, 0x9600)
	vdp.WriteControl(0, 0x9700)

	// Trigger DMA: VRAM write at address 0x0000 with CD5=1
	// First word: CD1:CD0=01, addr=0x0000 -> 0x4000
	vdp.WriteControl(0, 0x4000)
	// Second word: CD5=1, upper addr=0 -> 0x0080
	vdp.WriteControl(0, 0x0080)

	// Verify VRAM contents
	if vdp.vram[0] != 0x12 || vdp.vram[1] != 0x34 {
		t.Errorf("VRAM[0:2] expected 0x1234, got 0x%02X%02X", vdp.vram[0], vdp.vram[1])
	}
	if vdp.vram[2] != 0x56 || vdp.vram[3] != 0x78 {
		t.Errorf("VRAM[2:4] expected 0x5678, got 0x%02X%02X", vdp.vram[2], vdp.vram[3])
	}
	if vdp.vram[4] != 0x9A || vdp.vram[5] != 0xBC {
		t.Errorf("VRAM[4:6] expected 0x9ABC, got 0x%02X%02X", vdp.vram[4], vdp.vram[5])
	}
}

func TestVDP_DMA68KToVRAM_OddAddress(t *testing.T) {
	vdp := makeTestVDP()
	bus := &mockBusReader{data: map[uint32]uint16{
		0x000000: 0xAABB,
		0x000002: 0xCCDD,
	}}
	vdp.SetBus(bus)

	// Enable DMA (reg 1 bit 4)
	vdp.WriteControl(0, 0x8114)
	// Auto-increment = 2
	vdp.WriteControl(0, 0x8F02)
	// DMA length = 2
	vdp.WriteControl(0, 0x9302)
	vdp.WriteControl(0, 0x9400)
	// DMA source = 0
	vdp.WriteControl(0, 0x9500)
	vdp.WriteControl(0, 0x9600)
	vdp.WriteControl(0, 0x9700)

	// Trigger DMA: VRAM write at address 0x0001 (odd) with CD5=1
	// addr=0x0001: CD1:CD0=01 -> low word = 0x4001, high word = 0x0080
	vdp.WriteControl(0, 0x4001)
	vdp.WriteControl(0, 0x0080)

	// At odd address 0x0001: bytes swap, writing to word-aligned addr 0x0000
	// Word 0xAABB -> vram[0]=0xBB (low byte), vram[1]=0xAA (high byte)
	if vdp.vram[0] != 0xBB || vdp.vram[1] != 0xAA {
		t.Errorf("VRAM[0:2] at odd addr expected 0xBBAA, got 0x%02X%02X", vdp.vram[0], vdp.vram[1])
	}
	// After auto-inc of 2, address = 0x0003 (still odd)
	// Word 0xCCDD -> vram[2]=0xDD, vram[3]=0xCC
	if vdp.vram[2] != 0xDD || vdp.vram[3] != 0xCC {
		t.Errorf("VRAM[2:4] at odd addr expected 0xDDCC, got 0x%02X%02X", vdp.vram[2], vdp.vram[3])
	}
}

func TestVDP_DMA68K_SourceWraps128KB(t *testing.T) {
	vdp := makeTestVDP()
	// Place data at the end of bank 1 (0x020000-0x03FFFF) and verify wrap
	bus := &mockBusReader{data: map[uint32]uint16{
		0x03FFFE: 0x1111, // last word in bank 1
		0x020000: 0x2222, // first word in bank 1 (wrapped)
		0x040000: 0x9999, // first word in bank 2 (should NOT be read)
	}}
	vdp.SetBus(bus)

	// Enable DMA
	vdp.WriteControl(0, 0x8114)
	// Auto-increment = 2
	vdp.WriteControl(0, 0x8F02)
	// DMA length = 2
	vdp.WriteControl(0, 0x9302)
	vdp.WriteControl(0, 0x9400)
	// DMA source = 0x03FFFE >> 1 = 0x01FFFF
	// reg 21 = bits 8:1 = 0xFF, reg 22 = bits 16:9 = 0xFF, reg 23 = bits 22:17 = 0x01
	vdp.WriteControl(0, 0x95FF)
	vdp.WriteControl(0, 0x96FF)
	vdp.WriteControl(0, 0x9701)

	// Trigger DMA: VRAM write at address 0x0000
	vdp.WriteControl(0, 0x4000)
	vdp.WriteControl(0, 0x0080)

	// First word from 0x03FFFE = 0x1111
	if vdp.vram[0] != 0x11 || vdp.vram[1] != 0x11 {
		t.Errorf("VRAM[0:2] expected 0x1111, got 0x%02X%02X", vdp.vram[0], vdp.vram[1])
	}
	// Second word should wrap to 0x020000 = 0x2222, NOT 0x040000 = 0x9999
	if vdp.vram[2] != 0x22 || vdp.vram[3] != 0x22 {
		t.Errorf("VRAM[2:4] expected 0x2222 (wrapped), got 0x%02X%02X", vdp.vram[2], vdp.vram[3])
	}
}

func TestVDP_DMA68KToCRAM(t *testing.T) {
	vdp := makeTestVDP()
	bus := &mockBusReader{data: map[uint32]uint16{
		0x000000: 0x0EEE,
		0x000002: 0x000E,
	}}
	vdp.SetBus(bus)

	// Enable DMA
	vdp.WriteControl(0, 0x8114)
	vdp.WriteControl(0, 0x8F02)
	// DMA length = 2
	vdp.WriteControl(0, 0x9302)
	vdp.WriteControl(0, 0x9400)
	// DMA source = 0
	vdp.WriteControl(0, 0x9500)
	vdp.WriteControl(0, 0x9600)
	vdp.WriteControl(0, 0x9700)

	// CRAM write with DMA: code = 0x03 | CD5
	// First word: CD1:CD0=11, addr=0 -> 0xC000
	vdp.WriteControl(0, 0xC000)
	// Second word: CD5=1, CD3:CD2=00 -> 0x0080
	vdp.WriteControl(0, 0x0080)

	if vdp.cram[0] != 0x0E || vdp.cram[1] != 0xEE {
		t.Errorf("CRAM[0:2] expected 0x0EEE, got 0x%02X%02X", vdp.cram[0], vdp.cram[1])
	}
	if vdp.cram[2] != 0x00 || vdp.cram[3] != 0x0E {
		t.Errorf("CRAM[2:4] expected 0x000E, got 0x%02X%02X", vdp.cram[2], vdp.cram[3])
	}
}

func TestVDP_DMA68KToCRAM_LogsChanges(t *testing.T) {
	vdp := makeTestVDP()
	bus := &mockBusReader{data: map[uint32]uint16{
		0x000000: 0x0EEE,
		0x000002: 0x000E,
	}}
	vdp.SetBus(bus)

	// Begin scanline so change tracking is initialized
	vdp.BeginScanline(0, 488)

	// Enable DMA
	vdp.WriteControl(0, 0x8114)
	vdp.WriteControl(0, 0x8F02)
	// DMA length = 2
	vdp.WriteControl(0, 0x9302)
	vdp.WriteControl(0, 0x9400)
	// DMA source = 0
	vdp.WriteControl(0, 0x9500)
	vdp.WriteControl(0, 0x9600)
	vdp.WriteControl(0, 0x9700)

	// CRAM write with DMA
	vdp.WriteControl(0, 0xC000)
	vdp.WriteControl(0, 0x0080)

	if len(vdp.cramChanges) != 2 {
		t.Fatalf("expected 2 cramChanges, got %d", len(vdp.cramChanges))
	}
	// First word: 0x0EEE -> hi=0x0E, lo=0xEE at addr 0
	if vdp.cramChanges[0].addr != 0 || vdp.cramChanges[0].hi != 0x0E || vdp.cramChanges[0].lo != 0xEE {
		t.Errorf("cramChanges[0] = {addr:%d hi:0x%02X lo:0x%02X}, want {addr:0 hi:0x0E lo:0xEE}",
			vdp.cramChanges[0].addr, vdp.cramChanges[0].hi, vdp.cramChanges[0].lo)
	}
	// Second word: 0x000E -> hi=0x00, lo=0x0E at addr 2
	if vdp.cramChanges[1].addr != 2 || vdp.cramChanges[1].hi != 0x00 || vdp.cramChanges[1].lo != 0x0E {
		t.Errorf("cramChanges[1] = {addr:%d hi:0x%02X lo:0x%02X}, want {addr:2 hi:0x00 lo:0x0E}",
			vdp.cramChanges[1].addr, vdp.cramChanges[1].hi, vdp.cramChanges[1].lo)
	}
}

func TestVDP_DMA68KToVSRAM_LogsChanges(t *testing.T) {
	vdp := makeTestVDP()
	bus := &mockBusReader{data: map[uint32]uint16{
		0x000000: 0x0130, // scroll value 0x0130 (hi=0x01, lo=0x30)
		0x000002: 0x0260, // scroll value 0x0260 (hi=0x02, lo=0x60)
	}}
	vdp.SetBus(bus)

	// Begin scanline so change tracking is initialized
	vdp.BeginScanline(0, 488)

	// Enable DMA
	vdp.WriteControl(0, 0x8114)
	vdp.WriteControl(0, 0x8F02)
	// DMA length = 2
	vdp.WriteControl(0, 0x9302)
	vdp.WriteControl(0, 0x9400)
	// DMA source = 0
	vdp.WriteControl(0, 0x9500)
	vdp.WriteControl(0, 0x9600)
	vdp.WriteControl(0, 0x9700)

	// VSRAM write with DMA: code = 0x05 | CD5
	// First word: CD1:CD0=01, addr=0 -> 0x4000
	vdp.WriteControl(0, 0x4000)
	// Second word: CD5=1, CD3:CD2=01 -> 0x0090
	vdp.WriteControl(0, 0x0090)

	if len(vdp.vsramChanges) != 2 {
		t.Fatalf("expected 2 vsramChanges, got %d", len(vdp.vsramChanges))
	}
	// First word: 0x0130 -> hi=0x01, lo=0x30 at addr 0
	if vdp.vsramChanges[0].addr != 0 || vdp.vsramChanges[0].hi != 0x01 || vdp.vsramChanges[0].lo != 0x30 {
		t.Errorf("vsramChanges[0] = {addr:%d hi:0x%02X lo:0x%02X}, want {addr:0 hi:0x01 lo:0x30}",
			vdp.vsramChanges[0].addr, vdp.vsramChanges[0].hi, vdp.vsramChanges[0].lo)
	}
	// Second word: 0x0260 -> hi=0x02, lo=0x60 at addr 2
	if vdp.vsramChanges[1].addr != 2 || vdp.vsramChanges[1].hi != 0x02 || vdp.vsramChanges[1].lo != 0x60 {
		t.Errorf("vsramChanges[1] = {addr:%d hi:0x%02X lo:0x%02X}, want {addr:2 hi:0x02 lo:0x60}",
			vdp.vsramChanges[1].addr, vdp.vsramChanges[1].hi, vdp.vsramChanges[1].lo)
	}
}

func TestVDP_DMAFill(t *testing.T) {
	vdp := makeTestVDP()
	// Enable DMA
	vdp.WriteControl(0, 0x8114)
	// Auto-increment = 1
	vdp.WriteControl(0, 0x8F01)
	// DMA length = 4
	vdp.WriteControl(0, 0x9304)
	vdp.WriteControl(0, 0x9400)
	// DMA fill mode: reg 23 bits 7:6 = 10 -> 0x80
	vdp.WriteControl(0, 0x9780)

	// Set up VRAM write at address 0x0000 with CD5=1
	vdp.WriteControl(0, 0x4000)
	vdp.WriteControl(0, 0x0080)

	// Now write data to trigger the fill
	vdp.WriteData(0, 0xFF00)

	// First word written normally: vram[0]=0xFF, vram[1]=0x00
	if vdp.vram[0] != 0xFF {
		t.Errorf("VRAM[0] expected 0xFF, got 0x%02X", vdp.vram[0])
	}
	if vdp.vram[1] != 0x00 {
		t.Errorf("VRAM[1] expected 0x00, got 0x%02X", vdp.vram[1])
	}

	// Fill uses ^1 byte-swap: addr^1 gets the fill byte (0xFF)
	// With auto-inc=1, addresses after initial write are 1,2,3,4
	// addr=1 -> vram[1^1=0] = 0xFF (overwrites)
	// addr=2 -> vram[2^1=3] = 0xFF
	// addr=3 -> vram[3^1=2] = 0xFF
	// addr=4 -> vram[4^1=5] = 0xFF
	if vdp.vram[0] != 0xFF {
		t.Errorf("VRAM[0] after fill expected 0xFF, got 0x%02X", vdp.vram[0])
	}
	if vdp.vram[3] != 0xFF {
		t.Errorf("VRAM[3] after fill expected 0xFF, got 0x%02X", vdp.vram[3])
	}
	if vdp.vram[2] != 0xFF {
		t.Errorf("VRAM[2] after fill expected 0xFF, got 0x%02X", vdp.vram[2])
	}
	if vdp.vram[5] != 0xFF {
		t.Errorf("VRAM[5] after fill expected 0xFF, got 0x%02X", vdp.vram[5])
	}
}

func TestVDP_DMAFill_InitialWriteToCRAM(t *testing.T) {
	vdp := makeTestVDP()
	// Enable DMA
	vdp.WriteControl(0, 0x8114)
	// Auto-increment = 2
	vdp.WriteControl(0, 0x8F02)
	// DMA length = 2
	vdp.WriteControl(0, 0x9302)
	vdp.WriteControl(0, 0x9400)
	// DMA fill mode: reg 23 bits 7:6 = 10
	vdp.WriteControl(0, 0x9780)

	// Set up CRAM write at address 0x0000 with CD5=1
	// CRAM write: CD3:0 = 0011, CD5=1
	// First word: CD1:CD0=11 -> 0xC000
	vdp.WriteControl(0, 0xC000)
	// Second word: CD5=1, CD3:CD2=00 -> 0x0080
	vdp.WriteControl(0, 0x0080)

	// Trigger fill with value 0x0E00 - initial word should go to CRAM
	vdp.WriteData(0, 0x0E00)

	// CRAM[0:2] should have the initial word (0x0E, 0x00)
	if vdp.cram[0] != 0x0E || vdp.cram[1] != 0x00 {
		t.Errorf("CRAM[0:2] expected 0x0E00, got 0x%02X%02X", vdp.cram[0], vdp.cram[1])
	}

	// The fill loop still writes to VRAM (fill byte 0x0E via vram[addr^1])
	// Address after initial write + auto-inc = 2, then fill writes at addr 2 and 4
	// addr=2 -> vram[2^1=3] = 0x0E
	// addr=4 -> vram[4^1=5] = 0x0E
	if vdp.vram[3] != 0x0E {
		t.Errorf("VRAM[3] fill expected 0x0E, got 0x%02X", vdp.vram[3])
	}
	if vdp.vram[5] != 0x0E {
		t.Errorf("VRAM[5] fill expected 0x0E, got 0x%02X", vdp.vram[5])
	}
}

func TestVDP_DMACopy(t *testing.T) {
	vdp := makeTestVDP()
	// Pre-populate VRAM source data
	vdp.vram[0x100] = 0xAA
	vdp.vram[0x101] = 0xBB
	vdp.vram[0x102] = 0xCC

	// Enable DMA
	vdp.WriteControl(0, 0x8114)
	// Auto-increment = 1
	vdp.WriteControl(0, 0x8F01)
	// DMA length = 3
	vdp.WriteControl(0, 0x9303)
	vdp.WriteControl(0, 0x9400)
	// DMA source (within VRAM): reg 21=0x00, reg 22=0x01 -> source=0x0100
	vdp.WriteControl(0, 0x9500)
	vdp.WriteControl(0, 0x9601)
	// DMA copy mode: reg 23 bits 7:6 = 11 -> 0xC0
	vdp.WriteControl(0, 0x97C0)

	// Set up VRAM write at address 0x0200 with CD5=1
	vdp.WriteControl(0, 0x4200) // addr low = 0x0200
	vdp.WriteControl(0, 0x00C0) // CD5=1, CD4=1 (copy), upper addr = 0

	// Verify copied data
	if vdp.vram[0x200] != 0xAA {
		t.Errorf("VRAM[0x200] expected 0xAA, got 0x%02X", vdp.vram[0x200])
	}
	if vdp.vram[0x201] != 0xBB {
		t.Errorf("VRAM[0x201] expected 0xBB, got 0x%02X", vdp.vram[0x201])
	}
	if vdp.vram[0x202] != 0xCC {
		t.Errorf("VRAM[0x202] expected 0xCC, got 0x%02X", vdp.vram[0x202])
	}
}

func TestVDP_DMALength0Means65536(t *testing.T) {
	vdp := makeTestVDP()
	bus := &mockBusReader{data: make(map[uint32]uint16)}
	// Fill source data for a large transfer
	for i := uint32(0); i < 0x20000; i += 2 {
		bus.data[i] = 0x00FF
	}
	vdp.SetBus(bus)

	// Enable DMA
	vdp.WriteControl(0, 0x8114)
	vdp.WriteControl(0, 0x8F02)
	// DMA length = 0 (means 0x10000)
	vdp.WriteControl(0, 0x9300)
	vdp.WriteControl(0, 0x9400)
	// Source = 0
	vdp.WriteControl(0, 0x9500)
	vdp.WriteControl(0, 0x9600)
	vdp.WriteControl(0, 0x9700)

	// VRAM write with DMA
	vdp.WriteControl(0, 0x4000)
	vdp.WriteControl(0, 0x0080)

	// Check that the full 64KB was filled (length 0 = 0x10000 words = full VRAM)
	// Spot check a few locations
	if vdp.vram[0] != 0x00 || vdp.vram[1] != 0xFF {
		t.Errorf("VRAM[0:2] expected 0x00FF, got 0x%02X%02X", vdp.vram[0], vdp.vram[1])
	}
	if vdp.vram[0xFFFE] != 0x00 || vdp.vram[0xFFFF] != 0xFF {
		t.Errorf("VRAM[0xFFFE:0x10000] expected 0x00FF, got 0x%02X%02X", vdp.vram[0xFFFE], vdp.vram[0xFFFF])
	}
}

func TestVDP_DMARequiresEnable(t *testing.T) {
	vdp := makeTestVDP()
	bus := &mockBusReader{data: map[uint32]uint16{
		0x000000: 0x1234,
	}}
	vdp.SetBus(bus)

	// DMA NOT enabled (reg 1 bit 4 = 0)
	vdp.WriteControl(0, 0x8104)
	vdp.WriteControl(0, 0x8F02)
	vdp.WriteControl(0, 0x9301)
	vdp.WriteControl(0, 0x9400)
	vdp.WriteControl(0, 0x9500)
	vdp.WriteControl(0, 0x9600)
	vdp.WriteControl(0, 0x9700)

	// Try to trigger DMA
	vdp.WriteControl(0, 0x4000)
	vdp.WriteControl(0, 0x0080)

	// VRAM should be unchanged since DMA was not enabled
	if vdp.vram[0] != 0 || vdp.vram[1] != 0 {
		t.Errorf("VRAM should be unchanged when DMA disabled, got 0x%02X%02X", vdp.vram[0], vdp.vram[1])
	}
}

// --- Scanline/interrupt tests ---

func TestVDP_VBlankEntry(t *testing.T) {
	vdp := makeTestVDP()
	// Enable V-int
	vdp.WriteControl(0, 0x8120)

	vdp.StartScanline(0)
	if vdp.vBlank {
		t.Error("vBlank should be false at line 0")
	}

	vInt, _ := vdp.StartScanline(224)
	if !vdp.vBlank {
		t.Error("vBlank should be true at line 224")
	}
	if !vInt {
		t.Error("vInt should fire at line 224 when enabled")
	}
}

func TestVDP_VIntOnlyWhenEnabled(t *testing.T) {
	vdp := makeTestVDP()
	// V-int NOT enabled (reg 1 bit 5 = 0)
	vdp.WriteControl(0, 0x8100)

	vdp.StartScanline(0)
	vInt, _ := vdp.StartScanline(224)
	if vInt {
		t.Error("vInt should not fire when V-int is disabled")
	}
	// But vIntPending should still be set
	if !vdp.vIntPending {
		t.Error("vIntPending should be set even when V-int is disabled")
	}
}

func TestVDP_VIntReassertOnEnable(t *testing.T) {
	vdp := makeTestVDP()
	// Start with V-int disabled (reg 1 bit 5 = 0)
	vdp.WriteControl(0, 0x8100)

	// Trigger V-blank: vIntPending becomes true, but no vInt since disabled
	vdp.StartScanline(0)
	vInt, _ := vdp.StartScanline(224)
	if vInt {
		t.Error("vInt should not fire when V-int is disabled")
	}
	if !vdp.vIntPending {
		t.Error("vIntPending should be set even when V-int is disabled")
	}

	// No interrupt asserted yet
	if level := vdp.TakeAssertedInterrupt(); level != 0 {
		t.Errorf("no interrupt should be asserted yet, got level %d", level)
	}

	// Enable V-int while vIntPending is true: should assert level 6
	vdp.WriteControl(0, 0x8120)
	if level := vdp.TakeAssertedInterrupt(); level != 6 {
		t.Errorf("enabling V-int with pending should assert level 6, got %d", level)
	}

	// Second call should return 0 (already consumed)
	if level := vdp.TakeAssertedInterrupt(); level != 0 {
		t.Errorf("asserted interrupt should be cleared, got level %d", level)
	}
}

func TestVDP_VIntNoReassertWhenNotPending(t *testing.T) {
	vdp := makeTestVDP()
	// V-int disabled
	vdp.WriteControl(0, 0x8100)

	// No V-blank yet: vIntPending is false
	vdp.StartScanline(0)

	// Enable V-int without vIntPending: should NOT assert
	vdp.WriteControl(0, 0x8120)
	if level := vdp.TakeAssertedInterrupt(); level != 0 {
		t.Errorf("should not assert when vIntPending is false, got level %d", level)
	}
}

func TestVDP_VIntNoReassertWhenAlreadyEnabled(t *testing.T) {
	vdp := makeTestVDP()
	// V-int already enabled
	vdp.WriteControl(0, 0x8120)

	// Trigger V-blank
	vdp.StartScanline(0)
	vdp.StartScanline(224)

	// Write reg 1 again with V-int still enabled (no 0->1 transition)
	vdp.WriteControl(0, 0x8120)
	if level := vdp.TakeAssertedInterrupt(); level != 0 {
		t.Errorf("should not assert on re-write without 0->1 transition, got level %d", level)
	}
}

func TestVDP_HIntCounter(t *testing.T) {
	vdp := makeTestVDP()
	// Enable H-int (reg 0 bit 4)
	vdp.WriteControl(0, 0x8010)
	// Set H-int counter to 2 (reg 10)
	vdp.WriteControl(0, 0x8A02)

	// VBlank line reloads counter to reg[10]=2 (as on real hardware)
	vdp.StartScanline(224)

	// Line 0: decrements counter to 1
	_, hInt := vdp.StartScanline(0)
	if hInt {
		t.Error("H-int should not fire on first line")
	}

	// Line 1: counter decrements to 0
	_, hInt = vdp.StartScanline(1)
	if hInt {
		t.Error("H-int should not fire yet (counter=0)")
	}

	// Line 2: counter decrements to -1 -> fires and reloads
	_, hInt = vdp.StartScanline(2)
	if !hInt {
		t.Error("H-int should fire when counter expires")
	}

	// Line 3: counter reloaded to 2, decrements to 1
	_, hInt = vdp.StartScanline(3)
	if hInt {
		t.Error("H-int should not fire after reload")
	}
}

func TestVDP_HIntCounter_ReloadedAtVBlankStart(t *testing.T) {
	vdp := makeTestVDP()
	vdp.WriteControl(0, 0x8010) // enable H-int
	vdp.WriteControl(0, 0x8A02) // H-int counter = 2

	activeHeight := vdp.ActiveHeight() // 224

	// Run through a few active lines to put counter in an arbitrary state
	vdp.StartScanline(0) // loads 2, decrements to 1
	vdp.StartScanline(1) // decrements to 0

	// Enter VBlank: counter should be reloaded to reg[10] value
	vdp.StartScanline(activeHeight)
	if vdp.hIntCounter != 2 {
		t.Errorf("counter should reload to reg[10] on first VBlank line: got %d, want 2", vdp.hIntCounter)
	}

	// Subsequent VBlank lines decrement like active display
	vdp.StartScanline(activeHeight + 1) // decrements 2 -> 1
	if vdp.hIntCounter != 1 {
		t.Errorf("counter should decrement during VBlank: got %d, want 1", vdp.hIntCounter)
	}

	vdp.StartScanline(activeHeight + 2) // decrements 1 -> 0
	if vdp.hIntCounter != 0 {
		t.Errorf("counter should continue decrementing during VBlank: got %d, want 0", vdp.hIntCounter)
	}
}

func TestVDP_HIntFiresDuringVBlank(t *testing.T) {
	vdp := makeTestVDP()
	vdp.WriteControl(0, 0x8010) // enable H-int
	vdp.WriteControl(0, 0x8A01) // H-int counter = 1

	activeHeight := vdp.ActiveHeight() // 224

	// Enter VBlank: reloads counter to 1
	vdp.StartScanline(activeHeight)

	// VBlank+1: decrements 1 -> 0, no fire
	_, hInt := vdp.StartScanline(activeHeight + 1)
	if hInt {
		t.Error("H-int should not fire yet (counter=0)")
	}

	// VBlank+2: decrements 0 -> -1, fires and reloads to 1
	_, hInt = vdp.StartScanline(activeHeight + 2)
	if !hInt {
		t.Error("H-int should fire during VBlank when counter expires")
	}
	if vdp.hIntCounter != 1 {
		t.Errorf("counter should reload after firing: got %d, want 1", vdp.hIntCounter)
	}

	// VBlank+3: decrements 1 -> 0, no fire
	_, hInt = vdp.StartScanline(activeHeight + 3)
	if hInt {
		t.Error("H-int should not fire after reload")
	}
}

func TestVDP_HIntDisabledDuringVBlank(t *testing.T) {
	vdp := makeTestVDP()
	// H-int DISABLED (reg 0 bit 4 = 0)
	vdp.WriteControl(0, 0x8000)
	vdp.WriteControl(0, 0x8A00) // H-int counter = 0 (fires every line when enabled)

	activeHeight := vdp.ActiveHeight()

	// Enter VBlank: reloads counter to 0
	vdp.StartScanline(activeHeight)

	// Counter expires but H-int is disabled - should not fire
	_, hInt := vdp.StartScanline(activeHeight + 1)
	if hInt {
		t.Error("H-int should not fire when disabled, even if counter expires during VBlank")
	}
}

func TestVDP_VBlankClearsAtLine0(t *testing.T) {
	vdp := makeTestVDP()
	// Trigger vBlank
	vdp.StartScanline(0)
	vdp.StartScanline(224)
	if !vdp.vBlank {
		t.Error("vBlank should be set")
	}
	// Start new frame
	vdp.StartScanline(0)
	if vdp.vBlank {
		t.Error("vBlank should be cleared at line 0")
	}
}

// --- NewVDP tests ---

func TestVDP_NewVDPNTSC(t *testing.T) {
	vdp := NewVDP(false)
	if vdp.isPAL {
		t.Error("expected NTSC")
	}
	if vdp.framebuffer == nil {
		t.Error("expected framebuffer to be allocated")
	}
}

func TestVDP_NewVDPPAL(t *testing.T) {
	vdp := NewVDP(true)
	if !vdp.isPAL {
		t.Error("expected PAL")
	}
}

func TestVDP_GetFramebuffer(t *testing.T) {
	vdp := makeTestVDP()
	fb := vdp.GetFramebuffer()
	expected := ScreenWidth * MaxScreenHeight * 4
	if len(fb) != expected {
		t.Errorf("expected framebuffer size %d, got %d", expected, len(fb))
	}
}

func TestVDP_GetStride(t *testing.T) {
	vdp := makeTestVDP()
	stride := vdp.GetStride()
	if stride != ScreenWidth*4 {
		t.Errorf("expected stride %d, got %d", ScreenWidth*4, stride)
	}
}

func TestVDP_RenderScanlineNoOp(t *testing.T) {
	vdp := makeTestVDP()
	// Should not panic
	vdp.RenderScanline(0)
	vdp.RenderScanline(223)
}

// --- H counter and HV counter latch tests ---

func TestVDP_HCounter_Update_H32(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x00 // H32 mode: active $00-$93, HBlank $E9-$FF

	// At cycle 0, H counter should be 0
	vdp.UpdateHCounter(0, 488)
	if vdp.hCounter != 0 {
		t.Errorf("expected hCounter=0 at cycle 0, got 0x%02X", vdp.hCounter)
	}

	// At midpoint of active region, should be roughly in the middle of 0x00-0x93
	vdp.UpdateHCounter(178, 488) // ~50% of active region (73% of 488 ~= 356)
	if vdp.hCounter < 0x40 || vdp.hCounter > 0x55 {
		t.Errorf("expected hCounter in mid-range (0x40-0x55) at midpoint, got 0x%02X", vdp.hCounter)
	}

	// In HBlank region (past ~73% of scanline), should be >= 0xE9
	vdp.UpdateHCounter(450, 488)
	if vdp.hCounter < 0xE9 {
		t.Errorf("expected hCounter >= 0xE9 in HBlank (H32), got 0x%02X", vdp.hCounter)
	}
}

func TestVDP_HCounter_Update_H40(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40 mode: active $00-$B6, HBlank $E4-$FF

	// At cycle 0, H counter should be 0
	vdp.UpdateHCounter(0, 488)
	if vdp.hCounter != 0 {
		t.Errorf("expected hCounter=0 at cycle 0, got 0x%02X", vdp.hCounter)
	}

	// At midpoint of active region, should be roughly in the middle of 0x00-0xB6
	vdp.UpdateHCounter(178, 488)
	if vdp.hCounter < 0x50 || vdp.hCounter > 0x65 {
		t.Errorf("expected hCounter in mid-range (0x50-0x65) at midpoint, got 0x%02X", vdp.hCounter)
	}

	// In HBlank region (past ~73% of scanline), should be >= 0xE4
	vdp.UpdateHCounter(450, 488)
	if vdp.hCounter < 0xE4 {
		t.Errorf("expected hCounter >= 0xE4 in HBlank (H40), got 0x%02X", vdp.hCounter)
	}
}

func TestVDP_HVCounter_BothComponents(t *testing.T) {
	vdp := makeTestVDP()
	vdp.StartScanline(42)
	vdp.UpdateHCounter(100, 488)

	hv := vdp.ReadHVCounter()
	vCount := hv >> 8
	hCount := hv & 0xFF

	if vCount != 42 {
		t.Errorf("expected V counter=42, got %d", vCount)
	}
	if hCount != uint16(vdp.hCounter) {
		t.Errorf("expected H counter=0x%02X, got 0x%02X", vdp.hCounter, hCount)
	}
}

func TestVDP_HVCounterLatch_Enabled(t *testing.T) {
	vdp := makeTestVDP()

	// Enable HV counter latch (reg 0 bit 1)
	vdp.WriteControl(0, 0x8002)

	// Set up state and latch it
	vdp.StartScanline(10)
	vdp.UpdateHCounter(100, 488)
	vdp.LatchHVCounter()

	latchedHV := vdp.ReadHVCounter()

	// Change counters
	vdp.StartScanline(50)
	vdp.UpdateHCounter(400, 488)

	// Should still return latched value
	readHV := vdp.ReadHVCounter()
	if readHV != latchedHV {
		t.Errorf("expected latched value 0x%04X, got 0x%04X", latchedHV, readHV)
	}
}

func TestVDP_HVCounterLatch_Disabled(t *testing.T) {
	vdp := makeTestVDP()

	// Latch NOT enabled (reg 0 bit 1 = 0)
	vdp.WriteControl(0, 0x8000)

	vdp.StartScanline(10)
	vdp.UpdateHCounter(100, 488)
	vdp.LatchHVCounter() // should not latch

	firstHV := vdp.ReadHVCounter()

	// Change counters
	vdp.StartScanline(50)
	vdp.UpdateHCounter(400, 488)

	// Should return live value, not latched
	readHV := vdp.ReadHVCounter()
	if readHV == firstHV {
		t.Error("without latch, ReadHVCounter should return live value")
	}
}

func TestVDP_HVCounterLatch_ReleaseOnRegWrite(t *testing.T) {
	vdp := makeTestVDP()

	// Enable latch
	vdp.WriteControl(0, 0x8002)
	vdp.StartScanline(10)
	vdp.UpdateHCounter(100, 488)
	vdp.LatchHVCounter()

	latchedHV := vdp.ReadHVCounter()

	// Change counters
	vdp.StartScanline(50)
	vdp.UpdateHCounter(400, 488)

	// Release latch: write reg 0 with bit 1 clear
	vdp.WriteControl(0, 0x8000)

	// Should now return live value
	readHV := vdp.ReadHVCounter()
	if readHV == latchedHV {
		t.Error("after latch release, should return live HV value")
	}
}

func TestVDP_HBlankFlag(t *testing.T) {
	vdp := makeTestVDP()

	// Initially HBlank should be false
	status := vdp.ReadControl(0)
	if status&(1<<2) != 0 {
		t.Error("HBlank should be clear initially")
	}

	vdp.SetHBlank(true)
	status = vdp.ReadControl(0)
	if status&(1<<2) == 0 {
		t.Error("HBlank should be set after SetHBlank(true)")
	}

	vdp.SetHBlank(false)
	status = vdp.ReadControl(0)
	if status&(1<<2) != 0 {
		t.Error("HBlank should be clear after SetHBlank(false)")
	}
}

// --- Interlace mode tests ---

func TestVDP_InterlaceMode_Detection(t *testing.T) {
	vdp := makeTestVDP()

	// Default: no interlace (bits 2:1 = 00)
	if vdp.interlaceMode() != 0 {
		t.Errorf("expected interlace mode 0, got %d", vdp.interlaceMode())
	}

	// Interlace normal: bits 2:1 = 01 -> reg12 bit 1
	vdp.regs[12] = 0x02
	if vdp.interlaceMode() != 1 {
		t.Errorf("expected interlace mode 1, got %d", vdp.interlaceMode())
	}

	// Interlace double-res: bits 2:1 = 11 -> reg12 bits 2 and 1
	vdp.regs[12] = 0x06
	if vdp.interlaceMode() != 3 {
		t.Errorf("expected interlace mode 3, got %d", vdp.interlaceMode())
	}
	if !vdp.interlaceDoubleRes() {
		t.Error("expected interlaceDoubleRes() == true")
	}
}

func TestVDP_InterlaceMode_OddFieldStatusBit(t *testing.T) {
	vdp := makeTestVDP()

	// No interlace: bit 4 should always be 0 regardless of oddField
	vdp.StartScanline(0)
	status := vdp.ReadControl(0)
	if status&(1<<4) != 0 {
		t.Error("ODD bit should be 0 when interlace is off")
	}

	// Enable interlace mode 1
	vdp.regs[12] = 0x02
	vdp.oddField = true
	status = vdp.ReadControl(0)
	if status&(1<<4) == 0 {
		t.Error("ODD bit should be set when interlace is on and oddField is true")
	}

	vdp.oddField = false
	status = vdp.ReadControl(0)
	if status&(1<<4) != 0 {
		t.Error("ODD bit should be clear when oddField is false")
	}

	// Interlace double-res mode
	vdp.regs[12] = 0x06
	vdp.oddField = true
	status = vdp.ReadControl(0)
	if status&(1<<4) == 0 {
		t.Error("ODD bit should be set in double-res interlace with oddField true")
	}
}

func TestVDP_InterlaceMode2_FieldToggle(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x06 // interlace double-res

	initial := vdp.oddField

	// oddField should NOT toggle at line 0 (start of active display)
	vdp.StartScanline(0)
	if vdp.oddField != initial {
		t.Error("oddField should not toggle at line 0")
	}

	// oddField toggles at VBlank start (line == activeHeight)
	activeHeight := vdp.ActiveHeight()
	vdp.StartScanline(activeHeight)
	if vdp.oddField == initial {
		t.Error("oddField should toggle at VBlank start")
	}

	// Second VBlank start toggles back
	vdp.StartScanline(activeHeight)
	if vdp.oddField != initial {
		t.Error("oddField should toggle again on next VBlank start")
	}
}

func TestVDP_InterlaceMode2_FramebufferHeight(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x06

	// Framebuffer should support 480 rows
	fb := vdp.GetFramebuffer()
	expected := ScreenWidth * MaxScreenHeight * 4
	if len(fb) != expected {
		t.Errorf("expected framebuffer size %d, got %d", expected, len(fb))
	}
}

func TestVDP_InterlaceMode2_RenderHeight(t *testing.T) {
	vdp := makeTestVDP()

	// Normal: 224
	if vdp.RenderHeight() != 224 {
		t.Errorf("expected render height 224, got %d", vdp.RenderHeight())
	}

	// Interlace double-res: 448
	vdp.regs[12] = 0x06
	if vdp.RenderHeight() != 448 {
		t.Errorf("expected render height 448, got %d", vdp.RenderHeight())
	}

	// V30 + interlace double-res: 480
	vdp.regs[1] = 0x08
	if vdp.RenderHeight() != 480 {
		t.Errorf("expected render height 480, got %d", vdp.RenderHeight())
	}
}

func TestVDP_InterlaceMode2_TileAddressing(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x06 // interlace double-res: 64-byte tiles, 16 rows

	if vdp.tileSize() != 64 {
		t.Errorf("expected tile size 64, got %d", vdp.tileSize())
	}
	if vdp.tileRows() != 16 {
		t.Errorf("expected 16 tile rows, got %d", vdp.tileRows())
	}

	// Write a 16-row tile at tile index 0 (address 0)
	// Row 0: all color 1
	vdp.vram[0] = 0x11
	vdp.vram[1] = 0x11
	vdp.vram[2] = 0x11
	vdp.vram[3] = 0x11
	// Row 15: all color 2 (at byte offset 15*4=60)
	vdp.vram[60] = 0x22
	vdp.vram[61] = 0x22
	vdp.vram[62] = 0x22
	vdp.vram[63] = 0x22

	// Read row 0 pixel
	c := vdp.decodeTilePixel(0, 0, 0, false, false)
	if c != 1 {
		t.Errorf("expected color 1 at row 0, got %d", c)
	}

	// Read row 15 pixel
	c = vdp.decodeTilePixel(0, 0, 15, false, false)
	if c != 2 {
		t.Errorf("expected color 2 at row 15, got %d", c)
	}

	// VFlip: row 0 should read row 15 (16-row flip)
	c = vdp.decodeTilePixel(0, 0, 0, false, true)
	if c != 2 {
		t.Errorf("expected color 2 at row 0 with VFlip, got %d", c)
	}
}

func TestVDP_InterlaceMode2_RenderScanline(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x87 // H40 + interlace double-res
	vdp.regs[1] = 0x40  // display enabled

	// Set backdrop to color 1 = pure red
	vdp.regs[7] = 0x01
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E

	pix := vdp.framebuffer.Pix
	stride := vdp.framebuffer.Stride

	// Even field: scanline 0 -> fb row 0 or 1 depending on oddField
	vdp.StartScanline(0) // toggles oddField
	field := vdp.oddField
	vdp.RenderScanline(0)

	expectedRow := 0
	if field {
		expectedRow = 1
	}

	p := expectedRow * stride
	if pix[p] != 255 || pix[p+1] != 0 || pix[p+2] != 0 {
		t.Errorf("expected red at fb row %d, got (%d,%d,%d)", expectedRow, pix[p], pix[p+1], pix[p+2])
	}
}

func TestVDP_HVCounter_InterlaceMode1(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x02 // interlace mode 1

	// Line 10, even field: V=10, V8=0
	// Output V byte = (10 & 0xFE) | 0 = 0x0A
	vdp.StartScanline(10)
	vdp.oddField = false
	hv := vdp.ReadHVCounter()
	vByte := hv >> 8
	if vByte != 0x0A {
		t.Errorf("mode 1, line 10, even: expected V=0x0A, got 0x%02X", vByte)
	}

	// Line 10, odd field: V=10, V8=1
	// Output V byte = (10 & 0xFE) | 1 = 0x0B
	vdp.oddField = true
	hv = vdp.ReadHVCounter()
	vByte = hv >> 8
	if vByte != 0x0B {
		t.Errorf("mode 1, line 10, odd: expected V=0x0B, got 0x%02X", vByte)
	}

	// Line 11, even field: V=11, V8=0
	// Output V byte = (11 & 0xFE) | 0 = 0x0A (bit 0 of V dropped, replaced by V8=0)
	vdp.StartScanline(11)
	vdp.oddField = false
	hv = vdp.ReadHVCounter()
	vByte = hv >> 8
	if vByte != 0x0A {
		t.Errorf("mode 1, line 11, even: expected V=0x0A, got 0x%02X", vByte)
	}

	// Line 11, odd field: V=11, V8=1
	// Output V byte = (11 & 0xFE) | 1 = 0x0B
	vdp.oddField = true
	hv = vdp.ReadHVCounter()
	vByte = hv >> 8
	if vByte != 0x0B {
		t.Errorf("mode 1, line 11, odd: expected V=0x0B, got 0x%02X", vByte)
	}
}

func TestVDP_HVCounter_InterlaceMode3(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x06 // interlace mode 3 (double-res)

	// Line 0, even field: doubled V = 0*2+0 = 0
	// Output = (0 & 0xFE) | (0>>8) = 0x00
	vdp.StartScanline(0)
	vdp.oddField = false
	hv := vdp.ReadHVCounter()
	vByte := hv >> 8
	if vByte != 0x00 {
		t.Errorf("mode 3, line 0, even: expected V=0x00, got 0x%02X", vByte)
	}

	// Line 1, even field: doubled V = 1*2+0 = 2
	// Output = (2 & 0xFE) | (2>>8) = 0x02
	vdp.StartScanline(1)
	vdp.oddField = false
	hv = vdp.ReadHVCounter()
	vByte = hv >> 8
	if vByte != 0x02 {
		t.Errorf("mode 3, line 1, even: expected V=0x02, got 0x%02X", vByte)
	}

	// Line 128, even field: doubled V = 128*2+0 = 256 = 0x100
	// Output = (0x100 & 0xFE) | (0x100>>8) = 0x00 | 0x01 = 0x01
	vdp.StartScanline(128)
	vdp.oddField = false
	hv = vdp.ReadHVCounter()
	vByte = hv >> 8
	if vByte != 0x01 {
		t.Errorf("mode 3, line 128, even: expected V=0x01, got 0x%02X", vByte)
	}

	// Line 128, odd field: doubled V = 128*2+1 = 257 = 0x101
	// Output = (0x101 & 0xFE) | (0x101>>8) = 0x00 | 0x01 = 0x01
	vdp.oddField = true
	hv = vdp.ReadHVCounter()
	vByte = hv >> 8
	if vByte != 0x01 {
		t.Errorf("mode 3, line 128, odd: expected V=0x01, got 0x%02X", vByte)
	}
}

func TestVDP_HVCounter_NonInterlaceUnchanged(t *testing.T) {
	vdp := makeTestVDP()
	// No interlace (default)

	vdp.StartScanline(42)
	hv := vdp.ReadHVCounter()
	vByte := hv >> 8
	if vByte != 42 {
		t.Errorf("non-interlace, line 42: expected V=42, got %d", vByte)
	}

	// oddField should have no effect on readout
	vdp.oddField = true
	hv = vdp.ReadHVCounter()
	vByte = hv >> 8
	if vByte != 42 {
		t.Errorf("non-interlace, line 42, oddField=true: expected V=42, got %d", vByte)
	}
}

func TestVDP_HVCounter_InterlaceLatch(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x02         // interlace mode 1
	vdp.WriteControl(0, 0x8002) // enable HV latch

	vdp.StartScanline(10)
	vdp.oddField = true
	vdp.LatchHVCounter()

	// Latched value should use interlace formatting
	// V=10, V8=1: output = (10 & 0xFE) | 1 = 0x0B
	hv := vdp.ReadHVCounter()
	vByte := hv >> 8
	if vByte != 0x0B {
		t.Errorf("latched interlace mode 1: expected V=0x0B, got 0x%02X", vByte)
	}

	// Changing state shouldn't affect the latched value
	vdp.StartScanline(50)
	vdp.oddField = false
	hv = vdp.ReadHVCounter()
	if hv>>8 != 0x0B {
		t.Errorf("latched value should persist, expected V=0x0B, got 0x%02X", hv>>8)
	}
}

func TestVDP_NormalMode_Unchanged(t *testing.T) {
	vdp := makeTestVDP()
	// Normal mode (no interlace)
	vdp.regs[12] = 0x81 // H40 only
	vdp.regs[1] = 0x40  // display enabled

	// Set backdrop to color 1 = pure red
	vdp.regs[7] = 0x01
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E

	vdp.RenderScanline(0)

	pix := vdp.framebuffer.Pix
	// In normal mode, scanline 0 writes to fb row 0
	if pix[0] != 255 || pix[1] != 0 || pix[2] != 0 {
		t.Errorf("expected red at fb row 0, got (%d,%d,%d)", pix[0], pix[1], pix[2])
	}
}

// --- VRAM odd-address byte-swap tests ---

func TestVDP_VRAMWrite_OddAddress_ByteSwap(t *testing.T) {
	vdp := makeTestVDP()
	vdp.WriteControl(0, 0x8F02) // auto-increment 2

	// Set up VRAM write at odd address 0x0001
	vdp.WriteControl(0, 0x4001) // addr low = 0x0001
	vdp.WriteControl(0, 0x0000) // VRAM write

	vdp.WriteData(0, 0xABCD)

	// Odd address write: word-aligned address = 0x0000
	// Bytes are swapped: vram[0] = low byte (0xCD), vram[1] = high byte (0xAB)
	if vdp.vram[0] != 0xCD {
		t.Errorf("expected vram[0]=0xCD, got 0x%02X", vdp.vram[0])
	}
	if vdp.vram[1] != 0xAB {
		t.Errorf("expected vram[1]=0xAB, got 0x%02X", vdp.vram[1])
	}
}

func TestVDP_VRAMWrite_EvenAddress_Normal(t *testing.T) {
	vdp := makeTestVDP()
	vdp.WriteControl(0, 0x8F02)

	// VRAM write at even address 0x0000
	vdp.WriteControl(0, 0x4000)
	vdp.WriteControl(0, 0x0000)

	vdp.WriteData(0, 0xABCD)

	// Even address: normal byte order
	if vdp.vram[0] != 0xAB {
		t.Errorf("expected vram[0]=0xAB, got 0x%02X", vdp.vram[0])
	}
	if vdp.vram[1] != 0xCD {
		t.Errorf("expected vram[1]=0xCD, got 0x%02X", vdp.vram[1])
	}
}

// --- PAL V30 V-counter tests ---

func TestVDP_PAL_V30_VCounter_ActiveLines(t *testing.T) {
	vdp := NewVDP(true) // PAL
	vdp.regs[1] = 0x08  // V30 mode

	// Lines 0-239 should map directly
	for _, line := range []int{0, 100, 239} {
		vdp.StartScanline(line)
		hv := vdp.ReadHVCounter()
		v := hv >> 8
		if v != uint16(line) {
			t.Errorf("PAL V30 line %d: expected V=%d, got %d", line, line, v)
		}
	}
}

func TestVDP_PAL_V30_VCounter_Jump(t *testing.T) {
	vdp := NewVDP(true)
	vdp.regs[1] = 0x08

	// Line 266 should be 0x0A (266 & 0xFF = 10 = 0x0A)
	vdp.StartScanline(266)
	hv := vdp.ReadHVCounter()
	v := hv >> 8
	if v != 0x0A {
		t.Errorf("PAL V30 line 266: expected V=0x0A, got 0x%02X", v)
	}

	// Line 267 should jump to 0xD2
	vdp.StartScanline(267)
	hv = vdp.ReadHVCounter()
	v = hv >> 8
	if v != 0xD2 {
		t.Errorf("PAL V30 line 267: expected V=0xD2, got 0x%02X", v)
	}

	// Line 312 should be 0xFF (0xD2 + (312-267) = 0xD2 + 45 = 0xFF)
	vdp.StartScanline(312)
	hv = vdp.ReadHVCounter()
	v = hv >> 8
	if v != 0xFF {
		t.Errorf("PAL V30 line 312: expected V=0xFF, got 0x%02X", v)
	}
}

func TestVDP_PAL_V28_VCounter_Unchanged(t *testing.T) {
	vdp := NewVDP(true)
	// V28 mode (V30 bit not set)

	// Line 258: should be 0x02 (258 & 0xFF)
	vdp.StartScanline(258)
	hv := vdp.ReadHVCounter()
	v := hv >> 8
	if v != 0x02 {
		t.Errorf("PAL V28 line 258: expected V=0x02, got 0x%02X", v)
	}

	// Line 259: should jump to 0xCA
	vdp.StartScanline(259)
	hv = vdp.ReadHVCounter()
	v = hv >> 8
	if v != 0xCA {
		t.Errorf("PAL V28 line 259: expected V=0xCA, got 0x%02X", v)
	}
}

func TestVDP_NTSC_VCounter_Unchanged(t *testing.T) {
	vdp := NewVDP(false) // NTSC

	// Line 234: should be 234
	vdp.StartScanline(234)
	hv := vdp.ReadHVCounter()
	v := hv >> 8
	if v != 234 {
		t.Errorf("NTSC line 234: expected V=234, got %d", v)
	}

	// Line 235: should jump to 0xE5
	vdp.StartScanline(235)
	hv = vdp.ReadHVCounter()
	v = hv >> 8
	if v != 0xE5 {
		t.Errorf("NTSC line 235: expected V=0xE5, got 0x%02X", v)
	}
}

// --- Cycle-accurate HV counter tests ---

func TestVDP_ReadHVCounterAtCycle_StartOfScanline(t *testing.T) {
	vdp := makeTestVDP()
	vdp.StartScanline(10)
	vdp.BeginScanline(1000, 488)

	hv := vdp.ReadHVCounterAtCycle(1000)
	h := hv & 0xFF
	if h != 0x00 {
		t.Errorf("expected H=0x00 at start of scanline, got 0x%02X", h)
	}
	v := hv >> 8
	if v != 10 {
		t.Errorf("expected V=10, got %d", v)
	}
}

func TestVDP_ReadHVCounterAtCycle_MidActive_H32(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x00 // H32
	vdp.StartScanline(42)
	vdp.BeginScanline(1000, 488)

	// ~50% through active region: 0.5 * (488*73/100) ~= 178
	hv := vdp.ReadHVCounterAtCycle(1178)
	h := hv & 0xFF
	// Should be roughly midpoint of 0x00-0x93 ~= 0x4A
	if h < 0x40 || h > 0x55 {
		t.Errorf("expected H~=0x4A at mid-active (H32), got 0x%02X", h)
	}
}

func TestVDP_ReadHVCounterAtCycle_MidActive_H40(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40
	vdp.StartScanline(42)
	vdp.BeginScanline(1000, 488)

	hv := vdp.ReadHVCounterAtCycle(1178)
	h := hv & 0xFF
	// Should be roughly midpoint of 0x00-0xB6 ~= 0x5B
	if h < 0x50 || h > 0x65 {
		t.Errorf("expected H~=0x5B at mid-active (H40), got 0x%02X", h)
	}
}

func TestVDP_ReadHVCounterAtCycle_HBlank_H32(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x00 // H32
	vdp.StartScanline(42)
	vdp.BeginScanline(1000, 488)

	// Past active display (73% of 488 ~= 356): cycle 1400 = 400 into scanline
	hv := vdp.ReadHVCounterAtCycle(1400)
	h := hv & 0xFF
	if h < 0xE9 {
		t.Errorf("expected H>=0xE9 in HBlank (H32), got 0x%02X", h)
	}
}

func TestVDP_ReadHVCounterAtCycle_HBlank_H40(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40
	vdp.StartScanline(42)
	vdp.BeginScanline(1000, 488)

	hv := vdp.ReadHVCounterAtCycle(1400)
	h := hv & 0xFF
	if h < 0xE4 {
		t.Errorf("expected H>=0xE4 in HBlank (H40), got 0x%02X", h)
	}
}

func TestVDP_ReadHVCounterAtCycle_Latched(t *testing.T) {
	vdp := makeTestVDP()
	// Enable HV counter latch
	vdp.WriteControl(0, 0x8002)

	vdp.StartScanline(10)
	vdp.BeginScanline(1000, 488)
	vdp.UpdateHCounter(100, 488)
	vdp.LatchHVCounter()

	latchedHV := vdp.ReadHVCounterAtCycle(1000)

	// Move to different position
	vdp.StartScanline(50)
	vdp.BeginScanline(2000, 488)

	// Should still return latched value
	readHV := vdp.ReadHVCounterAtCycle(2200)
	if readHV != latchedHV {
		t.Errorf("expected latched value 0x%04X, got 0x%04X", latchedHV, readHV)
	}
}

func TestVDP_ReadHVCounterAtCycle_NoTracking(t *testing.T) {
	vdp := makeTestVDP()
	vdp.StartScanline(10)
	// Don't call BeginScanline - scanlineTotalCycles remains 0
	vdp.UpdateHCounter(200, 488)

	// Should fall back to stored hCounter
	hv := vdp.ReadHVCounterAtCycle(500)
	h := hv & 0xFF
	if h != uint16(vdp.hCounter) {
		t.Errorf("with no tracking, expected stored hCounter=0x%02X, got 0x%02X", vdp.hCounter, h)
	}
}

// --- VRAM 8-bit read tests ---

func TestVDP_VRAM8BitRead(t *testing.T) {
	vdp := makeTestVDP()
	vdp.WriteControl(0, 0x8F02) // auto-increment 2

	// Write known data to VRAM at address 0
	vdp.vram[0] = 0xAB
	vdp.vram[1] = 0xCD

	// Set up VRAM 8-bit read (code = 0x0C)
	// CD1:CD0 = 00 -> first word = 0x0000
	vdp.WriteControl(0, 0x0000)
	// CD5:CD4:CD3:CD2 = 0011 -> second word bits 5:2 = 0x0C -> (0x0C >> 2) << 2 = 0x000C in position -> 0x0030
	vdp.WriteControl(0, 0x0030)

	val := vdp.ReadData()
	// VRAM 8-bit read: byte at address^1 in low byte
	// Address 0: reads vram[0^1] = vram[1] = 0xCD
	if val != 0x00CD {
		t.Errorf("expected 0x00CD, got 0x%04X", val)
	}
}

func TestVDP_VRAM8BitRead_AutoIncrement(t *testing.T) {
	vdp := makeTestVDP()
	vdp.WriteControl(0, 0x8F02) // auto-increment 2

	vdp.vram[0] = 0x11
	vdp.vram[1] = 0x22
	vdp.vram[2] = 0x33
	vdp.vram[3] = 0x44

	// VRAM 8-bit read at address 0
	vdp.WriteControl(0, 0x0000)
	vdp.WriteControl(0, 0x0030)

	val1 := vdp.ReadData() // reads from addr 0, then increments by 2
	val2 := vdp.ReadData() // reads from addr 2

	// First read: vram[0^1] = vram[1] = 0x22
	if val1 != 0x0022 {
		t.Errorf("first read: expected 0x0022, got 0x%04X", val1)
	}
	// Second read: vram[2^1] = vram[3] = 0x44
	if val2 != 0x0044 {
		t.Errorf("second read: expected 0x0044, got 0x%04X", val2)
	}
}

func TestVDP_H40Mode(t *testing.T) {
	tests := []struct {
		name string
		reg  uint8
		want bool
	}{
		{"both bits set (0x81) = H40", 0x81, true},
		{"no bits set = H32", 0x00, false},
		{"only bit 7 set = H32", 0x80, false},
		{"only bit 0 set = H40", 0x01, true},
		{"both bits plus others = H40", 0xFF, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vdp := makeTestVDP()
			vdp.regs[12] = tt.reg
			if got := vdp.h40Mode(); got != tt.want {
				t.Errorf("h40Mode() with reg[12]=0x%02X: got %v, want %v", tt.reg, got, tt.want)
			}
		})
	}
}

func TestVDP_HBlankAtCycle_DuringActive(t *testing.T) {
	vdp := makeTestVDP()
	vdp.BeginScanline(1000, 488)

	// Cycle 1100 is 100 cycles into a 488-cycle scanline.
	// Active boundary is (488*73)/100 = 356 cycles. 100 < 356, so active.
	if vdp.isHBlankAtCycle(1100) {
		t.Error("expected active display (not HBlank) at cycle 1100")
	}
}

func TestVDP_HBlankAtCycle_DuringHBlank(t *testing.T) {
	vdp := makeTestVDP()
	vdp.BeginScanline(1000, 488)

	// Cycle 1400 is 400 cycles into a 488-cycle scanline.
	// Active boundary is 356. 400 >= 356, so HBlank.
	if !vdp.isHBlankAtCycle(1400) {
		t.Error("expected HBlank at cycle 1400")
	}
}

func TestVDP_ReadControl_HBlankCycleAware(t *testing.T) {
	vdp := makeTestVDP()
	vdp.BeginScanline(1000, 488)

	// Mid-scanline (active): bit 2 should be clear
	status := vdp.ReadControl(1100)
	if status&(1<<2) != 0 {
		t.Error("HBlank bit should be clear during active display")
	}

	// Late scanline (HBlank): bit 2 should be set
	// ReadControl clears writePending/vIntPending, but not hBlank-related state
	status = vdp.ReadControl(1400)
	if status&(1<<2) == 0 {
		t.Error("HBlank bit should be set during HBlank portion of scanline")
	}

	// Verify other bits are unaffected (FIFO empty + fixed upper bits)
	if status&0xFE00 != 0x7600 {
		t.Errorf("expected upper bits 0x7600, got 0x%04X", status&0xFE00)
	}
}

// --- DMA busy duration tests ---

func TestVDP_dmaBytesPerLine(t *testing.T) {
	// Throughput table: mode x H-mode x blank
	tests := []struct {
		name  string
		mode  int
		h40   bool
		blank bool
		want  int
	}{
		{"68K H32 active", 0, false, false, 16},
		{"68K H32 blank", 0, false, true, 167},
		{"68K H40 active", 0, true, false, 18},
		{"68K H40 blank", 0, true, true, 205},
		{"Fill H32 active", 1, false, false, 15},
		{"Fill H32 blank", 1, false, true, 166},
		{"Fill H40 active", 1, true, false, 17},
		{"Fill H40 blank", 1, true, true, 204},
		{"Copy H32 active", 2, false, false, 8},
		{"Copy H32 blank", 2, false, true, 83},
		{"Copy H40 active", 2, true, false, 9},
		{"Copy H40 blank", 2, true, true, 102},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vdp := makeTestVDP()
			if tt.h40 {
				vdp.regs[12] = 0x81
			}
			got := vdp.dmaBytesPerLine(tt.mode, tt.blank)
			if got != tt.want {
				t.Errorf("dmaBytesPerLine(%d, %v) = %d, want %d", tt.mode, tt.blank, got, tt.want)
			}
		})
	}
}

func TestVDP_DMA68K_BusyDuration(t *testing.T) {
	vdp := makeTestVDP()
	bus := &mockBusReader{data: make(map[uint32]uint16)}
	for i := uint32(0); i < 20; i += 2 {
		bus.data[i] = 0x1234
	}
	vdp.SetBus(bus)

	// H40 active display
	vdp.regs[12] = 0x81
	vdp.vBlank = false

	// Set up scanline timing: 488 cycles/line
	triggerCycle := uint64(1000)
	vdp.BeginScanline(triggerCycle, 488)

	// Enable DMA
	vdp.WriteControl(triggerCycle, 0x8114)
	vdp.WriteControl(triggerCycle, 0x8F02)
	// DMA length = 9 words -> 18 bytes; H40 active = 18 bytes/line = 488 cycles
	vdp.WriteControl(triggerCycle, 0x9309)
	vdp.WriteControl(triggerCycle, 0x9400)
	vdp.WriteControl(triggerCycle, 0x9500)
	vdp.WriteControl(triggerCycle, 0x9600)
	vdp.WriteControl(triggerCycle, 0x9700)

	// Trigger DMA: VRAM write at address 0x0000 with CD5=1
	vdp.WriteControl(triggerCycle, 0x4000)
	vdp.WriteControl(triggerCycle, 0x0080)

	// DMA should be busy at triggerCycle (1000 < 1000+488=1488)
	status := vdp.ReadControl(triggerCycle)
	if status&(1<<1) == 0 {
		t.Error("DMA should be busy at trigger cycle")
	}

	// DMA should be busy at triggerCycle+487 (1487 < 1488)
	status = vdp.ReadControl(triggerCycle + 487)
	if status&(1<<1) == 0 {
		t.Error("DMA should be busy at trigger+487")
	}

	// DMA should be done at triggerCycle+488 (1488 >= 1488)
	status = vdp.ReadControl(triggerCycle + 488)
	if status&(1<<1) != 0 {
		t.Error("DMA should not be busy at trigger+488")
	}
}

func TestVDP_DMAFill_BusyDuration(t *testing.T) {
	vdp := makeTestVDP()

	// H40 active display
	vdp.regs[12] = 0x81
	vdp.vBlank = false

	triggerCycle := uint64(1000)
	vdp.BeginScanline(triggerCycle, 488)

	// Enable DMA
	vdp.WriteControl(triggerCycle, 0x8114)
	vdp.WriteControl(triggerCycle, 0x8F01)
	// DMA length = 17 bytes; H40 active fill = 17 bytes/line = 488 cycles
	vdp.WriteControl(triggerCycle, 0x9311)
	vdp.WriteControl(triggerCycle, 0x9400)
	// DMA fill mode: reg 23 bits 7:6 = 10
	vdp.WriteControl(triggerCycle, 0x9780)

	// Set up VRAM write at address 0x0000 with CD5=1
	vdp.WriteControl(triggerCycle, 0x4000)
	vdp.WriteControl(triggerCycle, 0x0080)

	// Trigger fill
	vdp.WriteData(triggerCycle, 0xFF00)

	// DMA should be busy at triggerCycle
	status := vdp.ReadControl(triggerCycle)
	if status&(1<<1) == 0 {
		t.Error("DMA fill should be busy at trigger cycle")
	}

	// DMA should be busy at triggerCycle+487
	status = vdp.ReadControl(triggerCycle + 487)
	if status&(1<<1) == 0 {
		t.Error("DMA fill should be busy at trigger+487")
	}

	// DMA should be done at triggerCycle+488
	status = vdp.ReadControl(triggerCycle + 488)
	if status&(1<<1) != 0 {
		t.Error("DMA fill should not be busy at trigger+488")
	}
}

func TestVDP_DMACopy_BusyDuration(t *testing.T) {
	vdp := makeTestVDP()

	// H40 active display
	vdp.regs[12] = 0x81
	vdp.vBlank = false

	// Pre-populate VRAM source data
	for i := 0; i < 16; i++ {
		vdp.vram[0x100+i] = uint8(i)
	}

	triggerCycle := uint64(1000)
	vdp.BeginScanline(triggerCycle, 488)

	// Enable DMA
	vdp.WriteControl(triggerCycle, 0x8114)
	vdp.WriteControl(triggerCycle, 0x8F01)
	// DMA length = 9 bytes; H40 active copy = 9 bytes/line = 488 cycles
	vdp.WriteControl(triggerCycle, 0x9309)
	vdp.WriteControl(triggerCycle, 0x9400)
	// DMA source (within VRAM): reg 21=0x00, reg 22=0x01 -> source=0x0100
	vdp.WriteControl(triggerCycle, 0x9500)
	vdp.WriteControl(triggerCycle, 0x9601)
	// DMA copy mode: reg 23 bits 7:6 = 11
	vdp.WriteControl(triggerCycle, 0x97C0)

	// Trigger copy: VRAM write at address 0x0200 with CD5=1
	vdp.WriteControl(triggerCycle, 0x4200)
	vdp.WriteControl(triggerCycle, 0x00C0)

	// DMA should be busy at triggerCycle
	status := vdp.ReadControl(triggerCycle)
	if status&(1<<1) == 0 {
		t.Error("DMA copy should be busy at trigger cycle")
	}

	// DMA should be busy at triggerCycle+487
	status = vdp.ReadControl(triggerCycle + 487)
	if status&(1<<1) == 0 {
		t.Error("DMA copy should be busy at trigger+487")
	}

	// DMA should be done at triggerCycle+488
	status = vdp.ReadControl(triggerCycle + 488)
	if status&(1<<1) != 0 {
		t.Error("DMA copy should not be busy at trigger+488")
	}
}

func TestVDP_DMA_NotBusyWithoutScanlineCycles(t *testing.T) {
	vdp := makeTestVDP()
	bus := &mockBusReader{data: map[uint32]uint16{
		0x000000: 0x1234,
	}}
	vdp.SetBus(bus)

	// Do NOT call BeginScanline - scanlineTotalCycles stays 0
	vdp.regs[12] = 0x81

	// Enable DMA
	vdp.WriteControl(0, 0x8114)
	vdp.WriteControl(0, 0x8F02)
	vdp.WriteControl(0, 0x9301)
	vdp.WriteControl(0, 0x9400)
	vdp.WriteControl(0, 0x9500)
	vdp.WriteControl(0, 0x9600)
	vdp.WriteControl(0, 0x9700)

	// Trigger DMA with cycle=0 (like Z80 or pre-BeginScanline)
	vdp.WriteControl(0, 0x4000)
	vdp.WriteControl(0, 0x0080)

	// With scanlineTotalCycles=0, dmaEndCycle=triggerCycle=0
	// ReadControl with cycle=0: guard (cycle > 0) prevents busy
	status := vdp.ReadControl(0)
	if status&(1<<1) != 0 {
		t.Error("DMA should not appear busy without scanline timing")
	}

	// Even with a non-zero cycle, dmaEndCycle == cycle, so not busy
	status = vdp.ReadControl(100)
	if status&(1<<1) != 0 {
		t.Error("DMA should not appear busy when dmaEndCycle equals triggerCycle")
	}
}

func TestVDP_DMA68K_StallCycles(t *testing.T) {
	vdp := makeTestVDP()
	bus := &mockBusReader{data: make(map[uint32]uint16)}
	for i := uint32(0); i < 20; i += 2 {
		bus.data[i] = 0x1234
	}
	vdp.SetBus(bus)

	// H40 active display
	vdp.regs[12] = 0x81
	vdp.vBlank = false

	triggerCycle := uint64(1000)
	vdp.BeginScanline(triggerCycle, 488)

	// Enable DMA
	vdp.WriteControl(triggerCycle, 0x8114)
	vdp.WriteControl(triggerCycle, 0x8F02)
	// DMA length = 9 words -> 18 bytes; H40 active = 18 bytes/line = 488 cycles
	vdp.WriteControl(triggerCycle, 0x9309)
	vdp.WriteControl(triggerCycle, 0x9400)
	vdp.WriteControl(triggerCycle, 0x9500)
	vdp.WriteControl(triggerCycle, 0x9600)
	vdp.WriteControl(triggerCycle, 0x9700)

	// Trigger DMA: VRAM write at address 0x0000 with CD5=1
	vdp.WriteControl(triggerCycle, 0x4000)
	vdp.WriteControl(triggerCycle, 0x0080)

	// First call should return 488 stall cycles
	stall := vdp.DMAStallCycles()
	if stall != 488 {
		t.Errorf("DMAStallCycles() = %d, want 488", stall)
	}

	// Second call should return 0 (read-and-clear)
	stall = vdp.DMAStallCycles()
	if stall != 0 {
		t.Errorf("DMAStallCycles() second call = %d, want 0", stall)
	}
}

func TestVDP_DMAFill_NoStall(t *testing.T) {
	vdp := makeTestVDP()

	// H40 active display
	vdp.regs[12] = 0x81
	vdp.vBlank = false

	triggerCycle := uint64(1000)
	vdp.BeginScanline(triggerCycle, 488)

	// Enable DMA
	vdp.WriteControl(triggerCycle, 0x8114)
	vdp.WriteControl(triggerCycle, 0x8F01)
	// DMA length = 17 bytes
	vdp.WriteControl(triggerCycle, 0x9311)
	vdp.WriteControl(triggerCycle, 0x9400)
	// DMA fill mode: reg 23 bits 7:6 = 10
	vdp.WriteControl(triggerCycle, 0x9780)

	// Set up VRAM write at address 0x0000 with CD5=1
	vdp.WriteControl(triggerCycle, 0x4000)
	vdp.WriteControl(triggerCycle, 0x0080)

	// Trigger fill
	vdp.WriteData(triggerCycle, 0xFF00)

	stall := vdp.DMAStallCycles()
	if stall != 0 {
		t.Errorf("DMAStallCycles() after fill = %d, want 0", stall)
	}
}

func TestVDP_DMACopy_NoStall(t *testing.T) {
	vdp := makeTestVDP()

	// H40 active display
	vdp.regs[12] = 0x81
	vdp.vBlank = false

	// Pre-populate VRAM source data
	for i := 0; i < 16; i++ {
		vdp.vram[0x100+i] = uint8(i)
	}

	triggerCycle := uint64(1000)
	vdp.BeginScanline(triggerCycle, 488)

	// Enable DMA
	vdp.WriteControl(triggerCycle, 0x8114)
	vdp.WriteControl(triggerCycle, 0x8F01)
	// DMA length = 9 bytes
	vdp.WriteControl(triggerCycle, 0x9309)
	vdp.WriteControl(triggerCycle, 0x9400)
	// DMA source (within VRAM): reg 21=0x00, reg 22=0x01 -> source=0x0100
	vdp.WriteControl(triggerCycle, 0x9500)
	vdp.WriteControl(triggerCycle, 0x9601)
	// DMA copy mode: reg 23 bits 7:6 = 11
	vdp.WriteControl(triggerCycle, 0x97C0)

	// Trigger copy: VRAM write at address 0x0200 with CD5=1
	vdp.WriteControl(triggerCycle, 0x4200)
	vdp.WriteControl(triggerCycle, 0x00C0)

	stall := vdp.DMAStallCycles()
	if stall != 0 {
		t.Errorf("DMAStallCycles() after copy = %d, want 0", stall)
	}
}

func TestVDP_DMA68K_NoStallWithoutScanlineCycles(t *testing.T) {
	vdp := makeTestVDP()
	bus := &mockBusReader{data: map[uint32]uint16{
		0x000000: 0x1234,
	}}
	vdp.SetBus(bus)

	// Do NOT call BeginScanline - scanlineTotalCycles stays 0
	vdp.regs[12] = 0x81

	// Enable DMA
	vdp.WriteControl(0, 0x8114)
	vdp.WriteControl(0, 0x8F02)
	vdp.WriteControl(0, 0x9301)
	vdp.WriteControl(0, 0x9400)
	vdp.WriteControl(0, 0x9500)
	vdp.WriteControl(0, 0x9600)
	vdp.WriteControl(0, 0x9700)

	// Trigger DMA with cycle=0
	vdp.WriteControl(0, 0x4000)
	vdp.WriteControl(0, 0x0080)

	// Without BeginScanline, dmaEndCycle == triggerCycle, so no stall
	stall := vdp.DMAStallCycles()
	if stall != 0 {
		t.Errorf("DMAStallCycles() without scanline timing = %d, want 0", stall)
	}
}

func TestVDP_DMA_BoundarySpanning_ActiveToVBlank(t *testing.T) {
	// DMA starts near end of active display (line 222 of 224) and spans into VBlank.
	// H40 68K->VDP: active rate = 18 bytes/line, VBlank rate = 205 bytes/line.
	// 2 active lines = 36 bytes at active rate, then remaining at VBlank rate.
	vdp := makeTestVDP()
	bus := &mockBusReader{data: make(map[uint32]uint16)}
	for i := uint32(0); i < 600; i += 2 {
		bus.data[i] = 0x1234
	}
	vdp.SetBus(bus)

	// H40 active display
	vdp.regs[12] = 0x81
	vdp.StartScanline(222) // line 222, activeHeight=224, so 2 lines left
	vdp.vBlank = false

	triggerCycle := uint64(1000)
	vdp.BeginScanline(triggerCycle, 488)

	// DMA length: 100 words = 200 bytes
	// 2 active lines x 18 bytes/line = 36 bytes at active rate
	// Remaining: 200 - 36 = 164 bytes at VBlank rate (205 bytes/line)
	totalBytes := 200
	endCycle := vdp.dmaCalcEndCycle(triggerCycle, totalBytes, 0)

	// First segment: 2 lines x 488 cycles = 976 cycles
	firstDuration := 2 * 488
	// Second segment: 164 bytes at 205 bytes/line
	// fullLines = 164 / 205 = 0, remainder = 164
	// duration = (164 * 488) / 205 = 80024 / 205 = 390
	secondDuration := (164 * 488) / 205
	expectedEnd := triggerCycle + uint64(firstDuration+secondDuration)

	if endCycle != expectedEnd {
		t.Errorf("dmaCalcEndCycle active->VBlank = %d, want %d (first=%d, second=%d)",
			endCycle, expectedEnd, firstDuration, secondDuration)
	}

	// Now trigger a real 68K DMA and verify stall cycles match
	vdp.StartScanline(222)
	vdp.vBlank = false
	vdp.BeginScanline(triggerCycle, 488)

	vdp.WriteControl(triggerCycle, 0x8114) // enable DMA
	vdp.WriteControl(triggerCycle, 0x8F02) // auto-increment 2
	// DMA length = 100 words
	vdp.WriteControl(triggerCycle, 0x9364) // reg 19 = 100
	vdp.WriteControl(triggerCycle, 0x9400) // reg 20 = 0
	vdp.WriteControl(triggerCycle, 0x9500) // source low
	vdp.WriteControl(triggerCycle, 0x9600) // source mid
	vdp.WriteControl(triggerCycle, 0x9700) // source high (mode 0)

	vdp.WriteControl(triggerCycle, 0x4000)
	vdp.WriteControl(triggerCycle, 0x0080)

	stall := vdp.DMAStallCycles()
	expectedStall := int(expectedEnd - triggerCycle)
	if stall != expectedStall {
		t.Errorf("DMAStallCycles() active->VBlank = %d, want %d", stall, expectedStall)
	}
}

func TestVDP_DMA_BoundarySpanning_VBlankToActive(t *testing.T) {
	// DMA starts near end of VBlank (line 260, NTSC 262 total) and spans into active.
	// H40 68K->VDP: VBlank rate = 205 bytes/line, active rate = 18 bytes/line.
	// 2 VBlank lines = 410 bytes at VBlank rate, then remaining at active rate.
	vdp := makeTestVDP()
	bus := &mockBusReader{data: make(map[uint32]uint16)}
	for i := uint32(0); i < 2000; i += 2 {
		bus.data[i] = 0x1234
	}
	vdp.SetBus(bus)

	// H40 VBlank
	vdp.regs[12] = 0x81
	vdp.StartScanline(260) // 262 total, 2 lines left in VBlank
	vdp.vBlank = true

	triggerCycle := uint64(5000)
	vdp.BeginScanline(triggerCycle, 488)

	// DMA: 500 bytes total
	// 2 VBlank lines x 205 bytes/line = 410 bytes at VBlank rate
	// Remaining: 500 - 410 = 90 bytes at active rate (18 bytes/line)
	totalBytes := 500
	endCycle := vdp.dmaCalcEndCycle(triggerCycle, totalBytes, 0)

	// First segment: 2 lines x 488 cycles = 976 cycles
	firstDuration := 2 * 488
	// Second segment: 90 bytes at 18 bytes/line
	// fullLines = 90 / 18 = 5, remainder = 0
	// duration = 5 * 488 = 2440
	secondDuration := 5 * 488
	expectedEnd := triggerCycle + uint64(firstDuration+secondDuration)

	if endCycle != expectedEnd {
		t.Errorf("dmaCalcEndCycle VBlank->active = %d, want %d (first=%d, second=%d)",
			endCycle, expectedEnd, firstDuration, secondDuration)
	}
}

func TestVDP_DMA_NoBoundarySpan_SmallTransfer(t *testing.T) {
	// Small DMA that fits entirely within the current region.
	// Should match single-rate calculation (regression test).
	vdp := makeTestVDP()
	bus := &mockBusReader{data: make(map[uint32]uint16)}
	for i := uint32(0); i < 20; i += 2 {
		bus.data[i] = 0x1234
	}
	vdp.SetBus(bus)

	// H40, line 100 of 224 active - plenty of room
	vdp.regs[12] = 0x81
	vdp.StartScanline(100)
	vdp.vBlank = false

	triggerCycle := uint64(1000)
	vdp.BeginScanline(triggerCycle, 488)

	// 18 bytes = exactly 1 line at H40 active rate (18 bytes/line)
	totalBytes := 18
	endCycle := vdp.dmaCalcEndCycle(triggerCycle, totalBytes, 0)

	// Single-rate: 1 line x 488 cycles = 488
	expectedEnd := triggerCycle + 488
	if endCycle != expectedEnd {
		t.Errorf("dmaCalcEndCycle small transfer = %d, want %d", endCycle, expectedEnd)
	}

	// Also test partial line
	totalBytes = 9
	endCycle = vdp.dmaCalcEndCycle(triggerCycle, totalBytes, 0)
	// 9/18 * 488 = 244
	expectedEnd = triggerCycle + 244
	if endCycle != expectedEnd {
		t.Errorf("dmaCalcEndCycle partial line = %d, want %d", endCycle, expectedEnd)
	}
}

func TestVDP_DMA_BoundarySpanning_Fill(t *testing.T) {
	// Fill DMA spanning active->VBlank.
	// H40 Fill: active rate = 17 bytes/line, VBlank rate = 204 bytes/line.
	vdp := makeTestVDP()

	// H40 active display, line 222 of 224 -> 2 lines left
	vdp.regs[12] = 0x81
	vdp.StartScanline(222)
	vdp.vBlank = false

	triggerCycle := uint64(2000)
	vdp.BeginScanline(triggerCycle, 488)

	// Fill: 100 bytes total
	// 2 active lines x 17 bytes/line = 34 bytes at active rate
	// Remaining: 100 - 34 = 66 bytes at VBlank rate (204 bytes/line)
	totalBytes := 100
	endCycle := vdp.dmaCalcEndCycle(triggerCycle, totalBytes, 1)

	// First segment: 2 lines x 488 cycles = 976
	firstDuration := 2 * 488
	// Second segment: 66 bytes at 204 bytes/line
	// fullLines = 66 / 204 = 0, remainder = 66
	// duration = (66 * 488) / 204 = 32208 / 204 = 157
	secondDuration := (66 * 488) / 204
	expectedEnd := triggerCycle + uint64(firstDuration+secondDuration)

	if endCycle != expectedEnd {
		t.Errorf("dmaCalcEndCycle fill active->VBlank = %d, want %d (first=%d, second=%d)",
			endCycle, expectedEnd, firstDuration, secondDuration)
	}

	// Fill should NOT stall the 68K - trigger an actual fill DMA
	vdp.StartScanline(222)
	vdp.vBlank = false
	vdp.BeginScanline(triggerCycle, 488)

	vdp.WriteControl(triggerCycle, 0x8114) // enable DMA
	vdp.WriteControl(triggerCycle, 0x8F01) // auto-increment 1
	// DMA length = 100 bytes
	vdp.WriteControl(triggerCycle, 0x9364) // reg 19 = 100
	vdp.WriteControl(triggerCycle, 0x9400) // reg 20 = 0
	vdp.WriteControl(triggerCycle, 0x9780) // fill mode

	vdp.WriteControl(triggerCycle, 0x4000)
	vdp.WriteControl(triggerCycle, 0x0080)
	vdp.WriteData(triggerCycle, 0xFF00)

	stall := vdp.DMAStallCycles()
	if stall != 0 {
		t.Errorf("DMAStallCycles() after fill = %d, want 0 (fill doesn't stall 68K)", stall)
	}
}

func TestVDP_DMA68KToCRAM_PerWordPixelOffset(t *testing.T) {
	vdp := makeTestVDP()

	// Set up bus with 7 words of CRAM data
	busData := make(map[uint32]uint16)
	for i := uint32(0); i < 7; i++ {
		busData[i*2] = 0x0EEE
	}
	bus := &mockBusReader{data: busData}
	vdp.SetBus(bus)

	// H40 mode, active display
	vdp.regs[12] = 0x81
	vdp.BeginScanline(0, 488)

	// Enable DMA
	vdp.WriteControl(0, 0x8114)
	vdp.WriteControl(0, 0x8F02)
	// DMA length = 7
	vdp.WriteControl(0, 0x9307)
	vdp.WriteControl(0, 0x9400)
	// DMA source = 0
	vdp.WriteControl(0, 0x9500)
	vdp.WriteControl(0, 0x9600)
	vdp.WriteControl(0, 0x9700)

	// CRAM write with DMA at cycle 0
	vdp.WriteControl(0, 0xC000)
	vdp.WriteControl(0, 0x0080)

	if len(vdp.cramChanges) != 7 {
		t.Fatalf("expected 7 cramChanges, got %d", len(vdp.cramChanges))
	}

	// H40 active: 18 bytes/line = 9 words/line, cyclesPerWord = 488/9 = 54
	// activeEnd = (488*73)/100 = 356, width = 320
	// pixel = (relative * 320) / 356
	wantPixels := []int{0, 48, 97, 145, 194, 242, 291}
	for i, want := range wantPixels {
		got := vdp.cramChanges[i].pixelX
		if got != want {
			t.Errorf("cramChanges[%d].pixelX = %d, want %d", i, got, want)
		}
	}

	// Verify strictly increasing
	for i := 1; i < len(vdp.cramChanges); i++ {
		if vdp.cramChanges[i].pixelX <= vdp.cramChanges[i-1].pixelX {
			t.Errorf("pixelX not strictly increasing: [%d]=%d <= [%d]=%d",
				i, vdp.cramChanges[i].pixelX, i-1, vdp.cramChanges[i-1].pixelX)
		}
	}
}

func TestVDP_DMA68KToVSRAM_PerWordPixelOffset(t *testing.T) {
	vdp := makeTestVDP()

	// Set up bus with 4 words of VSRAM data
	bus := &mockBusReader{data: map[uint32]uint16{
		0x000000: 0x0130,
		0x000002: 0x0260,
		0x000004: 0x0130,
		0x000006: 0x0260,
	}}
	vdp.SetBus(bus)

	// H40 mode, active display
	vdp.regs[12] = 0x81
	vdp.BeginScanline(0, 488)

	// Enable DMA
	vdp.WriteControl(0, 0x8114)
	vdp.WriteControl(0, 0x8F02)
	// DMA length = 4
	vdp.WriteControl(0, 0x9304)
	vdp.WriteControl(0, 0x9400)
	// DMA source = 0
	vdp.WriteControl(0, 0x9500)
	vdp.WriteControl(0, 0x9600)
	vdp.WriteControl(0, 0x9700)

	// VSRAM write with DMA
	vdp.WriteControl(0, 0x4000)
	vdp.WriteControl(0, 0x0090)

	if len(vdp.vsramChanges) != 4 {
		t.Fatalf("expected 4 vsramChanges, got %d", len(vdp.vsramChanges))
	}

	// Same timing as CRAM: cyclesPerWord = 54
	wantPixels := []int{0, 48, 97, 145}
	for i, want := range wantPixels {
		got := vdp.vsramChanges[i].pixelX
		if got != want {
			t.Errorf("vsramChanges[%d].pixelX = %d, want %d", i, got, want)
		}
	}

	// Verify strictly increasing
	for i := 1; i < len(vdp.vsramChanges); i++ {
		if vdp.vsramChanges[i].pixelX <= vdp.vsramChanges[i-1].pixelX {
			t.Errorf("pixelX not strictly increasing: [%d]=%d <= [%d]=%d",
				i, vdp.vsramChanges[i].pixelX, i-1, vdp.vsramChanges[i-1].pixelX)
		}
	}
}

func TestVDP_DMA68KToCRAM_PerWordFallback(t *testing.T) {
	vdp := makeTestVDP()

	busData := make(map[uint32]uint16)
	for i := uint32(0); i < 4; i++ {
		busData[i*2] = 0x0EEE
	}
	bus := &mockBusReader{data: busData}
	vdp.SetBus(bus)

	// H40 mode but do NOT call BeginScanline - scanlineTotalCycles stays 0
	vdp.regs[12] = 0x81

	// Enable DMA
	vdp.WriteControl(0, 0x8114)
	vdp.WriteControl(0, 0x8F02)
	// DMA length = 4
	vdp.WriteControl(0, 0x9304)
	vdp.WriteControl(0, 0x9400)
	// DMA source = 0
	vdp.WriteControl(0, 0x9500)
	vdp.WriteControl(0, 0x9600)
	vdp.WriteControl(0, 0x9700)

	// CRAM write with DMA at cycle 100
	vdp.WriteControl(100, 0xC000)
	vdp.WriteControl(100, 0x0080)

	if len(vdp.cramChanges) != 4 {
		t.Fatalf("expected 4 cramChanges, got %d", len(vdp.cramChanges))
	}

	// Without BeginScanline, cyclesPerWord=0 so all words use the same cycle
	firstPixel := vdp.cramChanges[0].pixelX
	for i := 1; i < len(vdp.cramChanges); i++ {
		if vdp.cramChanges[i].pixelX != firstPixel {
			t.Errorf("cramChanges[%d].pixelX = %d, want %d (same as first - fallback behavior)",
				i, vdp.cramChanges[i].pixelX, firstPixel)
		}
	}
}
