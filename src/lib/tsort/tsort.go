// Package tsort is Ruby's tsort library, built into cameo: topological
// sorting and strongly connected components by Tarjan's algorithm, as
// module functions over two callables and as the methods of a class that
// includes TSort and defines tsort_each_node and tsort_each_child.
//
// It follows tsort 0.2.0, the version Ruby 3.4 ships, component for
// component in the same order. The id_map and stack arguments of
// each_strongly_connected_component_from, which Ruby lets a caller carry
// between calls, are accepted and start fresh each time, since a Hash is
// a value here.
package tsort

import (
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	module = object.NewModule("TSort")
	cyclic = object.NewClass("Cyclic", object.StandardError)
)

// graph is how the algorithm reaches the nodes and their children: by
// calling the two callables of the module functions, or the two methods
// of an including object.
type graph struct {
	ctx       object.Context
	eachNode  func(yield func(object.Value) error) error
	eachChild func(node object.Value, yield func(object.Value) error) error
}

// call runs a callable, a Proc, with a block that yields to f.
func call(ctx object.Context, callable object.Value, args []object.Value, f func(object.Value) error) error {
	var failed error
	block := ctx.Block(func(bargs []object.Value) (object.Value, error) {
		v := object.Nil
		if len(bargs) > 0 {
			v = bargs[0]
		}
		if err := f(v); err != nil {
			failed = err
			return object.Value{}, err
		}
		return object.Nil, nil
	})
	_, err := ctx.CallWith(callable, "call", args, nil, block)
	if failed != nil {
		return failed
	}
	return err
}

// send runs a method on self with a block that yields to f.
func send(ctx object.Context, self object.Value, name string, args []object.Value, f func(object.Value) error) error {
	var failed error
	block := ctx.Block(func(bargs []object.Value) (object.Value, error) {
		v := object.Nil
		if len(bargs) > 0 {
			v = bargs[0]
		}
		if err := f(v); err != nil {
			failed = err
			return object.Value{}, err
		}
		return object.Nil, nil
	})
	_, err := ctx.CallWith(self, name, args, nil, block)
	if failed != nil {
		return failed
	}
	return err
}

func ofCallables(ctx object.Context, eachNode, eachChild object.Value) *graph {
	return &graph{ctx: ctx,
		eachNode: func(f func(object.Value) error) error { return call(ctx, eachNode, nil, f) },
		eachChild: func(n object.Value, f func(object.Value) error) error {
			return call(ctx, eachChild, []object.Value{n}, f)
		},
	}
}

func ofObject(ctx object.Context, self object.Value) *graph {
	return &graph{ctx: ctx,
		eachNode: func(f func(object.Value) error) error { return send(ctx, self, "tsort_each_node", nil, f) },
		eachChild: func(n object.Value, f func(object.Value) error) error {
			return send(ctx, self, "tsort_each_child", []object.Value{n}, f)
		},
	}
}

// search is the state of Tarjan's algorithm: Ruby's id_map and stack.
type search struct {
	ids   map[string]int
	done  map[string]bool
	stack []object.Value
}

func newSearch() *search { return &search{ids: map[string]int{}, done: map[string]bool{}} }

