package lower

// WrapperRegistry tracks TaskGroup wrapper functions that need to be emitted.
// This is used to communicate between lowering phase and LLVM backend.

// WrapperInfo holds info needed to emit a TaskGroup wrapper function
type WrapperInfo struct {
	TargetFn    string // function to call (__lam$N or named function)
	NumCaptures int    // number of captures to unpack from ctx
	// CapTypes is the LLVM type of each capture as the target function
	// declares it. A primitive is boxed into the context, so the wrapper has to
	// load through the box to recover the value; "ptr" means the context holds
	// the pointer itself and one load is enough.
	//
	// Without this the wrapper assumed "ptr" for everything and handed the box
	// address to a function expecting an i32 — an int capture printed a garbage
	// address, while a str happened to look right because it really was a
	// pointer.
	CapTypes []string
}

// Global registry of wrappers - populated during lowering, read during LLVM emission
var tgWrapperRegistry = make(map[string]WrapperInfo)

// RegisterTGWrapper registers a TaskGroup wrapper to be emitted
func RegisterTGWrapper(wrapperName, targetFn string, numCaptures int, capTypes []string) {
	if tgWrapperRegistry[wrapperName].TargetFn != "" {
		return // Already registered
	}
	cp := make([]string, len(capTypes))
	copy(cp, capTypes)
	tgWrapperRegistry[wrapperName] = WrapperInfo{
		TargetFn:    targetFn,
		NumCaptures: numCaptures,
		CapTypes:    cp,
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
