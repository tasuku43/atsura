package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var errEvidenceWriter = errors.New("write failed")

type shortEvidenceWriter struct{}

func (shortEvidenceWriter) Write(value []byte) (int, error) { return len(value) - 1, nil }

type failedEvidenceWriter struct{}

func (failedEvidenceWriter) Write([]byte) (int, error) { return 0, errEvidenceWriter }

func TestWriteEvidenceRejectsShortAndFailedWriters(t *testing.T) {
	for _, writer := range []interface{ Write([]byte) (int, error) }{shortEvidenceWriter{}, failedEvidenceWriter{}} {
		if err := writeEvidence(writer, evidenceDocument{}); err == nil {
			t.Fatalf("writer %T was accepted", writer)
		}
	}
	var output bytes.Buffer
	if err := writeEvidence(&output, evidenceDocument{}); err != nil || !bytes.HasSuffix(output.Bytes(), []byte{'\n'}) {
		t.Fatalf("normal evidence write = %q, %v", output.Bytes(), err)
	}
}

func TestParseOptionsRequiresExactSupportedIdentity(t *testing.T) {
	valid := []string{
		"--archive", "artifact.tar.gz", "--source", "fixture", "--tag", "v1.2.3-rc.1",
		"--revision", strings.Repeat("a", 40), "--goos", "linux", "--goarch", "arm64",
		"--processor-archive", "rtk.tar.gz",
	}
	got, err := parseOptions(valid)
	if err != nil || got.tag != "v1.2.3-rc.1" || got.goos != "linux" || got.goarch != "arm64" || got.processorArchive != "rtk.tar.gz" {
		t.Fatalf("parseOptions() = %+v, %v", got, err)
	}
	tests := [][]string{
		{"--archive", "artifact.tar.gz", "--source", "fixture", "--tag", "v1.2.3-rc.1", "--revision", strings.Repeat("a", 40), "--goos", "linux", "--goarch", "arm64"},
		{"--archive", "artifact.tar.gz", "--source", "fixture", "--tag", "bad", "--revision", strings.Repeat("a", 40), "--goos", "linux", "--goarch", "arm64", "--processor-archive", "rtk.tar.gz"},
		{"--archive", "artifact.tar.gz", "--source", "fixture", "--tag", "v1.2.3", "--revision", strings.Repeat("A", 40), "--goos", "linux", "--goarch", "arm64", "--processor-archive", "rtk.tar.gz"},
		{"--archive", "artifact.tar.gz", "--source", "fixture", "--tag", "v1.2.3", "--revision", strings.Repeat("a", 40), "--goos", "plan9", "--goarch", "arm64", "--processor-archive", "rtk.tar.gz"},
		{"--archive", "artifact.zip", "--source", "fixture", "--tag", "v1.2.3", "--revision", strings.Repeat("a", 40), "--goos", "windows", "--goarch", "amd64", "--processor-archive", "rtk.tar.gz"},
		{"--archive", "artifact.tar.gz", "--source", "fixture", "--tag", "v1.2.3", "--revision", strings.Repeat("a", 40), "--goos", "linux", "--goarch", "arm64", "--processor-archive", "rtk.tar.gz", "--processor-archive", "other.tar.gz"},
	}
	for _, arguments := range tests {
		if _, err := parseOptions(arguments); err == nil {
			t.Fatalf("parseOptions(%v) succeeded", arguments)
		}
	}
	windows, err := parseOptions([]string{"--archive", "artifact.zip", "--source", "fixture", "--tag", "v1.2.3", "--revision", strings.Repeat("a", 40), "--goos", "windows", "--goarch", "amd64"})
	if err != nil || windows.processorArchive != "" {
		t.Fatalf("Windows options = %+v, %v", windows, err)
	}
}

