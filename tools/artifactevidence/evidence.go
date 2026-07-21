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
	"regexp"
	"strings"
)

const (
	revisionLength       = 40
	digestLength         = 64
	maxEvidenceFileBytes = 16 * 1024
	maxArchiveBytes      = int64(256 * 1024 * 1024)
	maxAggregateBytes    = 4 * 1024
	wantedHelpContracts  = 6
	wantedInspections    = 4
	wantedRejections     = 7
	wantedAttempts       = 10
)

var releaseTagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

var wantedFaults = []string{
	"source_command_failed",
	"source_stderr_not_supported",
	"source_json_invalid",
	"output_transform_failed",
}

var wantedCommands = []string{
	"issue list",
	"pr list",
}

type targetContract struct {
	fileName  string
	target    string
	goos      string
	goarch    string
	extension string
}

var targetContracts = []targetContract{
	{fileName: "linux_amd64.json", target: "linux/amd64", goos: "linux", goarch: "amd64", extension: "tar.gz"},
	{fileName: "linux_arm64.json", target: "linux/arm64", goos: "linux", goarch: "arm64", extension: "tar.gz"},
	{fileName: "darwin_amd64.json", target: "darwin/amd64", goos: "darwin", goarch: "amd64", extension: "tar.gz"},
	{fileName: "darwin_arm64.json", target: "darwin/arm64", goos: "darwin", goarch: "arm64", extension: "tar.gz"},
	{fileName: "windows_amd64.json", target: "windows/amd64", goos: "windows", goarch: "amd64", extension: "zip"},
}

func (c targetContract) archiveName(tag string) string {
	return fmt.Sprintf("atr_%s_%s_%s.%s", tag, c.goos, c.goarch, c.extension)
}

type evidenceDocument struct {
	SchemaVersion   int                     `json:"schema_version"`
	ArtifactJourney artifactJourneyEvidence `json:"artifact_journey"`
}

type artifactJourneyEvidence struct {
	Target                      string   `json:"target"`
	ObservedHost                string   `json:"observed_host"`
	ArchiveName                 string   `json:"archive_name"`
	ArchiveSHA256               string   `json:"archive_sha256"`
	Version                     string   `json:"version"`
	Revision                    string   `json:"revision"`
	HelpContractsVerified       int      `json:"help_contracts_verified"`
	CommandsVerified            []string `json:"commands_verified"`
	BundleDigest                string   `json:"bundle_digest"`
	PlanDigest                  string   `json:"plan_digest"`
	IssueBundleDigest           string   `json:"issue_bundle_digest"`
	IssuePlanDigest             string   `json:"issue_plan_digest"`
	SourceInspectionAttempts    int      `json:"source_inspection_attempts"`
	ZeroAttemptRejections       int      `json:"zero_attempt_rejections"`
	PostStartFaults             []string `json:"post_start_faults"`
	FixtureAttempts             int      `json:"fixture_attempts"`
	CredentialEnvironmentAbsent bool     `json:"credential_environment_absent"`
	SecretCanariesAbsent        bool     `json:"secret_canaries_absent"`
}

type aggregateDocument struct {
	SchemaVersion   int               `json:"schema_version"`
	Tag             string            `json:"tag"`
	Revision        string            `json:"revision"`
	ProvenanceLevel string            `json:"provenance_level"`
	Targets         []aggregateTarget `json:"targets"`
}

type aggregateTarget struct {
	Target        string `json:"target"`
	Result        string `json:"result"`
	ArchiveName   string `json:"archive_name"`
	ArchiveSHA256 string `json:"archive_sha256"`
}

