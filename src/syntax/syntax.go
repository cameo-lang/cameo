package syntax

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-ruby-parser/parser"
	"github.com/go-ruby-parser/parser/ast"
	"github.com/go-ruby-parser/parser/lexer"
	"github.com/go-ruby-parser/parser/token"
)

// Error is one thing wrong with a script: a syntax error from the parser, or
// a construct cameo leaves out.
type Error struct {
	File string
	Line int
	Msg  string
}

func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg)
	}
	return fmt.Sprintf("%s: %s", e.File, e.Msg)
}

// ErrorList is every error found in a script, in source order.
type ErrorList []*Error

func (l ErrorList) Error() string {
	lines := make([]string, len(l))
	for i, e := range l {
		lines[i] = e.Error()
	}
	return strings.Join(lines, "\n")
}

// parserLine finds the line number the parser puts into its error messages.
var parserLine = regexp.MustCompile(`line (\d+)`)

// Parse parses and checks a script. file names it in error messages. The
// error, when not nil, is an ErrorList.
func Parse(file, src string) (*Program, error) {
	src, errs := binaryLiterals(file, src)
	if len(errs) > 0 {
		return nil, errs
	}
	tree, err := parser.Parse(src)
	if err != nil {
		// The parser names what it did not expect, and a binary string
		// literal is a call of bytesCall to it.
		e := &Error{File: file, Msg: strings.ReplaceAll(err.Error(), bytesCall, "b")}
		if m := parserLine.FindStringSubmatch(err.Error()); m != nil {
			e.Line, _ = strconv.Atoi(m[1])
		}
		return nil, ErrorList{e}
	}
	c := &converter{file: file, lines: tree.Lines, stack: []frame{{kind: ctxTop}}, bangs: bangMethods(tree.Lines)}
	prog := &Program{Body: c.body(tree.Body)}
	if len(c.errs) > 0 {
		return nil, c.errs
	}
	return prog, nil
}

// bytesCall is the name a binary string literal is a call of when the
// parser sees it. The parser reads b"..." as Ruby does, as a call of b
// with a string, and so does not tell it from `b "..."`; the literal is
// rewritten to a call of this name, which the converter makes a Bytes of.
const bytesCall = "__cameo_bytes__"

// binaryLiterals rewrites every binary string literal of src, b"..." or
// b'...', to a call of bytesCall with the string as its argument. A
// literal is the name b with a quote right behind it and no receiver
// before it: `b "x"` and `s.b"x"` are calls. The lexer finds the names,
// so a b" inside a string, a comment or a heredoc is none. Lines stay
// the lines they were.
func binaryLiterals(file, src string) (string, ErrorList) {
	if !strings.Contains(src, `b"`) && !strings.Contains(src, `b'`) {
		return src, nil
	}
	lineStart := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			lineStart = append(lineStart, i+1)
		}
	}
	var out strings.Builder
	var errs ErrorList
	toks := lexer.New(src).Tokenize()
	done := 0
	for i := 0; i+1 < len(toks); i++ {
		name, str := toks[i], toks[i+1]
		if name.Type != token.IDENT || name.Lit != "b" || str.SpaceBefore || (str.Type != token.STRING && str.Type != token.STRBEG) {
			continue
		}
		if i > 0 {
			switch toks[i-1].Type {
			case token.DOT, token.SAFEDOT, token.SCOPE, token.DEF:
				continue
			}
		}
		if name.Line < 1 || name.Line > len(lineStart) {
			continue
		}
		at := lineStart[name.Line-1] + name.Col - 1
		if at < done || at+1 >= len(src) || src[at] != 'b' || (src[at+1] != '"' && src[at+1] != '\'') {
			continue
		}
		if str.Type == token.STRBEG {
			errs = append(errs, &Error{File: file, Line: name.Line, Msg: `a binary string literal, b"...", takes no interpolation`})
			continue
		}
		end := closingQuote(src, at+1)
		if end < 0 {
			continue
		}
		out.WriteString(src[done:at])
		out.WriteString(bytesCall)
		out.WriteByte('(')
		out.WriteString(src[at+1 : end+1])
		out.WriteByte(')')
		done = end + 1
	}
	if done == 0 {
		return src, errs
	}
	out.WriteString(src[done:])
	return out.String(), errs
}

// closingQuote returns the index of the quote that ends the string
// literal opening at src[open], or -1 when the source ends first.
func closingQuote(src string, open int) int {
	for i := open + 1; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case src[open]:
			return i
		}
	}
	return -1
}

// bangMethods returns the names of the bang methods a script defines. The
// parser records a line for every node, so its keys are the whole tree.
func bangMethods(lines map[ast.Node]int) map[string]bool {
	out := map[string]bool{}
	for n := range lines {
		if d, ok := n.(*ast.MethodDef); ok && strings.HasSuffix(d.Name, "!") && d.Name != "!" {
			out[d.Name] = true
		}
	}
	return out
}

// digest identifies a class or module definition by its normalised body: the
// AST with every line number left out, hashed. Two definitions with equal
// digests are one definition.
// Digest is a node's digest, as a class's: what identifies a class built
// at run time from a block, by Class.new.
func Digest(n Node) string { return digest(n) }

func digest(n Node) string {
	var b strings.Builder
	writeNode(&b, reflect.ValueOf(n))
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:8])
}

var posType = reflect.TypeOf(Pos{})

func writeNode(b *strings.Builder, v reflect.Value) {
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		if v.IsNil() {
			b.WriteString("nil")
			return
		}
		writeNode(b, v.Elem())
	case reflect.Struct:
		b.WriteString(v.Type().Name())
		b.WriteByte('(')
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if f.Type == posType || f.Name == "Digest" {
				continue
			}
			b.WriteString(f.Name)
			b.WriteByte('=')
			writeNode(b, v.Field(i))
			b.WriteByte(' ')
		}
		b.WriteByte(')')
	case reflect.Slice:
		b.WriteByte('[')
		for i := 0; i < v.Len(); i++ {
			writeNode(b, v.Index(i))
			b.WriteByte(' ')
		}
		b.WriteByte(']')
	case reflect.String:
		b.WriteString(strconv.Quote(v.String()))
	default:
		fmt.Fprintf(b, "%v", v.Interface())
	}
}

// keep the parser's ast package referenced for the converter's type switches.
var _ ast.Node
