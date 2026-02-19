package emu

import "testing"

// --- Tile decoding tests ---

func TestVDP_DecodeTilePixel_Basic(t *testing.T) {
	vdp := makeTestVDP()

	// Tile at address 0: 8x8 pixels, 4bpp
	// Row 0: bytes [0..3] -> 8 pixels
	// Byte 0: high nibble = pixel 0, low nibble = pixel 1
	vdp.vram[0] = 0x12 // pixel(0,0)=1, pixel(1,0)=2
	vdp.vram[1] = 0x34 // pixel(2,0)=3, pixel(3,0)=4
	vdp.vram[2] = 0x56 // pixel(4,0)=5, pixel(5,0)=6
	vdp.vram[3] = 0x78 // pixel(6,0)=7, pixel(7,0)=8

	for px := 0; px < 8; px++ {
		got := vdp.decodeTilePixel(0, px, 0, false, false)
		want := uint8(px + 1)
		if got != want {
			t.Errorf("pixel(%d,0): expected %d, got %d", px, want, got)
		}
	}
}

func TestVDP_DecodeTilePixel_Row1(t *testing.T) {
	vdp := makeTestVDP()
	// Row 1 starts at byte 4
	vdp.vram[4] = 0xAB // pixel(0,1)=0xA, pixel(1,1)=0xB
	got0 := vdp.decodeTilePixel(0, 0, 1, false, false)
	got1 := vdp.decodeTilePixel(0, 1, 1, false, false)
	if got0 != 0x0A {
		t.Errorf("pixel(0,1): expected 0x0A, got 0x%02X", got0)
	}
	if got1 != 0x0B {
		t.Errorf("pixel(1,1): expected 0x0B, got 0x%02X", got1)
	}
}

func TestVDP_DecodeTilePixel_HFlip(t *testing.T) {
	vdp := makeTestVDP()
	vdp.vram[0] = 0x12
	vdp.vram[1] = 0x34
	vdp.vram[2] = 0x56
	vdp.vram[3] = 0x78

	// With H-flip, pixel 0 should read from the last pixel position (7->8)
	got := vdp.decodeTilePixel(0, 0, 0, true, false)
	if got != 8 {
		t.Errorf("H-flip pixel(0,0): expected 8, got %d", got)
	}
	got = vdp.decodeTilePixel(0, 7, 0, true, false)
	if got != 1 {
		t.Errorf("H-flip pixel(7,0): expected 1, got %d", got)
	}
}

func TestVDP_DecodeTilePixel_VFlip(t *testing.T) {
	vdp := makeTestVDP()
	// Row 0 data
	vdp.vram[0] = 0x10 // pixel(0,0) = 1
	// Row 7 data (byte offset 7*4 = 28)
	vdp.vram[28] = 0xF0 // pixel(0,7) = 0xF

	// V-flip: pixel(0,0) should read from row 7
	got := vdp.decodeTilePixel(0, 0, 0, false, true)
	if got != 0x0F {
		t.Errorf("V-flip pixel(0,0): expected 0x0F, got 0x%02X", got)
	}
}

func TestVDP_DecodeTilePixel_HVFlip(t *testing.T) {
	vdp := makeTestVDP()
	// Put a known value at pixel(7,7) = row 7, pixel 7
	// Row 7 = bytes 28-31, pixel 7 = low nibble of byte 31
	vdp.vram[31] = 0x0F // pixel(6,7)=0, pixel(7,7)=0xF

	// With both H and V flip, pixel(0,0) should read pixel(7,7)
	got := vdp.decodeTilePixel(0, 0, 0, true, true)
	if got != 0x0F {
		t.Errorf("HV-flip pixel(0,0): expected 0x0F, got 0x%02X", got)
	}
}

// --- Nametable size tests ---

func TestVDP_NametableSize_32x32(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[16] = 0x00 // H=0, V=0
	h, v := vdp.nametableSize()
	if h != 32 || v != 32 {
		t.Errorf("expected 32x32, got %dx%d", h, v)
	}
}

