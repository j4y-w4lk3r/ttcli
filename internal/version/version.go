// Package version reports ttcli build metadata.
//
// Release builds (goreleaser, make install) stamp version/commit/date via
// -ldflags. Plain `go build` falls back to module + VCS info embedded by the
// Go toolchain (-buildvcs, on by default).
package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// Link-time vars; overridden by Makefile or goreleaser (-X main.version etc.).
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a human-readable version line for `ttcli version`.
func String() string {
	v, c, d := Version, Commit, Date
	if info, ok := debug.ReadBuildInfo(); ok {
		if v == "dev" {
			if mv := moduleVersion(info); mv != "" {
				v = mv
			}
		}
		if c == "none" {
			if rev := vcsSetting(info, "vcs.revision"); rev != "" {
				c = shortRev(rev)
			}
		}
		if d == "unknown" {
			d = vcsSetting(info, "vcs.time")
		}
	}
	return fmt.Sprintf("ttcli %s (commit %s, built %s)", v, c, d)
}

func moduleVersion(info *debug.BuildInfo) string {
	mv := info.Main.Version
	if mv == "" || mv == "(devel)" {
		return ""
	}
	// go install @v0.1.0 → "v0.1.0"; pseudo-version → keep as-is.
	return strings.TrimPrefix(mv, "v")
}

func vcsSetting(info *debug.BuildInfo, key string) string {
	for _, s := range info.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}

func shortRev(rev string) string {
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}
