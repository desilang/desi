package check

import "fmt"

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

func (s *scope) define(name string, v *varInfo) error {
	if _, exists := s.vars[name]; exists {
		return fmt.Errorf("redeclaration of %q", name)
	}
	s.vars[name] = v
	return nil
}
