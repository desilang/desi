// Package lsp implements the Desi Language Server.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/parse"
)

// Server is the LSP server.
type Server struct {
	mu               sync.Mutex
	documents        map[string]*Document // URI -> document
	rootURI          string
	workspaceFolders []WorkspaceFolder
	log              *log.Logger
	writer           io.Writer
}

// Document represents an open document.
type Document struct {
	URI     string
	Version int
	Text    string
	Module  *ast.Module  // Parsed AST
	Info    *check.Info  // Type info
	Diags   []Diagnostic // Current diagnostics
}

// New creates a new LSP server.
func New(logFile string) *Server {
	var logger *log.Logger
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			logger = log.New(f, "[desilsp] ", log.LstdFlags)
		}
	}
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Server{
		documents: make(map[string]*Document),
		log:       logger,
		writer:    os.Stdout,
	}
}

// Run starts the server on stdin/stdout.
func (s *Server) Run() error {
	s.log.Println("Desi LSP server starting...")
	reader := bufio.NewReader(os.Stdin)

	for {
		// Read headers
		contentLength := 0
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break // End of headers
			}
			if strings.HasPrefix(line, "Content-Length:") {
				length := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
				contentLength, _ = strconv.Atoi(length)
			}
		}

		if contentLength == 0 {
			continue
		}

		// Read body
		body := make([]byte, contentLength)
		_, err := io.ReadFull(reader, body)
		if err != nil {
			return err
		}

		s.log.Printf("Received: %s", string(body))

		// Parse request
		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			s.sendError(nil, ParseError, "Parse error")
			continue
		}

		// Handle request
		s.handleRequest(&req)
	}
}

func (s *Server) handleRequest(req *Request) {
	s.log.Printf("Method: %s", req.Method)

	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		// Nothing to do
	case "shutdown":
		s.sendResult(req.ID, nil)
	case "exit":
		os.Exit(0)
	case "textDocument/didOpen":
		s.handleDidOpen(req)
	case "textDocument/didChange":
		s.handleDidChange(req)
	case "textDocument/didClose":
		s.handleDidClose(req)
	case "textDocument/didSave":
		s.handleDidSave(req)
	case "textDocument/hover":
		s.handleHover(req)
	case "textDocument/definition":
		s.handleDefinition(req)
	case "textDocument/references":
		s.handleReferences(req)
	case "textDocument/documentSymbol":
		s.handleDocumentSymbol(req)
	case "textDocument/completion":
		s.handleCompletion(req)
	case "textDocument/signatureHelp":
		s.handleSignatureHelp(req)
	case "textDocument/rename":
		s.handleRename(req)
	case "textDocument/codeAction":
		s.handleCodeAction(req)
	case "textDocument/formatting":
		s.handleFormatting(req)
	case "textDocument/semanticTokens/full":
		s.handleSemanticTokens(req)
	case "textDocument/inlayHint":
		s.handleInlayHint(req)
	case "workspace/symbol":
		s.handleWorkspaceSymbol(req)
	case "textDocument/prepareCallHierarchy":
		s.handlePrepareCallHierarchy(req)
	case "callHierarchy/incomingCalls":
		s.handleIncomingCalls(req)
	case "callHierarchy/outgoingCalls":
		s.handleOutgoingCalls(req)
	default:
		if req.ID != nil {
			s.sendError(req.ID, MethodNotFound, "Method not found: "+req.Method)
		}
	}
}

