package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/tools/internal/processormanifest"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type errorBody struct {
	read bool
}

func (b *errorBody) Read(buffer []byte) (int, error) {
	if b.read {
		return 0, errors.New("synthetic read failure")
	}
	b.read = true
	return copy(buffer, "go"), nil
}

func (*errorBody) Close() error { return nil }

func TestFetchDownloadsVerifiedArchiveToPrivateLocalPath(t *testing.T) {
	requirePOSIX(t)
	payload := []byte("verified archive bytes")
	repository := processorRepository(t, "linux/amd64", payload, "https://github.com/example/project/releases/download/v1/linux-amd64.tar.gz")
	metadata := targetMetadata(t, repository, "linux/amd64")
	outputDirectory := privateDirectory(t)
	calls := 0
	client := newHTTPClient(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Method != http.MethodGet || request.URL.Scheme != "https" || request.URL.Host != "github.com" {
			t.Fatalf("request = %s %s://%s", request.Method, request.URL.Scheme, request.URL.Host)
		}
		if request.Header.Get("Accept") != "application/octet-stream" || request.Header.Get("Accept-Encoding") != "identity" || request.Header.Get("User-Agent") != "atsura-processorfetch/1" {
			t.Fatalf("headers = %#v", request.Header)
		}
		if request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" {
			t.Fatalf("secret-bearing headers = %#v", request.Header)
		}
		return archiveResponse(http.StatusOK, payload, int64(len(payload))), nil
	}))
	path, err := fetch(context.Background(), outputDirectory, metadata, client)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(outputDirectory, "linux-amd64.tar.gz")
	if path != wantPath || calls != 1 {
		t.Fatalf("path/calls = %q/%d", path, calls)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, payload) || !info.Mode().IsRegular() || info.Mode().Perm() != privateArchiveMode {
		t.Fatalf("archive data/mode = %q/%v", data, info.Mode())
	}
}

func TestRunAcceptsOnlyPinnedManifestBeforeFetch(t *testing.T) {
	requirePOSIX(t)
	repository := pinnedProcessorRepository(t)
	outputDirectory := privateDirectory(t)
	wantPath := filepath.Join(outputDirectory, processormanifest.PinnedManifest().Processors[0].Artifacts[0].ArchiveName)
	fetchCalls := 0
	var output bytes.Buffer
	err := run(context.Background(), []string{"--target", "linux/amd64", "--output-dir", outputDirectory}, &output, dependencies{
		repositoryRoot: repository,
		client: newHTTPClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network must not be used by injected fetch")
		})),
		fetchArchive: func(_ context.Context, gotDirectory string, metadata processormanifest.TargetMetadata, _ *http.Client) (string, error) {
			fetchCalls++
			if gotDirectory != outputDirectory || metadata.Target() != "linux/amd64" || metadata.ArchiveName() != filepath.Base(wantPath) {
				t.Fatalf("fetch inputs = %q/%q/%q", gotDirectory, metadata.Target(), metadata.ArchiveName())
			}
			return wantPath, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if output.String() != wantPath+"\n" || fetchCalls != 1 {
		t.Fatalf("output/fetch calls = %q/%d", output.String(), fetchCalls)
	}
}

func TestRunRejectsUnpinnedManifestBeforeNetworkOrFetch(t *testing.T) {
	requirePOSIX(t)
	payload := []byte("archive")
	repository := processorRepository(t, "linux/amd64", payload, "https://github.com/example/project/releases/download/v1/linux-amd64.tar.gz")
	outputDirectory := privateDirectory(t)
	networkCalls := 0
	fetchCalls := 0
	client := newHTTPClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		networkCalls++
		return archiveResponse(http.StatusOK, payload, int64(len(payload))), nil
	}))
	err := run(context.Background(), []string{"--target", "linux/amd64", "--output-dir", outputDirectory}, io.Discard, dependencies{
		repositoryRoot: repository,
		client:         client,
		fetchArchive: func(context.Context, string, processormanifest.TargetMetadata, *http.Client) (string, error) {
			fetchCalls++
			return "", nil
		},
	})
	if err == nil || networkCalls != 0 || fetchCalls != 0 {
		t.Fatalf("run() error/network/fetch calls = %v/%d/%d", err, networkCalls, fetchCalls)
	}
}

