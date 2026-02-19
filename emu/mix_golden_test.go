package emu

import (
	"math"
	"testing"

	"github.com/user-none/go-chip-sn76489"
)

// mixBuffers replicates the mixing logic from emulator.go lines 191-209.
// YM2612 produces stereo L/R pairs, PSG produces mono samples.
func mixBuffers(ym2612Samples []int16, psgBuf []float32, psgCount int) []int16 {
	ymPairs := len(ym2612Samples) / 2
	mixCount := ymPairs
	if psgCount < mixCount {
		mixCount = psgCount
	}

	var out []int16

	for i := 0; i < mixCount; i++ {
		fmL := int32(ym2612Samples[i*2])
		fmR := int32(ym2612Samples[i*2+1])
		psgVal := int32(psgBuf[i])
		mixL := clampInt32(fmL+psgVal, -32768, 32767)
		mixR := clampInt32(fmR+psgVal, -32768, 32767)
		out = append(out, int16(mixL), int16(mixR))
	}

	// Append remaining YM2612 stereo samples
	if ymPairs > mixCount {
		out = append(out, ym2612Samples[mixCount*2:]...)
	}

	// Append remaining PSG samples as stereo (mono duplicated)
	for i := mixCount; i < psgCount; i++ {
		s := int16(psgBuf[i])
		out = append(out, s, s)
	}

	return out
}

func lowPassStereo(buf []int16) []int16 {
	var prevL, prevR float64
	for i := 0; i < len(buf); i += 2 {
		inL := float64(buf[i])
		inR := float64(buf[i+1])
		prevL = lpfAlpha*inL + (1-lpfAlpha)*prevL
		prevR = lpfAlpha*inR + (1-lpfAlpha)*prevR
		buf[i] = int16(math.Round(prevL))
		buf[i+1] = int16(math.Round(prevR))
	}
	return buf
}

// --- Mix Golden Tests ---

func TestMixGolden_YM2612Only(t *testing.T) {
	// YM2612 ch0 algo 7 + PSG silent
	ym := setupTestChannel(7)
	psg := sn76489.New(3579545, 48000, 1024, sn76489.Sega)
	psg.SetGain(1898.0)

	// Silence all PSG channels
	psg.Write(0x9F)
	psg.Write(0xBF)
	psg.Write(0xDF)
	psg.Write(0xFF)

	// Generate one frame
	cyclesPerScanline := (7670454 / 60) / 262
	z80CyclesPerScanline := (3579545 / 60) / 262
	psg.ResetBuffer()
	for i := 0; i < 262; i++ {
		ym.GenerateSamples(cyclesPerScanline)
		psg.Run(z80CyclesPerScanline)
	}

	ymBuf := ym.GetBuffer()
	psgBuf, psgCount := psg.GetBuffer()
	mixed := lowPassStereo(mixBuffers(ymBuf, psgBuf, psgCount))

	expectedFirst := []int16{
		382, 382, 799, 799, 1241, 1241, 1720, 1720,
		2208, 2208, 2685, 2685, 3167, 3167, 3518, 3518,
		3775, 3775, 3962, 3962, 4098, 4098, 4197, 4197,
		4269, 4269, 4322, 4322, 4361, 4361, 4389, 4389,
		4409, 4409, 4424, 4424, 4435, 4435, 4443, 4443,
		4448, 4448, 4453, 4453, 4456, 4456, 4458, 4458,
		4460, 4460, 4461, 4461, 4462, 4462, 4462, 4462,
		4463, 4463, 4463, 4463, 4463, 4463, 4464, 4464,
	}
	expectedHash := "3a3040f0271a898609c6f505b6836b4d563e8ddbc8d8a1e3590f7aca8d32fcc1"

	compareGoldenInt16(t, "Mix_YM2612Only", mixed, expectedFirst, expectedHash)
}

