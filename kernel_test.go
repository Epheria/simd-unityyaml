package uyaml

import (
	"bytes"
	"math/rand"
	"testing"
)

// newlineMasksNaive is the trivially-correct oracle every kernel must match.
func newlineMasksNaive(data []byte, masks []uint64) {
	for i := range masks {
		masks[i] = 0
	}
	for i, b := range data {
		if b == '\n' {
			masks[i/64] |= 1 << (i % 64)
		}
	}
}

func maskSlices(n int) ([]uint64, []uint64) {
	blocks := (n + 63) / 64
	return make([]uint64, blocks), make([]uint64, blocks)
}

func checkKernel(t *testing.T, name string, fn func([]byte, []uint64), data []byte) {
	t.Helper()
	got, want := maskSlices(len(data))
	fn(data, got)
	newlineMasksNaive(data, want)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: len=%d block=%d got %016x want %016x", name, len(data), i, got[i], want[i])
		}
	}
}

func kernels() map[string]func([]byte, []uint64) {
	return map[string]func([]byte, []uint64){
		"generic":    newlineMasksGeneric,
		KernelName(): newlineMasks, // platform dispatch (neon on arm64)
	}
}

func TestKernelsFixed(t *testing.T) {
	cases := [][]byte{
		nil,
		[]byte("\n"),
		[]byte("no newline at all"),
		bytes.Repeat([]byte{'\n'}, 64),
		bytes.Repeat([]byte{'\n'}, 200),
		bytes.Repeat([]byte{'x'}, 512),
		append(bytes.Repeat([]byte{'a'}, 63), '\n'),
		append(bytes.Repeat([]byte{'a'}, 64), '\n'),
		[]byte("--- !u!1 &123\n  m_Name: X\r\n\n--- !u!4 &456 stripped\n"),
	}
	for name, fn := range kernels() {
		for _, data := range cases {
			checkKernel(t, name, fn, data)
		}
	}
}

func TestKernelsRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for name, fn := range kernels() {
		for size := 0; size < 700; size++ {
			data := make([]byte, size)
			for i := range data {
				if rng.Intn(8) == 0 {
					data[i] = '\n'
				} else {
					data[i] = byte(rng.Intn(256))
					if data[i] == '\n' {
						data[i] = 'x'
					}
				}
			}
			checkKernel(t, name, fn, data)
		}
	}
}

func TestKernelsUnaligned(t *testing.T) {
	// The kernel must not assume 16-byte alignment: exercise offset starts.
	backing := bytes.Repeat([]byte("line one\nline two longer\n\n"), 100)
	for name, fn := range kernels() {
		for off := 0; off < 32; off++ {
			checkKernel(t, name, fn, backing[off:])
		}
	}
}
