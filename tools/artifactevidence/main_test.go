package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
	"github.com/tasuku43/atsura/tools/internal/processormanifest"
)

const (
	testTag      = "v1.2.3-rc.1"
	testVersion  = "1.2.3-rc.1"
	testRevision = "0123456789abcdef0123456789abcdef01234567"
)

func TestRunAggregatesCanonicalNativeEvidence(t *testing.T) {
	directory := t.TempDir()
	archives := t.TempDir()
	digests := writeValidEvidenceSet(t, directory, archives)
	var output bytes.Buffer
	if err := run(validArguments(directory, archives), &output); err != nil {
		t.Fatal(err)
	}
	wanted := fmt.Sprintf(`{"schema_version":2,"tag":"v1.2.3-rc.1","revision":"0123456789abcdef0123456789abcdef01234567","provenance_level":"workflow_index_unattested","targets":[{"target":"linux/amd64","result":"passed","archive_name":"atr_v1.2.3-rc.1_linux_amd64.tar.gz","archive_sha256":"%s"},{"target":"linux/arm64","result":"passed","archive_name":"atr_v1.2.3-rc.1_linux_arm64.tar.gz","archive_sha256":"%s"},{"target":"darwin/amd64","result":"passed","archive_name":"atr_v1.2.3-rc.1_darwin_amd64.tar.gz","archive_sha256":"%s"},{"target":"darwin/arm64","result":"passed","archive_name":"atr_v1.2.3-rc.1_darwin_arm64.tar.gz","archive_sha256":"%s"},{"target":"windows/amd64","result":"passed","archive_name":"atr_v1.2.3-rc.1_windows_amd64.zip","archive_sha256":"%s"}]}`+"\n", digests[0], digests[1], digests[2], digests[3], digests[4])
	if output.String() != wanted {
		t.Fatalf("aggregate mismatch\n got: %s\nwant: %s", output.String(), wanted)
	}
	if strings.Contains(output.String(), directory) || strings.Contains(output.String(), archives) ||
		strings.Contains(output.String(), "bundle_digest") || strings.Contains(output.String(), "plan_digest") ||
		strings.Contains(output.String(), "commands_verified") || strings.Contains(output.String(), "caller_argv") {
		t.Fatalf("aggregate leaked non-contract evidence: %s", output.String())
	}
}

func TestSchemaEightEvidenceRemainsWithinJourneyBound(t *testing.T) {
	value := marshalEvidence(t, validEvidence("linux/amd64", targetContracts[0].archiveName(testTag), strings.Repeat("0", digestLength)))
	if len(value)+1 > maxEvidenceFileBytes {
		t.Fatalf("schema-8 evidence bytes=%d, limit=%d", len(value)+1, maxEvidenceFileBytes)
	}
}

func TestParseOptionsRejectsInvalidArguments(t *testing.T) {
	directory := t.TempDir()
	archives := t.TempDir()
	tests := [][]string{
		nil,
		{"--directory", directory, "--archives", archives, "--tag", testTag},
		{"--directory", directory, "--archives", archives, "--tag", testTag, "--revision", testRevision, "extra"},
		{"--directory", directory, "--archives", archives, "--tag", "1.2.3", "--revision", testRevision},
		{"--directory", directory, "--archives", archives, "--tag", "v1.2.3+build", "--revision", testRevision},
		{"--directory", directory, "--archives", archives, "--tag", "v1.2.3-01", "--revision", testRevision},
		{"--directory", directory, "--archives", archives, "--tag", testTag, "--revision", strings.ToUpper(testRevision)},
	}
	for _, arguments := range tests {
		if _, err := parseOptions(arguments); err == nil {
			t.Fatalf("parseOptions(%q) succeeded", arguments)
		}
	}
}

func TestCollectEvidenceRejectsUncleanAndSymlinkRoots(t *testing.T) {
	realDirectory := t.TempDir()
	archives := t.TempDir()
	writeValidEvidenceSet(t, realDirectory, archives)
	unclean := realDirectory + string(os.PathSeparator) + "."
	if _, err := collectEvidence(options{directory: unclean, archives: archives, tag: testTag, revision: testRevision}); err == nil {
		t.Fatal("unclean directory was accepted")
	}
	if _, err := collectEvidence(options{directory: realDirectory, archives: "relative", tag: testTag, revision: testRevision}); err == nil {
		t.Fatal("relative archive directory was accepted")
	}
	link := filepath.Join(t.TempDir(), "evidence")
	if err := os.Symlink(realDirectory, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := collectEvidence(options{directory: link, archives: archives, tag: testTag, revision: testRevision}); err == nil {
		t.Fatal("symlink root was accepted")
	}
}

func TestCollectEvidenceRejectsInvalidFileSets(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		directory := t.TempDir()
		archives := t.TempDir()
		writeValidEvidenceSet(t, directory, archives)
		if err := os.Remove(filepath.Join(directory, targetContracts[0].fileName)); err != nil {
			t.Fatal(err)
		}
		assertCollectFails(t, directory, archives)
	})
	t.Run("extra", func(t *testing.T) {
		directory := t.TempDir()
		archives := t.TempDir()
		writeValidEvidenceSet(t, directory, archives)
		if err := os.WriteFile(filepath.Join(directory, "extra.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		assertCollectFails(t, directory, archives)
	})
	t.Run("directory", func(t *testing.T) {
		directory := t.TempDir()
		archives := t.TempDir()
		writeValidEvidenceSet(t, directory, archives)
		name := filepath.Join(directory, targetContracts[0].fileName)
		if err := os.Remove(name); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(name, 0o700); err != nil {
			t.Fatal(err)
		}
		assertCollectFails(t, directory, archives)
	})
	t.Run("symlink", func(t *testing.T) {
		directory := t.TempDir()
		archives := t.TempDir()
		writeValidEvidenceSet(t, directory, archives)
		name := filepath.Join(directory, targetContracts[0].fileName)
		target := filepath.Join(t.TempDir(), "replacement")
		value, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(name); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, value, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, name); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		assertCollectFails(t, directory, archives)
	})
	t.Run("oversized", func(t *testing.T) {
		directory := t.TempDir()
		archives := t.TempDir()
		writeValidEvidenceSet(t, directory, archives)
		if err := os.WriteFile(filepath.Join(directory, targetContracts[0].fileName), bytes.Repeat([]byte("x"), maxEvidenceFileBytes+1), 0o600); err != nil {
			t.Fatal(err)
		}
		assertCollectFails(t, directory, archives)
	})
}

