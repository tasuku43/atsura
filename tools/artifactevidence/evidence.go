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

	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
	"github.com/tasuku43/atsura/tools/internal/processormanifest"
)

const (
	revisionLength        = 40
	digestLength          = 64
	maxEvidenceFileBytes  = 16 * 1024
	maxArchiveBytes       = int64(256 * 1024 * 1024)
	maxAggregateBytes     = 4 * 1024
	wantedHelpContracts   = 12
	wantedInspections     = 4
	wantedGoInspections   = 3
	wantedRejections      = 8
	wantedPOSIXAttempts   = 14
	wantedWindowsAttempts = 10
	wantedPOSIXWrappers   = 4
	wantedGoPOSIXWrappers = 1
	wantedGoPOSIXCases    = 4
	wantedGoPOSIXAttempts = 5
)

const (
	emptySHA256             = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	transformedStdoutSHA256 = "277258cb99075f67f56acb96a0d7a340644442f0147385cbfef6634897437ade"
	identityStdoutSHA256    = "211630ed346fee12b3e2c5135f3239dc7ce64e10eb149e8ef032bc04ff115b7b"
	identityStderrSHA256    = "cfc159919dad8548c6e2ed887297e77aed35d6f2d20d42c08b29d7caa4f8faa0"
	appendStdoutSHA256      = "162a8a6b49c40255d3d0d2e5ed86f5d4ca88b3963d8c667bd7b79e768bd26d29"
	appendStderrSHA256      = "b8f249840842aad27390cfb637be1e2456a9d873ab1141d01d2cdccff1699c4a"
	optimizedGoStdoutSHA256 = "a4f3dee01192dc3d1e710a3301d7f9f35bf7e7f14135b4a96ce398dc3af043b4"
)

var releaseTagPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

var go126VersionPattern = regexp.MustCompile(`^go1\.26\.(0|[1-9][0-9]*)$`)

const (
	goAdapterKind             = "atsura.source.go_cli"
	goAdapterContractVersion  = 2
	rtkAdapterContractVersion = 1
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

var (
	wantedDefaultAppliedCallerArgv    = []string{"pr", "list"}
	wantedDefaultAppliedSourceArgv    = []string{"pr", "list", "--limit=30", "--json=number,title,state"}
	wantedDefaultOverriddenCallerArgv = []string{"pr", "list", "--limit=2"}
	wantedDefaultOverriddenSourceArgv = []string{"pr", "list", "--limit=2", "--json=number,title,state"}
	wantedAppendOnlyCallerArgv        = []string{"issue", "list", "--search=append value", "--label=one", "--label=two"}
	wantedAppendOnlySourceArgv        = []string{"issue", "list", "--search=append value", "--label=one", "--label=two", "--limit=1"}
	wantedIdentityCallerArgv          = []string{
		"pr", "list",
		"--search=space value;$(touch atsura-artifact-injection)",
		"--label=first",
		"--label=Unicode 雪",
		"--repo=-dash",
	}
	wantedIdentitySourceArgv = append([]string{}, wantedIdentityCallerArgv...)
	wantedGoCallerArgv       = []string{"test"}
	wantedGoSourceArgv       = []string{"test"}
	wantedOptionDefaults     = []tailoringbundle.OptionDefault{{Option: "--limit", Value: "30"}}
)

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
	TailoredHelp                tailoredHelpEvidence  `json:"tailored_help"`
	GoSource                    goSourceEvidence      `json:"go_source"`
}

type tailoredHelpEvidence struct {
	Outcome                           string                      `json:"outcome"`
	BundleDigest                      string                      `json:"bundle_digest"`
	WrapperSourceSHA256               string                      `json:"wrapper_source_sha256"`
	WrapperContractVersion            int                         `json:"wrapper_contract_version"`
	Views                             []tailoredHelpViewEvidence  `json:"views"`
	FallthroughFaults                 []tailoredHelpFaultEvidence `json:"fallthrough_faults"`
	RuntimeNonExecutableDuringSuccess bool                        `json:"runtime_non_executable_during_success"`
	SourceProcessAttempts             int                         `json:"source_process_attempts"`
	ProcessorProcessAttempts          int                         `json:"processor_process_attempts"`
}

type tailoredHelpViewEvidence struct {
	Name         string   `json:"name"`
	Argv         []string `json:"argv"`
	StdoutSHA256 string   `json:"stdout_sha256"`
	StderrSHA256 string   `json:"stderr_sha256"`
}

