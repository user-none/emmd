// Package cli provides a command-line runner for the emulator.
// It handles input polling and runs the emulator in a window without the full UI.
package cli

import (
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	emubridge "github.com/user-none/emmd/bridge/ebiten"
	"github.com/user-none/emmd/ui"
)

// ADT buffer thresholds in bytes.
const (
	adtMinBuffer = 9600
	adtMaxBuffer = 19200
)

// Runner wraps an emulator for command-line mode.
// The emulator runs on a dedicated goroutine with audio-driven timing.
// The Ebiten thread handles input polling and rendering from the shared framebuffer.
type Runner struct {
	emulator    *emubridge.Emulator
	audioPlayer *ui.AudioPlayer

	// ADT goroutine control
	emuControl        *ui.EmuControl
	sharedInput       *ui.SharedInput
	sharedFramebuffer *ui.SharedFramebuffer
	emuDone           chan struct{}
}

// NewRunner creates a new Runner wrapping the given emulator.
// Audio initialization failure is non-fatal; the runner will work without sound.
func NewRunner(e *emubridge.Emulator) *Runner {
	player, err := ui.NewAudioPlayer(1.0)
	if err != nil {
		log.Printf("Warning: audio initialization failed: %v", err)
	}

	r := &Runner{
		emulator:          e,
		audioPlayer:       player,
		emuControl:        ui.NewEmuControl(),
		sharedInput:       &ui.SharedInput{},
		sharedFramebuffer: ui.NewSharedFramebuffer(),
		emuDone:           make(chan struct{}),
	}

	// Start emulation goroutine
	go r.emulationLoop()

	return r
}

// Close cleans up the runner's resources.
func (r *Runner) Close() {
	// Stop emulation goroutine
	if r.emuControl != nil {
		r.emuControl.Stop()
		<-r.emuDone
	}

	if r.audioPlayer != nil {
		r.audioPlayer.Close()
		r.audioPlayer = nil
	}
}

// emulationLoop runs on a dedicated goroutine with ADT.
func (r *Runner) emulationLoop() {
	defer close(r.emuDone)

	timing := r.emulator.GetTiming()
	frameTime := time.Duration(float64(time.Second) / float64(timing.FPS))
	lastFrameTime := time.Now()

	for {
		if !r.emuControl.CheckPause() {
			return
		}

		// Read input from shared state
		up, down, left, right, btnA, btnB, btnC, start, btnX, btnY, btnZ, btnMode := r.sharedInput.Read()
		r.emulator.SetInput(up, down, left, right, btnA, btnB, btnC, start, btnX, btnY, btnZ, btnMode)

		// Run one frame
		r.emulator.RunFrame()

		// Queue audio
		if r.audioPlayer != nil {
			r.audioPlayer.QueueSamples(r.emulator.GetAudioSamples())
		}

		// Update shared framebuffer
		r.sharedFramebuffer.Update(
			r.emulator.GetFramebuffer(),
			r.emulator.GetFramebufferStride(),
			r.emulator.GetActiveHeight(),
		)

		// ADT sleep
		elapsed := time.Since(lastFrameTime)
		sleepTime := frameTime - elapsed

		if r.audioPlayer != nil {
			bufferLevel := r.audioPlayer.GetBufferLevel()
			if bufferLevel < adtMinBuffer {
				sleepTime = time.Duration(float64(sleepTime) * 0.9)
			} else if bufferLevel > adtMaxBuffer {
				sleepTime = time.Duration(float64(sleepTime) * 1.1)
			}
		}

		if sleepTime > time.Millisecond {
			time.Sleep(sleepTime)
		}

		lastFrameTime = time.Now()
	}
}

// Update implements ebiten.Game.
func (r *Runner) Update() error {
	if !ebiten.IsFocused() {
		return nil
	}

	r.pollInputToShared()
	return nil
}

// Draw implements ebiten.Game.
func (r *Runner) Draw(screen *ebiten.Image) {
	pixels, stride, height := r.sharedFramebuffer.Read()
	if height == 0 {
		return
	}
	r.emulator.DrawCachedFramebuffer(screen, pixels, stride, height)
}

// Layout implements ebiten.Game.
func (r *Runner) Layout(outsideWidth, outsideHeight int) (int, int) {
	return r.emulator.Layout(outsideWidth, outsideHeight)
}

// pollInputToShared reads keyboard and gamepad input and writes to shared state.
func (r *Runner) pollInputToShared() {
	// Keyboard (WASD + arrows for movement, JKL for A/B/C, Enter for Start)
	up := ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyArrowUp)
	down := ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyArrowDown)
	left := ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft)
	right := ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight)
	btnA := ebiten.IsKeyPressed(ebiten.KeyJ)
	btnB := ebiten.IsKeyPressed(ebiten.KeyK)
	btnC := ebiten.IsKeyPressed(ebiten.KeyL)
	start := ebiten.IsKeyPressed(ebiten.KeyEnter)
	btnX := ebiten.IsKeyPressed(ebiten.KeyU)
	btnY := ebiten.IsKeyPressed(ebiten.KeyI)
	btnZ := ebiten.IsKeyPressed(ebiten.KeyO)
	btnMode := ebiten.IsKeyPressed(ebiten.KeyP)

	// Gamepad support (all connected gamepads)
	for _, id := range ebiten.AppendGamepadIDs(nil) {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}

		// D-pad
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftTop) {
			up = true
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftBottom) {
			down = true
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftLeft) {
			left = true
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftRight) {
			right = true
		}

		// Face buttons: A/Cross=A, B/Circle=B, X/Square=C, Start=Start
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightBottom) {
			btnA = true
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightRight) {
			btnB = true
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightLeft) {
			btnC = true
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonCenterRight) {
			start = true
		}

		// 6-button extra: LB=X, RB=Y, Y/Triangle=Z, Select/Back=Mode
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonFrontTopLeft) {
			btnX = true
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonFrontTopRight) {
			btnY = true
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightTop) {
			btnZ = true
		}
		if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonCenterLeft) {
			btnMode = true
		}

		// Left analog stick (with deadzone)
		const deadzone = 0.5
		axisX := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		axisY := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		if axisX < -deadzone {
			left = true
		}
		if axisX > deadzone {
			right = true
		}
		if axisY < -deadzone {
			up = true
		}
		if axisY > deadzone {
			down = true
		}
	}

	r.sharedInput.Set(up, down, left, right, btnA, btnB, btnC, start, btnX, btnY, btnZ, btnMode)
}
