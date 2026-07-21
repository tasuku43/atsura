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
	wanted := fmt.Sprintf(`{"schema_version":1,"tag":"v1.2.3-rc.1","revision":"0123456789abcdef0123456789abcdef01234567","provenance_level":"workflow_index_unattested","targets":[{"target":"linux/amd64","result":"passed","archive_name":"atr_v1.2.3-rc.1_linux_amd64.tar.gz","archive_sha256":"%s"},{"target":"linux/arm64","result":"passed","archive_name":"atr_v1.2.3-rc.1_linux_arm64.tar.gz","archive_sha256":"%s"},{"target":"darwin/amd64","result":"passed","archive_name":"atr_v1.2.3-rc.1_darwin_amd64.tar.gz","archive_sha256":"%s"},{"target":"darwin/arm64","result":"passed","archive_name":"atr_v1.2.3-rc.1_darwin_arm64.tar.gz","archive_sha256":"%s"},{"target":"windows/amd64","result":"passed","archive_name":"atr_v1.2.3-rc.1_windows_amd64.zip","archive_sha256":"%s"}]}`+"\n", digests[0], digests[1], digests[2], digests[3], digests[4])
	if output.String() != wanted {
		t.Fatalf("aggregate mismatch\n got: %s\nwant: %s", output.String(), wanted)
	}
	if strings.Contains(output.String(), directory) || strings.Contains(output.String(), archives) ||
		strings.Contains(output.String(), "bundle_digest") || strings.Contains(output.String(), "plan_digest") ||
		strings.Contains(output.String(), "commands_verified") {
		t.Fatalf("aggregate leaked non-contract evidence: %s", output.String())
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
		[]byte(`{"schema_version":1,"artifact_journey":`),
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
		{"schema", func(value *evidenceDocument) { value.SchemaVersion = 2 }},
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
		{"issue plan digest", func(value *evidenceDocument) { value.ArtifactJourney.IssuePlanDigest = "abc" }},
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
	return evidenceDocument{
		SchemaVersion: 1,
		ArtifactJourney: artifactJourneyEvidence{
			Target: target, ObservedHost: target, ArchiveName: archiveName, ArchiveSHA256: archiveDigest,
			Version: testVersion, Revision: testRevision,
			HelpContractsVerified: wantedHelpContracts,
			CommandsVerified:      append([]string{}, wantedCommands...),
			BundleDigest:          strings.Repeat("a", digestLength), PlanDigest: strings.Repeat("b", digestLength),
			IssueBundleDigest: strings.Repeat("c", digestLength), IssuePlanDigest: strings.Repeat("d", digestLength),
			SourceInspectionAttempts: wantedInspections, ZeroAttemptRejections: wantedRejections,
			PostStartFaults: append([]string{}, wantedFaults...), FixtureAttempts: wantedAttempts,
			CredentialEnvironmentAbsent: true, SecretCanariesAbsent: true,
		},
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
