package types

// M7B: Reference-counted shapes rc[T], arc[T], and weak[T].
// These are lightweight wrappers that integrate with the existing types.T lattice
// via the standard isType marker and String canonicalization.

type Rc struct{ Inner T }
type Arc struct{ Inner T }
type Weak struct{ Inner T }

func (*Rc) isType()   {}
func (*Arc) isType()  {}
func (*Weak) isType() {}

func (t *Rc) String() string   { return "rc[" + t.Inner.String() + "]" }
func (t *Arc) String() string  { return "arc[" + t.Inner.String() + "]" }
func (t *Weak) String() string { return "weak[" + t.Inner.String() + "]" }

// Helper constructors (handy in tests and call sites).
func RcOf(inner T) *Rc     { return &Rc{Inner: inner} }
func ArcOf(inner T) *Arc   { return &Arc{Inner: inner} }
func WeakOf(inner T) *Weak { return &Weak{Inner: inner} }