func TestMixGolden_PSGOnly(t *testing.T) {
	// YM2612 no keys + PSG single tone
	ym := NewYM2612(7670454, 48000)
	psg := sn76489.New(3579545, 48000, 1024, sn76489.Sega)
	psg.SetGain(1898.0)

	// PSG: ch0 ~440Hz, vol=0 (max), others silent
	psg.Write(0x80 | 0x0E)
	psg.Write(0x0F)
	psg.Write(0x90 | 0x00)
	psg.Write(0xBF)
	psg.Write(0xDF)
	psg.Write(0xFF)

	// Generate one frame
	cyclesPerScanline := (7670454 / 60) / 262
	z80CyclesPerScanline := (3579545 / 60) / 262
	psg.ResetBuffer()
	for i := 0; i < 262; i++ {
		ym.GenerateSamples(cyclesPerScanline)
		psg.Run(z80CyclesPerScanline)
	}

	ymBuf := ym.GetBuffer()
	psgBuf, psgCount := psg.GetBuffer()
	mixed := lowPassStereo(mixBuffers(ymBuf, psgBuf, psgCount))

	expectedFirst := []int16{
		618, 618, 1069, 1069, 1398, 1398, 1638, 1638,
		1812, 1812, 1940, 1940, 2032, 2032, 2100, 2100,
		2149, 2149, 2185, 2185, 2211, 2211, 2231, 2231,
		2245, 2245, 2255, 2255, 2262, 2262, 2267, 2267,
		2271, 2271, 2274, 2274, 2276, 2276, 2278, 2278,
		2279, 2279, 2280, 2280, 2280, 2280, 2281, 2281,
		2281, 2281, 2281, 2281, 2282, 2282, 2282, 2282,
		2282, 2282, 2282, 2282, 2282, 2282, 2282, 2282,
	}
	expectedHash := "6a5b2b11918e5fc6fc909effdb7b0482d2acef034886092a0f65dd48886a6079"

	compareGoldenInt16(t, "Mix_PSGOnly", mixed, expectedFirst, expectedHash)
}

func TestMixGolden_BothActive(t *testing.T) {
	// YM2612 ch0 algo 7 + PSG tone
	ym := setupTestChannel(7)
	psg := sn76489.New(3579545, 48000, 1024, sn76489.Sega)
	psg.SetGain(1898.0)

	// PSG: ch0 ~440Hz, vol=0 (max), others silent
	psg.Write(0x80 | 0x0E)
	psg.Write(0x0F)
	psg.Write(0x90 | 0x00)
	psg.Write(0xBF)
	psg.Write(0xDF)
	psg.Write(0xFF)

	// Generate one frame
	cyclesPerScanline := (7670454 / 60) / 262
	z80CyclesPerScanline := (3579545 / 60) / 262
	psg.ResetBuffer()
	for i := 0; i < 262; i++ {
		ym.GenerateSamples(cyclesPerScanline)
		psg.Run(z80CyclesPerScanline)
	}

	ymBuf := ym.GetBuffer()
	psgBuf, psgCount := psg.GetBuffer()
	mixed := lowPassStereo(mixBuffers(ymBuf, psgBuf, psgCount))

	expectedFirst := []int16{
		896, 896, 1688, 1688, 2404, 2404, 3082, 3082,
		3715, 3715, 4298, 4298, 4857, 4857, 5265, 5265,
		5562, 5562, 5779, 5779, 5937, 5937, 6052, 6052,
		6136, 6136, 6197, 6197, 6242, 6242, 6275, 6275,
		6298, 6298, 6316, 6316, 6328, 6328, 6337, 6337,
		6344, 6344, 6349, 6349, 6352, 6352, 6355, 6355,
		6357, 6357, 6358, 6358, 6359, 6359, 6360, 6360,
		6361, 6361, 6361, 6361, 6361, 6361, 6361, 6361,
	}
	expectedHash := "36527f5af2a3718d4a2dccfad200b7de7679dfc6aaf40b66485eca1f348dfeff"

	compareGoldenInt16(t, "Mix_BothActive", mixed, expectedFirst, expectedHash)
}

