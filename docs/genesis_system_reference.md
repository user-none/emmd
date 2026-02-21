# Sega Genesis / Mega Drive System Reference

Technical reference for the overall Genesis hardware architecture, memory map,
controller I/O, cartridge format, interrupt system, bus arbitration, and boot
sequence.

For details on the VDP (video display processor), see `vdp_reference.md`.
For details on the YM2612 FM synthesizer, see `ym2612_reference.md`.
For details on audio system integration, mixing, and Z80 sound driver
communication, see `genesis_sound_integration.md`.

This document does not cover the Motorola 68000 or Zilog Z80 CPU internals, nor
the TI SN76489 PSG internals, as those are documented in their respective module
source trees.

---

## 1. System Overview

The Sega Genesis (known as Mega Drive outside North America) is a 16-bit home
console built around a dual-CPU architecture with dedicated video and audio
hardware.

### 1.1 Major Components (Discrete Model 1, VA0-VA5)

| Component | Part Number | Function |
|-----------|-------------|----------|
| Main CPU | Motorola 68000 (MC68000P8/P10) | 16/32-bit main processor |
| Co-processor | Zilog Z80 (Z84C0008PEC or equiv.) | 8-bit sound CPU |
| VDP | Sega 315-5313 (Yamaha YM7101) | Video display processor + embedded SN76489 PSG |
| FM Synth | Yamaha YM2612 | 6-channel FM synthesis |
| I/O Controller | Sega 315-5309 | Controller ports, serial, version register |
| Bus Arbiter | Sega 315-5308 | 68K/Z80 bus arbitration |
| Work RAM | 64 KB DRAM | 68000 main memory |
| VRAM | 64 KB DRAM | Video RAM (VDP-local) |
| Z80 RAM | 8 KB SRAM | Z80 program and data RAM |

### 1.2 Bus Architecture

The system uses two distinct buses mediated by a bus arbiter:

**68000 Bus** (24-bit address, 16-bit data):
- Cartridge ROM (directly mapped at $000000)
- 64 KB work RAM ($FF0000, mirrored through $E00000-$FFFFFF)
- VDP ports ($C00000-$C0001F, mirrored through $C00000-$DFFFFF)
- I/O controller registers ($A10000-$A1001F)
- Z80 address space window ($A00000-$A0FFFF, accessible when 68K holds Z80 bus)

**Z80 Bus** (16-bit address, 8-bit data):
- 8 KB Z80 RAM ($0000-$1FFF)
- YM2612 FM synthesizer ($4000-$5FFF)
- PSG (SN76489) write port ($7F11)
- 32 KB banked window into 68K address space ($8000-$FFFF)

The bus arbiter mediates access between the two buses. The 68K can request the
Z80 bus (pausing the Z80), and the Z80 can access 68K memory through its bank
window (which stalls the 68K during the access). The PSG is physically inside the
VDP but is accessible from both CPUs. The YM2612 is on the Z80 bus; the 68K
accesses it at $A04000-$A04003 when it holds the Z80 bus.

---

## 2. 68000 Memory Map

The 68000 has a 24-bit address bus, providing a 16 MB address space. The upper
8 bits of any 32-bit address are ignored.

| Address Range | Size | Description |
|---------------|------|-------------|
| $000000-$3FFFFF | 4 MB | Cartridge ROM (directly mapped, read-only) |
| $400000-$7FFFFF | 4 MB | Unused (reads return open-bus values; writes ignored) |
| $800000-$9FFFFF | 2 MB | Reserved (32X maps here; accessing without 32X may lock system) |
| $A00000-$A0FFFF | 64 KB | Z80 address space window (only accessible when 68K holds Z80 bus) |
| $A10000-$A1001F | 32 bytes | I/O registers (version, controller data/ctrl, serial) |
| $A11100-$A11101 | 2 bytes | Z80 BUSREQ register |
| $A11200-$A11201 | 2 bytes | Z80 RESET register |
| $A14000-$A14003 | 4 bytes | TMSS VDP unlock register |
| $A14101 | 1 byte | TMSS ROM/cartridge bus select |
| $A130F0-$A130FF | 16 bytes | Cartridge bank-switching / SRAM control |
| $C00000-$DFFFFF | 2 MB | VDP ports (mirrored every 32 bytes; see `vdp_reference.md`) |
| $E00000-$FFFFFF | 2 MB | 68K work RAM (64 KB physical, mirrored every $10000) |

### 2.1 VDP Port Mirroring ($C00000-$DFFFFF)

The VDP address decode uses bits A1-A4 for port selection. Higher address bits
within the $C00000-$DFFFFF range are ignored, creating extensive mirroring.

| Base Address | Port | Access |
|-------------|------|--------|
| $C00000 | VDP data port | 8/16-bit R/W |
| $C00002 | VDP data port (mirror) | 8/16-bit R/W |
| $C00004 | VDP control / status port | 16-bit R/W (read = status) |
| $C00006 | VDP control (mirror) | 16-bit R/W |
| $C00008-$C0000E | HV counter (mirrored) | 8/16-bit read-only |
| $C00011 | SN76489 PSG write | 8-bit write-only (odd byte) |
| $C00013-$C00017 | PSG (mirrors) | 8-bit write-only |

### 2.2 Bank-Switching / SRAM Registers ($A130F0-$A130FF)

Used by cartridges larger than 4 MB (e.g., Super Street Fighter II uses the
SSF2 mapper) and cartridges with battery-backed SRAM.

