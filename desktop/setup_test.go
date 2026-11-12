package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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

// ── Release discovery (whisper.cpp's post-v1.9.3 layout) ─────────────────────

func namedAssets(names ...string) []ghAsset {
	assets := make([]ghAsset, len(names))
	for i, n := range names {
		assets[i] = ghAsset{Name: n, BrowserDownloadURL: "https://example.com/" + n}
	}
	return assets
}

// currentAssets mirrors build tag b5454 (the binaries for v1.9.5), where the
// CUDA zips were renamed from whisper-cublas-* to whisper-bin-win-cuda-*.
func currentAssets() []ghAsset {
	return namedAssets(
		"whisper-b5454-xcframework.zip",
		"whisper-bin-ubuntu-arm64.tar.gz",
		"whisper-bin-ubuntu-x64.tar.gz",
		"whisper-bin-win-cpu-arm64.zip",
		"whisper-bin-win-cuda-11.8.0-x64.zip",
		"whisper-bin-win-cuda-12.4.0-x64.zip",
		"whisper-bin-win-cuda-13.4-arm64.zip",
		"whisper-bin-win-opencl-adreno-arm64.zip",
		"whisper-bin-Win32.zip",
		"whisper-bin-x64.zip",
		"whisper-blas-bin-Win32.zip",
		"whisper-blas-bin-x64.zip",
	)
}

func TestPickBestAssetCurrentNamesWindowsNoGPU(t *testing.T) {
	// The renamed CUDA zips sort before the plain build and contain "x64";
	// unrecognized, one of them would be installed as the "cpu" build.
	url, variant := pickBestAsset(currentAssets(), "windows", "amd64", "none")
	if variant != "cpu" || url != "https://example.com/whisper-bin-x64.zip" {
		t.Errorf("got (%q, %q), want the plain x64 build as cpu", url, variant)
	}
}

func TestPickBestAssetCurrentNamesWindowsCUDA(t *testing.T) {
	url, variant := pickBestAsset(currentAssets(), "windows", "amd64", "cuda")
	if variant != "cuda" || url != "https://example.com/whisper-bin-win-cuda-11.8.0-x64.zip" {
		t.Errorf("got (%q, %q), want the CUDA 11.8 x64 build", url, variant)
	}
}

func TestPickBestAssetCurrentNamesLinux(t *testing.T) {
	url, variant := pickBestAsset(currentAssets(), "linux", "amd64", "none")
	if variant != "cpu" || url != "https://example.com/whisper-bin-ubuntu-x64.tar.gz" {
		t.Errorf("got (%q, %q), want the ubuntu x64 tarball", url, variant)
	}
}

func TestPickBestAssetLinuxNeverTakesAWindowsZip(t *testing.T) {
	// whisper-bin-x64.zip says "x64" and nothing about Windows.
	url, _ := pickBestAsset(namedAssets("whisper-bin-x64.zip", "whisper-blas-bin-x64.zip"), "linux", "amd64", "none")
	if url != "" {
		t.Errorf("url = %q, want no match", url)
	}
}

const v195Body = "## Overview\r\n\r\nMaintenance release.\r\n\r\n" +
	"**Nightly build:** [b5454](https://github.com/ggml-org/whisper.cpp/releases/tag/b5454)\r\n"

func TestSelectEngineReleaseFollowsBuildTagLink(t *testing.T) {
	// The real layout since v1.9.3: the versioned release has no assets and
	// links the build tag that does. A newer nightly must not be preferred
	// over the build belonging to the latest versioned release.
	rels := []ghRelease{
		{TagName: "b5460", Prerelease: true, Assets: currentAssets()},
		{TagName: "v1.9.5", Body: v195Body},
		{TagName: "b5454", Prerelease: true, Assets: currentAssets()},
		{TagName: "v1.9.4"},
	}
	rel, url, variant := selectEngineRelease(rels, "linux", "amd64", "none")
	if rel.TagName != "b5454" || variant != "cpu" || url != "https://example.com/whisper-bin-ubuntu-x64.tar.gz" {
		t.Errorf("got (%q, %q, %q), want b5454's ubuntu x64 tarball", rel.TagName, url, variant)
	}
}

