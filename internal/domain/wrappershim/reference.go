// Package wrappershim defines the immutable identity and reconciliation
// vocabulary for one Atsura-owned executable wrapper shim.
package wrappershim

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
)

const (
	// ContractVersion is the first persistent executable-shim contract.
	ContractVersion  = 1
	ReferencePrefix  = "wsh1_"
	DigestBytes      = 64
	ReferenceBytes   = len(ReferencePrefix) + DigestBytes
	MaxArtifacts     = 64
	MaxManifestBytes = 256 * 1024
	MaxShimBytes     = wrapperbinding.MaxRenderedSourceBytes
)

var ErrInvalidReference = errors.New("invalid wrapper shim reference")

// Reference is an opaque content address derived from the exact executable
// shim SHA-256. Callers pass it through unchanged; only this package parses it.
type Reference string

// NewReference derives the v1 opaque reference from one lowercase SHA-256.
func NewReference(shimSHA256 string) (Reference, error) {
	if !validDigest(shimSHA256) {
		return "", fmt.Errorf("%w: digest must contain 64 lowercase hex characters", ErrInvalidReference)
	}
	return Reference(ReferencePrefix + shimSHA256), nil
}

// ParseReference accepts only the exact canonical v1 representation.
func ParseReference(value string) (Reference, error) {
	reference := Reference(value)
	if err := reference.Validate(); err != nil {
		return "", err
	}
	return reference, nil
}

// Validate rejects every non-canonical, retired, future, or malformed value.
func (r Reference) Validate() error {
	value := string(r)
	if len(value) != ReferenceBytes || !strings.HasPrefix(value, ReferencePrefix) || !validDigest(strings.TrimPrefix(value, ReferencePrefix)) {
		return fmt.Errorf("%w: must match wsh1_<64 lowercase hex characters>", ErrInvalidReference)
	}
	return nil
}

func (r Reference) String() string { return string(r) }

// Digest returns the exact shim digest after validating the opaque reference.
func (r Reference) Digest() (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	return strings.TrimPrefix(string(r), ReferencePrefix), nil
}

func validDigest(value string) bool {
	if len(value) != DigestBytes {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}
