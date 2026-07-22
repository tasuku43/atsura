package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	capabilitiesPath = ".harness/capabilities.json"
	processorsPath   = ".harness/processors.json"
	schemasPath      = ".harness/schemas.json"

	processorManifestSchemaVersion = 1
	rtkContractID                  = "atsura.output.rtk_go_test_pass.v1"
	rtkKind                        = "atsura.processor.rtk"
	rtkVersion                     = "0.43.0"
	rtkUpstreamCommit              = "5a7880d404db8364d602f2ecdc41dd790f64013f"
	rtkReleaseURL                  = "https://github.com/rtk-ai/rtk/releases/tag/v0.43.0"
	rtkChecksumsURL                = "https://github.com/rtk-ai/rtk/releases/download/v0.43.0/checksums.txt"
	rtkChecksumsSHA256             = "b7f973a9693b0cb3de894ec71f74003992080cabcd5b039b9510ed3b299ed5bc"
	rtkLicenseSPDX                 = "Apache-2.0"
	rtkLicenseURL                  = "https://raw.githubusercontent.com/rtk-ai/rtk/5a7880d404db8364d602f2ecdc41dd790f64013f/LICENSE"
	rtkLicenseSHA256               = "4044ade9c21d8b084d3d16a03375cf3b7e166b946a327bb37a3fbbdb53287cfd"
	rtkNoticeStatus                = "absent_upstream"
	rtkDistribution                = "external_user_supplied_not_bundled"
	rtkSBOMReview                  = "not_provided_external_dependency"
)

var contractID = regexp.MustCompile(`^[a-z][a-z0-9-]*\.[a-z][a-z0-9-]*(?:\.[a-z][a-z0-9-]*)*$`)

type issue struct {
	Path    string
	Message string
}

type capability struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type schemaFixture struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Provenance string `json:"provenance"`
	License    string `json:"license"`
}

type processorManifest struct {
	SchemaVersion int                   `json:"schema_version"`
	Processors    []processorProvenance `json:"processors"`
}

type processorProvenance struct {
	ContractID     string              `json:"contract_id"`
	Kind           string              `json:"kind"`
	Version        string              `json:"version"`
	UpstreamCommit string              `json:"upstream_commit"`
	ReleaseURL     string              `json:"release_url"`
	Checksums      processorChecksums  `json:"checksums"`
	License        processorLicense    `json:"license"`
	Notice         processorNotice     `json:"notice"`
	Distribution   string              `json:"distribution"`
	SBOMReview     string              `json:"sbom_review"`
	Artifacts      []processorArtifact `json:"artifacts"`
}

