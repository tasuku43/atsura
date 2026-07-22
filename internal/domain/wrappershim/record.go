package wrappershim

import (
	"errors"
	"fmt"
	"sort"

	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
)

var ErrInvalidInventory = errors.New("invalid wrapper shim inventory")

// State is a bounded reconciliation result. It describes Atsura-owned state;
// it never classifies or authorizes the downstream source operation.
type State string

const (
	StateOwnedActive      State = "owned_active"
	StateOwnedInactive    State = "owned_inactive"
	StateCollisionForeign State = "collision_foreign"
	StateCollisionSymlink State = "collision_symlink"
	StateCollisionSpecial State = "collision_special"
	StateTampered         State = "tampered"
)

// Record is the compact public/store reconciliation shape. Owned and tampered
// records bind their expected material by opaque reference and digest;
// collisions intentionally carry neither because Atsura does not own them.
type Record struct {
	CommandName    string    `json:"command_name"`
	State          State     `json:"state"`
	Reference      Reference `json:"reference,omitempty"`
	MaterialSHA256 string    `json:"material_sha256,omitempty"`
}

func (r Record) Validate() error {
	if err := validateRecordCommand(r.CommandName); err != nil {
		return err
	}
	switch r.State {
	case StateOwnedActive, StateOwnedInactive, StateTampered:
		if err := r.Reference.Validate(); err != nil || !validDigest(r.MaterialSHA256) {
			return fmt.Errorf("%w: owned record requires an exact reference and material digest", ErrInvalidInventory)
		}
		digest, _ := r.Reference.Digest()
		if digest != r.MaterialSHA256 {
			return fmt.Errorf("%w: reference does not match material digest", ErrInvalidInventory)
		}
	case StateCollisionForeign, StateCollisionSymlink, StateCollisionSpecial:
		if r.Reference != "" || r.MaterialSHA256 != "" {
			return fmt.Errorf("%w: collision records must not claim owned material", ErrInvalidInventory)
		}
	default:
		return fmt.Errorf("%w: unknown state %q", ErrInvalidInventory, r.State)
	}
	return nil
}

func validateRecordCommand(value string) error {
	// Reuse the exact generated-command grammar without making this status
	// summary a second source of command-name policy.
	if err := wrapperbinding.ValidateCommandName(value); err != nil {
		return fmt.Errorf("%w: command_name: %v", ErrInvalidInventory, err)
	}
	return nil
}

// Inventory is a canonical bounded status result. Owned records and foreign
// collisions have independent bounds so a full owned store cannot suppress a
// complete collision report.
type Inventory struct {
	Records    []Record `json:"records"`
	Collisions []Record `json:"collisions"`
}

func (i Inventory) Validate() error {
	if i.Records == nil || len(i.Records) > MaxArtifacts || i.Collisions == nil || len(i.Collisions) > MaxArtifacts {
		return fmt.Errorf("%w: records must be an explicit list of at most %d entries", ErrInvalidInventory, MaxArtifacts)
	}
	if err := validatePartition(i.Records, false); err != nil {
		return err
	}
	if err := validatePartition(i.Collisions, true); err != nil {
		return err
	}
	return nil
}

// SortInventory returns a detached canonical inventory.
func SortInventory(records, collisions []Record) (Inventory, error) {
	result := Inventory{Records: append([]Record(nil), records...), Collisions: append([]Record(nil), collisions...)}
	sort.Slice(result.Records, func(left, right int) bool {
		return recordKey(result.Records[left]) < recordKey(result.Records[right])
	})
	sort.Slice(result.Collisions, func(left, right int) bool {
		return recordKey(result.Collisions[left]) < recordKey(result.Collisions[right])
	})
	if err := result.Validate(); err != nil {
		return Inventory{}, err
	}
	return result, nil
}

func (i Inventory) Clone() Inventory {
	return Inventory{Records: append([]Record(nil), i.Records...), Collisions: append([]Record(nil), i.Collisions...)}
}

func validatePartition(records []Record, collisions bool) error {
	previous := ""
	for index, record := range records {
		if err := record.Validate(); err != nil {
			return fmt.Errorf("%w: record %d: %v", ErrInvalidInventory, index, err)
		}
		isCollision := record.State == StateCollisionForeign || record.State == StateCollisionSymlink || record.State == StateCollisionSpecial
		if isCollision != collisions {
			return fmt.Errorf("%w: state %q is in the wrong inventory partition", ErrInvalidInventory, record.State)
		}
		key := recordKey(record)
		if previous != "" && key <= previous {
			return fmt.Errorf("%w: records must be sorted and unique", ErrInvalidInventory)
		}
		previous = key
	}
	return nil
}

func recordKey(record Record) string {
	return record.CommandName + "\x00" + string(record.State) + "\x00" + string(record.Reference)
}
