package emu

import (
	"crypto/sha256"
	"encoding/binary"
	"flag"
	"fmt"
	"testing"
)

var update = flag.Bool("update", false, "print golden data to stdout for copy-paste")

// hashInt16Buffer computes SHA-256 of a buffer of int16 values (little-endian).
func hashInt16Buffer(buf []int16) [32]byte {
	b := make([]byte, len(buf)*2)
	for i, v := range buf {
		binary.LittleEndian.PutUint16(b[i*2:], uint16(v))
	}
	return sha256.Sum256(b)
}

// compareGoldenInt16 compares the first N samples and the full-buffer SHA-256 hash.
// In update mode, it prints the golden data instead of comparing.
func compareGoldenInt16(t *testing.T, name string, buf []int16, expectedFirst []int16, expectedHash string) {
	t.Helper()

	hash := hashInt16Buffer(buf)
	hashStr := fmt.Sprintf("%x", hash)

	if *update {
		fmt.Printf("=== %s ===\n", name)
		fmt.Printf("// Buffer length: %d\n", len(buf))
		n := 64
		if len(buf) < n {
			n = len(buf)
		}
		fmt.Printf("expectedFirst := []int16{")
		for i := 0; i < n; i++ {
			if i > 0 {
				fmt.Print(", ")
			}
			if i%8 == 0 {
				fmt.Print("\n\t")
			}
			fmt.Printf("%d", buf[i])
		}
		fmt.Printf(",\n}\n")
		fmt.Printf("expectedHash := %q\n\n", hashStr)
		return
	}

	// Compare first N samples
	n := len(expectedFirst)
	if len(buf) < n {
		t.Fatalf("%s: buffer too short: got %d, want at least %d", name, len(buf), n)
	}
	for i := 0; i < n; i++ {
		if buf[i] != expectedFirst[i] {
			t.Errorf("%s: sample[%d] = %d, want %d", name, i, buf[i], expectedFirst[i])
			break
		}
	}

	// Compare full hash
	if hashStr != expectedHash {
		t.Errorf("%s: hash mismatch\n  got:  %s\n  want: %s", name, hashStr, expectedHash)
	}
}

// --- YM2612 Golden Tests ---

