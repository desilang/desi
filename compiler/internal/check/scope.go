package check

// Simple lexical scopes for locals/params/lets and type-ish names.
// We keep it small for M4. Symbols shadow in inner scopes.

type Scope struct {
	parent  *Scope
	symbols map[string]*Symbol
}

func NewScope(parent *Scope) *Scope {
	return &Scope{
		parent:  parent,
		symbols: make(map[string]*Symbol),
	}
}

// Define inserts a symbol into the current scope. Returns false if already present.
func (s *Scope) Define(sym *Symbol) bool {
	if _, exists := s.symbols[sym.Name]; exists {
		return false
	}
	s.symbols[sym.Name] = sym
	return true
}

// Lookup returns the nearest symbol bound to name, searching outward.
func (s *Scope) Lookup(name string) *Symbol {
	for sc := s; sc != nil; sc = sc.parent {
		if sym, ok := sc.symbols[name]; ok {
			return sym
		}
	}
	return nil
}