func TestFetchFollowsOnlyBoundedAllowedSecretFreeRedirects(t *testing.T) {
	requirePOSIX(t)
	payload := []byte("redirected archive")
	repository := processorRepository(t, "darwin/arm64", payload, "https://github.com/example/project/releases/download/v1/darwin-arm64.tar.gz")
	manifest, err := processormanifest.Load(repository)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := manifest.Target("darwin/arm64")
	if err != nil {
		t.Fatal(err)
	}
	outputDirectory := privateDirectory(t)
	var seen []*http.Request
	client := newHTTPClient(roundTripFunc(func(request *http.Request) (*http.Response, error) {
		seen = append(seen, request.Clone(request.Context()))
		if len(seen) == 1 {
			return redirectResponse("https://release-assets.githubusercontent.com/assets/archive?token=signed-secret"), nil
		}
		return archiveResponse(http.StatusOK, payload, int64(len(payload))), nil
	}))
	path, err := fetch(context.Background(), outputDirectory, metadata, client)
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[1].URL.Host != "release-assets.githubusercontent.com" || seen[1].Header.Get("Authorization") != "" || seen[1].Header.Get("Cookie") != "" || seen[1].Header.Get("Referer") != "" {
		t.Fatalf("redirected requests = %#v", seen)
	}
	if path != filepath.Join(outputDirectory, "darwin-arm64.tar.gz") {
		t.Fatalf("path = %q", path)
	}
}

func TestFetchRejectsForbiddenRedirectsAndNeverReturnsSignedURLs(t *testing.T) {
	requirePOSIX(t)
	payload := []byte("archive")
	tests := []struct {
		name     string
		location string
	}{
		{name: "host", location: "https://evil.example/archive?token=signed-secret"},
		{name: "scheme", location: "http://release-assets.githubusercontent.com/archive?token=signed-secret"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := processorRepository(t, "linux/amd64", payload, "https://github.com/example/project/releases/download/v1/linux-amd64.tar.gz")
			metadata := targetMetadata(t, repository, "linux/amd64")
			outputDirectory := privateDirectory(t)
			calls := 0
			client := newHTTPClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return redirectResponse(test.location), nil
			}))
			_, err := fetch(context.Background(), outputDirectory, metadata, client)
			if err == nil || calls != 1 {
				t.Fatalf("fetch() error/calls = %v/%d", err, calls)
			}
			for _, secret := range []string{"signed-secret", "evil.example", test.location} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error exposes redirect location: %q", err)
				}
			}
			assertMissing(t, filepath.Join(outputDirectory, metadata.ArchiveName()))
		})
	}
}

func TestDownloadURLRejectsUserinfoWithoutCredentialFixture(t *testing.T) {
	location := &url.URL{Scheme: "https", Host: "github.com", Path: "/archive", User: url.User("synthetic-user")}
	if err := validateDownloadURL(location); err == nil {
		t.Fatal("URL userinfo was accepted")
	}
}

func TestFetchRejectsNoncanonicalInitialLocationBeforeNetwork(t *testing.T) {
	requirePOSIX(t)
	payload := []byte("archive")
	tests := []string{
		"https://release-assets.githubusercontent.com/archive?token=signed-secret",
		"https://github.com/example/archive?token=signed-secret",
		"https://evil.example/archive?token=signed-secret",
	}
	for index, location := range tests {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			repository := processorRepository(t, "linux/amd64", payload, location)
			metadata := targetMetadata(t, repository, "linux/amd64")
			outputDirectory := privateDirectory(t)
			calls := 0
			client := newHTTPClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return archiveResponse(http.StatusOK, payload, int64(len(payload))), nil
			}))
			_, err := fetch(context.Background(), outputDirectory, metadata, client)
			if err == nil || calls != 0 || strings.Contains(err.Error(), "signed-secret") || strings.Contains(err.Error(), location) {
				t.Fatalf("fetch() error/calls = %v/%d", err, calls)
			}
			assertMissing(t, filepath.Join(outputDirectory, metadata.ArchiveName()))
		})
	}
}

func TestFetchRejectsExcessRedirectsWithoutCreatingOutput(t *testing.T) {
	requirePOSIX(t)
	payload := []byte("archive")
	repository := processorRepository(t, "linux/arm64", payload, "https://github.com/example/project/releases/download/v1/linux-arm64.tar.gz")
	metadata := targetMetadata(t, repository, "linux/arm64")
	outputDirectory := privateDirectory(t)
	calls := 0
	client := newHTTPClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return redirectResponse("https://release-assets.githubusercontent.com/archive?step=" + string(rune('0'+calls))), nil
	}))
	if _, err := fetch(context.Background(), outputDirectory, metadata, client); err == nil || strings.Contains(err.Error(), "step=") {
		t.Fatalf("fetch() error = %v", err)
	}
	if calls != maxRedirects+1 {
		t.Fatalf("transport calls = %d, want %d", calls, maxRedirects+1)
	}
	assertMissing(t, filepath.Join(outputDirectory, metadata.ArchiveName()))
}

