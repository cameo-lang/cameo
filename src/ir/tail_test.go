package ir

import (
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The calls named t… are tail calls and the ones named n… are not: the
// last expression of a method, a block and a lambda, through if, case,
// in, &&, || and &., a begin without clauses and the else of a begin
// without an ensure, and the value of a return in a method. A call that
// passes a block (a lambda as an argument is no block), one whose value is used, one in a loop, one a rescue
// clause or an ensure is around, and one at the top level are not.
func TestTailCallsAreMarked(t *testing.T) {
	out := listing(t, `def last(x)
  n1(x)
  t1(x)
end
def branches(x)
  if x
    t2(x)
  elsif n2(x)
    x.t3
  else
    case x
    when n3(x) then t4(x)
    else x && t5(x)
    end
  end
end
def either(x)
  n26(x) && x || x&.t6
end
def patterns(x)
  case x
  in [a] if n4(a) then t7(a)
  else t8(x)
  end
end
def returns(x)
  return t9(x) if x
  while x
    return t10(x)
  end
  begin
    return n5(x)
  rescue
    return n6(x)
  end
  begin
    return n7(x)
  ensure
    n8(x)
  end
  return n9(x) + 1
end
def plain_begin(x)
  begin
    t11(x)
  end
end
def rescued(x)
  begin
    n10(x)
  rescue
    n11(x)
  else
    t12(x)
  end
end
def ensured(x)
  begin
    n12(x)
  rescue
    n13(x)
  else
    n14(x)
  ensure
    n15(x)
  end
end
def used(x)
  y = n16(x)
  n17(y) + 1
end
def with_block(x)
  n18(x) { |v| return v }
end
def with_block_arg(x, &blk)
  n19(x, &blk)
end
def looping(x)
  while n20(x)
    n21(x)
  end
end
def in_block(list)
  n22(list) { |v| t13(v) }
end
def in_lambda
  t15(->(v) { t14(v) })
end
def interpolated(x)
  "#{n24(x)}"
end
n25(1)
`)
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "SEND" {
			continue
		}
		name := fields[1]
		tail := strings.Contains(" "+line+" ", " tail ")
		switch name[0] {
		case 't':
			assert.True(t, tail, name+" is a tail call")
		case 'n':
			assert.True(t, !tail, name+" is no tail call")
		default:
			continue
		}
		seen[name] = true
	}
	assert.Equals(t, len(seen), 15+25)
}
