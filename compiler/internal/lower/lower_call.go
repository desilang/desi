package lower

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

func (ls *lowerState) lowerVariadicCall(x *ast.CallExpr, ft *types.Func) hir.Value {
	callee := ls.calleeName(x.Callee)

	// Fixed params
	nFixed := len(ft.Params) - 1
	var args []hir.Value

	// Lower fixed args (positional only for Tier-0)
	for i := 0; i < nFixed && i < len(x.Args); i++ {
		args = append(args, ls.lowerExpr(x.Args[i]))
	}

	// Excess args
	var excess []hir.Value
	for i := nFixed; i < len(x.Args); i++ {
		excess = append(excess, ls.lowerExpr(x.Args[i]))
	}

	// Construct list
	// List type: ft.Params[nFixed] which is list[T]
	// Elem type T:
	var elemTy string = "ptr" // default
	if lst, ok := ft.Params[nFixed].(*types.List); ok {
		elemTy = lowerType(lst.Elem)
	}

	// 1. Allocate array
	count := len(excess)
	arrDst := ls.b.FreshTemp("varargs_arr")
	if count > 0 {
		ls.b.Emit(&hir.Alloca{Type: elemTy, Count: count, Dst: arrDst})

		// 2. Populate array
		for i, val := range excess {
			// GEP
			ptrDst := ls.b.FreshTemp("elem_ptr")
			idxVal := hir.ConstInt{Text: fmt.Sprintf("%d", i)}
			ls.b.Emit(&hir.GetElementPtr{
				Type:    elemTy,
				Base:    arrDst,
				Indices: []hir.Value{idxVal},
				Dst:     ptrDst,
			})
			// Store
			ls.b.Emit(&hir.Store{Dst: ptrDst, Val: val})
		}
	} else {
		// Allocate 1 dummy element to get a valid pointer
		ls.b.Emit(&hir.Alloca{Type: elemTy, Count: 1, Dst: arrDst})
	}

	// 3. Allocate list struct {ptr, i64}
	listDst := ls.b.FreshTemp("varargs_list")
	ls.b.Emit(&hir.Alloca{Type: "{ptr, i64}", Count: 1, Dst: listDst})

	// 4. Store array ptr to list.0
	// GEP to field 0
	f0Dst := ls.b.FreshTemp("list_ptr")
	ls.b.Emit(&hir.GetElementPtr{
		Type:    "{ptr, i64}",
		Base:    listDst,
		Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: "0"}},
		Dst:     f0Dst,
	})
	ls.b.Emit(&hir.Store{Dst: f0Dst, Val: arrDst})

	// 5. Store len to list.1
	// GEP to field 1 (FIXED: was 0, should be 1)
	f1Dst := ls.b.FreshTemp("list_len")
	ls.b.Emit(&hir.GetElementPtr{
		Type:    "{ptr, i64}",
		Base:    listDst,
		Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: "1"}},
		Dst:     f1Dst,
	})
	// Store as i64
	lenVal := hir.ConstInt{Text: fmt.Sprintf("%d", count), Type: "i64"}
	ls.b.Emit(&hir.Store{Dst: f1Dst, Val: lenVal})

	// Add list to args
	args = append(args, listDst)

	// Emit call
	dst := ls.b.FreshTemp("call")
	ls.b.Emit(&hir.Call{Dst: dst, Fn: callee, Args: args})
	return dst
}

