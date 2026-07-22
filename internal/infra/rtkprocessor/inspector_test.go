package rtkprocessor

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/processorprocess"
)

type fakeProcess struct {
	identity      processorprocess.Identity
	identifyErr   error
	result        processorprocess.Result
	runErr        error
	identifyCalls int
	runCalls      int
	request       processorprocess.Request
}

func (f *fakeProcess) Identify(_ context.Context, _ string) (processorprocess.Identity, error) {
	f.identifyCalls++
	return f.identity, f.identifyErr
}

func (f *fakeProcess) Run(_ context.Context, request processorprocess.Request) (processorprocess.Result, error) {
	f.runCalls++
	f.request = request
	return f.result, f.runErr
}

func fixtureInspector(t *testing.T, goos, goarch string) (*Inspector, *fakeProcess, string) {
	t.Helper()
	artifact, ok := OfficialArtifact(goos, goarch)
	if !ok {
		t.Fatalf("unsupported fixture %s/%s", goos, goarch)
	}
	path := filepath.Join(t.TempDir(), "rtk")
	identity := processorprocess.Identity{ResolvedPath: path, SHA256: artifact.SHA256, Size: artifact.Size}
	process := &fakeProcess{identity: identity}
	process.result = processorprocess.Result{Attempts: 1, ExitCode: 0, Stdout: []byte("rtk 0.43.0\n"), Identity: identity}
	return &Inspector{processes: process, goos: goos, goarch: goarch}, process, path
}

