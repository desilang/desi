package llvm_test

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
	"github.com/desilang/desi/compiler/internal/types"
)

func must(name string) types.T {
	t, ok := types.FromName(name)
	if !ok {
		return nil
	}
	return t
}

func TestLowerPrimType_SizedNumerics(t *testing.T) {
	cases := map[string]string{
		"i8": "i8", "u8": "i8",
		"i16": "i16", "u16": "i16",
		"i32": "i32", "u32": "i32",
		"i64": "i64", "u64": "i64",
		"i128": "i128", "u128": "i128",
		"f32":   "float",
		"f64":   "double",
		"float": "double", // alias
	}
	for src, want := range cases {
		got := llvm.LowerPrimType(must(src))
		if got != want {
			t.Fatalf("%s -> %s, want %s", src, got, want)
		}
	}
}

func TestLowerPrimType_ExistingStillMapped(t *testing.T) {
	// sanity against regressions of prior mappings
	if got := llvm.LowerPrimType(types.Int); got != "i32" {
		t.Fatalf("int -> %s, want i32", got)
	}
	if got := llvm.LowerPrimType(must("usize")); got != "i64" {
		t.Fatalf("usize -> %s, want i64", got)
	}
	if got := llvm.LowerPrimType(types.Str); got != "ptr" {
		t.Fatalf("str -> %s, want ptr", got)
	}
}
