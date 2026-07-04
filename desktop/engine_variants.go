package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ggml's dynamic CPU dispatch scans the engine directory for ggml-cpu-*
// libraries and loads the highest-scoring one for the running processor.
// Variants are named after Intel µarch tiers (sandybridge=AVX,
// haswell=AVX2, skylakex=AVX-512, …) but selected purely by CPUID feature
// flags, so they run on AMD too. The catch: scoring is a CPUID check, not a
// test run — a variant can pass scoring yet still fault the moment its
// kernels execute (seen in the wild on some AMD parts, and with corrupted
// downloads). The failure signature is distinctive: whisper-cli prints
// "loaded CPU backend from …" and dies with no error text at all.
//
// When that happens we quarantine the offending library (rename it into a
// subdirectory the scanner can't see) and rerun, letting the scorer settle
// on the next-fastest variant that actually works. Because the quarantine
// is on disk, the fix persists across launches — the crash-retry dance
// happens at most once per bad variant, ever.

var cpuBackendLoadRe = regexp.MustCompile(`load_backend: loaded CPU backend from ([^\r\n]+)`)

// loadedCPUVariant returns the path of the CPU backend library whisper-cli
// reported loading, or "" if none was reported. Other backends (BLAS,
// Metal) log similar lines but never fault this way, so only the CPU line
// is matched.
func loadedCPUVariant(stderr string) string {
	ms := cpuBackendLoadRe.FindAllStringSubmatch(stderr, -1)
	if len(ms) == 0 {
		return ""
	}
	return strings.TrimSpace(ms[len(ms)-1][1])
}

// looksLikeBackendCrash reports whether stderr indicates whisper-cli died
// without producing a real diagnostic — the signature of an instruction
// fault inside a backend library. Legitimate failures (unreadable audio,
// missing model, …) always print an "error"/"failed" line, and those must
// not trigger a quarantine: they'd recur identically on retry while
// permanently discarding a healthy variant.
func looksLikeBackendCrash(stderr string) bool {
	l := strings.ToLower(stderr)
	return !strings.Contains(l, "error") && !strings.Contains(l, "failed")
}

// libBaseName extracts the final path element regardless of separator
// style — the path comes from whisper-cli's stderr, so on Windows it uses
// backslashes that filepath.Base would not split on other platforms.
func libBaseName(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		p = p[i+1:]
	}
	return p
}

// isBaselineCPUVariant reports whether the library is a lowest-common-
// denominator build that must never be quarantined — with those gone
// whisper-cli would have no CPU backend left at all.
func isBaselineCPUVariant(libPath string) bool {
	name := strings.ToLower(libBaseName(libPath))
	name = strings.TrimPrefix(name, "lib")
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return name == "ggml-cpu" || name == "ggml-cpu-x64"
}

func quarantineDir(binPath string) string {
	return filepath.Join(filepath.Dir(binPath), "disabled-variants")
}

// quarantineCPUVariant moves a crashing variant library out of the engine
// directory so ggml's scanner can't select it again. Refuses to touch
// libraries outside the managed directory (e.g. a Homebrew install — not
// ours to modify, and its single-variant build can't be "fallen back" from)
// and refuses to remove the baseline builds.
func quarantineCPUVariant(binPath, libPath string) error {
	binDir := filepath.Dir(binPath)
	if !strings.EqualFold(filepath.Dir(libPath), binDir) {
		return fmt.Errorf("CPU backend %s is not managed by this app", libPath)
	}
	if isBaselineCPUVariant(libPath) {
		return fmt.Errorf("refusing to quarantine baseline CPU backend %s", filepath.Base(libPath))
	}
	qDir := quarantineDir(binPath)
	if err := os.MkdirAll(qDir, 0o755); err != nil {
		return err
	}
	if err := os.Rename(libPath, filepath.Join(qDir, filepath.Base(libPath))); err != nil {
		return err
	}
	log.Printf("whisper-cli crashed after loading %s — quarantined it, retrying with the next CPU variant",
		filepath.Base(libPath))
	return nil
}

// restoreAllQuarantinedVariants moves every library sitting in the
// quarantine directory back into the engine directory — including ones
// quarantined in a previous, separate run (e.g. by an earlier version of
// this logic that quarantined the whole way down to the baseline before
// this safeguard existed). Used when a retry sequence still ends in failure
// even on the last-resort baseline variant: strong evidence the crash was
// never about a bad variant in the first place (a genuinely broken variant
// would let some *other* variant succeed), so there is no reason to leave
// healthy, faster builds permanently disabled for a problem they didn't
// cause — on this machine or from an earlier session.
func restoreAllQuarantinedVariants(binPath string) {
	qDir := quarantineDir(binPath)
	entries, err := os.ReadDir(qDir)
	if err != nil {
		return // nothing quarantined (or dir doesn't exist) — nothing to do
	}
	binDir := filepath.Dir(binPath)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if err := os.Rename(filepath.Join(qDir, name), filepath.Join(binDir, name)); err != nil {
			log.Printf("could not restore quarantined CPU backend %s: %v", name, err)
			continue
		}
		log.Printf("restored CPU backend %s — the earlier crash wasn't specific to it", name)
	}
}
