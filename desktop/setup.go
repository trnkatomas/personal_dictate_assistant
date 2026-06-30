package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v3/mem"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// SystemInfo contains detected hardware and locale information.
type SystemInfo struct {
	GPU      string `json:"gpu"`      // "apple_silicon" | "cuda" | "none"
	RAMGB    int    `json:"ramGB"`
	Locale   string `json:"locale"`   // e.g. "en", "cs", "de"
	Platform string `json:"platform"` // "darwin/arm64", "darwin/amd64", "linux/amd64"
}

// SetupState describes whether the Whisper engine is ready to use.
type SetupState struct {
	BinaryExists bool   `json:"binaryExists"`
	ModelExists  bool   `json:"modelExists"`
	ModelName    string `json:"modelName"`
	BinaryPath   string `json:"binaryPath"`
	ModelPath    string `json:"modelPath"`
}

// DownloadProgress is emitted as a Wails event during downloads.
type DownloadProgress struct {
	Label   string  `json:"label"`
	Percent float64 `json:"percent"`
	Done    bool    `json:"done"`
	Error   string  `json:"error,omitempty"`
}

// engineDir returns the base directory for engine files.
func engineDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, settingsAppDirName), nil
}

// binaryPath returns the expected path for the whisper-cli binary.
func binaryPath() (string, error) {
	dir, err := engineDir()
	if err != nil {
		return "", err
	}
	name := "whisper-cli"
	if runtime.GOOS == "windows" {
		name = "whisper-cli.exe"
	}
	return filepath.Join(dir, "bin", name), nil
}

// modelPath returns the expected path for a given GGML model file.
func modelPath(modelName string) (string, error) {
	dir, err := engineDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "models", "ggml-"+modelName+".bin"), nil
}

// resolveWhisperBin returns the whisper-cli binary path, checking our managed
// directory first and falling back to PATH (handles `brew install whisper-cpp`).
func resolveWhisperBin() (string, error) {
	managed, err := binaryPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(managed); err == nil {
		return managed, nil
	}
	if p, err := exec.LookPath("whisper-cli"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("whisper-cli not found")
}

// GetSystemInfo detects GPU type, RAM, locale, and platform.
func (a *App) GetSystemInfo() (SystemInfo, error) {
	info := SystemInfo{
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
		GPU:      detectGPU(),
		Locale:   detectLocale(),
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		info.RAMGB = int(vm.Total / (1024 * 1024 * 1024))
	}
	return info, nil
}

func detectGPU() string {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "apple_silicon"
		}
		return "none"
	case "windows":
		out, err := exec.Command("wmic", "path", "win32_VideoController", "get", "name").Output()
		if err == nil && strings.Contains(strings.ToUpper(string(out)), "NVIDIA") {
			return "cuda"
		}
		return "none"
	case "linux":
		if _, err := exec.LookPath("nvidia-smi"); err == nil {
			return "cuda"
		}
		if _, err := os.Stat("/proc/driver/nvidia"); err == nil {
			return "cuda"
		}
		return "none"
	default:
		return "none"
	}
}

func detectLocale() string {
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = os.Getenv("LANGUAGE")
	}
	if lang == "" {
		return "en"
	}
	lang = strings.Split(lang, "_")[0]
	lang = strings.Split(lang, ".")[0]
	if lang == "" || lang == "C" || lang == "POSIX" {
		return "en"
	}
	return lang
}

// GetSetupState checks whether the binary and model files are present on disk.
func (a *App) GetSetupState() (SetupState, error) {
	s, err := loadSettings()
	if err != nil {
		return SetupState{}, err
	}

	binPath, err := binaryPath()
	if err != nil {
		return SetupState{}, err
	}

	state := SetupState{
		ModelName:  s.ModelName,
		BinaryPath: binPath,
	}

	// Accept the binary from our managed path OR from the system PATH
	// (e.g. installed via `brew install whisper-cpp`).
	if resolved, err := resolveWhisperBin(); err == nil {
		state.BinaryExists = true
		state.BinaryPath = resolved
	}

	if s.ModelName != "" {
		modPath, err := modelPath(s.ModelName)
		if err != nil {
			return SetupState{}, err
		}
		state.ModelPath = modPath
		if _, err := os.Stat(modPath); err == nil {
			state.ModelExists = true
		}
	}

	return state, nil
}

