package emu

import "testing"

// setupSpriteSAT writes a sprite entry into the SAT.
// yRaw and xRaw are raw values (screen + 128).
func setupSpriteSAT(vdp *VDP, satBase uint16, index int, yRaw, hSize, vSize, link int, priority, vFlip, hFlip bool, palette, baseTile, xRaw int) {
	addr := satBase + uint16(index)*8

	// Bytes 0-1: Y position
	vdp.vram[addr] = uint8(yRaw >> 8)
	vdp.vram[(addr+1)&0xFFFF] = uint8(yRaw)

	// Byte 2: size (hSize-1 << 2 | vSize-1)
	vdp.vram[(addr+2)&0xFFFF] = uint8((hSize-1)<<2 | (vSize - 1))

	// Byte 3: link
	vdp.vram[(addr+3)&0xFFFF] = uint8(link)

	// Bytes 4-5: attributes
	// Attribute format: P PAL[1:0] VF HF TILE[10:0]
	var attr uint16
	if priority {
		attr |= 0x8000
	}
	attr |= uint16(palette&0x03) << 13
	if vFlip {
		attr |= 0x1000
	}
	if hFlip {
		attr |= 0x0800
	}
	attr |= uint16(baseTile) & 0x07FF
	vdp.vram[(addr+4)&0xFFFF] = uint8(attr >> 8)
	vdp.vram[(addr+5)&0xFFFF] = uint8(attr)

	// Bytes 6-7: X position
	vdp.vram[(addr+6)&0xFFFF] = uint8(xRaw >> 8)
	vdp.vram[(addr+7)&0xFFFF] = uint8(xRaw)
}

func TestVDP_RenderSprites_SingleSprite(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40
	vdp.regs[5] = 0x40  // SAT base = 0x40<<9 = 0x8000

	satBase := uint16(0x8000)

	// Sprite 0: 1x1 cell at screen position (0, 0) -> yRaw=128, xRaw=128
	// Link=0 (end of list), tile 1, palette 0, no flip, no priority
	setupSpriteSAT(vdp, satBase, 0, 128, 1, 1, 0, false, false, false, 0, 1, 128)

	// Tile 1 at VRAM 32: row 0 all color 3
	vdp.vram[32] = 0x33
	vdp.vram[33] = 0x33
	vdp.vram[34] = 0x33
	vdp.vram[35] = 0x33

	// Clear sprite buffer
	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	for x := 0; x < 8; x++ {
		if vdp.lineBufSpr[x].colorIndex != 3 {
			t.Errorf("pixel %d: expected colorIndex 3, got %d", x, vdp.lineBufSpr[x].colorIndex)
		}
	}
	// Pixel 8 should be transparent (no sprite there)
	if vdp.lineBufSpr[8].colorIndex != 0 {
		t.Errorf("pixel 8: expected transparent, got %d", vdp.lineBufSpr[8].colorIndex)
	}
}

func TestVDP_RenderSprites_LinkOrder(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81
	vdp.regs[5] = 0x40 // SAT at 0x8000
	satBase := uint16(0x8000)

	// Sprite 0: at (0,0), tile 1 (all color 1), link to sprite 1
	setupSpriteSAT(vdp, satBase, 0, 128, 1, 1, 1, false, false, false, 0, 1, 128)
	// Sprite 1: same position, tile 2 (all color 2), link=0 (end)
	setupSpriteSAT(vdp, satBase, 1, 128, 1, 1, 0, false, false, false, 0, 2, 128)

	// Tile 1: all color 1
	vdp.vram[32] = 0x11
	vdp.vram[33] = 0x11
	vdp.vram[34] = 0x11
	vdp.vram[35] = 0x11

	// Tile 2: all color 2
	vdp.vram[64] = 0x22
	vdp.vram[65] = 0x22
	vdp.vram[66] = 0x22
	vdp.vram[67] = 0x22

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	// Sprite 0 should win (first in link order)
	if vdp.lineBufSpr[0].colorIndex != 1 {
		t.Errorf("expected colorIndex 1 (sprite 0 wins), got %d", vdp.lineBufSpr[0].colorIndex)
	}
}

