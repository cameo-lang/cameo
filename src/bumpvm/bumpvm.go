// Package bumpvm is an experiment, not part of cameo: a small evaluator
// for a subset of cameo's IR that keeps every value on a heap of its own,
// to see how fast cameo could run without Go's garbage collector.
//
// A value is one word. An integer, nil, true, false, a symbol and a
// string of up to seven bytes are in the word itself; a longer string, an
// array and a hash are objects on the heap, and the word is where. The
// heap is one slice of words Go's collector does not scan. An object is
// allocated by moving a pointer up (bump allocation), and when the heap
// is full the objects that can still be reached are copied to a second
// heap (a copying collector), which leaves the garbage behind.
//
// It runs integers, locals, while and if, calls of methods a script
// defines, strings, interpolation, array and hash literals, [] on both,
// size, to_s and p. Anything else panics.
package bumpvm

import (
	"fmt"
	"io"
	"strconv"
	"sync/atomic"
	"unsafe"

	"github.com/cameo-lang/cameo/src/cameopb"
)

// Value is one value: see the package comment.
//
//	…1    an integer, in the upper 63 bits
//	…000  an object on the heap, at the word the upper bits number (never 0)
//	…010  nil (2), false (10), true (18)
//	…100  a string of up to seven bytes: its length in bits 3 to 5, its
//	      bytes from bit 8 on
//	…110  a symbol, its number in the upper bits
type Value = uint64

const (
	vNil   Value = 2
	vFalse Value = 10
	vTrue  Value = 18

	tagMask   = 7
	tagObject = 0
	tagString = 4
	tagSymbol = 6
)

// The kinds of object, in the low byte of an object's first word; the
// rest of that word is a string's length in bytes, an array's in
// elements, or the entries a hash has room for. A forwarded object has
// its new place there.
const (
	kindString = 1 + iota
	kindArray
	kindHash
	kindForwarded
)

func fromInt(i int64) Value { return Value(i)<<1 | 1 }
func isInt(v Value) bool    { return v&1 == 1 }
func toInt(v Value) int64   { return int64(v) >> 1 }
func truthy(v Value) bool   { return v != vNil && v != vFalse }
func isObject(v Value) bool { return v&tagMask == tagObject && v != 0 }

func fromBool(b bool) Value {
	if b {
		return vTrue
	}
	return vFalse
}

// VM runs one program.
type VM struct {
	mem, other []uint64
	free       int
	stack      []Value
	sp         int
	consts     []Value
	procs      []*proc
	methods    []int32
	symbols    []string
	out        io.Writer
	stop       atomic.Bool

	// Collections counts the runs of the collector, Copied the words
	// they copied and Allocated the words allocated in all.
	Collections, Copied, Allocated int
}

type proc struct {
	code   []ins
	locals int
	params int
}

type ins struct {
	op   uint8
	a, b int32
}

const (
	opConst = iota
	opNil
	opLoad
	opStore
	opPop
	opBinary
	opJump
	opJumpIfNot
	opCheck
	opCall
	opRet
	opSend
	opList
	opMap
	opMapSet
	opInterpAdd
	opSym
)

const (
	sendP = iota
	sendIndex
	sendToS
	sendSize
)

// New makes a VM whose heap has room for words words before it collects.
func New(words int, out io.Writer) *VM {
	return &VM{mem: make([]uint64, words), other: make([]uint64, words), free: 1, stack: make([]Value, 1<<20), out: out}
}

// --- the heap ---

// alloc returns the place of n new words. It may collect: every value
// held outside the stack and the constants is then stale.
func (vm *VM) alloc(n int) int {
	if vm.free+n > len(vm.mem) {
		vm.collect(n)
	}
	p := vm.free
	vm.free += n
	vm.Allocated += n
	return p
}

// words is how many words the object at p of mem takes.
func words(mem []uint64, p int) int {
	h := mem[p]
	n := int(h >> 8)
	switch h & 0xff {
	case kindString:
		return 1 + (n+7)/8
	case kindArray:
		return 1 + n
	case kindHash:
		return 2 + indexSize(n) + 2*n
	}
	panic("bumpvm: no object")
}