type tailoredHelpFaultEvidence struct {
	Name                     string   `json:"name"`
	Argv                     []string `json:"argv"`
	Code                     string   `json:"code"`
	SourceProcessAttempts    int      `json:"source_process_attempts"`
	ProcessorProcessAttempts int      `json:"processor_process_attempts"`
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
	Optimizer                goOptimizerEvidence   `json:"optimizer"`
}

type goOptimizerEvidence struct {
	Outcome               string                      `json:"outcome"`
	Processor             *processorArtifactEvidence  `json:"processor,omitempty"`
	Execution             *optimizerExecutionEvidence `json:"execution,omitempty"`
	BundleDigest          string                      `json:"bundle_digest,omitempty"`
	PlanDigest            string                      `json:"plan_digest,omitempty"`
	WrapperSourceSHA256   string                      `json:"wrapper_source_sha256,omitempty"`
	Cases                 []optimizerCaseEvidence     `json:"cases"`
	Faults                []optimizerFaultEvidence    `json:"faults"`
	SourceProcessAttempts int                         `json:"source_process_attempts"`
	ZeroAttemptRejections int                         `json:"zero_attempt_rejections"`
}

type optimizerExecutionEvidence struct {
	CallerArgv                    []string `json:"caller_argv"`
	SourceArgv                    []string `json:"source_argv"`
	SourceStdinMode               string   `json:"source_stdin_mode"`
	SourceWorkingDirectoryMode    string   `json:"source_working_directory_mode"`
	SourceEnvironmentMode         string   `json:"source_environment_mode"`
	SourceMaxAttempts             int      `json:"source_max_attempts"`
	SourceTimeoutMillis           int64    `json:"source_timeout_millis"`
	SourceStdoutLimitBytes        int      `json:"source_stdout_limit_bytes"`
	SourceStderrLimitBytes        int      `json:"source_stderr_limit_bytes"`
	InputFormat                   string   `json:"input_format"`
	OutputFormat                  string   `json:"output_format"`
	AllowOriginalOutput           bool     `json:"allow_original_output"`
	ProcessorArgv                 []string `json:"processor_argv"`
	ProcessorStdinMode            string   `json:"processor_stdin_mode"`
	ProcessorWorkingDirectoryMode string   `json:"processor_working_directory_mode"`
	ProcessorEnvironmentContract  string   `json:"processor_environment_contract"`
	ProcessorMaxAttempts          int      `json:"processor_max_attempts"`
	ProcessorTimeoutMillis        int64    `json:"processor_timeout_millis"`
	ProcessorStdoutLimitBytes     int      `json:"processor_stdout_limit_bytes"`
	ProcessorStderrLimitBytes     int      `json:"processor_stderr_limit_bytes"`
}

type processorArtifactEvidence struct {
	ContractID                string `json:"contract_id"`
	AdapterKind               string `json:"adapter_kind"`
	AdapterContractVersion    int    `json:"adapter_contract_version"`
	Version                   string `json:"version"`
	Target                    string `json:"target"`
	ArchiveName               string `json:"archive_name"`
	ArchiveSHA256             string `json:"archive_sha256"`
	BinarySHA256              string `json:"binary_sha256"`
	BinarySize                int64  `json:"binary_size"`
	ObservationDigest         string `json:"observation_digest"`
	InspectionProcessAttempts int    `json:"inspection_process_attempts"`
}

type optimizerCaseEvidence struct {
	Name                  string `json:"name"`
	Disposition           string `json:"disposition"`
	StdoutSHA256          string `json:"stdout_sha256"`
	StderrSHA256          string `json:"stderr_sha256"`
	SourceExitCode        int    `json:"source_exit_code"`
	SourceProcessAttempts int    `json:"source_process_attempts"`
}

type optimizerFaultEvidence struct {
	Name                  string `json:"name"`
	Code                  string `json:"code"`
	SourceProcessAttempts int    `json:"source_process_attempts"`
}

