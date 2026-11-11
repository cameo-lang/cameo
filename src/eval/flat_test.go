package eval

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/ir"
	"github.com/cameo-lang/cameo/src/syntax"
)

// lowered compiles src and returns the flat code of its proc of the given
// name, nil when the proc runs from cursors.
func loweredProc(t *testing.T, src, name string) *flatCode {
	t.Helper()
	prog, err := syntax.Parse("t.cameo", src)
	assert.NoError(t, err)
	unit, err := ir.Compile("t.cameo", prog)
	assert.NoError(t, err)
	in := New(nil, nil)
	for _, p := range unit.Procs {
		if p.GetName() == name {
			return lower(p.GetCode(), in.symsOf(unit), true)
		}
	}
	t.Fatalf("no proc %s", name)
	return nil
}

// A proc of ifs, loops, operators and calls is lowered, with a jump back
// per loop and no instruction that holds lists.
func TestLowerMakesJumpsOfListsItKnows(t *testing.T) {
	flat := loweredProc(t, `def count(n)
  i = 0
  total = 0
  while i < n
    total += i if i.even? && i > 2 || i == 1
    i += 1
  end
  "#{total} of #{n&.to_s}"
end
`, "count")
	assert.True(t, flat != nil)
	ops := map[flatOp]int{}
	for _, ins := range flat.code {
		ops[ins.op]++
		if ins.op == fExec {
			assert.Equals(t, len(ins.src.Code), 0)
		}
	}
	assert.Equals(t, ops[fWhile], 1)
	assert.Equals(t, ops[fBack], 1)
	assert.Equals(t, ops[fTest], 1)
	assert.Equals(t, ops[fInterp], 1)
	assert.True(t, ops[fPart] >= 2)
	assert.True(t, ops[fIf] >= 1 && ops[fAnd] >= 1 && ops[fOr] >= 1 && ops[fSafe] == 1)
	assert.True(t, ops[fBinary] >= 4)
}

// A when of a case is a test and a jump. An instruction with lists that
// flat code has no jumps for, a rescue or a pattern match, is one
// instruction of the flat code, which enters its lists.
func TestLowerKeepsTheListsItHasNoJumpsFor(t *testing.T) {
	count := func(src string, op flatOp) int {
		flat := loweredProc(t, src, "f")
		assert.True(t, flat != nil)
		n := 0
		for _, ins := range flat.code {
			if ins.op == op {
				n++
			}
		}
		return n
	}
	assert.Equals(t, count("def f(x)\n  case x\n  when 1 then :one\n  when 2, 3 then :more\n  else :other\n  end\nend\n", fWhen), 2)
	assert.Equals(t, count("def f(x)\n  begin\n    x + 1\n  rescue => e\n    0\n  end\nend\n", fTree), 1)
	assert.Equals(t, count("def f(x)\n  case x\n  in [a, b] then a\n  else 0\n  end\nend\n", fTree), 1)
}

// A break, next or redo out of a rescue or an ensure inside a loop is the
// loop's, after the ensure clause ran; a case inside a loop and a loop
// inside a case run; and a case without a subject tests its conditions.
func TestLoopsAroundListsOnCursors(t *testing.T) {
	expectOutput(t, `def scan(n)
  out = []
  i = 0
  while i < n
    i += 1
    begin
      next if i == 2
      break :stopped if i == 5
      raise "odd" if i == 3
      out += [i]
    rescue => e
      out += [e.message]
      next
    ensure
      out += [:e]
    end
    out += [:after]
  end
end
def kind(x)
  case x
  when Integer, Float then :number
  when "a".."f" then :letter
  when nil then :none
  else :other
  end
end
def grade(n)
  case
  when n > 8 then :high
  when n > 4 then :mid
  end
end
def count_kinds(list)
  i = 0
  numbers = 0
  while i < list.size
    case list[i]
    when Integer
      numbers += 1
    when :stop
      break
    end
    i += 1
  end
  [numbers, i]
end
def retried
  tries = 0
  i = 0
  while i < 2
    i += 1
    begin
      tries += 1
      redo if tries == 2
    ensure
      tries += 10
    end
  end
  tries
end
p scan(6), scan(2)
p kind(1), kind(2.5), kind("c"), kind(nil), kind(:x), grade(9), grade(5), grade(1)
p count_kinds([1, "a", 2, :stop, 3]), retried
`, `:stopped
nil
:number
:number
:letter
:none
:other
:high
:mid
nil
[2, 3]
22
`)
}

