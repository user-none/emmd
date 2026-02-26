# Sega Genesis VDP (315-5313) Technical Reference

Compiled from multiple sources for emulator development reference.

---

## 1. Overview

The Video Display Processor (VDP) is the primary graphics chip in the Sega Mega
Drive/Genesis. It handles all video output including tile-based background planes,
sprites, scrolling, color management, and DMA transfers.

### Chip Identification

| Identifier | Description |
|------------|-------------|
| 315-5313 | Sega custom IC part number (initial revision) |
| 315-5313A | Sega custom IC part number (revised, die-shrunk) |
| YM7101 | Yamaha designation (manufactured by Yamaha) |
| FC1001 | Yamaha designation for the die-shrunk 315-5313A variant |

The 315-5313 is a 128-pin QFP package that integrates the video display processor
along with a clone of the TI SN76489 PSG sound chip. Its design descends from the
Sega Master System VDP, which itself derives from the Texas Instruments TMS9918A
family. Mode 5 (the standard Genesis display mode) extends the SMS Mode 4
capabilities significantly.

In later Genesis models, the VDP was integrated into larger ASICs:

| ASIC | Yamaha Name | Integrates | Used In |
|------|-------------|------------|---------|
| 315-5487 | FC1004 | VDP + I/O + YM3438 | Model 1 VA7, early Model 2 VA0 |
| 315-5660 | FC1004 variant | VDP + I/O + YM3438 | Model 2 VA0/VA1/VA3 |
| 315-5700 | FF1004 | VDP + I/O + YM3438 | Nomad, some Model 2 |
| 315-5786 | N/A (Toshiba) | VDP + I/O (NO YM3438) | Model 2 VA2/VA2.3 |
| 315-5960 | FJ3002 (GOAC) | 68000+Z80+Z80RAM+VDP+YM3438+I/O | Model 2 VA4, Model 3 VA1 |
| 315-6123 | FQ8007 (GOAC) | Same + unified RAM | Model 3 VA2 |

### Key Specifications

| Feature | Value |
|---------|-------|
| VRAM | 64 KB |
| CRAM | 128 bytes (64 entries x 2 bytes, 9-bit color) |
| VSRAM | 80 bytes (40 entries x 2 bytes, 10-bit scroll values) |
| Registers | 24 programmable registers (0-23) |
| FIFO | 4-level, 16-bit write buffer |
| Scroll planes | 2 (Plane A, Plane B) + Window plane |
| Sprites (H32) | 64 total, 16 per scanline, 256 pixels per scanline |
| Sprites (H40) | 80 total, 20 per scanline, 320 pixels per scanline |
| Color depth | 9-bit (3 bits per R/G/B, 512 total colors) |
| Simultaneous colors | 61 (4 palettes x 16, minus shared transparent entries) |
| Tile size | 8x8 pixels (8x16 in interlace double-res) |
| Tile data | 4 bits per pixel (32 bytes/tile, 64 in interlace double-res) |

### Display Modes

**Horizontal resolution:**

| Mode | Cells | Pixels |
|------|-------|--------|
| H32 | 32 | 256 |
| H40 | 40 | 320 |

**Vertical resolution:**

| Mode | Cells | Lines | Notes |
|------|-------|-------|-------|
| V28 | 28 | 224 | Standard for NTSC and PAL |
| V30 | 30 | 240 | PAL only; causes image roll on NTSC |

**Interlace modes (Register $0C bits 2:1):**

| Value | Mode |
|-------|------|
| 00 | No interlace (progressive) |
| 01 | Standard interlace (fields alternate, same resolution) |
| 10 | Invalid |
| 11 | Double-resolution interlace (448 or 480 lines effective) |

### Clock and Timing

**Master clock (MCLK):**
- NTSC: 53.693175 MHz
- PAL: 53.203424 MHz

**Clock dividers from MCLK:**

| Component | Divider | NTSC Frequency |
|-----------|---------|----------------|
| M68000 CPU | /7 | 7,670,454 Hz |
| Z80 CPU | /15 | 3,579,545 Hz |
| VDP (H40 active) | /4 | ~13.42 MHz |
| VDP (H32 / H40 HBlank) | /5 | ~10.74 MHz |

**Scanline timing:**

Every scanline takes exactly 3420 master clocks, regardless of H32 or H40 mode.

- H32: 342 pixels per scanline, each 10 MCLK
- H40: 420 pixels per scanline. 390 pixels at 8 MCLK + 30 pixels at 10 MCLK
  (HBlank uses /5 divisor) = 3420 MCLK total

**Frame timing:**

| Parameter | NTSC | PAL |
|-----------|------|-----|
| Total scanlines | 262 | 313 |
| Frame rate | ~60 Hz | ~50 Hz |
| M68K cycles/frame | ~127,841 | ~152,010 |
| M68K cycles/scanline | ~488 | ~486 |

---

## 2. Bus Interface

### Memory Map (68000 Addresses)

The VDP occupies `$C00000-$DFFFFF`. Addresses mirror every 32 bytes throughout
this 2 MB region.

