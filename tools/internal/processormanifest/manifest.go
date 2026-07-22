// Package processormanifest reads the one fixed processor provenance manifest
// used by repository-only validation and native CI tooling.
package processormanifest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// Path is the only repository-relative processor manifest location.
	Path = ".harness/processors.json"
	// SchemaVersion is the only processor manifest schema accepted by this
	// iteration.
	SchemaVersion = 1

	maxManifestBytes = 64 * 1024
	maxJSONDepth     = 32
	maxTextBytes     = 2048
	maxArchiveBytes  = 64 * 1024 * 1024
	maxBinaryBytes   = 128 * 1024 * 1024
)

var supportedTargets = [...]string{
	"linux/amd64",
	"linux/arm64",
	"darwin/amd64",
	"darwin/arm64",
}

// Manifest is the strict versioned processor provenance document. Callers
// that need one artifact should use Target, which returns detached immutable
// metadata rather than retaining a reference to the Artifacts slice.
type Manifest struct {
	SchemaVersion int         `json:"schema_version"`
	Processors    []Processor `json:"processors"`
}

// Processor records one finite external processor contract.
type Processor struct {
	ContractID     string     `json:"contract_id"`
	Kind           string     `json:"kind"`
	Version        string     `json:"version"`
	UpstreamCommit string     `json:"upstream_commit"`
	ReleaseURL     string     `json:"release_url"`
	Checksums      Checksums  `json:"checksums"`
	License        License    `json:"license"`
	Notice         Notice     `json:"notice"`
	Distribution   string     `json:"distribution"`
	SBOMReview     string     `json:"sbom_review"`
	Artifacts      []Artifact `json:"artifacts"`
}

