package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Building whisper-cli from source with Vulkan GPU acceleration (Linux only).
//
// whisper.cpp publishes no GPU-accelerated Linux binary — its Linux release
// is CPU-only — but the source builds with Vulkan support, which covers AMD,
// Intel and NVIDIA GPUs alike. The setup wizard offers this as an opt-in
// alternative to the prebuilt download: fetch the source of the current
// release, compile just the whisper-cli target as one static binary with
// -DGGML_VULKAN=ON, and install it where the prebuilt engine would go.

// engineVulkan is the engine-variant marker (see writeEngineVariant) for a
// source build with Vulkan support.
const engineVulkan = "vulkan"

// VulkanBuildInfo tells the wizard whether a Vulkan source build can be
// attempted on this machine, and if not, what's missing.
type VulkanBuildInfo struct {
	Supported   bool     `json:"supported"`   // false everywhere but Linux
	Ready       bool     `json:"ready"`       // every build prerequisite found
	Missing     []string `json:"missing"`     // human-readable names of what wasn't
	InstallHint string   `json:"installHint"` // a command likely to install them
	Device      string   `json:"device"`      // GPU name from vulkaninfo, "" if unknown
	// SoftwareOnly: vulkaninfo found Vulkan devices, but only CPU-type ones
	// (a software renderer such as llvmpipe). ggml skips those, so a build
	// would run on the processor — usually a missing GPU driver.
	SoftwareOnly bool `json:"softwareOnly"`
}

// GetVulkanBuildInfo checks for the tools a Vulkan source build needs.
func (a *App) GetVulkanBuildInfo() VulkanBuildInfo {
	if runtime.GOOS != "linux" {
		return VulkanBuildInfo{}
	}
	missing, installable := missingVulkanBuildTools()
	info := VulkanBuildInfo{Supported: true, Missing: missing}
	info.Device, info.SoftwareOnly = detectVulkanGPU()
	info.Ready = len(missing) == 0
	if installable {
		// Only when installing packages would actually help — headers that
		// are present but too old can't be fixed by the same command.
		info.InstallHint = vulkanBuildInstallHint()
	}
	return info
}

func haveAnyTool(names ...string) bool {
	for _, n := range names {
		if _, err := exec.LookPath(n); err == nil {
			return true
		}
	}
	return false
}

// missingVulkanBuildTools lists the build prerequisites that can't be found.
// installable reports whether any of them is a package to install, as
// opposed to only the Vulkan headers being present but too old.
func missingVulkanBuildTools() (missing []string, installable bool) {
	if !haveAnyTool("cmake") {
		missing = append(missing, "cmake")
	}
	if !haveAnyTool("make", "ninja") {
		missing = append(missing, "make")
	}
	if !haveAnyTool("c++", "g++", "clang++") {
		missing = append(missing, "a C++ compiler (g++)")
	}
	if !haveAnyTool("glslc") {
		missing = append(missing, "glslc (Vulkan shader compiler)")
	}
	if !haveVulkanHeaders() {
		missing = append(missing, "Vulkan development headers")
	} else if v, ok := installedVulkanHeaderVersion(); ok && v.less(minVulkanHeaders) {
		missing = append(missing, fmt.Sprintf("Vulkan headers %s or newer (found %s — "+
			"your distribution's package is too old; Ubuntu 24.04+ and Debian 13+ ship new enough ones)",
			minVulkanHeaders, v))
	}
	if !haveSPIRVHeaders() {
		missing = append(missing, "SPIRV-Headers")
	}
	if !haveAnyTool("tar") {
		missing = append(missing, "tar")
	}
	for _, m := range missing {
		if !strings.HasPrefix(m, "Vulkan headers ") {
			installable = true
		}
	}
	return missing, installable
}

func haveVulkanHeaders() bool {
	if haveAnyTool("pkg-config") && exec.Command("pkg-config", "--exists", "vulkan").Run() == nil {
		return true
	}
	for _, p := range []string{"/usr/include/vulkan/vulkan.h", "/usr/local/include/vulkan/vulkan.h"} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// vkVersion is a Vulkan header version, e.g. 1.3.272.
type vkVersion struct{ major, minor, patch int }

func (v vkVersion) String() string { return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch) }

func (v vkVersion) less(o vkVersion) bool {
	if v.major != o.major {
		return v.major < o.major
	}
	if v.minor != o.minor {
		return v.minor < o.minor
	}
	return v.patch < o.patch
}