| Address | Port | Access | Function |
|---------|------|--------|----------|
| $C00000 | Data | R/W | Read/write VRAM, CRAM, or VSRAM data |
| $C00002 | Data | R/W | Mirror of $C00000 |
| $C00004 | Control | R/W | Write: register set / command. Read: status |
| $C00006 | Control | R/W | Mirror of $C00004 |
| $C00008 | HV Counter | R | Read H/V counter value |
| $C0000A | HV Counter | R | Mirror of $C00008 |
| $C00011 | PSG | W | SN76489 PSG write port |
| $C0001C | Debug | R/W | Undocumented test/debug register |

**68000 byte reads:** Even address returns high byte; odd address returns low byte
of the 16-bit port value.

**68000 long word access:** Two sequential 16-bit operations. For the control port,
a long word write sends both halves of a two-word command in one instruction.

---

## 3. Control Port Protocol

The control port ($C00004) serves three functions: register writes, address/command
setup, and status reads.

### Register Write (Single Word)

Detected when bits 15:14 = `10`:

```
  15  14  13  12  11  10   9   8   7   6   5   4   3   2   1   0
[  1   0  R4  R3  R2  R1  R0   .  D7  D6  D5  D4  D3  D2  D1  D0 ]
```

- R4-R0: Register number (0-23; values 24+ ignored)
- D7-D0: Data value

Example: Writing `$8F02` sets register $0F (auto-increment) to $02.

**IMPORTANT:** A register write is always recognized, even if a two-word command is
pending (`writePending = true`). It cancels the pending command and clears
`writePending`.

### Two-Word Address/Command Setup

Used to set up VRAM/CRAM/VSRAM reads and writes, and to trigger DMA.

**First word** (when bits 15:14 are not `10`):

```
  15  14  13  12  11  10   9   8   7   6   5   4   3   2   1   0
[ CD1 CD0 A13 A12 A11 A10  A9  A8  A7  A6  A5  A4  A3  A2  A1  A0 ]
```

Sets `writePending = true`. Updates:
- CD bits 1:0 from bits 15:14
- Address bits 13:0 from bits 13:0
- CD bits 5:2 and address bits 15:14 retained from previous state

**Second word** (when `writePending` is true and not a register write):

```
  15  14  13  12  11  10   9   8   7   6   5   4   3   2   1   0
[  .   .   .   .   .   .   .   .  CD5 CD4 CD3 CD2   .   . A15 A14 ]
```

Clears `writePending`. Updates:
- CD bits 5:2 from bits 7:4
- Address bits 15:14 from bits 1:0

### CD Bit Encoding

| CD5 | CD3:CD0 | Operation |
|-----|---------|-----------|
| 0 | 0000 | VRAM read |
| 0 | 0001 | VRAM write |
| 0 | 0011 | CRAM write |
| 0 | 0100 | VSRAM read |
| 0 | 0101 | VSRAM write |
| 0 | 1000 | CRAM read |
| 0 | 1100 | 8-bit VRAM read (undocumented) |
| 1 | xxxx | DMA operation (requires DMA enabled in reg $01) |

### Write-Pending Flag

The `writePending` flag is cleared by:

1. Writing the second word of a command
2. A register write (bits 15:14 = `10`)
3. Reading the control port (status register)
4. Reading or writing the data port

### Pre-Fetch on Read Setup

When a read command is fully set up (both words written, CD0 = 0), the VDP
immediately pre-fetches the value at the current address into an internal read
buffer and increments the address by the auto-increment value. Subsequent data port
reads return the pre-fetched value, then fetch the next.

---

## 4. Status Register

Reading the control port ($C00004) returns a 16-bit status value:

| Bit | Name | Description |
|-----|------|-------------|
| 15-10 | -- | Fixed bits (typically read as `011101`) |
| 9 | FE | FIFO empty: 1 = write FIFO has no pending entries |
| 8 | FF | FIFO full: 1 = FIFO full (4 entries), 68K stalled |
| 7 | VIP | V-interrupt pending: set at VBlank start |
| 6 | SOV | Sprite overflow: too many sprites on a scanline |
| 5 | SCL | Sprite collision: two non-transparent sprite pixels overlap |
| 4 | ODD | Odd frame: 1 = odd field (interlace modes only) |
| 3 | VB | Vertical blanking: 1 = in VBlank |
| 2 | HB | Horizontal blanking: 1 = in HBlank |
| 1 | DMA | DMA busy: 1 = DMA transfer in progress |
| 0 | PAL | Region: 1 = PAL (50 Hz, 313 lines), 0 = NTSC (60 Hz, 262 lines) |

**Side effects of reading status:**
- Clears `writePending`
- Clears VIP (V-interrupt pending)
- Clears SOV (sprite overflow) and SCL (sprite collision) flags

---

## 5. VDP Register Set

### Register $00 -- Mode Set 1

| Bit | Name | Function |
|-----|------|----------|
| 5 | LCB | Left column blank: blank leftmost 8 pixels with backdrop |
| 4 | IE1 | Horizontal interrupt enable (Level 4 H-int) |
| 1 | M3 | HV counter latch enable |

When bit 1 transitions from 1 to 0, the HV latch is released.

### Register $01 -- Mode Set 2

