package object

import (
	"sync"
	"unsafe"
)

// identity numbers values that have no structure, such as procs, so they can
// be map keys.
var (
	identityMu  sync.Mutex
	identities  = map[identityKey]uint64{}
	identityNxt uint64
)

type identityKey struct {
	kind Kind
	p    unsafe.Pointer
}

func identity(v Value) uint64 {
	identityMu.Lock()
	defer identityMu.Unlock()
	k := identityKey{v.kind, v.p}
	if id, ok := identities[k]; ok {
		return id
	}
	identityNxt++
	identities[k] = identityNxt
	return identityNxt
}
