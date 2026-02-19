package ui

import (
	"fmt"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

const audioSampleRate = 48000

// ringBufferCapacity is ~167ms at 48kHz stereo 16-bit (~32KB).
const ringBufferCapacity = 32768

// AudioPlayer manages audio playback via oto.
// It writes int16 stereo samples to a ring buffer which oto's player
// reads from in a pull model.
type AudioPlayer struct {
	player     *oto.Player
	ringBuffer *AudioRingBuffer
	audioBytes []byte // Pre-allocated buffer for int16-to-byte conversion
}

// oto context singleton
var (
	otoCtx      *oto.Context
	otoInitOnce sync.Once
	otoInitErr  error
)

// ensureOtoContext initializes the oto audio context on first use.
func ensureOtoContext() (*oto.Context, error) {
	otoInitOnce.Do(func() {
		op := &oto.NewContextOptions{
			SampleRate:   audioSampleRate,
			ChannelCount: 2,
			Format:       oto.FormatSignedInt16LE,
			BufferSize:   50 * time.Millisecond,
		}
		var readyChan chan struct{}
		otoCtx, readyChan, otoInitErr = oto.NewContext(op)
		if otoInitErr != nil {
			return
		}
		<-readyChan
	})
	return otoCtx, otoInitErr
}

// NewAudioPlayer creates and initializes audio playback via oto.
func NewAudioPlayer(volume float64) (*AudioPlayer, error) {
	ctx, err := ensureOtoContext()
	if err != nil {
		return nil, fmt.Errorf("oto audio not available: %w", err)
	}

	rb := NewAudioRingBuffer(ringBufferCapacity)
	player := ctx.NewPlayer(rb)
	player.SetBufferSize(19200)
	player.SetVolume(volume)
	player.Play()

	return &AudioPlayer{
		player:     player,
		ringBuffer: rb,
		audioBytes: make([]byte, 0, 4096),
	}, nil
}

// QueueSamples converts int16 stereo samples to bytes and writes them
// to the ring buffer for oto to consume.
func (a *AudioPlayer) QueueSamples(samples []int16) {
	if len(samples) == 0 {
		return
	}

	// Convert int16 samples to little-endian bytes using pre-allocated buffer
	needed := len(samples) * 2
	if cap(a.audioBytes) < needed {
		a.audioBytes = make([]byte, 0, needed)
	}
	a.audioBytes = a.audioBytes[:0]
	for _, sample := range samples {
		a.audioBytes = append(a.audioBytes, byte(sample), byte(sample>>8))
	}

	a.ringBuffer.Write(a.audioBytes)
}

// GetBufferLevel returns the total bytes of audio data currently buffered
// (ring buffer + oto player internal buffer). Used for ADT pacing.
func (a *AudioPlayer) GetBufferLevel() int {
	return a.ringBuffer.Buffered() + a.player.BufferedSize()
}

// SetVolume sets the playback volume (0.0 = silent, 1.0 = full).
func (a *AudioPlayer) SetVolume(vol float64) {
	a.player.SetVolume(vol)
}

// Close cleans up audio resources.
func (a *AudioPlayer) Close() {
	if a.ringBuffer != nil {
		a.ringBuffer.Close()
	}
	if a.player != nil {
		a.player.Close()
	}
}
