package stdlib

import (
	"github.com/cameo-lang/cameo/src/buildinfo"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/str"
)

// installVersion defines Ruby's version constants, describing cameo and the
// Ruby it follows. doc/cameo-vs-ruby.md lists their values.
func installVersion() {
	info := buildinfo.Read()
	for name, v := range map[string]object.Value{
		"RUBY_COPYRIGHT":      object.NewString("cameo - Copyright (C) 2026 the cameo authors (MIT License)", str.ASCII),
		"RUBY_DESCRIPTION":    object.NewString(info.Description(), str.UTF8),
		"RUBY_ENGINE":         object.NewString("cameo", str.ASCII),
		"RUBY_ENGINE_VERSION": object.NewString(info.Version, str.UTF8),
		"RUBY_PATCHLEVEL":     object.Int(0),
		"RUBY_PLATFORM":       object.NewString(info.Platform, str.UTF8),
		"RUBY_RELEASE_DATE":   object.NewString(info.Date, str.UTF8),
		"RUBY_REVISION":       object.NewString(info.Revision, str.UTF8),
		"RUBY_VERSION":        object.NewString(buildinfo.RubyVersion, str.UTF8),
	} {
		object.ObjectClass.Consts[name] = v
	}
}
