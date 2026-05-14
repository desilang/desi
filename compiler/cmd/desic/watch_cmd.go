package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/desilang/desi/compiler/internal/term"
	"github.com/fsnotify/fsnotify"
)

// ---- watch subcommand ----

// runWatch handles the `desic watch` subcommand.
// Watches for .desi file changes and automatically rebuilds/restarts.
// Flags: -v (verbose), --run (run after build), --hot (true hot reload)
func runWatch(args []string) int {
	verbose := false
	runAfterBuild := false
	hotReload := false
	var targetDir string

	for _, a := range args {
		switch a {
		case "-v", "--verbose":
			verbose = true
		case "--run":
			runAfterBuild = true
		case "--hot":
			hotReload = true
			runAfterBuild = true // --hot implies --run
		default:
			if !strings.HasPrefix(a, "-") && targetDir == "" {
				targetDir = a
			}
		}
	}

	if targetDir == "" {
		targetDir = "."
	}

	// Resolve to absolute path
	absDir, err := filepath.Abs(targetDir)
	if err != nil {
		term.Eprintln("desic watch:", err)
		return 1
	}

	// Check directory exists
	stat, err := os.Stat(absDir)
	if err != nil || !stat.IsDir() {
		term.Eprintln("desic watch: not a directory:", absDir)
		return 1
	}

	term.Println("🔥 Watching for changes in:", absDir)
	term.Println("   Press Ctrl+C to stop")
	term.Println("")
	term.Flush()

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		term.Eprintln("desic watch: failed to create watcher:", err)
		return 1
	}
	defer watcher.Close()

	// Watch directory recursively
	watchCount := 0
	err = filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}
		if info.IsDir() {
			// Skip hidden directories and common non-source dirs
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || base == "node_modules" || base == "__pycache__" {
				return filepath.SkipDir
			}
			if err := watcher.Add(path); err != nil {
				if verbose {
					term.Eprintln("  warning: cannot watch", path)
				}
			} else {
				watchCount++
			}
		}
		return nil
	})
	if err != nil {
		term.Eprintln("desic watch: failed to walk directory:", err)
		return 1
	}

	if verbose {
		term.Printf("  Watching %d directories\n", watchCount)
		term.Flush()
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		term.Println("\n👋 Stopping watch...")
		term.Flush()
		cancel()
	}()

	// Debounce timer to avoid rebuilding on every keystroke
	var debounceTimer *time.Timer
	var debounceMu sync.Mutex
	const debounceDelay = 100 * time.Millisecond

	// Process manager for running the built program
	var currentProcess *exec.Cmd
	var processMu sync.Mutex

	// Reload tracking for state serialization
	reloadCount := 0
	stateFilePath := filepath.Join(os.TempDir(), "desi-reload-state.json")

	// Hot reload: persistent .so path and host PID
	var soPath string
	var hostPID int
	if hotReload {
		soPath = filepath.Join(os.TempDir(), "desi-hot-module.dylib")
	}

	// Initial build
	mainFile := findMainFile(absDir)
	if mainFile != "" {
		term.Println("📦 Building:", mainFile)
		term.Flush()
		if hotReload {
			hostPID = buildAndRunHot(mainFile, soPath, &currentProcess, &processMu, verbose)
		} else {
			buildAndRun(mainFile, runAfterBuild, &currentProcess, &processMu, verbose, reloadCount, stateFilePath)
			reloadCount++
		}
	}

	// Watch loop
	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown
			processMu.Lock()
			if currentProcess != nil && currentProcess.Process != nil {
				_ = currentProcess.Process.Signal(syscall.SIGTERM)
			}
			processMu.Unlock()
			return 0

		case event, ok := <-watcher.Events:
			if !ok {
				return 0
			}

			// Only care about .desi files
			if !strings.HasSuffix(event.Name, ".desi") {
				continue
			}

			// Only care about write/create events
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Debounce rapid changes
			debounceMu.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDelay, func() {
				relPath, _ := filepath.Rel(absDir, event.Name)
				term.Printf("\n📝 Changed: %s\n", relPath)
				term.Flush()

				// Find main file to build
				buildTarget := mainFile
				if buildTarget == "" {
					buildTarget = event.Name
				}

				if hotReload && hostPID != 0 {
					// True hot reload: rebuild .so and signal host
					doHotReload(buildTarget, soPath, hostPID, verbose)
				} else if hotReload && hostPID == 0 {
					// Host died, restart it
					hostPID = buildAndRunHot(buildTarget, soPath, &currentProcess, &processMu, verbose)
				} else {
					// Normal rebuild with process restart
					buildAndRun(buildTarget, runAfterBuild, &currentProcess, &processMu, verbose, reloadCount, stateFilePath)
					reloadCount++
				}
			})
			debounceMu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return 0
			}
			if verbose {
				term.Eprintln("  watcher error:", err)
				term.Flush()
			}
		}
	}
}

