package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
			continue
		}

		// Type check
		diags, _ := check.Check(mod)
		hasDiags := false
		for _, d := range diags {
			if d.Domain == "warn" || d.Domain == "type" || d.Domain == "class" {
				renderDiag(d)
				hasDiags = true
			}
		}

		if !hasDiags {
			term.Println("ok")
		}
	}

	term.Flush()
}

func renderDiag(d diag.Diagnostic) {
	prefix := "info"
	switch d.Domain {
	case "type":
		prefix = "type error"
	case "warn":
		prefix = "warning"
	case "class":
		prefix = "class error"
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
