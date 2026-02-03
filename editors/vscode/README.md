# Desi Language Extension for VS Code

Provides language support for the Desi programming language.

## Features

- **Syntax highlighting** - Keywords, strings, numbers, operators
- **Diagnostics** - Type errors and warnings as you type
- **Hover** - View type information on hover
- **Go to Definition** - Ctrl+Click to jump to definitions
- **Find References** - Find all usages of a symbol
- **Document Symbols** - List functions, classes, structs

## Installation

### Prerequisites
1. Build the language server:
   ```bash
   cd /path/to/desi
   make tools  # Builds desilsp to bin/desilsp
   ```

2. Ensure `desilsp` is in your PATH, or configure the path in settings.

### Install Extension
1. Open VS Code
2. Run: `Extensions: Install from VSIX...`
3. Or for development: `Run Extension` (F5) from this directory

## Configuration

```json
{
  "desi.lspPath": "/path/to/desilsp"
}
```

## Development

```bash
cd editors/vscode
npm install
code .
# Press F5 to launch extension development host
```
