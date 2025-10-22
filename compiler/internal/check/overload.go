package check

import "github.com/desilang/desi/compiler/internal/types"

// Add registers a candidate into the set.
func (s *OverloadSet) Add(c *FuncCand) {
	if s.Cands == nil {
		s.Cands = make([]*FuncCand, 0, 4)
	}
	s.Cands = append(s.Cands, c)
}

// ResolveExact returns all candidates whose parameter list exactly matches args.
// Callers should interpret len==0 as "no match", len==1 as unique, and len>1 as ambiguous.
func (s *OverloadSet) ResolveExact(args []types.T) []*FuncCand {
	if s == nil {
		return nil
	}
	var out []*FuncCand
	for _, c := range s.Cands {
		if len(c.Type.Params) != len(args) {
			continue
		}
		ok := true
		for i := range args {
			if !types.Equal(c.Type.Params[i], args[i]) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, c)
		}
	}
	return out
}
