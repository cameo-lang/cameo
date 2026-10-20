package stdlib

import (
	"github.com/cameo-lang/cameo/src/buildinfo"
	"github.com/cameo-lang/cameo/src/object"
)

// installVersion defines Ruby's version constants, describing cameo and the
// Ruby it follows. doc/cameo-vs-ruby.md lists their values.
func installVersion() {
	info := buildinfo.Read()
	for name, v := range map[string]object.Value{
		"RUBY_COPYRIGHT":      object.String("cameo - Copyright (C) 2026 the cameo authors (MIT License)"),
		"RUBY_DESCRIPTION":    object.String(info.Description()),
		"RUBY_ENGINE":         object.String("cameo"),
		"RUBY_ENGINE_VERSION": object.String(info.Version),
		"RUBY_PATCHLEVEL":     object.Int(0),
		"RUBY_PLATFORM":       object.String(info.Platform),
		"RUBY_RELEASE_DATE":   object.String(info.Date),
		"RUBY_REVISION":       object.String(info.Revision),
		"RUBY_VERSION":        object.String(buildinfo.RubyVersion),
	} {
		object.ObjectClass.Consts[name] = v
	}
}
