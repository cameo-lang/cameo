package ir

import (
	"errors"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/syntax"
)

// listing compiles src and returns its listing.
func listing(t *testing.T, src string) string {
	t.Helper()
	prog, err := syntax.Parse("t.cameo", src)
	assert.NoError(t, err)
	unit, err := Compile("t.cameo", prog)
	assert.NoError(t, err)
	assert.Equals(t, int(unit.GetVersion()), Version)
	return Listing(unit)
}

func TestCompileSeparatesStatementsWithDrop(t *testing.T) {
	assert.Equals(t, listing(t, "a = 1\nb = a\nb\n"), `proc #0 <main>
  CONST 1
  STORE a
  DROP
  LOAD a
  STORE b
  DROP
  LOAD b
`)
}

func TestCompileLiterals(t *testing.T) {
	assert.Equals(t, listing(t, `x = [nil, true, 2, 2.5, "s", :sym, 123456789012345678901234567890]
`), `proc #0 <main>
  CONST nil
  CONST true
  CONST 2
  CONST 2.5
  CONST "s"
  SYM sym
  CONST 123456789012345678901234567890
  LIST 7
  STORE x
`)
}

func TestCompileBuildsAListWithASplatPieceByPiece(t *testing.T) {
	assert.Equals(t, listing(t, "a = [1]\nb = [0, *a, 2]\n"), `proc #0 <main>
  CONST 1
  LIST 1
  STORE a
  DROP
  LIST 0
  CONST 0
  APPEND
  LOAD a
  SPLAT
  CONST 2
  APPEND
  STORE b
`)
}

func TestCompileAMapEvaluatesEachValueBeforeItsKey(t *testing.T) {
	assert.Equals(t, listing(t, "h = {a: 1}\ng = {\"k\" => 2, **h}\n"), `proc #0 <main>
  MAP 1
  CONST 1
  SYM a
  MAP_SET
  STORE h
  DROP
  MAP 1
  CONST 2
  CONST "k"
  MAP_SET
  LOAD h
  MAP_MERGE
  STORE g
`)
}

func TestCompileControlFlowCarriesItsLists(t *testing.T) {
	assert.Equals(t, listing(t, `i = 0
while i < 3
  i = i + 1
end
x = i > 2 && i < 9 ? "in" : "out"
`), `proc #0 <main>
  CONST 0
  STORE i
  DROP
  WHILE
    cond:
      LOAD i
      CONST 3
      BINARY_LT
    body:
      LOAD i
      CONST 1
      BINARY_PLUS
      STORE i
  DROP
  LOAD i
  CONST 2
  BINARY_GT
  AND
    LOAD i
    CONST 9
    BINARY_LT
  IF
    then:
      CONST "in"
    else:
      CONST "out"
  STORE x
`)
}

func TestCompileCalls(t *testing.T) {
	assert.Equals(t, listing(t, `a = [3]
b = a.first(1)
c = a&.push(4)
d = foo(*a, k: 1, &b)
e = "#{b} and #{c}"
`), `proc #0 <main>
  CONST 3
  LIST 1
  STORE a
  DROP
  LOAD a
  CONST 1
  SEND first 1 a
  STORE b
  DROP
  LOAD a
  SAFE
    CONST 4
    SEND push 1 a
  STORE c
  DROP
  LIST 0
  LOAD a
  SPLAT
  MAP 1
  CONST 1
  SYM k
  MAP_SET
  LOAD b
  SEND foo 0 self argv kw block_arg
  STORE d
  DROP
  INTERP
    0:
      CONST ""
    1:
      LOAD b
    2:
      CONST " and "
    3:
      LOAD c
    4:
      CONST ""
  STORE e
`)
}

func TestCompileALambdaIsAProcOfItsOwn(t *testing.T) {
	assert.Equals(t, listing(t, "f = ->(x, y = 2, *rest, k:, **opts, &blk) { x }\ng = proc { 1 }\n"), `proc #0 <main>
  LAMBDA #1 strict
  STORE f
  DROP
  LAMBDA #2
  STORE g

proc #1 block(x, y = …, *rest, k:, **opts, &blk)
  default y:
    CONST 2
  LOAD x

proc #2 block
  CONST 1
`)
}

func TestCompileBeginCarriesItsClausesAsLists(t *testing.T) {
	assert.Equals(t, listing(t, `x = begin
  work
rescue TypeError, KeyError => e
  retry
rescue
  2
else
  3
ensure
  done
end
`), `proc #0 <main>
  BEGIN e "" else ensure
    body:
      SEND work 0 self vcall
    else:
      CONST 3
    ensure:
      SEND done 0 self vcall dropped
    rescue 0 classes:
      GETCONST TypeError
      GETCONST KeyError
      LIST 2
    rescue 0:
      RETRY
    rescue 1 classes:
    rescue 1:
      CONST 2
  STORE x
`)
}

