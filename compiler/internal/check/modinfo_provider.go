package check

// ModuleInfoProvider lets the checker ask about a module’s exported symbols.
type ModuleInfoProvider interface {
	// Lookup returns the Info for a fully qualified module path (e.g., "util.math").
	// ok is false if the module is unknown to the provider.
	Lookup(modulePath string) (*Info, bool)
}

// DefaultModuleInfoProvider is an optional global used by the checker.
// The build/resolve step should assign this once before checking.
var DefaultModuleInfoProvider ModuleInfoProvider
