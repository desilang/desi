package token

import "testing"

func TestBuiltinNumericTypes(t *testing.T) {
	ints := []string{
		"int",
		"isize", "usize",
		"i8", "i16", "i32", "i64", "i128",
		"u8", "u16", "u32", "u64", "u128",
	}
	floats := []string{"f32", "f64"}

	for _, s := range ints {
		if !IsBuiltinType(s) {
			t.Fatalf("%q should be recognized as a builtin integer type", s)
		}
	}
	for _, s := range floats {
		if !IsBuiltinType(s) {
			t.Fatalf("%q should be recognized as a builtin float type", s)
		}
	}

	// Negative samples: not builtin numeric spellings.
	bad := []string{"i7", "u129", "f16", "fx", "zz"}
	for _, s := range bad {
		if IsBuiltinType(s) {
			t.Fatalf("%q must NOT be recognized as a builtin type", s)
		}
	}
}
