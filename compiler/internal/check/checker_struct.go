package check

type checker struct {
	info  *Info
	fnSig FuncSig

	scope *scope

	errors   []error
	warnings []Warning

	locals        []*varInfo
	blockReturned []bool

	// From-import alias map: alias -> original symbol name (includes no-`as` items as alias==name)
	aliases map[string]string

	// Module alias map: alias -> module path (e.g., "util.math")
	modAliases map[string]string

	// Feature gate (M11)
	features struct {
		Async bool
	}
}