func (ls *lowerState) lowerCall(x *ast.CallExpr) hir.Value {
	// 0. Method calls (FieldExpr callee)
	if fe, ok := x.Callee.(*ast.FieldExpr); ok {
		if ls.info != nil {
			feXType := ls.info.Types[fe.X]
			// DEBUG
			// fmt.Printf("DEBUG lowerCall: fe.X type = %T, value = %v, method = %s\n", feXType, feXType, fe.Name.Name)

			if t, ok := feXType.(*types.Dict); ok {
				return ls.lowerDictMethod(fe, x.Args, t)
			}
			if t, ok := feXType.(*types.Set); ok {
				return ls.lowerSetMethod(fe, x.Args, t)
			}
			if t, ok := feXType.(*types.List); ok {
				// Check for join method on list<str>
				if fe.Name.Name == "join" && t.Elem == types.Str {
					if res := ls.lowerListJoin(fe, x.Args, t); res != nil {
						return res
					}
				}
				return ls.lowerListMethod(fe, x.Args, t)
			}

			// String methods: split, replace
			if feXType == types.Str {
				if res := ls.lowerStringMethod(fe, x.Args); res != nil {
					return res
				}
			}

			// File methods: read, write, close, is_open
			if types.Equal(feXType, types.File) {
				return ls.lowerFileMethod(fe, x.Args)
			}

			if types.IsOption(feXType) {
				// We pass nil for *types.Enum because lowerOptionMethod doesn't strictly need it
				// or we can extract it. Let's update lowerOptionMethod to take types.T later if needed.
				// For now, just pass nil as it seems unused in my implementation above except for signature.
				// Wait, I defined it to take *types.Enum. I should cast it.
				var enum *types.Enum
				if e, ok := feXType.(*types.Enum); ok {
					enum = e
				} else if g, ok := feXType.(*types.Generic); ok {
					if e, ok := g.Base.(*types.Enum); ok {
						enum = e
					}
				}
				return ls.lowerOptionMethod(fe, x.Args, enum)
			}

			if types.IsResult(feXType) {
				var enum *types.Enum
				if e, ok := feXType.(*types.Enum); ok {
					enum = e
				} else if g, ok := feXType.(*types.Generic); ok {
					if e, ok := g.Base.(*types.Enum); ok {
						enum = e
					}
				}
				return ls.lowerResultMethod(fe, x.Args, enum)
			}

			// Arena methods: alloc
			if _, ok := feXType.(*types.Arena); ok {
				if fe.Name.Name == "alloc" {
					// arena.alloc(size) -> ArenaAlloc HIR node
					arenaVal := ls.lowerExpr(fe.X) // Lower the arena receiver
					dst := ls.b.FreshTemp("alloc")

					// Get size from args (default to 8 if not provided)
					var sizeVal hir.Value = hir.ConstInt{Text: "8", Type: "i64"}
					if len(x.Args) > 0 {
						sizeVal = ls.lowerExpr(x.Args[0])
					}

					ls.b.Emit(&hir.ArenaAlloc{Dst: dst, Arena: arenaVal, Args: []hir.Value{sizeVal}})
					ls.tempsFromArenaAlloc[dst.Name] = true
					return dst
				}
			}
		}
	}

	// 1. Variadic calls (M14)
	// Look up the function by name to get its signature
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee)
		if set, ok := ls.info.Funcs[calleeName]; ok && len(set.Cands) > 0 {
			// Check if any candidate is variadic
			// In practice, after type checking, we know which one was chosen
			// For simplicity, check the first variadic candidate
			// TODO: This could be improved by tracking which candidate was chosen
			for _, cand := range set.Cands {
				if cand.Type != nil && cand.Type.Variadic {
					return ls.lowerVariadicCall(x, cand.Type)
				}
			}
		}
	}

	// 1.5. len() builtin
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee)
		if calleeName == "len" && len(x.Args) == 1 {
			if argT := ls.info.Types[x.Args[0]]; argT != nil {
				// Lower argument
				argVal := ls.lowerExpr(x.Args[0])

				var lenFunc string
				retType := "i64"

				// Check if it's a string type
				if argT == types.Str {
					lenFunc = "string_len"
				} else if _, ok := argT.(*types.List); ok {
					lenFunc = "list_len"
				} else if _, ok := argT.(*types.Dict); ok {
					lenFunc = "dict_len"
				} else if _, ok := argT.(*types.Set); ok {
					lenFunc = "set_len"
				} else if cls, ok := argT.(*types.Class); ok {
					// Check for __len__
					if _, found := cls.Dunders["__len__"]; found {
						lenFunc = fmt.Sprintf("%s___len__", cls.Name)
						retType = "i32" // User methods return i32
					}
				}

				if lenFunc != "" {
					resTemp := ls.b.FreshTemp("len_res")
					ls.b.Emit(&hir.Call{Dst: resTemp, Fn: lenFunc, Args: []hir.Value{argVal}, Type: retType})

					if retType == "i64" {
						// Cast to i32 (Desi int)
						res := ls.b.FreshTemp("len")
						ls.b.Emit(&hir.Cast{Dst: res, Src: resTemp, Type: "i32"})
						return res
					} else {
						return resTemp
					}
				}
			}
		}
	}

	// 1.6. open() builtin - file I/O
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee)
		if calleeName == "open" && len(x.Args) == 2 {
			return ls.lowerFileOpen(x.Args)
		}
	}

	// 1.7. assert() builtin - testing/debugging
	// assert(condition) or assert(condition, message)
	// Calls __assert_check which aborts if condition is false
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee)
		if calleeName == "assert" && len(x.Args) >= 1 && len(x.Args) <= 2 {
			// Lower the condition
			condVal := ls.lowerExpr(x.Args[0])

			// Get optional message or use default
			var msgVal hir.Value
			if len(x.Args) == 2 {
				msgVal = ls.lowerExpr(x.Args[1])
			} else {
				msgVal = hir.ConstStr{Text: "assertion failed"}
			}

			// Call runtime helper: __assert_check(cond, msg)
			// If cond is false, it prints the message and aborts
			ls.b.Emit(&hir.Call{Fn: "__assert_check", Args: []hir.Value{condVal, msgVal}})

			// Return none/void
			return nil
		}
	}

	// 2. M14 Stage 3: print(Display) + Auto to_str for collections
	// If we have type info, check if this is print(arg) where:
	// - arg implements Display trait, OR
	// - arg is a collection (list/dict/set) with to_str method, OR
	// - arg is a custom class with to_str method
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee)
		if calleeName == "print" && len(x.Args) == 1 {
			if argT := ls.info.Types[x.Args[0]]; argT != nil {
				typeName := argT.String()
				shouldCallToStr := false
				toStrFuncName := ""

				// Case 1: Display trait implementation
				if impls, ok := ls.info.Impls[typeName]; ok {
					if _, hasDisplay := impls["Display"]; hasDisplay {
						shouldCallToStr = true
						toStrFuncName = fmt.Sprintf("%s_to_str", typeName)
					}
				}

				// Case 2: Built-in collections (list, dict, set)
				if !shouldCallToStr {
					switch t := argT.(type) {
					case *types.List:
						shouldCallToStr = true
						toStrFuncName = "list_to_str"
					case *types.Dict:
						shouldCallToStr = true
						toStrFuncName = "dict_to_str"
					case *types.Set:
						shouldCallToStr = true
						toStrFuncName = "set_to_str"
					case *types.Class:
						// Case 3: Custom class with __str__, __repr__, or to_str dunder
						if _, found := t.Dunders["__str__"]; found {
							shouldCallToStr = true
							toStrFuncName = fmt.Sprintf("%s___str__", t.Name)
						} else if _, found := t.Dunders["__repr__"]; found {
							// Python-style: __repr__ as fallback for __str__
							shouldCallToStr = true
							toStrFuncName = fmt.Sprintf("%s___repr__", t.Name)
						} else if _, found := t.Dunders["to_str"]; found {
							shouldCallToStr = true
							toStrFuncName = fmt.Sprintf("%s_to_str", t.Name)
						}
					}
				}

				if shouldCallToStr {
					// 1. Lower arg
					argVal := ls.lowerExpr(x.Args[0])
					// 2. Emit call to to_str
					strTemp := ls.b.FreshTemp("str")
					ls.b.Emit(&hir.Call{Dst: strTemp, Fn: toStrFuncName, Args: []hir.Value{argVal}, Type: "ptr"})
					// 3. Emit call to print(str)
					dst := ls.b.FreshTemp("print")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "print", Args: []hir.Value{strTemp}})
					return dst
				}
			}
		}
	}

	// 2. M14 Stage 1: Method Calls (obj.method())
	if fe, ok := x.Callee.(*ast.FieldExpr); ok && ls.info != nil {
		// Check if this is a method call
		// We need the type of the receiver (fe.X)
		if recvT := ls.info.Types[fe.X]; recvT != nil {
			typeName := recvT.String()
			methodName := fe.Name.Name

			// Handle Class Methods
			var cls *types.Class
			if c, ok := recvT.(*types.Class); ok {
				cls = c
			}

			if cls != nil {
				// Detect if fe.X is a Type symbol (unbound method call: Parent.method(self))
				// vs instance access (obj.method())
				// Heuristic: if fe.X is an Ident with the same name as the class, it's a type access
				isUnboundMethod := false
				if id, ok := fe.X.(*ast.Ident); ok {
					if id.Name == cls.Name {
						isUnboundMethod = true
					}
				}
				// Check if method exists in class
				// We can just trust the checker if we are sure, but let's be safe
				// Actually, for classes, we just mangle as ClassName_MethodName
				// The checker guarantees existence.

				// Find defining class to use for mangled name
				definingClass := cls

				// Check which map the method is in
				var targetMethod *types.Func
				var isStatic, isClass bool

				if m, ok := cls.Methods[methodName]; ok {
					targetMethod = m
				} else if m, ok := cls.StaticMethods[methodName]; ok {
					targetMethod = m
					isStatic = true
				} else if m, ok := cls.ClassMethods[methodName]; ok {
					targetMethod = m
					isClass = true
				} else if m, ok := cls.Dunders[methodName]; ok {
					targetMethod = m
				}

				if targetMethod != nil {
					// Walk up to find the original definition
					for definingClass.Base != nil {
						var baseMethod *types.Func
						if isStatic {
							baseMethod = definingClass.Base.StaticMethods[methodName]
						} else if isClass {
							baseMethod = definingClass.Base.ClassMethods[methodName]
						} else {
							baseMethod = definingClass.Base.Methods[methodName]
						}

						if baseMethod == targetMethod {
							definingClass = definingClass.Base
						} else {
							break
						}
					}
				}

				mangledName := fmt.Sprintf("%s_%s", definingClass.Name, methodName)

				// Check if this is a static method or class method
				isStaticMethod := false
				isClassMethod := false

				if _, ok := cls.StaticMethods[methodName]; ok {
					isStaticMethod = true
				}
				if _, ok := cls.ClassMethods[methodName]; ok {
					isClassMethod = true
				}

				// Lower args
				var args []hir.Value

				// For unbound method calls (Parent.method(self, ...)),
				// the user provides self explicitly, so we DON'T add receiver
				// For bound method calls (obj.method(...)), we add receiver as first arg
				// Static/class methods never get receiver from instance
				if !isUnboundMethod && !isStaticMethod && !isClassMethod {
					// Lower receiver for bound instance method call
					recvVal := ls.lowerExpr(fe.X)
					args = append(args, recvVal) // receiver is first arg
				}

				// Add user-provided args
				for _, a := range x.Args {
					args = append(args, ls.lowerExpr(a))
				}

				// Determine return type string for LLVM
				retType := "i32" // default
				if targetMethod != nil && targetMethod.Ret != nil {
					retType = lowerType(targetMethod.Ret)
				}

				dst := ls.b.FreshTemp("call")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: mangledName, Args: args, Type: retType})
				// Consume args (methods move by default)
				for _, arg := range args {
					ls.consumeTemp(arg)
				}
				return dst
			}

			// Check if method exists in Impls (Traits)
			// Note: This is a simplification. We should check if the method was actually resolved to a trait method.
			// But for M14, all methods on structs come from Impls (or are treated similarly).
			if impls, ok := ls.info.Impls[typeName]; ok {
				// Iterate all traits to find the method?
				// Or just check if we can find it.
				// For now, assume if we find it in any trait, it's the one.
				found := false
				for _, methods := range impls {
					for _, m := range methods {
						if m.Name.Name == methodName {
							found = true
							break
						}
					}
					if found {
						break
					}
				}

				if found {
					// Rewrite to TypeName_MethodName(obj, args...)
					mangledName := fmt.Sprintf("%s_%s", typeName, methodName)

					// Lower receiver
					recvVal := ls.lowerExpr(fe.X)

					// Lower args
					var args []hir.Value
					args = append(args, recvVal) // receiver is first arg
					for _, a := range x.Args {
						args = append(args, ls.lowerExpr(a))
					}

					dst := ls.b.FreshTemp("call")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: mangledName, Args: args})
					// Consume args (methods move by default)
					for _, arg := range args {
						ls.consumeTemp(arg)
					}
					return dst
				}
			}
		}
	}

	// 3. M14: Struct Instantiation
	// Check if callee is a Type (Struct or Generic Instance)
	// We rely on the fact that check_inst resolves the call type to the struct type.
	if ls.info != nil {
		resT := ls.info.Types[x]
		var st *types.Struct
		var cls *types.Class

		if s, ok := resT.(*types.Struct); ok {
			st = s
		} else if g, ok := resT.(*types.Generic); ok {
			if s, ok := g.Base.(*types.Struct); ok {
				st = s
			} else if c, ok := g.Base.(*types.Class); ok {
				cls = c
			}
		} else if c, ok := resT.(*types.Class); ok {
			cls = c
		}

		// Handle class instantiation (both Generic<Class> and regular Class)
		if cls != nil {
			// Class Instantiation
			// Check if it's a constructor call
			isConstructor := false

			// Heuristic: if callee name matches class name, it's a constructor.
			// This avoids issues where ls.info.Idents/Types lookup fails for the class name identifier.
			if ls.calleeName(x.Callee) == cls.Name {
				isConstructor = true
			}

			if isConstructor {
				// 1. Allocate instance
				// Calculate size
				size := 0
				for _, f := range cls.Fields {
					size += getSize(f.Type)
				}
				if size == 0 {
					size = 1
				}

				inst := ls.b.FreshTemp("inst")
				// Use GC malloc if available, or stack alloca for now?
				// Structs use alloca. Let's use alloca for consistency in Tier-0.
				// TODO: Switch to heap allocation for classes.
				ls.b.Emit(&hir.Alloca{Type: "i8", Count: size, Dst: inst})

				// 2. Call __new__
				// Find __new__
				// It should be in cls.Constructors or Dunders["__new__"]
				// We assume the checker validated arguments.
				// Mangled name: ClassName___new__
				ctorName := fmt.Sprintf("%s___new__", cls.Name)

				// Prepare args: [inst, user_args...]
				var args []hir.Value
				args = append(args, inst)
				for _, a := range x.Args {
					args = append(args, ls.lowerExpr(a))
				}

				// Emit call
				voidDst := ls.b.FreshTemp("void")
				ls.b.Emit(&hir.Call{Dst: voidDst, Fn: ctorName, Args: args})

				return inst
			}
		}

		if st != nil {
			// Check if the callee name matches the struct name (heuristic for constructor call)
			// Or just assume if the result type is a struct, it's a constructor call.
			// But it could be a function returning a struct.
			// We check if callee is an Ident that resolves to a Type symbol.
			isConstructor := false
			if id, ok := x.Callee.(*ast.Ident); ok {
				if sym := ls.info.Idents[id]; sym != nil && sym.Kind == check.SymType {
					isConstructor = true
				}
			}

			if isConstructor {
				// Emit Alloc
				// Calculate size
				size := 0
				for _, f := range st.Fields {
					size += getSize(f.Type)
				}
				// Align to 8 bytes for simplicity
				if size == 0 {
					size = 1
				} // Empty struct

				inst := ls.b.FreshTemp("inst")
				// Allocate as i8 array
				ls.b.Emit(&hir.Alloca{Type: "i8", Count: size, Dst: inst})

				// Initialize fields
				// We iterate ArgNodes to get names and values
				for _, arg := range x.ArgNodes {
					name := arg.Name.Name
					valExpr := arg.Expr
					val := ls.lowerExpr(valExpr)

					// Find field
					offset := 0
					var fieldType types.T
					for _, f := range st.Fields {
						if f.Name == name {
							fieldType = f.Type
							break
						}
						offset += getSize(f.Type)
					}

					// Emit GEP
					fieldPtr := ls.b.FreshTemp("field_ptr")
					ls.b.Emit(&hir.GetElementPtr{
						Type:    "i8",
						Base:    inst,
						Indices: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", offset)}},
						Dst:     fieldPtr,
					})

					// Check boxing (primitive -> Generic T (ptr))
					storageType := lowerType(fieldType)
					// We need to know the type of val.
					// We can look up valExpr type.
					valExprType := ls.info.Types[valExpr]
					valLowerType := lowerType(valExprType)

					if storageType == "ptr" && valLowerType != "ptr" && valLowerType != "void" {
						// Box: cast primitive to ptr
						boxed := ls.b.FreshTemp("boxed")
						ls.b.Emit(&hir.Cast{Dst: boxed, Src: val, Type: "ptr"})
						val = boxed
					}

					// Store
					ls.b.Emit(&hir.Store{
						Dst: fieldPtr,
						Val: val,
					})
				}
				return inst
			}
		}
	}

	// Legacy/Default behavior
	callee := ls.calleeName(x.Callee)
	var args []hir.Value
	for _, a := range x.Args {
		args = append(args, ls.lowerExpr(a))
	}

	// M15: Box arguments for generic functions
	// If the callee is a generic function (erased), we need to box primitive arguments to ptr
	// M15: Box arguments for generic functions
	// If the callee is a generic function (erased), we need to box primitive arguments to ptr
	if ls.info != nil {
		isGeneric := false

		// Case 1: Identifier (Function call)
		if id, ok := x.Callee.(*ast.Ident); ok {
			// Check if this is a generic function by looking at the original declaration
			if set, ok := ls.info.Funcs[id.Name]; ok && len(set.Cands) > 0 {
				// Check the declaration (not the instantiated type)
				if set.Cands[0].Decl != nil && len(set.Cands[0].Decl.TypeParams) > 0 {
					isGeneric = true
				}
			}
		}

		// Case 2: FieldExpr (Enum constructor like Option.Some)
		if field, ok := x.Callee.(*ast.FieldExpr); ok {
			// Check if the receiver is an identifier (e.g. Option)
			if id, ok := field.X.(*ast.Ident); ok {
				if sym := ls.info.Idents[id]; sym != nil {
					t := sym.Type
					// If it's a generic instance, unwrap it
					var enumType *types.Enum
					if e, ok := t.(*types.Enum); ok {
						enumType = e
					} else if g, ok := t.(*types.Generic); ok {
						if e, ok := g.Base.(*types.Enum); ok {
							enumType = e
						}
					}

					if enumType != nil && len(enumType.TypeParams) > 0 {
						isGeneric = true
					}
				} else {
					// Fallback: check types map
					if t := ls.info.Types[field.X]; t != nil {
						// If it's a generic instance, unwrap it
						var enumType *types.Enum
						if e, ok := t.(*types.Enum); ok {
							enumType = e
						} else if g, ok := t.(*types.Generic); ok {
							if e, ok := g.Base.(*types.Enum); ok {
								enumType = e
							}
						}

						if enumType != nil {
							if len(enumType.TypeParams) > 0 {
								isGeneric = true
							}
						}
					}
				}
			}
		}

		if isGeneric {
			// This is a call to a generic function/constructor
			// Box all primitive arguments to ptr (allocate + store + return pointer)
			for i, arg := range args {
				argType := "i32" // default
				if i < len(x.Args) {
					if t := ls.info.Types[x.Args[i]]; t != nil {
						argType = lowerType(t)
					}
				}

				if argType != "ptr" && argType != "void" {
					// Allocate storage
					boxPtr := ls.b.FreshTemp("arg_box_ptr")
					ls.b.Emit(&hir.Alloca{Type: argType, Count: 1, Dst: boxPtr})
					// Store value
					ls.b.Emit(&hir.Store{Dst: boxPtr, Val: arg})
					// Use pointer as argument
					args[i] = boxPtr
				}
			}
		}
	}

	// Special-cases for arena helpers
	switch callee {
	case "arena.alloc":
		dst := ls.b.FreshTemp("alloc")
		// Emit ArenaAlloc HIR node (LLVM backend will emit __arena_alloc call)
		ls.b.Emit(&hir.ArenaAlloc{Dst: dst, Arena: args[0], Args: args[1:]})
		ls.tempsFromArenaAlloc[dst.Name] = true
		return dst
	case "arena.register_poll":
		ls.b.Emit(&hir.Call{Fn: "arena.register_poll", Args: args})
		return nil
	}

	dst := ls.b.FreshTemp("call")
	if callee == "" {
		callee = "<call>"
	}

	// Infer return type from type checker
	var retType string
	if ls.info != nil {
		if t := ls.info.Types[x]; t != nil {
			retType = lowerType(t)
			// Don't set void - let backend use defaults
			if retType == "void" {
				retType = ""
			}
		}

		// M15: If calling a generic function, the return type is always ptr (erased)
		// regardless of the instantiated type.
		if id, ok := x.Callee.(*ast.Ident); ok {
			if set, ok := ls.info.Funcs[id.Name]; ok && len(set.Cands) > 0 {
				if set.Cands[0].Decl != nil && len(set.Cands[0].Decl.TypeParams) > 0 {
					// It's a generic function, so it returns ptr (unless void)
					if retType != "" {
						retType = "ptr"
					}
				}
			}
		}
	}

	// Fix for Option/Result constructors (built-in enums)
	// These are not always in ls.info.Funcs, so we force ptr return type.
	if strings.HasPrefix(callee, "Option.") || strings.HasPrefix(callee, "Result.") {
		retType = "ptr"
	}

	ls.b.Emit(&hir.Call{Dst: dst, Fn: callee, Args: args, Type: retType})

	// Consume args if not a known borrowing function
	// TODO: Use type checker info to determine if callee borrows
	if callee != "print" && callee != "asprintf" && callee != "bool_to_cstring" && !strings.HasPrefix(callee, "arena.") {
		for _, arg := range args {
			ls.consumeTemp(arg)
		}
	}

	return dst
}
