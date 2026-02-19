package emu

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/user-none/go-chip-sn76489"
)

// hashFloat32Buffer computes SHA-256 of count float32 values from buf.
func hashFloat32Buffer(buf []float32, count int) [32]byte {
	b := make([]byte, count*4)
	for i := 0; i < count; i++ {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(buf[i]))
	}
	return sha256.Sum256(b)
}

// compareGoldenFloat32 compares the first N samples and the full-buffer SHA-256 hash.
func compareGoldenFloat32(t *testing.T, name string, buf []float32, count int, expectedFirst []float32, expectedHash string) {
	t.Helper()

	hash := hashFloat32Buffer(buf, count)
	hashStr := fmt.Sprintf("%x", hash)

	if *update {
		fmt.Printf("=== %s ===\n", name)
		fmt.Printf("// Buffer count: %d\n", count)
		n := 32
		if count < n {
			n = count
		}
		fmt.Printf("expectedFirst := []float32{")
		for i := 0; i < n; i++ {
			if i > 0 {
				fmt.Print(", ")
			}
			if i%4 == 0 {
				fmt.Print("\n\t")
			}
			fmt.Printf("%v", buf[i])
		}
		fmt.Printf(",\n}\n")
		fmt.Printf("expectedHash := %q\n\n", hashStr)
		return
	}

	n := len(expectedFirst)
	if count < n {
		t.Fatalf("%s: buffer too short: got %d, want at least %d", name, count, n)
	}
	for i := 0; i < n; i++ {
		if buf[i] != expectedFirst[i] {
			t.Errorf("%s: sample[%d] = %v, want %v", name, i, buf[i], expectedFirst[i])
			break
		}
	}

	if hashStr != expectedHash {
		t.Errorf("%s: hash mismatch\n  got:  %s\n  want: %s", name, hashStr, expectedHash)
	}
}

// newTestPSG creates a PSG matching emulator config.
func newTestPSG() *sn76489.SN76489 {
	psg := sn76489.New(3579545, 48000, 1024, sn76489.Sega)
	psg.SetGain(1898.0)
	return psg
}

// runPSGFrame runs the PSG for one NTSC frame (262 scanlines).
func runPSGFrame(psg *sn76489.SN76489) {
	z80CyclesPerScanline := (3579545 / 60) / 262
	for i := 0; i < 262; i++ {
		psg.Run(z80CyclesPerScanline)
	}
}

// --- PSG Golden Tests ---

func TestPSGGolden_SingleTone(t *testing.T) {
	psg := newTestPSG()

	// Ch0: tone freq register ~440Hz, vol=0 (max)
	// Tone register value for ~440Hz: 3579545 / (32 * 440) = ~254
	// 254 = 0x0FE -> low4=0xE, high6=0x0F
	psg.Write(0x80 | 0x0E) // Latch ch0 tone, low4=0xE
	psg.Write(0x0F)        // Data: high6=0x0F (tone reg = 0x0FE = 254)
	psg.Write(0x90 | 0x00) // Latch ch0 vol = 0 (max)

	// Silence other channels
	psg.Write(0xBF) // Ch1 vol = 15 (off)
	psg.Write(0xDF) // Ch2 vol = 15 (off)
	psg.Write(0xFF) // Ch3 (noise) vol = 15 (off)

	psg.ResetBuffer()
	runPSGFrame(psg)

	buf, count := psg.GetBuffer()

	expectedFirst := []float32{
		1898, 1898, 1898, 1898,
		1898, 1898, 1898, 1898,
		1898, 1898, 1898, 1898,
		1898, 1898, 1898, 1898,
		1898, 1898, 1898, 1898,
		1898, 1898, 1898, 1898,
		1898, 1898, 1898, 1898,
		1898, 1898, 1898, 1898,
	}
	expectedHash := "4ae74fc18108249eecd43fbfaa05b711f9ac0fca889b9e37e861c51444b936d3"

	compareGoldenFloat32(t, "PSG_SingleTone", buf, count, expectedFirst, expectedHash)
}