| Bit | Name | Function |
|-----|------|----------|
| 7 | VR | 128 KB VRAM mode (requires hardware mod) |
| 6 | DE | Display enable: 1 = active, 0 = backdrop only |
| 5 | IE0 | Vertical interrupt enable (Level 6 V-int) |
| 4 | M1 | DMA enable |
| 3 | M2 | V30 cell mode: 1 = 240 lines (PAL only) |
| 2 | -- | Must be 1 (Mode 5 select) |

**V-int enable and pending interaction:** Disabling IE0 does not clear the V-int
pending flag (VIP). If IE0 is re-enabled while VIP is still set, the V-int fires
immediately. On real hardware, the interrupt output is effectively
`VIP AND IE0`, so any transition of IE0 from 0 to 1 with VIP set causes immediate
interrupt assertion.

### Register $02 -- Plane A Nametable Base Address

| Bits | Function |
|------|----------|
| 5:3 | Address bits 15-13 |

**Formula:** `(reg[2] & 0x38) << 10`

Valid addresses: $0000, $2000, $4000, $6000, $8000, $A000, $C000, $E000.

### Register $03 -- Window Nametable Base Address

| Bits | Function |
|------|----------|
| 5:1 | Address bits 15-11 |

**H32 formula:** `(reg[3] & 0x3E) << 10` -- $800 aligned

**H40 formula:** `(reg[3] & 0x3C) << 10` -- $1000 aligned (bit 1 must be 0)

### Register $04 -- Plane B Nametable Base Address

| Bits | Function |
|------|----------|
| 2:0 | Address bits 15-13 |

**Formula:** `(reg[4] & 0x07) << 13`

### Register $05 -- Sprite Attribute Table Base Address

| Bits | Function |
|------|----------|
| 6:0 | Address bits 15-9 |

**Formula:** `(reg[5] & 0x7F) << 9`

H32: $200 aligned. H40: $400 aligned (bit 0 must be 0).

### Register $06 -- Sprite Pattern Generator Base (128 KB mode only)

Only relevant in 128 KB VRAM mode. No effect in standard 64 KB mode.

### Register $07 -- Background Color

| Bits | Function |
|------|----------|
| 5:4 | Palette line (0-3) |
| 3:0 | Color index within palette (0-15) |

The backdrop fills any pixel not covered by a non-transparent plane or sprite.

### Register $0A -- Horizontal Interrupt Counter

| Bits | Function |
|------|----------|
| 7:0 | H-interrupt interval (0-255 scanlines) |

Loaded at line 0 and each time H-int fires. Decremented each active scanline.
When it underflows below 0, H-int fires (if enabled) and counter reloads.
Value 0 = fire every line; value 1 = every 2 lines.

### Register $0B -- Mode Set 3

| Bit | Name | Function |
|-----|------|----------|
| 3 | IE2 | External interrupt enable (Level 2) |
| 2 | VSCR | V-scroll mode: 0 = full screen, 1 = per 2-cell column |
| 1:0 | HSCR | H-scroll mode (see below) |

**Horizontal scroll modes:**

| HSCR | Mode |
|------|------|
| 00 | Full screen |
| 01 | Invalid (behaves as per-cell on some hardware) |
| 10 | Per-cell (every 8 lines) |
| 11 | Per-line |

### Register $0C -- Mode Set 4

| Bit | Name | Function |
|-----|------|----------|
| 7 | RS0 | Analog clock source: 1 = use EDCLK for serial/pixel clock |
| 3 | SHI | Shadow/highlight mode enable |
| 2:1 | LSM | Interlace mode: 00=off, 01=standard, 11=double-res |
| 0 | RS1 | Horizontal cell mode: 1 = H40 (40 cells), 0 = H32 (32 cells) |

RS0 and RS1 control independent VDP subsystems. RS1 (bit 0) controls the digital
cell mode (32 vs 40 cells). RS0 (bit 7) controls the analog clock source, switching
the serial clock (SC) to use the EDCLK signal generated by the bus arbiter. Games
should set both to the same value for correct display, but the cell count is
determined solely by RS1.

### Register $0D -- H-Scroll Data Table Base Address

| Bits | Function |
|------|----------|
| 5:0 | Address bits 15-10 |

**Formula:** `(reg[13] & 0x3F) << 10` -- $400 aligned.

### Register $0E -- Nametable Pattern Generator Base (128 KB mode only)

Only relevant in 128 KB VRAM mode.

### Register $0F -- Auto-Increment Value

| Bits | Function |
|------|----------|
| 7:0 | Bytes added to address after each VRAM/CRAM/VSRAM access |

Commonly set to 2 for sequential word access. Value 0 = no increment.

### Register $10 -- Scroll Plane Size

| Bits | Function |
|------|----------|
| 5:4 | Vertical plane size (VSZ) |
| 1:0 | Horizontal plane size (HSZ) |

**Size encoding (same for H and V):**

| Value | Cells | Pixels |
|-------|-------|--------|
| 00 | 32 | 256 |
| 01 | 64 | 512 |
| 10 | Invalid (treated as 32) |
| 11 | 128 | 1024 |

Both planes share this size setting. The nametable cannot exceed 8192 bytes,
limiting valid combinations (e.g., 128x128 is invalid).

