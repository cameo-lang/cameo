package eval

import (
	"errors"
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/cameo-lang/cameo/src/cameopb"
	"github.com/cameo-lang/cameo/src/ir"
	"github.com/cameo-lang/cameo/src/lib"
	"github.com/cameo-lang/cameo/src/syntax"
)

// ErrOtherCode is what a program limited by OnlyCode returns, wrapped,
// for a unit of code it does not allow.
var ErrOtherCode = errors.New("the code is not the script's or a library's")

// onlyCode is the code a program takes from a dump or a message when
// OnlyCode limits it: the unit compiled from the script's source, and the
// unit of each library written in cameo.
type onlyCode struct {
	file, source string
	mu           sync.Mutex
	// units holds the units the script's file and each library's file
	// may have, by file, once a unit of the file was checked: the
	// script's, the library's, or both when the script has a library's
	// file name.
	units map[string][]*cameopb.Unit
}

// OnlyCode limits the units of code the program takes from a dump, and
// from a closure in any message it decodes, to the one compiled from
// source as file and those of the libraries of lib.Sources, each as
// require compiles it. A unit the program compiles itself is not checked.
func (in *Interp) OnlyCode(file, source string) {
	in.code.only = &onlyCode{file: file, source: source, units: map[string][]*cameopb.Unit{}}
}

// allow returns nil when the program may take unit, and an error wrapping
// ErrOtherCode when OnlyCode does not allow it. A unit is compared whole,
// not by its digest, which is too short to tell code apart that was made
// to collide.
func (in *Interp) allow(unit *cameopb.Unit, digest string) error {
	only := in.code.only
	if only == nil {
		return nil
	}
	for _, want := range only.unitsOf(unit.GetFile()) {
		if proto.Equal(unit, want) {
			return nil
		}
	}
	return fmt.Errorf("%w: unit %s of %s", ErrOtherCode, digest, unit.GetFile())
}

// unitsOf returns the units of the script and of the library file names,
// compiled; none for any other file.
func (o *onlyCode) unitsOf(file string) []*cameopb.Unit {
	o.mu.Lock()
	defer o.mu.Unlock()
	if units, ok := o.units[file]; ok {
		return units
	}
	var units []*cameopb.Unit
	compile := func(source string) {
		prog, err := syntax.Parse(file, source)
		if err != nil {
			return
		}
		if unit, err := ir.Compile(file, prog); err == nil {
			units = append(units, unit)
		}
	}
	known := file == o.file
	if known {
		compile(o.source)
	}
	for name, source := range lib.Sources {
		if libraryFile(name) == file {
			known = true
			compile(source)
		}
	}
	// A file that is neither is not kept: its name is the dump's or the
	// message's, and each would add an entry.
	if known {
		o.units[file] = units
	}
	return units
}
