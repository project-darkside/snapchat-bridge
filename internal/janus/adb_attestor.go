package janus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AdbAttestor implements Attestor by driving an on-device app_process64 helper
// over adb. The helper loads the installed Snapchat libscplugin.so, calls
// yblj.prh.a(afx), and returns the native attestation blob. Because adb shell
// does not reliably forward binary stdin to app_process64, the request and
// response are staged as files on the device rather than piped through
// stdin/stdout.
//
// The proven device layout is:
//
//	RemoteDir = "/data/local/tmp/bridge-snapchat-probe-20261022"
//	BaseAPK   = the installed com.snapchat.android base.apk (from `adb shell pm
//	            path com.snapchat.android`)
//	OracleDex = RemoteDir + "/attestation-oracle.zip" (dexed AttestationOracle)
//	LibPath   = RemoteDir + "/libscplugin.so" (extracted from the installed APK)
type AdbAttestor struct {
	// ADB is the adb executable path; defaults to "adb".
	ADB string
	// Serial selects a device via `adb -s`; empty for the single attached
	// device.
	Serial string
	// RemoteDir is the device staging directory. Required.
	RemoteDir string
	// BaseAPK is the device path to the installed com.snapchat.android base.apk.
	// Required.
	BaseAPK string
	// OracleDex is the device path to the dexed AttestationOracle helper.
	// Required.
	OracleDex string
	// LibPath is the device path to libscplugin.so. Required.
	LibPath string
	// TempDir is the local directory used to stage the afx request; defaults
	// to os.TempDir().
	TempDir string
}

// DefaultAdbAttestor returns the proven single-device configuration. It does
// not contact adb; callers should verify their device paths with `adb shell pm
// path com.snapchat.android` before relying on it.
func DefaultAdbAttestor() *AdbAttestor {
	return &AdbAttestor{
		ADB:       "adb",
		RemoteDir: "/data/local/tmp/bridge-snapchat-probe-20261022",
	}
}

func (attestor *AdbAttestor) adbArgs(args ...string) []string {
	if attestor.Serial != "" {
		return append([]string{"-s", attestor.Serial}, args...)
	}
	return args
}

func (attestor *AdbAttestor) command(ctx context.Context, args ...string) *exec.Cmd {
	name := attestor.ADB
	if name == "" {
		name = "adb"
	}
	return exec.CommandContext(ctx, name, attestor.adbArgs(args...)...)
}

// Attest stages the afx request on the device, runs the oracle helper, and
// returns the raw attestation blob.
func (attestor *AdbAttestor) Attest(ctx context.Context, request []byte) ([]byte, error) {
	if attestor.RemoteDir == "" {
		return nil, errors.New("adb attestor: RemoteDir is required")
	}
	if attestor.BaseAPK == "" {
		return nil, errors.New("adb attestor: BaseAPK is required")
	}
	if attestor.OracleDex == "" {
		return nil, errors.New("adb attestor: OracleDex is required")
	}
	if attestor.LibPath == "" {
		return nil, errors.New("adb attestor: LibPath is required")
	}

	tempDir := attestor.TempDir
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	localInput, err := os.CreateTemp(tempDir, "snapchat-janus-afx-*.bin")
	if err != nil {
		return nil, fmt.Errorf("adb attestor: stage input: %w", err)
	}
	localInputPath := localInput.Name()
	defer os.Remove(localInputPath)
	if _, err := localInput.Write(request); err != nil {
		localInput.Close()
		return nil, fmt.Errorf("adb attestor: write input: %w", err)
	}
	if err := localInput.Close(); err != nil {
		return nil, fmt.Errorf("adb attestor: close input: %w", err)
	}

	inputPath := filepath.ToSlash(filepath.Join(attestor.RemoteDir, "afx-input.bin"))
	outputPath := filepath.ToSlash(filepath.Join(attestor.RemoteDir, "afx-output.bin"))
	logPath := filepath.ToSlash(filepath.Join(attestor.RemoteDir, "oracle-run.log"))

	if output, err := attestor.command(ctx, "push", localInputPath, inputPath).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("adb attestor: push input: %w: %s", err, strings.TrimSpace(string(output)))
	}

	// Quote paths defensively; the staging dir is under our control, but the
	// installed base.apk path is not.
	shell := fmt.Sprintf("cd %s && CLASSPATH=%s:%s app_process64 / AttestationOracle %s < %s > %s 2> %s",
		shellQuote(attestor.RemoteDir),
		shellQuote(attestor.BaseAPK),
		shellQuote(attestor.OracleDex),
		shellQuote(attestor.LibPath),
		shellQuote(inputPath),
		shellQuote(outputPath),
		shellQuote(logPath),
	)
	if output, err := attestor.command(ctx, "shell", shell).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("adb attestor: run oracle: %w: %s", err, strings.TrimSpace(string(output)))
	}

	blob, err := attestor.command(ctx, "exec-out", "cat", outputPath).Output()
	if err != nil {
		return nil, fmt.Errorf("adb attestor: read output: %w", err)
	}
	if len(blob) == 0 {
		return nil, errors.New("adb attestor: oracle returned an empty blob")
	}
	return blob, nil
}

// shellQuote wraps a value in single quotes for the device shell, escaping
// embedded single quotes.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