// StartDownload starts downloading the binary (if missing) and the given model.
// Progress is reported via "download:progress" Wails events.
func (a *App) StartDownload(model string) error {
	var ctx context.Context
	ctx, a.downloadCancel = context.WithCancel(context.Background())
	go func() {
		if err := a.doDownload(ctx, model); err != nil {
			if ctx.Err() == nil {
				wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
					Label: "Error",
					Error: err.Error(),
					Done:  true,
				})
			}
		}
	}()
	return nil
}

// CancelDownload cancels an in-progress download.
func (a *App) CancelDownload() {
	if a.downloadCancel != nil {
		a.downloadCancel()
		a.downloadCancel = nil
	}
}

func (a *App) doDownload(ctx context.Context, model string) error {
	binPath, err := binaryPath()
	if err != nil {
		return err
	}

	// Install binary if not yet available (managed path or PATH).
	if _, err := resolveWhisperBin(); err != nil {
		if err := a.downloadBinary(ctx, binPath); err != nil {
			return fmt.Errorf("download binary: %w", err)
		}
	} else {
		wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
			Label:   "Whisper binary",
			Percent: 100,
		})
	}

	modPath, err := modelPath(model)
	if err != nil {
		return err
	}

	// Download model if missing.
	if _, err := os.Stat(modPath); os.IsNotExist(err) {
		if err := a.downloadModel(ctx, model, modPath); err != nil {
			return fmt.Errorf("download model: %w", err)
		}
	} else {
		wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
			Label:   "Model",
			Percent: 100,
		})
	}

	// Persist model name and mode to settings.
	if err := updateSettingsAfterDownload(model); err != nil {
		return err
	}

	wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
		Label: "Complete",
		Done:  true,
	})
	return nil
}

// ── Binary download (via GitHub Releases API) ────────────────────────────────

// ghRelease is the relevant subset of the GitHub releases API response.
type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// fetchLatestRelease queries the GitHub releases API for the latest whisper.cpp release.
func fetchLatestRelease(ctx context.Context) (ghRelease, error) {
	const apiURL = "https://api.github.com/repos/ggerganov/whisper.cpp/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return ghRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ghRelease{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ghRelease{}, fmt.Errorf("GitHub API returned HTTP %d — are you online?", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return ghRelease{}, fmt.Errorf("parse GitHub API response: %w", err)
	}
	return rel, nil
}

// pickBestAsset finds the most suitable zip asset for the current platform.
// For macOS ARM64 it prefers Metal (GPU) over BLAS (CPU-only).
func pickBestAsset(assets []ghAsset) (url, tag string) {
	type candidate struct {
		url   string
		score int
	}
	var best candidate

	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if !strings.HasSuffix(name, ".zip") && !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		score := 0
		switch runtime.GOOS + "/" + runtime.GOARCH {
		case "darwin/arm64":
			// Must contain "arm64" — skip anything else (x86, linux, etc.)
			if !strings.Contains(name, "arm64") {
				continue
			}
			// Skip CoreML-only builds; they need a different model format.
			if strings.Contains(name, "coreml") {
				continue
			}
			// Prefer Metal (uses Apple GPU), then any BLAS or vanilla build.
			if strings.Contains(name, "metal") {
				score = 3
			} else if strings.Contains(name, "blas") {
				score = 2
			} else {
				score = 1
			}
		case "darwin/amd64":
			if !strings.Contains(name, "x86") && !strings.Contains(name, "x64") {
				continue
			}
			if strings.Contains(name, "arm") {
				continue
			}
			if strings.Contains(name, "blas") {
				score = 2
			} else {
				score = 1
			}
		case "linux/amd64":
			if strings.Contains(name, "darwin") || strings.Contains(name, "windows") {
				continue
			}
			if !strings.Contains(name, "x64") && !strings.Contains(name, "x86") {
				continue
			}
			score = 1
		case "windows/amd64":
			if !strings.HasSuffix(name, ".zip") || !strings.Contains(name, "x64") {
				continue // skip Win32 (32-bit) and non-zip assets
			}
			if strings.Contains(name, "cublas") {
				// Needs a matching CUDA runtime installed system-wide; most
				// users won't have it, so the exe would fail to start.
				continue
			}
			if strings.Contains(name, "blas") {
				score = 1 // OpenBLAS build: faster, but +50MB dependency DLL
			} else {
				score = 2 // plain build: smaller, fewer ways to fail
			}
		default:
			continue
		}
		if score > best.score {
			best = candidate{a.BrowserDownloadURL, score}
		}
	}
	return best.url, ""
}

