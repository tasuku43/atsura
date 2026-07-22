package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tasuku43/atsura/tools/internal/processormanifest"
)

const (
	fetchTimeout       = 30 * time.Second
	dialTimeout        = 5 * time.Second
	headerTimeout      = 10 * time.Second
	maxRedirects       = 3
	privateArchiveMode = 0o600
)

func newHTTPClient(roundTripper http.RoundTripper) *http.Client {
	if roundTripper == nil {
		dialer := &net.Dialer{Timeout: dialTimeout, KeepAlive: dialTimeout}
		roundTripper = &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          2,
			MaxIdleConnsPerHost:   1,
			IdleConnTimeout:       dialTimeout,
			TLSHandshakeTimeout:   dialTimeout,
			ResponseHeaderTimeout: headerTimeout,
			ExpectContinueTimeout: time.Second,
			DisableCompression:    true,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		}
	}
	return &http.Client{
		Transport:     roundTripper,
		Jar:           nil,
		Timeout:       fetchTimeout,
		CheckRedirect: checkRedirect,
	}
}

func checkRedirect(request *http.Request, via []*http.Request) error {
	if len(via) > maxRedirects {
		return fmt.Errorf("processor archive redirect limit exceeded")
	}
	if err := validateDownloadURL(request.URL); err != nil {
		return err
	}
	setSecretFreeHeaders(request)
	return nil
}

func setSecretFreeHeaders(request *http.Request) {
	request.Header = make(http.Header)
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("User-Agent", "atsura-processorfetch/1")
}

func validateDownloadURL(value *url.URL) error {
	if value == nil || value.Scheme != "https" || value.Opaque != "" || value.User != nil || value.Fragment != "" {
		return fmt.Errorf("processor archive location is not an allowed HTTPS URL")
	}
	hostname := strings.ToLower(value.Hostname())
	if value.Host != hostname {
		return fmt.Errorf("processor archive location uses a noncanonical host")
	}
	if !allowedDownloadHost(hostname) {
		return fmt.Errorf("processor archive location host is not allowed")
	}
	return nil
}

func allowedDownloadHost(hostname string) bool {
	switch hostname {
	case "github.com", "release-assets.githubusercontent.com":
		return true
	default:
		return false
	}
}

func validateInitialDownloadURL(value *url.URL) error {
	if err := validateDownloadURL(value); err != nil {
		return err
	}
	if value.Host != "github.com" || value.RawQuery != "" {
		return fmt.Errorf("processor archive manifest location must be a query-free github.com URL")
	}
	return nil
}

func fetch(ctx context.Context, outputDirectory string, metadata processormanifest.TargetMetadata, client *http.Client) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("processor archive context is nil")
	}
	if client == nil || client.Transport == nil || client.CheckRedirect == nil || client.Jar != nil || client.Timeout <= 0 || client.Timeout > fetchTimeout {
		return "", fmt.Errorf("processor archive HTTP client is not bounded and secret-free")
	}
	if !processormanifest.SupportedTarget(metadata.Target()) || metadata.ArchiveSize() <= 0 {
		return "", fmt.Errorf("processor archive metadata is invalid")
	}
	parsedURL, err := url.Parse(metadata.ArchiveURL())
	if err != nil || validateInitialDownloadURL(parsedURL) != nil {
		return "", fmt.Errorf("processor archive location is not allowed")
	}
	outputRoot, err := openPrivateOutputRoot(outputDirectory)
	if err != nil {
		return "", err
	}
	defer outputRoot.root.Close()
	if err := requireMissingOutput(outputRoot.root, metadata.ArchiveName()); err != nil {
		return "", err
	}

	boundedContext, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(boundedContext, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("processor archive request could not be constructed")
	}
	setSecretFreeHeaders(request)
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("processor archive request failed")
	}
	archive, err := readAndVerifyResponse(response, metadata)
	if err != nil {
		return "", err
	}
	if err := outputRoot.validate(); err != nil {
		return "", err
	}
	if err := writeVerifiedArchive(outputRoot.root, metadata.ArchiveName(), archive); err != nil {
		return "", err
	}
	if err := outputRoot.validate(); err != nil {
		_ = outputRoot.root.Remove(metadata.ArchiveName())
		return "", err
	}
	return filepath.Join(outputDirectory, metadata.ArchiveName()), nil
}