func TestTransformDraftProducesOneTypedTransform(t *testing.T) {
	for _, command := range []string{"pr", "issue"} {
		t.Run(command, func(t *testing.T) {
			draft := []byte(`schema_version: 4
catalog_digest: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
surface:
    default: exclude
commands:
    - command:
        - ` + command + `
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
			result, err := transformDraft(draft, []string{command, "list"})
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range []string{
				"kind: transform", `append_args: ["--json=number,title,state"]`,
				"default: exclude", "include: [--limit]", "kind: projection", "projection:", "select: [number, title, state]", "from: number", "to: id", "render: compact_json",
			} {
				if !bytes.Contains(result, []byte(required)) {
					t.Fatalf("transform missing %q:\n%s", required, result)
				}
			}
			if bytes.Contains(result, []byte("kind: identity")) {
				t.Fatalf("identity wrapper remains:\n%s", result)
			}
			for _, invalid := range [][]byte{[]byte("kind: identity\n"), append(append([]byte{}, draft...), draft...)} {
				if _, err := transformDraft(invalid, []string{command, "list"}); err == nil {
					t.Fatalf("transformDraft accepted invalid draft")
				}
			}
		})
	}
	if _, err := transformDraft([]byte("irrelevant"), []string{"repo", "list"}); err == nil {
		t.Fatal("unsupported command was accepted")
	}
	if _, err := transformDraft([]byte("irrelevant"), []string{"pr"}); err == nil {
		t.Fatal("incomplete command was accepted")
	}
}

func TestTransformSourceStreamDraftProducesExactIdentityAndAppendOnlyCases(t *testing.T) {
	draft := func(command string) []byte {
		return []byte(`schema_version: 4
catalog_digest: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
surface:
    default: exclude
commands:
    - command:
        - ` + command + `
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
	}
	tests := []struct {
		name      string
		command   []string
		required  []string
		forbidden []string
	}{
		{
			name: "identity", command: []string{"pr", "list"},
			required:  []string{"kind: identity", "append_args: []", "include: [--label, --repo, --search]"},
			forbidden: []string{"output:", "--limit=1"},
		},
		{
			name: "append_only", command: []string{"issue", "list"},
			required:  []string{"kind: transform", `append_args: ["--limit=1"]`, "include: [--label, --search]"},
			forbidden: []string{"output:"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := transformSourceStreamDraft(draft(test.command[0]), test.name, test.command)
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range test.required {
				if !bytes.Contains(result, []byte(required)) {
					t.Fatalf("source-stream draft missing %q:\n%s", required, result)
				}
			}
			for _, forbidden := range test.forbidden {
				if bytes.Contains(result, []byte(forbidden)) {
					t.Fatalf("source-stream draft contains %q:\n%s", forbidden, result)
				}
			}
		})
	}
	for _, invalid := range []struct {
		name    string
		command []string
	}{
		{name: "identity", command: []string{"issue", "list"}},
		{name: "append_only", command: []string{"pr", "list"}},
		{name: "unknown", command: []string{"pr", "list"}},
	} {
		if _, err := transformSourceStreamDraft(draft(invalid.command[0]), invalid.name, invalid.command); err == nil {
			t.Fatalf("invalid source-stream case succeeded: %+v", invalid)
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
			archive, digest, err := readReleaseArchive(archivePath)
			if err != nil || digest != digestBytes(archive) {
				t.Fatalf("read release archive = %s, %v", digest, err)
			}
			got, err := extractReleaseArchive(archive, goos, filepath.Join(root, "extract"))
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
		archive, _, err := readReleaseArchive(archivePath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := extractReleaseArchive(archive, "linux", filepath.Join(root, "extract")); err == nil {
			t.Fatalf("unsafe archive %d was accepted", index)
		}
	}
}

func TestReleaseArchiveDigestAndExtractionUseTheSameBytes(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "archive.tar.gz")
	writeTarGzip(t, archivePath, map[string]archiveTestMember{
		"atr": {mode: 0o755, value: "first-binary"}, "LICENSE": {mode: 0o644, value: "license"},
	})
	archive, digest, err := readReleaseArchive(archivePath)
	if err != nil || digest != digestBytes(archive) {
		t.Fatalf("read release archive = %s, %v", digest, err)
	}
	if err := os.Remove(archivePath); err != nil {
		t.Fatal(err)
	}
	writeTarGzip(t, archivePath, map[string]archiveTestMember{
		"atr": {mode: 0o755, value: "replacement"}, "LICENSE": {mode: 0o644, value: "license"},
	})
	executable, err := extractReleaseArchive(archive, "linux", filepath.Join(root, "extract"))
	if err != nil {
		t.Fatal(err)
	}
	value, err := os.ReadFile(executable)
	if err != nil || string(value) != "first-binary" {
		t.Fatalf("extracted executable = %q, %v", value, err)
	}
}

func TestExtractProcessorArchiveRequiresExactArchiveAndBinaryIdentity(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "rtk-test.tar.gz")
	writeTarGzip(t, archivePath, map[string]archiveTestMember{"rtk": {mode: 0o755, value: "processor-binary"}})
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	contract := processorArchiveContract{
		archiveName: filepath.Base(archivePath), archiveSHA256: digestBytes(archive), archiveSize: int64(len(archive)),
		binaryMember: "rtk", binarySHA256: digestBytes([]byte("processor-binary")), binarySize: int64(len("processor-binary")),
	}
	executable, err := extractProcessorArchiveContract(archivePath, filepath.Join(root, "extract"), contract)
	if err != nil {
		t.Fatal(err)
	}
	value, err := os.ReadFile(executable)
	if err != nil || string(value) != "processor-binary" {
		t.Fatalf("processor executable = %q, %v", value, err)
	}

	wrong := contract
	wrong.archiveSHA256 = strings.Repeat("0", 64)
	if _, err := extractProcessorArchiveContract(archivePath, filepath.Join(root, "wrong-hash"), wrong); err == nil {
		t.Fatal("processor archive with wrong provenance hash was accepted")
	}
	wrongBinary := contract
	wrongBinary.binarySHA256 = strings.Repeat("0", 64)
	if _, err := extractProcessorArchiveContract(archivePath, filepath.Join(root, "wrong-binary"), wrongBinary); err == nil {
		t.Fatal("processor archive with wrong binary identity was accepted")
	}
	unsafePath := filepath.Join(root, "rtk-extra.tar.gz")
	writeTarGzip(t, unsafePath, map[string]archiveTestMember{
		"rtk": {mode: 0o755, value: "processor-binary"}, "extra": {mode: 0o644, value: "extra"},
	})
	unsafeArchive, err := os.ReadFile(unsafePath)
	if err != nil {
		t.Fatal(err)
	}
	unsafe := contract
	unsafe.archiveName, unsafe.archiveSHA256, unsafe.archiveSize = filepath.Base(unsafePath), digestBytes(unsafeArchive), int64(len(unsafeArchive))
	if _, err := extractProcessorArchiveContract(unsafePath, filepath.Join(root, "unsafe"), unsafe); err == nil {
		t.Fatal("processor archive with an extra member was accepted")
	}
}

func TestStageSourceFixtureUsesOrdinaryCommandSpellingInIsolatedPath(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "native-fixture")
	if err := os.WriteFile(source, []byte("native fixture bytes"), 0o700); err != nil {
		t.Fatal(err)
	}
	workRoot := filepath.Join(root, "work")
	if err := os.Mkdir(workRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	staged, binPath, err := stageSourceFixture(source, "linux", workRoot)
	if err != nil {
		t.Fatal(err)
	}
	if staged != filepath.Join(binPath, "gh") {
		t.Fatalf("staged path = %q", staged)
	}
	value, err := os.ReadFile(staged)
	if err != nil || string(value) != "native fixture bytes" {
		t.Fatalf("staged value = %q, %v", value, err)
	}
	trustPath, environment, err := isolatedEnvironment(workRoot, binPath, filepath.Join(workRoot, "attempts.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if trustPath == "" || !strings.Contains(strings.Join(environment, "\n"), "PATH="+binPath) {
		t.Fatalf("isolated environment = %q", environment)
	}
	joined := strings.Join(environment, "\n")
	for _, required := range []string{
		"GOTOOLCHAIN=local", "GOPROXY=off", "CGO_ENABLED=0",
		goTestAttemptEnv + "=" + filepath.Join(workRoot, "go-test-attempts.log"),
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("isolated environment missing %q: %q", required, environment)
		}
	}
}

func TestGoSourceFixtureAndInspectionEvidenceAreFinite(t *testing.T) {
	root := t.TempDir()
	if err := createGoTestModule(root); err != nil {
		t.Fatal(err)
	}
	module, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || string(module) != "module example.com/atsura-artifact-go\n\ngo 1.26.0\n" {
		t.Fatalf("module = %q, %v", module, err)
	}
	testSource, err := os.ReadFile(filepath.Join(root, "artifact_test.go"))
	if err != nil || !bytes.Contains(testSource, []byte(goTestAttemptEnv)) || !bytes.Contains(testSource, []byte("func TestMain")) ||
		!bytes.Contains(testSource, []byte("func TestOne")) || !bytes.Contains(testSource, []byte("func TestTwo")) ||
		!bytes.Contains(testSource, []byte(goTestModeEnv)) || !bytes.Contains(testSource, []byte(goTestProcessorDrift)) {
		t.Fatalf("test source = %q, %v", testSource, err)
	}
	attemptLog := filepath.Join(root, "go-test-attempts.log")
	if err := requireGoTestAttempts(attemptLog, 0); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attemptLog, []byte("attempt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := requireGoTestAttempts(attemptLog, 1); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attemptLog, []byte("attempt\nattempt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := requireGoTestAttempts(attemptLog, 2); err != nil {
		t.Fatal(err)
	}

	inspection := inspectionEvidence{}
	inspection.Catalog.Commands = append(inspection.Catalog.Commands,
		struct {
			Path []string `json:"path"`
		}{Path: []string{"test"}},
	)
	if !inspectionHasCommand(inspection, []string{"test"}) || inspectionHasCommand(inspection, []string{"build"}) {
		t.Fatal("Go inspection command matching is not exact")
	}
	inspection.Catalog.Commands = append(inspection.Catalog.Commands, inspection.Catalog.Commands[0])
	if inspectionHasCommand(inspection, []string{"test"}) {
		t.Fatal("duplicate Go inspection command was accepted")
	}
	skip := []byte("{\"Time\":\"2026-01-01T00:00:00Z\",\"Action\":\"skip\",\"Package\":\"example.com/atsura-artifact-go\",\"Test\":\"TestOne\"}\n")
	if err := validateGoTestJSONL(skip, "skip"); err != nil {
		t.Fatal(err)
	}
	if err := validateGoTestJSONL(skip, "fail"); err == nil {
		t.Fatal("mismatched Go test action was accepted")
	}
	noTests := []byte("{\"Time\":\"2026-01-01T00:00:00Z\",\"Action\":\"pass\",\"Package\":\"example.com/atsura-artifact-go\"}\n")
	if err := validateGoTestJSONL(noTests, "no_tests"); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{
		"PASS\nok  \texample.com/atsura-artifact-go\t0.003s\n",
		"PASS\nok example.com/atsura-artifact-go 1s\n",
	} {
		if !goTestIdentityOutputPattern.MatchString(output) {
			t.Fatalf("valid Go identity output rejected: %q", output)
		}
	}
	if goTestIdentityOutputPattern.MatchString("ok example.com/atsura-artifact-go 0.003s\n") {
		t.Fatal("Go identity output without PASS was accepted")
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

	baseRecords := []fixtureAttemptRecord{
		{SchemaVersion: 1, Kind: "probe", Mode: "success", Argv: []string{"version"}},
		{SchemaVersion: 1, Kind: "probe", Mode: "success", Argv: []string{"help", "reference"}},
		{SchemaVersion: 1, Kind: "probe", Mode: "success", Argv: []string{"issue", "list", "--help"}},
		{SchemaVersion: 1, Kind: "probe", Mode: "success", Argv: []string{"pr", "list", "--help"}},
		{SchemaVersion: 1, Kind: "runtime", Mode: "command_failure", Argv: []string{"pr", "list", "--limit=1", "--json=number,title,state"}},
		{SchemaVersion: 1, Kind: "runtime", Mode: "stderr", Argv: []string{"pr", "list", "--limit=1", "--json=number,title,state"}},
		{SchemaVersion: 1, Kind: "runtime", Mode: "malformed", Argv: []string{"pr", "list", "--limit=1", "--json=number,title,state"}},
		{SchemaVersion: 1, Kind: "runtime", Mode: "missing_field", Argv: []string{"pr", "list", "--limit=1", "--json=number,title,state"}},
		{SchemaVersion: 1, Kind: "runtime", Mode: "success", Argv: []string{"pr", "list", "--limit=1", "--json=number,title,state"}},
		{SchemaVersion: 1, Kind: "runtime", Mode: "success", Argv: []string{"issue", "list", "--limit=1", "--json=number,title,state"}},
	}
	wrapperRecords := []fixtureAttemptRecord{
		{SchemaVersion: 1, Kind: "runtime", Mode: "success", Argv: []string{"pr", "list", "--limit=1", "--json=number,title,state"}},
		{SchemaVersion: 1, Kind: "runtime", Mode: "success", Argv: []string{
			"pr", "list", "--search=space value;$(touch atsura-artifact-injection)", "--label=first", "--label=Unicode 雪", "--repo=-dash",
		}},
		{SchemaVersion: 1, Kind: "runtime", Mode: "success", Argv: []string{
			"issue", "list", "--search=append value", "--label=one", "--label=two", "--limit=1",
		}},
	}
	posixRecords := append(append([]fixtureAttemptRecord{}, baseRecords...), wrapperRecords...)
	encodeRecords := func(records []fixtureAttemptRecord) []byte {
		var sequence bytes.Buffer
		for _, record := range records {
			if err := json.NewEncoder(&sequence).Encode(record); err != nil {
				t.Fatal(err)
			}
		}
		return sequence.Bytes()
	}
	sequence := encodeRecords(posixRecords)
	windowsSequence := encodeRecords(baseRecords)
	if err := validateAttemptSequence(sequence, "linux"); err != nil {
		t.Fatal(err)
	}
	if err := validateAttemptSequence(windowsSequence, "windows"); err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(sequence, []byte(`"mode":"stderr"`), []byte(`"mode":"success"`), 1)
	if err := validateAttemptSequence(changed, "linux"); err == nil {
		t.Fatal("changed attempt sequence was accepted")
	}
	if err := validateAttemptSequence(windowsSequence, "plan9"); err == nil {
		t.Fatal("unsupported attempt platform was accepted")
	}
	if !bytes.Contains(sequence, []byte("atsura-artifact-injection")) {
		t.Fatal("hostile argv evidence is missing")
	}

	declaration := helpFaultDeclaration{Code: "source_command_failed", Kind: "rejected", Retryable: false, NextActions: []helpNextAction{{Command: "help bundle execute", Reason: "Inspect independently."}}}
	fault := []byte(`{"schema_version":1,"error":{"kind":"rejected","code":"source_command_failed","retryable":false,"next_actions":[{"command":"help bundle execute","reason":"Inspect independently."}]}}`)
	if err := validateFault(fault, declaration); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*helpFaultDeclaration){
		func(value *helpFaultDeclaration) { value.Kind = "contract" },
		func(value *helpFaultDeclaration) { value.Retryable = true },
		func(value *helpFaultDeclaration) { value.NextActions[0].Command = "bundle status" },
		func(value *helpFaultDeclaration) { value.NextActions[0].Reason = "Different." },
	} {
		changed := helpFaultDeclaration{Code: declaration.Code, Kind: declaration.Kind, Retryable: declaration.Retryable, NextActions: append([]helpNextAction{}, declaration.NextActions...)}
		mutate(&changed)
		if err := validateFault(fault, changed); err == nil {
			t.Fatal("mismatched packaged fault declaration was accepted")
		}
	}
	if err := scanCanaries([]byte("safe"), []byte(secretCanaries[0])); err == nil {
		t.Fatal("secret canary was accepted")
	}
}

func TestOutputSchemaInventoriesAreExact(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		id      string
		version int
		fields  []helpSchemaFieldProjection
	}{
		{name: "catalog", field: "catalog", id: "source-command-catalog", version: 2, fields: sourceCatalogSchemaFields},
		{name: "processor", field: "observation", id: "processor-observation", version: 1, fields: processorObservationSchemaFields},
		{name: "specification", field: "specification", id: "tailoring-specification", version: 4, fields: tailoringSpecificationSchemaFields},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			valid := helpCommandProjection{}
			valid.Contract.Output.Fields = []helpOutputFieldProjection{{
				Name: test.field, Type: "object",
				Schema: &helpSchemaProjection{ID: test.id, Version: test.version, Fields: append([]helpSchemaFieldProjection{}, test.fields...)},
			}}
			if err := validateOutputSchema(valid, test.field, test.id, test.version, test.fields); err != nil {
				t.Fatal(err)
			}
			mutations := []func(*helpCommandProjection){
				func(value *helpCommandProjection) { value.Contract.Output.Fields[0].Type = "array" },
				func(value *helpCommandProjection) { value.Contract.Output.Fields[0].Schema.Fields[0].Type = "invalid" },
				func(value *helpCommandProjection) {
					value.Contract.Output.Fields[0].Schema.Fields[0].Required = !value.Contract.Output.Fields[0].Schema.Fields[0].Required
				},
				func(value *helpCommandProjection) {
					value.Contract.Output.Fields[0].Schema.Fields = append(value.Contract.Output.Fields[0].Schema.Fields, helpSchemaFieldProjection{Path: "/extra", Type: "string", Required: true})
				},
			}
			for _, mutate := range mutations {
				changed := cloneHelpCommand(t, valid)
				mutate(&changed)
				if err := validateOutputSchema(changed, test.field, test.id, test.version, test.fields); err == nil {
					t.Fatal("changed output schema inventory was accepted")
				}
			}
		})
	}
}

func TestBundleRuntimeHelpFaultMatricesAreExact(t *testing.T) {
	tests := []struct {
		path   string
		count  int
		faults []helpFaultDeclaration
	}{
		{path: "bundle preview", count: 27, faults: bundlePreviewHelpFaults},
		{path: "bundle execute", count: 41, faults: bundleExecuteHelpFaults},
		{path: "wrapper render", count: 33, faults: wrapperRenderHelpFaults},
		{path: "wrapper run", count: 63, faults: wrapperRunHelpFaults},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if len(test.faults) != test.count {
				t.Fatalf("fault count = %d, want %d", len(test.faults), test.count)
			}
			if err := validateHelpFaultMatrix(test.faults, test.faults); err != nil {
				t.Fatal(err)
			}

			mutations := []struct {
				name   string
				mutate func([]helpFaultDeclaration) []helpFaultDeclaration
			}{
				{name: "missing", mutate: func(value []helpFaultDeclaration) []helpFaultDeclaration {
					return value[:len(value)-1]
				}},
				{name: "extra", mutate: func(value []helpFaultDeclaration) []helpFaultDeclaration {
					return append(value, expectedHelpFault("unexpected", "internal", false, "help", "Unexpected."))
				}},
				{name: "duplicate", mutate: func(value []helpFaultDeclaration) []helpFaultDeclaration {
					value[len(value)-1] = value[0]
					return value
				}},
				{name: "order", mutate: func(value []helpFaultDeclaration) []helpFaultDeclaration {
					value[0], value[1] = value[1], value[0]
					return value
				}},
				{name: "code", mutate: func(value []helpFaultDeclaration) []helpFaultDeclaration {
					value[0].Code += "_changed"
					return value
				}},
				{name: "kind", mutate: func(value []helpFaultDeclaration) []helpFaultDeclaration {
					value[0].Kind = "contract"
					return value
				}},
				{name: "retryable", mutate: func(value []helpFaultDeclaration) []helpFaultDeclaration {
					value[0].Retryable = !value[0].Retryable
					return value
				}},
				{name: "missing next action", mutate: func(value []helpFaultDeclaration) []helpFaultDeclaration {
					value[0].NextActions = nil
					return value
				}},
				{name: "extra next action", mutate: func(value []helpFaultDeclaration) []helpFaultDeclaration {
					value[0].NextActions = append(value[0].NextActions, helpNextAction{Command: "help", Reason: "Unexpected."})
					return value
				}},
				{name: "next command", mutate: func(value []helpFaultDeclaration) []helpFaultDeclaration {
					value[0].NextActions[0].Command += " changed"
					return value
				}},
				{name: "next reason", mutate: func(value []helpFaultDeclaration) []helpFaultDeclaration {
					value[0].NextActions[0].Reason += " Changed."
					return value
				}},
			}
			for _, mutation := range mutations {
				t.Run(mutation.name, func(t *testing.T) {
					changed := mutation.mutate(cloneHelpFaults(test.faults))
					if err := validateHelpFaultMatrix(changed, test.faults); err == nil {
						t.Fatal("changed fault matrix was accepted")
					}
				})
			}
		})
	}
}

