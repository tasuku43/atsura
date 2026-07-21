// Package tailoringbundle defines the canonical vendor-neutral policy and
// compiled runtime authority shared by gateways and host adapters.
package tailoringbundle

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
)

const (
	PolicySchemaVersion = 2
	BundleSchemaVersion = 1
	MaxRules            = 256
)

var (
	ErrInvalidPolicy = errors.New("invalid compiled tailoring policy")
	ErrInvalidBundle = errors.New("invalid tailoring bundle")
)

type Visibility string

const (
	VisibilityVisible Visibility = "visible"
	VisibilityHidden  Visibility = "hidden"
)

type Decision string

const (
	DecisionAllow   Decision = "allow"
	DecisionConfirm Decision = "confirm"
	DecisionDeny    Decision = "deny"
)

// TargetBinding declares how a mutation plan obtains one target from exact
// argv. It is semantic data, never an interpolation or shell expression.
type TargetBinding struct {
	Kind          string `json:"kind"`
	ArgumentIndex *int   `json:"argument_index,omitempty"`
	Flag          string `json:"flag,omitempty"`
}

// Rename changes one selected source field in the agent-facing result.
type Rename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Output is the normalized built-in structured transformation contract.
type Output struct {
	Input  string   `json:"input"`
	Select []string `json:"select"`
	Rename []Rename `json:"rename"`
	Render string   `json:"render"`
}

// Rule tailors one exact catalog command. Missing commands are deny-by-default.
type Rule struct {
	Command    []string          `json:"command"`
	Visibility Visibility        `json:"visibility"`
	Effect     operation.Effect  `json:"effect"`
	Decision   Decision          `json:"decision"`
	Reason     string            `json:"reason"`
	AppendArgs []string          `json:"append_args"`
	Target     *TargetBinding    `json:"target,omitempty"`
	Impact     *operation.Impact `json:"impact,omitempty"`
	Output     *Output           `json:"output,omitempty"`
}

// Policy is the normalized schema-2 content bound to one exact catalog.
type Policy struct {
	SchemaVersion int    `json:"schema_version"`
	CatalogDigest string `json:"catalog_digest"`
	Rules         []Rule `json:"rules"`
}

// SurfaceEntry is the host-independent agent-facing projection.
type SurfaceEntry struct {
	Command  []string         `json:"command"`
	Effect   operation.Effect `json:"effect"`
	Decision Decision         `json:"decision"`
	Reason   string           `json:"reason"`
}

// Bundle is the sole compiled policy artifact consumed at runtime.
type Bundle struct {
	SchemaVersion int                   `json:"schema_version"`
	CatalogDigest string                `json:"catalog_digest"`
	Catalog       sourcecatalog.Catalog `json:"catalog"`
	PolicyDigest  string                `json:"policy_digest"`
	Policy        Policy                `json:"policy"`
	Surface       []SurfaceEntry        `json:"surface"`
}

// Compile validates and binds catalog, policy, and tailored surface.
func Compile(catalog sourcecatalog.Catalog, policy Policy) (Bundle, error) {
	catalogDigest, err := catalog.Digest()
	if err != nil {
		return Bundle{}, invalidBundle("catalog: %v", err)
	}
	if err := policy.Validate(catalog); err != nil {
		return Bundle{}, err
	}
	if policy.CatalogDigest != catalogDigest {
		return Bundle{}, invalidPolicy("catalog digest does not match the supplied catalog")
	}
	policyDigest, err := policy.Digest(catalog)
	if err != nil {
		return Bundle{}, err
	}
	return Bundle{
		SchemaVersion: BundleSchemaVersion,
		CatalogDigest: catalogDigest,
		Catalog:       catalog,
		PolicyDigest:  policyDigest,
		Policy:        policy,
		Surface:       deriveSurface(policy),
	}, nil
}

