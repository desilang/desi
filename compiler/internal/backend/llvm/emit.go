package llvm

import (
	"github.com/desilang/desi/compiler/internal/hir"
)

// EmitModule converts a HIR module to LLVM IR string.
func EmitModule(mod *hir.Module) string {
	m := NewModule(mod.Name)

	// 1. Register all functions first (so calls can be resolved)
	for _, f := range mod.Funcs {
		m.RegisterFunc(f.Name)
	}

	// 2. Emit all functions
	for _, f := range mod.Funcs {
		m.EmitFunc(f)
	}

	// 3. Emit global string constants and other globals
	// The writeGlobals() method in module.go handles string literals collected during emission
	// But we need to support global variables from mod.Globals too?
	// module_lower.go populated mod.Globals.
	// hir.Module has Globals map[string]hir.Global.
	// We need to transfer these to llvm.Module.

	// Transfer globals
	if mod.Globals != nil {
		for name, g := range mod.Globals {
			// hir.Global has Type (string) and Val (initial value string? or generic?)
			// We need to map HIR type to LLVM type
			// hir.Global definition:
			// type Global struct {
			//     Name string
			//     Type string
			//     Val  string // literal value or zero
			//     IsConst bool
			// }

			// Allow @ prefix (optional)
			globalName := name
			if len(globalName) > 0 && globalName[0] != '@' {
				globalName = "@" + globalName
			}

			// Map type (simple mapping for now, can be improved)
			llvmType := g.Type
			if llvmType == "" {
				llvmType = "i64" // default
			} else if llvmType == "int" {
				llvmType = "i64" // or i32 depending on arch? default to i64 for Desi int
			} else if llvmType == "ptr" {
				llvmType = "ptr"
			}
			// Add to staticFieldGlobals which is used for global var emission in writeGlobals
			val := "0"
			if llvmType == "ptr" {
				val = "null"
			}
			m.staticFieldGlobals[globalName] = GlobalDef{Type: llvmType, Value: val}
		}
	}

	return m.IR()
}
