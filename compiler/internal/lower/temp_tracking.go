package lower

import (
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// addTempDrop registers a temporary that needs to be dropped if not consumed.
func (ls *lowerState) addTempDrop(name string) {
	if len(ls.scopes) > 0 {
		ls.cur().tempDrops[name] = true
	}
}

// consumeTemp marks a temporary as consumed (moved), so it won't be dropped.
func (ls *lowerState) consumeTemp(val hir.Value) {
	if t, ok := val.(hir.Temp); ok {
		if len(ls.scopes) > 0 {
			delete(ls.cur().tempDrops, t.Name)
		}
	}
}

// emitTempDrops emits Drop instructions for all registered temporaries in the current scope
// and clears the list. This should be called at the end of a statement.
func (ls *lowerState) emitTempDrops() {
	if len(ls.scopes) > 0 {
		sc := ls.cur()
		// Sort keys for deterministic output
		// But map iteration order is random.
		// For now, just iterate.
		for name := range sc.tempDrops {
			// We don't have the type here, but Drop instruction needs it?
			// hir.Drop takes Type interface{}.
			// For strings, it's types.Str.
			// We can assume it's a string for now as we only track strings.
			ls.b.Emit(&hir.Drop{Val: hir.Temp{Name: name}, Type: types.Str})
		}
		// Clear map
		sc.tempDrops = map[string]bool{}
	}
}
