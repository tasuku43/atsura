package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fixtureRun(args []string, environment map[string]string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	getenv := func(name string) string { return environment[name] }
	exit := run(args, getenv, &stdout, &stderr)
	return exit, stdout.String(), stderr.String()
}

func TestExactFourProbeArgv(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		contains string
	}{
		{name: "version", args: []string{"version"}, contains: "gh version 2.72.0"},
		{name: "reference", args: []string{"help", "reference"}, contains: "## gh api <endpoint> [flags]"},
		{name: "issue help", args: []string{"issue", "list", "--help"}, contains: "gh issue list [flags]"},
		{name: "pr help", args: []string{"pr", "list", "--help"}, contains: "gh pr list [flags]"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exit, stdout, stderr := fixtureRun(test.args, map[string]string{})
			if exit != exitOK || stderr != "" || !strings.Contains(stdout, test.contains) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
			}
		})
	}
	for _, args := range [][]string{
		{}, {"--version"}, {"help"}, {"help", "reference", "extra"},
		{"issue", "list", "-h"}, {"pr", "list", "--help", "extra"}, {"api", "--help"},
	} {
		exit, stdout, stderr := fixtureRun(args, map[string]string{})
		if exit != exitUsage || stdout != "" || !strings.Contains(stderr, "unsupported argv") {
			t.Fatalf("args=%v exit=%d stdout=%q stderr=%q", args, exit, stdout, stderr)
		}
	}
}

func TestListRuntimeSuccessForPullRequestsAndIssues(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		number float64
		title  string
	}{
		{
			name: "pull requests", args: []string{"pr", "list", "--limit=1", "--json=number,title,state"},
			number: 101, title: "Review policy",
		},
		{
			name: "issues", args: []string{"issue", "list", "--limit=1", "--json=number,title,state"},
			number: 202, title: "Fix deterministic wrapper",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exit, stdout, stderr := fixtureRun(test.args, map[string]string{modeEnvironment: modeSuccess})
			if exit != exitOK || stderr != "" || !strings.Contains(stdout, unselectedCanary) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
			}
			var records []map[string]any
			if err := json.Unmarshal([]byte(stdout), &records); err != nil {
				t.Fatal(err)
			}
			if len(records) != 1 || records[0]["number"] != test.number || records[0]["title"] != test.title || records[0]["state"] != "OPEN" || records[0]["ignored"] != unselectedCanary {
				t.Fatalf("records=%v", records)
			}
		})
	}
}

func TestOrdinarySourceStreamFixturesRequireExactReviewedArgv(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantExit   int
		wantStdout string
		wantStderr string
	}{
		{
			name: "identity",
			args: []string{
				"pr", "list",
				"--search=space value;$(touch atsura-artifact-injection)",
				"--label=first",
				"--label=Unicode 雪",
				"--repo=-dash",
			},
			wantExit: exitOK, wantStdout: identityStreamStdout, wantStderr: identityStreamStderr,
		},
		{
			name: "append only",
			args: []string{
				"issue", "list",
				"--search=append value",
				"--label=one",
				"--label=two",
				"--limit=1",
			},
			wantExit: exitAppendOnly, wantStdout: appendOnlyStreamStdout, wantStderr: appendOnlyStreamStderr,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			exit, stdout, stderr := fixtureRun(test.args, map[string]string{modeEnvironment: modeSuccess})
			if exit != test.wantExit || stdout != test.wantStdout || stderr != test.wantStderr {
				t.Fatalf("exit=%d stdout=%x stderr=%x", exit, []byte(stdout), []byte(stderr))
			}
			changed := append([]string{}, test.args...)
			changed[len(changed)-1] += "-changed"
			exit, stdout, stderr = fixtureRun(changed, map[string]string{modeEnvironment: modeSuccess})
			if exit != exitUsage || stdout != "" || !strings.Contains(stderr, "unsupported argv") {
				t.Fatalf("changed argv accepted: exit=%d stdout=%x stderr=%q", exit, []byte(stdout), stderr)
			}
		})
	}
}