### Register $11 -- Window Horizontal Position

| Bit | Name | Function |
|-----|------|----------|
| 7 | RGHT | 0 = left edge to boundary, 1 = boundary to right edge |
| 4:0 | WHP | Boundary in 2-cell (16-pixel) units |

### Register $12 -- Window Vertical Position

| Bit | Name | Function |
|-----|------|----------|
| 7 | DOWN | 0 = top to boundary, 1 = boundary to bottom |
| 4:0 | WVP | Boundary in 1-cell (8-pixel) units |

### Registers $13-$14 -- DMA Length

| Register | Content |
|----------|---------|
| $13 | Length bits 7-0 (low byte) |
| $14 | Length bits 15-8 (high byte) |

16-bit value. **Length of 0 means $10000 (65536).** For 68K transfers, the unit is
words. For VRAM fill and copy, the unit is bytes.

### Registers $15-$17 -- DMA Source Address

| Register | Content |
|----------|---------|
| $15 | Source address bits 8-1 (A0 not stored) |
| $16 | Source address bits 16-9 |
| $17 | Mode bits 7:6, source address bits 22-17 in 5:0 |

**DMA modes (reg $17 bits 7:6):**

| Bits 7:6 | Mode |
|----------|------|
| 0x | 68K to VDP transfer |
| 10 | VRAM fill |
| 11 | VRAM copy |

For 68K transfers: `source = (reg[23] & 0x7F) << 17 | reg[22] << 9 | reg[21] << 1`

For VRAM copy: registers $15-$16 provide 16-bit VRAM source address (not shifted).

---

## 6. Tile/Pattern Format

Tiles are 8x8 pixel graphics stored in VRAM using 4 bits per pixel (4bpp).

### Storage Layout

- Normal mode: **32 bytes** per tile
- Interlace double-res: **64 bytes** per tile (8x16 pixels)
- Each pixel row is **4 bytes** (8 pixels x 4 bits)
- High nibble (bits 7:4) = left pixel, low nibble (bits 3:0) = right pixel
- Pixel value 0 = transparent; values 1-15 index into a palette line

**Byte layout for one 8-pixel row:**

```
Byte 0: [px0 : px1]
Byte 1: [px2 : px3]
Byte 2: [px4 : px5]
Byte 3: [px6 : px7]
```

**Full tile (32 bytes):**

```
Bytes  0- 3: Row 0
Bytes  4- 7: Row 1
Bytes  8-11: Row 2
Bytes 12-15: Row 3
Bytes 16-19: Row 4
Bytes 20-23: Row 5
Bytes 24-27: Row 6
Bytes 28-31: Row 7
```

The tile VRAM address is `tileIndex * 32` (or `tileIndex * 64` in interlace
double-res). The 11-bit tile index field allows addressing tiles 0-2047.

### Nametable Entry Format

Each nametable entry is a **16-bit word** (big-endian in VRAM):

```
  15  14  13  12  11  10   9   8   7   6   5   4   3   2   1   0
[  P  PAL1 PAL0 VF  HF  T10  T9  T8  T7  T6  T5  T4  T3  T2  T1  T0 ]
```

| Field | Bits | Description |
|-------|------|-------------|
| P | 15 | Priority (1 = high, 0 = low) |
| PAL | 14:13 | Palette line (0-3) |
| VF | 12 | Vertical flip |
| HF | 11 | Horizontal flip |
| TILE | 10:0 | Tile index (0-2047) |

---

## 7. Scroll Planes (Plane A and Plane B)

### Nametable Organization

Both planes share the size configured in register $10. Nametable entries are stored
in row-major order: the entry for cell (x, y) is at `base + (y * hCells + x) * 2`.

### Horizontal Scrolling

H-scroll mode is controlled by register $0B bits 1:0.

**H-Scroll Table in VRAM:**

The table base is set by register $0D. It stores 4 bytes per entry:

```
Word 0: Plane A horizontal scroll value (signed 10-bit)
Word 1: Plane B horizontal scroll value (signed 10-bit)
```

**Table offsets by mode:**

| Mode | Offset |
|------|--------|
| Full screen (00) | Always 0 |
| Per-cell (10) | `(line & ~7) * 4` |
| Per-line (11) | `line * 4` |

The scroll value is a signed 10-bit number. H-scroll is subtracted from the screen
X coordinate: `vramX = (screenX - hScroll) mod planeWidth`.

### Vertical Scrolling

V-scroll mode is controlled by register $0B bit 2.

**VSRAM Layout:**

- **Full screen:** VSRAM[0] = Plane A, VSRAM[2] = Plane B
- **Per-2-cell:** Entries alternate: A0, B0, A1, B1, ...
  - Column N (pixels N\*16 to N\*16+15):
    - Plane A: byte offset `N * 4`
    - Plane B: byte offset `N * 4 + 2`
  - Maximum 20 columns in H40 (320px / 16px = 20)

V-scroll is added to the screen Y coordinate:
`vramY = (screenY + vScroll) mod planeHeight`.

---

## 8. Window Plane

The Window plane is a fixed (non-scrolling) sub-plane that **replaces Plane A** in
its active region. Screen coordinates map directly to nametable cells.

