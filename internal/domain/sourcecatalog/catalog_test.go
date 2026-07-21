package sourcecatalog

import (
	"errors"
	"strings"
	"testing"
)

func validCatalog() Catalog {
	return Catalog{
		SchemaVersion: SchemaVersion,
		Adapter:       Adapter{Kind: "atsura.source.synthetic", ContractVersion: 1},
		Source: Source{
			RequestedExecutable: "fixture", ResolvedPath: "/opt/fixture/bin/fixture",
			SHA256: strings.Repeat("a", 64), Size: 42, Version: "1.2.3",
		},
		Probe: Probe{IDs: []string{"help_reference", "version"}, Attempts: 2},
		Commands: []Command{{
			Path: []string{"item", "list"}, Summary: "List items",
			Provenance:       ProvenanceVerifiedBuiltin,
			Options:          []Option{{Name: "--json", TakesValue: true}},
			StructuredOutput: []StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{"id", "name"}}},
		}},
	}
}

func TestCatalogCanonicalJSONAndDigestAreDeterministic(t *testing.T) {
	first := validCatalog()
	second := validCatalog()
	firstBytes, err := first.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := second.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	firstDigest, _ := first.Digest()
	secondDigest, _ := second.Digest()
	if string(firstBytes) != string(secondBytes) || firstDigest != secondDigest || len(firstDigest) != 64 {
		t.Fatalf("canonical mismatch: %q %q", firstDigest, secondDigest)
	}
	if strings.Contains(string(firstBytes), "github") || strings.Contains(string(firstBytes), "claude") {
		t.Fatalf("synthetic core fixture contains a reference-vendor assumption: %s", firstBytes)
	}
}

func TestCatalogRejectsAuthorityAndCanonicalityGaps(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Catalog)
	}{
		{name: "unnamespaced adapter", mutate: func(c *Catalog) { c.Adapter.Kind = "github" }},
		{name: "missing source identity", mutate: func(c *Catalog) { c.Source.SHA256 = "" }},
		{name: "probe mismatch", mutate: func(c *Catalog) { c.Probe.Attempts = 1 }},
		{name: "unknown provenance", mutate: func(c *Catalog) { c.Commands[0].Provenance = "trusted" }},
		{name: "nil options", mutate: func(c *Catalog) { c.Commands[0].Options = nil }},
		{name: "unsorted fields", mutate: func(c *Catalog) { c.Commands[0].StructuredOutput[0].Fields = []string{"name", "id"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := validCatalog()
			test.mutate(&catalog)
			if err := catalog.Validate(); !errors.Is(err, ErrInvalidCatalog) {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestSortCreatesDetachedCanonicalOrder(t *testing.T) {
	catalog := validCatalog()
	catalog.Probe.IDs = []string{"version", "help_reference"}
	catalog.Commands[0].Options = []Option{{Name: "--repo"}, {Name: "--json", TakesValue: true}}
	catalog.Commands[0].StructuredOutput[0].Fields = []string{"name", "id"}
	sorted := Sort(catalog)
	if err := sorted.Validate(); err != nil {
		t.Fatal(err)
	}
	if sorted.Probe.IDs[0] != "help_reference" || sorted.Commands[0].Options[0].Name != "--json" || sorted.Commands[0].StructuredOutput[0].Fields[0] != "id" {
		t.Fatalf("Sort() = %+v", sorted)
	}
	if catalog.Probe.IDs[0] != "version" || catalog.Commands[0].Options[0].Name != "--repo" {
		t.Fatal("Sort mutated its input")
	}
}