func (s *Server) handleInitialize(req *Request) {
	// Parse params
	paramsJSON, _ := json.Marshal(req.Params)
	var params InitializeParams
	json.Unmarshal(paramsJSON, &params)

	s.rootURI = params.RootURI
	s.workspaceFolders = params.WorkspaceFolders
	s.log.Printf("Root URI: %s, Workspace Folders: %d", s.rootURI, len(s.workspaceFolders))

	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync:       2, // Incremental sync
			HoverProvider:          true,
			DefinitionProvider:     true,
			ReferencesProvider:     true,
			DocumentSymbolProvider: true,
			CompletionProvider: &CompletionOptions{
				TriggerCharacters: []string{"."},
			},
			SignatureHelpProvider: &SignatureHelpOptions{
				TriggerCharacters: []string{"(", ","},
			},
			RenameProvider:             true,
			CodeActionProvider:         true,
			DocumentFormattingProvider: true,
			SemanticTokensProvider: &SemanticTokensOptions{
				Legend: SemanticTokensLegend{
					TokenTypes: []string{
						"namespace", "type", "class", "enum", "interface",
						"struct", "typeParameter", "parameter", "variable",
						"property", "enumMember", "function", "method",
						"macro", "keyword", "comment", "string", "number", "operator",
					},
					TokenModifiers: []string{"declaration", "definition", "readonly"},
				},
				Full: true,
			},
			InlayHintProvider:       true,
			WorkspaceSymbolProvider: true,
			CallHierarchyProvider:   true,
		},
	}
	s.sendResult(req.ID, result)
}

func (s *Server) handleDidOpen(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params struct {
		TextDocument TextDocumentItem `json:"textDocument"`
	}
	json.Unmarshal(paramsJSON, &params)

	doc := &Document{
		URI:     params.TextDocument.URI,
		Version: params.TextDocument.Version,
		Text:    params.TextDocument.Text,
	}

	s.mu.Lock()
	s.documents[doc.URI] = doc
	s.mu.Unlock()

	s.analyzeAndPublish(doc)
}

func (s *Server) handleDidChange(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params struct {
		TextDocument   VersionedTextDocumentIdentifier `json:"textDocument"`
		ContentChanges []struct {
			Range *Range `json:"range,omitempty"` // nil for full sync
			Text  string `json:"text"`
		} `json:"contentChanges"`
	}
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	if ok && len(params.ContentChanges) > 0 {
		doc.Version = params.TextDocument.Version

		for _, change := range params.ContentChanges {
			if change.Range == nil {
				// Full sync: replace entire content
				doc.Text = change.Text
			} else {
				// Incremental sync: apply range-based edit
				doc.Text = applyTextEdit(doc.Text, *change.Range, change.Text)
			}
		}
	}
	s.mu.Unlock()

	if ok {
		s.analyzeAndPublish(doc)
	}
}

func (s *Server) handleDidClose(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params struct {
		TextDocument TextDocumentIdentifier `json:"textDocument"`
	}
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	delete(s.documents, params.TextDocument.URI)
	s.mu.Unlock()

	// Clear diagnostics
	s.publishDiagnostics(params.TextDocument.URI, nil, []Diagnostic{})
}

// handleDidSave triggers project-wide diagnostics on save.
// Efficient: only re-analyzes open documents, not entire workspace.
func (s *Server) handleDidSave(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params struct {
		TextDocument TextDocumentIdentifier `json:"textDocument"`
	}
	json.Unmarshal(paramsJSON, &params)

	// Re-analyze saved document
	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.Unlock()

	if ok {
		s.analyzeAndPublish(doc)
	}

	// Optional: analyze other open documents that may import this file
	// This is bounded by # of open documents, not workspace size
	s.mu.Lock()
	docs := make([]*Document, 0, len(s.documents))
	for uri, d := range s.documents {
		if uri != params.TextDocument.URI {
			docs = append(docs, d)
		}
	}
	s.mu.Unlock()

	for _, d := range docs {
		s.analyzeAndPublish(d)
	}
}

func (s *Server) handleHover(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params TextDocumentPositionParams
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.Unlock()

	if !ok || doc.Info == nil {
		s.sendResult(req.ID, nil)
		return
	}

	// Find type at position
	typeStr := s.findTypeAtPosition(doc, params.Position)
	if typeStr == "" {
		s.sendResult(req.ID, nil)
		return
	}

	hover := Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: "```desi\n" + typeStr + "\n```",
		},
	}
	s.sendResult(req.ID, hover)
}

func (s *Server) handleDefinition(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params TextDocumentPositionParams
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.Unlock()

	if !ok || doc.Info == nil {
		s.sendResult(req.ID, nil)
		return
	}

	loc := s.findDefinition(doc, params.Position)
	s.sendResult(req.ID, loc)
}

