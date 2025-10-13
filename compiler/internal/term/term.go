package term

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	stdoutOnce sync.Once
	stderrOnce sync.Once
	_stdout    *bufio.Writer
	_stderr    *bufio.Writer
)

// out returns a buffered stdout writer (lazy).
func out() *bufio.Writer {
	stdoutOnce.Do(func() { _stdout = bufio.NewWriter(os.Stdout) })
	return _stdout
}

// err returns a buffered stderr writer (lazy).
func err() *bufio.Writer {
	stderrOnce.Do(func() { _stderr = bufio.NewWriter(os.Stderr) })
	return _stderr
}

// Flush flushes both stdout and stderr buffers (ignore errors).
func Flush() {
	if _stdout != nil {
		_ = _stdout.Flush()
	}
	if _stderr != nil {
		_ = _stderr.Flush()
	}
}

// -------- stdout helpers (ignore write errors) --------

func Print(a ...any) {
	_, _ = fmt.Fprint(out(), a...)
}

func Println(a ...any) {
	_, _ = fmt.Fprintln(out(), a...)
}

func Printf(format string, a ...any) {
	_, _ = fmt.Fprintf(out(), format, a...)
}

// Prompt prints a prompt without newline and flushes so it appears immediately.
func Prompt(s string) {
	_, _ = fmt.Fprint(out(), s)
	_ = out().Flush()
}

// Write writes raw bytes to an io.Writer, ignoring errors (useful for os.Stdout).
func Write(w io.Writer, b []byte) {
	_, _ = w.Write(b)
}

// -------- stderr helpers (ignore write errors) --------

func Eprint(a ...any) {
	_, _ = fmt.Fprint(err(), a...)
}

func Eprintln(a ...any) {
	_, _ = fmt.Fprintln(err(), a...)
}

func Eprintf(format string, a ...any) {
	_, _ = fmt.Fprintf(err(), format, a...)
}

// -------- convenience for common patterns --------

// Check prints an error to stderr (if non-nil) and returns true if an error was printed.
func Check(err error, context string) bool {
	if err == nil {
		return false
	}
	Eprintf("%s: %v\n", context, err)
	return true
}
