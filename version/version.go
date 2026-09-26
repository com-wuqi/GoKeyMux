package version

import (
	"fmt"
	"runtime/debug"
)

// These variables are injected at build time via
//
//	-ldflags "-X GoKeyMux/version.Version=... -X GoKeyMux/version.Commit=... -X GoKeyMux/version.BuildTime=..."
//
// When they are not injected, Info falls back to the VCS stamp that the Go
// toolchain embeds into binaries built inside a git repository.
var (
	// Version is the release version, e.g. a git tag/describe such as
	// "v0.1.0" or "v0.1.0-2-g6262409".
	Version = "dev"
	// Commit is the short git commit hash.
	Commit = "unknown"
	// BuildTime is the UTC build timestamp (RFC 3339).
	BuildTime = "unknown"
)

// Info returns the effective version, commit and build time. When Commit or
// BuildTime were not injected via -ldflags, they fall back to the VCS revision
// and commit time embedded by "go build".
func Info() (version, commit, buildTime string) {
	version, commit, buildTime = Version, Commit, BuildTime
	if commit != "unknown" && buildTime != "unknown" {
		return version, commit, buildTime
	}

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return version, commit, buildTime
	}

	var revision, vcsTime string
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			vcsTime = s.Value
		}
	}
	if commit == "unknown" && revision != "" {
		commit = shortHash(revision)
	}
	if buildTime == "unknown" && vcsTime != "" {
		buildTime = vcsTime
	}
	return version, commit, buildTime
}

// String returns a single-line human-readable summary of the build info.
func String() string {
	v, c, b := Info()
	return fmt.Sprintf("%s (commit=%s built=%s)", v, c, b)
}

func shortHash(revision string) string {
	if len(revision) > 7 {
		return revision[:7]
	}
	return revision
}