func (s *Server) handleReferences(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params TextDocumentPositionParams
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.Unlock()

	if !ok || doc.Info == nil {
		s.sendResult(req.ID, []Location{})
		return
	}

	refs := s.findReferences(doc, params.Position)
	s.sendResult(req.ID, refs)
}

func (s *Server) handleDocumentSymbol(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params struct {
		TextDocument TextDocumentIdentifier `json:"textDocument"`
	}
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.Unlock()

	if !ok || doc.Module == nil {
		s.sendResult(req.ID, []DocumentSymbol{})
		return
	}

	symbols := s.getDocumentSymbols(doc)
	s.sendResult(req.ID, symbols)
}

// handleCompletion provides auto-completion suggestions.
func (s *Server) handleCompletion(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params TextDocumentPositionParams
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.Unlock()

	if !ok {
		s.sendResult(req.ID, CompletionList{Items: []CompletionItem{}})
		return
	}

	items := s.getCompletions(doc, params.Position)
	s.sendResult(req.ID, CompletionList{
		IsIncomplete: false,
		Items:        items,
	})
}

// handleSignatureHelp provides function parameter hints.
func (s *Server) handleSignatureHelp(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params TextDocumentPositionParams
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.Unlock()

	if !ok || doc.Info == nil {
		s.sendResult(req.ID, nil)
		return
	}

	sig := s.getSignatureHelp(doc, params.Position)
	s.sendResult(req.ID, sig)
}

// handleRename renames a symbol across the document.
func (s *Server) handleRename(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params RenameParams
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.Unlock()

	if !ok || doc.Info == nil {
		s.sendResult(req.ID, WorkspaceEdit{})
		return
	}

	// Find all references and create edits
	refs := s.findReferences(doc, params.Position)
	edits := make([]TextEdit, 0, len(refs))
	for _, ref := range refs {
		edits = append(edits, TextEdit{
			Range:   ref.Range,
			NewText: params.NewName,
		})
	}

	result := WorkspaceEdit{
		Changes: map[string][]TextEdit{
			params.TextDocument.URI: edits,
		},
	}
	s.sendResult(req.ID, result)
}

// handleCodeAction provides quick fixes for diagnostics.
func (s *Server) handleCodeAction(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params CodeActionParams
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.Unlock()

	if !ok {
		s.sendResult(req.ID, []CodeAction{})
		return
	}

	var actions []CodeAction

	// Generate quick fixes for each diagnostic in the context
	for _, clientDiag := range params.Context.Diagnostics {
		// Look up suggestions from our diagnostic catalog
		if entry, ok := diag.Lookup(clientDiag.Code); ok {
			for _, sugg := range entry.Suggestions {
				if sugg.Replacement != "" {
					actions = append(actions, CodeAction{
						Title:       sugg.Label,
						Kind:        CodeActionKindQuickFix,
						Diagnostics: []Diagnostic{clientDiag},
						Edit: &WorkspaceEdit{
							Changes: map[string][]TextEdit{
								params.TextDocument.URI: {
									{
										Range:   clientDiag.Range,
										NewText: sugg.Replacement,
									},
								},
							},
						},
					})
				}
			}
		}
	}

	// Also check local diagnostics for any matching quick fixes
	for _, localDiag := range doc.Diags {
		if posInRange(Position{Line: params.Range.Start.Line, Character: params.Range.Start.Character}, localDiag.Range) {
			if entry, ok := diag.Lookup(localDiag.Code); ok {
				for _, sugg := range entry.Suggestions {
					if sugg.Label != "" && sugg.Message != "" {
						actions = append(actions, CodeAction{
							Title:       sugg.Label + ": " + sugg.Message,
							Kind:        CodeActionKindQuickFix,
							Diagnostics: []Diagnostic{localDiag},
						})
					}
				}
			}
		}
	}

	s.sendResult(req.ID, actions)
}

