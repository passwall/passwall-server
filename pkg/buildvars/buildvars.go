package buildvars

// Build variables. Version, CommitID, BuildID and BuildTime are set at build time via ldflags.
var (
	// Version is set from git tag or VERSION file when releasing.
	Version = "dev"
	// CommitID is the full git commit hash.
	CommitID = "unknown"
	// BuildID is the short commit hash or CI build identifier.
	BuildID = "unknown"
	// BuildTime is the UTC build timestamp.
	BuildTime = "unknown"
)
