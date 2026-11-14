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
		"RUBY_COPYRIGHT":      object.NewString("cameo - Copyright (C) 2026 the cameo authors (MIT License)", object.ASCII),
		"RUBY_DESCRIPTION":    object.NewString(info.Description(), object.UTF8),
		"RUBY_ENGINE":         object.NewString("cameo", object.ASCII),
		"RUBY_ENGINE_VERSION": object.NewString(info.Version, object.UTF8),
		"RUBY_PATCHLEVEL":     object.Int(0),
		"RUBY_PLATFORM":       object.NewString(info.Platform, object.UTF8),
		"RUBY_RELEASE_DATE":   object.NewString(info.Date, object.UTF8),
		"RUBY_REVISION":       object.NewString(info.Revision, object.UTF8),
		"RUBY_VERSION":        object.NewString(buildinfo.RubyVersion, object.UTF8),
	} {
		object.ObjectClass.Consts[name] = v
	}
}