// minVulkanHeaders is the oldest Vulkan-Headers release that defines
// VK_EXT_layer_settings, which ggml's Vulkan backend uses unconditionally
// (verified against whisper.cpp v1.9.5 / ggml 0.26). Older headers fail
// several minutes into the compile, so it's checked up front instead.
var minVulkanHeaders = vkVersion{1, 3, 272}

var (
	vkHeaderPatchRe    = regexp.MustCompile(`(?m)^#define\s+VK_HEADER_VERSION\s+(\d+)\s*$`)
	vkHeaderCompleteRe = regexp.MustCompile(`(?m)^#define\s+VK_HEADER_VERSION_COMPLETE\s+VK_MAKE_API_VERSION\(\s*\d+\s*,\s*(\d+)\s*,\s*(\d+)\s*,\s*VK_HEADER_VERSION\s*\)`)
)

// parseVulkanHeaderVersion reads the version out of vulkan_core.h's text.
func parseVulkanHeaderVersion(header []byte) (vkVersion, bool) {
	p := vkHeaderPatchRe.FindSubmatch(header)
	c := vkHeaderCompleteRe.FindSubmatch(header)
	if p == nil || c == nil {
		return vkVersion{}, false
	}
	major, _ := strconv.Atoi(string(c[1]))
	minor, _ := strconv.Atoi(string(c[2]))
	patch, _ := strconv.Atoi(string(p[1]))
	return vkVersion{major, minor, patch}, true
}

// installedVulkanHeaderVersion finds the system's vulkan_core.h and reports
// its version; ok is false if it can't be found or read (the build then just
// proceeds and reports its own error).
func installedVulkanHeaderVersion() (vkVersion, bool) {
	dirs := []string{"/usr/include", "/usr/local/include"}
	if haveAnyTool("pkg-config") {
		if out, err := exec.Command("pkg-config", "--variable=includedir", "vulkan").Output(); err == nil {
			if d := strings.TrimSpace(string(out)); d != "" {
				dirs = append([]string{d}, dirs...)
			}
		}
	}
	for _, d := range dirs {
		data, err := os.ReadFile(filepath.Join(d, "vulkan", "vulkan_core.h"))
		if err != nil {
			continue
		}
		if v, ok := parseVulkanHeaderVersion(data); ok {
			return v, true
		}
	}
	return vkVersion{}, false
}

// haveSPIRVHeaders reports whether CMake will be able to find SPIRV-Headers,
// which ggml's Vulkan backend requires (find_package(SPIRV-Headers)) since
// ggml 0.26. Distributions ship it as a separate package.
func haveSPIRVHeaders() bool {
	if haveAnyTool("pkg-config") && exec.Command("pkg-config", "--exists", "SPIRV-Headers").Run() == nil {
		return true
	}
	for _, pattern := range []string{
		"/usr/share/cmake*/SPIRV-Headers/SPIRV-HeadersConfig.cmake",
		"/usr/lib/cmake/SPIRV-Headers/SPIRV-HeadersConfig.cmake",
		"/usr/lib*/cmake/SPIRV-Headers/SPIRV-HeadersConfig.cmake",
		"/usr/lib/*/cmake/SPIRV-Headers/SPIRV-HeadersConfig.cmake",
		"/usr/local/share/cmake*/SPIRV-Headers/SPIRV-HeadersConfig.cmake",
		"/usr/local/lib*/cmake/SPIRV-Headers/SPIRV-HeadersConfig.cmake",
	} {
		if m, _ := filepath.Glob(pattern); len(m) > 0 {
			return true
		}
	}
	return false
}

// vulkanBuildInstallHint suggests the install command for whichever common
// package manager is present. Package names differ per distribution, so this
// is a best guess to start from, not a guarantee.
func vulkanBuildInstallHint() string {
	switch {
	case haveAnyTool("apt-get"):
		return "sudo apt install build-essential cmake libvulkan-dev glslc spirv-headers"
	case haveAnyTool("dnf"):
		return "sudo dnf install gcc-c++ make cmake vulkan-headers vulkan-loader-devel glslc spirv-headers-devel"
	case haveAnyTool("pacman"):
		return "sudo pacman -S base-devel cmake vulkan-headers vulkan-icd-loader shaderc spirv-headers"
	case haveAnyTool("zypper"):
		return "sudo zypper install gcc-c++ make cmake vulkan-devel shaderc spirv-headers"
	}
	return ""
}