func TestCollectEvidenceRejectsInvalidArchiveSets(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		directory := t.TempDir()
		archives := t.TempDir()
		writeValidEvidenceSet(t, directory, archives)
		if err := os.Remove(filepath.Join(archives, targetContracts[0].archiveName(testTag))); err != nil {
			t.Fatal(err)
		}
		assertCollectFails(t, directory, archives)
	})
	t.Run("extra", func(t *testing.T) {
		directory := t.TempDir()
		archives := t.TempDir()
		writeValidEvidenceSet(t, directory, archives)
		if err := os.WriteFile(filepath.Join(archives, "checksums.txt"), []byte("extra"), 0o600); err != nil {
			t.Fatal(err)
		}
		assertCollectFails(t, directory, archives)
	})
	t.Run("symlink", func(t *testing.T) {
		directory := t.TempDir()
		archives := t.TempDir()
		writeValidEvidenceSet(t, directory, archives)
		name := filepath.Join(archives, targetContracts[0].archiveName(testTag))
		target := filepath.Join(t.TempDir(), "archive")
		value, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(name); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, value, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, name); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		assertCollectFails(t, directory, archives)
	})
	t.Run("oversized", func(t *testing.T) {
		directory := t.TempDir()
		archives := t.TempDir()
		writeValidEvidenceSet(t, directory, archives)
		name := filepath.Join(archives, targetContracts[0].archiveName(testTag))
		if err := os.Truncate(name, maxArchiveBytes+1); err != nil {
			t.Fatal(err)
		}
		assertCollectFails(t, directory, archives)
	})
	t.Run("digest mismatch", func(t *testing.T) {
		directory := t.TempDir()
		archives := t.TempDir()
		writeValidEvidenceSet(t, directory, archives)
		name := filepath.Join(archives, targetContracts[0].archiveName(testTag))
		if err := os.WriteFile(name, []byte("changed archive"), 0o600); err != nil {
			t.Fatal(err)
		}
		assertCollectFails(t, directory, archives)
	})
}

func TestDecodeEvidenceRejectsUnknownDuplicateAndTrailingJSON(t *testing.T) {
	valid := marshalEvidence(t, validEvidence("linux/amd64", targetContracts[0].archiveName(testTag), strings.Repeat("0", digestLength)))
	unknown := bytes.Replace(valid, []byte(`"target":`), []byte(`"unknown":true,"target":`), 1)
	duplicate := bytes.Replace(valid, []byte(`"target":"linux/amd64"`), []byte(`"target":"linux/amd64","target":"linux/amd64"`), 1)
	for _, value := range [][]byte{
		unknown,
		duplicate,
		append(append([]byte{}, valid...), []byte(` {}`)...),
		[]byte(`{"schema_version":8,"artifact_journey":`),
	} {
		if _, err := decodeEvidence(value); err == nil {
			t.Fatalf("invalid JSON was accepted: %s", value)
		}
	}
}

