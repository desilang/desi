package lower

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// resolveToStrFunc returns the to_str function name for a type, or null for primitives
func resolveToStrFunc(t types.T) hir.Value {
	if t == nil {
		return hir.ConstStr{Text: "null"}
	}

	// For primitives, use null (runtime will use type_tag)
	if t == types.Int || t == types.Float || t == types.Bool || t == types.Str || t == types.None {
		return hir.ConstStr{Text: "null"}
	}

	// For classes with __str__, __repr__, or to_str
	if cls, ok := t.(*types.Class); ok {
		// Check for __str__ first (preferred for human-readable)
		if _, found := cls.Dunders["__str__"]; found {
			return hir.Var{Name: "@" + cls.Name + "___str__"}
		}
		// Check for __repr__ (Python-style fallback)
		if _, found := cls.Dunders["__repr__"]; found {
			return hir.Var{Name: "@" + cls.Name + "___repr__"}
		}
		// Check for to_str (legacy Desi-style)
		if _, found := cls.Methods["to_str"]; found {
			return hir.Var{Name: "@" + cls.Name + "_to_str"}
		}
	}

	// For unknown types, use null
	return hir.ConstStr{Text: "null"}
}

func (ls *lowerState) lowerDictLit(d *ast.DictLit) hir.Value {
	// Create new dict handle
	// For Tier-0, assume value_size = sizeof(int) = 8 (64-bit)
	res := ls.b.FreshTemp("dict")
	valueSize := hir.ConstInt{Text: "8", Type: "i64"}

	// Determine type tag and to_str function
	var typeTag hir.Value = hir.ConstInt{Text: "0", Type: "i32"}
	var toStrFunc hir.Value = hir.ConstStr{Text: "null"}

	if ls.info != nil {
		if t, ok := ls.info.Types[d].(*types.Dict); ok {
			typeTag = getTypeTag(t.Val)
			toStrFunc = resolveToStrFunc(t.Val)
		}
	}

	ls.b.Emit(&hir.Call{Dst: res, Fn: "dict_new", Args: []hir.Value{valueSize, typeTag, toStrFunc}})

	// Insert each key-value pair
	for i := range d.Keys {
		key := ls.lowerExpr(d.Keys[i])
		val := ls.lowerExpr(d.Values[i])

		// For Tier-0, we need to pass pointers to the values.
		// Since 'val' might be an immediate (e.g. integer), we spill it to a temp alloca.
		valPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Alloca{Dst: valPtr, Type: "i64", Count: 1})
		ls.b.Emit(&hir.Store{Dst: valPtr, Val: val})

		// Emit a call to dict_insert(dict, key, &value, type_tag)
		ls.b.Emit(&hir.Call{Fn: "dict_insert", Args: []hir.Value{res, key, valPtr, typeTag}})
	}

	return res
}

// lowerType maps types to LLVM strings (Tier-0 subset).
func lowerType(t types.T) string {
	if t == nil {
		return "void"
	}

	// Handle struct types explicitly
	if _, ok := t.(*types.Struct); ok {
		return "ptr"
	}
	name := t.String()
	switch name {
	// Signed integers
	case "int", "i32":
		return "i32"
	case "i8":
		return "i8"
	case "i16":
		return "i16"
	case "i64", "isize":
		return "i64"
	case "i128":
		return "i128"

	// Unsigned integers (LLVM uses same iN types, signedness is in ops)
	case "u8":
		return "i8"
	case "u16":
		return "i16"
	case "u32":
		return "i32"
	case "u64", "usize":
		return "i64"
	case "u128":
		return "i128"

	// Floats
	case "float", "f64":
		return "double"
	case "f32":
		return "float"

	// Other primitives
	case "bool":
		return "i1"
	case "str":
		return "ptr"
	case "none":
		return "void"
	}
	if strings.HasPrefix(name, "list[") {
		return "ptr" // list struct pointer
	}
	return "ptr" // default
}

func getTypeTag(t types.T) hir.Value {
	if t == nil {
		return hir.ConstInt{Text: "0", Type: "i32"}
	}
	if types.Equal(t, types.Str) {
		return hir.ConstInt{Text: "1", Type: "i32"}
	}
	if types.Equal(t, types.Bool) {
		return hir.ConstInt{Text: "2", Type: "i32"}
	}
	if types.Equal(t, types.Float) {
		return hir.ConstInt{Text: "3", Type: "i32"}
	}
	return hir.ConstInt{Text: "0", Type: "i32"}
}
