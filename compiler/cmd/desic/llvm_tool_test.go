package main

import (
	"os"
	"path/filepath"
	"testing"
)

// findLLVMTool falls back to the bare tool name when it locates nothing, so
// the caller cannot tell "found it" from "gave up" by the return value alone.
// llvmToolFound is what makes that distinction, and it decides whether the
// unoptimized build path uses llc or falls back to clang.
//
// Getting this wrong is not hypothetical: llc is absent on GitHub's macOS
// runners — the Homebrew LLVM formula is keg-only, so clang is on PATH and
// llc is not — and `desic build` failed there with "executable file not found
// in $PATH" until the fallback existed.
func TestLLVMToolFound_MissingToolIsNotFound(t *testing.T) {
	// The bare-name fallback for a tool that cannot exist on PATH.
	if llvmToolFound("desi-no-such-llvm-tool-xyz") {
		t.Fatal("a tool that is not on PATH must not be reported as found")
	}
}

func TestLLVMToolFound_EmptyIsNotFound(t *testing.T) {
	if llvmToolFound("") {
		t.Fatal("empty tool path must not be reported as found")
	}
}

func TestLLVMToolFound_AbsolutePathMustExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-here")
	if llvmToolFound(missing) {
		t.Fatal("absolute path to a missing file must not be reported as found")
	}

	// An absolute path that does exist is found. Use this test binary, which
	// is guaranteed to be present and executable.
	self, err := os.Executable()
	if err != nil {
		t.Skip("cannot resolve test binary:", err)
	}
	if !llvmToolFound(self) {
		t.Fatalf("absolute path to an existing file must be found: %s", self)
	}
}
