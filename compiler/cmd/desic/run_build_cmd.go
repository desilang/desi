package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/project"
	"github.com/desilang/desi/compiler/internal/term"
	"github.com/desilang/desi/compiler/internal/version"
)

func exit(code int) {
	term.Flush()
	os.Exit(code)
}

func init() {
	if len(os.Args) < 2 {
		return
	}
	switch os.Args[1] {
	case "init":
		exit(initCmd(os.Args[2:]))
	case "run":
		exit(runCmd(os.Args[2:]))
	case "build":
		exit(buildCmd(os.Args[2:]))
	case "test":
		exit(testCmd(os.Args[2:]))
	case "perf":
		exit(perfCmd(os.Args[2:]))
	case "version":
		term.Println("desic", version.Version)
		exit(0)
	case "help":
		printUsage()
		exit(0)
	default:
		return
	}
}

func printUsage() {
	term.Println("Desi Compiler v" + version.Version)
	term.Println("")
	term.Println("Usage: desic <command> [arguments]")
	term.Println("")
	term.Println("Commands:")
	term.Println("  init [name]              Create a new Desi project")
	term.Println("  build [file] [-o name]   Build executable (uses desi.mod if no file given)")
	term.Println("  run [file] [-- args]     Build and run (uses desi.mod if no file given)")
	term.Println("  test [files] [-v]        Run test files (*_test.desi)")
	term.Println("  perf [--level] [files]   Run performance advisor")
	term.Println("  check <file>             Type-check a file")
	term.Println("  fmt [-w] <file|dir>      Format source code")
	term.Println("  doc [--all] <file>       Generate documentation")
	term.Println("  watch [file]             Watch and re-check on change")
	term.Println("  emit-ir <file>           Emit LLVM IR to stdout")
	term.Println("  version                  Print version")
	term.Println("  help                     Show this help")
	term.Println("")
	term.Println("Flags:")
	term.Println("  --release, -r            Optimized build (alias for -O2)")
	term.Println("  -O2                      Optimize output")
	term.Println("  -I <roots>               Import roots (colon-separated)")
	term.Println("  --error-format <fmt>     Error format: human|json")
	term.Println("  --color <mode>           Color: auto|always|never")
	term.Println("")
	term.Println("Examples:")
	term.Println("  desic init myapp         Create project in ./myapp")
	term.Println("  desic run                Run project (uses desi.mod)")
	term.Println("  desic run hello.desi     Run a single file")
	term.Println("  desic build -o myapp     Build with custom output name")
	term.Println("  desic test               Run all *_test.desi files")
	term.Println("  desic fmt -w src/        Format all files in src/")
}

// --------------------- init ---------------------