func TestFetchRejectsSizeDigestAndBodyFailuresWithoutPartialOutput(t *testing.T) {
	requirePOSIX(t)
	want := []byte("good")
	tests := []struct {
		name     string
		response func() *http.Response
		wantText string
	}{
		{name: "oversize", response: func() *http.Response { return archiveResponse(http.StatusOK, []byte("goodx"), -1) }, wantText: "size"},
		{name: "undersize", response: func() *http.Response { return archiveResponse(http.StatusOK, []byte("goo"), -1) }, wantText: "size"},
		{name: "content length", response: func() *http.Response { return archiveResponse(http.StatusOK, want, int64(len(want)+1)) }, wantText: "content length"},
		{name: "digest", response: func() *http.Response { return archiveResponse(http.StatusOK, []byte("baad"), int64(len(want))) }, wantText: "digest"},
		{name: "body read", response: func() *http.Response {
			response := archiveResponse(http.StatusOK, nil, -1)
			response.Body = &errorBody{}
			return response
		}, wantText: "body"},
		{name: "status", response: func() *http.Response { return archiveResponse(http.StatusNotFound, nil, 0) }, wantText: "status"},
		{name: "encoding", response: func() *http.Response {
			response := archiveResponse(http.StatusOK, want, int64(len(want)))
			response.Header.Set("Content-Encoding", "gzip")
			return response
		}, wantText: "encoding"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := processorRepository(t, "linux/amd64", want, "https://github.com/example/project/releases/download/v1/linux-amd64.tar.gz")
			metadata := targetMetadata(t, repository, "linux/amd64")
			outputDirectory := privateDirectory(t)
			client := newHTTPClient(roundTripFunc(func(*http.Request) (*http.Response, error) { return test.response(), nil }))
			_, err := fetch(context.Background(), outputDirectory, metadata, client)
			if err == nil || !strings.Contains(err.Error(), test.wantText) {
				t.Fatalf("fetch() error = %v, want %q", err, test.wantText)
			}
			assertMissing(t, filepath.Join(outputDirectory, metadata.ArchiveName()))
		})
	}
}

func TestFetchRejectsExistingOutputSymlinksAndNonprivateRootsBeforeNetwork(t *testing.T) {
	requirePOSIX(t)
	payload := []byte("archive")
	repository := processorRepository(t, "darwin/amd64", payload, "https://github.com/example/project/releases/download/v1/darwin-amd64.tar.gz")
	metadata := targetMetadata(t, repository, "darwin/amd64")
	transportCalls := 0
	client := newHTTPClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		transportCalls++
		return nil, errors.New("network must not start")
	}))

	t.Run("existing output", func(t *testing.T) {
		outputDirectory := privateDirectory(t)
		if err := os.WriteFile(filepath.Join(outputDirectory, metadata.ArchiveName()), []byte("existing"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := fetch(context.Background(), outputDirectory, metadata, client); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("fetch() error = %v", err)
		}
	})

	t.Run("output symlink", func(t *testing.T) {
		outputDirectory := privateDirectory(t)
		if err := os.Symlink("missing-target", filepath.Join(outputDirectory, metadata.ArchiveName())); err != nil {
			t.Skipf("symbolic links unavailable: %v", err)
		}
		if _, err := fetch(context.Background(), outputDirectory, metadata, client); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("fetch() error = %v", err)
		}
	})

	t.Run("root symlink", func(t *testing.T) {
		realDirectory := privateDirectory(t)
		link := filepath.Join(t.TempDir(), "output")
		if err := os.Symlink(realDirectory, link); err != nil {
			t.Skipf("symbolic links unavailable: %v", err)
		}
		if _, err := fetch(context.Background(), link, metadata, client); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("fetch() error = %v", err)
		}
	})

	t.Run("public mode", func(t *testing.T) {
		outputDirectory := t.TempDir()
		if err := os.Chmod(outputDirectory, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := fetch(context.Background(), outputDirectory, metadata, client); err == nil || !strings.Contains(err.Error(), "private") {
			t.Fatalf("fetch() error = %v", err)
		}
	})

	t.Run("special root", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "output-file")
		if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := fetch(context.Background(), path, metadata, client); err == nil || !strings.Contains(err.Error(), "special file") {
			t.Fatalf("fetch() error = %v", err)
		}
	})

	t.Run("unsafe path", func(t *testing.T) {
		outputDirectory := privateDirectory(t)
		for _, path := range []string{"relative-output", outputDirectory + string(filepath.Separator) + ".", filepath.VolumeName(outputDirectory) + string(filepath.Separator)} {
			if _, err := fetch(context.Background(), path, metadata, client); err == nil || !strings.Contains(err.Error(), "absolute clean") {
				t.Fatalf("fetch(%q) error = %v", path, err)
			}
		}
	})

	if transportCalls != 0 {
		t.Fatalf("transport calls = %d", transportCalls)
	}
}

