package lower

// WrapperRegistry tracks TaskGroup wrapper functions that need to be emitted.
// This is used to communicate between lowering phase and LLVM backend.

// WrapperInfo holds info needed to emit a TaskGroup wrapper function
type WrapperInfo struct {
	TargetFn    string // function to call (__lam$N or named function)
	NumCaptures int    // number of captures to unpack from ctx
}

// Global registry of wrappers - populated during lowering, read during LLVM emission
var tgWrapperRegistry = make(map[string]WrapperInfo)

// RegisterTGWrapper registers a TaskGroup wrapper to be emitted
func RegisterTGWrapper(wrapperName, targetFn string, numCaptures int) {
	if tgWrapperRegistry[wrapperName].TargetFn != "" {
		return // Already registered
	}
	tgWrapperRegistry[wrapperName] = WrapperInfo{
		TargetFn:    targetFn,
		NumCaptures: numCaptures,
	}
}

// GetTGWrappers returns all registered wrappers
func GetTGWrappers() map[string]WrapperInfo {
	return tgWrapperRegistry
}

// ClearTGWrappers clears the registry (for testing)
func ClearTGWrappers() {
	tgWrapperRegistry = make(map[string]WrapperInfo)
}
