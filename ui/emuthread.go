package ui

import (
	"sync"
	"time"

	"github.com/user-none/emmd/emu"
)

// SharedInput holds controller state written by the Ebiten thread
// and read by the emulation goroutine.
type SharedInput struct {
	mu                                      sync.Mutex
	up, down, left, right, btnA, btnB, btnC bool
	btnX, btnY, btnZ, btnMode               bool
	start                                   bool
}

// Set updates directional and button state from the Ebiten thread.
func (si *SharedInput) Set(up, down, left, right, btnA, btnB, btnC, start, btnX, btnY, btnZ, btnMode bool) {
	si.mu.Lock()
	si.up = up
	si.down = down
	si.left = left
	si.right = right
	si.btnA = btnA
	si.btnB = btnB
	si.btnC = btnC
	si.start = start
	si.btnX = btnX
	si.btnY = btnY
	si.btnZ = btnZ
	si.btnMode = btnMode
	si.mu.Unlock()
}

// Read returns the current input state.
func (si *SharedInput) Read() (up, down, left, right, btnA, btnB, btnC, start, btnX, btnY, btnZ, btnMode bool) {
	si.mu.Lock()
	up = si.up
	down = si.down
	left = si.left
	right = si.right
	btnA = si.btnA
	btnB = si.btnB
	btnC = si.btnC
	start = si.start
	btnX = si.btnX
	btnY = si.btnY
	btnZ = si.btnZ
	btnMode = si.btnMode
	si.mu.Unlock()
	return
}

// SharedFramebuffer holds pixel data written by the emulation goroutine
// and read by Ebiten's Draw() method. Uses separate write and read buffers
// so the emu goroutine can write new data while Draw uses the read copy.
type SharedFramebuffer struct {
	mu           sync.Mutex
	writePixels  []byte // Written by emu goroutine under lock
	readPixels   []byte // Snapshot copied on Read for safe external use
	stride       int
	activeHeight int
}

// NewSharedFramebuffer creates a pre-allocated framebuffer.
func NewSharedFramebuffer() *SharedFramebuffer {
	return &SharedFramebuffer{
		writePixels: make([]byte, emu.ScreenWidth*emu.MaxScreenHeight*4),
		readPixels:  make([]byte, emu.ScreenWidth*emu.MaxScreenHeight*4),
	}
}

// Update copies framebuffer data from the emulation goroutine.
func (sf *SharedFramebuffer) Update(pixels []byte, stride, activeHeight int) {
	sf.mu.Lock()
	n := stride * activeHeight
	if n > len(sf.writePixels) {
		n = len(sf.writePixels)
	}
	if n > len(pixels) {
		n = len(pixels)
	}
	copy(sf.writePixels[:n], pixels[:n])
	sf.stride = stride
	sf.activeHeight = activeHeight
	sf.mu.Unlock()
}

// Read returns a snapshot of the current framebuffer state.
// Copies the write buffer into the read buffer under the lock,
// then returns the read buffer which is safe to use without holding the lock.
func (sf *SharedFramebuffer) Read() (pixels []byte, stride, activeHeight int) {
	sf.mu.Lock()
	stride = sf.stride
	activeHeight = sf.activeHeight
	n := stride * activeHeight
	if n > len(sf.writePixels) {
		n = len(sf.writePixels)
	}
	if n > 0 {
		copy(sf.readPixels[:n], sf.writePixels[:n])
	}
	pixels = sf.readPixels
	sf.mu.Unlock()
	return
}

// EmuControl manages pause/resume/stop coordination between
// the Ebiten thread and the emulation goroutine.
type EmuControl struct {
	mu       sync.Mutex
	pauseReq bool
	paused   bool
	running  bool
	stopReq  bool
	ackCh    chan struct{}
}

// NewEmuControl creates a new emulation control.
func NewEmuControl() *EmuControl {
	return &EmuControl{
		running: true,
		ackCh:   make(chan struct{}, 1),
	}
}

// RequestPause asks the emulation goroutine to pause and blocks
// until it acknowledges the pause.
func (ec *EmuControl) RequestPause() {
	ec.mu.Lock()
	if ec.paused || ec.pauseReq {
		ec.mu.Unlock()
		return
	}
	ec.pauseReq = true
	ec.mu.Unlock()

	// Wait for emu goroutine to acknowledge
	<-ec.ackCh
}

// RequestResume tells the emulation goroutine to resume.
func (ec *EmuControl) RequestResume() {
	ec.mu.Lock()
	ec.pauseReq = false
	ec.paused = false
	ec.mu.Unlock()
}

// CheckPause is called by the emulation goroutine between frames.
// If a pause has been requested, it sends an acknowledgment and
// spins until resumed or stopped. Returns false if the goroutine
// should exit.
func (ec *EmuControl) CheckPause() bool {
	ec.mu.Lock()
	if !ec.running || ec.stopReq {
		ec.mu.Unlock()
		return false
	}
	if !ec.pauseReq {
		ec.mu.Unlock()
		return true
	}

	// Acknowledge pause request
	ec.paused = true
	ec.mu.Unlock()

	// Non-blocking send of ack (buffer size 1)
	select {
	case ec.ackCh <- struct{}{}:
	default:
	}

	// Spin-wait until resumed or stopped
	for {
		ec.mu.Lock()
		if !ec.running || ec.stopReq {
			ec.mu.Unlock()
			return false
		}
		if !ec.pauseReq {
			ec.paused = false
			ec.mu.Unlock()
			return true
		}
		ec.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
}

// Stop signals the emulation goroutine to exit.
func (ec *EmuControl) Stop() {
	ec.mu.Lock()
	ec.running = false
	ec.stopReq = true
	// Also clear pause so CheckPause unblocks
	ec.pauseReq = false
	ec.mu.Unlock()
}

// ShouldRun returns true if the goroutine should continue running.
func (ec *EmuControl) ShouldRun() bool {
	ec.mu.Lock()
	r := ec.running && !ec.stopReq
	ec.mu.Unlock()
	return r
}

// IsPaused returns true if the emulation goroutine is currently paused.
func (ec *EmuControl) IsPaused() bool {
	ec.mu.Lock()
	p := ec.paused
	ec.mu.Unlock()
	return p
}