// indexSize is the size of the index of a hash with room for n entries:
// a power of two at least twice n.
func indexSize(n int) int {
	size := 4
	for size < 2*n {
		size *= 2
	}
	return size
}

// collect copies what the stack and the constants reach into the other
// heap, which becomes the heap; need is the room the caller is about to
// take.
func (vm *VM) collect(need int) {
	vm.Collections++
	from, to := vm.mem, vm.other
	free := 1
	move := func(v Value) Value {
		if !isObject(v) {
			return v
		}
		p := int(v >> 3)
		h := from[p]
		if h&0xff == kindForwarded {
			return Value(h>>8) << 3
		}
		n := words(from, p)
		if free+n > len(to) {
			panic("bumpvm: the heap is too small for what is alive")
		}
		copy(to[free:free+n], from[p:p+n])
		from[p] = uint64(free)<<8 | kindForwarded
		v = Value(free) << 3
		free += n
		return v
	}
	for i := 0; i < vm.sp; i++ {
		vm.stack[i] = move(vm.stack[i])
	}
	for i := range vm.consts {
		vm.consts[i] = move(vm.consts[i])
	}
	for scan := 1; scan < free; {
		h := to[scan]
		n := int(h >> 8)
		switch h & 0xff {
		case kindArray:
			for i := scan + 1; i <= scan+n; i++ {
				to[i] = move(to[i])
			}
		case kindHash:
			entries := scan + 2 + indexSize(n)
			for i := entries; i < entries+2*int(to[scan+1]); i++ {
				to[i] = move(to[i])
			}
		}
		scan += words(to, scan)
	}
	vm.Copied += free - 1
	vm.mem, vm.other, vm.free = to, from, free
	if free+need > len(to)*3/4 {
		// More is alive than a heap of this size leaves room beside:
		// the next heap is twice what is needed.
		vm.other = make([]uint64, 2*(free+need))
		vm.collect(need)
		vm.other = make([]uint64, len(vm.mem))
	}
}

// --- strings ---

// strLen is the length of the string v in bytes.
func (vm *VM) strLen(v Value) int {
	if v&tagMask == tagString {
		return int(v>>3) & 7
	}
	return int(vm.mem[v>>3] >> 8)
}

func (vm *VM) isString(v Value) bool {
	return v&tagMask == tagString || isObject(v) && vm.mem[v>>3]&0xff == kindString
}

// bytes returns the bytes of the string v: from buf for a string in the
// word, from the heap otherwise, where they stay until something is
// allocated.
func (vm *VM) bytes(v Value, buf *[8]byte) []byte {
	if v&tagMask == tagString {
		*(*uint64)(unsafe.Pointer(buf)) = v >> 8
		return buf[:int(v>>3)&7]
	}
	p := int(v >> 3)
	return unsafe.Slice((*byte)(unsafe.Pointer(&vm.mem[p+1])), int(vm.mem[p]>>8))
}

// short makes the string of b, at most seven bytes, in a word.
func short(b []byte) Value {
	var bits uint64
	for i, c := range b {
		bits |= uint64(c) << (8 * i)
	}
	return bits<<8 | uint64(len(b))<<3 | tagString
}

// newString makes the string of b, which is not on the heap.
func (vm *VM) newString(b []byte) Value {
	if len(b) <= 7 {
		return short(b)
	}
	p := vm.allocString(len(b))
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&vm.mem[p+1])), len(b)), b)
	return Value(p) << 3
}

// allocString allocates a string of n bytes, its last word cleared.
func (vm *VM) allocString(n int) int {
	w := 1 + (n+7)/8
	p := vm.alloc(w)
	vm.mem[p] = uint64(n)<<8 | kindString
	vm.mem[p+w-1] = 0
	return p
}