### Window Nametable

- Width: 32 cells in H32, 64 cells in H40 (always fixed)
- Uses the same nametable entry format as scroll planes

### Window Region

Defined by registers $11 (horizontal) and $12 (vertical):

- If only one boundary is active, it solely determines the window region
- If both are active, the window covers the **union** of both regions
- A boundary value of 0 means that constraint is inactive

---

## 9. Sprite System

### Sprite Attribute Table (SAT)

Base address set by register $05. Each sprite is **8 bytes**:

```
Bytes 0-1 (Word 0): ------YY YYYYYYYY
  Bits 9:0 = Y position (offset by +128; screen Y = raw - 128)

Byte 2 (Word 1 high): ----HHVV
  Bits 3:2 = Horizontal size in cells - 1 (0-3 = 1-4 cells)
  Bits 1:0 = Vertical size in cells - 1 (0-3 = 1-4 cells)

Byte 3 (Word 1 low): -LLLLLLL
  Bits 6:0 = Link to next sprite index (0 = end of list)

Bytes 4-5 (Word 2): PCCVHTTT TTTTTTTT
  Bit 15    = Priority
  Bits 14:13 = Palette line (0-3)
  Bit 12    = Vertical flip
  Bit 11    = Horizontal flip
  Bits 10:0  = Base tile index

Bytes 6-7 (Word 3): -------X XXXXXXXX
  Bits 8:0 = X position (offset by +128; screen X = raw - 128)
```

### Link List Traversal

Sprites are NOT rendered in linear SAT order. The VDP starts at sprite 0 and
follows the link field chain. A link value of 0 terminates the list. This allows
reordering sprite priority without moving SAT data.

### Sprite Sizes

Sprites can be 1-4 cells wide and 1-4 cells tall (8x8 to 32x32 pixels). All 16
combinations are valid. Tiles within multi-cell sprites use **column-major order**:

```
tileIndex = baseTile + cellCol * vSizeCells + cellRow
```

### Per-Line Limits

| Mode | Max Sprites/Line | Max Pixels/Line | Max Total |
|------|-----------------|-----------------|-----------|
| H32 | 16 | 256 | 64 |
| H40 | 20 | 320 | 80 |

When the sprite-per-line limit is exceeded, the sprite overflow flag (status bit 6)
is set and remaining sprites on that line are not rendered.

### X=0 Masking

When a sprite's raw X position is 0 (screen X = -128) and at least one previous
sprite on the same scanline had a non-zero X position, all subsequent sprites on
that scanline are masked. This is commonly used to hide sprites at screen edges
during scrolling.

### Sprite Collision

When two non-transparent sprite pixels overlap at the same position, the collision
flag (status bit 5) is set. The first sprite (earlier in link order) takes visual
priority.

---

## 10. Color System

### CRAM Format

128 bytes storing 64 color entries as big-endian 16-bit words.

**Bit layout:**

```
High byte (even addr): 0000 BBB0
Low byte  (odd addr):  GGG0 RRR0
```

Each channel (R, G, B) is 3 bits (values 0-7), giving 9-bit color and 512 possible
colors.

**3-bit to 8-bit conversion:**

```
output = (val << 5) | (val << 2) | (val >> 1)
```

Maps 0 to 0, 7 to 255, with proportional intermediate values.

### Palette Organization

| Palette | CRAM Index | Byte Offset |
|---------|-----------|-------------|
| 0 | 0-15 | $00-$1F |
| 1 | 16-31 | $20-$3F |
| 2 | 32-47 | $40-$5F |
| 3 | 48-63 | $60-$7F |

Color index 0 in each palette line is transparent. The final CRAM index for a pixel
is `palette * 16 + colorIndex`.

---

## 11. Priority and Compositing

### Layer Priority Order

The VDP composites layers in the following order (highest to lowest):

1. High-priority sprite (non-transparent)
2. High-priority Plane A / Window (non-transparent)
3. High-priority Plane B (non-transparent)
4. Low-priority sprite (non-transparent)
5. Low-priority Plane A / Window (non-transparent)
6. Low-priority Plane B (non-transparent)
7. Backdrop color (register $07)

At each pixel position, the first non-transparent result wins. The priority bit in
nametable entries and sprite attributes controls high vs low grouping.

**Left column blank** (register $00 bit 5): When set, the leftmost 8 pixels are
replaced with the backdrop color regardless of layer content.

---

## 12. Shadow/Highlight Mode

Enabled by register $0C bit 3. Provides three brightness levels per pixel,
effectively tripling the visible color palette from 512 to ~1,536 colors.

### Brightness Levels

| Level | Calculation |
|-------|-------------|
| Shadow | R/G/B halved (`value >> 1`) |
| Normal | Standard color |
| Highlight | R/G/B + 128, clamped to 255 |

### Rules

1. Default brightness is **shadow** (everything starts shadowed)
2. High-priority plane pixels (Plane A, B, Window with priority=1) = **normal**
3. High-priority sprite pixels = **normal**
4. Low-priority sprite pixels from **palette 3** (non-operator) = **normal**
5. Other low-priority sprite pixels = **shadow**
6. Low-priority plane pixels = **shadow**

