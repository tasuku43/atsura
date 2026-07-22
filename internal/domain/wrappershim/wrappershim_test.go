package wrappershim

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
)

func validBinding(t *testing.T) wrapperbinding.Binding {
	t.Helper()
	root := t.TempDir()
	defaultValue := "30"
	return wrapperbinding.Binding{
		ContractVersion: wrapperbinding.ContractVersion,
		BundleLocator:   filepath.Join(root, "purpose.json"),
		BundleDigest:    strings.Repeat("a", 64),
		CommandName:     "gh",
		Help: wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
			Path:    []string{"pr", "list"},
			Summary: "List pull requests",
			Reason:  "Keep a bounded inventory",
			Options: []wrapperbinding.HelpOption{{Name: "--limit", TakesValue: true, DefaultValue: &defaultValue}},
		}}},
		Runtime: wrapperbinding.RuntimeIdentity{
			ResolvedPath: filepath.Join(root, "atr"),
			SHA256:       strings.Repeat("b", 64),
			Size:         4242,
		},
	}
}

func validMaterial(t *testing.T) wrapperbinding.RenderedMaterial {
	t.Helper()
	material, err := wrapperbinding.NewRenderedMaterial([]byte("#!/bin/sh\n'/fixed/atr' -- \"$@\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	return material
}

func TestReferenceIsExactV1ShimDigestAddress(t *testing.T) {
	digest := strings.Repeat("a", 64)
	reference, err := NewReference(digest)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := reference.String(), ReferencePrefix+digest; got != want || len(got) != ReferenceBytes {
		t.Fatalf("reference = %q, want %q", got, want)
	}
	parsed, err := ParseReference(reference.String())
	if err != nil || parsed != reference {
		t.Fatalf("ParseReference() = %q, %v", parsed, err)
	}
	gotDigest, err := reference.Digest()
	if err != nil || gotDigest != digest {
		t.Fatalf("Digest() = %q, %v", gotDigest, err)
	}
}

func TestReferenceRejectsNonCanonicalAndUnsafeStoreKeys(t *testing.T) {
	digest := strings.Repeat("a", 64)
	values := []string{
		"", digest, "wsh0_" + digest, "wsh2_" + digest,
		"wsh1_" + strings.Repeat("A", 64), "wsh1_" + strings.Repeat("g", 64),
		"wsh1_" + digest[:63], "wsh1_" + digest + "0", "wsh1_../" + digest,
		"wsh1_" + digest[:32] + "\x00" + digest[33:],
	}
	for _, value := range values {
		if _, err := ParseReference(value); !errors.Is(err, ErrInvalidReference) {
			t.Errorf("ParseReference(%q) error = %v", value, err)
		}
	}
	for _, digest := range []string{"", strings.Repeat("A", 64), strings.Repeat("a", 63), strings.Repeat("a", 65)} {
		if _, err := NewReference(digest); !errors.Is(err, ErrInvalidReference) {
			t.Errorf("NewReference(%q) error = %v", digest, err)
		}
	}
}

func TestManifestBindsDetachedBindingAndExactMaterial(t *testing.T) {
	binding := validBinding(t)
	material := validMaterial(t)
	manifest, err := NewManifest(binding, material)
	if err != nil {
		t.Fatal(err)
	}
	wantReference, _ := NewReference(material.SHA256)
	if manifest.ContractVersion != ContractVersion || manifest.Reference != wantReference || manifest.MaterialSHA256 != material.SHA256 || manifest.MaterialSize != int64(len(material.Source)) || !manifest.Binding.Equal(binding) {
		t.Fatalf("manifest = %+v", manifest)
	}
	binding.Help.Commands[0].Path[0] = "mutated"
	material.Source[0] = 'x'
	if manifest.Binding.Help.Commands[0].Path[0] != "pr" {
		t.Fatal("NewManifest shared caller binding help")
	}
	clone := manifest.Clone()
	clone.Binding.Help.Commands[0].Options[0].Name = "--mutated"
	if manifest.Binding.Help.Commands[0].Options[0].Name != "--limit" {
		t.Fatal("Manifest.Clone shared compiled help")
	}
	if manifest.Equal(clone) {
		t.Fatal("Manifest.Equal ignored compiled help")
	}
}

func TestManifestCanonicalRoundTripIsStrictAndDetached(t *testing.T) {
	manifest, err := NewManifest(validBinding(t), validMaterial(t))
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := manifest.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	if len(canonical) == 0 || len(canonical) > MaxManifestBytes || canonical[len(canonical)-1] != '\n' {
		t.Fatalf("canonical length/terminator = %d/%q", len(canonical), canonical[len(canonical)-1:])
	}
	decoded, err := DecodeManifest(canonical)
	if err != nil || !decoded.Equal(manifest) {
		t.Fatalf("DecodeManifest() = %+v, %v", decoded, err)
	}
	decoded.Binding.Help.Commands[0].Path[0] = "changed"
	if manifest.Binding.Help.Commands[0].Path[0] != "pr" {
		t.Fatal("DecodeManifest shared decoded help")
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(canonical, &document); err != nil {
		t.Fatal(err)
	}
	document["unknown"] = json.RawMessage("true")
	unknown, _ := json.Marshal(document)
	for name, data := range map[string][]byte{
		"empty":                nil,
		"alternate whitespace": append([]byte(" "), canonical...),
		"missing newline":      bytes.TrimSuffix(canonical, []byte("\n")),
		"trailing value":       append(append([]byte{}, canonical...), []byte("{}")...),
		"unknown field":        append(unknown, '\n'),
		"unbounded":            bytes.Repeat([]byte("x"), MaxManifestBytes+1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeManifest(data); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("DecodeManifest() error = %v", err)
			}
		})
	}
}

func TestManifestRejectsDriftAndUnsupportedContract(t *testing.T) {
	valid, err := NewManifest(validBinding(t), validMaterial(t))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{name: "contract", mutate: func(value *Manifest) { value.ContractVersion++ }},
		{name: "reference", mutate: func(value *Manifest) { value.Reference = Reference("wsh1_" + strings.Repeat("c", 64)) }},
		{name: "binding", mutate: func(value *Manifest) { value.Binding.CommandName = "if" }},
		{name: "digest", mutate: func(value *Manifest) { value.MaterialSHA256 = strings.Repeat("A", 64) }},
		{name: "zero size", mutate: func(value *Manifest) { value.MaterialSize = 0 }},
		{name: "oversized", mutate: func(value *Manifest) { value.MaterialSize = MaxShimBytes + 1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid.Clone()
			test.mutate(&candidate)
			if err := candidate.Validate(); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestRecordAndInventoryEnforceOwnershipPartitionsAndBounds(t *testing.T) {
	digest := strings.Repeat("a", 64)
	reference, _ := NewReference(digest)
	inactiveDigest := strings.Repeat("b", 64)
	inactiveReference, _ := NewReference(inactiveDigest)
	tamperedDigest := strings.Repeat("c", 64)
	tamperedReference, _ := NewReference(tamperedDigest)
	owned := Record{CommandName: "gh", State: StateOwnedActive, Reference: reference, MaterialSHA256: digest}
	inactive := Record{CommandName: "go", State: StateOwnedInactive, Reference: inactiveReference, MaterialSHA256: inactiveDigest}
	tampered := Record{CommandName: "git", State: StateTampered, Reference: tamperedReference, MaterialSHA256: tamperedDigest}
	collision := Record{CommandName: "cargo", State: StateCollisionForeign}
	symlink := Record{CommandName: "go", State: StateCollisionSymlink}
	special := Record{CommandName: "git", State: StateCollisionSpecial}
	for _, record := range []Record{owned, inactive, tampered, collision, symlink, special} {
		if err := record.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v", record, err)
		}
	}
	inventory, err := SortInventory([]Record{owned, inactive, tampered}, []Record{collision, symlink, special})
	if err != nil || len(inventory.Records) != 3 || len(inventory.Collisions) != 3 {
		t.Fatalf("SortInventory() = %+v, %v", inventory, err)
	}
	clone := inventory.Clone()
	clone.Records[0].CommandName = "changed"
	if inventory.Records[0].CommandName == "changed" {
		t.Fatal("Inventory.Clone shared records")
	}

	invalid := []Record{
		{},
		{CommandName: "unsafe-name", State: StateOwnedActive, Reference: reference, MaterialSHA256: digest},
		{CommandName: "gh", State: "unknown"},
		{CommandName: "gh", State: StateOwnedActive},
		{CommandName: "gh", State: StateOwnedActive, Reference: reference, MaterialSHA256: strings.Repeat("b", 64)},
		{CommandName: "gh", State: StateCollisionForeign, Reference: reference, MaterialSHA256: digest},
	}
	for _, record := range invalid {
		if err := record.Validate(); !errors.Is(err, ErrInvalidInventory) {
			t.Errorf("invalid record %+v error = %v", record, err)
		}
	}
	if err := (Inventory{}).Validate(); !errors.Is(err, ErrInvalidInventory) {
		t.Fatalf("nil inventory error = %v", err)
	}
	empty, err := SortInventory([]Record{}, []Record{})
	if err != nil || empty.Records == nil || empty.Collisions == nil || empty.Clone().Records == nil || empty.Clone().Collisions == nil {
		t.Fatalf("explicit empty inventory lost its list shape: %+v, %v", empty, err)
	}
	if _, err := SortInventory([]Record{collision}, []Record{}); !errors.Is(err, ErrInvalidInventory) {
		t.Fatalf("collision in owned partition error = %v", err)
	}
	if _, err := SortInventory([]Record{}, []Record{owned}); !errors.Is(err, ErrInvalidInventory) {
		t.Fatalf("owned in collision partition error = %v", err)
	}
	tooMany := make([]Record, MaxArtifacts+1)
	for index := range tooMany {
		tooMany[index] = Record{CommandName: "gh", State: StateCollisionForeign}
	}
	if _, err := SortInventory([]Record{}, tooMany); !errors.Is(err, ErrInvalidInventory) {
		t.Fatalf("unbounded collision inventory error = %v", err)
	}

	otherDigest := strings.Repeat("d", 64)
	otherReference, _ := NewReference(otherDigest)
	invalidInventories := []Inventory{
		{Records: []Record{owned, {CommandName: "gh", State: StateOwnedActive, Reference: otherReference, MaterialSHA256: otherDigest}}, Collisions: []Record{}},
		{Records: []Record{owned, {CommandName: "go", State: StateOwnedInactive, Reference: reference, MaterialSHA256: digest}}, Collisions: []Record{}},
		{Records: []Record{}, Collisions: []Record{{CommandName: "gh", State: StateCollisionForeign}, {CommandName: "gh", State: StateCollisionSymlink}}},
		{Records: []Record{owned}, Collisions: []Record{{CommandName: "gh", State: StateCollisionForeign}}},
	}
	for index, candidate := range invalidInventories {
		_, sortErr := SortInventory(candidate.Records, candidate.Collisions)
		if !errors.Is(sortErr, ErrInvalidInventory) {
			t.Errorf("invalid inventory %d error = %v", index, sortErr)
		}
	}
}