func TestVDP_RenderSprites_MultiCell(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// 2x1 cell sprite (2 cells wide, 1 cell tall) at (0,0)
	// Tile base = 1, so tiles 1 and 2 are used (column-major: tile 1 = col 0, tile 2 = col 1)
	setupSpriteSAT(vdp, satBase, 0, 128, 2, 1, 0, false, false, false, 0, 1, 128)

	// Tile 1: all color 5
	vdp.vram[32] = 0x55
	vdp.vram[33] = 0x55
	vdp.vram[34] = 0x55
	vdp.vram[35] = 0x55

	// Tile 2: all color 7
	vdp.vram[64] = 0x77
	vdp.vram[65] = 0x77
	vdp.vram[66] = 0x77
	vdp.vram[67] = 0x77

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	// First 8 pixels from tile 1
	for x := 0; x < 8; x++ {
		if vdp.lineBufSpr[x].colorIndex != 5 {
			t.Errorf("pixel %d: expected 5, got %d", x, vdp.lineBufSpr[x].colorIndex)
		}
	}
	// Next 8 pixels from tile 2
	for x := 8; x < 16; x++ {
		if vdp.lineBufSpr[x].colorIndex != 7 {
			t.Errorf("pixel %d: expected 7, got %d", x, vdp.lineBufSpr[x].colorIndex)
		}
	}
}

func TestVDP_RenderSprites_PerLineLimit(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40: max 20 sprites per line
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// Create 21 sprites all on line 0, each 1x1 at different X positions
	for i := 0; i < 21; i++ {
		link := i + 1
		if i == 20 {
			link = 0
		}
		xPos := 128 + i*8
		setupSpriteSAT(vdp, satBase, i, 128, 1, 1, link, false, false, false, 0, 1, xPos)
	}

	// Tile 1: all color 3
	vdp.vram[32] = 0x33
	vdp.vram[33] = 0x33
	vdp.vram[34] = 0x33
	vdp.vram[35] = 0x33

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	// First 20 sprites should render (pixels 0-159)
	if vdp.lineBufSpr[0].colorIndex != 3 {
		t.Errorf("sprite 0 should render, got colorIndex %d", vdp.lineBufSpr[0].colorIndex)
	}
	if vdp.lineBufSpr[19*8].colorIndex != 3 {
		t.Errorf("sprite 19 should render, got colorIndex %d", vdp.lineBufSpr[19*8].colorIndex)
	}

	// 21st sprite should NOT render
	if vdp.lineBufSpr[20*8].colorIndex != 0 {
		t.Errorf("sprite 20 should be masked (per-line limit), got colorIndex %d", vdp.lineBufSpr[20*8].colorIndex)
	}
}

func TestVDP_RenderSprites_X0Masking(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// Sprite 0: at screen (16,0), tile 1, links to sprite 1
	setupSpriteSAT(vdp, satBase, 0, 128, 1, 1, 1, false, false, false, 0, 1, 128+16)

	// Sprite 1: xRaw=0 (X=0 masking), links to sprite 2
	setupSpriteSAT(vdp, satBase, 1, 128, 1, 1, 2, false, false, false, 0, 1, 0)

	// Sprite 2: at screen (24,0), should be masked by X=0
	setupSpriteSAT(vdp, satBase, 2, 128, 1, 1, 0, false, false, false, 0, 2, 128+24)

	// Tile 1: all color 3
	vdp.vram[32] = 0x33
	vdp.vram[33] = 0x33
	vdp.vram[34] = 0x33
	vdp.vram[35] = 0x33

	// Tile 2: all color 5
	vdp.vram[64] = 0x55
	vdp.vram[65] = 0x55
	vdp.vram[66] = 0x55
	vdp.vram[67] = 0x55

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	// Sprite 0 should render at x=16
	if vdp.lineBufSpr[16].colorIndex != 3 {
		t.Errorf("sprite 0 should render at x=16, got %d", vdp.lineBufSpr[16].colorIndex)
	}

	// Sprite 2 should NOT render (masked by X=0 sprite)
	if vdp.lineBufSpr[24].colorIndex != 0 {
		t.Errorf("sprite 2 should be masked by X=0, got %d", vdp.lineBufSpr[24].colorIndex)
	}
}

