package types

// OptionOf creates an Option<T> type.
// Option<T> is a built-in enum with two variants:
//   - Some: T   (tag 0)
//   - Nothing   (tag 1) - using "Nothing" since "None" is a keyword
func OptionOf(elem T) *Enum {
	return &Enum{
		Name:       "Option",
		TypeParams: []TypeParam{{Name: "T"}},
		Variants: []Variant{
			{Name: "Some", Fields: []Field{{Name: "value", Type: elem}}, Tag: 0},
			{Name: "Nothing", Fields: nil, Tag: 1},
		},
	}
}

// ResultOf creates a Result<T, E> type.
// Result<T, E> is a built-in enum with two variants:
//   - Ok: T  (tag 0)
//   - Err: E (tag 1)
func ResultOf(ok T, err T) *Enum {
	return &Enum{
		Name:       "Result",
		TypeParams: []TypeParam{{Name: "T"}, {Name: "E"}},
		Variants: []Variant{
			{Name: "Ok", Fields: []Field{{Name: "value", Type: ok}}, Tag: 0},
			{Name: "Err", Fields: []Field{{Name: "error", Type: err}}, Tag: 1},
		},
	}
}

// IsOption returns true if the type is Option<T>
func IsOption(t T) bool {
	if e, ok := t.(*Enum); ok {
		return e.Name == "Option" && len(e.Variants) == 2 &&
			e.Variants[0].Name == "Some" && e.Variants[1].Name == "Nothing"
	}
	if g, ok := t.(*Generic); ok {
		return IsOption(g.Base)
	}
	return false
}

// IsResult returns true if the type is Result<T, E>
func IsResult(t T) bool {
	if e, ok := t.(*Enum); ok {
		return e.Name == "Result" && len(e.Variants) == 2 &&
			e.Variants[0].Name == "Ok" && e.Variants[1].Name == "Err"
	}
	if g, ok := t.(*Generic); ok {
		return IsResult(g.Base)
	}
	return false
}

// OptionSomeType returns the T in Option<T>, or nil if not an Option
func OptionSomeType(t T) T {
	if g, ok := t.(*Generic); ok && IsOption(g.Base) {
		if len(g.Args) > 0 {
			return g.Args[0]
		}
	}
	if e, ok := t.(*Enum); ok && IsOption(e) {
		if len(e.Variants) > 0 && len(e.Variants[0].Fields) > 0 {
			return e.Variants[0].Fields[0].Type
		}
	}
	return nil
}

// ResultOkType returns the T in Result<T, E>, or nil if not a Result
func ResultOkType(t T) T {
	if g, ok := t.(*Generic); ok && IsResult(g.Base) {
		if len(g.Args) > 0 {
			return g.Args[0]
		}
	}
	return nil
}

// ResultErrType returns the E in Result<T, E>, or nil if not a Result
func ResultErrType(t T) T {
	if g, ok := t.(*Generic); ok && IsResult(g.Base) {
		if len(g.Args) > 1 {
			return g.Args[1]
		}
	}
	return nil
}
