package runtimetest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// print skips its lock while the program is still single-threaded, which is
// only correct if every runtime path that starts a thread says so first. That
// is a standing obligation on runtime C code rather than something the compiler
// can check, so this test enforces it: a source file that creates a thread must
// also call __desi_note_thread_start.
//
// File-level granularity is deliberate. Matching each creation to a preceding
// call would mean parsing C; requiring the file to mention the function at all
// is enough to make someone adding a thread to a fresh file think about it, and
// someone adding one to a file that already has it is editing code with the
// call visible a few lines away.
func TestEveryThreadCreatorNotesItsThread(t *testing.T) {
	root := filepath.Join("..", "..", "..", "compiler", "runtime")

	// sqlite3 runs its own threads and never calls back into Desi code, so
	// nothing it does can reach print.
	exempt := map[string]bool{
		"sqlite3_amalg.c": true,
		"sqlite3.c":       true,
	}

	creators := []string{"pthread_create", "CreateThread(", "_beginthreadex"}

	var missing []string
	walk := func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".c") {
			return err
		}
		if exempt[filepath.Base(path)] {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(body)
		makesThreads := false
		for _, c := range creators {
			if strings.Contains(src, c) {
				makesThreads = true
				break
			}
		}
		if makesThreads && !strings.Contains(src, "__desi_note_thread_start") {
			missing = append(missing, filepath.Base(path))
		}
		return nil
	}
	if err := filepath.WalkDir(root, walk); err != nil {
		t.Fatalf("walking the runtime sources: %v", err)
	}

	if len(missing) != 0 {
		t.Errorf("these runtime files start a thread without calling "+
			"__desi_note_thread_start() first: %v\n\n"+
			"print skips its lock until that call happens, so a thread started "+
			"without it can tear another thread's output. Add the call before "+
			"the thread is created; see print.c.", missing)
	}
}

// The flag is only useful if the files that do start threads are actually
// found by the search above — a typo in the creator list would make this suite
// pass by finding nothing at all.
func TestThreadCreatorScanFindsTheKnownFiles(t *testing.T) {
	root := filepath.Join("..", "..", "..", "compiler", "runtime")
	want := []string{"future.c", "scheduler.c", "supervisor.c", "websocket.c"}

	found := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".c") {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(body), "__desi_note_thread_start();") {
			found[filepath.Base(path)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the runtime sources: %v", err)
	}
	for _, name := range want {
		if !found[name] {
			t.Errorf("%s no longer calls __desi_note_thread_start(); if its threads "+
				"were removed, drop it from this list", name)
		}
	}
}
