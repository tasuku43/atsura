package sourceprocess

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func validRequest() Request {
	return Request{Executable: "source", Args: []string{"list", "--json"}, Timeout: 30 * time.Second, StdoutLimit: 1024, StderrLimit: 256}
}

func validIdentity() Identity {
	return Identity{ResolvedPath: "/synthetic/source", SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", Size: 42}
}

func TestRequestValidationFailsClosed(t *testing.T) {
	tests := map[string]func(*Request){
		"executable":  func(value *Request) { value.Executable = "" },
		"args nil":    func(value *Request) { value.Args = nil },
		"unsafe arg":  func(value *Request) { value.Args[0] = "bad\narg" },
		"timeout":     func(value *Request) { value.Timeout = 0 },
		"timeout max": func(value *Request) { value.Timeout = MaxTimeout + time.Nanosecond },
		"stdout":      func(value *Request) { value.StdoutLimit = 0 },
		"stdout max":  func(value *Request) { value.StdoutLimit = MaxStdoutBytes + 1 },
		"stderr":      func(value *Request) { value.StderrLimit = 0 },
		"stderr max":  func(value *Request) { value.StderrLimit = MaxStderrBytes + 1 },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			value := validRequest()
			mutate(&value)
			if err := value.Validate(); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestRequestPreservesEmptyArgvElementsButNotAnEmptyExecutable(t *testing.T) {
	request := validRequest()
	request.Args = []string{"", "value", ""}
	if err := request.Validate(); err != nil {
		t.Fatalf("empty argv elements must remain representable: %v", err)
	}

	request.Executable = ""
	if err := request.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("empty executable error = %v", err)
	}
}

func TestRequestStillBoundsEveryArgvElement(t *testing.T) {
	tests := []string{
		strings.Repeat("x", MaxArgumentBytes+1),
		"bad\narg",
		string([]byte{0xff}),
	}
	for index, argument := range tests {
		request := validRequest()
		request.Args = []string{argument}
		if err := request.Validate(); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("argument %d error = %v", index, err)
		}
	}
}

func TestIdentityValidationEnforcesExecutableSizeBound(t *testing.T) {
	value := validIdentity()
	value.Size = MaxExecutableBytes + 1
	if err := value.Validate(); err == nil {
		t.Fatal("oversized executable identity passed validation")
	}
}

func TestResultValidationDistinguishesZeroAndOneAttempt(t *testing.T) {
	request := validRequest()
	zero := Result{ExitCode: -1}
	if err := zero.Validate(request, false); err != nil {
		t.Fatalf("zero result: %v", err)
	}
	one := Result{Attempts: 1, ExitCode: 0, Stdout: []byte(`[]`), Identity: validIdentity()}
	if err := one.Validate(request, true); err != nil {
		t.Fatalf("one result: %v", err)
	}

	invalid := []struct {
		result    Result
		succeeded bool
	}{
		{result: Result{Attempts: 2, ExitCode: -1}},
		{result: Result{ExitCode: 0}},
		{result: Result{Attempts: 1, ExitCode: 1, Identity: validIdentity()}, succeeded: true},
		{result: Result{Attempts: 1, ExitCode: 0}},
		{result: Result{Attempts: 1, ExitCode: 0, Stdout: make([]byte, request.StdoutLimit+1), Identity: validIdentity()}, succeeded: true},
	}
	for index, test := range invalid {
		if err := test.result.Validate(request, test.succeeded); !errors.Is(err, ErrInvalidResult) {
			t.Errorf("invalid result %d error = %v", index, err)
		}
	}
}

func TestBoundRequestAndResultRequireExactExpectedIdentity(t *testing.T) {
	identity := validIdentity()
	request := validRequest()
	request.Executable = identity.ResolvedPath
	bound := BoundRequest{Process: request, ExpectedIdentity: identity}
	if err := bound.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	result := Result{Attempts: 1, ExitCode: 0, Stdout: []byte(`[]`), Identity: identity}
	if err := result.ValidateBound(bound, true); err != nil {
		t.Fatalf("ValidateBound() = %v", err)
	}

	wrongPath := bound
	wrongPath.Process.Executable = "/synthetic/other"
	if err := wrongPath.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("wrong path error = %v", err)
	}
	wrongHash := bound
	wrongHash.ExpectedIdentity.SHA256 = "1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := result.ValidateBound(wrongHash, true); !errors.Is(err, ErrInvalidResult) {
		t.Fatalf("wrong identity error = %v", err)
	}
	zero := Result{ExitCode: -1}
	if err := zero.ValidateBound(bound, false); err != nil {
		t.Fatalf("zero-attempt bound result = %v", err)
	}
}

func TestValidateBoundCompletionAcceptsConventionalZeroAndNonzeroStatus(t *testing.T) {
	identity := validIdentity()
	request := validRequest()
	request.Executable = identity.ResolvedPath
	bound := BoundRequest{Process: request, ExpectedIdentity: identity}
	for _, exitCode := range []int{0, 7} {
		result := Result{
			Attempts: 1,
			ExitCode: exitCode,
			Stdout:   []byte{'o', 0xff},
			Stderr:   []byte{'e', 0x00},
			Identity: identity,
		}
		if err := result.ValidateBoundCompletion(bound); err != nil {
			t.Fatalf("exit %d completion error = %v", exitCode, err)
		}
	}
}

func TestValidateBoundCompletionRejectsUncertainOrInconsistentEvidence(t *testing.T) {
	identity := validIdentity()
	request := validRequest()
	request.Executable = identity.ResolvedPath
	bound := BoundRequest{Process: request, ExpectedIdentity: identity}
	wrongIdentity := identity
	wrongIdentity.SHA256 = "1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	tests := []Result{
		{ExitCode: -1},
		{Attempts: 1, ExitCode: -1, Identity: identity},
		{Attempts: 1, ExitCode: 0, Identity: wrongIdentity},
		{Attempts: 1, ExitCode: 0, Stdout: make([]byte, request.StdoutLimit+1), Identity: identity},
		{Attempts: 1, ExitCode: 0, Stderr: make([]byte, request.StderrLimit+1), Identity: identity},
	}
	for index, result := range tests {
		if err := result.ValidateBoundCompletion(bound); !errors.Is(err, ErrInvalidResult) {
			t.Errorf("result %d error = %v", index, err)
		}
	}
}