func initCmd(argv []string) int {
	// desic init [NAME] [-p PATH|--path PATH] [-v VERSION|--version VERSION] [-e EDITION|--edition EDITION] [--force|-f]
	force := false
	var name string
	var pathOpt string
	version := "0.1.0"
	edition := "2025"

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch a {
		case "-f", "--force":
			force = true
		case "-p", "--path":
			if i+1 >= len(argv) {
				term.Eprintln("init: missing value for", a)
				return 2
			}
			pathOpt = argv[i+1]
			i++
		case "-v", "--version":
			if i+1 >= len(argv) {
				term.Eprintln("init: missing value for", a)
				return 2
			}
			version = argv[i+1]
			i++
		case "-e", "--edition":
			if i+1 >= len(argv) {
				term.Eprintln("init: missing value for", a)
				return 2
			}
			edition = argv[i+1]
			i++
		default:
			if strings.HasPrefix(a, "-") {
				term.Eprintln("init: unknown flag:", a)
				return 2
			}
			if name == "" {
				name = a
			} else {
				term.Eprintln("init: unexpected extra argument:", a)
				return 2
			}
		}
	}

	// Resolve target dir & package name
	var target, pkgName string
	cwd, _ := os.Getwd()
	switch {
	case pathOpt != "":
		target = pathOpt
		if name != "" {
			pkgName = name
		} else if target == "." {
			pkgName = filepath.Base(cwd)
		} else {
			pkgName = filepath.Base(target)
		}
	case name != "":
		target = filepath.Join(cwd, name)
		pkgName = name
	default:
		target = "."
		pkgName = filepath.Base(cwd)
	}

	if err := os.MkdirAll(target, 0o755); err != nil {
		term.Eprintln("init:", err)
		return 2
	}
	mp := filepath.Join(target, "desi.mod")
	if _, err := os.Stat(mp); err == nil && !force {
		term.Eprintln("init: desi.mod already exists (use --force to overwrite)")
		return 2
	}

	// Write desi.mod
	mod := "[package]\n" +
		"name    = " + quote(pkgName) + "\n" +
		"version = " + quote(version) + "\n" +
		"edition = " + quote(edition) + "\n" +
		"entry   = \"src/main.desi\"\n" +
		"roots   = [\"src\"]\n\n" +
		"[build]\n" +
		"mode    = \"debug\"\n" +
		"out_dir = \"build\"\n\n" +
		"[target]\n" +
		"triple  = \"native\"\n\n" +
		"[diagnostics]\n" +
		"error_format = \"human\"\n" +
		"color        = \"auto\"\n" +
		"max_errors   = \"100\"\n\n" +
		"[ffi]\n" +
		"libs   = []\n" +
		"search = []\n"
	if err := os.WriteFile(mp, []byte(mod), 0o644); err != nil {
		term.Eprintln("init:", err)
		return 2
	}

	// Write src/main.desi (no prelude import; print is in prelude)
	srcDir := filepath.Join(target, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		term.Eprintln("init:", err)
		return 2
	}
	mainDesi := "def main() -> int:\n" +
		"  print(\"Hello, Desi!\")\n" +
		"  0\n"
	if err := os.WriteFile(filepath.Join(srcDir, "main.desi"), []byte(mainDesi), 0o644); err != nil {
		term.Eprintln("init:", err)
		return 2
	}

	// Create tests/ with a sample test. An empty tests/ meant `desic test`
	// failed with "no test files found" on a brand new project, so the
	// documented init → run → test → build path broke at the third step.
	// A test is any *_test.desi program: exit 0 passes, non-zero fails.
	testsDir := filepath.Join(target, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		term.Eprintln("init:", err)
		return 2
	}
	sampleTest := "# Tests are ordinary programs: exiting 0 passes, non-zero fails.\n" +
		"# Run them all with `desic test`.\n\n" +
		"def add(a: int, b: int) -> int:\n" +
		"  a + b\n\n" +
		"def main() -> int:\n" +
		"  assert add(2, 2) == 4\n" +
		"  print(\"ok\")\n" +
		"  0\n"
	if err := os.WriteFile(filepath.Join(testsDir, "main_test.desi"), []byte(sampleTest), 0o644); err != nil {
		term.Eprintln("init:", err)
		return 2
	}

	// A .gitignore, because the very next thing a new project does is build,
	// and `desic build` writes build/output/. Without this the first commit
	// carries a compiled binary. Not overwritten if one already exists.
	gitignore := "# Desi build output\n" +
		"build/\n\n" +
		"# Compiled executables\n" +
		"*.exe\n" +
		"*.o\n" +
		"*.obj\n"
	gip := filepath.Join(target, ".gitignore")
	if _, err := os.Stat(gip); os.IsNotExist(err) {
		if err := os.WriteFile(gip, []byte(gitignore), 0o644); err != nil {
			term.Eprintln("init:", err)
			return 2
		}
	}

	term.Println("initialized desi project at", target)
	return 0
}

// --------------------- run/build/test ---------------------

func runCmd(argv []string) int {
	// desic run <file.desi> [-- args...]
	// OR: desic run (uses desi.mod entry)
	//
	// Builds to a temp dir, runs the executable, then cleans up.

	var file string
	var progArgs []string
	optLevel := ""

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			progArgs = argv[i+1:]
			break
		}
		switch {
		case a == "--release" || a == "-r":
			optLevel = "-O2"
		case strings.HasPrefix(a, "-O"):
			optLevel = a
		case strings.HasPrefix(a, "-"):
			// skip render flags etc.
		default:
			if file == "" {
				file = a
			}
		}
	}

	// If no file given, try desi.mod
	if file == "" {
		m, _, ok := loadManifestOrFail("run")
		if !ok {
			return 2
		}
		file = m.EntryPath()
	}

	if _, err := os.Stat(file); os.IsNotExist(err) {
		term.Eprintln("run: file not found:", file)
		return 2
	}

	// Build to temp dir
	tmpDir, err := os.MkdirTemp("", "desi-run-*")
	if err != nil {
		term.Eprintln("run: failed to create temp dir:", err)
		return 2
	}
	defer os.RemoveAll(tmpDir)

	basename := strings.TrimSuffix(filepath.Base(file), ".desi")
	exePath := filepath.Join(tmpDir, basename)
	if runtime.GOOS == "windows" {
		exePath += ".exe"
	}

	code := buildFile(file, exePath, optLevel, argv, false)
	if code != 0 {
		return code
	}

	// Run the executable
	cmd := exec.Command(exePath, progArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return exitCode(err)
	}
	return 0
}