// Checksums records the pinned upstream checksum-list provenance.
type Checksums struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// License records the pinned upstream license provenance.
type License struct {
	SPDX   string `json:"spdx"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// Notice records the reviewed upstream notice state.
type Notice struct {
	Status string `json:"status"`
}

// Artifact records one native archive and its extracted executable identity.
type Artifact struct {
	Target        string `json:"target"`
	ArchiveName   string `json:"archive_name"`
	ArchiveURL    string `json:"archive_url"`
	ArchiveSHA256 string `json:"archive_sha256"`
	ArchiveSize   int64  `json:"archive_size"`
	BinaryMember  string `json:"binary_member"`
	BinarySHA256  string `json:"binary_sha256"`
	BinarySize    int64  `json:"binary_size"`
}

// TargetMetadata is a detached value for one exact processor target. Its
// fields are private so callers cannot mutate the manifest-derived identity.
type TargetMetadata struct {
	contractID    string
	processorKind string
	version       string
	target        string
	archiveName   string
	archiveURL    string
	archiveSHA256 string
	archiveSize   int64
	binaryMember  string
	binarySHA256  string
	binarySize    int64
}

func (m TargetMetadata) ContractID() string    { return m.contractID }
func (m TargetMetadata) ProcessorKind() string { return m.processorKind }
func (m TargetMetadata) Version() string       { return m.version }
func (m TargetMetadata) Target() string        { return m.target }
func (m TargetMetadata) ArchiveName() string   { return m.archiveName }
func (m TargetMetadata) ArchiveURL() string    { return m.archiveURL }
func (m TargetMetadata) ArchiveSHA256() string { return m.archiveSHA256 }
func (m TargetMetadata) ArchiveSize() int64    { return m.archiveSize }
func (m TargetMetadata) BinaryMember() string  { return m.binaryMember }
func (m TargetMetadata) BinarySHA256() string  { return m.binarySHA256 }
func (m TargetMetadata) BinarySize() int64     { return m.binarySize }

// SupportedTarget reports whether target belongs to the exact initial POSIX
// processor matrix. Windows intentionally has no optimizer contract.
func SupportedTarget(target string) bool {
	for _, candidate := range supportedTargets {
		if target == candidate {
			return true
		}
	}
	return false
}

// Load reads only Path below one absolute, clean, non-symlink repository root.
// The read is bounded and rejects duplicate keys, unknown fields, trailing JSON,
// symlink components, special files, replacement races, and invalid structure.
func Load(repositoryRoot string) (Manifest, error) {
	data, err := readManifest(repositoryRoot)
	if err != nil {
		return Manifest{}, err
	}
	if !utf8.Valid(data) {
		return Manifest{}, fmt.Errorf("processor manifest must be valid UTF-8")
	}
	if err := rejectDuplicateKeys(data); err != nil {
		return Manifest{}, fmt.Errorf("processor manifest is not strict JSON: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest *Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode processor manifest: %w", err)
	}
	if manifest == nil {
		return Manifest{}, fmt.Errorf("processor manifest top level must be an object")
	}
	if err := requireEOF(decoder); err != nil {
		return Manifest{}, fmt.Errorf("decode processor manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return *manifest, nil
}

// Validate applies the bounded structural contract. Exact ADR 0012 pinned
// values remain a separate contractlint responsibility.
func (m Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("processor manifest schema_version must be exactly %d", SchemaVersion)
	}
	if len(m.Processors) != 1 {
		return fmt.Errorf("processor manifest must contain exactly one processor")
	}
	processor := m.Processors[0]
	for name, value := range map[string]string{
		"contract_id": processor.ContractID, "kind": processor.Kind,
		"version": processor.Version, "upstream_commit": processor.UpstreamCommit,
		"distribution": processor.Distribution, "sbom_review": processor.SBOMReview,
		"checksums.sha256": processor.Checksums.SHA256,
		"license.spdx":     processor.License.SPDX, "license.sha256": processor.License.SHA256,
		"notice.status": processor.Notice.Status,
	} {
		if err := validText(name, value); err != nil {
			return err
		}
	}
	if !lowercaseHex(processor.UpstreamCommit, 40) {
		return fmt.Errorf("processor manifest upstream_commit must be a full lowercase commit SHA")
	}
	for name, value := range map[string]string{
		"checksums.sha256": processor.Checksums.SHA256,
		"license.sha256":   processor.License.SHA256,
	} {
		if !lowercaseHex(value, 64) {
			return fmt.Errorf("processor manifest %s must be a lowercase SHA-256", name)
		}
	}
	for name, value := range map[string]string{
		"release_url":   processor.ReleaseURL,
		"checksums.url": processor.Checksums.URL,
		"license.url":   processor.License.URL,
	} {
		if err := validHTTPSURL(name, value); err != nil {
			return err
		}
	}
	if len(processor.Artifacts) != len(supportedTargets) {
		return fmt.Errorf("processor manifest must contain every supported target exactly once")
	}
	seen := make(map[string]struct{}, len(processor.Artifacts))
	for index, artifact := range processor.Artifacts {
		if !SupportedTarget(artifact.Target) {
			return fmt.Errorf("processor artifact %d has an unsupported target", index)
		}
		if _, exists := seen[artifact.Target]; exists {
			return fmt.Errorf("processor manifest contains duplicate target %q", artifact.Target)
		}
		seen[artifact.Target] = struct{}{}
		if err := validateArtifact(index, artifact); err != nil {
			return err
		}
	}
	for _, target := range supportedTargets {
		if _, exists := seen[target]; !exists {
			return fmt.Errorf("processor manifest is missing target %q", target)
		}
	}
	return nil
}

// Target returns detached immutable metadata for exactly one supported target.
func (m Manifest) Target(target string) (TargetMetadata, error) {
	if !SupportedTarget(target) {
		return TargetMetadata{}, fmt.Errorf("unsupported processor target")
	}
	if err := m.Validate(); err != nil {
		return TargetMetadata{}, err
	}
	processor := m.Processors[0]
	var selected *Artifact
	for index := range processor.Artifacts {
		if processor.Artifacts[index].Target != target {
			continue
		}
		if selected != nil {
			return TargetMetadata{}, fmt.Errorf("processor manifest contains duplicate target %q", target)
		}
		copy := processor.Artifacts[index]
		selected = &copy
	}
	if selected == nil {
		return TargetMetadata{}, fmt.Errorf("processor manifest is missing target %q", target)
	}
	return TargetMetadata{
		contractID: processor.ContractID, processorKind: processor.Kind,
		version: processor.Version, target: selected.Target,
		archiveName: selected.ArchiveName, archiveURL: selected.ArchiveURL,
		archiveSHA256: selected.ArchiveSHA256, archiveSize: selected.ArchiveSize,
		binaryMember: selected.BinaryMember, binarySHA256: selected.BinarySHA256,
		binarySize: selected.BinarySize,
	}, nil
}

func validateArtifact(index int, artifact Artifact) error {
	for name, value := range map[string]string{
		"target": artifact.Target, "archive_name": artifact.ArchiveName,
		"archive_sha256": artifact.ArchiveSHA256, "binary_member": artifact.BinaryMember,
		"binary_sha256": artifact.BinarySHA256,
	} {
		if err := validText(fmt.Sprintf("artifact %d %s", index, name), value); err != nil {
			return err
		}
	}
	if !safeBaseName(artifact.ArchiveName) {
		return fmt.Errorf("processor artifact %d archive_name must be a safe basename", index)
	}
	if !safeBaseName(artifact.BinaryMember) {
		return fmt.Errorf("processor artifact %d binary_member must be a safe basename", index)
	}
	if err := validHTTPSURL(fmt.Sprintf("artifact %d archive_url", index), artifact.ArchiveURL); err != nil {
		return err
	}
	if !lowercaseHex(artifact.ArchiveSHA256, 64) {
		return fmt.Errorf("processor artifact %d archive_sha256 must be a lowercase SHA-256", index)
	}
	if !lowercaseHex(artifact.BinarySHA256, 64) {
		return fmt.Errorf("processor artifact %d binary_sha256 must be a lowercase SHA-256", index)
	}
	if artifact.ArchiveSize <= 0 || artifact.ArchiveSize > maxArchiveBytes {
		return fmt.Errorf("processor artifact %d archive_size is outside the supported bound", index)
	}
	if artifact.BinarySize <= 0 || artifact.BinarySize > maxBinaryBytes {
		return fmt.Errorf("processor artifact %d binary_size is outside the supported bound", index)
	}
	return nil
}

func readManifest(repositoryRoot string) ([]byte, error) {
	if repositoryRoot == "" || !filepath.IsAbs(repositoryRoot) || filepath.Clean(repositoryRoot) != repositoryRoot || filepath.Dir(repositoryRoot) == repositoryRoot {
		return nil, fmt.Errorf("repository root must be an absolute clean non-filesystem-root path")
	}
	beforeRoot, err := os.Lstat(repositoryRoot)
	if err != nil {
		return nil, fmt.Errorf("inspect repository root: %w", err)
	}
	if !beforeRoot.IsDir() || beforeRoot.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("repository root must be a directory, not a symbolic link or special file")
	}
	root, err := os.OpenRoot(repositoryRoot)
	if err != nil {
		return nil, fmt.Errorf("open repository root: %w", err)
	}
	defer root.Close()
	openedRoot, err := root.Stat(".")
	if err != nil || !openedRoot.IsDir() || !os.SameFile(beforeRoot, openedRoot) {
		return nil, fmt.Errorf("repository root changed during open")
	}
	harnessInfo, err := root.Lstat(".harness")
	if err != nil {
		return nil, fmt.Errorf("inspect processor manifest directory: %w", err)
	}
	if !harnessInfo.IsDir() || harnessInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("processor manifest directory must not be a symbolic link or special file")
	}
	harnessRoot, err := root.OpenRoot(".harness")
	if err != nil {
		return nil, fmt.Errorf("open processor manifest directory: %w", err)
	}
	defer harnessRoot.Close()
	openedHarness, err := harnessRoot.Stat(".")
	if err != nil || !openedHarness.IsDir() || !os.SameFile(harnessInfo, openedHarness) {
		return nil, fmt.Errorf("processor manifest directory changed during open")
	}
	const manifestName = "processors.json"
	before, err := harnessRoot.Lstat(manifestName)
	if err != nil {
		return nil, fmt.Errorf("inspect processor manifest: %w", err)
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("processor manifest must be a regular file, not a symbolic link or special file")
	}
	if before.Size() > maxManifestBytes {
		return nil, fmt.Errorf("processor manifest exceeds the byte limit")
	}
	file, err := harnessRoot.Open(manifestName)
	if err != nil {
		return nil, fmt.Errorf("open processor manifest: %w", err)
	}
	opened, statErr := file.Stat()
	if statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		if closeErr := file.Close(); closeErr != nil {
			return nil, fmt.Errorf("close changed processor manifest failed")
		}
		return nil, fmt.Errorf("processor manifest changed during open")
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxManifestBytes+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, fmt.Errorf("read processor manifest failed")
	}
	if len(data) > maxManifestBytes {
		return nil, fmt.Errorf("processor manifest exceeds the byte limit")
	}
	after, err := harnessRoot.Lstat(manifestName)
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return nil, fmt.Errorf("processor manifest changed during read")
	}
	afterHarnessHandle, handleErr := harnessRoot.Stat(".")
	afterHarnessPath, pathErr := root.Lstat(".harness")
	if handleErr != nil || pathErr != nil || !afterHarnessHandle.IsDir() || !afterHarnessPath.IsDir() ||
		!os.SameFile(harnessInfo, afterHarnessHandle) || !os.SameFile(harnessInfo, afterHarnessPath) {
		return nil, fmt.Errorf("processor manifest directory changed during read")
	}
	afterRootHandle, handleErr := root.Stat(".")
	afterRoot, err := os.Lstat(repositoryRoot)
	if handleErr != nil || err != nil || !afterRootHandle.IsDir() || !afterRoot.IsDir() ||
		!os.SameFile(beforeRoot, afterRootHandle) || !os.SameFile(beforeRoot, afterRoot) {
		return nil, fmt.Errorf("repository root changed during read")
	}
	return data, nil
}

func validText(name, value string) error {
	if value == "" || len(value) > maxTextBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return fmt.Errorf("processor manifest %s must be nonempty bounded UTF-8 without surrounding whitespace", name)
	}
	for _, character := range value {
		if unicode.Is(unicode.C, character) {
			return fmt.Errorf("processor manifest %s must not contain control characters", name)
		}
	}
	return nil
}

func validHTTPSURL(name, value string) error {
	if err := validText(name, value); err != nil {
		return err
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("processor manifest %s must be an absolute credential-free HTTPS URL without a fragment", name)
	}
	return nil
}

func safeBaseName(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, `/\\`) && filepath.Base(value) == value
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

func rejectDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bufio.NewReader(bytes.NewReader(data)))
	decoder.UseNumber()
	if err := consumeValue(decoder, "$", 0); err != nil {
		return err
	}
	return requireEOF(decoder)
}

func consumeValue(decoder *json.Decoder, path string, depth int) error {
	if depth > maxJSONDepth {
		return fmt.Errorf("JSON nesting exceeds %d levels", maxJSONDepth)
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key at %s is not a string", path)
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate object key %q at %s", key, path)
			}
			seen[key] = struct{}{}
			if err := consumeValue(decoder, path+"."+key, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("object at %s is not closed", path)
		}
	case '[':
		index := 0
		for decoder.More() {
			if err := consumeValue(decoder, fmt.Sprintf("%s[%d]", path, index), depth+1); err != nil {
				return err
			}
			index++
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("array at %s is not closed", path)
		}
	default:
		return fmt.Errorf("unexpected delimiter at %s", path)
	}
	return nil
}

func requireEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("multiple top-level JSON values are not allowed")
	}
	return fmt.Errorf("invalid trailing JSON: %w", err)
}
