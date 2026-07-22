// Package bundletrust defines exact-digest user adoption receipts and source
// drift states for one content-addressed tailoring bundle. The package name is
// retained for the public `bundle trust` command; a receipt grants no source
// operation permission.
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
	StateAdopted    State = "adopted"
	StateNotAdopted State = "not_adopted"
	StateInvalid    State = "invalid"
)

type SourceState string

const (
	SourceCurrent     SourceState = "current"
	SourceDrifted     SourceState = "drifted"
	SourceUnavailable SourceState = "unavailable"
)

// ProcessorState reports whether one exact bundle-bound processor executable
// can still be identified without starting it.
type ProcessorState string

const (
	ProcessorCurrent     ProcessorState = "current"
	ProcessorDrifted     ProcessorState = "drifted"
	ProcessorUnavailable ProcessorState = "unavailable"
)

// ProcessorSummary is the exact non-secret processor evidence shown before
// adoption. It contains no output bytes or ambient configuration.
type ProcessorSummary struct {
	Contract     string
	AdapterKind  string
	Version      string
	ResolvedPath string
	SHA256       string
	Size         int64
	InputFormat  string
	OutputFormat string
}

// Receipt records only exact reviewed bundle identity. Surface, wrapper, and
// source facts are recomputed from the validated bundle whenever status is
// requested.
type Receipt struct {
	BundleDigest string `json:"bundle_digest"`
}

// Store is the one user-local, non-secret bundle-adoption document.
type Store struct {
	SchemaVersion int       `json:"schema_version"`
	Receipts      []Receipt `json:"receipts"`
}

// Summary is the material surface-and-wrapper summary shown on a controlling
// terminal before an adoption receipt is added. It contains no captured source
// output and no source-operation permission classification.
type Summary struct {
	BundleDigest              string
	CatalogDigest             string
	SpecificationDigest       string
	SourcePath                string
	SourceSHA256              string
	SourceVersion             string
	SurfaceDefault            string
	IncludedCommandCount      int
	ExcludedCommandCount      int
	IdentityWrapperCount      int
	TransformWrapperCount     int
	OptionOverrideCount       int
	OptionDefaultCount        int
	ArgvTransformationCount   int
	BeforeActionCount         int
	AfterActionCount          int
	OutputTransformationCount int
	SourceStreamResultCount   int
	OptimizerResultCount      int
	Processors                []ProcessorSummary
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
