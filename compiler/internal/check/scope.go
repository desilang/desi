package check

type varInfo struct {
  kind     Kind
  mutable  bool
  declName string

  read    bool
  written bool
}

type scope struct {
  parent *scope
  vars   map[string]*varInfo
}

func (s *scope) lookup(name string) (*varInfo, bool) {
  for cur := s; cur != nil; cur = cur.parent {
    if v, ok := cur.vars[name]; ok {
      return v, true
    }
  }
  return nil, false
}

// lookupLocal checks only the current scope frame.
func (s *scope) lookupLocal(name string) (*varInfo, bool) {
  v, ok := s.vars[name]
  return v, ok
}

// existsInOuter reports whether a name exists in any outer (ancestor) scope.
func (s *scope) existsInOuter(name string) bool {
  for cur := s.parent; cur != nil; cur = cur.parent {
    if _, ok := cur.vars[name]; ok {
      return true
    }
  }
  return false
}

func (s *scope) define(name string, v *varInfo) error {
  if _, exists := s.vars[name]; exists {
    // Redeclaration in the same scope -> catalog-backed error DTE0003
    return ErrRedeclaredSymbol(name, "declaration")
  }
  s.vars[name] = v
  return nil
}
