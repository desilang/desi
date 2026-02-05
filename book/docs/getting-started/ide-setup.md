# IDE Setup

Desi has full IDE support via the Language Server Protocol (LSP).

## Quick Start

1. Build the language server:
   ```bash
   make tools
   ```

2. Add `bin/desilsp` to your PATH

3. Configure your editor (see below)

## VS Code

```bash
cd editors/vscode
npm install
code .  # F5 to launch dev host
```

Or install the packaged extension:
```bash
npx vsce package --allow-missing-repository
```

## IntelliJ IDEA / WebStorm / PyCharm

1. Install [LSP4IJ](https://plugins.jetbrains.com/plugin/23257-lsp4ij) plugin
2. Build the Desi plugin:
   ```bash
   cd editors/intellij
   ./gradlew buildPlugin
   ```
3. Install from `build/distributions/desi-language-*.zip`

## Neovim

Using `nvim-lspconfig`:

```lua
require('lspconfig').desilsp.setup{
  cmd = { 'desilsp' },
  filetypes = { 'desi' },
}
```

## Other Editors

Any LSP-compatible editor works. Configure to run `desilsp` via stdio:

- **Sublime Text**: Use the `LSP` package
- **Emacs**: Use `lsp-mode` or `eglot`
- **Helix**: Add to `languages.toml`

## Features

| Feature | Description |
|---------|-------------|
| Hover | Type info on hover |
| Go to Definition | Jump to declarations |
| Find References | All usages |
| Rename | Rename across files |
| Completion | Auto-complete |
| Signature Help | Parameter hints |
| Code Actions | Quick fixes |
| Formatting | Format document |
| Diagnostics | Errors/warnings |
| Semantic Tokens | Rich highlighting |
| Inlay Hints | Inline types |
