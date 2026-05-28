package version

// Version is set at build time via goreleaser ldflags.
// Default "dev" is used for local/go install builds.
var Version = "dev"
