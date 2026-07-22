package wrappershim

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
)

var ErrInvalidManifest = errors.New("invalid wrapper shim manifest")

// Manifest is the immutable store record for one exact executable shim. It
// closes the bytes over the complete wrapper binding rather than retaining a
// path, command spelling, or digest that can drift independently.
type Manifest struct {
	ContractVersion int                    `json:"contract_version"`
	Reference       Reference              `json:"reference"`
	Binding         wrapperbinding.Binding `json:"binding"`
	MaterialSHA256  string                 `json:"material_sha256"`
	MaterialSize    int64                  `json:"material_size"`
}

// NewManifest binds one validated complete wrapper binding to exact rendered
// bytes. Both slice-bearing values are detached from caller-owned buffers.
func NewManifest(binding wrapperbinding.Binding, material wrapperbinding.RenderedMaterial) (Manifest, error) {
	if err := binding.Validate(); err != nil {
		return Manifest{}, invalidManifest("binding: %v", err)
	}
	if err := material.Validate(); err != nil {
		return Manifest{}, invalidManifest("material: %v", err)
	}
	reference, err := NewReference(material.SHA256)
	if err != nil {
		return Manifest{}, invalidManifest("artifact: %v", err)
	}
	manifest := Manifest{
		ContractVersion: ContractVersion,
		Reference:       reference,
		Binding:         binding.Clone(),
		MaterialSHA256:  material.SHA256,
		MaterialSize:    int64(len(material.Source)),
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest.Clone(), nil
}

// Validate rejects incomplete, mismatched, non-canonical, or unsupported
// records without consulting the filesystem.
func (m Manifest) Validate() error {
	if m.ContractVersion != ContractVersion {
		return invalidManifest("contract_version must be %d", ContractVersion)
	}
	if err := m.Reference.Validate(); err != nil {
		return invalidManifest("reference: %v", err)
	}
	if err := m.Binding.Validate(); err != nil {
		return invalidManifest("binding: %v", err)
	}
	if !validDigest(m.MaterialSHA256) {
		return invalidManifest("material_sha256 must contain 64 lowercase hex characters")
	}
	if m.MaterialSize <= 0 || m.MaterialSize > MaxShimBytes {
		return invalidManifest("material_size must be between 1 and %d", MaxShimBytes)
	}
	digest, err := m.Reference.Digest()
	if err != nil || digest != m.MaterialSHA256 {
		return invalidManifest("reference does not match material_sha256")
	}
	return nil
}

// CanonicalBytes returns the only accepted on-disk v1 representation.
func (m Manifest) CanonicalBytes() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	var document bytes.Buffer
	encoder := json.NewEncoder(&document)
	// Printable help text is already structurally validated and is persisted as
	// data, not embedded into HTML. Disabling HTML escaping keeps the manifest
	// bound aligned with the fixed shell material budget without changing text.
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(m.Clone()); err != nil || document.Len() > MaxManifestBytes {
		return nil, invalidManifest("canonical document exceeds the %d-byte bound", MaxManifestBytes)
	}
	return append([]byte(nil), document.Bytes()...), nil
}

// DecodeManifest strictly accepts only the bounded canonical v1 document.
// Unknown fields, trailing values, alternate whitespace, and retired schemas
// fail instead of being normalized into store authority.
func DecodeManifest(data []byte) (Manifest, error) {
	if len(data) == 0 || len(data) > MaxManifestBytes {
		return Manifest{}, invalidManifest("document must be non-empty and at most %d bytes", MaxManifestBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, invalidManifest("decode: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Manifest{}, invalidManifest("document contains trailing data")
	}
	canonical, err := manifest.CanonicalBytes()
	if err != nil {
		return Manifest{}, err
	}
	if !bytes.Equal(data, canonical) {
		return Manifest{}, invalidManifest("document is not canonical")
	}
	return manifest.Clone(), nil
}

// Clone deeply detaches the compiled-help slices inside the exact binding.
func (m Manifest) Clone() Manifest {
	result := m
	result.Binding = m.Binding.Clone()
	return result
}

// Equal compares all authority-bearing facts, including compiled help.
func (m Manifest) Equal(other Manifest) bool {
	return m.ContractVersion == other.ContractVersion &&
		m.Reference == other.Reference &&
		m.MaterialSHA256 == other.MaterialSHA256 &&
		m.MaterialSize == other.MaterialSize &&
		m.Binding.Equal(other.Binding)
}

func invalidManifest(format string, values ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidManifest, fmt.Sprintf(format, values...))
}