| Address | Function |
|---------|----------|
| $A130F1 | SRAM control (bit 0: SRAM enable; bit 1: write enable) |
| $A130F3 | Bank 1: selects 512 KB ROM page for $080000-$0FFFFF |
| $A130F5 | Bank 2: selects 512 KB ROM page for $100000-$17FFFF |
| $A130F7 | Bank 3: selects 512 KB ROM page for $180000-$1FFFFF |
| $A130F9 | Bank 4: selects 512 KB ROM page for $200000-$27FFFF |
| $A130FB | Bank 5: selects 512 KB ROM page for $280000-$2FFFFF |
| $A130FD | Bank 6: selects 512 KB ROM page for $300000-$37FFFF |
| $A130FF | Bank 7: selects 512 KB ROM page for $380000-$3FFFFF |

Bank 0 ($000000-$07FFFF) is fixed and cannot be remapped. Each bank register
uses 6 bits, allowing selection of up to 64 pages (64 x 512 KB = 32 MB maximum
ROM). Default state: bank N maps to page N (identity mapping).

---

## 3. Z80 Memory Map

The Z80 has a 16-bit address bus. All peripherals are memory-mapped; the Z80 I/O
port space is unused on the Genesis.

| Address Range | Size | Description |
|---------------|------|-------------|
| $0000-$1FFF | 8 KB | Z80 RAM (program and data) |
| $2000-$3FFF | 8 KB | Z80 RAM mirror |
| $4000-$5FFF | 8 KB | YM2612 ports (4 registers, heavily mirrored) |
| $6000 | 1 byte | Bank address register (write-only; reads return $FF) |
| $6001-$7EFF | | Unused (reads return $FF) |
| $7F00-$7F1F | 32 bytes | VDP ports (same layout as 68K $C00000-$C0001F) |
| $7F20-$7FFF | | Reserved (accessing can lock the system) |
| $8000-$FFFF | 32 KB | 68K address space bank window |

### 3.1 YM2612 Ports

Only 2 address lines (A0, A1) are decoded; the remaining bits mirror:

| Z80 Address | 68K Address | Function |
|-------------|-------------|----------|
| $4000 | $A04000 | Part I address register (also status read port) |
| $4001 | $A04001 | Part I data register |
| $4002 | $A04002 | Part II address register |
| $4003 | $A04003 | Part II data register |

See `ym2612_reference.md` for register details.

### 3.2 Bank Register ($6000)

The bank register selects which 32 KB page of the 68K address space is visible
at Z80 addresses $8000-$FFFF. The lower 15 bits of the 68K address come from the
Z80 address lines. The upper 9 bits (bits 15-23) are set via the bank register.

**Programming**: Write 9 times to bit 0 of address $6000, LSB first (bit 15
first, bit 23 last). Each write shifts in one bit:

```
bank_register = (bank_register >> 1) | ((written_value & 1) << 8)
```

After reset, the bank register defaults to $000 (all zero), mapping $8000-$FFFF
to 68K addresses $000000-$007FFF (beginning of cartridge ROM).

### 3.3 Z80 Access to 68K Address Space

When the Z80 reads or writes to $8000-$FFFF, the bus arbiter pauses the 68K,
performs the access on the 68K bus, then resumes the 68K. This introduces delays
of approximately 2-5 Z80 cycles for the Z80 and 1-6 68K cycles for the 68K per
access.

---

## 4. Z80 Bus Arbitration

### 4.1 BUSREQ Register ($A11100)

The 68K requests and releases the Z80 bus through this register.

**Writing:**

| Operation | Value (word) | Value (byte to $A11100) | Effect |
|-----------|-------------|------------------------|--------|
| Request bus | $0100 | $01 | Pause Z80, grant bus to 68K |
| Release bus | $0000 | $00 | Resume Z80 execution |

**Reading:**

| Bit 0 value | Meaning |
|-------------|---------|
| 0 | Bus granted to 68K (Z80 halted; safe to access Z80 address space) |
| 1 | Z80 still running / bus not yet granted |

After writing $0100, the Z80 finishes its current instruction before granting
the bus. The 68K must poll the register until bit 0 reads 0.

### 4.2 RESET Register ($A11200)

| Operation | Value (word) | Value (byte to $A11200) | Effect |
|-----------|-------------|------------------------|--------|
| Assert reset | $0000 | $00 | Hold Z80 in reset state |
| Deassert reset | $0100 | $01 | Release Z80 from reset |

The Z80 /RESET line must be held low for a minimum of 3 Z80 clock cycles
(~838 ns) for the reset to take effect. In practice, software waits
approximately 192 68K cycles.

**IMPORTANT**: The YM2612 shares the Z80 reset line. Resetting the Z80 also
resets the YM2612.

### 4.3 Standard Z80 Initialization Sequence

1. Assert Z80 reset: write $0000 to $A11200
2. Request Z80 bus: write $0100 to $A11100
3. Deassert Z80 reset: write $0100 to $A11200
4. Poll $A11100 until bit 0 reads 0 (bus granted)
5. Load Z80 program to $A00000-$A01FFF using byte writes
6. Assert Z80 reset: write $0000 to $A11200
7. Wait at least 192 68K cycles
8. Deassert reset: write $0100 to $A11200
9. Release bus: write $0000 to $A11100

The Z80 now executes from address $0000 in its RAM.

### 4.4 Mid-Operation Bus Access

To access Z80 bus devices (YM2612, Z80 RAM) from the 68K during normal
operation:

1. Write $0100 to $A11100 (request bus)
2. Poll $A11100 until bit 0 reads 0 (bus granted)
3. Perform accesses (write YM2612 at $A04000-$A04003, read/write Z80 RAM, etc.)
4. Write $0000 to $A11100 (release bus)

### 4.5 Access Constraints

- Z80 address space can only be accessed by the 68K when the 68K holds the bus
  (BUSREQ granted) and the Z80 is not in reset