func readAndVerifyResponse(response *http.Response, metadata processormanifest.TargetMetadata) ([]byte, error) {
	if response == nil || response.Body == nil {
		return nil, fmt.Errorf("processor archive response is invalid")
	}
	if response.StatusCode != http.StatusOK {
		if err := response.Body.Close(); err != nil {
			return nil, fmt.Errorf("processor archive response body failed")
		}
		return nil, fmt.Errorf("processor archive response status is not successful")
	}
	if encoding := response.Header.Get("Content-Encoding"); encoding != "" && encoding != "identity" {
		if err := response.Body.Close(); err != nil {
			return nil, fmt.Errorf("processor archive response body failed")
		}
		return nil, fmt.Errorf("processor archive response encoding is not identity")
	}
	if response.ContentLength >= 0 && response.ContentLength != metadata.ArchiveSize() {
		if err := response.Body.Close(); err != nil {
			return nil, fmt.Errorf("processor archive response body failed")
		}
		return nil, fmt.Errorf("processor archive content length does not match the manifest")
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, metadata.ArchiveSize()+1))
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		return nil, fmt.Errorf("processor archive response body failed")
	}
	if int64(len(data)) != metadata.ArchiveSize() {
		return nil, fmt.Errorf("processor archive size does not match the manifest")
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != metadata.ArchiveSHA256() {
		return nil, fmt.Errorf("processor archive digest does not match the manifest")
	}
	return data, nil
}

type privateOutputRoot struct {
	root     *os.Root
	path     string
	identity os.FileInfo
}

func openPrivateOutputRoot(path string) (*privateOutputRoot, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("processor archive fetch is unsupported on Windows")
	}
	if !validLocalPath(path) || !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Dir(path) == path {
		return nil, fmt.Errorf("--output-dir must be an absolute clean non-filesystem-root path")
	}
	before, err := os.Lstat(path) // #nosec G703 -- the explicit output root is validated before any child name is opened.
	if err != nil {
		return nil, fmt.Errorf("inspect processor archive output directory failed")
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("processor archive output directory must not be a symbolic link or special file")
	}
	if before.Mode().Perm()&0o077 != 0 || before.Mode().Perm()&0o300 != 0o300 {
		return nil, fmt.Errorf("processor archive output directory must be private and owner-writable")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, fmt.Errorf("open processor archive output directory failed")
	}
	opened, err := root.Stat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(before, opened) {
		_ = root.Close()
		return nil, fmt.Errorf("processor archive output directory changed during open")
	}
	destination := &privateOutputRoot{root: root, path: path, identity: before}
	if err := destination.validate(); err != nil {
		if closeErr := root.Close(); closeErr != nil {
			return nil, fmt.Errorf("close processor archive output directory failed")
		}
		return nil, err
	}
	return destination, nil
}

func (r *privateOutputRoot) validate() error {
	opened, handleErr := r.root.Stat(".")
	current, err := os.Lstat(r.path) // #nosec G703 -- path is the already-validated absolute output root.
	if handleErr != nil || err != nil || !opened.IsDir() || !current.IsDir() || current.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(r.identity, opened) || !os.SameFile(r.identity, current) {
		return fmt.Errorf("processor archive output directory changed during use")
	}
	if current.Mode().Perm()&0o077 != 0 || current.Mode().Perm()&0o300 != 0o300 {
		return fmt.Errorf("processor archive output directory must remain private and owner-writable")
	}
	return nil
}

func validLocalPath(path string) bool {
	if path == "" || len(path) > 4096 || !utf8.ValidString(path) || strings.TrimSpace(path) != path {
		return false
	}
	for _, character := range path {
		if unicode.Is(unicode.C, character) {
			return false
		}
	}
	return true
}

func requireMissingOutput(root *os.Root, name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return fmt.Errorf("processor archive output name is unsafe")
	}
	if _, err := root.Lstat(name); err == nil {
		return fmt.Errorf("processor archive output already exists")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect processor archive output failed")
	}
	return nil
}

func writeVerifiedArchive(root *os.Root, name string, data []byte) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, privateArchiveMode)
	if err != nil {
		return fmt.Errorf("create processor archive output failed")
	}
	remove := true
	defer func() {
		if remove {
			_ = root.Remove(name)
		}
	}()
	written, copyErr := io.Copy(file, bytes.NewReader(data))
	syncErr := file.Sync()
	opened, statErr := file.Stat()
	closeErr := file.Close()
	if copyErr != nil || syncErr != nil || statErr != nil || closeErr != nil || written != int64(len(data)) {
		return fmt.Errorf("write processor archive output failed")
	}
	info, err := root.Lstat(name)
	if err != nil || !opened.Mode().IsRegular() || !info.Mode().IsRegular() || !os.SameFile(opened, info) ||
		opened.Mode().Perm() != privateArchiveMode || info.Mode().Perm() != privateArchiveMode ||
		opened.Size() != int64(len(data)) || info.Size() != int64(len(data)) {
		return fmt.Errorf("verify processor archive output failed")
	}
	// Consumers re-open and re-hash this pathname before extraction or execution;
	// no returned filesystem path can itself remain race-proof after this check.
	remove = false
	return nil
}