type processorChecksums struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type processorLicense struct {
	SPDX   string `json:"spdx"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type processorNotice struct {
	Status string `json:"status"`
}

type processorArtifact struct {
	Target        string `json:"target"`
	ArchiveName   string `json:"archive_name"`
	ArchiveURL    string `json:"archive_url"`
	ArchiveSHA256 string `json:"archive_sha256"`
	ArchiveSize   int64  `json:"archive_size"`
	BinaryMember  string `json:"binary_member"`
	BinarySHA256  string `json:"binary_sha256"`
	BinarySize    int64  `json:"binary_size"`
}

func inspectContracts(root string, catalogIDs map[string]struct{}) ([]issue, error) {
	capabilities, err := loadStrictArray[capability](root, capabilitiesPath)
	if err != nil {
		return nil, err
	}
	schemas, err := loadStrictArray[schemaFixture](root, schemasPath)
	if err != nil {
		return nil, err
	}
	processors, err := loadStrictObject[processorManifest](root, processorsPath)
	if err != nil {
		return nil, err
	}
	issues := validateCapabilities(capabilities, catalogIDs)
	issues = append(issues, validateSchemas(root, schemas)...)
	issues = append(issues, validateProcessorManifest(processors)...)
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Path != issues[j].Path {
			return issues[i].Path < issues[j].Path
		}
		return issues[i].Message < issues[j].Message
	})
	return issues, nil
}

func validateProcessorManifest(manifest processorManifest) []issue {
	var issues []issue
	if manifest.SchemaVersion != processorManifestSchemaVersion {
		issues = append(issues, issue{Path: processorsPath + ".schema_version", Message: "schema_version must be exactly 1"})
	}
	if len(manifest.Processors) != 1 {
		issues = append(issues, issue{Path: processorsPath + ".processors", Message: "processors must contain exactly the one ADR 0012 RTK contract"})
		if len(manifest.Processors) == 0 {
			return issues
		}
	}

	actual := manifest.Processors[0]
	expected := expectedRTKProcessor()
	processorLocation := processorsPath + ".processors[0]"
	for _, value := range []struct {
		field string
		got   string
		want  string
	}{
		{field: "contract_id", got: actual.ContractID, want: expected.ContractID},
		{field: "kind", got: actual.Kind, want: expected.Kind},
		{field: "version", got: actual.Version, want: expected.Version},
		{field: "upstream_commit", got: actual.UpstreamCommit, want: expected.UpstreamCommit},
		{field: "release_url", got: actual.ReleaseURL, want: expected.ReleaseURL},
		{field: "checksums.url", got: actual.Checksums.URL, want: expected.Checksums.URL},
		{field: "checksums.sha256", got: actual.Checksums.SHA256, want: expected.Checksums.SHA256},
		{field: "license.spdx", got: actual.License.SPDX, want: expected.License.SPDX},
		{field: "license.url", got: actual.License.URL, want: expected.License.URL},
		{field: "license.sha256", got: actual.License.SHA256, want: expected.License.SHA256},
		{field: "notice.status", got: actual.Notice.Status, want: expected.Notice.Status},
		{field: "distribution", got: actual.Distribution, want: expected.Distribution},
		{field: "sbom_review", got: actual.SBOMReview, want: expected.SBOMReview},
	} {
		if value.got != value.want {
			issues = append(issues, issue{
				Path:    processorLocation + "." + value.field,
				Message: fmt.Sprintf("must equal the ADR 0012 pinned value %q", value.want),
			})
		}
	}

	if len(actual.Artifacts) != len(expected.Artifacts) {
		issues = append(issues, issue{
			Path:    processorLocation + ".artifacts",
			Message: "artifacts must contain exactly linux/amd64, linux/arm64, darwin/amd64, and darwin/arm64 in that order; Windows is not claimed",
		})
	}
	for index := 0; index < len(actual.Artifacts) && index < len(expected.Artifacts); index++ {
		got := actual.Artifacts[index]
		want := expected.Artifacts[index]
		location := fmt.Sprintf("%s.artifacts[%d]", processorLocation, index)
		for _, value := range []struct {
			field string
			got   string
			want  string
		}{
			{field: "target", got: got.Target, want: want.Target},
			{field: "archive_name", got: got.ArchiveName, want: want.ArchiveName},
			{field: "archive_url", got: got.ArchiveURL, want: want.ArchiveURL},
			{field: "archive_sha256", got: got.ArchiveSHA256, want: want.ArchiveSHA256},
			{field: "binary_member", got: got.BinaryMember, want: want.BinaryMember},
			{field: "binary_sha256", got: got.BinarySHA256, want: want.BinarySHA256},
		} {
			if value.got != value.want {
				issues = append(issues, issue{
					Path:    location + "." + value.field,
					Message: fmt.Sprintf("must equal the ADR 0012 pinned value %q", value.want),
				})
			}
		}
		for _, value := range []struct {
			field string
			got   int64
			want  int64
		}{
			{field: "archive_size", got: got.ArchiveSize, want: want.ArchiveSize},
			{field: "binary_size", got: got.BinarySize, want: want.BinarySize},
		} {
			if value.got != value.want {
				issues = append(issues, issue{
					Path:    location + "." + value.field,
					Message: fmt.Sprintf("must equal the ADR 0012 pinned value %d", value.want),
				})
			}
		}
	}
	return issues
}

func expectedRTKProcessor() processorProvenance {
	const releaseDownload = "https://github.com/rtk-ai/rtk/releases/download/v0.43.0/"
	return processorProvenance{
		ContractID:     rtkContractID,
		Kind:           rtkKind,
		Version:        rtkVersion,
		UpstreamCommit: rtkUpstreamCommit,
		ReleaseURL:     rtkReleaseURL,
		Checksums:      processorChecksums{URL: rtkChecksumsURL, SHA256: rtkChecksumsSHA256},
		License:        processorLicense{SPDX: rtkLicenseSPDX, URL: rtkLicenseURL, SHA256: rtkLicenseSHA256},
		Notice:         processorNotice{Status: rtkNoticeStatus},
		Distribution:   rtkDistribution,
		SBOMReview:     rtkSBOMReview,
		Artifacts: []processorArtifact{
			{
				Target: "linux/amd64", ArchiveName: "rtk-x86_64-unknown-linux-musl.tar.gz",
				ArchiveURL: releaseDownload + "rtk-x86_64-unknown-linux-musl.tar.gz", ArchiveSHA256: "ff8a1e7766496e175291a85aeca1dc97c9ff6df33e51e5893d1fbc78fea2a609", ArchiveSize: 4460416,
				BinaryMember: "rtk", BinarySHA256: "f160611f3baee17fe4eb3a04c56a8bc3d15fec4274d8838016088d4776c6f628", BinarySize: 10083968,
			},
			{
				Target: "linux/arm64", ArchiveName: "rtk-aarch64-unknown-linux-gnu.tar.gz",
				ArchiveURL: releaseDownload + "rtk-aarch64-unknown-linux-gnu.tar.gz", ArchiveSHA256: "5519f7ca12e5c143a609f0d28a0a77b97413a8dce31c2681f1a41c24519a8731", ArchiveSize: 4087098,
				BinaryMember: "rtk", BinarySHA256: "86bd2badb697e41fa4fae805ed1a42d9b2495600260918d6ba9c148bc40013cf", BinarySize: 8544624,
			},
			{
				Target: "darwin/amd64", ArchiveName: "rtk-x86_64-apple-darwin.tar.gz",
				ArchiveURL: releaseDownload + "rtk-x86_64-apple-darwin.tar.gz", ArchiveSHA256: "a85f60e2637811be68366208b8d8b9c5ba1b748cb5df4477ab20cd73d3c5d9f8", ArchiveSize: 4139835,
				BinaryMember: "rtk", BinarySHA256: "22adaa27b3fd6d8906159ba3ff7ca8346e914df112408bcc7a88cda30a3a6107", BinarySize: 9006316,
			},
			{
				Target: "darwin/arm64", ArchiveName: "rtk-aarch64-apple-darwin.tar.gz",
				ArchiveURL: releaseDownload + "rtk-aarch64-apple-darwin.tar.gz", ArchiveSHA256: "8a17e49acbd378997eb21d0eb6f7f861111f35b4fc9b1c74edf4c7448e576c65", ArchiveSize: 3759961,
				BinaryMember: "rtk", BinarySHA256: "2dab449f32ea744c30b02a3ef9806e3e7d3b356a145332f3f2aaabb5ea48edee", BinarySize: 7763408,
			},
		},
	}
}

func validateCapabilities(entries []capability, catalogIDs map[string]struct{}) []issue {
	seen := make(map[string]int, len(entries))
	public := make(map[string]struct{})
	var issues []issue
	for index, entry := range entries {
		location := fmt.Sprintf("%s[%d]", capabilitiesPath, index)
		if !validContractID(entry.ID) {
			issues = append(issues, issue{Path: location, Message: "id must use lowercase dot syntax within 128 bytes, for example items.read"})
		}
		if first, exists := seen[entry.ID]; exists {
			issues = append(issues, issue{Path: location, Message: fmt.Sprintf("duplicate capability id %q; first declared at index %d", entry.ID, first)})
		} else {
			seen[entry.ID] = index
		}
		switch entry.Status {
		case "public":
			public[entry.ID] = struct{}{}
			if _, exists := catalogIDs[entry.ID]; !exists {
				issues = append(issues, issue{Path: location, Message: fmt.Sprintf("public capability %q has no command catalog entry; add its CapabilityID to a command or change its status", entry.ID)})
			}
		case "internal", "deferred", "excluded":
			if strings.TrimSpace(entry.Reason) == "" {
				issues = append(issues, issue{Path: location, Message: fmt.Sprintf("%s capability %q requires a non-empty reason", entry.Status, entry.ID)})
			}
			if _, exists := catalogIDs[entry.ID]; exists {
				issues = append(issues, issue{Path: location, Message: fmt.Sprintf("non-public capability %q is exposed by the command catalog; mark it public or remove the catalog exposure", entry.ID)})
			}
		default:
			issues = append(issues, issue{Path: location, Message: "status must be public, internal, deferred, or excluded"})
		}
		if err := validatePublicText("reason", entry.Reason, false); err != nil {
			issues = append(issues, issue{Path: location, Message: err.Error()})
		}
	}
	for id := range catalogIDs {
		if _, exists := public[id]; !exists {
			issues = append(issues, issue{
				Path:    capabilitiesPath,
				Message: fmt.Sprintf("catalog capability %q is not declared public; add one public ledger entry", id),
			})
		}
	}
	return issues
}

func validateSchemas(root string, entries []schemaFixture) []issue {
	seenIDs := make(map[string]int, len(entries))
	seenPaths := make(map[string]int, len(entries))
	var issues []issue
	for index, entry := range entries {
		location := fmt.Sprintf("%s[%d]", schemasPath, index)
		if !validContractID(entry.ID) {
			issues = append(issues, issue{Path: location, Message: "id must use lowercase dot syntax within 128 bytes, for example provider.v1"})
		}
		if first, exists := seenIDs[entry.ID]; exists {
			issues = append(issues, issue{Path: location, Message: fmt.Sprintf("duplicate schema id %q; first declared at index %d", entry.ID, first)})
		} else {
			seenIDs[entry.ID] = index
		}
		if first, exists := seenPaths[entry.Path]; exists {
			issues = append(issues, issue{Path: location, Message: fmt.Sprintf("duplicate schema path %q; first declared at index %d", entry.Path, first)})
		} else {
			seenPaths[entry.Path] = index
		}
		for name, value := range map[string]string{
			"provenance": entry.Provenance,
			"license":    entry.License,
		} {
			if err := validatePublicText(name, value, true); err != nil {
				issues = append(issues, issue{Path: location, Message: err.Error()})
			}
		}

		fixture, pathIssues := readSchemaFixture(root, entry.Path)
		for _, message := range pathIssues {
			issues = append(issues, issue{Path: location, Message: message})
		}
		if len(pathIssues) != 0 {
			continue
		}
		if !validDigest(entry.SHA256) {
			issues = append(issues, issue{Path: location, Message: "sha256 must be exactly 64 lowercase hexadecimal characters"})
			continue
		}
		actual := sha256.Sum256(fixture)
		actualText := hex.EncodeToString(actual[:])
		if actualText != entry.SHA256 {
			issues = append(issues, issue{
				Path:    location,
				Message: fmt.Sprintf("sha256 mismatch for %q: manifest has %s, computed %s; review the fixture change and update the digest deliberately", entry.Path, entry.SHA256, actualText),
			})
		}
	}
	return issues
}

func readSchemaFixture(root, relative string) ([]byte, []string) {
	if relative == "" {
		return nil, []string{"path is required"}
	}
	if strings.Contains(relative, `\`) || filepath.IsAbs(relative) || !filepath.IsLocal(relative) || filepath.ToSlash(filepath.Clean(relative)) != relative {
		return nil, []string{"path must be a canonical repository-relative path without traversal"}
	}
	parts := strings.Split(relative, "/")
	hasTestdata := false
	for _, part := range parts {
		if part == "testdata" {
			hasTestdata = true
		}
		if part == "" || strings.HasPrefix(part, ".") {
			return nil, []string{"path must not contain empty or hidden components"}
		}
	}
	if !hasTestdata {
		return nil, []string{"path must identify a publishable fixture below a testdata directory"}
	}
	lowerBase := strings.ToLower(filepath.Base(relative))
	if forbiddenFixtureName(lowerBase) {
		return nil, []string{"path looks credential-bearing; schema fixtures must be publishable test data"}
	}

	rootInfo, err := os.Lstat(root)
	if err != nil {
		return nil, []string{fmt.Sprintf("cannot inspect repository root: %v", err)}
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, []string{"repository root must be a regular directory, not a symbolic link"}
	}
	current := root
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, []string{fmt.Sprintf("fixture %q cannot be inspected: %v", relative, err)}
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, []string{fmt.Sprintf("fixture path %q contains a symbolic link", relative)}
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, []string{fmt.Sprintf("fixture path component in %q is not a directory", relative)}
		}
		if index == len(parts)-1 && !info.Mode().IsRegular() {
			return nil, []string{fmt.Sprintf("fixture %q is not a regular file", relative)}
		}
	}
	data, err := os.ReadFile(current) // #nosec G304 -- every repository-relative component was validated with Lstat above.
	if err != nil {
		return nil, []string{fmt.Sprintf("fixture %q cannot be read: %v", relative, err)}
	}
	return data, nil
}

func forbiddenFixtureName(base string) bool {
	switch base {
	case ".env", "credentials.json", "secrets.json", "id_rsa", "id_ed25519":
		return true
	}
	for _, suffix := range []string{".pem", ".key", ".p12", ".pfx"} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func validContractID(value string) bool {
	return len(value) > 0 && len(value) <= 128 && contractID.MatchString(value)
}

func validatePublicText(name, value string, required bool) error {
	if value == "" {
		if required {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
	if len(value) > 2048 || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return fmt.Errorf("%s must be bounded, valid UTF-8 without surrounding whitespace", name)
	}
	for _, r := range value {
		if unicode.Is(unicode.C, r) {
			return fmt.Errorf("%s must not contain control characters", name)
		}
	}
	return nil
}

func loadStrictArray[T any](root, relative string) ([]T, error) {
	data, err := readRegularManifest(root, relative)
	if err != nil {
		return nil, err
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return nil, fmt.Errorf("%s: invalid strict JSON: %w", relative, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var entries []T
	if err := decoder.Decode(&entries); err != nil {
		return nil, fmt.Errorf("%s: decode strict JSON array: %w", relative, err)
	}
	if entries == nil {
		return nil, fmt.Errorf("%s: top level must be an array; use [] when there are no entries", relative)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("%s: %w", relative, err)
	}
	return entries, nil
}

func loadStrictObject[T any](root, relative string) (T, error) {
	var zero T
	data, err := readRegularManifest(root, relative)
	if err != nil {
		return zero, err
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return zero, fmt.Errorf("%s: invalid strict JSON: %w", relative, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value *T
	if err := decoder.Decode(&value); err != nil {
		return zero, fmt.Errorf("%s: decode strict JSON object: %w", relative, err)
	}
	if value == nil {
		return zero, fmt.Errorf("%s: top level must be an object", relative)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return zero, fmt.Errorf("%s: %w", relative, err)
	}
	return *value, nil
}

func readRegularManifest(root, relative string) ([]byte, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect repository root for %s: %w", relative, err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s: repository root must be a regular directory, not a symbolic link", relative)
	}
	current := root
	parts := strings.Split(filepath.FromSlash(relative), string(filepath.Separator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", relative, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%s: manifest path contains a symbolic link", relative)
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, fmt.Errorf("%s: manifest path component is not a directory", relative)
		}
		if index == len(parts)-1 && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s: manifest must be a regular file", relative)
		}
	}
	data, err := os.ReadFile(current) // #nosec G304 -- this fixed manifest path and all of its components were validated above.
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", relative, err)
	}
	return data, nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bufio.NewReader(bytes.NewReader(data)))
	decoder.UseNumber()
	if err := consumeJSONValue(decoder, "$", 0); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

func consumeJSONValue(decoder *json.Decoder, path string, depth int) error {
	if depth > 128 {
		return fmt.Errorf("JSON nesting exceeds 128 levels at %s", path)
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
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
			if err := consumeJSONValue(decoder, path+"."+key, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim('}') {
			return fmt.Errorf("object at %s is not closed", path)
		}
	case '[':
		index := 0
		for decoder.More() {
			if err := consumeJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index), depth+1); err != nil {
				return err
			}
			index++
		}
		closing, err := decoder.Token()
		if err != nil {
			return err
		}
		if closing != json.Delim(']') {
			return fmt.Errorf("array at %s is not closed", path)
		}
	default:
		return fmt.Errorf("unexpected delimiter %q at %s", delimiter, path)
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
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
