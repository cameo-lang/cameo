package object

import (
	crand "crypto/rand"
	"encoding/binary"
	"math"
	"math/big"
	"sync"
	"unsafe"
)

// Random is a generator a Random handle names: MT19937 seeded as CRuby
// seeds it, so a seeded script draws Ruby's numbers. A draw advances it,
// which every holder of the handle sees; it is the one cameo value that
// changes, as a file handle's position does. A handle does not cross to
// an async process, so its generator stays in the interpreter that made it;
// the lock covers a handle reached through a constant a process shares.
type Random struct {
	mu    sync.Mutex
	state [mtN]uint32
	next  int
	seed  Value
}

const (
	mtN = 624
	mtM = 397
)

// RandomValue wraps a generator as a value.
func RandomValue(r *Random) Value { return Value{kind: KindRandom, p: unsafe.Pointer(r)} }

// Random returns the generator a Random handle names.
func (v Value) Random() *Random {
	v.must(KindRandom)
	return (*Random)(v.p)
}

// AsRandom returns the generator of a Random handle, and false for any
// other value.
func (v Value) AsRandom() (*Random, bool) {
	if v.kind != KindRandom {
		return nil, false
	}
	return (*Random)(v.p), true
}

// NewRandom returns a generator seeded with seed, an Integer.
func NewRandom(seed Value) *Random {
	r := &Random{}
	r.reseed(seed)
	return r
}

// NewSeed returns a random seed, 128 bits as CRuby's Random.new_seed, with
// CRuby's guard word: a 1 above the top word when that word is 0 or 1, so
// that the seed keeps its four words when seeding drops the guard.
func NewSeed() Value {
	var b [16]byte
	_, _ = crand.Read(b[:])
	words := []uint32{
		binary.LittleEndian.Uint32(b[0:]), binary.LittleEndian.Uint32(b[4:]),
		binary.LittleEndian.Uint32(b[8:]), binary.LittleEndian.Uint32(b[12:]),
	}
	if words[3] <= 1 {
		words = append(words, 1)
	}
	n := new(big.Int)
	for i := len(words) - 1; i >= 0; i-- {
		n.Lsh(n, 32).Or(n, big.NewInt(int64(words[i])))
	}
	return BigInt(n)
}

// Seed returns the seed the generator started from.
func (r *Random) Seed() Value { return r.seed }

// Reseed starts the generator again from seed and returns the seed it
// had.
func (r *Random) Reseed(seed Value) Value {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.seed
	r.reseed(seed)
	return old
}

// reseed is rand_init of CRuby's random.c: the seed's magnitude as 32-bit
// words, least significant first. One word seeds with init_genrand; more
// go to init_by_array, without a top word of 1, which is the guard NewSeed
// adds.
func (r *Random) reseed(seed Value) {
	r.seed = seed
	n, _ := seed.AsBigInt()
	n = new(big.Int).Abs(n)
	var key []uint32
	for n.Sign() > 0 {
		key = append(key, uint32(n.Uint64()))
		n.Rsh(n, 32)
	}
	if len(key) <= 1 {
		var w uint32
		if len(key) == 1 {
			w = key[0]
		}
		r.initGenrand(w)
		return
	}
	if key[len(key)-1] == 1 {
		key = key[:len(key)-1]
	}
	r.initByArray(key)
}

func (r *Random) initGenrand(s uint32) {
	r.state[0] = s
	for j := 1; j < mtN; j++ {
		r.state[j] = 1812433253*(r.state[j-1]^(r.state[j-1]>>30)) + uint32(j)
	}
	r.next = mtN
}

func (r *Random) initByArray(key []uint32) {
	r.initGenrand(19650218)
	i, j := 1, 0
	for k := max(mtN, len(key)); k > 0; k-- {
		r.state[i] = (r.state[i] ^ ((r.state[i-1] ^ (r.state[i-1] >> 30)) * 1664525)) + key[j] + uint32(j)
		i++
		j++
		if i >= mtN {
			r.state[0] = r.state[mtN-1]
			i = 1
		}
		if j >= len(key) {
			j = 0
		}
	}
	for k := mtN - 1; k > 0; k-- {
		r.state[i] = (r.state[i] ^ ((r.state[i-1] ^ (r.state[i-1] >> 30)) * 1566083941)) - uint32(i)
		i++
		if i >= mtN {
			r.state[0] = r.state[mtN-1]
			i = 1
		}
	}
	r.state[0] = 0x80000000
	r.next = mtN
}