func TestPSGGolden_ThreeTones(t *testing.T) {
	psg := newTestPSG()

	// Ch0: ~440Hz, vol=0
	psg.Write(0x80 | 0x0E) // Latch ch0 tone, low4=0xE
	psg.Write(0x0F)        // high6=0x0F -> 254
	psg.Write(0x90 | 0x00) // Ch0 vol=0

	// Ch1: ~880Hz (half period -> 127)
	// 127 = 0x7F -> low4=0xF, high6=0x07
	psg.Write(0xA0 | 0x0F) // Latch ch1 tone, low4=0xF
	psg.Write(0x07)        // high6=0x07 -> 127
	psg.Write(0xB0 | 0x04) // Ch1 vol=4

	// Ch2: ~220Hz (double period -> 508)
	// 508 = 0x1FC -> low4=0xC, high6=0x1F
	psg.Write(0xC0 | 0x0C) // Latch ch2 tone, low4=0xC
	psg.Write(0x1F)        // high6=0x1F -> 508
	psg.Write(0xD0 | 0x08) // Ch2 vol=8

	// Silence noise
	psg.Write(0xFF) // Ch3 vol=15

	psg.ResetBuffer()
	runPSGFrame(psg)

	buf, count := psg.GetBuffer()

	expectedFirst := []float32{
		2954.4202, 2954.4202, 2954.4202, 2954.4202,
		2954.4202, 2954.4202, 2954.4202, 2954.4202,
		2954.4202, 2954.4202, 2954.4202, 2954.4202,
		2954.4202, 2954.4202, 2954.4202, 2954.4202,
		2954.4202, 2954.4202, 2954.4202, 2954.4202,
		2954.4202, 2954.4202, 2954.4202, 2954.4202,
		2954.4202, 2954.4202, 2954.4202, 2198.8127,
		2198.8127, 2198.8127, 2198.8127, 2198.8127,
	}
	expectedHash := "11be169f537383d73632fc2c3f58801c6e8f20082dc6217e2202ab833ae592e9"

	compareGoldenFloat32(t, "PSG_ThreeTones", buf, count, expectedFirst, expectedHash)
}

func TestPSGGolden_PeriodicNoise(t *testing.T) {
	psg := newTestPSG()

	// Silence tone channels
	psg.Write(0x9F) // Ch0 vol=15
	psg.Write(0xBF) // Ch1 vol=15
	psg.Write(0xDF) // Ch2 vol=15

	// Periodic noise, rate=1
	psg.Write(0xE0 | 0x01) // Periodic, rate=1
	psg.Write(0xF0 | 0x00) // Noise vol=0 (max)

	psg.ResetBuffer()
	runPSGFrame(psg)

	buf, count := psg.GetBuffer()

	expectedFirst := []float32{
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	}
	expectedHash := "9a15562ff455829b79e512d05edc69a63c64a1140684d4961287b09b8180666e"

	compareGoldenFloat32(t, "PSG_PeriodicNoise", buf, count, expectedFirst, expectedHash)
}

func TestPSGGolden_WhiteNoise(t *testing.T) {
	psg := newTestPSG()

	// Silence tone channels
	psg.Write(0x9F) // Ch0 vol=15
	psg.Write(0xBF) // Ch1 vol=15
	psg.Write(0xDF) // Ch2 vol=15

	// White noise, rate=2
	psg.Write(0xE0 | 0x04 | 0x02) // White noise (bit2=1), rate=2
	psg.Write(0xF0 | 0x00)        // Noise vol=0 (max)

	psg.ResetBuffer()
	runPSGFrame(psg)

	buf, count := psg.GetBuffer()

	expectedFirst := []float32{
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	}
	expectedHash := "afb015c0a30dc45bae1e7dc93ace747d06b8ec59225b1b33157ecee88bdb69a6"

	compareGoldenFloat32(t, "PSG_WhiteNoise", buf, count, expectedFirst, expectedHash)
}

func TestPSGGolden_Silence(t *testing.T) {
	psg := newTestPSG()

	// All channels vol=15 (off)
	psg.Write(0x9F) // Ch0 vol=15
	psg.Write(0xBF) // Ch1 vol=15
	psg.Write(0xDF) // Ch2 vol=15
	psg.Write(0xFF) // Ch3 vol=15

	psg.ResetBuffer()
	runPSGFrame(psg)

	buf, count := psg.GetBuffer()

	expectedFirst := []float32{
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	}
	expectedHash := "2d9247f519c6a738da65184abf881cd246d10ae28a978011c5f43d4ea3609e3c"

	compareGoldenFloat32(t, "PSG_Silence", buf, count, expectedFirst, expectedHash)
}
