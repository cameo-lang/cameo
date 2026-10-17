package object

import (
	"sync"
)

// identity numbers values that have no structure, such as procs, so they can
// be map keys.
var (
	identityMu  sync.Mutex
	identities  = map[Value]uint64{}
	identityNxt uint64
)

func identity(v Value) uint64 {
	identityMu.Lock()
	defer identityMu.Unlock()
	if id, ok := identities[v]; ok {
		return id
	}
	identityNxt++
	identities[v] = identityNxt
	return identityNxt
}