// findMainFile looks for a main.desi or single .desi file in the directory
func findMainFile(dir string) string {
	mainFile := filepath.Join(dir, "main.desi")
	if _, err := os.Stat(mainFile); err == nil {
		return mainFile
	}

	// Look for any .desi file
	var desiFiles []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".desi") {
			desiFiles = append(desiFiles, filepath.Join(dir, e.Name()))
		}
	}

	if len(desiFiles) == 1 {
		return desiFiles[0]
	}

	return ""
}

// buildAndRun compiles the file and optionally runs it
// reloadCount tracks how many times we've reloaded (0 = first run)
// stateFilePath is the path for state serialization between reloads
func buildAndRun(file string, runAfterBuild bool, currentProcess **exec.Cmd, processMu *sync.Mutex, verbose bool, reloadCount int, stateFilePath string) {
	startTime := time.Now()

	// Kill existing process if running
	processMu.Lock()
	if *currentProcess != nil && (*currentProcess).Process != nil {
		if verbose {
			term.Println("  🔄 Stopping previous process...")
		}
		_ = (*currentProcess).Process.Signal(syscall.SIGTERM)
		// Give it a moment to exit gracefully
		done := make(chan error, 1)
		go func() {
			done <- (*currentProcess).Wait()
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = (*currentProcess).Process.Kill()
		}
		*currentProcess = nil
	}
	processMu.Unlock()

	// If not running, just type-check
	if !runAfterBuild {
		exePath, _ := os.Executable()
		cmd := exec.Command(exePath, "check", file)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		elapsed := time.Since(startTime)

		if err != nil {
			term.Printf("❌ Check failed (%.2fs)\n", elapsed.Seconds())
		} else {
			term.Printf("✅ Check passed (%.2fs)\n", elapsed.Seconds())
		}
		term.Flush()
		return
	}

	// Full build pipeline: emit-ir → clang → run
	tempDir, err := os.MkdirTemp("", "desi-watch-*")
	if err != nil {
		term.Eprintln("❌ Failed to create temp dir:", err)
		term.Flush()
		return
	}
	// Note: We don't remove tempDir immediately so the binary can run
	// It will be cleaned up on next build or OS cleanup

	baseName := strings.TrimSuffix(filepath.Base(file), ".desi")
	irPath := filepath.Join(tempDir, baseName+".ll")
	binPath := filepath.Join(tempDir, baseName)

	// Step 1: Emit LLVM IR
	if verbose {
		term.Println("  📝 Emitting IR...")
	}
	exePath, _ := os.Executable()
	emitCmd := exec.Command(exePath, "emit-ir", file)
	irOutput, err := emitCmd.Output()
	if err != nil {
		elapsed := time.Since(startTime)
		// Show stderr if available
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(exitErr.Stderr)
		}
		term.Printf("❌ Compile failed (%.2fs)\n", elapsed.Seconds())
		term.Flush()
		return
	}

	// Write IR to file
	if err := os.WriteFile(irPath, irOutput, 0644); err != nil {
		term.Eprintln("❌ Failed to write IR:", err)
		term.Flush()
		return
	}

	// Step 2: Compile with llc to object file
	if verbose {
		term.Println("  🔨 Compiling to object code...")
	}
	objPath := filepath.Join(tempDir, baseName+".o")
	llcCmd := exec.Command(findLLVMTool("llc"), irPath, "-filetype=obj", "-o", objPath)
	llcCmd.Stderr = os.Stderr
	if err := llcCmd.Run(); err != nil {
		elapsed := time.Since(startTime)
		term.Printf("❌ Compile failed (%.2fs) - is llc installed?\n", elapsed.Seconds())
		term.Flush()
		return
	}

	// Step 4: Create main stub (Desi uses __top__ as entry, but linker needs main)
	mainStub := filepath.Join(tempDir, "main_stub.c")
	stubContent := `extern int __top__(void);
int main(void) { return __top__(); }
`
	if err := os.WriteFile(mainStub, []byte(stubContent), 0644); err != nil {
		term.Eprintln("❌ Failed to write main stub:", err)
		term.Flush()
		return
	}

	// Step 5: Link with clang
	if verbose {
		term.Println("  🔗 Linking...")
	}

	// Find the build directory for libdesi.a
	buildDir := findBuildDir()
	clangArgs := []string{objPath, mainStub, "-o", binPath}
	if buildDir != "" {
		clangArgs = append(clangArgs, "-L"+buildDir, "-ldesi")
	}
	// Add common flags
	clangArgs = append(clangArgs, "-lm", "-Wl,-dead_strip")

	clangCmd := exec.Command(findLLVMTool("clang"), clangArgs...)
	clangCmd.Stderr = os.Stderr
	if err := clangCmd.Run(); err != nil {
		elapsed := time.Since(startTime)
		term.Printf("❌ Link failed (%.2fs) - is clang installed?\n", elapsed.Seconds())
		term.Flush()
		return
	}

	elapsed := time.Since(startTime)
	term.Printf("✅ Build succeeded (%.2fs)\n", elapsed.Seconds())
	term.Flush()

	// Step 6: Run the binary
	isReload := reloadCount > 0
	if isReload {
		term.Println("🔄 Restarting with state...")
	} else {
		term.Println("🚀 Running...")
	}
	term.Println("─────────────────────────────────────")
	term.Flush()

	runCmd := exec.Command(binPath)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	// Set environment variables for state serialization
	runCmd.Env = append(os.Environ(),
		fmt.Sprintf("DESI_RELOAD=%v", isReload),
		fmt.Sprintf("DESI_RELOAD_COUNT=%d", reloadCount),
		fmt.Sprintf("DESI_STATE_FILE=%s", stateFilePath),
	)

	processMu.Lock()
	*currentProcess = runCmd
	processMu.Unlock()

	// Start the process
	if err := runCmd.Start(); err != nil {
		term.Eprintln("❌ Failed to start:", err)
		term.Flush()
		return
	}

	if verbose {
		term.Printf("  (PID: %d)\n", runCmd.Process.Pid)
		term.Flush()
	}

	// Wait for process in background (don't block the watcher)
	go func() {
		err := runCmd.Wait()
		processMu.Lock()
		if *currentProcess == runCmd {
			*currentProcess = nil
		}
		processMu.Unlock()

		term.Println("─────────────────────────────────────")
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				term.Printf("⚠️  Process exited with code %d\n", exitErr.ExitCode())
			} else {
				term.Printf("⚠️  Process terminated: %v\n", err)
			}
		} else {
			term.Println("✅ Process exited normally")
		}
		term.Flush()
	}()
}