// parseVulkanDevices reads `vulkaninfo --summary` output and returns the
// first device that isn't a CPU-type (software) device, plus whether CPU-type
// devices were the only ones listed. ggml's Vulkan backend ignores CPU-type
// devices unless forced, so naming one as "the GPU" would mislead.
func parseVulkanDevices(out []byte) (gpu string, softwareOnly bool) {
	var name, typ string
	sawCPU := false
	flush := func() bool {
		if name == "" {
			return false
		}
		if typ == "PHYSICAL_DEVICE_TYPE_CPU" {
			sawCPU = true
		} else {
			gpu = name
			return true
		}
		name, typ = "", ""
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "GPU") && strings.HasSuffix(trimmed, ":") {
			if flush() {
				return gpu, false
			}
			name, typ = "", ""
			continue
		}
		k, v, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "deviceName":
			name = strings.TrimSpace(v)
		case "deviceType":
			typ = strings.TrimSpace(v)
		}
	}
	if flush() {
		return gpu, false
	}
	return "", sawCPU
}

// detectVulkanGPU runs vulkaninfo, if installed. Purely informational: the
// result is shown in the wizard, never used to decide anything.
func detectVulkanGPU() (gpu string, softwareOnly bool) {
	if !haveAnyTool("vulkaninfo") {
		return "", false
	}
	out, err := exec.Command("vulkaninfo", "--summary").Output()
	if err != nil {
		return "", false
	}
	return parseVulkanDevices(out)
}

// sourceReleaseTag picks which whisper.cpp tag to build: the newest
// versioned release (every tag has a source tarball, whether or not it has
// binaries attached), or failing that the newest release of any kind.
func sourceReleaseTag(rels []ghRelease) string {
	for _, r := range rels {
		if !r.Draft && !r.Prerelease {
			return r.TagName
		}
	}
	for _, r := range rels {
		if !r.Draft {
			return r.TagName
		}
	}
	return ""
}

// cmakeProgressRe matches the "[ 42%] Building CXX object …" lines the
// Makefile generator prints; ninjaProgressRe the "[17/120] …" ones.
var (
	cmakeProgressRe = regexp.MustCompile(`^\[\s*(\d+)%\]`)
	ninjaProgressRe = regexp.MustCompile(`^\[(\d+)/(\d+)\]`)
)

// buildProgressPercent extracts 0–100 from one line of build output.
func buildProgressPercent(line string) (float64, bool) {
	if m := cmakeProgressRe.FindStringSubmatch(line); m != nil {
		p, _ := strconv.Atoi(m[1])
		return float64(p), true
	}
	if m := ninjaProgressRe.FindStringSubmatch(line); m != nil {
		done, _ := strconv.Atoi(m[1])
		total, _ := strconv.Atoi(m[2])
		if total > 0 {
			return float64(done) / float64(total) * 100, true
		}
	}
	return 0, false
}

// runBuildStep runs one command of the build, feeding each output line to
// onLine and returning the last lines of output as part of any error —
// a compiler error is only useful if the user can see it.
func runBuildStep(ctx context.Context, dir string, onLine func(string), name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		return err
	}

	const keepLines = 25
	var tail []string
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			tail = append(tail, line)
			if len(tail) > keepLines {
				tail = tail[1:]
			}
			if onLine != nil {
				onLine(line)
			}
		}
		io.Copy(io.Discard, pr) // keep draining if a line overflowed the scanner
	}()

	err := cmd.Wait()
	pw.Close()
	<-scanDone
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, strings.Join(tail, "\n"))
	}
	return nil
}

// buildVulkanEngine compiles whisper-cli with Vulkan support from srcDir (an
// unpacked whisper.cpp source tree) and returns the path of the binary.
// progress receives 0–100 as the compile advances.
func buildVulkanEngine(ctx context.Context, srcDir string, progress func(float64)) (string, error) {
	bin, err := buildVulkanEngineWith(ctx, srcDir, true, progress)
	if err == nil || ctx.Err() != nil {
		return bin, err
	}
	// The first attempt tunes ggml's CPU code for this exact processor
	// (GGML_NATIVE). That's the fragile half of the build — it depends on
	// what the compiler makes of the detected CPU features, and has been
	// seen to fail on some toolchains — while the GPU (Vulkan) code is the
	// same either way. Retry once with portable CPU code before giving up.
	log.Printf("vulkan build: CPU-tuned build failed, retrying with portable CPU code: %v", err)
	bin, retryErr := buildVulkanEngineWith(ctx, srcDir, false, progress)
	if retryErr != nil {
		return "", retryErr
	}
	return bin, nil
}