func TestVDP_NametableSize_64x32(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[16] = 0x01 // H=1, V=0
	h, v := vdp.nametableSize()
	if h != 64 || v != 32 {
		t.Errorf("expected 64x32, got %dx%d", h, v)
	}
}

func TestVDP_NametableSize_128x64(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[16] = 0x13 // H=3, V=1
	h, v := vdp.nametableSize()
	if h != 128 || v != 64 {
		t.Errorf("expected 128x64, got %dx%d", h, v)
	}
}

// --- Plane B rendering test ---

func TestVDP_RenderPlaneB_SimpleTile(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40

	// Nametable size 32x32
	vdp.regs[16] = 0x00

	// Plane B nametable at VRAM address 0x2000 (reg 4 = 0x01 -> 0x01<<13 = 0x2000)
	vdp.regs[4] = 0x01

	// H-scroll table at 0x1000 (no overlap with tile data)
	vdp.regs[13] = 0x04
	// No scroll (all zeros in hscroll table -> default)

	// Set tile index 1 at cell (0,0) of Plane B nametable
	// Nametable entry: no priority, no flip, palette 0, tile 1
	// Entry = 0x0001
	vdp.vram[0x2000] = 0x00
	vdp.vram[0x2001] = 0x01

	// Tile 1 at VRAM address 32 (1*32)
	// Row 0: all pixels = color 5
	vdp.vram[32] = 0x55
	vdp.vram[33] = 0x55
	vdp.vram[34] = 0x55
	vdp.vram[35] = 0x55

	vdp.renderPlaneB(0)

	// First 8 pixels should all have color index 5
	for x := 0; x < 8; x++ {
		if vdp.lineBufB[x].colorIndex != 5 {
			t.Errorf("pixel %d: expected colorIndex 5, got %d", x, vdp.lineBufB[x].colorIndex)
		}
		if vdp.lineBufB[x].palette != 0 {
			t.Errorf("pixel %d: expected palette 0, got %d", x, vdp.lineBufB[x].palette)
		}
	}
}

func TestVDP_RenderPlaneB_WithPriority(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81
	vdp.regs[16] = 0x00
	vdp.regs[4] = 0x01
	vdp.regs[13] = 0x04 // H-scroll table at 0x1000 to avoid tile data overlap

	// Tile with priority bit set, palette 2, tile index 2
	// Format P PAL[1:0] VF HF TILE[10:0]: 1_10_0_0_00000000010 = 0xC002
	vdp.vram[0x2000] = 0xC0
	vdp.vram[0x2001] = 0x02

	// Tile 2 at VRAM 64: row 0 = all color 1
	vdp.vram[64] = 0x11
	vdp.vram[65] = 0x11
	vdp.vram[66] = 0x11
	vdp.vram[67] = 0x11

	vdp.renderPlaneB(0)

	if !vdp.lineBufB[0].priority {
		t.Error("expected priority=true")
	}
	if vdp.lineBufB[0].palette != 2 {
		t.Errorf("expected palette 2, got %d", vdp.lineBufB[0].palette)
	}
}

// --- H-scroll test ---

func TestVDP_HScrollValues_FullScreen(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[11] = 0x00 // full screen scroll
	vdp.regs[13] = 0x00 // hscroll table at 0x0000

	// Write hscroll: Plane A = 8, Plane B = -4 (0xFFFC as signed 10-bit)
	vdp.vram[0] = 0x00
	vdp.vram[1] = 0x08 // Plane A = 8
	vdp.vram[2] = 0xFF
	vdp.vram[3] = 0xFC // Plane B = -4

	hA, hB := vdp.hScrollValues(0)
	if hA != 8 {
		t.Errorf("expected hScrollA=8, got %d", hA)
	}
	if hB != -4 {
		t.Errorf("expected hScrollB=-4, got %d", hB)
	}

	// Same values for any line in full-screen mode
	hA2, hB2 := vdp.hScrollValues(100)
	if hA2 != hA || hB2 != hB {
		t.Errorf("full-screen scroll should be same for all lines, got (%d,%d)", hA2, hB2)
	}
}

