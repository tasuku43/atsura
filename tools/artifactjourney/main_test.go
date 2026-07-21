package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOptionsRequiresExactSupportedIdentity(t *testing.T) {
	valid := []string{
		"--archive", "artifact.tar.gz", "--source", "fixture", "--tag", "v1.2.3-rc.1",
		"--revision", strings.Repeat("a", 40), "--goos", "linux", "--goarch", "arm64",
	}
	got, err := parseOptions(valid)
	if err != nil || got.tag != "v1.2.3-rc.1" || got.goos != "linux" || got.goarch != "arm64" {
		t.Fatalf("parseOptions() = %+v, %v", got, err)
	}
	tests := [][]string{
		valid[:len(valid)-2],
		append(append([]string{}, valid[:7]...), append([]string{"bad"}, valid[8:]...)...),
		append(append([]string{}, valid[:9]...), append([]string{strings.Repeat("A", 40)}, valid[10:]...)...),
		append(append([]string{}, valid[:11]...), append([]string{"plan9"}, valid[12:]...)...),
	}
	for _, arguments := range tests {
		if _, err := parseOptions(arguments); err == nil {
			t.Fatalf("parseOptions(%v) succeeded", arguments)
		}
	}
}

func TestTransformDraftProducesOneTypedTransform(t *testing.T) {
	draft := []byte(`schema_version: 3
catalog_digest: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
surface:
    default: exclude
commands:
    - command:
        - pr
        - list
      presence: include
      reason: Include this verified command without transformation.
      options:
        default: inherit
        include: []
        exclude: []
      wrapper:
        kind: identity
        before: []
        invoke:
            append_args: []
        after: []
`)
	result, err := transformDraft(draft)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"kind: transform", `append_args: ["--json=number,title,state"]`,
		"select: [number, title, state]", "from: number", "to: id", "render: compact_json",
	} {
		if !bytes.Contains(result, []byte(required)) {
			t.Fatalf("transform missing %q:\n%s", required, result)
		}
	}
	if bytes.Contains(result, []byte("kind: identity")) {
		t.Fatalf("identity wrapper remains:\n%s", result)
	}
	for _, invalid := range [][]byte{[]byte("kind: identity\n"), append(append([]byte{}, draft...), draft...)} {
		if _, err := transformDraft(invalid); err == nil {
			t.Fatalf("transformDraft accepted invalid draft")
		}
	}
}

func TestExtractReleaseArchiveAcceptsExactTarAndZIPMembers(t *testing.T) {
	for _, format := range []string{"tar", "zip"} {
		t.Run(format, func(t *testing.T) {
			root := t.TempDir()
			archivePath := filepath.Join(root, "archive")
			executable := "atr"
			goos := "linux"
			if format == "zip" {
				executable = "atr.exe"
				goos = "windows"
				writeZIP(t, archivePath, map[string]archiveTestMember{executable: {mode: 0o755, value: "binary"}, "LICENSE": {mode: 0o644, value: "license"}})
			} else {
				writeTarGzip(t, archivePath, map[string]archiveTestMember{executable: {mode: 0o755, value: "binary"}, "LICENSE": {mode: 0o644, value: "license"}})
			}
			got, err := extractReleaseArchive(archivePath, goos, filepath.Join(root, "extract"))
			if err != nil {
				t.Fatal(err)
			}
			value, err := os.ReadFile(got)
			if err != nil || string(value) != "binary" {
				t.Fatalf("executable = %q, %v", value, err)
			}
		})
	}
}

func TestExtractReleaseArchiveRejectsUnsafeMembers(t *testing.T) {
	tests := []map[string]archiveTestMember{
		{"atr": {mode: 0o755, value: "binary"}},
		{"atr": {mode: 0o644, value: "binary"}, "LICENSE": {mode: 0o644, value: "license"}},
		{"atr": {mode: 0o755, value: "binary"}, "LICENSE": {mode: 0o644, value: "license"}, "../escape": {mode: 0o644, value: "unsafe"}},
	}
	for index, members := range tests {
		root := t.TempDir()
		archivePath := filepath.Join(root, "archive.tar.gz")
		writeTarGzip(t, archivePath, members)
		if _, err := extractReleaseArchive(archivePath, "linux", filepath.Join(root, "extract")); err == nil {
			t.Fatalf("unsafe archive %d was accepted", index)
		}
	}
}