// genrand is genrand_int32, with the lock held.
func (r *Random) genrand() uint32 {
	if r.next >= mtN {
		for k := 0; k < mtN; k++ {
			y := r.state[k]&0x80000000 | r.state[(k+1)%mtN]&0x7fffffff
			v := r.state[(k+mtM)%mtN] ^ (y >> 1)
			if y&1 != 0 {
				v ^= 0x9908b0df
			}
			r.state[k] = v
		}
		r.next = 0
	}
	y := r.state[r.next]
	r.next++
	y ^= y >> 11
	y ^= (y << 7) & 0x9d2c5680
	y ^= (y << 15) & 0xefc60000
	y ^= y >> 18
	return y
}

// Uint32 returns the next 32 bits.
func (r *Random) Uint32() uint32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.genrand()
}

// Float returns a float in [0, 1), or in [0, 1] when inclusive, from two
// draws, as CRuby's genrand_res53 and int_pair_to_real_inclusive make it.
func (r *Random) Float(inclusive bool) float64 {
	r.mu.Lock()
	a, b := r.genrand(), r.genrand()
	r.mu.Unlock()
	if !inclusive {
		return (float64(a>>5)*67108864 + float64(b>>6)) * (1.0 / 9007199254740992.0)
	}
	// ((a << 32 | b) * (1 << 53 | 1)) >> 64, scaled by 2**-53.
	x := new(big.Int).SetUint64(uint64(a)<<32 | uint64(b))
	x.Mul(x, new(big.Int).SetUint64(1<<53|1)).Rsh(x, 64)
	return math.Ldexp(float64(x.Uint64()), -53)
}

// Below returns an integer in [0, limit], limit included, as CRuby's
// limited_rand and limited_big_rand draw it: 32-bit words from the top,
// each masked to the bits limit has, and drawn again from the top word
// while the result passes limit.
func (r *Random) Below(limit *big.Int) *big.Int {
	if limit.Sign() == 0 {
		return new(big.Int)
	}
	var lim []uint32
	for n := new(big.Int).Set(limit); n.Sign() > 0; n.Rsh(n, 32) {
		lim = append(lim, uint32(n.Uint64()))
	}
	out := make([]uint32, len(lim))
	r.mu.Lock()
retry:
	mask := uint32(0)
	boundary := true
	for i := len(lim) - 1; i >= 0; i-- {
		var v uint32
		if mask != 0 {
			mask = 0xffffffff
		} else {
			mask = makeMask(lim[i])
		}
		if mask != 0 {
			v = r.genrand() & mask
			if boundary {
				if v > lim[i] {
					goto retry
				}
				if v < lim[i] {
					boundary = false
				}
			}
		}
		out[i] = v
	}
	r.mu.Unlock()
	n := new(big.Int)
	for i := len(out) - 1; i >= 0; i-- {
		n.Lsh(n, 32).Or(n, big.NewInt(int64(out[i])))
	}
	return n
}

// BelowInt is Below for a limit that fits 64 bits.
func (r *Random) BelowInt(limit uint64) uint64 {
	if limit <= 0xffffffff {
		if limit == 0 {
			return 0
		}
		mask := makeMask(uint32(limit))
		r.mu.Lock()
		defer r.mu.Unlock()
		for {
			if v := r.genrand() & mask; uint64(v) <= limit {
				return uint64(v)
			}
		}
	}
	return r.Below(new(big.Int).SetUint64(limit)).Uint64()
}

// makeMask returns the bits up to x's top bit set.
func makeMask(x uint32) uint32 {
	x |= x >> 1
	x |= x >> 2
	x |= x >> 4
	x |= x >> 8
	x |= x >> 16
	return x
}

// Bytes returns n bytes, each draw's four in little-endian order, as
// CRuby's rb_rand_bytes_int32.
func (r *Random) Bytes(n int) []byte {
	out := make([]byte, n)
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 0; i < n; i += 4 {
		w := r.genrand()
		for j := i; j < n && j < i+4; j++ {
			out[j] = byte(w)
			w >>= 8
		}
	}
	return out
}

// SameState reports whether two generators have the same seed and stand
// at the same place in their sequence, Ruby's Random#==.
func (r *Random) SameState(o *Random) bool {
	if r == o {
		return true
	}
	r.mu.Lock()
	a, an, as := r.state, r.next, r.seed
	r.mu.Unlock()
	o.mu.Lock()
	b, bn, bs := o.state, o.next, o.seed
	o.mu.Unlock()
	return a == b && an == bn && Equal(as, bs)
}
