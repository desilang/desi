package types

// Sized integers and floats.
// Use IntKind/FloatKind for backward compatibility with types.Equal().
// The new specific Kind constants (I8Kind, U8Kind, etc.) are defined in types.go
// for future use when we add proper numeric conversion rules.

var (
	// Signed integers (use IntKind for compatibility)
	I8   = &basic{kind: IntKind, name: "i8"}
	I16  = &basic{kind: IntKind, name: "i16"}
	I32  = &basic{kind: IntKind, name: "i32"}
	I64  = &basic{kind: IntKind, name: "i64"}
	I128 = &basic{kind: IntKind, name: "i128"}

	// Unsigned integers (use IntKind for compatibility)
	U8   = &basic{kind: IntKind, name: "u8"}
	U16  = &basic{kind: IntKind, name: "u16"}
	U32  = &basic{kind: IntKind, name: "u32"}
	U64  = &basic{kind: IntKind, name: "u64"}
	U128 = &basic{kind: IntKind, name: "u128"}

	// Floats (use FloatKind for compatibility)
	F32 = &basic{kind: FloatKind, name: "f32"}
	F64 = &basic{kind: FloatKind, name: "f64"}

	// Aliases for semantic clarity
	Byte = U8   // byte = u8
	Rune = Char // rune = char (Go-style)
	UInt = U64  // uint = u64 (unsigned counterpart to int)
)
