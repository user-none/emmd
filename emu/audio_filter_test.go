package emu

import (
	"math"
	"testing"
)

func TestLowPass_StepResponse(t *testing.T) {
	e := &Emulator{
		audioBuffer: make([]int16, 0, 64),
	}
	// Fill buffer with constant 1000 on both channels
	for i := 0; i < 32; i++ {
		e.audioBuffer = append(e.audioBuffer, 1000, 1000)
	}

	e.applyLowPass()

	// First sample: alpha * 1000 + (1-alpha) * 0 = alpha * 1000
	expected0 := int16(math.Round(lpfAlpha * 1000))
	if e.audioBuffer[0] != expected0 {
		t.Errorf("sample 0 L: got %d, want %d", e.audioBuffer[0], expected0)
	}
	if e.audioBuffer[1] != expected0 {
		t.Errorf("sample 0 R: got %d, want %d", e.audioBuffer[1], expected0)
	}

	// Each successive sample should be larger than the previous (ramping up)
	for i := 2; i < len(e.audioBuffer); i += 2 {
		if e.audioBuffer[i] < e.audioBuffer[i-2] {
			t.Errorf("sample %d L (%d) < sample %d L (%d): expected monotonic ramp",
				i/2, e.audioBuffer[i], i/2-1, e.audioBuffer[i-2])
			break
		}
	}
}

func TestLowPass_Silence(t *testing.T) {
	e := &Emulator{
		audioBuffer: make([]int16, 64),
	}

	e.applyLowPass()

	for i, v := range e.audioBuffer {
		if v != 0 {
			t.Errorf("sample %d: got %d, want 0", i, v)
			break
		}
	}
}

func TestLowPass_SteadyState(t *testing.T) {
	e := &Emulator{
		audioBuffer: make([]int16, 0, 2000),
	}
	// Fill with enough constant samples for convergence
	for i := 0; i < 1000; i++ {
		e.audioBuffer = append(e.audioBuffer, 500, 500)
	}

	e.applyLowPass()

	// Last sample should be very close to 500
	lastL := e.audioBuffer[len(e.audioBuffer)-2]
	lastR := e.audioBuffer[len(e.audioBuffer)-1]
	if lastL != 500 {
		t.Errorf("steady state L: got %d, want 500", lastL)
	}
	if lastR != 500 {
		t.Errorf("steady state R: got %d, want 500", lastR)
	}
}

func TestLowPass_NegativeStep(t *testing.T) {
	e := &Emulator{
		audioBuffer: make([]int16, 0, 64),
	}
	for i := 0; i < 32; i++ {
		e.audioBuffer = append(e.audioBuffer, -1000, -1000)
	}

	e.applyLowPass()

	// First sample: alpha * -1000
	expected0 := int16(math.Round(lpfAlpha * -1000))
	if e.audioBuffer[0] != expected0 {
		t.Errorf("sample 0 L: got %d, want %d", e.audioBuffer[0], expected0)
	}

	// Each successive sample should be more negative (ramping down)
	for i := 2; i < len(e.audioBuffer); i += 2 {
		if e.audioBuffer[i] > e.audioBuffer[i-2] {
			t.Errorf("sample %d L (%d) > sample %d L (%d): expected monotonic ramp down",
				i/2, e.audioBuffer[i], i/2-1, e.audioBuffer[i-2])
			break
		}
	}
}

func TestLowPass_StatePersistence(t *testing.T) {
	// Run filter on two consecutive buffers and verify continuity
	e := &Emulator{
		audioBuffer: make([]int16, 0, 64),
	}

	// First buffer: constant 1000
	for i := 0; i < 16; i++ {
		e.audioBuffer = append(e.audioBuffer, 1000, 1000)
	}
	e.applyLowPass()
	lastL := e.audioBuffer[len(e.audioBuffer)-2]
	lastR := e.audioBuffer[len(e.audioBuffer)-1]

	// Second buffer: same constant, state carries over
	e.audioBuffer = e.audioBuffer[:0]
	for i := 0; i < 16; i++ {
		e.audioBuffer = append(e.audioBuffer, 1000, 1000)
	}
	e.applyLowPass()
	firstL := e.audioBuffer[0]
	firstR := e.audioBuffer[1]

	// First sample of second buffer should be >= last of first buffer
	// (continuing ramp, not resetting to alpha*1000)
	if firstL < lastL {
		t.Errorf("state not persisted L: second buf first %d < first buf last %d", firstL, lastL)
	}
	if firstR < lastR {
		t.Errorf("state not persisted R: second buf first %d < first buf last %d", firstR, lastR)
	}

	// Verify it's not just equal to a fresh start
	freshFirst := int16(math.Round(lpfAlpha * 1000))
	if firstL == freshFirst {
		t.Errorf("L appears to have reset: got %d (same as fresh alpha*1000)", firstL)
	}
}
