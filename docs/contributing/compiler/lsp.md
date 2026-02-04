# LSP Server Architecture

The Desi Language Server (`desilsp`) provides IDE features via the [Language Server Protocol](https://microsoft.github.io/language-server-protocol/).

## Overview

```
┌──────────────┐      stdio       ┌─────────────────────┐
│   VS Code    │ ◄──────────────► │      desilsp        │
│   (client)   │    JSON-RPC      │  compiler/cmd/...   │
└──────────────┘                  └─────────────────────┘
                                           │
                                           ▼
                                  ┌─────────────────────┐
                                  │   parse + check     │
                                  │   (reuse compiler)  │
                                  └─────────────────────┘
```

## Key Files

| File | Purpose |
|------|---------|
| `compiler/cmd/desilsp/main.go` | Entry point, CLI args |
| `compiler/internal/lsp/protocol.go` | LSP types (Request, Response, Diagnostic) |
| `compiler/internal/lsp/server.go` | All handlers |

## Adding a New Feature

### 1. Add handler in `server.go`

```go
func (s *Server) handleCompletion(req *Request) {
    // Parse params
    // Look up Info from document
    // Build response
    s.sendResult(req.ID, completions)
}
```

### 2. Register in `handleRequest()`

```go
case "textDocument/completion":
    s.handleCompletion(req)
```

### 3. Advertise capability in `handleInitialize()`

```go
Capabilities: ServerCapabilities{
    CompletionProvider: &CompletionOptions{...},
}
```

## Testing

```bash
# Build
go build -o desilsp ./compiler/cmd/desilsp

# Test with VS Code
cd editors/vscode && npm install
code .  # F5 to launch dev host
```

## Current Capabilities

- `textDocumentSync` - Full document sync
- `hoverProvider` - Type info on hover
- `definitionProvider` - Go to definition
- `referencesProvider` - Find all references
- `documentSymbolProvider` - Outline view
