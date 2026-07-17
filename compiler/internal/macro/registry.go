// Package macro provides the Desi compiler macro DSL system.
//
// The macro system has two tiers:
//
//	Tier 1 — Built-in decorators (@extern, @ffi_struct, @staticmethod, etc.)
//	         Hardcoded in the compiler, checked FIRST. Never overridden.
//
//	Tier 2 — Macro protocols (@model, @cache, @trace, user-defined, etc.)
//	         Registry-based, extensible at compile time.
//
// The compiler dispatcher code is 100% GENERIC — it never contains strings
// like "model", "objects", "Q", "F". All domain-specific names come from
// the macro protocol definitions. This means:
//
//   - Adding a new @snodel macro that injects ".snobjects" requires ZERO
//     Go compiler changes — just a new protocol definition.
//   - v0.1.0: protocol definitions live in Go (orm_protocol.go) because
//     Desi isn't self-hosting yet.
//   - v0.2.0+: protocol definitions move to .desi macro files, and the
//     compiler reads them generically.
//
// All macro work is compile-time only — zero runtime overhead.
package macro

import (
	"fmt"
	"sync"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/types"
)

// MacroTarget specifies what AST node type a macro can decorate.
type MacroTarget int

const (
	ClassMacro  MacroTarget = iota // decorates class declarations
	FuncMacro                      // decorates function declarations
	FieldMacro                     // decorates field declarations
	StructMacro                    // decorates struct declarations
)

// MacroContext provides context to macro protocol callbacks during type checking.
type MacroContext struct {
	Class     *types.Class   // the class being decorated (ClassMacro only)
	Decorator *ast.Decorator // the decorator AST node
	ClassDecl *ast.ClassDecl // the class AST declaration
	Info      interface{}    // *check.Info — interface{} to avoid import cycle
}

// LowerContext provides context to macro protocol callbacks during HIR lowering.
type LowerContext struct {
	Class     *types.Class
	ClassDecl *ast.ClassDecl
	Info      interface{} // *check.Info
}

// MacroBuiltin defines a function introduced by a macro protocol (e.g., Q(), F()).
type MacroBuiltin struct {
	Name     string  // function name (e.g., "Q", "F", "Count")
	RetType  types.T // return type of the function
	Protocol string  // which macro protocol introduced this builtin
}

// MacroOp defines an operator overload for macro-produced types.
type MacroOp struct {
	Op        string                                     // operator symbol: "|", "&", "~", "+", "-", "*", "/"
	LhsCheck  func(expr ast.Expr, info interface{}) bool // returns true if LHS is a macro expression
	RhsCheck  func(expr ast.Expr, info interface{}) bool // returns true if RHS is a macro expression (optional)
	RetType   types.T                                    // return type of the operation
	LowerFunc func(lhs, rhs hir.Value) *hir.Call         // compile-time lowering to C call
	Protocol  string
}

// MacroUnaryOp defines a unary operator overload for macro-produced types.
type MacroUnaryOp struct {
	Op        string // "~" for Q negation
	Check     func(expr ast.Expr, info interface{}) bool
	RetType   types.T
	LowerFunc func(operand hir.Value) *hir.Call
	Protocol  string
}

// PropertySpec defines an injected property on a macro-decorated class.
// For example, @model injects ".objects" — a @snodel macro could inject ".snobjects".
// The compiler never hardcodes property names; it reads them from the protocol.
type PropertySpec struct {
	Name         string                // property name (e.g., "objects", "snobjects")
	Methods      map[string]MethodSpec // methods available on this property
	RuntimeFuncs map[string]string     // method name → C runtime function name (e.g., "filter" → "__qs_filter")
	RuntimeType  string                // "c" (default) or "desi" — determines how runtime funcs are called
}

// MethodSpec defines a synthesized method on a macro-injected property.
type MethodSpec struct {
	Name        string
	RetType     types.T
	ParamTypes  []types.T
	IsChainable bool // true if method returns the manager (for chaining)
	IsTerminal  bool // true if method triggers query execution

	// Generic dispatch metadata — drives the lowerer WITHOUT hardcoded switch.
	// The lowerer reads these fields and emits calls generically.
	//
	// ArgStyle controls how call arguments are translated to C runtime calls:
	//   "kwargs_filter" — each kwarg (name=val) emits KwargsFunc(name_str, val)
	//                     Q expression positional args emit __qs_filter_q(qval)
	//   "kwargs_set"    — each kwarg (name=val) emits KwargsFunc(name_str, val)
	//   "positional"    — each positional arg emits KwargsFunc(arg)
	//   "none"          — no arg processing
	ArgStyle string

	// KwargsFunc is the C runtime function called per argument.
	// For "kwargs_filter"/"kwargs_set": called as Fn(key_str, val) for each kwarg.
	// For "positional": called as Fn(arg) for each positional arg.
	KwargsFunc string

	// TerminalFunc is the C runtime function that executes the terminal action.
	// Called after all args are processed. E.g. "__qs_fetch", "__qs_first", "__qs_count".
	// If empty, no terminal call (intermediate chainable method).
	TerminalFunc string

	// ReturnsModel indicates that this method returns a model class instance
	// constructed from the first row of the result set. The lowerer will:
	//   1. Call TerminalFunc to execute the query
	//   2. Check row count; if 0, return null (None)
	//   3. If rows exist, allocate a class instance and populate fields
	//      from __db_get_field_by(0, "field_name") for each class field
	ReturnsModel bool
}

