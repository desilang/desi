# Desi Language IntelliJ Plugin

LSP-based language support for Desi in JetBrains IDEs.

## Prerequisites

1. Build `desilsp` and add to PATH:
   ```bash
   cd /path/to/desi
   make all
   export PATH=$PATH:$(pwd)/bin
   ```

2. Install [LSP4IJ](https://plugins.jetbrains.com/plugin/23257-lsp4ij) plugin in your IDE

## Build

```bash
./gradlew buildPlugin
```

The plugin `.zip` will be in `build/distributions/`.

## Install

1. Open IDE Settings → Plugins → ⚙️ → Install Plugin from Disk
2. Select the `.zip` file
3. Restart IDE

## Features

All features powered by `desilsp`:
- Syntax highlighting
- Auto-completion
- Hover documentation
- Go to definition
- Find references
- Rename symbol
- Error diagnostics
- Code formatting
