package lower

import (
	"sort"

	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// Ownership model for string temporaries:
//
// A string temp is registered here when it is the result of a heap-producing
// operation (string_concat). If nothing takes ownership of it by the end of
// its scope, emitTempDrops emits a Drop and the backend frees it. Every
// construct that stores the pointer somewhere longer-lived (let bindings,
// assignments, returns, call arguments that move, collection inserts,
// struct/enum/tuple construction, channel sends) must call consumeTemp to
// transfer ownership and prevent the scope-end free.
//
// Registration is suppressed inside expression-level control flow
// (comprehension bodies, match-expression arms): a temp defined in a
// conditionally-executed block does not dominate the scope end, so a free
// there would produce invalid LLVM IR. Those temps leak instead — leaking
// is safe, a dominance violation is a compile failure.

// suppressTempDrops disables temp registration until resumeTempDrops.
// Calls nest.
func (ls *lowerState) suppressTempDrops() {
	ls.tempSuppress++
}

// resumeTempDrops re-enables temp registration after suppressTempDrops.
func (ls *lowerState) resumeTempDrops() {
	if ls.tempSuppress > 0 {
		ls.tempSuppress--
	}
}

// addTempDrop registers a temporary that needs to be dropped if not consumed.
func (ls *lowerState) addTempDrop(name string) {
	if ls.tempSuppress > 0 {
		return
	}
	if len(ls.scopes) > 0 {
		ls.cur().tempDrops[name] = true
	}
}

// consumeTemp marks a temporary as consumed (ownership transferred), so it
// won't be dropped at scope end. Safe to call on any hir.Value; non-temps
// and untracked temps are ignored. Searches all open scopes — a temp may be
// consumed in a deeper scope than the one it was registered in.
func (ls *lowerState) consumeTemp(val hir.Value) {
	t, ok := val.(hir.Temp)
	if !ok {
		return
	}
	for i := len(ls.scopes) - 1; i >= 0; i-- {
		if ls.scopes[i].tempDrops[t.Name] {
			delete(ls.scopes[i].tempDrops, t.Name)
			return
		}
	}
}

// emitTempDrops emits Drop instructions for all unconsumed temporaries in
// the given scope. Called from emitScopeDrops — which, like local drops,
// runs once per exit path (early returns and the normal scope end), so the
// set must NOT be cleared here: each runtime execution takes exactly one of
// those paths. The map dies with the scope. Names are sorted so the emitted
// IR is deterministic.
func (ls *lowerState) emitTempDrops(sc *scope) {
	if len(sc.tempDrops) == 0 {
		return
	}
	names := make([]string, 0, len(sc.tempDrops))
	for name := range sc.tempDrops {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		// Only strings are tracked here; the Drop type tells the backend
		// which free path to use.
		ls.b.Emit(&hir.Drop{Val: hir.Temp{Name: name}, Type: types.Str})
	}
}
