package uyaml

import "encoding/binary"

// newlineMasksGeneric fills masks with one bit per input byte (bit i of
// masks[k] set iff data[k*64+i] == '\n') using portable SWAR — no assembly,
// works on every architecture, and serves as the differential-testing oracle
// for the SIMD kernels.
func newlineMasksGeneric(data []byte, masks []uint64) {
	newlineMasksGenericFrom(data, masks, 0)
}

// newlineMasksGenericFrom processes blocks starting at block index from.
// SIMD kernels handle full 64-byte blocks and delegate the tail here.
func newlineMasksGenericFrom(data []byte, masks []uint64, from int) {
	const (
		splat = 0x0A0A0A0A0A0A0A0A // '\n' in every byte
		un    = 0x7F7F7F7F7F7F7F7F
		// movemask multiplier: gathers the low bit of every byte (after >>7)
		// into the top byte of the product.
		mul = 0x0102040810204080
	)
	n := len(data)
	nb := n / 64
	for k := from; k < nb; k++ {
		base := k * 64
		var m uint64
		for w := 0; w < 8; w++ {
			x := binary.LittleEndian.Uint64(data[base+w*8:]) ^ splat
			// Exact zero-byte detect (Hacker's Delight): 0x80 in every byte
			// that was '\n', with no inter-byte borrow false positives —
			// the cheaper (x-lo)&^x&hi variant is NOT exact.
			z := ^(((x & un) + un) | x | un)
			m |= ((z >> 7) * mul) >> 56 << (w * 8)
		}
		masks[k] = m
	}
	if tail := n - nb*64; tail > 0 && nb >= from {
		var m uint64
		for j := 0; j < tail; j++ {
			if data[nb*64+j] == '\n' {
				m |= 1 << j
			}
		}
		masks[nb] = m
	}
}