// from is each_strongly_connected_component_from: the components
// reachable from node, children first, and the minimum id seen.
func (g *graph) from(s *search, node object.Value, yield func([]object.Value) error) (int, error) {
	key := object.Key(node)
	nodeID := len(s.ids)
	s.ids[key] = nodeID
	minimum := nodeID
	stackLength := len(s.stack)
	s.stack = append(s.stack, node)
	err := g.eachChild(node, func(child object.Value) error {
		ck := object.Key(child)
		if id, seen := s.ids[ck]; seen {
			if !s.done[ck] && id < minimum {
				minimum = id
			}
			return nil
		}
		sub, err := g.from(s, child, yield)
		if err != nil {
			return err
		}
		if sub < minimum {
			minimum = sub
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if nodeID == minimum {
		component := append([]object.Value(nil), s.stack[stackLength:]...)
		s.stack = s.stack[:stackLength]
		for _, n := range component {
			s.done[object.Key(n)] = true
		}
		if err := yield(component); err != nil {
			return 0, err
		}
	}
	return minimum, nil
}

// components is each_strongly_connected_component over every node.
func (g *graph) components(yield func([]object.Value) error) error {
	s := newSearch()
	return g.eachNode(func(node object.Value) error {
		if _, seen := s.ids[object.Key(node)]; seen {
			return nil
		}
		_, err := g.from(s, node, yield)
		return err
	})
}

// sorted is tsort_each: each component must be one node.
func (g *graph) sorted(yield func(object.Value) error) error {
	return g.components(func(c []object.Value) error {
		if len(c) != 1 {
			inspect, err := g.ctx.Inspect(object.NewList(c...))
			if err != nil {
				return err
			}
			return &object.Exception{Cls: cyclic, Message: "topological sort failed: " + inspect}
		}
		return yield(c[0])
	})
}

func listOf(c []object.Value) object.Value { return object.NewList(c...) }

// results runs a traversal with the block, or returns the Enumerator of
// it.
func results(ctx object.Context, recv object.Value, meth string, block object.Value, run func(yield func(object.Value) error) error) (object.Value, error) {
	if block.IsZero() {
		return object.EnumeratorValue(&object.Enumerator{Recv: recv, Meth: meth, Each: func(ctx object.Context, yield func(object.Value) error) error {
			return run(yield)
		}}), nil
	}
	err := run(func(v object.Value) error {
		_, err := ctx.Yield(block, v)
		return err
	})
	if err != nil {
		return object.Value{}, err
	}
	return object.Nil, nil
}

// collect runs a traversal into an Array.
func collect(run func(yield func(object.Value) error) error) (object.Value, error) {
	var out []object.Value
	if err := run(func(v object.Value) error {
		out = append(out, v)
		return nil
	}); err != nil {
		return object.Value{}, err
	}
	return object.NewList(out...), nil
}

// Install defines TSort. lib.Install calls it once.
func Install() {
	m := module
	object.DefineNative("TSort", object.ClassValue(m))
	m.Consts["VERSION"] = object.String("0.2.0")
	cyclic.Lexical = m
	m.Consts["Cyclic"] = object.ClassValue(cyclic)

	callables := func(ctx object.Context, args []object.Value) (*graph, error) {
		if len(args) != 2 {
			return nil, stdlib.ArgError(ctx, len(args), "2")
		}
		return ofCallables(ctx, args[0], args[1]), nil
	}
	m.DefineSingleton("tsort", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		g, err := callables(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		return collect(g.sorted)
	})
	m.DefineSingleton("tsort_each", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		g, err := callables(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		return results(ctx, self, "tsort_each", block, g.sorted)
	})
	m.DefineSingleton("strongly_connected_components", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		g, err := callables(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		return collect(func(yield func(object.Value) error) error {
			return g.components(func(c []object.Value) error { return yield(listOf(c)) })
		})
	})
	m.DefineSingleton("each_strongly_connected_component", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		g, err := callables(ctx, args)
		if err != nil {
			return object.Value{}, err
		}
		return results(ctx, self, "each_strongly_connected_component", block, func(yield func(object.Value) error) error {
			return g.components(func(c []object.Value) error { return yield(listOf(c)) })
		})
	})
	m.DefineSingleton("each_strongly_connected_component_from", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 2 || len(args) > 4 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "2..4")
		}
		g := ofCallables(ctx, object.Nil, args[1])
		node := args[0]
		return results(ctx, self, "each_strongly_connected_component_from", block, func(yield func(object.Value) error) error {
			_, err := g.from(newSearch(), node, func(c []object.Value) error { return yield(listOf(c)) })
			return err
		})
	})

	// The instance methods of a class that includes TSort.
	m.Define("tsort", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return collect(ofObject(ctx, self).sorted)
	})
	m.Define("tsort_each", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return results(ctx, self, "tsort_each", block, ofObject(ctx, self).sorted)
	})
	m.Define("strongly_connected_components", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		g := ofObject(ctx, self)
		return collect(func(yield func(object.Value) error) error {
			return g.components(func(c []object.Value) error { return yield(listOf(c)) })
		})
	})
	m.Define("each_strongly_connected_component", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		g := ofObject(ctx, self)
		return results(ctx, self, "each_strongly_connected_component", block, func(yield func(object.Value) error) error {
			return g.components(func(c []object.Value) error { return yield(listOf(c)) })
		})
	})
	m.Define("each_strongly_connected_component_from", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 3 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..3")
		}
		g := ofObject(ctx, self)
		node := args[0]
		return results(ctx, self, "each_strongly_connected_component_from", block, func(yield func(object.Value) error) error {
			_, err := g.from(newSearch(), node, func(c []object.Value) error { return yield(listOf(c)) })
			return err
		})
	})
}
