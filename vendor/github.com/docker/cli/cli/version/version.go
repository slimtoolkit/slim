package version

// Default build-time variable.
// These values are overridden via ldflags
var (
	PlatformName = ""
	Version      = "unknown-version"
	GitCommit    = "unknown-commit"
	BuildTime    = "unknown-buildtime"
)