- Only byte-wide (8-bit) writes to Z80 RAM are reliable; the Z80 bus is 8 bits
  wide
- Halting the Z80 during VDP DMA transfers is recommended on early board
  revisions to avoid bus conflicts

---

## 5. Interrupt System

### 5.1 68000 Interrupt Levels

The 68000 supports 7 hardware interrupt priority levels. The Genesis uses three:

| Level | Vector # | Vector Address | Source | Description |
|-------|----------|----------------|--------|-------------|
| 2 | 26 | $068 | External / TH pin | Controller TH-pin interrupt (when enabled via I/O control register bit 7) |
| 4 | 28 | $070 | HBlank (VDP) | Horizontal blank interrupt, configurable via VDP register $0A |
| 6 | 30 | $078 | VBlank (VDP) | Vertical blank interrupt, once per frame at start of vertical blanking |

### 5.2 Interrupt Masking

The 68000 status register contains a 3-bit interrupt priority mask (bits 8-10).
Only interrupts with a priority level **strictly greater** than the current mask
value are accepted. Level 7 is non-maskable but is not used by the Genesis in
normal operation.

The 68000 interrupt inputs are **level-triggered**, not edge-triggered. If an
interrupt is asserted while the mask blocks it, and the mask is later lowered,
the interrupt fires immediately if the signal is still asserted.

### 5.3 VDP Interrupt Enable Bits

The VDP has independent enable bits controlling whether interrupt signals are
generated at all. Both the VDP enable bit and the 68000 SR mask must allow the
interrupt for a handler to execute.

| VDP Register | Bit | Name | Function |
|-------------|-----|------|----------|
| $00 | 4 | IE1 | HBlank interrupt enable (1 = generate Level 4 interrupt) |
| $01 | 5 | IE0 | VBlank interrupt enable (1 = generate Level 6 interrupt) |
| $0B | 3 | IE2 | External interrupt enable (1 = generate Level 2 interrupt) |

### 5.4 HBlank Interrupt Counter (VDP Register $0A)

The VDP maintains an internal counter loaded with the value in register $0A at
the top of the active display area. The counter decrements each scanline. When
it reaches zero:

- If IE1 is set, a Level 4 interrupt is generated
- The counter is reloaded from register $0A

Register $0A = $00 generates an interrupt every scanline, $01 every other
scanline, $02 every third scanline, etc.

During VBlank, the counter continues to run but is reloaded from register $0A
each line.

### 5.5 Z80 Interrupt

The Z80 operates in Interrupt Mode 1 (IM 1). When the maskable interrupt (/INT)
is asserted and interrupts are enabled (EI), the Z80 executes RST $38 (push PC,
jump to $0038).

The Z80 /INT line is tied to the VDP VBlank signal. It fires once per frame at
the start of VBlank and is held low for approximately 228 Z80 clock cycles
(~63.7 us at NTSC speed). If the interrupt handler completes within that window
and re-enables interrupts (EI), it will re-trigger. Games typically include
delay loops to avoid this.

If interrupts are disabled (DI) when /INT is asserted, the interrupt is lost for
that frame. The Z80 has no NMI connection in normal Genesis operation.

---

## 6. Controller I/O

### 6.1 DB-9 Connector Pinout

The Genesis controller port uses a standard male DB-9 connector on the console.

```
  DB-9 Male (console front)
  -------------------------
  \  1   2   3   4   5  /
   \  6   7   8   9  /
    -----------------
```

| Pin | Signal | Direction | Function |
|-----|--------|-----------|----------|
| 1 | D0 | Controller to Console | Up |
| 2 | D1 | Controller to Console | Down |
| 3 | D2 | Controller to Console | Left / GND when TH=0 |
| 4 | D3 | Controller to Console | Right / GND when TH=0 |
| 5 | VCC | Console to Controller | +5V power |
| 6 | TL | Controller to Console | Button B (TH=1) / Button A (TH=0) |
| 7 | TH | Console to Controller | Select line (active output) |
| 8 | GND | -- | Ground |
| 9 | TR | Controller to Console | Button C (TH=1) / Start (TH=0) |

All button signals use **active-low** signaling: 0 = pressed, 1 = released.

The 3-button controller uses a 74HC157 quad 2-input multiplexer. The TH (select)
line switches which set of inputs is routed to the output pins. All pins have
internal pull-up resistors.

### 6.2 I/O Register Map ($A10001-$A1001F)

All I/O registers are byte-wide at odd addresses.

#### 6.2.1 Version Register ($A10001, Read-Only)

| Bit | Name | Description |
|-----|------|-------------|
| 7 | MODE | 0 = Domestic (Japan), 1 = Overseas (US/Europe) |
| 6 | VMOD | 0 = NTSC (60 Hz), 1 = PAL (50 Hz) |
| 5 | DISK | 0 = Expansion unit (Mega CD) connected, 1 = not connected |
| 4 | -- | Reserved |
| 3-0 | VER | Hardware version ($0 = no TMSS, $1+ = TMSS present) |

Common composite values (no expansion unit):

| Value | Region |
|-------|--------|
| $20 | Japan NTSC (Mega Drive) |
| $A0 | USA NTSC (Genesis) |
| $E0 | Europe PAL (Mega Drive) |

#### 6.2.2 Data Registers ($A10003, $A10005, $A10007)

| Address | Port |
|---------|------|
| $A10003 | Port 1 (player 1 controller) |
| $A10005 | Port 2 (player 2 controller) |
| $A10007 | Port 3 (expansion / modem) |

Default state: $7F.

Bit layout:

