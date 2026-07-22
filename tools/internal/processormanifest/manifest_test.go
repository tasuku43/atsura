package processormanifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReturnsDetachedExactTargetMetadata(t *testing.T) {
	root := t.TempDir()
	manifest := validManifest()
	writeManifest(t, root, manifest)

	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := loaded.Target("darwin/arm64")
	if err != nil {
		t.Fatal(err)
	}
	if metadata.ContractID() != "atsura.output.example.v1" ||
		metadata.ProcessorKind() != "atsura.processor.example" ||
		metadata.Version() != "1.2.3" ||
		metadata.Target() != "darwin/arm64" ||
		metadata.ArchiveName() != "darwin-arm64.tar.gz" ||
		metadata.ArchiveURL() != "https://github.com/example/project/releases/download/v1/darwin-arm64.tar.gz" ||
		metadata.ArchiveSHA256() != strings.Repeat("4", 64) || metadata.ArchiveSize() != 104 ||
		metadata.BinaryMember() != "processor" || metadata.BinarySHA256() != strings.Repeat("8", 64) || metadata.BinarySize() != 204 {
		t.Fatalf("metadata = %+v", metadata)
	}
	loaded.Processors[0].Artifacts[3].ArchiveName = "mutated.tar.gz"
	if metadata.ArchiveName() != "darwin-arm64.tar.gz" {
		t.Fatalf("metadata changed through manifest mutation: %q", metadata.ArchiveName())
	}
}

func TestLoadPinnedAcceptsOnlyExactADR0012Manifest(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, PinnedManifest())
	loaded, err := LoadPinned(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.ValidatePinned(); err != nil {
		t.Fatal(err)
	}

	changed := PinnedManifest()
	changed.Processors[0].Artifacts[0].ArchiveURL = "https://github.com/example/project/releases/download/v1/rtk.tar.gz"
	writeManifest(t, root, changed)
	if _, err := Load(root); err != nil {
		t.Fatalf("structurally valid changed manifest was rejected too early: %v", err)
	}
	if _, err := LoadPinned(root); err == nil || !strings.Contains(err.Error(), "exact ADR 0012") {
		t.Fatalf("LoadPinned() error = %v", err)
	}
}

func TestPinnedManifestReturnsFreshValues(t *testing.T) {
	changed := PinnedManifest()
	changed.Processors[0].Artifacts[0].ArchiveName = "changed.tar.gz"
	if got := PinnedManifest().Processors[0].Artifacts[0].ArchiveName; got == "changed.tar.gz" {
		t.Fatal("PinnedManifest returned shared mutable state")
	}
}

func TestLoadRejectsUnknownDuplicateTrailingNullAndOversizedJSON(t *testing.T) {
	valid := encodedManifest(t, validManifest())
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "unknown", data: bytesBeforeFinalBrace(valid, `,"unexpected":true`), want: "unknown field"},
		{name: "duplicate", data: []byte(strings.Replace(string(valid), `"schema_version":1`, `"schema_version":1,"schema_version":1`, 1)), want: "duplicate object key"},
		{name: "trailing", data: append(append([]byte(nil), valid...), []byte(` {}`)...), want: "multiple top-level"},
		{name: "null", data: []byte("null\n"), want: "top level must be an object"},
		{name: "array", data: []byte("[]\n"), want: "cannot unmarshal"},
		{name: "invalid UTF-8", data: []byte("{\"schema_version\":1,\"processors\":[{\"contract_id\":\"\xff\"}]}\n"), want: "valid UTF-8"},
		{name: "oversized", data: []byte(strings.Repeat(" ", maxManifestBytes+1)), want: "exceeds the byte limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeRawManifest(t, root, test.data)
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestManifestRejectsDuplicateMissingAndUnsupportedTargets(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
		want   string
	}{
		{name: "missing", mutate: func(value *Manifest) { value.Processors[0].Artifacts = value.Processors[0].Artifacts[:3] }, want: "every supported target"},
		{name: "duplicate", mutate: func(value *Manifest) { value.Processors[0].Artifacts[3].Target = "linux/amd64" }, want: "duplicate target"},
		{name: "windows", mutate: func(value *Manifest) { value.Processors[0].Artifacts[0].Target = "windows/amd64" }, want: "unsupported target"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := validManifest()
			test.mutate(&manifest)
			root := t.TempDir()
			writeManifest(t, root, manifest)
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want %q", err, test.want)
			}
		})
	}
	if _, err := validManifest().Target("windows/amd64"); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("Target(windows/amd64) error = %v", err)
	}
}