func collectEvidence(configuration options) (aggregateDocument, error) {
	evidenceRoot, err := openConfinedRoot(configuration.directory, "evidence")
	if err != nil {
		return aggregateDocument{}, err
	}
	defer evidenceRoot.Close()
	archiveRoot, err := openConfinedRoot(configuration.archives, "archive")
	if err != nil {
		return aggregateDocument{}, err
	}
	defer archiveRoot.Close()

	evidenceNames := make([]string, 0, len(targetContracts))
	archiveNames := make([]string, 0, len(targetContracts))
	for _, contract := range targetContracts {
		evidenceNames = append(evidenceNames, contract.fileName)
		archiveNames = append(archiveNames, contract.archiveName(configuration.tag))
	}
	if err := validateEntrySet(evidenceRoot, evidenceNames, "evidence"); err != nil {
		return aggregateDocument{}, err
	}
	if err := validateEntrySet(archiveRoot, archiveNames, "archive"); err != nil {
		return aggregateDocument{}, err
	}
	version := strings.TrimPrefix(configuration.tag, "v")
	targets := make([]aggregateTarget, 0, len(targetContracts))
	for _, contract := range targetContracts {
		archiveName := contract.archiveName(configuration.tag)
		value, err := readEvidenceFile(evidenceRoot, contract.fileName)
		if err != nil {
			return aggregateDocument{}, err
		}
		document, err := decodeEvidence(value)
		if err != nil {
			return aggregateDocument{}, err
		}
		if err := validateEvidence(document, contract.target, archiveName, version, configuration.revision); err != nil {
			return aggregateDocument{}, err
		}
		archiveDigest, err := digestArchive(archiveRoot, archiveName)
		if err != nil {
			return aggregateDocument{}, err
		}
		if archiveDigest != document.ArtifactJourney.ArchiveSHA256 {
			return aggregateDocument{}, fmt.Errorf("archive digest does not match evidence")
		}
		targets = append(targets, aggregateTarget{
			Target:        contract.target,
			Result:        "passed",
			ArchiveName:   archiveName,
			ArchiveSHA256: archiveDigest,
		})
	}
	if err := validateEntrySet(evidenceRoot, evidenceNames, "evidence"); err != nil {
		return aggregateDocument{}, err
	}
	if err := validateEntrySet(archiveRoot, archiveNames, "archive"); err != nil {
		return aggregateDocument{}, err
	}
	return aggregateDocument{
		SchemaVersion:   1,
		Tag:             configuration.tag,
		Revision:        configuration.revision,
		ProvenanceLevel: "workflow_index_unattested",
		Targets:         targets,
	}, nil
}

func openConfinedRoot(path, label string) (*os.Root, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("%s directory must be absolute and clean", label)
	}
	directoryInfo, err := os.Lstat(path)
	if err != nil || directoryInfo.Mode()&os.ModeSymlink != 0 || !directoryInfo.IsDir() {
		return nil, fmt.Errorf("%s directory is invalid", label)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("%s directory could not be opened", label)
	}
	openedDirectory, openedErr := root.Stat(".")
	currentDirectory, currentErr := os.Lstat(path)
	if openedErr != nil || currentErr != nil || currentDirectory.Mode()&os.ModeSymlink != 0 ||
		!openedDirectory.IsDir() || !currentDirectory.IsDir() ||
		!os.SameFile(directoryInfo, openedDirectory) || !os.SameFile(openedDirectory, currentDirectory) {
		_ = root.Close()
		return nil, fmt.Errorf("%s directory changed while opening", label)
	}
	return root, nil
}

func validateEntrySet(root *os.Root, names []string, label string) error {
	directory, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("%s directory could not be read", label)
	}
	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil || closeErr != nil || len(entries) != len(names) {
		return fmt.Errorf("%s file set is invalid", label)
	}
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}
	for _, entry := range entries {
		if _, ok := wanted[entry.Name()]; !ok {
			return fmt.Errorf("%s file set is invalid", label)
		}
	}
	return nil
}

func readEvidenceFile(root *os.Root, name string) ([]byte, error) {
	before, err := root.Lstat(name)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > maxEvidenceFileBytes {
		return nil, fmt.Errorf("evidence entry is invalid")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("evidence entry could not be opened")
	}
	opened, statErr := file.Stat()
	if statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		_ = file.Close()
		return nil, fmt.Errorf("evidence entry changed while opening")
	}
	value, readErr := io.ReadAll(io.LimitReader(file, maxEvidenceFileBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(value) > maxEvidenceFileBytes {
		return nil, fmt.Errorf("evidence entry could not be read within its bound")
	}
	after, err := root.Lstat(name)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() ||
		!os.SameFile(opened, after) || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
		return nil, fmt.Errorf("evidence entry changed while reading")
	}
	return value, nil
}