func buildCmd(argv []string) int {
	// desic build <file.desi> [-o name] [-O2]
	// OR: desic build (uses desi.mod entry)
	//
	// Produces a native executable via: emit-ir → llc → clang.

	var file string
	var outputName string
	optLevel := ""

	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case a == "-o" || a == "--output":
			if i+1 >= len(argv) {
				term.Eprintln("build: missing value for", a)
				return 2
			}
			outputName = argv[i+1]
			i++
		case a == "--release" || a == "-r":
			optLevel = "-O2"
		case strings.HasPrefix(a, "-O"):
			optLevel = a
		case strings.HasPrefix(a, "-"):
			// skip render flags etc.
		default:
			if file == "" {
				file = a
			}
		}
	}

	// If no file given, try desi.mod
	if file == "" {
		m, mp, ok := loadManifestOrFail("build")
		if !ok {
			return 2
		}
		file = m.EntryPath()
		if outputName == "" {
			// Project mode keeps its documented default of build/output/<pkg>.
			// Spelled as a path because -o is now one: a bare name here would
			// put the binary in the project root instead.
			outputName = filepath.Join("build", "output", safePkgName(m.Package.Name))
		}

		// --- Build Audit ---
		// Run if [permissions] section has audit=true or any allow/deny rules
		perms := m.Permissions
		if perms.Audit || len(perms.Allow) > 0 || len(perms.Deny) > 0 {
			code := runBuildAudit(file, &perms)
			if code != 0 {
				return code
			}
		}

		_ = mp
	}

	if _, err := os.Stat(file); os.IsNotExist(err) {
		term.Eprintln("build: file not found:", file)
		return 2
	}

	// -o is a path, the way cc, go and rustc all treat it: `-o hello` writes
	// ./hello and `-o dist/app` writes dist/app. It used to be a bare name
	// forced under build/output/, so the file never appeared where it was
	// asked for and a build/ directory turned up in the user's project.
	//
	// Without -o the default is unchanged: build/output/<basename>.
	var exePath string
	if outputName != "" {
		exePath = outputName
	} else {
		exePath = filepath.Join("build", "output",
			strings.TrimSuffix(filepath.Base(file), ".desi"))
	}

	// Windows will not execute a file without the .exe extension, so a build
	// that omits it produces an artifact the user cannot run. `desic run`
	// already does this for its temporary binary; `build` has to as well.
	if runtime.GOOS == "windows" && filepath.Ext(exePath) == "" {
		exePath += ".exe"
	}

	// Create whatever directory the output lands in — build/output by
	// default, or the one named in -o.
	if dir := filepath.Dir(exePath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			term.Eprintln("build:", err)
			return 2
		}
	}
	code := buildFile(file, exePath, optLevel, argv, true)
	if code != 0 {
		return code
	}

	term.Println("✓ Built executable:", exePath)
	return 0
}

