package term

import (
	"fmt"
	"os"
)

// Stdout/Stderr print helpers that ignore (n, err) to satisfy linters.
func Printf(format string, a ...any)  { _, _ = fmt.Printf(format, a...) }
func Println(a ...any)                { _, _ = fmt.Println(a...) }
func Eprintf(format string, a ...any) { _, _ = fmt.Fprintf(os.Stderr, format, a...) }
func Eprintln(a ...any)               { _, _ = fmt.Fprintln(os.Stderr, a...) }