// handleFormatting formats the entire document.
func (s *Server) handleFormatting(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params DocumentFormattingParams
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.Unlock()

	if !ok {
		s.sendResult(req.ID, []TextEdit{})
		return
	}

	// Format using desifmt (we inline the formatting logic here)
	formatted := s.formatDocument(doc.Text, params.Options)
	if formatted == doc.Text {
		// No changes needed
		s.sendResult(req.ID, []TextEdit{})
		return
	}

	// Calculate full document range
	lines := strings.Split(doc.Text, "\n")
	lastLine := len(lines) - 1
	lastChar := 0
	if lastLine >= 0 {
		lastChar = len(lines[lastLine])
	}

	edits := []TextEdit{
		{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: lastLine, Character: lastChar},
			},
			NewText: formatted,
		},
	}
	s.sendResult(req.ID, edits)
}

// formatDocument applies basic formatting to the document text.
func (s *Server) formatDocument(text string, opts FormattingOptions) string {
	// Basic formatting: normalize indentation and trailing whitespace
	lines := strings.Split(text, "\n")
	var result []string
	for _, line := range lines {
		// Trim trailing whitespace
		trimmed := strings.TrimRight(line, " \t")
		result = append(result, trimmed)
	}
	return strings.Join(result, "\n")
}

// handleSemanticTokens provides semantic highlighting data.
func (s *Server) handleSemanticTokens(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params SemanticTokensParams
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.Unlock()

	if !ok || doc.Module == nil {
		s.sendResult(req.ID, SemanticTokens{Data: []int{}})
		return
	}

	tokens := s.getSemanticTokens(doc)
	s.sendResult(req.ID, SemanticTokens{Data: tokens})
}

// handleInlayHint provides inline type hints.
func (s *Server) handleInlayHint(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params InlayHintParams
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.Unlock()

	if !ok || doc.Info == nil {
		s.sendResult(req.ID, []InlayHint{})
		return
	}

	hints := s.getInlayHints(doc, params.Range)
	s.sendResult(req.ID, hints)
}

// getSemanticTokens generates encoded semantic token data.
func (s *Server) getSemanticTokens(doc *Document) []int {
	var data []int
	prevLine := 0
	prevChar := 0

	// Walk declarations and add tokens
	for _, decl := range doc.Module.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == "__top__" {
				continue
			}
			// Function name
			span := d.Name.Span
			line := span.Start.Line - 1
			char := span.Start.Col - 1
			length := len(d.Name.Name)

			// Encode: deltaLine, deltaChar, length, tokenType, modifiers
			data = append(data, line-prevLine)
			if line == prevLine {
				data = append(data, char-prevChar)
			} else {
				data = append(data, char)
			}
			data = append(data, length, SemanticTokenTypeFunction, 1) // 1 = declaration modifier
			prevLine = line
			prevChar = char

		case *ast.ClassDecl:
			span := d.Name.Span
			line := span.Start.Line - 1
			char := span.Start.Col - 1
			length := len(d.Name.Name)

			data = append(data, line-prevLine)
			if line == prevLine {
				data = append(data, char-prevChar)
			} else {
				data = append(data, char)
			}
			data = append(data, length, SemanticTokenTypeClass, 1)
			prevLine = line
			prevChar = char

		case *ast.StructDecl:
			span := d.Name.Span
			line := span.Start.Line - 1
			char := span.Start.Col - 1
			length := len(d.Name.Name)

			data = append(data, line-prevLine)
			if line == prevLine {
				data = append(data, char-prevChar)
			} else {
				data = append(data, char)
			}
			data = append(data, length, SemanticTokenTypeStruct, 1)
			prevLine = line
			prevChar = char

		case *ast.EnumDecl:
			span := d.Name.Span
			line := span.Start.Line - 1
			char := span.Start.Col - 1
			length := len(d.Name.Name)

			data = append(data, line-prevLine)
			if line == prevLine {
				data = append(data, char-prevChar)
			} else {
				data = append(data, char)
			}
			data = append(data, length, SemanticTokenTypeEnum, 1)
			prevLine = line
			prevChar = char
		}
	}

	return data
}