// findBuildDir looks for the Desi build directory containing libdesi.a
func findBuildDir() string {
	// Try common locations
	candidates := []string{
		"build",
		".",
	}

	// Also check relative to executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			exeDir,
			filepath.Join(exeDir, "build"),
			filepath.Join(exeDir, "..", "build"),
		)
	}

	// Check relative to working directory
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(wd, "build"),
			wd,
		)
	}

	for _, c := range candidates {
		libPath := filepath.Join(c, "libdesi.a")
		if _, err := os.Stat(libPath); err == nil {
			if abs, err := filepath.Abs(c); err == nil {
				return abs
			}
			return c
		}
	}

	return "" // Build dir not found
}

// buildAndRunHot compiles to shared library and runs via desi-host
// Returns the host process PID for sending reload signals
func buildAndRunHot(file string, soPath string, currentProcess **exec.Cmd, processMu *sync.Mutex, verbose bool) int {
	startTime := time.Now()

	// Stop existing host if running
	processMu.Lock()
	if *currentProcess != nil && (*currentProcess).Process != nil {
		_ = (*currentProcess).Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- (*currentProcess).Wait() }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = (*currentProcess).Process.Kill()
		}
		*currentProcess = nil
	}
	processMu.Unlock()

	// Create temp dir for build artifacts
	tempDir, err := os.MkdirTemp("", "desi-hot-*")
	if err != nil {
		term.Eprintln("❌ Failed to create temp dir:", err)
		term.Flush()
		return 0
	}

	baseName := strings.TrimSuffix(filepath.Base(file), ".desi")
	irPath := filepath.Join(tempDir, baseName+".ll")
	objPath := filepath.Join(tempDir, baseName+".o")

	// Step 1: Emit LLVM IR
	if verbose {
		term.Println("  📝 Emitting IR...")
	}
	exePath, _ := os.Executable()
	emitCmd := exec.Command(exePath, "emit-ir", file)
	irOutput, err := emitCmd.Output()
	if err != nil {
		elapsed := time.Since(startTime)
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(exitErr.Stderr)
		}
		term.Printf("❌ Compile failed (%.2fs)\n", elapsed.Seconds())
		term.Flush()
		return 0
	}

	if err := os.WriteFile(irPath, irOutput, 0644); err != nil {
		term.Eprintln("❌ Failed to write IR:", err)
		term.Flush()
		return 0
	}

	// Step 2: Compile to PIC object
	if verbose {
		term.Println("  🔨 Compiling to PIC object...")
	}
	llcCmd := exec.Command(findLLVMTool("llc"), irPath, "-filetype=obj", "-relocation-model=pic", "-o", objPath)
	llcCmd.Stderr = os.Stderr
	if err := llcCmd.Run(); err != nil {
		elapsed := time.Since(startTime)
		term.Printf("❌ LLC failed (%.2fs)\n", elapsed.Seconds())
		term.Flush()
		return 0
	}

	// Step 3: Link as shared library
	if verbose {
		term.Println("  🔗 Linking shared library...")
	}
	buildDir := findBuildDir()
	clangArgs := []string{"-shared", "-fPIC", "-o", soPath, objPath}
	if buildDir != "" {
		clangArgs = append(clangArgs, "-L"+buildDir, "-ldesi")
	}
	clangCmd := exec.Command(findLLVMTool("clang"), clangArgs...)
	clangCmd.Stderr = os.Stderr
	if err := clangCmd.Run(); err != nil {
		elapsed := time.Since(startTime)
		term.Printf("❌ Link failed (%.2fs)\n", elapsed.Seconds())
		term.Flush()
		return 0
	}

	elapsed := time.Since(startTime)
	term.Printf("✅ Build succeeded (%.2fs)\n", elapsed.Seconds())

	// Step 4: Start desi-host
	if verbose {
		term.Println("  🚀 Starting desi-host...")
	}
	term.Println("🔥 Running with hot reload...")
	term.Println("─────────────────────────────────────")
	term.Flush()

	hostPath := findHostBinary()
	if hostPath == "" {
		term.Eprintln("❌ desi-host binary not found")
		term.Flush()
		return 0
	}

	runCmd := exec.Command(hostPath, soPath)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	runCmd.Stdin = os.Stdin

	processMu.Lock()
	*currentProcess = runCmd
	processMu.Unlock()

	if err := runCmd.Start(); err != nil {
		term.Eprintln("❌ Failed to start host:", err)
		term.Flush()
		return 0
	}

	pid := runCmd.Process.Pid
	term.Printf("   [desi-host PID: %d - send SIGUSR1 to reload]\n", pid)
	term.Flush()

	// Wait for process in background
	go func() {
		err := runCmd.Wait()
		term.Println("─────────────────────────────────────")
		if err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				term.Printf("⚠️  Host terminated: %v\n", err)
			}
		} else {
			term.Println("✅ Host exited normally")
		}
		term.Flush()
	}()

	return pid
}

