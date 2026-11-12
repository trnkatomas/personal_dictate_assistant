package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Native microphone capture, used on Linux only.
//
// Everywhere else the frontend records through the webview's getUserMedia +
// MediaRecorder. On Linux that can't work: Wails v2's WebKitGTK frontend
// never answers WebKit's "permission-request" signal, and WebKitGTK denies
// any media request nobody answers — so getUserMedia is rejected no matter
// what the system's audio permissions say. Rather than depend on the webview
// for audio at all, Linux records with ffmpeg, which the app already requires
// there for format conversion. The capture is written straight to the
// 16 kHz mono WAV whisper-cli wants.

// ffmpegRecordInputs are the capture backends tried in order. "pulse" covers
// PulseAudio and PipeWire (through pipewire-pulse); "alsa" is the fallback
// for systems with neither, or an ffmpeg built without PulseAudio support.
var ffmpegRecordInputs = [][2]string{{"pulse", "default"}, {"alsa", "default"}}

// recordReadyTimeout bounds how long StartRecording waits for the first
// captured audio to reach disk. Capture is only reported as started once it
// has — a fixed delay would show "Recording" while ffmpeg is still connecting
// to the sound server (measured at ~2 s on one setup), silently losing
// whatever the user says first.
var recordReadyTimeout = 8 * time.Second // var so tests can shorten it

// recordPollInterval is how often the capture file is checked while waiting.
const recordPollInterval = 50 * time.Millisecond

// minCapturedBytes is the file size taken to mean "audio is arriving": past
// any WAV header (whose length varies with ffmpeg's version and metadata)
// plus a little audio — 16 kHz mono 16-bit is 32 000 bytes per second, so
// this is ~0.1 s.
const minCapturedBytes = 4096

// recordStopTimeout bounds each stage of stopping ffmpeg (asking nicely,
// then interrupting) before the next, harsher one.
const recordStopTimeout = 3 * time.Second

// ffmpegRecording is one in-flight capture.
type ffmpegRecording struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr *bytes.Buffer
	done   chan error // receives cmd.Wait()'s result exactly once
	path   string     // the WAV being written
}

// startFFmpegRecording starts capturing the default microphone into outPath,
// trying each backend in ffmpegRecordInputs until one keeps running.
func startFFmpegRecording(ffmpegPath, outPath string) (*ffmpegRecording, error) {
	var failures []string
	for _, in := range ffmpegRecordInputs {
		rec, err := startFFmpegRecordingWith(ffmpegPath, in[0], in[1], outPath)
		if err == nil {
			log.Printf("recording: capturing via ffmpeg -f %s -i %s -> %s", in[0], in[1], outPath)
			return rec, nil
		}
		log.Printf("recording: ffmpeg -f %s failed to start: %v", in[0], err)
		failures = append(failures, fmt.Sprintf("%s: %v", in[0], err))
	}
	return nil, fmt.Errorf("could not start recording from the microphone\n%s", strings.Join(failures, "\n"))
}