// getInlayHints generates inline type hints for variable declarations.
func (s *Server) getInlayHints(doc *Document, r Range) []InlayHint {
	var hints []InlayHint

	if doc.Info == nil {
		return hints
	}

	// Walk identifiers and add type hints for variables without explicit types
	for ident, sym := range doc.Info.Idents {
		if sym == nil || sym.Type == nil {
			continue
		}

		span := ident.SpanOf()
		lspRange := diagSpanToRange(span)

		// Check if in requested range
		if lspRange.Start.Line < r.Start.Line || lspRange.End.Line > r.End.Line {
			continue
		}

		// Add type hint after the identifier
		hints = append(hints, InlayHint{
			Position: Position{
				Line:      lspRange.End.Line,
				Character: lspRange.End.Character,
			},
			Label: ": " + sym.Type.String(),
			Kind:  InlayHintKindType,
		})
	}

	return hints
}

// handleWorkspaceSymbol searches for symbols across all open documents.
func (s *Server) handleWorkspaceSymbol(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params WorkspaceSymbolParams
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	defer s.mu.Unlock()

	var symbols []SymbolInformation
	query := strings.ToLower(params.Query)

	// Search all open documents
	for uri, doc := range s.documents {
		if doc.Module == nil {
			continue
		}

		for _, decl := range doc.Module.Decls {
			var name string
			var kind int
			var span diag.Span

			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.Name == "__top__" {
					continue
				}
				name = d.Name.Name
				kind = SymbolKindFunction
				span = d.Name.Span
			case *ast.ClassDecl:
				name = d.Name.Name
				kind = SymbolKindClass
				span = d.Name.Span
			case *ast.StructDecl:
				name = d.Name.Name
				kind = SymbolKindStruct
				span = d.Name.Span
			case *ast.EnumDecl:
				name = d.Name.Name
				kind = SymbolKindEnum
				span = d.Name.Span
			default:
				continue
			}

			// Filter by query (case-insensitive substring match)
			if query != "" && !strings.Contains(strings.ToLower(name), query) {
				continue
			}

			symbols = append(symbols, SymbolInformation{
				Name: name,
				Kind: kind,
				Location: Location{
					URI:   uri,
					Range: diagSpanToRange(span),
				},
			})
		}
	}

	s.sendResult(req.ID, symbols)
}

// handlePrepareCallHierarchy prepares call hierarchy for a position.
func (s *Server) handlePrepareCallHierarchy(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params CallHierarchyPrepareParams
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	s.mu.Unlock()

	if !ok || doc.Module == nil {
		s.sendResult(req.ID, []CallHierarchyItem{})
		return
	}

	// Find function at position
	for _, decl := range doc.Module.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name == "__top__" {
			continue
		}

		span := fd.Name.Span
		lspRange := diagSpanToRange(span)

		// Check if position is within the function name
		if params.Position.Line >= lspRange.Start.Line && params.Position.Line <= lspRange.End.Line {
			item := CallHierarchyItem{
				Name:           fd.Name.Name,
				Kind:           SymbolKindFunction,
				URI:            params.TextDocument.URI,
				Range:          diagSpanToRange(fd.Span),
				SelectionRange: lspRange,
				Data:           fd.Name.Name,
			}
			s.sendResult(req.ID, []CallHierarchyItem{item})
			return
		}
	}

	s.sendResult(req.ID, []CallHierarchyItem{})
}

// handleIncomingCalls finds callers of a function.
// TODO: Implement full AST walking to find call sites.
func (s *Server) handleIncomingCalls(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params CallHierarchyIncomingCallsParams
	json.Unmarshal(paramsJSON, &params)

	// For now, return empty - full implementation requires AST walking
	s.sendResult(req.ID, []CallHierarchyIncomingCall{})
}

// handleOutgoingCalls finds functions called by this function.
// TODO: Implement full AST walking to find call sites.
func (s *Server) handleOutgoingCalls(req *Request) {
	paramsJSON, _ := json.Marshal(req.Params)
	var params CallHierarchyOutgoingCallsParams
	json.Unmarshal(paramsJSON, &params)

	// For now, return empty - full implementation requires AST walking
	s.sendResult(req.ID, []CallHierarchyOutgoingCall{})
}