// buildFile does the full compile pipeline: emit-ir → llc → clang → executable.
// Returns exit code (0 = success).
func buildFile(file, exePath, optLevel string, argv []string, verbose bool) int {
	exe, _ := os.Executable()

	// Step 1: Emit LLVM IR
	if verbose {
		term.Println("==> Compiling Desi to LLVM IR...")
	}
	emitArgs := []string{"emit-ir"}
	emitArgs = append(emitArgs, forwardRenderFlags(argv)...)
	emitArgs = append(emitArgs, file)

	var irBuf bytes.Buffer
	emitCmd := exec.Command(exe, emitArgs...)
	emitCmd.Stdout = &irBuf
	emitCmd.Stderr = os.Stderr
	if err := emitCmd.Run(); err != nil {
		return exitCode(err)
	}

	// Create temp dir for intermediate files
	tmpDir, err := os.MkdirTemp("", "desi-build-*")
	if err != nil {
		term.Eprintln("build: failed to create temp dir:", err)
		return 2
	}
	defer os.RemoveAll(tmpDir)

	// Write IR to temp file
	irPath := filepath.Join(tmpDir, "program.ll")
	if err := os.WriteFile(irPath, irBuf.Bytes(), 0o644); err != nil {
		term.Eprintln("build: failed to write IR:", err)
		return 2
	}

	// Step 2: Compile IR to object file
	// Windows: clang -c (llc not bundled with LLVM for Windows)
	// Unix:    llc -filetype=obj (faster, no driver overhead)
	if verbose {
		term.Println("==> Compiling LLVM IR to object file...")
	}
	// Release builds on Windows use the LTO hot set when build.ps1 produced
	// it: hot runtime files as bitcode, cross-module inlined into the program.
	var ltoObjs []string
	if optLevel != "" && runtime.GOOS == "windows" {
		ltoObjs = findLTOObjs()
	}

	objPath := filepath.Join(tmpDir, "program.o")
	var irCompileCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		irArgs := []string{"-w", "-c", irPath, "-o", objPath}
		if optLevel != "" {
			irArgs = append(irArgs, optLevel)
		}
		if len(ltoObjs) > 0 {
			irArgs = append(irArgs, "-flto")
		}
		irCompileCmd = exec.Command(findLLVMTool("clang"), irArgs...)
	} else if optLevel != "" {
		// Optimized builds go through clang so the full -O pipeline runs
		// (inlining, loop opts) — llc alone only optimizes codegen.
		irCompileCmd = exec.Command(findLLVMTool("clang"), "-w", "-c", optLevel, irPath, "-o", objPath)
	} else if llc := findLLVMTool("llc"); llvmToolFound(llc) {
		// llc is preferred unoptimized: it skips the compiler driver and is
		// measurably faster, which matters when every example test shells out.
		//
		// Linux links position-independent executables by default, so an
		// object llc produced without this fails at link with "relocation
		// R_X86_64_32 ... can not be used when making a PIE object". clang
		// gets it right on its own, which is why only this path needs the
		// flag; macOS requires PIC regardless, so it is correct on both.
		// build-desi.sh has carried the same flag for the same reason.
		llcArgs := []string{"-filetype=obj", "-relocation-model=pic", "-o", objPath}
		llcArgs = append(llcArgs, irPath)
		irCompileCmd = exec.Command(llc, llcArgs...)
	} else {
		// llc is not always installed — the Homebrew LLVM formula is
		// keg-only, so GitHub's macOS runners have clang on PATH and no llc
		// at all, and `desic build` failed there with "executable file not
		// found in $PATH". clang can do the same job and has to be present
		// regardless, since linking goes through it. build-desi.sh has used
		// clang with an llc fallback since Linux bring-up for this reason.
		irCompileCmd = exec.Command(findLLVMTool("clang"), "-w", "-c", irPath, "-o", objPath)
	}
	irCompileCmd.Stderr = os.Stderr
	if err := irCompileCmd.Run(); err != nil {
		term.Eprintln("build: IR compilation failed:", err)
		term.Eprintln("  Make sure LLVM is installed (brew install llvm  /  choco install llvm)")
		return exitCode(err)
	}

	// Step 3: Link to executable
	if verbose {
		term.Println("==> Linking executable...")
	}
	runtimeLib := findRuntimeLib()
	clangArgs := []string{objPath}
	// LTO bitcode objects go before the native archive so they win symbol
	// resolution (same sources — the archive members are never pulled).
	clangArgs = append(clangArgs, ltoObjs...)
	// Only add entry.obj when the program does NOT define @main — entry.obj
	// provides a main() that calls @__top__. A program can have BOTH a
	// def main() and a @__top__ (imports and module-level statements emit
	// one), and force-linking entry.obj then fails with a duplicate main.
	// The archive path (libdesi.lib also contains entry.obj) resolves this
	// lazily; the explicit path must apply the same rule.
	hasMain := bytes.Contains(irBuf.Bytes(), []byte("@main("))
	if entryObj := findEntryObj(); entryObj != "" && !hasMain && bytes.Contains(irBuf.Bytes(), []byte("@__top__")) {
		clangArgs = append(clangArgs, entryObj)
	}
	clangArgs = append(clangArgs, "-o", exePath)
	if optLevel != "" {
		clangArgs = append(clangArgs, optLevel)
	}
	if len(ltoObjs) > 0 {
		clangArgs = append(clangArgs, "-flto", "-fuse-ld=lld")
	}
	if runtimeLib != "" {
		if runtime.GOOS == "windows" {
			// On Windows, link the lib directly (MSVC linker doesn't use -L/-l well)
			clangArgs = append(clangArgs, runtimeLib)
		} else {
			clangArgs = append(clangArgs, "-L"+filepath.Dir(runtimeLib), "-ldesi")
		}
	}
	// Suppress noisy linker warnings (macOS version mismatch etc.)
	clangArgs = append(clangArgs, "-w")
	// Dead code elimination
	if runtime.GOOS == "darwin" {
		clangArgs = append(clangArgs, "-Wl,-dead_strip")
		// HTTPS TLS via OpenSSL (Homebrew or system)
		if opensslPrefix := detectOpenSSLPrefix(); opensslPrefix != "" {
			clangArgs = append(clangArgs, "-L"+opensslPrefix+"/lib", "-lssl", "-lcrypto")
		}
		// Note: -lz removed — compression is bundled via miniz in libdesi.a
	} else if runtime.GOOS == "linux" {
		clangArgs = append(clangArgs, "-Wl,--gc-sections")
		// HTTPS TLS via system OpenSSL
		clangArgs = append(clangArgs, "-lssl", "-lcrypto")
		// Note: -lz removed — compression is bundled via miniz in libdesi.a
	}
	// Windows: no extra linker flags needed; MSVC link.exe handles gc internally
	clangCmd := exec.Command(findLLVMTool("clang"), clangArgs...)
	clangCmd.Stderr = os.Stderr
	if err := clangCmd.Run(); err != nil {
		term.Eprintln("build: linking failed:", err)
		return exitCode(err)
	}

	return 0
}