// Validate proves every digest and derived value rather than trusting stored
// bundle metadata.
func (b Bundle) Validate() error {
	if b.SchemaVersion != BundleSchemaVersion {
		return invalidBundle("schema_version must be %d", BundleSchemaVersion)
	}
	catalogDigest, err := b.Catalog.Digest()
	if err != nil || catalogDigest != b.CatalogDigest {
		return invalidBundle("catalog digest is invalid or mismatched")
	}
	if err := b.Policy.Validate(b.Catalog); err != nil {
		return invalidBundle("policy: %v", err)
	}
	policyDigest, err := b.Policy.Digest(b.Catalog)
	if err != nil || policyDigest != b.PolicyDigest {
		return invalidBundle("policy digest is invalid or mismatched")
	}
	if b.Policy.CatalogDigest != b.CatalogDigest {
		return invalidBundle("policy is bound to a different catalog")
	}
	if !reflect.DeepEqual(b.Surface, deriveSurface(b.Policy)) {
		return invalidBundle("tailored surface is not the policy-derived projection")
	}
	return nil
}

// Validate rejects ambiguous, unbounded, unsafe, or non-canonical policy.
func (p Policy) Validate(catalog sourcecatalog.Catalog) error {
	if p.SchemaVersion != PolicySchemaVersion {
		return invalidPolicy("schema_version must be %d", PolicySchemaVersion)
	}
	if len(p.CatalogDigest) != 64 || p.Rules == nil || len(p.Rules) == 0 || len(p.Rules) > MaxRules {
		return invalidPolicy("catalog digest and a non-empty bounded rules list are required")
	}
	wantedDigest, err := catalog.Digest()
	if err != nil || wantedDigest != p.CatalogDigest {
		return invalidPolicy("catalog digest does not match the validated catalog")
	}
	commands := make(map[string]sourcecatalog.Command, len(catalog.Commands))
	for _, command := range catalog.Commands {
		commands[strings.Join(command.Path, " ")] = command
	}
	previous := ""
	for index, rule := range p.Rules {
		if err := rule.validate(commands); err != nil {
			return invalidPolicy("rule %d: %v", index, err)
		}
		key := strings.Join(rule.Command, " ")
		if previous != "" && key <= previous {
			return invalidPolicy("rules must be sorted and unique by command")
		}
		previous = key
	}
	return nil
}

func (r Rule) validate(commands map[string]sourcecatalog.Command) error {
	if len(r.Command) == 0 || len(r.Command) > sourcecatalog.MaxCommandSegments {
		return fmt.Errorf("command must be a non-empty bounded path")
	}
	key := strings.Join(r.Command, " ")
	command, exists := commands[key]
	if !exists || command.Provenance != sourcecatalog.ProvenanceVerifiedBuiltin {
		return fmt.Errorf("command %q is not verified catalog evidence", key)
	}
	if r.Visibility != VisibilityVisible && r.Visibility != VisibilityHidden {
		return fmt.Errorf("visibility is invalid")
	}
	if err := r.Effect.Validate(); err != nil {
		return err
	}
	if err := validateText(r.Reason, 4096); err != nil {
		return fmt.Errorf("reason: %v", err)
	}
	if r.AppendArgs == nil || len(r.AppendArgs) > 64 {
		return fmt.Errorf("append_args must be an explicit bounded list")
	}
	for _, argument := range r.AppendArgs {
		if err := validateText(argument, 4096); err != nil {
			return fmt.Errorf("append argument: %v", err)
		}
	}
	switch r.Effect {
	case operation.EffectRead:
		if r.Decision != DecisionAllow && r.Decision != DecisionDeny {
			return fmt.Errorf("read decision must be allow or deny")
		}
		if r.Target != nil || r.Impact != nil {
			return fmt.Errorf("read rules must not declare mutation target or impact")
		}
	case operation.EffectCreate, operation.EffectWrite:
		if r.Decision != DecisionConfirm && r.Decision != DecisionDeny {
			return fmt.Errorf("create/write decision must be confirm or deny")
		}
		if r.Target == nil || r.Impact == nil {
			return fmt.Errorf("create/write rules require target and impact")
		}
		if err := r.Target.validate(); err != nil {
			return err
		}
		if err := r.Impact.Validate(); err != nil {
			return fmt.Errorf("impact: %v", err)
		}
	}
	if r.Decision == DecisionDeny {
		if len(r.AppendArgs) != 0 || r.Output != nil {
			return fmt.Errorf("deny rules cannot transform invocation or output")
		}
		return nil
	}
	if r.Output == nil {
		return fmt.Errorf("allowed or confirmed rules require a typed output plan")
	}
	return r.Output.validate(command)
}