### Palette 3 Special Operators

When a **low-priority** sprite pixel uses **palette 3** with specific color indices,
it acts as a brightness modifier instead of displaying its own color:

| Palette 3 Index | Effect |
|-----------------|--------|
| 14 | **Highlight operator**: sprite is invisible, underlying pixel at highlight brightness |
| 15 | **Shadow operator**: sprite is invisible, underlying pixel at shadow brightness |

These operators only apply to low-priority sprites. High-priority palette 3 sprites
with index 14 or 15 render normally (not as operators).

The underlying pixel for an operator is found by checking Plane A, then Plane B,
then backdrop.

---

## 13. DMA (Direct Memory Access)

### DMA Modes

Selected by register $17 bits 7:6:

| Mode | Bits 7:6 | Description |
|------|----------|-------------|
| 68K to VDP | 0x | Transfer from 68K address space to VRAM/CRAM/VSRAM |
| VRAM Fill | 10 | Fill VRAM with a byte value |
| VRAM Copy | 11 | Copy data within VRAM |

DMA must be globally enabled via register $01 bit 4. DMA is triggered when CD5=1
in the command word.

### 68K to VDP Transfer

Reads word-aligned data from 68K bus and writes to the target VDP memory.

**Process per word:**

1. Read 16-bit word from 68K bus at source address
2. Write to VDP destination at current address register
3. Source address += 2 (internally stored as address >> 1, increments by 1)
4. Destination address += auto-increment value
5. Length counter -= 1
6. Repeat until length = 0

**Bus behavior:** The 68K CPU is halted during the transfer (bus seized by VDP).

**Source register update:** Registers $15-$17 are updated to reflect the final
source address after completion. Mode bits (reg $17 bit 7) are preserved.

**128KB boundary:** Transfers must not cross a 128KB boundary. The source address
wraps within the bank.

### VRAM Fill

Uses a two-step "fill pending" mechanism:

1. **Setup:** Control port command with CD5=1 and mode=10 sets `dmaFillPending`
2. **Trigger:** Next data port write starts the fill

**Fill behavior:**

1. Written word is first stored normally at the current address
2. Address increments by auto-increment
3. For each remaining byte in length:
   - The **high byte** of the written value goes to `vram[address ^ 1]`
   - The XOR with 1 targets the opposite byte within each word-aligned pair
   - Address increments by auto-increment

The 68K CPU is **not** halted during VRAM fill.

### VRAM Copy

Byte-by-byte copy within VRAM:

1. Read byte from `vram[source]`
2. Write byte to `vram[destination]`
3. Source += 1 (linear)
4. Destination += auto-increment
5. Length -= 1

Source uses registers $15-$16 as a 16-bit VRAM byte address (not shifted).

The 68K CPU is **not** halted during VRAM copy.

### DMA Throughput

Bytes transferred per scanline:

| DMA Mode | H32 Active | H32 Blank | H40 Active | H40 Blank |
|----------|-----------|-----------|-----------|-----------|
| 68K to VDP | 16 | 167 | 18 | 205 |
| VRAM Fill | 15 | 166 | 17 | 204 |
| VRAM Copy | 8 | 83 | 9 | 102 |

VRAM Copy is roughly half-speed because each byte needs both a read and write slot.

"Blank" applies to VBlank and when the display is forcibly disabled via register 1
bit 6. When the display is disabled, DMA operates at blanking rates during the
entire frame.

---

## 14. VRAM Addressing

### Odd-Address Byte Swap

When a 16-bit write targets an odd VRAM address:

- The address is forced to even (bit 0 cleared)
- The bytes are **swapped**: low byte goes to even address, high byte to odd

This occurs because VRAM address bit 0 is used internally for byte-lane selection,
not address decode. The same mechanism causes the `address ^ 1` behavior in VRAM
fill and the 8-bit VRAM read mode.

### 8-Bit VRAM Read

Command code `0x0C` reads a single byte from `vram[address ^ 1]`, returning it in
the low byte of the 16-bit result.

---

## 15. H/V Counters

### H Counter

The H counter is internally 9 bits; the upper 8 bits are externally readable.

**Ranges (external 8-bit values):**

| Mode | Active | Gap | HBlank |
|------|--------|-----|--------|
| H32 | $00-$93 | $94-$E8 skipped | $E9-$FF |
| H40 | $00-$B6 | $B7-$E3 skipped | $E4-$FF |

### V Counter

The V counter is internally 9 bits with 8 bits externally readable.

**Ranges and jump points:**

| Region/Mode | Count | Jump After | Jump To | Total Lines |
|-------------|-------|-----------|---------|-------------|
| NTSC V28 | $00-$EA | $EA | $E5 | 262 |
| PAL V28 | $00-$FF, $00-$02 | $02 | $CA | 313 |
| PAL V30 | $00-$FF, $00-$0A | $0A | $D2 | 313 |

### HV Counter Read Format ($C00008)

**Non-interlace:** `V7:V0 | H8:H1` (V counter high byte, H counter low byte)

**Interlace normal:** `V7:V1:V8 | H8:H1` (V counter bit 8 replaces bit 0)

