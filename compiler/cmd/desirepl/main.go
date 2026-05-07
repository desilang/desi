package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/term"
	"github.com/desilang/desi/compiler/internal/version"
)

func main() {
	term.Println("desirepl", version.String())
	term.Println("Type expressions or statements. :quit to exit.")
	term.Println("")
	in := bufio.NewScanner(os.Stdin)

	lineNum := 0
	showAST := false
	for {
		term.Prompt(">>> ")
		if !in.Scan() {
			break
		}
		line := strings.TrimSpace(in.Text())
		switch line {
		case ":quit", ":q", "quit", "exit":
			term.Flush()
			return
		case "":
			continue
		case ":help", ":h":
			term.Println(":quit    Exit the REPL")
			term.Println(":help    Show this message")
			term.Println(":ast     Show AST for next input")
			continue
		case ":ast":
			showAST = true
			term.Println("(will show AST for next input)")
			continue
		}

		lineNum++
		// Wrap input in a function body so statements parse correctly
		src := fmt.Sprintf("def __repl_%d():\n\t%s\n", lineNum, line)
		fileName := fmt.Sprintf("<repl:%d>", lineNum)

		// Parse
		mod, parseErrs := parse.ParseFile(fileName, []byte(src))
		if len(parseErrs) > 0 {
			for _, e := range parseErrs {
				fmt.Fprintf(os.Stderr, "  parse error: %s\n", e.Message)
			}
			showAST = false
			continue
		}

		// Show AST if requested
		if showAST {
			ast.Print(os.Stdout, mod)
			showAST = false
		}

		// Type check
		diags, _ := check.Check(mod)
		hasDiags := false
		for _, d := range diags {
			// Show all diagnostic domains (type, warn, class, borrow,
			// call, collections, numeric, ffi, sync, module, project)
			renderDiag(d)
			hasDiags = true
		}

		if !hasDiags {
			term.Println("ok")
		}
	}

	term.Flush()
}

func renderDiag(d diag.Diagnostic) {
	prefix := d.Domain
	switch d.Domain {
	case "type":
		prefix = "type error"
	case "warn":
		prefix = "warning"
	case "class":
		prefix = "class error"
	case "borrow":
		prefix = "borrow error"
	case "call":
		prefix = "call error"
	case "collections":
		prefix = "collection error"
	case "numeric":
		prefix = "numeric error"
	case "ffi":
		prefix = "ffi error"
	case "sync":
		prefix = "concurrency error"
	case "module":
		prefix = "module error"
	case "project":
		prefix = "project error"
	}
	msg := d.Message
	if msg == "" {
		msg = d.Title
	}
	if d.CodeID != "" {
		fmt.Fprintf(os.Stderr, "  [%s] %s: %s\n", d.CodeID, prefix, msg)
	} else {
		fmt.Fprintf(os.Stderr, "  %s: %s\n", prefix, msg)
	}
}