// analyzeAndPublish parses and type-checks a document.
func (s *Server) analyzeAndPublish(doc *Document) {
	// Convert URI to filename
	filename := uriToPath(doc.URI)

	// Parse
	mod, parseErrs := parse.ParseFile(filename, []byte(doc.Text))
	var diags []Diagnostic

	for _, err := range parseErrs {
		diags = append(diags, Diagnostic{
			Range:    diagSpanToRange(err.Primary.Span),
			Severity: codeIDToSeverity(err.CodeID),
			Code:     err.CodeID,
			Source:   "desi",
			Message:  err.Message,
		})
	}

	if mod != nil {
		doc.Module = mod
		// Type check
		checkDiags, info := check.Check(mod)
		doc.Info = info
		for _, d := range checkDiags {
			diags = append(diags, Diagnostic{
				Range:    diagSpanToRange(d.Primary.Span),
				Severity: codeIDToSeverity(d.CodeID),
				Code:     d.CodeID,
				Source:   "desi",
				Message:  d.Message,
			})
		}
	}

	doc.Diags = diags
	s.publishDiagnostics(doc.URI, &doc.Version, diags)
}

func (s *Server) publishDiagnostics(uri string, version *int, diags []Diagnostic) {
	params := PublishDiagnosticsParams{
		URI:         uri,
		Version:     version,
		Diagnostics: diags,
	}
	s.sendNotification("textDocument/publishDiagnostics", params)
}

// findTypeAtPosition looks up the type at a given position.
func (s *Server) findTypeAtPosition(doc *Document, pos Position) string {
	if doc.Info == nil {
		return ""
	}

	// Walk Info.Types to find matching span
	for expr, t := range doc.Info.Types {
		span := expr.SpanOf()
		r := diagSpanToRange(span)
		if posInRange(pos, r) {
			return t.String()
		}
	}
	return ""
}

// findDefinition finds the definition location for a symbol.
func (s *Server) findDefinition(doc *Document, pos Position) *Location {
	if doc.Info == nil {
		return nil
	}

	// Walk Info.Idents to find matching span
	for ident, sym := range doc.Info.Idents {
		span := ident.SpanOf()
		r := diagSpanToRange(span)
		if posInRange(pos, r) && sym != nil {
			// Check if this is an imported symbol
			if sym.Node != nil {
				declSpan := sym.Node.SpanOf()
				return &Location{
					URI:   doc.URI,
					Range: diagSpanToRange(declSpan),
				}
			}

			// Check for cross-file import
			if doc.Info.ImportPaths != nil {
				if modPath, ok := doc.Info.ImportPaths[ident.Name]; ok {
					// Resolve module path to file URI
					fileURI := s.resolveModuleURI(modPath)
					if fileURI != "" {
						return &Location{
							URI:   fileURI,
							Range: Range{Start: Position{0, 0}, End: Position{0, 0}},
						}
					}
				}
			}
		}
	}
	return nil
}

// resolveModuleURI converts a module path to a file URI.
func (s *Server) resolveModuleURI(modPath string) string {
	// Convert dotted path to file path
	filePath := strings.ReplaceAll(modPath, ".", "/") + ".desi"

	// Try relative to root
	if s.rootURI != "" {
		rootPath := uriToPath(s.rootURI)
		fullPath := rootPath + "/" + filePath
		return pathToURI(fullPath)
	}
	return ""
}

// findReferences finds all references to a symbol.
func (s *Server) findReferences(doc *Document, pos Position) []Location {
	if doc.Info == nil {
		return nil
	}

	// Find the symbol at position
	var targetSym *check.Symbol
	for ident, sym := range doc.Info.Idents {
		span := ident.SpanOf()
		r := diagSpanToRange(span)
		if posInRange(pos, r) {
			targetSym = sym
			break
		}
	}

	if targetSym == nil {
		return nil
	}

	// Find all refs to this symbol
	var refs []Location
	for ident, sym := range doc.Info.Idents {
		if sym.Name == targetSym.Name {
			span := ident.SpanOf()
			refs = append(refs, Location{
				URI:   doc.URI,
				Range: diagSpanToRange(span),
			})
		}
	}
	return refs
}

