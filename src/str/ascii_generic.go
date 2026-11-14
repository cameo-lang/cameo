//go:build !amd64 && !arm64

package str

import "unsafe"

// asciiBlocks returns how many of the n bytes at p, counted in blocks
// of 16 from the first, are ASCII. A machine without the vector
// instructions this package is written for reads the block as two
// words.
func asciiBlocks(p *byte, n int) int {
	const high = 0x8080808080808080
	i := 0
	for ; i+16 <= n; i += 16 {
		a := *(*uint64)(unsafe.Add(unsafe.Pointer(p), i))
		b := *(*uint64)(unsafe.Add(unsafe.Pointer(p), i+8))
		if (a|b)&high != 0 {
			break
		}
	}
	return i
}
