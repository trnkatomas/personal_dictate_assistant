package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBuildProgressPercent(t *testing.T) {
	cases := []struct {
		line string
		want float64
		ok   bool
	}{
		{"[ 42%] Building CXX object src/CMakeFiles/whisper.dir/whisper.cpp.o", 42, true},
		{"[100%] Built target whisper-cli", 100, true},
		{"[17/68] Building CXX object ggml/src/ggml-vulkan/ggml-vulkan.cpp.o", 25, true},
		{"-- Vulkan found", 0, false},
		{"[/] nonsense", 0, false},
	}
	for _, c := range cases {
		got, ok := buildProgressPercent(c.line)
		if ok != c.ok || got != c.want {
			t.Errorf("%q: got (%v, %v), want (%v, %v)", c.line, got, ok, c.want, c.ok)
		}
	}
}

func TestSourceReleaseTag(t *testing.T) {
	rels := []ghRelease{
		{TagName: "b5460", Prerelease: true},
		{TagName: "v2.0.0", Draft: true},
		{TagName: "v1.9.5"},
		{TagName: "b5454", Prerelease: true},
	}
	if got := sourceReleaseTag(rels); got != "v1.9.5" {
		t.Errorf("got %q, want the newest versioned release v1.9.5", got)
	}
	if got := sourceReleaseTag([]ghRelease{{TagName: "b1", Prerelease: true}}); got != "b1" {
		t.Errorf("with only build tags, got %q, want b1", got)
	}
	if got := sourceReleaseTag(nil); got != "" {
		t.Errorf("with no releases, got %q", got)
	}
}

func TestParseVulkanDevices(t *testing.T) {
	gpu := "GPU0:\n\tapiVersion         = 1.3.255\n\tdeviceType         = PHYSICAL_DEVICE_TYPE_DISCRETE_GPU\n\tdeviceName         = AMD Radeon RX 6700 XT (RADV NAVI22)\n"
	// Real llvmpipe block from the Debian 13 test image.
	cpu := "GPU1:\n\tapiVersion         = 1.4.305\n\tdeviceType         = PHYSICAL_DEVICE_TYPE_CPU\n\tdeviceName         = llvmpipe (LLVM 19.1.7, 128 bits)\n"
	cases := []struct {
		name, out, wantGPU string
		wantSoftwareOnly   bool
	}{
		{"real GPU", "Devices:\n========\n" + gpu, "AMD Radeon RX 6700 XT (RADV NAVI22)", false},
		{"GPU listed after llvmpipe", "Devices:\n" + cpu + gpu, "AMD Radeon RX 6700 XT (RADV NAVI22)", false},
		{"llvmpipe only", "Devices:\n" + cpu, "", true},
		{"nothing", "", "", false},
	}
	for _, c := range cases {
		g, so := parseVulkanDevices([]byte(c.out))
		if g != c.wantGPU || so != c.wantSoftwareOnly {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", c.name, g, so, c.wantGPU, c.wantSoftwareOnly)
		}
	}
}

func TestEngineVariantVulkanRoundTrip(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "whisper-cli")
	if err := writeEngineVariant(binPath, engineVulkan); err != nil {
		t.Fatal(err)
	}
	if got := readEngineVariant(binPath); got != engineVulkan {
		t.Errorf("got %q, want %q", got, engineVulkan)
	}
}

func TestGetVulkanBuildInfoOffLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("covered by the Linux container run")
	}
	if info := (&App{}).GetVulkanBuildInfo(); info.Supported {
		t.Errorf("Vulkan build must not be offered on %s", runtime.GOOS)
	}
}

