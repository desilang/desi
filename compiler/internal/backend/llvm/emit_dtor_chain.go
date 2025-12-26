package llvm

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/types"
)

// emitDestructorChain emits destructor calls for a class and all its base classes
// in child-to-parent order (most-derived first, base last).
// This ensures proper cleanup in the presence of inheritance.
func (m *Module) emitDestructorChain(cls *types.Class, valOperand string) {
	// Collect destructor chain: [Child, Parent, GrandParent, ...]
	var chain []*types.Class
	current := cls
	for current != nil {
		// Check if this class has a __del__ method
		if _, hasDel := current.Dunders["__del__"]; hasDel {
			chain = append(chain, current)
		}
		current = current.Base
	}

	// Emit calls in order (child → parent)
	for _, c := range chain {
		// Generate the mangled __del__ name
		delName := fmt.Sprintf("%s___del__", c.Name)
		// Emit call: ClassName___del__(ptr %valOperand)
		wprintf(&m.funcs, "  call void @%s(ptr %s)\n", delName, valOperand)
		// Ensure declaration
		m.ensureDecl(fmt.Sprintf("declare void @%s(ptr)", delName))
	}
}
