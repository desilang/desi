package parsebridge

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func runAndClean(binPath, cacheRoot string, verbose bool) ([]byte, error) {
	absBin, _ := filepath.Abs(binPath)
	cmd := exec.Command(absBin)
	var out bytes.Buffer
	if verbose {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = io.Discard
	}
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		if verbose {
			return nil, fmt.Errorf("run parsebridge: %w", err)
		}
		return nil, fmt.Errorf("run parsebridge failed; re-run with --bridge-verbose for details")
	}

	raw := out.Bytes()
	if verbose {
		_ = os.WriteFile(filepath.Join(cacheRoot, "bridge_raw.out.txt"), raw, 0o644)
	}

	clean, err := sanitizeJSONOutput(raw)
	if err != nil {
		if verbose {
			_, _ = fmt.Fprintln(os.Stderr, "---- parsebridge raw stdout (first 4KB) ----")
			lim := raw
			if len(lim) > 4096 {
				lim = lim[:4096]
			}
			_, _ = os.Stderr.Write(lim)
			_, _ = fmt.Fprintln(os.Stderr, "\n---- end raw ----")
		}
		return nil, err
	}

	if verbose {
		_ = os.WriteFile(filepath.Join(cacheRoot, "bridge_clean.json"), clean, 0o644)
	}
	return clean, nil
}
