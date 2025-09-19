package parsebridge

import (
	"bytes"
	"errors"
	"fmt"
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
	cmd.Stdout = &out
	if verbose {
		cmd.Stderr = os.Stderr
	} else {
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

	data := out.Bytes()
	// Same sanitizer as auto-build path: grab the first top-level JSON object.
	data = sanitizeBridgeOutput(data)
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("parsebridge produced empty output")
	}
	return data, nil
}