// concat replaces the two strings on top of the stack with the one they
// make. The strings are read from the stack after the allocation, which
// may have moved them.
func (vm *VM) concat(sp int) {
	la, lb := vm.strLen(vm.stack[sp-2]), vm.strLen(vm.stack[sp-1])
	switch {
	case lb == 0:
		return
	case la == 0:
		vm.stack[sp-2] = vm.stack[sp-1]
		return
	}
	var x, y [8]byte
	if la+lb <= 7 {
		var both [16]byte
		copy(both[:], vm.bytes(vm.stack[sp-2], &x))
		copy(both[la:], vm.bytes(vm.stack[sp-1], &y))
		vm.stack[sp-2] = short(both[:la+lb])
		return
	}
	vm.sp = sp
	p := vm.allocString(la + lb)
	dst := unsafe.Slice((*byte)(unsafe.Pointer(&vm.mem[p+1])), la+lb)
	copy(dst, vm.bytes(vm.stack[sp-2], &x))
	copy(dst[la:], vm.bytes(vm.stack[sp-1], &y))
	vm.stack[sp-2] = Value(p) << 3
}

// toS replaces the value at i of the stack with its string.
func (vm *VM) toS(i int, sp int) {
	v := vm.stack[i]
	switch {
	case vm.isString(v):
	case isInt(v):
		var buf [24]byte
		vm.sp = sp
		vm.stack[i] = vm.newString(strconv.AppendInt(buf[:0], toInt(v), 10))
	default:
		panic("bumpvm: to_s of this value is not supported")
	}
}

// --- hashes ---

// hash is the hash of a key.
func (vm *VM) hash(k Value) uint64 {
	if !isObject(k) {
		k *= 0x9e3779b97f4a7c15
		return k ^ k>>32
	}
	p := int(k >> 3)
	h := vm.mem[p]
	if h&0xff != kindString {
		panic("bumpvm: this value is no key")
	}
	n := 1 + (int(h>>8)+7)/8
	for _, w := range vm.mem[p+1 : p+n] {
		h = (h ^ w) * 0x9e3779b97f4a7c15
	}
	return h ^ h>>32
}

// same reports whether two keys are equal: two words that are, or two
// strings on the heap of the same bytes.
func (vm *VM) same(a, b Value) bool {
	if a == b {
		return true
	}
	if !isObject(a) || !isObject(b) {
		return false
	}
	p, q := int(a>>3), int(b>>3)
	if vm.mem[p] != vm.mem[q] || vm.mem[p]&0xff != kindString {
		return false
	}
	n := (int(vm.mem[p]>>8) + 7) / 8
	for i := 1; i <= n; i++ {
		if vm.mem[p+i] != vm.mem[q+i] {
			return false
		}
	}
	return true
}

// hashFind returns where the entry of k is in the hash at p, and where
// in its index the key is or goes.
func (vm *VM) hashFind(p int, k Value) (entry, slot int) {
	n := int(vm.mem[p] >> 8)
	size := indexSize(n)
	index, entries := p+2, p+2+size
	for i := int(vm.hash(k)) & (size - 1); ; i = (i + 1) & (size - 1) {
		e := int(vm.mem[index+i])
		if e == 0 {
			return -1, index + i
		}
		if at := entries + 2*(e-1); vm.same(vm.mem[at], k) {
			return at, index + i
		}
	}
}

// --- running ---

