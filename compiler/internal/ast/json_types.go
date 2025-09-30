package ast

/*
Stable JSON for AST comparison/parity tests.

We convert the Go AST (with interfaces) into a plain, tagged JSON tree.
Every node has a "kind" field; spans are included as {start{line,col}, end{line,col}}.
*/

type jPos struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}
type jSpan struct {
	Start jPos `json:"start"`
	End   jPos `json:"end"`
}

/* ---------- top level ---------- */

type jFile struct {
	Kind        string        `json:"kind"` // "File"
	Package     *jPackage     `json:"package,omitempty"`
	Imports     []jImport     `json:"imports,omitempty"`
	FromImports []jFromImport `json:"from_imports,omitempty"`
	Decls       []any         `json:"decls,omitempty"`
}

type jPackage struct {
	Kind string `json:"kind"` // "PackageDecl"
	Name string `json:"name"`
}

type jImport struct {
	Kind string `json:"kind"` // "ImportDecl"
	Path string `json:"path"`
	As   string `json:"as,omitempty"`
	Span jSpan  `json:"span"`
}

/*** NEW: from-imports ***/

type jFromImport struct {
	Kind   string        `json:"kind"` // "FromImportDecl"
	Module string        `json:"module"`
	Items  []jImportItem `json:"items,omitempty"`
	Span   jSpan         `json:"span"`
}

type jImportItem struct {
	Kind string `json:"kind"` // "ImportItem"
	Name string `json:"name"`
	As   string `json:"as,omitempty"`
	Span jSpan  `json:"span"`
}

/* ---------- decls ---------- */

type jFuncDecl struct {
	Kind   string   `json:"kind"` // "FuncDecl"
	Name   string   `json:"name"`
	Params []jParam `json:"params,omitempty"`
	Ret    string   `json:"ret,omitempty"`
	Body   []any    `json:"body,omitempty"`
	Pub    bool     `json:"pub,omitempty"`   // NEW (M10)
	Async  bool     `json:"async,omitempty"` // NEW (M11)
	Span   jSpan    `json:"span"`
}

type jTypeDecl struct {
	Kind       string `json:"kind"` // "TypeDecl"
	Name       string `json:"name"`
	Underlying string `json:"underlying"`
	Pub        bool   `json:"pub,omitempty"` // NEW (M10)
	Span       jSpan  `json:"span"`
}

type jStructDecl struct {
	Kind   string   `json:"kind"` // "StructDecl"
	Name   string   `json:"name"`
	Fields []jField `json:"fields"`
	Pub    bool     `json:"pub,omitempty"` // NEW (M10)
	Span   jSpan    `json:"span"`
}

type jEnumDecl struct {
	Kind     string         `json:"kind"` // "EnumDecl"
	Name     string         `json:"name"`
	Variants []jEnumVariant `json:"variants"`
	Pub      bool           `json:"pub,omitempty"` // NEW (M10)
	Span     jSpan          `json:"span"`
}

type jEnumVariant struct {
	Kind    string `json:"kind"` // "EnumVariant"
	Name    string `json:"name"`
	Payload string `json:"payload,omitempty"`
	Span    jSpan  `json:"span"`
}

type jConstDecl struct {
	Kind    string `json:"kind"` // "ConstDecl"
	Name    string `json:"name"`
	Type    string `json:"type,omitempty"`
	Value   any    `json:"value"`
	Pub     bool   `json:"pub,omitempty"`
	Mutable bool   `json:"mutable,omitempty"` // should be false for top-level
	Span    jSpan  `json:"span"`
}

type jParam struct {
	Kind string `json:"kind"` // "Param"
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	Span jSpan  `json:"span"`
}

type jField struct {
	Kind string `json:"kind"` // "Field"
	Name string `json:"name"`
	Type string `json:"type"`
	Span jSpan  `json:"span"`
}

/* ---------- stmts ---------- */

type jLetBind struct {
	Kind string `json:"kind"` // "LetBind"
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	Span jSpan  `json:"span"`
}

type jLetStmt struct {
	Kind      string     `json:"kind"` // "LetStmt"
	Mutable   bool       `json:"mutable"`
	Binds     []jLetBind `json:"binds"`
	GroupType string     `json:"groupType,omitempty"`
	Values    []any      `json:"values"`
	Span      jSpan      `json:"span"`
}

type jAssignStmt struct {
	Kind  string   `json:"kind"` // "AssignStmt"
	Names []string `json:"names"`
	Exprs []any    `json:"exprs"`
	Span  jSpan    `json:"span"`
}

type jReturnStmt struct {
	Kind string `json:"kind"` // "ReturnStmt"
	Expr any    `json:"expr,omitempty"`
	Span jSpan  `json:"span"`
}

type jExprStmt struct {
	Kind string `json:"kind"` // "ExprStmt"
	Expr any    `json:"expr"`
	Span jSpan  `json:"span"`
}

type jIfStmt struct {
	Kind  string    `json:"kind"` // "IfStmt"
	Cond  any       `json:"cond"`
	Then  []any     `json:"then"`
	Elifs []jElseIf `json:"elifs,omitempty"`
	Else  []any     `json:"else,omitempty"`
	Span  jSpan     `json:"span"`
}

type jElseIf struct {
	Kind string `json:"kind"` // "ElseIf"
	Cond any    `json:"cond"`
	Body []any  `json:"body"`
	Span jSpan  `json:"span"`
}

type jWhileStmt struct {
	Kind string `json:"kind"` // "WhileStmt"
	Cond any    `json:"cond"`
	Body []any  `json:"body"`
	Span jSpan  `json:"span"`
}

type jDeferStmt struct {
	Kind string `json:"kind"` // "DeferStmt"
	Call any    `json:"call"`
	Span jSpan  `json:"span"`
}

/* ---------- exprs ---------- */

type jIdentExpr struct {
	Kind string `json:"kind"` // "IdentExpr"
	Name string `json:"name"`
	Span jSpan  `json:"span"`
}

type jIntLit struct {
	Kind  string `json:"kind"` // "IntLit"
	Value string `json:"value"`
	Span  jSpan  `json:"span"`
}

type jStrLit struct {
	Kind  string `json:"kind"` // "StrLit"
	Value string `json:"value"`
	Span  jSpan  `json:"span"`
}

type jBoolLit struct {
	Kind  string `json:"kind"` // "BoolLit"
	Value bool   `json:"value"`
	Span  jSpan  `json:"span"`
}

type jCallExpr struct {
	Kind   string `json:"kind"` // "CallExpr"
	Callee any    `json:"callee"`
	Args   []any  `json:"args"`
	Span   jSpan  `json:"span"`
}

type jIndexExpr struct {
	Kind  string `json:"kind"` // "IndexExpr"
	Seq   any    `json:"seq"`
	Index any    `json:"index"`
	Span  jSpan  `json:"span"`
}

type jFieldExpr struct {
	Kind string `json:"kind"` // "FieldExpr"
	X    any    `json:"x"`
	Name string `json:"name"`
	Span jSpan  `json:"span"`
}

type jUnaryExpr struct {
	Kind string `json:"kind"` // "UnaryExpr"
	Op   string `json:"op"`
	X    any    `json:"x"`
	Span jSpan  `json:"span"`
}

type jBinaryExpr struct {
	Kind  string `json:"kind"` // "BinaryExpr"
	Op    string `json:"op"`
	Left  any    `json:"left"`
	Right any    `json:"right"`
	Span  jSpan  `json:"span"`
}

type jAwaitExpr struct {
	Kind string `json:"kind"` // "AwaitExpr"
	Expr any    `json:"expr"`
	Span jSpan  `json:"span"`
}