func TestLoadRejectsUnsafeRepositoryAndManifestRoots(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, validManifest())
	if _, err := Load("relative"); err == nil || !strings.Contains(err.Error(), "absolute clean") {
		t.Fatalf("relative root error = %v", err)
	}
	if _, err := Load(root + string(filepath.Separator) + "."); err == nil || !strings.Contains(err.Error(), "absolute clean") {
		t.Fatalf("unclean root error = %v", err)
	}
	filesystemRoot := filepath.VolumeName(root) + string(filepath.Separator)
	if _, err := Load(filesystemRoot); err == nil || !strings.Contains(err.Error(), "non-filesystem-root") {
		t.Fatalf("filesystem root error = %v", err)
	}

	t.Run("root file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "root-file")
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "directory") {
			t.Fatalf("Load() error = %v", err)
		}
	})

	t.Run("root symlink", func(t *testing.T) {
		realRoot := t.TempDir()
		writeManifest(t, realRoot, validManifest())
		link := filepath.Join(t.TempDir(), "repository")
		if err := os.Symlink(realRoot, link); err != nil {
			t.Skipf("symbolic links unavailable: %v", err)
		}
		if _, err := Load(link); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("Load() error = %v", err)
		}
	})

	t.Run("harness symlink", func(t *testing.T) {
		repository := t.TempDir()
		realHarness := filepath.Join(repository, "real-harness")
		if err := os.Mkdir(realHarness, 0o700); err != nil {
			t.Fatal(err)
		}
		data := encodedManifest(t, validManifest())
		if err := os.WriteFile(filepath.Join(realHarness, "processors.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("real-harness", filepath.Join(repository, ".harness")); err != nil {
			t.Skipf("symbolic links unavailable: %v", err)
		}
		if _, err := Load(repository); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("Load() error = %v", err)
		}
	})

	t.Run("manifest symlink", func(t *testing.T) {
		repository := t.TempDir()
		if err := os.Mkdir(filepath.Join(repository, ".harness"), 0o700); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(repository, ".harness", "target.json")
		if err := os.WriteFile(target, encodedManifest(t, validManifest()), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("target.json", filepath.Join(repository, filepath.FromSlash(Path))); err != nil {
			t.Skipf("symbolic links unavailable: %v", err)
		}
		if _, err := Load(repository); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("Load() error = %v", err)
		}
	})

	t.Run("manifest directory", func(t *testing.T) {
		repository := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repository, filepath.FromSlash(Path)), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(repository); err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("Load() error = %v", err)
		}
	})
}

func validManifest() Manifest {
	artifacts := make([]Artifact, 0, len(supportedTargets))
	for index, target := range supportedTargets {
		name := strings.ReplaceAll(target, "/", "-") + ".tar.gz"
		artifacts = append(artifacts, Artifact{
			Target: target, ArchiveName: name,
			ArchiveURL:    "https://github.com/example/project/releases/download/v1/" + name,
			ArchiveSHA256: strings.Repeat(string(rune('1'+index)), 64), ArchiveSize: int64(101 + index),
			BinaryMember: "processor", BinarySHA256: strings.Repeat(string(rune('5'+index)), 64), BinarySize: int64(201 + index),
		})
	}
	return Manifest{
		SchemaVersion: SchemaVersion,
		Processors: []Processor{{
			ContractID: "atsura.output.example.v1", Kind: "atsura.processor.example", Version: "1.2.3",
			UpstreamCommit: strings.Repeat("a", 40), ReleaseURL: "https://github.com/example/project/releases/tag/v1",
			Checksums: Checksums{URL: "https://github.com/example/project/releases/download/v1/checksums.txt", SHA256: strings.Repeat("b", 64)},
			License:   License{SPDX: "Apache-2.0", URL: "https://github.com/example/project/blob/main/LICENSE", SHA256: strings.Repeat("c", 64)},
			Notice:    Notice{Status: "absent_upstream"}, Distribution: "external", SBOMReview: "not_provided",
			Artifacts: artifacts,
		}},
	}
}

func writeManifest(t *testing.T, root string, manifest Manifest) {
	t.Helper()
	writeRawManifest(t, root, encodedManifest(t, manifest))
}

func encodedManifest(t *testing.T, manifest Manifest) []byte {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

func writeRawManifest(t *testing.T, root string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func bytesBeforeFinalBrace(data []byte, insertion string) []byte {
	trimmed := strings.TrimSpace(string(data))
	return []byte(strings.TrimSuffix(trimmed, "}") + insertion + "}\n")
}
