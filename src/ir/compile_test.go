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
  MAP
  CONST 1
  SYM a
  MAP_SET
  STORE h
  DROP
  MAP
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
      SEND < 1 operator
    body:
      LOAD i
      CONST 1
      SEND + 1 operator
      STORE i
  DROP
  LOAD i
  CONST 2
  SEND > 1 operator
  AND
    LOAD i
    CONST 9
    SEND < 1 operator
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
  MAP
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

func TestCompileReportsWhatItDoesNotCompileYet(t *testing.T) {
	prog, err := syntax.Parse("t.cameo", "x = 1\nclass Point\nend\n")
	assert.NoError(t, err)
	_, err = Compile("t.cameo", prog)
	var unsupported *Unsupported
	assert.True(t, errors.As(err, &unsupported), "Compile returned %v", err)
	assert.Equals(t, err.Error(), "line 2: *syntax.ClassDef does not compile to the IR yet")
}