func TestInspectReturnsExactOfficialObservationAndRequest(t *testing.T) {
	inspector, process, path := fixtureInspector(t, "darwin", "arm64")
	observation, err := inspector.Inspect(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if process.identifyCalls != 1 || process.runCalls != 1 {
		t.Fatalf("calls = %d/%d", process.identifyCalls, process.runCalls)
	}
	request := process.request
	if request.Executable != path || !slicesEqual(request.Args, []string{"--version"}) || request.Input != nil ||
		request.Timeout != processorprocess.MaxTimeout || request.StdoutLimit != versionStdoutLimit ||
		request.StderrLimit != processorprocess.MaxStderrBytes || request.EnvironmentContract != processorprocess.EnvironmentRTKIsolatedV1 {
		t.Fatalf("request = %+v", request)
	}
	if observation.Adapter.Kind != AdapterKind || observation.Adapter.ContractVersion != ContractVersion ||
		observation.Platform != (processorprocess.Platform{OS: "darwin", Arch: "arm64"}) || observation.Identity != process.identity ||
		observation.Version != Version || observation.Probe.Attempts != 1 {
		t.Fatalf("observation = %+v", observation)
	}
	if err := VerifyObservation(observation); err != nil {
		t.Fatal(err)
	}
}

func TestOfficialArtifactMatrixIsExact(t *testing.T) {
	want := map[string]Artifact{
		"linux/amd64":  {SHA256: "f160611f3baee17fe4eb3a04c56a8bc3d15fec4274d8838016088d4776c6f628", Size: 10083968},
		"linux/arm64":  {SHA256: "86bd2badb697e41fa4fae805ed1a42d9b2495600260918d6ba9c148bc40013cf", Size: 8544624},
		"darwin/amd64": {SHA256: "22adaa27b3fd6d8906159ba3ff7ca8346e914df112408bcc7a88cda30a3a6107", Size: 9006316},
		"darwin/arm64": {SHA256: "2dab449f32ea744c30b02a3ef9806e3e7d3b356a145332f3f2aaabb5ea48edee", Size: 7763408},
	}
	for tuple, expected := range want {
		parts := strings.Split(tuple, "/")
		actual, ok := OfficialArtifact(parts[0], parts[1])
		if !ok || actual != expected {
			t.Fatalf("OfficialArtifact(%s) = %+v, %t", tuple, actual, ok)
		}
	}
	if _, ok := OfficialArtifact("windows", "amd64"); ok {
		t.Fatal("windows unexpectedly supported")
	}
}

func TestInspectRejectsUnsupportedPlatformAndArtifactBeforeRun(t *testing.T) {
	inspector, process, path := fixtureInspector(t, "darwin", "arm64")
	inspector.goos = "windows"
	_, err := inspector.Inspect(context.Background(), path)
	if !errors.Is(err, processorprocess.ErrUnsupportedPlatform) || process.identifyCalls != 0 || process.runCalls != 0 {
		t.Fatalf("platform error=%v calls=%d/%d", err, process.identifyCalls, process.runCalls)
	}

	inspector, process, path = fixtureInspector(t, "linux", "amd64")
	process.identity.SHA256 = strings.Repeat("a", 64)
	_, err = inspector.Inspect(context.Background(), path)
	if !errors.Is(err, processorprocess.ErrUnsupportedArtifact) || process.identifyCalls != 1 || process.runCalls != 0 {
		t.Fatalf("artifact error=%v calls=%d/%d", err, process.identifyCalls, process.runCalls)
	}
}

func TestInspectRequiresExactVersionStdoutStatusStderrAndIdentity(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fakeProcess)
		want   error
	}{
		{name: "missing newline", mutate: func(f *fakeProcess) { f.result.Stdout = []byte("rtk 0.43.0") }, want: processorprocess.ErrUnsupportedVersion},
		{name: "other version", mutate: func(f *fakeProcess) { f.result.Stdout = []byte("rtk 0.44.0\n") }, want: processorprocess.ErrUnsupportedVersion},
		{name: "stderr", mutate: func(f *fakeProcess) { f.result.Stderr = []byte("warning") }, want: processorprocess.ErrUnsupportedVersion},
		{name: "nonzero", mutate: func(f *fakeProcess) { f.result.ExitCode = 7; f.runErr = errors.New("failed") }, want: errors.New("failed")},
		{name: "zero attempts", mutate: func(f *fakeProcess) { f.result = processorprocess.Result{ExitCode: -1} }, want: processorprocess.ErrInspectionFailed},
		{name: "wrong result identity", mutate: func(f *fakeProcess) { f.result.Identity.SHA256 = strings.Repeat("b", 64) }, want: processorprocess.ErrInspectionFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inspector, process, path := fixtureInspector(t, "darwin", "amd64")
			test.mutate(process)
			_, err := inspector.Inspect(context.Background(), path)
			if test.name == "nonzero" {
				if err == nil || err.Error() != "failed" {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestVerifyObservationRejectsContractVersionProbePlatformAndArtifactDrift(t *testing.T) {
	inspector, _, path := fixtureInspector(t, "linux", "arm64")
	observation, err := inspector.Inspect(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*processorprocess.Observation)
		want   error
	}{
		{name: "adapter", mutate: func(v *processorprocess.Observation) { v.Adapter.Kind = "atsura.processor.other" }, want: processorprocess.ErrInvalidObservation},
		{name: "contract", mutate: func(v *processorprocess.Observation) { v.Adapter.ContractVersion = 2 }, want: processorprocess.ErrInvalidObservation},
		{name: "version", mutate: func(v *processorprocess.Observation) { v.Version = "0.44.0" }, want: processorprocess.ErrUnsupportedVersion},
		{name: "probe", mutate: func(v *processorprocess.Observation) { v.Probe.Argv = []string{"version"} }, want: processorprocess.ErrInvalidObservation},
		{name: "platform", mutate: func(v *processorprocess.Observation) { v.Platform.OS = "windows" }, want: processorprocess.ErrUnsupportedPlatform},
		{name: "artifact", mutate: func(v *processorprocess.Observation) { v.Identity.SHA256 = strings.Repeat("a", 64) }, want: processorprocess.ErrUnsupportedArtifact},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := observation
			value.Probe.Argv = append([]string(nil), observation.Probe.Argv...)
			test.mutate(&value)
			if err := VerifyObservation(value); !errors.Is(err, test.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestInspectRejectsNilProcessRelativePathAndCanceledContext(t *testing.T) {
	if _, err := New(nil).Inspect(context.Background(), "/tmp/rtk"); err == nil {
		t.Fatal("nil process succeeded")
	}
	inspector, process, _ := fixtureInspector(t, "darwin", "arm64")
	if _, err := inspector.Inspect(context.Background(), "rtk"); !errors.Is(err, processorprocess.ErrInspectionFailed) || process.identifyCalls != 0 {
		t.Fatalf("relative error=%v calls=%d", err, process.identifyCalls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := inspector.Inspect(ctx, process.identity.ResolvedPath); !errors.Is(err, context.Canceled) || process.identifyCalls != 0 {
		t.Fatalf("canceled error=%v calls=%d", err, process.identifyCalls)
	}
}

func slicesEqual(left, right []string) bool {
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