| Bit 7 | Bit 6 | Bit 5 | Bit 4 | Bit 3 | Bit 2 | Bit 1 | Bit 0 |
|-------|-------|-------|-------|-------|-------|-------|-------|
| -- | TH | TL | TR | D3 (Right) | D2 (Left) | D1 (Down) | D0 (Up) |

- **Bit 7**: Not connected to the connector; latches any value written to it
- **Bit 6 (TH)**: DB-9 pin 7, select line for controllers
- **Bit 5 (TL)**: DB-9 pin 6, Button B/A
- **Bit 4 (TR)**: DB-9 pin 9, Button C/Start
- **Bits 3-0 (D3-D0)**: DB-9 pins 4-1, direction/button data

Writing to the data port sets the value of pins configured as outputs. Reading
returns the current state: external values for input pins, last written values
for output pins.

#### 6.2.3 Control Registers ($A10009, $A1000B, $A1000D)

| Address | Port |
|---------|------|
| $A10009 | Port 1 |
| $A1000B | Port 2 |
| $A1000D | Port 3 |

Default state: $00 (all inputs).

Bit layout:

| Bit 7 | Bit 6 | Bit 5 | Bit 4 | Bit 3 | Bit 2 | Bit 1 | Bit 0 |
|-------|-------|-------|-------|-------|-------|-------|-------|
| INT | PC6 | PC5 | PC4 | PC3 | PC2 | PC1 | PC0 |

- **Bit 7 (INT)**: TH-interrupt enable. 1 = TH transition triggers Level 2
  interrupt
- **Bits 6-0**: Pin direction. 0 = input (from peripheral), 1 = output (to
  peripheral)

For standard gamepad operation, write **$40** to configure TH as output while
bits 5-0 remain inputs.

#### 6.2.4 Serial Communication Registers

Each port has three serial registers:

| Address | Register | Port | Access |
|---------|----------|------|--------|
| $A1000F | TxData1 | 1 | Write |
| $A10011 | RxData1 | 1 | Read |
| $A10013 | S-Ctrl1 | 1 | R/W |
| $A10015 | TxData2 | 2 | Write |
| $A10017 | RxData2 | 2 | Read |
| $A10019 | S-Ctrl2 | 2 | R/W |
| $A1001B | TxData3 | 3 | Write |
| $A1001D | RxData3 | 3 | Read |
| $A1001F | S-Ctrl3 | 3 | R/W |

**S-Ctrl bit layout:**

| Bit 7-6 | Bit 5 | Bit 4 | Bit 3 | Bit 2 | Bit 1 | Bit 0 |
|---------|-------|-------|-------|-------|-------|-------|
| BPS (baud rate) | SIN | SOUT | RINT | RERR | RRDY | TFUL |

Baud rate select (bits 7-6): 00 = 4800, 01 = 2400, 10 = 1200, 11 = 300 bps.

SIN/SOUT enable serial mode on the TR/TL pins respectively. In serial mode, TL
becomes serial data out and TR becomes serial data in.

### 6.3 Three-Button Controller Protocol

#### 6.3.1 Setup

Write $40 to the control register to configure TH as output and all other pins
as inputs.

#### 6.3.2 Reading Buttons

The TH line (bit 6 of the data register) is toggled to select which set of
buttons appears on the data pins.

**TH = 1** (write $40 to data port, then read):

```
Bit:    7    6    5    4    3      2     1     0
        x    1    C    B    Right  Left  Down  Up
```

**TH = 0** (write $00 to data port, then read):

```
Bit:    7    6    5    4    3      2     1     0
        x    0    Start A   0      0     Down  Up
```

All button bits are active-low: 0 = pressed, 1 = released.

When TH=0, bits 3 and 2 (normally Left and Right) are forced to 0 by the
controller hardware. This serves as a controller-connected detection mechanism:
if both bits read low when TH is low, a controller is present.

### 6.4 Six-Button Controller Protocol

#### 6.4.1 Overview

The 6-button controller (model MK-1653) adds X, Y, Z, and Mode buttons while
maintaining full backward compatibility. It contains an internal state machine
that tracks TH transitions.

#### 6.4.2 State Machine and Timeout

The controller maintains an internal counter (states 0-7). Each TH transition
increments the counter by 1. If approximately 1.5 ms elapses with no TH
transition, the counter resets to 0 (implemented via an internal RC circuit).

This timeout means games must not read controllers more than once per frame.
At 60 fps (NTSC), the ~16.6 ms between frames provides sufficient reset time.

#### 6.4.3 TH Cycle State Table

| State | TH | Data Returned | Description |
|-------|----|---------------|-------------|
| 0 | 1 | `x1CBRLDU` | Standard: C, B, Right, Left, Down, Up |
| 1 | 0 | `x0SA00DU` | Standard: Start, A, 0, 0, Down, Up |
| 2 | 1 | `x1CBRLDU` | Repeat of state 0 |
| 3 | 0 | `x0SA00DU` | Repeat of state 1 |
| 4 | 1 | `x1CBRLDU` | Repeat of state 0 |
| 5 | 0 | `x0SA0000` | **Detection**: bits 3-0 all zero (6-button ID) |
| 6 | 1 | `x1CBMXYZ` | **Extra buttons**: Mode, X, Y, Z |
| 7 | 0 | `x0SA1111` | End marker: bits 3-0 all one |

- **States 0-3**: Identical to a 3-button controller. Games that only toggle TH
  twice per frame see normal 3-button behavior.
- **State 5**: If bits 3-0 are NOT all zero, the device is a 3-button controller
  and the sequence should stop.
- **State 6**: Bits 3-0 contain Mode (3), X (2), Y (1), Z (0). All active-low.
- **State 7**: Bits 3-0 all one, distinguishing end of sequence.

#### 6.4.4 Recommended Read Sequence

