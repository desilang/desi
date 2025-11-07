package llvm

type funcSig struct {
	ret    string
	params []string
}

var funcSigOverrides = map[string]funcSig{}

// SetFuncSig registers a textual LLVM signature override.
// Use empty strings to keep defaults for ret or any param.
func SetFuncSig(name, ret string, params []string) {
	cp := make([]string, len(params))
	copy(cp, params)
	funcSigOverrides[name] = funcSig{ret: ret, params: cp}
}

func getFuncSig(name string) (funcSig, bool) {
	s, ok := funcSigOverrides[name]
	return s, ok
}
