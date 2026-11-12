package ir

import "github.com/cameo-lang/cameo/src/cameopb"

// Tail calls. A call whose value is the value of the method, block or
// lambda it is written in is a tail call: the evaluator runs it in place
// of the call that makes it, which does not stay on the stack
// (doc/design/cameo.md, "Tail calls"). The compiler says which calls are:
// it sets FLAG_TAIL on their SEND once the body of a proc is compiled.
//
// The value of a body is the value of its last instruction. Where that
// is an instruction with lists, it is the value of the list that runs:
// a branch of an if, the body of a when or an in, the right side of &&,
// || and &., the body of a begin without clauses, and the else of a
// begin without an ensure. In a method, the value a return is given is
// the method's too, unless a begin around the return has a rescue clause
// or an ensure to run after it.
//
// A call that passes a block is not a tail call: the block may return
// from the method it is written in, which is then to be on the stack.
// Neither is one in a rescue clause, which the exception it handles is
// the cause of what is raised in (the clause ends after the call).

// markTail sets FLAG_TAIL on the tail calls of code, the body of a proc.
// returns is set for the body of a method, where a return ends the
// method.
func markTail(code []*cameopb.Code, returns bool) {
	tailOf(code)
	if returns {
		tailReturns(code, false)
	}
}

// tailOf marks the calls the value of the list code is the value of.
func tailOf(code []*cameopb.Code) {
	if len(code) > 0 {
		tailValue(code[len(code)-1])
	}
}

// tailValue marks the calls the value of the instruction c is the value
// of.
func tailValue(c *cameopb.Code) {
	lists := c.GetCode()
	switch c.Op {
	case cameopb.Op_SEND:
		if c.Flags&uint64(cameopb.Flag_FLAG_BLOCK|cameopb.Flag_FLAG_BLOCK_ARG|cameopb.Flag_FLAG_DROPPED) != 0 {
			return
		}
		for _, a := range c.Args {
			if _, ok := a.GetKind().(*cameopb.Arg_Proc); ok {
				return
			}
		}
		c.Flags |= uint64(cameopb.Flag_FLAG_TAIL)
	case cameopb.Op_IF, cameopb.Op_AND, cameopb.Op_OR, cameopb.Op_SAFE:
		for _, list := range lists {
			tailOf(list.GetCode())
		}
	case cameopb.Op_CASE:
		// Two lists per when, its conditions and its body, then the else.
		for i := 1; i < len(lists); i += 2 {
			tailOf(lists[i].GetCode())
		}
		if len(lists) > 0 {
			tailOf(lists[len(lists)-1].GetCode())
		}
	case cameopb.Op_CASE_IN:
		// Two lists per clause, its guard and its body, then the else
		// when it has one.
		clauses := len(c.Args) / 2
		for i := 0; i < clauses && 2*i+1 < len(lists); i++ {
			tailOf(lists[2*i+1].GetCode())
		}
		if c.Flags&uint64(cameopb.Flag_FLAG_ELSE) != 0 && len(lists) > 0 {
			tailOf(lists[len(lists)-1].GetCode())
		}
	case cameopb.Op_BEGIN:
		// The body, the else and the ensure, then two lists per rescue
		// clause.
		if len(lists) < 3 || c.Flags&uint64(cameopb.Flag_FLAG_ENSURE) != 0 {
			return
		}
		switch {
		case c.Flags&uint64(cameopb.Flag_FLAG_ELSE) != 0:
			tailOf(lists[1].GetCode())
		case len(lists) == 3:
			tailOf(lists[0].GetCode())
		}
	}
}

// tailReturns marks the calls a return in code, or in the lists of its
// instructions, is given the value of. guarded is set inside a begin
// that has something to do when the return leaves it.
func tailReturns(code []*cameopb.Code, guarded bool) {
	for i, c := range code {
		if c.Op == cameopb.Op_RETURN && i > 0 && !guarded {
			tailValue(code[i-1])
		}
		for j, list := range c.GetCode() {
			inner := guarded
			if c.Op == cameopb.Op_BEGIN {
				// A return out of the body is the business of the rescue
				// clauses' frame and of the ensure; one out of a clause
				// ends the clause, and one out of the else or a clause
				// runs the ensure.
				clauses := len(c.GetCode()) > 3
				ensure := c.Flags&uint64(cameopb.Flag_FLAG_ENSURE) != 0
				switch {
				case j == 0:
					inner = inner || clauses || ensure
				case j == 1:
					inner = inner || ensure
				default:
					inner = true
				}
			}
			tailReturns(list.GetCode(), inner)
		}
	}
}
