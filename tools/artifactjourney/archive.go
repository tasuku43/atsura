package main

import (
	"archive/tar"
	"archive/zip"
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

func archiveDigest(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= 0 || info.Size() > maxArchiveBytes {
		return "", fmt.Errorf("archive size is invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("archive open failed")
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("archive digest failed")
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func extractReleaseArchive(path, goos, destination string) (string, error) {
	if err := os.Mkdir(destination, 0o700); err != nil {
		return "", fmt.Errorf("extraction directory creation failed")
	}
	executable := "atr"
	if goos == "windows" {
		executable = "atr.exe"
	}
	var err error
	if goos == "windows" {
		err = extractZIP(path, destination)
	} else {
		err = extractTarGzip(path, destination)
	}
	if err != nil {
		return "", err
	}
	executablePath := filepath.Join(destination, executable)
	if info, statErr := os.Stat(executablePath); statErr != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return "", fmt.Errorf("archive executable is missing")
	}
	if err := os.Chmod(executablePath, 0o755); err != nil {
		return "", fmt.Errorf("archive executable mode failed")
	}
	return executablePath, nil
}

func extractTarGzip(path, destination string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("archive open failed")
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
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
		if err := extractMember(destination, header.Name, fs.FileMode(header.Mode), header.Size, reader, seen); err != nil {
			return err
		}
	}
	return validateMemberSet(seen)
}

func extractZIP(path, destination string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("archive zip stream is invalid")
	}
	defer reader.Close()
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

func extractMember(destination, name string, mode fs.FileMode, size int64, source io.Reader, seen map[string]struct{}) error {
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
	target, err := os.OpenFile(filepath.Join(destination, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, wantedMode)
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
