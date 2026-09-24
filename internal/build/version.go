// Package build holds the version string injected via ldflags.
package build

// Version is set at build time:
// go build -ldflags "-X lazypass/internal/build.Version=v1.0.0"
var Version = "dev"