func TestValidateEvidenceRejectsInvalidContracts(t *testing.T) {
	archiveName := targetContracts[0].archiveName(testTag)
	base := validEvidence("linux/amd64", archiveName, strings.Repeat("0", digestLength))
	tests := []struct {
		name   string
		mutate func(*evidenceDocument)
	}{
		{"schema 7", func(value *evidenceDocument) { value.SchemaVersion = 7 }},
		{"target", func(value *evidenceDocument) { value.ArtifactJourney.Target = "linux/arm64" }},
		{"observed host", func(value *evidenceDocument) { value.ArtifactJourney.ObservedHost = "linux/arm64" }},
		{"archive name", func(value *evidenceDocument) { value.ArtifactJourney.ArchiveName = "other.tar.gz" }},
		{"version", func(value *evidenceDocument) { value.ArtifactJourney.Version = "1.2.4" }},
		{"revision", func(value *evidenceDocument) { value.ArtifactJourney.Revision = strings.Repeat("f", revisionLength) }},
		{"archive digest", func(value *evidenceDocument) { value.ArtifactJourney.ArchiveSHA256 = strings.Repeat("G", digestLength) }},
		{"bundle digest", func(value *evidenceDocument) { value.ArtifactJourney.BundleDigest = strings.Repeat("x", digestLength) }},
		{"plan digest", func(value *evidenceDocument) { value.ArtifactJourney.PlanDigest = "abc" }},
		{"issue bundle digest", func(value *evidenceDocument) {
			value.ArtifactJourney.IssueBundleDigest = strings.Repeat("X", digestLength)
		}},
		{"direct issue bundle reuses multi bundle", func(value *evidenceDocument) {
			value.ArtifactJourney.IssueBundleDigest = value.ArtifactJourney.BundleDigest
		}},
		{"issue plan digest", func(value *evidenceDocument) { value.ArtifactJourney.IssuePlanDigest = "abc" }},
		{"distinct plan relationship", func(value *evidenceDocument) {
			value.ArtifactJourney.IssuePlanDigest = value.ArtifactJourney.PlanDigest
		}},
		{"wrapper outcome", func(value *evidenceDocument) { value.ArtifactJourney.WrapperOutcome = "other" }},
		{"wrapper cases absent", func(value *evidenceDocument) { value.ArtifactJourney.WrapperCases = nil }},
		{"wrapper case missing", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases = value.ArtifactJourney.WrapperCases[:3]
		}},
		{"wrapper case order", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[0], value.ArtifactJourney.WrapperCases[1] = value.ArtifactJourney.WrapperCases[1], value.ArtifactJourney.WrapperCases[0]
		}},
		{"wrapper case name", func(value *evidenceDocument) { value.ArtifactJourney.WrapperCases[1].Name = "other" }},
		{"wrapper case kind", func(value *evidenceDocument) { value.ArtifactJourney.WrapperCases[2].WrapperKind = "identity" }},
		{"wrapper case mode", func(value *evidenceDocument) { value.ArtifactJourney.WrapperCases[3].ResultMode = "transformed_json" }},
		{"default-applied caller argv", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[0].CallerArgv = []string{"pr", "list", "--limit=30"}
		}},
		{"default-overridden caller argv", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[1].CallerArgv = []string{"pr", "list"}
		}},
		{"append caller argv", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[2].CallerArgv = append(value.ArtifactJourney.WrapperCases[2].CallerArgv, "--limit=1")
		}},
		{"identity caller argv", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[3].CallerArgv = nil
		}},
		{"default-applied source argv", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[0].SourceArgv[2] = "--limit=2"
		}},
		{"default-overridden source argv", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[1].SourceArgv[2] = "--limit=30"
		}},
		{"append source argv", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[2].SourceArgv = value.ArtifactJourney.WrapperCases[2].SourceArgv[:5]
		}},
		{"identity source argv", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[3].SourceArgv = nil
		}},
		{"declared option defaults absent", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[0].OptionDefaults = nil
		}},
		{"declared option default value", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[1].OptionDefaults[0].Value = "2"
		}},
		{"applied option defaults absent", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[0].AppliedOptionDefaults = nil
		}},
		{"overridden default recorded as applied", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[1].AppliedOptionDefaults = append([]tailoringbundle.OptionDefault{}, wantedOptionDefaults...)
		}},
		{"append defaults not explicit", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[2].OptionDefaults = nil
		}},
		{"identity applied defaults not explicit", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[3].AppliedOptionDefaults = nil
		}},
		{"wrapper bundle digest", func(value *evidenceDocument) { value.ArtifactJourney.WrapperCases[0].BundleDigest = "abc" }},
		{"wrapper plan digest", func(value *evidenceDocument) { value.ArtifactJourney.WrapperCases[0].PlanDigest = "abc" }},
		{"wrapper source digest", func(value *evidenceDocument) { value.ArtifactJourney.WrapperCases[0].WrapperSourceSHA256 = "abc" }},
		{"shared case bundle binding", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[1].BundleDigest = strings.Repeat("f", digestLength)
		}},
		{"append plan reuses direct issue plan", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[2].PlanDigest = value.ArtifactJourney.IssuePlanDigest
		}},
		{"shared wrapper source", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[1].WrapperSourceSHA256 = strings.Repeat("f", digestLength)
		}},
		{"duplicate shared plans", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[1].PlanDigest = value.ArtifactJourney.WrapperCases[0].PlanDigest
		}},
		{"identity reuses shared bundle", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[3].BundleDigest = value.ArtifactJourney.WrapperCases[0].BundleDigest
		}},
		{"identity reuses direct issue bundle", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[3].BundleDigest = value.ArtifactJourney.IssueBundleDigest
		}},
		{"identity reuses transformed plan", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[3].PlanDigest = value.ArtifactJourney.WrapperCases[0].PlanDigest
		}},
		{"identity reuses append plan", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[3].PlanDigest = value.ArtifactJourney.WrapperCases[2].PlanDigest
		}},
		{"identity reuses direct issue plan", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[3].PlanDigest = value.ArtifactJourney.IssuePlanDigest
		}},
		{"identity reuses shared wrapper", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[3].WrapperSourceSHA256 = value.ArtifactJourney.WrapperCases[0].WrapperSourceSHA256
		}},
		{"wrapper stdout digest", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[2].StdoutSHA256 = strings.Repeat("f", digestLength)
		}},
		{"transformed wrapper stdout digest", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[0].StdoutSHA256 = strings.Repeat("f", digestLength)
		}},
		{"wrapper stderr digest", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[3].StderrSHA256 = strings.Repeat("f", digestLength)
		}},
		{"wrapper source status", func(value *evidenceDocument) { value.ArtifactJourney.WrapperCases[3].SourceExitCode = 1 }},
		{"wrapper case attempts", func(value *evidenceDocument) { value.ArtifactJourney.WrapperCases[0].SourceProcessAttempts++ }},
		{"wrapper source attempts", func(value *evidenceDocument) { value.ArtifactJourney.WrapperSourceAttempts++ }},
		{"tailored help outcome", func(value *evidenceDocument) { value.ArtifactJourney.TailoredHelp.Outcome = "other" }},
		{"tailored help bundle binding", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.BundleDigest = strings.Repeat("f", digestLength)
		}},
		{"tailored help wrapper binding", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.WrapperSourceSHA256 = strings.Repeat("f", digestLength)
		}},
		{"tailored help empty linked wrapper binding", func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases[0].WrapperSourceSHA256 = emptySHA256
			value.ArtifactJourney.TailoredHelp.WrapperSourceSHA256 = emptySHA256
		}},
		{"tailored help contract", func(value *evidenceDocument) { value.ArtifactJourney.TailoredHelp.WrapperContractVersion++ }},
		{"tailored help views absent", func(value *evidenceDocument) { value.ArtifactJourney.TailoredHelp.Views = nil }},
		{"tailored help view missing", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.Views = value.ArtifactJourney.TailoredHelp.Views[:2]
		}},
		{"tailored help view order", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.Views[0], value.ArtifactJourney.TailoredHelp.Views[1] =
				value.ArtifactJourney.TailoredHelp.Views[1], value.ArtifactJourney.TailoredHelp.Views[0]
		}},
		{"tailored help view argv", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.Views[1].Argv = []string{"pr", "list", "--help"}
		}},
		{"tailored help view name", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.Views[2].Name = "exact_command"
		}},
		{"tailored help stdout digest", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.Views[0].StdoutSHA256 = strings.Repeat("f", digestLength)
		}},
		{"tailored help stderr digest", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.Views[0].StderrSHA256 = strings.Repeat("f", digestLength)
		}},
		{"tailored help runtime proof", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.RuntimeNonExecutableDuringSuccess = false
		}},
		{"tailored help source attempts", func(value *evidenceDocument) { value.ArtifactJourney.TailoredHelp.SourceProcessAttempts++ }},
		{"tailored help processor attempts", func(value *evidenceDocument) { value.ArtifactJourney.TailoredHelp.ProcessorProcessAttempts++ }},
		{"tailored help faults absent", func(value *evidenceDocument) { value.ArtifactJourney.TailoredHelp.FallthroughFaults = nil }},
		{"tailored help fault missing", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.FallthroughFaults = value.ArtifactJourney.TailoredHelp.FallthroughFaults[:1]
		}},
		{"tailored help fault code", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.FallthroughFaults[0].Code = "invalid_invocation"
		}},
		{"tailored help fault order", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.FallthroughFaults[0], value.ArtifactJourney.TailoredHelp.FallthroughFaults[1] =
				value.ArtifactJourney.TailoredHelp.FallthroughFaults[1], value.ArtifactJourney.TailoredHelp.FallthroughFaults[0]
		}},
		{"tailored help hidden fault argv", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.FallthroughFaults[0].Argv = []string{"issue", "list", "--help"}
		}},
		{"tailored help unknown fault argv", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.FallthroughFaults[1].Argv = []string{"api", "--help"}
		}},
		{"tailored help fault source attempts", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.FallthroughFaults[0].SourceProcessAttempts++
		}},
		{"tailored help fault processor attempts", func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.FallthroughFaults[0].ProcessorProcessAttempts++
		}},
		{"help count", func(value *evidenceDocument) { value.ArtifactJourney.HelpContractsVerified-- }},
		{"inspection count", func(value *evidenceDocument) { value.ArtifactJourney.SourceInspectionAttempts-- }},
		{"rejection count", func(value *evidenceDocument) { value.ArtifactJourney.ZeroAttemptRejections-- }},
		{"attempt count", func(value *evidenceDocument) { value.ArtifactJourney.FixtureAttempts-- }},
		{"fault order", func(value *evidenceDocument) {
			value.ArtifactJourney.PostStartFaults[0], value.ArtifactJourney.PostStartFaults[1] = value.ArtifactJourney.PostStartFaults[1], value.ArtifactJourney.PostStartFaults[0]
		}},
		{"command order", func(value *evidenceDocument) {
			value.ArtifactJourney.CommandsVerified[0], value.ArtifactJourney.CommandsVerified[1] = value.ArtifactJourney.CommandsVerified[1], value.ArtifactJourney.CommandsVerified[0]
		}},
		{"credential assertion", func(value *evidenceDocument) { value.ArtifactJourney.CredentialEnvironmentAbsent = false }},
		{"canary assertion", func(value *evidenceDocument) { value.ArtifactJourney.SecretCanariesAbsent = false }},
		{"Go adapter", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.AdapterKind = "atsura.source.other" }},
		{"Go contract", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.AdapterContractVersion++ }},
		{"Go version", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.SourceVersion = "go1.27.0" }},
		{"Go catalog digest", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.CatalogDigest = "abc" }},
		{"Go inspection count", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.SourceInspectionAttempts-- }},
		{"Go command", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.CommandsVerified[0] = "build" }},
		{"Go bundle digest", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.BundleDigest = "abc" }},
		{"Go plan digest", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.PlanDigest = "abc" }},
		{"Go wrapper outcome", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.WrapperOutcome = "other" }},
		{"Go wrapper case", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.WrapperCases[0].Name = "other" }},
		{"Go wrapper kind", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.WrapperCases[0].WrapperKind = "transform"
		}},
		{"Go wrapper result mode", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.WrapperCases[0].ResultMode = "original_preserving_optimizer"
		}},
		{"Go wrapper caller argv", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.WrapperCases[0].CallerArgv = []string{"test", "extra"}
		}},
		{"Go wrapper source argv", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.WrapperCases[0].SourceArgv = []string{"test", "extra"}
		}},
		{"Go wrapper defaults absent", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.WrapperCases[0].OptionDefaults = nil
		}},
		{"Go wrapper applied defaults absent", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.WrapperCases[0].AppliedOptionDefaults = nil
		}},
		{"Go wrapper bundle binding", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.WrapperCases[0].BundleDigest = strings.Repeat("f", digestLength)
		}},
		{"Go wrapper plan binding", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.WrapperCases[0].PlanDigest = strings.Repeat("f", digestLength)
		}},
		{"Go wrapper stdout", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.WrapperCases[0].StdoutSHA256 = emptySHA256
		}},
		{"Go wrapper stderr", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.WrapperCases[0].StderrSHA256 = strings.Repeat("f", digestLength)
		}},
		{"Go wrapper attempts", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.WrapperSourceAttempts++ }},
		{"Go zero-attempt count", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.ZeroAttemptRejections = 0 }},
		{"Go empty wrapper source", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.WrapperCases[0].WrapperSourceSHA256 = emptySHA256
		}},
		{"Go wrapper status", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.WrapperCases[0].SourceExitCode = 1 }},
		{"Go wrapper case attempts", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.WrapperCases[0].SourceProcessAttempts++
		}},
		{"optimizer outcome", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.Outcome = "other" }},
		{"optimizer processor absent", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.Processor = nil }},
		{"optimizer processor contract", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Processor.ContractID = "atsura.output.other.v1"
		}},
		{"optimizer processor adapter", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Processor.AdapterKind = "atsura.processor.other"
		}},
		{"optimizer processor adapter contract", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Processor.AdapterContractVersion++
		}},
		{"optimizer processor version", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Processor.Version = "0.43.1"
		}},
		{"optimizer processor target", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Processor.Target = "linux/arm64"
		}},
		{"optimizer processor archive", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Processor.ArchiveName = "rtk.tar.gz"
		}},
		{"optimizer processor archive digest", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Processor.ArchiveSHA256 = strings.Repeat("0", digestLength)
		}},
		{"optimizer processor binary digest", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Processor.BinarySHA256 = strings.Repeat("0", digestLength)
		}},
		{"optimizer processor binary size", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Processor.BinarySize++
		}},
		{"optimizer observation digest", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Processor.ObservationDigest = "abc"
		}},
		{"optimizer inspection attempts", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Processor.InspectionProcessAttempts++
		}},
		{"optimizer execution absent", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.Execution = nil }},
		{"optimizer caller argv", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.CallerArgv = []string{"test", "extra"}
		}},
		{"optimizer source argv", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.SourceArgv = []string{"test"}
		}},
		{"optimizer source stdin", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.SourceStdinMode = "inherit"
		}},
		{"optimizer source cwd", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.SourceWorkingDirectoryMode = "isolated"
		}},
		{"optimizer source environment", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.SourceEnvironmentMode = "isolated"
		}},
		{"optimizer source max attempts", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.Execution.SourceMaxAttempts++ }},
		{"optimizer source timeout", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.SourceTimeoutMillis++
		}},
		{"optimizer source stdout bound", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.SourceStdoutLimitBytes--
		}},
		{"optimizer source stderr bound", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.SourceStderrLimitBytes--
		}},
		{"optimizer input format", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.InputFormat = "other"
		}},
		{"optimizer output format", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.OutputFormat = "other"
		}},
		{"optimizer original output", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.AllowOriginalOutput = false
		}},
		{"optimizer processor argv", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.ProcessorArgv = []string{"pipe"}
		}},
		{"optimizer processor stdin", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.ProcessorStdinMode = "closed"
		}},
		{"optimizer processor cwd", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.ProcessorWorkingDirectoryMode = "inherit"
		}},
		{"optimizer processor environment", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.ProcessorEnvironmentContract = "atsura.processor.rtk_isolated.v1"
		}},
		{"optimizer processor max attempts", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.ProcessorMaxAttempts++
		}},
		{"optimizer processor timeout", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.ProcessorTimeoutMillis++
		}},
		{"optimizer processor stdout bound", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.ProcessorStdoutLimitBytes--
		}},
		{"optimizer processor stderr bound", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution.ProcessorStderrLimitBytes--
		}},
		{"optimizer bundle digest", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.BundleDigest = "abc" }},
		{"optimizer plan digest", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.PlanDigest = "abc" }},
		{"optimizer wrapper digest", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.WrapperSourceSHA256 = "abc" }},
		{"optimizer cases absent", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.Cases = nil }},
		{"optimizer case missing", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Cases = value.ArtifactJourney.GoSource.Optimizer.Cases[:3]
		}},
		{"optimizer case order", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Cases[0], value.ArtifactJourney.GoSource.Optimizer.Cases[1] =
				value.ArtifactJourney.GoSource.Optimizer.Cases[1], value.ArtifactJourney.GoSource.Optimizer.Cases[0]
		}},
		{"optimizer preserved after", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Cases[1].Disposition = "preserved_after_processor"
		}},
		{"optimizer summary digest", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Cases[0].StdoutSHA256 = strings.Repeat("f", digestLength)
		}},
		{"optimizer duplicate preserved digest", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Cases[2].StdoutSHA256 = value.ArtifactJourney.GoSource.Optimizer.Cases[1].StdoutSHA256
		}},
		{"optimizer case stderr", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Cases[1].StderrSHA256 = strings.Repeat("f", digestLength)
		}},
		{"optimizer case status", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.Cases[2].SourceExitCode = 0 }},
		{"optimizer case attempts", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Cases[0].SourceProcessAttempts++
		}},
		{"optimizer faults absent", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.Faults = nil }},
		{"optimizer fault missing", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Faults = value.ArtifactJourney.GoSource.Optimizer.Faults[:2]
		}},
		{"optimizer fault order", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Faults[0], value.ArtifactJourney.GoSource.Optimizer.Faults[1] =
				value.ArtifactJourney.GoSource.Optimizer.Faults[1], value.ArtifactJourney.GoSource.Optimizer.Faults[0]
		}},
		{"optimizer arbitrary fault", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Faults[0].Code = "processor_process_start_failed_after_source"
		}},
		{"optimizer fault attempts", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Faults[0].SourceProcessAttempts++
		}},
		{"optimizer source attempts", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.SourceProcessAttempts-- }},
		{"optimizer zero-attempt count", func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.ZeroAttemptRejections-- }},
		{"optimizer reuses identity bundle", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.BundleDigest = value.ArtifactJourney.GoSource.BundleDigest
		}},
		{"optimizer reuses identity plan", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.PlanDigest = value.ArtifactJourney.GoSource.PlanDigest
		}},
		{"optimizer reuses identity wrapper", func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.WrapperSourceSHA256 = value.ArtifactJourney.GoSource.WrapperCases[0].WrapperSourceSHA256
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := cloneEvidence(t, base)
			test.mutate(&value)
			if err := validateEvidence(value, "linux/amd64", archiveName, testVersion, testRevision); err == nil {
				t.Fatal("invalid evidence was accepted")
			}
		})
	}
}

