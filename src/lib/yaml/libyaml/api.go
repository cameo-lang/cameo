// Package libyaml is libyaml, the C library CRuby's psych is built on, as
// gopkg.in/yaml.v3 ported it to Go: apic.go, emitterc.go, parserc.go,
// readerc.go, scannerc.go, writerc.go, yamlh.go and yamlprivateh.go are
// that port's files, under their MIT license (see LICENSE), with the
// emitter's own rules of libyaml 0.2.5 restored where the port had changed
// them; the changes are marked [cameo]. This file is the surface the yaml
// library uses: a Parser that yields events with libyaml's error marks,
// and an Emitter that takes events and writes what libyaml writes.
//
// Carrying the port rather than depending on the module is what makes
// cameo's YAML Ruby's: Psych is libyaml plus a Ruby visitor, so the same
// parser and emitter under a Go visitor give the same documents, the same
// quoting and folding, and the same error messages and positions.
package libyaml

import (
	"fmt"
)

// EventType is the kind of an event.
type EventType int

// The event types, in the order libyaml numbers them.
const (
	NoEvent EventType = iota
	StreamStart
	StreamEnd
	DocumentStart
	DocumentEnd
	Alias
	Scalar
	SequenceStart
	SequenceEnd
	MappingStart
	MappingEnd
)

// Style is a scalar's or a collection's style. Any lets the emitter
// choose.
type Style int

// The styles. The first six are scalar styles; Block and Flow are for
// sequences and mappings.
const (
	Any Style = iota
	Plain
	SingleQuoted
	DoubleQuoted
	Literal
	Folded
	Block
	Flow
)

// Mark is a position in the input: zero-based line and column, and the
// byte index.
type Mark struct{ Index, Line, Column int }

// Event is one parser or emitter event.
type Event struct {
	Type       EventType
	Start, End Mark
	// Anchor and Tag are empty when the node has none.
	Anchor, Tag string
	// Value is a scalar's text.
	Value string
	// Implicit is a document's implicit start or end, a collection's
	// implicit tag, and a scalar's plain_implicit: the tag may be left out
	// when the scalar is plain.
	Implicit bool
	// QuotedImplicit is a scalar's quoted_implicit: the tag may be left out
	// when the scalar is quoted.
	QuotedImplicit bool
	Style          Style
	// Version is a document's %YAML directive, nil when there is none.
	Version *[2]int
}

// Error is what the parser or the emitter refuses, with libyaml's problem,
// its context when there is one, and their marks.
type Error struct {
	Problem, Context string
	ProblemMark      Mark
	ContextMark      Mark
	Offset           int
}

func (e *Error) Error() string {
	if e.Context == "" {
		return e.Problem
	}
	return e.Problem + " " + e.Context
}

// Parser yields the events of one input.
type Parser struct {
	p yaml_parser_t
}

// NewParser makes a parser of input.
func NewParser(input []byte) *Parser {
	p := &Parser{}
	yaml_parser_initialize(&p.p)
	yaml_parser_set_input_string(&p.p, input)
	return p
}

// Next returns the next event. After StreamEnd it keeps returning
// StreamEnd.
func (p *Parser) Next() (Event, error) {
	if p.p.state == yaml_PARSE_END_STATE {
		return Event{Type: StreamEnd}, nil
	}
	var ev yaml_event_t
	if !yaml_parser_parse(&p.p, &ev) {
		return Event{}, &Error{
			Problem:     p.p.problem,
			Context:     p.p.context,
			ProblemMark: mark(p.p.problem_mark),
			ContextMark: mark(p.p.context_mark),
			Offset:      p.p.problem_offset,
		}
	}
	return fromEvent(&ev), nil
}

func mark(m yaml_mark_t) Mark { return Mark{Index: m.index, Line: m.line, Column: m.column} }

func fromEvent(ev *yaml_event_t) Event {
	e := Event{
		Type:           EventType(ev.typ),
		Start:          mark(ev.start_mark),
		End:            mark(ev.end_mark),
		Anchor:         string(ev.anchor),
		Tag:            string(ev.tag),
		Value:          string(ev.value),
		Implicit:       ev.implicit,
		QuotedImplicit: ev.quoted_implicit,
	}
	switch ev.typ {
	case yaml_SCALAR_EVENT:
		e.Style = scalarStyle(yaml_scalar_style_t(ev.style))
	case yaml_SEQUENCE_START_EVENT:
		if yaml_sequence_style_t(ev.style) == yaml_FLOW_SEQUENCE_STYLE {
			e.Style = Flow
		} else {
			e.Style = Block
		}
	case yaml_MAPPING_START_EVENT:
		if yaml_mapping_style_t(ev.style) == yaml_FLOW_MAPPING_STYLE {
			e.Style = Flow
		} else {
			e.Style = Block
		}
	case yaml_DOCUMENT_START_EVENT:
		if ev.version_directive != nil {
			e.Version = &[2]int{int(ev.version_directive.major), int(ev.version_directive.minor)}
		}
	}
	return e
}

