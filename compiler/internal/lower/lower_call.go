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
					ls.b.Emit(&hir.Call{Dst: boxPtr, Fn: "malloc", Args: []hir.Value{hir.ConstInt{Text: "8", Type: "i64"}}, Type: "ptr"})
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

				case "RwLock":
					// sync.RwLock(value) -> rwlock_new(boxed_value)
					if len(x.Args) != 1 {
						panic("sync.RwLock requires exactly 1 argument")
					}
					argVal := ls.lowerExpr(x.Args[0])
					res := ls.b.FreshTemp("rwlock")
					// Allocate memory for the value (8 bytes for i64/ptr)
					boxPtr := ls.b.FreshTemp("rwlock_box")
					ls.b.Emit(&hir.Call{Dst: boxPtr, Fn: "malloc", Args: []hir.Value{hir.ConstInt{Text: "8", Type: "i64"}}, Type: "ptr"})
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
					// Box the value for the channel
					boxPtr := ls.b.FreshTemp("send_box")
					ls.b.Emit(&hir.Call{Dst: boxPtr, Fn: "malloc", Args: []hir.Value{hir.ConstInt{Text: "8", Type: "i64"}}, Type: "ptr"})
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
					ls.b.Emit(&hir.Call{Dst: boxPtr, Fn: "malloc", Args: []hir.Value{hir.ConstInt{Text: "8", Type: "i64"}}, Type: "ptr"})
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

			// TaskGroup methods: spawn, wait, cancel, is_cancelled
			if _, ok := feXType.(*types.TaskGroup); ok {
				tgVal := ls.lowerExpr(fe.X)
				switch fe.Name.Name {
				case "run":
					// tg.run(fn) -> taskgroup_spawn(tg, fn_ptr, NULL)
					// The argument is a function reference
					if len(x.Args) > 0 {
						// Check if argument is an identifier (function name)
						var fnVal hir.Value
						if id, ok := x.Args[0].(*ast.Ident); ok {
							// It's a function name - emit as FuncRef
							fnVal = hir.FuncRef{Name: id.Name}
						} else {
							// Lower normally (for closures in future)
							fnVal = ls.lowerExpr(x.Args[0])
						}
						// taskgroup_spawn expects (TaskGroup*, fn_ptr, ctx)
						// Pass NULL for context since we're not capturing closures yet
						nullCtx := hir.ConstNull{}
						ls.b.Emit(&hir.Call{
							Fn:   "taskgroup_spawn",
							Args: []hir.Value{tgVal, fnVal, nullCtx},
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
					// Box the value to ptr
					boxPtr := ls.b.FreshTemp("send_box")
					ls.b.Emit(&hir.Call{Dst: boxPtr, Fn: "malloc", Args: []hir.Value{hir.ConstInt{Text: "8", Type: "i64"}}, Type: "ptr"})
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
		calleeName := ls.calleeName(x.Callee)
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

				// Return the original value
				return argVal
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
		}
	}
handlePrint:

	// taskgroup_new() -> TaskGroup*
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee)
		if calleeName == "taskgroup_new" && len(x.Args) == 0 {
			res := ls.b.FreshTemp("taskgroup")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "taskgroup_new", Args: []hir.Value{}, Type: "ptr"})
			return res
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

				// Unwrap TypeAlias to check underlying type
				unwrappedT := argT
				if ta, ok := argT.(*types.TypeAlias); ok {
					unwrappedT = ta.Target
				}

				// Check if it's a string type
				if unwrappedT == types.Str {
					lenFunc = "string_len"
				} else if _, ok := unwrappedT.(*types.List); ok {
					lenFunc = "list_len"
				} else if _, ok := unwrappedT.(*types.Dict); ok {
					lenFunc = "dict_len"
				} else if _, ok := unwrappedT.(*types.Set); ok {
					lenFunc = "set_len"
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

	// 1.6. sum(), min(), max(), any(), all() builtins
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee)

		// TaskGroup() - zero-arg constructor
		if calleeName == "TaskGroup" && len(x.Args) == 0 {
			res := ls.b.FreshTemp("taskgroup")
			ls.b.Emit(&hir.Call{Dst: res, Fn: "taskgroup_new", Args: []hir.Value{}, Type: "ptr"})
			return res
		}

		if len(x.Args) == 1 {
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
				res := ls.b.FreshTemp("sorted_res")
				ls.b.Emit(&hir.Call{Dst: res, Fn: "list_sorted_int", Args: []hir.Value{argVal}, Type: "ptr"})
				return res
			case "mutex_new", "Mutex":
				// mutex_new(value) or Mutex(value) -> DesiMutex*
				// For primitive values, we need to box them (allocate + store)
				res := ls.b.FreshTemp("mutex")
				// Allocate memory for the value (8 bytes for i64/ptr)
				boxPtr := ls.b.FreshTemp("mutex_box")
				ls.b.Emit(&hir.Call{Dst: boxPtr, Fn: "malloc", Args: []hir.Value{hir.ConstInt{Text: "8", Type: "i64"}}, Type: "ptr"})
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
				ls.b.Emit(&hir.Call{Dst: innerPtr, Fn: "malloc", Args: []hir.Value{hir.ConstInt{Text: "8", Type: "i64"}}, Type: "ptr"})
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
				ls.b.Emit(&hir.Call{Dst: innerPtr, Fn: "malloc", Args: []hir.Value{hir.ConstInt{Text: "8", Type: "i64"}}, Type: "ptr"})
				ls.b.Emit(&hir.Store{Dst: innerPtr, Val: argVal})
				ls.b.Emit(&hir.Call{Dst: res, Fn: "__rc_new", Args: []hir.Value{innerPtr}, Type: "ptr"})
				ls.cur().rcLike[res.Name] = true
				return res
			}
		}
	}

	// 1.65. reduce(), foldl(), foldr() builtins
	// reduce(func, iterable, initial) -> accumulated value
	// foldl is alias for reduce (left-to-right)
	// foldr processes right-to-left
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee)
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

	// Handle log.info(), log.warn(), log.error(), log.debug()
	if fe, ok := x.Callee.(*ast.FieldExpr); ok {
		if id, ok := fe.X.(*ast.Ident); ok && id.Name == "log" {
			method := fe.Name.Name
			if method == "info" || method == "warn" || method == "error" || method == "debug" {
				// Get the message argument (if any)
				var msgVal hir.Value = hir.ConstStr{Text: ""}
				if len(x.Args) > 0 {
					msgVal = ls.lowerExpr(x.Args[0])
				}
				// Emit call to log_info/warn/error/debug
				dst := ls.b.FreshTemp("log")
				funcName := "log_" + method
				ls.b.Emit(&hir.Call{Dst: dst, Fn: funcName, Args: []hir.Value{msgVal}, Type: "void"})
				return nil
			}
		}
		// Handle json.* functions
		if id, ok := fe.X.(*ast.Ident); ok && id.Name == "json" {
			method := fe.Name.Name
			if method == "parse" && len(x.Args) >= 1 {
				// json.parse(text) -> __json_parse(text) returns ptr
				textVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("json_node")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_parse", Args: []hir.Value{textVal}, Type: "ptr"})
				return dst
			}
			if method == "stringify" && len(x.Args) >= 1 {
				// json.stringify(node) -> __json_stringify(node) returns ptr (string)
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("json_str")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_stringify", Args: []hir.Value{nodeVal}, Type: "ptr"})
				return dst
			}
			// Type check functions - return i1 (bool)
			if method == "is_null" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("is_null")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_is_null", Args: []hir.Value{nodeVal}, Type: "i1"})
				return dst
			}
			if method == "is_bool" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("is_bool")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_is_bool", Args: []hir.Value{nodeVal}, Type: "i1"})
				return dst
			}
			if method == "is_number" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("is_number")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_is_number", Args: []hir.Value{nodeVal}, Type: "i1"})
				return dst
			}
			if method == "is_string" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("is_string")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_is_string", Args: []hir.Value{nodeVal}, Type: "i1"})
				return dst
			}
			if method == "is_array" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("is_array")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_is_array", Args: []hir.Value{nodeVal}, Type: "i1"})
				return dst
			}
			if method == "is_object" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("is_object")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_is_object", Args: []hir.Value{nodeVal}, Type: "i1"})
				return dst
			}
			if method == "is_int" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("is_int")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_is_int", Args: []hir.Value{nodeVal}, Type: "i1"})
				return dst
			}
			// Value extract functions
			if method == "get_type" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("json_type")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_type", Args: []hir.Value{nodeVal}, Type: "i32"})
				return dst
			}
			if method == "get_bool" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("json_bool")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_get_bool", Args: []hir.Value{nodeVal}, Type: "i32"})
				return dst
			}
			if method == "get_number" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("json_num")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_get_number", Args: []hir.Value{nodeVal}, Type: "double"})
				return dst
			}
			if method == "get_float" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("json_float")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_get_float", Args: []hir.Value{nodeVal}, Type: "double"})
				return dst
			}
			if method == "get_int" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("json_int")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_get_int", Args: []hir.Value{nodeVal}, Type: "i64"})
				return dst
			}
			if method == "get_string" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("json_string")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_get_string", Args: []hir.Value{nodeVal}, Type: "ptr"})
				return dst
			}
			if method == "array_len" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("json_arr_len")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_array_len", Args: []hir.Value{nodeVal}, Type: "i32"})
				return dst
			}
			if method == "object_len" && len(x.Args) >= 1 {
				nodeVal := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("json_obj_len")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_object_len", Args: []hir.Value{nodeVal}, Type: "i32"})
				return dst
			}
			if method == "array_get" && len(x.Args) >= 2 {
				nodeVal := ls.lowerExpr(x.Args[0])
				indexVal := ls.lowerExpr(x.Args[1])
				dst := ls.b.FreshTemp("json_arr_elem")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_array_get", Args: []hir.Value{nodeVal, indexVal}, Type: "ptr"})
				return dst
			}
			if method == "object_get" && len(x.Args) >= 2 {
				nodeVal := ls.lowerExpr(x.Args[0])
				keyVal := ls.lowerExpr(x.Args[1])
				dst := ls.b.FreshTemp("json_obj_val")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__json_object_get", Args: []hir.Value{nodeVal, keyVal}, Type: "ptr"})
				return dst
			}
		}

		// Handle math module functions (hardcoded like json module)
		if id, ok := fe.X.(*ast.Ident); ok && id.Name == "math" {
			method := fe.Name.Name
			if method == "is_nan" && len(x.Args) >= 1 {
				val := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("is_nan")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__math_is_nan", Args: []hir.Value{val}, Type: "i1"})
				return dst
			}
			if method == "is_inf" && len(x.Args) >= 1 {
				val := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("is_inf")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__math_is_inf", Args: []hir.Value{val}, Type: "i1"})
				return dst
			}
			if method == "is_finite" && len(x.Args) >= 1 {
				val := ls.lowerExpr(x.Args[0])
				dst := ls.b.FreshTemp("is_finite")
				ls.b.Emit(&hir.Call{Dst: dst, Fn: "__math_is_finite", Args: []hir.Value{val}, Type: "i1"})
				return dst
			}
		}
	}

	// 2. M14 Stage 3: print(Display) + Auto to_str for collections
	// If we have type info, check if this is print(arg...) where:
	// - arg implements Display trait, OR
	// - arg is a collection (list/dict/set) with to_str method, OR
	// - arg is a custom class with to_str method
	// Supports multiple arguments - prints each separated by space, ending with newline
	// Supports keyword args: sep (default " "), end (default "\n")
	if ls.info != nil {
		calleeName := ls.calleeName(x.Callee)
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

			// If style is set, emit style start
			if styleHIR != nil {
				styleTmp := ls.b.FreshTemp("style_start")
				ls.b.Emit(&hir.Call{Dst: styleTmp, Fn: "print_style_start_stdout", Args: []hir.Value{styleHIR}})
			}

			if len(argExprs) == 0 {
				// print() with no args - just print end (default newline)
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
				return dst
			}

			// For each argument, convert to string representation and emit print
			for i, arg := range argExprs {
				argT := ls.info.Types[arg]
				if argT == nil {
					// Fall through to regular handling - lower arg directly
					argVal := ls.lowerExpr(arg)
					if i < len(argExprs)-1 {
						dst := ls.b.FreshTemp("print")
						if streamHIR != nil {
							ls.b.Emit(&hir.Call{Dst: dst, Fn: "stream_print_str", Args: []hir.Value{streamHIR, argVal}})
						} else {
							ls.b.Emit(&hir.Call{Dst: dst, Fn: "print_item", Args: []hir.Value{argVal}})
						}
						sepTemp := ls.b.FreshTemp("sep")
						if streamHIR != nil {
							ls.b.Emit(&hir.Call{Dst: sepTemp, Fn: "stream_print_str", Args: []hir.Value{streamHIR, sepHIR}})
						} else {
							ls.b.Emit(&hir.Call{Dst: sepTemp, Fn: "print_raw", Args: []hir.Value{sepHIR}})
						}
					} else {
						dst := ls.b.FreshTemp("print")
						if streamHIR != nil {
							ls.b.Emit(&hir.Call{Dst: dst, Fn: "stream_print_str", Args: []hir.Value{streamHIR, argVal}})
						} else {
							ls.b.Emit(&hir.Call{Dst: dst, Fn: "print_item", Args: []hir.Value{argVal}})
						}
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
						return dst
					}
					continue
				}
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
					case *types.Struct:
						// Structs can implement Display trait - handled in Case 1 above
						// Or we can check for __repr__ method on the struct type
						// For now, skip - structs need Display trait implementation
					}
				}

				// Lower argument
				argVal := ls.lowerExpr(arg)

				// If custom to_str, call it first
				if shouldCallToStr {
					strTemp := ls.b.FreshTemp("str")
					ls.b.Emit(&hir.Call{Dst: strTemp, Fn: toStrFuncName, Args: []hir.Value{argVal}, Type: "ptr"})
					argVal = strTemp
				}

				// Emit print for this argument
				// For multiple args, emit print_item then separator
				// For last arg, emit print_item then end terminator
				if i < len(argExprs)-1 {
					// Print value, then separator
					dst := ls.b.FreshTemp("print")
					if streamHIR != nil {
						ls.b.Emit(&hir.Call{Dst: dst, Fn: "stream_print_str", Args: []hir.Value{streamHIR, argVal}})
					} else {
						ls.b.Emit(&hir.Call{Dst: dst, Fn: "print_item", Args: []hir.Value{argVal}})
					}
					// Print separator (customizable via sep=)
					sepTemp := ls.b.FreshTemp("sep")
					if streamHIR != nil {
						ls.b.Emit(&hir.Call{Dst: sepTemp, Fn: "stream_print_str", Args: []hir.Value{streamHIR, sepHIR}})
					} else {
						ls.b.Emit(&hir.Call{Dst: sepTemp, Fn: "print_raw", Args: []hir.Value{sepHIR}})
					}
				} else {
					// Last arg - print value then end terminator
					dst := ls.b.FreshTemp("print")
					if streamHIR != nil {
						ls.b.Emit(&hir.Call{Dst: dst, Fn: "stream_print_str", Args: []hir.Value{streamHIR, argVal}})
					} else {
						ls.b.Emit(&hir.Call{Dst: dst, Fn: "print_item", Args: []hir.Value{argVal}})
					}
					// Emit style end BEFORE newline to prevent color bleed
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
					return dst
				}
			}
			// Emit style end if style was set
			if styleHIR != nil {
				styleEndTmp := ls.b.FreshTemp("style_end")
				ls.b.Emit(&hir.Call{Dst: styleEndTmp, Fn: "print_style_end_stdout", Args: []hir.Value{}})
			}
			// If we processed all args, return
			return nil
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
			callee := ls.calleeName(x.Callee)
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
				ls.b.Emit(&hir.Call{
					Dst:  inst,
					Fn:   "malloc",
					Args: []hir.Value{hir.ConstInt{Text: fmt.Sprintf("%d", size), Type: "i32"}},
					Type: "ptr",
				})

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
