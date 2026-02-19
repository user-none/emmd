# Sega Genesis Sound System Integration

Technical reference for how the Genesis integrates its two sound generators,
routes CPU control to them, and mixes their outputs into a final stereo signal.

For details on the YM2612 FM synthesizer internals (operators, algorithms,
envelope generator, etc.), see `ym2612_reference.md`.

---

## 1. System Audio Architecture

The Sega Genesis produces audio from two independent sound generators:

| Chip | Type | Channels | Output | Location |
|------|------|----------|--------|----------|
| YM2612 (OPN2) | FM synthesizer | 6 FM (or 5 FM + 1 DAC) | Stereo | Discrete IC (or integrated into ASIC on later boards) |
| SN76489 (PSG) | Programmable sound generator | 3 tone + 1 noise | Mono | Embedded inside the VDP (315-5313) |

Both chips operate continuously once initialized. Their analog outputs are
mixed on the motherboard through passive and active circuitry before reaching
the audio output jacks. Neither chip is aware of the other.

### Signal Flow

```
               +----------+
   68000 ----->|  YM2612   |---> FM Left ----+
   Z80 ------->|  (FM)     |---> FM Right ---+---> Mixing ---> Low-Pass
               +----------+                 |    Circuit      Filter
                                             |
               +----------+                 |
   68000 ----->|  SN76489  |---> PSG Mono ---+
   Z80 ------->|  (PSG)    |
               +----------+
                (inside VDP)
```

---

## 2. CPU Control

### 2.1 Dual-CPU Design

The Genesis has two CPUs that share responsibility for sound:

- **Motorola 68000** (main CPU): Runs game logic. Can directly program both
  sound chips. Clocked at ~7.67 MHz (NTSC) or ~7.60 MHz (PAL).
- **Zilog Z80** (sound CPU): Dedicated to audio processing in most games.
  Has direct access to the YM2612 and PSG. Clocked at ~3.58 MHz (NTSC) or
  ~3.55 MHz (PAL).

Either CPU can write to either sound chip. In practice, the typical division
of labor is:

| Approach | 68000 Role | Z80 Role | Used By |
|----------|-----------|----------|---------|
| Z80 sound driver | Sends high-level commands to Z80 via shared RAM | Runs sound driver, programs YM2612 and PSG directly | Most games (SMPS, GEMS, etc.) |
| 68000 direct | Programs both chips directly | Unused or minimal | Some early titles, homebrew |
| Hybrid | Programs some registers directly, delegates others | Handles time-critical updates (DAC streaming, etc.) | Some advanced titles |

### 2.2 Z80 as Sound Processor

The Z80 is the designated sound CPU. It has its own 8KB of RAM for sound
driver code and data. A typical boot sequence is:

1. 68000 asserts Z80 reset (`$A11200` = `$0000`)
2. 68000 requests Z80 bus (`$A11100` = `$0100`)
3. 68000 uploads sound driver code to Z80 RAM (`$A00000`-`$A01FFF`)
4. 68000 releases Z80 bus (`$A11100` = `$0000`)
5. 68000 deasserts Z80 reset (`$A11200` = `$0100`)
6. Z80 begins executing from address `$0000`

Once running, the Z80 typically:
- Reads command bytes from a shared mailbox region in Z80 RAM
- Interprets commands (play note, stop channel, change instrument, etc.)
- Programs YM2612 registers and PSG registers accordingly
- Manages PCM/DAC sample playback by writing sample bytes to YM2612 register
  `$2A` at the appropriate rate

### 2.3 Bus Arbitration

The 68000 and Z80 share the system bus, requiring arbitration:

| Register | Address | Function |
|----------|---------|----------|
| Z80 bus request | `$A11100` | 68000 writes bit 8 = 1 to request Z80 bus; reads bit 8 = 0 when granted |
| Z80 reset | `$A11200` | 68000 writes bit 8 = 0 to assert reset, bit 8 = 1 to deassert |

When the 68000 holds the Z80 bus, the Z80 is frozen and the 68000 can
read/write Z80 RAM and YM2612 ports directly through the `$A00000`-`$A0FFFF`
window. The Z80 cannot execute while the bus is held.

The Z80 can also access 68000 address space through a bank window
(`$8000`-`$FFFF` in Z80 space), configured via the bank register at `$6000`.
This allows the Z80 to read ROM data (instrument definitions, sample data)
without 68000 intervention.