func (a *App) downloadBinary(ctx context.Context, binPath string) error {
	wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
		Label:   "Whisper binary",
		Percent: 0,
	})

	switch runtime.GOOS {
	case "darwin":
		// The whisper.cpp GitHub releases only ship Windows CLI binaries.
		// On macOS the standard path is `brew install whisper-cpp`.
		return a.installViaHomebrew(ctx)
	default:
		return a.downloadBinaryFromGitHub(ctx, binPath)
	}
}

// installViaHomebrew runs `brew install whisper-cpp` to get whisper-cli on macOS.
func (a *App) installViaHomebrew(ctx context.Context) error {
	brewPath, err := exec.LookPath("brew")
	if err != nil {
		return fmt.Errorf(
			"Homebrew not found.\n\n" +
				"Install whisper-cpp with:\n" +
				"  brew install whisper-cpp\n\n" +
				"Get Homebrew at https://brew.sh",
		)
	}

	wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
		Label:   "Installing via Homebrew — this may take a few minutes…",
		Percent: 10,
	})

	cmd := exec.CommandContext(ctx, brewPath, "install", "whisper-cpp", "ffmpeg")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("brew install failed:\n%s", strings.TrimSpace(string(out)))
	}

	wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
		Label:   "Whisper binary",
		Percent: 100,
	})
	return nil
}

// downloadBinaryFromGitHub downloads the whisper-cli zip from the latest GitHub release.
// Used on Linux (and Windows in future). macOS uses Homebrew instead.
func (a *App) downloadBinaryFromGitHub(ctx context.Context, binPath string) error {
	wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
		Label:   "Checking latest release…",
		Percent: 0,
	})
	rel, err := fetchLatestRelease(ctx)
	if err != nil {
		return err
	}

	archiveURL, _ := pickBestAsset(rel.Assets)
	if archiveURL == "" {
		var names []string
		for _, a := range rel.Assets {
			lower := strings.ToLower(a.Name)
			if strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar.gz") {
				names = append(names, a.Name)
			}
		}
		return fmt.Errorf("no pre-built binary for %s/%s in whisper.cpp %s\navailable assets: %s",
			runtime.GOOS, runtime.GOARCH, rel.TagName, strings.Join(names, ", "))
	}

	tmpArchive, err := downloadToTemp(ctx, archiveURL, func(pct float64) {
		wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
			Label:   "Whisper binary (" + rel.TagName + ")",
			Percent: pct * 0.9,
		})
	})
	if err != nil {
		return err
	}

	// downloadToTemp's filename carries no extension; extractEngineArchive
	// switches on it to pick zip vs. tar.gz, so restore it from the source URL.
	archiveFile := tmpArchive
	if strings.HasSuffix(strings.ToLower(archiveURL), ".tar.gz") {
		archiveFile += ".tar.gz"
	} else {
		archiveFile += ".zip"
	}
	if err := os.Rename(tmpArchive, archiveFile); err != nil {
		os.Remove(tmpArchive)
		return err
	}
	defer os.Remove(archiveFile)

	if err := extractEngineArchive(archiveFile, filepath.Dir(binPath), filepath.Base(binPath)); err != nil {
		return fmt.Errorf("extract binary from archive: %w", err)
	}
	if err := os.Chmod(binPath, 0o755); err != nil {
		return err
	}

	wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
		Label:   "Whisper binary",
		Percent: 100,
	})
	return nil
}