func TestVDP_HScrollValues_PerLine(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[11] = 0x03 // per-line
	vdp.regs[13] = 0x00

	// Line 5: offset = 5 * 4 = 20
	vdp.vram[20] = 0x00
	vdp.vram[21] = 0x10 // Plane A = 16
	vdp.vram[22] = 0x00
	vdp.vram[23] = 0x20 // Plane B = 32

	hA, hB := vdp.hScrollValues(5)
	if hA != 16 {
		t.Errorf("expected hScrollA=16, got %d", hA)
	}
	if hB != 32 {
		t.Errorf("expected hScrollB=32, got %d", hB)
	}
}

func TestVDP_HScrollValues_10BitSignExtension(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[11] = 0x00 // full screen scroll
	vdp.regs[13] = 0x00 // hscroll table at 0x0000

	// 0x0200 = bit 9 set, positive in 16-bit but negative in 10-bit (-512)
	vdp.vram[0] = 0x02
	vdp.vram[1] = 0x00 // Plane A = 0x0200 -> should be -512
	// 0x03FF = all 10 bits set, positive in 16-bit but -1 in 10-bit
	vdp.vram[2] = 0x03
	vdp.vram[3] = 0xFF // Plane B = 0x03FF -> should be -1

	hA, hB := vdp.hScrollValues(0)
	if hA != -512 {
		t.Errorf("expected hScrollA=-512, got %d", hA)
	}
	if hB != -1 {
		t.Errorf("expected hScrollB=-1, got %d", hB)
	}
}

// --- V-scroll test ---

func TestVDP_VScrollValue_FullScreen(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[11] = 0x00 // V-scroll full screen (bit 2=0)

	// VSRAM[0:2] = Plane A, VSRAM[2:4] = Plane B
	vdp.vsram[0] = 0x00
	vdp.vsram[1] = 0x0A // Plane A = 10
	vdp.vsram[2] = 0x00
	vdp.vsram[3] = 0x14 // Plane B = 20

	va := vdp.vScrollValue(0, false)
	vb := vdp.vScrollValue(0, true)
	if va != 10 {
		t.Errorf("expected Plane A vscroll=10, got %d", va)
	}
	if vb != 20 {
		t.Errorf("expected Plane B vscroll=20, got %d", vb)
	}
}

// --- Window tests ---

func TestVDP_WindowPixel_VBoundaryBottom(t *testing.T) {
	vdp := makeTestVDP()
	// Window covers bottom starting at row 10 (80px): reg 18 = 0x80 | 10 = 0x8A
	vdp.regs[18] = 0x8A
	vdp.regs[17] = 0x00 // no H boundary

	// Line 79 should NOT be window (below boundary)
	if vdp.isWindowPixel(0, 79) {
		t.Error("line 79 should not be window")
	}
	// Line 80 should be window
	if !vdp.isWindowPixel(0, 80) {
		t.Error("line 80 should be window")
	}
}

func TestVDP_WindowPixel_VBoundaryTop(t *testing.T) {
	vdp := makeTestVDP()
	// Window covers top up to row 10 (80px): reg 18 = 0x0A
	vdp.regs[18] = 0x0A
	vdp.regs[17] = 0x00

	// Line 79 should be window
	if !vdp.isWindowPixel(0, 79) {
		t.Error("line 79 should be window")
	}
	// Line 80 should NOT be window
	if vdp.isWindowPixel(0, 80) {
		t.Error("line 80 should not be window")
	}
}

func TestVDP_WindowPixel_HBoundaryRight(t *testing.T) {
	vdp := makeTestVDP()
	// Window covers right side starting at column 10 (160px): reg 17 = 0x80 | 10 = 0x8A
	vdp.regs[17] = 0x8A
	vdp.regs[18] = 0x00 // no V boundary

	// Pixel 159 should NOT be window
	if vdp.isWindowPixel(159, 0) {
		t.Error("pixel 159 should not be window")
	}
	// Pixel 160 should be window
	if !vdp.isWindowPixel(160, 0) {
		t.Error("pixel 160 should be window")
	}
}

