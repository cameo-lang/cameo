package eval

import "github.com/cameo-lang/cameo/src/object"

// The errors a script meets when it runs into something cameo leaves out on
// purpose end, as the checker's refusals do, with the link to the page of
// the code that explains it (codes.Link).

// leftOutConstants are the top-level constants cameo leaves out, by the
// code of the reason. A NameError for one links its page.
var leftOutConstants = map[string]string{
	"Thread": "thr", "ThreadGroup": "thr", "Mutex": "thr", "Queue": "thr", "SizedQueue": "thr",
	"ConditionVariable": "thr", "Fiber": "thr", "Ractor": "thr", "Monitor": "thr",
	"IO": "io", "Dir": "io", "ARGF": "io", "STDIN": "io", "FileTest": "io", "Socket": "io",
	"Process": "proc", "Signal": "proc", "ENV": "proc", "GC": "proc", "ObjectSpace": "proc",
	"RubyVM": "proc", "TracePoint": "proc", "Binding": "proc",
	"Rational": "rat", "Complex": "rat",
	"Struct":              "struct",
	"Data":                "data",
	"Encoding::Converter": "enc",
}

// leftOutMethods are the methods cameo leaves out on every receiver, by the
// code of the reason. A NoMethodError for one links its page.
var leftOutMethods = map[string]string{
	"equal?": "id", "object_id": "id", "__id__": "id",
	"ruby2_keywords": "r2k",
	"Rational":       "rat", "Complex": "rat", "to_r": "rat", "to_c": "rat", "rationalize": "rat",
	"system": "proc", "spawn": "proc", "exec": "proc", "trap": "proc", "fork": "proc", "`": "proc",
	"instance_method": "meta", "singleton_class": "meta", "define_singleton_method": "meta",
	"method": "meta", "public_method": "meta", "singleton_methods": "meta", "instance_exec": "meta",
	"class_exec": "meta", "module_eval": "meta", "remove_method": "meta", "undef_method": "meta",
	"const_set": "meta", "remove_const": "meta",
	"instance_variables": "ivar", "instance_variable_defined?": "ivar",
	"private_methods": "vis", "protected_methods": "vis", "public_methods": "vis",
	"private_instance_methods": "vis", "protected_instance_methods": "vis", "public_instance_methods": "vis",
}

// conversions are the methods of leftOutMethods that a script's own class
// may define for itself.
var conversions = map[string]bool{"to_r": true, "to_c": true, "rationalize": true}

// enumeratorMethods are the methods of an external enumerator, which a
// cameo Enumerator does not have.
var enumeratorMethods = map[string]bool{"next": true, "peek": true, "rewind": true, "next_values": true, "peek_values": true, "feed": true}

// constantCode is the code of a constant cameo leaves out, shown as the
// NameError shows it, or "".
func constantCode(shown string) string { return leftOutConstants[shown] }

// methodCode is the code of a method cameo leaves out of recv, or "".
func methodCode(recv object.Value, name string) string {
	if enumeratorMethods[name] {
		for k := recv.Class(); k != nil; k = k.Super {
			if k == object.EnumeratorClass {
				return "next"
			}
		}
	}
	// to_r and to_c are left out of cameo's numbers and strings; a class
	// of the script's own that lacks one lacks its own method.
	if conversions[name] && recv.Class() != nil && recv.Class().Digest != "" {
		return ""
	}
	return leftOutMethods[name]
}