func TestSelectEngineReleaseUsesAssetsOnVersionedRelease(t *testing.T) {
	// The layout up to v1.9.2: binaries attached to the versioned release.
	rels := []ghRelease{
		{TagName: "b9999", Prerelease: true, Assets: currentAssets()},
		{TagName: "v1.9.1", Assets: windowsAssets()},
	}
	rel, url, _ := selectEngineRelease(rels, "windows", "amd64", "none")
	if rel.TagName != "v1.9.1" || url != "https://example.com/whisper-bin-x64.zip" {
		t.Errorf("got (%q, %q), want v1.9.1's plain x64 build", rel.TagName, url)
	}
}

func TestSelectEngineReleaseFallsBackToNewestWithAssets(t *testing.T) {
	// Release notes without a recognizable build link, and drafts skipped.
	rels := []ghRelease{
		{TagName: "draft", Draft: true, Assets: currentAssets()},
		{TagName: "v2.0.0", Body: "no link here"},
		{TagName: "b6000", Prerelease: true, Assets: currentAssets()},
		{TagName: "b5999", Prerelease: true, Assets: currentAssets()},
	}
	rel, url, _ := selectEngineRelease(rels, "linux", "amd64", "none")
	if rel.TagName != "b6000" || url == "" {
		t.Errorf("got (%q, %q), want b6000", rel.TagName, url)
	}
}

func TestSelectEngineReleaseNothingUsable(t *testing.T) {
	rels := []ghRelease{{TagName: "v1.9.5", Body: v195Body}, {TagName: "b5454", Prerelease: true}}
	if _, url, _ := selectEngineRelease(rels, "linux", "amd64", "none"); url != "" {
		t.Errorf("url = %q, want empty", url)
	}
}

// ── Linux archive extraction ────────────────────────────────────────────────

func TestExtractTarGzEngineKeepsLibrarySymlinks(t *testing.T) {
	// Mirrors the real tarball: whisper-cli links against libggml.so.0, which
	// exists only as a symlink to the versioned file.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	file := func(name, content string) {
		tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len(content))})
		tw.Write([]byte(content))
	}
	link := func(name, target string) {
		tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeSymlink, Linkname: target, Mode: 0o777})
	}
	link("pkg/libggml.so", "libggml.so.0") // link listed before its target, as in the real archive
	link("pkg/libggml.so.0", "libggml.so.0.26.0")
	file("pkg/libggml.so.0.26.0", "ggml")
	file("pkg/whisper-cli", "cli")
	file("pkg/whisper-server", "not wanted")
	link("pkg/libevil.so", "../../outside")        // escapes destDir
	link("pkg/libevil2.so", "/etc/passwd")         // absolute
	link("pkg/whisper-bench-link", "libggml.so.0") // not a library name
	tw.Close()
	gz.Close()

	archive := filepath.Join(t.TempDir(), "engine.tar.gz")
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := extractEngineArchive(archive, dest, "whisper-cli"); err != nil {
		t.Fatalf("extract: %v", err)
	}

	// The soname must resolve, through the chain, to the real file's content.
	for _, name := range []string{"libggml.so", "libggml.so.0", "libggml.so.0.26.0"} {
		got, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil || string(got) != "ggml" {
			t.Errorf("%s: content %q, err %v — want it to resolve to the library", name, got, err)
		}
	}
	if fi, err := os.Lstat(filepath.Join(dest, "libggml.so.0")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("libggml.so.0 should be a symlink (mode %v, err %v)", fi.Mode(), err)
	}
	for _, name := range []string{"whisper-server", "libevil.so", "libevil2.so", "whisper-bench-link"} {
		if _, err := os.Lstat(filepath.Join(dest, name)); !os.IsNotExist(err) {
			t.Errorf("%s should not have been extracted", name)
		}
	}

	// A reinstall over an existing install must succeed (links replaced), and
	// resetEngineBinDir must clear links as well as files.
	if err := extractEngineArchive(archive, dest, "whisper-cli"); err != nil {
		t.Fatalf("re-extract over existing install: %v", err)
	}
	if err := resetEngineBinDir(dest); err != nil {
		t.Fatal(err)
	}
	if left, _ := os.ReadDir(dest); len(left) != 0 {
		t.Errorf("resetEngineBinDir left %d entries behind", len(left))
	}
}