// Run runs the program from its first proc to its end.
func (vm *VM) Run() {
	type frame struct {
		code   []ins
		pc, fp int
	}
	stack := vm.stack
	main := vm.procs[0]
	code, pc, fp, sp := main.code, 0, 0, main.locals
	for i := 0; i < sp; i++ {
		stack[i] = vNil
	}
	frames := make([]frame, 0, 1024)
	for {
		in := code[pc]
		pc++
		switch in.op {
		case opConst:
			stack[sp] = vm.consts[in.a]
			sp++
		case opNil:
			stack[sp] = vNil
			sp++
		case opSym:
			stack[sp] = Value(in.a)<<3 | tagSymbol
			sp++
		case opLoad:
			stack[sp] = stack[fp+int(in.a)]
			sp++
		case opStore:
			stack[fp+int(in.a)] = stack[sp-1]
		case opPop:
			sp--
		case opJump:
			pc = int(in.a)
		case opJumpIfNot:
			sp--
			if !truthy(stack[sp]) {
				pc = int(in.a)
			}
		case opCheck:
			if vm.stop.Load() {
				return
			}
		case opBinary:
			a, b := stack[sp-2], stack[sp-1]
			if a&b&1 == 1 {
				var r Value
				switch cameopb.Op(in.a) {
				case cameopb.Op_BINARY_PLUS:
					r = a + b - 1
					if (int64(a) >= 0) == (int64(b) >= 0) && (int64(r) >= 0) != (int64(a) >= 0) {
						panic("bumpvm: integer overflow")
					}
				case cameopb.Op_BINARY_MINUS:
					r = a - b + 1
					if (int64(a) >= 0) != (int64(b) >= 0) && (int64(r) >= 0) != (int64(a) >= 0) {
						panic("bumpvm: integer overflow")
					}
				case cameopb.Op_BINARY_TIMES:
					r = fromInt(toInt(a) * toInt(b))
				case cameopb.Op_BINARY_MODULO:
					x, y := toInt(a), toInt(b)
					m := x % y
					if m != 0 && (m < 0) != (y < 0) {
						m += y
					}
					r = fromInt(m)
				case cameopb.Op_BINARY_LT:
					r = fromBool(int64(a) < int64(b))
				case cameopb.Op_BINARY_LE:
					r = fromBool(int64(a) <= int64(b))
				case cameopb.Op_BINARY_GT:
					r = fromBool(int64(a) > int64(b))
				case cameopb.Op_BINARY_GE:
					r = fromBool(int64(a) >= int64(b))
				case cameopb.Op_BINARY_EQ:
					r = fromBool(a == b)
				case cameopb.Op_BINARY_NE:
					r = fromBool(a != b)
				default:
					panic("bumpvm: operator not supported")
				}
				sp--
				stack[sp-1] = r
				continue
			}
			switch {
			case cameopb.Op(in.a) == cameopb.Op_BINARY_PLUS && vm.isString(a) && vm.isString(b):
				vm.concat(sp)
				sp--
			case cameopb.Op(in.a) == cameopb.Op_BINARY_EQ:
				sp--
				stack[sp-1] = fromBool(vm.same(a, b))
			default:
				panic("bumpvm: operator not supported on these values")
			}
		case opCall:
			if vm.stop.Load() {
				return
			}
			p := vm.procs[vm.methods[in.a]]
			if int(in.b) != p.params {
				panic("bumpvm: wrong number of arguments")
			}
			frames = append(frames, frame{code, pc, fp})
			fp = sp - p.params
			for i := sp; i < fp+p.locals; i++ {
				stack[i] = vNil
			}
			sp = fp + p.locals
			code, pc = p.code, 0
		case opRet:
			v := stack[sp-1]
			if len(frames) == 0 {
				vm.sp = 0
				return
			}
			sp = fp
			stack[sp] = v
			sp++
			f := frames[len(frames)-1]
			frames = frames[:len(frames)-1]
			code, pc, fp = f.code, f.pc, f.fp
		case opList:
			n := int(in.a)
			vm.sp = sp
			p := vm.alloc(1 + n)
			vm.mem[p] = uint64(n)<<8 | kindArray
			copy(vm.mem[p+1:p+1+n], stack[sp-n:sp])
			sp -= n
			stack[sp] = Value(p) << 3
			sp++
		case opMap:
			n := int(in.a)
			vm.sp = sp
			w := 2 + indexSize(n) + 2*n
			p := vm.alloc(w)
			clear(vm.mem[p : p+w])
			vm.mem[p] = uint64(n)<<8 | kindHash
			stack[sp] = Value(p) << 3
			sp++
		case opMapSet:
			// The hash, the value and the key, the key on top.
			p := int(stack[sp-3] >> 3)
			k, v := stack[sp-1], stack[sp-2]
			at, slot := vm.hashFind(p, k)
			if at < 0 {
				count := int(vm.mem[p+1])
				at = p + 2 + indexSize(int(vm.mem[p]>>8)) + 2*count
				vm.mem[at] = k
				vm.mem[p+1] = uint64(count + 1)
				vm.mem[slot] = uint64(count + 1)
			}
			vm.mem[at+1] = v
			sp -= 2
		case opInterpAdd:
			vm.toS(sp-1, sp)
			vm.concat(sp)
			sp--
		case opSend:
			switch in.a {
			case sendP:
				n := int(in.b)
				for _, v := range stack[sp-n : sp] {
					vm.inspect(v)
				}
				sp -= n
				stack[sp] = vNil
				sp++
			case sendIndex:
				recv, k := stack[sp-2], stack[sp-1]
				sp--
				if !isObject(recv) {
					panic("bumpvm: [] on this value is not supported")
				}
				p := int(recv >> 3)
				switch h := vm.mem[p]; h & 0xff {
				case kindArray:
					i := toInt(k)
					if !isInt(k) || i < 0 || i >= int64(h>>8) {
						stack[sp-1] = vNil
					} else {
						stack[sp-1] = vm.mem[p+1+int(i)]
					}
				case kindHash:
					if at, _ := vm.hashFind(p, k); at >= 0 {
						stack[sp-1] = vm.mem[at+1]
					} else {
						stack[sp-1] = vNil
					}
				default:
					panic("bumpvm: [] on this value is not supported")
				}
			case sendToS:
				vm.toS(sp-1, sp)
			case sendSize:
				v := stack[sp-1]
				switch {
				case vm.isString(v):
					stack[sp-1] = fromInt(int64(vm.strLen(v)))
				case isObject(v) && vm.mem[v>>3]&0xff == kindArray:
					stack[sp-1] = fromInt(int64(vm.mem[v>>3] >> 8))
				case isObject(v) && vm.mem[v>>3]&0xff == kindHash:
					stack[sp-1] = fromInt(int64(vm.mem[v>>3+1]))
				default:
					panic("bumpvm: size of this value is not supported")
				}
			}
		}
	}
}

