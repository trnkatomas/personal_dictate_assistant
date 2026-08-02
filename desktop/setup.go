package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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

	// A cublas build needs an NVIDIA driver new enough for the CUDA runtime
	// it was built against — the CPU-variant scoring in pickBestAsset can't
	// verify that ahead of time, only whether an NVIDIA GPU is present at
	// all. This is the first point a real check is possible: an actual
	// whisper-cli run against the model we just confirmed is on disk. If it
	// fails, fall back to the plain CPU build now rather than let the user
	// discover it on their first real transcription.
	if readEngineVariant(binPath) == "cuda" {
		wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
			Label:   "Verifying GPU acceleration…",
			Percent: 100,
		})
		if err := smokeTestWhisper(ctx, binPath, modPath); err != nil {
			log.Printf("cublas build failed its startup check (%v) — falling back to the CPU build", err)
			wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
				Label:   "GPU build didn't start correctly — falling back to CPU build…",
				Percent: 0,
			})
			if fbErr := a.downloadBinaryFromGitHub(ctx, binPath, "none"); fbErr != nil {
				return fmt.Errorf("fall back to CPU build after failed GPU check: %w", fbErr)
			}
		}
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

// pickBestAsset finds the most suitable zip asset for the given platform and
// detected GPU. Returns the download URL and an "engine variant" tag
// ("cuda" | "cpu") that gets persisted alongside the binary (see
// writeEngineVariant) so later code can tell whether the installed build
// needs GPU acceleration to function.
//
// For macOS ARM64 it prefers Metal (GPU) over BLAS (CPU-only). Note macOS
// never actually reaches this function — see downloadBinary, which installs
// via Homebrew there instead — so the darwin cases below are effectively
// unreachable today but kept for whenever that changes.
func pickBestAsset(assets []ghAsset, goos, goarch, gpu string) (url, variant string) {
	type candidate struct {
		url     string
		variant string
		score   int
	}
	var best candidate

	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if !strings.HasSuffix(name, ".zip") && !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		score := 0
		v := "cpu"
		switch goos + "/" + goarch {
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
			// whisper.cpp's Linux releases don't ship a CUDA build at all —
			// only the CPU one — so gpu is irrelevant here for now.
			score = 1
		case "windows/amd64":
			if !strings.HasSuffix(name, ".zip") || !strings.Contains(name, "x64") {
				continue // skip Win32 (32-bit) and non-zip assets
			}
			switch {
			case strings.Contains(name, "cublas"):
				if gpu != "cuda" {
					// Multi-hundred-MB download only worth it when we've
					// actually detected an NVIDIA GPU to use it.
					continue
				}
				// Two CUDA-toolkit variants are published (e.g. 11.8 and
				// 12.4); prefer the older one for broader driver
				// compatibility — a newer driver still runs an older CUDA
				// runtime, but not the reverse.
				if strings.Contains(name, "11.8") {
					score, v = 4, "cuda"
				} else {
					score, v = 3, "cuda"
				}
			case strings.Contains(name, "blas"):
				score, v = 1, "cpu" // OpenBLAS build: faster, but +50MB dependency DLL
			default:
				score, v = 2, "cpu" // plain build: smaller, fewer ways to fail
			}
		default:
			continue
		}
		if score > best.score {
			best = candidate{a.BrowserDownloadURL, v, score}
		}
	}
	return best.url, best.variant
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
		return a.downloadBinaryFromGitHub(ctx, binPath, detectGPU())
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
// Used on Linux and Windows. macOS uses Homebrew instead. gpu is normally the
// result of detectGPU(), but the CUDA-smoke-test fallback in doDownload
// passes "none" explicitly to force the plain CPU build regardless of what
// hardware is actually present.
func (a *App) downloadBinaryFromGitHub(ctx context.Context, binPath string, gpu string) error {
	wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
		Label:   "Checking latest release…",
		Percent: 0,
	})
	rel, err := fetchLatestRelease(ctx)
	if err != nil {
		return err
	}

	archiveURL, variant := pickBestAsset(rel.Assets, runtime.GOOS, runtime.GOARCH, gpu)
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

	// Clear out whatever engine files are currently installed before
	// extracting — most importantly so falling back from a cublas build
	// (which bundles several hundred MB of CUDA runtime DLLs) to the plain
	// CPU build doesn't leave those unused libraries behind. A no-op on a
	// fresh install, since there's nothing there yet.
	binDir := filepath.Dir(binPath)
	if err := resetEngineBinDir(binDir); err != nil {
		return fmt.Errorf("clear existing engine files: %w", err)
	}

	if err := extractEngineArchive(archiveFile, binDir, filepath.Base(binPath)); err != nil {
		return fmt.Errorf("extract binary from archive: %w", err)
	}
	if err := os.Chmod(binPath, 0o755); err != nil {
		return err
	}
	if err := writeEngineVariant(binPath, variant); err != nil {
		// Non-fatal — worst case, the CUDA smoke-test check below defaults
		// to treating this as a plain CPU build and skips verification.
		log.Printf("could not persist engine variant marker: %v", err)
	}

	wailsRuntime.EventsEmit(a.ctx, "download:progress", DownloadProgress{
		Label:   "Whisper binary",
		Percent: 100,
	})
	return nil
}

