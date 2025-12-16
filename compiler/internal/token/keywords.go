package token

// keywords maps source spellings -> keyword tokens.
// Builtin types are tracked separately in builtinTypes.
var keywords = map[string]Token{
	"import":   KW_import,
	"from":     KW_from,
	"as":       KW_as,
	"pub":      KW_pub,
	"def":      KW_def,
	"async":    KW_async,
	"class":    KW_class,
	"struct":   KW_struct,
	"enum":     KW_enum,
	"trait":    KW_trait,
	"impl":     KW_impl,
	"type":     KW_type,
	"let":      KW_let,
	"mut":      KW_mut,
	"return":   KW_return,
	"if":       KW_if,
	"elif":     KW_elif,
	"else":     KW_else,
	"while":    KW_while,
	"for":      KW_for,
	"in":       KW_in,
	"using":    KW_using,
	"defer":    KW_defer,
	"match":    KW_match,
	"select":   KW_select,
	"await":    KW_await,
	"true":     KW_true,
	"false":    KW_false,
	"none":     KW_none,
	"and":      KW_and,
	"or":       KW_or,
	"not":      KW_not,
	"ref":      KW_ref,
	"inout":    KW_inout,
	"unsafe":   KW_unsafe,
	"break":    KW_break,
	"continue": KW_continue,
	"const":    KW_const,
	"static":   KW_static,
	"lambda":   KW_lambda,
}

// Builtin type spellings per revised grammar.
var builtinTypes = map[string]struct{}{
	"bool":   {},
	"int":    {},
	"i8":     {},
	"i16":    {},
	"i32":    {},
	"i64":    {},
	"i128":   {},
	"isize":  {},
	"u8":     {},
	"u16":    {},
	"u32":    {},
	"u64":    {},
	"u128":   {},
	"usize":  {},
	"f32":    {},
	"f64":    {},
	"str":    {},
	"string": {},
	"future": {},
	"none":   {},

	// Reserved words in type positions (containers).
	"list":  {},
	"dict":  {},
	"set":   {},
	"tuple": {},
}

// IsKeyword reports whether s is a reserved language keyword (non-type).
func IsKeyword(s string) bool {
	_, ok := keywords[s]
	return ok
}

// IsBuiltinType reports whether s is a builtin/reserved type name.
func IsBuiltinType(s string) bool {
	_, ok := builtinTypes[s]
	return ok
}