1. Write $40 (TH=1) -> Read: C, B, Right, Left, Down, Up
2. Write $00 (TH=0) -> Read: Start, A, 0, 0, Down, Up
3. Write $40 (TH=1) -> Read: C, B, Right, Left, Down, Up
4. Write $00 (TH=0) -> Read: Start, A, 0, 0, Down, Up
5. Write $40 (TH=1) -> Read: C, B, Right, Left, Down, Up
6. Write $00 (TH=0) -> Read: Start, A, 0, 0, 0, 0 (check bits 3-0)
7. Write $40 (TH=1) -> Read: C, B, Mode, X, Y, Z (extra buttons)

Insert a brief delay (2 NOP instructions) after each write before reading to
allow the controller hardware to respond.

#### 6.4.5 Backward Compatibility

Games that only toggle TH twice per frame interact with states 0-1, which
behave identically to a 3-button controller. However, some games toggle TH
more frequently, causing the state counter to advance past state 3 and
producing unexpected values. The 6-button controller's Mode button forces
3-button compatibility mode when held during power-on.

### 6.5 Controller Detection

**3-button controller present**: When TH=0, bits 3 and 2 both read 0 (grounded
by the controller). If these float high, nothing is connected.

**6-button controller**: At state 5 (TH=0), all four lower bits (D0-D3) read 0.
A 3-button controller would still show Up and Down state in D0-D1 at this point.

---

## 7. ROM Cartridge Format

### 7.1 ROM Layout Overview

The first 512 bytes ($000-$1FF) contain:

- **$000-$0FF**: Motorola 68000 exception vector table (256 bytes)
- **$100-$1FF**: Cartridge information header (256 bytes)

Game code and data begin at $200.

### 7.2 Exception Vector Table ($000-$0FF)

Each entry is a 32-bit big-endian longword (64 entries total).

| Vector # | Address | Description |
|-----------|---------|-------------|
| 0 | $000-$003 | Initial Stack Pointer (SSP) |
| 1 | $004-$007 | Initial Program Counter (entry point) |
| 2 | $008-$00B | Bus Error |
| 3 | $00C-$00F | Address Error |
| 4 | $010-$013 | Illegal Instruction |
| 5 | $014-$017 | Division by Zero |
| 6 | $018-$01B | CHK Instruction |
| 7 | $01C-$01F | TRAPV Instruction |
| 8 | $020-$023 | Privilege Violation |
| 9 | $024-$027 | Trace |
| 10 | $028-$02B | Line-A Emulator |
| 11 | $02C-$02F | Line-F Emulator |
| 12-23 | $030-$05F | Reserved |
| 24 | $060-$063 | Spurious Interrupt |
| 25 | $064-$067 | Level 1 Interrupt (unused) |
| 26 | $068-$06B | Level 2 Interrupt (external / TH pin) |
| 27 | $06C-$06F | Level 3 Interrupt (unused) |
| 28 | $070-$073 | Level 4 Interrupt (HBlank) |
| 29 | $074-$077 | Level 5 Interrupt (unused) |
| 30 | $078-$07B | Level 6 Interrupt (VBlank) |
| 31 | $07C-$07F | Level 7 Interrupt (unused on Genesis) |
| 32-47 | $080-$0BF | TRAP #0 through TRAP #15 |
| 48-63 | $0C0-$0FF | Reserved |

**Key entries**:
- Vector 0: Initial SSP, typically $00FFFE00 or similar (top of 64 KB work RAM)
- Vector 1: Entry point, typically $00000200 (immediately after header)

### 7.3 Cartridge Information Header ($100-$1FF)

All text fields are ASCII, space-padded to their full length.

| Offset | Size | Field |
|--------|------|-------|
| $100-$10F | 16 bytes | System type |
| $110-$11F | 16 bytes | Copyright / release date |
| $120-$14F | 48 bytes | Domestic title |
| $150-$17F | 48 bytes | Overseas title |
| $180-$18D | 14 bytes | Serial number / version |
| $18E-$18F | 2 bytes | Checksum |
| $190-$19F | 16 bytes | I/O device support |
| $1A0-$1A3 | 4 bytes | ROM start address |
| $1A4-$1A7 | 4 bytes | ROM end address |
| $1A8-$1AB | 4 bytes | RAM start address |
| $1AC-$1AF | 4 bytes | RAM end address |
| $1B0-$1BB | 12 bytes | External memory (SRAM) info |
| $1BC-$1C7 | 12 bytes | Modem support |
| $1C8-$1EF | 40 bytes | Memo / reserved |
| $1F0-$1FF | 16 bytes | Region codes |

#### 7.3.1 System Type ($100-$10F)

16-byte ASCII string identifying the platform:

- `"SEGA MEGA DRIVE "` -- Standard for Japanese and European releases
- `"SEGA GENESIS    "` -- Used by some North American releases

The TMSS firmware validates only that the first 4 bytes are `"SEGA"`.

#### 7.3.2 Copyright / Release Date ($110-$11F)

Format: `"(C)XXXX YYYY.ZZZ"` where XXXX is a 4-character publisher code, YYYY
is the year, and ZZZ is a 3-letter month abbreviation.

Example: `"(C)SEGA 1991.DEC"`

#### 7.3.3 Domestic and Overseas Titles ($120-$17F)

48 bytes each, ASCII or Shift-JIS for Japanese titles, space-padded.

#### 7.3.4 Serial Number / Version ($180-$18D)

Format: `"TT SSSSSSSS-VV"` where:
- TT = product type (GM = game, AI/AL = educational, OS = TMSS boot ROM)
- SSSSSSSS = serial number
- VV = revision (00 = first release)

#### 7.3.5 Checksum ($18E-$18F)