func TestVDP_WindowReplacesPlaneA(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40
	vdp.regs[16] = 0x00 // 32x32 nametable

	// Plane A nametable at 0x4000 (reg 2 = 0x10 -> 0x10&0x38<<10 = 0x4000)
	vdp.regs[2] = 0x10

	// Window nametable at 0x6000 (H40: reg 3 = 0x18 -> 0x18&0x3C<<10 = 0x6000)
	vdp.regs[3] = 0x18

	// H-scroll table at 0xF000 (far away)
	vdp.regs[13] = 0x3C

	// Window covers top 8 lines
	vdp.regs[18] = 0x01 // top, boundary at 1*8 = 8px
	vdp.regs[17] = 0x00

	// Window nametable cell (0,0): tile 3, palette 1
	// Entry = 0_01_0_0_00000000011 = 0x2003
	vdp.vram[0x6000] = 0x20
	vdp.vram[0x6001] = 0x03

	// Tile 3 at VRAM 96: row 0, all color 7
	vdp.vram[96] = 0x77
	vdp.vram[97] = 0x77
	vdp.vram[98] = 0x77
	vdp.vram[99] = 0x77

	// Plane A cell (0,0): tile 4, palette 0
	vdp.vram[0x4000] = 0x00
	vdp.vram[0x4001] = 0x04

	// Tile 4 at VRAM 128: row 0, all color 2
	vdp.vram[128] = 0x22
	vdp.vram[129] = 0x22
	vdp.vram[130] = 0x22
	vdp.vram[131] = 0x22

	// Render line 0 (within window region)
	for i := 0; i < 320; i++ {
		vdp.lineBufA[i] = layerPixel{}
	}
	vdp.renderPlaneAAndWindow(0)

	// Should get window tile (palette 1, color 7)
	if vdp.lineBufA[0].colorIndex != 7 {
		t.Errorf("expected window colorIndex 7, got %d", vdp.lineBufA[0].colorIndex)
	}
	if vdp.lineBufA[0].palette != 1 {
		t.Errorf("expected window palette 1, got %d", vdp.lineBufA[0].palette)
	}
}

func TestVDP_VScrollValue_Per2Cell(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[11] = 0x04 // V-scroll per-2-cell (bit 2=1)

	// Column 0 (x=0-15): VSRAM[0] for A, VSRAM[2] for B
	vdp.vsram[0] = 0x00
	vdp.vsram[1] = 0x05
	vdp.vsram[2] = 0x00
	vdp.vsram[3] = 0x0A

	// Column 1 (x=16-31): VSRAM[4] for A, VSRAM[6] for B
	vdp.vsram[4] = 0x00
	vdp.vsram[5] = 0x0F
	vdp.vsram[6] = 0x00
	vdp.vsram[7] = 0x14

	vaCol0 := vdp.vScrollValue(0, false)
	vbCol0 := vdp.vScrollValue(0, true)
	if vaCol0 != 5 || vbCol0 != 10 {
		t.Errorf("column 0: expected (5,10), got (%d,%d)", vaCol0, vbCol0)
	}

	vaCol1 := vdp.vScrollValue(16, false)
	vbCol1 := vdp.vScrollValue(16, true)
	if vaCol1 != 15 || vbCol1 != 20 {
		t.Errorf("column 1: expected (15,20), got (%d,%d)", vaCol1, vbCol1)
	}
}

// --- VSRAM mid-scanline change tracking tests ---

