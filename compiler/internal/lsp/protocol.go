// Package lsp implements LSP protocol types and utilities.
package lsp

// Position in a text document (0-indexed line and character).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range in a text document.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location links a range to a document URI.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier identifies a text document.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentItem is a document opened in the editor.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// VersionedTextDocumentIdentifier identifies a specific version.
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// TextDocumentPositionParams identifies a position in a document.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// --- Diagnostics ---

// DiagnosticSeverity levels.
const (
	SeverityError       = 1
	SeverityWarning     = 2
	SeverityInformation = 3
	SeverityHint        = 4
)

// Diagnostic represents a compiler error or warning.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

// PublishDiagnosticsParams is sent from server to client.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// --- Initialize ---

// WorkspaceFolder represents a root folder in a multi-root workspace.
type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// InitializeParams is sent by client on startup.
type InitializeParams struct {
	ProcessID        *int              `json:"processId"`
	RootURI          string            `json:"rootUri"`
	WorkspaceFolders []WorkspaceFolder `json:"workspaceFolders,omitempty"`
	Capabilities     interface{}       `json:"capabilities"`
}

// ServerCapabilities advertises what the server can do.
type ServerCapabilities struct {
	TextDocumentSync           int                    `json:"textDocumentSync"` // 1=Full, 2=Incremental
	HoverProvider              bool                   `json:"hoverProvider"`
	DefinitionProvider         bool                   `json:"definitionProvider"`
	ReferencesProvider         bool                   `json:"referencesProvider"`
	DocumentSymbolProvider     bool                   `json:"documentSymbolProvider"`
	CompletionProvider         *CompletionOptions     `json:"completionProvider,omitempty"`
	SignatureHelpProvider      *SignatureHelpOptions  `json:"signatureHelpProvider,omitempty"`
	RenameProvider             bool                   `json:"renameProvider,omitempty"`
	CodeActionProvider         bool                   `json:"codeActionProvider,omitempty"`
	DocumentFormattingProvider bool                   `json:"documentFormattingProvider,omitempty"`
	SemanticTokensProvider     *SemanticTokensOptions `json:"semanticTokensProvider,omitempty"`
	InlayHintProvider          bool                   `json:"inlayHintProvider,omitempty"`
	WorkspaceSymbolProvider    bool                   `json:"workspaceSymbolProvider,omitempty"`
	CallHierarchyProvider      bool                   `json:"callHierarchyProvider,omitempty"`
}

// InitializeResult is the response to initialize.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

// --- Hover ---

// Hover result with markdown content.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// MarkupContent is markdown or plaintext.
type MarkupContent struct {
	Kind  string `json:"kind"` // "markdown" or "plaintext"
	Value string `json:"value"`
}

// --- Document Symbols ---

// SymbolKind values.
const (
	SymbolKindFile        = 1
	SymbolKindModule      = 2
	SymbolKindNamespace   = 3
	SymbolKindPackage     = 4
	SymbolKindClass       = 5
	SymbolKindMethod      = 6
	SymbolKindProperty    = 7
	SymbolKindField       = 8
	SymbolKindConstructor = 9
	SymbolKindEnum        = 10
	SymbolKindInterface   = 11
	SymbolKindFunction    = 12
	SymbolKindVariable    = 13
	SymbolKindConstant    = 14
	SymbolKindString      = 15
	SymbolKindNumber      = 16
	SymbolKindBoolean     = 17
	SymbolKindArray       = 18
	SymbolKindStruct      = 23
)

// DocumentSymbol represents a symbol in a document.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// --- Completion ---

// CompletionItemKind values.
const (
	CompletionKindText          = 1
	CompletionKindMethod        = 2
	CompletionKindFunction      = 3
	CompletionKindConstructor   = 4
	CompletionKindField         = 5
	CompletionKindVariable      = 6
	CompletionKindClass         = 7
	CompletionKindInterface     = 8
	CompletionKindModule        = 9
	CompletionKindProperty      = 10
	CompletionKindUnit          = 11
	CompletionKindValue         = 12
	CompletionKindEnum          = 13
	CompletionKindKeyword       = 14
	CompletionKindSnippet       = 15
	CompletionKindColor         = 16
	CompletionKindFile          = 17
	CompletionKindReference     = 18
	CompletionKindFolder        = 19
	CompletionKindEnumMember    = 20
	CompletionKindConstant      = 21
	CompletionKindStruct        = 22
	CompletionKindEvent         = 23
	CompletionKindOperator      = 24
	CompletionKindTypeParameter = 25
)

// CompletionItem represents a completion suggestion.
type CompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind,omitempty"`
	Detail        string `json:"detail,omitempty"`
	Documentation string `json:"documentation,omitempty"`
	InsertText    string `json:"insertText,omitempty"`
}

// CompletionList is a collection of completion items.
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// CompletionOptions for server capabilities.
type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// --- Signature Help ---