### 2.4 Z80 Interrupt

The Z80 receives a single maskable interrupt (INT) tied to the VDP's vertical
blank output. This fires once per frame (60 Hz NTSC, 50 Hz PAL) and is
commonly used by sound drivers as a timing reference for music tempo.

The interrupt is active-low and stays asserted until the Z80 acknowledges it
(IFF1 transitions from enabled to disabled during the interrupt handler entry).

---

## 3. Memory-Mapped I/O

### 3.1 68000 Address Map (Sound-Related)

| Address Range | Function |
|---------------|----------|
| `$A00000`-`$A01FFF` | Z80 RAM (8KB, requires bus grant) |
| `$A02000`-`$A03FFF` | Z80 RAM mirror |
| `$A04000`-`$A04003` | YM2612 ports (requires bus grant) |
| `$A04004`-`$A05FFF` | YM2612 port mirrors |
| `$A11100`-`$A11101` | Z80 bus request/grant |
| `$A11200`-`$A11201` | Z80 reset control |
| `$C00011` | PSG write port (directly, no bus grant needed) |

The 68000 accesses the YM2612 through the Z80 address space window. This means
the 68000 must hold the Z80 bus (`$A11100` bit 8 = 1) before writing to YM2612
registers at `$A04000`-`$A04003`.

The PSG, being embedded in the VDP, is accessed through the VDP address space
at `$C00011`. This does not require Z80 bus arbitration.

### 3.2 Z80 Address Map (Sound-Related)

| Address Range | Function |
|---------------|----------|
| `$0000`-`$1FFF` | Z80 RAM (8KB) |
| `$2000`-`$3FFF` | Z80 RAM mirror |
| `$4000`-`$5FFF` | YM2612 ports (directly mapped) |
| `$6000` | Bank register (bit-serial, sets `$8000`-`$FFFF` window) |
| `$7F00`-`$7FFF` | VDP ports (PSG write at `$7F11`) |
| `$8000`-`$FFFF` | 68000 bank window (32KB, configured by bank register) |

The Z80 has direct, low-latency access to the YM2612 without any bus
arbitration. This is one of the primary reasons the Z80 serves as the
sound CPU.

### 3.3 YM2612 Port Layout

| Z80 Address | 68000 Address | Port | Function |
|-------------|---------------|------|----------|
| `$4000` | `$A04000` | 0 | Address latch, Part I (channels 1-3 + global) |
| `$4001` | `$A04001` | 1 | Data write, Part I |
| `$4002` | `$A04002` | 2 | Address latch, Part II (channels 4-6) |
| `$4003` | `$A04003` | 3 | Data write, Part II |

To program a YM2612 register:
1. Write the register address to port 0 (Part I) or port 2 (Part II)
2. Write the data value to port 1 or port 3

Word writes from the 68000 can combine both steps: the high byte is written
to the address port and the low byte to the data port in a single bus
operation.

### 3.4 PSG Write Protocol

The PSG uses a single write-only data port. Each byte encodes either a
latch/data command or a data-only command:

| Bit 7 | Type | Bits 6-4 | Bits 3-0 |
|-------|------|----------|----------|
| 1 | Latch + data | `RR C` (register, channel) | Data (low 4 bits) |
| 0 | Data only | - | Data (6 bits, to last latched register) |

Where `RR` selects tone/volume and `C` selects which of the 4 channels.

---

## 4. Clock Derivation and Timing

### 4.1 Master Clock

Both CPUs and both sound chips derive their clocks from a single master
oscillator:

| | NTSC | PAL |
|---|------|-----|
| Master oscillator | 53.693175 MHz | 53.203424 MHz |
| 68000 clock (master / 7) | 7,670,454 Hz | 7,600,489 Hz |
| Z80 clock (master / 15) | 3,579,545 Hz | 3,546,893 Hz |

### 4.2 YM2612 Timing

The YM2612 is clocked by the 68000 clock and applies two levels of internal
division:

```
M68K clock (7.67 MHz) --> /6 internal divider --> /24 per-sample cycle
                          (~1.28 MHz)              (~53,267 Hz native sample rate)
```

Combined divider: 144 M68K cycles per FM sample.

