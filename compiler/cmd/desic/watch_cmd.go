package main

import (
	"context"
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
// Flags: -v (verbose), --run (run after build)
func runWatch(args []string) int {
	verbose := false
	runAfterBuild := false
	var targetDir string

	for _, a := range args {
		switch a {
		case "-v", "--verbose":
			verbose = true
		case "--run":
			runAfterBuild = true
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

	// Initial build
	mainFile := findMainFile(absDir)
	if mainFile != "" {
		term.Println("📦 Building:", mainFile)
		term.Flush()
		buildAndRun(mainFile, runAfterBuild, &currentProcess, &processMu, verbose)
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

				buildAndRun(buildTarget, runAfterBuild, &currentProcess, &processMu, verbose)
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
func buildAndRun(file string, runAfterBuild bool, currentProcess **exec.Cmd, processMu *sync.Mutex, verbose bool) {
	startTime := time.Now()

	// Kill existing process if running
	processMu.Lock()
	if *currentProcess != nil && (*currentProcess).Process != nil {
		if verbose {
			term.Println("  Stopping previous process...")
		}
		_ = (*currentProcess).Process.Signal(syscall.SIGTERM)
		_ = (*currentProcess).Wait()
		*currentProcess = nil
	}
	processMu.Unlock()

	// Run desic build (or check for now)
	// For Phase 1, we'll just run 'desic check' to verify the code compiles
	cmd := exec.Command("desic", "check", file)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	elapsed := time.Since(startTime)

	if err != nil {
		term.Printf("❌ Build failed (%.2fs)\n", elapsed.Seconds())
	} else {
		term.Printf("✅ Build succeeded (%.2fs)\n", elapsed.Seconds())

		// If --run flag and we have a build command that produces a binary,
		// we would run it here. For now, just show success.
		if runAfterBuild {
			term.Println("   (--run: binary execution not yet implemented)")
		}
	}
	term.Flush()
}