// findLTOObjs locates the bitcode hot-set objects build.ps1 produces in
// build/lto (Windows release builds). Empty when the set wasn't built —
// release then just links the native archive without cross-module LTO.
func findLTOObjs() []string {
	dirs := []string{}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		dirs = append(dirs,
			filepath.Join(exeDir, "..", "build", "lto"),
			filepath.Join(exeDir, "..", "lib", "lto"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs,
			filepath.Join(cwd, "build", "lto"),
			filepath.Join(cwd, "lib", "lto"),
		)
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var objs []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".obj") {
				objs = append(objs, filepath.Join(dir, e.Name()))
			}
		}
		if len(objs) > 0 {
			return objs
		}
	}
	return nil
}

// findRuntimeLib locates libdesi.a (Unix) or libdesi.lib (Windows).
func findRuntimeLib() string {
	names := []string{"libdesi.a", "libdesi.lib"}
	dirs := []string{}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		dirs = append(dirs,
			filepath.Join(exeDir, "..", "build"),
			filepath.Join(exeDir, "..", "lib"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs,
			filepath.Join(cwd, "build"),
			filepath.Join(cwd, "lib"),
		)
	}

	for _, dir := range dirs {
		for _, name := range names {
			c := filepath.Join(dir, name)
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	return ""
}

// findEntryObj locates entry.obj (Windows) needed as explicit link input
// because MSVC linker won't pull main() from a static lib unless referenced.
func findEntryObj() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	dirs := []string{}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		// "build" is the developer tree; "lib" is where an installed release
		// puts it, next to bin/. findRuntimeLib already accepts both.
		dirs = append(dirs,
			filepath.Join(exeDir, "..", "build"),
			filepath.Join(exeDir, "..", "lib"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs,
			filepath.Join(cwd, "build"),
			filepath.Join(cwd, "lib"),
		)
	}
	for _, dir := range dirs {
		c := filepath.Join(dir, "entry.obj")
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func testCmd(argv []string) int {
	// Parse arguments: desic test [file.desi | pattern] [-v|--verbose]
	var testFiles []string
	verbose := false

	for _, a := range argv {
		switch {
		case a == "-v" || a == "--verbose":
			verbose = true
		case !strings.HasPrefix(a, "-"):
			testFiles = append(testFiles, a)
		}
	}

	// If no files specified, auto-discover *_test.desi files
	if len(testFiles) == 0 {
		cwd, _ := os.Getwd()
		discovered := discoverTestFiles(cwd)
		if len(discovered) == 0 {
			term.Eprintln("test: no test files found")
			term.Eprintln("  Place *_test.desi files in the current directory or tests/ subdirectory,")
			term.Eprintln("  or pass a file directly: desic test <file.desi>")
			return 2
		}
		testFiles = discovered
	}

	// Run each test file
	passed := 0
	failed := 0
	var failures []string

	for _, testFile := range testFiles {
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			term.Eprintln("test: file not found:", testFile)
			failed++
			failures = append(failures, testFile)
			continue
		}

		baseName := filepath.Base(testFile)
		if verbose {
			term.Println("==>", baseName)
		}

		rc := runSingleTest(testFile, verbose)
		if rc == 0 {
			passed++
			if !verbose {
				term.Println("  ✓", baseName)
			}
		} else {
			failed++
			failures = append(failures, testFile)
			term.Eprintln("  ✗", baseName)
		}
	}

	// Summary
	term.Println("")
	if failed == 0 {
		term.Println(fmt.Sprintf("✓ %d test(s) passed!", passed))
		return 0
	}
	term.Eprintln(fmt.Sprintf("✗ %d passed, %d failed", passed, failed))
	for _, f := range failures {
		term.Eprintln("  FAIL:", f)
	}
	return 1
}

// discoverTestFiles finds *_test.desi files in dir and dir/tests/.
func discoverTestFiles(dir string) []string {
	var files []string
	searchDirs := []string{dir, filepath.Join(dir, "tests")}
	for _, d := range searchDirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.desi") {
				files = append(files, filepath.Join(d, e.Name()))
			}
		}
	}
	return files
}

