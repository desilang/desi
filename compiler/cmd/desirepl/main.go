package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/desilang/desi/compiler/internal/term"
)

const Version = "0.0.1-rev6-bootstrap"

func main() {
	term.Println("desirepl", Version)
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
