package llvm

import "os"

// noRecursionGuard suppresses the per-call recursion guard.
//
// This exists to measure what the guard costs, not as a supported option: a
// program built with it turns unbounded recursion into a stack overflow crash
// instead of a clean RuntimeError. Set DESI_NO_RECURSION_GUARD=1 when
// benchmarking, and read any number it produces as an upper bound on what
// eliding the guard could ever win.
var noRecursionGuard = os.Getenv("DESI_NO_RECURSION_GUARD") == "1"
