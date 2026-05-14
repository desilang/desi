package version

// Version is the current compiler version.
// Overridden at build time via:
//
//	go build -ldflags="-X github.com/desilang/desi/compiler/internal/version.Version=v0.1.0"
var Version = "0.1.0"

func String() string { return Version }
