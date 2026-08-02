//go:build !windows

package main

// toShortPath is a no-op outside Windows — the ANSI-code-page argv mangling
// this works around (see shortpath_windows.go) is a Windows-only concern.
func toShortPath(path string) string {
	return path
}