- **NTSC:** 7,670,454 / 144 = 53,267 Hz native output rate
- **PAL:** 7,600,489 / 144 = 52,781 Hz native output rate

Each FM sample, all 6 channels are evaluated. The envelope generator updates
at 1/3 the sample rate (every 3 FM samples = every 432 M68K cycles).

### 4.3 PSG Timing

The SN76489 PSG is clocked by the Z80 clock:

```
Z80 clock (3.58 MHz) --> /16 internal divider --> tone counter input
                          (~223,721 Hz)
```

The tone counter divides further by the programmed period value to produce
the square wave output. The noise channel uses a similar mechanism with an
LFSR (linear feedback shift register) for pseudo-random output.

### 4.4 Samples Per Frame

At 48 kHz output sample rate:

| | NTSC (60 Hz) | PAL (50 Hz) |
|---|------|-----|
| Samples per frame | 800 | 960 |
| Scanlines per frame | 262 | 313 |
| Samples per scanline | ~3.05 | ~3.07 |

Both chips generate audio in lockstep with scanline processing. Each
scanline, the YM2612 is advanced by the number of M68K cycles in that
scanline, and the PSG is advanced by the number of Z80 cycles.

---

## 5. Output Characteristics

### 5.1 YM2612 Output

- **Format:** Stereo (independent left/right per channel)
- **Internal precision:** 14-bit signed per operator
- **DAC resolution:** 9-bit (lowest 5 bits truncated before analog output)
- **Panning:** Per-channel, binary on/off for left and right (register `$B4+`)
- **Channel 6 DAC mode:** Replaces FM output with 8-bit unsigned PCM
  (register `$2A`, enabled by register `$2B` bit 7)

The YM2612 uses time-division multiplexing on real hardware: each channel's
9-bit sample is output sequentially through the DAC, and the external
low-pass filter smooths them into a combined waveform. Emulators mix channels
digitally instead.

### 5.2 PSG Output

- **Format:** Mono (single output pin)
- **Channels:** 3 square-wave tone generators + 1 noise generator
- **Volume:** 4-bit per channel (16 levels, roughly 2 dB per step)
- **Output polarity:** Unipolar on real hardware (0 or +volume, not +/- volume)

The PSG is embedded in the VDP, not the YM2612. Its output pin is separate
from the FM outputs and connects to the mixing circuit independently.

### 5.3 Volume Levels

The raw PSG output is significantly louder than the raw YM2612 output,
approximately 27 dB higher. The Genesis motherboard uses attenuation
resistors (typically 51K ohm on Model 1) to reduce the PSG level before
mixing with FM.

The relative FM/PSG balance varies by board revision. The Model 1 VA3-VA6.8
is considered the reference target, having well-tuned balance. Some later
Model 2 revisions have notably poor balance due to wiring errors or
component choices (see Section 7).

---

## 6. Mixing

### 6.1 Hardware Mixing

On real hardware, mixing is entirely analog:

1. The YM2612 outputs a multiplexed stereo signal (left and right channels
   alternating through the DAC)
2. The PSG outputs a mono signal from the VDP
3. Both signals pass through the motherboard's mixing circuit
4. The PSG mono signal is mixed into both the left and right FM channels
5. The combined signal passes through a low-pass filter
6. The filtered signal goes to an amplifier IC and then to the output jacks

The mixing circuit topology varies by board revision but the principle is the
same: PSG mono is summed into FM stereo with appropriate attenuation.

### 6.2 Low-Pass Filter

The Genesis motherboard includes a low-pass filter that applies to both FM
and PSG outputs. This is part of the console's analog output stage, not
part of either sound chip.

| Board Revision | Filter Type | Approximate Cutoff | Roll-off |
|----------------|------------|-------------------|----------|
| Model 1 VA0-VA2 | 1st-order RC | ~3.39 kHz | 20 dB/decade |
| Model 1 VA3-VA6.8 | 1st-order RC | ~2.84 kHz | 20 dB/decade |
| Model 2 VA0-VA1.8 | 2nd-order Sallen-Key | Aggressive (high Q) | 40 dB/decade |
| Model 2 VA3-VA4 | 2nd-order (315-5684 IC) | ~2.84 kHz equivalent | 40 dB/decade |

Many games were designed with this filter in mind. High-frequency FM timbres
and the noise channel character are shaped significantly by the filter.