// inspect prints v as p does.
func (vm *VM) inspect(v Value) {
	var buf [8]byte
	switch {
	case isInt(v):
		fmt.Fprintln(vm.out, toInt(v))
	case vm.isString(v):
		fmt.Fprintf(vm.out, "%q\n", vm.bytes(v, &buf))
	case v == vNil:
		fmt.Fprintln(vm.out, "nil")
	case v == vTrue || v == vFalse:
		fmt.Fprintln(vm.out, v == vTrue)
	case v&tagMask == tagSymbol:
		fmt.Fprintln(vm.out, ":"+vm.symbols[v>>3])
	default:
		panic("bumpvm: p of this value is not supported")
	}
}

// --- compiling the IR ---

// Load compiles unit for the VM.
func (vm *VM) Load(unit *cameopb.Unit) {
	vm.methods = make([]int32, len(unit.Names))
	for i := range vm.methods {
		vm.methods[i] = -1
	}
	vm.procs = make([]*proc, len(unit.Procs))
	symbols := map[string]int32{}
	for i, src := range unit.Procs {
		c := &compiler{vm: vm, unit: unit, slots: map[uint32]int32{}, symbols: symbols}
		for _, param := range src.GetParams().GetPositional() {
			c.slot(param.GetName())
		}
		params := len(c.slots)
		c.list(src.GetCode())
		c.emit(opRet, 0, 0)
		vm.procs[i] = &proc{code: c.code, locals: len(c.slots), params: params}
	}
}

type compiler struct {
	vm      *VM
	unit    *cameopb.Unit
	code    []ins
	slots   map[uint32]int32
	symbols map[string]int32
}

func (c *compiler) emit(op uint8, a, b int32) int {
	c.code = append(c.code, ins{op, a, b})
	return len(c.code) - 1
}

func (c *compiler) slot(name uint32) int32 {
	s, ok := c.slots[name]
	if !ok {
		s = int32(len(c.slots))
		c.slots[name] = s
	}
	return s
}

