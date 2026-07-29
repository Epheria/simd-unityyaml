//go:build arm64

#include "textflag.h"

// func neonNewlineMasks64(p *byte, nblocks int, masks *uint64, weights *byte)
//
// Per 64-byte block: four 16-byte compares against '\n', AND with per-lane
// bit weights, then three pairwise-add reductions collapse the four match
// vectors into eight mask bytes (simdjson's neon movemask-bulk shape).
TEXT ·neonNewlineMasks64(SB), NOSPLIT, $0-32
	MOVD p+0(FP), R0
	MOVD nblocks+8(FP), R1
	MOVD masks+16(FP), R2
	MOVD weights+24(FP), R3

	VLD1 (R3), [V1.B16]      // V1 = {1,2,4,...,128, 1,2,4,...,128}
	MOVD $10, R4
	VMOV R4, V0.B16          // V0 = '\n' splat

loop:
	CBZ  R1, done
	VLD1.P 64(R0), [V2.B16, V3.B16, V4.B16, V5.B16]

	VCMEQ V0.B16, V2.B16, V2.B16
	VCMEQ V0.B16, V3.B16, V3.B16
	VCMEQ V0.B16, V4.B16, V4.B16
	VCMEQ V0.B16, V5.B16, V5.B16

	VAND V1.B16, V2.B16, V2.B16
	VAND V1.B16, V3.B16, V3.B16
	VAND V1.B16, V4.B16, V4.B16
	VAND V1.B16, V5.B16, V5.B16

	// ADDP Vd, Vn, Vm -> Go operand order VADDP Vm, Vn, Vd.
	// Round 1: byte pairs -> 2-bit groups (low half from V2, high from V3).
	VADDP V3.B16, V2.B16, V2.B16
	VADDP V5.B16, V4.B16, V4.B16
	// Round 2: 4-bit groups.
	VADDP V4.B16, V2.B16, V2.B16
	// Round 3: full mask bytes in the low 8 bytes of V2.
	VADDP V2.B16, V2.B16, V2.B16

	VMOV V2.D[0], R5
	MOVD.P R5, 8(R2)
	SUB  $1, R1, R1
	B    loop

done:
	RET
