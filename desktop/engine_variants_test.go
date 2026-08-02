package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Real stderr fragment from the field report that motivated this feature
// (AMD Zen 3 machine, whisper-cli died right after this line).
const windowsCrashStderr = `load_backend: loaded CPU backend from C:\Users\Eva Trnková\AppData\Roaming\dictate-assistant\bin\ggml-cpu-haswell.dll`

const linuxCrashStderr = `load_backend: loaded CPU backend from /home/eva/.config/dictate-assistant/bin/libggml-cpu-haswell.so`

// Multiple backends load on a healthy run; only the CPU line must match.
const macosHealthyPrefix = `load_backend: loaded BLAS backend from /opt/homebrew/Cellar/ggml/0.15.3/libexec/libggml-blas.so
load_backend: loaded MTL backend from /opt/homebrew/Cellar/ggml/0.15.3/libexec/libggml-metal.so
load_backend: loaded CPU backend from /opt/homebrew/Cellar/ggml/0.15.3/libexec/libggml-cpu-apple_m1.so`

func TestLoadedCPUVariant(t *testing.T) {
	cases := []struct {
		name, stderr, want string
	}{
		{"windows path with spaces and diacritics", windowsCrashStderr,
			`C:\Users\Eva Trnková\AppData\Roaming\dictate-assistant\bin\ggml-cpu-haswell.dll`},
		{"linux path", linuxCrashStderr,
			"/home/eva/.config/dictate-assistant/bin/libggml-cpu-haswell.so"},
		{"picks CPU line among other backends", macosHealthyPrefix,
			"/opt/homebrew/Cellar/ggml/0.15.3/libexec/libggml-cpu-apple_m1.so"},
		{"no backend line", "whisper_model_load: loading model", ""},
	}
	for _, c := range cases {
		if got := loadedCPUVariant(c.stderr); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestLooksLikeBackendCrash(t *testing.T) {
	if !looksLikeBackendCrash(windowsCrashStderr) {
		t.Error("bare backend-load line should look like a crash")
	}
	legit := windowsCrashStderr + "\nread_audio_data: failed to read audio data\nerror: failed to read audio file 'x.wav'"
	if looksLikeBackendCrash(legit) {
		t.Error("a real whisper-cli error must not be treated as a crash")
	}
}

func TestIsBaselineCPUVariant(t *testing.T) {
	baselines := []string{
		`C:\x\ggml-cpu-x64.dll`, `C:\x\ggml-cpu.dll`,
		"/x/libggml-cpu-x64.so", "/x/libggml-cpu.so",
	}
	for _, p := range baselines {
		if !isBaselineCPUVariant(p) {
			t.Errorf("%s should be baseline", p)
		}
	}
	quarantinable := []string{
		`C:\x\ggml-cpu-haswell.dll`, "/x/libggml-cpu-skylakex.so", `C:\x\ggml-cpu-sse42.dll`,
	}
	for _, p := range quarantinable {
		if isBaselineCPUVariant(p) {
			t.Errorf("%s should be quarantinable", p)
		}
	}
}

func TestQuarantineCPUVariant(t *testing.T) {
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "whisper-cli")
	lib := filepath.Join(binDir, "ggml-cpu-haswell.dll")
	for _, f := range []string{binPath, lib} {
		if err := os.WriteFile(f, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := quarantineCPUVariant(binPath, lib); err != nil {
		t.Fatalf("quarantine failed: %v", err)
	}
	if _, err := os.Stat(lib); !os.IsNotExist(err) {
		t.Error("library should have been moved out of the engine dir")
	}
	moved := filepath.Join(binDir, "disabled-variants", "ggml-cpu-haswell.dll")
	if _, err := os.Stat(moved); err != nil {
		t.Errorf("library should exist in quarantine dir: %v", err)
	}

	// Refuses libraries outside the managed dir (e.g. Homebrew's).
	foreign := filepath.Join(t.TempDir(), "libggml-cpu-apple_m1.so")
	if err := os.WriteFile(foreign, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := quarantineCPUVariant(binPath, foreign); err == nil {
		t.Error("must refuse to quarantine a library outside the engine dir")
	}

	// Refuses baseline variants.
	baseline := filepath.Join(binDir, "ggml-cpu-x64.dll")
	if err := os.WriteFile(baseline, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := quarantineCPUVariant(binPath, baseline); err == nil {
		t.Error("must refuse to quarantine the baseline variant")
	}
}

func TestRestoreQuarantinedVariants(t *testing.T) {
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "whisper-cli")
	skylakex := filepath.Join(binDir, "ggml-cpu-skylakex.dll")
	haswell := filepath.Join(binDir, "ggml-cpu-haswell.dll")
	for _, f := range []string{binPath, skylakex, haswell} {
		if err := os.WriteFile(f, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Quarantine both, simulating a run that cycled through every variant.
	if err := quarantineCPUVariant(binPath, skylakex); err != nil {
		t.Fatal(err)
	}
	if err := quarantineCPUVariant(binPath, haswell); err != nil {
		t.Fatal(err)
	}

	// The terminal failure (baseline crashed too) should put everything back,
	// including anything quarantined by an earlier, separate run.
	restoreAllQuarantinedVariants(binPath)

	for _, name := range []string{"ggml-cpu-skylakex.dll", "ggml-cpu-haswell.dll"} {
		if _, err := os.Stat(filepath.Join(binDir, name)); err != nil {
			t.Errorf("%s should be back in the engine dir: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(binDir, "disabled-variants", name)); !os.IsNotExist(err) {
			t.Errorf("%s should no longer be in quarantine", name)
		}
	}
}

func TestDiagnoseCrashMentionsRealCauses(t *testing.T) {
	msg := diagnoseCrash("large-v3-turbo", 3221225477, "load_backend: loaded CPU backend from x64.dll")
	for _, want := range []string{"large-v3-turbo", "3221225477", "vc_redist", "RAM"} {
		if !strings.Contains(msg, want) {
			t.Errorf("diagnostic message missing %q:\n%s", want, msg)
		}
	}
}
