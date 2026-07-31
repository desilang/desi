package types

// Range is the value produced by range(start, stop, step). It is a lazy
// arithmetic sequence of ints, not a materialised list: iterating one allocates
// nothing, and binding one costs three integers.
//
// The element type is always int, so Range carries no type parameter.
type Range struct{}

func (*Range) isType() {}
func (t *Range) String() string {
	return "range"
}

// RangeOf returns the Range type.
func RangeOf() *Range {
	return &Range{}
}

// Elem is the type produced by iterating or indexing a range.
func (t *Range) Elem() T { return Int }