func TestExpectedTailoredHelpOutputDigestsAreExact(t *testing.T) {
	digest := strings.Repeat("a", digestLength)
	tests := []struct {
		name string
		want string
	}{
		{name: "root", want: "da2d149caceb26edd5e22b6e1a2697e8a72f7fd815855a7ead20d81ece012bf6"},
		{name: "issue_namespace", want: "f47dd2952fd2bef65ef1c197e54366bfe9625979e3daeea1a7d35a318e12f76a"},
		{name: "issue_exact_command", want: "9faf4dca4089df3330aae6ab5b348115a8bd9eb8b423f0fcd685cde18282cab5"},
		{name: "pr_namespace", want: "48b1a4ba341fe7c3830bb8aa97bebd89f2ff76d0a90265074bd9ab220d33d998"},
		{name: "pr_exact_command", want: "f2d0d86d175087e82792332cf2f973642a0bf84978f53dbe06d11138c1003fbf"},
	}
	for _, test := range tests {
		output, err := expectedTailoredHelpOutput(digest, test.name)
		if err != nil || digestEvidenceBytes(output) != test.want {
			t.Fatalf("tailored help %s digest=%s err=%v", test.name, digestEvidenceBytes(output), err)
		}
	}
}

func TestValidateEvidenceRequiresStructuredUnsupportedWindowsWrapperCases(t *testing.T) {
	contract := targetContracts[len(targetContracts)-1]
	archiveName := contract.archiveName(testTag)
	base := validEvidence(contract.target, archiveName, strings.Repeat("0", digestLength))
	if err := validateEvidence(base, contract.target, archiveName, testVersion, testRevision); err != nil {
		t.Fatal(err)
	}
	mutations := []func(*evidenceDocument){
		func(value *evidenceDocument) { value.ArtifactJourney.WrapperCases = nil },
		func(value *evidenceDocument) {
			value.ArtifactJourney.WrapperCases = []wrapperCaseEvidence{{Name: "identity"}}
		},
		func(value *evidenceDocument) { value.ArtifactJourney.WrapperOutcome = "ordinary_command_verified" },
		func(value *evidenceDocument) { value.ArtifactJourney.WrapperSourceAttempts = 1 },
		func(value *evidenceDocument) { value.ArtifactJourney.FixtureAttempts = wantedPOSIXAttempts },
		func(value *evidenceDocument) { value.ArtifactJourney.TailoredHelp.Outcome = "compiled_views_verified" },
		func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.BundleDigest = strings.Repeat("a", digestLength)
		},
		func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.WrapperSourceSHA256 = strings.Repeat("b", digestLength)
		},
		func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.WrapperContractVersion = wrapperbinding.ContractVersion
		},
		func(value *evidenceDocument) { value.ArtifactJourney.TailoredHelp.Views = nil },
		func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.Views = []tailoredHelpViewEvidence{{Name: "root"}}
		},
		func(value *evidenceDocument) { value.ArtifactJourney.TailoredHelp.FallthroughFaults = nil },
		func(value *evidenceDocument) {
			value.ArtifactJourney.TailoredHelp.RuntimeNonExecutableDuringSuccess = true
		},
		func(value *evidenceDocument) { value.ArtifactJourney.TailoredHelp.SourceProcessAttempts = 1 },
		func(value *evidenceDocument) { value.ArtifactJourney.TailoredHelp.ProcessorProcessAttempts = 1 },
		func(value *evidenceDocument) { value.ArtifactJourney.GoSource.WrapperCases = nil },
		func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.WrapperOutcome = "ordinary_command_verified"
		},
		func(value *evidenceDocument) { value.ArtifactJourney.GoSource.WrapperSourceAttempts = 1 },
		func(value *evidenceDocument) { value.ArtifactJourney.GoSource.ZeroAttemptRejections = 0 },
		func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Outcome = "reachable_outcomes_verified"
		},
		func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Processor = validProcessorEvidence("linux/amd64")
		},
		func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.Execution = validOptimizerExecution()
		},
		func(value *evidenceDocument) {
			value.ArtifactJourney.GoSource.Optimizer.BundleDigest = strings.Repeat("a", digestLength)
		},
		func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.Cases = nil },
		func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.Faults = nil },
		func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.SourceProcessAttempts = 1 },
		func(value *evidenceDocument) { value.ArtifactJourney.GoSource.Optimizer.ZeroAttemptRejections = 0 },
	}
	for index, mutate := range mutations {
		value := cloneEvidence(t, base)
		mutate(&value)
		if err := validateEvidence(value, contract.target, archiveName, testVersion, testRevision); err == nil {
			t.Fatalf("invalid Windows wrapper evidence %d was accepted", index)
		}
	}
}