// runSingleTest compiles and runs a single test file. Returns 0 on success.
func runSingleTest(testFile string, verbose bool) int {
	exe, _ := os.Executable()

	// Type-check
	if verbose {
		term.Println("    type-checking...")
	}
	checkCmd := exec.Command(exe, "check", testFile)
	checkCmd.Stderr = os.Stderr
	if err := checkCmd.Run(); err != nil {
		term.Eprintln("test: type check failed for", testFile)
		return exitCode(err)
	}

	// Emit IR
	if verbose {
		term.Println("    emitting IR...")
	}
	var irBuf bytes.Buffer
	emitCmd := exec.Command(exe, "emit-ir", testFile)
	emitCmd.Stdout = &irBuf
	emitCmd.Stderr = os.Stderr
	if err := emitCmd.Run(); err != nil {
		term.Eprintln("test: emit-ir failed for", testFile)
		return exitCode(err)
	}

	// Temp dir for artifacts
	tmpDir, err := os.MkdirTemp("", "desi-test-*")
	if err != nil {
		term.Eprintln("test: failed to create temp dir:", err)
		return 2
	}
	defer os.RemoveAll(tmpDir)

	irPath := filepath.Join(tmpDir, "test.ll")
	if err := os.WriteFile(irPath, irBuf.Bytes(), 0o644); err != nil {
		term.Eprintln("test: failed to write IR:", err)
		return 2
	}

	// IR → object file (Windows: clang -c, Unix: llc)
	if verbose {
		term.Println("    compiling...")
	}
	objPath := filepath.Join(tmpDir, "test.o")
	var irCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		irCmd = exec.Command(findLLVMTool("clang"), "-w", "-c", irPath, "-o", objPath)
	} else {
		irCmd = exec.Command(findLLVMTool("llc"), "-filetype=obj", "-o", objPath, irPath)
	}
	irCmd.Stderr = os.Stderr
	if err := irCmd.Run(); err != nil {
		term.Eprintln("test: IR compilation failed for", testFile)
		return exitCode(err)
	}

	// Link with clang
	if verbose {
		term.Println("    linking...")
	}
	exePath := filepath.Join(tmpDir, "test_runner")
	if runtime.GOOS == "windows" {
		exePath += ".exe"
	}
	clangArgs := []string{"-o", exePath, objPath}
	if entryObj := findEntryObj(); entryObj != "" && bytes.Contains(irBuf.Bytes(), []byte("@__top__")) {
		clangArgs = append(clangArgs, entryObj)
	}

	// Suppress noisy linker warnings
	clangArgs = append(clangArgs, "-w")
	// Dead-code elimination + runtime lib
	if runtime.GOOS == "darwin" {
		clangArgs = append(clangArgs, "-Wl,-dead_strip")
		if opensslPrefix := detectOpenSSLPrefix(); opensslPrefix != "" {
			clangArgs = append(clangArgs, "-L"+opensslPrefix+"/lib", "-lssl", "-lcrypto")
		}
	} else if runtime.GOOS == "linux" {
		clangArgs = append(clangArgs, "-Wl,--gc-sections", "-lssl", "-lcrypto")
	}

	// Runtime library
	if rtLib := findRuntimeLib(); rtLib != "" {
		if runtime.GOOS == "windows" {
			clangArgs = append(clangArgs, rtLib)
		} else {
			clangArgs = append(clangArgs, rtLib)
			clangArgs = append(clangArgs, "-lm")
		}
	}

	clangCmd := exec.Command(findLLVMTool("clang"), clangArgs...)
	clangCmd.Stderr = os.Stderr
	if err := clangCmd.Run(); err != nil {
		term.Eprintln("test: linking failed for", testFile)
		return exitCode(err)
	}

	// Run test
	if verbose {
		term.Println("    running...")
	}
	testRunner := exec.Command(exePath)
	testRunner.Stdout = os.Stdout
	testRunner.Stderr = os.Stderr
	if err := testRunner.Run(); err != nil {
		return exitCode(err)
	}
	return 0
}