func TestCompileCaseAndPatterns(t *testing.T) {
	assert.Equals(t, listing(t, `v = 1
a = case v
    when 1, 2 then :low
    else :high
    end
b = case v
    in Integer => n if n > 0 then n
    in [x, *rest] then x
    in {k: String, **nil} then 0
    end
`), `proc #0 <main>
  CONST 1
  STORE v
  DROP
  LOAD v
  CASE else subject
    when 0 conds:
      CONST 1
      CONST 2
      LIST 2
    when 0:
      SYM low
    else:
      SYM high
  STORE a
  DROP
  LOAD v
  CASE_IN … => n 1 [x, *rest] 0 {k: …, **nil} 0
    in 0 guard:
      LOAD n
      CONST 0
      BINARY_GT
    in 0:
      LOAD n
    in 1 guard:
    in 1:
      LOAD x
    in 2 guard:
    in 2:
      CONST 0
  STORE b
`)
}

func TestCompileAssignmentForms(t *testing.T) {
	assert.Equals(t, listing(t, "a, *b = 1, 2\nLIMIT = 3\nx = ::LIMIT\ny = Math::PI\nz = /a#{x}/i\nw = /lit/\nd = defined?(a)\n"), `proc #0 <main>
  CONST 1
  CONST 2
  LIST 2
  MULTI_ASSIGN 2 1 a b
  DROP
  CONST 3
  SETCONST LIMIT
  DROP
  SCOPED LIMIT top
  STORE x
  DROP
  GETCONST Math
  SCOPED PI
  STORE y
  DROP
  REGEXP 1
    0:
      CONST "a"
    1:
      VAR_OR_CALL x
  STORE z
  DROP
  REGEXP 0 static
    0:
      CONST "lit"
  STORE w
  DROP
  DEFINED_LOCAL a
  STORE d
`)
}

func TestCompileSuperWithAndAsync(t *testing.T) {
	assert.Equals(t, listing(t, `def go(a, &b)
  super
  super(a, k: 1) { 2 }
  x = a.with(n: 1)
  p = async a, :run, 1
  await p
  await a, :run
  Cameo.await :all
end
`), `proc #0 <main>
  DEF go #1

proc #1 go(a, &b)
  NEED_METHOD
  SUPER 0 forward
  DROP
  NEED_METHOD
  LOAD a
  MAP 1
  CONST 1
  SYM k
  MAP_SET
  SUPER 1 #2 kw block
  DROP
  LOAD a
  MAP 1
  CONST 1
  SYM n
  MAP_SET
  SEND with 0 a kw
  STORE x
  DROP
  NEED_CAMEO async
  LOAD a
  SYM run
  CONST 1
  ASYNC 1
  STORE p
  DROP
  NEED_CAMEO await
  LOAD p
  AWAIT
  DROP
  NEED_CAMEO await
  LOAD a
  SYM run
  AWAIT_CALL 0
  DROP
  AWAIT_ALL qualified

proc #2 block
  CONST 2
`)
}

func TestCompileDefinitions(t *testing.T) {
	assert.Equals(t, listing(t, `module Geo
  class Point < Base
    include Comparable
    attribute :count, default: 0
    int :x, :y, default: 0
    ref :origin, Point
    reserved 9, :old
    def self.zero = new
    def norm = x
    alias size norm
  end
end
class Geo::Line
end
`), `proc #0 <main>
  MODULE Geo #1 42f25bd4201e79a3
  DROP
  GETCONST Geo
  CLASS Line #5 073d4ac4aec98f4c scoped

proc #1 <module:Geo>
  GETCONST Base
  CLASS Point #2 be8b26eda1bcb341 super

proc #2 <class:Point>
  GETCONST Comparable
  INCLUDE
  DROP
  CONST 0
  ATTR count default
  DROP
  CONST 0
  FIELD (int :x :y) default
  DROP
  GETCONST Point
  FIELD (ref :origin) ref
  DROP
  RESERVED 9 old
  DROP
  DEF zero #3 singleton
  DROP
  DEF norm #4
  DROP
  ALIAS size norm

proc #3 zero
  SEND new 0 self vcall

proc #4 norm
  SEND x 0 self vcall

proc #5 <class:Line>
  CONST nil
`)
}

// A splat outside a list is the one node the IR has no instruction for;
// the checker refuses it, so only a tree built by hand has one.
func TestCompileReportsANodeItHasNoInstructionFor(t *testing.T) {
	prog := &syntax.Program{Body: []syntax.Node{&syntax.Splat{Pos: syntax.Pos{L: 3}, Value: &syntax.Nil{}}}}
	_, err := Compile("t.cameo", prog)
	var unsupported *Unsupported
	assert.True(t, errors.As(err, &unsupported), "Compile returned %v", err)
	assert.Equals(t, err.Error(), "line 3: *syntax.Splat does not compile to the IR")
}
