package llvm

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/types"
)

// LowerPrimType maps a Desi high-level type to a textual LLVM IR type.
// Tier-0 rules:
//   - int              -> i32
//   - usize, isize     -> i64
//   - float            -> double
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
	case "int":
		return "i32"
	case "usize", "isize":
		return "i64"
	case "float":
		return "double"
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
// (Used by extern-declare emission in the next task.)
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
