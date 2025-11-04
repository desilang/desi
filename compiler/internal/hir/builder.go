package hir

import "fmt"

// Builder helps construct a function and generate named temporaries.
type Builder struct {
	f    *Func
	cur  *Block
	temp int
}

func NewFunc(name string) *Builder {
	b := &Builder{f: &Func{Name: name}}
	b.cur = NewBlock("entry")
	b.f.Blocks = append(b.f.Blocks, b.cur)
	return b
}

func (b *Builder) Func() *Func   { return b.f }
func (b *Builder) Block() *Block { return b.cur }

func (b *Builder) NewBlock(name string) *Block {
	blk := NewBlock(name)
	b.f.Blocks = append(b.f.Blocks, blk)
	return blk
}

func (b *Builder) SetBlock(blk *Block) { b.cur = blk }

func (b *Builder) FreshTemp(hint string) Temp {
	b.temp++
	name := fmt.Sprintf("%%t%d", b.temp)
	if hint != "" {
		name = fmt.Sprintf("%%%s%d", hint, b.temp)
	}
	return Temp{Name: name}
}

func (b *Builder) Emit(s Stmt) { b.cur.Stmts = append(b.cur.Stmts, s) }
