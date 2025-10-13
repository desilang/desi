package diag

import (
	"fmt"
	"strings"
)

type Pos struct{ Line, Col, Byte int }
type Span struct {
	File       string
	Start, End Pos
}

type Label struct {
	Span    Span
	Text    string
	Primary bool
}

type Diagnostic struct {
	CodeID  string // e.g., "DTE0004"
	Domain  string // e.g., "type"
	Title   string // e.g., "type mismatch"
	Message string // optional custom message override
	Primary Label
	Labels  []Label
	Notes   []string
	Help    string
}

type Builder struct {
	cat Catalog
}

// NewBuilder wires a catalog into a small factory.
func NewBuilder(cat Catalog) Builder { return Builder{cat: cat} }

// New makes a Diagnostic from a dotted code path (e.g., "type.type_mismatch").
func (b Builder) New(codePath string, primary Label, opts ...Option) (Diagnostic, error) {
	e, ok := b.cat.Get(codePath)
	if !ok {
		return Diagnostic{}, fmt.Errorf("unknown code path: %s", codePath)
	}
	domain := codePath[:strings.IndexByte(codePath, '.')]
	d := Diagnostic{
		CodeID:  e.ID,
		Domain:  domain,
		Title:   e.Title,
		Primary: primary,
		Help:    e.Help,
	}
	for _, o := range opts {
		o(&d)
	}
	return d, nil
}

// Options to avoid rewriting call sites when evolving the API.
type Option func(*Diagnostic)

func WithMessage(msg string) Option { return func(d *Diagnostic) { d.Message = msg } }
func WithLabels(ls ...Label) Option {
	return func(d *Diagnostic) { d.Labels = append(d.Labels, ls...) }
}
func WithNotes(ns ...string) Option { return func(d *Diagnostic) { d.Notes = append(d.Notes, ns...) } }