16-bit big-endian checksum. Calculated as the sum of all big-endian 16-bit words
from $200 to end of ROM, allowing natural 16-bit overflow:

```
checksum = 0
for offset = $200 to (ROM_end - 1), step 2:
    word = (ROM[offset] << 8) | ROM[offset + 1]
    checksum = (checksum + word) & $FFFF
```

If the ROM has an odd trailing byte, it is treated as the high byte of a word
with low byte = 0.

#### 7.3.6 I/O Device Support ($190-$19F)

16 bytes of single-character codes for supported peripherals, space-padded:

| Code | Device |
|------|--------|
| J | 3-button joypad |
| 6 | 6-button joypad |
| 0 | Master System joypad |
| A | Analog joystick |
| 4 | Team Player (multitap) |
| G | Light gun |
| L | Activator |
| M | Mouse |
| B | Trackball |
| T | Tablet |
| V | Paddle controller |
| K | Keyboard / keypad |
| R | RS-232 serial / modem |
| C | CD-ROM (Mega CD) |

Most cartridge games specify `"J"` or `"J6"`.

#### 7.3.7 ROM and RAM Address Ranges ($1A0-$1AF)

- $1A0-$1A3: ROM start (always $00000000)
- $1A4-$1A7: ROM end (ROM size - 1, e.g., $001FFFFF for 2 MB)
- $1A8-$1AB: RAM start (always $00FF0000)
- $1AC-$1AF: RAM end (always $00FFFFFF)

#### 7.3.8 External Memory / SRAM ($1B0-$1BB)

12 bytes describing battery-backed SRAM or EEPROM. Filled with spaces if no
external memory is present.

| Offset | Size | Description |
|--------|------|-------------|
| $1B0-$1B1 | 2 bytes | Signature: ASCII `"RA"` |
| $1B2 | 1 byte | Type/flags byte |
| $1B3 | 1 byte | Second byte ($20 for SRAM, $40 for EEPROM) |
| $1B4-$1B7 | 4 bytes | Start address (big-endian) |
| $1B8-$1BB | 4 bytes | End address (big-endian) |

Type byte ($1B2) bit layout:

| Bits | Meaning |
|------|---------|
| 7 | Always 1 |
| 6 | 1 = non-volatile (battery-backed), 0 = volatile |
| 5 | Always 1 |
| 4-3 | Address mode: 00 = word-wide, 10 = even addresses, 11 = odd addresses |
| 2-0 | Always 0 |

Common type byte values: $F8 = non-volatile, odd byte access (most common for
battery-backed SRAM).

#### 7.3.9 Region Codes ($1F0-$1FF)

16 bytes; first 3 bytes contain active codes, remainder space-padded.

**Character format** (used by most games):

| Code | Region | Video |
|------|--------|-------|
| J | Japan / Asia | NTSC (60 Hz) |
| U | Americas | NTSC (60 Hz) |
| E | Europe | PAL (50 Hz) |

Multi-region ROMs combine codes: `"JUE"` for worldwide, `"JU "` for Japan +
Americas.

**Hex digit format** (some later games): A single hex character where each bit
represents a region (bit 0 = Japan NTSC, bit 2 = Americas NTSC, bit 3 = Europe
PAL). Example: `"F"` = all regions.

---

## 8. SRAM / Battery Backup

### 8.1 SRAM Control Register ($A130F1)

| Bit | Name | Description |
|-----|------|-------------|
| 7-2 | -- | Reserved |
| 1 | W | Write enable: 1 = SRAM is writable, 0 = read-only |
| 0 | E | SRAM enable: 1 = SRAM visible, 0 = ROM visible |

Combined states:

| Value | Effect |
|-------|--------|
| $00 | SRAM hidden; ROM visible at $200000-$3FFFFF |
| $01 | SRAM visible, read-only |
| $03 | SRAM visible, read-write |

### 8.2 Address Mapping

When SRAM is enabled, it appears in the $200000-$3FFFFF range, overlaying the
ROM that would otherwise be visible at those addresses. The exact sub-range is
defined by the start and end addresses in the ROM header ($1B4-$1BB).

Most games with battery-backed SRAM use odd-byte addressing in the range
$200001-$20FFFF (32 KB effective, appearing at odd addresses only).

### 8.3 Typical Access Pattern

1. Write $01 to $A130F1 (enable SRAM, read-only) or $03 (read-write)
2. Read/write SRAM at header-specified address range
3. Write $00 to $A130F1 (disable SRAM, restore ROM visibility)

---

## 9. TMSS (Trademark Security System)

### 9.1 Background

Introduced on Model 1 VA6 boards (late 1991) and present on all subsequent
hardware. The version register ($A10001) bits 3-0 are non-zero on TMSS-equipped
consoles.

### 9.2 Boot ROM Behavior

TMSS hardware includes a small boot ROM in the I/O controller. At power-on:

1. The TMSS ROM is mapped at $000000 (replacing the cartridge)
2. It reads $100 from the cartridge and checks for `"SEGA"` or `" SEGA"`
3. If found, it displays the trademark splash screen (~2.5 seconds)
4. It switches the cartridge back onto the bus and transfers control to the game
5. If not found, the console displays an error screen

### 9.3 VDP Unlock Register ($A14000)

Games must write the ASCII longword `"SEGA"` ($53454741) to $A14000 before
accessing the VDP. Without this write, VDP access locks the system.

### 9.4 Bus Select Register ($A14101)

| Bit 0 | Effect |
|-------|--------|
| 0 | TMSS ROM on the bus |
| 1 | Cartridge ROM on the bus |

Used by the TMSS boot ROM during startup. Games do not normally write this
register.

### 9.5 Standard Initialization Code