func TestRunRejectsShortAndFailedWriters(t *testing.T) {
	directory := t.TempDir()
	archives := t.TempDir()
	writeValidEvidenceSet(t, directory, archives)
	for _, writer := range []io.Writer{shortWriter{}, errorWriter{}} {
		if err := run(validArguments(directory, archives), writer); err == nil {
			t.Fatalf("writer %T was accepted", writer)
		}
	}
}

func writeValidEvidenceSet(t *testing.T, directory, archives string) []string {
	t.Helper()
	digests := make([]string, 0, len(targetContracts))
	for index, contract := range targetContracts {
		archiveName := contract.archiveName(testTag)
		archive := []byte(fmt.Sprintf("archive-%d", index+1))
		digest := fmt.Sprintf("%x", sha256.Sum256(archive))
		if err := os.WriteFile(filepath.Join(archives, archiveName), archive, 0o600); err != nil {
			t.Fatal(err)
		}
		value := marshalEvidence(t, validEvidence(contract.target, archiveName, digest))
		if err := os.WriteFile(filepath.Join(directory, contract.fileName), value, 0o600); err != nil {
			t.Fatal(err)
		}
		digests = append(digests, digest)
	}
	return digests
}

func validEvidence(target, archiveName, archiveDigest string) evidenceDocument {
	result := evidenceDocument{
		SchemaVersion: 8,
		ArtifactJourney: artifactJourneyEvidence{
			Target: target, ObservedHost: target, ArchiveName: archiveName, ArchiveSHA256: archiveDigest,
			Version: testVersion, Revision: testRevision,
			HelpContractsVerified: wantedHelpContracts,
			CommandsVerified:      append([]string{}, wantedCommands...),
			BundleDigest:          strings.Repeat("a", digestLength), PlanDigest: strings.Repeat("b", digestLength),
			IssueBundleDigest: strings.Repeat("c", digestLength), IssuePlanDigest: strings.Repeat("d", digestLength),
			SourceInspectionAttempts: wantedInspections, ZeroAttemptRejections: wantedRejections,
			PostStartFaults:             append([]string{}, wantedFaults...),
			CredentialEnvironmentAbsent: true, SecretCanariesAbsent: true,
			GoSource: goSourceEvidence{
				AdapterKind: goAdapterKind, AdapterContractVersion: goAdapterContractVersion,
				SourceVersion: "go1.26.5", CatalogDigest: strings.Repeat("8", digestLength),
				SourceInspectionAttempts: wantedGoInspections, CommandsVerified: []string{"test"},
				BundleDigest: strings.Repeat("9", digestLength), PlanDigest: strings.Repeat("1", digestLength),
			},
		},
	}
	if target == "windows/amd64" {
		result.ArtifactJourney.WrapperOutcome = "platform_not_supported"
		result.ArtifactJourney.WrapperCases = []wrapperCaseEvidence{}
		result.ArtifactJourney.WrapperSourceAttempts = 0
		result.ArtifactJourney.FixtureAttempts = wantedWindowsAttempts
		result.ArtifactJourney.GoSource.WrapperOutcome = "platform_not_supported"
		result.ArtifactJourney.GoSource.WrapperCases = []wrapperCaseEvidence{}
		result.ArtifactJourney.GoSource.WrapperSourceAttempts = 0
		result.ArtifactJourney.GoSource.ZeroAttemptRejections = 1
		result.ArtifactJourney.GoSource.Optimizer = goOptimizerEvidence{
			Outcome: "platform_not_supported", Cases: []optimizerCaseEvidence{}, Faults: []optimizerFaultEvidence{},
			SourceProcessAttempts: 0, ZeroAttemptRejections: 1,
		}
		result.ArtifactJourney.TailoredHelp = tailoredHelpEvidence{
			Outcome: "platform_not_supported", Views: []tailoredHelpViewEvidence{},
			FallthroughFaults: []tailoredHelpFaultEvidence{},
		}
	} else {
		result.ArtifactJourney.WrapperOutcome = "ordinary_command_verified"
		result.ArtifactJourney.WrapperCases = []wrapperCaseEvidence{
			{
				Name: "default_applied", WrapperKind: "transform", ResultMode: "transformed_json",
				CallerArgv: append([]string{}, wantedDefaultAppliedCallerArgv...), SourceArgv: append([]string{}, wantedDefaultAppliedSourceArgv...),
				OptionDefaults:        append([]tailoringbundle.OptionDefault{}, wantedOptionDefaults...),
				AppliedOptionDefaults: append([]tailoringbundle.OptionDefault{}, wantedOptionDefaults...),
				BundleDigest:          strings.Repeat("a", digestLength), PlanDigest: strings.Repeat("b", digestLength),
				WrapperSourceSHA256: strings.Repeat("e", digestLength), StdoutSHA256: transformedStdoutSHA256, StderrSHA256: emptySHA256,
				SourceExitCode: 0, SourceProcessAttempts: 1,
			},
			{
				Name: "default_overridden", WrapperKind: "transform", ResultMode: "transformed_json",
				CallerArgv: append([]string{}, wantedDefaultOverriddenCallerArgv...), SourceArgv: append([]string{}, wantedDefaultOverriddenSourceArgv...),
				OptionDefaults: append([]tailoringbundle.OptionDefault{}, wantedOptionDefaults...), AppliedOptionDefaults: []tailoringbundle.OptionDefault{},
				BundleDigest: strings.Repeat("a", digestLength), PlanDigest: strings.Repeat("6", digestLength),
				WrapperSourceSHA256: strings.Repeat("e", digestLength), StdoutSHA256: transformedStdoutSHA256, StderrSHA256: emptySHA256,
				SourceExitCode: 0, SourceProcessAttempts: 1,
			},
			{
				Name: "append_only", WrapperKind: "transform", ResultMode: "source_stream_passthrough",
				CallerArgv: append([]string{}, wantedAppendOnlyCallerArgv...), SourceArgv: append([]string{}, wantedAppendOnlySourceArgv...),
				OptionDefaults: []tailoringbundle.OptionDefault{}, AppliedOptionDefaults: []tailoringbundle.OptionDefault{},
				BundleDigest: strings.Repeat("a", digestLength), PlanDigest: strings.Repeat("7", digestLength),
				WrapperSourceSHA256: strings.Repeat("e", digestLength), StdoutSHA256: appendStdoutSHA256, StderrSHA256: appendStderrSHA256,
				SourceExitCode: 23, SourceProcessAttempts: 1,
			},
			{
				Name: "identity", WrapperKind: "identity", ResultMode: "source_stream_passthrough",
				CallerArgv: append([]string{}, wantedIdentityCallerArgv...), SourceArgv: append([]string{}, wantedIdentitySourceArgv...),
				OptionDefaults: []tailoringbundle.OptionDefault{}, AppliedOptionDefaults: []tailoringbundle.OptionDefault{},
				BundleDigest: strings.Repeat("2", digestLength), PlanDigest: strings.Repeat("3", digestLength),
				WrapperSourceSHA256: strings.Repeat("4", digestLength), StdoutSHA256: identityStdoutSHA256, StderrSHA256: identityStderrSHA256,
				SourceExitCode: 0, SourceProcessAttempts: 1,
			},
		}
		result.ArtifactJourney.WrapperSourceAttempts = wantedPOSIXWrappers
		result.ArtifactJourney.FixtureAttempts = wantedPOSIXAttempts
		result.ArtifactJourney.TailoredHelp = validTailoredHelpEvidence(result.ArtifactJourney)
		identityBundleDigest := result.ArtifactJourney.GoSource.BundleDigest
		identityPlanDigest := result.ArtifactJourney.GoSource.PlanDigest
		identityWrapperDigest := strings.Repeat("2", digestLength)
		optimizerBundleDigest := strings.Repeat("3", digestLength)
		optimizerPlanDigest := strings.Repeat("4", digestLength)
		optimizerWrapperDigest := strings.Repeat("5", digestLength)
		caseValues := []optimizerCaseEvidence{
			{Name: "optimized_pass", Disposition: "optimized", StdoutSHA256: optimizedGoStdoutSHA256, StderrSHA256: emptySHA256, SourceExitCode: 0, SourceProcessAttempts: 1},
			{Name: "preserved_before_skip", Disposition: "preserved_before_processor", StdoutSHA256: strings.Repeat("6", digestLength), StderrSHA256: emptySHA256, SourceExitCode: 0, SourceProcessAttempts: 1},
			{Name: "preserved_before_fail", Disposition: "preserved_before_processor", StdoutSHA256: strings.Repeat("7", digestLength), StderrSHA256: emptySHA256, SourceExitCode: 1, SourceProcessAttempts: 1},
			{Name: "preserved_before_ineligible", Disposition: "preserved_before_processor", StdoutSHA256: strings.Repeat("8", digestLength), StderrSHA256: emptySHA256, SourceExitCode: 0, SourceProcessAttempts: 1},
		}
		result.ArtifactJourney.GoSource.WrapperOutcome = "ordinary_command_verified"
		result.ArtifactJourney.GoSource.WrapperCases = []wrapperCaseEvidence{{
			Name: "go_test_identity", WrapperKind: "identity", ResultMode: "source_stream_passthrough",
			CallerArgv: append([]string{}, wantedGoCallerArgv...), SourceArgv: append([]string{}, wantedGoSourceArgv...),
			OptionDefaults: []tailoringbundle.OptionDefault{}, AppliedOptionDefaults: []tailoringbundle.OptionDefault{},
			BundleDigest: identityBundleDigest, PlanDigest: identityPlanDigest, WrapperSourceSHA256: identityWrapperDigest,
			StdoutSHA256: strings.Repeat("f", digestLength), StderrSHA256: emptySHA256,
			SourceExitCode: 0, SourceProcessAttempts: 1,
		}}
		result.ArtifactJourney.GoSource.WrapperSourceAttempts = wantedGoPOSIXWrappers
		result.ArtifactJourney.GoSource.ZeroAttemptRejections = 1
		result.ArtifactJourney.GoSource.Optimizer = goOptimizerEvidence{
			Outcome: "reachable_outcomes_verified", Processor: validProcessorEvidence(target), Execution: validOptimizerExecution(),
			BundleDigest: optimizerBundleDigest, PlanDigest: optimizerPlanDigest, WrapperSourceSHA256: optimizerWrapperDigest,
			Cases: caseValues,
			Faults: []optimizerFaultEvidence{
				{Name: "projection_rejection", Code: "wrapper_runtime_not_supported", SourceProcessAttempts: 0},
				{Name: "preflight_processor_drift", Code: "processor_identity_changed", SourceProcessAttempts: 0},
				{Name: "post_source_processor_drift", Code: "processor_identity_changed", SourceProcessAttempts: 1},
			},
			SourceProcessAttempts: wantedGoPOSIXAttempts, ZeroAttemptRejections: 2,
		}
	}
	return result
}

