package parsebridge

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type NotInstalledError struct {
	Tried string
}

func (e NotInstalledError) Error() string {
	return "parsebridge not installed (tried: " + e.Tried + ")"
}

// Run executes a Desi parser binary that prints AST JSON for <path> to stdout.
// If bin is empty, we try $DESI_PARSEBRIDGE, then "desi-parsebridge" on PATH.
// On success, returns stdout (JSON). On failure, returns stderr in the error.
func Run(path string, bin string, verbose bool) ([]byte, error) {
	tried := []string{}
	if strings.TrimSpace(bin) == "" {
		if env := strings.TrimSpace(os.Getenv("DESI_PARSEBRIDGE")); env != "" {
			bin = env
		}
	}
	if strings.TrimSpace(bin) == "" {
		bin = "desi-parsebridge"
	}

	if _, err := exec.LookPath(bin); err != nil {
		tried = append(tried, bin)
		return nil, NotInstalledError{Tried: strings.Join(tried, ", ")}
	}

	cmd := exec.Command(bin, path)
	var out, errb bytes.Buffer
	if verbose {
		cmd.Stdout = io.MultiWriter(os.Stdout, &out) // tee for debugging
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = &out
		cmd.Stderr = &errb
	}

	if err := cmd.Run(); err != nil {
		if !verbose {
			msg := strings.TrimSpace(errb.String())
			if msg != "" {
				return nil, fmt.Errorf("parsebridge failed: %s", msg)
			}
		}
		return nil, fmt.Errorf("parsebridge failed: %w", err)
	}

	raw := out.Bytes()
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("parsebridge produced empty output")
	}
	clean, err := sanitizeJSONOutput(raw)
	if err != nil {
		return nil, err
	}
	return clean, nil
}
