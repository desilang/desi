package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/desilang/desi/compiler/internal/term"
	"github.com/desilang/desi/compiler/internal/version"
)

func main() {
	term.Println("desirepl", version.String())
	term.Println("Type :quit to exit.")
	in := bufio.NewScanner(os.Stdin)

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
		default:
			// TODO: parse+typecheck+JIT-eval (LLVM ORC) later
			term.Println("echo:", line)
		}
	}

	term.Flush()
}