func (t TargetBinding) validate() error {
	if !stableName(t.Kind) || (t.ArgumentIndex == nil) == (t.Flag == "") {
		return fmt.Errorf("target requires a stable kind and exactly one argument_index or flag binding")
	}
	if t.ArgumentIndex != nil && (*t.ArgumentIndex < 0 || *t.ArgumentIndex > 127) {
		return fmt.Errorf("target argument_index is out of bounds")
	}
	if t.Flag != "" && (!strings.HasPrefix(t.Flag, "--") || !stableName(strings.TrimPrefix(t.Flag, "--"))) {
		return fmt.Errorf("target flag is invalid")
	}
	return nil
}

func (o Output) validate(command sourcecatalog.Command) error {
	renames := make([]tailoring.Rename, len(o.Rename))
	for index, rename := range o.Rename {
		renames[index] = tailoring.Rename{From: rename.From, To: rename.To}
	}
	plan := tailoring.OutputPlan{Input: tailoring.InputFormat(o.Input), Select: o.Select, Rename: renames, Render: tailoring.RenderFormat(o.Render)}
	if err := plan.Validate(); err != nil {
		return fmt.Errorf("output: %v", err)
	}
	hasJSON := false
	for _, output := range command.StructuredOutput {
		if output.Format == "json" {
			hasJSON = true
			break
		}
	}
	if !hasJSON {
		return fmt.Errorf("output requests JSON not observed for command")
	}
	return nil
}

// CanonicalJSON encodes the normalized policy only after catalog-bound
// validation.
func (p Policy) CanonicalJSON(catalog sourcecatalog.Catalog) ([]byte, error) {
	if err := p.Validate(catalog); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("encode canonical policy: %w", err)
	}
	return append(encoded, '\n'), nil
}

func (p Policy) Digest(catalog sourcecatalog.Catalog) (string, error) {
	encoded, err := p.CanonicalJSON(catalog)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded)), nil
}

func (b Bundle) CanonicalJSON() ([]byte, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("encode canonical bundle: %w", err)
	}
	return append(encoded, '\n'), nil
}

func (b Bundle) Digest() (string, error) {
	encoded, err := b.CanonicalJSON()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded)), nil
}

// SortPolicy detaches and canonicalizes rule order and list fields.
func SortPolicy(p Policy) Policy {
	result := p
	result.Rules = append(make([]Rule, 0, len(p.Rules)), p.Rules...)
	for index := range result.Rules {
		rule := &result.Rules[index]
		rule.Command = append(make([]string, 0, len(rule.Command)), rule.Command...)
		rule.AppendArgs = append(make([]string, 0, len(rule.AppendArgs)), rule.AppendArgs...)
		if rule.Output != nil {
			copy := *rule.Output
			copy.Select = append(make([]string, 0, len(copy.Select)), copy.Select...)
			copy.Rename = append(make([]Rename, 0, len(copy.Rename)), copy.Rename...)
			rule.Output = &copy
		}
	}
	sort.Slice(result.Rules, func(i, j int) bool {
		return strings.Join(result.Rules[i].Command, " ") < strings.Join(result.Rules[j].Command, " ")
	})
	return result
}

func deriveSurface(policy Policy) []SurfaceEntry {
	result := make([]SurfaceEntry, 0, len(policy.Rules))
	for _, rule := range policy.Rules {
		if rule.Visibility == VisibilityVisible {
			result = append(result, SurfaceEntry{Command: append([]string(nil), rule.Command...), Effect: rule.Effect, Decision: rule.Decision, Reason: rule.Reason})
		}
	}
	return result
}

func stableName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index, r := range value {
		if (r >= 'a' && r <= 'z') || (index > 0 && r >= '0' && r <= '9') || (index > 0 && (r == '-' || r == '_')) {
			continue
		}
		return false
	}
	return true
}

func validateText(value string, limit int) error {
	if value == "" || len(value) > limit || !utf8.ValidString(value) {
		return fmt.Errorf("must be non-empty bounded UTF-8")
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029'
	}) >= 0 {
		return fmt.Errorf("contains unsupported structural text")
	}
	return nil
}

func invalidPolicy(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidPolicy, fmt.Sprintf(format, args...))
}

func invalidBundle(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidBundle, fmt.Sprintf(format, args...))
}