// getDocumentSymbols returns all symbols in a document.
func (s *Server) getDocumentSymbols(doc *Document) []DocumentSymbol {
	if doc.Module == nil {
		return nil
	}

	var symbols []DocumentSymbol
	for _, decl := range doc.Module.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == "__top__" {
				continue // Skip synthetic __top__ blocks
			}
			symbols = append(symbols, DocumentSymbol{
				Name:           d.Name.Name,
				Kind:           SymbolKindFunction,
				Range:          diagSpanToRange(d.Span),
				SelectionRange: diagSpanToRange(d.Name.Span),
			})
		case *ast.ClassDecl:
			symbols = append(symbols, DocumentSymbol{
				Name:           d.Name.Name,
				Kind:           SymbolKindClass,
				Range:          diagSpanToRange(d.Span),
				SelectionRange: diagSpanToRange(d.Name.Span),
			})
		case *ast.StructDecl:
			symbols = append(symbols, DocumentSymbol{
				Name:           d.Name.Name,
				Kind:           SymbolKindStruct,
				Range:          diagSpanToRange(d.Span),
				SelectionRange: diagSpanToRange(d.Name.Span),
			})
		case *ast.EnumDecl:
			symbols = append(symbols, DocumentSymbol{
				Name:           d.Name.Name,
				Kind:           SymbolKindEnum,
				Range:          diagSpanToRange(d.Span),
				SelectionRange: diagSpanToRange(d.Name.Span),
			})
		}
	}
	return symbols
}

// getCompletions generates completion items for a document position.
func (s *Server) getCompletions(doc *Document, pos Position) []CompletionItem {
	var items []CompletionItem

	// Add Desi keywords
	keywords := []string{
		"def", "class", "struct", "enum", "trait", "impl",
		"if", "elif", "else", "for", "while", "match",
		"return", "break", "continue", "pass",
		"let", "mut", "pub", "async", "await",
		"import", "from", "as",
		"true", "false", "none",
		"and", "or", "not", "in", "is",
		"try", "except", "finally", "raise",
		"using", "defer", "unsafe",
	}
	for _, kw := range keywords {
		items = append(items, CompletionItem{
			Label: kw,
			Kind:  CompletionKindKeyword,
		})
	}

	// Add symbols from the document
	if doc.Info != nil {
		// Add functions
		for name, overloads := range doc.Info.Funcs {
			if name == "__top__" {
				continue
			}
			detail := ""
			if overloads != nil && len(overloads.Cands) > 0 && overloads.Cands[0].Type != nil {
				detail = overloads.Cands[0].Type.String()
			}
			items = append(items, CompletionItem{
				Label:  name,
				Kind:   CompletionKindFunction,
				Detail: detail,
			})
		}

		// Add types from declarations
		if doc.Module != nil {
			for _, decl := range doc.Module.Decls {
				switch d := decl.(type) {
				case *ast.ClassDecl:
					items = append(items, CompletionItem{
						Label: d.Name.Name,
						Kind:  CompletionKindClass,
					})
				case *ast.StructDecl:
					items = append(items, CompletionItem{
						Label: d.Name.Name,
						Kind:  CompletionKindStruct,
					})
				case *ast.EnumDecl:
					items = append(items, CompletionItem{
						Label: d.Name.Name,
						Kind:  CompletionKindEnum,
					})
				}
			}
		}
	}

	return items
}

