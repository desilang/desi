# Installation

This guide covers installing Desi on your system.

---

## Prerequisites

Before installing Desi, ensure you have:

- **LLVM** (version 15 or later)
- **Clang** (for linking)
- **Go** (version 1.21+, for building from source)
- **Windows only:** [Visual Studio Build Tools 2022](https://visualstudio.microsoft.com/downloads/#build-tools-for-visual-studio-2022)
  with the *Desktop development with C++* workload (provides `cl.exe` and the
  Windows SDK used to build the C runtime)

---

## Installation Methods

### From Source (Recommended)

Clone the repository and build:

=== "macOS / Linux"

    ```bash
    git clone https://github.com/desilang/desi.git
    cd desi
    make
    ./bin/desic version
    ```

=== "Windows"

    ```powershell
    git clone https://github.com/desilang/desi.git
    cd desi
    .\build.ps1
    .\bin\desic.exe version
    ```

### Verify LLVM

Desi requires LLVM for compilation. Check your LLVM version:

=== "macOS / Linux"

    ```bash
    llc --version
    ```

=== "Windows"

    ```powershell
    clang --version
    ```

If LLVM / Clang is not installed:

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

=== "Windows"

    Download the LLVM installer from [llvm.org/releases](https://releases.llvm.org/) and install it.
    Also install [Visual Studio Build Tools](https://visualstudio.microsoft.com/downloads/#build-tools-for-visual-studio-2022)
    with the **Desktop development with C++** workload — this provides `cl.exe` and `lib.exe` for the runtime build.

---

## Project Structure

After cloning, the Desi project has this structure:

```
desi/
├── bin/              # Compiled binaries
│   ├── desic         # The Desi compiler
│   ├── desifmt       # Code formatter
│   ├── desirepl      # Interactive REPL
│   └── desilsp       # Language server (for editors)
├── compiler/         # Compiler source code
│   ├── cmd/          # CLI tools
│   ├── internal/     # Compiler internals
│   ├── lib/          # Standard library (.desi files)
│   └── runtime/      # C runtime library
├── build/            # Build artifacts
│   ├── libdesi.a     # Static runtime library
│   └── output/       # Compiled executables
├── examples/         # Example programs
├── book/             # This documentation
└── docs/             # Internal documentation
```

---

## Testing Your Installation

Create a simple test file:

```desi
# hello.desi
def main() -> int:
    print("Desi is working!")
    0
```

Run it directly:

```bash
desic run hello.desi
```

Or build an executable:

```bash
desic build hello.desi -o hello
./build/output/hello
```

You should see:

```
Desi is working!
```

---

## The `desic` CLI

The Desi compiler provides subcommands for the full development workflow:

### Core Commands

| Command | Description |
|---------|-------------|
| `desic init [name]` | Create a new Desi project with `desi.mod` |
| `desic build [file]` | Build an executable |
| `desic run [file]` | Build and run immediately |
| `desic test [files]` | Run test files (`*_test.desi`) |
| `desic check <file>` | Type-check without compiling |

### Tool Commands

| Command | Description |
|---------|-------------|
| `desic fmt [-w] <file\|dir>` | Format source code |
| `desic doc [--all] <file>` | Generate documentation |
| `desic watch [file]` | Watch and re-check on save |
| `desic emit-ir <file>` | Emit LLVM IR to stdout |
| `desic version` | Print compiler version |
| `desic help` | Show all commands |

### Flags

| Flag | Description |
|------|-------------|
| `-O2` | Optimize output |
| `-o <name>` | Set output executable name |
| `-I <roots>` | Import roots (colon-separated) |
| `--error-format <fmt>` | Error format: `human` or `json` |
| `--color <mode>` | Color output: `auto`, `always`, or `never` |

### Project Mode vs File Mode

When you run `desic build` or `desic run` **without** a file argument, it uses the `desi.mod` manifest:

```bash
# File mode — compile a single file
desic run hello.desi

# Project mode — uses desi.mod entry point
cd myproject/
desic run
```

See [First Program](first-program.md) for a complete project walkthrough.

---

## Troubleshooting

### LLVM Not Found

If you see LLVM-related errors:

1. Ensure LLVM is installed and in your PATH
2. On macOS with Homebrew, you may need:
   ```bash
   export PATH="/opt/homebrew/opt/llvm/bin:$PATH"
   ```
3. On Windows, add the LLVM bin folder to your PATH:
   ```
   C:\Program Files\LLVM\bin
   ```

### Linker Errors

If linking fails:

1. Ensure Clang is installed
2. Check that the runtime library is built:
   ```bash
   make          # macOS / Linux
   .\build.ps1   # Windows
   ```

### Windows: Runtime Build Fails

The Windows build script (`build.ps1`) requires Visual Studio Build Tools with the C++ workload. If `cl.exe` is not found:

1. Install [Visual Studio Build Tools](https://visualstudio.microsoft.com/downloads/#build-tools-for-visual-studio-2022)
2. Select **Desktop development with C++**
3. Re-run `.\build.ps1`

---

## Next Steps

Now that Desi is installed, let's write your [first program](first-program.md)!