func validTailoredHelpEvidence(journey artifactJourneyEvidence) tailoredHelpEvidence {
	views := []tailoredHelpViewEvidence{
		{Name: "root", Argv: []string{"--help"}},
		{Name: "issue_namespace", Argv: []string{"issue", "--help"}},
		{Name: "issue_exact_command", Argv: []string{"issue", "list", "--help"}},
		{Name: "pr_namespace", Argv: []string{"pr", "--help"}},
		{Name: "pr_exact_command", Argv: []string{"pr", "list", "--help"}},
	}
	for index := range views {
		output, err := expectedTailoredHelpOutput(journey.BundleDigest, views[index].Name)
		if err != nil {
			panic(err)
		}
		views[index].StdoutSHA256 = digestEvidenceBytes(output)
		views[index].StderrSHA256 = emptySHA256
	}
	return tailoredHelpEvidence{
		Outcome: "compiled_views_verified", BundleDigest: journey.BundleDigest,
		WrapperSourceSHA256:    journey.WrapperCases[0].WrapperSourceSHA256,
		WrapperContractVersion: wrapperbinding.ContractVersion,
		Views:                  views,
		FallthroughFaults: []tailoredHelpFaultEvidence{
			{Name: "hidden_command", Argv: []string{"api", "--help"}, Code: "command_not_in_surface"},
			{Name: "unknown_selector", Argv: []string{"unknown", "--help"}, Code: "invalid_invocation"},
		},
		RuntimeNonExecutableDuringSuccess: true,
	}
}

