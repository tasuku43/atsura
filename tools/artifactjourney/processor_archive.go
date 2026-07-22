package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tasuku43/atsura/tools/internal/processormanifest"
)

type processorArchiveContract struct {
	archiveName, archiveSHA256, binaryMember, binarySHA256 string
	archiveSize, binarySize                                int64
}

// extractProcessorArchive admits one already-downloaded archive only after it
// matches the code-pinned repository manifest byte for byte. The official RTK
// archives in this contract contain exactly one executable member.
func extractProcessorArchive(path, destination string, metadata processormanifest.TargetMetadata) (string, error) {
	return extractProcessorArchiveContract(path, destination, processorArchiveContract{
		archiveName: metadata.ArchiveName(), archiveSHA256: metadata.ArchiveSHA256(), archiveSize: metadata.ArchiveSize(),
		binaryMember: metadata.BinaryMember(), binarySHA256: metadata.BinarySHA256(), binarySize: metadata.BinarySize(),
	})
}

func extractProcessorArchiveContract(path, destination string, contract processorArchiveContract) (string, error) {
	if filepath.Base(path) != contract.archiveName {
		return "", fmt.Errorf("processor archive name does not match pinned provenance")
	}
	archive, err := readExactRegularFile(path, contract.archiveSize)
	if err != nil || digestBytes(archive) != contract.archiveSHA256 {
		return "", fmt.Errorf("processor archive bytes do not match pinned provenance")
	}
	if err := os.Mkdir(destination, 0o700); err != nil {
		return "", fmt.Errorf("processor extraction directory creation failed")
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return "", fmt.Errorf("processor extraction directory open failed")
	}
	defer root.Close()

	compressed, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return "", fmt.Errorf("processor archive gzip header is invalid")
	}
	compressed.Multistream(false)
	reader := tar.NewReader(compressed)
	header, err := reader.Next()
	if err != nil || header.Typeflag != tar.TypeReg || header.Name != contract.binaryMember ||
		filepath.Base(header.Name) != header.Name || filepath.Clean(header.Name) != header.Name ||
		header.Mode != 0o755 || header.Size != contract.binarySize {
		_ = compressed.Close()
		return "", fmt.Errorf("processor archive member contract is invalid")
	}
	target, err := root.OpenFile(header.Name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o755)
	if err != nil {
		_ = compressed.Close()
		return "", fmt.Errorf("processor archive member creation failed")
	}
	binaryHash := sha256.New()
	written, copyErr := io.CopyN(io.MultiWriter(target, binaryHash), reader, header.Size)
	closeErr := target.Close()
	if copyErr != nil || written != header.Size || closeErr != nil {
		_ = compressed.Close()
		return "", fmt.Errorf("processor archive member extraction failed")
	}
	if fmt.Sprintf("%x", binaryHash.Sum(nil)) != contract.binarySHA256 {
		_ = compressed.Close()
		return "", fmt.Errorf("processor archive binary identity does not match pinned provenance")
	}
	if _, err := reader.Next(); err != io.EOF {
		_ = compressed.Close()
		return "", fmt.Errorf("processor archive member set is invalid")
	}
	if err := compressed.Close(); err != nil {
		return "", fmt.Errorf("processor archive close failed")
	}
	executable := filepath.Join(destination, contract.binaryMember)
	digest, size, err := regularFileIdentity(executable)
	if err != nil || digest != contract.binarySHA256 || size != contract.binarySize {
		return "", fmt.Errorf("extracted processor identity is invalid")
	}
	return executable, nil
}

func readExactRegularFile(path string, wantedSize int64) ([]byte, error) {
	if wantedSize <= 0 || wantedSize > maxArchiveBytes {
		return nil, fmt.Errorf("expected file size is invalid")
	}
	file, info, err := openRegularInput(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if info.Size() != wantedSize {
		return nil, fmt.Errorf("file size does not match")
	}
	value, err := io.ReadAll(io.LimitReader(file, wantedSize+1))
	if err != nil || int64(len(value)) != wantedSize {
		return nil, fmt.Errorf("file content is incomplete")
	}
	return value, nil
}