func TestVDP_VSRAMChangeTracking_NoChanges(t *testing.T) {
	vdp := makeTestVDP()

	// Full-screen mode
	vdp.regs[11] = 0x00
	vdp.vsram[0] = 0x00
	vdp.vsram[1] = 0x0A // Plane A = 10
	vdp.vsram[2] = 0x00
	vdp.vsram[3] = 0x14 // Plane B = 20

	vdp.BeginScanline(0, 488)

	va := vdp.vScrollValue(0, false)
	vb := vdp.vScrollValue(0, true)
	if va != 10 {
		t.Errorf("full-screen Plane A: expected 10, got %d", va)
	}
	if vb != 20 {
		t.Errorf("full-screen Plane B: expected 20, got %d", vb)
	}

	// Per-2-cell mode
	vdp.regs[11] = 0x04
	vdp.vsram[4] = 0x00
	vdp.vsram[5] = 0x0F

	vdp.BeginScanline(0, 488)

	va2 := vdp.vScrollValue(16, false)
	if va2 != 15 {
		t.Errorf("per-2-cell column 1 Plane A: expected 15, got %d", va2)
	}
}

func TestVDP_VSRAMChangeTracking_FullScreen(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[11] = 0x00 // full-screen V-scroll

	vdp.vsram[0] = 0x00
	vdp.vsram[1] = 0x0A // Plane A = 10

	vdp.BeginScanline(0, 488)

	// Simulate a mid-scanline VSRAM write: Plane A changes to 30
	vdp.vsram[0] = 0x00
	vdp.vsram[1] = 0x1E
	vdp.vsramChanges = append(vdp.vsramChanges, vsramChange{
		pixelX: 160,
		addr:   0,
		hi:     0x00,
		lo:     0x1E,
	})

	// Full-screen mode should return snapshot (pre-write) value
	va := vdp.vScrollValue(0, false)
	if va != 10 {
		t.Errorf("full-screen should latch at scanline start: expected 10, got %d", va)
	}

	// Even at a pixel past the write, full-screen mode still uses snapshot
	va2 := vdp.vScrollValue(200, false)
	if va2 != 10 {
		t.Errorf("full-screen past write pixel: expected 10, got %d", va2)
	}
}

func TestVDP_VSRAMChangeTracking_Per2Cell(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[11] = 0x04 // per-2-cell V-scroll

	// Column 5 (x=80-95): Plane A at VSRAM addr 20, initial = 10
	vdp.vsram[20] = 0x00
	vdp.vsram[21] = 0x0A

	// Column 10 (x=160-175): Plane A at VSRAM addr 40, initial = 20
	vdp.vsram[40] = 0x00
	vdp.vsram[41] = 0x14

	vdp.BeginScanline(0, 488)

	// Write at pixel 48: change column 5 (addr 20) to 50.
	// This write happens BEFORE column 5 starts rendering (pixel 80).
	vdp.vsram[20] = 0x00
	vdp.vsram[21] = 0x32
	vdp.vsramChanges = append(vdp.vsramChanges, vsramChange{
		pixelX: 48,
		addr:   20,
		hi:     0x00,
		lo:     0x32,
	})

	// Write at pixel 200: change column 10 (addr 40) to 256.
	// This write happens AFTER column 10 starts rendering (pixel 160).
	vdp.vsram[40] = 0x01
	vdp.vsram[41] = 0x00
	vdp.vsramChanges = append(vdp.vsramChanges, vsramChange{
		pixelX: 200,
		addr:   40,
		hi:     0x01,
		lo:     0x00,
	})

	// Column 5: write at pixel 48 <= column start 80 -> change applied -> 50
	va5 := vdp.vScrollValue(80, false)
	if va5 != 50 {
		t.Errorf("column 5 (write before column): expected 50, got %d", va5)
	}

	// Column 10: write at pixel 200 > column start 160 -> not applied -> snapshot 20
	va10 := vdp.vScrollValue(160, false)
	if va10 != 20 {
		t.Errorf("column 10 (write after column): expected 20, got %d", va10)
	}

	// Column 3 (x=48-63, addr 12): unaffected by writes to addrs 20 and 40
	vdp.vsramSnapshot[12] = 0x00
	vdp.vsramSnapshot[13] = 0x07
	va3 := vdp.vScrollValue(48, false)
	if va3 != 7 {
		t.Errorf("column 3 (different addr): expected 7, got %d", va3)
	}
}