func TestYM2612Golden_Algo0Serial(t *testing.T) {
	y := setupTestChannel(0)
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		1920, 1920, -1360, -1360, -240, -240, -1808, -1808,
		-1120, -1120, -3744, -3744, -3600, -3600, -880, -880,
		2656, 2656, 2864, 2864, -592, -592, -1424, -1424,
		944, 944, 1360, 1360, 1184, 1184, 544, 544,
		3760, 3760, -3824, -3824, -3728, -3728, 2880, 2880,
		4288, 4288, -1152, -1152, 3936, 3936, -1056, -1056,
		1792, 1792, -1408, -1408, -3552, -3552, 3680, 3680,
		2640, 2640, 176, 176, 1664, 1664, -304, -304,
	}
	expectedHash := "63ab06d02d20d80a8553648649943d42595086bb8c2ef5b140a2beed17bdadcb"

	compareGoldenInt16(t, "YM2612_Algo0Serial", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_Algo4DualCarrier(t *testing.T) {
	y := setupTestChannel(4)
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		4464, 4464, 4464, 4464, -752, -752, -3816, -3816,
		-3816, -3816, -3664, -3664, 2720, 2720, 4464, 4464,
		4464, 4464, -2480, -2480, -3816, -3816, -3816, -3816,
		-3440, -3440, 2336, 2336, 4464, 4464, 4464, 4464,
		4464, 4464, 1056, 1056, -3816, -3816, -3816, -3816,
		-3816, -3816, -16, -16, 4192, 4192, 4464, 4464,
		4464, 4464, 4464, 4464, 4384, 4384, -2896, -2896,
		-3816, -3816, -3816, -3816, -3816, -3816, -3816, -3816,
	}
	expectedHash := "9e6a025c47ef1f1bf3617064066b4335cdeca77e719e24f85876d8297e4a4364"

	compareGoldenInt16(t, "YM2612_Algo4DualCarrier", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_Algo7Additive(t *testing.T) {
	y := setupTestChannel(7)
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		1408, 1408, 1920, 1920, 2432, 2432, 3008, 3008,
		3520, 3520, 3968, 3968, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "4737e8f317dc75ddbf7531da9fa74ca47ba346da50f23566dbc0d3399d9b4aad"

	compareGoldenInt16(t, "YM2612_Algo7Additive", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_DACMode(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable DAC and write max sample
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80) // DAC enable
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xFF) // Max DAC sample

	// Ch5 = Part II ch2 (reg offset +2). Pan is at 0xB4+chSlot.
	// Part II uses ports 2/3.
	y.WritePort(2, 0xB6) // 0xB4 + 2 = 0xB6
	y.WritePort(3, 0xC0) // L+R pan

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		4448, 4448, 4448, 4448, 4448, 4448, 4448, 4448,
		4448, 4448, 4448, 4448, 4448, 4448, 4448, 4448,
		4448, 4448, 4448, 4448, 4448, 4448, 4448, 4448,
		4448, 4448, 4448, 4448, 4448, 4448, 4448, 4448,
		4448, 4448, 4448, 4448, 4448, 4448, 4448, 4448,
		4448, 4448, 4448, 4448, 4448, 4448, 4448, 4448,
		4448, 4448, 4448, 4448, 4448, 4448, 4448, 4448,
		4448, 4448, 4448, 4448, 4448, 4448, 4448, 4448,
	}
	expectedHash := "f70604296d6cfa6702fd13d4b03232f948973a1856685254356ece7a8a2b1b66"

	compareGoldenInt16(t, "YM2612_DACMode", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_KeyOnOff(t *testing.T) {
	y := setupTestChannel(7)

	// Generate half frame with key on
	halfCycles := (7670454 / 60) / 2
	y.GenerateSamples(halfCycles)

	// Key off all operators
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00) // ch0, all ops off

	// Generate second half
	y.GenerateSamples(halfCycles)

	buf := y.GetBuffer()

	expectedFirst := []int16{
		1408, 1408, 1920, 1920, 2432, 2432, 3008, 3008,
		3520, 3520, 3968, 3968, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "d5075778ad6606332006bf03b911e9517ae2765653f6860f26e83d0a11cc0715"

	compareGoldenInt16(t, "YM2612_KeyOnOff", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_LFOWithPM(t *testing.T) {
	y := setupTestChannel(7)

	// Enable LFO with fastest frequency
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x0F) // LFO enable, freq=7

	// Set ch0 FMS=7 (max PM sensitivity)
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC7) // L+R, AMS=0, FMS=7

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		1408, 1408, 1920, 1920, 2432, 2432, 3008, 3008,
		3520, 3520, 3968, 3968, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "369a56dff88c116647b9dabd3da3e3c81db7416a92a690c1974161ad10c0acad"

	compareGoldenInt16(t, "YM2612_LFOWithPM", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_Ch3Special(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable ch3 special mode
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40) // Ch3 special mode

	// Ch2 (index 2) = channel 3 in YM2612 numbering
	// Algorithm 7, fb=0
	y.WritePort(0, 0xB2)
	y.WritePort(1, 0x07)

	// Pan L+R
	y.WritePort(0, 0xB6)
	y.WritePort(1, 0xC0)

	// Set per-operator frequencies for ch3 special mode
	// Slot 0 -> OP3 (opIdx 2): A8/AC
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x22) // block=4
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x9A) // fNum LSB

	// Slot 1 -> OP1 (opIdx 0): A9/AD
	y.WritePort(0, 0xAD)
	y.WritePort(1, 0x26) // block=4, different fNum
	y.WritePort(0, 0xA9)
	y.WritePort(1, 0x50)

	// Slot 2 -> OP2 (opIdx 1): AA/AE
	y.WritePort(0, 0xAE)
	y.WritePort(1, 0x2A) // block=5
	y.WritePort(0, 0xAA)
	y.WritePort(1, 0x00)

	// Channel frequency for OP4 (uses channel freq, not special mode)
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x1E) // block=3
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0xD0)

	// All operators: DT=0, MUL=1
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}
	// All operators: TL=0
	for _, reg := range []uint8{0x42, 0x46, 0x4A, 0x4E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// All operators: RS=3, AR=31
	for _, reg := range []uint8{0x52, 0x56, 0x5A, 0x5E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xDF)
	}
	// D1R=0
	for _, reg := range []uint8{0x62, 0x66, 0x6A, 0x6E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// D2R=0
	for _, reg := range []uint8{0x72, 0x76, 0x7A, 0x7E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// D1L=0, RR=15
	for _, reg := range []uint8{0x82, 0x86, 0x8A, 0x8E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x0F)
	}

	// Key on all operators for ch2 (channel 3)
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF2) // ch2 = 0x02, all ops on

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		2000, 2000, 2816, 2816, 3600, 3600, 4416, 4416,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "a46a92539bf7aae8d6e8c9dc77f28841196fccd0e21a85bd52d1fda7738964c9"

	compareGoldenInt16(t, "YM2612_Ch3Special", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_Algo1(t *testing.T) {
	y := setupTestChannel(1)
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		2512, 2512, 2112, 2112, -240, -240, -2960, -2960,
		1744, 1744, 4352, 4352, 4000, 4000, -2192, -2192,
		4464, 4464, 4432, 4432, 48, 48, -2832, -2832,
		3376, 3376, -464, -464, -1280, -1280, 1088, 1088,
		4432, 4432, 4384, 4384, 1808, 1808, 2736, 2736,
		1088, 1088, 2448, 2448, -3664, -3664, -2784, -2784,
		4272, 4272, -3568, -3568, 144, 144, 4464, 4464,
		1472, 1472, 2000, 2000, 4352, 4352, 3888, 3888,
	}
	expectedHash := "9fd85578e19603bc8d2a800731adc9dbe0ad2554ed228c9d62cfd41941b7bffc"

	compareGoldenInt16(t, "YM2612_Algo1", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_Algo2(t *testing.T) {
	y := setupTestChannel(2)
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		4432, 4432, -2928, -2928, 896, 896, 3696, 3696,
		-80, -80, 512, 512, 4192, 4192, -2928, -2928,
		32, 32, 4400, 4400, 3696, 3696, -1520, -1520,
		3184, 3184, 144, 144, 3920, 3920, 4464, 4464,
		2672, 2672, -3456, -3456, -672, -672, -1808, -1808,
		3984, 3984, -2880, -2880, -1072, -1072, -1968, -1968,
		4448, 4448, -3632, -3632, 3264, 3264, 1008, 1008,
		-3504, -3504, -1104, -1104, -3600, -3600, 4432, 4432,
	}
	expectedHash := "275de9cfff9adebe32b3ff86ad048cc46c239ba690b5f0f7f5eb070407cc5f69"

	compareGoldenInt16(t, "YM2612_Algo2", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_Algo3(t *testing.T) {
	y := setupTestChannel(3)
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		4432, 4432, -2928, -2928, 896, 896, 3696, 3696,
		-80, -80, 512, 512, 4192, 4192, -2928, -2928,
		32, 32, 4400, 4400, 3696, 3696, -1520, -1520,
		3184, 3184, 144, 144, 3920, 3920, 4464, 4464,
		2672, 2672, -3456, -3456, -672, -672, -1808, -1808,
		3984, 3984, -2880, -2880, -1072, -1072, -1968, -1968,
		4448, 4448, -3632, -3632, 3264, 3264, 1008, 1008,
		-3504, -3504, -1104, -1104, -3600, -3600, 4432, 4432,
	}
	expectedHash := "275de9cfff9adebe32b3ff86ad048cc46c239ba690b5f0f7f5eb070407cc5f69"

	compareGoldenInt16(t, "YM2612_Algo3", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_Algo5(t *testing.T) {
	y := setupTestChannel(5)
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		4464, 4464, 4464, 4464, -1264, -1264, -3816, -3816,
		-3816, -3816, -3816, -3816, 3888, 3888, 4464, 4464,
		4464, 4464, -3816, -3816, -3816, -3816, -3816, -3816,
		-3816, -3816, 3312, 3312, 4464, 4464, 4464, 4464,
		4464, 4464, 1392, 1392, -3816, -3816, -3816, -3816,
		-3816, -3816, -160, -160, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, -3816, -3816,
		-3816, -3816, -3816, -3816, -3816, -3816, -3816, -3816,
	}
	expectedHash := "98a6ce76545f3a21faa74fd2237c042667ebb8024173e4f105a726d043e58807"

	compareGoldenInt16(t, "YM2612_Algo5", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_Algo6(t *testing.T) {
	y := setupTestChannel(6)
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		4464, 4464, 3664, 3664, 896, 896, -2032, -2032,
		-2064, -2064, 96, 96, 3600, 3600, 4464, 4464,
		4464, 4464, 2048, 2048, -80, -80, -176, -176,
		2304, 2304, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 1840, 1840, 1888, 1888,
		3232, 3232, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4432, 4432, 3744, 3744, 3648, 3648, 4128, 4128,
	}
	expectedHash := "62f6f5587e852768fb57102ae86d1af4b5e1b3ff73ecc286a785e844ba425a3a"

	compareGoldenInt16(t, "YM2612_Algo6", buf, expectedFirst, expectedHash)
}

// --- YM2612 Parameter Golden Tests ---

func TestYM2612Golden_Feedback(t *testing.T) {
	y := setupTestChannel(0)
	// Set fb=7 (max feedback) on algo 0
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x38) // algo=0, fb=7

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		-3792, -3792, 1600, 1600, -1104, -1104, 4224, 4224,
		3872, 3872, 3152, 3152, -1584, -1584, 2048, 2048,
		-3696, -3696, 3856, 3856, -2544, -2544, 3056, 3056,
		3728, 3728, 3920, 3920, -960, -960, -3808, -3808,
		2816, 2816, 4336, 4336, -3696, -3696, 4352, 4352,
		256, 256, 3072, 3072, 4096, 4096, 4336, 4336,
		-880, -880, 1968, 1968, 2016, 2016, -1696, -1696,
		4064, 4064, 2704, 2704, 2864, 2864, 4096, 4096,
	}
	expectedHash := "c85028911d5ed12ccf27125a0f7f50fd417a35c209150faaff6c26352d08b41a"

	compareGoldenInt16(t, "YM2612_Feedback", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_LFOWithAM(t *testing.T) {
	y := setupTestChannel(7)

	// Enable LFO
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x0F) // LFO enable, freq=7 (fastest)

	// Set ch0 AMS=3 (max AM sensitivity)
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xF0) // L+R, AMS=3, FMS=0

	// Enable AM on all operators
	for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x80) // AM=1, D1R=0
	}

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		640, 640, 768, 768, 896, 896, 1024, 1024,
		1152, 1152, 1280, 1280, 1408, 1408, 1536, 1536,
		1728, 1728, 1984, 1984, 2112, 2112, 2240, 2240,
		2432, 2432, 2560, 2560, 2624, 2624, 2752, 2752,
		2880, 2880, 3072, 3072, 3328, 3328, 3392, 3392,
		3520, 3520, 3712, 3712, 3776, 3776, 3904, 3904,
		3968, 3968, 4096, 4096, 4224, 4224, 4416, 4416,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "427c06ff9cef9f7004a02ea7f4ed80191da7082beafaaf4d065e003e7fe1fff2"

	compareGoldenInt16(t, "YM2612_LFOWithAM", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_Detune(t *testing.T) {
	y := setupTestChannel(7)

	// Set different detune values per operator
	// OP1(slot0=0x30): DT=1, MUL=1
	y.WritePort(0, 0x30)
	y.WritePort(1, 0x11)
	// OP3(slot1=0x34): DT=2, MUL=1
	y.WritePort(0, 0x34)
	y.WritePort(1, 0x21)
	// OP2(slot2=0x38): DT=5 (negative DT=1), MUL=1
	y.WritePort(0, 0x38)
	y.WritePort(1, 0x51)
	// OP4(slot3=0x3C): DT=7 (negative DT=3), MUL=1
	y.WritePort(0, 0x3C)
	y.WritePort(1, 0x71)

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		1408, 1408, 1920, 1920, 2432, 2432, 2976, 2976,
		3520, 3520, 3968, 3968, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "c825bcfd3af9076101128424028435ccd531e649801c328a26e8b41dc904d37d"

	compareGoldenInt16(t, "YM2612_Detune", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_MUL(t *testing.T) {
	y := setupTestChannel(7)

	// Set different MUL values per operator
	// OP1(slot0=0x30): MUL=0 (x0.5)
	y.WritePort(0, 0x30)
	y.WritePort(1, 0x00)
	// OP3(slot1=0x34): MUL=2
	y.WritePort(0, 0x34)
	y.WritePort(1, 0x02)
	// OP2(slot2=0x38): MUL=4
	y.WritePort(0, 0x38)
	y.WritePort(1, 0x04)
	// OP4(slot3=0x3C): MUL=8
	y.WritePort(0, 0x3C)
	y.WritePort(1, 0x08)

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		4048, 4048, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 3728, 3728, 2736, 2736,
		1856, 1856, 1136, 1136, 656, 656, 400, 400,
		416, 416, 688, 688, 1936, 1936, 2832, 2832,
		3840, 3840, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 3680, 3680, 2496, 2496,
	}
	expectedHash := "ef008bb7f5420af82a4d7b51ca1dd90c63b6309727e5654d07b6d31649d392d7"

	compareGoldenInt16(t, "YM2612_MUL", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_TLAttenuation(t *testing.T) {
	y := setupTestChannel(7)

	// Set different TL values per operator (carriers in algo 7)
	// OP1(slot0=0x40): TL=16
	y.WritePort(0, 0x40)
	y.WritePort(1, 0x10)
	// OP3(slot1=0x44): TL=32
	y.WritePort(0, 0x44)
	y.WritePort(1, 0x20)
	// OP2(slot2=0x48): TL=48
	y.WritePort(0, 0x48)
	y.WritePort(1, 0x30)
	// OP4(slot3=0x4C): TL=64
	y.WritePort(0, 0x4C)
	y.WritePort(1, 0x40)

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		464, 464, 496, 496, 544, 544, 576, 576,
		624, 624, 656, 656, 720, 720, 736, 736,
		800, 800, 848, 848, 896, 896, 928, 928,
		976, 976, 1008, 1008, 1056, 1056, 1088, 1088,
		1136, 1136, 1168, 1168, 1232, 1232, 1248, 1248,
		1296, 1296, 1312, 1312, 1344, 1344, 1392, 1392,
		1408, 1408, 1424, 1424, 1472, 1472, 1504, 1504,
		1520, 1520, 1552, 1552, 1568, 1568, 1584, 1584,
	}
	expectedHash := "414fce4871f9880dadec98f3bbf1fb5b3cd655ac69cc058f48f263a42697b964"

	compareGoldenInt16(t, "YM2612_TLAttenuation", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_AllChannels(t *testing.T) {
	y := setupMaxOutputYM2612()
	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		6528, 6528, 9600, 9600, 12672, 12672, 16128, 16128,
		19200, 19200, 21888, 21888, 24864, 24864, 24864, 24864,
		24864, 24864, 24864, 24864, 24864, 24864, 24864, 24864,
		24864, 24864, 24864, 24864, 24864, 24864, 24864, 24864,
		24864, 24864, 24864, 24864, 24864, 24864, 24864, 24864,
		24864, 24864, 24864, 24864, 24864, 24864, 24864, 24864,
		24864, 24864, 24864, 24864, 24864, 24864, 24864, 24864,
		24864, 24864, 24864, 24864, 24864, 24864, 24864, 24864,
	}
	expectedHash := "92d10244a14a1b437b470937654a3d49193e2d45301055e703cd0de4603b7957"

	compareGoldenInt16(t, "YM2612_AllChannels", buf, expectedFirst, expectedHash)
}

// --- Panning Golden Tests ---

func TestYM2612Golden_PanLeftOnly(t *testing.T) {
	y := setupTestChannel(7)
	// Override pan to left-only
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0x80) // panL=true, panR=false

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		1408, 384, 1920, 384, 2432, 384, 3008, 384,
		3520, 384, 3968, 384, 4464, 384, 4464, 384,
		4464, 384, 4464, 384, 4464, 384, 4464, 384,
		4464, 384, 4464, 384, 4464, 384, 4464, 384,
		4464, 384, 4464, 384, 4464, 384, 4464, 384,
		4464, 384, 4464, 384, 4464, 384, 4464, 384,
		4464, 384, 4464, 384, 4464, 384, 4464, 384,
		4464, 384, 4464, 384, 4464, 384, 4464, 384,
	}
	expectedHash := "ed817c3fe60bf160592e59137955f0c9574e2112a431069c905ff46bd753c665"

	compareGoldenInt16(t, "YM2612_PanLeftOnly", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_PanRightOnly(t *testing.T) {
	y := setupTestChannel(7)
	// Override pan to right-only
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0x40) // panL=false, panR=true

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		384, 1408, 384, 1920, 384, 2432, 384, 3008,
		384, 3520, 384, 3968, 384, 4464, 384, 4464,
		384, 4464, 384, 4464, 384, 4464, 384, 4464,
		384, 4464, 384, 4464, 384, 4464, 384, 4464,
		384, 4464, 384, 4464, 384, 4464, 384, 4464,
		384, 4464, 384, 4464, 384, 4464, 384, 4464,
		384, 4464, 384, 4464, 384, 4464, 384, 4464,
		384, 4464, 384, 4464, 384, 4464, 384, 4464,
	}
	expectedHash := "5d97568fb6d973ecf3d80f5f09e32c3d87c23efe54e77ea55f6a2b35ed2e7676"

	compareGoldenInt16(t, "YM2612_PanRightOnly", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_PanDisabled(t *testing.T) {
	y := setupTestChannel(7)
	// Override pan to both disabled
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0x00) // panL=false, panR=false

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		384, 384, 384, 384, 384, 384, 384, 384,
		384, 384, 384, 384, 384, 384, 384, 384,
		384, 384, 384, 384, 384, 384, 384, 384,
		384, 384, 384, 384, 384, 384, 384, 384,
		384, 384, 384, 384, 384, 384, 384, 384,
		384, 384, 384, 384, 384, 384, 384, 384,
		384, 384, 384, 384, 384, 384, 384, 384,
		384, 384, 384, 384, 384, 384, 384, 384,
	}
	expectedHash := "b9b80b0b9d5de8f8a582cf44bce4366eb81b12b5215e613990d6f81812b5d007"

	compareGoldenInt16(t, "YM2612_PanDisabled", buf, expectedFirst, expectedHash)
}

// --- Envelope ADSR Golden Test ---

func TestYM2612Golden_EnvelopeDecaySustain(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Ch0: algo 7, fb=0
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x07)

	// Pan L+R
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC0)

	// Frequency: block=4, fNum=0x29A
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A)

	// All operators: DT=0, MUL=1
	for _, reg := range []uint8{0x30, 0x34, 0x38, 0x3C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}
	// All operators: TL=0
	for _, reg := range []uint8{0x40, 0x44, 0x48, 0x4C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// All operators: RS=3, AR=31 (instant attack)
	for _, reg := range []uint8{0x50, 0x54, 0x58, 0x5C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xDF)
	}
	// All operators: D1R=20, AM=0
	for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x14)
	}
	// All operators: D2R=8
	for _, reg := range []uint8{0x70, 0x74, 0x78, 0x7C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x08)
	}
	// All operators: D1L=4, RR=8
	for _, reg := range []uint8{0x80, 0x84, 0x88, 0x8C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x48)
	}

	// Key on all operators
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF0)

	// Generate first half frame
	halfCycles := (7670454 / 60) / 2
	y.GenerateSamples(halfCycles)

	// Key off all operators
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00)

	// Generate second half frame
	y.GenerateSamples(halfCycles)

	buf := y.GetBuffer()

	expectedFirst := []int16{
		1408, 1408, 1856, 1856, 2304, 2304, 2880, 2880,
		3264, 3264, 3648, 3648, 4160, 4160, 4416, 4416,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "be561e78249d65d9458d7953de05d0ac49c53811890c7dc9af4786f35ac4f4f9"

	compareGoldenInt16(t, "YM2612_EnvelopeDecaySustain", buf, expectedFirst, expectedHash)
}

