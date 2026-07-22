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
	"slices"
	"strings"
)

const (
	revisionLength        = 40
	digestLength          = 64
	maxEvidenceFileBytes  = 16 * 1024
	maxArchiveBytes       = int64(256 * 1024 * 1024)
	maxAggregateBytes     = 4 * 1024
	wantedHelpContracts   = 8
	wantedInspections     = 4
	wantedGoInspections   = 3
	wantedRejections      = 8
	wantedPOSIXAttempts   = 13
	wantedWindowsAttempts = 10
	wantedPOSIXWrappers   = 3
	wantedGoPOSIXWrappers = 1
)

const (
	emptySHA256             = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	transformedStdoutSHA256 = "277258cb99075f67f56acb96a0d7a340644442f0147385cbfef6634897437ade"
	identityStdoutSHA256    = "211630ed346fee12b3e2c5135f3239dc7ce64e10eb149e8ef032bc04ff115b7b"
	identityStderrSHA256    = "cfc159919dad8548c6e2ed887297e77aed35d6f2d20d42c08b29d7caa4f8faa0"
	appendStdoutSHA256      = "162a8a6b49c40255d3d0d2e5ed86f5d4ca88b3963d8c667bd7b79e768bd26d29"
	appendStderrSHA256      = "b8f249840842aad27390cfb637be1e2456a9d873ab1141d01d2cdccff1699c4a"
)

var releaseTagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

var go126VersionPattern = regexp.MustCompile(`^go1\.26\.(0|[1-9][0-9]*)$`)

const (
	goAdapterKind            = "atsura.source.go_cli"
	goAdapterContractVersion = 1
)

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
	Target                      string                `json:"target"`
	ObservedHost                string                `json:"observed_host"`
	ArchiveName                 string                `json:"archive_name"`
	ArchiveSHA256               string                `json:"archive_sha256"`
	Version                     string                `json:"version"`
	Revision                    string                `json:"revision"`
	HelpContractsVerified       int                   `json:"help_contracts_verified"`
	CommandsVerified            []string              `json:"commands_verified"`
	BundleDigest                string                `json:"bundle_digest"`
	PlanDigest                  string                `json:"plan_digest"`
	IssueBundleDigest           string                `json:"issue_bundle_digest"`
	IssuePlanDigest             string                `json:"issue_plan_digest"`
	WrapperOutcome              string                `json:"wrapper_outcome"`
	WrapperCases                []wrapperCaseEvidence `json:"wrapper_cases"`
	WrapperSourceAttempts       int                   `json:"wrapper_source_process_attempts"`
	SourceInspectionAttempts    int                   `json:"source_inspection_attempts"`
	ZeroAttemptRejections       int                   `json:"zero_attempt_rejections"`
	PostStartFaults             []string              `json:"post_start_faults"`
	FixtureAttempts             int                   `json:"fixture_attempts"`
	CredentialEnvironmentAbsent bool                  `json:"credential_environment_absent"`
	SecretCanariesAbsent        bool                  `json:"secret_canaries_absent"`
	GoSource                    goSourceEvidence      `json:"go_source"`
}

// goSourceEvidence is the bounded second-source proof. It deliberately uses
// the same wrapper-case shape as the first source while retaining only
// adapter identity, digests, counters, and conventional process status.
type goSourceEvidence struct {
	AdapterKind              string                `json:"adapter_kind"`
	AdapterContractVersion   int                   `json:"adapter_contract_version"`
	SourceVersion            string                `json:"source_version"`
	CatalogDigest            string                `json:"catalog_digest"`
	SourceInspectionAttempts int                   `json:"source_inspection_attempts"`
	CommandsVerified         []string              `json:"commands_verified"`
	BundleDigest             string                `json:"bundle_digest"`
	PlanDigest               string                `json:"plan_digest"`
	WrapperOutcome           string                `json:"wrapper_outcome"`
	WrapperCases             []wrapperCaseEvidence `json:"wrapper_cases"`
	WrapperSourceAttempts    int                   `json:"wrapper_source_process_attempts"`
	ZeroAttemptRejections    int                   `json:"zero_attempt_rejections"`
}

