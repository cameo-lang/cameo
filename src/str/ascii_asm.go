//go:build amd64 || arm64

package str

// asciiBlocks returns how many of the n bytes at p, counted in blocks
// of 16 from the first, are ASCII: it stops before the first block with
// a byte above 127, and before the bytes that fill no block. It is
// written with the vector instructions of the machine, SSE2 on amd64
// and NEON on arm64, which every such machine has.
//
//go:noescape
func asciiBlocks(p *byte, n int) int