// --- Rate Scaling Golden Test ---

func TestYM2612Golden_RateScaling(t *testing.T) {
	type rsExpected struct {
		first []int16
		hash  string
	}
	goldens := map[string]rsExpected{
		"RS0": {
			first: []int16{
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
			},
			hash: "bf151fec30423e453edaf6af1bd1060d941853965ce4f2ed66149e198c47b03f",
		},
		"RS3": {
			first: []int16{
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 384, 384, 384, 384,
			},
			hash: "9f664b67a16ea149f485305f16f2a1a7bb4040a18aa93238c2f3494d3fa7ef19",
		},
	}

	for _, tc := range []struct {
		name string
		rs   uint8
	}{
		{"RS0", 0x0F}, // RS=0, AR=15
		{"RS3", 0xCF}, // RS=3, AR=15
	} {
		t.Run(tc.name, func(t *testing.T) {
			y := NewYM2612(7670454, 48000)

			// Ch0: algo 7, fb=0
			y.WritePort(0, 0xB0)
			y.WritePort(1, 0x07)

			// Pan L+R
			y.WritePort(0, 0xB4)
			y.WritePort(1, 0xC0)

			// Frequency: block=4, fNum=0x29A
			y.WritePort(0, 0xA4)
			y.WritePort(1, 0x22)
			y.WritePort(0, 0xA0)
			y.WritePort(1, 0x9A)

			// All operators: DT=0, MUL=1
			for _, reg := range []uint8{0x30, 0x34, 0x38, 0x3C} {
				y.WritePort(0, reg)
				y.WritePort(1, 0x01)
			}
			// All operators: TL=0
			for _, reg := range []uint8{0x40, 0x44, 0x48, 0x4C} {
				y.WritePort(0, reg)
				y.WritePort(1, 0x00)
			}
			// All operators: RS and AR from test case
			for _, reg := range []uint8{0x50, 0x54, 0x58, 0x5C} {
				y.WritePort(0, reg)
				y.WritePort(1, tc.rs)
			}
			// D1R=0
			for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
				y.WritePort(0, reg)
				y.WritePort(1, 0x00)
			}
			// D2R=0
			for _, reg := range []uint8{0x70, 0x74, 0x78, 0x7C} {
				y.WritePort(0, reg)
				y.WritePort(1, 0x00)
			}
			// D1L=0, RR=15
			for _, reg := range []uint8{0x80, 0x84, 0x88, 0x8C} {
				y.WritePort(0, reg)
				y.WritePort(1, 0x0F)
			}

			// Key on all operators
			y.WritePort(0, 0x28)
			y.WritePort(1, 0xF0)

			y.GenerateSamples(7670454 / 60)
			buf := y.GetBuffer()

			g := goldens[tc.name]
			compareGoldenInt16(t, "YM2612_RateScaling_"+tc.name, buf, g.first, g.hash)
		})
	}
}