type wrapperCaseEvidence struct {
	Name                  string `json:"name"`
	WrapperKind           string `json:"wrapper_kind"`
	ResultMode            string `json:"result_mode"`
	BundleDigest          string `json:"bundle_digest"`
	PlanDigest            string `json:"plan_digest"`
	WrapperSourceSHA256   string `json:"wrapper_source_sha256"`
	StdoutSHA256          string `json:"stdout_sha256"`
	StderrSHA256          string `json:"stderr_sha256"`
	SourceExitCode        int    `json:"source_exit_code"`
	SourceProcessAttempts int    `json:"source_process_attempts"`
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
	if document.SchemaVersion != 4 {
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
	if journey.HelpContractsVerified != wantedHelpContracts || journey.SourceInspectionAttempts != wantedInspections || journey.ZeroAttemptRejections != wantedRejections {
		return fmt.Errorf("evidence counters are invalid")
	}
	if target == "windows/amd64" {
		if journey.WrapperOutcome != "platform_not_supported" || journey.WrapperCases == nil || len(journey.WrapperCases) != 0 || journey.WrapperSourceAttempts != 0 || journey.FixtureAttempts != wantedWindowsAttempts {
			return fmt.Errorf("Windows wrapper evidence is invalid")
		}
	} else {
		if journey.WrapperOutcome != "ordinary_command_verified" || journey.WrapperSourceAttempts != wantedPOSIXWrappers || journey.FixtureAttempts != wantedPOSIXAttempts {
			return fmt.Errorf("POSIX wrapper evidence is invalid")
		}
		if err := validateWrapperCases(journey.WrapperCases, journey.BundleDigest, journey.PlanDigest); err != nil {
			return err
		}
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
	if err := validateGoSourceEvidence(journey.GoSource, target); err != nil {
		return err
	}
	return nil
}

func validateGoSourceEvidence(evidence goSourceEvidence, target string) error {
	if evidence.AdapterKind != goAdapterKind || evidence.AdapterContractVersion != goAdapterContractVersion ||
		!go126VersionPattern.MatchString(evidence.SourceVersion) ||
		!lowercaseHex(evidence.CatalogDigest, digestLength) ||
		!lowercaseHex(evidence.BundleDigest, digestLength) ||
		!lowercaseHex(evidence.PlanDigest, digestLength) ||
		evidence.SourceInspectionAttempts != wantedGoInspections ||
		!equalStrings(evidence.CommandsVerified, []string{"test"}) {
		return fmt.Errorf("Go source evidence identity is invalid")
	}
	if target == "windows/amd64" {
		if evidence.WrapperOutcome != "platform_not_supported" || evidence.WrapperCases == nil || len(evidence.WrapperCases) != 0 ||
			evidence.WrapperSourceAttempts != 0 || evidence.ZeroAttemptRejections != 1 {
			return fmt.Errorf("Windows Go wrapper evidence is invalid")
		}
		return nil
	}
	if evidence.WrapperOutcome != "ordinary_command_verified" || evidence.WrapperSourceAttempts != wantedGoPOSIXWrappers ||
		evidence.ZeroAttemptRejections != 1 || len(evidence.WrapperCases) != wantedGoPOSIXWrappers {
		return fmt.Errorf("POSIX Go wrapper evidence is invalid")
	}
	actual := evidence.WrapperCases[0]
	if actual.Name != "go_test_identity" || actual.WrapperKind != "identity" || actual.ResultMode != "source_stream_passthrough" ||
		actual.BundleDigest != evidence.BundleDigest || actual.PlanDigest != evidence.PlanDigest ||
		!lowercaseHex(actual.WrapperSourceSHA256, digestLength) || actual.WrapperSourceSHA256 == emptySHA256 || !lowercaseHex(actual.StdoutSHA256, digestLength) ||
		actual.StdoutSHA256 == emptySHA256 || actual.StderrSHA256 != emptySHA256 || actual.SourceExitCode != 0 || actual.SourceProcessAttempts != 1 {
		return fmt.Errorf("POSIX Go wrapper case is invalid")
	}
	return nil
}

func validateWrapperCases(cases []wrapperCaseEvidence, transformedBundleDigest, transformedPlanDigest string) error {
	if len(cases) != wantedPOSIXWrappers {
		return fmt.Errorf("POSIX wrapper case inventory is invalid")
	}
	if cases[0].BundleDigest != transformedBundleDigest || cases[0].PlanDigest != transformedPlanDigest {
		return fmt.Errorf("POSIX transformed wrapper identity is invalid")
	}
	wanted := []struct {
		name, kind, mode     string
		stdoutSHA, stderrSHA string
		exitCode             int
	}{
		{name: "transformed_json", kind: "transform", mode: "transformed_json", stdoutSHA: transformedStdoutSHA256, stderrSHA: emptySHA256, exitCode: 0},
		{name: "identity", kind: "identity", mode: "source_stream_passthrough", stdoutSHA: identityStdoutSHA256, stderrSHA: identityStderrSHA256, exitCode: 0},
		{name: "append_only", kind: "transform", mode: "source_stream_passthrough", stdoutSHA: appendStdoutSHA256, stderrSHA: appendStderrSHA256, exitCode: 23},
	}
	bundleDigests := make(map[string]struct{}, len(cases))
	planDigests := make(map[string]struct{}, len(cases))
	wrapperDigests := make(map[string]struct{}, len(cases))
	for index, expected := range wanted {
		actual := cases[index]
		if actual.Name != expected.name || actual.WrapperKind != expected.kind || actual.ResultMode != expected.mode || actual.SourceExitCode != expected.exitCode || actual.SourceProcessAttempts != 1 ||
			!lowercaseHex(actual.BundleDigest, digestLength) || !lowercaseHex(actual.PlanDigest, digestLength) || !lowercaseHex(actual.WrapperSourceSHA256, digestLength) ||
			!lowercaseHex(actual.StdoutSHA256, digestLength) || !lowercaseHex(actual.StderrSHA256, digestLength) {
			return fmt.Errorf("POSIX wrapper case %d is invalid", index)
		}
		if actual.StdoutSHA256 != expected.stdoutSHA || actual.StderrSHA256 != expected.stderrSHA {
			return fmt.Errorf("POSIX wrapper case %d stream evidence is invalid", index)
		}
		if _, duplicate := bundleDigests[actual.BundleDigest]; duplicate {
			return fmt.Errorf("POSIX wrapper bundle identity is duplicated")
		}
		if _, duplicate := planDigests[actual.PlanDigest]; duplicate {
			return fmt.Errorf("POSIX wrapper plan identity is duplicated")
		}
		if _, duplicate := wrapperDigests[actual.WrapperSourceSHA256]; duplicate {
			return fmt.Errorf("POSIX rendered wrapper identity is duplicated")
		}
		bundleDigests[actual.BundleDigest] = struct{}{}
		planDigests[actual.PlanDigest] = struct{}{}
		wrapperDigests[actual.WrapperSourceSHA256] = struct{}{}
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
	return slices.Equal(left, right)
}