func TestVDP_RenderSprites_VFlip(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// 1x2 sprite (1 cell wide, 2 cells tall = 16 pixels high) with V-flip at (0,0)
	setupSpriteSAT(vdp, satBase, 0, 128, 1, 2, 0, false, true, false, 0, 1, 128)

	// Tile 1 row 0: all color 1
	vdp.vram[32] = 0x11
	vdp.vram[33] = 0x11
	vdp.vram[34] = 0x11
	vdp.vram[35] = 0x11

	// Tile 2 (index 2): row 0: all color 9
	vdp.vram[64] = 0x99
	vdp.vram[65] = 0x99
	vdp.vram[66] = 0x99
	vdp.vram[67] = 0x99

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	// With V-flip on a 1x2 sprite, line 0 should read from the bottom tile's bottom row
	// Sprite row = 0 -> after V-flip = 15
	// Cell row = 15/8 = 1 -> tile index = baseTile + cellCol*vSize + cellRow = 1 + 0*2 + 1 = 2
	// Pixel row = 15%8 = 7
	// Tile 2 row 7 should be at vram[64 + 7*4 = 92..95]
	vdp.vram[92] = 0xAA
	vdp.vram[93] = 0xAA
	vdp.vram[94] = 0xAA
	vdp.vram[95] = 0xAA

	vdp.renderSprites(0)

	if vdp.lineBufSpr[0].colorIndex != 0x0A {
		t.Errorf("V-flip line 0: expected colorIndex 0x0A, got 0x%02X", vdp.lineBufSpr[0].colorIndex)
	}
}

func TestVDP_RenderSprites_HFlip(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// 2x1 sprite with H-flip at (0,0)
	setupSpriteSAT(vdp, satBase, 0, 128, 2, 1, 0, false, false, true, 0, 1, 128)

	// Tile 1 row 0: all color 1
	vdp.vram[32] = 0x11
	vdp.vram[33] = 0x11
	vdp.vram[34] = 0x11
	vdp.vram[35] = 0x11

	// Tile 2 row 0: all color 2
	vdp.vram[64] = 0x22
	vdp.vram[65] = 0x22
	vdp.vram[66] = 0x22
	vdp.vram[67] = 0x22

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	// Without H-flip: col 0 = tile 1, col 1 = tile 2
	// With H-flip: pixels 0-7 should be from tile 2, pixels 8-15 from tile 1
	if vdp.lineBufSpr[0].colorIndex != 2 {
		t.Errorf("H-flip pixel 0: expected 2 (tile 2), got %d", vdp.lineBufSpr[0].colorIndex)
	}
	if vdp.lineBufSpr[8].colorIndex != 1 {
		t.Errorf("H-flip pixel 8: expected 1 (tile 1), got %d", vdp.lineBufSpr[8].colorIndex)
	}
}

func TestVDP_SpriteCollisionFlag(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// Two sprites at the same position, both non-transparent
	setupSpriteSAT(vdp, satBase, 0, 128, 1, 1, 1, false, false, false, 0, 1, 128)
	setupSpriteSAT(vdp, satBase, 1, 128, 1, 1, 0, false, false, false, 0, 2, 128)

	// Tile 1: all color 1
	for row := 0; row < 8; row++ {
		base := 32 + row*4
		vdp.vram[base] = 0x11
		vdp.vram[base+1] = 0x11
		vdp.vram[base+2] = 0x11
		vdp.vram[base+3] = 0x11
	}
	// Tile 2: all color 2
	for row := 0; row < 8; row++ {
		base := 64 + row*4
		vdp.vram[base] = 0x22
		vdp.vram[base+1] = 0x22
		vdp.vram[base+2] = 0x22
		vdp.vram[base+3] = 0x22
	}

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	if !vdp.spriteCollision {
		t.Error("spriteCollision should be set when two sprites overlap")
	}

	// Status read should report bit 5 and then clear it
	status := vdp.ReadControl(0)
	if status&(1<<5) == 0 {
		t.Error("status bit 5 (collision) should be set")
	}
	status = vdp.ReadControl(0)
	if status&(1<<5) != 0 {
		t.Error("status bit 5 should be cleared after read")
	}
}

