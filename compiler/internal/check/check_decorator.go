package check

import "github.com/desilang/desi/compiler/internal/ast"

// hasDecorator checks if a FuncDecl has a specific decorator by name
func hasDecorator(fd *ast.FuncDecl, name string) bool {
	if fd == nil {
		return false
	}
	for _, dec := range fd.Decorators {
		if dec != nil && dec.Name.Name == name {
			return true
		}
	}
	return false
}
