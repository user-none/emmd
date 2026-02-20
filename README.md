# emmd

A Sega Genesis (Mega Drive) emulator written in Go targeting Model 1 hardware.

## Status

The focus is on core emulation accuracy for officially licensed and released
games. The standalone UI provides a full game library with ROM scanning,
artwork, metadata, save state management, rewind, fast-forward, shader effects,
RetroAchievements integration, and configurable settings.

## Target Hardware

This emulator targets the **Sega Genesis Model 1** console.

- **Audio path:** Model 1 VA3 motherboard revision
  - First-order RC low-pass filter at ~2840 Hz (20 dB/decade rolloff)
  - YM2612 discrete DAC with ladder effect distortion at zero crossing
- **Video:** VDP 315-5313
- **CPUs:** Motorola 68000 + Zilog Z80

The Model 1 VA3 was chosen because it uses a discrete YM2612 FM chip (as
opposed to the integrated ASIC in later revisions) and produces the audio
characteristics most associated with the Genesis sound.

## Features

- Full Motorola 68000 and Zilog Z80 CPU emulation with per-scanline
  synchronization
- VDP rendering with background planes, sprites, window layer, DMA, H/V
  interrupts, and interlace modes
- YM2612 FM synthesis with all 6 channels, 4 operators, 8 algorithms,
  LFO modulation, SSG-EG envelopes, CSM mode, DAC channel, and ladder
  effect distortion
- SN76489 PSG with 3 tone channels and 1 noise channel
- Stereo audio output at 48 kHz, 16-bit PCM with Model 1 VA3 low-pass filter
- 3-button and 6-button controller support for 2 players
- Battery-backed SRAM save/load
- Save state serialization and deserialization
- NTSC and PAL region support with automatic detection from ROM header
- Standalone desktop application with library, settings, and shader effects
- LibRetro core for use with LibRetro-compatible frontends
- macOS application bundle with icon generation

## Building

Requires Go 1.25+.

### Standalone Application

```
make standalone
```

Produces `build/emmd`.

### LibRetro Core

```
make libretro
```

Produces `build/emmd_libretro.dylib` for use with LibRetro-compatible
frontends.

### macOS Application Bundle

```
make macos
```

Creates `build/emmd.app` with proper icon and code signing.

### All Targets

```
make all
```

Builds both standalone and libretro targets.

## Running

### Launch the full UI

```
emmd
```

### Direct (no UI)

Launch with a ROM file:

```
emmd -rom <path-to-rom> [-region auto|ntsc|pal] [-six-button=true|false]
```


**Flags:**

| Flag          | Default | Description                              |
|--------|-----|---------------------|
| `-rom`        |         | Path to ROM file (opens UI if omitted)   |
| `-region`     | `auto`  | Region: `auto`, `ntsc`, or `pal`         |
| `-six-button` | `true`  | Enable 6-button controller               |

Region defaults to `auto` which reads the ROM header region field and
prefers NTSC for multi-region ROMs. The 6-button controller is enabled by
default; use `-six-button=false` to force 3-button mode for games that have
compatibility issues with 6-button detection.

### Controls

**Keyboard:**

| Action    | Key           |
|-----------|---------------|
| Up        | W             |
| Down      | S             |
| Left      | A             |
| Right     | D             |
| Button A  | J             |
| Button B  | K             |
| Button C  | L             |
| Start     | Enter         |
| Button X  | U             |
| Button Y  | I             |
| Button Z  | O             |

**Gamepad:**

| Action    | Button             |
|-----------|--------------------|
| Up        | D-pad / Left Stick |
| Down      | D-pad / Left Stick |
| Left      | D-pad / Left Stick |
| Right     | D-pad / Left Stick |
| Button A  | Square             |
| Button B  | Cross              |
| Button C  | Circle             |
| Start     | Start              |
| Button X  | L1                 |
| Button Y  | Triangle           |
| Button Z  | R1                 |

## Emulated Hardware

### CPUs

| Chip              | Role      | NTSC Clock     | PAL Clock      |
|----------|------|--------|--------|
| Motorola 68000    | Main CPU  | 7.670454 MHz   | 7.600489 MHz   |
| Zilog Z80         | Sound CPU | 3.579545 MHz   | 3.546893 MHz   |

