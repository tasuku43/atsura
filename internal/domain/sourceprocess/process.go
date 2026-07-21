// Package sourceprocess defines the bounded direct-process contract used by
// Atsura's local tailoring runner.
package sourceprocess

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	MaxArguments       = 127
	MaxArgumentBytes   = 4096
	MaxTimeout         = 30 * time.Second
	MaxStdoutBytes     = 4 * 1024 * 1024
	MaxStderrBytes     = 256 * 1024
	MaxExecutableBytes = int64(512 * 1024 * 1024)
)

var (
	ErrInvalidRequest = errors.New("invalid source process request")
	ErrInvalidResult  = errors.New("invalid source process result")
)

// Request is one no-shell process attempt with explicit finite bounds.
type Request struct {
	Executable  string
	Args        []string
	Timeout     time.Duration
	StdoutLimit int
	StderrLimit int
}

// BoundRequest binds one process request to executable identity evidence that
// was established outside the process adapter. Bundle runtime uses this form;
// inspection uses Request because discovering identity is part of that task.
type BoundRequest struct {
	Process          Request
	ExpectedIdentity Identity
}

// Validate rejects a process request that could resolve to anything other
// than its expected executable bytes.
func (r BoundRequest) Validate() error {
	if err := r.Process.Validate(); err != nil {
		return err
	}
	if err := r.ExpectedIdentity.Validate(); err != nil {
		return fmt.Errorf("%w: expected identity: %v", ErrInvalidRequest, err)
	}
	if r.Process.Executable != r.ExpectedIdentity.ResolvedPath {
		return fmt.Errorf("%w: executable must equal the expected resolved path", ErrInvalidRequest)
	}
	return nil
}

// Validate rejects an incomplete or unbounded process request.
func (r Request) Validate() error {
	if err := validateArgument(r.Executable); err != nil {
		return fmt.Errorf("%w: executable: %v", ErrInvalidRequest, err)
	}
	if r.Args == nil || len(r.Args) > MaxArguments {
		return fmt.Errorf("%w: args must be an explicit bounded list", ErrInvalidRequest)
	}
	for index, argument := range r.Args {
		if err := validateArgument(argument); err != nil {
			return fmt.Errorf("%w: argument %d: %v", ErrInvalidRequest, index, err)
		}
	}
	if r.Timeout <= 0 || r.Timeout > MaxTimeout {
		return fmt.Errorf("%w: timeout must be positive and at most %s", ErrInvalidRequest, MaxTimeout)
	}
	if r.StdoutLimit <= 0 || r.StdoutLimit > MaxStdoutBytes || r.StderrLimit <= 0 || r.StderrLimit > MaxStderrBytes {
		return fmt.Errorf("%w: output limits must be positive and within the supported maxima", ErrInvalidRequest)
	}
	return nil
}

// Identity is private execution evidence for one resolved executable.
type Identity struct {
	ResolvedPath string
	SHA256       string
	Size         int64
}

// IsZero reports whether no identity evidence exists.
func (i Identity) IsZero() bool {
	return i.ResolvedPath == "" && i.SHA256 == "" && i.Size == 0
}

// Validate rejects relative, malformed, or incomplete identity evidence.
func (i Identity) Validate() error {
	if i.ResolvedPath == "" || !filepath.IsAbs(i.ResolvedPath) || filepath.Clean(i.ResolvedPath) != i.ResolvedPath {
		return fmt.Errorf("resolved executable path must be absolute and clean")
	}
	if len(i.SHA256) != 64 {
		return fmt.Errorf("executable SHA-256 must contain 64 lowercase hex characters")
	}
	for _, r := range i.SHA256 {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return fmt.Errorf("executable SHA-256 must contain 64 lowercase hex characters")
		}
	}
	if i.Size <= 0 || i.Size > MaxExecutableBytes {
		return fmt.Errorf("executable size must be positive and at most %d bytes", MaxExecutableBytes)
	}
	return nil
}

// Result records facts from zero or one direct process attempts. ExitCode is
// -1 when no conventional exit code exists.
type Result struct {
	Attempts int
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Identity Identity
}

// Validate proves attempt count, capture bounds, and success identity before
// the application interprets a result. succeeded means the port returned nil.
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
	if err := r.Identity.Validate(); err != nil {
		return fmt.Errorf("%w: identity: %v", ErrInvalidResult, err)
	}
	if succeeded && r.ExitCode != 0 {
		return fmt.Errorf("%w: successful result must have exit code zero", ErrInvalidResult)
	}
	if !succeeded && r.ExitCode < -1 {
		return fmt.Errorf("%w: failed exit code is invalid", ErrInvalidResult)
	}
	return nil
}

// ValidateBound additionally proves that a one-attempt result belongs to the
// exact identity authorized by a bound request.
func (r Result) ValidateBound(request BoundRequest, succeeded bool) error {
	if err := request.Validate(); err != nil {
		return fmt.Errorf("%w: request: %v", ErrInvalidResult, err)
	}
	if err := r.Validate(request.Process, succeeded); err != nil {
		return err
	}
	if r.Attempts == 1 && r.Identity != request.ExpectedIdentity {
		return fmt.Errorf("%w: result identity does not match the bound request", ErrInvalidResult)
	}
	return nil
}

func validateArgument(value string) error {
	if value == "" || len(value) > MaxArgumentBytes || !utf8.ValidString(value) {
		return fmt.Errorf("must be non-empty bounded UTF-8")
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029'
	}) >= 0 {
		return fmt.Errorf("contains unsupported structural text")
	}
	return nil
}