// --- DAC Variants Golden Test ---

func TestYM2612Golden_DACVariants(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable DAC
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)

	// Write DAC sample 0x00 (max negative: (0-128)<<6 = -8192)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x00)

	// Ch5 pan L+R
	y.WritePort(2, 0xB6)
	y.WritePort(3, 0xC0)

	// Generate first half frame
	halfCycles := (7670454 / 60) / 2
	y.GenerateSamples(halfCycles)

	// Change DAC to 0x80 (center/silence: (128-128)<<6 = 0)
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0x80)

	// Generate second half frame
	y.GenerateSamples(halfCycles)

	buf := y.GetBuffer()

	expectedFirst := []int16{
		-3824, -3824, -3824, -3824, -3824, -3824, -3824, -3824,
		-3824, -3824, -3824, -3824, -3824, -3824, -3824, -3824,
		-3824, -3824, -3824, -3824, -3824, -3824, -3824, -3824,
		-3824, -3824, -3824, -3824, -3824, -3824, -3824, -3824,
		-3824, -3824, -3824, -3824, -3824, -3824, -3824, -3824,
		-3824, -3824, -3824, -3824, -3824, -3824, -3824, -3824,
		-3824, -3824, -3824, -3824, -3824, -3824, -3824, -3824,
		-3824, -3824, -3824, -3824, -3824, -3824, -3824, -3824,
	}
	expectedHash := "079e2cd10bd7645f5343a34c3dbca80c085a8aed91313aa173e5e99f786a2b4d"

	compareGoldenInt16(t, "YM2612_DACVariants", buf, expectedFirst, expectedHash)
}

// --- Multi-Channel Mixed Algos Golden Test ---

