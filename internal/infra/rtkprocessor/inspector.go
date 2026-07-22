// Package rtkprocessor adapts one exact official RTK artifact to Atsura's
// generic processor-observation contract.
package rtkprocessor

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/tasuku43/atsura/internal/domain/processorprocess"
)

const (
	AdapterKind     = "atsura.processor.rtk"
	ContractVersion = 1
	Version         = "0.43.0"

	versionStdoutLimit = 64 * 1024
)

// Artifact is the extracted official binary identity for one supported native
// release tuple. Archive checksums and provenance remain harness evidence.
type Artifact struct {
	SHA256 string
	Size   int64
}

var officialArtifacts = map[string]Artifact{
	"linux/amd64":  {SHA256: "f160611f3baee17fe4eb3a04c56a8bc3d15fec4274d8838016088d4776c6f628", Size: 10083968},
	"linux/arm64":  {SHA256: "86bd2badb697e41fa4fae805ed1a42d9b2495600260918d6ba9c148bc40013cf", Size: 8544624},
	"darwin/amd64": {SHA256: "22adaa27b3fd6d8906159ba3ff7ca8346e914df112408bcc7a88cda30a3a6107", Size: 9006316},
	"darwin/arm64": {SHA256: "2dab449f32ea744c30b02a3ef9806e3e7d3b356a145332f3f2aaabb5ea48edee", Size: 7763408},
}

// OfficialArtifact returns the immutable extracted-binary identity for one
// supported RTK v0.43.0 native tuple.
func OfficialArtifact(goos, goarch string) (Artifact, bool) {
	artifact, exists := officialArtifacts[goos+"/"+goarch]
	return artifact, exists
}

// ProcessPort owns exact identity reads and one bounded isolated process.
type ProcessPort interface {
	Identify(context.Context, string) (processorprocess.Identity, error)
	Run(context.Context, processorprocess.Request) (processorprocess.Result, error)
}

// Inspector performs only the exact RTK v0.43.0 version observation.
type Inspector struct {
	processes ProcessPort
	goos      string
	goarch    string
}

// New creates an inspector bound to the current native platform.
func New(processes ProcessPort) *Inspector {
	return &Inspector{processes: processes, goos: runtime.GOOS, goarch: runtime.GOARCH}
}

// Inspect identifies one explicit absolute path, admits only the official
// native binary, and executes exactly `--version` once in the RTK isolation
// contract.
func (i *Inspector) Inspect(ctx context.Context, executable string) (processorprocess.Observation, error) {
	if ctx == nil {
		return processorprocess.Observation{}, fmt.Errorf("rtk processor inspection context is nil")
	}
	if err := ctx.Err(); err != nil {
		return processorprocess.Observation{}, err
	}
	if i == nil || i.processes == nil {
		return processorprocess.Observation{}, fmt.Errorf("rtk processor process adapter is not configured")
	}
	if err := processorprocess.ValidateExecutablePath(executable); err != nil {
		return processorprocess.Observation{}, fmt.Errorf("%w: %v", processorprocess.ErrInspectionFailed, err)
	}
	artifact, supported := OfficialArtifact(i.goos, i.goarch)
	if !supported {
		return processorprocess.Observation{}, fmt.Errorf("%w: %s/%s", processorprocess.ErrUnsupportedPlatform, i.goos, i.goarch)
	}
	identity, err := i.processes.Identify(ctx, executable)
	if err != nil {
		return processorprocess.Observation{}, err
	}
	if identity.ResolvedPath != executable || identity.SHA256 != artifact.SHA256 || identity.Size != artifact.Size {
		return processorprocess.Observation{}, fmt.Errorf("%w: binary identity does not match the official %s/%s artifact", processorprocess.ErrUnsupportedArtifact, i.goos, i.goarch)
	}
	request := processorprocess.Request{
		Executable:          executable,
		Args:                []string{"--version"},
		Input:               nil,
		Timeout:             5 * time.Second,
		StdoutLimit:         versionStdoutLimit,
		StderrLimit:         processorprocess.MaxStderrBytes,
		ExpectedIdentity:    identity,
		EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV1,
	}
	result, err := i.processes.Run(ctx, request)
	if validateErr := result.Validate(request, err == nil); validateErr != nil {
		return processorprocess.Observation{}, fmt.Errorf("%w: invalid version process result: %v", processorprocess.ErrInspectionFailed, validateErr)
	}
	if err != nil {
		return processorprocess.Observation{}, err
	}
	if result.Attempts != 1 || result.ExitCode != 0 || string(result.Stdout) != "rtk 0.43.0\n" || len(result.Stderr) != 0 {
		return processorprocess.Observation{}, fmt.Errorf("%w: version probe must return exact RTK v0.43.0 evidence", processorprocess.ErrUnsupportedVersion)
	}

	observation := processorprocess.Observation{
		SchemaVersion: processorprocess.ObservationSchemaVersion,
		Adapter: processorprocess.Adapter{
			Kind:            AdapterKind,
			ContractVersion: ContractVersion,
		},
		Platform: processorprocess.Platform{OS: i.goos, Arch: i.goarch},
		Identity: identity,
		Version:  Version,
		Probe: processorprocess.Probe{
			Argv:                []string{"--version"},
			EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV1,
			Attempts:            1,
		},
	}
	if err := VerifyObservation(observation); err != nil {
		return processorprocess.Observation{}, err
	}
	return observation, nil
}

// VerifyObservation proves that loaded evidence still names exactly the
// maintained RTK adapter, version probe, platform, and official binary.
func VerifyObservation(observation processorprocess.Observation) error {
	if err := observation.Validate(); err != nil {
		return fmt.Errorf("%w: %v", processorprocess.ErrInvalidObservation, err)
	}
	if observation.Adapter.Kind != AdapterKind || observation.Adapter.ContractVersion != ContractVersion {
		return fmt.Errorf("%w: RTK adapter contract is not supported", processorprocess.ErrInvalidObservation)
	}
	if observation.Version != Version {
		return fmt.Errorf("%w: %s", processorprocess.ErrUnsupportedVersion, observation.Version)
	}
	if len(observation.Probe.Argv) != 1 || observation.Probe.Argv[0] != "--version" ||
		observation.Probe.EnvironmentContract != processorprocess.EnvironmentRTKIsolatedV1 || observation.Probe.Attempts != 1 {
		return fmt.Errorf("%w: RTK probe contract does not match", processorprocess.ErrInvalidObservation)
	}
	artifact, supported := OfficialArtifact(observation.Platform.OS, observation.Platform.Arch)
	if !supported {
		return fmt.Errorf("%w: %s/%s", processorprocess.ErrUnsupportedPlatform, observation.Platform.OS, observation.Platform.Arch)
	}
	if observation.Identity.SHA256 != artifact.SHA256 || observation.Identity.Size != artifact.Size {
		return fmt.Errorf("%w: binary identity does not match the official artifact", processorprocess.ErrUnsupportedArtifact)
	}
	return nil
}
