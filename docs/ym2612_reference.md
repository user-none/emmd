# YM2612 (OPN2) Technical Reference

Compiled from multiple sources for emulator development reference.

---

## 1. Overview

The YM2612 (also known as OPN2) is a 6-channel FM synthesis sound chip developed by
Yamaha, used in the Sega Mega Drive/Genesis. It is a stripped-down version of the
YM2608 (OPNA).

### Key Specifications

- 6 FM channels, 4 operators per channel (24 operators total)
- 8 algorithm configurations
- 1 LFO (shared across all channels)
- Channel 6 can be used as an 8-bit DAC for PCM playback
- 2 software timers (Timer A: 10-bit, Timer B: 8-bit)
- Stereo output (per-channel L/R panning)
- 9-bit internal DAC
- SSG-EG envelope modes (undocumented/advanced)

### Clock and Timing

- Master clock: ~7.67 MHz (NTSC) or ~7.61 MHz (PAL)
  - Note: Sega documentation incorrectly states 8 MHz
- Internal divider: /6 (~1.28 MHz effective)
- FM sample output: every 24 internal cycles (~53,267 Hz NTSC)
- Envelope generators clock at 1/3 the phase generator rate (every 72 internal cycles)

---

## 2. Bus Interface

### Memory Map

The YM2612 is accessed through 4 ports:

| Z80 Address | 68000 Address | Function |
|-------------|--------------|----------|
| $4000 | $A04000 | Address port, Part I (Ch 1-3 + global) |
| $4001 | $A04001 | Data port, Part I |
| $4002 | $A04002 | Address port, Part II (Ch 4-6) |
| $4003 | $A04003 | Data port, Part II |

- Z80 addresses mirror throughout $4000-$5FFF
- To write: set address port first, then write data to data port
- The Z80 accesses the YM2612 directly; the 68000 must assert BUSREQ first

### Read Port ($4000)

| Bit | Function |
|-----|----------|
| 7 | Busy flag (processing a write, ~32 internal cycles) |
| 1 | Timer B overflow flag |
| 0 | Timer A overflow flag |

**Hardware variant differences:**
- Discrete YM2612: reads from $4001-$4003 return decaying last-read value (~250ms decay)
- YM3438: reads from $4001-$4003 mirror $4000 (always current status)

### Register Address Ranges

| Range | Type |
|-------|------|
| $20-$2F | Global registers |
| $30-$9F | Per-operator registers |
| $A0-$BF | Per-channel registers |

---

## 3. Global Registers

### $22 - LFO Control

| Bits | Field | Description |
|------|-------|-------------|
| 7-4 | - | Reserved (0) |
| 3 | LFOEN | LFO enable (1=on, 0=off and reset counter) |
| 2-0 | LFOFREQ | LFO frequency selection |

**LFO Frequencies:**

| Value | Frequency | Divider (samples between increments) |
|-------|-----------|---------------------------------------|
| 0 | 3.82 Hz | 108 |
| 1 | 5.33 Hz | 77 |
| 2 | 5.77 Hz | 71 |
| 3 | 6.11 Hz | 67 |
| 4 | 6.60 Hz | 62 |
| 5 | 9.23 Hz | 44 |
| 6 | 46.11 Hz | 8 |
| 7 | 69.22 Hz | 5 |

Disabling the LFO resets the 7-bit LFO counter to 0 and holds it there.

### $24-$25 - Timer A Frequency (10-bit)

- $24: bits 7-0 = TMRA bits 9-2
- $25: bits 1-0 = TMRA bits 1-0

Timer A overflow period: `(0x400 - TMRA) * 18.77 us`

### $26 - Timer B Frequency (8-bit)

- Bits 7-0 = TMRB

Timer B overflow period: `(0x100 - TMRB) * 300.34 us`

### $27 - Channel 3 Mode / Timer Control

| Bits | Field | Description |
|------|-------|-------------|
| 7-6 | MODE | Channel 3 operating mode |
| 5 | RST:B | Clear Timer B flag |
| 4 | RST:A | Clear Timer A flag |
| 3 | ENBL:B | Enable Timer B flag generation |
| 2 | ENBL:A | Enable Timer A flag generation |
| 1 | LOAD:B | Run Timer B |
| 0 | LOAD:A | Run Timer A |

**Channel 3 Modes:**

| Value | Mode |
|-------|------|
| 00 | Normal (all operators share frequency) |
| 01 | Special (per-operator frequencies) |
| 10 | CSM (composite sine mode / speech synthesis) |

### $28 - Key On/Off

| Bits | Field | Description |
|------|-------|-------------|
| 7 | OP4 | Operator 4 key state (1=on, 0=off) |
| 6 | OP3 | Operator 3 key state |
| 5 | OP2 | Operator 2 key state |
| 4 | OP1 | Operator 1 key state |
| 3-1 | - | Reserved (0) |
| 2-0 | CH | Channel selection |

**Channel encoding:** Ch 1-3 = 000-010, Ch 4-6 = 100-110

Key on resets the phase generator's 20-bit counter to 0 and starts Attack phase.
Key off starts Release phase.

### $2A - DAC Output

8-bit unsigned PCM sample value. When DAC is enabled, this replaces channel 6 FM
output. Converted to signed by subtracting 128 (i.e., 0x80 is silence).

### $2B - DAC Enable

| Bits | Field | Description |
|------|-------|-------------|
| 7 | DACEN | DAC enable (1=DAC output, 0=FM output on ch6) |
| 6-0 | - | Reserved (0) |

---

## 4. Per-Operator Registers

### Register Address Layout

Operator registers use a specific offset pattern within each register group.
The lowest 2 bits select the channel (0-2), and bits 2-3 select the operator
in a **swapped** order:

| Offset | Operator |
|--------|----------|
| +$00 | Slot 1 (OP1) |
| +$08 | Slot 3 (OP3) |
| +$04 | Slot 2 (OP2) |
| +$0C | Slot 4 (OP4) |

**IMPORTANT: Register slot order is S1, S3, S2, S4 - NOT sequential S1, S2, S3, S4.**

