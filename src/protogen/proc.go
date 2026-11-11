package protogen

import (
	"github.com/cameo-lang/cameo/src/object"
)

// A proc field holds a block. In the class's message it is a field of
// type cameo.Closure: the proc of a unit it runs and the binding it runs
// in (value.proto). So a class with a proc field is a message like any
// other, and its instances cross the envelope, async and a dump with
// their procs.

// ProcField returns the name of the first proc field of cls, or of a
// class a field of cls refers to, and the class that declares it; "" and
// nil when the class holds no proc.
func ProcField(cls *object.Class) (field string, owner *object.Class) {
	return procField(cls, map[*object.Class]bool{})
}

// HasProc reports whether an instance of cls may hold a proc, in a field
// of its own or of a class a field refers to. Such an instance does not
// keep its message: the closure in it is made for one reader.
func HasProc(cls *object.Class) bool {
	field, _ := ProcField(cls)
	return field != ""
}

// holdsProc reports whether a value of the field type t may hold a proc.
func holdsProc(t *object.FieldType) bool {
	name, _ := procIn(t, map[*object.Class]bool{})
	return name != ""
}

// ownProc reports whether cls declares a proc field itself, also as the
// element of a list, a set or a map: its message refers to
// cameo.Closure.
func ownProc(cls *object.Class) bool {
	for _, f := range cls.Fields {
		for t := f.Type; t != nil; t = t.Elem {
			if t.Kind == "proc" {
				return true
			}
		}
	}
	return false
}

func procField(cls *object.Class, seen map[*object.Class]bool) (string, *object.Class) {
	if seen[cls] {
		return "", nil
	}
	seen[cls] = true
	for _, f := range cls.Fields {
		if name, owner := procIn(f.Type, seen); name != "" {
			if owner == nil {
				return f.Name, cls
			}
			return name, owner
		}
	}
	return "", nil
}

// procIn returns the proc a field type holds: "proc" with a nil owner
// when the type itself is one, for the caller to fill in, or the field and
// the class of a proc in a class the type refers to.
func procIn(t *object.FieldType, seen map[*object.Class]bool) (string, *object.Class) {
	if t == nil {
		return "", nil
	}
	switch t.Kind {
	case "proc":
		return "proc", nil
	case "list", "set", "map":
		return procIn(t.Elem, seen)
	case "ref":
		if t.Ref != nil {
			return procField(t.Ref, seen)
		}
	}
	return "", nil
}
