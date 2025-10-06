package check

// Centralized visibility predicates for the checker.
// Keep ALL cross-module visibility rules behind these helpers so sites don't
// reimplement the policy (and so tests only need to cover these).

// isPublicFunc reports whether `name` is visible across module boundaries.
// Local functions are always visible inside the same module.
func (c *checker) isPublicFunc(name string) bool {
	if name == "" {
		return false
	}
	// If the function is defined in the current file/module, it is visible.
	if c.info.FuncsLocal[name] {
		return true
	}
	// Otherwise, require public export.
	return c.info.FuncsPublic[name]
}

// isPublicConst reports whether a top-level const is public.
// Local consts are always visible inside the same module; cross-module uses
// must be in the public set (ConstsPublic).
func (c *checker) isPublicConst(name string) bool {
	if name == "" {
		return false
	}
	return c.info.ConstsPublic[name]
}

// isPublicTypeName reports visibility for named user types (struct/enum/alias).
func (c *checker) isPublicTypeName(name string) bool {
	if name == "" {
		return false
	}
	if c.info.StructsPublic[name] {
		return true
	}
	if c.info.TypesPublic[name] {
		return true
	}
	if c.info.EnumsPublic[name] {
		return true
	}
	return false
}