Full address mapping for each base register (e.g., $30):

| | OP1 (S1) | OP3 (S2) | OP2 (S3) | OP4 (S4) |
|---------|----------|----------|----------|----------|
| Ch 1/4 | base+$00 | base+$08 | base+$04 | base+$0C |
| Ch 2/5 | base+$01 | base+$09 | base+$05 | base+$0D |
| Ch 3/6 | base+$02 | base+$0A | base+$06 | base+$0E |

Channels 1-3 use Part I (address port $4000), channels 4-6 use Part II ($4002).

### $30+ - Detune and Multiplier

| Bits | Field | Description |
|------|-------|-------------|
| 7 | - | Reserved (0) |
| 6-4 | DT | Detune value |
| 3-0 | MUL | Frequency multiplier |

**MUL behavior:**
- 0: multiplies base frequency by 0.5 (right shift by 1)
- 1-15: multiplies by that value directly

**DT values:**
- 000: no detune
- 001-011: positive detune (+1 to +3 units, tone-dependent)
- 100: no detune
- 101-111: negative detune (-1 to -3 units, tone-dependent)

Detune uses a 32-entry lookup table indexed by 5-bit key code derived from F-number
and block. Maximum delta is 22 phase increment units.

**Detune overflow:** Subtractive detune can underflow from 0 to 0x1FFFF. This is
intentional and important for GEMS sound driver compatibility.

### $40+ - Total Level (TL)

| Bits | Field | Description |
|------|-------|-------------|
| 7 | - | Reserved (0) |
| 6-0 | TL | Total level (0-127) |

Each step = 0.75 dB attenuation. TL is added to envelope attenuation during output
calculation (not during EG processing):

```
final_attenuation = min(envelope_level + (TL << 3), 0x3FF)
```

Only modify TL for carrier operators to control channel volume. Modulator TL
controls modulation depth.

### $50+ - Attack Rate and Rate Scaling

| Bits | Field | Description |
|------|-------|-------------|
| 7-6 | RS | Rate scaling (key scale) |
| 5 | - | Reserved (0) |
| 4-0 | AR | Attack rate (0-31) |

### $60+ - Decay Rate and AM Enable

| Bits | Field | Description |
|------|-------|-------------|
| 7 | AMON | AM enable (1=affected by LFO AM) |
| 6-5 | - | Reserved (0) |
| 4-0 | DR | First decay rate / D1R (0-31) |

### $70+ - Sustain Rate (Second Decay)

| Bits | Field | Description |
|------|-------|-------------|
| 7-5 | - | Reserved (0) |
| 4-0 | SR | Sustain rate / D2R (0-31) |

### $80+ - Release Rate and Sustain Level

| Bits | Field | Description |
|------|-------|-------------|
| 7-4 | SL | Sustain level (0-15) |
| 3-0 | RR | Release rate (0-15, 4-bit) |

**SL values:** Each step = 3 dB (32 attenuation units). SL=15 is special-cased
to 0x3E0 (992) instead of 480.

**RR:** Only 4 bits (vs 5 for other rates). Effective rate = 2*RR + 1.

### $90+ - SSG-EG (Envelope Generator Type)

| Bits | Field | Description |
|------|-------|-------------|
| 7-4 | - | Reserved (0) |
| 3 | SSGEG:EN | Enable SSG-EG mode |
| 2 | SSGEG:ATT | Attack/invert bit |
| 1 | SSGEG:ALT | Alternate direction bit |
| 0 | SSGEG:HLD | Hold bit |

See Section 9 for detailed SSG-EG documentation.

---

## 5. Per-Channel Registers

### $A0-$A6 - Frequency (F-number and Block)

Two registers per channel form a 14-bit value (11-bit F-number + 3-bit block):

**High register ($A4-$A6):**

| Bits | Field | Description |
|------|-------|-------------|
| 7-6 | - | Reserved (0) |
| 5-3 | BLK | Block/octave (0-7) |
| 2-0 | FNUM | F-number bits 10-8 |

**Low register ($A0-$A2):**

| Bits | Field | Description |
|------|-------|-------------|
| 7-0 | FNUM | F-number bits 7-0 |

**IMPORTANT:** Must write high byte ($A4-$A6) first, then low byte ($A0-$A2).
The frequency only takes effect on the low byte write.

**Address mapping:**

| Channel | High | Low |
|---------|------|-----|
| 1 / 4 | $A4 | $A0 |
| 2 / 5 | $A5 | $A1 |
| 3 / 6 | $A6 | $A2 |

**Frequency formula:**

```
phase_increment = Fnum * 2^(Block-1) / 2^20
```

If Block=0, F-number is right-shifted by 1. Otherwise, left-shifted by (Block-1).

**Approximate semitone F-numbers (for any block):**

| Note | F-number |
|------|----------|
| C | 644 |
| C# | 681 |
| D | 722 |
| D# | 765 |
| E | 810 |
| F | 858 |
| F# | 910 |
| G | 964 |
| G# | 1021 |
| A | 1081 |
| A# | 1146 |
| B | 1214 |

### Channel 3 Special Mode - Per-Operator Frequencies

When $27 bits 7-6 enable special mode, channel 3 operators get individual frequencies:

| Operator | Slot | High Register | Low Register |
|----------|------|--------------|--------------|
| OP1 | S1 | $AD | $A9 |
| OP2 | S3 | $AE | $AA |
| OP3 | S2 | $AC | $A8 |
| OP4 | S4 | $A6 | $A2 |

Note: The register address order ($A8, $A9, $AA) does NOT match the operator
functional order. $A8/$AC is S2 (OP3), $A9/$AD is S1 (OP1), $AA/$AE is S3 (OP2).

OP4 uses the standard channel 3 frequency registers.

### $B0-$B2 - Algorithm and Feedback

| Bits | Field | Description |
|------|-------|-------------|
| 7-6 | - | Reserved (0) |
| 5-3 | FB | OP1 self-modulation feedback level (0-7) |
| 2-0 | ALGO | Algorithm selection (0-7) |

