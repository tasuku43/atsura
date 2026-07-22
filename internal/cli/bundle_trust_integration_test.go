package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/infra/bundlejson"
)

const (
	bundleTrustNoTerminalModeEnv = "ATSURA_TEST_BUNDLE_TRUST_NO_TERMINAL_MODE"
	bundleTrustNoTerminalPathEnv = "ATSURA_TEST_BUNDLE_TRUST_NO_TERMINAL_PATH"
)

func TestBundleTrustRejectsRedirectedDigestWithoutControllingTerminal(t *testing.T) {
	if mode := os.Getenv(bundleTrustNoTerminalModeEnv); mode != "" {
		path := os.Getenv(bundleTrustNoTerminalPathEnv)
		arguments := []string{"--error-format=json", "bundle", mode, "--bundle", path}
		os.Exit(New(os.Stdin, os.Stdout, os.Stderr).RunContext(context.Background(), arguments))
	}

	catalogPath, specificationPath := bundleArtifactPaths(t)
	bundlePath := bundleArtifactPath(t, catalogPath, specificationPath)
	_, digest, err := bundlejson.New().Load(context.Background(), bundlePath)
	if err != nil {
		t.Fatal(err)
	}

	configRoot := t.TempDir()
	configDirectory := filepath.Join(configRoot, "config")
	if runtime.GOOS == "darwin" {
		configDirectory = filepath.Join(configRoot, "Library", "Application Support")
	}
	if err := os.MkdirAll(configDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(configDirectory, "atsura", "bundle-trust.json")
	environment := isolatedBundleTrustEnvironment(configRoot, configDirectory, bundlePath)

	trust := runBundleTrustSubprocess(t, environment, "trust", digest+"\n")
	if trust.exitCode != ExitRejected || len(trust.stdout) != 0 {
		t.Fatalf("bundle trust exit=%d stdout=%q stderr=%q", trust.exitCode, trust.stdout, trust.stderr)
	}
	var failure errorDocument
	if err := json.Unmarshal(trust.stderr, &failure); err != nil {
		t.Fatal(err)
	}
	if failure.SchemaVersion != 1 || failure.Error.Kind != fault.KindRejected || failure.Error.Code != "mutation_rejected" || failure.Error.Retryable || len(failure.Error.NextActions) != 1 || failure.Error.NextActions[0].Command != "bundle status" {
		t.Fatalf("bundle trust failure = %+v", failure)
	}
	assertNoTrustReceipt(t, receiptPath)

	status := runBundleTrustSubprocess(t, environment, "status", "")
	if status.exitCode != ExitOK || len(status.stderr) != 0 {
		t.Fatalf("bundle status exit=%d stdout=%q stderr=%q", status.exitCode, status.stdout, status.stderr)
	}
	var document bundleStatusDocument
	if err := json.Unmarshal(status.stdout, &document); err != nil {
		t.Fatal(err)
	}
	if document.SchemaVersion != 3 || document.Status.BundleDigest != digest || document.Status.Adoption != bundletrust.StateNotAdopted || document.Status.Adopted || document.Status.Source != bundletrust.SourceCurrent ||
		document.Status.Processors == nil || len(document.Status.Processors) != 0 || document.Status.SourceProcessAttempts != 0 || document.Status.ProcessorProcessAttempts != 0 {
		t.Fatalf("bundle status = %+v", document)
	}
	assertNoTrustReceipt(t, receiptPath)
}

type bundleTrustSubprocessResult struct {
	exitCode int
	stdout   []byte
	stderr   []byte
}

func runBundleTrustSubprocess(t *testing.T, environment []string, mode, stdin string) bundleTrustSubprocessResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBundleTrustRejectsRedirectedDigestWithoutControllingTerminal$")
	command.Env = replaceBundleTrustEnvironment(environment, map[string]string{bundleTrustNoTerminalModeEnv: mode})
	command.Stdin = strings.NewReader(stdin)
	withoutControllingTerminal(command)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if ctx.Err() != nil {
		t.Fatalf("bundle %s subprocess did not terminate: %v", mode, ctx.Err())
	}
	result := bundleTrustSubprocessResult{stdout: stdout.Bytes(), stderr: stderr.Bytes()}
	if err == nil {
		return result
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		t.Fatalf("bundle %s subprocess failed to start: %v", mode, err)
	}
	result.exitCode = exitError.ExitCode()
	return result
}

func isolatedBundleTrustEnvironment(root, config, bundlePath string) []string {
	return replaceBundleTrustEnvironment(os.Environ(), map[string]string{
		"HOME":                       root,
		"USERPROFILE":                root,
		"XDG_CONFIG_HOME":            config,
		"APPDATA":                    config,
		"LOCALAPPDATA":               config,
		bundleTrustNoTerminalPathEnv: bundlePath,
	})
}

func replaceBundleTrustEnvironment(base []string, replacements map[string]string) []string {
	result := make([]string, 0, len(base)+len(replacements))
	for _, item := range base {
		key, _, present := strings.Cut(item, "=")
		if !present {
			continue
		}
		replaced := false
		for replacement := range replacements {
			if strings.EqualFold(key, replacement) {
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, item)
		}
	}
	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result = append(result, key+"="+replacements[key])
	}
	return result
}

func assertNoTrustReceipt(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("trust receipt exists or could not be inspected: %q: %v", path, err)
	}
}