func TestRunRejectsWindowsUnknownRepeatedAndIncompleteArguments(t *testing.T) {
	tests := [][]string{
		{"--target", "windows/amd64", "--output-dir", "/tmp/output"},
		{"--target", "plan9/amd64", "--output-dir", "/tmp/output"},
		{"--target", "linux/amd64", "--target", "darwin/arm64", "--output-dir", "/tmp/output"},
		{"--target", "linux/amd64", "--output-dir", "/tmp/one", "--output-dir", "/tmp/two"},
		{"--target", "linux/amd64"},
	}
	for index, args := range tests {
		if _, err := parseOptions(args); err == nil {
			t.Fatalf("case %d accepted args %#v", index, args)
		}
	}
}

func TestProductionHTTPClientHasNoAmbientProxyCookiesOrAuthState(t *testing.T) {
	client := newHTTPClient(nil)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T", client.Transport)
	}
	if transport.Proxy != nil || client.Jar != nil || client.CheckRedirect == nil || client.Timeout != fetchTimeout || !transport.DisableCompression || transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.Fatalf("client/transport are not closed: client=%+v transport=%+v", client, transport)
	}
}

func archiveResponse(status int, data []byte, contentLength int64) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: contentLength,
	}
}

func redirectResponse(location string) *http.Response {
	response := archiveResponse(http.StatusFound, nil, 0)
	response.Header.Set("Location", location)
	return response
}

func processorRepository(t *testing.T, selectedTarget string, payload []byte, selectedURL string) string {
	t.Helper()
	repository := t.TempDir()
	targets := []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"}
	artifacts := make([]processormanifest.Artifact, 0, len(targets))
	for index, target := range targets {
		name := strings.ReplaceAll(target, "/", "-") + ".tar.gz"
		data := []byte{byte(index + 1)}
		archiveURL := "https://github.com/example/project/releases/download/v1/" + name
		if target == selectedTarget {
			data = payload
			archiveURL = selectedURL
		}
		artifacts = append(artifacts, processormanifest.Artifact{
			Target: target, ArchiveName: name, ArchiveURL: archiveURL,
			ArchiveSHA256: digest(data), ArchiveSize: int64(len(data)),
			BinaryMember: "rtk", BinarySHA256: strings.Repeat(string(rune('a'+index)), 64), BinarySize: int64(index + 1),
		})
	}
	manifest := processormanifest.Manifest{
		SchemaVersion: processormanifest.SchemaVersion,
		Processors: []processormanifest.Processor{{
			ContractID: "atsura.output.rtk_go_test_pass.v1", Kind: "atsura.processor.rtk", Version: "0.43.0",
			UpstreamCommit: strings.Repeat("a", 40), ReleaseURL: "https://github.com/example/project/releases/tag/v1",
			Checksums: processormanifest.Checksums{URL: "https://github.com/example/project/releases/download/v1/checksums.txt", SHA256: strings.Repeat("b", 64)},
			License:   processormanifest.License{SPDX: "Apache-2.0", URL: "https://github.com/example/project/blob/main/LICENSE", SHA256: strings.Repeat("c", 64)},
			Notice:    processormanifest.Notice{Status: "absent_upstream"}, Distribution: "external", SBOMReview: "not_provided",
			Artifacts: artifacts,
		}},
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repository, filepath.FromSlash(processormanifest.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return repository
}

func pinnedProcessorRepository(t *testing.T) string {
	t.Helper()
	repository := t.TempDir()
	encoded, err := json.Marshal(processormanifest.PinnedManifest())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repository, filepath.FromSlash(processormanifest.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return repository
}

func targetMetadata(t *testing.T, repository, target string) processormanifest.TargetMetadata {
	t.Helper()
	manifest, err := processormanifest.Load(repository)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := manifest.Target(target)
	if err != nil {
		t.Fatal(err)
	}
	return metadata
}

func privateDirectory(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	return directory
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output %q exists or cannot be inspected: %v", path, err)
	}
}

func digest(data []byte) string {
	value := sha256.Sum256(data)
	return hex.EncodeToString(value[:])
}

func requirePOSIX(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("processorfetch is intentionally unsupported on Windows")
	}
}