// ── Engine variant tracking (for the CUDA smoke test / fallback below) ───────

// engineVariantMarkerPath returns the path to the small marker file recording
// which build (cuda-accelerated or plain CPU) is currently installed
// alongside the binary.
func engineVariantMarkerPath(binPath string) string {
	return filepath.Join(filepath.Dir(binPath), ".engine-variant")
}

func writeEngineVariant(binPath, variant string) error {
	return os.WriteFile(engineVariantMarkerPath(binPath), []byte(variant), 0o644)
}

// readEngineVariant returns "cuda" only if a marker file says so; any other
// case (no marker — e.g. a Homebrew install, or one predating this feature —
// read error, or an unrecognized value) defaults to "cpu", which is always
// the safe assumption: it just means the smoke test below is skipped.
func readEngineVariant(binPath string) string {
	data, err := os.ReadFile(engineVariantMarkerPath(binPath))
	if err != nil {
		return "cpu"
	}
	if v := strings.TrimSpace(string(data)); v == "cuda" {
		return v
	}
	return "cpu"
}

// resetEngineBinDir removes every file directly inside binDir (the currently
// installed binary and its libraries), leaving subdirectories — notably
// disabled-variants/, the CPU-backend quarantine dir managed elsewhere —
// alone. A missing binDir is not an error (nothing to clear on a fresh
// install).
func resetEngineBinDir(binDir string) error {
	entries, err := os.ReadDir(binDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(binDir, e.Name())); err != nil {
			return err
		}
	}
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

// ── CUDA smoke test ────────────────────────────────────────────────────────

// smokeTestWhisper runs the installed whisper-cli once against a tiny
// synthetic silent WAV to confirm it actually starts and produces output —
// the only reliable way to tell a cublas build will work on this machine,
// since ggml only initializes CUDA when a model is loaded, not at process
// startup. Bounded by a timeout well beyond what loading a model and
// transcribing half a second of silence should ever take, so a hung CUDA
// init can't block setup indefinitely.
func smokeTestWhisper(ctx context.Context, binPath, modPath string) error {
	tmpWav, err := os.CreateTemp("", "dictate-smoketest-*.wav")
	if err != nil {
		return err
	}
	tmpWav.Close()
	defer os.Remove(tmpWav.Name())
	if err := writeSilentWAV(tmpWav.Name(), 0.5, 16000); err != nil {
		return err
	}

	testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(testCtx, binPath,
		"-m", modPath,
		"-f", tmpWav.Name(),
		"--no-timestamps",
		"-np",
		"-l", "en",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// writeSilentWAV writes a minimal valid 16-bit PCM mono WAV file containing
// `seconds` of silence at the given sample rate — just enough for whisper-cli
// to load and run its backend init against, with no real audio needed.
func writeSilentWAV(path string, seconds float64, sampleRate int) error {
	numSamples := int(seconds * float64(sampleRate))
	dataSize := numSamples * 2 // 16-bit mono = 2 bytes/sample

	var buf bytes.Buffer
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16)) // fmt chunk size
	binary.Write(&buf, binary.LittleEndian, uint16(1))  // PCM
	binary.Write(&buf, binary.LittleEndian, uint16(1))  // mono
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate*2)) // byte rate
	binary.Write(&buf, binary.LittleEndian, uint16(2))            // block align
	binary.Write(&buf, binary.LittleEndian, uint16(16))           // bits/sample
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(dataSize))
	buf.Write(make([]byte, dataSize)) // silence

	return os.WriteFile(path, buf.Bytes(), 0o644)
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