// doHotReload rebuilds .so and sends SIGUSR1 to reload
func doHotReload(file string, soPath string, hostPID int, verbose bool) bool {
	startTime := time.Now()
	term.Println("🔄 Hot reloading...")

	// Build new .so but save to temp location first
	tempDir, err := os.MkdirTemp("", "desi-hot-*")
	if err != nil {
		term.Eprintln("❌ Failed to create temp dir:", err)
		return false
	}
	defer os.RemoveAll(tempDir)

	baseName := strings.TrimSuffix(filepath.Base(file), ".desi")
	irPath := filepath.Join(tempDir, baseName+".ll")
	objPath := filepath.Join(tempDir, baseName+".o")

	// Step 1: Emit LLVM IR
	exePath, _ := os.Executable()
	emitCmd := exec.Command(exePath, "emit-ir", file)
	irOutput, err := emitCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Stderr.Write(exitErr.Stderr)
		}
		term.Printf("❌ Compile failed (%.2fs)\n", time.Since(startTime).Seconds())
		return false
	}

	if err := os.WriteFile(irPath, irOutput, 0644); err != nil {
		term.Eprintln("❌ Failed to write IR:", err)
		return false
	}

	// Step 2: Compile to PIC object
	llcCmd := exec.Command(findLLVMTool("llc"), irPath, "-filetype=obj", "-relocation-model=pic", "-o", objPath)
	if err := llcCmd.Run(); err != nil {
		term.Printf("❌ LLC failed (%.2fs)\n", time.Since(startTime).Seconds())
		return false
	}

	// Step 3: Link as shared library (overwrite existing)
	buildDir := findBuildDir()
	clangArgs := []string{"-shared", "-fPIC", "-o", soPath, objPath}
	if buildDir != "" {
		clangArgs = append(clangArgs, "-L"+buildDir, "-ldesi")
	}
	clangCmd := exec.Command(findLLVMTool("clang"), clangArgs...)
	if err := clangCmd.Run(); err != nil {
		term.Printf("❌ Link failed (%.2fs)\n", time.Since(startTime).Seconds())
		return false
	}

	// Step 4: Send SIGUSR1 to host
	proc, err := os.FindProcess(hostPID)
	if err != nil {
		term.Eprintln("❌ Failed to find host process:", err)
		return false
	}
	if err := proc.Signal(syscall.SIGUSR1); err != nil {
		term.Eprintln("❌ Failed to send reload signal:", err)
		return false
	}

	elapsed := time.Since(startTime)
	term.Printf("✅ Hot reload sent (%.2fs)\n", elapsed.Seconds())
	term.Flush()
	return true
}

// findHostBinary looks for desi-host binary
func findHostBinary() string {
	candidates := []string{
		"bin/desi-host",
		"desi-host",
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "desi-host"),
			filepath.Join(exeDir, "..", "bin", "desi-host"),
		)
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			if abs, err := filepath.Abs(c); err == nil {
				return abs
			}
			return c
		}
	}
	return ""
}