func validOptimizerExecution() *optimizerExecutionEvidence {
	return &optimizerExecutionEvidence{
		CallerArgv: []string{"test"}, SourceArgv: []string{"test", "-json"},
		SourceStdinMode: "closed", SourceWorkingDirectoryMode: "inherit", SourceEnvironmentMode: "inherit",
		SourceMaxAttempts: 1, SourceTimeoutMillis: sourceprocess.MaxTimeout.Milliseconds(),
		SourceStdoutLimitBytes: sourceprocess.MaxStdoutBytes, SourceStderrLimitBytes: sourceprocess.MaxStderrBytes,
		InputFormat: "go_test_jsonl", OutputFormat: "go_test_pass_summary", AllowOriginalOutput: true,
		ProcessorArgv: []string{"pipe", "--filter=go-test"}, ProcessorStdinMode: "stage_input",
		ProcessorWorkingDirectoryMode: "isolated", ProcessorEnvironmentContract: processorprocess.EnvironmentRTKIsolatedV2,
		ProcessorMaxAttempts: 1, ProcessorTimeoutMillis: processorprocess.MaxTimeout.Milliseconds(),
		ProcessorStdoutLimitBytes: processorprocess.MaxStdoutBytes, ProcessorStderrLimitBytes: processorprocess.MaxStderrBytes,
	}
}

