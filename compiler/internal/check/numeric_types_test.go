package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/types"
)

// Ensures name->type lookup and surfaceToType cooperate for the new numerics.
func TestNumericTypeNamesAndSurfaceMapping(t *testing.T) {
	lookup := []string{
		// ints
		"i8", "i16", "i32", "i64", "i128",
		"u8", "u16", "u32", "u64", "u128",
		// ptr-sized
		"isize", "usize",
		// floats (float is alias of f64)
		"f32", "f64", "float",
	}
	for _, s := range lookup {
		if tt, ok := types.FromName(s); !ok || tt == nil {
			t.Fatalf("types.FromName(%q) should resolve, got ok=%v type=%v", s, ok, tt)
		}
	}

	// surfaceToType should leverage types.FromName and round-trip basic spellings.
	surface := []string{"u8", "i16", "f32", "f64", "float", "usize", "isize"}
	for _, s := range surface {
		if tt := surfaceToType(s); tt == nil {
			t.Fatalf("surfaceToType(%q) should return a non-nil type", s)
		}
	}
}
