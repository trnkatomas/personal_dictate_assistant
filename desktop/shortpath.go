package main

// isASCIIPath reports whether p contains only ASCII bytes. Used to decide
// whether a path needs Windows short-path conversion before being handed to
// whisper-cli as a command-line argument — see toShortPath in
// shortpath_windows.go.
func isASCIIPath(p string) bool {
	for i := 0; i < len(p); i++ {
		if p[i] > 127 {
			return false
		}
	}
	return true
}
