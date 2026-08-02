package main

import "testing"

func TestIsASCIIPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{`C:\Users\tomas\AppData\Roaming\dictate-assistant\models\ggml-large-v3.bin`, true},
		{`C:\Users\Miloš Trnka\AppData\Roaming\dictate-assistant\models\ggml-large-v3.bin`, false},
		{`/home/user/recording.wav`, true},
		{`/home/uživatel/recording.wav`, false},
		{"", true},
	}
	for _, c := range cases {
		if got := isASCIIPath(c.path); got != c.want {
			t.Errorf("isASCIIPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// toShortPath itself is only meaningfully testable on Windows (it shells out
// to GetShortPathName, which requires the path to exist on an NTFS/FAT
// volume) — the !windows build's no-op passthrough is trivial by
// construction, and the windows build isn't runnable in this environment.
// isASCIIPath above covers the one piece of logic shared across both.