### $B4-$B6 - Panning and LFO Sensitivity

| Bits | Field | Description |
|------|-------|-------------|
| 7 | L | Left speaker output enable |
| 6 | R | Right speaker output enable |
| 5-4 | AMS | Amplitude modulation sensitivity |
| 3 | - | Reserved (0) |
| 2-0 | PMS | Phase modulation sensitivity |

**IMPORTANT:** All panning bits should be initialized to 1 at power-on. Some games
(e.g., After Burner II) depend on this.

**AMS values (tremolo depth):**

| Value | Depth |
|-------|-------|
| 00 | Disabled |
| 01 | +/- 1.4 dB |
| 10 | +/- 5.9 dB |
| 11 | +/- 11.8 dB |

**PMS values (vibrato depth, in cents):**

| Value | Depth |
|-------|-------|
| 000 | Disabled |
| 001 | +/- 3.4 cents |
| 010 | +/- 6.7 cents |
| 011 | +/- 10 cents |
| 100 | +/- 14 cents |
| 101 | +/- 20 cents |
| 110 | +/- 40 cents |
| 111 | +/- 80 cents |

---

## 6. Phase Generator

Each operator has a 20-bit phase counter that determines the current position in the
sine wave.

### Phase Increment Calculation

```
base_increment = fnum_shifted  (see frequency formula above)
detune_delta = detune_table[key_code][detune]
detuned = base_increment +/- detune_delta

if MUL == 0:
    final_increment = detuned >> 1
else:
    final_increment = (detuned * MUL) & 0xFFFFF  // truncate to 20 bits
```

The 10-bit phase output is the top 10 bits of the 20-bit counter.

### Key Code (for detune and rate scaling)

5-bit key code derived from F-number and block:

```
key_code = (block << 1) | f_number_bit_10  // simplified
```

The exact derivation also considers F-number bits 8-9 for finer resolution.

### Vibrato (LFO FM)

Vibrato modifies the F-number before phase increment calculation. The LFO 7-bit
counter's top 5 bits index a vibrato multiplier table. The highest bit determines
add vs subtract.

**Vibrato delta table (indexed by PMS level and LFO counter bits):**

```
Level 0: [0, 0, 0, 0, 0, 0, 0, 0]
Level 1: [0, 0, 0, 0, 4, 4, 4, 4]
Level 2: [0, 0, 0, 4, 4, 4, 8, 8]
Level 3: [0, 0, 4, 4, 8, 8, 12, 12]
Level 4: [0, 0, 4, 8, 8, 8, 12, 16]
Level 5: [0, 0, 8, 12, 16, 16, 20, 24]
Level 6: [0, 0, 16, 24, 32, 32, 40, 48]
Level 7: [0, 0, 32, 48, 64, 64, 80, 96]
```

These values are added/subtracted from the F-number left-shifted by 12 bits.

**IMPORTANT:** Detune and envelope key scaling must use the base F-number, NOT the
vibrato-modulated value.

---

## 7. Envelope Generator

Each operator has an ADSR (Attack-Decay-Sustain-Release) envelope generator that
outputs a 10-bit attenuation value in 4.6 fixed-point format.

### Attenuation Scale

- Range: 0 (no attenuation, max volume) to 1023 (0x3FF, silence)
- Values above 832 (0x340) produce zero amplitude output
- Community convention uses ~96 dB for the full range (~0.094 dB per unit)

**Note on dB values:** Nemesis (SpritesMind hardware research) found that Yamaha's
official documentation overstates dB values by a factor of 2. The actual hardware
attenuation range may be 0-48 dB (~0.047 dB per unit). All dB values in this document
and most community references use the conventional (uncorrected) 96 dB scale.

### ADSR Phase Transitions

```
Key On  --> Attack
Attack reaches 0 --> Decay
Decay reaches SL --> Sustain
Key Off (any phase) --> Release
```

### Rate Calculation

```
rate = 2 * R + Rks
clamped to range [0, 63]
```

Where R is the register value for the current phase:
- Attack: AR (5-bit, 0-31)
- Decay: DR (5-bit, 0-31)
- Sustain: SR (5-bit, 0-31)
- Release: 2*RR + 1 (RR is 4-bit, so effective rate is always odd, minimum 1)

### Rate Scaling (Rks)

```
key_code = 5-bit value from F-number and block
Rks = key_code >> (3 - RS)
```

RS is the 2-bit rate scaling field. Higher RS = more pitch-dependent envelope speed.

### Envelope Counter

- 12-bit global counter, shared by all operators
- Wraps from 0xFFF to 1 (skips 0)
- Update frequency depends on rate:
  - Rate 0-3: every 2^11 cycles
  - Rate 4-7: every 2^10 cycles
  - ...
  - Rate 44-63: every cycle
- General formula: shift = 11 - (rate >> 2), clamped to >= 0

### Envelope Increment

The increment is determined by an 8x64 lookup table indexed by rate and 3 bits
from the global envelope counter.

Pattern selection: `rate & 3`

Typical increments:
- Rates 0-47: 0 or 1
- Rates 48-51: up to 2
- Rates 52-55: up to 4
- Rates 56-59: up to 8
- Rates 60-63: always 8

### Attack Phase Update

```
attenuation = attenuation + ((increment * ~attenuation) >> 4)
```

Where `~attenuation` is the bitwise NOT. This creates exponential decrease toward 0.

**Special cases for high attack rates (62-63):**
- Immediately set attenuation to 0 at key on
- Skip Attack phase entirely

### Decay/Sustain/Release Phase Update

```
attenuation = min(attenuation + increment, 0x3FF)
```

Linear increase in attenuation (exponential volume decrease in dB).

### Sustain Level

4-bit value (0-15), each step = 32 attenuation units (3 dB):
- SL 0 = 0 (transition immediately)
- SL 1-14 = SL * 32
- SL 15 = 0x3E0 (992) - special case, NOT 480

---

## 8. Operator Output and Algorithms

### Sine/Log Table Lookup