// --------------------- helpers ---------------------

func loadManifestOrFail(verb string) (project.Manifest, string, bool) {
	cwd, _ := os.Getwd()
	root, mp, ok := project.FindRoot(cwd)
	if !ok {
		term.Eprintln(verb+":", "no desi.mod found (run in a project directory)")
		return project.Manifest{}, "", false
	}
	m, diags := project.Load(mp)
	for _, d := range diags {
		d.RenderTTY(os.Stderr, diag.Theme{})
	}
	if len(diags) > 0 {
		return project.Manifest{}, "", false
	}
	_ = root
	return m, mp, true
}

func pickIRoots(argv []string, m project.Manifest) string {
	if cli := parseIRootsArg(argv); cli != "" {
		return cli
	}
	rs := m.Roots()
	if len(rs) == 0 {
		return ""
	}
	sep := string(os.PathListSeparator)
	return strings.Join(rs, sep)
}

func parseIRootsArg(argv []string) string {
	iroots := ""
	sawSep := false
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if sawSep {
			break
		}
		switch {
		case a == "--":
			sawSep = true
		case a == "-I":
			if i+1 < len(argv) {
				iroots = argv[i+1]
				i++
			}
		case strings.HasPrefix(a, "-I="):
			iroots = strings.TrimPrefix(a, "-I=")
		}
	}
	return iroots
}

func forwardRenderFlags(argv []string) []string {
	out := []string{}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case strings.HasPrefix(a, "--error-format="),
			a == "--error-format",
			strings.HasPrefix(a, "--color="),
			a == "--color":
			out = append(out, a)
			if (a == "--error-format" || a == "--color") && i+1 < len(argv) {
				out = append(out, argv[i+1])
				i++
			}
		}
	}
	return out
}

func hasFlag(argv []string, flag string) bool {
	for _, a := range argv {
		if a == flag {
			return true
		}
	}
	return false
}

func verifyWithLLVMAs(ir []byte, workdir string) error {
	// Try to run `llvm-as -o <devnull>` with IR on stdin.
	as, err := exec.LookPath("llvm-as")
	if err != nil {
		term.Eprintln("verify-llvm: llvm-as not found on PATH — skipping verification")
		return nil
	}
	devnull := "/dev/null"
	if runtime.GOOS == "windows" {
		devnull = "NUL"
	}
	cmd := exec.Command(as, "-o", devnull)
	cmd.Stdin = bytes.NewReader(ir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = workdir
	if err := cmd.Run(); err != nil {
		return err
	}
	term.Println("verify-llvm: OK")
	return nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		if ws, ok := ee.Sys().(interface{ ExitStatus() int }); ok {
			return ws.ExitStatus()
		}
		if runtime.GOOS == "windows" {
			return int(ee.ExitCode())
		}
	}
	return 1
}

func safePkgName(name string) string {
	if name == "" {
		return "app"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, name)
}

