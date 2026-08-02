package main

import (
	"os"
	"path/filepath"
	"testing"
)

func windowsAssets() []ghAsset {
	// Mirrors the actual asset list from whisper.cpp's latest GitHub release
	// (v1.9.1) at the time this was written.
	names := []string{
		"whisper-bin-ubuntu-arm64.tar.gz",
		"whisper-bin-ubuntu-x64.tar.gz",
		"whisper-bin-Win32.zip",
		"whisper-bin-x64.zip",
		"whisper-blas-bin-Win32.zip",
		"whisper-blas-bin-x64.zip",
		"whisper-cublas-11.8.0-bin-x64.zip",
		"whisper-cublas-12.4.0-bin-x64.zip",
	}
	assets := make([]ghAsset, len(names))
	for i, n := range names {
		assets[i] = ghAsset{Name: n, BrowserDownloadURL: "https://example.com/" + n}
	}
	return assets
}

func TestPickBestAssetWindowsNoGPUSkipsCublas(t *testing.T) {
	url, variant := pickBestAsset(windowsAssets(), "windows", "amd64", "none")
	if variant != "cpu" {
		t.Errorf("variant = %q, want cpu", variant)
	}
	if url != "https://example.com/whisper-bin-x64.zip" {
		t.Errorf("url = %q, want the plain x64 build", url)
	}
}

func TestPickBestAssetWindowsCUDAPrefersOlderToolkit(t *testing.T) {
	// This is the actual bug report: CUDA was detected but the app still
	// downloaded the CPU-only build. gpu="cuda" must select a cublas asset,
	// and specifically the 11.8 one for broader driver compatibility.
	url, variant := pickBestAsset(windowsAssets(), "windows", "amd64", "cuda")
	if variant != "cuda" {
		t.Errorf("variant = %q, want cuda", variant)
	}
	if url != "https://example.com/whisper-cublas-11.8.0-bin-x64.zip" {
		t.Errorf("url = %q, want the cublas 11.8.0 build", url)
	}
}

func TestPickBestAssetWindowsCUDAFallsBackToNewerToolkitIfOnlyOneAvailable(t *testing.T) {
	assets := []ghAsset{
		{Name: "whisper-bin-x64.zip", BrowserDownloadURL: "https://example.com/plain"},
		{Name: "whisper-cublas-12.4.0-bin-x64.zip", BrowserDownloadURL: "https://example.com/cublas124"},
	}
	url, variant := pickBestAsset(assets, "windows", "amd64", "cuda")
	if variant != "cuda" || url != "https://example.com/cublas124" {
		t.Errorf("got (%q, %q), want the cublas 12.4.0 build", url, variant)
	}
}

func TestPickBestAssetLinuxIgnoresGPU(t *testing.T) {
	// whisper.cpp's Linux releases have no CUDA build at all — gpu="cuda"
	// must not change anything or cause a failure to match.
	url, variant := pickBestAsset(windowsAssets(), "linux", "amd64", "cuda")
	if variant != "cpu" {
		t.Errorf("variant = %q, want cpu", variant)
	}
	if url != "https://example.com/whisper-bin-ubuntu-x64.tar.gz" {
		t.Errorf("url = %q, want the ubuntu x64 build", url)
	}
}

func TestEngineVariantRoundTrip(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "whisper-cli")

	if got := readEngineVariant(binPath); got != "cpu" {
		t.Errorf("with no marker, readEngineVariant = %q, want cpu", got)
	}

	if err := writeEngineVariant(binPath, "cuda"); err != nil {
		t.Fatal(err)
	}
	if got := readEngineVariant(binPath); got != "cuda" {
		t.Errorf("readEngineVariant = %q, want cuda", got)
	}

	if err := writeEngineVariant(binPath, "cpu"); err != nil {
		t.Fatal(err)
	}
	if got := readEngineVariant(binPath); got != "cpu" {
		t.Errorf("readEngineVariant = %q, want cpu", got)
	}
}

func TestResetEngineBinDirClearsFilesKeepsSubdirs(t *testing.T) {
	dir := t.TempDir()
	files := []string{"whisper-cli.exe", "ggml-cuda.dll", "cudart64_12.dll", ".engine-variant"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	quarantineSubdir := filepath.Join(dir, "disabled-variants")
	if err := os.MkdirAll(quarantineSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(quarantineSubdir, "ggml-cpu-haswell.dll"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := resetEngineBinDir(dir); err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed", f)
		}
	}
	if _, err := os.Stat(filepath.Join(quarantineSubdir, "ggml-cpu-haswell.dll")); err != nil {
		t.Errorf("quarantine subdir contents should survive a reset: %v", err)
	}
}

func TestResetEngineBinDirToleratesMissingDir(t *testing.T) {
	if err := resetEngineBinDir(filepath.Join(t.TempDir(), "does-not-exist")); err != nil {
		t.Errorf("missing binDir should not be an error, got: %v", err)
	}
}

func TestWriteSilentWAVProducesValidHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "silence.wav")
	if err := writeSilentWAV(path, 0.5, 16000); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	wantDataSize := int(0.5 * 16000 * 2)
	wantTotal := 44 + wantDataSize // standard WAV header is 44 bytes
	if len(data) != wantTotal {
		t.Errorf("file size = %d, want %d", len(data), wantTotal)
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		t.Errorf("missing RIFF/WAVE markers: %q", data[:12])
	}
	if string(data[12:16]) != "fmt " || string(data[36:40]) != "data" {
		t.Errorf("missing fmt /data chunk markers")
	}
}
