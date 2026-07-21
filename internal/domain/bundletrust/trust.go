// Package bundletrust defines the vendor-neutral user trust and drift states
// for one content-addressed tailoring bundle.
package bundletrust

import (
	"errors"
	"fmt"
	"sort"
)

const (
	StoreSchemaVersion = 1
	MaxReceipts        = 256
)

var ErrInvalidStore = errors.New("invalid bundle trust store")

type State string

const (
	StateTrusted   State = "trusted"
	StateUntrusted State = "untrusted"
	StateInvalid   State = "invalid"
)

type SourceState string

const (
	SourceCurrent     SourceState = "current"
	SourceDrifted     SourceState = "drifted"
	SourceUnavailable SourceState = "unavailable"
)

// Receipt records only exact reviewed content identity. All authority facts
// are recomputed from the validated bundle when status or execution is asked.
type Receipt struct {
	BundleDigest string `json:"bundle_digest"`
}

// Store is the one user-local, non-secret trust authority document.
type Store struct {
	SchemaVersion int       `json:"schema_version"`
	Receipts      []Receipt `json:"receipts"`
}

// Summary is the material authority shown on a controlling terminal before a
// receipt is added. It deliberately contains no captured source output.
type Summary struct {
	BundleDigest  string
	CatalogDigest string
	PolicyDigest  string
	SourcePath    string
	SourceSHA256  string
	SourceVersion string
	VisibleCount  int
	ReadCount     int
	CreateCount   int
	WriteCount    int
	AllowCount    int
	ConfirmCount  int
	DenyCount     int
}

func EmptyStore() Store {
	return Store{SchemaVersion: StoreSchemaVersion, Receipts: []Receipt{}}
}

func (s Store) Validate() error {
	if s.SchemaVersion != StoreSchemaVersion || s.Receipts == nil || len(s.Receipts) > MaxReceipts {
		return fmt.Errorf("%w: schema and bounded receipts are required", ErrInvalidStore)
	}
	previous := ""
	for _, receipt := range s.Receipts {
		if !validDigest(receipt.BundleDigest) || (previous != "" && receipt.BundleDigest <= previous) {
			return fmt.Errorf("%w: receipt digests must be sorted, unique SHA-256 values", ErrInvalidStore)
		}
		previous = receipt.BundleDigest
	}
	return nil
}

func (s Store) Contains(digest string) bool {
	index := sort.Search(len(s.Receipts), func(i int) bool { return s.Receipts[i].BundleDigest >= digest })
	return index < len(s.Receipts) && s.Receipts[index].BundleDigest == digest
}

func (s Store) Add(digest string) (Store, bool, error) {
	if err := s.Validate(); err != nil || !validDigest(digest) {
		return Store{}, false, ErrInvalidStore
	}
	if s.Contains(digest) {
		return s, false, nil
	}
	if len(s.Receipts) == MaxReceipts {
		return Store{}, false, fmt.Errorf("%w: receipt limit reached", ErrInvalidStore)
	}
	result := Store{SchemaVersion: s.SchemaVersion, Receipts: append([]Receipt(nil), s.Receipts...)}
	result.Receipts = append(result.Receipts, Receipt{BundleDigest: digest})
	sort.Slice(result.Receipts, func(i, j int) bool { return result.Receipts[i].BundleDigest < result.Receipts[j].BundleDigest })
	return result, true, result.Validate()
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
