# Installation

This guide covers installing Desi on your system.

---

## Prerequisites

Before installing Desi, ensure you have:

- **LLVM** (version 15 or later)
- **Clang** (for linking)
- **Go** (version 1.21+, for building from source)

---

## Installation Methods

### From Source (Recommended)

Clone the repository and build:

```bash
# Clone the repository
git clone https://github.com/desilang/desi.git
cd desi

# Build the compiler
make

# Verify installation
./bin/desic -version
```

### Verify LLVM

Desi requires LLVM for compilation. Check your LLVM version:

```bash
llc --version
```

If LLVM is not installed:

=== "macOS"

    ```bash
    brew install llvm
    ```

=== "Ubuntu/Debian"

    ```bash
    sudo apt install llvm clang
    ```

=== "Fedora"

    ```bash
    sudo dnf install llvm clang
    ```

---

## Project Structure

After cloning, the Desi project has this structure:

```
desi/
├── bin/              # Compiled binaries
│   └── desic         # The Desi compiler
├── compiler/         # Compiler source code
├── runtime/          # Runtime library
├── examples/         # Example programs
├── book/             # This documentation
└── docs/             # Internal documentation
```

---

## Testing Your Installation

Create a simple test file:

```python
# test.desi
def main():
    print("Desi is working!")
```

Compile and run:

```bash
./bin/desic test.desi
./build/output/test_exec
```

You should see:

```
Desi is working!
```

---

## Build Options

The `desic` compiler supports several options:

| Option | Description |
|--------|-------------|
| `-emit-ir` | Output LLVM IR instead of compiling |
| `-ast` | Print the AST |
| `-tokens` | Print tokens |
| `-check` | Type-check only |
| `-version` | Show version |

Example:

```bash
# View the generated LLVM IR
./bin/desic -emit-ir test.desi
```

---

## Troubleshooting

### LLVM Not Found

If you see LLVM-related errors:

1. Ensure LLVM is installed and in your PATH
2. On macOS with Homebrew, you may need:
   ```bash
   export PATH="/opt/homebrew/opt/llvm/bin:$PATH"
   ```

### Linker Errors

If linking fails:

1. Ensure Clang is installed
2. Check that the runtime library is built:
   ```bash
   make -C runtime
   ```

---

## Next Steps

Now that Desi is installed, let's write your [first program](first-program.md)!