func TestYM2612Golden_MultiChannelMixedAlgos(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// --- Ch0 (Part I, slot 0): algo 0 (serial) ---
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x00) // algo=0, fb=0
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC0) // L+R
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A) // block=4, fNum=0x29A

	for _, reg := range []uint8{0x30, 0x34, 0x38, 0x3C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01) // DT=0, MUL=1
	}
	for _, reg := range []uint8{0x40, 0x44, 0x48, 0x4C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00) // TL=0
	}
	for _, reg := range []uint8{0x50, 0x54, 0x58, 0x5C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xDF) // RS=3, AR=31
	}
	for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00) // D1R=0
	}
	for _, reg := range []uint8{0x70, 0x74, 0x78, 0x7C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00) // D2R=0
	}
	for _, reg := range []uint8{0x80, 0x84, 0x88, 0x8C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x0F) // D1L=0, RR=15
	}

	// Key on ch0
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF0)

	// --- Ch1 (Part I, slot 1): algo 4 (dual carrier) ---
	y.WritePort(0, 0xB1)
	y.WritePort(1, 0x04) // algo=4, fb=0
	y.WritePort(0, 0xB5)
	y.WritePort(1, 0xC0) // L+R
	y.WritePort(0, 0xA5)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA1)
	y.WritePort(1, 0x9A) // block=4, fNum=0x29A

	for _, reg := range []uint8{0x31, 0x35, 0x39, 0x3D} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}
	for _, reg := range []uint8{0x41, 0x45, 0x49, 0x4D} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	for _, reg := range []uint8{0x51, 0x55, 0x59, 0x5D} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xDF)
	}
	for _, reg := range []uint8{0x61, 0x65, 0x69, 0x6D} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	for _, reg := range []uint8{0x71, 0x75, 0x79, 0x7D} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	for _, reg := range []uint8{0x81, 0x85, 0x89, 0x8D} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x0F)
	}

	// Key on ch1
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF1)

	// --- Ch2 (Part I, slot 2): algo 7 (additive) ---
	y.WritePort(0, 0xB2)
	y.WritePort(1, 0x07) // algo=7, fb=0
	y.WritePort(0, 0xB6)
	y.WritePort(1, 0xC0) // L+R
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0x9A) // block=4, fNum=0x29A

	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}
	for _, reg := range []uint8{0x42, 0x46, 0x4A, 0x4E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	for _, reg := range []uint8{0x52, 0x56, 0x5A, 0x5E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xDF)
	}
	for _, reg := range []uint8{0x62, 0x66, 0x6A, 0x6E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	for _, reg := range []uint8{0x72, 0x76, 0x7A, 0x7E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	for _, reg := range []uint8{0x82, 0x86, 0x8A, 0x8E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x0F)
	}

	// Key on ch2
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF2)

	// --- Ch5: DAC enabled, sample 0xC0 ---
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80) // DAC enable
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xC0) // DAC sample
	y.WritePort(2, 0xB6)
	y.WritePort(3, 0xC0) // Ch5 pan L+R

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		9072, 9072, 6304, 6304, 2720, 2720, -1336, -1336,
		-136, -136, -2160, -2160, 4864, 4864, 9328, 9328,
		12864, 12864, 6128, 6128, 1336, 1336, 504, 504,
		3248, 3248, 9440, 9440, 11392, 11392, 10752, 10752,
		13968, 13968, 2976, 2976, -1800, -1800, 4808, 4808,
		6216, 6216, 4576, 4576, 13872, 13872, 9152, 9152,
		12000, 12000, 8800, 8800, 6576, 6576, 6528, 6528,
		4568, 4568, 2104, 2104, 3592, 3592, 1624, 1624,
	}
	expectedHash := "ba96d2662ad625af83b4fbddd7740132f56c5110609d9475e2b4ced9af1d6189"

	compareGoldenInt16(t, "YM2612_MultiChannelMixedAlgos", buf, expectedFirst, expectedHash)
}

// --- PM + Ch3 Special Golden Test ---

func TestYM2612Golden_PMWithCh3Special(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable ch3 special mode
	y.WritePort(0, 0x27)
	y.WritePort(1, 0x40)

	// Ch2 (index 2) = channel 3 in YM2612 numbering
	// Algorithm 7, fb=0
	y.WritePort(0, 0xB2)
	y.WritePort(1, 0x07)

	// Enable LFO with fastest frequency
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x0F) // LFO enable, freq=7

	// Set FMS=7 on ch2 (L+R, AMS=0, FMS=7)
	y.WritePort(0, 0xB6)
	y.WritePort(1, 0xC7)

	// Set per-operator frequencies for ch3 special mode
	// Slot 0 -> OP3 (opIdx 2): A8/AC
	y.WritePort(0, 0xAC)
	y.WritePort(1, 0x22) // block=4
	y.WritePort(0, 0xA8)
	y.WritePort(1, 0x9A) // fNum LSB

	// Slot 1 -> OP1 (opIdx 0): A9/AD
	y.WritePort(0, 0xAD)
	y.WritePort(1, 0x26) // block=4, different fNum
	y.WritePort(0, 0xA9)
	y.WritePort(1, 0x50)

	// Slot 2 -> OP2 (opIdx 1): AA/AE
	y.WritePort(0, 0xAE)
	y.WritePort(1, 0x2A) // block=5
	y.WritePort(0, 0xAA)
	y.WritePort(1, 0x00)

	// Channel frequency for OP4 (uses channel freq, not special mode)
	y.WritePort(0, 0xA6)
	y.WritePort(1, 0x1E) // block=3
	y.WritePort(0, 0xA2)
	y.WritePort(1, 0xD0)

	// All operators: DT=0, MUL=1
	for _, reg := range []uint8{0x32, 0x36, 0x3A, 0x3E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}
	// All operators: TL=0
	for _, reg := range []uint8{0x42, 0x46, 0x4A, 0x4E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// All operators: RS=3, AR=31
	for _, reg := range []uint8{0x52, 0x56, 0x5A, 0x5E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xDF)
	}
	// D1R=0
	for _, reg := range []uint8{0x62, 0x66, 0x6A, 0x6E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// D2R=0
	for _, reg := range []uint8{0x72, 0x76, 0x7A, 0x7E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// D1L=0, RR=15
	for _, reg := range []uint8{0x82, 0x86, 0x8A, 0x8E} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x0F)
	}

	// Key on all operators for ch2 (channel 3)
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF2)

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		2000, 2000, 2816, 2816, 3600, 3600, 4416, 4416,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "8c6b9dc4e7276ccbc8ed0a4502cdb5edb88c50f685228f90de5ead67da2667b6"

	compareGoldenInt16(t, "YM2612_PMWithCh3Special", buf, expectedFirst, expectedHash)
}

// --- Round 2 Golden Tests ---