func cloneHelpFaults(value []helpFaultDeclaration) []helpFaultDeclaration {
	result := make([]helpFaultDeclaration, len(value))
	for index := range value {
		result[index] = value[index]
		result[index].NextActions = append([]helpNextAction{}, value[index].NextActions...)
	}
	return result
}

func cloneHelpCommand(t *testing.T, value helpCommandProjection) helpCommandProjection {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result helpCommandProjection
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	return result
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
	for _, test := range []struct {
		command string
		id      string
		title   string
	}{
		{command: "pr list", id: "101", title: `"Review policy"`},
		{command: "issue list", id: "202", title: `"Fix deterministic wrapper"`},
	} {
		value := executionEvidence{MatchedCommand: strings.Split(test.command, " "), WrapperKind: "transform", SourceProcessAttempts: 1}
		value.Output.Render = "compact_json"
		value.Output.Shape = "array"
		value.Output.Fields = []string{"id", "title", "state"}
		value.Output.Records = []map[string]json.RawMessage{{
			"id": json.RawMessage(test.id), "title": json.RawMessage(test.title), "state": json.RawMessage(`"OPEN"`),
		}}
		value.Source.ExitCode = 0
		if err := validateSelectedOutput(value, test.command); err != nil {
			t.Fatal(err)
		}
		value.Output.Records[0]["ignored"] = json.RawMessage(`"secret"`)
		if err := validateSelectedOutput(value, test.command); err == nil {
			t.Fatal("unselected field was accepted")
		}
	}
}

