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
	mu        sync.Mutex
	documents map[string]*Document // URI -> document
	rootURI   string
	log       *log.Logger
	writer    io.Writer
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
	case "textDocument/hover":
		s.handleHover(req)
	case "textDocument/definition":
		s.handleDefinition(req)
	case "textDocument/references":
		s.handleReferences(req)
	case "textDocument/documentSymbol":
		s.handleDocumentSymbol(req)
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
	s.log.Printf("Root URI: %s", s.rootURI)

	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync:       1, // Full sync
			HoverProvider:          true,
			DefinitionProvider:     true,
			ReferencesProvider:     true,
			DocumentSymbolProvider: true,
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
			Text string `json:"text"`
		} `json:"contentChanges"`
	}
	json.Unmarshal(paramsJSON, &params)

	s.mu.Lock()
	doc, ok := s.documents[params.TextDocument.URI]
	if ok && len(params.ContentChanges) > 0 {
		doc.Version = params.TextDocument.Version
		doc.Text = params.ContentChanges[0].Text // Full sync
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
		if posInRange(pos, r) && sym.Node != nil {
			declSpan := sym.Node.SpanOf()
			return &Location{
				URI:   doc.URI,
				Range: diagSpanToRange(declSpan),
			}
		}
	}
	return nil
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
