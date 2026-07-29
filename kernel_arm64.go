//go:build arm64

package uyaml

// ASIMD (NEON) is mandatory in the ARMv8-A baseline, so no runtime feature
// detection is needed on arm64.

// KernelName reports which newline-scan kernel Index uses on this platform.
func KernelName() string { return "neon" }

// bit weight per lane, repeated for both 8-byte halves of a q-register.
var bitWeights = [16]byte{1, 2, 4, 8, 16, 32, 64, 128, 1, 2, 4, 8, 16, 32, 64, 128}

// neonNewlineMasks64 writes one 64-bit newline mask per 64-byte block.
// Implemented in kernel_arm64.s; processes exactly nblocks full blocks.
//
//go:noescape
func neonNewlineMasks64(p *byte, nblocks int, masks *uint64, weights *byte)

func newlineMasks(data []byte, masks []uint64) {
	nb := len(data) / 64
	if nb > 0 {
		neonNewlineMasks64(&data[0], nb, &masks[0], &bitWeights[0])
	}
	if len(data)%64 != 0 {
		newlineMasksGenericFrom(data, masks, nb)
	}
}