func TestWrapperResultDigestsMatchOrderedEvidenceContract(t *testing.T) {
	tests := []struct {
		name       string
		stdout     []byte
		stderr     []byte
		stdoutHash string
		stderrHash string
	}{
		{
			name:   "transformed_json",
			stdout: []byte("[{\"id\":101,\"title\":\"Review policy\",\"state\":\"OPEN\"}]\n"), stderr: []byte{},
			stdoutHash: "277258cb99075f67f56acb96a0d7a340644442f0147385cbfef6634897437ade",
			stderrHash: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:   "identity",
			stdout: []byte{'I', 'D', ':', 0x00, 0xff, '\n'}, stderr: []byte{'I', 'D', 'E', 'R', 'R', ':', 0xfe},
			stdoutHash: "211630ed346fee12b3e2c5135f3239dc7ce64e10eb149e8ef032bc04ff115b7b",
			stderrHash: "cfc159919dad8548c6e2ed887297e77aed35d6f2d20d42c08b29d7caa4f8faa0",
		},
		{
			name:   "append_only",
			stdout: []byte{'A', 'P', 'P', ':', 0xff, 0x00}, stderr: []byte("APPERR:\n"),
			stdoutHash: "162a8a6b49c40255d3d0d2e5ed86f5d4ca88b3963d8c667bd7b79e768bd26d29",
			stderrHash: "b8f249840842aad27390cfb637be1e2456a9d873ab1141d01d2cdccff1699c4a",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := digestBytes(test.stdout); got != test.stdoutHash {
				t.Fatalf("stdout digest=%s want=%s", got, test.stdoutHash)
			}
			if got := digestBytes(test.stderr); got != test.stderrHash {
				t.Fatalf("stderr digest=%s want=%s", got, test.stderrHash)
			}
		})
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