// Loops run from flat code as they run from cursors: break with a value,
// next, redo, a loop in a loop, a loop that runs its body first, a break
// in the condition of an inner loop, and a block's break through a loop.
func TestLoopsOfFlatCode(t *testing.T) {
	expectOutput(t, `def first_square_over(limit)
  i = 0
  while true
    i += 1
    next if i.odd?
    break i * i if i * i > limit
  end
end
def pairs(n)
  out = []
  a = 0
  while a < n
    b = 0
    while b < n
      b += 1
      next if b == a
      break if b > 2
      out += [[a, b]]
    end
    a += 1
  end
  out
end
def once_more
  tries = 0
  i = 0
  while i < 2
    i += 1
    tries += 1
    redo if tries == 1
  end
  tries
end
def body_first
  i = 10
  begin
    i += 1
  end while i < 5
  i
end
def until_found(list)
  i = 0
  i += 1 until list[i] == :x || i >= list.size
  i
end
def find_in_loop(list)
  i = 0
  while i < 3
    list.each { |x| return [i, x] if x > 1 && i == 1 }
    i += 1
  end
  nil
end
def nested_value
  x = while true
    y = while true
      break 7
    end
    break y + 1
  end
  x
end
p first_square_over(50), pairs(3), once_more, body_first
p until_found([1, 2, :x]), until_found([1]), find_in_loop([1, 2]), nested_value
p (while false; end)
`, `64
[[0, 1], [0, 2], [1, 2], [2, 1]]
2
11
2
1
[1, 2]
8
nil
`)
}

// A run that pauses in flat code is frozen as cursors: an activation
// that called is after the instruction that called, and the one that
// runs is before its next instruction, in each list it is inside.
func TestCursorsAtIsThePlaceInTheLists(t *testing.T) {
	prog, err := syntax.Parse("t.cameo", "def f(n)\n  i = 0\n  while i < n\n    if i > 0\n      g(i)\n    end\n    i += 1\n  end\nend\n")
	assert.NoError(t, err)
	unit, err := ir.Compile("t.cameo", prog)
	assert.NoError(t, err)
	in := New(nil, nil)
	var code []*cameopb.Code
	for _, p := range unit.Procs {
		if p.GetName() == "f" {
			code = p.GetCode()
		}
	}
	flat := lower(code, in.symsOf(unit), true)
	assert.True(t, flat != nil)
	// The call of g, inside the if inside the body of the loop.
	call := -1
	for i, ins := range flat.code {
		if ins.op == fExec && ins.src.Op == cameopb.Op_SEND {
			call = i
		}
	}
	assert.True(t, call >= 0)

	m := &machine{in: in, flat: flat, pc: call + 1, bases: []int{0}, flatBody: true}
	m.cursorsAt(true)
	assert.True(t, m.flat == nil)
	assert.Equals(t, len(m.cursors), 3)
	root, body, then := m.cursors[0], m.cursors[1], m.cursors[2]
	assert.True(t, root.owner == nil && root.code[root.next-1].Op == cameopb.Op_WHILE)
	assert.True(t, body.owner.Op == cameopb.Op_WHILE && body.list == 1 && body.phase == phaseBody && body.base == 0)
	assert.True(t, body.code[body.next-1].Op == cameopb.Op_IF)
	assert.True(t, then.owner.Op == cameopb.Op_IF && then.list == 0 && then.phase == phaseValue)
	assert.True(t, then.code[then.next-1].Op == cameopb.Op_SEND)

	// Before the same call: the cursor of the innermost list is at it.
	m = &machine{in: in, flat: flat, pc: call, bases: []int{0}, flatBody: true}
	m.cursorsAt(false)
	then = m.cursors[2]
	assert.True(t, then.code[then.next].Op == cameopb.Op_SEND)
}