func TestRuntimeFailureModesDoNotChangeProbeBehavior(t *testing.T) {
	args := []string{"pr", "list", "--limit=1", "--json=number,title,state"}
	tests := []struct {
		mode         string
		wantExit     int
		wantStdout   string
		wantStderr   string
		missingState bool
		validJSON    bool
	}{
		{mode: modeCommandFailure, wantExit: exitCommandFailure, wantStdout: stdoutCanary, wantStderr: stderrCanary},
		{mode: modeStderr, wantExit: exitOK, wantStderr: stderrCanary + "\n", validJSON: true},
		{mode: modeMalformed, wantExit: exitOK, wantStdout: `[{` + stdoutCanary},
		{mode: modeMissingField, wantExit: exitOK, missingState: true, validJSON: true},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			exit, stdout, stderr := fixtureRun(args, map[string]string{modeEnvironment: test.mode})
			if exit != test.wantExit || stderr != test.wantStderr {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
			}
			if test.wantStdout != "" && stdout != test.wantStdout {
				t.Fatalf("stdout=%q, want %q", stdout, test.wantStdout)
			}
			if test.validJSON {
				var records []map[string]any
				if err := json.Unmarshal([]byte(stdout), &records); err != nil {
					t.Fatal(err)
				}
				if len(records) != 1 {
					t.Fatalf("records=%v", records)
				}
				_, hasState := records[0]["state"]
				if hasState == test.missingState {
					t.Fatalf("state present=%t record=%v", hasState, records[0])
				}
			}
		})
	}
	for _, mode := range []string{modeCommandFailure, modeStderr, modeMalformed, modeMissingField} {
		exit, stdout, stderr := fixtureRun([]string{"version"}, map[string]string{modeEnvironment: mode})
		if exit != exitOK || stderr != "" || stdout != versionFixture {
			t.Fatalf("mode=%s exit=%d stdout=%q stderr=%q", mode, exit, stdout, stderr)
		}
	}
}

func TestRuntimeRequiresExactReviewedArgvAndMode(t *testing.T) {
	for _, args := range [][]string{
		{"pr", "list", "--json=number,title,state"},
		{"pr", "list", "--limit", "1", "--json=number,title,state"},
		{"pr", "list", "--json=number,title,state", "--limit=1"},
		{"pr", "list", "--limit=1", "--json=title,number,state"},
		{"issue", "list", "--limit=2", "--json=number,title,state"},
	} {
		exit, stdout, stderr := fixtureRun(args, map[string]string{})
		if exit != exitUsage || stdout != "" || !strings.Contains(stderr, "unsupported argv") {
			t.Fatalf("args=%v exit=%d stdout=%q stderr=%q", args, exit, stdout, stderr)
		}
	}
	exit, stdout, stderr := fixtureRun([]string{"version"}, map[string]string{modeEnvironment: "unknown"})
	if exit != exitUsage || stdout != "" || !strings.Contains(stderr, "unsupported "+modeEnvironment) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
}

func TestAttemptLogIsAppendOnlyJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attempts.jsonl")
	environment := map[string]string{attemptLogEnvironment: path}
	if exit, _, stderr := fixtureRun([]string{"version"}, environment); exit != exitOK || stderr != "" {
		t.Fatalf("version exit=%d stderr=%q", exit, stderr)
	}
	environment[modeEnvironment] = modeMissingField
	if exit, _, stderr := fixtureRun([]string{"pr", "list", "--limit=1", "--json=number,title,state"}, environment); exit != exitOK || stderr != "" {
		t.Fatalf("runtime exit=%d stderr=%q", exit, stderr)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("log=%q", raw)
	}
	var records []attemptRecord
	for _, line := range lines {
		var record attemptRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	if records[0].SchemaVersion != 1 || records[0].Kind != "probe" || records[0].Mode != modeSuccess || strings.Join(records[0].Argv, " ") != "version" {
		t.Fatalf("first=%+v", records[0])
	}
	if records[1].Kind != "runtime" || records[1].Mode != modeMissingField || strings.Join(records[1].Argv, " ") != "pr list --limit=1 --json=number,title,state" {
		t.Fatalf("second=%+v", records[1])
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode=%o", info.Mode().Perm())
		}
	}
}

func TestAttemptLogRejectsUnsafePathsAndFiles(t *testing.T) {
	exit, stdout, stderr := fixtureRun([]string{"version"}, map[string]string{attemptLogEnvironment: "relative.jsonl"})
	if exit != exitUsage || stdout != "" || !strings.Contains(stderr, "absolute clean path") {
		t.Fatalf("relative exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
	directory := t.TempDir()
	target := filepath.Join(directory, "target.jsonl")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "link.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	exit, stdout, stderr = fixtureRun([]string{"version"}, map[string]string{attemptLogEnvironment: link})
	if exit != exitUsage || stdout != "" || !strings.Contains(stderr, "regular file") {
		t.Fatalf("symlink exit=%d stdout=%q stderr=%q", exit, stdout, stderr)
	}
}

func TestSensitiveEnvironmentDetectionIsFailClosed(t *testing.T) {
	for _, key := range []string{"GH_TOKEN", "GITHUB_SECRET", "DB_PASSWORD", "AWS_ACCESS_KEY_ID", "CLIENT_CREDENTIAL"} {
		if !sensitiveEnvironmentPresent([]string{key + "=synthetic"}) {
			t.Errorf("sensitive environment key %q was accepted", key)
		}
	}
	if sensitiveEnvironmentPresent([]string{"SYSTEMROOT=C:\\Windows", "TMP=/tmp", attemptLogEnvironment + "=/tmp/attempts"}) {
		t.Fatal("bounded non-credential environment was rejected")
	}
}
