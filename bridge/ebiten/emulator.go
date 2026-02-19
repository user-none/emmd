// Package ebiten provides an Ebiten-specific wrapper for the emulator.
package ebiten

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/user-none/emmd/emu"
)

// Emulator wraps emu.EmulatorBase with Ebiten-specific functionality
type Emulator struct {
	emu.EmulatorBase

	offscreen *ebiten.Image           // Offscreen buffer for native resolution rendering
	drawOpts  ebiten.DrawImageOptions // Pre-allocated draw options to avoid per-frame allocation
}

// NewEmulator creates a new emulator instance with Ebiten rendering.
func NewEmulator(rom []byte, region emu.Region) (*Emulator, error) {
	base, err := emu.InitEmulatorBase(rom, region)
	if err != nil {
		return nil, err
	}

	return &Emulator{
		EmulatorBase: base,
	}, nil
}

// Close cleans up the emulator resources.
func (e *Emulator) Close() {
}

// Layout implements ebiten.Game.
func (e *Emulator) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

// DrawCachedFramebuffer renders pre-cached pixel data to the screen.
// Used by the ADT architecture where the emulation goroutine writes pixels
// to a shared framebuffer, and the Ebiten Draw() thread renders them.
func (e *Emulator) DrawCachedFramebuffer(screen *ebiten.Image, pixels []byte, stride, activeHeight int) {
	if activeHeight == 0 || stride == 0 {
		return
	}

	requiredLen := stride * activeHeight
	if len(pixels) < requiredLen {
		return
	}

	// Create or resize offscreen buffer if needed
	if e.offscreen == nil || e.offscreen.Bounds().Dy() != activeHeight {
		e.offscreen = ebiten.NewImage(emu.ScreenWidth, activeHeight)
	}

	e.offscreen.WritePixels(pixels[:requiredLen])

	// Calculate scaling to fit window while preserving aspect ratio
	screenW, screenH := screen.Bounds().Dx(), screen.Bounds().Dy()
	nativeW := float64(emu.ScreenWidth)
	nativeH := float64(activeHeight)

	scaleX := float64(screenW) / nativeW
	scaleY := float64(screenH) / nativeH
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	scaledW := nativeW * scale
	scaledH := nativeH * scale
	offsetX := (float64(screenW) - scaledW) / 2
	offsetY := (float64(screenH) - scaledH) / 2

	e.drawOpts = ebiten.DrawImageOptions{}
	e.drawOpts.GeoM.Scale(scale, scale)
	e.drawOpts.GeoM.Translate(offsetX, offsetY)
	e.drawOpts.Filter = ebiten.FilterNearest
	screen.DrawImage(e.offscreen, &e.drawOpts)
}
