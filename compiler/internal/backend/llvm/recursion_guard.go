package llvm

import (
	"os"

	"github.com/desilang/desi/compiler/internal/term"
)

// noRecursionGuard suppresses the stack guard entirely.
//
// This exists to measure what the guard costs, not as a supported option. A
// program built with it has no protection against stack exhaustion: runaway
// recursion faults instead of raising a RuntimeError, which is exactly the
// memory-safety property the guard is there to provide.
//
// It says so on stderr every time, because a safety guarantee that can be
// switched off quietly is not a guarantee.
var noRecursionGuard = func() bool {
	if os.Getenv("DESI_NO_RECURSION_GUARD") != "1" {
		return false
	}
	term.Eprintln("desic: warning: DESI_NO_RECURSION_GUARD=1 — stack guard omitted.")
	term.Eprintln("desic:          Runaway recursion in this build will fault instead of")
	term.Eprintln("desic:          raising a RuntimeError. For benchmarking only.")
	return true
}()