```
    move.b  ($A10001), d0       ; Read version register
    andi.b  #$0F, d0            ; Isolate hardware version
    beq.s   skip_tmss           ; If version 0, skip TMSS write
    move.l  #$53454741, ($A14000) ; Write "SEGA" to unlock VDP
skip_tmss:
```

---

## 10. Region and Timing

### 10.1 Master Oscillator

| Region | Master Clock | Derivation |
|--------|-------------|------------|
| NTSC | 53.693175 MHz | NTSC color subcarrier x 15 (3.579545 MHz x 15) |
| PAL | 53.203424 MHz | PAL color subcarrier x 15 (3.546895 MHz x 15) |

### 10.2 Clock Division

| Component | Divisor | NTSC Frequency | PAL Frequency |
|-----------|---------|----------------|---------------|
| 68000 CPU | /7 | 7.670454 MHz | 7.600489 MHz |
| Z80 CPU | /15 | 3.579545 MHz | 3.546895 MHz |
| YM2612 FM | /7 | 7.670454 MHz | 7.600489 MHz |
| SN76489 PSG | /15 | 3.579545 MHz | 3.546895 MHz |
| VDP pixel clock (H40) | /8 | ~6.712 MHz | ~6.650 MHz |
| VDP pixel clock (H32) | /10 | ~5.369 MHz | ~5.320 MHz |

The Z80 clock equals the NTSC color subcarrier frequency. The 68K clock is
master / 7 because the MC68000P8 is rated for 8 MHz maximum, and dividing by 7
stays under that limit.

### 10.3 Scanline and Frame Timing

| Parameter | NTSC | PAL |
|-----------|------|-----|
| Total scanlines per frame | 262 | 313 |
| Active display lines | 224 (or 240 in V30 mode) | 224 (or 240 in V30 mode) |
| VBlank lines | 38 | 89 |
| Frame rate | ~59.923 Hz | ~49.701 Hz |
| Master clocks per scanline | 3420 | 3420 |
| 68K cycles per scanline | ~488.57 (3420 / 7) | ~488.57 (3420 / 7) |
| Z80 cycles per scanline | 228 (3420 / 15) | 228 (3420 / 15) |

Each scanline is exactly 3420 master clock cycles in both regions. The frame
rate difference comes solely from the different number of scanlines per frame.

### 10.4 Region Detection

The version register ($A10001) provides hardware region identification:
- Bit 7 (MODE): 0 = domestic (Japan), 1 = overseas
- Bit 6 (VMOD): 0 = NTSC, 1 = PAL

Software-side region detection uses the ROM header region codes at $1F0-$1FF
(see Section 7.3.9). Multi-region ROMs prefer NTSC when both NTSC and PAL
regions are specified.

---

## 11. Boot Sequence

### 11.1 Hardware Reset

1. 68000 reads initial SSP from ROM $000-$003
2. 68000 reads initial PC from ROM $004-$007
3. Execution begins at the initial PC address

### 11.2 Typical Software Initialization

1. Set status register to $2700 (supervisor mode, all interrupts masked)
2. Read version register at $A10001, check bits 3-0
3. If TMSS present (version != 0): write $53454741 to $A14000
4. Request Z80 bus: write $0100 to $A11100
5. Assert Z80 reset: write $0000 to $A11200
6. Wait for Z80 bus grant (poll $A11100)
7. Load Z80 sound driver to $A00000-$A01FFF (if applicable)
8. Initialize VDP registers via control port $C00004
9. Clear 68K work RAM ($FF0000-$FFFFFF)
10. Clear VRAM, CRAM, VSRAM via VDP writes
11. Configure controller ports: write $40 to $A10009 and $A1000B
12. Deassert Z80 reset: write $0100 to $A11200
13. Release Z80 bus: write $0000 to $A11100
14. Enable interrupts by lowering SR mask
15. Enter game main loop

---

## 12. Hardware Revisions

For audio-related hardware differences (YM2612 vs YM3438 variants, analog
mixing circuits, low-pass filter characteristics), see `ym2612_reference.md`
Appendix B-C and `genesis_sound_integration.md` Section 9.

### 12.1 Model 1 (1988-1993)

| Revision | Key Characteristics |
|----------|-------------------|
| VA0 | Discrete 68K, Z80, YM2612, VDP, I/O, bus arbiter. Daughterboard for EDCLK fix. |
| VA1 | Daughterboard consolidated into 315-5339. |
| VA2 | 315-5345 EDCLK generator. First North American boards. |
| VA3 | Bus arbiter + EDCLK combined into 315-5364. Generally considered best audio quality. |
| VA4 | Nearly identical to VA3; video encoder repositioned. |
| VA5 | I/O + bus arbiter combined into 315-5402 gate array. |
| VA6 | **Introduced TMSS** via 315-5433 I/O chip. Last revision with EXT port in most markets. |
| VA6.5 | Same as VA6, EXT port removed. |
| VA6.8 | Surface-mount video encoder. Final PAL Model 1. |
| VA7 | **First FC1004 ASIC** (VDP + I/O + YM3438 integrated). Most significant Model 1 revision. |

### 12.2 Model 2 (1993-1998)

| Revision | Key Characteristics |
|----------|-------------------|
| VA0 | Cost-reduced redesign. FC1004 ASIC. |
| VA1 | Consolidated RAM chips. |
| VA1.8 | Minor VA1 variant. |
| VA2 | **Uses discrete YM2612** (Toshiba 315-5786 ASIC lacks YM3438). |
| VA2.3 | Revised VA2 layout. |
| VA3 | FC1004 variant with 315-5684 amplifier. |
| VA4 | **First GOAC (315-5960)**: 68K + Z80 + VDP + YM3438 + I/O + Z80 RAM in single chip. |

