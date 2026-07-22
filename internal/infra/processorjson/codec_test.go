package processorjson

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
)

func observationFixture(t *testing.T) processorprocess.Observation {
	t.Helper()
	return processorprocess.Observation{
		SchemaVersion: processorprocess.ObservationSchemaVersion,
		Adapter:       processorprocess.Adapter{Kind: "atsura.processor.rtk", ContractVersion: 1},
		Platform:      processorprocess.Platform{OS: "linux", Arch: "amd64"},
		Identity: processorprocess.Identity{
			ResolvedPath: filepath.Join(t.TempDir(), "rtk"), SHA256: strings.Repeat("a", 64), Size: 42,
		},
		Version: "0.43.0",
		Probe: processorprocess.Probe{
			Argv: []string{"--version"}, EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV1, Attempts: 1,
		},
	}
}

func TestEncodeDecodeCanonicalObservation(t *testing.T) {
	want := observationFixture(t)
	raw, err := Encode(want)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' || strings.Contains(string(raw[:len(raw)-1]), "\n") {
		t.Fatalf("encoded = %q", raw)
	}
	got, err := Decode(raw)
	if err != nil || got.Identity != want.Identity || got.Adapter != want.Adapter || got.Platform != want.Platform || got.Version != want.Version || got.Probe.Attempts != 1 {
		t.Fatalf("Decode() = %+v, %v", got, err)
	}
	if len(got.Probe.Argv) != 1 || got.Probe.Argv[0] != "--version" {
		t.Fatalf("probe argv = %#v", got.Probe.Argv)
	}
}

func TestDecodeRejectsUnknownDuplicateTrailingAndInvalidObservation(t *testing.T) {
	valid, err := Encode(observationFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	unknown := strings.Replace(string(valid), `"schema_version":1`, `"schema_version":1,"unknown":true`, 1)
	duplicate := strings.Replace(string(valid), `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1)
	wrongSchema := strings.Replace(string(valid), `"schema_version":1`, `"schema_version":2`, 1)
	nullArgs := strings.Replace(string(valid), `"argv":["--version"]`, `"argv":null`, 1)
	tests := [][]byte{
		nil,
		[]byte(unknown),
		[]byte(duplicate),
		append(append([]byte(nil), valid...), []byte(` {}`)...),
		[]byte(wrongSchema),
		[]byte(nullArgs),
		make([]byte, MaxObservationBytes+1),
	}
	for index, raw := range tests {
		if _, err := Decode(raw); !errors.Is(err, processorprocess.ErrInvalidObservation) {
			t.Fatalf("case %d error = %v", index, err)
		}
	}
}

func TestLoaderReadsStableRegularObservation(t *testing.T) {
	want := observationFixture(t)
	raw, err := Encode(want)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "processor.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := New().Load(context.Background(), path)
	if err != nil || got.Identity != want.Identity {
		t.Fatalf("Load() = %+v, %v", got, err)
	}
}

func TestLoaderMapsInvalidMissingUnsafeAndOversizedFiles(t *testing.T) {
	directory := t.TempDir()
	tests := []struct {
		name string
		path func() string
		code string
	}{
		{name: "missing", path: func() string { return filepath.Join(directory, "missing") }, code: "processor_observation_file_not_found"},
		{name: "invalid", path: func() string {
			path := filepath.Join(directory, "invalid.json")
			if err := os.WriteFile(path, []byte(`{"schema_version":1}`), 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		}, code: "invalid_processor_observation_file"},
		{name: "large", path: func() string {
			path := filepath.Join(directory, "large.json")
			if err := os.WriteFile(path, make([]byte, MaxObservationBytes+1), 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		}, code: "processor_observation_file_too_large"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New().Load(context.Background(), test.path())
			public, ok := fault.PublicCopy(err)
			if !ok || public.Code != test.code {
				t.Fatalf("error = %v", err)
			}
		})
	}

	target := filepath.Join(directory, "target.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "link.json")
	if err := os.Symlink(target, link); err != nil {
		if errors.Is(err, os.ErrPermission) {
			t.Skip("symlink creation is unavailable")
		}
		t.Fatal(err)
	}
	_, err := New().Load(context.Background(), link)
	public, ok := fault.PublicCopy(err)
	if !ok || public.Code != "unsafe_processor_observation_file" {
		t.Fatalf("symlink error = %v", err)
	}
}

func TestLoaderHonorsCanceledContextBeforeRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New().Load(ctx, filepath.Join(t.TempDir(), "processor.json"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}