func TestMixGolden_Clipping(t *testing.T) {
	// YM2612 all 6 channels at max + PSG at max volume to force clipping
	ym := setupMaxOutputYM2612()
	psg := sn76489.New(3579545, 48000, 1024, sn76489.Sega)
	psg.SetGain(1898.0)

	// PSG: all 3 tone channels at max volume, low frequency for sustained output
	psg.Write(0x80 | 0x0E) // Ch0 tone low
	psg.Write(0x0F)        // Ch0 tone high -> 254
	psg.Write(0x90 | 0x00) // Ch0 vol=0 (max)
	psg.Write(0xA0 | 0x0E) // Ch1 tone low
	psg.Write(0x0F)        // Ch1 tone high -> 254
	psg.Write(0xB0 | 0x00) // Ch1 vol=0 (max)
	psg.Write(0xC0 | 0x0E) // Ch2 tone low
	psg.Write(0x1F)        // Ch2 tone high -> 254+256=510? No: 0x0FE
	psg.Write(0xD0 | 0x00) // Ch2 vol=0 (max)
	psg.Write(0xFF)        // Noise vol=15 (off)

	// Generate one frame
	cyclesPerScanline := (7670454 / 60) / 262
	z80CyclesPerScanline := (3579545 / 60) / 262
	psg.ResetBuffer()
	for i := 0; i < 262; i++ {
		ym.GenerateSamples(cyclesPerScanline)
		psg.Run(z80CyclesPerScanline)
	}

	ymBuf := ym.GetBuffer()
	psgBuf, psgCount := psg.GetBuffer()
	mixed := lowPassStereo(mixBuffers(ymBuf, psgBuf, psgCount))

	expectedFirst := []int16{
		3312, 3312, 6559, 6559, 9759, 9759, 13028, 13028,
		16244, 16244, 19317, 19317, 22363, 22363, 24584, 24584,
		26203, 26203, 27383, 27383, 28244, 28244, 28871, 28871,
		29328, 29328, 29661, 29661, 29904, 29904, 30082, 30082,
		30211, 30211, 30305, 30305, 30373, 30373, 30423, 30423,
		30460, 30460, 30486, 30486, 30506, 30506, 30520, 30520,
		30530, 30530, 30538, 30538, 30543, 30543, 30547, 30547,
		30550, 30550, 30552, 30552, 30554, 30554, 30555, 30555,
	}
	expectedHash := "16cb0c5b272d4a5a4df40e59bd39643ba117bd04d4a7401204d031dd8d930168"

	compareGoldenInt16(t, "Mix_Clipping", mixed, expectedFirst, expectedHash)
}

func TestMixGolden_ExtraYM2612Samples(t *testing.T) {
	// Generate YM2612 for more scanlines than PSG to produce extra FM samples
	ym := setupTestChannel(7)
	psg := sn76489.New(3579545, 48000, 1024, sn76489.Sega)
	psg.SetGain(1898.0)

	// PSG: single tone
	psg.Write(0x80 | 0x0E)
	psg.Write(0x0F)
	psg.Write(0x90 | 0x00)
	psg.Write(0xBF)
	psg.Write(0xDF)
	psg.Write(0xFF)

	cyclesPerScanline := (7670454 / 60) / 262
	z80CyclesPerScanline := (3579545 / 60) / 262

	psg.ResetBuffer()
	// Run YM2612 for 262 scanlines but PSG for only 200
	for i := 0; i < 262; i++ {
		ym.GenerateSamples(cyclesPerScanline)
		if i < 200 {
			psg.Run(z80CyclesPerScanline)
		}
	}

	ymBuf := ym.GetBuffer()
	psgBuf, psgCount := psg.GetBuffer()
	mixed := lowPassStereo(mixBuffers(ymBuf, psgBuf, psgCount))

	expectedFirst := []int16{
		896, 896, 1688, 1688, 2404, 2404, 3082, 3082,
		3715, 3715, 4298, 4298, 4857, 4857, 5265, 5265,
		5562, 5562, 5779, 5779, 5937, 5937, 6052, 6052,
		6136, 6136, 6197, 6197, 6242, 6242, 6275, 6275,
		6298, 6298, 6316, 6316, 6328, 6328, 6337, 6337,
		6344, 6344, 6349, 6349, 6352, 6352, 6355, 6355,
		6357, 6357, 6358, 6358, 6359, 6359, 6360, 6360,
		6361, 6361, 6361, 6361, 6361, 6361, 6361, 6361,
	}
	expectedHash := "25a78bc28654fac24a6520903379964fb78374c22548e01a7b83f38ac58787db"

	compareGoldenInt16(t, "Mix_ExtraYM2612Samples", mixed, expectedFirst, expectedHash)
}

