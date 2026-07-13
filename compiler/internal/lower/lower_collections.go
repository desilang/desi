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
		return hir.ConstNull{}
	}

	// For primitives, use null (runtime will use type_tag)
	if t == types.Int || t == types.Float || t == types.Bool || t == types.Str || t == types.None {
		return hir.ConstNull{}
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
	return hir.ConstNull{}
}

func (ls *lowerState) lowerDictLit(d *ast.DictLit) hir.Value {
	// Create new dict handle
	res := ls.b.FreshTemp("dict")
	valueSize := hir.ConstInt{Text: "8", Type: "i64"}

	// Determine key type tag, key size, value type tag, and to_str function
	var keyTypeTag hir.Value = hir.ConstInt{Text: "0", Type: "i32"} // default int
	var keySize hir.Value = hir.ConstInt{Text: "0", Type: "i64"}    // 0 for primitives
	var valTypeTag hir.Value = hir.ConstInt{Text: "0", Type: "i32"}
	var toStrFunc hir.Value = hir.ConstNull{}
	var keyHashFn hir.Value = hir.ConstNull{} // For custom types
	var keyEqFn hir.Value = hir.ConstNull{}   // For custom types

	var keyType types.T

	if ls.info != nil {
		if t, ok := ls.info.Types[d].(*types.Dict); ok {
			keyType = t.Key
			keyTypeTag = getTypeTag(t.Key)
			valTypeTag = getTypeTag(t.Val)
			toStrFunc = resolveToStrFunc(t.Val)

			// Check for custom key type (class, struct, enum, list, set)
			// All non-primitive types use TYPE_TAG_CUSTOM with optional hash/eq functions
			isCustomKey := false
			var clsName string

			switch kt := t.Key.(type) {
			case *types.Class:
				isCustomKey = true
				clsName = kt.Name
				// Check for optional __hash__ and __eq__ dunders
				if _, hasHash := kt.Dunders["__hash__"]; hasHash {
					keyHashFn = hir.Var{Name: "@" + kt.Name + "___hash__"}
				}
				if _, hasEq := kt.Dunders["__eq__"]; hasEq {
					keyEqFn = hir.Var{Name: "@" + kt.Name + "___eq__"}
				}
			case *types.Struct:
				isCustomKey = true
				clsName = kt.Name
			case *types.Enum:
				isCustomKey = true
				clsName = kt.Name
			case *types.List:
				isCustomKey = true
				clsName = "list"
			case *types.Set:
				isCustomKey = true
				clsName = "set"
			}

			if isCustomKey {
				// TYPE_TAG_CUSTOM = 4
				keyTypeTag = hir.ConstInt{Text: "4", Type: "i32"}
				// Pointer size for key storage
				keySize = hir.ConstInt{Text: "8", Type: "i64"}
				_ = clsName // may be used for debugging
			}
		}
	}

	// dict_new(key_type_tag, key_size, value_size, value_type_tag, key_hash_fn, key_eq_fn, to_str_fn)
	ls.b.Emit(&hir.Call{Dst: res, Fn: "dict_new", Args: []hir.Value{
		keyTypeTag, keySize, valueSize, valTypeTag, keyHashFn, keyEqFn, toStrFunc,
	}})

	// Insert each key-value pair
	for i := range d.Keys {
		keyVal := ls.lowerExpr(d.Keys[i])
		val := ls.lowerExpr(d.Values[i])

		// Dict retains stored pointer values (heap types) — treat as moved
		if id, ok := d.Values[i].(*ast.Ident); ok {
			if t := ls.info.Types[d.Values[i]]; t != nil {
				switch t.(type) {
				case *types.List, *types.Set, *types.Dict, *types.Enum, *types.Struct:
					ls.cur().moved[id.Name] = true
				}
			}
		}

		// Determine key type for this entry
		var entryKeyType types.T
		if ls.info != nil {
			entryKeyType = ls.info.Types[d.Keys[i]]
		}
		if entryKeyType == nil {
			entryKeyType = keyType
		}

		// Prepare key arguments based on key type
		// dict_insert(dict, key_int, key_str, key_float, key_ptr, &value, value_type_tag)
		var keyInt hir.Value = hir.ConstInt{Text: "0", Type: "i64"}
		var keyStr hir.Value = hir.ConstNull{}
		var keyFloat hir.Value = hir.ConstFloat{Text: "0.0"}
		var keyPtr hir.Value = hir.ConstNull{}

		isStrKey := types.Equal(entryKeyType, types.Str)
		isFloatKey := entryKeyType == types.Float || entryKeyType == types.F32 || entryKeyType == types.F64

		// Check for any custom key type (class, struct, enum, list, set)
		isCustomKey := false
		switch entryKeyType.(type) {
		case *types.Class, *types.Struct, *types.Enum, *types.List, *types.Set:
			isCustomKey = true
		}

		if isStrKey {
			keyStr = keyVal
		} else if isFloatKey {
			keyFloat = keyVal
		} else if isCustomKey {
			keyPtr = keyVal
		} else {
			// int or bool - cast to i64
			keyInt64 := ls.b.FreshTemp("key_i64")
			ls.b.Emit(&hir.Cast{Dst: keyInt64, Src: keyVal, Type: "i64"})
			keyInt = keyInt64
		}

		// Get value type to determine if float (need BitCast to preserve bits)
		var entryValType types.T
		if ls.info != nil {
			entryValType = ls.info.Types[d.Values[i]]
		}
		isFloatVal := entryValType == types.Float || entryValType == types.F32 || entryValType == types.F64

		// Spill value to stack to pass as pointer
		valPtr := ls.b.FreshTemp("val_ptr")
		ls.b.Emit(&hir.Alloca{Dst: valPtr, Type: "i64", Count: 1})

		if isFloatVal {
			val64 := ls.b.FreshTemp("val64")
			ls.b.Emit(&hir.BitCast{Val: val, Dst: val64, Type: "i64"})
			ls.b.Emit(&hir.Store{Dst: valPtr, Val: val64})
		} else {
			val64 := ls.b.FreshTemp("val64")
			ls.b.Emit(&hir.Cast{Dst: val64, Src: val, Type: "i64"})
			ls.b.Emit(&hir.Store{Dst: valPtr, Val: val64})
		}

		// dict_insert(dict, key_int, key_str, key_float, key_ptr, &value, value_type_tag)
		// When value type is Any, use the actual per-entry type tag
		insertTag := valTypeTag
		if entryValType != nil && types.Equal(ls.info.Types[d].(*types.Dict).Val, types.Any) {
			insertTag = getTypeTag(entryValType)
		}
		ls.b.Emit(&hir.Call{Fn: "dict_insert", Args: []hir.Value{res, keyInt, keyStr, keyFloat, keyPtr, valPtr, insertTag}})
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

	// Unicode character (32-bit scalar)
	case "char", "rune":
		return "i32"

	// Decimal (ptr to libmpdec struct)
	case "decimal":
		return "ptr"

	// Aliases
	case "byte":
		return "i8"
	case "uint":
		return "i64"
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
