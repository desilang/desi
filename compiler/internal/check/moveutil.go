package check

import (
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/types"
)

// isCopyType reports whether a type is treated as Copy (not moved) by the checker.
// Current policy: primitives are Copy (int, float, bool, str).
func isCopyType(t types.T) bool {
	return types.Equal(t, types.Int) ||
		types.Equal(t, types.Float) ||
		types.Equal(t, types.Bool) ||
		types.Equal(t, types.Str)
}

// MoveSet tracks identifiers that have been moved, mapping name -> move site.
type MoveSet map[string]diag.Span

// Mark records that 'name' was moved at 'at'.
func (ms MoveSet) Mark(name string, at diag.Span) {
	if ms == nil {
		return
	}
	ms[name] = at
}

// IsMoved reports whether 'name' has been previously moved and returns the move site.
func (ms MoveSet) IsMoved(name string) (diag.Span, bool) {
	if ms == nil {
		return diag.Span{}, false
	}
	sp, ok := ms[name]
	return sp, ok
}

// makeUseAfterMoveDiag builds DBR0004 with a secondary label at the move site.
func makeUseAfterMoveDiag(useSpan, movedAt diag.Span) diag.Diagnostic {
	return diag.Diagnostic{
		CodeID:  "DBR0004",
		Domain:  "borrow",
		Message: "value was moved earlier and cannot be used again",
		Primary: diag.Label{Span: useSpan, Primary: true},
		Labels:  []diag.Label{{Span: movedAt, Text: "moved here"}},
	}
}
