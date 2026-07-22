package processorprocess

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixtureIdentity(t *testing.T) Identity {
	t.Helper()
	return Identity{
		ResolvedPath: filepath.Join(t.TempDir(), "rtk"),
		SHA256:       strings.Repeat("a", 64),
		Size:         42,
	}
}

func fixtureObservation(t *testing.T) Observation {
	t.Helper()
	return Observation{
		SchemaVersion: ObservationSchemaVersion,
		Adapter:       Adapter{Kind: "atsura.processor.rtk", ContractVersion: 1},
		Platform:      Platform{OS: "darwin", Arch: "arm64"},
		Identity:      fixtureIdentity(t),
		Version:       "0.43.0",
		Probe: Probe{
			Argv:                []string{"--version"},
			EnvironmentContract: EnvironmentRTKIsolatedV1,
			Attempts:            1,
		},
	}
}

func fixtureRequest(t *testing.T) Request {
	t.Helper()
	identity := fixtureIdentity(t)
	return Request{
		Executable:          identity.ResolvedPath,
		Args:                []string{"pipe", "--filter=go-test"},
		Input:               []byte("bounded input\n"),
		Timeout:             MaxTimeout,
		StdoutLimit:         MaxStdoutBytes,
		StderrLimit:         MaxStderrBytes,
		ExpectedIdentity:    identity,
		EnvironmentContract: EnvironmentRTKIsolatedV1,
	}
}

func TestObservationCanonicalJSONAndDigestAreDeterministic(t *testing.T) {
	observation := fixtureObservation(t)
	first, err := observation.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	second, err := observation.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) || first[len(first)-1] != '\n' || strings.Contains(string(first[:len(first)-1]), "\n") {
		t.Fatalf("canonical bytes = %q", first)
	}
	firstDigest, err := observation.Digest()
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, _ := observation.Digest()
	if len(firstDigest) != 64 || firstDigest != secondDigest {
		t.Fatalf("digests = %q, %q", firstDigest, secondDigest)
	}
}

func TestObservationRejectsEveryIncompleteContractDimension(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Observation)
	}{
		{name: "schema", mutate: func(v *Observation) { v.SchemaVersion = 2 }},
		{name: "adapter", mutate: func(v *Observation) { v.Adapter.Kind = "rtk" }},
		{name: "adapter version", mutate: func(v *Observation) { v.Adapter.ContractVersion = 0 }},
		{name: "platform os", mutate: func(v *Observation) { v.Platform.OS = "" }},
		{name: "platform arch", mutate: func(v *Observation) { v.Platform.Arch = "ARM64" }},
		{name: "identity", mutate: func(v *Observation) { v.Identity.SHA256 = strings.Repeat("A", 64) }},
		{name: "version", mutate: func(v *Observation) { v.Version = " 0.43.0" }},
		{name: "nil args", mutate: func(v *Observation) { v.Probe.Argv = nil }},
		{name: "empty args", mutate: func(v *Observation) { v.Probe.Argv = []string{} }},
		{name: "unsafe arg", mutate: func(v *Observation) { v.Probe.Argv = []string{"--version\n"} }},
		{name: "environment", mutate: func(v *Observation) { v.Probe.EnvironmentContract = "isolated" }},
		{name: "attempts", mutate: func(v *Observation) { v.Probe.Attempts = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := fixtureObservation(t)
			test.mutate(&value)
			if err := value.Validate(); !errors.Is(err, ErrInvalidObservation) {
				t.Fatalf("Validate() = %v", err)
			}
		})
	}
}

func TestIdentityRequiresAbsoluteCleanLowercaseBoundedEvidence(t *testing.T) {
	identity := fixtureIdentity(t)
	if err := identity.Validate(); err != nil {
		t.Fatal(err)
	}
	tests := []Identity{
		{},
		{ResolvedPath: "relative", SHA256: identity.SHA256, Size: 1},
		{ResolvedPath: identity.ResolvedPath + string(filepath.Separator) + ".." + string(filepath.Separator) + "rtk", SHA256: identity.SHA256, Size: 1},
		{ResolvedPath: identity.ResolvedPath, SHA256: "abc", Size: 1},
		{ResolvedPath: identity.ResolvedPath, SHA256: strings.Repeat("A", 64), Size: 1},
		{ResolvedPath: identity.ResolvedPath, SHA256: identity.SHA256, Size: 0},
		{ResolvedPath: identity.ResolvedPath, SHA256: identity.SHA256, Size: MaxExecutableBytes + 1},
	}
	for _, value := range tests {
		if err := value.Validate(); !errors.Is(err, ErrInvalidIdentity) {
			t.Fatalf("Validate(%+v) = %v", value, err)
		}
	}
}

func TestRequestEnforcesBoundIdentityIsolationAndFiniteLimits(t *testing.T) {
	request := fixtureRequest(t)
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Request)
	}{
		{name: "identity mismatch", mutate: func(v *Request) { v.Executable = filepath.Join(t.TempDir(), "other") }},
		{name: "nil args", mutate: func(v *Request) { v.Args = nil }},
		{name: "too many args", mutate: func(v *Request) { v.Args = make([]string, MaxArguments+1) }},
		{name: "unsafe arg", mutate: func(v *Request) { v.Args = []string{"line\n"} }},
		{name: "input", mutate: func(v *Request) { v.Input = make([]byte, MaxInputBytes+1) }},
		{name: "timeout zero", mutate: func(v *Request) { v.Timeout = 0 }},
		{name: "timeout high", mutate: func(v *Request) { v.Timeout = MaxTimeout + time.Nanosecond }},
		{name: "stdout", mutate: func(v *Request) { v.StdoutLimit = MaxStdoutBytes + 1 }},
		{name: "stderr", mutate: func(v *Request) { v.StderrLimit = MaxStderrBytes + 1 }},
		{name: "environment", mutate: func(v *Request) { v.EnvironmentContract = "atsura.processor.other.v1" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := request
			value.Args = append([]string(nil), request.Args...)
			value.Input = append([]byte(nil), request.Input...)
			test.mutate(&value)
			if err := value.Validate(); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("Validate() = %v", err)
			}
		})
	}
}

func TestResultTruthTableDistinguishesZeroAndOneAttempt(t *testing.T) {
	request := fixtureRequest(t)
	zero := Result{ExitCode: -1}
	if err := zero.Validate(request, false); err != nil {
		t.Fatalf("zero result: %v", err)
	}
	one := Result{Attempts: 1, ExitCode: 0, Stdout: []byte("ok"), Identity: request.ExpectedIdentity}
	if err := one.Validate(request, true); err != nil {
		t.Fatalf("one result: %v", err)
	}

	tests := []struct {
		name      string
		result    Result
		succeeded bool
	}{
		{name: "successful zero", result: zero, succeeded: true},
		{name: "zero with bytes", result: Result{ExitCode: -1, Stdout: []byte("x")}},
		{name: "two attempts", result: Result{Attempts: 2, ExitCode: 0}},
		{name: "wrong identity", result: Result{Attempts: 1, ExitCode: 0, Identity: fixtureIdentity(t)}, succeeded: true},
		{name: "successful nonzero", result: Result{Attempts: 1, ExitCode: 7, Identity: request.ExpectedIdentity}, succeeded: true},
		{name: "oversized stdout", result: Result{Attempts: 1, ExitCode: 0, Stdout: make([]byte, request.StdoutLimit+1), Identity: request.ExpectedIdentity}, succeeded: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.result.Validate(request, test.succeeded); !errors.Is(err, ErrInvalidResult) {
				t.Fatalf("Validate() = %v", err)
			}
		})
	}
}
