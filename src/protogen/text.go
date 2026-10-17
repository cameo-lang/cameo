package protogen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/radiospiel/amber/src/object"
)

// Text is the .proto text of one class: its message, preceded by its
// package line when the class lives in a module.
func Text(cls *object.Class) string {
	var b strings.Builder
	if pkg := Package(cls); pkg != "amber" {
		fmt.Fprintf(&b, "package %s;\n\n", pkg)
	}
	writeMessage(&b, cls)
	return b.String()
}

// File is a whole .proto file for one package: every class of that package
// among classes, with imports for the packages their fields refer to.
func File(pkg string, classes []*object.Class) string {
	var b strings.Builder
	b.WriteString("syntax = \"proto3\";\n\n")
	fmt.Fprintf(&b, "package %s;\n", pkg)
	imports := map[string]bool{}
	for _, cls := range classes {
		for _, dep := range refs(cls) {
			if p := Package(dep); p != pkg {
				imports[strings.ReplaceAll(p, ".", "/")+".proto"] = true
			}
		}
	}
	if len(imports) > 0 {
		b.WriteString("\n")
		names := make([]string, 0, len(imports))
		for n := range imports {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Fprintf(&b, "import \"%s\";\n", n)
		}
	}
	for _, cls := range classes {
		b.WriteString("\n")
		writeMessage(&b, cls)
	}
	return b.String()
}

// Files groups classes by package and renders one file per package, keyed
// by file name (amber.proto, geo.proto, geo/shapes.proto).
func Files(classes []*object.Class) map[string]string {
	byPkg := map[string][]*object.Class{}
	for _, cls := range classes {
		pkg := Package(cls)
		byPkg[pkg] = append(byPkg[pkg], cls)
	}
	out := map[string]string{}
	for pkg, list := range byPkg {
		out[strings.ReplaceAll(pkg, ".", "/")+".proto"] = File(pkg, list)
	}
	return out
}

func writeMessage(b *strings.Builder, cls *object.Class) {
	fmt.Fprintf(b, "message %s {\n", cls.Name)
	for _, f := range cls.Fields {
		if f.Type.Kind != "enum" {
			continue
		}
		fmt.Fprintf(b, "  enum %s {\n", camel(f.Name))
		prefix := strings.ToUpper(f.Name) + "_"
		fmt.Fprintf(b, "    %sUNSPECIFIED = 0;\n", prefix)
		for i, v := range f.Type.Enum {
			fmt.Fprintf(b, "    %s%s = %d;\n", prefix, strings.ToUpper(v), i+1)
		}
		b.WriteString("  }\n")
	}
	for _, n := range cls.ReservedNumbers {
		fmt.Fprintf(b, "  reserved %d;\n", n)
	}
	for _, n := range cls.ReservedNames {
		fmt.Fprintf(b, "  reserved \"%s\";\n", n)
	}
	for _, f := range cls.Fields {
		switch f.Type.Kind {
		case "list", "set":
			fmt.Fprintf(b, "  repeated %s %s = %d;\n", typeName(f.Type.Elem, cls), f.Name, f.Number)
		case "map":
			fmt.Fprintf(b, "  map<%s, %s> %s = %d;\n", scalarName(f.Type.Key), typeName(f.Type.Elem, cls), f.Name, f.Number)
		case "enum":
			fmt.Fprintf(b, "  optional %s %s = %d;\n", camel(f.Name), f.Name, f.Number)
		default:
			fmt.Fprintf(b, "  optional %s %s = %d;\n", typeName(f.Type, cls), f.Name, f.Number)
		}
	}
	b.WriteString("}\n")
}

// typeName is a field type as written in .proto; a class in another
// package is qualified.
func typeName(t *object.FieldType, from *object.Class) string {
	if t.Kind == "ref" {
		if Package(t.Ref) != Package(from) {
			return Package(t.Ref) + "." + t.Ref.Name
		}
		return t.Ref.Name
	}
	return scalarName(t.Kind)
}

// scalarName is a scalar type's name in .proto.
func scalarName(kind string) string { return object.Scalar(kind).Proto }
