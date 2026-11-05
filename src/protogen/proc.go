package protogen

import (
	"fmt"

	"github.com/cameo-lang/cameo/src/object"
)

// ProcField returns the name of the first proc field of cls, or of a
// class a field of cls refers to, and the class that declares it; "" and
// nil when every field is a message's. A block cannot be serialised, so
// such a class is no message: it has no .proto, and its instances cannot
// cross to a host, to async or into a dump.
func ProcField(cls *object.Class) (field string, owner *object.Class) {
	return procField(cls, map[*object.Class]bool{})
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

// NoMessage is the error for a class with a proc field, or nil when cls
// is a message. It is a NotSerialisable, the one error for everything
// the envelope cannot carry, so a host, async and the dump report it as
// they report a Proc; its text says which field is the proc.
func NoMessage(cls *object.Class) error {
	field, owner := ProcField(cls)
	if field == "" {
		return nil
	}
	via := ""
	if owner != cls {
		via = fmt.Sprintf(" through %s", owner.QualifiedName())
	}
	return &NotSerialisable{
		Kind:  article(cls.QualifiedName()) + " with a proc field",
		Class: cls.QualifiedName(),
		Why:   fmt.Sprintf("%s is no message: its field %s%s is a proc, which cannot be serialised, so it has no .proto and its instances cannot cross to a host, to async or into a dump", cls.QualifiedName(), field, via),
	}
}