func TestAttemptsFaultsAndCanariesAreStrict(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "attempts.jsonl")
	if err := os.WriteFile(logPath, []byte("{\"argv\":[]}\n{\"argv\":[\"pr\"]}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := requireAttempts(logPath, 2); err != nil {
		t.Fatal(err)
	}
	if err := requireAttempts(logPath, 1); err == nil {
		t.Fatal("attempt mismatch was accepted")
	}
	if err := os.WriteFile(logPath, []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := requireAttempts(logPath, 1); err == nil {
		t.Fatal("invalid JSONL was accepted")
	}

	var sequence bytes.Buffer
	for _, record := range []fixtureAttemptRecord{
		{SchemaVersion: 1, Kind: "probe", Mode: "success", Argv: []string{"version"}},
		{SchemaVersion: 1, Kind: "probe", Mode: "success", Argv: []string{"help", "reference"}},
		{SchemaVersion: 1, Kind: "probe", Mode: "success", Argv: []string{"issue", "list", "--help"}},
		{SchemaVersion: 1, Kind: "probe", Mode: "success", Argv: []string{"pr", "list", "--help"}},
		{SchemaVersion: 1, Kind: "runtime", Mode: "command_failure", Argv: []string{"pr", "list", "--limit=1", "--json=number,title,state"}},
		{SchemaVersion: 1, Kind: "runtime", Mode: "stderr", Argv: []string{"pr", "list", "--limit=1", "--json=number,title,state"}},
		{SchemaVersion: 1, Kind: "runtime", Mode: "malformed", Argv: []string{"pr", "list", "--limit=1", "--json=number,title,state"}},
		{SchemaVersion: 1, Kind: "runtime", Mode: "missing_field", Argv: []string{"pr", "list", "--limit=1", "--json=number,title,state"}},
		{SchemaVersion: 1, Kind: "runtime", Mode: "success", Argv: []string{"pr", "list", "--limit=1", "--json=number,title,state"}},
	} {
		if err := json.NewEncoder(&sequence).Encode(record); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateAttemptSequence(sequence.Bytes()); err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(sequence.Bytes(), []byte(`"mode":"stderr"`), []byte(`"mode":"success"`), 1)
	if err := validateAttemptSequence(changed); err == nil {
		t.Fatal("changed attempt sequence was accepted")
	}

	fault := []byte(`{"schema_version":1,"error":{"code":"source_command_failed","retryable":false}}`)
	if err := validateFault(fault, "source_command_failed", false); err != nil {
		t.Fatal(err)
	}
	if err := validateFault(fault, "source_command_failed", true); err == nil {
		t.Fatal("wrong retryability was accepted")
	}
	if err := scanCanaries([]byte("safe"), []byte(secretCanaries[0])); err == nil {
		t.Fatal("secret canary was accepted")
	}
}

func TestMinimalChildEnvironmentDropsCredentialsAndRepositoryContext(t *testing.T) {
	base := []string{
		"PATH=/untrusted/path",
		"GH_TOKEN=secret",
		"GITHUB_TOKEN=secret",
		"AWS_ACCESS_KEY_ID=secret",
		"SYSTEMROOT=C:\\Windows",
		"TMP=/safe/temp",
	}
	got := minimalChildEnvironment(base)
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "secret") || strings.Contains(joined, "PATH=") || !strings.Contains(joined, "SYSTEMROOT=C:\\Windows") || !strings.Contains(joined, "TMP=/safe/temp") {
		t.Fatalf("minimal child environment=%q", joined)
	}
}

func TestSelectedOutputRequiresOnlyRenamedFields(t *testing.T) {
	value := executionEvidence{
		MatchedCommand: []string{"pr", "list"}, WrapperKind: "transform", SourceProcessAttempts: 1,
	}
	value.Output.Render = "compact_json"
	value.Output.Shape = "array"
	value.Output.Fields = []string{"id", "title", "state"}
	value.Output.Records = []map[string]json.RawMessage{{
		"id": json.RawMessage("101"), "title": json.RawMessage(`"Review policy"`), "state": json.RawMessage(`"OPEN"`),
	}}
	value.Source.ExitCode = 0
	if err := validateSelectedOutput(value); err != nil {
		t.Fatal(err)
	}
	value.Output.Records[0]["ignored"] = json.RawMessage(`"secret"`)
	if err := validateSelectedOutput(value); err == nil {
		t.Fatal("unselected field was accepted")
	}
}

type archiveTestMember struct {
	mode  os.FileMode
	value string
}

func writeTarGzip(t *testing.T, path string, members map[string]archiveTestMember) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(file)
	archive := tar.NewWriter(compressed)
	for name, member := range members {
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: int64(member.mode), Size: int64(len(member.value)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(member.value)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeZIP(t *testing.T, path string, members map[string]archiveTestMember) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for name, member := range members {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.SetMode(member.mode)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(member.value)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