The phase generator outputs a 10-bit phase value. The chip uses a quarter-wave
log2-sine table with 256 entries:

```
table[n] = -log2(sin((2n+1)/512 * pi/2))
```

Output is 12-bit, 4.8 fixed-point.

**Symmetry exploitation:**
- Phase bits 0-7: table index
- Phase bit 8: mirror (index = 0x1FF - phase)
- Phase bit 9: sign (negate output)

### Attenuation Combination

```
total_attenuation = sine_attenuation + (envelope_attenuation << 2)
```

This is addition in log-space = multiplication in linear space.
Result is 5.8 fixed-point (13-bit).

### Power Table (Exponentiation)

256-entry table for 2^(-(n+1)/256), producing 11-bit 0.11 fixed-point values.

```
linear_output = (pow_table[frac_part] << 2) >> int_part
```

Result is a 13-bit unsigned value, then negated if phase bit 9 is set,
producing a signed 14-bit PCM sample.

**Output zero threshold:** When the integer part of the combined attenuation
(total_attenuation >> 8) reaches 13 or more, the right-shift zeroes out the
13-bit power table result. This corresponds to a combined attenuation of
0x0D00 or higher in 5.8 format, which is reached when envelope attenuation is
at 0x340 (832) or above with zero sine attenuation.

### Operator 1 Feedback

OP1 (only) has unique self-modulation using its previous two outputs:

```
if feedback_level == 0:
    phase_offset = 0
else:
    phase_offset = (prev_output[0] + prev_output[1]) >> (10 - feedback_level)
```

Feedback level 0 disables; levels 1-7 provide increasing modulation depth.

Only OP1 uses feedback. On the actual hardware, the chip maintains a shared pipeline
with delay stages rather than per-operator history buffers. An emulator only needs to
store the previous two outputs for OP1 in each channel, not for all operators.

### Phase Modulation (Operator Chaining)

Modulator outputs add to carrier phase before sine lookup. The 14-bit operator
output is converted to a 10-bit phase modulation input by right-shifting by 1
(discarding bit 0 and the upper 3 sign-extension bits, using bits 1-10):

```
modulated_phase = carrier_phase + modulator_output[bits 1-10]
```

This is a phase offset applied to the phase generator's output, NOT a modification
to the phase counter or frequency. Multiple modulators are summed before application.
Maximum operator output causes phase shift of roughly +/-8*pi.

### The 8 Algorithms

Operator evaluation order is: OP1 -> OP3 -> OP2 -> OP4 (slot order, NOT
functional order).

In these diagrams, S1=OP1, S2=OP3(slot), S3=OP2(slot), S4=OP4:

```
Algorithm 0: S1 -> S3 -> S2 -> S4     Output: S4
             (OP1 -> OP2 -> OP3 -> OP4 in functional terms)

Algorithm 1: (S1+S3) -> S2 -> S4      Output: S4
             (OP1+OP3 -> OP2 -> OP4 mixed)

Algorithm 2: S1 -> S4                  Output: S4
             S3 -> S2 -> S4
             (Two paths merge at OP4)

Algorithm 3: S1 -> S3 -> S4           Output: S4
             S2 -> S4
             (OP1->OP3 chain + OP2 merge at OP4)

Algorithm 4: S1 -> S2                  Output: S2 + S4
             S3 -> S4
             (Two independent pairs)

Algorithm 5: S1 -> S2+S3+S4           Output: S2 + S3 + S4
             (OP1 feeds all three)

Algorithm 6: S1 -> S2                  Output: S2 + S3 + S4
             S3 and S4 independent
             (One modulated, two carriers)

Algorithm 7: S1, S2, S3, S4           Output: S1 + S2 + S3 + S4
             (All carriers, no modulation)
```

