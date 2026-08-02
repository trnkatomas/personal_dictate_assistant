package main

import (
	"log"
	"syscall"
)

// toShortPath converts an existing file's path to its legacy 8.3 short-path
// form when it contains non-ASCII characters.
//
// Why this exists: whisper-cli is a plain `int main(int argc, char **argv)`
// C++ program. On Windows, the CRT populates that narrow argv by converting
// the real (Unicode) command line through the process's ANSI code page, not
// UTF-8. A path containing a character outside that code page — e.g. a
// Czech "š" in the user's profile directory, under an English-US system
// locale — gets silently mangled in that conversion. whisper-cli then fails
// to open the (corrupted) path with no error text printed at all: the exact
// same "loaded backend, then died with nothing else printed" signature this
// app already watches for as a bad-CPU-variant crash (see
// engine_variants.go). Quarantining CPU variants can never fix this,
// because a bad variant was never the actual cause — confirmed on a real
// machine where an absolute model path containing "Miloš" reproduced this
// exact hang, while the same invocation with a relative path (never
// mentioning the accented directory) worked every time.
//
// The short-path form is pure ASCII by construction, so it survives the
// ANSI round-trip unchanged regardless of system locale — no need to
// restructure how/where we invoke whisper-cli, or reason about relative
// paths and working directories.
//
// Requires the path to already exist (GetShortPathName looks it up on
// disk) — true at every call site here, since the model and audio files
// are always downloaded/written before whisper-cli is invoked on them.
// Returns the original path unchanged if shortening isn't possible for any
// reason (already ASCII, the file doesn't exist, or the syscall fails) —
// never worse than doing nothing.
func toShortPath(path string) string {
	if isASCIIPath(path) {
		return path
	}

	longPath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return path
	}

	buf := make([]uint16, 260)
	n, err := syscall.GetShortPathName(longPath, &buf[0], uint32(len(buf)))
	if err != nil {
		log.Printf("toShortPath: could not shorten %q, using it as-is: %v", path, err)
		return path
	}
	if n > uint32(len(buf)) {
		buf = make([]uint16, n)
		if n, err = syscall.GetShortPathName(longPath, &buf[0], uint32(len(buf))); err != nil {
			log.Printf("toShortPath: could not shorten %q, using it as-is: %v", path, err)
			return path
		}
	}
	return syscall.UTF16ToString(buf[:n])
}