### 6.3 DAC/PCM Playback

Channel 6 of the YM2612 can be switched to DAC mode, replacing its FM output
with direct 8-bit PCM sample playback. This is the primary mechanism for
digitized audio (drums, voice samples, sound effects) in Genesis games.

The Z80 sound driver typically handles DAC playback by:
1. Reading PCM sample data from ROM via the bank window
2. Writing sample bytes to YM2612 register `$2A` at a fixed rate
3. Timing writes to achieve the desired playback sample rate

Common PCM playback rates range from ~4 kHz to ~26 kHz depending on the
sound driver and desired quality. Higher rates consume more Z80 CPU time,
leaving less for FM and PSG music processing.

The DAC sample is an unsigned 8-bit value where `$80` (128) represents
silence. The YM2612 converts this to signed by subtracting 128 and then
scales it to match the FM output range.

---

## 7. Hardware Variations Affecting Audio

### 7.1 FM Chip Revisions

The FM synthesizer went through several revisions:

| Chip | Process | Ladder Effect | Status Port Behavior |
|------|---------|--------------|---------------------|
| YM2612 | NMOS | Full (audible distortion at zero crossing) | `$4001`-`$4003` return decaying last-read value |
| YM3438 | CMOS | Reduced | `$4001`-`$4003` mirror `$4000` (current status) |
| ASIC (FC1004/GOAC) | CMOS integrated | Minimal/none | Mirrors `$4000` |

The ladder effect is a DAC nonlinearity in the original YM2612 where the
voltage gap between output values -1 and 0 is disproportionately large.
This amplifies low-volume signals and adds audible grain during quiet
passages. Many game soundtracks were composed with this distortion in mind.

### 7.2 Board-Level Audio Differences

The analog circuitry surrounding the sound chips varies significantly across
board revisions and dominates the perceived audio character:

**Model 1:**
- VA0-VA2: Overdriven preamp (Sony CXA1034P) causes clipping on loud content
- VA3-VA6.8: Corrected preamp gain; considered the gold standard for Genesis audio
- VA7: ASIC FM chip with poor surrounding op-amp choices; worst-sounding Model 1

**Model 2:**
- VA0-VA1.8: LM324 op-amp with aggressive Sallen-Key filter; muffled and distorted
- VA2-VA2.3: Discrete YM2612 with good FM clarity but PSG wiring error causes
  PSG to be out of tune and distorted
- VA3: Best-sounding Model 2; near-perfect FM/PSG balance
- VA4: GOAC chip; similar quality to VA3

### 7.3 Emulation Reference Target

The recommended reference for emulation is the **Model 1 VA3-VA6.8**:
- Discrete YM2612 with full ladder effect
- First-order low-pass filter at ~2.84 kHz
- Correct FM/PSG balance
- Considered the intended sound for the commercial Genesis library

---

## 8. Common Sound Drivers

Games do not program the sound chips directly from game code. Instead, they
use sound driver software that runs on the Z80 (or sometimes the 68000).
The most common sound drivers are:

| Driver | Developer | Notable Games |
|--------|-----------|---------------|
| SMPS (Sample Music Playback System) | Sega | Sonic series, most first-party Sega titles |
| GEMS (Genesis Editor for Music and Sound) | Recreational Brainware / Sega | Western third-party titles (Aladdin, Earthworm Jim, etc.) |
| TFM Music Maker / custom | Various | Many Japanese third-party titles |
| MUCOM88 | Yuzo Koshiro | Streets of Rage series, Bare Knuckle |
| Echo | Sik | Homebrew and some commercial titles |

Sound drivers manage:
- Music sequencing (note timing, tempo, patterns)
- Instrument voice programming (operator parameters, algorithm selection)
- PSG tone and noise control
- DAC sample playback scheduling
- Sound effect priority and channel allocation
- Communication with the 68000 via shared Z80 RAM

### 8.1 68000-to-Z80 Communication

The standard communication pattern between the two CPUs for sound:

1. 68000 writes a command byte to a known location in Z80 RAM (e.g., `$A01C00`)
2. Z80 checks this location during its main loop or interrupt handler
3. Z80 processes the command (play music track, play SFX, stop, etc.)
4. Z80 clears the command byte to signal completion

This requires the 68000 to request the Z80 bus, write the command, then
release the bus. The Z80 is briefly paused during this exchange.

