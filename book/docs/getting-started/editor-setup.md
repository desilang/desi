# Editor Setup

Configure your editor for the best Desi development experience.

---

## Language Server (desilsp)

Desi includes a Language Server for IDE features like error highlighting, hover, and go-to-definition.

### Build the LSP

```bash
make tools  # Creates bin/desilsp
export PATH="$PATH:/path/to/desi/bin"
```

### Features

| Feature | Description |
|---------|-------------|
| **Diagnostics** | Red squiggles for type errors |
| **Hover** | View type info on mouse hover |
| **Go to Definition** | Ctrl+Click to jump to declaration |
| **Find References** | Find all usages of a symbol |
| **Document Symbols** | List functions/classes in outline |

---

## Visual Studio Code (with LSP)

1. Install the Desi extension:
   ```bash
   cd editors/vscode
   npm install
   code --install-extension .
   ```

2. Open a `.desi` file and the LSP starts automatically.

---

## Neovim (with LSP)

Add to your Neovim config (`init.lua`):

```lua
local lspconfig = require('lspconfig')
local configs = require('lspconfig.configs')

configs.desilsp = {
  default_config = {
    cmd = { 'desilsp' },
    filetypes = { 'desi' },
    root_dir = lspconfig.util.root_pattern('desi.toml', '.git'),
  },
}

lspconfig.desilsp.setup{}
```

---

## Syntax Highlighting (Fallback)

## Visual Studio Code

### Recommended Settings

Create or edit `.vscode/settings.json` in your Desi projects:

```json
{
  "files.associations": {
    "*.desi": "python"
  },
  "[desi]": {
    "editor.tabSize": 4,
    "editor.insertSpaces": true
  }
}
```

This tells VSCode to treat `.desi` files as Python for syntax highlighting.

### Recommended Extensions

- **Python** (by Microsoft) - For syntax highlighting
- **Error Lens** - Inline error display
- **Code Runner** - Quick run support

---

## Vim / Neovim

Add to your `.vimrc` or `init.vim`:

```vim
" Treat .desi files like Python for syntax
autocmd BufRead,BufNewFile *.desi set filetype=python

" Use 4 spaces for indentation
autocmd FileType python setlocal shiftwidth=4 tabstop=4 expandtab
```

### Neovim with Lua

```lua
-- In your init.lua
vim.api.nvim_create_autocmd({"BufRead", "BufNewFile"}, {
  pattern = "*.desi",
  callback = function()
    vim.bo.filetype = "python"
  end
})
```

---

## JetBrains IDEs (PyCharm, IntelliJ)

1. Go to **Settings** → **Editor** → **File Types**
2. Find **Python** in the list
3. Add `*.desi` to the registered patterns

---

## Emacs

Add to your Emacs config:

```elisp
(add-to-list 'auto-mode-alist '("\\.desi\\'" . python-mode))
```

---

## Sublime Text

1. Open a `.desi` file
2. Go to **View** → **Syntax** → **Python**
3. To make it permanent: **View** → **Syntax** → **Open all with current extension as...** → **Python**

---

## Build Integration

### VSCode Tasks

Create `.vscode/tasks.json`:

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "Build Desi",
      "type": "shell",
      "command": "./bin/desic ${file}",
      "group": {
        "kind": "build",
        "isDefault": true
      },
      "problemMatcher": []
    },
    {
      "label": "Run Desi",
      "type": "shell",
      "command": "./bin/desic ${file} && ./build/output/test_exec",
      "group": "test",
      "problemMatcher": []
    }
  ]
}
```

Now you can:

- Press `Ctrl+Shift+B` (or `Cmd+Shift+B` on Mac) to build
- Run the "Run Desi" task to build and execute

### Makefile Integration

Create a simple `Makefile` for your project:

```makefile
.PHONY: build run clean

DESIC := ./bin/desic
SRC := main.desi
OUT := ./build/output/test_exec

build:
	$(DESIC) $(SRC)

run: build
	$(OUT)

clean:
	rm -rf ./build/output/
```

---

## What's Next?

Your editor is ready! Now let's learn Desi:

- [Introduction to Desi](../tutorials/intro.md)
- [Variables & Types](../tutorials/variables.md)