func TestMixGolden_ExtraPSGSamples(t *testing.T) {
	// Generate PSG for more scanlines than YM2612 to produce extra PSG samples
	ym := setupTestChannel(7)
	psg := sn76489.New(3579545, 48000, 1024, sn76489.Sega)
	psg.SetGain(1898.0)

	// PSG: single tone
	psg.Write(0x80 | 0x0E)
	psg.Write(0x0F)
	psg.Write(0x90 | 0x00)
	psg.Write(0xBF)
	psg.Write(0xDF)
	psg.Write(0xFF)

	cyclesPerScanline := (7670454 / 60) / 262
	z80CyclesPerScanline := (3579545 / 60) / 262

	psg.ResetBuffer()
	// Run YM2612 for 200 scanlines but PSG for 262
	for i := 0; i < 262; i++ {
		if i < 200 {
			ym.GenerateSamples(cyclesPerScanline)
		}
		psg.Run(z80CyclesPerScanline)
	}

	ymBuf := ym.GetBuffer()
	psgBuf, psgCount := psg.GetBuffer()
	mixed := lowPassStereo(mixBuffers(ymBuf, psgBuf, psgCount))

	expectedFirst := []int16{
		896, 896, 1688, 1688, 2404, 2404, 3082, 3082,
		3715, 3715, 4298, 4298, 4857, 4857, 5265, 5265,
		5562, 5562, 5779, 5779, 5937, 5937, 6052, 6052,
		6136, 6136, 6197, 6197, 6242, 6242, 6275, 6275,
		6298, 6298, 6316, 6316, 6328, 6328, 6337, 6337,
		6344, 6344, 6349, 6349, 6352, 6352, 6355, 6355,
		6357, 6357, 6358, 6358, 6359, 6359, 6360, 6360,
		6361, 6361, 6361, 6361, 6361, 6361, 6361, 6361,
	}
	expectedHash := "19f1c7cf336c381305f59acf4f0e4f6459d9c232efe3aedab3f65b60709574d0"

	compareGoldenInt16(t, "Mix_ExtraPSGSamples", mixed, expectedFirst, expectedHash)
}

func TestMixGolden_AsymmetricStereo(t *testing.T) {
	// YM2612 ch0 algo 7 panned left-only + PSG mono tone
	ym := setupTestChannel(7)
	// Override pan to left-only
	ym.WritePort(0, 0xB4)
	ym.WritePort(1, 0x80) // panL=true, panR=false

	psg := sn76489.New(3579545, 48000, 1024, sn76489.Sega)
	psg.SetGain(1898.0)

	// PSG: ch0 ~440Hz, vol=0 (max), others silent
	psg.Write(0x80 | 0x0E)
	psg.Write(0x0F)
	psg.Write(0x90 | 0x00)
	psg.Write(0xBF)
	psg.Write(0xDF)
	psg.Write(0xFF)

	// Generate one frame per-scanline
	cyclesPerScanline := (7670454 / 60) / 262
	z80CyclesPerScanline := (3579545 / 60) / 262
	psg.ResetBuffer()
	for i := 0; i < 262; i++ {
		ym.GenerateSamples(cyclesPerScanline)
		psg.Run(z80CyclesPerScanline)
	}

	ymBuf := ym.GetBuffer()
	psgBuf, psgCount := psg.GetBuffer()
	mixed := lowPassStereo(mixBuffers(ymBuf, psgBuf, psgCount))

	expectedFirst := []int16{
		896, 618, 1688, 1069, 2404, 1398, 3082, 1638,
		3715, 1812, 4298, 1940, 4857, 2032, 5265, 2100,
		5562, 2149, 5779, 2185, 5937, 2211, 6052, 2231,
		6136, 2245, 6197, 2255, 6242, 2262, 6275, 2267,
		6298, 2271, 6316, 2274, 6328, 2276, 6337, 2278,
		6344, 2279, 6349, 2280, 6352, 2280, 6355, 2281,
		6357, 2281, 6358, 2281, 6359, 2282, 6360, 2282,
		6361, 2282, 6361, 2282, 6361, 2282, 6361, 2282,
	}
	expectedHash := "67ce41eaf2958a401fbc0c5161e050f92d20550e5d406cb75591a31e62d7252c"

	compareGoldenInt16(t, "Mix_AsymmetricStereo", mixed, expectedFirst, expectedHash)
}
