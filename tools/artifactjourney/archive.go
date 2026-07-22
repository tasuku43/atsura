package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	maxArchiveBytes = int64(256 * 1024 * 1024)
	maxMemberBytes  = int64(128 * 1024 * 1024)
)

var allowedArchiveMembers = map[string]fs.FileMode{
	"atr":                 0o755,
	"atr.exe":             0o755,
	"LICENSE":             0o644,
	"THIRD_PARTY_NOTICES": 0o644,
}

func prepareInputFile(value string) (string, error) {
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve input")
	}
	absolute = filepath.Clean(absolute)
	info, err := os.Lstat(absolute)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("input is not a regular file")
	}
	return absolute, nil
}

// readReleaseArchive binds the evidence digest and later extraction to one
// bounded open of the candidate archive.
func readReleaseArchive(path string) ([]byte, string, error) {
	file, info, err := openRegularInput(path)
	if err != nil || info.Size() <= 0 || info.Size() > maxArchiveBytes {
		if file != nil {
			_ = file.Close()
		}
		return nil, "", fmt.Errorf("archive size is invalid")
	}
	defer file.Close()
	value, err := io.ReadAll(io.LimitReader(file, maxArchiveBytes+1))
	if err != nil || int64(len(value)) != info.Size() {
		return nil, "", fmt.Errorf("archive read failed")
	}
	digest := sha256.Sum256(value)
	return value, fmt.Sprintf("%x", digest), nil
}

func extractReleaseArchive(archive []byte, goos, destination string) (string, error) {
	if len(archive) == 0 || int64(len(archive)) > maxArchiveBytes {
		return "", fmt.Errorf("archive size is invalid")
	}
	if err := os.Mkdir(destination, 0o700); err != nil {
		return "", fmt.Errorf("extraction directory creation failed")
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return "", fmt.Errorf("extraction directory open failed")
	}
	defer root.Close()
	executable := "atr"
	if goos == "windows" {
		executable = "atr.exe"
	}
	if goos == "windows" {
		err = extractZIP(archive, root)
	} else {
		err = extractTarGzip(archive, root)
	}
	if err != nil {
		return "", err
	}
	executablePath := filepath.Join(destination, executable)
	if info, statErr := root.Stat(executable); statErr != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return "", fmt.Errorf("archive executable is missing")
	}
	// #nosec G302 -- an extracted release binary must retain its reviewed executable mode.
	if err := root.Chmod(executable, 0o755); err != nil {
		return "", fmt.Errorf("archive executable mode failed")
	}
	return executablePath, nil
}

func extractTarGzip(archive []byte, destination *os.Root) error {
	compressed, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("archive gzip header is invalid")
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	seen := map[string]struct{}{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("archive tar stream is invalid")
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return fmt.Errorf("archive contains a non-regular member")
		}
		var mode fs.FileMode
		switch header.Mode {
		case 0o644:
			mode = 0o644
		case 0o755:
			mode = 0o755
		default:
			return fmt.Errorf("archive member mode is invalid")
		}
		if err := extractMember(destination, header.Name, mode, header.Size, reader, seen); err != nil {
			return err
		}
	}
	return validateMemberSet(seen)
}

func extractZIP(archive []byte, destination *os.Root) error {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return fmt.Errorf("archive zip stream is invalid")
	}
	seen := map[string]struct{}{}
	for _, member := range reader.File {
		if member.FileInfo().Mode()&os.ModeType != 0 || member.UncompressedSize64 > uint64(maxMemberBytes) {
			return fmt.Errorf("archive contains an invalid member")
		}
		stream, err := member.Open()
		if err != nil {
			return fmt.Errorf("archive member open failed")
		}
		err = extractMember(destination, member.Name, member.Mode(), int64(member.UncompressedSize64), stream, seen)
		closeErr := stream.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return fmt.Errorf("archive member close failed")
		}
	}
	return validateMemberSet(seen)
}

func extractMember(destination *os.Root, name string, mode fs.FileMode, size int64, source io.Reader, seen map[string]struct{}) error {
	wantedMode, allowed := allowedArchiveMembers[name]
	if !allowed || filepath.Base(name) != name || filepath.Clean(name) != name || size < 0 || size > maxMemberBytes {
		return fmt.Errorf("archive member contract is invalid")
	}
	if _, duplicate := seen[name]; duplicate {
		return fmt.Errorf("archive contains a duplicate member")
	}
	if mode.Perm() != wantedMode {
		return fmt.Errorf("archive member mode is invalid")
	}
	seen[name] = struct{}{}
	target, err := destination.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, wantedMode)
	if err != nil {
		return fmt.Errorf("archive member creation failed")
	}
	written, copyErr := io.CopyN(target, source, size)
	closeErr := target.Close()
	if copyErr != nil || written != size {
		return fmt.Errorf("archive member content is truncated")
	}
	var trailing [1]byte
	if count, err := source.Read(trailing[:]); count != 0 || (err != nil && err != io.EOF) {
		return fmt.Errorf("archive member size is inconsistent")
	}
	if closeErr != nil {
		return fmt.Errorf("archive member close failed")
	}
	return nil
}

func openRegularInput(path string) (*os.File, fs.FileInfo, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, nil, fmt.Errorf("input path is not absolute and clean")
	}
	parent, name := filepath.Split(path)
	if name == "" || filepath.Base(name) != name {
		return nil, nil, fmt.Errorf("input path does not name a file")
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	before, err := root.Lstat(name)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("input is not a regular file")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, nil, err
	}
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		_ = file.Close()
		return nil, nil, fmt.Errorf("input changed while opening")
	}
	return file, after, nil
}

func validateMemberSet(seen map[string]struct{}) error {
	if _, present := seen["LICENSE"]; !present {
		return fmt.Errorf("archive LICENSE is missing")
	}
	_, unixExecutable := seen["atr"]
	_, windowsExecutable := seen["atr.exe"]
	if unixExecutable == windowsExecutable {
		return fmt.Errorf("archive executable set is invalid")
	}
	if len(seen) != 2 && len(seen) != 3 {
		return fmt.Errorf("archive member set is invalid")
	}
	return nil
}