func TestYM2612Golden_DACPanLeft(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Enable DAC
	y.WritePort(0, 0x2B)
	y.WritePort(1, 0x80)

	// DAC sample 0xC0
	y.WritePort(0, 0x2A)
	y.WritePort(1, 0xC0)

	// Ch5 pan left-only
	y.WritePort(2, 0xB6)
	y.WritePort(3, 0x80)

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		2432, 384, 2432, 384, 2432, 384, 2432, 384,
		2432, 384, 2432, 384, 2432, 384, 2432, 384,
		2432, 384, 2432, 384, 2432, 384, 2432, 384,
		2432, 384, 2432, 384, 2432, 384, 2432, 384,
		2432, 384, 2432, 384, 2432, 384, 2432, 384,
		2432, 384, 2432, 384, 2432, 384, 2432, 384,
		2432, 384, 2432, 384, 2432, 384, 2432, 384,
		2432, 384, 2432, 384, 2432, 384, 2432, 384,
	}
	expectedHash := "6ade708935f03175f2758ad51030208702179490d444298ef90952f1175fe323"

	compareGoldenInt16(t, "YM2612_DACPanLeft", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_FreqExtremes(t *testing.T) {
	type freqExpected struct {
		first []int16
		hash  string
	}
	goldens := map[string]freqExpected{
		"HighFreq": {
			first: []int16{
				4464, 4464, 4464, 4464, 384, 384, -3816, -3816,
				-3816, -3816, -3816, -3816, 208, 208, 4464, 4464,
				4464, 4464, 384, 384, -3816, -3816, -3816, -3816,
				-3816, -3816, 208, 208, 4464, 4464, 4464, 4464,
				4464, 4464, 512, 512, -3816, -3816, -3816, -3816,
				80, 80, 4464, 4464, 4464, 4464, 4464, 4464,
				512, 512, -3816, -3816, -3816, -3816, 80, 80,
				4464, 4464, 4464, 4464, 4464, 4464, 576, 576,
			},
			hash: "e9ad4b298bbf4373d21369164e67a82177f076e14c4fd248eff95ea9ae9cc6b3",
		},
		"LowFreq": {
			first: []int16{
				384, 384, 384, 384, 384, 384, 384, 384,
				384, 384, 384, 384, 512, 512, 512, 512,
				512, 512, 512, 512, 512, 512, 512, 512,
				512, 512, 576, 576, 576, 576, 576, 576,
				576, 576, 576, 576, 576, 576, 576, 576,
				704, 704, 704, 704, 704, 704, 704, 704,
				704, 704, 704, 704, 704, 704, 832, 832,
				832, 832, 832, 832, 832, 832, 832, 832,
			},
			hash: "1b1551d7d204e8c76568ea2517ed90b84bb584289223cecb9b4c75799ecbbe43",
		},
	}

	for _, tc := range []struct {
		name   string
		freqHi uint8 // A4 register value
		freqLo uint8 // A0 register value
	}{
		{"HighFreq", 0x3F, 0xFF}, // block=7, fNum=0x7FF
		{"LowFreq", 0x01, 0x00},  // block=0, fNum=0x100
	} {
		t.Run(tc.name, func(t *testing.T) {
			y := NewYM2612(7670454, 48000)

			// Ch0: algo 7, fb=0
			y.WritePort(0, 0xB0)
			y.WritePort(1, 0x07)

			// Pan L+R
			y.WritePort(0, 0xB4)
			y.WritePort(1, 0xC0)

			// Frequency from test case
			y.WritePort(0, 0xA4)
			y.WritePort(1, tc.freqHi)
			y.WritePort(0, 0xA0)
			y.WritePort(1, tc.freqLo)

			// All operators: DT=0, MUL=1
			for _, reg := range []uint8{0x30, 0x34, 0x38, 0x3C} {
				y.WritePort(0, reg)
				y.WritePort(1, 0x01)
			}
			// All operators: TL=0
			for _, reg := range []uint8{0x40, 0x44, 0x48, 0x4C} {
				y.WritePort(0, reg)
				y.WritePort(1, 0x00)
			}
			// All operators: RS=3, AR=31
			for _, reg := range []uint8{0x50, 0x54, 0x58, 0x5C} {
				y.WritePort(0, reg)
				y.WritePort(1, 0xDF)
			}
			// D1R=0
			for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
				y.WritePort(0, reg)
				y.WritePort(1, 0x00)
			}
			// D2R=0
			for _, reg := range []uint8{0x70, 0x74, 0x78, 0x7C} {
				y.WritePort(0, reg)
				y.WritePort(1, 0x00)
			}
			// D1L=0, RR=15
			for _, reg := range []uint8{0x80, 0x84, 0x88, 0x8C} {
				y.WritePort(0, reg)
				y.WritePort(1, 0x0F)
			}

			// Key on all operators
			y.WritePort(0, 0x28)
			y.WritePort(1, 0xF0)

			y.GenerateSamples(7670454 / 60)
			buf := y.GetBuffer()

			g := goldens[tc.name]
			compareGoldenInt16(t, "YM2612_FreqExtremes_"+tc.name, buf, g.first, g.hash)
		})
	}
}

func TestYM2612Golden_D1L15Boundary(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Ch0: algo 7, fb=0
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x07)

	// Pan L+R
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC0)

	// Frequency: block=4, fNum=0x29A
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A)

	// All operators: DT=0, MUL=1
	for _, reg := range []uint8{0x30, 0x34, 0x38, 0x3C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}
	// All operators: TL=0
	for _, reg := range []uint8{0x40, 0x44, 0x48, 0x4C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// All operators: RS=3, AR=31 (instant attack)
	for _, reg := range []uint8{0x50, 0x54, 0x58, 0x5C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xDF)
	}
	// All operators: D1R=31 (fast decay)
	for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x1F)
	}
	// All operators: D2R=0 (frozen sustain)
	for _, reg := range []uint8{0x70, 0x74, 0x78, 0x7C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// All operators: D1L=15, RR=15
	for _, reg := range []uint8{0x80, 0x84, 0x88, 0x8C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xFF)
	}

	// Key on all operators
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF0)

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		1408, 1408, 1792, 1792, 2240, 2240, 2752, 2752,
		3008, 3008, 3392, 3392, 3840, 3840, 3904, 3904,
		4352, 4352, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "ec17302d9e6805797051e93124e43c1f048c383cc19809a630677e8c90f2b59d"

	compareGoldenInt16(t, "YM2612_D1L15Boundary", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_PartIIChannel(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Ch3 (Part II slot 0): algo 7, fb=0
	y.WritePort(2, 0xB0)
	y.WritePort(3, 0x07)

	// Pan L+R
	y.WritePort(2, 0xB4)
	y.WritePort(3, 0xC0)

	// Frequency: block=4, fNum=0x29A
	y.WritePort(2, 0xA4)
	y.WritePort(3, 0x22)
	y.WritePort(2, 0xA0)
	y.WritePort(3, 0x9A)

	// All operators via Part II ports
	for _, reg := range []uint8{0x30, 0x34, 0x38, 0x3C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x01) // DT=0, MUL=1
	}
	for _, reg := range []uint8{0x40, 0x44, 0x48, 0x4C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x00) // TL=0
	}
	for _, reg := range []uint8{0x50, 0x54, 0x58, 0x5C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0xDF) // RS=3, AR=31
	}
	for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x00) // D1R=0
	}
	for _, reg := range []uint8{0x70, 0x74, 0x78, 0x7C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x00) // D2R=0
	}
	for _, reg := range []uint8{0x80, 0x84, 0x88, 0x8C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x0F) // D1L=0, RR=15
	}

	// Key on: Part II ch0 = 0x04, all ops on
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF4)

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		1408, 1408, 1920, 1920, 2432, 2432, 3008, 3008,
		3520, 3520, 3968, 3968, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "4737e8f317dc75ddbf7531da9fa74ca47ba346da50f23566dbc0d3399d9b4aad"

	compareGoldenInt16(t, "YM2612_PartIIChannel", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_LFODisableMidFrame(t *testing.T) {
	y := setupTestChannel(7)

	// Enable LFO
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x0F) // LFO enable, freq=7

	// Set AMS=3
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xF0) // L+R, AMS=3, FMS=0

	// Enable AM on all operators
	for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x80) // AM=1, D1R=0
	}

	// Generate first half frame with LFO enabled
	halfCycles := (7670454 / 60) / 2
	y.GenerateSamples(halfCycles)

	// Disable LFO
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x00)

	// Generate second half frame
	y.GenerateSamples(halfCycles)

	buf := y.GetBuffer()

	expectedFirst := []int16{
		640, 640, 768, 768, 896, 896, 1024, 1024,
		1152, 1152, 1280, 1280, 1408, 1408, 1536, 1536,
		1728, 1728, 1984, 1984, 2112, 2112, 2240, 2240,
		2432, 2432, 2560, 2560, 2624, 2624, 2752, 2752,
		2880, 2880, 3072, 3072, 3328, 3328, 3392, 3392,
		3520, 3520, 3712, 3712, 3776, 3776, 3904, 3904,
		3968, 3968, 4096, 4096, 4224, 4224, 4416, 4416,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "7a2f4d5304682cec6f39087015a4a4474d47fc38f603d0c6ffc99c7634c3e33c"

	compareGoldenInt16(t, "YM2612_LFODisableMidFrame", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_FrozenSustain(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// Ch0: algo 7, fb=0
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x07)

	// Pan L+R
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC0)

	// Frequency: block=4, fNum=0x29A
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A)

	// All operators: DT=0, MUL=1
	for _, reg := range []uint8{0x30, 0x34, 0x38, 0x3C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}
	// All operators: TL=0
	for _, reg := range []uint8{0x40, 0x44, 0x48, 0x4C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// All operators: RS=3, AR=31 (instant attack)
	for _, reg := range []uint8{0x50, 0x54, 0x58, 0x5C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xDF)
	}
	// All operators: D1R=31 (fast decay)
	for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x1F)
	}
	// All operators: D2R=0 (frozen sustain)
	for _, reg := range []uint8{0x70, 0x74, 0x78, 0x7C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	// All operators: D1L=8, RR=15
	for _, reg := range []uint8{0x80, 0x84, 0x88, 0x8C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x8F)
	}

	// Key on all operators
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF0)

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		1408, 1408, 1792, 1792, 2240, 2240, 2752, 2752,
		3008, 3008, 3392, 3392, 3840, 3840, 3904, 3904,
		4352, 4352, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "c608b79da3bfe4c09062b250206a43b5453abb3340f5e8ba3c56a2dadcffc46f"

	compareGoldenInt16(t, "YM2612_FrozenSustain", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_FeedbackMid(t *testing.T) {
	y := setupTestChannel(0)
	// Override to algo=0, fb=4
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x20) // algo=0, fb=4

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		3168, 3168, -3488, -3488, -1872, -1872, -2944, -2944,
		4128, 4128, -3520, -3520, 1920, 1920, -3776, -3776,
		-400, -400, -3440, -3440, 960, 960, 4432, 4432,
		544, 544, 736, 736, 4016, 4016, -1168, -1168,
		-3280, -3280, 4224, 4224, 3296, 3296, 4400, 4400,
		3088, 3088, 4288, 4288, -3824, -3824, 2272, 2272,
		1088, 1088, 2400, 2400, -3376, -3376, 2768, 2768,
		-2416, -2416, 2928, 2928, 3792, 3792, -2608, -2608,
	}
	expectedHash := "4abf681928e570b26b234d10d63fe5bfc27fd7f1259305de6f6fd8110348198f"

	compareGoldenInt16(t, "YM2612_FeedbackMid", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_AMSensitivity(t *testing.T) {
	type amsExpected struct {
		first []int16
		hash  string
	}
	goldens := map[string]amsExpected{
		"AMS1": {
			first: []int16{
				1216, 1216, 1664, 1664, 2112, 2112, 2624, 2624,
				3008, 3008, 3456, 3456, 3840, 3840, 4288, 4288,
				4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
				4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
				4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
				4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
				4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
				4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
			},
			hash: "0b06382f8415c135c24fb0a94d402349636e6f6b1530ba57fb1051c49d441bc6",
		},
		"AMS2": {
			first: []int16{
				896, 896, 1152, 1152, 1408, 1408, 1728, 1728,
				1984, 1984, 2176, 2176, 2432, 2432, 2688, 2688,
				3008, 3008, 3520, 3520, 3776, 3776, 3968, 3968,
				4288, 4288, 4464, 4464, 4464, 4464, 4464, 4464,
				4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
				4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
				4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
				4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
			},
			hash: "3e9a176063be533c273f2e0f74161bc48e7330cd1ff408bae12795823f2a4ecd",
		},
	}

	for _, tc := range []struct {
		name string
		ams  uint8
	}{
		{"AMS1", 1},
		{"AMS2", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			y := setupTestChannel(7)

			// Enable LFO
			y.WritePort(0, 0x22)
			y.WritePort(1, 0x0F) // LFO enable, freq=7

			// Set AMS
			y.WritePort(0, 0xB4)
			y.WritePort(1, 0xC0|(tc.ams<<4)) // L+R, FMS=0

			// Enable AM on all operators
			for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
				y.WritePort(0, reg)
				y.WritePort(1, 0x80) // AM=1, D1R=0
			}

			y.GenerateSamples(7670454 / 60)
			buf := y.GetBuffer()

			g := goldens[tc.name]
			compareGoldenInt16(t, "YM2612_AMSensitivity_"+tc.name, buf, g.first, g.hash)
		})
	}
}

