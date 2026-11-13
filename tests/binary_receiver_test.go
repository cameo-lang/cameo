package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// Every kind of String method raises for a binary receiver when it reads
// characters: one the encodings wrap, one defined again for US-ASCII
// (upcase, each_char, each_line) and one that reads the encoding itself
// (to_i, ord, to_sym, casecmp). The methods on bytes and the US-ASCII
// ones run.
func TestAMethodThatReadsCharactersRaisesForABinaryReceiver(t *testing.T) {
	out := runScript(t, `s = b"\xffab"
names = %w[upcase downcase swapcase capitalize each_char each_line chars strip to_i to_f hex oct ord to_sym intern casecmp casecmp? reverse center tr split lines succ]
names.each do |n|
  args = case n
         when "center" then [9]
         when "tr" then ["a", "b"]
         when "casecmp", "casecmp?" then ["x"]
         else []
         end
  begin
    s.send(n, *args)
    puts "#{n}: ran"
  rescue => e
    puts "#{n}: #{e.class}"
  end
end
a = "abc".force_encoding("US-ASCII")
p a.upcase, a.upcase.encoding, a.capitalize, a.swapcase, a.each_char.to_a, b"ab".bytes, b"\xffab".length, b"\xffab"[1], b"a\xff".sub(b"\xff", b"b")
`)
	assert.Equals(t, out, `upcase: Encoding::CompatibilityError
downcase: Encoding::CompatibilityError
swapcase: Encoding::CompatibilityError
capitalize: Encoding::CompatibilityError
each_char: Encoding::CompatibilityError
each_line: Encoding::CompatibilityError
chars: Encoding::CompatibilityError
strip: Encoding::CompatibilityError
to_i: Encoding::CompatibilityError
to_f: Encoding::CompatibilityError
hex: Encoding::CompatibilityError
oct: Encoding::CompatibilityError
ord: Encoding::CompatibilityError
to_sym: Encoding::CompatibilityError
intern: Encoding::CompatibilityError
casecmp: Encoding::CompatibilityError
casecmp?: Encoding::CompatibilityError
reverse: Encoding::CompatibilityError
center: Encoding::CompatibilityError
tr: Encoding::CompatibilityError
split: Encoding::CompatibilityError
lines: Encoding::CompatibilityError
succ: Encoding::CompatibilityError
"ABC"
#<Encoding:US-ASCII>
"Abc"
"ABC"
["a", "b", "c"]
[97, 98]
3
"a"
"ab"
`)
}