Model 2 removed: headphone jack, EXT port.

### 12.3 Model 3 / Genesis 3 (1998-1999, Majesco)

| Revision | Key Characteristics |
|----------|-------------------|
| VA1 | Uses same 315-5960 GOAC as Model 2 VA4. No expansion port. |
| VA2 | 315-6123 GOAC. Combined work RAM + VRAM into single 128 KB SDRAM. Fixed TAS instruction (breaks Gargoyles). |

### 12.4 Sega Nomad (1995)

Portable Genesis using 315-5700 (FF1004) ASIC variant. Functionally identical
to Model 2 with FC1004.

### 12.5 ASIC Summary

| Part Number | Components Integrated | Found In |
|-------------|----------------------|----------|
| 315-5313 (YM7101) | VDP + SN76489 PSG | Model 1 VA0-VA6 |
| 315-5309 | I/O controller | Model 1 VA0-VA4 |
| 315-5308 | Bus arbiter | Model 1 VA0-VA4 |
| 315-5433 | I/O + bus arbiter + TMSS | Model 1 VA6-VA6.8 |
| 315-5487 (FC1004) | VDP + I/O + YM3438 | Model 1 VA7, Model 2 VA0-VA1 |
| 315-5660 (FC1004) | VDP + I/O + YM3438 | Model 2 VA1.8/VA3 |
| 315-5700 (FF1004) | VDP + I/O + YM3438 | Nomad |
| 315-5786 (Toshiba) | VDP + I/O (no YM3438) | Model 2 VA2 |
| 315-5960 (FJ3002 GOAC) | 68K + Z80 + VDP + YM3438 + I/O + Z80 RAM | Model 2 VA4, Model 3 VA1 |
| 315-6123 (FQ8007 GOAC) | Same + unified RAM | Model 3 VA2 |

### 12.6 Emulation-Relevant Differences

1. **YM2612 vs YM3438**: The original YM2612 has a DAC ladder effect distortion
   producing a characteristic warm sound. The YM3438 in ASICs has a cleaner DAC.
   See `ym2612_reference.md` for details.

2. **TMSS**: Version register bits 0-3 = 0 means no TMSS. Non-zero requires
   $A14000 write before VDP access.

3. **TAS instruction**: The 315-6123 GOAC (Model 3 VA2) fixed TAS to properly
   assert /UDS and /LDS, which broke Gargoyles (which relied on the original
   bug).

4. **VDP timing**: Toshiba ASICs (315-5786) had issues with shadow/highlight
   mode and certain raster effects.

---

## Sources

- [Sega Genesis Technical Overview v1.00 (Sega, 1991)](https://segaretro.org/images/1/18/GenesisTechnicalOverview.pdf) - Official Sega hardware documentation
- [Charles MacDonald - Genesis Hardware Notes (gen-hw.txt)](https://gendev.spritesmind.net/mirrors/cmd/gen-hw.txt) - Comprehensive hardware reference including controller protocol
- [Plutiedev - Controllers](https://plutiedev.com/controllers) - Controller reading protocol and 6-button state machine
- [Plutiedev - I/O Ports](https://plutiedev.com/io-ports) - I/O register details and serial communication
- [Plutiedev - ROM Header Reference](https://plutiedev.com/rom-header) - ROM header field-by-field breakdown
- [Plutiedev - Saving with SRAM](https://plutiedev.com/saving-sram) - SRAM control register and memory mapping
- [Plutiedev - Using the Z80](https://plutiedev.com/using-the-z80) - Z80 bus arbitration and bank register
- [Plutiedev - Kabuto Hardware Notes](https://plutiedev.com/mirror/kabuto-hardware-notes) - Detailed hardware measurements
- [MegaDrive Development Wiki - IO Registers](https://wiki.megadrive.org/index.php?title=IO_Registers) - I/O register map
- [MegaDrive Development Wiki - MD Rom Header](https://wiki.megadrive.org/index.php?title=MD_Rom_Header) - ROM header specification
- [MegaDrive Development Wiki - TMSS](https://wiki.megadrive.org/index.php?title=TMSS) - TMSS register details
- [Sega Retro - TradeMark Security System](https://segaretro.org/TMSS) - TMSS hardware history
- [Copetti - Mega Drive / Genesis Architecture](https://www.copetti.org/writings/consoles/mega-drive-genesis/) - System architecture overview
- [ConsoleMods Wiki - Genesis ASIC Information](https://consolemods.org/wiki/Genesis:ASIC_Information) - ASIC part numbers and integration details
- [ConsoleMods Wiki - Genesis Motherboard Differences](https://consolemods.org/wiki/Genesis:Motherboard_Differences) - Board revision differences
- [Hugues Johnson - 6 Button Controllers](https://huguesjohnson.com/programming/genesis/6button/) - 6-button controller programming guide
- [Jon Thysell - How To Read Sega Controllers](https://github.com/jonthysell/SegaController/wiki/How-To-Read-Sega-Controllers) - Controller hardware protocol
- [Bumbershoot Software - Sega Genesis Startup Code](https://bumbershootsoft.wordpress.com/2018/03/02/the-sega-genesis-startup-code/) - Boot sequence and TMSS initialization
- [Nicole Express - Trademarks and Region Locks](https://nicole.express/2020/trademarks-and-region-locks.html) - TMSS validation mechanism
- [SpritesMind Forums - Hardware Research](https://gendev.spritesmind.net/forum/) - Community hardware measurements and analysis
- [Jabberwocky - Emulating the Sega Genesis](https://jabberwocky.ca/posts/2022-01-emulating_the_sega_genesis_part1.html) - Memory map and bus architecture reference