// ValidationRules defines compile-time validation for macro-decorated classes.
type ValidationRules struct {
	RequireFields       bool     // error if class has no fields
	AutoPK              bool     // auto-generate primary key field
	ForbiddenFieldNames []string // error if user declares these field names
}

// MacroProtocol defines a compile-time decorator transformation.
// All work happens at compile time — zero runtime cost.
//
// The compiler dispatcher is 100% generic — it reads Names, Properties,
// Builtins, Operators from this struct. No domain-specific strings
// (like "model", "objects") exist anywhere in the compiler's Go code.
type MacroProtocol struct {
	Name   string      // "model", "snodel", etc. — defined here, NOT in compiler code
	Target MacroTarget // ClassMacro, FuncMacro, etc.

	// Phase 1 (collection): called during collectClass/collectFunc
	OnCollect func(ctx *MacroContext) error

	// Phase 2 (checking): called during checkClass/checkFunc
	OnCheck func(ctx *MacroContext) error

	// Phase 3 (lowering): emit HIR init function for the macro
	OnLower func(ctx *LowerContext) *hir.Func

	// Properties this macro injects on decorated classes.
	// Key = property name (e.g., "objects"). The compiler never hardcodes this.
	// A class `User` decorated with @model gets `User.objects` because the
	// @model protocol declares Properties["objects"].
	Properties map[string]*PropertySpec

	// Builtin functions this macro introduces.
	Builtins map[string]*MacroBuiltin

	// Binary operator overloads
	Operators map[string]*MacroOp

	// Unary operator overloads
	UnaryOperators map[string]*MacroUnaryOp

	// Validation rules for decorated classes
	Validation *ValidationRules

	// StripInRelease marks this macro's generated code for removal in release builds.
	// When true, the lowering phase omits all instrumentation emitted by OnLower,
	// and decorated functions become no-ops in `desic build --release`.
	// Use case: @perf, @trace, @debug — dev-only instrumentation with zero release cost.
	StripInRelease bool
}

// MacroRegistry is the central registry for all macro protocols.
type MacroRegistry struct {
	mu         sync.RWMutex
	protocols  map[string]*MacroProtocol
	builtins   map[string]*MacroBuiltin // global builtin function index
	operators  map[string][]*MacroOp    // op -> list of macro ops
	unaryOps   map[string][]*MacroUnaryOp
	properties map[string]*propertyIndex // property name -> protocol + spec
}

// propertyIndex maps a property name back to its protocol.
type propertyIndex struct {
	Protocol *MacroProtocol
	Spec     *PropertySpec
}

// Registry is the global macro registry instance.
var Registry = &MacroRegistry{
	protocols:  make(map[string]*MacroProtocol),
	builtins:   make(map[string]*MacroBuiltin),
	operators:  make(map[string][]*MacroOp),
	unaryOps:   make(map[string][]*MacroUnaryOp),
	properties: make(map[string]*propertyIndex),
}

// Register adds a macro protocol to the registry.
// Panics if the name conflicts with a built-in decorator.
func (r *MacroRegistry) Register(proto *MacroProtocol) {
	if proto == nil || proto.Name == "" {
		panic("macro: cannot register nil or unnamed protocol")
	}
	if IsBuiltinDecorator(proto.Name) {
		panic(fmt.Sprintf("macro: cannot register protocol '%s' — conflicts with built-in decorator", proto.Name))
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.protocols[proto.Name]; exists {
		panic(fmt.Sprintf("macro: duplicate protocol registration: '%s'", proto.Name))
	}

	r.protocols[proto.Name] = proto

	// Index builtins globally
	for name, builtin := range proto.Builtins {
		builtin.Protocol = proto.Name
		r.builtins[name] = builtin
	}

	// Index operator overloads
	for op, macroOp := range proto.Operators {
		macroOp.Protocol = proto.Name
		r.operators[op] = append(r.operators[op], macroOp)
	}
	for op, unaryOp := range proto.UnaryOperators {
		unaryOp.Protocol = proto.Name
		r.unaryOps[op] = append(r.unaryOps[op], unaryOp)
	}

	// Index properties globally
	for _, prop := range proto.Properties {
		r.properties[prop.Name] = &propertyIndex{Protocol: proto, Spec: prop}
	}
}

// Lookup returns the macro protocol for the given decorator name, or nil.
func (r *MacroRegistry) Lookup(name string) *MacroProtocol {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.protocols[name]
}

// LookupBuiltin returns a macro-registered builtin function, or nil.
func (r *MacroRegistry) LookupBuiltin(name string) *MacroBuiltin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.builtins[name]
}

