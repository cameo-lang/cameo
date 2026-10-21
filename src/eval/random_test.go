package eval

import "testing"

// The numbers in these tests are what CRuby 3.4.1 prints for the same
// script.

func TestRandomDrawsCRubysNumbers(t *testing.T) {
	src := `
r = Random.new(42)
p r.rand(0.0...100.0), r.rand, r.rand(1.0..2.0)
p r.rand(6), r.rand(1..6), r.rand(2**40), r.rand(2**70)
p r.bytes(6).unpack1("H*")
p Random.new(2**32 + 5).rand(2**32), Random.new(10**30).rand(2**32), Random.new(-42).rand(2**32)
srand(1)
p rand(100), rand, rand(-7), rand(1.5..2.5)
srand(1)
p [1, 2, 3, 4, 5, 6, 7, 8].shuffle, (1..20).to_a.sample(3), (1..20).to_a.sample(6), (1..100).to_a.sample(12)
`
	expectOutput(t, src, `37.454011884736246
0.9507143064099162
1.731993938512097
4
5
440001501305
333885605099413082442
"ca55917557b8"
2675342405
4012147334
1608637542
37
0.9971848109388686
0
1.5001143811103246
[8, 3, 2, 7, 1, 5, 4, 6]
[6, 17, 1]
[17, 2, 14, 9, 18, 16]
[7, 27, 53, 24, 23, 90, 18, 36, 38, 4, 61, 80]
`)
}

func TestRandomHandleAdvancesForEveryHolder(t *testing.T) {
	// A draw advances the generator the handle names, whoever holds the
	// handle: a second name, a hash, a method's argument or a block.
	src := `
def draw(g) = g.rand(100)
r = Random.new(42)
s = r
h = {gen: r}
p [r.rand(100), s.rand(100), h[:gen].rand(100), draw(r), [r].map { |g| g.rand(100) }.first]
q = Random.new(42)
p (1..5).map { q.rand(100) }
p r == q, r == Random.new(42), Random.new(42) == Random.new(42), r.seed, r
`
	expectOutput(t, src, "[51, 92, 14, 71, 60]\n[51, 92, 14, 71, 60]\ntrue\nfalse\ntrue\n42\n#<Random>\n")
}

func TestRandomIsNotCopied(t *testing.T) {
	// A copy would draw the numbers its original draws next, so dup,
	// clone, async and a typed field refuse a Random.
	for src, want := range map[string]string{
		"Random.new(1).dup\n":   "a Random cannot be copied (TypeError)",
		"Random.new(1).clone\n": "a Random cannot be copied (TypeError)",
		"include Cameo\nclass W\n  def go(r) = r.rand\nend\nasync(W.new, :go, Random.new(1))\n":     "a Random cannot be passed to async (TypeError)",
		"include Cameo\nclass W\n  def go(r) = 1\nend\nasync(W.new, :go, [{gen: Random.new(1)}])\n": "a Random cannot be passed to async (TypeError)",
		"include Cameo\nclass W\n  def make = [Random.new(1)]\nend\nawait(async(W.new, :make))\n":   "a worker returned a Random, which cannot be passed back (TypeError)",
		"include Cameo\nresolve(promise, Random.new(1))\n":                                          "resolve got a Random, which cannot be passed back (TypeError)",
		"class C\n  int :n\nend\nC.new(n: Random.new(1))\n":                                         "field n of C takes an int, not an instance of Random (TypeError)",
		"Random.new(1).rand(0)\n":   "invalid argument - 0 (ArgumentError)",
		"Random.new(1).rand(1..)\n": "Numerical argument out of domain (Errno::EDOM)",
	} {
		expectError(t, src, want)
	}
}