// getSignatureHelp returns signature help for a function call.
func (s *Server) getSignatureHelp(doc *Document, pos Position) *SignatureHelp {
	if doc.Info == nil || doc.Module == nil {
		return nil
	}

	// Walk through expressions to find call at/near position
	// This is a simplified implementation - a real one would track parentheses
	for expr, t := range doc.Info.Types {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			continue
		}
		span := call.SpanOf()
		r := diagSpanToRange(span)
		if posInRange(pos, r) {
			// Found a call, get function info
			if fn, ok := call.Callee.(*ast.Ident); ok {
				if overloads, ok := doc.Info.Funcs[fn.Name]; ok && len(overloads.Cands) > 0 {
					cand := overloads.Cands[0]
					if cand.Type != nil {
						var params []ParameterInfo
						for i, p := range cand.Type.Params {
							label := fmt.Sprintf("param%d", i)
							if cand.Decl != nil && i < len(cand.Decl.Params) {
								label = cand.Decl.Params[i].Name.Name
							}
							params = append(params, ParameterInfo{Label: label + ": " + p.String()})
						}
						return &SignatureHelp{
							Signatures: []SignatureInfo{
								{
									Label:      fn.Name + "(" + t.String() + ")",
									Parameters: params,
								},
							},
							ActiveSignature: 0,
							ActiveParameter: len(call.Args), // Approximation
						}
					}
				}
			}
		}
	}
	return nil
}

// diagSpanToRange converts a diag.Span to LSP Range.
func diagSpanToRange(span diag.Span) Range {
	return Range{
		Start: Position{
			Line:      span.Start.Line - 1, // LSP is 0-indexed
			Character: span.Start.Col - 1,
		},
		End: Position{
			Line:      span.End.Line - 1,
			Character: span.End.Col - 1,
		},
	}
}

// posInRange checks if a position is within a range.
func posInRange(pos Position, r Range) bool {
	if pos.Line < r.Start.Line || pos.Line > r.End.Line {
		return false
	}
	if pos.Line == r.Start.Line && pos.Character < r.Start.Character {
		return false
	}
	if pos.Line == r.End.Line && pos.Character > r.End.Character {
		return false
	}
	return true
}

// applyTextEdit applies an incremental text edit to a document.
// Converts LSP line/character positions to byte offsets and splices in new text.
func applyTextEdit(text string, r Range, newText string) string {
	lines := strings.Split(text, "\n")

	// Convert start position to offset
	startOffset := 0
	for i := 0; i < r.Start.Line && i < len(lines); i++ {
		startOffset += len(lines[i]) + 1 // +1 for newline
	}
	if r.Start.Line < len(lines) {
		startOffset += min(r.Start.Character, len(lines[r.Start.Line]))
	}

	// Convert end position to offset
	endOffset := 0
	for i := 0; i < r.End.Line && i < len(lines); i++ {
		endOffset += len(lines[i]) + 1
	}
	if r.End.Line < len(lines) {
		endOffset += min(r.End.Character, len(lines[r.End.Line]))
	}

	// Ensure offsets are within bounds
	if startOffset > len(text) {
		startOffset = len(text)
	}
	if endOffset > len(text) {
		endOffset = len(text)
	}

	// Apply the edit
	return text[:startOffset] + newText + text[endOffset:]
}

// codeIDToSeverity maps Desi code ID prefixes to LSP severities.
func codeIDToSeverity(codeID string) int {
	if strings.HasPrefix(codeID, "DW") {
		return SeverityWarning
	}
	if strings.HasPrefix(codeID, "DH") {
		return SeverityHint
	}
	// DTE, DSY, DFI, etc. are errors
	return SeverityError
}

// uriToPath converts a file:// URI to a filesystem path.
func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}

// pathToURI converts a filesystem path to a file:// URI.
func pathToURI(path string) string {
	if !strings.HasPrefix(path, "file://") {
		return "file://" + path
	}
	return path
}

// --- JSON-RPC Helpers ---

func (s *Server) sendResult(id interface{}, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.send(resp)
}

func (s *Server) sendError(id interface{}, code int, message string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &Error{Code: code, Message: message},
	}
	s.send(resp)
}

func (s *Server) sendNotification(method string, params interface{}) {
	notif := Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	s.send(notif)
}

func (s *Server) send(msg interface{}) {
	body, _ := json.Marshal(msg)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	s.writer.Write([]byte(header))
	s.writer.Write(body)
	s.log.Printf("Sent: %s", string(body))
}
