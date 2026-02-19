# emmd

A Sega Genesis (Mega Drive) emulator written in Go.

## Status

Early development (v0.1.0). The focus is on core emulation accuracy for
officially licensed and released games. The UI is minimal - enough to load
a ROM and play with keyboard or gamepad.

## Building

Requires Go 1.25+.

```
go build ./...
```

## Usage

```
emmd -rom <path-to-rom> [-region auto|ntsc|pal] [-6button=true|false]
```

Region defaults to `auto` (NTSC). The 6-button controller is enabled by
default; use `-6button=false` to force 3-button mode for games that have
compatibility issues. SRAM saves are written automatically to a `.srm`
file alongside the ROM on exit.

### Controls

| Action    | Keyboard (primary) | Keyboard (alt) | Gamepad              |
|-----------|--------------------|----------------|----------------------|
| Up        | W                  | Arrow Up       | D-pad / Left Stick   |
| Down      | S                  | Arrow Down     | D-pad / Left Stick   |
| Left      | A                  | Arrow Left     | D-pad / Left Stick   |
| Right     | D                  | Arrow Right    | D-pad / Left Stick   |
| Button A  | J                  |                | A / Cross            |
| Button B  | K                  |                | B / Circle           |
| Button C  | L                  |                | X / Square           |
| Button X  | U                  |                | LB / L1              |
| Button Y  | I                  |                | RB / R1              |
| Button Z  | O                  |                | Y / Triangle         |
| Mode      | P                  |                | Select / Back        |
| Start     | Enter              |                | Start                |

## Emulated Hardware

### CPUs

- **Motorola 68000** - main CPU (7.67 MHz NTSC, 7.60 MHz PAL)
- **Zilog Z80** - sound CPU (3.58 MHz NTSC, 3.55 MHz PAL)

### Video

- **VDP (315-5313)** - background planes, sprites, window layer, DMA,
  H/V interrupts, interlace modes

### Audio System

The Genesis has two independent sound generators whose outputs are mixed
on the motherboard:

| Chip              | Type                         | Channels               | Output |
|-------------------|------------------------------|------------------------|--------|
| YM2612 (OPN2)     | FM synthesizer               | 6 FM (or 5 FM + 1 DAC)| Stereo |
| SN76489 (PSG)     | Programmable sound generator | 3 tone + 1 noise       | Mono   |

The emulator targets the **Model 1 VA3** audio path. Key characteristics:

- **YM2612 FM synthesis** - 4-operator, 8-algorithm FM with per-channel
  stereo panning, LFO (amplitude/frequency modulation), SSG-EG envelope
  modes, CSM mode (Timer A overflow key-on), DAC channel, and the
  "ladder effect" distortion present in discrete YM2612 chips
- **PSG** - embedded inside the VDP; 3 square-wave tone channels plus
  1 noise channel with configurable gain to match hardware output levels
- **Mixing** - FM stereo output and PSG mono output are summed with
  clamping to 16-bit range
- **Low-pass filter** - first-order RC filter at ~2840 Hz applied
  post-mix, matching the Model 1 VA3 motherboard rolloff
  (20 dB/decade). Filter state persists across frames for continuity
- **Output** - 48 kHz, 16-bit stereo PCM

### I/O

- Player 1 and Player 2 controllers (3-button or 6-button)
- Battery-backed SRAM save/load
- Save state serialization/deserialization (not yet exposed in UI)

## Architecture

```
main.go              Entry point, ROM loading, SRAM persistence
cli/                 Command-line runner, input polling, audio timing
bridge/ebiten/       Ebiten rendering bridge
ui/                  Audio player, emulator thread, shared framebuffer
emu/                 Core emulator (platform-independent)
  emulator.go          EmulatorBase, per-frame execution loop
  audio.go             FM+PSG mixing, low-pass filter
  ym2612*.go           YM2612 FM synthesizer
  vdp*.go              Video display processor
  mem.go               68000 bus, ROM/RAM/SRAM mapping
  z80mem.go            Z80 memory and bank switching
  io.go                I/O ports and controller reads
  region.go            NTSC/PAL timing
  rom.go               ROM header parsing, checksum validation
  serialize*.go        Save state serialization/deserialization
  version.go           Application name and version constants
```

## Testing

```
go test ./emu/ -count=1
```

The test suite includes unit tests and golden tests for the VDP, YM2612,
PSG, audio mixing, low-pass filter, I/O controllers, memory bus, ROM
parsing, and save state serialization.

## Dependencies

- [ebiten](https://github.com/hajimehoshi/ebiten) - rendering and input
- [oto](https://github.com/ebitengine/oto) - audio output
- [go-chip-m68k](https://github.com/user-none/go-chip-m68k) - 68000 CPU
- [go-chip-z80](https://github.com/user-none/go-chip-z80) - Z80 CPU
- [go-chip-sn76489](https://github.com/user-none/go-chip-sn76489) - PSG

## License

See LICENSE file.