**Interlace double-res:** `V7:V1:V8 | H8:H1` (counter doubled: V = vCounter*2 + oddField, bit 8 to bit 0)

### HV Counter Latching

Controlled by register $00 bit 1 (M3):

- Setting bit 1 to 1: captures the current HV counter value
- While bit 1 is set: reads return the latched value
- Clearing bit 1 (transition 1 to 0): releases the latch

The TH pin on controller ports also triggers latching (used by light guns).

---

## 16. V-Blank and H-Blank Timing

### V-Blank

| Event | NTSC V28 | PAL V28 | PAL V30 |
|-------|----------|---------|---------|
| VBlank start | Line 224 | Line 224 | Line 240 |
| VBlank end | Line 0 (next frame) | Line 0 | Line 0 |
| VBlank duration | 38 lines | 89 lines | 73 lines |

V-interrupt pending flag is set simultaneously with VBlank start.

### H-Blank

- **Start:** After active display (H counter ~$B2 in H40, ~$92 in H32)
- **End:** Near start of next active display (H counter ~$05)
- Duration: approximately 27% of each scanline

---

## 17. Interlace Modes

### Mode 0 (No Interlace)

Standard progressive display. Each frame shows all lines.

### Mode 1 (Standard Interlace)

Alternates odd/even fields each frame. The ODD status flag toggles. Resolution is
unchanged; the effect is a slight vertical shimmer on real CRT hardware.

### Mode 3 (Double Resolution Interlace)

- Tiles become 8x16 pixels (64 bytes each instead of 32)
- Effective vertical resolution doubles: 224 becomes 448, 240 becomes 480
- Scanline N renders to framebuffer row `N*2 + oddField`
- The odd/even field flag toggles at the start of each VBlank

---

## 18. SMS and Game Gear VDP Comparison

The Genesis VDP descends from the SMS VDP line but adds substantial capabilities.
This section documents the key differences for context.

### SMS VDP (315-5124 / 315-5246)

The SMS VDP derives from the Texas Instruments TMS9918A. It implements "Mode 4"
which provides improved capabilities over TMS9918A modes 0-3.

**Chip revisions:**

| Chip | Notes |
|------|-------|
| 315-5124 | Mark III / SMS1. Supports TMS9918A legacy modes. Has sprite zoom bug. |
| 315-5246 | SMS2. Adds 224/240-line modes. Fixes sprite zoom. Drops some TMS9918A behaviors. |

**Memory:**

- VRAM: 16 KB (external)
- CRAM: 32 bytes (32 entries x 1 byte, write-only from CPU)

**Display:**

- Mode 4 (primary): 256x192 (32x24 tiles)
- 224-line mode (315-5246 only): 256x224, nametable becomes 32x32
- 240-line mode (315-5246 only): 256x240, rarely used

**Color:**

- 64 total colors (6-bit: 2 bits per R/G/B channel)
- CRAM format: 1 byte per entry, `--BBGGRR`
- 32 simultaneous colors (2 palettes of 16)

**Background:**

- Single scrolling plane
- Nametable: 32x28 entries (16-bit each)
- Entry format: 9-bit tile index, H/V flip, palette select, priority
- 512 addressable tiles

**Scrolling:**

- Whole-screen horizontal scroll (register 8, per-line via H-int raster tricks)
- Whole-screen vertical scroll (register 9, latched per frame)
- Scroll inhibit: top 2 rows (horizontal), right 8 columns (vertical)
- No per-tile or per-line hardware scrolling

**Sprites:**

- 64 total, 8 per scanline
- Sizes: 8x8 or 8x16 (global setting)
- 2x zoom magnification available
- SAT: 256 bytes (Y coords, then X + pattern pairs)
- Always use palette 1
- No per-sprite flip
- Hardware collision flag

### Game Gear VDP

Essentially identical to the SMS2 VDP (315-5246) with two modifications:

**Color:**

- 4,096 total colors (12-bit: 4 bits per R/G/B channel)
- CRAM format: 2 bytes per entry, `----BBBBGGGGRRRR`
- 64 bytes CRAM (32 entries x 2 bytes)
- Still 32 simultaneous colors

**Display:**

- Internal rendering: 256x192 (same as SMS)
- LCD viewport: 160x144 (cropped from center, 48px each side, 24px top/bottom)
- Sprites and tiles outside viewport still consume rendering resources

All other capabilities identical to SMS2 (tile format, scrolling, sprites, etc.).

### Genesis Mode 4 (SMS Backwards Compatibility)

The Genesis VDP includes a Mode 4 for SMS compatibility, activated by the M3
hardware pin (cartridge pin B30 active low).

**What works:**

- Standard Mode 4 (256x192)
- Full SMS tile rendering, scrolling, sprites
- SMS 6-bit color palette
- Z80 runs SMS code natively

**What does NOT work:**

- No TMS9918A modes (0-3) -- SG-1000 games will not display correctly
- No 224-line or 240-line extended modes
- No sprite zoom/magnification
- Genesis 3 and Nomad have pin B30 disconnected (no Mode 4 without hardware mod)

### Comparison Table

