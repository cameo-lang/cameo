package object

import (
	"math/big"
	"testing"
)

func TestRandomSeedsAsCRubyDoes(t *testing.T) {
	// The first words of each seed's sequence are what CRuby 3.4.1's
	// Random.new(seed).rand(2**32) returns: a seed of one word seeds
	// init_genrand, more words init_by_array, without a top word of 1.
	for seed, want := range map[string][3]uint32{
		"0":                              {2357136044, 2546248239, 3071714933},
		"1":                              {1791095845, 4282876139, 3093770124},
		"42":                             {1608637542, 3421126067, 4083286876},
		"-42":                            {1608637542, 3421126067, 4083286876},
		"4294967295":                     {419326371, 479346978, 3918654476},
		"4294967296":                     {3626764237, 1654615998, 3255389356},
		"4294967301":                     {2675342405, 1097127993, 3185950873},
		"8589934592":                     {49182197, 2090552240, 2944842790},
		"12884901895":                    {4196490768, 1353615594, 1118595839},
		"18446744073709551616":           {647325673, 1560931061, 839849605},
		"123456789012345678901234567890": {375377819, 626075135, 2526195630},
	} {
		n, _ := new(big.Int).SetString(seed, 10)
		r := NewRandom(BigInt(n))
		got := [3]uint32{r.Uint32(), r.Uint32(), r.Uint32()}
		if got != want {
			t.Errorf("seed %s: got %v, want %v", seed, got, want)
		}
	}
}

func TestRandomPassesTheFirstBlockOfState(t *testing.T) {
	// The 625th word comes from the regenerated state; CRuby 3.4.1 draws
	// the same.
	r := NewRandom(Int(1))
	var last uint32
	for range 625 {
		last = r.Uint32()
	}
	if want := uint32(1104314680); last != want {
		t.Errorf("625th word of seed 1: got %d, want %d", last, want)
	}
}

func TestRandomFloats(t *testing.T) {
	// CRuby 3.4.1: Random.new(42).rand and Random.new(42).rand(0.0..1.0).
	if got := NewRandom(Int(42)).Float(false); got != 0.3745401188473625 {
		t.Errorf("exclusive: got %v", got)
	}
	if got := NewRandom(Int(42)).Float(true); got != 0.3745401144950984 {
		t.Errorf("inclusive: got %v", got)
	}
}
