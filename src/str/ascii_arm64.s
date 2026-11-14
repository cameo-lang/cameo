#include "textflag.h"

// func asciiBlocks(p *byte, n int) int
//
// UMAXV takes the largest of the 16 bytes of a block: one above 127
// says a byte that is not ASCII is among them.
TEXT ·asciiBlocks(SB), NOSPLIT, $0-24
	MOVD p+0(FP), R0
	MOVD n+8(FP), R1
	MOVD $0, R2

loop:
	ADD  $16, R2, R3
	CMP  R1, R3
	BHI  done
	VLD1.P 16(R0), [V0.B16]
	WORD $0x6E30A801 // UMAXV V0.16B, B1: the assembler has no name for it
	VMOV V1.B[0], R4
	CMPW $127, R4
	BHI  done
	MOVD R3, R2
	B    loop

done:
	MOVD R2, ret+16(FP)
	RET
