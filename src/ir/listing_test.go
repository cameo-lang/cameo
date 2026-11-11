package ir

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameopb"
)

func nameArg(i uint32) *cameopb.Arg { return &cameopb.Arg{Kind: &cameopb.Arg_Name{Name: i}} }
func procArg(i uint32) *cameopb.Arg { return &cameopb.Arg{Kind: &cameopb.Arg_Proc{Proc: i}} }
func intArg(i int64) *cameopb.Arg   { return &cameopb.Arg{Kind: &cameopb.Arg_Int{Int: i}} }
func intConst(i int64) *cameopb.Arg {
	return &cameopb.Arg{Kind: &cameopb.Arg_Constant{Constant: &cameopb.Value{Kind: &cameopb.Value_IntValue{IntValue: i}}}}
}

// doubleAll is `def double_all(list) = list.map { |x| x * 2 }`, written
// out by hand as the compiler is to emit it.
func doubleAll() *cameopb.Unit {
	const (
		nDoubleAll = iota
		nList
		nMap
		nX
		nTimes
	)
	block := uint64(cameopb.Flag_FLAG_BLOCK)
	return &cameopb.Unit{
		Version: Version,
		File:    "t.cameo",
		Names:   []string{"double_all", "list", "map", "x", "*"},
		Procs: []*cameopb.Proc{
			{Name: "<main>", Line: 1, Code: []*cameopb.Code{
				{Op: cameopb.Op_DEF, Args: []*cameopb.Arg{nameArg(nDoubleAll), procArg(1)}, Line: 1},
			}},
			{Name: "double_all", Line: 1,
				Params: &cameopb.Params{Positional: []*cameopb.Params_Positional{{Name: nList}}},
				Code: []*cameopb.Code{
					{Op: cameopb.Op_LOAD, Args: []*cameopb.Arg{nameArg(nList)}, Line: 1},
					{Op: cameopb.Op_SEND, Args: []*cameopb.Arg{nameArg(nMap), intArg(0), procArg(2)}, Flags: block, Line: 1},
				}},
			{Name: "block", Line: 1,
				Params: &cameopb.Params{Positional: []*cameopb.Params_Positional{{Name: nX}}},
				Code: []*cameopb.Code{
					{Op: cameopb.Op_LOAD, Args: []*cameopb.Arg{nameArg(nX)}, Line: 1},
					{Op: cameopb.Op_CONST, Args: []*cameopb.Arg{intConst(2)}, Line: 1},
					{Op: cameopb.Op_SEND, Args: []*cameopb.Arg{nameArg(nTimes), intArg(1)}, Flags: uint64(cameopb.Flag_FLAG_OPERATOR), Line: 1},
				}},
		},
	}
}

func TestListingShowsOneInstructionPerLine(t *testing.T) {
	assert.Equals(t, Listing(doubleAll()), `proc #0 <main>
  DEF double_all #1

proc #1 double_all(list)
  LOAD list
  SEND map 0 #2 block

proc #2 block(x)
  LOAD x
  CONST 2
  SEND * 1 operator
`)
}

func TestListingIndentsTheListsOfControlFlow(t *testing.T) {
	const (
		nI = iota
		nN
		nLess
		nPlus
	)
	load := func(n uint32) *cameopb.Code {
		return &cameopb.Code{Op: cameopb.Op_LOAD, Args: []*cameopb.Arg{nameArg(n)}}
	}
	op := func(n uint32) *cameopb.Code {
		return &cameopb.Code{Op: cameopb.Op_SEND, Args: []*cameopb.Arg{nameArg(n), intArg(1)}, Flags: uint64(cameopb.Flag_FLAG_OPERATOR)}
	}
	u := &cameopb.Unit{
		Names: []string{"i", "n", "<", "+"},
		Procs: []*cameopb.Proc{{Name: "count",
			Params: &cameopb.Params{Positional: []*cameopb.Params_Positional{{Name: nN}}},
			Code: []*cameopb.Code{
				{Op: cameopb.Op_CONST, Args: []*cameopb.Arg{intConst(0)}},
				{Op: cameopb.Op_STORE, Args: []*cameopb.Arg{nameArg(nI)}},
				{Op: cameopb.Op_DROP},
				{Op: cameopb.Op_WHILE, Code: []*cameopb.Seq{
					{Code: []*cameopb.Code{load(nI), load(nN), op(nLess)}},
					{Code: []*cameopb.Code{load(nI), {Op: cameopb.Op_CONST, Args: []*cameopb.Arg{intConst(1)}}, op(nPlus), {Op: cameopb.Op_STORE, Args: []*cameopb.Arg{nameArg(nI)}}}},
				}},
				{Op: cameopb.Op_DROP},
				load(nI),
			}}},
	}
	assert.Equals(t, Listing(u), `proc #0 count(n)
  CONST 0
  STORE i
  DROP
  WHILE
    cond:
      LOAD i
      LOAD n
      SEND < 1 operator
    body:
      LOAD i
      CONST 1
      SEND + 1 operator
      STORE i
  DROP
  LOAD i
`)
}

// A unit is a message: it reads back from its wire form as it was written.
func TestAUnitRoundTripsThroughItsWireForm(t *testing.T) {
	u := doubleAll()
	data, err := proto.Marshal(u)
	assert.NoError(t, err)
	back := &cameopb.Unit{}
	assert.NoError(t, proto.Unmarshal(data, back))
	assert.True(t, proto.Equal(back, u), "read back %v", back)
	assert.Equals(t, Listing(back), Listing(u))
}