type wrapperCaseEvidence struct {
	Name                  string                          `json:"name"`
	WrapperKind           string                          `json:"wrapper_kind"`
	ResultMode            string                          `json:"result_mode"`
	CallerArgv            []string                        `json:"caller_argv"`
	SourceArgv            []string                        `json:"source_argv"`
	OptionDefaults        []tailoringbundle.OptionDefault `json:"option_defaults"`
	AppliedOptionDefaults []tailoringbundle.OptionDefault `json:"applied_option_defaults"`
	BundleDigest          string                          `json:"bundle_digest"`
	PlanDigest            string                          `json:"plan_digest"`
	WrapperSourceSHA256   string                          `json:"wrapper_source_sha256"`
	StdoutSHA256          string                          `json:"stdout_sha256"`
	StderrSHA256          string                          `json:"stderr_sha256"`
	SourceExitCode        int                             `json:"source_exit_code"`
	SourceProcessAttempts int                             `json:"source_process_attempts"`
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
		SchemaVersion:   2,
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
	if document.SchemaVersion != 8 {
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
		if err := validateWrapperCases(
			journey.WrapperCases,
			journey.BundleDigest,
			journey.PlanDigest,
			journey.IssueBundleDigest,
			journey.IssuePlanDigest,
		); err != nil {
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
	if err := validateTailoredHelp(journey.TailoredHelp, journey, target); err != nil {
		return err
	}
	if err := validateGoSourceEvidence(journey.GoSource, target); err != nil {
		return err
	}
	return nil
}

func validateTailoredHelp(evidence tailoredHelpEvidence, journey artifactJourneyEvidence, target string) error {
	if target == "windows/amd64" {
		if evidence.Outcome != "platform_not_supported" || evidence.BundleDigest != "" || evidence.WrapperSourceSHA256 != "" ||
			evidence.WrapperContractVersion != 0 || evidence.Views == nil || len(evidence.Views) != 0 ||
			evidence.FallthroughFaults == nil || len(evidence.FallthroughFaults) != 0 ||
			evidence.RuntimeNonExecutableDuringSuccess || evidence.SourceProcessAttempts != 0 || evidence.ProcessorProcessAttempts != 0 {
			return fmt.Errorf("Windows tailored help evidence is invalid")
		}
		return nil
	}
	if evidence.Outcome != "compiled_views_verified" || evidence.BundleDigest != journey.BundleDigest ||
		len(journey.WrapperCases) < 3 || evidence.BundleDigest != journey.WrapperCases[0].BundleDigest ||
		evidence.BundleDigest != journey.WrapperCases[1].BundleDigest ||
		evidence.BundleDigest != journey.WrapperCases[2].BundleDigest ||
		evidence.WrapperSourceSHA256 == emptySHA256 ||
		evidence.WrapperSourceSHA256 != journey.WrapperCases[0].WrapperSourceSHA256 ||
		evidence.WrapperSourceSHA256 != journey.WrapperCases[1].WrapperSourceSHA256 ||
		evidence.WrapperSourceSHA256 != journey.WrapperCases[2].WrapperSourceSHA256 ||
		evidence.WrapperContractVersion != wrapperbinding.ContractVersion || !evidence.RuntimeNonExecutableDuringSuccess ||
		evidence.SourceProcessAttempts != 0 || evidence.ProcessorProcessAttempts != 0 {
		return fmt.Errorf("POSIX tailored help binding evidence is invalid")
	}
	wantedViews := []struct {
		name string
		argv []string
	}{
		{name: "root", argv: []string{"--help"}},
		{name: "issue_namespace", argv: []string{"issue", "--help"}},
		{name: "issue_exact_command", argv: []string{"issue", "list", "--help"}},
		{name: "pr_namespace", argv: []string{"pr", "--help"}},
		{name: "pr_exact_command", argv: []string{"pr", "list", "--help"}},
	}
	if evidence.Views == nil || len(evidence.Views) != len(wantedViews) {
		return fmt.Errorf("POSIX tailored help view inventory is invalid")
	}
	for index, wanted := range wantedViews {
		view := evidence.Views[index]
		output, err := expectedTailoredHelpOutput(evidence.BundleDigest, wanted.name)
		if err != nil || view.Name != wanted.name || !equalStrings(view.Argv, wanted.argv) ||
			view.StdoutSHA256 != digestEvidenceBytes(output) || view.StderrSHA256 != emptySHA256 {
			return fmt.Errorf("POSIX tailored help view %d is invalid", index)
		}
	}
	wantedFaults := []struct {
		name, code string
		argv       []string
	}{
		{name: "hidden_command", code: "command_not_in_surface", argv: []string{"api", "--help"}},
		{name: "unknown_selector", code: "invalid_invocation", argv: []string{"unknown", "--help"}},
	}
	if evidence.FallthroughFaults == nil || len(evidence.FallthroughFaults) != len(wantedFaults) {
		return fmt.Errorf("POSIX tailored help fault inventory is invalid")
	}
	for index, wanted := range wantedFaults {
		fault := evidence.FallthroughFaults[index]
		if fault.Name != wanted.name || fault.Code != wanted.code || !equalStrings(fault.Argv, wanted.argv) ||
			fault.SourceProcessAttempts != 0 || fault.ProcessorProcessAttempts != 0 {
			return fmt.Errorf("POSIX tailored help fault %d is invalid", index)
		}
	}
	return nil
}

func expectedTailoredHelpOutput(bundleDigest, view string) ([]byte, error) {
	if !lowercaseHex(bundleDigest, digestLength) || bundleDigest == emptySHA256 {
		return nil, fmt.Errorf("tailored help bundle digest is invalid")
	}
	lines := []string{"Atsura tailored help", "Bundle digest: " + bundleDigest}
	switch view {
	case "root":
		lines = append(lines, "Commands:", "  issue list", "  pr list")
	case "issue_namespace":
		lines = append(lines, "Commands:", "  issue list")
	case "issue_exact_command":
		lines = append(lines,
			"Command: issue list",
			"Source summary: List issues",
			"Tailoring reason: Append one fixed reviewed source argument and preserve its streams.",
			"Options:",
			"  --label=<value> (value required)",
			"  --search=<value> (value required)",
		)
	case "pr_namespace":
		lines = append(lines, "Commands:", "  pr list")
	case "pr_exact_command":
		lines = append(lines,
			"Command: pr list",
			"Source summary: List pull requests",
			"Tailoring reason: Return one reviewed compact result.",
			"Options:",
			"  --limit=<value> (value required; default when omitted: \"30\")",
		)
	default:
		return nil, fmt.Errorf("tailored help view is unsupported")
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

func digestEvidenceBytes(value []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(value))
}

func validateGoSourceEvidence(evidence goSourceEvidence, target string) error {
	if evidence.AdapterKind != goAdapterKind || evidence.AdapterContractVersion != goAdapterContractVersion ||
		!go126VersionPattern.MatchString(evidence.SourceVersion) ||
		!lowercaseHex(evidence.CatalogDigest, digestLength) || evidence.CatalogDigest == emptySHA256 ||
		!lowercaseHex(evidence.BundleDigest, digestLength) || evidence.BundleDigest == emptySHA256 ||
		!lowercaseHex(evidence.PlanDigest, digestLength) || evidence.PlanDigest == emptySHA256 ||
		evidence.SourceInspectionAttempts != wantedGoInspections ||
		!equalStrings(evidence.CommandsVerified, []string{"test"}) {
		return fmt.Errorf("Go source evidence identity is invalid")
	}
	if target == "windows/amd64" {
		if evidence.WrapperOutcome != "platform_not_supported" || evidence.WrapperCases == nil || len(evidence.WrapperCases) != 0 ||
			evidence.WrapperSourceAttempts != 0 || evidence.ZeroAttemptRejections != 1 {
			return fmt.Errorf("Windows Go wrapper evidence is invalid")
		}
		return validateWindowsOptimizerEvidence(evidence.Optimizer)
	}
	if evidence.WrapperOutcome != "ordinary_command_verified" || evidence.WrapperSourceAttempts != wantedGoPOSIXWrappers ||
		evidence.ZeroAttemptRejections != 1 || evidence.WrapperCases == nil || len(evidence.WrapperCases) != wantedGoPOSIXWrappers {
		return fmt.Errorf("POSIX Go wrapper evidence is invalid")
	}
	identity := evidence.WrapperCases[0]
	if identity.Name != "go_test_identity" || identity.WrapperKind != "identity" ||
		identity.ResultMode != "source_stream_passthrough" || !equalStrings(identity.CallerArgv, wantedGoCallerArgv) ||
		!equalStrings(identity.SourceArgv, wantedGoSourceArgv) || identity.OptionDefaults == nil || len(identity.OptionDefaults) != 0 ||
		identity.AppliedOptionDefaults == nil || len(identity.AppliedOptionDefaults) != 0 ||
		identity.BundleDigest != evidence.BundleDigest ||
		identity.PlanDigest != evidence.PlanDigest || !lowercaseHex(identity.WrapperSourceSHA256, digestLength) ||
		identity.WrapperSourceSHA256 == emptySHA256 || !lowercaseHex(identity.StdoutSHA256, digestLength) ||
		identity.StdoutSHA256 == emptySHA256 || identity.StderrSHA256 != emptySHA256 || identity.SourceExitCode != 0 ||
		identity.SourceProcessAttempts != 1 {
		return fmt.Errorf("POSIX Go identity wrapper case is invalid")
	}
	return validatePOSIXOptimizerEvidence(evidence, target)
}

func validateWindowsOptimizerEvidence(evidence goOptimizerEvidence) error {
	if evidence.Outcome != "platform_not_supported" || evidence.Processor != nil || evidence.Execution != nil || evidence.BundleDigest != "" ||
		evidence.PlanDigest != "" || evidence.WrapperSourceSHA256 != "" || evidence.Cases == nil || len(evidence.Cases) != 0 ||
		evidence.Faults == nil || len(evidence.Faults) != 0 || evidence.SourceProcessAttempts != 0 || evidence.ZeroAttemptRejections != 1 {
		return fmt.Errorf("Windows optimizer evidence is invalid")
	}
	return nil
}

func validatePOSIXOptimizerEvidence(evidence goSourceEvidence, target string) error {
	optimizer := evidence.Optimizer
	if optimizer.Outcome != "reachable_outcomes_verified" || optimizer.Processor == nil || optimizer.Execution == nil ||
		!lowercaseHex(optimizer.BundleDigest, digestLength) || optimizer.BundleDigest == emptySHA256 || optimizer.BundleDigest == evidence.BundleDigest ||
		!lowercaseHex(optimizer.PlanDigest, digestLength) || optimizer.PlanDigest == emptySHA256 || optimizer.PlanDigest == evidence.PlanDigest ||
		!lowercaseHex(optimizer.WrapperSourceSHA256, digestLength) || optimizer.WrapperSourceSHA256 == emptySHA256 ||
		optimizer.WrapperSourceSHA256 == evidence.WrapperCases[0].WrapperSourceSHA256 ||
		optimizer.Cases == nil || len(optimizer.Cases) != wantedGoPOSIXCases ||
		optimizer.Faults == nil || len(optimizer.Faults) != 3 ||
		optimizer.SourceProcessAttempts != wantedGoPOSIXAttempts || optimizer.ZeroAttemptRejections != 2 {
		return fmt.Errorf("POSIX optimizer evidence is invalid")
	}
	if err := validateProcessorEvidence(*optimizer.Processor, target); err != nil {
		return err
	}
	if err := validateOptimizerExecution(*optimizer.Execution); err != nil {
		return err
	}
	wantedCases := []struct {
		name, disposition string
		exitCode          int
		exactStdout       string
	}{
		{name: "optimized_pass", disposition: "optimized", exitCode: 0, exactStdout: optimizedGoStdoutSHA256},
		{name: "preserved_before_skip", disposition: "preserved_before_processor", exitCode: 0},
		{name: "preserved_before_fail", disposition: "preserved_before_processor", exitCode: 1},
		{name: "preserved_before_ineligible", disposition: "preserved_before_processor", exitCode: 0},
	}
	seenStdout := make(map[string]struct{}, len(wantedCases))
	for index, wanted := range wantedCases {
		optimizerCase := optimizer.Cases[index]
		if optimizerCase.Name != wanted.name || optimizerCase.Disposition != wanted.disposition ||
			!lowercaseHex(optimizerCase.StdoutSHA256, digestLength) || optimizerCase.StdoutSHA256 == emptySHA256 ||
			optimizerCase.StderrSHA256 != emptySHA256 || optimizerCase.SourceExitCode != wanted.exitCode ||
			optimizerCase.SourceProcessAttempts != 1 {
			return fmt.Errorf("POSIX optimizer case %d is invalid", index)
		}
		if wanted.exactStdout != "" && optimizerCase.StdoutSHA256 != wanted.exactStdout {
			return fmt.Errorf("POSIX optimizer case %d stream evidence is invalid", index)
		}
		if _, duplicate := seenStdout[optimizerCase.StdoutSHA256]; duplicate {
			return fmt.Errorf("POSIX optimizer stdout evidence is duplicated")
		}
		seenStdout[optimizerCase.StdoutSHA256] = struct{}{}
	}
	wantedFaults := []optimizerFaultEvidence{
		{Name: "projection_rejection", Code: "wrapper_runtime_not_supported", SourceProcessAttempts: 0},
		{Name: "preflight_processor_drift", Code: "processor_identity_changed", SourceProcessAttempts: 0},
		{Name: "post_source_processor_drift", Code: "processor_identity_changed", SourceProcessAttempts: 1},
	}
	for index, wanted := range wantedFaults {
		if optimizer.Faults[index] != wanted {
			return fmt.Errorf("POSIX optimizer fault %d is invalid", index)
		}
	}
	return nil
}

func validateOptimizerExecution(evidence optimizerExecutionEvidence) error {
	if !equalStrings(evidence.CallerArgv, []string{"test"}) ||
		!equalStrings(evidence.SourceArgv, []string{"test", "-json"}) ||
		evidence.SourceStdinMode != "closed" || evidence.SourceWorkingDirectoryMode != "inherit" ||
		evidence.SourceEnvironmentMode != "inherit" || evidence.SourceMaxAttempts != 1 ||
		evidence.SourceTimeoutMillis != sourceprocess.MaxTimeout.Milliseconds() ||
		evidence.SourceStdoutLimitBytes != sourceprocess.MaxStdoutBytes ||
		evidence.SourceStderrLimitBytes != sourceprocess.MaxStderrBytes ||
		evidence.InputFormat != "go_test_jsonl" || evidence.OutputFormat != "go_test_pass_summary" ||
		!evidence.AllowOriginalOutput || !equalStrings(evidence.ProcessorArgv, []string{"pipe", "--filter=go-test"}) ||
		evidence.ProcessorStdinMode != "stage_input" || evidence.ProcessorWorkingDirectoryMode != "isolated" ||
		evidence.ProcessorEnvironmentContract != processorprocess.EnvironmentRTKIsolatedV2 ||
		evidence.ProcessorMaxAttempts != 1 || evidence.ProcessorTimeoutMillis != processorprocess.MaxTimeout.Milliseconds() ||
		evidence.ProcessorStdoutLimitBytes != processorprocess.MaxStdoutBytes ||
		evidence.ProcessorStderrLimitBytes != processorprocess.MaxStderrBytes {
		return fmt.Errorf("POSIX optimizer execution evidence is invalid")
	}
	return nil
}

func validateProcessorEvidence(evidence processorArtifactEvidence, target string) error {
	manifest := processormanifest.PinnedManifest()
	metadata, err := manifest.Target(target)
	if err != nil {
		return fmt.Errorf("processor target evidence is invalid")
	}
	if evidence.ContractID != metadata.ContractID() || evidence.AdapterKind != metadata.ProcessorKind() ||
		evidence.AdapterContractVersion != rtkAdapterContractVersion || evidence.Version != metadata.Version() ||
		evidence.Target != metadata.Target() || evidence.ArchiveName != metadata.ArchiveName() ||
		evidence.ArchiveSHA256 != metadata.ArchiveSHA256() || evidence.BinarySHA256 != metadata.BinarySHA256() ||
		evidence.BinarySize != metadata.BinarySize() || !lowercaseHex(evidence.ObservationDigest, digestLength) ||
		evidence.ObservationDigest == emptySHA256 || evidence.InspectionProcessAttempts != 1 {
		return fmt.Errorf("processor provenance evidence is invalid")
	}
	return nil
}

func validateWrapperCases(
	cases []wrapperCaseEvidence,
	defaultedBundleDigest, defaultedPlanDigest string,
	directIssueBundleDigest, directIssuePlanDigest string,
) error {
	if len(cases) != wantedPOSIXWrappers {
		return fmt.Errorf("POSIX wrapper case inventory is invalid")
	}
	if defaultedBundleDigest == directIssueBundleDigest || defaultedPlanDigest == directIssuePlanDigest ||
		directIssueBundleDigest == emptySHA256 || directIssuePlanDigest == emptySHA256 ||
		cases[0].BundleDigest != defaultedBundleDigest || cases[0].PlanDigest != defaultedPlanDigest ||
		cases[1].BundleDigest != cases[0].BundleDigest || cases[2].BundleDigest != cases[0].BundleDigest ||
		cases[1].PlanDigest == directIssuePlanDigest || cases[2].PlanDigest == directIssuePlanDigest {
		return fmt.Errorf("POSIX shared wrapper plan identity is invalid")
	}
	wanted := []struct {
		name, kind, mode                      string
		callerArgv, sourceArgv                []string
		optionDefaults, appliedOptionDefaults []tailoringbundle.OptionDefault
		stdoutSHA, stderrSHA                  string
		exitCode                              int
	}{
		{
			name: "default_applied", kind: "transform", mode: "transformed_json",
			callerArgv: wantedDefaultAppliedCallerArgv, sourceArgv: wantedDefaultAppliedSourceArgv,
			optionDefaults: wantedOptionDefaults, appliedOptionDefaults: wantedOptionDefaults,
			stdoutSHA: transformedStdoutSHA256, stderrSHA: emptySHA256, exitCode: 0,
		},
		{
			name: "default_overridden", kind: "transform", mode: "transformed_json",
			callerArgv: wantedDefaultOverriddenCallerArgv, sourceArgv: wantedDefaultOverriddenSourceArgv,
			optionDefaults: wantedOptionDefaults, appliedOptionDefaults: []tailoringbundle.OptionDefault{},
			stdoutSHA: transformedStdoutSHA256, stderrSHA: emptySHA256, exitCode: 0,
		},
		{
			name: "append_only", kind: "transform", mode: "source_stream_passthrough",
			callerArgv: wantedAppendOnlyCallerArgv, sourceArgv: wantedAppendOnlySourceArgv,
			optionDefaults: []tailoringbundle.OptionDefault{}, appliedOptionDefaults: []tailoringbundle.OptionDefault{},
			stdoutSHA: appendStdoutSHA256, stderrSHA: appendStderrSHA256, exitCode: 23,
		},
		{
			name: "identity", kind: "identity", mode: "source_stream_passthrough",
			callerArgv: wantedIdentityCallerArgv, sourceArgv: wantedIdentitySourceArgv,
			optionDefaults: []tailoringbundle.OptionDefault{}, appliedOptionDefaults: []tailoringbundle.OptionDefault{},
			stdoutSHA: identityStdoutSHA256, stderrSHA: identityStderrSHA256, exitCode: 0,
		},
	}
	planDigests := make(map[string]struct{}, len(cases))
	for index, expected := range wanted {
		actual := cases[index]
		if actual.CallerArgv == nil || actual.SourceArgv == nil || actual.OptionDefaults == nil || actual.AppliedOptionDefaults == nil ||
			actual.Name != expected.name || actual.WrapperKind != expected.kind || actual.ResultMode != expected.mode ||
			!equalStrings(actual.CallerArgv, expected.callerArgv) ||
			!equalStrings(actual.SourceArgv, expected.sourceArgv) ||
			!slices.Equal(actual.OptionDefaults, expected.optionDefaults) ||
			!slices.Equal(actual.AppliedOptionDefaults, expected.appliedOptionDefaults) ||
			actual.SourceExitCode != expected.exitCode || actual.SourceProcessAttempts != 1 ||
			!lowercaseHex(actual.BundleDigest, digestLength) || !lowercaseHex(actual.PlanDigest, digestLength) || !lowercaseHex(actual.WrapperSourceSHA256, digestLength) ||
			actual.BundleDigest == emptySHA256 || actual.PlanDigest == emptySHA256 || actual.WrapperSourceSHA256 == emptySHA256 ||
			!lowercaseHex(actual.StdoutSHA256, digestLength) || !lowercaseHex(actual.StderrSHA256, digestLength) {
			return fmt.Errorf("POSIX wrapper case %d is invalid", index)
		}
		if actual.StdoutSHA256 != expected.stdoutSHA || actual.StderrSHA256 != expected.stderrSHA {
			return fmt.Errorf("POSIX wrapper case %d stream evidence is invalid", index)
		}
		if _, duplicate := planDigests[actual.PlanDigest]; duplicate {
			return fmt.Errorf("POSIX wrapper plan identity is duplicated")
		}
		planDigests[actual.PlanDigest] = struct{}{}
	}
	sharedWrapperDigest := cases[0].WrapperSourceSHA256
	if cases[1].WrapperSourceSHA256 != sharedWrapperDigest || cases[2].WrapperSourceSHA256 != sharedWrapperDigest ||
		cases[3].BundleDigest == defaultedBundleDigest || cases[3].BundleDigest == directIssueBundleDigest ||
		cases[3].PlanDigest == directIssuePlanDigest || cases[3].WrapperSourceSHA256 == sharedWrapperDigest {
		return fmt.Errorf("POSIX wrapper material relationship is invalid")
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