func digestArchive(root *os.Root, name string) (string, error) {
	before, err := root.Lstat(name)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() <= 0 || before.Size() > maxArchiveBytes {
		return "", fmt.Errorf("archive entry is invalid")
	}
	file, err := root.Open(name)
	if err != nil {
		return "", fmt.Errorf("archive entry could not be opened")
	}
	opened, statErr := file.Stat()
	if statErr != nil || !opened.Mode().IsRegular() || opened.Size() <= 0 || opened.Size() > maxArchiveBytes || !os.SameFile(before, opened) {
		_ = file.Close()
		return "", fmt.Errorf("archive entry changed while opening")
	}
	hash := sha256.New()
	written, readErr := io.Copy(hash, io.LimitReader(file, maxArchiveBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || written != opened.Size() || written > maxArchiveBytes {
		return "", fmt.Errorf("archive entry could not be read within its bound")
	}
	after, err := root.Lstat(name)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() ||
		!os.SameFile(opened, after) || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
		return "", fmt.Errorf("archive entry changed while reading")
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func decodeEvidence(value []byte) (evidenceDocument, error) {
	if len(value) == 0 || len(value) > maxEvidenceFileBytes {
		return evidenceDocument{}, fmt.Errorf("evidence JSON is invalid")
	}
	if err := rejectDuplicateJSONFields(value); err != nil {
		return evidenceDocument{}, fmt.Errorf("evidence JSON is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	var document evidenceDocument
	if err := decoder.Decode(&document); err != nil {
		return evidenceDocument{}, fmt.Errorf("evidence JSON is invalid")
	}
	if err := requireJSONEOF(decoder); err != nil {
		return evidenceDocument{}, fmt.Errorf("evidence JSON is invalid")
	}
	return document, nil
}

func rejectDuplicateJSONFields(value []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if err := consumeJSONValue(decoder); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

func consumeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate object key")
			}
			seen[key] = struct{}{}
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return fmt.Errorf("object is not closed")
		}
		return nil
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return fmt.Errorf("array is not closed")
		}
		return nil
	default:
		return fmt.Errorf("unexpected JSON delimiter")
	}
}

func requireJSONEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

func validateEvidence(document evidenceDocument, target, archiveName, version, revision string) error {
	journey := document.ArtifactJourney
	if document.SchemaVersion != 1 {
		return fmt.Errorf("evidence schema version is invalid")
	}
	if journey.Target != target || journey.ObservedHost != target || journey.ArchiveName != archiveName || journey.Version != version || journey.Revision != revision {
		return fmt.Errorf("evidence identity is invalid")
	}
	if !lowercaseHex(journey.ArchiveSHA256, digestLength) ||
		!lowercaseHex(journey.BundleDigest, digestLength) ||
		!lowercaseHex(journey.PlanDigest, digestLength) ||
		!lowercaseHex(journey.IssueBundleDigest, digestLength) ||
		!lowercaseHex(journey.IssuePlanDigest, digestLength) {
		return fmt.Errorf("evidence digest is invalid")
	}
	if journey.HelpContractsVerified != wantedHelpContracts || journey.SourceInspectionAttempts != wantedInspections || journey.ZeroAttemptRejections != wantedRejections || journey.FixtureAttempts != wantedAttempts {
		return fmt.Errorf("evidence counters are invalid")
	}
	if !equalStrings(journey.PostStartFaults, wantedFaults) {
		return fmt.Errorf("evidence fault sequence is invalid")
	}
	if !equalStrings(journey.CommandsVerified, wantedCommands) {
		return fmt.Errorf("evidence command sequence is invalid")
	}
	if !journey.CredentialEnvironmentAbsent || !journey.SecretCanariesAbsent {
		return fmt.Errorf("evidence safety assertions are invalid")
	}
	return nil
}

func encodeAggregate(document aggregateDocument) ([]byte, error) {
	encoded, err := json.Marshal(document)
	if err != nil || len(encoded)+1 > maxAggregateBytes {
		return nil, fmt.Errorf("aggregate encoding failed")
	}
	return append(encoded, '\n'), nil
}

func validReleaseTag(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	match := releaseTagPattern.FindStringSubmatch(value)
	if match == nil {
		return false
	}
	prerelease := match[4]
	for _, identifier := range strings.Split(prerelease, ".") {
		if prerelease == "" {
			break
		}
		if numeric(identifier) && len(identifier) > 1 && identifier[0] == '0' {
			return false
		}
	}
	return true
}

func numeric(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func lowercaseHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