The 68000 runs per-scanline with budget-based execution and DMA stall
support. The Z80 runs in sync per scanline and is paused while the 68000
holds the bus. Z80 V-blank interrupts are driven by the VDP V-blank output,
independent of V-int enable, and remain asserted until acknowledged.

### Video Display Processor (VDP 315-5313)

- **Resolution:** 320x224 (NTSC H40/V28) with interlace modes up to 320x448
- **Planes:** Plane A, Plane B, Window, and up to 80 sprites per frame
- **VRAM:** 64 KB
- **CRAM:** 128 bytes (64 entries, 512-color palette)
- **VSRAM:** 80 bytes (40 vertical scroll entries)
- **DMA:** Memory-to-VRAM, fill, and copy operations with CPU stall emulation
- **Interrupts:** V-blank (level 6) and H-blank (level 4)
- **Features:** H/V counter latching, per-scanline CRAM/VSRAM updates,
  interlace mode 2 (doubled vertical resolution)

### Audio

The Genesis audio system consists of two independent sound generators mixed
on the motherboard:

| Chip              | Type                         | Channels               | Output |
|----------|---------------|------------|----|
| YM2612 (OPN2)     | FM synthesizer               | 6 FM (or 5 FM + 1 DAC)| Stereo |
| SN76489 (PSG)     | Programmable sound generator | 3 tone + 1 noise       | Mono   |

#### YM2612 FM Synthesis

- 4-operator FM synthesis with 8 algorithms per channel
- Per-channel stereo panning (left, right, or both)
- LFO with amplitude and frequency modulation
- SSG-EG envelope modes for complex waveform shaping
- CSM mode (Timer A overflow triggers key-on for channel 3 operators)
- DAC channel (channel 6) for 8-bit PCM sample playback
- Ladder effect: discrete DAC resistor-ladder distortion at zero crossing,
  characteristic of the original YM2612 chip
- Timers A and B with overflow interrupts

#### SN76489 PSG

- 3 square-wave tone channels with 10-bit period registers
- 1 noise channel with white and periodic noise modes
- Integrated inside the VDP; addressed through the VDP PSG port
- Gain-adjusted to match hardware output levels relative to the YM2612

#### Audio Pipeline

1. YM2612 generates stereo samples; PSG generates mono samples
2. FM stereo and PSG mono are summed with 16-bit clamping
3. First-order RC low-pass filter at ~2840 Hz applied post-mix (Model 1 VA3
   motherboard characteristic)
4. Output: 48 kHz, 16-bit stereo PCM
5. Filter state persists across frame boundaries for continuity

### Memory Map (68000 Bus)

| Address Range     | Size  | Description                      |
|----------|----|-----------------|
| $000000-$3FFFFF   | 4 MB  | Cartridge ROM                    |
| $200001-$20FFFF   | 32 KB | Battery-backed SRAM (if present) |
| $A00000-$A01FFF   | 8 KB  | Z80 address space                |
| $A04000-$A04003   |       | YM2612 ports                     |
| $A10000-$A1001F   |       | I/O ports and version register   |
| $C00000-$C00003   |       | VDP data port                    |
| $C00004-$C00007   |       | VDP control port                 |
| $C00011           |       | PSG port                         |
| $FF0000-$FFFFFF   | 64 KB | Main RAM (work RAM)              |

### Z80 Memory Map

| Address Range | Description                           |
|--------|--------------------|
| $0000-$1FFF   | Z80 RAM (8 KB, mirrored to $3FFF)     |
| $4000-$5FFF   | YM2612 ports                          |
| $6000-$60FF   | Bank register (68000 ROM window)      |
| $7F11          | PSG port                              |
| $8000-$FFFF   | Bank window into 68000 address space  |

### I/O

- **Controllers:** 3-button and 6-button Genesis pads with TH-based state
  machine for 6-button detection and ~1.5 ms timeout (at 7.67 MHz)
- **Players:** 2 controller ports with independent state
- **SRAM:** Battery-backed save RAM parsed from ROM header ($1B0-$1BB),
  up to 32 KB

### Region Support

| Region | Scanlines | FPS | M68K Clock   | Z80 Clock    |
|----|------|---|-------|-------|
| NTSC   | 262       | 60  | 7.670454 MHz | 3.579545 MHz |
| PAL    | 313       | 50  | 7.600489 MHz | 3.546893 MHz |

