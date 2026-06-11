package main

import "runtime/debug"

// version is the bast CLI version.
// Set to a real tag by -ldflags "-X main.version=vX.Y.Z" at release build time.
// Falls back to the module version embedded automatically by go install.
var version = "dev"

// resolveVersion returns the best available version string.
// Priority: ldflags injection → go install embedded version → "dev".
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}
