#include "textflag.h"

// func asciiBlocks(p *byte, n int) int
//
// PMOVMSKB gathers the top bit of each of the 16 bytes of a block: a
// mask that is not zero says a byte above 127 is among them.
TEXT ·asciiBlocks(SB), NOSPLIT, $0-24
	MOVQ p+0(FP), SI
	MOVQ n+8(FP), CX
	XORQ AX, AX

loop:
	LEAQ 16(AX), DX
	CMPQ DX, CX
	JA   done
	MOVOU (SI)(AX*1), X0
	PMOVMSKB X0, BX
	TESTL BX, BX
	JNZ  done
	MOVQ DX, AX
	JMP  loop

done:
	MOVQ AX, ret+16(FP)
	RET
