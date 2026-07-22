// Package processorprocess defines the identity-bound, isolated process
// contract for finite external output processors.
package processorprocess

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	ObservationSchemaVersion = 1
	EnvironmentRTKIsolatedV1 = "atsura.processor.rtk_isolated.v1"

	MaxArguments       = 16
	MaxArgumentBytes   = 4096
	MaxInputBytes      = 4 * 1024 * 1024
	MaxStdoutBytes     = 4 * 1024 * 1024
	MaxStderrBytes     = 64 * 1024
	MaxTimeout         = 5 * time.Second
	MaxExecutableBytes = int64(512 * 1024 * 1024)
)

var (
	ErrInvalidIdentity     = errors.New("invalid processor identity")
	ErrInvalidRequest      = errors.New("invalid processor process request")
	ErrInvalidResult       = errors.New("invalid processor process result")
	ErrInvalidObservation  = errors.New("invalid processor observation")
	ErrUnsupportedPlatform = errors.New("unsupported processor platform")
	ErrUnsupportedVersion  = errors.New("unsupported processor version")
	ErrUnsupportedArtifact = errors.New("unsupported processor artifact")
	ErrInspectionFailed    = errors.New("processor inspection failed")
)

// Identity names the exact regular executable bytes authorized at a process
// boundary. The path is an absolute physical path rather than a PATH name.
type Identity struct {
	ResolvedPath string `json:"resolved_path"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
}

// IsZero reports whether no identity evidence is present.
func (i Identity) IsZero() bool {
	return i.ResolvedPath == "" && i.SHA256 == "" && i.Size == 0
}

// Validate rejects incomplete, relative, aliased, or malformed evidence.
func (i Identity) Validate() error {
	if err := ValidateExecutablePath(i.ResolvedPath); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidIdentity, err)
	}
	if !validSHA256(i.SHA256) {
		return fmt.Errorf("%w: executable SHA-256 must contain 64 lowercase hex characters", ErrInvalidIdentity)
	}
	if i.Size <= 0 || i.Size > MaxExecutableBytes {
		return fmt.Errorf("%w: executable size must be positive and at most %d bytes", ErrInvalidIdentity, MaxExecutableBytes)
	}
	return nil
}

// ValidateExecutablePath validates the public path shape before an adapter
// touches the filesystem. Infrastructure additionally rejects links and
// special files while identifying the path.
func ValidateExecutablePath(value string) error {
	if value == "" || len(value) > MaxArgumentBytes || !utf8.ValidString(value) {
		return fmt.Errorf("executable path must be non-empty bounded UTF-8")
	}
	if !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return fmt.Errorf("executable path must be absolute and clean")
	}
	return validateStructuralText(value)
}

// Adapter identifies one finite processor inspection contract.
type Adapter struct {
	Kind            string `json:"kind"`
	ContractVersion int    `json:"contract_version"`
}

// Platform binds an observation to one native operating-system and
// architecture tuple.
type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

// Probe records the exact bounded version observation used to construct an
// Observation. Args excludes the executable itself.
type Probe struct {
	Argv                []string `json:"argv"`
	EnvironmentContract string   `json:"environment_contract"`
	Attempts            int      `json:"attempts"`
}

// Observation is schema-1 canonical identity and version evidence for one
// explicitly selected processor executable.
type Observation struct {
	SchemaVersion int      `json:"schema_version"`
	Adapter       Adapter  `json:"adapter"`
	Platform      Platform `json:"platform"`
	Identity      Identity `json:"identity"`
	Version       string   `json:"version"`
	Probe         Probe    `json:"probe"`
}

// Validate rejects an incomplete or ambiguous processor observation.
func (o Observation) Validate() error {
	if o.SchemaVersion != ObservationSchemaVersion {
		return fmt.Errorf("%w: schema version must be %d", ErrInvalidObservation, ObservationSchemaVersion)
	}
	if !validNamespaced(o.Adapter.Kind) || o.Adapter.ContractVersion <= 0 {
		return fmt.Errorf("%w: adapter contract is invalid", ErrInvalidObservation)
	}
	if !validStableName(o.Platform.OS) || !validStableName(o.Platform.Arch) {
		return fmt.Errorf("%w: platform is invalid", ErrInvalidObservation)
	}
	if err := o.Identity.Validate(); err != nil {
		return fmt.Errorf("%w: identity: %v", ErrInvalidObservation, err)
	}
	if err := validateText(o.Version, 128); err != nil {
		return fmt.Errorf("%w: version: %v", ErrInvalidObservation, err)
	}
	if o.Probe.Argv == nil || len(o.Probe.Argv) == 0 || len(o.Probe.Argv) > MaxArguments {
		return fmt.Errorf("%w: probe args must be an explicit non-empty bounded list", ErrInvalidObservation)
	}
	for index, argument := range o.Probe.Argv {
		if err := validateArgvElement(argument); err != nil {
			return fmt.Errorf("%w: probe argument %d: %v", ErrInvalidObservation, index, err)
		}
	}
	if !validNamespaced(o.Probe.EnvironmentContract) {
		return fmt.Errorf("%w: probe environment contract is invalid", ErrInvalidObservation)
	}
	if o.Probe.Attempts != 1 {
		return fmt.Errorf("%w: probe attempts must equal one", ErrInvalidObservation)
	}
	return nil
}

// ValidateAdapterKind validates the namespaced identifier used by a finite
// application-owned processor registry.
func ValidateAdapterKind(value string) error {
	if !validNamespaced(value) {
		return fmt.Errorf("processor adapter kind is invalid")
	}
	return nil
}

// CanonicalJSON returns the one compact LF-terminated representation used for
// processor-observation digests and persisted evidence.
func (o Observation) CanonicalJSON() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(o)
	if err != nil {
		return nil, fmt.Errorf("encode canonical processor observation: %w", err)
	}
	return append(encoded, '\n'), nil
}

// Digest returns the lowercase SHA-256 of CanonicalJSON.
func (o Observation) Digest() (string, error) {
	encoded, err := o.CanonicalJSON()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded)), nil
}

// Request authorizes one no-shell processor attempt for an exact executable
// and isolated environment contract.
type Request struct {
	Executable          string
	Args                []string
	Input               []byte
	Timeout             time.Duration
	StdoutLimit         int
	StderrLimit         int
	ExpectedIdentity    Identity
	EnvironmentContract string
}

// Validate rejects an unbound, ambient, or unbounded processor request.
func (r Request) Validate() error {
	if err := ValidateExecutablePath(r.Executable); err != nil {
		return fmt.Errorf("%w: executable: %v", ErrInvalidRequest, err)
	}
	if err := r.ExpectedIdentity.Validate(); err != nil {
		return fmt.Errorf("%w: expected identity: %v", ErrInvalidRequest, err)
	}
	if r.Executable != r.ExpectedIdentity.ResolvedPath {
		return fmt.Errorf("%w: executable must equal the expected resolved path", ErrInvalidRequest)
	}
	if r.Args == nil || len(r.Args) > MaxArguments {
		return fmt.Errorf("%w: args must be an explicit bounded list", ErrInvalidRequest)
	}
	for index, argument := range r.Args {
		if err := validateArgvElement(argument); err != nil {
			return fmt.Errorf("%w: argument %d: %v", ErrInvalidRequest, index, err)
		}
	}
	if len(r.Input) > MaxInputBytes {
		return fmt.Errorf("%w: input exceeds %d bytes", ErrInvalidRequest, MaxInputBytes)
	}
	if r.Timeout <= 0 || r.Timeout > MaxTimeout {
		return fmt.Errorf("%w: timeout must be positive and at most %s", ErrInvalidRequest, MaxTimeout)
	}
	if r.StdoutLimit <= 0 || r.StdoutLimit > MaxStdoutBytes || r.StderrLimit <= 0 || r.StderrLimit > MaxStderrBytes {
		return fmt.Errorf("%w: output limits must be positive and within the supported maxima", ErrInvalidRequest)
	}
	if r.EnvironmentContract != EnvironmentRTKIsolatedV1 {
		return fmt.Errorf("%w: environment contract is not supported", ErrInvalidRequest)
	}
	return nil
}

// Result records facts from zero or one processor attempts. ExitCode is -1
// when there was no conventional completion.
type Result struct {
	Attempts int
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Identity Identity
}

// Validate proves attempt, capture, identity, and conventional-success facts.
// succeeded means the process port returned nil.
func (r Result) Validate(request Request, succeeded bool) error {
	if err := request.Validate(); err != nil {
		return fmt.Errorf("%w: request: %v", ErrInvalidResult, err)
	}
	if r.Attempts < 0 || r.Attempts > 1 {
		return fmt.Errorf("%w: attempts must be zero or one", ErrInvalidResult)
	}
	if len(r.Stdout) > request.StdoutLimit || len(r.Stderr) > request.StderrLimit {
		return fmt.Errorf("%w: captured output exceeds request bounds", ErrInvalidResult)
	}
	if r.Attempts == 0 {
		if succeeded || r.ExitCode != -1 || len(r.Stdout) != 0 || len(r.Stderr) != 0 || !r.Identity.IsZero() {
			return fmt.Errorf("%w: zero-attempt result contains process facts", ErrInvalidResult)
		}
		return nil
	}
	if r.Identity != request.ExpectedIdentity {
		return fmt.Errorf("%w: result identity does not match the bound request", ErrInvalidResult)
	}
	if succeeded && r.ExitCode != 0 {
		return fmt.Errorf("%w: successful result must have exit code zero", ErrInvalidResult)
	}
	if !succeeded && r.ExitCode < -1 {
		return fmt.Errorf("%w: failed exit code is invalid", ErrInvalidResult)
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func validNamespaced(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) < 3 {
		return false
	}
	for _, part := range parts {
		if !validStableName(part) {
			return false
		}
	}
	return true
}

func validStableName(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	for index, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case index > 0 && r >= '0' && r <= '9':
		case index > 0 && r == '_':
		default:
			return false
		}
	}
	return true
}

func validateText(value string, maxBytes int) error {
	if value == "" || len(value) > maxBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return fmt.Errorf("must be non-empty bounded structural text")
	}
	return validateStructuralText(value)
}

func validateArgvElement(value string) error {
	if len(value) > MaxArgumentBytes || !utf8.ValidString(value) {
		return fmt.Errorf("must be bounded UTF-8")
	}
	return validateStructuralText(value)
}

func validateStructuralText(value string) error {
	if strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029'
	}) >= 0 {
		return fmt.Errorf("contains unsupported structural text")
	}
	return nil
}