func TestVDP_SpriteCollision_NoOverlap(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// Two non-overlapping sprites
	setupSpriteSAT(vdp, satBase, 0, 128, 1, 1, 1, false, false, false, 0, 1, 128)
	setupSpriteSAT(vdp, satBase, 1, 128, 1, 1, 0, false, false, false, 0, 2, 128+16)

	// Tile 1: all color 1
	vdp.vram[32] = 0x11
	vdp.vram[33] = 0x11
	vdp.vram[34] = 0x11
	vdp.vram[35] = 0x11
	// Tile 2: all color 2
	vdp.vram[64] = 0x22
	vdp.vram[65] = 0x22
	vdp.vram[66] = 0x22
	vdp.vram[67] = 0x22

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	if vdp.spriteCollision {
		t.Error("spriteCollision should NOT be set for non-overlapping sprites")
	}
}

func TestVDP_SpriteCollision_TransparentNoCollision(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// Two sprites at same position, but second one is transparent (color 0)
	setupSpriteSAT(vdp, satBase, 0, 128, 1, 1, 1, false, false, false, 0, 1, 128)
	setupSpriteSAT(vdp, satBase, 1, 128, 1, 1, 0, false, false, false, 0, 2, 128)

	// Tile 1: all color 1
	vdp.vram[32] = 0x11
	vdp.vram[33] = 0x11
	vdp.vram[34] = 0x11
	vdp.vram[35] = 0x11
	// Tile 2: all transparent (color 0) - already zeros

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	if vdp.spriteCollision {
		t.Error("spriteCollision should NOT be set when overlapping pixel is transparent")
	}
}

func TestVDP_SpriteCollision_PersistsAcrossFrame(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// Two overlapping sprites
	setupSpriteSAT(vdp, satBase, 0, 128, 1, 1, 1, false, false, false, 0, 1, 128)
	setupSpriteSAT(vdp, satBase, 1, 128, 1, 1, 0, false, false, false, 0, 2, 128)

	// Tile 1: all color 1
	for row := 0; row < 8; row++ {
		base := 32 + row*4
		vdp.vram[base] = 0x11
		vdp.vram[base+1] = 0x11
		vdp.vram[base+2] = 0x11
		vdp.vram[base+3] = 0x11
	}
	// Tile 2: all color 2
	for row := 0; row < 8; row++ {
		base := 64 + row*4
		vdp.vram[base] = 0x22
		vdp.vram[base+1] = 0x22
		vdp.vram[base+2] = 0x22
		vdp.vram[base+3] = 0x22
	}

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}
	vdp.renderSprites(0)

	if !vdp.spriteCollision {
		t.Fatal("spriteCollision should be set after overlap")
	}

	// Simulate new frame starting (line 0) - collision must persist
	vdp.StartScanline(0)
	if !vdp.spriteCollision {
		t.Error("spriteCollision should persist across frame start; only status read should clear it")
	}

	// Status read clears it
	vdp.ReadControl(0)
	if vdp.spriteCollision {
		t.Error("spriteCollision should be cleared after status read")
	}
}

func TestVDP_SpriteOverflowFlag(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40: max 20 sprites per line
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// Create 21 sprites all on line 0
	for i := 0; i < 21; i++ {
		link := i + 1
		if i == 20 {
			link = 0
		}
		xPos := 128 + i*8
		setupSpriteSAT(vdp, satBase, i, 128, 1, 1, link, false, false, false, 0, 1, xPos)
	}

	// Tile 1: all color 3
	vdp.vram[32] = 0x33
	vdp.vram[33] = 0x33
	vdp.vram[34] = 0x33
	vdp.vram[35] = 0x33

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	if !vdp.spriteOverflow {
		t.Error("spriteOverflow should be set when more than 20 sprites on line (H40)")
	}

	// Status read should report bit 6 and then clear it
	status := vdp.ReadControl(0)
	if status&(1<<6) == 0 {
		t.Error("status bit 6 (overflow) should be set")
	}
	status = vdp.ReadControl(0)
	if status&(1<<6) != 0 {
		t.Error("status bit 6 should be cleared after read")
	}
}

