package lower

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/macro"
	"github.com/desilang/desi/compiler/internal/types"
)

// classFullyInit reports whether the checker proved that every __new__ on this
// class assigns every field, which makes zeroing the instance beforehand dead
// work. Unproven classes — and every class in a module compiled without checker
// info — keep the zeroing.
func (ls *lowerState) classFullyInit(name string) bool {
	if ls == nil || ls.info == nil {
		return false
	}
	return ls.info.FullyInitClasses[name]
}

func (ls *lowerState) lowerVariadicCall(x *ast.CallExpr, ft *types.Func) hir.Value {
	callee := ls.calleeName(x.Callee, x)

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

func (ls *lowerState) lowerKwargsCall(x *ast.CallExpr, cand *check.FuncCand) hir.Value {
	callee := ls.calleeName(x.Callee, x)
	ft := cand.Type

	// Determine how many non-kwargs params there are
	nParams := len(ft.Params)
	kwargsIdx := nParams - 1 // kwargs is always last param

	// Lower positional args (non-kwargs params)
	var args []hir.Value

	// Collect positional and named args from ArgNodes or Args
	argSource := x.ArgNodes
	if len(argSource) == 0 && len(x.Args) > 0 {
		// Fallback to legacy Args
		for i := 0; i < kwargsIdx && i < len(x.Args); i++ {
			args = append(args, ls.lowerExpr(x.Args[i]))
		}
	} else {
		// Use ArgNodes — separate positional from named
		for _, an := range argSource {
			if an.Name == nil {
				// positional arg
				args = append(args, ls.lowerExpr(an.Expr))
			}
		}
	}

	// Build kwargs dict
	// Get the value type from the dict type
	var valTypeTag hir.Value = hir.ConstInt{Text: "1", Type: "i32"} // default: str (TYPE_TAG_STR=1)
	if kwargsIdx >= 0 && kwargsIdx < len(ft.Params) {
		if dictT, ok := ft.Params[kwargsIdx].(*types.Dict); ok {
			valTypeTag = getTypeTag(dictT.Val)
		}
	}

	// dict_new(key_type_tag, key_size, value_size, value_type_tag, key_hash_fn, key_eq_fn, to_str_fn)
	dictRes := ls.b.FreshTemp("kwargs_dict")
	ls.b.Emit(&hir.Call{Dst: dictRes, Fn: "dict_new", Args: []hir.Value{
		hir.ConstInt{Text: "1", Type: "i32"}, // key_type_tag = str
		hir.ConstInt{Text: "0", Type: "i64"}, // key_size = 0 (primitives)
		hir.ConstInt{Text: "8", Type: "i64"}, // value_size = 8
		valTypeTag,                           // value_type_tag
		hir.ConstNull{},                      // key_hash_fn
		hir.ConstNull{},                      // key_eq_fn
		hir.ConstNull{},                      // to_str_fn
	}})

	// Determine if kwargs value type is Any (mixed types)
	isAnyVal := false
	if kwargsIdx >= 0 && kwargsIdx < len(ft.Params) {
		if dictT, ok := ft.Params[kwargsIdx].(*types.Dict); ok {
			isAnyVal = types.Equal(dictT.Val, types.Any)
		}
	}

	// Insert each kwargs entry
	for _, an := range argSource {
		if an.Name == nil {
			continue // skip positional
		}
		keyVal := hir.ConstStr{Text: an.Name.Name}
		val := ls.lowerExpr(an.Expr)

		// Determine the actual type of this value for proper lowering
		var entryValType types.T
		if ls.info != nil {
			entryValType = ls.info.Types[an.Expr]
		}
		isFloatVal := entryValType == types.Float || entryValType == types.F32 || entryValType == types.F64

		// Spill value to stack pointer for dict_insert
		valPtr := ls.b.FreshTemp("kw_val_ptr")
		ls.b.Emit(&hir.Alloca{Dst: valPtr, Type: "i64", Count: 1})

		if isFloatVal {
			// Float: use BitCast to preserve bits
			val64 := ls.b.FreshTemp("kw_val64")
			ls.b.Emit(&hir.BitCast{Val: val, Dst: val64, Type: "i64"})
			ls.b.Emit(&hir.Store{Dst: valPtr, Val: val64})
		} else {
			// int, str, bool, ptr: use Cast
			val64 := ls.b.FreshTemp("kw_val64")
			ls.b.Emit(&hir.Cast{Dst: val64, Src: val, Type: "i64"})
			ls.b.Emit(&hir.Store{Dst: valPtr, Val: val64})
		}

		// For Any-typed kwargs, use per-entry type tag
		insertTag := valTypeTag
		if isAnyVal && entryValType != nil {
			insertTag = getTypeTag(entryValType)
		}

		// dict_insert(dict, key_int, key_str, key_float, key_ptr, &value, value_type_tag)
		ls.b.Emit(&hir.Call{Fn: "dict_insert", Args: []hir.Value{
			dictRes,
			hir.ConstInt{Text: "0", Type: "i64"}, // key_int (unused)
			keyVal,                               // key_str
			hir.ConstFloat{Text: "0.0"},          // key_float (unused)
			hir.ConstNull{},                      // key_ptr (unused)
			valPtr,                               // &value
			insertTag,                            // value_type_tag (per-entry for Any)
		}})
	}

	// Append dict as last arg (kwargs param)
	args = append(args, dictRes)

	// Determine return type
	retType := lowerType(ft.Ret)

	// Emit call
	dst := ls.b.FreshTemp("call")
	ls.b.Emit(&hir.Call{Dst: dst, Fn: callee, Args: args, Type: retType})
	return dst
}

func (ls *lowerState) lowerCall(x *ast.CallExpr) hir.Value {
	// 0. Method calls (FieldExpr callee)
	if fe, ok := x.Callee.(*ast.FieldExpr); ok {
		if ls.info != nil {
			// Get the receiver type
			// For chained calls like nums.map(...).filter(...), fe.X is a CallExpr
			// We need to get the return type of that CallExpr
			feXType := ls.info.Types[fe.X]

			// If the type lookup failed and fe.X is a CallExpr, try to get the return type
			if feXType == nil {
				if callExpr, ok := fe.X.(*ast.CallExpr); ok {
					// Try to get the type of the whole call expression
					feXType = ls.info.Types[callExpr]
				}
			}

			// Handle sync module constructors: sync.Mutex(value), sync.Channel(cap), sync.TaskGroup()
			if id, ok := fe.X.(*ast.Ident); ok && id.Name == "sync" {
				switch fe.Name.Name {
				case "Mutex":
					// sync.Mutex(value) -> mutex_new(boxed_value)
					if len(x.Args) != 1 {
						panic("sync.Mutex requires exactly 1 argument")
					}
					argVal := ls.lowerExpr(x.Args[0])
					res := ls.b.FreshTemp("mutex")
					// Allocate memory for the value (8 bytes for i64/ptr)
					boxPtr := ls.b.FreshTemp("mutex_box")
					ls.emitAlloc(boxPtr, hir.ConstInt{Text: "8", Type: "i64"})
					// Store the value into the box
					ls.b.Emit(&hir.Store{Dst: boxPtr, Val: argVal})
					// Create mutex with pointer to boxed value
					ls.b.Emit(&hir.Call{Dst: res, Fn: "mutex_new", Args: []hir.Value{boxPtr}, Type: "ptr"})
					return res

				case "Channel":
					// sync.Channel(capacity) -> channel_new(capacity)
					if len(x.Args) != 1 {
						panic("sync.Channel requires exactly 1 argument")
					}
					argVal := ls.lowerExpr(x.Args[0])
					res := ls.b.FreshTemp("channel")
					// Convert i32 to i64 for capacity
					cap64 := ls.b.FreshTemp("cap64")
					ls.b.Emit(&hir.Cast{Src: argVal, Dst: cap64, Type: "i64"})
					ls.b.Emit(&hir.Call{Dst: res, Fn: "channel_new", Args: []hir.Value{cap64}, Type: "ptr"})
					return res

				case "TaskGroup":
					// sync.TaskGroup() -> taskgroup_new()
					if len(x.Args) != 0 {
						panic("sync.TaskGroup takes no arguments")
					}
					res := ls.b.FreshTemp("taskgroup")
					ls.b.Emit(&hir.Call{Dst: res, Fn: "taskgroup_new", Args: nil, Type: "ptr"})
					return res

				case "Supervisor":
					// sync.Supervisor() -> supervisor_new(0, 4)
					// sync.Supervisor(8) -> supervisor_new(0, 8)
					// strategy=0 (ONE_FOR_ONE), workers=N (default 4)
					var workersArg hir.Value = hir.ConstInt{Text: "4"} // default
					if len(x.Args) == 1 {
						workersArg = ls.lowerExpr(x.Args[0])
					}
					res := ls.b.FreshTemp("supervisor")
					ls.b.Emit(&hir.Call{
						Dst: res,
						Fn:  "supervisor_new",
						Args: []hir.Value{
							hir.ConstInt{Text: "0"}, // ONE_FOR_ONE
							workersArg,
						},
						Type: "ptr",
					})
					return res

				case "RwLock":
					// sync.RwLock(value) -> rwlock_new(boxed_value)
					if len(x.Args) != 1 {
						panic("sync.RwLock requires exactly 1 argument")
					}
					argVal := ls.lowerExpr(x.Args[0])
					res := ls.b.FreshTemp("rwlock")
					// Allocate memory for the value (8 bytes for i64/ptr)
					boxPtr := ls.b.FreshTemp("rwlock_box")
					ls.emitAlloc(boxPtr, hir.ConstInt{Text: "8", Type: "i64"})
					// Store the value into the box
					ls.b.Emit(&hir.Store{Dst: boxPtr, Val: argVal})
					// Create rwlock with pointer to boxed value
					ls.b.Emit(&hir.Call{Dst: res, Fn: "rwlock_new", Args: []hir.Value{boxPtr}, Type: "ptr"})
					return res

				case "Semaphore":
					// sync.Semaphore(count) -> semaphore_new(count)
					if len(x.Args) != 1 {
						panic("sync.Semaphore requires exactly 1 argument")
					}
					argVal := ls.lowerExpr(x.Args[0])
					res := ls.b.FreshTemp("semaphore")
					// Convert i32 to i64 for count
					count64 := ls.b.FreshTemp("count64")
					ls.b.Emit(&hir.Cast{Src: argVal, Dst: count64, Type: "i64"})
					ls.b.Emit(&hir.Call{Dst: res, Fn: "semaphore_new", Args: []hir.Value{count64}, Type: "ptr"})
					return res

				case "Atomic":
					// sync.Atomic(initial) -> atomic_int_new(initial)
					if len(x.Args) != 1 {
						panic("sync.Atomic requires exactly 1 argument")
					}
					argVal := ls.lowerExpr(x.Args[0])
					res := ls.b.FreshTemp("atomic")
					// Convert i32 to i64 for initial value
					val64 := ls.b.FreshTemp("val64")
					ls.b.Emit(&hir.Cast{Src: argVal, Dst: val64, Type: "i64"})
					ls.b.Emit(&hir.Call{Dst: res, Fn: "atomic_int_new", Args: []hir.Value{val64}, Type: "ptr"})
					return res
				}
			}

			// NOTE: HTTP interceptor block removed — all http.* calls are now handled
			// by the Desi module wrappers in lib/http/__mod.desi which call @extern("C")
			// functions directly. The lowerCallArgExpr() mechanism ensures callback
			// arguments (handlers, middleware) are emitted as FuncRef automatically.
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

			// Special case: if feXType is nil but method name is map or filter,
			// assume it's a list method (common for chained calls where CallExpr type lookup fails)
			if feXType == nil && (fe.Name.Name == "map" || fe.Name.Name == "filter") {
				// Try to infer the list type by lowering the receiver and assuming it's a list
				// Create a dummy list type - the actual element type will be inferred during lowering
				dummyListType := &types.List{Elem: types.Any}
				return ls.lowerListMethod(fe, x.Args, dummyListType)
			}

			// ListIter methods: collect, map, filter, first
			if t, ok := feXType.(*types.ListIter); ok {
				return ls.lowerListIterMethod(fe, x.Args, t)
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
				res := ls.lowerOptionMethod(fe, x.Args, enum)
				// Option methods return scalars, payload aliases, or
				// stack-allocated slots — never owned heap enums. A let
				// binding must not drop (free) any of these.
				ls.markNonOwnedResult(res)
				return res
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
				res := ls.lowerResultMethod(fe, x.Args, enum)
				// Same as Option: results are never owned heap enums.
				ls.markNonOwnedResult(res)
				return res
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

			// Mutex methods: lock, try_lock
			if _, ok := feXType.(*types.Mutex); ok {
				mutexVal := ls.lowerExpr(fe.X)
				dst := ls.b.FreshTemp("guard")
				switch fe.Name.Name {
				case "lock":
					// mutex.lock() -> MutexGuard*
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "mutex_lock", Args: []hir.Value{mutexVal}, Type: "ptr"})
					return dst
				case "try_lock":
					// mutex.try_lock() -> Option<MutexGuard>
					// Call mutex_try_lock which returns ptr or null
					rawGuard := ls.b.FreshTemp("raw_guard")
					ls.b.Emit(&hir.Call{Dst: rawGuard, Fn: "mutex_try_lock", Args: []hir.Value{mutexVal}, Type: "ptr"})
					// Wrap in Option.Some(guard) or Option.Nothing() based on null
					// Option.Some and Option.Nothing are functions generated by the compiler
					optResult := ls.b.FreshTemp("opt_result")
					ls.b.Emit(&hir.Call{Dst: optResult, Fn: "Option.Some", Args: []hir.Value{rawGuard}, Type: "ptr"})
					return optResult
				}
			}

			// RwLock methods: read, write, try_read, try_write
			if _, ok := feXType.(*types.RwLock); ok {
				rwlockVal := ls.lowerExpr(fe.X)
				dst := ls.b.FreshTemp("guard")
				switch fe.Name.Name {
				case "read":
					// rw.read() -> ReadGuard*
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "rwlock_read", Args: []hir.Value{rwlockVal}, Type: "ptr"})
					return dst
				case "write":
					// rw.write() -> WriteGuard*
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "rwlock_write", Args: []hir.Value{rwlockVal}, Type: "ptr"})
					return dst
				case "try_read":
					// rw.try_read() -> Option<ReadGuard>
					rawGuard := ls.b.FreshTemp("raw_guard")
					ls.b.Emit(&hir.Call{Dst: rawGuard, Fn: "rwlock_try_read", Args: []hir.Value{rwlockVal}, Type: "ptr"})
					optResult := ls.b.FreshTemp("opt_result")
					ls.b.Emit(&hir.Call{Dst: optResult, Fn: "Option.Some", Args: []hir.Value{rawGuard}, Type: "ptr"})
					return optResult
				case "try_write":
					// rw.try_write() -> Option<WriteGuard>
					rawGuard := ls.b.FreshTemp("raw_guard")
					ls.b.Emit(&hir.Call{Dst: rawGuard, Fn: "rwlock_try_write", Args: []hir.Value{rwlockVal}, Type: "ptr"})
					optResult := ls.b.FreshTemp("opt_result")
					ls.b.Emit(&hir.Call{Dst: optResult, Fn: "Option.Some", Args: []hir.Value{rawGuard}, Type: "ptr"})
					return optResult
				}
			}

			// Semaphore methods: acquire, release, try_acquire
			if _, ok := feXType.(*types.Semaphore); ok {
				semVal := ls.lowerExpr(fe.X)
				switch fe.Name.Name {
				case "acquire":
					// sem.acquire() -> void (blocks)
					ls.b.Emit(&hir.Call{Fn: "semaphore_acquire", Args: []hir.Value{semVal}, Type: "void"})
					return hir.ConstInt{Text: "0"}
				case "release":
					// sem.release() -> void
					ls.b.Emit(&hir.Call{Fn: "semaphore_release", Args: []hir.Value{semVal}, Type: "void"})
					return hir.ConstInt{Text: "0"}
				case "try_acquire":
					// sem.try_acquire() -> bool
					dst := ls.b.FreshTemp("acquired")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "semaphore_try_acquire", Args: []hir.Value{semVal}, Type: "i1"})
					return dst
				}
			}

			// Atomic methods: load, store, add, sub, inc, dec, compare_exchange, exchange
			if _, ok := feXType.(*types.Atomic); ok {
				atomicVal := ls.lowerExpr(fe.X)
				switch fe.Name.Name {
				case "load":
					// a.load() -> int
					dst := ls.b.FreshTemp("loaded")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "atomic_int_load", Args: []hir.Value{atomicVal}, Type: "i64"})
					// Convert i64 to i32 for Desi int
					result := ls.b.FreshTemp("load_i32")
					ls.b.Emit(&hir.Cast{Src: dst, Dst: result, Type: "i32"})
					return result
				case "store":
					// a.store(value) -> void
					if len(x.Args) != 1 {
						return hir.ConstInt{Text: "0"}
					}
					argVal := ls.lowerExpr(x.Args[0])
					val64 := ls.b.FreshTemp("val64")
					ls.b.Emit(&hir.Cast{Src: argVal, Dst: val64, Type: "i64"})
					ls.b.Emit(&hir.Call{Fn: "atomic_int_store", Args: []hir.Value{atomicVal, val64}, Type: "void"})
					return hir.ConstInt{Text: "0"}
				case "add":
					// a.add(delta) -> int (new value)
					if len(x.Args) != 1 {
						return hir.ConstInt{Text: "0"}
					}
					argVal := ls.lowerExpr(x.Args[0])
					delta64 := ls.b.FreshTemp("delta64")
					ls.b.Emit(&hir.Cast{Src: argVal, Dst: delta64, Type: "i64"})
					dst := ls.b.FreshTemp("add_result")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "atomic_int_add", Args: []hir.Value{atomicVal, delta64}, Type: "i64"})
					result := ls.b.FreshTemp("add_i32")
					ls.b.Emit(&hir.Cast{Src: dst, Dst: result, Type: "i32"})
					return result
				case "sub":
					// a.sub(delta) -> int (new value)
					if len(x.Args) != 1 {
						return hir.ConstInt{Text: "0"}
					}
					argVal := ls.lowerExpr(x.Args[0])
					delta64 := ls.b.FreshTemp("delta64")
					ls.b.Emit(&hir.Cast{Src: argVal, Dst: delta64, Type: "i64"})
					dst := ls.b.FreshTemp("sub_result")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "atomic_int_sub", Args: []hir.Value{atomicVal, delta64}, Type: "i64"})
					result := ls.b.FreshTemp("sub_i32")
					ls.b.Emit(&hir.Cast{Src: dst, Dst: result, Type: "i32"})
					return result
				case "inc":
					// a.inc() -> int (new value)
					dst := ls.b.FreshTemp("inc_result")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "atomic_int_inc", Args: []hir.Value{atomicVal}, Type: "i64"})
					result := ls.b.FreshTemp("inc_i32")
					ls.b.Emit(&hir.Cast{Src: dst, Dst: result, Type: "i32"})
					return result
				case "dec":
					// a.dec() -> int (new value)
					dst := ls.b.FreshTemp("dec_result")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "atomic_int_dec", Args: []hir.Value{atomicVal}, Type: "i64"})
					result := ls.b.FreshTemp("dec_i32")
					ls.b.Emit(&hir.Cast{Src: dst, Dst: result, Type: "i32"})
					return result
				case "compare_exchange":
					// a.compare_exchange(expected, desired) -> bool
					if len(x.Args) != 2 {
						return hir.ConstInt{Text: "0"}
					}
					expectedVal := ls.lowerExpr(x.Args[0])
					desiredVal := ls.lowerExpr(x.Args[1])
					exp64 := ls.b.FreshTemp("exp64")
					des64 := ls.b.FreshTemp("des64")
					ls.b.Emit(&hir.Cast{Src: expectedVal, Dst: exp64, Type: "i64"})
					ls.b.Emit(&hir.Cast{Src: desiredVal, Dst: des64, Type: "i64"})
					dst := ls.b.FreshTemp("cas_result")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "atomic_int_compare_exchange", Args: []hir.Value{atomicVal, exp64, des64}, Type: "i1"})
					return dst
				case "exchange":
					// a.exchange(new_value) -> int (old value)
					if len(x.Args) != 1 {
						return hir.ConstInt{Text: "0"}
					}
					argVal := ls.lowerExpr(x.Args[0])
					val64 := ls.b.FreshTemp("val64")
					ls.b.Emit(&hir.Cast{Src: argVal, Dst: val64, Type: "i64"})
					dst := ls.b.FreshTemp("exchange_result")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "atomic_int_exchange", Args: []hir.Value{atomicVal, val64}, Type: "i64"})
					result := ls.b.FreshTemp("exchange_i32")
					ls.b.Emit(&hir.Cast{Src: dst, Dst: result, Type: "i32"})
					return result
				}
			}

			// Channel methods: sender, receiver, close
			if chType, ok := feXType.(*types.Channel); ok {
				_ = chType // For future type param use
				channelVal := ls.lowerExpr(fe.X)
				dst := ls.b.FreshTemp("channel_result")
				switch fe.Name.Name {
				case "sender":
					// ch.sender() -> Sender*
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "channel_sender", Args: []hir.Value{channelVal}, Type: "ptr"})
					return dst
				case "receiver":
					// ch.receiver() -> Receiver*
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "channel_receiver", Args: []hir.Value{channelVal}, Type: "ptr"})
					return dst
				case "close":
					// ch.close() -> void
					ls.b.Emit(&hir.Call{Fn: "channel_close", Args: []hir.Value{channelVal}, Type: "void"})
					return hir.ConstInt{Text: "0"}
				}
			}

			// Sender methods: send, try_send
			if _, ok := feXType.(*types.ChannelSender); ok {
				senderVal := ls.lowerExpr(fe.X)
				switch fe.Name.Name {
				case "send":
					// tx.send(value) -> bool
					if len(x.Args) != 1 {
						return hir.ConstInt{Text: "0"}
					}
					argVal := ls.lowerExpr(x.Args[0])
					// The pointer crosses the channel to the receiver —
					// ownership transfers, don't free at scope end.
					ls.consumeTemp(argVal)
					// Box the value for the channel
					boxPtr := ls.b.FreshTemp("send_box")
					ls.emitAlloc(boxPtr, hir.ConstInt{Text: "8", Type: "i64"})
					ls.b.Emit(&hir.Store{Dst: boxPtr, Val: argVal})
					dst := ls.b.FreshTemp("send_result")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "channel_send", Args: []hir.Value{senderVal, boxPtr}, Type: "i1"})
					return dst
				case "try_send":
					// tx.try_send(value) -> bool
					if len(x.Args) != 1 {
						return hir.ConstInt{Text: "0"}
					}
					argVal := ls.lowerExpr(x.Args[0])
					boxPtr := ls.b.FreshTemp("try_send_box")
					ls.emitAlloc(boxPtr, hir.ConstInt{Text: "8", Type: "i64"})
					ls.b.Emit(&hir.Store{Dst: boxPtr, Val: argVal})
					dst := ls.b.FreshTemp("try_send_result")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "channel_try_send", Args: []hir.Value{senderVal, boxPtr}, Type: "i1"})
					return dst
				}
			}

			// Receiver methods: recv, try_recv
			if _, ok := feXType.(*types.ChannelReceiver); ok {
				receiverVal := ls.lowerExpr(fe.X)
				switch fe.Name.Name {
				case "recv":
					// rx.recv() -> T (unboxed value wrapped in Option)
					boxPtr := ls.b.FreshTemp("recv_box")
					ls.b.Emit(&hir.Call{Dst: boxPtr, Fn: "channel_recv", Args: []hir.Value{receiverVal}, Type: "ptr"})
					// Load the value from the box (i32 for int values)
					unboxedVal := ls.b.FreshTemp("recv_val")
					ls.b.Emit(&hir.Load{Src: boxPtr, Dst: unboxedVal, Type: "i32"})
					// Wrap in Option.Some
					optResult := ls.b.FreshTemp("recv_opt")
					ls.b.Emit(&hir.Call{Dst: optResult, Fn: "Option.Some", Args: []hir.Value{unboxedVal}, Type: "ptr"})
					return optResult
				case "try_recv":
					// rx.try_recv() -> Option[ptr]
					boxPtr := ls.b.FreshTemp("try_recv_box")
					ls.b.Emit(&hir.Call{Dst: boxPtr, Fn: "channel_try_recv", Args: []hir.Value{receiverVal}, Type: "ptr"})
					optResult := ls.b.FreshTemp("try_recv_opt")
					ls.b.Emit(&hir.Call{Dst: optResult, Fn: "Option.Some", Args: []hir.Value{boxPtr}, Type: "ptr"})
					return optResult
				}
			}

			// Supervisor methods: submit, start_child, stop, pool_size, child_count, is_running
			if _, ok := feXType.(*types.Supervisor); ok {
				supVal := ls.lowerExpr(fe.X)
				switch fe.Name.Name {
				case "stop":
					ls.b.Emit(&hir.Call{Fn: "supervisor_stop", Args: []hir.Value{supVal}})
					return hir.ConstNull{}
				case "pool_size":
					res := ls.b.FreshTemp("pool_size")
					ls.b.Emit(&hir.Call{Dst: res, Fn: "supervisor_pool_size", Args: []hir.Value{supVal}, Type: "i32"})
					return res
				case "child_count":
					res := ls.b.FreshTemp("child_count")
					ls.b.Emit(&hir.Call{Dst: res, Fn: "supervisor_child_count", Args: []hir.Value{supVal}, Type: "i32"})
					return res
				case "is_running":
					res := ls.b.FreshTemp("is_running")
					ls.b.Emit(&hir.Call{Dst: res, Fn: "__supervisor_is_running", Args: []hir.Value{supVal}, Type: "i1"})
					return res
				case "submit":
					// sup.submit(fn) -> supervisor_submit(sup, fn, NULL)
					if len(x.Args) > 0 {
						if fnIdent, ok := x.Args[0].(*ast.Ident); ok {
							fnRef := hir.FuncRef{Name: fnIdent.Name}
							ls.b.Emit(&hir.Call{Fn: "supervisor_submit", Args: []hir.Value{supVal, fnRef, hir.ConstNull{}}})
							return hir.ConstNull{}
						}
					}
					return hir.ConstNull{}
				case "start_child":
					// sup.start_child(fn) -> supervisor_start_child(sup, fn, NULL)
					if len(x.Args) > 0 {
						if fnIdent, ok := x.Args[0].(*ast.Ident); ok {
							fnRef := hir.FuncRef{Name: fnIdent.Name}
							ls.b.Emit(&hir.Call{Fn: "supervisor_start_child", Args: []hir.Value{supVal, fnRef, hir.ConstNull{}}})
							return hir.ConstNull{}
						}
					}
					return hir.ConstNull{}
				}
			}

			// TaskGroup methods: spawn, wait, cancel, is_cancelled
			if _, ok := feXType.(*types.TaskGroup); ok {
				tgVal := ls.lowerExpr(fe.X)
				switch fe.Name.Name {
				case "run":
					// tg.run(fn) -> taskgroup_spawn(tg, wrapper_fn, ctx)
					// All spawned functions must have signature fn(void* ctx)
					// We generate wrapper functions that unpack ctx and call target
					if len(x.Args) > 0 {
						var wrapperName string
						var ctxVal hir.Value = hir.ConstNull{}
						var targetFnName string
						var numCaptures int
						// LLVM type of each capture, in order, so the wrapper can
						// tell a boxed primitive from a pointer stored directly.
						var capTypes []string

						// Check if this is a capture marker: fn.__captures__(args...)
						if callExpr, ok := x.Args[0].(*ast.CallExpr); ok {
							if fieldExpr, ok := callExpr.Callee.(*ast.FieldExpr); ok {
								if fieldExpr.Name.Name == "__captures__" {
									// Extract function name from __lam$N.__captures__
									if fnIdent, ok := fieldExpr.X.(*ast.Ident); ok {
										targetFnName = fnIdent.Name
									}
									numCaptures = len(callExpr.Args)

									// Create context struct with captured values
									if numCaptures > 0 {
										ctxPtr := ls.b.FreshTemp("closure_ctx")
										ctxSize := numCaptures * 8 // 8 bytes per capture
										ls.b.Emit(&hir.Call{
											Dst:  ctxPtr,
											Fn:   "malloc",
											Args: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", ctxSize), Type: "i64"}},
											Type: "ptr",
										})

										// Store each captured value into context
										for i, arg := range callExpr.Args {
											argVal := ls.lowerExpr(arg)
											offset := ls.b.FreshTemp("cap_ptr")
											ls.b.Emit(&hir.GetElementPtr{
												Dst:     offset,
												Base:    ctxPtr,
												Indices: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", i*8), Type: "i64"}},
												Type:    "i8",
											})
											// Check if value is a primitive type that needs boxing
											// Primitives are stored as values, but we need pointers
											// Try to get type from type checker (try both the arg and Ident lookup)
											argType := ls.info.Types[arg]
											// If arg is an Ident, also try looking it up by Ident key or in scope chain
											if argType == nil {
												if id, ok := arg.(*ast.Ident); ok {
													argType = ls.info.Types[id]
													// Search all scopes for the variable type
													if argType == nil {
														for i := len(ls.scopes) - 1; i >= 0; i-- {
															if t, ok := ls.scopes[i].types[id.Name]; ok {
																argType = t
																break
															}
														}
													}
												}
											}
											needsBoxing := isPrimitiveType(argType)
											if needsBoxing {
												// Box the primitive: malloc, store value, store ptr
												boxPtr := ls.b.FreshTemp("boxed")
												ls.b.Emit(&hir.Call{
													Dst:  boxPtr,
													Fn:   "malloc",
													Args: []hir.Value{hir.ConstInt{Text: "8", Type: "i64"}},
													Type: "ptr",
												})
												ls.b.Emit(&hir.Store{Dst: boxPtr, Val: argVal})
												ls.b.Emit(&hir.Store{Dst: offset, Val: boxPtr})
												// Boxed: the wrapper loads through it.
												capTypes = append(capTypes, lowerType(argType))
											} else {
												ls.b.Emit(&hir.Store{Dst: offset, Val: argVal})
												capTypes = append(capTypes, "ptr")
											}
										}
										ctxVal = ctxPtr
									}

									// The targetFnName is __lam$N from desugaring
									// Create wrapper name with prefix to avoid collision
									wrapperName = fmt.Sprintf("__tgwrap$%s", targetFnName)
									// Emit wrapper function declaration (done once per unique wrapper)
									ls.emitTaskGroupWrapper(wrapperName, targetFnName, numCaptures, capTypes)
								}
							}
						}

						// If not capture marker, handle named function or lambda identifier
						if wrapperName == "" {
							if id, ok := x.Args[0].(*ast.Ident); ok {
								targetFnName = id.Name
								// If it's already a wrapper, use it directly
								if strings.HasPrefix(targetFnName, "__tgwrap$") {
									wrapperName = targetFnName
								} else {
									// Check if this is a lambda with captures
									var captures []string
									if ls.info != nil && ls.info.LambdaCaptures != nil {
										captures = ls.info.LambdaCaptures[targetFnName]
									}
									numCaptures = len(captures)

									// If there are captures, build context struct
									if numCaptures > 0 {
										ctxPtr := ls.b.FreshTemp("closure_ctx")
										ctxSize := numCaptures * 8 // 8 bytes per capture
										ls.b.Emit(&hir.Call{
											Dst:  ctxPtr,
											Fn:   "malloc",
											Args: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", ctxSize), Type: "i64"}},
											Type: "ptr",
										})

										// Store each captured value into context
										for i, capName := range captures {
											capVal := hir.Var{Name: capName}
											offset := ls.b.FreshTemp("cap_ptr")
											ls.b.Emit(&hir.GetElementPtr{
												Dst:     offset,
												Base:    ctxPtr,
												Indices: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", i*8), Type: "i64"}},
												Type:    "i8",
											})
											// Check if value is a primitive type that needs boxing
											var argType types.T
											for j := len(ls.scopes) - 1; j >= 0; j-- {
												if t, ok := ls.scopes[j].types[capName]; ok {
													argType = t
													break
												}
											}
											needsBoxing := isPrimitiveType(argType)
											if needsBoxing {
												// Box the primitive: malloc, store value, store ptr
												boxPtr := ls.b.FreshTemp("boxed")
												ls.b.Emit(&hir.Call{
													Dst:  boxPtr,
													Fn:   "malloc",
													Args: []hir.Value{hir.ConstInt{Text: "8", Type: "i64"}},
													Type: "ptr",
												})
												ls.b.Emit(&hir.Store{Dst: boxPtr, Val: capVal})
												ls.b.Emit(&hir.Store{Dst: offset, Val: boxPtr})
												// Boxed: the wrapper loads through it.
												capTypes = append(capTypes, lowerType(argType))
											} else {
												ls.b.Emit(&hir.Store{Dst: offset, Val: capVal})
												capTypes = append(capTypes, "ptr")
											}
										}
										ctxVal = ctxPtr
									}

									// Generate wrapper
									wrapperName = fmt.Sprintf("__tgwrap$%s", targetFnName)
									ls.emitTaskGroupWrapper(wrapperName, targetFnName, numCaptures, capTypes)
								}
							} else if fieldExpr, ok := x.Args[0].(*ast.FieldExpr); ok {
								// Check if this is a bound method: obj.method
								argType := ls.info.Types[fieldExpr.X]
								var cls *types.Class
								if c, ok := argType.(*types.Class); ok {
									cls = c
								} else if g, ok := argType.(*types.Generic); ok {
									if c, ok := g.Base.(*types.Class); ok {
										cls = c
									}
								}

								if cls != nil {
									methodName := fieldExpr.Name.Name
									if _, exists := cls.Methods[methodName]; exists {
										// Bound method: tg.run(obj.method)
										boundMethod := ls.lowerExpr(x.Args[0])

										// Extract fn from offset 0
										fnSlot := ls.b.FreshTemp("bound_fn_slot")
										ls.b.Emit(&hir.GetElementPtr{
											Type:    "i8",
											Base:    boundMethod,
											Indices: []hir.Value{hir.ConstInt{Text: "0"}},
											Dst:     fnSlot,
										})
										fnPtr := ls.b.FreshTemp("bound_fn_ptr")
										ls.b.Emit(&hir.Load{Type: "ptr", Src: fnSlot, Dst: fnPtr})

										// Extract receiver from offset 8
										recvSlot := ls.b.FreshTemp("bound_recv_slot")
										ls.b.Emit(&hir.GetElementPtr{
											Type:    "i8",
											Base:    boundMethod,
											Indices: []hir.Value{hir.ConstInt{Text: "8"}},
											Dst:     recvSlot,
										})
										recvPtr := ls.b.FreshTemp("bound_recv_ptr")
										ls.b.Emit(&hir.Load{Type: "ptr", Src: recvSlot, Dst: recvPtr})

										ls.b.Emit(&hir.Call{
											Fn:   "taskgroup_spawn",
											Args: []hir.Value{tgVal, fnPtr, recvPtr},
											Type: "void",
										})
										return hir.ConstInt{Text: "0"}
									}
								}

								// Not a class method, fallback
								fnVal := ls.lowerExpr(x.Args[0])
								ls.b.Emit(&hir.Call{
									Fn:   "taskgroup_spawn",
									Args: []hir.Value{tgVal, fnVal, ctxVal},
									Type: "void",
								})
								return hir.ConstInt{Text: "0"}
							} else {
								// Fallback: lower expression (shouldn't happen often)
								fnVal := ls.lowerExpr(x.Args[0])
								ls.b.Emit(&hir.Call{
									Fn:   "taskgroup_spawn",
									Args: []hir.Value{tgVal, fnVal, ctxVal},
									Type: "void",
								})
								return hir.ConstInt{Text: "0"}
							}
						}

						// Call taskgroup_spawn with wrapper and context
						ls.b.Emit(&hir.Call{
							Fn:   "taskgroup_spawn",
							Args: []hir.Value{tgVal, hir.FuncRef{Name: wrapperName}, ctxVal},
							Type: "void",
						})
					}
					return hir.ConstInt{Text: "0"}
				case "wait":
					// tg.wait() -> void
					ls.b.Emit(&hir.Call{Fn: "taskgroup_wait", Args: []hir.Value{tgVal}, Type: "void"})
					return hir.ConstInt{Text: "0"}
				case "cancel":
					// tg.cancel() -> void
					ls.b.Emit(&hir.Call{Fn: "taskgroup_cancel", Args: []hir.Value{tgVal}, Type: "void"})
					return hir.ConstInt{Text: "0"}
				case "is_cancelled":
					// tg.is_cancelled() -> bool
					dst := ls.b.FreshTemp("is_cancelled")
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "taskgroup_is_cancelled", Args: []hir.Value{tgVal}, Type: "i1"})
					return dst
				}
			}

			// Rc methods: get, clone
			if rcType, ok := feXType.(*types.Rc); ok {
				rcVal := ls.lowerExpr(fe.X)
				dst := ls.b.FreshTemp("rc_result")
				switch fe.Name.Name {
				case "get":
					// rc.get() -> T (get inner pointer, then load value)
					innerPtr := ls.b.FreshTemp("rc_inner_ptr")
					ls.b.Emit(&hir.Call{Dst: innerPtr, Fn: "__rc_get", Args: []hir.Value{rcVal}, Type: "ptr"})
					// Load the actual value from the inner pointer
					innerType := lowerType(rcType.Inner)
					ls.b.Emit(&hir.Load{Src: innerPtr, Dst: dst, Type: innerType})
					return dst
				case "clone":
					// rc.clone() -> Rc[T] (increments refcount, returns same ptr)
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "__rc_clone", Args: []hir.Value{rcVal}, Type: "ptr"})
					// Mark result as rcLike for cleanup
					ls.cur().rcLike[dst.Name] = true
					return dst
				}
			}

			// Arc methods: get, clone (same runtime as Rc for now)
			if arcType, ok := feXType.(*types.Arc); ok {
				arcVal := ls.lowerExpr(fe.X)
				dst := ls.b.FreshTemp("arc_result")
				switch fe.Name.Name {
				case "get":
					innerPtr := ls.b.FreshTemp("arc_inner_ptr")
					ls.b.Emit(&hir.Call{Dst: innerPtr, Fn: "__rc_get", Args: []hir.Value{arcVal}, Type: "ptr"})
					innerType := lowerType(arcType.Inner)
					ls.b.Emit(&hir.Load{Src: innerPtr, Dst: dst, Type: innerType})
					return dst
				case "clone":
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "__rc_clone", Args: []hir.Value{arcVal}, Type: "ptr"})
					ls.cur().rcLike[dst.Name] = true
					return dst
				}
			}

			// Weak methods: upgrade
			if _, ok := feXType.(*types.Weak); ok {
				weakVal := ls.lowerExpr(fe.X)
				switch fe.Name.Name {
				case "upgrade":
					rawPtr := ls.b.FreshTemp("weak_upgrade_raw")
					ls.b.Emit(&hir.Call{Dst: rawPtr, Fn: "__weak_upgrade", Args: []hir.Value{weakVal}, Type: "ptr"})

					isNotNull := ls.b.FreshTemp("weak_upgrade_is_not_null")
					ls.b.Emit(&hir.BinaryOp{Op: "!=", LHS: rawPtr, RHS: hir.ConstNull{}, Dst: isNotNull, Type: "i1"})

					resOptSlot := ls.b.FreshTemp("weak_upgrade_opt_slot")
					ls.b.Emit(&hir.Alloca{Type: "ptr", Dst: resOptSlot})

					curBlock := ls.b.Block()
					someBlock := ls.b.NewBlock("upgrade_some")
					nothingBlock := ls.b.NewBlock("upgrade_nothing")

					// Some block: call Option.Some(rawPtr) and store in slot
					ls.b.SetBlock(someBlock)
					someVal := ls.b.FreshTemp("weak_upgrade_some")
					ls.b.Emit(&hir.Call{Dst: someVal, Fn: "Option.Some", Args: []hir.Value{rawPtr}, Type: "ptr"})
					ls.b.Emit(&hir.Store{Val: someVal, Dst: resOptSlot})

					// Nothing block: call Option.Nothing() and store in slot
					ls.b.SetBlock(nothingBlock)
					nothingVal := ls.b.FreshTemp("weak_upgrade_nothing")
					ls.b.Emit(&hir.Call{Dst: nothingVal, Fn: "Option.Nothing", Args: []hir.Value{}, Type: "ptr"})
					ls.b.Emit(&hir.Store{Val: nothingVal, Dst: resOptSlot})

					ls.b.SetBlock(curBlock)
					ls.b.Emit(&hir.If{Cond: isNotNull, Then: someBlock, Else: nothingBlock})

					resOpt := ls.b.FreshTemp("weak_upgrade_opt")
					ls.b.Emit(&hir.Load{Type: "ptr", Src: resOptSlot, Dst: resOpt})
					return resOpt
				}
			}

			// Channel methods: sender, receiver, close
			if _, ok := feXType.(*types.Channel); ok {
				channelVal := ls.lowerExpr(fe.X)
				dst := ls.b.FreshTemp("channel_result")
				switch fe.Name.Name {
				case "sender":
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "channel_sender", Args: []hir.Value{channelVal}, Type: "ptr"})
					return dst
				case "receiver":
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "channel_receiver", Args: []hir.Value{channelVal}, Type: "ptr"})
					return dst
				case "close":
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "channel_close", Args: []hir.Value{channelVal}, Type: "void"})
					return dst
				}
			}

			// ChannelSender methods: send, try_send
			if _, ok := feXType.(*types.ChannelSender); ok {
				senderVal := ls.lowerExpr(fe.X)
				dst := ls.b.FreshTemp("send_result")
				if len(x.Args) > 0 {
					argVal := ls.lowerExpr(x.Args[0])
					// The pointer crosses the channel to the receiver —
					// ownership transfers, don't free at scope end.
					ls.consumeTemp(argVal)
					// Box the value to ptr
					boxPtr := ls.b.FreshTemp("send_box")
					ls.emitAlloc(boxPtr, hir.ConstInt{Text: "8", Type: "i64"})
					ls.b.Emit(&hir.Store{Dst: boxPtr, Val: argVal})
					switch fe.Name.Name {
					case "send":
						ls.b.Emit(&hir.Call{Dst: dst, Fn: "channel_send", Args: []hir.Value{senderVal, boxPtr}, Type: "i1"})
						return dst
					case "try_send":
						ls.b.Emit(&hir.Call{Dst: dst, Fn: "channel_try_send", Args: []hir.Value{senderVal, boxPtr}, Type: "i1"})
						return dst
					}
				}
			}

			// ChannelReceiver methods: recv, try_recv
			if _, ok := feXType.(*types.ChannelReceiver); ok {
				receiverVal := ls.lowerExpr(fe.X)
				dst := ls.b.FreshTemp("recv_result")
				switch fe.Name.Name {
				case "recv":
					// Returns ptr (boxed value or null)
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "channel_recv", Args: []hir.Value{receiverVal}, Type: "ptr"})
					return dst
				case "try_recv":
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "channel_try_recv", Args: []hir.Value{receiverVal}, Type: "ptr"})
					return dst
				}
			}

			// TaskGroup methods: spawn, wait, cancel, is_cancelled
			if _, ok := feXType.(*types.TaskGroup); ok {
				groupVal := ls.lowerExpr(fe.X)
				dst := ls.b.FreshTemp("taskgroup_result")
				switch fe.Name.Name {
				case "spawn":
					// spawn(fn) - fn is lowered as a function pointer
					if len(x.Args) > 0 {
						fnVal := ls.lowerExpr(x.Args[0])
						// Pass NULL for context for now
						nullCtx := hir.ConstInt{Text: "0", Type: "ptr"}
						ls.b.Emit(&hir.Call{Dst: dst, Fn: "taskgroup_spawn", Args: []hir.Value{groupVal, fnVal, nullCtx}, Type: "void"})
						return dst
					}
				case "wait":
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "taskgroup_wait", Args: []hir.Value{groupVal}, Type: "void"})
					return dst
				case "cancel":
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "taskgroup_cancel", Args: []hir.Value{groupVal}, Type: "void"})
					return dst
				case "is_cancelled":
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "taskgroup_is_cancelled", Args: []hir.Value{groupVal}, Type: "i1"})
					return dst
				}
			}
		}
	}

	// 1. Variadic calls (M14)
	// Look up the function by name to get its signature
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee, x)
		// Special case: print() - don't use standard variadic lowering
		// We handle print with custom print_item calls for proper formatting
		if calleeName == "print" {
			// Skip variadic lowering, let the print handler below handle it
			goto handlePrint
		}
		// dbg(expr) - prints [file:line] expr = value and returns the value
		if calleeName == "dbg" {
			if dbgInfo := ls.info.DbgCalls[x]; dbgInfo != nil {
				// Lower the argument to get its value
				argVal := ls.lowerExpr(x.Args[0])

				// Create prefix string: "[file:line] expr = "
				prefix := fmt.Sprintf("[%s:%d] %s = ", dbgInfo.File, dbgInfo.Line, dbgInfo.ExprText)
				prefixVal := hir.ConstStr{Text: prefix}

				// The argument is already evaluated, so the three output calls
				// can be bracketed directly.
				ls.printLock()

				// Print the prefix
				prefixTemp := ls.b.FreshTemp("dbg_prefix")
				ls.b.Emit(&hir.Call{Dst: prefixTemp, Fn: "print_raw", Args: []hir.Value{prefixVal}})

				// Print the value using print_item (handles all types)
				printTemp := ls.b.FreshTemp("dbg_print")
				ls.b.Emit(&hir.Call{Dst: printTemp, Fn: "print_item", Args: []hir.Value{argVal}})

				// Print newline
				nlVal := hir.ConstStr{Text: "\n"}
				nlTemp := ls.b.FreshTemp("dbg_nl")
				ls.b.Emit(&hir.Call{Dst: nlTemp, Fn: "print_raw", Args: []hir.Value{nlVal}})

				ls.printUnlock()

				// Return the original value
				return argVal
			}
		}
		// type_of(expr) - returns a Type[T] wrapper with runtime type info
		if calleeName == "type_of" {
			if len(x.Args) > 0 {
				// Get the type of the argument from type info
				argType := ls.info.Types[x.Args[0]]
				if argType == nil {
					argType = types.Any
				}
				typeName := argType.String()

				// Generate a unique type ID from the type name
				var typeID uint64
				for _, c := range typeName {
					typeID = typeID*31 + uint64(c)
				}

				// Get size (use 8 for pointers/refs, actual size calculation is complex)
				var typeSize int64 = 8 // Default pointer size

				// Call __desi_type_new(id, name, size)
				res := ls.b.FreshTemp("type_info")
				ls.b.Emit(&hir.Call{
					Dst: res,
					Fn:  "__desi_type_new",
					Args: []hir.Value{
						hir.ConstInt{Text: fmt.Sprintf("%d", typeID), Type: "i64"},
						hir.ConstStr{Text: typeName},
						hir.ConstInt{Text: fmt.Sprintf("%d", typeSize), Type: "i64"},
					},
					Type: "ptr",
				})
				return res
			}
		}
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

			// Check if any candidate has **kwargs
			for _, cand := range set.Cands {
				if cand.Type != nil && cand.Type.HasKwargs {
					return ls.lowerKwargsCall(x, cand)
				}
			}
		}
	}