| Feature | SMS VDP | Game Gear VDP | Genesis VDP |
|---------|---------|---------------|-------------|
| VRAM | 16 KB | 16 KB | 64 KB |
| CRAM | 32 bytes (1B/entry) | 64 bytes (2B/entry) | 128 bytes (2B/entry) |
| VSRAM | None | None | 80 bytes |
| Color depth | 6-bit (64 colors) | 12-bit (4,096 colors) | 9-bit (512 colors) |
| On-screen colors | 32 | 32 | 61 |
| Resolution | 256x192 | 160x144 viewport | 320x224 / 256x224 |
| Interlace | No | No | Yes (up to 448/480 lines) |
| Background planes | 1 | 1 | 2 + Window |
| Tile size | 8x8, 4bpp | 8x8, 4bpp | 8x8, 4bpp |
| Max tiles | 512 | 512 | ~2048 |
| Total sprites | 64 | 64 | 80 (H40) / 64 (H32) |
| Sprites/line | 8 | 8 | 20 (H40) / 16 (H32) |
| Sprite sizes | 8x8 or 8x16 (global) | Same as SMS | 1x1 to 4x4 cells (per-sprite) |
| Sprite zoom | 2x magnification | 2x magnification | None (variable size instead) |
| Per-sprite flip | No | No | Yes (H and V) |
| Per-sprite palette | No (palette 1 only) | No | Yes (any of 4 palettes) |
| H-scroll modes | Whole-screen only | Same as SMS | Full-screen, per-8-line, per-line |
| V-scroll modes | Whole-screen (latched) | Same as SMS | Full-screen, per-2-column |
| Shadow/Highlight | No | No | Yes |
| DMA | No | No | Yes (3 modes) |
| Registers | 11 | 11 | 24 |

---

## Sources

- [SMS Power - YM2612 / VDP Technical Manual](https://www.smspower.org/maxim/Documents/YM2612)
- [Plutiedev - VDP Registers](https://www.plutiedev.com/vdp-registers)
- [Plutiedev - SEGA Mega Drive / Genesis Hardware Notes (Kabuto)](https://plutiedev.com/mirror/kabuto-hardware-notes)
- [MegaDrive Wiki - VDP](https://md.railgun.works/index.php?title=VDP)
- [MegaDrive Development Wiki - VDP Registers](https://wiki.megadrive.org/index.php?title=VDP_Registers)
- [MegaDrive Development Wiki - VDP Ports](https://wiki.megadrive.org/index.php?title=VDP_Ports)
- [MegaDrive Development Wiki - VDP DMA](https://wiki.megadrive.org/index.php?title=VDP_DMA)
- [MegaDrive Development Wiki - VDP Sprites](https://wiki.megadrive.org/index.php?title=VDP_Sprites)
- [Charles MacDonald - Sega Genesis VDP Documentation (genvdp.txt v1.5f)](https://www.neperos.com/article/picoxo1fc33c8979)
- [Sega Technical Overview (Official PDF)](https://segaretro.org/images/1/18/GenesisTechnicalOverview.pdf)
- [Mega Cat Studios - VDP Graphics Guide](https://megacatstudios.com/blogs/retro-development/sega-genesis-mega-drive-vdp-graphics-guide-v1-2a-03-14-17)
- [Raster Scroll - Overview of the VDP](https://rasterscroll.com/mdgraphics/vdp-overview/)
- [Raster Scroll - Shadow and Highlight](https://rasterscroll.com/mdgraphics/graphical-effects/shadow-and-highlight/)
- [RadDad772 - Genesis VDP Internals (Parts 1-3)](https://raddad772.github.io/2024/07/18/genesis-vdp-pt-1.html)
- [Copetti - Mega Drive / Genesis Architecture](https://www.copetti.org/writings/consoles/mega-drive-genesis/)
- [SpritesMind Forums - VDP Hardware Research](https://gendev.spritesmind.net/forum/viewtopic.php?t=768)
- [SpritesMind Forums - VDP VRAM Access Timing (RS0/RS1 analysis)](https://gendev.spritesmind.net/forum/viewtopic.php?t=851)
- [SpritesMind Forums - Enable/Disable V-int in Mode Register 2 (hardware test)](https://gendev.spritesmind.net/forum/viewtopic.php?t=3337)
- [Console5 TechWiki - 315-5313](https://wiki.console5.com/wiki/315-5313)
- [ConsoleMods Wiki - Genesis ASIC Information](https://consolemods.org/wiki/Genesis:ASIC_Information)
- [SMS Power - SMS VDP Documentation (Charles MacDonald)](https://www.smspower.org/uploads/Development/msvdp-20021112.txt)
- [SMS Power - VDP Registers](https://www.smspower.org/Development/VDPRegisters)
- [SMS Power - SMS1/SMS2 VDPs](https://www.smspower.org/Development/SMS1SMS2VDPs)
- [SMS Power - Sprites](https://www.smspower.org/Development/Sprites)
- [SMS Power - Palette](https://www.smspower.org/Development/Palette)
- [SMS Power - GG VDP](https://www.smspower.org/Development/GGVDP)
- [Plutiedev - Changing Screen Resolution](https://plutiedev.com/screen-resolution)
- [Hugues Johnson - Sega Genesis Programming: Palettes](https://huguesjohnson.com/programming/genesis/palettes/)