// LookupProperty checks if a field name is a macro-injected property on a
// class that has the matching macro decorator. Returns (protocol, property spec, true)
// or (nil, nil, false).
//
// This is the 100% generic replacement for hardcoded checks like:
//
//	if cls.IsModel && fieldName == "objects" { ... }
//
// Now the compiler says:
//
//	if proto, prop, ok := macro.Registry.LookupProperty(cls, fieldName); ok { ... }
//
// It doesn't know what "objects" is. It just knows the protocol says this
// field name is an injected property.
func (r *MacroRegistry) LookupProperty(cls *types.Class, fieldName string) (*MacroProtocol, *PropertySpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	idx, ok := r.properties[fieldName]
	if !ok {
		return nil, nil, false
	}

	// Verify the class actually has this macro's decorator applied.
	// We check via the MacroDecorator field on the class type.
	if cls.MacroDecorator != idx.Protocol.Name {
		return nil, nil, false
	}

	return idx.Protocol, idx.Spec, true
}

// LookupMethod checks if a method name is valid on a macro-injected property.
// Called when the compiler sees `User.objects.filter(...)` — it first resolves
// "objects" via LookupProperty, then resolves "filter" via this method.
func (r *MacroRegistry) LookupMethod(prop *PropertySpec, methodName string) (*MethodSpec, bool) {
	if prop == nil || prop.Methods == nil {
		return nil, false
	}
	m, ok := prop.Methods[methodName]
	if !ok {
		return nil, false
	}
	return &m, true
}

// LookupMethodOnClass is a convenience method that checks if a method name
// is valid on ANY macro-injected property of the given class.
// Used when the checker sees `User.objects.filter()` — it needs to know
// if "filter" is a valid method on any property of the class's macro protocol.
func (r *MacroRegistry) LookupMethodOnClass(cls *types.Class, methodName string) (*PropertySpec, *MethodSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if cls.MacroDecorator == "" {
		return nil, nil, false
	}

	proto, ok := r.protocols[cls.MacroDecorator]
	if !ok {
		return nil, nil, false
	}

	for _, prop := range proto.Properties {
		if m, ok := prop.Methods[methodName]; ok {
			return prop, &m, true
		}
	}
	return nil, nil, false
}

// FindBinaryOp finds a matching binary operator overload for the given op.
// Returns nil if no macro op matches.
func (r *MacroRegistry) FindBinaryOp(op string, lhs, rhs ast.Expr, info interface{}) *MacroOp {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, macroOp := range r.operators[op] {
		if macroOp.LhsCheck != nil && macroOp.LhsCheck(lhs, info) {
			return macroOp
		}
		if macroOp.RhsCheck != nil && macroOp.RhsCheck(rhs, info) {
			return macroOp
		}
	}
	return nil
}

// FindUnaryOp finds a matching unary operator overload for the given op.
func (r *MacroRegistry) FindUnaryOp(op string, operand ast.Expr, info interface{}) *MacroUnaryOp {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, unaryOp := range r.unaryOps[op] {
		if unaryOp.Check != nil && unaryOp.Check(operand, info) {
			return unaryOp
		}
	}
	return nil
}

// HasProtocol checks if any protocol is registered for the given decorator name.
func (r *MacroRegistry) HasProtocol(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.protocols[name]
	return ok
}

// GetClassProtocol returns the macro protocol applied to a class, or nil.
// Uses the class's MacroDecorator field to find its protocol.
func (r *MacroRegistry) GetClassProtocol(cls *types.Class) *MacroProtocol {
	if cls == nil || cls.MacroDecorator == "" {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.protocols[cls.MacroDecorator]
}

// IsStrippable returns true if the named macro should be stripped in release builds.
// Used by the lowering phase to decide whether to emit instrumentation code.
func (r *MacroRegistry) IsStrippable(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if proto, ok := r.protocols[name]; ok {
		return proto.StripInRelease
	}
	return false
}