handlePrint:

	// taskgroup_new() -> TaskGroup*
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee, x)
		if calleeName == "taskgroup_new" && len(x.Args) == 0 {
			res := ls.b.FreshTemp("taskgroup")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "taskgroup_new", Args: []hir.Value{}, Type: "ptr"})
			return res
		}
	}

	// 1.4. Conversions into and out of decimal.
	//
	// Intercepted here rather than left to the backend, which picks a builtin's
	// implementation from the argument's *LLVM* type. A decimal is a pointer,
	// and the backend has no recorded Desi type for the temp an arithmetic
	// helper like __decimal_add returns, so str(d) fell through to a
	// synthesized Unknown_to_str. The checker's type is available here and is
	// definitive.
	if ls.info != nil {
		callee := ls.calleeName(x.Callee, x)
		if len(x.Args) == 1 {
			argT := ls.typeOf(x.Args[0])
			fromDecimal := types.Equal(argT, types.Decimal)

			var fn, retTy, tempName string
			switch {
			case callee == "str" && fromDecimal:
				fn, retTy, tempName = "__decimal_to_str", "ptr", "decimal_str"
			case callee == "int" && fromDecimal:
				fn, retTy, tempName = "__decimal_to_int", "i64", "decimal_int"
			case callee == "bool" && fromDecimal:
				fn, retTy, tempName = "__decimal_to_bool", "i1", "decimal_bool"
			case callee == "float" && fromDecimal:
				fn, retTy, tempName = "__decimal_to_float", "double", "decimal_float"
			case callee == "decimal" && types.Equal(argT, types.Str):
				fn, retTy, tempName = "__decimal_new", "ptr", "decimal_val"
			case callee == "decimal" && types.Equal(argT, types.Int):
				fn, retTy, tempName = "__decimal_from_int", "ptr", "decimal_val"
			case callee == "decimal" && types.Equal(argT, types.Float):
				fn, retTy, tempName = "__decimal_from_float", "ptr", "decimal_val"
			}

			if fn != "" {
				argVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp(tempName)
				emitArg := argVal
				if fn == "__decimal_from_int" {
					// The runtime takes an int64; a Desi int is i32.
					w := ls.b.FreshTemp("dec_i64")
					ls.b.Emit(&hir.Cast{Dst: w, Src: argVal, Type: "i64"})
					emitArg = w
				}
				ls.b.Emit(&hir.Call{Dst: dst, Fn: fn, Args: []hir.Value{emitArg}, Type: retTy})
				if fn == "__decimal_to_str" {
					// mpd_to_sci mallocs; the caller owns the string.
					ls.addTempDrop(dst.Name)
				}
				if retTy == "i64" {
					// Narrow back to the i32 a Desi int is.
					n := ls.b.FreshTemp("dec_i32")
					ls.b.Emit(&hir.Cast{Dst: n, Src: dst, Type: "i32"})
					return n
				}
				return dst
			}
		}
	}

	// 1.5. len() builtin
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee, x)
		if calleeName == "len" && len(x.Args) == 1 {
			if argT := ls.typeOf(x.Args[0]); argT != nil {
				// Lower argument
				argVal := ls.lowerExpr(x.Args[0])

				var lenFunc string
				retType := "i64"

				// Unwrap TypeAlias to check underlying type
				unwrappedT := argT
				if ta, ok := argT.(*types.TypeAlias); ok {
					unwrappedT = ta.Target
				}

				// Check if it's a string or bytes type
				if unwrappedT == types.Str {
					lenFunc = "string_len"
				} else if unwrappedT == types.Bytes {
					lenFunc = "__bytes_len"
				} else if _, ok := unwrappedT.(*types.List); ok {
					lenFunc = "list_len"
				} else if _, ok := unwrappedT.(*types.Dict); ok {
					lenFunc = "dict_len"
				} else if _, ok := unwrappedT.(*types.Set); ok {
					lenFunc = "set_len"
				} else if _, ok := unwrappedT.(*types.Range); ok {
					lenFunc = "range_len"
				} else if tupT, ok := unwrappedT.(*types.Tuple); ok {
					// Tuple length is compile-time known - return constant directly
					return hir.ConstInt{Text: fmt.Sprintf("%d", len(tupT.Elems)), Type: "i32"}
				} else if cls, ok := unwrappedT.(*types.Class); ok {
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

	// 1.54. range(...) used as a value.
	//
	// A `for` over a literal range(...) never reaches here — that path is an
	// inline index loop with no allocation. This is for the first-class uses:
	// binding one, passing one, len(), indexing.
	if ls.info != nil {
		if ls.calleeName(x.Callee, x) == "range" && len(x.Args) >= 1 && len(x.Args) <= 3 {
			if _, isRange := ls.typeOf(x).(*types.Range); isRange {
				return ls.emitRangeNew(x.Args)
			}
		}
	}

	// 1.55. chr/ord/hex/oct/bin/abs/round/pow/todo/hash/id builtins
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee, x)

		// int(arg) -> convert arg to int32
		if calleeName == "int" && len(x.Args) == 1 {
			argVal := ls.lowerExpr(x.Args[0])
			argT := ls.typeOf(x.Args[0])
			res := ls.b.FreshTemp("int_res")
			if argT != nil {
				if types.Equal(argT, types.Float) || types.Equal(argT, types.F32) || types.Equal(argT, types.F64) {
					ls.b.Emit(&hir.Cast{Dst: res, Src: argVal, Type: "i32"})
					return res
				}
				if types.Equal(argT, types.Str) {
					ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_str_to_int", Args: []hir.Value{argVal}, Type: "i32"})
					return res
				}
				if types.Equal(argT, types.Bool) {
					ls.b.Emit(&hir.Select{Cond: argVal, Then: hir.ConstInt{Text: "1"}, Else: hir.ConstInt{Text: "0"}, Dst: res, Type: "i32"})
					return res
				}
				if types.Equal(argT, types.Int) || types.Equal(argT, types.I32) {
					return argVal
				}
				if types.Equal(argT, types.I64) || types.Equal(argT, types.U64) || types.Equal(argT, types.ISize) || types.Equal(argT, types.USize) {
					ls.b.Emit(&hir.Cast{Dst: res, Src: argVal, Type: "i32"})
					return res
				}
			}
			ls.b.Emit(&hir.Cast{Dst: res, Src: argVal, Type: "i32"})
			return res
		}

		// float(arg) -> convert arg to double
		if calleeName == "float" && len(x.Args) == 1 {
			argVal := ls.lowerExpr(x.Args[0])
			argT := ls.typeOf(x.Args[0])
			res := ls.b.FreshTemp("float_res")
			if argT != nil {
				if types.Equal(argT, types.Int) || types.Equal(argT, types.I32) || types.Equal(argT, types.I64) || types.Equal(argT, types.U64) {
					ls.b.Emit(&hir.Cast{Dst: res, Src: argVal, Type: "double"})
					return res
				}
				if types.Equal(argT, types.Str) {
					ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_str_to_float", Args: []hir.Value{argVal}, Type: "double"})
					return res
				}
				if types.Equal(argT, types.Bool) {
					ls.b.Emit(&hir.Select{Cond: argVal, Then: hir.ConstFloat{Text: "1.0"}, Else: hir.ConstFloat{Text: "0.0"}, Dst: res, Type: "double"})
					return res
				}
				if types.Equal(argT, types.Float) || types.Equal(argT, types.F64) {
					return argVal
				}
			}
			ls.b.Emit(&hir.Cast{Dst: res, Src: argVal, Type: "double"})
			return res
		}

		// bool(arg) -> convert arg to i1 (boolean)
		if calleeName == "bool" && len(x.Args) == 1 {
			argVal := ls.lowerExpr(x.Args[0])
			argT := ls.typeOf(x.Args[0])
			res := ls.b.FreshTemp("bool_res")
			if argT != nil {
				if types.Equal(argT, types.Int) || types.Equal(argT, types.I32) || types.Equal(argT, types.I64) || types.Equal(argT, types.U64) {
					ls.b.Emit(&hir.BinaryOp{Dst: res, Op: "!=", LHS: argVal, RHS: hir.ConstInt{Text: "0"}, Type: lowerType(argT)})
					return res
				}
				if types.Equal(argT, types.Float) || types.Equal(argT, types.F64) {
					ls.b.Emit(&hir.BinaryOp{Dst: res, Op: "!=", LHS: argVal, RHS: hir.ConstFloat{Text: "0.0"}, Type: "double"})
					return res
				}
				if types.Equal(argT, types.Str) {
					lenVal := ls.b.FreshTemp("strlen")
					ls.b.Emit(&hir.Call{Dst: lenVal, Fn: "strlen", Args: []hir.Value{argVal}, Type: "i64"})
					ls.b.Emit(&hir.BinaryOp{Dst: res, Op: ">", LHS: lenVal, RHS: hir.ConstInt{Text: "0"}, Type: "i64"})
					return res
				}
				if types.Equal(argT, types.Bool) {
					return argVal
				}
			}
			ls.b.Emit(&hir.Cast{Dst: res, Src: argVal, Type: "i1"})
			return res
		}

		// chr(n) -> __desi_chr(n) returns ptr (string)
		if calleeName == "chr" && len(x.Args) == 1 {
			argVal := ls.lowerExpr(x.Args[0])
			res := ls.b.FreshTemp("chr_res")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_chr", Args: []hir.Value{argVal}, Type: "ptr"})
			return res
		}
		// ord(s) -> __desi_ord(s) returns i32
		if calleeName == "ord" && len(x.Args) == 1 {
			argVal := ls.lowerExpr(x.Args[0])
			res := ls.b.FreshTemp("ord_res")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_ord", Args: []hir.Value{argVal}, Type: "i32"})
			return res
		}
		// hex(n) -> __desi_hex(n) returns ptr (string)
		if calleeName == "hex" && len(x.Args) == 1 {
			argVal := ls.lowerExpr(x.Args[0])
			res := ls.b.FreshTemp("hex_res")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_hex", Args: []hir.Value{argVal}, Type: "ptr"})
			return res
		}
		// oct(n) -> __desi_oct(n) returns ptr (string)
		if calleeName == "oct" && len(x.Args) == 1 {
			argVal := ls.lowerExpr(x.Args[0])
			res := ls.b.FreshTemp("oct_res")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_oct", Args: []hir.Value{argVal}, Type: "ptr"})
			return res
		}
		// bin(n) -> __desi_bin(n) returns ptr (string)
		if calleeName == "bin" && len(x.Args) == 1 {
			argVal := ls.lowerExpr(x.Args[0])
			res := ls.b.FreshTemp("bin_res")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_bin", Args: []hir.Value{argVal}, Type: "ptr"})
			return res
		}
		// abs(n) -> __desi_abs_int(n) or __desi_abs_float(n)
		if calleeName == "abs" && len(x.Args) == 1 {
			argVal := ls.lowerExpr(x.Args[0])
			argT := ls.typeOf(x.Args[0])
			if argT != nil && types.Equal(argT, types.Float) {
				res := ls.b.FreshTemp("abs_res")
				ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_abs_float", Args: []hir.Value{argVal}, Type: "double"})
				return res
			}
			res := ls.b.FreshTemp("abs_res")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_abs_int", Args: []hir.Value{argVal}, Type: "i32"})
			return res
		}
		// round(n) -> __desi_round(n, 0) or round(n, d) -> __desi_round(n, d)
		if calleeName == "round" && len(x.Args) >= 1 {
			nVal := ls.lowerExpr(x.Args[0])
			var digitsVal hir.Value = hir.ConstInt{Text: "0"}
			if len(x.Args) >= 2 {
				digitsVal = ls.lowerExpr(x.Args[1])
			}
			res := ls.b.FreshTemp("round_res")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_round", Args: []hir.Value{nVal, digitsVal}, Type: "double"})
			return res
		}
		// pow(base, exp) -> __desi_pow(base, exp)
		if calleeName == "pow" && len(x.Args) == 2 {
			baseVal := ls.lowerExpr(x.Args[0])
			expVal := ls.lowerExpr(x.Args[1])
			res := ls.b.FreshTemp("pow_res")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_pow", Args: []hir.Value{baseVal, expVal}, Type: "i32"})
			return res
		}
		// todo() or todo(msg) -> __desi_todo(msg)
		if calleeName == "todo" {
			var msgVal hir.Value = hir.ConstStr{Text: ""}
			if len(x.Args) >= 1 {
				msgVal = ls.lowerExpr(x.Args[0])
			}
			res := ls.b.FreshTemp("todo")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_todo", Args: []hir.Value{msgVal}})
			return res
		}
		// unreachable() -> __desi_unreachable()
		if calleeName == "unreachable" && len(x.Args) == 0 {
			res := ls.b.FreshTemp("unreachable")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_unreachable", Args: []hir.Value{}})
			return res
		}
		// hash(value) -> __desi_hash(value)
		if calleeName == "hash" && len(x.Args) == 1 {
			argVal := ls.lowerExpr(x.Args[0])
			res := ls.b.FreshTemp("hash_res")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_hash", Args: []hir.Value{argVal}, Type: "i32"})
			return res
		}
		// id(value) -> __desi_id(value)
		if calleeName == "id" && len(x.Args) == 1 {
			argVal := ls.lowerExpr(x.Args[0])
			res := ls.b.FreshTemp("id_res")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "__desi_id", Args: []hir.Value{argVal}, Type: "i32"})
			return res
		}
	}

	// 1.6. sum(), min(), max(), any(), all() builtins
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee, x)

		// TaskGroup() - zero-arg constructor
		if calleeName == "TaskGroup" && len(x.Args) == 0 {
			res := ls.b.FreshTemp("taskgroup")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "taskgroup_new", Args: []hir.Value{}, Type: "ptr"})
			return res
		}

		// Supervisor() or Supervisor(N) - optional pool size (from-import path)
		if calleeName == "Supervisor" && len(x.Args) <= 1 {
			var workersArg hir.Value = hir.ConstInt{Text: "4"} // default
			if len(x.Args) == 1 {
				workersArg = ls.lowerExpr(x.Args[0])
			}
			res := ls.b.FreshTemp("supervisor")
			ls.b.Emit(&hir.Call{
				Dst: res,
				Fn:  "supervisor_new",
				Args: []hir.Value{
					hir.ConstInt{Text: "0"}, // ONE_FOR_ONE
					workersArg,
				},
				Type: "ptr",
			})
			return res
		}

		// Skip print and str - they have their own arg handling
		// Skip print and str - they have their own arg handling
		if calleeName == "sorted" {
			// Find "items" and "reverse" arguments
			var itemsExpr ast.Expr
			var reverseExpr ast.Expr

			for i, arg := range x.ArgNodes {
				if arg.Name == nil {
					// Positional
					if i == 0 {
						itemsExpr = arg.Expr
					}
					if i == 1 {
						reverseExpr = arg.Expr
					}
				} else {
					// Named
					if arg.Name.Name == "items" {
						itemsExpr = arg.Expr
					}
					if arg.Name.Name == "reverse" {
						reverseExpr = arg.Expr
					}
				}
			}

			if itemsExpr != nil {
				// sorted(items) or sorted(items, reverse) -> copy list, sort in-place, return
				argVal := ls.lowerExpr(itemsExpr)
				res := ls.b.FreshTemp("sorted_res")
				ls.b.Emit(&hir.Call{Dst: res, Fn: "list_copy", Args: []hir.Value{argVal}, Type: "ptr"})
				// Get reverse arg (default to 0 = ascending)
				var reverseArg hir.Value = hir.ConstInt{Text: "0", Type: "i32"}
				if reverseExpr != nil {
					reverseArg = ls.lowerExpr(reverseExpr)
				}
				ls.b.Emit(&hir.Call{Fn: "list_sort", Args: []hir.Value{res, reverseArg}})
				return res
			}
		}

		if len(x.Args) == 1 && calleeName != "print" && calleeName != "str" {
			argType := ls.info.Types[x.Args[0]]
			argVal := ls.lowerExpr(x.Args[0])

			// Check if argument is a tuple (compile-time unroll)
			if tupType, ok := argType.(*types.Tuple); ok && len(tupType.Elems) > 0 {
				// Build struct type for GEP
				var elemTypes []string
				for range tupType.Elems {
					elemTypes = append(elemTypes, "ptr")
				}
				structType := "{" + strings.Join(elemTypes, ", ") + "}"

				switch calleeName {
				case "sum":
					// Unroll sum: result = e0 + e1 + e2 + ...
					var result hir.Value
					for i := range tupType.Elems {
						elemPtr := ls.b.FreshTemp(fmt.Sprintf("sum_elem%d_ptr", i))
						ls.b.Emit(&hir.GetElementPtr{
							Type:    structType,
							Base:    argVal,
							Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
							Dst:     elemPtr,
						})
						elemBoxed := ls.b.FreshTemp(fmt.Sprintf("sum_elem%d_boxed", i))
						ls.b.Emit(&hir.Load{Type: "ptr", Src: elemPtr, Dst: elemBoxed})
						elemVal := ls.b.FreshTemp(fmt.Sprintf("sum_elem%d", i))
						ls.b.Emit(&hir.Load{Type: "i32", Src: elemBoxed, Dst: elemVal})

						if i == 0 {
							result = elemVal
						} else {
							newResult := ls.b.FreshTemp(fmt.Sprintf("sum_acc%d", i))
							ls.b.Emit(&hir.BinaryOp{Op: "+", LHS: result, RHS: elemVal, Dst: newResult, Type: "i32"})
							result = newResult
						}
					}
					return result

				case "min":
					// Unroll min: result = min(e0, min(e1, min(e2, ...)))
					var result hir.Value
					for i := range tupType.Elems {
						elemPtr := ls.b.FreshTemp(fmt.Sprintf("min_elem%d_ptr", i))
						ls.b.Emit(&hir.GetElementPtr{
							Type:    structType,
							Base:    argVal,
							Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
							Dst:     elemPtr,
						})
						elemBoxed := ls.b.FreshTemp(fmt.Sprintf("min_elem%d_boxed", i))
						ls.b.Emit(&hir.Load{Type: "ptr", Src: elemPtr, Dst: elemBoxed})
						elemVal := ls.b.FreshTemp(fmt.Sprintf("min_elem%d", i))
						ls.b.Emit(&hir.Load{Type: "i32", Src: elemBoxed, Dst: elemVal})

						if i == 0 {
							result = elemVal
						} else {
							cmp := ls.b.FreshTemp(fmt.Sprintf("min_cmp%d", i))
							ls.b.Emit(&hir.BinaryOp{Op: "<", LHS: elemVal, RHS: result, Dst: cmp, Type: "i1"})
							newResult := ls.b.FreshTemp(fmt.Sprintf("min_sel%d", i))
							ls.b.Emit(&hir.Select{Cond: cmp, Then: elemVal, Else: result, Dst: newResult, Type: "i32"})
							result = newResult
						}
					}
					return result

				case "max":
					// Unroll max: result = max(e0, max(e1, max(e2, ...)))
					var result hir.Value
					for i := range tupType.Elems {
						elemPtr := ls.b.FreshTemp(fmt.Sprintf("max_elem%d_ptr", i))
						ls.b.Emit(&hir.GetElementPtr{
							Type:    structType,
							Base:    argVal,
							Indices: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstInt{Text: fmt.Sprintf("%d", i)}},
							Dst:     elemPtr,
						})
						elemBoxed := ls.b.FreshTemp(fmt.Sprintf("max_elem%d_boxed", i))
						ls.b.Emit(&hir.Load{Type: "ptr", Src: elemPtr, Dst: elemBoxed})
						elemVal := ls.b.FreshTemp(fmt.Sprintf("max_elem%d", i))
						ls.b.Emit(&hir.Load{Type: "i32", Src: elemBoxed, Dst: elemVal})

						if i == 0 {
							result = elemVal
						} else {
							cmp := ls.b.FreshTemp(fmt.Sprintf("max_cmp%d", i))
							ls.b.Emit(&hir.BinaryOp{Op: ">", LHS: elemVal, RHS: result, Dst: cmp, Type: "i1"})
							newResult := ls.b.FreshTemp(fmt.Sprintf("max_sel%d", i))
							ls.b.Emit(&hir.Select{Cond: cmp, Then: elemVal, Else: result, Dst: newResult, Type: "i32"})
							result = newResult
						}
					}
					return result
				}
			}

			// List case - use runtime functions
			switch calleeName {
			case "sum":
				res := ls.b.FreshTemp("sum_res")
				ls.b.Emit(&hir.Call{Dst: res, Fn: "list_sum_int", Args: []hir.Value{argVal}, Type: "i64"})
				// Cast to i32 for Desi int
				res32 := ls.b.FreshTemp("sum")
				ls.b.Emit(&hir.Cast{Dst: res32, Src: res, Type: "i32"})
				return res32
			case "min":
				res := ls.b.FreshTemp("min_res")
				ls.b.Emit(&hir.Call{Dst: res, Fn: "list_min_int", Args: []hir.Value{argVal}, Type: "i64"})
				res32 := ls.b.FreshTemp("min")
				ls.b.Emit(&hir.Cast{Dst: res32, Src: res, Type: "i32"})
				return res32
			case "max":
				res := ls.b.FreshTemp("max_res")
				ls.b.Emit(&hir.Call{Dst: res, Fn: "list_max_int", Args: []hir.Value{argVal}, Type: "i64"})
				res32 := ls.b.FreshTemp("max")
				ls.b.Emit(&hir.Cast{Dst: res32, Src: res, Type: "i32"})
				return res32
			case "any":
				res := ls.b.FreshTemp("any_res")
				ls.b.Emit(&hir.Call{Dst: res, Fn: "list_any_builtin", Args: []hir.Value{argVal}, Type: "i1"})
				return res
			case "all":
				res := ls.b.FreshTemp("all_res")
				ls.b.Emit(&hir.Call{Dst: res, Fn: "list_all_builtin", Args: []hir.Value{argVal}, Type: "i1"})
				return res
			case "sorted":
				// sorted(items) or sorted(items, reverse) -> copy list, sort in-place, return
				res := ls.b.FreshTemp("sorted_res")
				ls.b.Emit(&hir.Call{Dst: res, Fn: "list_copy", Args: []hir.Value{argVal}, Type: "ptr"})
				// Get reverse arg (default to 0 = ascending)
				var reverseArg hir.Value = hir.ConstInt{Text: "0", Type: "i32"}
				if len(x.Args) >= 2 {
					reverseArg = ls.lowerExpr(x.Args[1])
				}
				ls.b.Emit(&hir.Call{Fn: "list_sort", Args: []hir.Value{res, reverseArg}})
				return res
			case "mutex_new", "Mutex":
				// mutex_new(value) or Mutex(value) -> DesiMutex*
				// For primitive values, we need to box them (allocate + store)
				res := ls.b.FreshTemp("mutex")
				// Allocate memory for the value (8 bytes for i64/ptr)
				boxPtr := ls.b.FreshTemp("mutex_box")
				ls.emitAlloc(boxPtr, hir.ConstInt{Text: "8", Type: "i64"})
				// Store the value into the box
				ls.b.Emit(&hir.Store{Dst: boxPtr, Val: argVal})
				// Create mutex with pointer to boxed value
				ls.b.Emit(&hir.Call{Dst: res, Fn: "mutex_new", Args: []hir.Value{boxPtr}, Type: "ptr"})
				return res
			case "channel_new", "Channel":
				// channel_new(capacity) or Channel(capacity) -> DesiChannel*
				res := ls.b.FreshTemp("channel")
				// Convert i32 to i64 for capacity
				cap64 := ls.b.FreshTemp("cap64")
				ls.b.Emit(&hir.Cast{Src: argVal, Dst: cap64, Type: "i64"})
				ls.b.Emit(&hir.Call{Dst: res, Fn: "channel_new", Args: []hir.Value{cap64}, Type: "ptr"})
				return res
			case "rc":
				// rc(value) -> Rc* (reference-counted wrapper)
				// Box the value and wrap it in an Rc
				res := ls.b.FreshTemp("rc")
				// Allocate memory for the inner value (8 bytes for ptr/i64)
				innerPtr := ls.b.FreshTemp("rc_inner")
				ls.emitAlloc(innerPtr, hir.ConstInt{Text: "8", Type: "i64"})
				// Store the value into the inner allocation
				ls.b.Emit(&hir.Store{Dst: innerPtr, Val: argVal})
				// Create Rc with pointer to inner value
				ls.b.Emit(&hir.Call{Dst: res, Fn: "__rc_new", Args: []hir.Value{innerPtr}, Type: "ptr"})
				// Mark result as rc-like so it gets DecRef at scope exit
				ls.cur().rcLike[res.Name] = true
				return res
			case "arc":
				// arc(value) -> Arc* (thread-safe reference-counted wrapper)
				// Same as rc for now (Arc runtime not yet implemented)
				res := ls.b.FreshTemp("arc")
				innerPtr := ls.b.FreshTemp("arc_inner")
				ls.emitAlloc(innerPtr, hir.ConstInt{Text: "8", Type: "i64"})
				ls.b.Emit(&hir.Store{Dst: innerPtr, Val: argVal})
				ls.b.Emit(&hir.Call{Dst: res, Fn: "__rc_new", Args: []hir.Value{innerPtr}, Type: "ptr"})
				ls.cur().rcLike[res.Name] = true
				return res
			case "weak":
				// weak(rc) -> Weak*
				res := ls.b.FreshTemp("weak")
				ls.b.Emit(&hir.Call{Dst: res, Fn: "__weak_new", Args: []hir.Value{argVal}, Type: "ptr"})
				ls.cur().weakLike[res.Name] = true
				return res
			}
		}
	}

	// 1.65. reduce(), foldl(), foldr() builtins
	// reduce(func, iterable, initial) -> accumulated value
	// foldl is alias for reduce (left-to-right)
	// foldr processes right-to-left
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee, x)
		if (calleeName == "reduce" || calleeName == "foldl" || calleeName == "foldr") && len(x.Args) == 3 {
			funcExpr := x.Args[0]
			iterExpr := x.Args[1]
			initExpr := x.Args[2]

			// Get the function name to call
			funcName := ""
			if id, ok := funcExpr.(*ast.Ident); ok {
				funcName = id.Name
			} else if fe, ok := funcExpr.(*ast.FieldExpr); ok {
				// Qualified name like mod.func
				funcName = ls.calleeName(funcExpr)
				_ = fe // used above
			}

			// Lower initial value -> accumulator
			accVal := ls.lowerExpr(initExpr)

			// Lower the iterable
			iterVal := ls.lowerExpr(iterExpr)

			// Get list length
			lenTemp := ls.b.FreshTemp("reduce_len")
			ls.b.Emit(&hir.Call{Dst: lenTemp, Fn: "list_len", Args: []hir.Value{iterVal}, Type: "i64"})

			// Allocate accumulator variable (mutable)
			accPtr := ls.b.FreshTemp("acc_ptr")
			ls.b.Emit(&hir.Alloca{Dst: accPtr, Type: "i32", Count: 1})
			ls.b.Emit(&hir.Store{Dst: accPtr, Val: accVal})

			// Allocate index variable
			idxPtr := ls.b.FreshTemp("reduce_idx_ptr")
			ls.b.Emit(&hir.Alloca{Dst: idxPtr, Type: "i64", Count: 1})

			// Initialize index based on direction
			if calleeName == "foldr" {
				// Start at len-1 for right-to-left
				startIdx := ls.b.FreshTemp("start_idx")
				ls.b.Emit(&hir.BinaryOp{Dst: startIdx, Op: "-", LHS: lenTemp, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
				ls.b.Emit(&hir.Store{Dst: idxPtr, Val: startIdx})
			} else {
				// Start at 0 for left-to-right
				ls.b.Emit(&hir.Store{Dst: idxPtr, Val: hir.ConstInt{Text: "0", Type: "i64"}})
			}

			// Create condition block
			condBlk := ls.b.NewBlock("reduce_cond")
			oldCur := ls.b.Block()

			ls.b.SetBlock(condBlk)
			idxVal := ls.b.FreshTemp("reduce_idx")
			ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxVal})

			condTemp := ls.b.FreshTemp("reduce_cond")
			if calleeName == "foldr" {
				// Continue while idx >= 0
				ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: ">=", LHS: idxVal, RHS: hir.ConstInt{Text: "0", Type: "i64"}, Type: "i1"})
			} else {
				// Continue while idx < len
				ls.b.Emit(&hir.BinaryOp{Dst: condTemp, Op: "<", LHS: idxVal, RHS: lenTemp, Type: "i1"})
			}
			ls.b.SetBlock(oldCur)

			// Create body block
			bodyBlk := ls.b.NewBlock("reduce_body")
			ls.b.SetBlock(bodyBlk)

			// Load current index
			idxBody := ls.b.FreshTemp("reduce_idx_body")
			ls.b.Emit(&hir.Load{Type: "i64", Src: idxPtr, Dst: idxBody})

			// Get element: list_get(iter, idx)
			idxI32 := ls.b.FreshTemp("reduce_idx_i32")
			ls.b.Emit(&hir.Cast{Dst: idxI32, Src: idxBody, Type: "i32"})

			elemPtr := ls.b.FreshTemp("reduce_elem_ptr")
			ls.b.Emit(&hir.Call{Dst: elemPtr, Fn: "list_get", Args: []hir.Value{iterVal, idxI32}, Type: "ptr"})

			// Cast to element type (assume i32 for now, could be improved with type info)
			elemVal := ls.b.FreshTemp("reduce_elem")
			ls.b.Emit(&hir.Cast{Dst: elemVal, Src: elemPtr, Type: "i32"})

			// Load current accumulator
			currAcc := ls.b.FreshTemp("curr_acc")
			ls.b.Emit(&hir.Load{Type: "i32", Src: accPtr, Dst: currAcc})

			// Call the function
			// For reduce/foldl: f(acc, elem) - left fold
			// For foldr: f(elem, acc) - right fold (argument order swapped)
			newAcc := ls.b.FreshTemp("new_acc")
			if calleeName == "foldr" {
				ls.b.Emit(&hir.Call{Dst: newAcc, Fn: funcName, Args: []hir.Value{elemVal, currAcc}, Type: "i32"})
			} else {
				ls.b.Emit(&hir.Call{Dst: newAcc, Fn: funcName, Args: []hir.Value{currAcc, elemVal}, Type: "i32"})
			}

			// Store new accumulator
			ls.b.Emit(&hir.Store{Dst: accPtr, Val: newAcc})

			// Update index
			incTemp := ls.b.FreshTemp("reduce_inc")
			if calleeName == "foldr" {
				// Decrement for right-to-left
				ls.b.Emit(&hir.BinaryOp{Dst: incTemp, Op: "-", LHS: idxBody, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
			} else {
				// Increment for left-to-right
				ls.b.Emit(&hir.BinaryOp{Dst: incTemp, Op: "+", LHS: idxBody, RHS: hir.ConstInt{Text: "1", Type: "i64"}, Type: "i64"})
			}
			ls.b.Emit(&hir.Store{Dst: idxPtr, Val: incTemp})

			// Emit while loop
			ls.b.SetBlock(oldCur)
			ls.b.Emit(&hir.While{Cond: condTemp, CondBlock: condBlk, Body: bodyBlk})

			// Load final accumulator value
			result := ls.b.FreshTemp("reduce_result")
			ls.b.Emit(&hir.Load{Type: "i32", Src: accPtr, Dst: result})

			return result
		}
	}

	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee, x)
		if calleeName == "open" && len(x.Args) == 2 {
			return ls.lowerFileOpen(x.Args)
		}
	}

	// 1.7. assert() builtin - testing/debugging
	// assert(condition) or assert(condition, message)
	// Calls __assert_check which aborts if condition is false
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee, x)
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

		// assert_eq(expected, actual) / assert_eq(expected, actual, msg)
		if calleeName == "assert_eq" && len(x.Args) >= 2 && len(x.Args) <= 3 {
			expectedVal := ls.lowerExpr(x.Args[0])
			actualVal := ls.lowerExpr(x.Args[1])

			// Build context message
			line := 0
			if x.Callee != nil {
				line = x.Callee.SpanOf().Start.Line
			}
			contextMsg := fmt.Sprintf("line %d: assert_eq failed", line)
			if len(x.Args) == 3 {
				if strLit, ok := x.Args[2].(*ast.StrLit); ok {
					contextMsg = fmt.Sprintf("line %d: %s", line, strLit.Value)
				}
			}
			contextVal := hir.ConstStr{Text: contextMsg}

			// Determine type and call appropriate runtime function
			fnName := "__desi_assert_eq_int" // default
			if ls.info != nil {
				if t := ls.info.Types[x.Args[0]]; t != nil {
					switch t {
					case types.Str:
						fnName = "__desi_assert_eq_str"
					case types.Bool:
						fnName = "__desi_assert_eq_bool"
					default:
						fnName = "__desi_assert_eq_int"
					}
				}
			}
			if fnName == "__desi_assert_eq_bool" {
				// C signature takes int: zext i1 args to i32 — on Windows x64
				// a raw i1 leaves the upper register bits undefined and the
				// int comparison fails even for equal booleans.
				e32 := ls.b.FreshTemp("assert_b32")
				ls.b.Emit(&hir.Cast{Dst: e32, Src: expectedVal, Type: "i32"})
				expectedVal = e32
				a32 := ls.b.FreshTemp("assert_b32")
				ls.b.Emit(&hir.Cast{Dst: a32, Src: actualVal, Type: "i32"})
				actualVal = a32
			}
			ls.b.Emit(&hir.Call{Fn: fnName, Args: []hir.Value{expectedVal, actualVal, contextVal}})
			return nil
		}

		// assert_ne(a, b) / assert_ne(a, b, msg)
		if calleeName == "assert_ne" && len(x.Args) >= 2 && len(x.Args) <= 3 {
			aVal := ls.lowerExpr(x.Args[0])
			bVal := ls.lowerExpr(x.Args[1])

			line := 0
			if x.Callee != nil {
				line = x.Callee.SpanOf().Start.Line
			}
			contextMsg := fmt.Sprintf("line %d: assert_ne failed", line)
			if len(x.Args) == 3 {
				if strLit, ok := x.Args[2].(*ast.StrLit); ok {
					contextMsg = fmt.Sprintf("line %d: %s", line, strLit.Value)
				}
			}
			contextVal := hir.ConstStr{Text: contextMsg}

			fnName := "__desi_assert_ne_int" // default
			if ls.info != nil {
				if t := ls.info.Types[x.Args[0]]; t != nil {
					switch t {
					case types.Str:
						fnName = "__desi_assert_ne_str"
					case types.Bool:
						fnName = "__desi_assert_ne_bool"
					default:
						fnName = "__desi_assert_ne_int"
					}
				}
			}
			if fnName == "__desi_assert_ne_bool" {
				// Same i1→i32 ABI requirement as assert_eq_bool above
				a32 := ls.b.FreshTemp("assert_b32")
				ls.b.Emit(&hir.Cast{Dst: a32, Src: aVal, Type: "i32"})
				aVal = a32
				b32 := ls.b.FreshTemp("assert_b32")
				ls.b.Emit(&hir.Cast{Dst: b32, Src: bVal, Type: "i32"})
				bVal = b32
			}
			ls.b.Emit(&hir.Call{Fn: fnName, Args: []hir.Value{aVal, bVal, contextVal}})
			return nil
		}
	}

	// 1.8. set_recursion_limit(n) - configure maximum recursion depth
	// Like Python's sys.setrecursionlimit(n)
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee, x)
		if calleeName == "set_recursion_limit" && len(x.Args) == 1 {
			// Lower the limit value
			limitVal := ls.lowerExpr(x.Args[0])

			// Cast to i64 for the C function
			limit64 := ls.b.FreshTemp("limit_i64")
			ls.b.Emit(&hir.Cast{Dst: limit64, Src: limitVal, Type: "i64"})

			// Call runtime helper: __desi_set_max_recursion(limit)
			ls.b.Emit(&hir.Call{Fn: "__desi_set_max_recursion", Args: []hir.Value{limit64}})

			// Return none/void
			return nil
		}
	}

	// 1.9. Hot reload state serialization builtins
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee, x)

		// is_reload() -> bool
		if calleeName == "is_reload" && len(x.Args) == 0 {
			dst := ls.b.FreshTemp("is_reload")
			ls.b.Emit(&hir.Call{Dst: dst, Fn: "__desi_is_reload", Args: nil, Type: "i1"})
			return dst
		}

		// reload_count() -> int
		if calleeName == "reload_count" && len(x.Args) == 0 {
			dst := ls.b.FreshTemp("reload_count")
			ls.b.Emit(&hir.Call{Dst: dst, Fn: "__desi_reload_count", Args: nil, Type: "i64"})
			return dst
		}

		// state_file() -> Option[str]
		if calleeName == "state_file" && len(x.Args) == 0 {
			// Call C function that returns ptr (NULL if not set)
			rawPtr := ls.b.FreshTemp("state_file_raw")
			ls.b.Emit(&hir.Call{Dst: rawPtr, Fn: "__desi_state_file", Args: nil, Type: "ptr"})
			return ls.wrapPtrInOption(rawPtr, "state_file")
		}

		// write_state(json: str) -> bool
		if calleeName == "write_state" && len(x.Args) == 1 {
			jsonVal := ls.lowerExpr(x.Args[0])
			dst := ls.b.FreshTemp("write_state")
			ls.b.Emit(&hir.Call{Dst: dst, Fn: "__desi_write_state", Args: []hir.Value{jsonVal}, Type: "i1"})
			return dst
		}

		// read_state() -> Option[str]
		if calleeName == "read_state" && len(x.Args) == 0 {
			rawPtr := ls.b.FreshTemp("read_state_raw")
			ls.b.Emit(&hir.Call{Dst: rawPtr, Fn: "__desi_read_state", Args: nil, Type: "ptr"})
			return ls.wrapPtrInOption(rawPtr, "read_state")
		}

		// delete_state() -> bool
		if calleeName == "delete_state" && len(x.Args) == 0 {
			dst := ls.b.FreshTemp("delete_state")
			ls.b.Emit(&hir.Call{Dst: dst, Fn: "__desi_delete_state", Args: nil, Type: "i1"})
			return dst
		}
	}

	// Handle module.method() calls
	// Note: json.* and math.* calls are handled through normal module import system
	// (json.desi/math __mod.desi → @extern("C") → C runtime)
	// log.* calls are also handled through normal module import system (log.desi → __log_* C runtime)
	if fe, ok := x.Callee.(*ast.FieldExpr); ok {
		_ = fe // module calls now go through standard path
	}

	// 2. M14 Stage 3: print(Display) + Auto to_str for collections
	// If we have type info, check if this is print(arg...) where:
	// - arg implements Display trait, OR
	// - arg is a collection (list/dict/set) with to_str method, OR
	// - arg is a custom class with to_str method
	// Supports multiple arguments - prints each separated by space, ending with newline
	// Supports keyword args: sep (default " "), end (default "\n")
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee, x)
		if calleeName == "print" {
			// Default sep and end as HIR values
			var sepHIR hir.Value = hir.ConstStr{Text: " "}
			var endHIR hir.Value = hir.ConstStr{Text: "\n"}
			flushOutput := false   // flush=True forces output buffer flush
			fileStreamID := 1      // 1=stdout, 2=stderr, 3=user file
			var fileExpr ast.Expr  // For file= with user file variable
			var styleHIR hir.Value // For style= colored output

			// Collect positional arguments and extract kwargs
			var argExprs []ast.Expr
			if len(x.ArgNodes) > 0 {
				for _, an := range x.ArgNodes {
					if an.Name != nil {
						// Keyword argument
						kwName := an.Name.Name
						if kwName == "sep" {
							// Lower sep expression to get HIR value
							sepHIR = ls.lowerExpr(an.Expr)
						} else if kwName == "end" {
							// Lower end expression to get HIR value
							endHIR = ls.lowerExpr(an.Expr)
						} else if kwName == "flush" {
							// Check for boolean true literal
							if boolLit, ok := an.Expr.(*ast.BoolLit); ok && boolLit.Value {
								flushOutput = true
							}
						} else if kwName == "file" {
							// Detect sys.stdout/sys.stderr from AST
							// Look for FieldExpr with "sys" as X (the object)
							if fe, ok := an.Expr.(*ast.FieldExpr); ok {
								if id, ok := fe.X.(*ast.Ident); ok && id.Name == "sys" {
									switch fe.Name.Name {
									case "stdout":
										fileStreamID = 1
									case "stderr":
										fileStreamID = 2
									default:
										// Unknown sys.* field - treat as file variable
										fileStreamID = 3 // 3 = user file
									}
								} else {
									// Not sys.*, treat as user file
									fileStreamID = 3
								}
							} else if id, ok := an.Expr.(*ast.Ident); ok {
								// Ident - could be 'stdout'/'stderr' directly or a file variable
								switch id.Name {
								case "stdout":
									fileStreamID = 1
								case "stderr":
									fileStreamID = 2
								default:
									// Regular variable - it's a file
									fileStreamID = 3
								}
							} else {
								// Other expression (function call, etc) - treat as file
								fileStreamID = 3
							}
							// Store file expression for non-magic cases
							if fileStreamID == 3 {
								fileExpr = an.Expr
							}
						} else if kwName == "style" {
							// Lower style expression for colored output
							styleHIR = ls.lowerExpr(an.Expr)
						}
						// Skip kwargs from positional args list
					} else {
						argExprs = append(argExprs, an.Expr)
					}
				}
			} else {
				argExprs = x.Args
			}

			// Get stream pointer if not stdout
			var streamHIR hir.Value = nil
			if fileStreamID == 2 {
				// Get stderr stream pointer
				streamHIR = ls.b.FreshTemp("stream")
				ls.b.Emit(&hir.Call{Dst: streamHIR.(hir.Temp), Fn: "__get_stderr", Args: []hir.Value{}, Type: "ptr"})
			} else if fileStreamID == 3 && fileExpr != nil {
				// User file - lower expression and convert to stream
				fileVal := ls.lowerExpr(fileExpr)
				streamHIR = ls.b.FreshTemp("stream")
				ls.b.Emit(&hir.Call{Dst: streamHIR.(hir.Temp), Fn: "desifile_get_stream", Args: []hir.Value{fileVal}, Type: "ptr"})
			}

			if len(argExprs) == 0 {
				// print() with no args - just print end (default newline)
				ls.printLock()
				if styleHIR != nil {
					styleTmp := ls.b.FreshTemp("style_start")
					ls.b.Emit(&hir.Call{Dst: styleTmp, Fn: "print_style_start_stdout", Args: []hir.Value{styleHIR}})
				}
				dst := ls.b.FreshTemp("print")
				if streamHIR != nil {
					// Use stream-aware print
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "stream_print_str", Args: []hir.Value{streamHIR, endHIR}})
				} else if fileStreamID != 1 {
					// Non-stdout: use stream-aware print with stream ID (legacy path)
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "print_raw_stream", Args: []hir.Value{&hir.ConstInt{Text: fmt.Sprintf("%d", fileStreamID)}, endHIR}})
				} else {
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "print_raw", Args: []hir.Value{endHIR}})
				}
				if flushOutput {
					flushTemp := ls.b.FreshTemp("flush")
					if streamHIR != nil {
						ls.b.Emit(&hir.Call{Dst: flushTemp, Fn: "stream_flush", Args: []hir.Value{streamHIR}})
					} else if fileStreamID != 1 {
						ls.b.Emit(&hir.Call{Dst: flushTemp, Fn: "fflush_stream", Args: []hir.Value{&hir.ConstInt{Text: fmt.Sprintf("%d", fileStreamID)}}})
					} else {
						ls.b.Emit(&hir.Call{Dst: flushTemp, Fn: "fflush_stdout", Args: []hir.Value{}})
					}
				}
				ls.printUnlock()
				return dst
			}

			// Pass 1: evaluate every argument, and run any to_str for it, before
			// a single byte is written. The lock taken afterwards must not span
			// argument evaluation — an argument is arbitrary user code and may
			// block (print(ch.recv())), and holding the print lock across that
			// would stall every other task's print behind it. A to_str dunder
			// may print on its own account, which is fine out here: it produces
			// its own lines rather than being spliced into this one.
			printVals := make([]hir.Value, 0, len(argExprs))
			for _, arg := range argExprs {
				argT := ls.info.Types[arg]
				if argT == nil {
					// No type information — print the value however it lowers.
					printVals = append(printVals, ls.lowerExpr(arg))
					continue
				}
				typeName := argT.String()
				shouldCallToStr := false
				toStrFuncName := ""
				defaultReprTypeName := "" // non-empty when using __desi_default_repr

				// Case 1: Display trait implementation (user-defined, not auto-generated)
				if impls, ok := ls.info.Impls[typeName]; ok {
					if methods, hasDisplay := impls["Display"]; hasDisplay {
						// Check if the Display impl has a real body (user-defined)
						// or is auto-generated (Body == nil from ensureDefaultDisplay)
						for _, m := range methods {
							if m.Body != nil {
								shouldCallToStr = true
								toStrFuncName = fmt.Sprintf("%s_to_str", typeName)
								break
							}
						}
						// If auto-generated (no real body), fall through to Case 2/3/4
					}
				}

				// A decimal is a pointer to an mpd_t, so print_item had nothing
				// it could do with it and the value came out as an empty line.
				// Render it the same way str() does.
				if !shouldCallToStr && types.Equal(argT, types.Decimal) {
					shouldCallToStr = true
					toStrFuncName = "__decimal_to_str"
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
						} else {
							// Case 4: No custom repr — use default "<ClassName at 0xADDR>"
							shouldCallToStr = true
							toStrFuncName = "__desi_default_repr"
							defaultReprTypeName = t.Name
						}
					case *types.Struct:
						// Structs can implement Display trait - handled in Case 1 above
						if !shouldCallToStr {
							// No Display trait — use default "<StructName at 0xADDR>"
							shouldCallToStr = true
							toStrFuncName = "__desi_default_repr"
							defaultReprTypeName = t.Name
						}
					}
				}

				// Lower argument
				argVal := ls.lowerExpr(arg)

				// If custom to_str, call it first
				if shouldCallToStr {
					strTemp := ls.b.FreshTemp("str")
					if defaultReprTypeName != "" {
						// __desi_default_repr(obj, type_name) → "<TypeName at 0xADDR>"
						ls.b.Emit(&hir.Call{Dst: strTemp, Fn: toStrFuncName, Args: []hir.Value{argVal, hir.ConstStr{Text: defaultReprTypeName}}, Type: "ptr"})
					} else {
						ls.b.Emit(&hir.Call{Dst: strTemp, Fn: toStrFuncName, Args: []hir.Value{argVal}, Type: "ptr"})
					}
					// Runtime to_str helpers malloc their result — track it so
					// the printed temp is freed at scope end. User dunders
					// (__str__/__repr__/to_str) may return literals: not tracked.
					switch toStrFuncName {
					case "list_to_str", "dict_to_str", "set_to_str", "__desi_default_repr", "__decimal_to_str":
						ls.addTempDrop(strTemp.Name)
					}
					argVal = strTemp
				}
				printVals = append(printVals, argVal)
			}

			// Pass 2: everything from here is output, and goes out as one
			// uninterrupted line.
			ls.printLock()

			// If style is set, emit style start
			if styleHIR != nil {
				styleTmp := ls.b.FreshTemp("style_start")
				ls.b.Emit(&hir.Call{Dst: styleTmp, Fn: "print_style_start_stdout", Args: []hir.Value{styleHIR}})
			}

			var lastPrint hir.Value
			for i, v := range printVals {
				dst := ls.b.FreshTemp("print")
				if streamHIR != nil {
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "stream_print_str", Args: []hir.Value{streamHIR, v}})
				} else {
					ls.b.Emit(&hir.Call{Dst: dst, Fn: "print_item", Args: []hir.Value{v}})
				}
				lastPrint = dst
				if i < len(printVals)-1 {
					// Print separator (customizable via sep=)
					sepTemp := ls.b.FreshTemp("sep")
					if streamHIR != nil {
						ls.b.Emit(&hir.Call{Dst: sepTemp, Fn: "stream_print_str", Args: []hir.Value{streamHIR, sepHIR}})
					} else {
						ls.b.Emit(&hir.Call{Dst: sepTemp, Fn: "print_raw", Args: []hir.Value{sepHIR}})
					}
				}
			}

			// Emit style end BEFORE the terminator to prevent color bleed
			if styleHIR != nil {
				styleEndTmp := ls.b.FreshTemp("style_end")
				ls.b.Emit(&hir.Call{Dst: styleEndTmp, Fn: "print_style_end_stdout", Args: []hir.Value{}})
			}

			// Print end terminator (customizable via end=)
			endTemp := ls.b.FreshTemp("end")
			if streamHIR != nil {
				ls.b.Emit(&hir.Call{Dst: endTemp, Fn: "stream_print_str", Args: []hir.Value{streamHIR, endHIR}})
			} else {
				ls.b.Emit(&hir.Call{Dst: endTemp, Fn: "print_raw", Args: []hir.Value{endHIR}})
			}

			if flushOutput {
				flushTemp := ls.b.FreshTemp("flush")
				if streamHIR != nil {
					ls.b.Emit(&hir.Call{Dst: flushTemp, Fn: "stream_flush", Args: []hir.Value{streamHIR}})
				} else {
					ls.b.Emit(&hir.Call{Dst: flushTemp, Fn: "fflush_stdout", Args: []hir.Value{}})
				}
			}

			ls.printUnlock()
			return lastPrint
		}
	}

	// ============================================================
	// Q() — Django-style Q objects for complex lookups
	// Q(status="active") → __q_new("status", "active")
	// Q(age__gt="18", name="Ali") → __q_and(__q_new("age__gt","18"), __q_new("name","Ali"))
	// ============================================================
	if id, ok := x.Callee.(*ast.Ident); ok && id.Name == "Q" && ls.info != nil {
		var result hir.Value
		for _, an := range x.ArgNodes {
			if an.Name != nil {
				lookup := an.Name.Name
				valHIR := ls.lowerExpr(an.Expr)
				dst := ls.b.FreshTemp("q_cond")
				ls.b.Emit(&hir.Call{
					Dst:  dst,
					Fn:   "__q_new",
					Args: []hir.Value{hir.ConstStr{Text: lookup}, valHIR},
					Type: "ptr",
				})
				if result == nil {
					result = dst
				} else {
					// Combine multiple kwargs with AND
					combined := ls.b.FreshTemp("q_and")
					ls.b.Emit(&hir.Call{
						Dst:  combined,
						Fn:   "__q_and",
						Args: []hir.Value{result, dst},
						Type: "ptr",
					})
					result = combined
				}
			}
		}
		if result == nil {
			result = hir.ConstStr{Text: ""}
		}
		return result
	}

	// ============================================================
	// F() — Django-style F expression for field references
	// F("price") → __f_ref("price")
	// ============================================================
	if id, ok := x.Callee.(*ast.Ident); ok && id.Name == "F" && ls.info != nil {
		if len(x.Args) == 1 {
			colVal := ls.lowerExpr(x.Args[0])
			dst := ls.b.FreshTemp("f_ref")
			ls.b.Emit(&hir.Call{
				Dst:  dst,
				Fn:   "__f_ref",
				Args: []hir.Value{colVal},
				Type: "ptr",
			})
			return dst
		}
	}

	// ============================================================
	// Model.objects Manager with Method Chaining Support
	// Handles both direct calls:   User.objects.filter(name="Ali")
	// and chained calls:           User.objects.filter(age__gt="18").order_by("-name").first()
	//
	// AST for chained: CallExpr(.first, CallExpr(.order_by, CallExpr(.filter, FieldExpr(.objects, User))))
	//
	// GENERIC DISPATCH: The lowerer reads MethodSpec metadata from the macro
	// protocol to determine how to emit calls. No ORM-specific switch statements.
	// ============================================================
	if fe, ok := x.Callee.(*ast.FieldExpr); ok && ls.info != nil {
		if tableName, cls, qsHandle, ok := ls.resolveModelObjectsChain(fe.X, x); ok {
			methodName := fe.Name.Name

			// Look up method spec from the macro protocol
			if cls != nil && cls.MacroDecorator != "" {
				if _, method, found := macro.Registry.LookupMethodOnClass(cls, methodName); found {
					return ls.emitQsGeneric(method, x, cls, qsHandle)
				}
			}

			// Fallback: method not found in protocol, emit as terminal fetch
			_ = tableName
			dst := ls.b.FreshTemp("qs_fetch")
			ls.b.Emit(&hir.Call{Dst: dst, Fn: "__qs_fetch", Args: nil, Type: "i32"})
			// Free the handle after terminal call
			if qsHandle != nil {
				ls.b.Emit(&hir.Call{Fn: "__qs_handle_free", Args: []hir.Value{qsHandle}})
			}
			return dst
		}
	}

	// 2. M14 Stage 1: Method Calls (obj.method())
	if fe, ok := x.Callee.(*ast.FieldExpr); ok && ls.info != nil {
		// Check if this is a nested class constructor call (e.g. Container.Box())
		if t := ls.info.Types[fe]; t != nil {
			if _, ok := t.(*types.Class); ok {
				goto skipMethodCall
			}
		}

		// Check if this is a method call
		// We need the type of the receiver (fe.X)
		if recvT := ls.info.Types[fe.X]; recvT != nil {
			typeName := recvT.String()
			methodName := fe.Name.Name

			// Handle Class Methods (including generic class instances like Box[int])
			var cls *types.Class
			var genericArgs []types.T // Track generic args for monomorphized naming
			if c, ok := recvT.(*types.Class); ok {
				cls = c
			} else if gen, ok := recvT.(*types.Generic); ok {
				// Generic class instance: extract base class and args
				if c, ok := gen.Base.(*types.Class); ok {
					cls = c
					genericArgs = gen.Args
				}
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

				// Use monomorphized name for generic class instances
				var mangledName string
				if len(genericArgs) > 0 {
					// Generic instantiation: use Box_int_get instead of Box_get
					mangledName = fmt.Sprintf("%s_%s", mangleGenericClassName(definingClass.Name, genericArgs), methodName)
				} else {
					// Non-generic class: use Box_get (mangleGenericClassName handles nested class dots)
					mangledName = fmt.Sprintf("%s_%s", mangleGenericClassName(definingClass.Name, nil), methodName)
				}

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
					// For generic class instances, substitute type parameters
					if len(genericArgs) > 0 {
						// Build substitution map
						subst := make(map[string]types.T)
						for i, tp := range cls.TypeParams {
							if i < len(genericArgs) {
								subst[tp.Name] = genericArgs[i]
							}
						}
						concreteRet := substituteType(targetMethod.Ret, subst)
						retType = lowerType(concreteRet)
					} else {
						retType = lowerType(targetMethod.Ret)
					}
				}

				// WORKAROUND: Fix incorrect return type for Duration factory methods
				// The type checker seems to resolve the return type as "none"/void for static methods
				// that return the enclosing class type (recursive reference issue).
				// We force "ptr" return type for known factory methods.
				if cls.Name == "Duration" && strings.HasPrefix(methodName, "from_") && (retType == "void" || retType == "") {
					retType = "ptr"
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
					mangledName := fmt.Sprintf("%s_%s", mangleGenericClassName(typeName, nil), methodName)

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

skipMethodCall:

	// 3. M14: Struct Instantiation
	// Check if callee is a Type (Struct or Generic Instance)
	// We rely on the fact that check_inst resolves the call type to the struct type.
	if ls.info != nil {
		resT := ls.info.Types[x]
		var st *types.Struct
		var cls *types.Class
		var genericArgsForCtor []types.T // Track generic args for monomorphized constructor

		if s, ok := resT.(*types.Struct); ok {
			st = s
		} else if g, ok := resT.(*types.Generic); ok {
			if s, ok := g.Base.(*types.Struct); ok {
				st = s
			} else if c, ok := g.Base.(*types.Class); ok {
				cls = c
				genericArgsForCtor = g.Args // Save generic args for constructor name
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
			callee := ls.calleeName(x.Callee, x)
			if callee == cls.Name {
				isConstructor = true
			}

			if isConstructor {
				// Special case: inside __new__ method, ClassName(field=val) should initialize self
				// rather than allocating a new instance (which would cause infinite recursion)
				if ls.inDunderNew && cls.Name == ls.dunderNewClass {
					// Initialize self's fields directly
					for i, arg := range x.Args {
						argVal := ls.lowerExpr(arg)
						// Field stores the raw pointer — temp ownership
						// transfers to the instance.
						ls.consumeTemp(argVal)
						if i < len(cls.Fields) {
							offset := 0
							for j := 0; j < i; j++ {
								offset += getSize(cls.Fields[j].Type)
							}
							fieldPtr := ls.b.FreshTemp("field_ptr")
							// Use GEP to get field pointer at offset
							ls.b.Emit(&hir.GetElementPtr{
								Type:    "i8",
								Base:    ls.dunderNewSelf,
								Indices: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", offset)}},
								Dst:     fieldPtr,
							})
							// Store the value
							ls.b.Emit(&hir.Store{Dst: fieldPtr, Val: argVal})
						}
					}
					// __new__ is void - doesn't return a value
					// The caller (wrapper) has already allocated and will return the instance
					return nil
				}

				// Normal case: allocate new instance
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
				// Use heap allocation (malloc) for classes since they may escape
				// the current scope (e.g., returned from functions like __copy__)
				ls.emitAlloc(inst, hir.ConstInt{Text: fmt.Sprintf("%d", size), Type: "i32"})

				// Zero it before __new__ runs. Storage came back holding whatever
				// the allocator had, and a field the constructor does not assign
				// kept it: `Point()` printed garbage for x and y, and the
				// generated default constructor assigns nothing at all. __new__
				// still overwrites whatever it does set, so this only decides
				// what an unset field reads as — 0, or null for a pointer field,
				// which is what the drop paths null-check before freeing.
				//
				// Unless the checker proved every __new__ on this class writes
				// every field, in which case the memset is dead work: the
				// constructor overwrites all of it before anything can read it.
				if !ls.classFullyInit(cls.Name) {
					ls.b.Emit(&hir.Call{
						Fn:   "__desi_zero",
						Args: []hir.Value{inst, hir.ConstInt{Text: fmt.Sprintf("%d", size), Type: "i32"}},
						Type: "void",
					})
				}

				// 2. Call __new__
				// Find __new__
				// It should be in cls.Constructors or Dunders["__new__"]
				// We assume the checker validated arguments.
				// Mangled name: ClassName___new__ or MangledClassName___new__
				var ctorName string
				if len(genericArgsForCtor) > 0 {
					// Use monomorphized constructor: Box_int___new__
					ctorName = fmt.Sprintf("%s___new__", mangleGenericClassName(cls.Name, genericArgsForCtor))
				} else {
					// Use regular constructor: Box___new__ (mangleGenericClassName handles nested class dots)
					ctorName = fmt.Sprintf("%s___new__", mangleGenericClassName(cls.Name, nil))
				}

				// Prepare args: [inst, user_args...]
				var args []hir.Value
				args = append(args, inst)
				for _, a := range x.Args {
					av := ls.lowerExpr(a)
					// __new__ stores args into instance fields — temp
					// ownership transfers to the instance.
					ls.consumeTemp(av)
					args = append(args, av)
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
					// Field stores the raw pointer — temp ownership
					// transfers to the struct.
					ls.consumeTemp(val)

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
	callee := ls.calleeName(x.Callee, x)
	var args []hir.Value

	// M14: Resolve FuncDecl for default filling and named arg reordering.
	//
	// The checker's choice wins where it recorded one. Falling straight to
	// Cands[0] took the first declaration sharing the name, so an overloaded
	// call filled its omitted arguments from a different overload's defaults:
	// f(1.0) against `f(a: float, b: float = 1.0)` passed the int overload's
	// `b: int = 1`.
	var decl *ast.FuncDecl
	if ls.info != nil {
		if chosen, ok := ls.info.ChosenOverloads[x]; ok && chosen != nil {
			decl = chosen.Decl
			if decl == nil {
				decl = chosen.ModuleDecl
			}
		}
		if decl == nil {
			if id, ok := x.Callee.(*ast.Ident); ok {
				if set, ok := ls.info.Funcs[id.Name]; ok && len(set.Cands) > 0 {
					decl = set.Cands[0].Decl
				}
			} else if fe, ok := x.Callee.(*ast.FieldExpr); ok {
				// Module-qualified: mod.func
				qualName := ""
				if base, ok := fe.X.(*ast.Ident); ok {
					qualName = base.Name + "." + fe.Name.Name
				}
				if qualName != "" {
					if set, ok := ls.info.Funcs[qualName]; ok && len(set.Cands) > 0 {
						decl = set.Cands[0].Decl
					}
				}
			}
		}
	}

	// Check if this call has any named arguments that need reordering.
	hasNamedArgs := false
	if len(x.ArgNodes) > 0 {
		for _, an := range x.ArgNodes {
			if an.Name != nil {
				hasNamedArgs = true
				break
			}
		}
	}

	if hasNamedArgs && decl != nil && len(decl.Params) > 0 {
		// Named-arg-aware reordering: build args in parameter order.
		// For each param, check if a named arg matches it; otherwise use
		// the next positional arg or the param's default value.
		nParams := len(decl.Params)
		// Skip **kwargs param if present (it's handled by lowerKwargsCall)
		if nParams > 0 && decl.Params[nParams-1].Kwargs {
			nParams--
		}
		// Skip *args param if present
		if nParams > 0 && decl.Params[nParams-1].Variadic {
			nParams--
		}

		args = make([]hir.Value, nParams)
		filled := make([]bool, nParams)

		// Build a name→index map for params
		paramIdx := make(map[string]int, nParams)
		for i := 0; i < nParams; i++ {
			paramIdx[decl.Params[i].Name.Name] = i
		}

		// First pass: place named args at their correct param positions
		// and collect positional args in order.
		var positionals []ast.Expr
		for _, an := range x.ArgNodes {
			if an.Name != nil {
				if idx, ok := paramIdx[an.Name.Name]; ok {
					args[idx] = ls.lowerCallArgExpr(an.Expr)
					filled[idx] = true
				}
			} else {
				positionals = append(positionals, an.Expr)
			}
		}

		// Second pass: assign positional args to unfilled slots in order
		posIdx := 0
		for i := 0; i < nParams && posIdx < len(positionals); i++ {
			if !filled[i] {
				args[i] = ls.lowerCallArgExpr(positionals[posIdx])
				filled[i] = true
				posIdx++
			}
		}

		// Third pass: fill remaining unfilled slots with defaults
		for i := 0; i < nParams; i++ {
			if !filled[i] && decl.Params[i].Default != nil {
				args[i] = ls.lowerExpr(decl.Params[i].Default)
				filled[i] = true
			}
		}

		// Trim trailing unfilled args (shouldn't happen if defaults are correct)
		// but keep the slice at nParams length for safety
	} else {
		// No named args or no decl — use legacy positional lowering
		for _, a := range x.Args {
			args = append(args, ls.lowerCallArgExpr(a))
		}

		// Fill trailing defaults for omitted positional args
		if decl != nil && len(x.Args) < len(decl.Params) {
			for i := len(x.Args); i < len(decl.Params); i++ {
				if decl.Params[i].Default != nil {
					args = append(args, ls.lowerExpr(decl.Params[i].Default))
				}
			}
		}
	}

	// Append captured values for lambdas with captures
	if ls.info != nil && ls.info.LambdaCaptures != nil {
		if captures, ok := ls.info.LambdaCaptures[callee]; ok {
			for _, capName := range captures {
				args = append(args, hir.Var{Name: capName})
			}
		}
	}

	// M15: Box arguments for generic functions
	// If the callee is a generic function (erased), we need to box primitive arguments to ptr
	if ls.info != nil {
		isGeneric := false
		// A generic enum's constructor takes its payload as a pointer to a box,
		// and copies a fixed eight bytes out of it. That only works if every
		// argument is boxed, pointers included — see the boxing loop below.
		isGenericEnumCtor := false

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
						isGenericEnumCtor = true
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
								isGenericEnumCtor = true
							}
						}
					}
				}
			}
		}

		// Case 3: Generic struct constructor (e.g. Box<int>)
		if _, ok := x.Callee.(*ast.Ident); ok && !isGeneric {
			// Check if the result type is a Generic wrapping a Struct
			if resT := ls.info.Types[x]; resT != nil {
				if g, ok := resT.(*types.Generic); ok {
					if _, ok := g.Base.(*types.Struct); ok {
						isGeneric = true
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

					// A generic call used to be left as the raw pointer it returns,
					// and this treated such an argument as already boxed. The
					// result is unboxed at the call now — see unboxGenericResult
					// — so it has the same shape as any other value of its type,
					// and the type the checker recorded above is the truth. Left
					// in, the special case skipped boxing an argument that was no
					// longer a pointer, and `ident(ident(5))` dereferenced an int.
				}

				// A generic enum constructor boxes everything, a pointer included.
				// Boxing only the primitives left the two cases inconsistent: an
				// int arrived as a pointer to itself while a str arrived as the
				// value, and one constructor cannot store both correctly. The box
				// is eight bytes wide whatever it holds, which is the width the
				// constructor copies out.
				if isGenericEnumCtor {
					if argType != "void" {
						boxPtr := ls.b.FreshTemp("arg_box_ptr")
						ls.b.Emit(&hir.Alloca{Type: "i64", Count: 1, Dst: boxPtr})
						ls.b.Emit(&hir.Store{Dst: boxPtr, Val: arg})
						args[i] = boxPtr
					}
				} else if argType != "ptr" && argType != "void" {
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

	// Fix for class constructor calls (nested classes etc)
	// When the result type is a Class (or Generic instantiation of Class) AND callee matches class name,
	// force ptr return type and use mangled class name
	if ls.info != nil {
		if t := ls.info.Types[x]; t != nil {
			var cls *types.Class
			var genericArgs []types.T

			// Debug

			if c, ok := t.(*types.Class); ok {
				cls = c
			} else if g, ok := t.(*types.Generic); ok {
				if c, ok := g.Base.(*types.Class); ok {
					cls = c
					genericArgs = g.Args
				}
			}

			if cls != nil {
				retType = "ptr"
				// Check if callee IS the constructor:
				// - callee == cls.Name (simple case: "Foo" == "Foo")
				// - callee is last segment of qualified name (nested case: "Inner" in "Container.Inner")
				isConstructorCall := callee == cls.Name
				if !isConstructorCall {
					// For nested classes imported as "Inner" (from Container.Inner)
					// cls.Name is "Container.Inner" but callee is "Inner"
					if parts := strings.Split(cls.Name, "."); len(parts) > 1 {
						lastName := parts[len(parts)-1]
						if callee == lastName {
							isConstructorCall = true
						}
					}
				}
				if isConstructorCall {
					callee = mangleGenericClassName(cls.Name, genericArgs)
				}
			}
		}
	}

	// Fix for Option/Result constructors (built-in enums)
	// These are not always in ls.info.Funcs, so we force ptr return type.
	if strings.HasPrefix(callee, "Option.") || strings.HasPrefix(callee, "Result.") {
		retType = "ptr"
	}

	// Conservative ownership: a collection local passed to a user-defined
	// function (or method) may be retained by the callee — the drop tracker
	// can't see that. Mark it moved so scope exit doesn't free it: leaking
	// is safer than use-after-free. Runtime helpers only borrow, so plain
	// non-user callees don't mark.
	if ls.info != nil {
		userCallee := false
		if id, ok := x.Callee.(*ast.Ident); ok {
			if set, ok := ls.info.Funcs[id.Name]; ok && len(set.Cands) > 0 && set.Cands[0].Decl != nil {
				userCallee = true
			}
		} else if _, ok := x.Callee.(*ast.FieldExpr); ok {
			userCallee = true // method / module-qualified call — may retain via self
		}
		if userCallee {
			for _, argNode := range x.Args {
				if id, ok := argNode.(*ast.Ident); ok {
					if t := ls.info.Types[argNode]; t != nil {
						switch t.(type) {
						case *types.List, *types.Set, *types.Dict, *types.Enum, *types.Struct:
							for i := len(ls.scopes) - 1; i >= 0; i-- {
								ls.scopes[i].moved[id.Name] = true
							}
						}
					}
				}
			}
		}
	}

	isMarshalDumps := callee == "__marshal_dumps"
	if !isMarshalDumps && ls.info != nil {
		if chosen, ok := ls.info.ChosenOverloads[x]; ok && chosen.Type != nil {
			if callee == "__desi$dumps" && types.Equal(chosen.Type.Ret, types.Bytes) {
				isMarshalDumps = true
			}
		}
	}
	if isMarshalDumps && ls.info != nil && len(x.Args) == 1 {
		argType := ls.typeOf(x.Args[0])
		if argType == nil {
			argType = types.Any
		}
		valExpr := x.Args[0]
		argVal := ls.lowerExpr(valExpr)

		var passVal hir.Value
		if types.Equal(argType, types.Int) || types.Equal(argType, types.Float) || types.Equal(argType, types.Bool) || argType.String() == "char" {
			tempSlot := ls.b.FreshTemp("marshal_val_slot")
			ls.b.Emit(&hir.Alloca{Type: lowerType(argType), Dst: tempSlot})
			ls.b.Emit(&hir.Store{Val: argVal, Dst: tempSlot})
			passVal = tempSlot
		} else {
			passVal = argVal
		}

		typeInfoVal := ls.emitTypeInfo(argType)
		ls.b.Emit(&hir.Call{Dst: dst, Fn: "__marshal_dumps", Args: []hir.Value{passVal, typeInfoVal}, Type: "ptr"})
		return dst
	}

	isMarshalLoads := callee == "__marshal_loads"
	if !isMarshalLoads && ls.info != nil {
		if chosen, ok := ls.info.ChosenOverloads[x]; ok && chosen.Type != nil {
			if callee == "__desi$loads" && len(chosen.Type.Params) == 1 && types.Equal(chosen.Type.Params[0], types.Bytes) {
				isMarshalLoads = true
			}
		}
	}
	if isMarshalLoads && ls.info != nil && len(x.Args) == 1 {
		dataExpr := x.Args[0]
		dataVal := ls.lowerExpr(dataExpr)

		retTypeObj := ls.typeOf(x)
		if retTypeObj == nil {
			retTypeObj = types.Any
		}
		typeInfoVal := ls.emitTypeInfo(retTypeObj)

		rawRes := ls.b.FreshTemp("marshal_raw_res")
		ls.b.Emit(&hir.Call{Dst: rawRes, Fn: "__marshal_loads", Args: []hir.Value{dataVal, typeInfoVal}, Type: "ptr"})

		if types.Equal(retTypeObj, types.Int) || types.Equal(retTypeObj, types.Float) || types.Equal(retTypeObj, types.Bool) || retTypeObj.String() == "char" {
			ls.b.Emit(&hir.Load{Type: lowerType(retTypeObj), Src: rawRes, Dst: dst})
			ls.b.Emit(&hir.Call{Fn: "free", Args: []hir.Value{rawRes}})
		} else {
			ls.b.Emit(&hir.Cast{Src: rawRes, Dst: dst, Type: retType})
		}
		return dst
	}

	ls.b.Emit(&hir.Call{Dst: dst, Fn: callee, Args: args, Type: retType})

	// Consume args if not a known borrowing function
	// TODO: Use type checker info to determine if callee borrows
	if callee != "print" && callee != "asprintf" && callee != "bool_to_cstring" && !strings.HasPrefix(callee, "arena.") {
		for _, arg := range args {
			ls.consumeTemp(arg)
		}
	}

	// str(int/float/bool) mallocs its result (int_to_str/float_to_str/
	// bool_to_str) — track it like a concat temp so a transient use
	// (print("n: " + str(n)) in a loop) is freed at scope end.
	// str(str) is an identity bitcast of the SAME pointer: never track it.
	if callee == "str" && len(x.Args) == 1 && ls.info != nil {
		argT := ls.info.Types[x.Args[0]]
		if types.Equal(argT, types.Int) || types.Equal(argT, types.Float) || types.Equal(argT, types.Bool) {
			ls.addTempDrop(dst.Name)
		}
	}

	return dst
}

// isPrimitiveType returns true for primitive types that need boxing
// when captured in closure contexts (int, float, bool, char, sized integers)
func isPrimitiveType(t types.T) bool {
	if t == nil {
		return false
	}
	switch t {
	case types.Int, types.Float, types.Bool, types.Char,
		types.I8, types.I16, types.I32, types.I64,
		types.U8, types.U16, types.U32, types.U64,
		types.ISize, types.USize, types.F32, types.F64:
		return true
	}
	return false
}

// ============================================================
// Model.objects Method Chaining Support — GENERIC DISPATCH
//
// The lowerer reads MethodSpec metadata from the macro protocol to
// determine how to process arguments and which C runtime functions
// to call. No ORM-specific switch statements — all behavior is
// defined in the macro protocol (orm_protocol.go).
// ============================================================

// resolveModelObjectsChain walks up the chain of CallExpr/FieldExpr nodes,
// emitting intermediate QuerySet operations. Returns (tableName, class, handle, true)
// if this is a valid Model.objects chain; ("", nil, nil, false) otherwise.
//
// Handle lifecycle (Phase 5):
//
//	At the chain root (User.objects), emits:
//	  %qs = call ptr @__qs_handle_new("users")   → allocate handle
//	  call void @__qs_handle_bind(ptr %qs)         → set as active
//	The handle is returned so the terminal caller can free it.
//
// Handles:
//   - Direct:   FieldExpr(.objects, Ident(User))       → emit handle_new + handle_bind
//   - Chained:  CallExpr(.filter, FieldExpr(.objects, Ident(User)))
//     → emit handle_new + handle_bind + generic intermediate
//
// The caller (emitQsGeneric) emits the final/terminal operation and __qs_handle_free.
func (ls *lowerState) resolveModelObjectsChain(receiver ast.Expr, outerCall *ast.CallExpr) (string, *types.Class, hir.Value, bool) {
	// Case 1: receiver is FieldExpr(.propertyName, Ident(ClassName)) — direct Class.property.method()
	if fe, ok := receiver.(*ast.FieldExpr); ok {
		if recvT := ls.info.Types[fe.X]; recvT != nil {
			if cls, ok := recvT.(*types.Class); ok && cls.MacroDecorator != "" {
				// Check if the field name is a macro-injected property
				if _, _, ok := macro.Registry.LookupProperty(cls, fe.Name.Name); ok {
					// Phase 5: Allocate a new QuerySet handle
					qsHandle := ls.b.FreshTemp("qs_handle")
					ls.b.Emit(&hir.Call{
						Dst:  qsHandle,
						Fn:   "__qs_handle_new",
						Args: []hir.Value{hir.ConstStr{Text: cls.TableName}},
						Type: "ptr",
					})
					// Bind as the active handle for subsequent __qs_* calls
					ls.b.Emit(&hir.Call{
						Fn:   "__qs_handle_bind",
						Args: []hir.Value{qsHandle},
					})
					// If model has db=[...] routing, switch to the named connection
					if len(cls.ModelDB) > 0 {
						useDst := ls.b.FreshTemp("db_use")
						ls.b.Emit(&hir.Call{
							Dst:  useDst,
							Fn:   "__db_use_conn",
							Args: []hir.Value{hir.ConstStr{Text: cls.ModelDB[0]}},
							Type: "i32",
						})
					}
					return cls.TableName, cls, qsHandle, true
				}
			}
		}
		return "", nil, nil, false
	}

	// Case 2: receiver is a CallExpr — chained method like User.objects.filter(...).order_by(...)
	// The receiver is the inner CallExpr (e.g., User.objects.filter(...))
	if innerCall, ok := receiver.(*ast.CallExpr); ok {
		if innerFe, ok := innerCall.Callee.(*ast.FieldExpr); ok {
			// Recursively resolve the chain (walks up to the root User.objects)
			tableName, cls, qsHandle, isChain := ls.resolveModelObjectsChain(innerFe.X, innerCall)
			if !isChain {
				return "", nil, nil, false
			}

			// Emit the intermediate operation for this link in the chain
			// using generic dispatch from the macro protocol
			methodName := innerFe.Name.Name
			if cls != nil && cls.MacroDecorator != "" {
				if _, method, found := macro.Registry.LookupMethodOnClass(cls, methodName); found {
					ls.emitQsArgs(method, innerCall)
				}
			}
			return tableName, cls, qsHandle, true
		}
	}

	return "", nil, nil, false
}

// emitQsArgs processes the arguments for a QuerySet method call
// using the generic dispatch metadata from MethodSpec.
// This handles kwargs, positional args, and Q expressions.
func (ls *lowerState) emitQsArgs(spec *macro.MethodSpec, call *ast.CallExpr) {
	if spec == nil {
		return
	}

	switch spec.ArgStyle {
	case "kwargs_filter":
		// Each kwarg (name=val) → KwargsFunc(name_str, val)
		// Q expression positional args → __qs_filter_q(qval)
		for _, an := range call.ArgNodes {
			if an.Name != nil {
				lookup := an.Name.Name
				valHIR := ls.lowerExpr(an.Expr)
				dst := ls.b.FreshTemp("qs_arg")
				ls.b.Emit(&hir.Call{
					Dst:  dst,
					Fn:   spec.KwargsFunc,
					Args: []hir.Value{hir.ConstStr{Text: lookup}, valHIR},
					Type: "i32",
				})
			} else if ls.isQExpression(an.Expr) {
				qVal := ls.lowerExpr(an.Expr)
				dst := ls.b.FreshTemp("qs_filter_q")
				ls.b.Emit(&hir.Call{
					Dst:  dst,
					Fn:   "__qs_filter_q",
					Args: []hir.Value{qVal},
					Type: "i32",
				})
			}
		}

	case "kwargs_set":
		// Each kwarg (name=val) → KwargsFunc(name_str, val)
		for _, an := range call.ArgNodes {
			if an.Name != nil {
				key := an.Name.Name
				valHIR := ls.lowerExpr(an.Expr)
				dst := ls.b.FreshTemp("qs_arg")
				ls.b.Emit(&hir.Call{
					Dst:  dst,
					Fn:   spec.KwargsFunc,
					Args: []hir.Value{hir.ConstStr{Text: key}, valHIR},
					Type: "i32",
				})
			}
		}

	case "positional":
		// Each positional arg → KwargsFunc(arg)
		if spec.KwargsFunc != "" {
			for _, arg := range call.Args {
				val := ls.lowerExpr(arg)
				dst := ls.b.FreshTemp("qs_arg")
				ls.b.Emit(&hir.Call{
					Dst:  dst,
					Fn:   spec.KwargsFunc,
					Args: []hir.Value{val},
					Type: "i32",
				})
			}
		}

	case "none":
		// No arg processing — used for terminal-only methods like count(), delete()
		// and chainable no-arg methods like distinct(), all()
		if spec.KwargsFunc != "" {
			// Some "none" args still have a func to call (e.g., distinct)
			dst := ls.b.FreshTemp("qs_arg")
			ls.b.Emit(&hir.Call{
				Dst:  dst,
				Fn:   spec.KwargsFunc,
				Args: nil,
				Type: "i32",
			})
		}
	}
}

// emitQsGeneric is the generic dispatch for terminal QuerySet methods.
// It replaces the old 150-line emitQsTerminal switch statement.
//
// The method reads MethodSpec metadata to determine:
//  1. How to process arguments (ArgStyle + KwargsFunc)
//  2. Which C function executes the terminal action (TerminalFunc)
//  3. Whether to construct a model instance from the result (ReturnsModel)
//
// Phase 5: After the terminal call completes and results are extracted,
// the QuerySet handle is freed via __qs_handle_free. This ensures
// deterministic cleanup regardless of how the result is used.
//
// This function is 100% domain-agnostic — it knows nothing about "filter",
// "get", or "create". All behavior comes from the macro protocol definition.
func (ls *lowerState) emitQsGeneric(spec *macro.MethodSpec, call *ast.CallExpr, cls *types.Class, qsHandle hir.Value) hir.Value {
	if spec == nil {
		return nil
	}

	// 1. Process arguments using generic dispatch
	ls.emitQsArgs(spec, call)

	// 2. Call terminal function (if specified)
	if spec.TerminalFunc != "" {
		dst := ls.b.FreshTemp("qs_result")
		ls.b.Emit(&hir.Call{
			Dst:  dst,
			Fn:   spec.TerminalFunc,
			Args: nil,
			Type: "i32",
		})

		// 3. If ReturnsModel, construct class instance from row 0
		var result hir.Value
		if spec.ReturnsModel && cls != nil && len(cls.Fields) > 0 {
			result = ls.constructModelFromRow(cls, dst)
		} else {
			result = dst
		}

		// 4. Free the handle after result extraction (Phase 5)
		if qsHandle != nil {
			ls.b.Emit(&hir.Call{Fn: "__qs_handle_free", Args: []hir.Value{qsHandle}})
		}

		return result
	}

	// Chainable method with no terminal func — return nil (no value produced)
	return nil
}

// constructModelFromRow generates HIR to construct a model class instance
// from the current database result set (row 0).
//
// Generated pseudocode:
//
//	if rowCount > 0:
//	    inst = malloc(sizeof(Class))
//	    for each field in cls.Fields:
//	        raw_str = __db_get_field_by(0, field_name)
//	        typed_val = __db_str_to_<type>(raw_str)
//	        store typed_val at inst + field_offset
//	    return inst
//	else:
//	    return null
//
// The caller is responsible for null-checking the returned pointer.
func (ls *lowerState) constructModelFromRow(cls *types.Class, rowCountVar hir.Value) hir.Value {
	// Check if query returned any rows
	rowCount := ls.b.FreshTemp("db_rows")
	ls.b.Emit(&hir.Call{
		Dst:  rowCount,
		Fn:   "__db_row_count",
		Args: nil,
		Type: "i32",
	})

	// Compare: rowCount > 0
	hasRows := ls.b.FreshTemp("has_rows")
	ls.b.Emit(&hir.BinaryOp{
		Dst: hasRows, Op: ">",
		LHS: rowCount, RHS: hir.ConstInt{Text: "0"},
		Type: "i1",
	})

	// Calculate class instance size
	classSize := getClassSize(cls.Fields)

	// Create the build block — only runs when rows > 0
	buildBlk := ls.b.NewBlock("model_build")
	oldCur := ls.b.Block()

	ls.b.SetBlock(buildBlk)

	// Allocate instance on heap
	inst := ls.b.FreshTemp("model_inst")
	ls.b.Emit(&hir.Call{
		Dst:  inst,
		Fn:   "malloc",
		Args: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", classSize), Type: "i32"}},
		Type: "ptr",
	})

	// Populate each field from the DB result
	offset := 0
	for _, field := range cls.Fields {
		fieldSize := getSize(field.Type)
		fieldAlign := getAlign(field.Type)

		// Align offset
		if fieldAlign > 0 && offset%fieldAlign != 0 {
			offset += fieldAlign - (offset % fieldAlign)
		}

		// Read raw string value from DB: __db_get_field_by(0, "field_name")
		rawStr := ls.b.FreshTemp("db_raw")
		ls.b.Emit(&hir.Call{
			Dst:  rawStr,
			Fn:   "__db_get_field_by",
			Args: []hir.Value{hir.ConstInt{Text: "0"}, hir.ConstStr{Text: field.Name}},
			Type: "ptr",
		})

		// Convert raw string to the field's type
		typedVal := ls.emitTypeConverter(rawStr, field.Type)

		// GEP to field offset
		fieldPtr := ls.b.FreshTemp("field_ptr")
		ls.b.Emit(&hir.GetElementPtr{
			Type:    "i8",
			Base:    inst,
			Indices: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", offset)}},
			Dst:     fieldPtr,
		})

		// Store converted value
		ls.b.Emit(&hir.Store{Dst: fieldPtr, Val: typedVal})

		offset += fieldSize
	}

	ls.b.SetBlock(oldCur)

	// Emit: if hasRows then buildBlk (builds instance) else skip
	ls.b.Emit(&hir.If{Cond: hasRows, Then: buildBlk, Else: nil})

	// Select: result = hasRows ? inst : null
	result := ls.b.FreshTemp("model_result")
	ls.b.Emit(&hir.Select{
		Dst:  result,
		Cond: hasRows,
		Then: inst,
		Else: hir.ConstInt{Text: "0", Type: "ptr"},
		Type: "ptr",
	})

	return result
}

// emitTypeConverter generates HIR to convert a raw DB string (ptr) to the
// appropriate Desi type. Returns the converted value.
//
// Mapping:
//
//	int, i32, i64, etc. → __db_str_to_int(raw)
//	float, f64, etc.    → __db_str_to_float(raw)
//	bool                → __db_str_to_bool(raw)
//	str, ptr            → __db_str_to_str(raw)  (identity)
func (ls *lowerState) emitTypeConverter(rawStr hir.Value, fieldType types.T) hir.Value {
	llvmType := lowerType(fieldType)
	var converterFn string
	var retType string

	switch llvmType {
	case "i64":
		converterFn = "__db_str_to_int"
		retType = "i64"
	case "i32":
		converterFn = "__db_str_to_int"
		retType = "i64" // returns i64, we truncate if needed
	case "double":
		converterFn = "__db_str_to_float"
		retType = "double"
	case "float":
		converterFn = "__db_str_to_float"
		retType = "double"
	case "i1":
		converterFn = "__db_str_to_bool"
		retType = "i32"
	default:
		// str, ptr, or any reference type — identity
		converterFn = "__db_str_to_str"
		retType = "ptr"
	}

	dst := ls.b.FreshTemp("db_conv")
	ls.b.Emit(&hir.Call{
		Dst:  dst,
		Fn:   converterFn,
		Args: []hir.Value{rawStr},
		Type: retType,
	})
	return dst
}

// isQExpression delegates to macro.IsQExpression.
// Checks if an AST expression produces a Q object value.
func (ls *lowerState) isQExpression(expr ast.Expr) bool {
	return macro.IsQExpression(expr, ls.info)
}

// isFExpression delegates to macro.IsFExpression.
// Checks if an AST expression is an F() call or F binary expression.
func isFExpression(expr ast.Expr) bool {
	return macro.IsFExpression(expr, nil)
}

// lowerCallArgExpr lowers a call argument expression, automatically emitting
// hir.FuncRef when the argument is a function identifier (not a local variable
// or function parameter). This allows callback functions to be passed through
// wrapper functions naturally, e.g. http.serve(srv, my_handler) →
// FuncRef{"my_handler"} instead of Var{"my_handler"}.
func (ls *lowerState) lowerCallArgExpr(expr ast.Expr) hir.Value {
	if id, ok := expr.(*ast.Ident); ok && ls.info != nil {
		name := id.Name
		// Lambda aliases: "double" → "__lam$0"
		if ls.info.LambdaAliases != nil {
			if alias, ok := ls.info.LambdaAliases[name]; ok {
				return hir.FuncRef{Name: alias}
			}
		}
		// Known function AND not shadowed by a local variable, parameter, or global
		if _, ok := ls.info.Funcs[name]; ok {
			if !ls.hasLocal(name) && !ls.isParam(name) && !ls.isGlobal(name) {
				return hir.FuncRef{Name: name}
			}
		}
	}
	return ls.lowerExpr(expr)
}

// isParam checks if the given name is a parameter of the current function.
// Uses ls.paramNames which is populated before body lowering begins,
// since ls.b.Func().Params is only populated after lowerBlock returns.
func (ls *lowerState) isParam(name string) bool {
	return ls.paramNames != nil && ls.paramNames[name]
}

// isGlobal checks if the given name is a global variable.
func (ls *lowerState) isGlobal(name string) bool {
	return ls.globals != nil && ls.globals[name]
}

func (ls *lowerState) emitTypeInfo(t types.T) hir.Value {
	typeName := t.String()
	var typeID uint64
	for _, c := range typeName {
		typeID = typeID*31 + uint64(c)
	}
	var typeSize int64 = 8 // Default pointer size
	res := ls.b.FreshTemp("type_info")
	ls.b.Emit(&hir.Call{
		Dst: res,
		Fn:  "__desi_type_new",
		Args: []hir.Value{
			hir.ConstInt{Text: fmt.Sprintf("%d", typeID), Type: "i64"},
			hir.ConstStr{Text: typeName},
			hir.ConstInt{Text: fmt.Sprintf("%d", typeSize), Type: "i64"},
		},
		Type: "ptr",
	})
	return res
}

func (ls *lowerState) wrapPtrInOption(rawPtr hir.Value, prefix string) hir.Value {
	isNotNull := ls.b.FreshTemp(prefix + "_is_not_null")
	ls.b.Emit(&hir.BinaryOp{Op: "!=", LHS: rawPtr, RHS: hir.ConstNull{}, Dst: isNotNull, Type: "i1"})

	resOptSlot := ls.b.FreshTemp(prefix + "_opt_slot")
	ls.b.Emit(&hir.Alloca{Type: "ptr", Dst: resOptSlot})

	curBlock := ls.b.Block()
	someBlock := ls.b.NewBlock(prefix + "_some")
	nothingBlock := ls.b.NewBlock(prefix + "_nothing")

	// Some block: call Option.Some(rawPtr) and store in slot
	ls.b.SetBlock(someBlock)
	someVal := ls.b.FreshTemp(prefix + "_some")
	ls.b.Emit(&hir.Call{Dst: someVal, Fn: "Option.Some", Args: []hir.Value{rawPtr}, Type: "ptr"})
	ls.b.Emit(&hir.Store{Val: someVal, Dst: resOptSlot})

	// Nothing block: call Option.Nothing() and store in slot
	ls.b.SetBlock(nothingBlock)
	nothingVal := ls.b.FreshTemp(prefix + "_nothing")
	ls.b.Emit(&hir.Call{Dst: nothingVal, Fn: "Option.Nothing", Args: []hir.Value{}, Type: "ptr"})
	ls.b.Emit(&hir.Store{Val: nothingVal, Dst: resOptSlot})

	ls.b.SetBlock(curBlock)
	ls.b.Emit(&hir.If{Cond: isNotNull, Then: someBlock, Else: nothingBlock})

	resOpt := ls.b.FreshTemp(prefix + "_opt")
	ls.b.Emit(&hir.Load{Type: "ptr", Src: resOptSlot, Dst: resOpt})
	return resOpt
}