// SignatureHelp contains active signature and parameter.
type SignatureHelp struct {
	Signatures      []SignatureInfo `json:"signatures"`
	ActiveSignature int             `json:"activeSignature"`
	ActiveParameter int             `json:"activeParameter"`
}

// SignatureInfo describes a function signature.
type SignatureInfo struct {
	Label         string          `json:"label"`
	Documentation string          `json:"documentation,omitempty"`
	Parameters    []ParameterInfo `json:"parameters,omitempty"`
}

// ParameterInfo describes a parameter.
type ParameterInfo struct {
	Label string `json:"label"`
}

// SignatureHelpOptions for server capabilities.
type SignatureHelpOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// --- Rename ---

// RenameParams sent by the client.
type RenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	NewName      string                 `json:"newName"`
}

// WorkspaceEdit contains changes to multiple documents.
type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes,omitempty"`
}

// TextEdit represents a change to a document.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// --- Code Actions ---

// CodeActionParams sent by the client.
type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

// CodeActionContext provides context for code actions.
type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
	Only        []string     `json:"only,omitempty"`
}

// CodeAction represents a quick fix or refactoring.
type CodeAction struct {
	Title       string         `json:"title"`
	Kind        string         `json:"kind,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
}

// CodeActionKind values.
const (
	CodeActionKindQuickFix       = "quickfix"
	CodeActionKindRefactor       = "refactor"
	CodeActionKindSourceOrganize = "source.organizeImports"
)

// --- Formatting ---

// DocumentFormattingParams sent by the client.
type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      FormattingOptions      `json:"options"`
}

// FormattingOptions control formatting behavior.
type FormattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}

// --- Semantic Tokens ---

// SemanticTokensLegend describes token types and modifiers.
type SemanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

// SemanticTokensOptions for server capabilities.
type SemanticTokensOptions struct {
	Legend SemanticTokensLegend `json:"legend"`
	Full   bool                 `json:"full"`
}

// SemanticTokensParams sent by the client.
type SemanticTokensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// SemanticTokens response with encoded token data.
type SemanticTokens struct {
	Data []int `json:"data"`
}

// Semantic token types (indices into legend.tokenTypes)
const (
	SemanticTokenTypeNamespace = iota
	SemanticTokenTypeType
	SemanticTokenTypeClass
	SemanticTokenTypeEnum
	SemanticTokenTypeInterface
	SemanticTokenTypeStruct
	SemanticTokenTypeTypeParameter
	SemanticTokenTypeParameter
	SemanticTokenTypeVariable
	SemanticTokenTypeProperty
	SemanticTokenTypeEnumMember
	SemanticTokenTypeFunction
	SemanticTokenTypeMethod
	SemanticTokenTypeMacro
	SemanticTokenTypeKeyword
	SemanticTokenTypeComment
	SemanticTokenTypeString
	SemanticTokenTypeNumber
	SemanticTokenTypeOperator
)

// --- Inlay Hints ---

// InlayHintParams sent by the client.
type InlayHintParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
}

// InlayHint represents an inline type hint.
type InlayHint struct {
	Position Position `json:"position"`
	Label    string   `json:"label"`
	Kind     int      `json:"kind,omitempty"`
}

// InlayHintKind values.
const (
	InlayHintKindType      = 1
	InlayHintKindParameter = 2
)

// --- Workspace Symbols ---

// WorkspaceSymbolParams sent by the client.
type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

// SymbolInformation represents a symbol in the workspace.
type SymbolInformation struct {
	Name          string   `json:"name"`
	Kind          int      `json:"kind"`
	Location      Location `json:"location"`
	ContainerName string   `json:"containerName,omitempty"`
}

// --- Call Hierarchy ---

// CallHierarchyPrepareParams sent by the client.
type CallHierarchyPrepareParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// CallHierarchyItem represents a function/method in the call hierarchy.
type CallHierarchyItem struct {
	Name           string `json:"name"`
	Kind           int    `json:"kind"`
	URI            string `json:"uri"`
	Range          Range  `json:"range"`
	SelectionRange Range  `json:"selectionRange"`
	Data           any    `json:"data,omitempty"`
}

// CallHierarchyIncomingCallsParams sent by the client.
type CallHierarchyIncomingCallsParams struct {
	Item CallHierarchyItem `json:"item"`
}

// CallHierarchyIncomingCall represents a caller.
type CallHierarchyIncomingCall struct {
	From       CallHierarchyItem `json:"from"`
	FromRanges []Range           `json:"fromRanges"`
}

// CallHierarchyOutgoingCallsParams sent by the client.
type CallHierarchyOutgoingCallsParams struct {
	Item CallHierarchyItem `json:"item"`
}

// CallHierarchyOutgoingCall represents a callee.
type CallHierarchyOutgoingCall struct {
	To         CallHierarchyItem `json:"to"`
	FromRanges []Range           `json:"fromRanges"`
}

// --- JSON-RPC ---

// Request is a JSON-RPC request.
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response is a JSON-RPC response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Notification is a JSON-RPC notification (no ID).
type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Error is a JSON-RPC error.
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error codes.
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)
