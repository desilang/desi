//go:build windows

package main

import "fmt"

// runWatch is not supported on Windows — hot reload requires Unix signals (SIGUSR1)
// and shared library (.so) support which are unavailable on Windows.
func runWatch(args []string) int {
	fmt.Println("desic watch: hot reload is not supported on Windows")
	fmt.Println("Use Linux or macOS for the watch/hot-reload workflow.")
	return 1
}
