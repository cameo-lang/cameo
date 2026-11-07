package readline

import (
	"bytes"
	"io"
	"sync/atomic"
	"unicode/utf8"
)

// keyReader is the stream's keys. It notes a Ctrl-C, which the editor ends
// the line on as it does on Ctrl-D, so that ReadLine can tell the two apart, and
// ends a read at it: the keys after it go to the next read, which the new
// editor ReadLine starts after a Ctrl-C, where the old one would keep them.
// It rewrites the Alt chords first; see altKeys.
type keyReader struct {
	r     io.Reader
	ctrlC atomic.Bool
	rest  []byte
	// held is the end of the last read that altKeys cannot tell yet: an Esc,
	// which the next key makes a chord or the start of a sequence.
	held []byte
}

func (k *keyReader) Read(p []byte) (int, error) {
	for len(k.rest) == 0 {
		buf := make([]byte, len(p))
		n, err := k.r.Read(buf)
		if n == 0 {
			// An Esc held at the end of the stream was a lone Esc.
			k.held = nil
			return 0, err
		}
		k.rest, k.held = altKeys(append(k.held, buf[:n]...))
	}
	chunk := k.rest
	if i := bytes.IndexByte(chunk, 3); i >= 0 {
		chunk = chunk[:i+1]
		k.ctrlC.Store(true)
	}
	n := copy(p, chunk)
	k.rest = k.rest[n:]
	return n, nil
}

const esc = 0x1b

// altKeys rewrites the Alt chords in b, an Esc and a key, which a terminal
// sends for Option-Backspace, or Cmd-Backspace where it is set up so. x/term
// knows none: it takes an Esc and a key for the start of an escape sequence,
// and swallows the keys after it, the Enter too, until one that can end a
// sequence, a letter or ~. Alt-Backspace becomes Ctrl-W, which deletes the
// word before the cursor, and Alt-B and Alt-F the Alt arrows, which move by
// a word, as in readline. An Esc before [ or O starts a sequence and stays.
// The other chords go, and of an Esc before a control key, such as Enter,
// the Esc goes. held is an Esc at the end of b, and an incomplete character
// after it, which the next read tells.
func altKeys(b []byte) (keys, held []byte) {
	if bytes.IndexByte(b, esc) < 0 {
		return b, nil
	}
	keys = make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		if b[i] != esc {
			keys = append(keys, b[i])
			continue
		}
		after := b[i+1:]
		if len(after) == 0 || !utf8.FullRune(after) {
			return keys, append([]byte(nil), b[i:]...)
		}
		switch next := after[0]; {
		case next == '[' || next == 'O':
			keys = append(keys, esc)
		case next == 0x7f || next == 8:
			keys = append(keys, 23) // Ctrl-W
			i++
		case next == 'b':
			keys = append(keys, "\x1b[1;3D"...)
			i++
		case next == 'f':
			keys = append(keys, "\x1b[1;3C"...)
			i++
		case next < ' ':
			// The Esc goes and the control key stays; an Esc starts again.
		default:
			_, size := utf8.DecodeRune(after)
			i += size
		}
	}
	return keys, nil
}
