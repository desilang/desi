# Desi Language Extension for VS Code

Full language support for Desi powered by `desilsp`.

## Features

| Feature | Shortcut |
|---------|----------|
| **Hover** | Mouse hover - type info |
| **Go to Definition** | `Cmd+Click` / `F12` |
| **Find References** | `Shift+F12` |
| **Rename Symbol** | `F2` |
| **Auto-Complete** | `.` trigger |
| **Signature Help** | `(` trigger |
| **Code Actions** | `Cmd+.` |
| **Format Document** | `Shift+Alt+F` |
| **Outline View** | Sidebar |
| **Workspace Symbols** | `Cmd+T` |
| **Semantic Highlighting** | Automatic |
| **Inlay Hints** | Automatic |
| **Call Hierarchy** | Right-click |

## Installation

### 1. Build the Language Server

```bash
cd /path/to/desi
make tools  # Builds desilsp to bin/desilsp
```

### 2. Install Extension

**Development:**
```bash
cd editors/vscode
npm install
code .
# Press F5 to launch extension development host
```

**Package for distribution:**
```bash
npx vsce package --allow-missing-repository
# Produces desi-language-0.1.0.vsix
```

Then: `Extensions: Install from VSIX...`

## Configuration

```json
{
  "desi.lspPath": "/path/to/desilsp"
}
```

If `desi.lspPath` is not set, the extension looks for `desilsp` in PATH.
