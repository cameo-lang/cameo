// Package cameopb holds cameo's own protobuf messages: the Go code that
// protoc-gen-go generates from the .proto files under cameo/. The messages
// of a script's classes are not here: those are built when a class is
// loaded (package protogen).
//
// The generated files are committed, so building cameo needs no protoc.
// After changing a .proto file, run `make proto` and commit the result; CI
// fails when the two differ.
package cameopb

import _ "embed"

// ValueProto is the text of cameo/value.proto, the envelope, which `cameo
// proto` writes beside the .proto files of a script's classes.
//
//go:embed cameo/value.proto
var ValueProto string
