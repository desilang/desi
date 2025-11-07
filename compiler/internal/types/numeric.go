package types

// Sized integers and floats (Phase: M10-Pre).
// We intentionally keep Kind grouping coarse for Tier-0:
// - All integers (signed/unsigned/sized) => IntKind
// - All floats (f32/f64/float) => FloatKind
// This preserves existing Equal() behavior while we layer stricter rules in the checker.

var (
	// Signed
	I8   = &basic{kind: IntKind, name: "i8"}
	I16  = &basic{kind: IntKind, name: "i16"}
	I32  = &basic{kind: IntKind, name: "i32"}
	I64  = &basic{kind: IntKind, name: "i64"}
	I128 = &basic{kind: IntKind, name: "i128"}

	// Unsigned
	U8   = &basic{kind: IntKind, name: "u8"}
	U16  = &basic{kind: IntKind, name: "u16"}
	U32  = &basic{kind: IntKind, name: "u32"}
	U64  = &basic{kind: IntKind, name: "u64"}
	U128 = &basic{kind: IntKind, name: "u128"}

	// Floats
	F32 = &basic{kind: FloatKind, name: "f32"}
	// Keep legacy "float" spelling as the canonical f64; alias F64 to Float.
	F64 = Float
)