func scalarStyle(s yaml_scalar_style_t) Style {
	switch s {
	case yaml_PLAIN_SCALAR_STYLE:
		return Plain
	case yaml_SINGLE_QUOTED_SCALAR_STYLE:
		return SingleQuoted
	case yaml_DOUBLE_QUOTED_SCALAR_STYLE:
		return DoubleQuoted
	case yaml_LITERAL_SCALAR_STYLE:
		return Literal
	case yaml_FOLDED_SCALAR_STYLE:
		return Folded
	}
	return Any
}

func toScalarStyle(s Style) yaml_scalar_style_t {
	switch s {
	case Plain:
		return yaml_PLAIN_SCALAR_STYLE
	case SingleQuoted:
		return yaml_SINGLE_QUOTED_SCALAR_STYLE
	case DoubleQuoted:
		return yaml_DOUBLE_QUOTED_SCALAR_STYLE
	case Literal:
		return yaml_LITERAL_SCALAR_STYLE
	case Folded:
		return yaml_FOLDED_SCALAR_STYLE
	}
	return yaml_ANY_SCALAR_STYLE
}

// Emitter writes the events it is given as libyaml does.
type Emitter struct {
	e   yaml_emitter_t
	out []byte
}

// NewEmitter makes an emitter writing UTF-8. indent is the indentation, 2
// unless between 2 and 9; width is the line width, 80 when it is not above
// twice the indentation, unlimited when negative. canonical asks for the
// canonical form.
func NewEmitter(indent, width int, canonical bool) *Emitter {
	e := &Emitter{}
	yaml_emitter_initialize(&e.e)
	yaml_emitter_set_output_string(&e.e, &e.out)
	yaml_emitter_set_unicode(&e.e, true)
	yaml_emitter_set_indent(&e.e, indent)
	yaml_emitter_set_width(&e.e, width)
	yaml_emitter_set_canonical(&e.e, canonical)
	return e
}

// Emit takes one event.
func (e *Emitter) Emit(ev Event) error {
	var y yaml_event_t
	switch ev.Type {
	case StreamStart:
		yaml_stream_start_event_initialize(&y, yaml_UTF8_ENCODING)
	case StreamEnd:
		yaml_stream_end_event_initialize(&y)
	case DocumentStart:
		var version *yaml_version_directive_t
		if ev.Version != nil {
			version = &yaml_version_directive_t{major: int8(ev.Version[0]), minor: int8(ev.Version[1])}
		}
		yaml_document_start_event_initialize(&y, version, nil, ev.Implicit)
	case DocumentEnd:
		yaml_document_end_event_initialize(&y, ev.Implicit)
	case Alias:
		yaml_alias_event_initialize(&y, []byte(ev.Anchor))
	case Scalar:
		yaml_scalar_event_initialize(&y, nullable(ev.Anchor), nullable(ev.Tag), []byte(ev.Value), ev.Implicit, ev.QuotedImplicit, toScalarStyle(ev.Style))
	case SequenceStart:
		style := yaml_BLOCK_SEQUENCE_STYLE
		if ev.Style == Flow {
			style = yaml_FLOW_SEQUENCE_STYLE
		}
		yaml_sequence_start_event_initialize(&y, nullable(ev.Anchor), nullable(ev.Tag), ev.Implicit, style)
	case SequenceEnd:
		yaml_sequence_end_event_initialize(&y)
	case MappingStart:
		style := yaml_BLOCK_MAPPING_STYLE
		if ev.Style == Flow {
			style = yaml_FLOW_MAPPING_STYLE
		}
		yaml_mapping_start_event_initialize(&y, nullable(ev.Anchor), nullable(ev.Tag), ev.Implicit, style)
	case MappingEnd:
		yaml_mapping_end_event_initialize(&y)
	default:
		return fmt.Errorf("libyaml: unknown event %d", ev.Type)
	}
	if !yaml_emitter_emit(&e.e, &y) {
		return &Error{Problem: e.e.problem}
	}
	return nil
}

// nullable is s as a slice, nil when empty: libyaml tells an absent anchor or
// tag by a null pointer.
func nullable(s string) []byte {
	if s == "" {
		return nil
	}
	return []byte(s)
}

// String is what has been written so far.
func (e *Emitter) String() string { return string(e.out) }