func TestYM2612Golden_PMSensitivityMid(t *testing.T) {
	y := setupTestChannel(7)

	// Enable LFO
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x0F) // LFO enable, freq=7

	// Set FMS=4
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC4) // L+R, AMS=0, FMS=4

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		1408, 1408, 1920, 1920, 2432, 2432, 3008, 3008,
		3520, 3520, 3968, 3968, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "48b9ef3390d7dcbe6f1a00feed8bccb34d9e43e3ab4091ef2f906a43e92403d5"

	compareGoldenInt16(t, "YM2612_PMSensitivityMid", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_PartialKeyOn(t *testing.T) {
	y := setupTestChannel(7) // Keys on all 4 ops

	// Key off all
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x00)

	// Re-key on only OP1 and OP3 (bits 4,6 = 0x10+0x40 = 0x50)
	y.WritePort(0, 0x28)
	y.WritePort(1, 0x50)

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		1408, 1408, 1856, 1856, 2336, 2336, 2880, 2880,
		3264, 3264, 3680, 3680, 4160, 4160, 4416, 4416,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "ebe62ad32deb84720435631561d5004d8e9c7e32e2f8879da11c6ec11dfc1718"

	compareGoldenInt16(t, "YM2612_PartialKeyOn", buf, expectedFirst, expectedHash)
}

// --- Round 3 Golden Tests ---