func startFFmpegRecordingWith(ffmpegPath, format, device, outPath string) (*ffmpegRecording, error) {
	cmd := exec.Command(ffmpegPath,
		"-hide_banner", "-loglevel", "error",
		"-f", format, "-i", device,
		"-ac", "1", "-ar", "16000",
		// Write each packet as it arrives instead of in 32 KB blocks, so the
		// file's size shows when capture has really begun (see waitForAudio).
		"-flush_packets", "1",
		"-y", outPath,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	rec := &ffmpegRecording{cmd: cmd, stdin: stdin, stderr: stderr, done: make(chan error, 1), path: outPath}
	go func() { rec.done <- cmd.Wait() }()

	if err := rec.waitForAudio(recordReadyTimeout); err != nil {
		return nil, err
	}
	return rec, nil
}

// waitForAudio blocks until captured audio is reaching the output file. A bad
// input (unknown format, no such device, no sound server) makes ffmpeg exit,
// which is reported with ffmpeg's own message; a device that never delivers
// anything (e.g. held exclusively by another program) times out.
func (r *ffmpegRecording) waitForAudio(timeout time.Duration) error {
	deadline := time.After(timeout)
	tick := time.NewTicker(recordPollInterval)
	defer tick.Stop()
	for {
		select {
		case <-r.done:
			msg := strings.TrimSpace(r.stderr.String())
			if msg == "" {
				msg = "ffmpeg exited before capturing any audio"
			}
			return fmt.Errorf("%s", msg)
		case <-deadline:
			r.cmd.Process.Kill()
			<-r.done
			os.Remove(r.path)
			return fmt.Errorf("no audio arrived from the microphone within %v", timeout)
		case <-tick.C:
			if fi, err := os.Stat(r.path); err == nil && fi.Size() >= minCapturedBytes {
				return nil
			}
		}
	}
}

// stop ends the capture and waits for ffmpeg to finalize the WAV header.
// ffmpeg is asked to quit the way a user at a terminal would ("q"), then
// interrupted, then killed — the first two both leave a valid file.
func (r *ffmpegRecording) stop() error {
	io.WriteString(r.stdin, "q\n")
	r.stdin.Close()

	select {
	case <-r.done:
		return nil
	case <-time.After(recordStopTimeout):
	}
	r.cmd.Process.Signal(os.Interrupt)
	select {
	case <-r.done:
		return nil
	case <-time.After(recordStopTimeout):
	}
	r.cmd.Process.Kill()
	<-r.done
	return fmt.Errorf("ffmpeg did not stop cleanly: %s", strings.TrimSpace(r.stderr.String()))
}

// abort kills the capture and discards the file.
func (r *ffmpegRecording) abort() {
	r.cmd.Process.Kill()
	<-r.done
	os.Remove(r.path)
}

// nativeRecorder holds the app's single in-flight recording, if any.
type nativeRecorder struct {
	mu  sync.Mutex
	rec *ffmpegRecording
}

// UsesNativeRecorder tells the frontend whether to record through
// StartRecording/StopRecording instead of the webview's MediaRecorder.
func (a *App) UsesNativeRecorder() bool {
	return runtime.GOOS == "linux"
}

// StartRecording begins capturing the default microphone (Linux only).
func (a *App) StartRecording() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("native recording is only used on Linux")
	}
	a.recorder.mu.Lock()
	defer a.recorder.mu.Unlock()
	if a.recorder.rec != nil {
		return fmt.Errorf("a recording is already in progress")
	}

	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("%s\n(Required to record from the microphone.)", ffmpegInstallHint())
	}
	tmp, err := os.CreateTemp("", "dictate-rec-*.wav")
	if err != nil {
		return fmt.Errorf("create temp recording: %w", err)
	}
	tmp.Close()

	rec, err := startFFmpegRecording(ffmpegPath, tmp.Name())
	if err != nil {
		os.Remove(tmp.Name())
		return err
	}
	a.recorder.rec = rec
	return nil
}

// StopRecording ends the capture started by StartRecording and transcribes
// it with the configured backend, exactly as a webview recording would be.
func (a *App) StopRecording(s Settings) (string, error) {
	a.recorder.mu.Lock()
	rec := a.recorder.rec
	a.recorder.rec = nil
	a.recorder.mu.Unlock()
	if rec == nil {
		return "", fmt.Errorf("no recording in progress")
	}
	defer os.Remove(rec.path)

	if err := rec.stop(); err != nil {
		return "", err
	}
	audio, err := os.ReadFile(rec.path)
	if err != nil {
		return "", fmt.Errorf("read recording: %w", err)
	}
	if len(audio) < minCapturedBytes {
		detail := strings.TrimSpace(rec.stderr.String())
		if detail != "" {
			detail = ": " + detail
		}
		return "", fmt.Errorf("the recording is empty — no audio was captured from the microphone%s", detail)
	}
	// transcribeWhisper keeps its own copy for failed-clips on error, so the
	// capture file itself can always be removed.
	return transcribeWhisper(audio, "audio/wav", s)
}

// CancelRecording discards an in-flight recording, if there is one.
func (a *App) CancelRecording() {
	a.recorder.mu.Lock()
	rec := a.recorder.rec
	a.recorder.rec = nil
	a.recorder.mu.Unlock()
	if rec != nil {
		rec.abort()
	}
}