func validProcessorEvidence(target string) *processorArtifactEvidence {
	metadata, err := processormanifest.PinnedManifest().Target(target)
	if err != nil {
		panic(err)
	}
	return &processorArtifactEvidence{
		ContractID: metadata.ContractID(), AdapterKind: metadata.ProcessorKind(), AdapterContractVersion: rtkAdapterContractVersion,
		Version: metadata.Version(), Target: metadata.Target(), ArchiveName: metadata.ArchiveName(), ArchiveSHA256: metadata.ArchiveSHA256(),
		BinarySHA256: metadata.BinarySHA256(), BinarySize: metadata.BinarySize(), ObservationDigest: strings.Repeat("5", digestLength),
		InspectionProcessAttempts: 1,
	}
}

func marshalEvidence(t *testing.T, value evidenceDocument) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func cloneEvidence(t *testing.T, value evidenceDocument) evidenceDocument {
	t.Helper()
	encoded := marshalEvidence(t, value)
	var result evidenceDocument
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func validArguments(directory, archives string) []string {
	return []string{"--directory", directory, "--archives", archives, "--tag", testTag, "--revision", testRevision}
}

func assertCollectFails(t *testing.T, directory, archives string) {
	t.Helper()
	if _, err := collectEvidence(options{directory: directory, archives: archives, tag: testTag, revision: testRevision}); err == nil {
		t.Fatal("invalid evidence directory was accepted")
	}
}

type shortWriter struct{}

func (shortWriter) Write(value []byte) (int, error) {
	return len(value) - 1, nil
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