---

## 9. Timing and Synchronization

### 9.1 Per-Scanline Execution

The Genesis processes audio on a per-scanline basis. Each scanline:

1. 68000 executes ~488 cycles (NTSC) or ~486 cycles (PAL)
2. Z80 executes ~228 cycles (NTSC) or ~226 cycles (PAL) (when not bus-held)
3. YM2612 advances by the scanline's M68K cycle count, generating FM samples
   as needed (roughly 3 FM native samples per scanline)
4. PSG advances by the scanline's Z80 cycle count, generating PSG samples
   as needed

### 9.2 Frame Boundary

At the end of each frame (after all scanlines):

1. YM2612 buffer contains ~800 stereo sample pairs (NTSC at 48 kHz output)
2. PSG buffer contains ~800 mono samples (NTSC at 48 kHz output)
3. The buffers are mixed: each mono PSG sample is added to both the left and
   right FM samples
4. The mixed stereo buffer is delivered to the audio output system

### 9.3 Resampling

The YM2612 generates audio at its native rate (~53 kHz) which must be
converted to the output sample rate (typically 48 kHz). This is a
downsampling operation. A simple approach is Bresenham-style decimation:
accumulate the output rate, emit a sample when the accumulator exceeds the
native rate, then subtract the native rate.

The PSG library handles its own resampling from the Z80 clock-derived
native rate to the output sample rate internally.

---

## 10. Initialization

### 10.1 Power-On Defaults

At power-on or reset, the sound system should be initialized to:

| Setting | Default |
|---------|---------|
| YM2612 panning (all channels) | Left = on, Right = on |
| YM2612 DAC sample | `$80` (silence) |
| YM2612 DAC enable | Off (FM output on channel 6) |
| YM2612 all operators | Release state, attenuation = max (silent) |
| PSG all channels | Maximum attenuation (silent) |
| Z80 | Held in reset |

Some games depend on panning bits being set at power-on. After Burner II, for
example, does not explicitly set panning and relies on the default L+R
enabled state.

### 10.2 Typical Game Boot Sequence

1. 68000 initializes hardware (VDP, controllers, etc.)
2. 68000 asserts Z80 reset and requests Z80 bus
3. 68000 uploads sound driver to Z80 RAM
4. 68000 releases Z80 bus and deasserts reset
5. Z80 boots, initializes YM2612 registers (silence all channels, reset timers)
6. 68000 sends "play music" command to Z80 via shared RAM
7. Z80 begins sequencing music and programming both chips each frame

---

## Sources

- [Sega Genesis Technical Manual (Sega, 1989)](https://segaretro.org/Sega_Mega_Drive/Technical_specifications) - Official hardware specifications
- [Sega2 Doc v1.1 (Charles MacDonald)](https://segaretro.org/Sega_Mega_Drive/Technical_specifications) - Comprehensive Genesis hardware documentation
- [SMS Power - YM2612 Technical Manual](https://www.smspower.org/maxim/Documents/YM2612) - Detailed FM chip register reference
- [Plutiedev - YM2612 Registers](https://www.plutiedev.com/ym2612-registers) - YM2612 programming reference
- [Plutiedev - PSG (SN76489)](https://www.plutiedev.com/psg) - PSG programming reference
- [Plutiedev - Genesis Sound Overview](https://www.plutiedev.com/sound-overview) - System-level sound architecture
- [SpritesMind Forum - Hardware Research (Nemesis)](https://gendev.spritesmind.net/forum/) - Hardware measurements and analysis
- [MegaDrive Development Wiki](https://wiki.megadrive.org/) - Community-maintained hardware documentation
- [ConsoleMods Wiki - Genesis Motherboard Differences](https://consolemods.org/wiki/Genesis:Motherboard_Differences) - Board revision audio characteristics
- [ConsoleMods Wiki - Genesis Audio Chip Notes](https://consolemods.org/wiki/Genesis:Audio_Chip_Notes) - FM chip variant differences
- [VGMPF - SN76489](https://vgmpf.com/Wiki/index.php?title=SN76489) - PSG documentation and specifications
- [jsgroth - Genesis & Sega CD Audio Filtering](https://jsgroth.dev/blog/posts/genesis-sega-cd-audio-filtering/) - Low-pass filter analysis
