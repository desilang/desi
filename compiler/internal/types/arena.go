package types

// Arena is a lightweight handle shape for region allocation.
//
// It participates in the types.T lattice via isType(), and prints as "arena".
type Arena struct{}

func (*Arena) isType() {}

func (*Arena) String() string { return "arena" }

// ArenaOf is a convenience ctor in case we ever need a distinct value.
func ArenaOf() *Arena { return &Arena{} }