func (c *compiler) constant(v Value) {
	c.vm.consts = append(c.vm.consts, v)
	c.emit(opConst, int32(len(c.vm.consts)-1), 0)
}

func (c *compiler) list(code []*cameopb.Code) {
	for _, in := range code {
		c.ins(in)
	}
}

func (c *compiler) ins(in *cameopb.Code) {
	name := func(i int) uint32 { return in.Args[i].GetName() }
	switch in.Op {
	case cameopb.Op_CONST:
		switch k := in.Args[0].GetConstant().GetKind().(type) {
		case nil:
			c.emit(opNil, 0, 0)
		case *cameopb.Constant_BoolValue:
			c.constant(fromBool(k.BoolValue))
		case *cameopb.Constant_IntValue:
			c.constant(fromInt(k.IntValue))
		case *cameopb.Constant_StringValue:
			c.constant(c.vm.newString([]byte(k.StringValue)))
		default:
			panic("bumpvm: constant not supported")
		}
	case cameopb.Op_SYM:
		s := c.unit.Names[name(0)]
		id, ok := c.symbols[s]
		if !ok {
			id = int32(len(c.vm.symbols))
			c.symbols[s] = id
			c.vm.symbols = append(c.vm.symbols, s)
		}
		c.emit(opSym, id, 0)
	case cameopb.Op_LOAD:
		c.emit(opLoad, c.slot(name(0)), 0)
	case cameopb.Op_STORE:
		c.emit(opStore, c.slot(name(0)), 0)
	case cameopb.Op_DROP, cameopb.Op_POP:
		c.emit(opPop, 0, 0)
	case cameopb.Op_IF:
		test := c.emit(opJumpIfNot, 0, 0)
		c.list(in.Code[0].GetCode())
		end := c.emit(opJump, 0, 0)
		c.code[test].a = int32(len(c.code))
		c.list(in.Code[1].GetCode())
		c.code[end].a = int32(len(c.code))
	case cameopb.Op_WHILE:
		cond := int32(len(c.code))
		c.list(in.Code[0].GetCode())
		test := c.emit(opJumpIfNot, 0, 0)
		c.list(in.Code[1].GetCode())
		c.emit(opPop, 0, 0)
		c.emit(opCheck, 0, 0)
		c.emit(opJump, cond, 0)
		c.code[test].a = int32(len(c.code))
		c.emit(opNil, 0, 0)
	case cameopb.Op_DEF:
		c.vm.methods[name(0)] = int32(in.Args[1].GetProc())
		c.emit(opNil, 0, 0)
	case cameopb.Op_LIST:
		c.emit(opList, int32(in.Args[0].GetInt()), 0)
	case cameopb.Op_MAP:
		c.emit(opMap, int32(in.Args[0].GetInt()), 0)
	case cameopb.Op_MAP_SET:
		c.emit(opMapSet, 0, 0)
	case cameopb.Op_INTERP:
		c.constant(short(nil))
		for _, part := range in.Code {
			c.list(part.GetCode())
			c.emit(opInterpAdd, 0, 0)
		}
	case cameopb.Op_SEND:
		argc := int32(in.Args[1].GetInt())
		if in.Flags&uint64(cameopb.Flag_FLAG_SELF) != 0 {
			if c.unit.Names[name(0)] == "p" {
				c.emit(opSend, sendP, argc)
			} else {
				c.emit(opCall, int32(name(0)), argc)
			}
			return
		}
		switch c.unit.Names[name(0)] {
		case "[]":
			c.emit(opSend, sendIndex, argc)
		case "to_s":
			c.emit(opSend, sendToS, argc)
		case "size":
			c.emit(opSend, sendSize, argc)
		default:
			panic("bumpvm: method not supported: " + c.unit.Names[name(0)])
		}
	default:
		if in.Op >= cameopb.Op_BINARY_PLUS && in.Op <= cameopb.Op_BINARY_NE {
			c.emit(opBinary, int32(in.Op), 0)
			return
		}
		panic("bumpvm: instruction not supported: " + in.Op.String())
	}
}