**Delayed modulation:** Some algorithm paths use the previous sample's output
(because the modulator hasn't been computed yet in the current cycle):

- Algorithm 0: OP2->OP3 uses previous sample
- Algorithm 1: OP1->OP3, OP2->OP3 use previous sample
- Algorithm 2: OP2->OP3 uses previous sample
- Algorithm 3: OP2->OP4 uses previous sample
- Algorithm 5: OP1->OP3 uses previous sample
- Algorithms 4, 6, 7: no delays needed

### Multi-Carrier Output Clamping

When an algorithm has multiple carriers, outputs are summed and clamped to
signed 14-bit range. Per the 9-bit DAC quantization, each carrier output is
quantized before summing.

---

## 9. SSG-EG Mode (Undocumented)

SSG-EG modifies the envelope generator behavior to create looping/alternating
envelope shapes, similar to the SSG (PSG) envelope modes on the AY-3-8910.

### Control Bits (Register $90+)

| Bit 2 (Attack) | Bit 1 (Alternate) | Bit 0 (Hold) | Shape |
|----------------|-------------------|--------------|-------|
| 0 | 0 | 0 | Repeating sawtooth (down) |
| 0 | 0 | 1 | Single sawtooth then silence |
| 0 | 1 | 0 | Triangle wave (repeating) |
| 0 | 1 | 1 | Triangle, hold at 0 attenuation |
| 1 | 0 | 0 | Inverted repeating sawtooth (up) |
| 1 | 0 | 1 | Inverted sawtooth then hold |
| 1 | 1 | 0 | Inverted triangle (repeating) |
| 1 | 1 | 1 | Inverted triangle, hold at max |

### Attenuation Range

SSG-EG causes attenuation to oscillate between 0 and 0x200 (512).
In the conventional dB scale this is ~48 dB; see Section 7 dB note.

### Decay Rate in SSG-EG Mode

```
if attenuation < 0x200:
    attenuation += 4 * increment   // 4x normal speed
else:
    attenuation unchanged          // hold at boundary
```

The 4x multiplier is confirmed by Nemesis hardware research on SpritesMind.
Earlier documentation incorrectly stated 6x. MAME also uses 4x.

### Output Inversion

```
inverted = ssg_attack_bit XOR ssg_internal_invert_flag

if inverted:
    output = (0x200 - internal_attenuation) & 0x3FF
else:
    output = internal_attenuation
```

Inversion only applies when SSG-EG is enabled AND operator is NOT in Release phase.

### SSG-EG State Update (when attenuation >= 0x200)

1. If Alternate: toggle internal inversion flag (or set to 1 if Hold also set)
2. If not Alternate and not Hold: reset phase counter to 0
3. If not Hold and operator is keyed on: re-enter Attack phase
4. If Hold, not in Attack, and output not inverted: set attenuation to 0x3FF
5. If operator keyed off: set attenuation to 0x3FF

### Attack Rate Requirement

The YM2608 manual states SSG-EG should only be used with AR=31 (maximum).
With AR=31, Attack phase is always skipped. Lower attack rates create
unpredictable behavior (some games like Olympic Gold depend on this).

---

## 10. LFO (Low Frequency Oscillator)

A single shared LFO provides both vibrato (FM) and tremolo (AM) effects.

### LFO Counter

7-bit counter (0-127) that increments at the rate selected by register $22.
A complete oscillation cycle = 128 increments.

### Tremolo (Amplitude Modulation)

Tremolo adds attenuation to envelope output using the LFO counter:

```
if lfo_counter bit 6 == 0:
    lfo_am = 0x3F - (counter & 0x3F)    // decreasing
else:
    lfo_am = counter & 0x3F              // increasing

lfo_am <<= 1    // scale to match envelope units
```

Then apply per-channel AMS sensitivity:
- AMS 0: disabled (0)
- AMS 1: lfo_am >> 3 (~1.4 dB max)
- AMS 2: lfo_am >> 1 (~5.9 dB max)
- AMS 3: lfo_am (full, ~11.8 dB max)

Tremolo is enabled per-operator via the AMON bit in register $60+.

### Vibrato (Frequency Modulation)

Vibrato modifies F-number using the LFO counter's top 5 bits.
The highest bit determines direction (add/subtract).

See Section 6 for the vibrato delta table.

PMS is set per-channel in register $B4+ bits 2-0.

---

## 11. DAC and Analog Output

### 9-Bit DAC Quantization

The YM2612 internally produces signed 14-bit samples but uses a 9-bit DAC.
The lowest 5 bits are truncated during output.

**Hardware output method:** The chip uses time-division multiplexing (TDM) to
output channels through the DAC sequentially - it does NOT mix them internally.
Each channel's 9-bit sample is output one at a time, cycling through all 6
channels. The external analog low-pass filter on the console smooths the
multiplexed signal into a combined waveform.

**Emulator implication:** Since emulators typically mix channels digitally rather
than multiplexing, quantize each carrier output BEFORE mixing:

```
// Per carrier output, mask off lowest 5 bits
quantized = output & 0xFFFFFFE0   // i.e., & ^0x1F

// Then sum carriers and clamp
channel_output = clamp(sum_of_quantized_carriers, -0x1FF0, +0x1FE0)
```

### Ladder Effect (YM2612 only, not YM3438)

The DAC has nonlinear crossover distortion: the voltage gap between samples
-1 and 0 is twice as large as linear.

**DAC crossover distortion compensation (from die-shot analysis):**

Per channel, per L/R output:
- If muted: output +4 (non-negative) or -4 (negative) based on sample sign
- If unmuted and sample >= 0: add +4
- If unmuted and sample < 0: add -3

These values are for 9-bit samples. For 14-bit, left-shift by 5.

### Low-Pass Filtering

The Genesis hardware (not the YM2612 itself) includes a low-pass filter:
- Recommended: first-order Butterworth IIR at 3.39 kHz or 2.84 kHz cutoff
- Applies to both YM2612 and PSG outputs
- Many games have audio designed around this characteristic

---

## 12. Timers

### Timer A (10-bit)

- Period: (1024 - TMRA) * 18.77 us
- Range: ~18 us to ~19.2 ms
- Registers: $24 (high 8 bits), $25 (low 2 bits)

### Timer B (8-bit)

- Period: (256 - TMRB) * 18.77 us * 16
- Equivalent: (256 - TMRB) * 300.34 us
- Range: ~300 us to ~76.8 ms
- Register: $26

Timer B uses a /16 prescaler relative to the sample clock. It increments once
every 16 FM sample clocks, compared to Timer A which increments every sample clock.
This is the source of the 16x factor in the period formula.

### Timer Control ($27)

- LOAD bits: start timer counting
- ENBL bits: allow overflow to set status flags
- RST bits: clear overflow flags (write 1 to clear)

Timer overflow flags are readable from the status register ($4000 bit 0/1).

---

## Appendix A: Key Implementation Notes

1. **Operator processing order** in registers is S1, S3, S2, S4 (NOT S1, S2, S3, S4).
   This affects how you index operator registers.

2. **Frequency register write order** must be high byte first ($A4-$A6), then low
   byte ($A0-$A2). The value latches on the low byte write.

3. **Panning defaults** should be L=1, R=1 (both enabled) at power-on.

4. **DAC initial value** should be 0x80 (silence when converted to signed).

5. **Envelope counter** wraps to 1, not 0 (skips 0 on overflow).

6. **SSG-EG** implementation varies between real YM2612 and YM3438 clones.
   Setting to 0 ensures compatibility.

7. **The ladder effect** is most pronounced on the original YM2612; reduced on
   YM3438; largely eliminated on ASIC versions. See Appendix B.

8. **Busy flag** duration is approximately 32 internal cycles after a write.

## Appendix B: Hardware Revisions

The FM chip went through several revisions across Genesis hardware, each with
distinct audio characteristics. The earlier revisions sound "crunchier" with more
pronounced distortion artifacts; later revisions are "smoother" and cleaner.

### YM2612 (OPN2) - NMOS, Discrete

- **Process:** NMOS
- **Found in:** Genesis Model 1 VA2-VA6.8, Model 2 VA2/VA2.8
- **Sound character:** Most "crunchy". Pronounced ladder effect distortion.
- **Ladder effect:** Full. The DAC has a nonlinear voltage gap between output
  values -1 and 0 that is 8x larger than a linear DAC would produce. This
  amplifies low-volume signals, adds crossover distortion at the zero crossing,
  and makes fades and quiet passages noisier/grainier.
- **Read ports $4001-$4003:** Return the last value read from $4000, decaying
  to 0 over ~250ms. This causes bugs in some games (Earthworm Jim stutters
  because it polls the busy flag from $4002).
- **Channel 6 DAC:** Separate handling from FM output path.
- **Notable:** Some composers (Yuzo Koshiro for Streets of Rage, After Burner II)
  intentionally designed their music around the ladder effect distortion.

### YM3438 (OPN2C) - CMOS, Discrete

- **Process:** CMOS (lower power consumption and heat)
- **Found in:** Brief transitional use in some boards
- **Sound character:** Smoother. Reduced but not eliminated ladder effect.
- **Ladder effect:** Greatly reduced. Yamaha improved the DAC to significantly
  reduce crossover distortion, producing a higher signal-to-noise ratio.
- **Read ports $4001-$4003:** Mirror $4000 (always return current status flags).
  This fixes the Earthworm Jim bug but causes Hellfire to play at slower tempo
  due to different busy flag polling behavior.
- **Channel 6 DAC:** Changed to same 9-bit output format as FM mode.
- **Output impedance:** Higher than YM2612, requiring heavier external filtering
  but producing louder output.

### ASIC YM3438 (FC1004 / GOAC Integrated)

- **Process:** CMOS, integrated into larger ASICs
- **Found in:** Genesis Model 1 VA7, Model 2 VA0-VA1.8/VA3-VA4, all Model 3s
- **Sound character:** Cleanest/smoothest. Minimal or no ladder effect.
- **Ladder effect:** Largely eliminated. Sega's Model 2 ASIC uses a modified
  output multiplexer that does not perform the bit-depth truncation that causes
  the ladder effect in the discrete chips.
- **ASICs used:**
  - FC1004 (Yamaha) - combines VDP (315-5313) + I/O (315-5433) + YM3438
  - Later GOAC variants - further integration used in VA4 Model 2 and Model 3
- **Note:** The VA7 Model 1 and VA0-VA1.8 Model 2 sound poor despite having the
  ASIC chip. This is due to bad op-amp and circuit component choices in the
  surrounding audio circuitry, NOT the FM core itself.

### Comparison Table

| Feature | YM2612 | YM3438 | ASIC YM3438 |
|---------|--------|--------|-------------|
| Process | NMOS | CMOS | CMOS (integrated) |
| Ladder effect | Full (8x gap) | Reduced | Minimal/none |
| Status read $4001-3 | Decaying last read | Mirrors $4000 | Mirrors $4000 |
| Ch6 DAC format | Separate path | Unified 9-bit | Unified 9-bit |
| Power/heat | Higher | Lower | Lowest |
| Sound character | Crunchy/warm | Cleaner | Cleanest |

### Ladder Effect Technical Detail

The ladder effect is a DAC crossover distortion caused by a design flaw in the
YM2612's resistor-ladder DAC. The output voltage is fairly linear for 9-bit
signed values in the ranges -256 to -1 and 0 to +255 separately, but there is
a disproportionate voltage jump at the zero crossing (between -1 and 0).

**Per-slot behavior (cycle-accurate):**
The YM2612 outputs to its DAC in groups of 4 slots per channel. The first slot
carries the audio sample; the other 3 are "silence" slots that output +1 or -1
based on the sample's sign bit (even when the channel is muted by panning).

**Emulation (non-cycle-accurate, per channel per L/R output):**
- If channel is muted: output fixed +4 (sample >= 0) or -4 (sample < 0)
- If channel is unmuted and sample >= 0: add +4 to sample
- If channel is unmuted and sample < 0: add -3 to sample

These values are in 9-bit scale. For 14-bit output, left-shift by 5.

**Effect on audio:**
- Amplifies low-volume waveforms (quiet sounds become louder than expected)
- Adds a pulse-like signal tracking the input waveform's zero crossings
- Creates audible noise/grain during fades and quiet passages
- Games designed for YM2612 may sound too quiet on YM3438 hardware

### Emulator Chip Selection

An emulator can offer a choice between YM2612 and YM3438 behavior:
- **YM2612 mode:** Apply ladder effect + 9-bit DAC quantization
- **YM3438 mode:** Apply 9-bit DAC quantization only (no ladder effect)
- **Read port behavior:** Mirror $4000 for YM3438, decay for YM2612

Most emulators default to YM2612 behavior since the majority of the Genesis
library was developed and tested on YM2612 hardware.

## Appendix C: Related Chips

| Chip | Channels | Notes |
|------|----------|-------|
| YM2203 (OPN) | 3 FM + SSG | Predecessor |
| YM2608 (OPNA) | 6 FM + SSG + ADPCM | Enhanced version |
| YM3438 (OPN2C) | Same as YM2612 | CMOS version, reduced ladder effect |
| YM2610 (OPNB) | 4 FM + ADPCM | Neo Geo |

## Appendix D: Genesis Board Revisions and System Audio

The overall audio character of a Genesis depends not just on the FM chip variant
(Appendix B) but also on the surrounding analog circuitry: op-amps, low-pass
filters, gain stages, and FM/PSG mixing networks. These vary significantly across
board revisions and are often the dominant factor in perceived audio quality.

### Model 1 Board Revisions

#### VA0 (837-6656, 1988, Japan only)

- **FM chip:** Discrete YM2612
- **PSG:** Embedded in VDP (315-5313)
- **Audio circuit:** Sony CXA1034P headphone amplifier with LM358 op-amp in
  voltage follower configuration. The audio preamp is overdriven, causing
  clipping distortion on bass-heavy or loud content.
- **Low-pass filter:** First-order RC at ~3.39 kHz cutoff (20 dB/decade).
- **Mono AV output:** Broken - outputs only the right channel instead of L+R mix.

#### VA1 (837-6832, 1989, Japan only)

- **FM chip:** Discrete YM2612
- **Audio circuit:** Same overdriven CXA1034P preamp and LM358 as VA0.
- **Low-pass filter:** Same 3.39 kHz first-order RC.
- **Mono AV output:** Same broken mono mixing as VA0.

#### VA2 (837-6955/837-6992, 1989)

- **FM chip:** Discrete YM2612
- **Audio circuit:** Same overdriven CXA1034P preamp. Still uses LM358.
- **Low-pass filter:** Same 3.39 kHz first-order RC.
- **Mono AV output:** Still broken (right channel only).
- **Note:** First revision available in North America.

#### VA3 (837-7071, 1989-1991)

- **FM chip:** Discrete YM2612
- **Audio circuit:** Major improvement. Preamp gain corrected - no more clipping.
  Slightly more low-pass filtering added to headphone output.
- **Low-pass filter:** First-order RC at ~2.84 kHz cutoff (slightly more
  aggressive than VA0-VA2).
- **Mono AV output:** Fixed - proper L+R mono mixing.
- **Assessment:** Beginning of the "sweet spot" for Model 1 audio.

#### VA4-VA6.8 (1989-1993)

- **FM chip:** Discrete YM2612
- **Audio circuit:** Same corrected design as VA3. No audio changes through
  VA4, VA5, VA6, VA6.5, and VA6.8 (PAL-only surface-mount video encoder).
- **Low-pass filter:** Same 2.84 kHz first-order RC.
- **Assessment:** Audio identical to VA3. VA3-VA6.8 are the gold standard
  for Genesis audio quality.

#### VA7 (171-6217, 1992-1993)

- **FM chip:** Yamaha FC1004 ASIC (315-5487 or 315-5660)
- **Audio circuit:** Drastically different and much worse. Poor op-amp choices
  in the surrounding circuitry produce muffled, distorted audio.
- **Assessment:** Worst-sounding Model 1 despite the ASIC FM core being capable
  of clean output. Excellent video output, terrible audio. Problems are entirely
  in the analog output stage.

### Model 2 Board Revisions

#### VA0/VA1/VA1.8 (171-6349/171-6534, 1993-1996)

- **FM chip:** Yamaha FC1004 ASIC (315-5487, 315-5660, 315-5700, or 315-5708)
- **Audio circuit:** LM324 quad op-amp - an inexpensive part with poor audio
  characteristics: low slew rate, high crossover distortion, limited bandwidth.
  A Sallen-Key low-pass filter topology with excessive Q-factor on FM channels
  causes ringing artifacts and reduced high-frequency response.
- **PSG volume:** Reduced relative to FM.
- **Assessment:** Terrible audio quality. Among the worst-sounding Genesis units.

#### VA2/VA2.3 (171-6535F/171-7039, 1994-1996, North America only)

- **FM chip:** Discrete YM2612 - the Toshiba ASIC (315-5786) used in these
  boards does NOT contain a YM3438 core, so a separate YM2612 is required.
- **Audio circuit:** 315-5684 audio amplifier (ROHM BA6166FS) replaces the
  dual LM324s. FM channels are considerably louder than PSG and external audio.
- **PSG wiring error:** The PSG requires a pull-down resistor before any other
  circuit element. On VA2/VA2.3, a series resistor is placed before the
  pull-down, creating a voltage divider. This causes the PSG to be severely
  out of tune and distorted. Master System games are particularly affected.
- **Assessment:** Clear FM sound but badly imbalanced volume and broken PSG.

#### VA3 (171-6615, 1994-1996)

- **FM chip:** FC1004 ASIC (315-5660 or 315-5700)
- **Audio circuit:** First "short board" revision. Uses 315-5684 amplifier with
  corrected PSG wiring. Second-order low-pass filter with adjustable external
  capacitors. Volume balance is near-perfect - described as the best FM/PSG
  balance of any Model 2.
- **Assessment:** Best-sounding Model 2. Nearly as good as Model 1 VA3-VA6.8
  but slightly more muffled due to second-order filter (40 dB/decade vs
  20 dB/decade on Model 1).

#### VA4 (171-7229, 1996-1998)

- **FM chip:** GOAC 315-5960 (Yamaha FJ3002) - integrates 68000, Z80, Z80 RAM,
  VDP, YM3438 core, I/O, and bus arbiter into a single chip.
- **Audio circuit:** Same 315-5684 amplifier as VA3. FM synthesis slightly
  louder than normal but generally well-balanced.
- **Assessment:** Good audio, on par with VA3.

### Model 3

- **FM chip:** GOAC 315-5960 (VA1) or 315-6123/FQ8007 (VA2)
- **Audio circuit:** Mono only. Stereo was omitted entirely. Single LM324 op-amp
  for amplification. No headphone jack.
- **Assessment:** Adequate mono audio. The GOAC FM core is clean but the mono
  output and LM324 limit appeal. Can be modded for stereo.

### Other Variants

| Variant | FM Chip | Audio Notes |
|---------|---------|-------------|
| CDX/Multi-Mega | FC1004 ASIC | Clearest of all Genesis systems but weak output levels, slightly clinical |
| Nomad | FF1004 (315-5700) | Surprisingly good. Stronger bass than pre-VA7 Model 1. Slightly louder PSG |
| Wondermega | FC1004 ASIC | JVC-designed high-fidelity output components. Very clean, good balance |
| X'Eye | FC1004 ASIC | Clean and precise but clinical. Best Genesis+CD combo for audio quality |

### FM/PSG Mixing Balance

The PSG (SN76489 equivalent embedded in the VDP) outputs at a significantly higher
raw voltage level than the YM2612 FM channels - approximately 27 dB louder.
This requires substantial attenuation before mixing.

**Model 1 mixing:**
- FM stereo output goes through the Sony CXA1034P mixing/amplifier IC.
- PSG mono output (from VDP) is mixed into both L and R channels through
  51K ohm resistors. The high resistance attenuates the PSG to bring it
  closer to FM levels.
- For rear AV output, mono mixing uses two 10K ohm resistors (one per channel)
  to combine L+R.
- VA3+ has the correct, well-tuned FM/PSG balance. VA0-VA2 has the same
  mixing circuit but overdriven preamp gain.

**Model 2 VA0-VA1.8 mixing:**
- LM324 quad op-amp handles mixing. PSG volume is reduced relative to FM.
- The aggressive Sallen-Key filter further colors the balance.

**Model 2 VA2/VA2.3 mixing:**
- 315-5684 amplifier. FM is considerably louder than PSG due to both the
  amplifier design and the PSG voltage divider wiring error.

**Model 2 VA3/VA4 mixing:**
- 315-5684 amplifier with corrected PSG wiring. Near-perfect FM/PSG balance.

### Low-Pass Filter Summary

The low-pass filter is part of the Genesis motherboard, not the YM2612 chip.
It applies to both FM and PSG outputs. Many games were designed with this
filter in mind, and it significantly shapes noise channel and high-frequency
FM timbres.

| Revision | Filter Type | Cutoff | Roll-off |
|----------|------------|--------|----------|
| Model 1 VA0-VA2 | 1st-order RC | ~3.39 kHz | 20 dB/decade |
| Model 1 VA3-VA6.8 | 1st-order RC | ~2.84 kHz | 20 dB/decade |
| Model 2 VA0-VA1.8 | Sallen-Key (2nd-order, high Q) | Aggressive | 40 dB/decade |
| Model 2 VA3-VA4 | 315-5684 internal | ~2.84 kHz equiv | 40 dB/decade |

### ASIC Reference

| Part Number | Yamaha Name | Integrates | Used In |
|-------------|-------------|------------|---------|
| 315-5487 | FC1004 | VDP + I/O + YM3438 | Model 1 VA7, early Model 2 VA0 |
| 315-5660 | FC1004 variant | VDP + I/O + YM3438 (50Hz fixed) | Model 2 VA0/VA1/VA3 |
| 315-5700 | FF1004 | VDP + I/O + YM3438 (low power) | Nomad, some Model 2 VA1.8/VA3 |
| 315-5786 | N/A (Toshiba) | VDP + I/O (NO YM3438) | Model 2 VA2/VA2.3 |
| 315-5960 | FJ3002 (GOAC) | 68000+Z80+Z80RAM+VDP+YM3438+I/O | Model 2 VA4, Model 3 VA1 |
| 315-6123 | FQ8007 (GOAC) | Same + unified RAM | Model 3 VA2 |

### Audio Quality Rankings

**Model 1 (best to worst):**
1. VA3-VA6.8 - Best stock audio. Clean, well-balanced.
2. VA0-VA2 - Good underlying audio with overdriven preamp (fixable).
3. VA7 - Poor audio due to analog circuit despite clean ASIC FM core.

**Model 2 (best to worst):**
1. VA3 - Best balance, good clarity, proper FM/PSG mixing.
2. VA4 - Nearly identical to VA3, slightly louder FM.
3. VA2/VA2.3 - Clear FM but broken PSG, bad volume balance.
4. VA0/VA1/VA1.8 - Worst. Muffled, distorted from LM324 and Sallen-Key filter.

### Emulator Reference Target

The recommended reference target for emulation is the **Model 1 VA3-VA6.8**:
- Discrete YM2612 with full ladder effect
- First-order low-pass at ~2.84 kHz
- Correct FM/PSG balance (PSG attenuated ~27 dB from raw via 51K mixing resistors)
- Well-regarded as the intended sound for the commercial Genesis library

## Sources

- [SMS Power - YM2612 Sega Genesis Technical Manual](https://www.smspower.org/maxim/Documents/YM2612)
- [Plutiedev - YM2612 Register Reference](https://www.plutiedev.com/ym2612-registers)
- [jsgroth - Emulating the YM2612 (Parts 1-7)](https://jsgroth.dev/blog/posts/emulating-ym2612-part-1/)
- [sauraen/YM2612 - VHDL description and documentation](https://github.com/sauraen/YM2612)
- [SpritesMind - Authoritative YM2612 reference (Nemesis hardware research)](https://gendev.spritesmind.net/forum/viewtopic.php?t=386)
- [MegaDrive Development Wiki - YM2612 Registers](https://wiki.megadrive.org/index.php?title=YM2612_Registers)
- [ConsoleMods Wiki - Genesis Audio Chip Notes](https://consolemods.org/wiki/Genesis:Audio_Chip_Notes)
- [VGMPF - YM2612](https://vgmpf.com/Wiki/index.php?title=YM2612)
- [Joel KP - YM2612 Ladder Effect Analysis](https://joelkp.frama.io/tech/dist-ladder-effect.html)
- [Wikipedia - Yamaha YM2612](https://en.wikipedia.org/wiki/Yamaha_YM2612)
- [ConsoleMods Wiki - Genesis Motherboard Differences](https://consolemods.org/wiki/Genesis:Motherboard_Differences)
- [ConsoleMods Wiki - Genesis Buying Guide](https://consolemods.org/wiki/Genesis:Buying_Guide)
- [ConsoleMods Wiki - Genesis Preamp Fix (Model 1)](https://consolemods.org/wiki/Genesis:Preamp_Fix_(Model_1))
- [ConsoleMods Wiki - Genesis Audio Circuit Mod (Model 2)](https://consolemods.org/wiki/Genesis:Audio_Circuit_Mod_(Model_2))
- [RetroRGB - The Best Version of the Genesis](https://retrorgb.com/genesisversions.html)
- [Sega-16 Forums - Guide: Telling Apart Good Genesis 1s and 2s](https://www.sega-16forums.com/forum/general-discussion/tech-aid/7756-guide-telling-apart-good-genesis-1s-and-genesis-2s-from-bad-ones)
- [jsgroth - Genesis & Sega CD Audio Filtering](https://jsgroth.dev/blog/posts/genesis-sega-cd-audio-filtering/)
- [Console5 Wiki - Mega Drive II / Genesis 2](https://wiki.console5.com/wiki/Mega_Drive_II_/_Genesis_2)
