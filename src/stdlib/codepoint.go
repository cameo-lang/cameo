package stdlib

// EncodeCodepoint is UTF-8's encoding of r, a surrogate included, as Ruby
// writes one it does not validate: what unescapeHTML and JSON.parse give
// for a \uXXXX escape.
func EncodeCodepoint(r rune) string {
	switch {
	case r < 0x80:
		return string([]byte{byte(r)})
	case r < 0x800:
		return string([]byte{0xc0 | byte(r>>6), 0x80 | byte(r)&0x3f})
	case r < 0x10000:
		return string([]byte{0xe0 | byte(r>>12), 0x80 | byte(r>>6)&0x3f, 0x80 | byte(r)&0x3f})
	}
	return string([]byte{0xf0 | byte(r>>18), 0x80 | byte(r>>12)&0x3f, 0x80 | byte(r>>6)&0x3f, 0x80 | byte(r)&0x3f})
}
