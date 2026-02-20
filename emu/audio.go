package emu

import "math"

const (
	sampleRate    = 48000
	psgBufferSize = 1024
	psgGain       = 1898.0
	lpfCutoffHz   = 2840.0
)

// lpfAlpha is the smoothing factor for the first-order RC low-pass filter.
// Derived from: alpha = dt / (RC + dt) where RC = 1/(2*pi*fc).
var lpfAlpha = 1.0 / (float64(sampleRate)/(2*math.Pi*lpfCutoffHz) + 1)

// mixAudio collects YM2612 and PSG output buffers and mixes them into
// the emulator's stereo audio buffer. YM2612 produces stereo L/R pairs,
// PSG produces mono samples that are duplicated to both channels.
func (e *Emulator) mixAudio() {
	ym2612Samples := e.ym2612.GetBuffer()
	psgBuf, psgCount := e.psg.GetBuffer()

	ymPairs := len(ym2612Samples) / 2
	mixCount := ymPairs
	if psgCount < mixCount {
		mixCount = psgCount
	}

	for i := 0; i < mixCount; i++ {
		fmL := int32(ym2612Samples[i*2])
		fmR := int32(ym2612Samples[i*2+1])
		psgVal := int32(psgBuf[i])
		mixL := clampInt32(fmL+psgVal, -32768, 32767)
		mixR := clampInt32(fmR+psgVal, -32768, 32767)
		e.audioBuffer = append(e.audioBuffer, int16(mixL), int16(mixR))
	}

	// Append any remaining YM2612 stereo samples
	if ymPairs > mixCount {
		e.audioBuffer = append(e.audioBuffer, ym2612Samples[mixCount*2:]...)
	}

	// Append any remaining PSG samples as stereo (mono duplicated to L/R)
	for i := mixCount; i < psgCount; i++ {
		s := int16(psgBuf[i])
		e.audioBuffer = append(e.audioBuffer, s, s)
	}

	e.applyLowPass()
}

// applyLowPass applies a first-order RC low-pass filter to the audio
// buffer. This emulates the Model 1 VA3 motherboard filter (fc ~= 2840 Hz,
// 20 dB/decade rolloff). Applied per stereo channel with state persisting
// across frames.
func (e *Emulator) applyLowPass() {
	for i := 0; i < len(e.audioBuffer); i += 2 {
		inL := float64(e.audioBuffer[i])
		inR := float64(e.audioBuffer[i+1])
		e.filterPrevL = lpfAlpha*inL + (1-lpfAlpha)*e.filterPrevL
		e.filterPrevR = lpfAlpha*inR + (1-lpfAlpha)*e.filterPrevR
		e.audioBuffer[i] = int16(math.Round(e.filterPrevL))
		e.audioBuffer[i+1] = int16(math.Round(e.filterPrevR))
	}
}

// GetAudioSamples returns accumulated audio samples as 16-bit stereo PCM.
func (e *Emulator) GetAudioSamples() []int16 {
	return e.audioBuffer
}

// clampInt32 clamps v to [min, max].
func clampInt32(v, min, max int32) int32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
