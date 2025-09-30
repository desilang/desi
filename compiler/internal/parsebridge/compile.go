package parsebridge

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
)

// compileBin compiles C to outBin using clang.
func compileBin(cpath, rtDir, outBin string, verbose bool) error {
	clang := "clang"
	args := []string{
		cpath,
		filepath.Join(rtDir, "desi_std.c"),
		"-I", rtDir,
		"-D_CRT_SECURE_NO_WARNINGS",
		"-o", outBin,
	}
	cc := exec.Command(clang, args...)
	if verbose {
		cc.Stdout = os.Stdout
		cc.Stderr = os.Stderr
	} else {
		var sink bytes.Buffer
		cc.Stdout = &sink
		cc.Stderr = &sink
	}
	return cc.Run()
}