// ── Model download ────────────────────────────────────────────────────────────

func (a *App) downloadModel(ctx context.Context, model, modPath string) error {
	u := "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-" + model + ".bin"
	wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
		Label:   "Model",
		Percent: 0,
	})

	if err := os.MkdirAll(filepath.Dir(modPath), 0o755); err != nil {
		return err
	}

	tmp, err := downloadToTemp(ctx, u, func(pct float64) {
		wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
			Label:   "Model (" + model + ")",
			Percent: pct,
		})
	})
	if err != nil {
		return err
	}

	if err := os.Rename(tmp, modPath); err != nil {
		if err2 := copyFile(tmp, modPath); err2 != nil {
			os.Remove(tmp)
			return err2
		}
		os.Remove(tmp)
	}

	wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
		Label:   "Model",
		Percent: 100,
	})
	return nil
}

// ── Shared download utilities ─────────────────────────────────────────────────

func downloadToTemp(ctx context.Context, u string, progress func(float64)) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, u)
	}

	tmp, err := os.CreateTemp("", "dictate-dl-*")
	if err != nil {
		return "", err
	}

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024)
	for {
		if ctx.Err() != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return "", ctx.Err()
		}
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := tmp.Write(buf[:n]); writeErr != nil {
				tmp.Close()
				os.Remove(tmp.Name())
				return "", writeErr
			}
			downloaded += int64(n)
			if total > 0 && progress != nil {
				progress(float64(downloaded) / float64(total) * 100)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return "", readErr
		}
	}
	tmp.Close()
	return tmp.Name(), nil
}

// extractEngineArchive extracts the whisper-cli binary (binName, e.g.
// "whisper-cli" or "whisper-cli.exe") plus every shared library (.dll/.so*/
// .dylib) from the archive into destDir, flattening whatever subdirectory
// they're nested in. whisper.cpp's prebuilt Windows/Linux releases ship the
// binary alongside several required runtime libraries (GGML CPU-dispatch
// variants, the core ggml/whisper libs) in the same directory — extracting
// only the binary leaves it unable to start. Unrelated bundled tools
// (bench, server, test binaries, …) are skipped to keep the install small.
func extractEngineArchive(archivePath, destDir, binName string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		return extractZipEngine(archivePath, destDir, binName)
	case strings.HasSuffix(archivePath, ".tar.gz"):
		return extractTarGzEngine(archivePath, destDir, binName)
	default:
		return fmt.Errorf("unsupported archive format: %s", archivePath)
	}
}

// isEngineLibrary reports whether a filename looks like a shared library
// whisper-cli needs at runtime (.dll, .so / .so.N / .so.N.N, .dylib).
func isEngineLibrary(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".dll") ||
		strings.Contains(lower, ".so") ||
		strings.HasSuffix(lower, ".dylib")
}

func extractZipEngine(zipPath, destDir, binName string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	found := false
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		if base != binName && !isEngineLibrary(base) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeExtractedFile(filepath.Join(destDir, base), rc)
		rc.Close()
		if err != nil {
			return err
		}
		if base == binName {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("%s not found in archive", binName)
	}
	return nil
}

func extractTarGzEngine(tarGzPath, destDir, binName string) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if base != binName && !isEngineLibrary(base) {
			continue
		}
		if err := writeExtractedFile(filepath.Join(destDir, base), tr); err != nil {
			return err
		}
		if base == binName {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("%s not found in archive", binName)
	}
	return nil
}

func writeExtractedFile(destPath string, src io.Reader) error {
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, src)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func updateSettingsAfterDownload(model string) error {
	s, err := loadSettings()
	if err != nil {
		return err
	}
	s.Mode = "integrated"
	s.ModelName = model
	return saveSettings(s)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