func TestVDP_SpriteOverflow_AtLimit(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40: max 20 sprites per line
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// Exactly 20 sprites on line 0 (at the limit, not over)
	for i := 0; i < 20; i++ {
		link := i + 1
		if i == 19 {
			link = 0
		}
		xPos := 128 + i*8
		setupSpriteSAT(vdp, satBase, i, 128, 1, 1, link, false, false, false, 0, 1, xPos)
	}

	vdp.vram[32] = 0x33
	vdp.vram[33] = 0x33
	vdp.vram[34] = 0x33
	vdp.vram[35] = 0x33

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	if vdp.spriteOverflow {
		t.Error("spriteOverflow should NOT be set at exactly 20 sprites (H40 limit)")
	}
}

func TestVDP_SpriteOverflow_H32(t *testing.T) {
	vdp := makeTestVDP()
	vdp.regs[12] = 0x00 // H32: max 16 sprites per line
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// Create 17 sprites all on line 0
	for i := 0; i < 17; i++ {
		link := i + 1
		if i == 16 {
			link = 0
		}
		xPos := 128 + i*8
		setupSpriteSAT(vdp, satBase, i, 128, 1, 1, link, false, false, false, 0, 1, xPos)
	}

	vdp.vram[32] = 0x33
	vdp.vram[33] = 0x33
	vdp.vram[34] = 0x33
	vdp.vram[35] = 0x33

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	if !vdp.spriteOverflow {
		t.Error("spriteOverflow should be set when more than 16 sprites on line (H32)")
	}
}

func TestVDP_SpriteX0Mask_MultipleZeroX(t *testing.T) {
	// Two sprites with xRaw=0 followed by a visible sprite.
	// Since no previous sprite had non-zero X, masking should NOT trigger
	// and the third sprite should render.
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// Sprite 0: xRaw=0, on line 0
	setupSpriteSAT(vdp, satBase, 0, 128, 1, 1, 1, false, false, false, 0, 1, 0)
	// Sprite 1: xRaw=0, on line 0
	setupSpriteSAT(vdp, satBase, 1, 128, 1, 1, 2, false, false, false, 0, 1, 0)
	// Sprite 2: xRaw=160 (visible), on line 0
	setupSpriteSAT(vdp, satBase, 2, 128, 1, 1, 0, false, false, false, 0, 2, 160)

	// Tile 2: all color 5
	for row := 0; row < 8; row++ {
		base := 64 + row*4
		vdp.vram[base] = 0x55
		vdp.vram[base+1] = 0x55
		vdp.vram[base+2] = 0x55
		vdp.vram[base+3] = 0x55
	}

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	// Sprite 2 at screen X=32 (160-128) should have rendered
	if vdp.lineBufSpr[32].colorIndex == 0 {
		t.Error("sprite at X=160 should render; multiple xRaw=0 sprites should not trigger masking")
	}
}

func TestVDP_SpriteX0Mask_AfterNonZero(t *testing.T) {
	// A sprite with non-zero X, then a sprite with xRaw=0.
	// Masking should trigger and prevent any further sprites.
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// Sprite 0: xRaw=160 (non-zero), on line 0
	setupSpriteSAT(vdp, satBase, 0, 128, 1, 1, 1, false, false, false, 0, 1, 160)
	// Sprite 1: xRaw=0 (mask trigger), on line 0
	setupSpriteSAT(vdp, satBase, 1, 128, 1, 1, 2, false, false, false, 0, 1, 0)
	// Sprite 2: xRaw=200 (should be masked), on line 0
	setupSpriteSAT(vdp, satBase, 2, 128, 1, 1, 0, false, false, false, 0, 2, 200)

	// Tile 2: all color 3
	for row := 0; row < 8; row++ {
		base := 64 + row*4
		vdp.vram[base] = 0x33
		vdp.vram[base+1] = 0x33
		vdp.vram[base+2] = 0x33
		vdp.vram[base+3] = 0x33
	}

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	// Sprite 2 at screen X=72 (200-128) should NOT have rendered
	if vdp.lineBufSpr[72].colorIndex != 0 {
		t.Error("sprite at X=200 should be masked after xRaw=0 following a non-zero-X sprite")
	}
}

