package ast

// ParamMode describes how a parameter is passed.
// - ParamMove  (default): move-by-value semantics
// - ParamRef   : shared borrow (immutable reference)
// - ParamInout : unique mutable borrow (exclusive reference)
type ParamMode int

const (
	ParamMove ParamMode = iota
	ParamRef
	ParamInout
)

// String returns a human-friendly label for debugging/printers.
func (m ParamMode) String() string {
	switch m {
	case ParamRef:
		return "ref"
	case ParamInout:
		return "inout"
	default:
		return ""
	}
}
