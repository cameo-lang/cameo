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
)

// Error is one thing wrong with a script: a syntax error from the parser, or
// a construct amber leaves out.
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
	tree, err := parser.Parse(src)
	if err != nil {
		e := &Error{File: file, Msg: err.Error()}
		if m := parserLine.FindStringSubmatch(err.Error()); m != nil {
			e.Line, _ = strconv.Atoi(m[1])
		}
		return nil, ErrorList{e}
	}
	c := &converter{file: file, lines: tree.Lines, stack: []frame{{kind: ctxTop}}}
	prog := &Program{Body: c.body(tree.Body)}
	if len(c.errs) > 0 {
		return nil, c.errs
	}
	return prog, nil
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
