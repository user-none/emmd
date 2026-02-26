package emu

import "testing"

// --- CRAM color conversion tests ---

func TestVDP_CRAMColor_White(t *testing.T) {
	vdp := makeTestVDP()
	// White: BBB=7 (0x0E), GGG=7, RRR=7
	// High byte: 0000_1110 = 0x0E (blue=7 in bits 3:1)
	// Low byte: 1110_1110 = 0xEE (green=7 in bits 7:5, red=7 in bits 3:1)
	vdp.cram[0] = 0x0E
	vdp.cram[1] = 0xEE
	r, g, b := vdp.cramColor(0)
	// Each 3-bit component 7 -> (7<<5)|(7<<2)|(7>>1) = 224+28+3 = 255
	if r != 255 || g != 255 || b != 255 {
		t.Errorf("expected white (255,255,255), got (%d,%d,%d)", r, g, b)
	}
}

func TestVDP_CRAMColor_Black(t *testing.T) {
	vdp := makeTestVDP()
	vdp.cram[0] = 0x00
	vdp.cram[1] = 0x00
	r, g, b := vdp.cramColor(0)
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("expected black (0,0,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestVDP_CRAMColor_PureRed(t *testing.T) {
	vdp := makeTestVDP()
	// Red only: RRR=7 -> low byte bits 3:1 = 0x0E
	vdp.cram[0] = 0x00
	vdp.cram[1] = 0x0E
	r, g, b := vdp.cramColor(0)
	if r != 255 || g != 0 || b != 0 {
		t.Errorf("expected pure red (255,0,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestVDP_CRAMColor_PureGreen(t *testing.T) {
	vdp := makeTestVDP()
	// Green only: GGG=7 -> low byte bits 7:5 = 0xE0
	vdp.cram[0] = 0x00
	vdp.cram[1] = 0xE0
	r, g, b := vdp.cramColor(0)
	if r != 0 || g != 255 || b != 0 {
		t.Errorf("expected pure green (0,255,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestVDP_CRAMColor_PureBlue(t *testing.T) {
	vdp := makeTestVDP()
	// Blue only: BBB=7 -> high byte bits 3:1 = 0x0E
	vdp.cram[0] = 0x0E
	vdp.cram[1] = 0x00
	r, g, b := vdp.cramColor(0)
	if r != 0 || g != 0 || b != 255 {
		t.Errorf("expected pure blue (0,0,255), got (%d,%d,%d)", r, g, b)
	}
}

func TestVDP_CRAMColor_SecondEntry(t *testing.T) {
	vdp := makeTestVDP()
	// Color index 1 -> cram[2], cram[3]
	vdp.cram[2] = 0x04 // Blue=2
	vdp.cram[3] = 0x44 // Green=2, Red=2
	r, g, b := vdp.cramColor(1)
	// 2 -> (2<<5)|(2<<2)|(2>>1) = 64+8+1 = 73
	if r != 73 || g != 73 || b != 73 {
		t.Errorf("expected (73,73,73), got (%d,%d,%d)", r, g, b)
	}
}

func TestVDP_CRAMColor_IndexMasked(t *testing.T) {
	vdp := makeTestVDP()
	// Index 64 should wrap to 0 (mask &0x3F)
	vdp.cram[0] = 0x0E
	vdp.cram[1] = 0xEE
	r, g, b := vdp.cramColor(64)
	if r != 255 || g != 255 || b != 255 {
		t.Errorf("index 64 should wrap to 0, got (%d,%d,%d)", r, g, b)
	}
}

// --- Backdrop tests ---

func TestVDP_FillBackdrop(t *testing.T) {
	vdp := makeTestVDP()
	// Set backdrop to palette 0, color 1
	vdp.regs[7] = 0x01
	// Set CRAM color 1 to pure red
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E

	// H40 mode for full 320px width
	vdp.regs[12] = 0x81

	vdp.fillBackdrop(0)

	pix := vdp.framebuffer.Pix
	// Check first pixel
	if pix[0] != 255 || pix[1] != 0 || pix[2] != 0 || pix[3] != 0xFF {
		t.Errorf("expected red pixel at (0,0), got (%d,%d,%d,%d)", pix[0], pix[1], pix[2], pix[3])
	}
	// Check last active pixel (319)
	p := 319 * 4
	if pix[p] != 255 || pix[p+1] != 0 || pix[p+2] != 0 {
		t.Errorf("expected red at pixel 319, got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
	}
}

func TestVDP_RenderScanline_DisplayDisabled(t *testing.T) {
	vdp := makeTestVDP()
	// Display disabled (reg 1 bit 6 = 0)
	vdp.regs[1] = 0x00
	// Set backdrop to palette 1, color 2 -> CRAM index 18
	vdp.regs[7] = 0x12
	// Set CRAM color 18: pure green
	vdp.cram[36] = 0x00
	vdp.cram[37] = 0xE0

	vdp.regs[12] = 0x81 // H40

	vdp.RenderScanline(0)

	pix := vdp.framebuffer.Pix
	if pix[0] != 0 || pix[1] != 255 || pix[2] != 0 || pix[3] != 0xFF {
		t.Errorf("expected green backdrop, got (%d,%d,%d,%d)", pix[0], pix[1], pix[2], pix[3])
	}
}

func TestVDP_RenderScanline_OutOfBounds(t *testing.T) {
	vdp := makeTestVDP()
	// Should not panic for out-of-bounds lines
	vdp.RenderScanline(-1)
	vdp.RenderScanline(240)
	vdp.RenderScanline(300)
}

// TestVDP_RenderScanline_FullPipeline simulates realistic game VDP initialization
// and verifies the full rendering pipeline produces non-black visible output.
func TestVDP_RenderScanline_FullPipeline(t *testing.T) {
	vdp := makeTestVDP()

	// --- Simulate typical game VDP initialization ---

	// Enable display + DMA + V-int (reg 1 = 0x64)
	vdp.regs[1] = 0x64
	// H40 mode (reg 12 = 0x81)
	vdp.regs[12] = 0x81
	// Nametable size: 64x32 (reg 16 = 0x01)
	vdp.regs[16] = 0x01
	// Plane A nametable at 0xC000 (reg 2 = 0x30)
	vdp.regs[2] = 0x30
	// Plane B nametable at 0xE000 (reg 4 = 0x07)
	vdp.regs[4] = 0x07
	// H-scroll table at 0xFC00 (reg 13 = 0x3F)
	vdp.regs[13] = 0x3F
	// Sprite table at 0xD800 (reg 5 = 0x6C)
	vdp.regs[5] = 0x6C
	// Backdrop = palette 0, color 2 (reg 7 = 0x02)
	vdp.regs[7] = 0x02

	// --- Load palette to CRAM ---
	// Color 0 = black (transparent)
	vdp.cram[0] = 0x00
	vdp.cram[1] = 0x00
	// Color 1 = white (0x0EEE)
	vdp.cram[2] = 0x0E
	vdp.cram[3] = 0xEE
	// Color 2 = blue (0x0E00) - this is the backdrop
	vdp.cram[4] = 0x0E
	vdp.cram[5] = 0x00
	// Color 3 = red (0x000E)
	vdp.cram[6] = 0x00
	vdp.cram[7] = 0x0E

	// --- Load tile data to VRAM ---
	// Tile 1 at VRAM address 32: all pixels = color 1 (white) for all 8 rows
	for row := 0; row < 8; row++ {
		base := 32 + row*4
		vdp.vram[base] = 0x11
		vdp.vram[base+1] = 0x11
		vdp.vram[base+2] = 0x11
		vdp.vram[base+3] = 0x11
	}

	// Tile 2 at VRAM address 64: all pixels = color 3 (red) for all 8 rows
	for row := 0; row < 8; row++ {
		base := 64 + row*4
		vdp.vram[base] = 0x33
		vdp.vram[base+1] = 0x33
		vdp.vram[base+2] = 0x33
		vdp.vram[base+3] = 0x33
	}

	// --- Set up Plane A nametable at 0xC000 ---
	// Cell (0,0): tile 1, palette 0, no flip, no priority
	vdp.vram[0xC000] = 0x00
	vdp.vram[0xC001] = 0x01
	// Cell (1,0): tile 2, palette 0, no flip, no priority
	vdp.vram[0xC002] = 0x00
	vdp.vram[0xC003] = 0x02

	// --- H-scroll = 0 (no scroll) ---
	// H-scroll table at 0xFC00 is already all zeros

	// --- V-scroll = 0 ---
	// VSRAM is already all zeros

	// --- Render scanline 0 ---
	vdp.RenderScanline(0)

	pix := vdp.framebuffer.Pix

	// Pixels 0-7 should be white (tile 1, color 1)
	for x := 0; x < 8; x++ {
		p := x * 4
		if pix[p] != 255 || pix[p+1] != 255 || pix[p+2] != 255 {
			t.Errorf("pixel %d: expected white (255,255,255), got (%d,%d,%d)", x, pix[p], pix[p+1], pix[p+2])
		}
		if pix[p+3] != 0xFF {
			t.Errorf("pixel %d: expected alpha 255, got %d", x, pix[p+3])
		}
	}

	// Pixels 8-15 should be red (tile 2, color 3)
	for x := 8; x < 16; x++ {
		p := x * 4
		if pix[p] != 255 || pix[p+1] != 0 || pix[p+2] != 0 {
			t.Errorf("pixel %d: expected red (255,0,0), got (%d,%d,%d)", x, pix[p], pix[p+1], pix[p+2])
		}
	}

	// Pixels 16+ where nametable is all zeros -> tile 0 (all transparent) -> backdrop (blue)
	p := 16 * 4
	if pix[p] != 0 || pix[p+1] != 0 || pix[p+2] != 255 {
		t.Errorf("pixel 16: expected blue backdrop (0,0,255), got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
	}
}

// --- Shadow/Highlight tests ---

func TestVDP_ShadowHighlight_ModeDetection(t *testing.T) {
	vdp := makeTestVDP()
	if vdp.shadowHighlightMode() {
		t.Error("SH mode should be off by default")
	}
	vdp.regs[12] = 0x08 // set bit 3
	if !vdp.shadowHighlightMode() {
		t.Error("SH mode should be on when reg 12 bit 3 is set")
	}
}

func TestVDP_CRAMColor_Shadow(t *testing.T) {
	vdp := makeTestVDP()
	// White: all 255
	vdp.cram[0] = 0x0E
	vdp.cram[1] = 0xEE
	r, g, b := vdp.cramColorShadow(0)
	// 255 >> 1 = 127
	if r != 127 || g != 127 || b != 127 {
		t.Errorf("expected shadow white (127,127,127), got (%d,%d,%d)", r, g, b)
	}
}

func TestVDP_CRAMColor_Highlight(t *testing.T) {
	vdp := makeTestVDP()
	// Black: all 0 -> highlight = 0 + 128 = 128
	vdp.cram[0] = 0x00
	vdp.cram[1] = 0x00
	r, g, b := vdp.cramColorHighlight(0)
	if r != 128 || g != 128 || b != 128 {
		t.Errorf("expected highlight black (128,128,128), got (%d,%d,%d)", r, g, b)
	}

	// White: all 255 -> highlight = 255 + 128 = 383 -> clamped to 255
	vdp.cram[2] = 0x0E
	vdp.cram[3] = 0xEE
	r, g, b = vdp.cramColorHighlight(1)
	if r != 255 || g != 255 || b != 255 {
		t.Errorf("expected highlight white clamped (255,255,255), got (%d,%d,%d)", r, g, b)
	}
}

// --- Merged path integration tests ---

// TestVDP_RenderScanline_WindowRendering verifies window rendering through the
// full RenderScanline merged path. Sets up a window region covering the left
// half of the screen and verifies window tile content appears correctly while
// plane A content appears outside the window.
func TestVDP_RenderScanline_WindowRendering(t *testing.T) {
	vdp := makeTestVDP()

	vdp.regs[1] = 0x64  // Enable display + DMA + V-int
	vdp.regs[12] = 0x81 // H40
	vdp.regs[16] = 0x01 // 64x32 nametable

	// Plane A nametable at 0xC000
	vdp.regs[2] = 0x30
	// Plane B nametable at 0xE000
	vdp.regs[4] = 0x07
	// H-scroll table at 0xFC00
	vdp.regs[13] = 0x3F
	// Sprite table at 0xD800
	vdp.regs[5] = 0x6C
	// Backdrop = palette 0, color 2 (blue)
	vdp.regs[7] = 0x02

	// Window nametable at 0x6000 (H40: reg 3 = 0x18)
	vdp.regs[3] = 0x18

	// Window covers left 10 cells (160 pixels): hRight=0, boundary=10
	vdp.regs[17] = 0x0A // left side, boundary at 10*16 = 160px
	vdp.regs[18] = 0x00 // no V boundary

	// CRAM setup
	vdp.cram[0] = 0x00
	vdp.cram[1] = 0x00 // color 0 = black (transparent)
	vdp.cram[2] = 0x0E
	vdp.cram[3] = 0xEE // color 1 = white
	vdp.cram[4] = 0x0E
	vdp.cram[5] = 0x00 // color 2 = blue (backdrop)
	vdp.cram[6] = 0x00
	vdp.cram[7] = 0x0E // color 3 = red

	// Tile 1 at VRAM 32: all color 1 (white) - for window
	for row := 0; row < 8; row++ {
		base := 32 + row*4
		vdp.vram[base] = 0x11
		vdp.vram[base+1] = 0x11
		vdp.vram[base+2] = 0x11
		vdp.vram[base+3] = 0x11
	}

	// Tile 2 at VRAM 64: all color 3 (red) - for plane A
	for row := 0; row < 8; row++ {
		base := 64 + row*4
		vdp.vram[base] = 0x33
		vdp.vram[base+1] = 0x33
		vdp.vram[base+2] = 0x33
		vdp.vram[base+3] = 0x33
	}

	// Window nametable at 0x6000: cell (0,0) = tile 1 (white)
	// Window nametable width in H40 is 64 cells
	vdp.vram[0x6000] = 0x00
	vdp.vram[0x6001] = 0x01

	// Plane A nametable at 0xC000: cell (20,0) = tile 2 (red)
	// Cell 20 is at offset 20*2 = 40 bytes from base
	vdp.vram[0xC000+40] = 0x00
	vdp.vram[0xC000+41] = 0x02

	vdp.RenderScanline(0)

	pix := vdp.framebuffer.Pix

	// Pixels 0-7 should be white (window tile 1)
	for x := 0; x < 8; x++ {
		p := x * 4
		if pix[p] != 255 || pix[p+1] != 255 || pix[p+2] != 255 {
			t.Errorf("pixel %d: expected white (window), got (%d,%d,%d)", x, pix[p], pix[p+1], pix[p+2])
		}
	}

	// Pixels 160-167 should be red (plane A tile 2, outside window)
	for x := 160; x < 168; x++ {
		p := x * 4
		if pix[p] != 255 || pix[p+1] != 0 || pix[p+2] != 0 {
			t.Errorf("pixel %d: expected red (plane A), got (%d,%d,%d)", x, pix[p], pix[p+1], pix[p+2])
		}
	}

	// Pixel 170 (outside window, no plane A tile) should be backdrop (blue)
	p := 170 * 4
	if pix[p] != 0 || pix[p+1] != 0 || pix[p+2] != 255 {
		t.Errorf("pixel 170: expected blue (backdrop), got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
	}
}

// TestVDP_RenderScanline_ShadowHighlight verifies shadow/highlight rendering
// through the full RenderScanline merged path.
func TestVDP_RenderScanline_ShadowHighlight(t *testing.T) {
	vdp := makeTestVDP()

	vdp.regs[1] = 0x64  // Enable display
	vdp.regs[12] = 0x89 // H40 + shadow/highlight mode
	vdp.regs[16] = 0x01 // 64x32 nametable

	// Plane A nametable at 0xC000
	vdp.regs[2] = 0x30
	// Plane B nametable at 0xE000
	vdp.regs[4] = 0x07
	// H-scroll table at 0xFC00
	vdp.regs[13] = 0x3F
	// Sprite table at 0xD800
	vdp.regs[5] = 0x6C
	// Backdrop = palette 0, color 2
	vdp.regs[7] = 0x02

	// CRAM setup
	vdp.cram[0] = 0x00
	vdp.cram[1] = 0x00 // color 0 = black (transparent)
	vdp.cram[2] = 0x0E
	vdp.cram[3] = 0xEE // color 1 = white
	vdp.cram[4] = 0x0E
	vdp.cram[5] = 0x00 // color 2 = blue (backdrop)

	// Tile 1 at VRAM 32: all color 1 (white)
	for row := 0; row < 8; row++ {
		base := 32 + row*4
		vdp.vram[base] = 0x11
		vdp.vram[base+1] = 0x11
		vdp.vram[base+2] = 0x11
		vdp.vram[base+3] = 0x11
	}

	// Plane A: cell (0,0) = tile 1, high priority
	// Entry = 1_00_0_0_00000000001 = 0x8001
	vdp.vram[0xC000] = 0x80
	vdp.vram[0xC001] = 0x01

	// Plane A: cell (1,0) = tile 1, low priority
	// Entry = 0_00_0_0_00000000001 = 0x0001
	vdp.vram[0xC002] = 0x00
	vdp.vram[0xC003] = 0x01

	vdp.RenderScanline(0)

	pix := vdp.framebuffer.Pix

	// Pixels 0-7: high-priority plane A -> normal brightness white (255,255,255)
	for x := 0; x < 8; x++ {
		p := x * 4
		if pix[p] != 255 || pix[p+1] != 255 || pix[p+2] != 255 {
			t.Errorf("pixel %d: expected normal white (255,255,255), got (%d,%d,%d)", x, pix[p], pix[p+1], pix[p+2])
		}
	}

	// Pixels 8-15: low-priority plane A -> shadow brightness white (127,127,127)
	for x := 8; x < 16; x++ {
		p := x * 4
		if pix[p] != 127 || pix[p+1] != 127 || pix[p+2] != 127 {
			t.Errorf("pixel %d: expected shadow white (127,127,127), got (%d,%d,%d)", x, pix[p], pix[p+1], pix[p+2])
		}
	}

	// Pixel 16+: backdrop at shadow brightness
	// Blue = (0,0,255), shadow = (0,0,127)
	p := 16 * 4
	if pix[p] != 0 || pix[p+1] != 0 || pix[p+2] != 127 {
		t.Errorf("pixel 16: expected shadow blue (0,0,127), got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
	}
}
