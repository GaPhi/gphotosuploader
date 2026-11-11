// Package version gives the version of the binary, read from the build information recorded by the Go toolchain:
// the module version for "go install github.com/GaPhi/gphotosuploader@<version>" (a pseudo-version such as
// v0.0.0-20261111165802-c3ad335b83b8 contains the commit date and hash), the VCS information for a build in a clone.
package version

import (
	"runtime/debug"
	"time"
)

var (
	// Commit hash (short) or module version
	Hash = "unknown"
	// Commit date
	Date = "unknown"
	// Module version, as recorded by the Go toolchain ("(devel)" or "" if unknown)
	Module = ""
)

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	Module = info.Main.Version
	if Module != "" && Module != "(devel)" {
		Hash = Module
	}
	modified := false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			Hash = s.Value
			if len(Hash) > 12 {
				Hash = Hash[:12]
			}
		case "vcs.time":
			if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
				Date = t.Local().Format(time.DateTime)
			}
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if modified {
		Hash += "-dirty"
	}
	if Date == "unknown" {
		Date = pseudoVersionDate(Module)
	}
}

// Commit date of a pseudo-version (vX.Y.Z-yyyymmddhhmmss-abcdefabcdef), "unknown" otherwise
func pseudoVersionDate(v string) string {
	for i := len(v) - 1; i >= 0; i-- {
		if v[i] == '-' && i >= 15 && v[i-15] == '-' {
			if t, err := time.Parse("20060102150405", v[i-14:i]); err == nil {
				return t.Local().Format(time.DateTime)
			}
		}
	}
	return "unknown"
}
