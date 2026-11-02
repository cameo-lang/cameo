package stdlib

import "github.com/cameo-lang/cameo/src/object"

// What a library under src/lib takes from the core: the argument checks
// every builtin method makes, and the pieces of the core two libraries
// share. A library imports stdlib; stdlib imports no library.

// ArgError is the ArgumentError for a wrong number of arguments:
// "wrong number of arguments (given 2, expected 1..3)".
func ArgError(ctx object.Context, given int, expected string) error {
	return argError(ctx, given, expected)
}

// WantInt reads an Integer argument, a Float truncated, or fails with
// Ruby's TypeError; what follows "into Integer" in the message.
func WantInt(ctx object.Context, v object.Value, what string) (int64, error) {
	return wantInt(ctx, v, what)
}

// WantString reads a String or Symbol argument or fails with Ruby's
// TypeError.
func WantString(ctx object.Context, v object.Value) (string, error) {
	return wantString(ctx, v)
}

// KindName names a value's class as a TypeError does: "nil", "true",
// "Integer".
func KindName(v object.Value) string { return kindName(v) }

// NeedBlock fails with LocalJumpError when a method that needs a block
// has none.
func NeedBlock(ctx object.Context, block object.Value, name string) error {
	return needBlock(ctx, block, name)
}

// EncodeCodepoint is UTF-8's encoding of r, a surrogate included, as
// Ruby writes one it does not validate.
func EncodeCodepoint(r rune) string { return encodeCodepoint(r) }