// TestFFmpegRecordingLifecycle drives the capture start/stop logic against a
// fake ffmpeg (a shell script), so it runs anywhere with sh. Real capture is
// exercised separately on Linux with PulseAudio.
func TestFFmpegRecordingLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "ffmpeg")
	// Fails immediately for -f pulse (like a system without PulseAudio);
	// for alsa, "connects" for 0.3 s, then writes audio and keeps recording
	// until it reads "q".
	script := `#!/bin/sh
case "$*" in *"-f pulse"*) echo "pulse: Connection refused" >&2; exit 1;; esac
for last; do :; done
: > "$last"
sleep 0.3
head -c 8000 /dev/zero >> "$last"
while read line; do [ "$line" = q ] && { printf 'tail' >> "$last"; exit 0; }; done
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "rec.wav")

	began := time.Now()
	rec, err := startFFmpegRecording(fake, out)
	if err != nil {
		t.Fatalf("expected fallback to alsa to succeed: %v", err)
	}
	if waited := time.Since(began); waited < 300*time.Millisecond {
		t.Errorf("reported recording after %v, before any audio was written", waited)
	}
	if got := rec.cmd.Args; got[5] != "alsa" {
		t.Errorf("expected the alsa backend after pulse failed, got args %v", got)
	}
	start := time.Now()
	if err := rec.stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Errorf("graceful stop took %v — the q command wasn't honoured", time.Since(start))
	}
	data, _ := os.ReadFile(out)
	if len(data) != 8004 || string(data[8000:]) != "tail" {
		t.Errorf("output is %d bytes — ffmpeg should have been allowed to finish writing", len(data))
	}
}

func TestFFmpegRecordingAllBackendsFail(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil || runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	fake := filepath.Join(t.TempDir(), "ffmpeg")
	os.WriteFile(fake, []byte("#!/bin/sh\necho 'no such device' >&2\nexit 1\n"), 0o755)
	_, err := startFFmpegRecording(fake, filepath.Join(t.TempDir(), "x.wav"))
	if err == nil {
		t.Fatal("expected an error when every backend fails")
	}
	for _, want := range []string{"pulse:", "alsa:", "no such device"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func TestParseVulkanHeaderVersion(t *testing.T) {
	// Excerpt of the real Debian 12 vulkan_core.h.
	header := []byte("// Version of this file\n#define VK_HEADER_VERSION 239\n\n" +
		"// Complete version of this file\n#define VK_HEADER_VERSION_COMPLETE VK_MAKE_API_VERSION(0, 1, 3, VK_HEADER_VERSION)\n")
	v, ok := parseVulkanHeaderVersion(header)
	if !ok || v != (vkVersion{1, 3, 239}) {
		t.Fatalf("got %v (ok=%v), want 1.3.239", v, ok)
	}
	if !v.less(minVulkanHeaders) {
		t.Error("1.3.239 must be reported as too old")
	}
	for _, newer := range []vkVersion{{1, 3, 272}, {1, 3, 275}, {1, 4, 0}, {2, 0, 0}} {
		if newer.less(minVulkanHeaders) {
			t.Errorf("%v must be accepted", newer)
		}
	}
	if _, ok := parseVulkanHeaderVersion([]byte("nothing here")); ok {
		t.Error("garbage must not parse")
	}
}

func TestFFmpegRecordingNoAudioTimesOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	old := recordReadyTimeout
	recordReadyTimeout = 500 * time.Millisecond
	defer func() { recordReadyTimeout = old }()

	dir := t.TempDir()
	fake := filepath.Join(dir, "ffmpeg")
	// Starts fine but never delivers audio — like a device another program holds.
	os.WriteFile(fake, []byte("#!/bin/sh\nfor last; do :; done\n: > \"$last\"\nexec sleep 30\n"), 0o755)
	out := filepath.Join(dir, "rec.wav")

	began := time.Now()
	_, err := startFFmpegRecordingWith(fake, "alsa", "default", out)
	if err == nil || !strings.Contains(err.Error(), "no audio arrived") {
		t.Fatalf("got %v, want a no-audio timeout", err)
	}
	if took := time.Since(began); took > 3*time.Second {
		t.Errorf("timeout took %v — the stuck ffmpeg wasn't killed promptly", took)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Error("the empty capture file should have been removed")
	}
}