Region is auto-detected from the ROM header region field at $1F0-$1FF.
Multi-region ROMs default to NTSC. Manual override is available via the
`-region` flag.

## Architecture

```
cmd/
  standalone/          Standalone desktop entry point (Ebiten UI)
  libretro/            LibRetro core entry point (shared library)
adapter/
  adapter.go           CoreFactory: system info, emulator creation, region detection
emu/                   Core emulator (platform-independent)
  emulator.go            Main loop: per-scanline CPU sync, interrupt dispatch, audio mix
  audio.go               FM+PSG mixing and Model 1 VA3 low-pass filter
  ym2612.go              YM2612 FM synthesizer (channels, operators, algorithms)
  ym2612_operator.go     Operator phase/amplitude computation, ladder effect
  ym2612_envelope.go     ADSR envelope generator with rate scaling
  ym2612_lfo.go          Low-frequency oscillator (AM/FM modulation)
  ym2612_phase.go        Phase generator
  ym2612_timer.go        Timer A/B management
  vdp.go                 VDP core: control port, registers, DMA, interrupts
  vdp_render.go          Scanline rendering pipeline and layer compositing
  vdp_plane.go           Background plane rendering
  vdp_sprite.go          Sprite rendering
  vdp_window.go          Window layer rendering
  vdp_dma.go             DMA transfer implementation
  mem.go                 68000 bus: ROM, RAM, SRAM, I/O, VDP port mapping
  z80mem.go              Z80 memory: RAM, YM2612 ports, bank switching
  io.go                  Controller ports, version register, I/O control
  region.go              NTSC/PAL timing constants and ROM region detection
  rom.go                 ROM header parsing and checksum validation
  serialize.go           Save state serialization and deserialization
  version.go             Application name and version constants
assets/
  icon.png               Application icon
  macos_info.plist       macOS app bundle metadata
docs/                    Technical reference documentation
```

The core `emu/` package is fully platform-independent. It implements the
`eblitui/api` interfaces:

- **Emulator** - frame execution, framebuffer, audio, input
- **SaveStater** - save and load state with CRC verification
- **BatterySaver** - SRAM get/set for persistent saves
- **MemoryInspector** - read individual bytes from RAM regions
- **MemoryMapper** - enumerate and access memory regions

The `adapter/` package bridges between the core and the UI frameworks by
implementing `CoreFactory`. The `cmd/` packages are thin entry points that
wire the adapter to a specific frontend.

## Save States

Save states capture the complete emulator state including:

- 68000 and Z80 CPU registers and execution state
- 64 KB main RAM and 8 KB Z80 RAM
- SRAM contents (if present)
- Full VDP state (VRAM, CRAM, VSRAM, registers, DMA state)
- YM2612 state (all channels, operators, envelopes, timers, DAC)
- SN76489 PSG state
- I/O controller state and 6-button detection counters
- Audio filter state for seamless audio continuity

States are validated with CRC32 checksums and ROM CRC matching to prevent
loading states from different ROMs.

## Testing

```
go test ./emu/ -count=1
```

The test suite covers:

- VDP rendering, sprites, planes, window, DMA, and interlace modes
- YM2612 algorithms, envelope generation, LFO, phase, DAC, timers, CSM
  mode, and SSG-EG
- PSG tone and noise generation
- Audio mixing and low-pass filter behavior
- I/O controller reads for 3-button and 6-button pads
- Memory bus mapping and access
- ROM header parsing and checksum validation
- Save state serialization and deserialization round-trips
- Region detection from ROM headers

## Compatibility

This emulator targets officially licensed and released Genesis games.

**Not supported:**

- Unlicensed or homebrew games
- Prototype or beta ROMs
- Non-controller peripherals (light gun, mouse, multitap, etc.)

## Dependencies

- [ebiten](https://github.com/hajimehoshi/ebiten) - rendering and input
  (standalone)
- [oto](https://github.com/ebitengine/oto) - audio output (standalone)
- [go-chip-m68k](https://github.com/user-none/go-chip-m68k) - Motorola
  68000 CPU
- [go-chip-z80](https://github.com/user-none/go-chip-z80) - Zilog Z80 CPU
- [go-chip-sn76489](https://github.com/user-none/go-chip-sn76489) -
  SN76489 PSG
- [eblitui](https://github.com/user-none/eblitui) - shared emulator UI
  framework (standalone, libretro)

## License

See LICENSE file.