func TestYM2612Golden_CombinedAMPM(t *testing.T) {
	y := setupTestChannel(7)

	// Enable LFO with fastest frequency
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x0F) // LFO enable, freq=7

	// Set AMS=3, FMS=7 (both AM and PM active)
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xF7) // L+R, AMS=3, FMS=7

	// Enable AM on all operators
	for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x80) // AM=1, D1R=0
	}

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		640, 640, 768, 768, 896, 896, 1024, 1024,
		1152, 1152, 1280, 1280, 1408, 1408, 1536, 1536,
		1728, 1728, 1984, 1984, 2112, 2112, 2240, 2240,
		2432, 2432, 2560, 2560, 2624, 2624, 2752, 2752,
		2880, 2880, 3072, 3072, 3328, 3328, 3392, 3392,
		3520, 3520, 3712, 3712, 3776, 3776, 3904, 3904,
		3968, 3968, 4096, 4096, 4224, 4224, 4416, 4416,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "3dfff6d6561cc8eb2c3ed50d523e2728feb54b6e0241d67b60074704004cc687"

	compareGoldenInt16(t, "YM2612_CombinedAMPM", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_SelectiveAM(t *testing.T) {
	y := setupTestChannel(7)

	// Enable LFO with fastest frequency
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x0F) // LFO enable, freq=7

	// Set AMS=3, FMS=0
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xF0) // L+R, AMS=3, FMS=0

	// Enable AM on OP1 and OP3 only (slot0=0x60, slot1=0x64)
	y.WritePort(0, 0x60)
	y.WritePort(1, 0x80) // OP1: AM=1, D1R=0
	y.WritePort(0, 0x64)
	y.WritePort(1, 0x80) // OP3: AM=1, D1R=0
	// OP2 (slot2=0x68) and OP4 (slot3=0x6C) keep AM=0 from setupTestChannel

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		1024, 1024, 1344, 1344, 1664, 1664, 2016, 2016,
		2336, 2336, 2624, 2624, 2944, 2944, 3232, 3232,
		3616, 3616, 4224, 4224, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "c26359078d9a391046af02f31ce60ac9d207eb3226f8f489fccd50243d88fe27"

	compareGoldenInt16(t, "YM2612_SelectiveAM", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_LFOSlowFreq(t *testing.T) {
	y := setupTestChannel(7)

	// Enable LFO with slowest frequency (freq=0, period=108)
	y.WritePort(0, 0x22)
	y.WritePort(1, 0x08) // LFO enable, freq=0

	// Set FMS=7 (max PM sensitivity)
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0xC7) // L+R, AMS=0, FMS=7

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		1408, 1408, 1920, 1920, 2432, 2432, 3008, 3008,
		3520, 3520, 3968, 3968, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
		4464, 4464, 4464, 4464, 4464, 4464, 4464, 4464,
	}
	expectedHash := "fc76774de4b15f82ecabfb112e43126bc3b94acd365de78596140eddc9457edf"

	compareGoldenInt16(t, "YM2612_LFOSlowFreq", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_MixedPanChannels(t *testing.T) {
	y := NewYM2612(7670454, 48000)

	// --- Ch0 (Part I, slot 0): algo 7, pan Left-only ---
	y.WritePort(0, 0xB0)
	y.WritePort(1, 0x07) // algo=7, fb=0
	y.WritePort(0, 0xB4)
	y.WritePort(1, 0x80) // Left-only
	y.WritePort(0, 0xA4)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA0)
	y.WritePort(1, 0x9A) // block=4, fNum=0x29A

	for _, reg := range []uint8{0x30, 0x34, 0x38, 0x3C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01) // DT=0, MUL=1
	}
	for _, reg := range []uint8{0x40, 0x44, 0x48, 0x4C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00) // TL=0
	}
	for _, reg := range []uint8{0x50, 0x54, 0x58, 0x5C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xDF) // RS=3, AR=31
	}
	for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00) // D1R=0
	}
	for _, reg := range []uint8{0x70, 0x74, 0x78, 0x7C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00) // D2R=0
	}
	for _, reg := range []uint8{0x80, 0x84, 0x88, 0x8C} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x0F) // D1L=0, RR=15
	}

	// Key on ch0
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF0)

	// --- Ch1 (Part I, slot 1): algo 7, pan Right-only ---
	y.WritePort(0, 0xB1)
	y.WritePort(1, 0x07) // algo=7, fb=0
	y.WritePort(0, 0xB5)
	y.WritePort(1, 0x40) // Right-only
	y.WritePort(0, 0xA5)
	y.WritePort(1, 0x22)
	y.WritePort(0, 0xA1)
	y.WritePort(1, 0x9A) // block=4, fNum=0x29A

	for _, reg := range []uint8{0x31, 0x35, 0x39, 0x3D} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x01)
	}
	for _, reg := range []uint8{0x41, 0x45, 0x49, 0x4D} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	for _, reg := range []uint8{0x51, 0x55, 0x59, 0x5D} {
		y.WritePort(0, reg)
		y.WritePort(1, 0xDF)
	}
	for _, reg := range []uint8{0x61, 0x65, 0x69, 0x6D} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	for _, reg := range []uint8{0x71, 0x75, 0x79, 0x7D} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x00)
	}
	for _, reg := range []uint8{0x81, 0x85, 0x89, 0x8D} {
		y.WritePort(0, reg)
		y.WritePort(1, 0x0F)
	}

	// Key on ch1
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF1)

	// --- Ch3 (Part II, slot 0): algo 7, pan L+R ---
	y.WritePort(2, 0xB0)
	y.WritePort(3, 0x07) // algo=7, fb=0
	y.WritePort(2, 0xB4)
	y.WritePort(3, 0xC0) // L+R
	y.WritePort(2, 0xA4)
	y.WritePort(3, 0x22)
	y.WritePort(2, 0xA0)
	y.WritePort(3, 0x9A) // block=4, fNum=0x29A

	for _, reg := range []uint8{0x30, 0x34, 0x38, 0x3C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x01)
	}
	for _, reg := range []uint8{0x40, 0x44, 0x48, 0x4C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x00)
	}
	for _, reg := range []uint8{0x50, 0x54, 0x58, 0x5C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0xDF)
	}
	for _, reg := range []uint8{0x60, 0x64, 0x68, 0x6C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x00)
	}
	for _, reg := range []uint8{0x70, 0x74, 0x78, 0x7C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x00)
	}
	for _, reg := range []uint8{0x80, 0x84, 0x88, 0x8C} {
		y.WritePort(2, reg)
		y.WritePort(3, 0x0F)
	}

	// Key on ch3 (Part II ch0 = 0x04 in key-on register)
	y.WritePort(0, 0x28)
	y.WritePort(1, 0xF4)

	y.GenerateSamples(7670454 / 60)
	buf := y.GetBuffer()

	expectedFirst := []int16{
		2432, 2432, 3456, 3456, 4480, 4480, 5632, 5632,
		6656, 6656, 7552, 7552, 8544, 8544, 8544, 8544,
		8544, 8544, 8544, 8544, 8544, 8544, 8544, 8544,
		8544, 8544, 8544, 8544, 8544, 8544, 8544, 8544,
		8544, 8544, 8544, 8544, 8544, 8544, 8544, 8544,
		8544, 8544, 8544, 8544, 8544, 8544, 8544, 8544,
		8544, 8544, 8544, 8544, 8544, 8544, 8544, 8544,
		8544, 8544, 8544, 8544, 8544, 8544, 8544, 8544,
	}
	expectedHash := "c0a8df99b79a2f841089fd8eda27a19f72225a25cd536a77a5562a82e01907db"

	compareGoldenInt16(t, "YM2612_MixedPanChannels", buf, expectedFirst, expectedHash)
}

func TestYM2612Golden_FeedbackRange(t *testing.T) {
	type fbExpected struct {
		first []int16
		hash  string
	}
	goldens := map[string]fbExpected{
		"FB2": {
			first: []int16{
				-576, -576, 176, 176, 4432, 4432, 2144, 2144,
				2928, 2928, -2768, -2768, 1808, 1808, -368, -368,
				2576, 2576, 4256, 4256, -240, -240, 3920, 3920,
				4112, 4112, -2384, -2384, -3184, -3184, 4320, 4320,
				4000, 4000, 3248, 3248, 3456, 3456, -1072, -1072,
				4208, 4208, 4128, 4128, -1232, -1232, 3264, 3264,
				-1952, -1952, 3792, 3792, 3616, 3616, -2512, -2512,
				4160, 4160, 3344, 3344, 864, 864, 2000, 2000,
			},
			hash: "b917ac490ed4dcfc5cdc5021aa4dc38c8d3713374aaebf61d4d57ff871437ed4",
		},
		"FB6": {
			first: []int16{
				3856, 3856, 1312, 1312, 1408, 1408, 2432, 2432,
				-1376, -1376, 4448, 4448, 2208, 2208, 3760, 3760,
				-3824, -3824, -3280, -3280, -3584, -3584, 1616, 1616,
				2272, 2272, -1024, -1024, 2160, 2160, -3712, -3712,
				1472, 1472, 1856, 1856, 4448, 4448, 2656, 2656,
				-352, -352, -2416, -2416, -3824, -3824, 208, 208,
				3360, 3360, 4448, 4448, 2976, 2976, 4288, 4288,
				-2416, -2416, 2704, 2704, 3600, 3600, -1568, -1568,
			},
			hash: "5ea8fa478ee41101904599892b24f6b8aa2109bcf570bdc83ef3ab8cbbde59d3",
		},
	}

	for _, tc := range []struct {
		name string
		fb   uint8
	}{
		{"FB2", 2},
		{"FB6", 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			y := setupTestChannel(0)
			// Override to algo=0 with specified feedback
			y.WritePort(0, 0xB0)
			y.WritePort(1, tc.fb<<3) // algo=0, fb=tc.fb

			y.GenerateSamples(7670454 / 60)
			buf := y.GetBuffer()

			g := goldens[tc.name]
			compareGoldenInt16(t, "YM2612_FeedbackRange_"+tc.name, buf, g.first, g.hash)
		})
	}
}
