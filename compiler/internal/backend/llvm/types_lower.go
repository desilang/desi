package llvm

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/types"
)

// LowerPrimType maps a Desi high-level type to a textual LLVM IR type.
// Tier-0 rules:
//   - int              -> i32
//   - i8/u8            -> i8
//   - i16/u16          -> i16
//   - i32/u32          -> i32
//   - i64/u64          -> i64
//   - i128/u128        -> i128
//   - isize/usize      -> i64
//   - f32              -> float
//   - f64/float        -> double
//   - bool             -> i1
//   - str              -> ptr           (opaque pointer; pointee ignored)
//   - cptr[T]          -> ptr           (ignore T at ABI layer tier-0)
//   - none             -> void
//   - tuple[...]       -> ptr           (opaque for now)
//   - future[T]        -> ptr           (coarse handle by async wrappers)
//
// Unknown forms default to "ptr".
func LowerPrimType(t types.T) string {
	if t == nil {
		return "void"
	}
	name := t.String()
	return lowerByName(name)
}

func lowerByName(name string) string {
	switch name {
	// Unsized & pointer-sized ints
	case "int":
		return "i32"
	case "usize", "isize":
		return "i64"

	// Sized signed/unsigned ints
	case "i8", "u8":
		return "i8"
	case "i16", "u16":
		return "i16"
	case "i32", "u32":
		return "i32"
	case "i64", "u64":
		return "i64"
	case "i128", "u128":
		return "i128"

	// Floats
	case "f32":
		return "float"
	case "f64", "float": // legacy 'float' is our f64 alias
		return "double"

	// Other primitives
	case "bool":
		return "i1"
	case "str":
		return "ptr"
	case "none":
		return "void"
	}

	// Parameterized spellings like cptr[T], future[T], tuple[...]
	if strings.HasPrefix(name, "cptr[") {
		return "ptr"
	}
	if strings.HasPrefix(name, "future[") {
		return "ptr"
	}
	if strings.HasPrefix(name, "tuple[") {
		return "ptr"
	}
	return "ptr"
}

// LowerFuncSignature returns LLVM textual types for a function type.
// (Used by extern-declare emission.)
func LowerFuncSignature(ft *types.Func) (ret string, params []string) {
	if ft == nil {
		return "void", nil
	}
	ret = LowerPrimType(ft.Ret)
	params = make([]string, len(ft.Params))
	for i, p := range ft.Params {
		params[i] = LowerPrimType(p)
	}
	return
}
