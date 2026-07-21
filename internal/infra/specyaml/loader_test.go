package specyaml

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
)

const specificationFixture = `schema_version: 3
catalog_digest: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
surface:
  default: exclude
commands:
  - command: [item, delete]
    presence: exclude
    reason: Deletion is outside this purpose.
  - command: [item, list]
    presence: include
    reason: Compact inventory.
    options:
      default: inherit
      include: []
      exclude: [--format]
    wrapper:
      kind: transform
      before: []
      invoke:
        append_args: [--json=id,name]
      output:
        input: json
        select: [id, name]
        rename: []
        render: compact_json
      after: []
`

func writeFixture(t *testing.T, value string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "specification.yaml")
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadStrictSchema3Specification(t *testing.T) {
	specification, err := New().Load(context.Background(), writeFixture(t, specificationFixture))
	if err != nil {
		t.Fatal(err)
	}
	if specification.SchemaVersion != 3 || specification.Surface.Default != tailoringbundle.SurfaceDefaultExclude || len(specification.Commands) != 2 {
		t.Fatalf("specification = %+v", specification)
	}
	entry := specification.Commands[1]
	if entry.Presence != tailoringbundle.PresenceInclude || entry.Options == nil || entry.Wrapper == nil || entry.Wrapper.Kind != tailoringbundle.WrapperTransform {
		t.Fatalf("entry = %+v", entry)
	}
	if entry.Wrapper.Before == nil || entry.Wrapper.After == nil || entry.Wrapper.Invoke.AppendArgs == nil {
		t.Fatalf("explicit wrapper lists were lost: %+v", entry.Wrapper)
	}
}

func TestEncodeIdentitySpecificationRoundTrips(t *testing.T) {
	specification := tailoringbundle.Specification{
		SchemaVersion: 3, CatalogDigest: strings.Repeat("a", 64), Surface: tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands: []tailoringbundle.CommandEntry{{
			Command: []string{"item", "list"}, Presence: tailoringbundle.PresenceInclude, Reason: "Include without transformation.",
			Options: &tailoringbundle.OptionSurface{Default: tailoringbundle.SurfaceDefaultInherit, Include: []string{}, Exclude: []string{}},
			Wrapper: &tailoringbundle.Wrapper{Kind: tailoringbundle.WrapperIdentity, Before: []tailoringbundle.StageAction{}, Invoke: tailoringbundle.Invocation{AppendArgs: []string{}}, After: []tailoringbundle.StageAction{}},
		}},
	}
	raw, err := Encode(specification)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(raw)
	for _, expected := range []string{"surface:", "default: exclude", "before: []", "append_args: []", "after: []"} {
		if !strings.Contains(encoded, expected) {
			t.Fatalf("encoded specification missing %q: %s", expected, encoded)
		}
	}
	for _, retired := range []string{"decision:", "effect:", "impact:", "target:"} {
		if strings.Contains(encoded, retired) {
			t.Fatalf("encoded specification retained %q: %s", retired, encoded)
		}
	}
	decoded, err := decode(raw)
	if err != nil || !equalSpecification(decoded, specification) {
		t.Fatalf("decoded = %+v, error = %v\nraw = %s", decoded, err, raw)
	}
}

func TestLoadRejectsLegacySchemasWithMigrationDiagnostic(t *testing.T) {
	for _, version := range []int{1, 2} {
		legacy := strings.Replace(specificationFixture, "schema_version: 3", "schema_version: "+string(rune('0'+version)), 1)
		_, err := New().Load(context.Background(), writeFixture(t, legacy))
		public, ok := fault.PublicCopy(err)
		if !ok || public.Code != "legacy_tailoring_schema" || public.Retryable {
			t.Fatalf("schema %d error = %v", version, err)
		}
		if !errors.Is(err, ErrLegacyTailoringSchema) {
			t.Fatalf("schema %d did not retain migration cause: %v", version, err)
		}
	}
}

func TestLoadRejectsUnknownAliasMultipleAndOversize(t *testing.T) {
	tests := []struct {
		name  string
		value string
		code  string
	}{
		{name: "unknown", value: strings.Replace(specificationFixture, "surface:", "unknown: true\nsurface:", 1), code: "invalid_specification_yaml"},
		{name: "alias", value: strings.Replace(specificationFixture, "command: [item, delete]", "command: &path [item, delete]", 1) + "extra: *path\n", code: "invalid_specification_yaml"},
		{name: "multiple", value: specificationFixture + "---\n" + specificationFixture, code: "invalid_specification_yaml"},
		{name: "oversize", value: strings.Repeat("x", int(maxSpecificationBytes)+1), code: "specification_file_too_large"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New().Load(context.Background(), writeFixture(t, test.value))
			public, ok := fault.PublicCopy(err)
			if !ok || public.Code != test.code {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func equalSpecification(left, right tailoringbundle.Specification) bool {
	leftBytes, leftErr := Encode(left)
	rightBytes, rightErr := Encode(right)
	return leftErr == nil && rightErr == nil && string(leftBytes) == string(rightBytes)
}
