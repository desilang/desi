package check

import (
	"github.com/desilang/desi/compiler/internal/types"
)

// unify attempts to unify a pattern type (which may contain TypeParams)
// with a concrete type, populating the 'inferred' map.
// Returns true if unification is possible (or at least not contradictory).
func unify(pattern, concrete types.T, inferred map[string]types.T) bool {
	if pattern == nil || concrete == nil {
		return false
	}

	// If pattern is a TypeParam, infer it
	if tp, ok := pattern.(*types.TypeParam); ok {
		if existing, ok := inferred[tp.Name]; ok {
			// Already inferred, must match
			return types.Equal(existing, concrete)
		}
		inferred[tp.Name] = concrete
		return true
	}

	// If pattern is a Generic instance (e.g. List[T]), recurse
	if gPattern, ok := pattern.(*types.Generic); ok {
		if gConcrete, ok := concrete.(*types.Generic); ok {
			if !types.Equal(gPattern.Base, gConcrete.Base) {
				return false
			}
			if len(gPattern.Args) != len(gConcrete.Args) {
				return false
			}
			for i := range gPattern.Args {
				if !unify(gPattern.Args[i], gConcrete.Args[i], inferred) {
					return false
				}
			}
			return true
		}
		// Also handle built-in containers which are implicitly generic-like
		// e.g. list[T] vs list[int]
		// But types.List is a struct wrapping Elem.
	}

	// Handle built-in containers
	switch p := pattern.(type) {
	case *types.List:
		if c, ok := concrete.(*types.List); ok {
			return unify(p.Elem, c.Elem, inferred)
		}
	case *types.Set:
		if c, ok := concrete.(*types.Set); ok {
			return unify(p.Elem, c.Elem, inferred)
		}
	case *types.Dict:
		if c, ok := concrete.(*types.Dict); ok {
			return unify(p.Key, c.Key, inferred) && unify(p.Val, c.Val, inferred)
		}
	case *types.Tuple:
		if c, ok := concrete.(*types.Tuple); ok {
			if len(p.Elems) != len(c.Elems) {
				return false
			}
			for i := range p.Elems {
				if !unify(p.Elems[i], c.Elems[i], inferred) {
					return false
				}
			}
			return true
		}
	case *types.Future:
		if c, ok := concrete.(*types.Future); ok {
			return unify(p.Elem, c.Elem, inferred)
		}
	case *types.Func:
		if c, ok := concrete.(*types.Func); ok {
			if len(p.Params) != len(c.Params) {
				return false
			}
			for i := range p.Params {
				if !unify(p.Params[i], c.Params[i], inferred) {
					return false
				}
			}
			return unify(p.Ret, c.Ret, inferred)
		}
	}

	// Exact match for others
	return types.Equal(pattern, concrete)
}

// substitute replaces TypeParams in t with values from subst.
func substitute(t types.T, subst map[string]types.T) types.T {
	if t == nil {
		return nil
	}

	switch x := t.(type) {
	case *types.TypeParam:
		if replacement, ok := subst[x.Name]; ok {
			return replacement
		}
		return x

	case *types.Generic:
		// Substitute in args
		newArgs := make([]types.T, len(x.Args))
		changed := false
		for i, arg := range x.Args {
			newArgs[i] = substitute(arg, subst)
			if newArgs[i] != arg {
				changed = true
			}
		}
		if changed {
			return &types.Generic{Base: x.Base, Args: newArgs}
		}
		return x

	case *types.List:
		elem := substitute(x.Elem, subst)
		if elem != x.Elem {
			return types.ListOf(elem)
		}
		return x

	case *types.Set:
		elem := substitute(x.Elem, subst)
		if elem != x.Elem {
			return types.SetOf(elem)
		}
		return x

	case *types.Dict:
		k := substitute(x.Key, subst)
		v := substitute(x.Val, subst)
		if k != x.Key || v != x.Val {
			return types.DictOf(k, v)
		}
		return x

	case *types.Tuple:
		newElems := make([]types.T, len(x.Elems))
		changed := false
		for i, e := range x.Elems {
			newElems[i] = substitute(e, subst)
			if newElems[i] != e {
				changed = true
			}
		}
		if changed {
			return types.TupleOf(newElems...)
		}
		return x

	case *types.Func:
		newParams := make([]types.T, len(x.Params))
		changed := false
		for i, p := range x.Params {
			newParams[i] = substitute(p, subst)
			if newParams[i] != p {
				changed = true
			}
		}
		newRet := substitute(x.Ret, subst)
		if newRet != x.Ret {
			changed = true
		}
		if changed {
			f := types.FuncOf(newParams, newRet, x.Variadic)
			f.Name = x.Name
			f.TypeParams = x.TypeParams // Should be empty if we are substituting?
			return f
		}
		return x
	}

	return t
}