func quote(s string) string {
	if !strings.ContainsAny(s, " \t\"") {
		return `"` + s + `"`
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// detectOpenSSLPrefix finds the OpenSSL installation prefix for HTTPS support.
// Checks: Homebrew (macOS), then common system paths.
func detectOpenSSLPrefix() string {
	// Try Homebrew first (macOS common case)
	if out, err := exec.Command("brew", "--prefix", "openssl").Output(); err == nil {
		prefix := strings.TrimSpace(string(out))
		if prefix != "" {
			// Verify the lib actually exists
			if _, err := os.Stat(filepath.Join(prefix, "lib", "libssl.a")); err == nil {
				return prefix
			}
			if _, err := os.Stat(filepath.Join(prefix, "lib", "libssl.dylib")); err == nil {
				return prefix
			}
		}
	}
	// Try common system paths
	for _, prefix := range []string{"/usr", "/usr/local", "/opt/homebrew"} {
		libPath := filepath.Join(prefix, "lib", "libssl.a")
		if _, err := os.Stat(libPath); err == nil {
			return prefix
		}
		libPath = filepath.Join(prefix, "lib", "libssl.so")
		if _, err := os.Stat(libPath); err == nil {
			return prefix
		}
	}
	return ""
}

// findLLVMTool locates an LLVM tool (llc, clang, llvm-as, etc.) by checking:
// 1. The system PATH (exec.LookPath)
// 2. Platform-specific well-known locations:
//   - macOS: Homebrew (/opt/homebrew/opt/llvm/bin, /usr/local/opt/llvm/bin)
//   - Linux: /usr/lib/llvm-*/bin, /usr/bin
//   - Windows: Chocolatey, MSYS2, Program Files
//
// Returns the full path if found, or the bare tool name as fallback.
func findLLVMTool(name string) string {
	// Fast path: already on PATH
	if p, err := exec.LookPath(name); err == nil {
		return p
	}

	var candidates []string

	switch runtime.GOOS {
	case "darwin":
		// Homebrew: Apple Silicon, then Intel Mac
		candidates = append(candidates,
			filepath.Join("/opt/homebrew/opt/llvm/bin", name),
			filepath.Join("/usr/local/opt/llvm/bin", name),
			filepath.Join("/opt/homebrew/bin", name),
			filepath.Join("/usr/local/bin", name),
		)
	case "linux":
		// Versioned LLVM installs (e.g. /usr/lib/llvm-17/bin/llc)
		for v := 20; v >= 14; v-- {
			candidates = append(candidates,
				fmt.Sprintf("/usr/lib/llvm-%d/bin/%s", v, name),
			)
		}
		candidates = append(candidates,
			filepath.Join("/usr/bin", name),
			filepath.Join("/usr/local/bin", name),
		)
	case "windows":
		// Chocolatey, MSYS2, and Program Files
		if pf := os.Getenv("ProgramFiles"); pf != "" {
			candidates = append(candidates,
				filepath.Join(pf, "LLVM", "bin", name+".exe"),
			)
		}
		if msys := os.Getenv("MSYSTEM_PREFIX"); msys != "" {
			candidates = append(candidates,
				filepath.Join(msys, "bin", name+".exe"),
			)
		}
		candidates = append(candidates,
			filepath.Join("C:\\msys64\\mingw64\\bin", name+".exe"),
			filepath.Join("C:\\ProgramData\\chocolatey\\bin", name+".exe"),
		)
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Fallback: return bare name, let exec fail with a clear error
	return name
}

// llvmToolFound reports whether findLLVMTool actually located a tool rather
// than falling back to the bare name. Used to pick between tools that are
// interchangeable for a step, so a missing optional one is not fatal.
func llvmToolFound(p string) bool {
	if p == "" {
		return false
	}
	if filepath.IsAbs(p) {
		_, err := os.Stat(p)
		return err == nil
	}
	_, err := exec.LookPath(p)
	return err == nil
}

// runBuildAudit performs a build audit by scanning the entry file for stdlib
// imports and checking them against the [permissions] section in desi.mod.
// Returns 0 if the build should proceed, 1 if blocked.
func runBuildAudit(file string, perms *project.Permissions) int {
	src, err := os.ReadFile(file)
	if err != nil {
		term.Eprintln("audit: cannot read", file, ":", err)
		return 2
	}

	mod, pdiags := parse.ParseFile(file, src)
	if len(pdiags) > 0 {
		// Parse errors will be caught by emit-ir; just skip audit
		return 0
	}

	// Run the checker to discover stdlib imports
	_, info := check.Check(mod)

	// Collect stdlib module names
	var imports []string
	for modName := range info.StdlibImports {
		imports = append(imports, modName)
	}

	// Convert project.Permissions → check.Permissions
	auditPerms := &check.Permissions{
		Allow: perms.Allow,
		Deny:  perms.Deny,
		Audit: perms.Audit,
	}

	report := check.RunAudit(imports, auditPerms)

	// Print report if audit mode is on
	if perms.Audit {
		term.Eprint(check.FormatReport(report))
	}

	// Block build if denied imports were found
	if report.HasBlocked {
		term.Eprintln("build: blocked by [permissions] — denied stdlib imports detected")
		return 1
	}

	return 0
}