func TestVDP_SpriteOffScreenPixelsCountTowardLimit(t *testing.T) {
	// In H32 mode, pixel limit is 256. Place a 4-cell-wide sprite (32px)
	// mostly off the left edge so most pixels are off-screen. Those off-screen
	// pixels should still count toward the per-line pixel limit.
	vdp := makeTestVDP()
	vdp.regs[12] = 0x00 // H32: 256px wide, 256 pixel limit
	vdp.regs[5] = 0x40
	satBase := uint16(0x8000)

	// Create sprites that together exceed 256 pixels only if off-screen pixels count.
	// Sprite 0: xRaw=1 (screenX=-127), 4 cells wide = 32px, only 1 visible pixel
	// But all 32 pixels should count toward the limit.
	// We need 8 such sprites (8*32=256) to exhaust the limit,
	// then sprite 8 should not render.
	for i := 0; i < 8; i++ {
		link := i + 1
		setupSpriteSAT(vdp, satBase, i, 128, 4, 1, link, false, false, false, 0, 1, 1)
	}
	// Sprite 8: fully visible at xRaw=160 (screenX=32)
	setupSpriteSAT(vdp, satBase, 8, 128, 1, 1, 0, false, false, false, 0, 2, 160)

	// Tile 1: all color 1
	for row := 0; row < 8; row++ {
		base := 32 + row*4
		vdp.vram[base] = 0x11
		vdp.vram[base+1] = 0x11
		vdp.vram[base+2] = 0x11
		vdp.vram[base+3] = 0x11
	}
	// Tile 2: all color 2
	for row := 0; row < 8; row++ {
		base := 64 + row*4
		vdp.vram[base] = 0x22
		vdp.vram[base+1] = 0x22
		vdp.vram[base+2] = 0x22
		vdp.vram[base+3] = 0x22
	}

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	// Sprite 8 at screenX=32 should NOT render because the pixel limit
	// was already exhausted by the 8 off-screen sprites (8*32=256).
	if vdp.lineBufSpr[32].colorIndex != 0 {
		t.Error("sprite should not render; off-screen pixels should count toward per-line pixel limit")
	}
}

func TestVDP_RenderSprites_H40SATAlignment(t *testing.T) {
	// In H40 mode, SAT base address must be $400 aligned (bit 0 of reg[5] ignored).
	// reg[5]=0x41 should produce the same SAT base as reg[5]=0x40 in H40 mode.
	vdp := makeTestVDP()
	vdp.regs[12] = 0x81 // H40
	vdp.regs[5] = 0x41  // bit 0 set - should be masked in H40

	// Effective SAT base should be (0x41 & 0x7E) << 9 = 0x40 << 9 = 0x8000
	satBase := uint16(0x8000)

	setupSpriteSAT(vdp, satBase, 0, 128, 1, 1, 0, false, false, false, 0, 1, 128)

	// Tile 1: all color 3
	for row := 0; row < 8; row++ {
		base := 32 + row*4
		vdp.vram[base] = 0x33
		vdp.vram[base+1] = 0x33
		vdp.vram[base+2] = 0x33
		vdp.vram[base+3] = 0x33
	}

	for i := range vdp.lineBufSpr {
		vdp.lineBufSpr[i] = layerPixel{}
	}

	vdp.renderSprites(0)

	// Should find the sprite at $8000, not at $8200 (the misaligned address)
	if vdp.lineBufSpr[0].colorIndex != 3 {
		t.Errorf("H40 SAT alignment: expected colorIndex 3, got %d", vdp.lineBufSpr[0].colorIndex)
	}
}