// buildVulkanEngineWith runs one configure + compile in a fresh build
// directory; native selects CPU code tuned for this machine (GGML_NATIVE).
func buildVulkanEngineWith(ctx context.Context, srcDir string, native bool, progress func(float64)) (string, error) {
	buildDir := filepath.Join(srcDir, "build")
	if err := os.RemoveAll(buildDir); err != nil {
		return "", err
	}
	nativeFlag := "-DGGML_NATIVE=ON"
	if !native {
		nativeFlag = "-DGGML_NATIVE=OFF"
	}
	configure := []string{
		"-S", srcDir, "-B", buildDir,
		"-DCMAKE_BUILD_TYPE=Release",
		"-DGGML_VULKAN=ON",
		nativeFlag,
		// One self-contained binary: nothing to keep in sync beside it, and
		// no CPU-variant libraries for engine_variants.go to manage.
		"-DBUILD_SHARED_LIBS=OFF",
		"-DWHISPER_BUILD_TESTS=OFF",
		"-DWHISPER_BUILD_SERVER=OFF",
		"-DWHISPER_SDL2=OFF",
	}
	if err := runBuildStep(ctx, srcDir, nil, "cmake", configure...); err != nil {
		return "", fmt.Errorf("configure: %w", err)
	}

	jobs := runtime.NumCPU()
	onLine := func(line string) {
		if p, ok := buildProgressPercent(line); ok && progress != nil {
			progress(p)
		}
	}
	if err := runBuildStep(ctx, srcDir, onLine, "cmake",
		"--build", buildDir, "--target", "whisper-cli", "-j", strconv.Itoa(jobs)); err != nil {
		return "", fmt.Errorf("compile: %w", err)
	}

	bin := filepath.Join(buildDir, "bin", "whisper-cli")
	if _, err := os.Stat(bin); err != nil {
		return "", fmt.Errorf("build finished but %s is missing", bin)
	}
	return bin, nil
}

// buildAndInstallVulkanEngine downloads the current whisper.cpp source,
// builds it with Vulkan support, and installs the result as the app's
// engine at binPath, replacing whatever was there.
func (a *App) buildAndInstallVulkanEngine(ctx context.Context, binPath string) error {
	emit := func(label string, pct float64) {
		wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{Label: label, Percent: pct})
	}

	if missing, _ := missingVulkanBuildTools(); len(missing) > 0 {
		return fmt.Errorf("cannot build with Vulkan support — missing: %s", strings.Join(missing, ", "))
	}

	emit("Checking latest release…", 0)
	rels, err := fetchReleases(ctx)
	if err != nil {
		return err
	}
	tag := sourceReleaseTag(rels)
	if tag == "" {
		return fmt.Errorf("no whisper.cpp release found to build from")
	}

	srcURL := "https://github.com/ggml-org/whisper.cpp/archive/refs/tags/" + tag + ".tar.gz"
	archive, err := downloadToTemp(ctx, srcURL, func(pct float64) {
		emit("Whisper source ("+tag+")", pct*0.05)
	})
	if err != nil {
		return fmt.Errorf("download source: %w", err)
	}
	defer os.Remove(archive)

	workDir, err := os.MkdirTemp("", "dictate-vulkan-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	// GitHub's source tarballs wrap everything in one top-level directory.
	if err := runBuildStep(ctx, workDir, nil, "tar", "-xzf", archive, "-C", workDir, "--strip-components=1"); err != nil {
		return fmt.Errorf("unpack source: %w", err)
	}

	label := "Compiling with Vulkan support (" + tag + ") — this takes a few minutes…"
	emit(label, 5)
	built, err := buildVulkanEngine(ctx, workDir, func(pct float64) {
		emit(label, 5+pct*0.9)
	})
	if err != nil {
		return err
	}

	binDir := filepath.Dir(binPath)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	if err := resetEngineBinDir(binDir); err != nil {
		return fmt.Errorf("clear existing engine files: %w", err)
	}
	if err := copyFile(built, binPath); err != nil {
		return fmt.Errorf("install built engine: %w", err)
	}
	if err := os.Chmod(binPath, 0o755); err != nil {
		return err
	}
	if err := writeEngineVariant(binPath, engineVulkan); err != nil {
		log.Printf("could not persist engine variant marker: %v", err)
	}
	log.Printf("installed whisper-cli %s built from source with Vulkan support at %s", tag, binPath)
	emit("Whisper binary", 100)
	return nil
}
