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

// --- Compositing tests ---

func TestVDP_Composite_BackdropOnly(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40
	// Backdrop = palette 0, color 1
	vdp.regs[7] = 0x01
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E // pure red

	// Clear line buffers (all transparent)
	for i := 0; i < 320; i++ {
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufA[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	if pix[0] != 255 || pix[1] != 0 || pix[2] != 0 {
		t.Errorf("expected red backdrop, got (%d,%d,%d)", pix[0], pix[1], pix[2])
	}
}

func TestVDP_Composite_HighPriSpriteOverHighPriPlaneA(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81

	// Set CRAM: color 1 = red, color 17 = green (palette 1, index 1)
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E // color 1 = red
	vdp.cram[34] = 0x00
	vdp.cram[35] = 0xE0 // color 17 = green

	// Pixel 0: high-pri sprite (palette 0, color 1) vs high-pri plane A (palette 1, color 1)
	vdp.lineBufSpr[0] = layerPixel{colorIndex: 1, palette: 0, priority: true}
	vdp.lineBufA[0] = layerPixel{colorIndex: 1, palette: 1, priority: true}
	vdp.lineBufB[0] = layerPixel{}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// Sprite should win (priority level 1 > level 2)
	if pix[0] != 255 || pix[1] != 0 || pix[2] != 0 {
		t.Errorf("expected red (sprite wins), got (%d,%d,%d)", pix[0], pix[1], pix[2])
	}
}

func TestVDP_Composite_HighPriPlaneBOverLowPriSprite(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81

	// Color 1 = red (for sprite), color 17 = green (for plane B)
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E
	vdp.cram[34] = 0x00
	vdp.cram[35] = 0xE0

	vdp.lineBufSpr[0] = layerPixel{colorIndex: 1, palette: 0, priority: false} // low-pri sprite
	vdp.lineBufA[0] = layerPixel{}
	vdp.lineBufB[0] = layerPixel{colorIndex: 1, palette: 1, priority: true} // high-pri plane B

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// High-pri plane B (level 3) beats low-pri sprite (level 4)
	if pix[0] != 0 || pix[1] != 255 || pix[2] != 0 {
		t.Errorf("expected green (plane B wins), got (%d,%d,%d)", pix[0], pix[1], pix[2])
	}
}

func TestVDP_Composite_LowPriSpriteFallthrough(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81

	// Color 1 = red
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E

	// Only a low-pri sprite, no planes
	vdp.lineBufSpr[0] = layerPixel{colorIndex: 1, palette: 0, priority: false}
	vdp.lineBufA[0] = layerPixel{}
	vdp.lineBufB[0] = layerPixel{}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	if pix[0] != 255 || pix[1] != 0 || pix[2] != 0 {
		t.Errorf("expected red (low-pri sprite), got (%d,%d,%d)", pix[0], pix[1], pix[2])
	}
}

func TestVDP_Composite_H32Mode(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x00 // H32 mode (256px)

	// Backdrop = palette 0, color 1 = red
	vdp.regs[7] = 0x01
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E

	// Color 17 = green (palette 1, color 1)
	vdp.cram[34] = 0x00
	vdp.cram[35] = 0xE0

	// Put a sprite pixel at x=255 (last active pixel in H32)
	for i := 0; i < 320; i++ {
		vdp.lineBufSpr[i] = layerPixel{}
		vdp.lineBufA[i] = layerPixel{}
		vdp.lineBufB[i] = layerPixel{}
	}
	vdp.lineBufSpr[255] = layerPixel{colorIndex: 1, palette: 1, priority: false}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// Pixel 255 should be green (sprite)
	p := 255 * 4
	if pix[p] != 0 || pix[p+1] != 255 || pix[p+2] != 0 {
		t.Errorf("pixel 255: expected green, got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
	}
	// Pixel 256 should be backdrop (red) since it's beyond H32 active area
	p = 256 * 4
	if pix[p] != 255 || pix[p+1] != 0 || pix[p+2] != 0 {
		t.Errorf("pixel 256: expected red (backdrop), got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
	}
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

func TestVDP_Composite_LeftColumnBlank(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81
	vdp.regs[0] = 0x20 // left column blank

	// Backdrop = palette 0, color 2 -> green
	vdp.regs[7] = 0x02
	vdp.cram[4] = 0x00
	vdp.cram[5] = 0xE0 // color 2 = green

	// Color 1 = red
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E

	// Plane A with content in first 8 pixels
	for i := 0; i < 320; i++ {
		vdp.lineBufA[i] = layerPixel{colorIndex: 1, palette: 0, priority: false}
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// First 8 pixels should be backdrop (green), not plane A (red)
	for x := 0; x < 8; x++ {
		p := x * 4
		if pix[p] != 0 || pix[p+1] != 255 || pix[p+2] != 0 {
			t.Errorf("pixel %d: expected green (backdrop), got (%d,%d,%d)", x, pix[p], pix[p+1], pix[p+2])
		}
	}
	// Pixel 8 should be plane A (red)
	p := 8 * 4
	if pix[p] != 255 || pix[p+1] != 0 || pix[p+2] != 0 {
		t.Errorf("pixel 8: expected red (plane A), got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
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

func TestVDP_ShadowHighlight_DefaultShadow(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x89 // H40 + SH mode
	// Backdrop = palette 0, color 1 (white)
	vdp.regs[7] = 0x01
	vdp.cram[2] = 0x0E
	vdp.cram[3] = 0xEE

	// All layers transparent -> backdrop at shadow brightness
	for i := 0; i < 320; i++ {
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufA[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// Shadow of white = 127
	if pix[0] != 127 || pix[1] != 127 || pix[2] != 127 {
		t.Errorf("expected shadow white (127,127,127), got (%d,%d,%d)", pix[0], pix[1], pix[2])
	}
}

func TestVDP_ShadowHighlight_HighPriPlaneNormal(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x89 // H40 + SH mode
	vdp.regs[7] = 0x00  // backdrop = color 0

	// Color 1 = pure red (palette 0)
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E

	for i := 0; i < 320; i++ {
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufA[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	// High-priority plane A pixel
	vdp.lineBufA[0] = layerPixel{colorIndex: 1, palette: 0, priority: true}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// High-pri plane = normal brightness = full red (255,0,0)
	if pix[0] != 255 || pix[1] != 0 || pix[2] != 0 {
		t.Errorf("expected normal red (255,0,0), got (%d,%d,%d)", pix[0], pix[1], pix[2])
	}
}

func TestVDP_ShadowHighlight_LowPriPlaneShadow(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x89
	vdp.regs[7] = 0x00

	// Color 1 = pure red
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E

	for i := 0; i < 320; i++ {
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufA[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	// Low-priority plane A pixel
	vdp.lineBufA[0] = layerPixel{colorIndex: 1, palette: 0, priority: false}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// Low-pri plane = shadow brightness = half red (127,0,0)
	if pix[0] != 127 || pix[1] != 0 || pix[2] != 0 {
		t.Errorf("expected shadow red (127,0,0), got (%d,%d,%d)", pix[0], pix[1], pix[2])
	}
}

func TestVDP_ShadowHighlight_HighlightOperator(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x89
	vdp.regs[7] = 0x00

	// Color 1 (palette 0) = pure green for underlying plane
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0xE0

	for i := 0; i < 320; i++ {
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufA[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	// Low-priority plane A underneath
	vdp.lineBufA[0] = layerPixel{colorIndex: 1, palette: 0, priority: false}
	// Sprite: palette 3, color 14 = highlight operator (low priority)
	vdp.lineBufSpr[0] = layerPixel{colorIndex: 14, palette: 3, priority: false}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// Highlight of green: 255 + 128 = 383 -> clamped to 255 -> (0, 255, 0)
	// Wait: green = 255, highlight = 255 + 128 = 383 -> 255
	r, g, b := pix[0], pix[1], pix[2]
	hr, hg, hb := vdp.cramColorHighlight(1)
	if r != hr || g != hg || b != hb {
		t.Errorf("expected highlight green (%d,%d,%d), got (%d,%d,%d)", hr, hg, hb, r, g, b)
	}
}

func TestVDP_ShadowHighlight_ShadowOperator(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x89
	vdp.regs[7] = 0x00

	// Color 1 = pure blue for underlying plane
	vdp.cram[2] = 0x0E
	vdp.cram[3] = 0x00

	for i := 0; i < 320; i++ {
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufA[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	// Low-priority plane A underneath
	vdp.lineBufA[0] = layerPixel{colorIndex: 1, palette: 0, priority: false}
	// Sprite: palette 3, color 15 = shadow operator (low priority)
	vdp.lineBufSpr[0] = layerPixel{colorIndex: 15, palette: 3, priority: false}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// Shadow of blue = half brightness
	sr, sg, sb := vdp.cramColorShadow(1)
	if pix[0] != sr || pix[1] != sg || pix[2] != sb {
		t.Errorf("expected shadow blue (%d,%d,%d), got (%d,%d,%d)", sr, sg, sb, pix[0], pix[1], pix[2])
	}
}

func TestVDP_ShadowHighlight_Palette3NormalSprite(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x89
	vdp.regs[7] = 0x00

	// Palette 3, color 5 = CRAM index 53
	vdp.cram[106] = 0x00
	vdp.cram[107] = 0x0E // pure red

	for i := 0; i < 320; i++ {
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufA[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	// Low-priority sprite with palette 3, color 5 (not operator)
	vdp.lineBufSpr[0] = layerPixel{colorIndex: 5, palette: 3, priority: false}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// Palette 3 non-operator: normal brightness
	if pix[0] != 255 || pix[1] != 0 || pix[2] != 0 {
		t.Errorf("expected normal red (255,0,0), got (%d,%d,%d)", pix[0], pix[1], pix[2])
	}
}

func TestVDP_ShadowHighlight_HighPriSpriteNormal(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x89
	vdp.regs[7] = 0x00

	// Color 1 = pure red
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E

	for i := 0; i < 320; i++ {
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufA[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	// High-priority sprite: always normal brightness
	vdp.lineBufSpr[0] = layerPixel{colorIndex: 1, palette: 0, priority: true}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	if pix[0] != 255 || pix[1] != 0 || pix[2] != 0 {
		t.Errorf("expected normal red (255,0,0), got (%d,%d,%d)", pix[0], pix[1], pix[2])
	}
}

func TestVDP_ShadowHighlight_DisabledNoEffect(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40 but NO SH mode (bit 3 = 0)
	vdp.regs[7] = 0x00

	// Color 1 = pure red
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E

	for i := 0; i < 320; i++ {
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufA[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	// Low-priority plane A pixel
	vdp.lineBufA[0] = layerPixel{colorIndex: 1, palette: 0, priority: false}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// Without SH mode, low-pri plane is normal brightness
	if pix[0] != 255 || pix[1] != 0 || pix[2] != 0 {
		t.Errorf("expected normal red (255,0,0) with SH disabled, got (%d,%d,%d)", pix[0], pix[1], pix[2])
	}
}

// --- Mid-scanline CRAM change tests ---

func TestVDP_MidScanlineCRAM_NoChamges(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40
	// Backdrop = palette 0, color 1 = red
	vdp.regs[7] = 0x01
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E

	// No CRAM changes (fast path)
	vdp.cramChanges = nil

	for i := 0; i < 320; i++ {
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufA[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	if pix[0] != 255 || pix[1] != 0 || pix[2] != 0 {
		t.Errorf("expected red backdrop, got (%d,%d,%d)", pix[0], pix[1], pix[2])
	}
}

func TestVDP_MidScanlineCRAM_SingleChange(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40

	// All pixels from plane A, palette 0, color 1
	for i := 0; i < 320; i++ {
		vdp.lineBufA[i] = layerPixel{colorIndex: 1, palette: 0, priority: false}
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	// Initial CRAM: color 1 = red (0x000E)
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E

	// Snapshot CRAM (as if BeginScanline was called)
	vdp.cramSnapshot = vdp.cram

	// At pixel 160, change color 1 to green (0x00E0)
	vdp.cramChanges = []cramChange{
		{pixelX: 160, addr: 2, hi: 0x00, lo: 0xE0},
	}
	// Update live CRAM to final state (green)
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0xE0

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// Pixel 0 should be red (pre-change CRAM)
	if pix[0] != 255 || pix[1] != 0 || pix[2] != 0 {
		t.Errorf("pixel 0: expected red (255,0,0), got (%d,%d,%d)", pix[0], pix[1], pix[2])
	}
	// Pixel 160 should be green (post-change CRAM)
	p := 160 * 4
	if pix[p] != 0 || pix[p+1] != 255 || pix[p+2] != 0 {
		t.Errorf("pixel 160: expected green (0,255,0), got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
	}
	// Pixel 319 should still be green
	p = 319 * 4
	if pix[p] != 0 || pix[p+1] != 255 || pix[p+2] != 0 {
		t.Errorf("pixel 319: expected green (0,255,0), got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
	}
}

func TestVDP_MidScanlineCRAM_MultipleChanges(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40

	// All pixels from plane A, palette 0, color 1
	for i := 0; i < 320; i++ {
		vdp.lineBufA[i] = layerPixel{colorIndex: 1, palette: 0, priority: false}
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	// Initial CRAM: color 1 = red
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E
	vdp.cramSnapshot = vdp.cram

	// Change at pixel 100: red -> green
	// Change at pixel 200: green -> blue
	vdp.cramChanges = []cramChange{
		{pixelX: 100, addr: 2, hi: 0x00, lo: 0xE0},
		{pixelX: 200, addr: 2, hi: 0x0E, lo: 0x00},
	}
	// Final state is blue
	vdp.cram[2] = 0x0E
	vdp.cram[3] = 0x00

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// Pixel 50: red
	p := 50 * 4
	if pix[p] != 255 || pix[p+1] != 0 || pix[p+2] != 0 {
		t.Errorf("pixel 50: expected red, got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
	}
	// Pixel 150: green
	p = 150 * 4
	if pix[p] != 0 || pix[p+1] != 255 || pix[p+2] != 0 {
		t.Errorf("pixel 150: expected green, got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
	}
	// Pixel 250: blue
	p = 250 * 4
	if pix[p] != 0 || pix[p+1] != 0 || pix[p+2] != 255 {
		t.Errorf("pixel 250: expected blue, got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
	}
}

func TestVDP_MidScanlineCRAM_RestoresFinalState(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81

	for i := 0; i < 320; i++ {
		vdp.lineBufA[i] = layerPixel{colorIndex: 1, palette: 0, priority: false}
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	// Initial: red
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E
	vdp.cramSnapshot = vdp.cram

	// Change to green at pixel 160
	vdp.cramChanges = []cramChange{
		{pixelX: 160, addr: 2, hi: 0x00, lo: 0xE0},
	}
	// Final CRAM state = green
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0xE0

	vdp.compositeScanline(0)

	// After compositing, CRAM should be restored to the final (green) state
	if vdp.cram[2] != 0x00 || vdp.cram[3] != 0xE0 {
		t.Errorf("CRAM should be restored to final state, got [2]=0x%02X [3]=0x%02X", vdp.cram[2], vdp.cram[3])
	}
}

func TestVDP_MidScanlineCRAM_BackdropChange(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81
	// Backdrop = palette 0, color 1
	vdp.regs[7] = 0x01

	// All layers transparent -> backdrop used
	for i := 0; i < 320; i++ {
		vdp.lineBufA[i] = layerPixel{}
		vdp.lineBufB[i] = layerPixel{}
		vdp.lineBufSpr[i] = layerPixel{}
	}

	// Initial: backdrop is red
	vdp.cram[2] = 0x00
	vdp.cram[3] = 0x0E
	vdp.cramSnapshot = vdp.cram

	// At pixel 160, change backdrop color to blue
	vdp.cramChanges = []cramChange{
		{pixelX: 160, addr: 2, hi: 0x0E, lo: 0x00},
	}
	vdp.cram[2] = 0x0E
	vdp.cram[3] = 0x00

	vdp.compositeScanline(0)

	pix := vdp.framebuffer.Pix
	// Pixel 80: red backdrop
	p := 80 * 4
	if pix[p] != 255 || pix[p+1] != 0 || pix[p+2] != 0 {
		t.Errorf("pixel 80: expected red backdrop, got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
	}
	// Pixel 200: blue backdrop
	p = 200 * 4
	if pix[p] != 0 || pix[p+1] != 0 || pix[p+2] != 255 {
		t.Errorf("pixel 200: expected blue backdrop, got (%d,%d,%d)", pix[p], pix[p+1], pix[p+2])
	}
}
