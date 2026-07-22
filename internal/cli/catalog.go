package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tasuku43/atsura/internal/domain/authn"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

const (
	// ProgramName is intentionally a single bootstrap replacement token.
	ProgramName = "atr"

	// maxAgentIndexEntryBytes bounds the selection-only root help cost per
	// command. Detailed invocation contracts belong in scoped help.
	maxAgentIndexEntryBytes = 512
)

type commandHandler func(context.Context, *CLI, CommandSpec, operation.Intent, ParsedInputs) int

type catalogFaultSignature struct {
	command   string
	kind      fault.Kind
	retryable bool
}

// CommandRole describes how a command participates in a deterministic task
// flow. RoleUnknown is the zero value so missing declarations fail closed.
type CommandRole uint8

const (
	RoleUnknown CommandRole = iota
	RoleUtility
	RoleDiscover
	RoleAct
)

func (r CommandRole) String() string {
	switch r {
	case RoleUtility:
		return "utility"
	case RoleDiscover:
		return "discover"
	case RoleAct:
		return "act"
	default:
		return "unknown"
	}
}

func (r CommandRole) validate() error {
	switch r {
	case RoleUtility, RoleDiscover, RoleAct:
		return nil
	default:
		return fmt.Errorf("command role is missing or invalid: %d", r)
	}
}

// ProducedRef declares an opaque reference written to one output field.
type ProducedRef struct {
	Kind  string `json:"kind"`
	Field string `json:"field"`
}

// ConsumedRef declares an opaque reference accepted by one argument.
type ConsumedRef struct {
	Kind     string `json:"kind"`
	Argument string `json:"argument"`
}

// FixedTargetScope identifies where a command-bound target is owned. The
// template currently permits only a singleton owned by this CLI installation.
type FixedTargetScope string

const (
	FixedTargetScopeUnknown   FixedTargetScope = ""
	FixedTargetScopeToolLocal FixedTargetScope = "tool_local"
)

// FixedTarget identifies one stable target selected entirely by the command
// path. It is mutually exclusive with produced or consumed opaque references.
type FixedTarget struct {
	Kind        string           `json:"kind"`
	ID          string           `json:"id"`
	Description string           `json:"description"`
	Scope       FixedTargetScope `json:"scope"`
}

// InputSource identifies the public channel through which one command input is
// supplied. InputSourceUnknown is invalid so an omitted source fails closed.
type InputSource string

const (
	InputSourceUnknown       InputSource = ""
	InputSourceArgument      InputSource = "argument"
	InputSourceFlag          InputSource = "flag"
	InputSourceStdin         InputSource = "stdin"
	InputSourceEnvironment   InputSource = "environment"
	InputSourceConfiguration InputSource = "configuration"
)

func (s InputSource) validate() error {
	switch s {
	case InputSourceArgument, InputSourceFlag, InputSourceStdin, InputSourceEnvironment, InputSourceConfiguration:
		return nil
	default:
		return fmt.Errorf("input source is missing or invalid: %q", s)
	}
}

// InputValueKind identifies the value grammar enforced by the catalog-owned
// command-line parser. The zero value is invalid so a new input cannot silently
// fall back to unbounded text.
type InputValueKind string

const (
	InputValueUnknown InputValueKind = ""
	InputValueText    InputValueKind = "text"
	InputValueInteger InputValueKind = "integer"
	InputValueBoolean InputValueKind = "boolean"
)

func (k InputValueKind) validate() error {
	switch k {
	case InputValueText, InputValueInteger, InputValueBoolean:
		return nil
	default:
		return fmt.Errorf("input value kind is missing or invalid: %q", k)
	}
}

// InputCardinality states whether one input name may contribute one value or
// several. Required independently distinguishes zero-or-one from exactly-one,
// and zero-or-more from one-or-more.
type InputCardinality string

const (
	InputCardinalityUnknown    InputCardinality = ""
	InputCardinalitySingle     InputCardinality = "single"
	InputCardinalityRepeatable InputCardinality = "repeatable"
)

func (c InputCardinality) validate() error {
	switch c {
	case InputCardinalitySingle, InputCardinalityRepeatable:
		return nil
	default:
		return fmt.Errorf("input cardinality is missing or invalid: %q", c)
	}
}

// CommandInput is one executable machine-readable input contract.
// DefaultValue is nil when omission has no catalog-owned default. Minimum and
// Maximum apply only to integer values. Requires and ConflictsWith are checked
// against explicitly supplied command-line inputs. ReferenceKind is empty only
// when the input is not an opaque reference.
type CommandInput struct {
	Name          string           `json:"name"`
	Source        InputSource      `json:"source"`
	Required      bool             `json:"required"`
	ValueKind     InputValueKind   `json:"value_kind"`
	Cardinality   InputCardinality `json:"cardinality"`
	Description   string           `json:"description"`
	AllowedValues []string         `json:"allowed_values"`
	DefaultValue  *string          `json:"default_value,omitempty"`
	Minimum       *int64           `json:"minimum,omitempty"`
	Maximum       *int64           `json:"maximum,omitempty"`
	Requires      []string         `json:"requires,omitempty"`
	ConflictsWith []string         `json:"conflicts_with,omitempty"`
	ReferenceKind string           `json:"reference_kind,omitempty"`
}

// OutputFormat identifies one stable presentation supported by a command.
type OutputFormat string

const (
	OutputFormatUnknown    OutputFormat = ""
	OutputFormatNone       OutputFormat = "none"
	OutputFormatText       OutputFormat = "text"
	OutputFormatTSV        OutputFormat = "tsv"
	OutputFormatJSON       OutputFormat = "json"
	OutputFormatPlanResult OutputFormat = "plan_result"
)

func (f OutputFormat) validate() error {
	switch f {
	case OutputFormatNone, OutputFormatText, OutputFormatTSV, OutputFormatJSON, OutputFormatPlanResult:
		return nil
	default:
		return fmt.Errorf("output format is missing or invalid: %q", f)
	}
}

// OutputFieldType is the stable machine type of one logical output field.
type OutputFieldType string

const (
	OutputFieldTypeUnknown OutputFieldType = ""
	OutputFieldTypeString  OutputFieldType = "string"
	OutputFieldTypeBoolean OutputFieldType = "boolean"
	OutputFieldTypeInteger OutputFieldType = "integer"
	OutputFieldTypeObject  OutputFieldType = "object"
	OutputFieldTypeArray   OutputFieldType = "array"
)

func (t OutputFieldType) validate() error {
	switch t {
	case OutputFieldTypeString, OutputFieldTypeBoolean, OutputFieldTypeInteger, OutputFieldTypeObject, OutputFieldTypeArray:
		return nil
	default:
		return fmt.Errorf("output field type is missing or invalid: %q", t)
	}
}

// OutputField declares one logical field independently of its presentation.
// ReferenceKind is empty only when the field is not an opaque reference.
type OutputField struct {
	Name          string          `json:"name"`
	Type          OutputFieldType `json:"type"`
	Description   string          `json:"description"`
	ReferenceKind string          `json:"reference_kind,omitempty"`
	Schema        *OutputSchema   `json:"schema,omitempty"`
}

// OutputSchema publishes a versioned, flat JSON-pointer inventory for a
// structured object, or for each object element of a structured array, whose
// nested shape would otherwise be opaque in agent help. Required applies when
// the field's parent object is present.
type OutputSchema struct {
	ID      string              `json:"id"`
	Version int                 `json:"version"`
	Fields  []OutputSchemaField `json:"fields"`
}

// OutputSchemaField describes one nested value. ElementType is required only
// for arrays. Nullable distinguishes an explicit JSON null from omission.
type OutputSchemaField struct {
	Path        string          `json:"path"`
	Type        OutputFieldType `json:"type"`
	ElementType OutputFieldType `json:"element_type,omitempty"`
	Required    bool            `json:"required"`
	Nullable    bool            `json:"nullable"`
}

// OutputAuthority identifies the exclusive contract that governs how one
// command interprets and presents a successful result. Catalog authority
// publishes static fields and, for JSON, a versioned envelope. Fresh-wrapper-
// plan authority applies the result mode in the freshly rebuilt plan. That
// mode either governs JSON selection/rendering or deliberately returns bounded
// source streams and conventional status.
type OutputAuthority string

const (
	OutputAuthorityUnknown          OutputAuthority = ""
	OutputAuthorityCatalog          OutputAuthority = "catalog"
	OutputAuthorityFreshWrapperPlan OutputAuthority = "fresh_wrapper_plan"
)

func (a OutputAuthority) validate() error {
	switch a {
	case OutputAuthorityCatalog, OutputAuthorityFreshWrapperPlan:
		return nil
	default:
		return fmt.Errorf("output authority is missing or invalid: %q", a)
	}
}

// OutputSchemaReference points to one exact structured field schema published
// by another catalog command. The reference is resolved during whole-catalog
// validation rather than copied into a competing schema declaration.
type OutputSchemaReference struct {
	Command string `json:"command"`
	Field   string `json:"field"`
	ID      string `json:"id"`
	Version int    `json:"version"`
}

// OutputJSONShape bounds the top-level semantic JSON value supplied by the
// source. It does not claim that the plan determines the source value's shape.
type OutputJSONShape string

const (
	OutputJSONShapeUnknown       OutputJSONShape = ""
	OutputJSONShapeObjectOrArray OutputJSONShape = "object_or_array"
)

// OutputJSONRendering describes the exact JSON byte representation promised
// by a plan-governed output authority.
type OutputJSONRendering string

const (
	OutputJSONRenderingUnknown OutputJSONRendering = ""
	OutputJSONRenderingCompact OutputJSONRendering = "compact_json"
)

// OutputJSONFraming states how one complete dynamic JSON value is delimited
// on stdout. It is separate from semantic shape and compact rendering so a
// caller never has to guess whether the successful value ends at EOF or LF.
type OutputJSONFraming string

const (
	OutputJSONFramingUnknown    OutputJSONFraming = ""
	OutputJSONFramingOneValueLF OutputJSONFraming = "one_value_lf"
)

// PlanResultStream describes one stream of a fresh-plan-authoritative result.
// These values describe bytes, not caller-selectable presentation formats.
type PlanResultStream string

const (
	PlanResultStreamUnknown                   PlanResultStream = ""
	PlanResultStreamCompactJSON               PlanResultStream = "compact_json"
	PlanResultStreamEmpty                     PlanResultStream = "empty"
	PlanResultStreamExactSourceBytes          PlanResultStream = "exact_bounded_source_bytes"
	PlanResultStreamExactAdmittedInputBytes   PlanResultStream = "byte_identical_admitted_input"
	PlanResultStreamValidatedOptimizerSummary PlanResultStream = "validated_newline_free_utf8_optimizer_summary"
)

// PlanResultExitStatus identifies which authority supplies a successful
// wrapper process status.
type PlanResultExitStatus string

const (
	PlanResultExitStatusUnknown            PlanResultExitStatus = ""
	PlanResultExitStatusZero               PlanResultExitStatus = "zero"
	PlanResultExitStatusSourceConventional PlanResultExitStatus = "source_conventional"
)

type PlanResultFraming string

const (
	PlanResultFramingUnknown    PlanResultFraming = ""
	PlanResultFramingOneValueLF PlanResultFraming = "one_value_lf"
	PlanResultFramingNone       PlanResultFraming = "none"
)

type PlanResultProjection string

const (
	PlanResultProjectionUnknown     PlanResultProjection = ""
	PlanResultProjectionVisibleJSON PlanResultProjection = "visible_json"
	PlanResultProjectionNone        PlanResultProjection = "none"
)

type PlanResultDelivery string

const (
	PlanResultDeliveryUnknown                 PlanResultDelivery = ""
	PlanResultDeliveryBufferedAfterCompletion PlanResultDelivery = "buffered_after_completion"
)

type PlanResultCrossStreamOrder string

const (
	PlanResultCrossStreamOrderUnknown       PlanResultCrossStreamOrder = ""
	PlanResultCrossStreamOrderNotApplicable PlanResultCrossStreamOrder = "not_applicable"
	PlanResultCrossStreamOrderNotPreserved  PlanResultCrossStreamOrder = "not_preserved"
)

// PlanResultDisposition identifies the exact successful optimizer branch.
// Non-optimizer modes use not_applicable so every mode has the same finite
// success-variant shape.
type PlanResultDisposition string

const (
	PlanResultDispositionUnknown                  PlanResultDisposition = ""
	PlanResultDispositionNotApplicable            PlanResultDisposition = "not_applicable"
	PlanResultDispositionPreservedBeforeProcessor PlanResultDisposition = "preserved_before_processor"
	PlanResultDispositionPreservedAfterProcessor  PlanResultDisposition = "preserved_after_processor"
	PlanResultDispositionOptimized                PlanResultDisposition = "optimized"
)

// PlanResultSuccessContract is one complete successful byte, status, and
// attempt variant of a fresh plan result mode.
type PlanResultSuccessContract struct {
	Disposition              PlanResultDisposition      `json:"disposition"`
	Stdout                   PlanResultStream           `json:"stdout"`
	Stderr                   PlanResultStream           `json:"stderr"`
	ExitStatus               PlanResultExitStatus       `json:"exit_status"`
	Framing                  PlanResultFraming          `json:"framing"`
	Projection               PlanResultProjection       `json:"projection"`
	Delivery                 PlanResultDelivery         `json:"delivery"`
	CrossStreamOrder         PlanResultCrossStreamOrder `json:"cross_stream_order"`
	StdoutLimitBytes         int                        `json:"stdout_limit_bytes"`
	StderrLimitBytes         int                        `json:"stderr_limit_bytes"`
	SourceProcessAttempts    int                        `json:"source_process_attempts"`
	ProcessorProcessAttempts int                        `json:"processor_process_attempts"`
}

// PlanResultModeContract is the complete finite success union for one
// result_mode declared by the freshly rebuilt wrapper plan.
type PlanResultModeContract struct {
	Mode            tailoringplan.ResultMode    `json:"mode"`
	SuccessVariants []PlanResultSuccessContract `json:"success_variants"`
}

// OutputDelivery states whether one invocation returns its complete selected
// result or one page in a public cursor protocol. It makes no claim about how
// much of an external collection the task selected.
type OutputDelivery string

const (
	OutputDeliveryUnknown  OutputDelivery = ""
	OutputDeliveryComplete OutputDelivery = "complete"
	OutputDeliveryPaged    OutputDelivery = "paged"
)

func (d OutputDelivery) validate() error {
	switch d {
	case OutputDeliveryComplete, OutputDeliveryPaged:
		return nil
	default:
		return fmt.Errorf("output delivery is missing or invalid: %q", d)
	}
}

// CollectionCoverage states what completing the delivery protocol covers
// within the exact declared task scope and observation. It never means every
// object or all history in the provider universe.
type CollectionCoverage string

const (
	CollectionCoverageUnknown            CollectionCoverage = ""
	CollectionCoverageNotApplicable      CollectionCoverage = "not_applicable"
	CollectionCoverageExhaustive         CollectionCoverage = "exhaustive"
	CollectionCoverageBoundedWindow      CollectionCoverage = "bounded_window"
	CollectionCoverageDifferentialWindow CollectionCoverage = "differential_window"
)

func (c CollectionCoverage) validate() error {
	switch c {
	case CollectionCoverageNotApplicable, CollectionCoverageExhaustive,
		CollectionCoverageBoundedWindow, CollectionCoverageDifferentialWindow:
		return nil
	default:
		return fmt.Errorf("collection coverage is missing or invalid: %q", c)
	}
}

// CommandOutput is the stable logical result and its supported presentations.
// Catalog-authoritative Fields describe values inside JSONEnvelope, never
// top-level metadata. Fresh-wrapper-plan-authoritative output has no static
// fields or maintainer envelope; its finite dynamic result modes are declared
// through PlanResultModes and its governing plan through PlanSchema. Help
// remains a deliberate selector-dependent command:
// its catalog fields describe the root index while scoped help projects the
// selected catalog contract.
type CommandOutput struct {
	Authority          OutputAuthority          `json:"authority"`
	Formats            []OutputFormat           `json:"formats"`
	DefaultFormat      OutputFormat             `json:"default_format"`
	Fields             []OutputField            `json:"fields"`
	Delivery           OutputDelivery           `json:"delivery"`
	CollectionCoverage CollectionCoverage       `json:"collection_coverage"`
	JSONEnvelope       string                   `json:"json_envelope,omitempty"`
	JSONSchemaVersion  int                      `json:"json_schema_version,omitempty"`
	PlanSchema         *OutputSchemaReference   `json:"plan_schema,omitempty"`
	JSONShape          OutputJSONShape          `json:"json_shape,omitempty"`
	JSONRendering      OutputJSONRendering      `json:"json_rendering,omitempty"`
	JSONFraming        OutputJSONFraming        `json:"json_framing,omitempty"`
	PlanResultModes    []PlanResultModeContract `json:"plan_result_modes,omitempty"`
}

// PaginationCompletion states the one machine-readable condition that marks
// traversal complete. A missing, null, or omitted cursor is not completion.
type PaginationCompletion string

const (
	PaginationCompletionUnknown     PaginationCompletion = ""
	PaginationCompletionEmptyCursor PaginationCompletion = "empty_cursor"
)

func (c PaginationCompletion) validate() error {
	if c != PaginationCompletionEmptyCursor {
		return fmt.Errorf("pagination completion is missing or invalid: %q", c)
	}
	return nil
}

// PaginationContract binds one optional public cursor input to the top-level
// string cursor field returned beside schema_version and the JSON envelope.
type PaginationContract struct {
	CursorInput  string               `json:"cursor_input"`
	CursorOutput OutputField          `json:"cursor_output"`
	Completion   PaginationCompletion `json:"completion"`
}

// CommandError declares one stable failure agents may handle without parsing
// prose. Kind and Code use the exact runtime fault taxonomy.
type CommandError struct {
	Code        string             `json:"code"`
	Kind        fault.Kind         `json:"kind"`
	Retryable   bool               `json:"retryable"`
	NextActions []fault.NextAction `json:"next_actions"`
}

// MutationContract connects a mutating command's public inputs to the target
// and generic impact facts consumed by the project-specific policy gate.
type MutationContract struct {
	TargetKind    string           `json:"target_kind"`
	TargetInputs  []string         `json:"target_inputs"`
	ParentInput   string           `json:"parent_input,omitempty"`
	TargetIDInput string           `json:"target_id_input,omitempty"`
	Impact        operation.Impact `json:"impact"`
}

// MarshalJSON projects policy-relevant impact enums as stable words rather
// than implementation-specific integer values.
func (m MutationContract) MarshalJSON() ([]byte, error) {
	type impactDocument struct {
		Cardinality  string `json:"cardinality"`
		Notification string `json:"notification"`
		AccessChange string `json:"access_change"`
		Destructive  string `json:"destructive"`
	}
	type mutationDocument struct {
		TargetKind    string         `json:"target_kind"`
		TargetInputs  []string       `json:"target_inputs"`
		ParentInput   string         `json:"parent_input,omitempty"`
		TargetIDInput string         `json:"target_id_input,omitempty"`
		Impact        impactDocument `json:"impact"`
	}
	return json.Marshal(mutationDocument{
		TargetKind: m.TargetKind, TargetInputs: m.TargetInputs,
		ParentInput: m.ParentInput, TargetIDInput: m.TargetIDInput,
		Impact: impactDocument{
			Cardinality: m.Impact.Cardinality.String(), Notification: m.Impact.Notification.String(),
			AccessChange: m.Impact.AccessChange.String(), Destructive: m.Impact.Destructive.String(),
		},
	})
}

// AgentContract contains the bounded information needed to invoke and
// interpret a command without exploratory calls. Nil slices mean unknown and
// are invalid; non-nil empty slices explicitly mean none.
type AgentContract struct {
	CapabilityID   string              `json:"capability_id"`
	Outcome        string              `json:"outcome"`
	Inputs         []CommandInput      `json:"inputs"`
	Output         CommandOutput       `json:"output"`
	Pagination     *PaginationContract `json:"pagination,omitempty"`
	Prerequisites  []string            `json:"prerequisites"`
	Authentication *authn.Requirement  `json:"authentication,omitempty"`
	FixedTarget    *FixedTarget        `json:"fixed_target,omitempty"`
	Errors         []CommandError      `json:"errors"`
	Mutation       *MutationContract   `json:"mutation,omitempty"`
}

// CommandSpec is the single source of truth for dispatch, human help, and the
// machine-readable agent specification.
type CommandSpec struct {
	Path    string
	Summary string
	Args    string
	Effect  operation.Effect
	Role    CommandRole
	Agent   AgentContract
	handler commandHandler
}

// Usage returns the complete command invocation without optional prose.
func (s CommandSpec) Usage() string {
	usage := ProgramName + " " + s.Path
	if s.Args != "" {
		usage += " " + s.Args
	}
	return usage
}

// Catalog owns the complete set of public command paths.
type Catalog struct {
	commands []CommandSpec
}

// NewCatalog creates a catalog from declarative command specifications.
func NewCatalog(commands ...CommandSpec) Catalog {
	cloned := make([]CommandSpec, len(commands))
	for index, command := range commands {
		cloned[index] = cloneCommandSpec(command)
	}
	return Catalog{commands: cloned}
}

func declaredCommandError(kind fault.Kind, code string, retryable bool, command, reason string) CommandError {
	return CommandError{
		Kind:        kind,
		Code:        code,
		Retryable:   retryable,
		NextActions: []fault.NextAction{{Command: command, Reason: reason}},
	}
}

func stringPointer(value string) *string {
	return &value
}

func int64Pointer(value int64) *int64 {
	return &value
}

func isMutationEffect(effect operation.Effect) bool {
	return effect == operation.EffectCreate || effect == operation.EffectWrite
}

func artifactInputErrors(command string, includeBundle bool) []CommandError {
	errors := []CommandError{
		declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help "+command, "Pass exact catalog and schema-4 specification paths."),
		declaredCommandError(fault.KindNotFound, "catalog_file_not_found", false, "source inspect", "Generate and select a source inspection JSON file."),
		declaredCommandError(fault.KindPermission, "catalog_file_permission_denied", false, "source inspect", "Correct catalog file permissions."),
		declaredCommandError(fault.KindInvalidInput, "unsafe_catalog_file", false, "source inspect", "Use a stable regular source inspection file."),
		declaredCommandError(fault.KindInvalidInput, "catalog_file_too_large", false, "source inspect", "Regenerate a bounded source inspection file."),
		declaredCommandError(fault.KindUnavailable, "catalog_file_read_failed", true, "source inspect", "Retry after the catalog file is readable."),
		declaredCommandError(fault.KindInvalidInput, "invalid_catalog_file", false, "source inspect", "Regenerate strict source inspection JSON."),
		declaredCommandError(fault.KindRejected, "catalog_digest_mismatch", false, "source inspect", "Regenerate and review source inspection JSON."),
		declaredCommandError(fault.KindNotFound, "specification_file_not_found", false, "help spec validate", "Select an existing schema-4 specification file."),
		declaredCommandError(fault.KindPermission, "specification_file_permission_denied", false, "help spec validate", "Correct specification file permissions."),
		declaredCommandError(fault.KindInvalidInput, "unsafe_specification_file", false, "help spec validate", "Use a stable regular specification file."),
		declaredCommandError(fault.KindInvalidInput, "specification_file_too_large", false, "help spec validate", "Reduce the specification below 256 KiB."),
		declaredCommandError(fault.KindUnavailable, "specification_file_read_failed", true, "help spec validate", "Retry after the specification file is readable."),
		declaredCommandError(fault.KindInvalidInput, "invalid_specification_yaml", false, "help spec validate", "Correct the strict schema-4 YAML syntax."),
		declaredCommandError(fault.KindInvalidInput, "legacy_tailoring_schema", false, "help spec init", "Create a schema-4 surface and wrapper specification without automatic conversion."),
		declaredCommandError(fault.KindInvalidInput, "invalid_specification", false, "help spec validate", "Correct the catalog-bound surface and wrapper semantics."),
	}
	if includeBundle {
		errors = append(errors, declaredCommandError(fault.KindContract, "invalid_bundle", false, "help bundle build", "Repair canonical bundle compilation."))
	}
	return append(errors,
		declaredCommandError(fault.KindContract, "output_contract_exceeded", false, "help "+command, "Reduce the bounded canonical output."),
		declaredCommandError(fault.KindContract, "output_encoding_failed", false, "help "+command, "Repair canonical JSON projection."),
		declaredCommandError(fault.KindInternal, "internal_error", false, "help "+command, "Inspect artifact loading and compilation."),
		declaredCommandError(fault.KindInternal, "output_write_failed", true, command, "Retry with a writable output stream."),
		declaredCommandError(fault.KindCanceled, "operation_canceled", true, command, "Retry when the caller is ready."),
	)
}

func processorObservationErrors(command string) []CommandError {
	return []CommandError{
		declaredCommandError(fault.KindInvalidInput, "invalid_processor_observation_selection", false, "help "+command, "Select at most one non-empty processor inspection JSON path."),
		declaredCommandError(fault.KindNotFound, "processor_observation_file_not_found", false, "processor inspect", "Generate and select a current processor inspection JSON file."),
		declaredCommandError(fault.KindPermission, "processor_observation_file_permission_denied", false, "processor inspect", "Correct processor observation file permissions."),
		declaredCommandError(fault.KindInvalidInput, "unsafe_processor_observation_file", false, "processor inspect", "Use a stable regular processor inspection file."),
		declaredCommandError(fault.KindInvalidInput, "processor_observation_file_too_large", false, "processor inspect", "Regenerate a processor inspection file within the 64 KiB bound."),
		declaredCommandError(fault.KindUnavailable, "processor_observation_file_read_failed", true, "processor inspect", "Retry after the processor observation file is readable."),
		declaredCommandError(fault.KindInvalidInput, "invalid_processor_observation_file", false, "processor inspect", "Regenerate strict processor inspection JSON from the explicit executable."),
	}
}

func bundleFileErrors(command string) []CommandError {
	return []CommandError{
		declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help "+command, "Pass one exact bundle build JSON path."),
		declaredCommandError(fault.KindNotFound, "bundle_file_not_found", false, "bundle build", "Build and select a canonical bundle document."),
		declaredCommandError(fault.KindPermission, "bundle_file_permission_denied", false, "bundle status", "Correct bundle file permissions."),
		declaredCommandError(fault.KindInvalidInput, "unsafe_bundle_file", false, "bundle build", "Use a stable regular bundle file."),
		declaredCommandError(fault.KindInvalidInput, "bundle_file_too_large", false, "bundle build", "Build a bundle within the 2 MiB limit."),
		declaredCommandError(fault.KindUnavailable, "bundle_file_read_failed", true, "bundle status", "Retry after the bundle file is readable."),
		declaredCommandError(fault.KindInvalidInput, "invalid_bundle_file", false, "bundle build", "Rebuild and review strict canonical bundle JSON."),
		declaredCommandError(fault.KindInvalidInput, "legacy_tailoring_schema", false, "help bundle build", "Rebuild with a schema-4 specification and bundle schema 3."),
		declaredCommandError(fault.KindRejected, "bundle_digest_mismatch", false, "bundle build", "Rebuild and review the changed bundle content."),
	}
}

func bundlePreviewErrors() []CommandError {
	errors := bundleFileErrors("bundle preview")
	errors[0] = declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help bundle preview", "Pass one bundle path, the exact source executable, and at least one source argv element after --.")
	return append(errors,
		declaredCommandError(fault.KindRejected, "invalid_bundle_trust_store", false, "bundle status", "Repair or reconcile invalid user-local adoption state."),
		declaredCommandError(fault.KindRejected, "bundle_not_adopted", false, "bundle trust", "Review and adopt the exact bundle digest before previewing a plan."),
		declaredCommandError(fault.KindRejected, "bundle_source_drift", false, "bundle status", "Rebuild and adopt current source evidence before previewing a plan."),
		declaredCommandError(fault.KindNotFound, "source_executable_not_found", false, "bundle status", "Reconcile the missing bundle-bound source executable."),
		declaredCommandError(fault.KindUnavailable, "source_identity_unavailable", true, "bundle status", "Retry after the bundle-bound source identity can be read."),
		declaredCommandError(fault.KindInvalidInput, "unsafe_source_executable", false, "bundle status", "Select and inspect a supported regular source executable."),
		declaredCommandError(fault.KindRejected, "source_identity_changed", false, "bundle status", "Rebuild from stable current source identity evidence."),
		declaredCommandError(fault.KindContract, "invalid_source_identity", false, "bundle status", "Repair invalid source identity evidence."),
		declaredCommandError(fault.KindInvalidInput, "source_executable_mismatch", false, "help bundle preview", "Use the exact requested executable or resolved path recorded in the bundle."),
		declaredCommandError(fault.KindInvalidInput, "invalid_invocation", false, "help bundle preview", "Use a cataloged command path and deterministic observed long-option grammar."),
		declaredCommandError(fault.KindNotFound, "command_not_in_surface", false, "help bundle preview", "Select a command present in the compiled tailored surface."),
		declaredCommandError(fault.KindNotFound, "option_not_in_surface", false, "help bundle preview", "Use only options present in the matched command's tailored option surface."),
		declaredCommandError(fault.KindContract, "invalid_wrapper_plan", false, "help bundle preview", "Repair the bundle or plan constructor so it produces one complete typed plan."),
		declaredCommandError(fault.KindContract, "output_contract_exceeded", false, "help bundle preview", "Reduce the bounded invocation and plan output."),
		declaredCommandError(fault.KindContract, "output_encoding_failed", false, "help bundle preview", "Repair deterministic schema-2 preview JSON."),
		declaredCommandError(fault.KindInternal, "internal_error", false, "bundle status", "Inspect bundle, adoption, source identity, and plan wiring."),
		declaredCommandError(fault.KindInternal, "output_write_failed", true, "bundle preview", "Retry with a writable output stream."),
		declaredCommandError(fault.KindCanceled, "operation_canceled", true, "bundle preview", "Retry when the caller is ready."),
	)
}

func bundleExecuteErrors() []CommandError {
	errors := bundleFileErrors("bundle execute")
	errors[0] = declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help bundle execute", "Pass one bundle path, the exact source executable, and at least one source argv element after --.")
	return append(errors,
		declaredCommandError(fault.KindRejected, "invalid_bundle_trust_store", false, "bundle status", "Repair or reconcile invalid user-local adoption state."),
		declaredCommandError(fault.KindRejected, "bundle_not_adopted", false, "bundle trust", "Review and adopt the exact bundle digest before execution."),
		declaredCommandError(fault.KindRejected, "bundle_source_drift", false, "bundle status", "Rebuild and adopt current source evidence before execution."),
		declaredCommandError(fault.KindNotFound, "source_executable_not_found", false, "bundle status", "Reconcile the missing bundle-bound source executable."),
		declaredCommandError(fault.KindUnavailable, "source_identity_unavailable", true, "bundle status", "Retry after the bundle-bound source identity can be read."),
		declaredCommandError(fault.KindInvalidInput, "unsafe_source_executable", false, "bundle status", "Select and inspect a supported regular source executable."),
		declaredCommandError(fault.KindRejected, "source_identity_changed", false, "bundle status", "Rebuild from stable current source identity evidence; do not replay a started operation."),
		declaredCommandError(fault.KindContract, "invalid_source_identity", false, "bundle status", "Repair invalid source identity evidence."),
		declaredCommandError(fault.KindInvalidInput, "source_executable_mismatch", false, "help bundle execute", "Use the exact requested executable or resolved path recorded in the bundle."),
		declaredCommandError(fault.KindInvalidInput, "invalid_invocation", false, "help bundle execute", "Use a cataloged command path and deterministic observed long-option grammar."),
		declaredCommandError(fault.KindNotFound, "command_not_in_surface", false, "help bundle execute", "Select a command present in the compiled tailored surface."),
		declaredCommandError(fault.KindNotFound, "option_not_in_surface", false, "help bundle execute", "Use only options present in the matched command's tailored option surface."),
		declaredCommandError(fault.KindContract, "invalid_wrapper_plan", false, "bundle preview", "Inspect the fresh plan and repair incomplete wrapper construction."),
		declaredCommandError(fault.KindUnsupported, "wrapper_runtime_not_supported", false, "help bundle execute", "Use a transform wrapper and source adapter contract with accepted JSON selector behavior."),
		declaredCommandError(fault.KindContract, "invalid_source_process_request", false, "bundle preview", "Inspect the exact plan-derived source request before execution."),
		declaredCommandError(fault.KindUnavailable, "source_process_start_failed", true, "bundle execute", "Retry the same invocation only when the result proves no source process started."),
		declaredCommandError(fault.KindContract, "source_stdout_too_large", false, "help bundle execute", "Reduce source output within the declared bound; the source was not retried."),
		declaredCommandError(fault.KindContract, "source_stderr_too_large", false, "help bundle execute", "Reduce source stderr within the declared bound; the source was not retried."),
		declaredCommandError(fault.KindCanceled, "source_execution_canceled", false, "bundle status", "Reconcile source-owned effects before considering another invocation."),
		declaredCommandError(fault.KindUnavailable, "source_command_timeout", false, "bundle status", "Reconcile source-owned effects after the timed-out attempt."),
		declaredCommandError(fault.KindRejected, "source_command_failed", false, "help bundle execute", "Inspect the source command independently; Atsura does not expose raw failure output or retry it."),
		declaredCommandError(fault.KindUnavailable, "source_process_wait_failed", false, "bundle status", "Reconcile source-owned effects after the unclassified wait outcome."),
		declaredCommandError(fault.KindContract, "source_stderr_not_supported", false, "help bundle execute", "Use a successful source invocation with empty stderr for this initial transform runtime."),
		declaredCommandError(fault.KindCanceled, "source_output_processing_canceled", false, "bundle status", "The source already ran; reconcile before considering another invocation."),
		declaredCommandError(fault.KindContract, "source_json_invalid", false, "bundle preview", "Repair the source output selector or adapter contract; raw output is not a fallback."),
		declaredCommandError(fault.KindContract, "output_transform_failed", false, "bundle preview", "Repair selected fields and typed transform expectations; raw output is not a fallback."),
		declaredCommandError(fault.KindContract, "unclassified_source_execution_outcome", false, "bundle status", "Reconcile source-owned effects before considering another invocation."),
		declaredCommandError(fault.KindContract, "output_contract_exceeded", false, "bundle preview", "Reduce the bounded transformed result; the source was not retried."),
		declaredCommandError(fault.KindContract, "output_encoding_failed", false, "bundle preview", "Repair deterministic schema-2 execution JSON; the source was not retried."),
		declaredCommandError(fault.KindInternal, "internal_error", false, "bundle status", "Inspect bundle execution wiring without replaying the source."),
		declaredCommandError(fault.KindInternal, "execute_output_write_failed", false, "bundle status", "The source completed; reconcile before considering another invocation."),
		declaredCommandError(fault.KindCanceled, "operation_canceled", true, "bundle execute", "Retry only because cancellation occurred before a source attempt."),
	)
}

func wrapperRenderErrors() []CommandError {
	errors := bundleFileErrors("wrapper render")
	errors[0] = declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help wrapper render", "Pass one exact absolute bundle path and choose text or JSON output.")
	return append(errors,
		declaredCommandError(fault.KindInvalidInput, "invalid_wrapper_binding", false, "help wrapper render", "Use an absolute clean bundle path whose requested executable is one portable POSIX command name."),
		declaredCommandError(fault.KindUnsupported, "wrapper_platform_not_supported", false, "help wrapper render", "Render the POSIX function on a supported Linux or macOS runtime."),
		declaredCommandError(fault.KindRejected, "invalid_bundle_trust_store", false, "bundle status", "Repair or reconcile invalid user-local adoption state."),
		declaredCommandError(fault.KindRejected, "bundle_not_adopted", false, "bundle trust", "Review and adopt the exact bundle digest before rendering a wrapper."),
		declaredCommandError(fault.KindRejected, "bundle_source_drift", false, "bundle status", "Rebuild and adopt current source evidence before rendering a wrapper."),
		declaredCommandError(fault.KindNotFound, "source_executable_not_found", false, "bundle status", "Reconcile the missing bundle-bound source executable."),
		declaredCommandError(fault.KindUnavailable, "source_identity_unavailable", true, "bundle status", "Retry after the bundle-bound source identity can be read."),
		declaredCommandError(fault.KindInvalidInput, "unsafe_source_executable", false, "bundle status", "Select and inspect a supported regular source executable."),
		declaredCommandError(fault.KindRejected, "source_identity_changed", false, "bundle status", "Rebuild from stable current source identity evidence."),
		declaredCommandError(fault.KindContract, "invalid_source_identity", false, "bundle status", "Repair invalid source identity evidence."),
		declaredCommandError(fault.KindInvalidInput, "invalid_processor_executable", false, "bundle status", "Reconcile the exact bundle-bound processor executable."),
		declaredCommandError(fault.KindInvalidInput, "unsafe_processor_executable", false, "bundle status", "Replace or re-inspect the bundle-bound processor as a supported regular executable."),
		declaredCommandError(fault.KindUnavailable, "processor_identity_unavailable", true, "bundle status", "Retry only after the exact bundle-bound processor identity can be read."),
		declaredCommandError(fault.KindRejected, "processor_identity_changed", false, "bundle status", "Rebuild and adopt current processor identity evidence."),
		declaredCommandError(fault.KindContract, "invalid_processor_identity", false, "bundle status", "Repair invalid processor identity evidence."),
		declaredCommandError(fault.KindRejected, "bundle_processor_drift", false, "bundle status", "Rebuild and adopt current processor identity evidence before rendering a wrapper."),
		declaredCommandError(fault.KindUnsupported, "wrapper_runtime_not_supported", false, "help wrapper render", "Review the exact adopted-bundle, runtime, surface, and POSIX wrapper requirements."),
		declaredCommandError(fault.KindUnavailable, "wrapper_runtime_unavailable", false, "help wrapper render", "Retry only after the current Atsura executable identity is readable and stable."),
		declaredCommandError(fault.KindContract, "wrapper_render_failed", false, "help wrapper render", "Repair the fixed POSIX renderer or its validated product binding."),
		declaredCommandError(fault.KindContract, "output_contract_exceeded", false, "help wrapper render", "Reduce the bounded generated wrapper output."),
		declaredCommandError(fault.KindContract, "output_encoding_failed", false, "help wrapper render", "Repair deterministic wrapper review JSON."),
		declaredCommandError(fault.KindInternal, "internal_error", false, "bundle status", "Inspect bundle, adoption, source, runtime, and renderer wiring."),
		declaredCommandError(fault.KindInternal, "output_write_failed", true, "wrapper render", "Retry with a writable output stream."),
		declaredCommandError(fault.KindCanceled, "operation_canceled", true, "wrapper render", "Retry when the caller is ready."),
	)
}

func wrapperRunErrors() []CommandError {
	errors := bundleFileErrors("wrapper run")
	errors[0] = declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help wrapper run", "Use only the complete render-produced binding flags and forward argv after --.")
	resultRecovery := wrapperResultRecovery()
	return append(errors,
		declaredCommandError(fault.KindInvalidInput, "invalid_wrapper_binding", false, "wrapper render", "Render a complete binding from the exact current bundle and Atsura runtime."),
		declaredCommandError(fault.KindUnavailable, "wrapper_runtime_unavailable", false, "wrapper render", "Render again only after the current Atsura executable identity is readable and stable."),
		declaredCommandError(fault.KindRejected, "wrapper_runtime_drift", false, "wrapper render", "Render a new binding from the exact current Atsura runtime."),
		declaredCommandError(fault.KindRejected, "bundle_binding_mismatch", false, "wrapper render", "Render a new wrapper from the exact current bundle bytes."),
		declaredCommandError(fault.KindRejected, "invalid_bundle_trust_store", false, "bundle status", "Repair or reconcile invalid user-local adoption state."),
		declaredCommandError(fault.KindRejected, "bundle_not_adopted", false, "bundle trust", "Review and adopt the exact bundle digest before execution."),
		declaredCommandError(fault.KindRejected, "bundle_source_drift", false, "bundle status", "Rebuild and adopt current source evidence before execution."),
		declaredCommandError(fault.KindNotFound, "source_executable_not_found", false, "bundle status", "Reconcile the missing bundle-bound source executable."),
		declaredCommandError(fault.KindUnavailable, "source_identity_unavailable", true, "bundle status", "Retry after the bundle-bound source identity can be read."),
		declaredCommandError(fault.KindInvalidInput, "unsafe_source_executable", false, "bundle status", "Select and inspect a supported regular source executable."),
		declaredCommandError(fault.KindRejected, "source_identity_changed", false, "bundle status", "Rebuild from stable current source identity evidence; do not replay a started operation."),
		declaredCommandError(fault.KindContract, "invalid_source_identity", false, "bundle status", "Repair invalid source identity evidence."),
		declaredCommandError(fault.KindInvalidInput, "source_executable_mismatch", false, "wrapper render", "Render a new wrapper whose command spelling comes from the exact bundle."),
		declaredCommandError(fault.KindInvalidInput, "invalid_invocation", false, "help wrapper run", "Use a cataloged command path and deterministic observed long-option grammar."),
		declaredCommandError(fault.KindNotFound, "command_not_in_surface", false, "help wrapper run", "Use a command present in the compiled tailored surface."),
		declaredCommandError(fault.KindNotFound, "option_not_in_surface", false, "help wrapper run", "Use only options present in the matched command's tailored option surface."),
		declaredCommandError(fault.KindContract, "invalid_wrapper_plan", false, "bundle preview", "Inspect the fresh plan and repair incomplete wrapper construction."),
		declaredCommandError(fault.KindUnsupported, "wrapper_runtime_not_supported", false, "help wrapper run", "Review the supported generated-wrapper runtime contract."),
		declaredCommandError(fault.KindContract, "invalid_source_process_request", false, "bundle preview", "Inspect the exact plan-derived source request before execution."),
		declaredCommandError(fault.KindUnavailable, "source_process_start_failed", true, "wrapper run", "Retry the exact generated invocation only when the result proves no source process started."),
		declaredCommandError(fault.KindContract, "source_stdout_too_large", false, "help wrapper run", "Reduce source output within the declared bound; the source was not retried."),
		declaredCommandError(fault.KindContract, "source_stderr_too_large", false, "help wrapper run", "Reduce source stderr within the declared bound; the source was not retried."),
		declaredCommandError(fault.KindCanceled, "source_execution_canceled", false, "bundle status", "Reconcile source-owned effects before considering another invocation."),
		declaredCommandError(fault.KindUnavailable, "source_command_timeout", false, "bundle status", "Reconcile source-owned effects after the timed-out attempt."),
		declaredCommandError(fault.KindRejected, "source_command_failed", false, "help wrapper run", "Inspect the source command independently; Atsura does not expose raw failure output or retry it."),
		declaredCommandError(fault.KindUnavailable, "source_process_wait_failed", false, "bundle status", "Reconcile source-owned effects after the unclassified wait outcome."),
		declaredCommandError(fault.KindContract, "source_stderr_not_supported", false, "help wrapper run", "Use a successful source invocation with empty stderr for this initial transform runtime."),
		declaredCommandError(fault.KindCanceled, "source_output_processing_canceled", false, "bundle status", "The source already ran; reconcile before considering another invocation."),
		declaredCommandError(fault.KindContract, "source_json_invalid", false, "bundle preview", "Repair the source output selector or adapter contract; raw output is not a fallback."),
		declaredCommandError(fault.KindContract, "output_transform_failed", false, "bundle preview", "Repair selected fields and typed transform expectations; raw output is not a fallback."),
		declaredCommandError(fault.KindContract, "unclassified_source_execution_outcome", false, "bundle status", "Reconcile source-owned effects before considering another invocation."),
		declaredCommandError(fault.KindUnavailable, "processor_identity_unavailable", true, "wrapper run", "Retry only because the processor identity could not be read before any source attempt."),
		declaredCommandError(fault.KindUnavailable, "processor_identity_unavailable_after_source", false, "bundle status", "Reconcile the exact bundle-bound processor identity after the source completed; replay is not known to be safe."),
		declaredCommandError(fault.KindRejected, "processor_identity_changed", false, "bundle status", "Rebuild and adopt current processor evidence; do not replay a source attempt that may already have completed."),
		declaredCommandError(fault.KindInvalidInput, "invalid_processor_executable", false, "bundle status", "Reconcile the bundle-bound processor executable after the completed source attempt."),
		declaredCommandError(fault.KindInvalidInput, "unsafe_processor_executable", false, "bundle status", "Replace or re-inspect the unsafe bundle-bound processor without replaying the completed source attempt."),
		declaredCommandError(fault.KindContract, "invalid_processor_identity", false, "bundle status", "Repair invalid processor identity evidence without replaying the completed source attempt."),
		declaredCommandError(fault.KindContract, "invalid_processor_process_request", false, "bundle preview", "Repair the exact plan-derived processor request; the source was not retried."),
		declaredCommandError(fault.KindUnavailable, "processor_environment_setup_failed_after_source", false, "bundle status", "Reconcile source-owned effects and the isolated processor environment before another invocation."),
		declaredCommandError(fault.KindUnavailable, "processor_process_start_failed_after_source", false, "bundle status", "Reconcile source-owned effects before another invocation; no fallback bytes were published."),
		declaredCommandError(fault.KindContract, "processor_stdout_too_large", false, "bundle status", "Reconcile source-owned effects and reduce processor output within its declared bound."),
		declaredCommandError(fault.KindContract, "processor_stderr_too_large", false, "bundle status", "Reconcile source-owned effects and reduce processor stderr within its declared bound."),
		declaredCommandError(fault.KindCanceled, "processor_execution_canceled", false, "help wrapper run", "Review the exact optimizer runtime outcome; replay is not known to be safe."),
		declaredCommandError(fault.KindUnavailable, "processor_timeout", false, "bundle status", "Reconcile source-owned effects after the processor timed out."),
		declaredCommandError(fault.KindRejected, "processor_command_failed", false, "bundle status", "Reconcile source-owned effects and inspect the processor independently; no fallback bytes were published."),
		declaredCommandError(fault.KindUnavailable, "processor_process_wait_failed", false, "bundle status", "Reconcile source-owned effects after the processor result could not be collected."),
		declaredCommandError(fault.KindUnavailable, "processor_cleanup_failed", false, "bundle status", "Reconcile source-owned effects and the isolated processor environment before another invocation."),
		declaredCommandError(fault.KindContract, "processor_output_not_admitted", false, "bundle status", "Reconcile source-owned effects and the rejected optimizer result; no fallback bytes were published."),
		declaredCommandError(fault.KindContract, "unclassified_processor_execution_outcome", false, "bundle status", "Reconcile source-owned effects after the processor result could not be classified."),
		declaredCommandError(fault.KindContract, "output_contract_exceeded", false, resultRecovery.Command, resultRecovery.Reason),
		declaredCommandError(fault.KindContract, "output_encoding_failed", false, "bundle preview", "Repair deterministic compact wrapper JSON; the source was not retried."),
		declaredCommandError(fault.KindInternal, "internal_error", false, "bundle status", "Inspect wrapper execution wiring without replaying the source."),
		declaredCommandError(fault.KindInternal, "execute_output_write_failed", false, "bundle status", "The source completed; reconcile before considering another invocation."),
		declaredCommandError(fault.KindCanceled, "operation_canceled", true, "wrapper run", "Retry only because cancellation occurred before a source attempt."),
	)
}

func wrapperContractOutputSchema() *OutputSchema {
	return &OutputSchema{ID: "wrapper-contract", Version: 1, Fields: []OutputSchemaField{
		{Path: "/shell", Type: OutputFieldTypeString, Required: true},
		{Path: "/version", Type: OutputFieldTypeInteger, Required: true},
	}}
}

func wrapperBundleBindingOutputSchema() *OutputSchema {
	return &OutputSchema{ID: "wrapper-bundle-binding", Version: 1, Fields: []OutputSchemaField{
		{Path: "/digest", Type: OutputFieldTypeString, Required: true},
		{Path: "/locator", Type: OutputFieldTypeString, Required: true},
	}}
}

func wrapperRuntimeBindingOutputSchema() *OutputSchema {
	return &OutputSchema{ID: "wrapper-runtime-binding", Version: 1, Fields: []OutputSchemaField{
		{Path: "/resolved_path", Type: OutputFieldTypeString, Required: true},
		{Path: "/sha256", Type: OutputFieldTypeString, Required: true},
		{Path: "/size", Type: OutputFieldTypeInteger, Required: true},
	}}
}

func tailoredJSONOutputSchema() *OutputSchema {
	return &OutputSchema{ID: "tailored-json-result", Version: 2, Fields: []OutputSchemaField{
		{Path: "/fields", Type: OutputFieldTypeArray, ElementType: OutputFieldTypeString, Required: true},
		{Path: "/records", Type: OutputFieldTypeArray, ElementType: OutputFieldTypeObject, Required: true},
		{Path: "/render", Type: OutputFieldTypeString, Required: true},
		{Path: "/shape", Type: OutputFieldTypeString, Required: true},
	}}
}

func sourceExecutionOutputSchema() *OutputSchema {
	return &OutputSchema{ID: "source-execution-result", Version: 1, Fields: []OutputSchemaField{
		{Path: "/exit_code", Type: OutputFieldTypeInteger, Required: true},
	}}
}

func sourceCatalogOutputSchema() *OutputSchema {
	field := func(path string, fieldType OutputFieldType) OutputSchemaField {
		return OutputSchemaField{Path: path, Type: fieldType, Required: true}
	}
	array := func(path string, elementType OutputFieldType) OutputSchemaField {
		return OutputSchemaField{Path: path, Type: OutputFieldTypeArray, ElementType: elementType, Required: true}
	}
	fields := []OutputSchemaField{
		field("/adapter", OutputFieldTypeObject),
		field("/adapter/contract_version", OutputFieldTypeInteger),
		field("/adapter/kind", OutputFieldTypeString),
		array("/commands", OutputFieldTypeObject),
		array("/commands/*/options", OutputFieldTypeObject),
		field("/commands/*/options/*/name", OutputFieldTypeString),
		field("/commands/*/options/*/takes_value", OutputFieldTypeBoolean),
		array("/commands/*/path", OutputFieldTypeString),
		field("/commands/*/provenance", OutputFieldTypeString),
		array("/commands/*/structured_output", OutputFieldTypeObject),
		array("/commands/*/structured_output/*/fields", OutputFieldTypeString),
		field("/commands/*/structured_output/*/format", OutputFieldTypeString),
		field("/commands/*/structured_output/*/selector_flag", OutputFieldTypeString),
		field("/commands/*/summary", OutputFieldTypeString),
		field("/probe", OutputFieldTypeObject),
		field("/probe/attempts", OutputFieldTypeInteger),
		array("/probe/ids", OutputFieldTypeString),
		field("/schema_version", OutputFieldTypeInteger),
		field("/source", OutputFieldTypeObject),
		field("/source/requested_executable", OutputFieldTypeString),
		field("/source/resolved_path", OutputFieldTypeString),
		field("/source/sha256", OutputFieldTypeString),
		field("/source/size", OutputFieldTypeInteger),
		field("/source/version", OutputFieldTypeString),
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Path < fields[j].Path })
	return &OutputSchema{ID: "source-command-catalog", Version: sourcecatalog.SchemaVersion, Fields: fields}
}

func processorObservationOutputSchema() *OutputSchema {
	field := func(path string, fieldType OutputFieldType) OutputSchemaField {
		return OutputSchemaField{Path: path, Type: fieldType, Required: true}
	}
	array := func(path string, elementType OutputFieldType) OutputSchemaField {
		return OutputSchemaField{Path: path, Type: OutputFieldTypeArray, ElementType: elementType, Required: true}
	}
	fields := []OutputSchemaField{
		field("/adapter", OutputFieldTypeObject),
		field("/adapter/contract_version", OutputFieldTypeInteger),
		field("/adapter/kind", OutputFieldTypeString),
		field("/identity", OutputFieldTypeObject),
		field("/identity/resolved_path", OutputFieldTypeString),
		field("/identity/sha256", OutputFieldTypeString),
		field("/identity/size", OutputFieldTypeInteger),
		field("/platform", OutputFieldTypeObject),
		field("/platform/arch", OutputFieldTypeString),
		field("/platform/os", OutputFieldTypeString),
		field("/probe", OutputFieldTypeObject),
		array("/probe/argv", OutputFieldTypeString),
		field("/probe/attempts", OutputFieldTypeInteger),
		field("/probe/environment_contract", OutputFieldTypeString),
		field("/schema_version", OutputFieldTypeInteger),
		field("/version", OutputFieldTypeString),
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Path < fields[j].Path })
	return &OutputSchema{ID: "processor-observation", Version: processorprocess.ObservationSchemaVersion, Fields: fields}
}

func bundleProcessorStatusOutputSchema() *OutputSchema {
	return &OutputSchema{ID: "bundle-processor-status", Version: 1, Fields: []OutputSchemaField{
		{Path: "/adapter_kind", Type: OutputFieldTypeString, Required: true},
		{Path: "/contract", Type: OutputFieldTypeString, Required: true},
		{Path: "/resolved_path", Type: OutputFieldTypeString, Required: true},
		{Path: "/sha256", Type: OutputFieldTypeString, Required: true},
		{Path: "/size", Type: OutputFieldTypeInteger, Required: true},
		{Path: "/state", Type: OutputFieldTypeString, Required: true},
		{Path: "/version", Type: OutputFieldTypeString, Required: true},
	}}
}

func tailoringSpecificationOutputSchema() *OutputSchema {
	field := func(path string, fieldType OutputFieldType) OutputSchemaField {
		return OutputSchemaField{Path: path, Type: fieldType, Required: true}
	}
	array := func(path string, elementType OutputFieldType) OutputSchemaField {
		return OutputSchemaField{Path: path, Type: OutputFieldTypeArray, ElementType: elementType, Required: true}
	}
	fields := []OutputSchemaField{
		field("/catalog_digest", OutputFieldTypeString),
		array("/commands", OutputFieldTypeObject),
		array("/commands/*/command", OutputFieldTypeString),
		field("/commands/*/options", OutputFieldTypeObject),
		field("/commands/*/options/default", OutputFieldTypeString),
		array("/commands/*/options/exclude", OutputFieldTypeString),
		array("/commands/*/options/include", OutputFieldTypeString),
		field("/commands/*/presence", OutputFieldTypeString),
		field("/commands/*/reason", OutputFieldTypeString),
		field("/commands/*/wrapper", OutputFieldTypeObject),
		array("/commands/*/wrapper/after", OutputFieldTypeObject),
		array("/commands/*/wrapper/before", OutputFieldTypeObject),
		field("/commands/*/wrapper/invoke", OutputFieldTypeObject),
		array("/commands/*/wrapper/invoke/append_args", OutputFieldTypeString),
		field("/commands/*/wrapper/kind", OutputFieldTypeString),
		field("/commands/*/wrapper/output", OutputFieldTypeObject),
		field("/commands/*/wrapper/output/kind", OutputFieldTypeString),
		field("/commands/*/wrapper/output/optimizer", OutputFieldTypeObject),
		field("/commands/*/wrapper/output/optimizer/allow_original_output", OutputFieldTypeBoolean),
		field("/commands/*/wrapper/output/optimizer/contract", OutputFieldTypeString),
		field("/commands/*/wrapper/output/optimizer/input", OutputFieldTypeString),
		field("/commands/*/wrapper/output/projection", OutputFieldTypeObject),
		field("/commands/*/wrapper/output/projection/input", OutputFieldTypeString),
		array("/commands/*/wrapper/output/projection/rename", OutputFieldTypeObject),
		field("/commands/*/wrapper/output/projection/rename/*/from", OutputFieldTypeString),
		field("/commands/*/wrapper/output/projection/rename/*/to", OutputFieldTypeString),
		field("/commands/*/wrapper/output/projection/render", OutputFieldTypeString),
		array("/commands/*/wrapper/output/projection/select", OutputFieldTypeString),
		field("/schema_version", OutputFieldTypeInteger),
		field("/surface", OutputFieldTypeObject),
		field("/surface/default", OutputFieldTypeString),
	}
	for index := range fields {
		switch fields[index].Path {
		case "/commands/*/options", "/commands/*/wrapper", "/commands/*/wrapper/output", "/commands/*/wrapper/output/optimizer", "/commands/*/wrapper/output/projection":
			fields[index].Required = false
		}
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Path < fields[j].Path })
	return &OutputSchema{ID: "tailoring-specification", Version: tailoringbundle.SpecificationSchemaVersion, Fields: fields}
}

func wrapperPlanOutputSchema() *OutputSchema {
	field := func(path string, fieldType OutputFieldType) OutputSchemaField {
		return OutputSchemaField{Path: path, Type: fieldType, Required: true}
	}
	array := func(path string, elementType OutputFieldType) OutputSchemaField {
		return OutputSchemaField{Path: path, Type: OutputFieldTypeArray, ElementType: elementType, Required: true}
	}
	fields := []OutputSchemaField{
		field("/bundle_digest", OutputFieldTypeString),
		field("/catalog_digest", OutputFieldTypeString),
		array("/matched_command", OutputFieldTypeString),
		field("/mode", OutputFieldTypeString),
		field("/options", OutputFieldTypeObject),
		field("/options/default", OutputFieldTypeString),
		array("/options/exclude", OutputFieldTypeString),
		array("/options/include", OutputFieldTypeString),
		array("/original_argv", OutputFieldTypeString),
		field("/processor", OutputFieldTypeObject),
		field("/processor/allow_original_output", OutputFieldTypeBoolean),
		field("/processor/contract", OutputFieldTypeString),
		field("/processor/execution", OutputFieldTypeObject),
		array("/processor/execution/args", OutputFieldTypeString),
		field("/processor/execution/environment_contract", OutputFieldTypeString),
		field("/processor/execution/max_attempts", OutputFieldTypeInteger),
		field("/processor/execution/stderr_limit_bytes", OutputFieldTypeInteger),
		field("/processor/execution/stdin_mode", OutputFieldTypeString),
		field("/processor/execution/stdout_limit_bytes", OutputFieldTypeInteger),
		field("/processor/execution/timeout_millis", OutputFieldTypeInteger),
		field("/processor/execution/working_directory_mode", OutputFieldTypeString),
		field("/processor/input_format", OutputFieldTypeString),
		field("/processor/observation", OutputFieldTypeObject),
		field("/processor/observation/adapter", OutputFieldTypeObject),
		field("/processor/observation/adapter/contract_version", OutputFieldTypeInteger),
		field("/processor/observation/adapter/kind", OutputFieldTypeString),
		field("/processor/observation/identity", OutputFieldTypeObject),
		field("/processor/observation/identity/resolved_path", OutputFieldTypeString),
		field("/processor/observation/identity/sha256", OutputFieldTypeString),
		field("/processor/observation/identity/size", OutputFieldTypeInteger),
		field("/processor/observation/platform", OutputFieldTypeObject),
		field("/processor/observation/platform/arch", OutputFieldTypeString),
		field("/processor/observation/platform/os", OutputFieldTypeString),
		field("/processor/observation/probe", OutputFieldTypeObject),
		array("/processor/observation/probe/argv", OutputFieldTypeString),
		field("/processor/observation/probe/attempts", OutputFieldTypeInteger),
		field("/processor/observation/probe/environment_contract", OutputFieldTypeString),
		field("/processor/observation/schema_version", OutputFieldTypeInteger),
		field("/processor/observation/version", OutputFieldTypeString),
		field("/processor/output_format", OutputFieldTypeString),
		field("/reason", OutputFieldTypeString),
		field("/result_mode", OutputFieldTypeString),
		field("/schema_version", OutputFieldTypeInteger),
		field("/source", OutputFieldTypeObject),
		field("/source/adapter_contract_version", OutputFieldTypeInteger),
		field("/source/adapter_kind", OutputFieldTypeString),
		field("/source/requested_executable", OutputFieldTypeString),
		field("/source/resolved_path", OutputFieldTypeString),
		field("/source/sha256", OutputFieldTypeString),
		field("/source/size", OutputFieldTypeInteger),
		field("/source/version", OutputFieldTypeString),
		field("/specification_digest", OutputFieldTypeString),
		field("/specification_entry", OutputFieldTypeObject),
		array("/specification_entry/command", OutputFieldTypeString),
		field("/specification_entry/options", OutputFieldTypeObject),
		field("/specification_entry/options/default", OutputFieldTypeString),
		array("/specification_entry/options/exclude", OutputFieldTypeString),
		array("/specification_entry/options/include", OutputFieldTypeString),
		field("/specification_entry/presence", OutputFieldTypeString),
		field("/specification_entry/reason", OutputFieldTypeString),
		field("/specification_entry/wrapper", OutputFieldTypeObject),
		array("/specification_entry/wrapper/after", OutputFieldTypeObject),
		field("/specification_entry/wrapper/after/*/kind", OutputFieldTypeString),
		array("/specification_entry/wrapper/before", OutputFieldTypeObject),
		field("/specification_entry/wrapper/before/*/kind", OutputFieldTypeString),
		field("/specification_entry/wrapper/invoke", OutputFieldTypeObject),
		array("/specification_entry/wrapper/invoke/append_args", OutputFieldTypeString),
		field("/specification_entry/wrapper/kind", OutputFieldTypeString),
		field("/specification_entry/wrapper/output", OutputFieldTypeObject),
		field("/specification_entry/wrapper/output/kind", OutputFieldTypeString),
		field("/specification_entry/wrapper/output/optimizer", OutputFieldTypeObject),
		field("/specification_entry/wrapper/output/optimizer/allow_original_output", OutputFieldTypeBoolean),
		field("/specification_entry/wrapper/output/optimizer/contract", OutputFieldTypeString),
		field("/specification_entry/wrapper/output/optimizer/input", OutputFieldTypeString),
		field("/specification_entry/wrapper/output/projection", OutputFieldTypeObject),
		field("/specification_entry/wrapper/output/projection/input", OutputFieldTypeString),
		array("/specification_entry/wrapper/output/projection/rename", OutputFieldTypeObject),
		field("/specification_entry/wrapper/output/projection/rename/*/from", OutputFieldTypeString),
		field("/specification_entry/wrapper/output/projection/rename/*/to", OutputFieldTypeString),
		field("/specification_entry/wrapper/output/projection/render", OutputFieldTypeString),
		array("/specification_entry/wrapper/output/projection/select", OutputFieldTypeString),
		field("/stages", OutputFieldTypeObject),
		array("/stages/after", OutputFieldTypeObject),
		field("/stages/after/*/kind", OutputFieldTypeString),
		array("/stages/before", OutputFieldTypeObject),
		field("/stages/before/*/kind", OutputFieldTypeString),
		field("/stages/invoke", OutputFieldTypeObject),
		array("/stages/invoke/appended_args", OutputFieldTypeString),
		array("/stages/invoke/args", OutputFieldTypeString),
		field("/stages/invoke/environment_mode", OutputFieldTypeString),
		field("/stages/invoke/executable", OutputFieldTypeString),
		field("/stages/invoke/max_attempts", OutputFieldTypeInteger),
		field("/stages/invoke/stderr_limit_bytes", OutputFieldTypeInteger),
		field("/stages/invoke/stdin_mode", OutputFieldTypeString),
		field("/stages/invoke/stdout_limit_bytes", OutputFieldTypeInteger),
		field("/stages/invoke/timeout_millis", OutputFieldTypeInteger),
		field("/stages/invoke/working_directory_mode", OutputFieldTypeString),
		array("/stages/order", OutputFieldTypeString),
		field("/stages/output", OutputFieldTypeObject),
		field("/stages/output/kind", OutputFieldTypeString),
		field("/stages/output/optimizer", OutputFieldTypeObject),
		field("/stages/output/optimizer/allow_original_output", OutputFieldTypeBoolean),
		field("/stages/output/optimizer/contract", OutputFieldTypeString),
		field("/stages/output/optimizer/input", OutputFieldTypeString),
		field("/stages/output/projection", OutputFieldTypeObject),
		field("/stages/output/projection/input", OutputFieldTypeString),
		array("/stages/output/projection/rename", OutputFieldTypeObject),
		field("/stages/output/projection/rename/*/from", OutputFieldTypeString),
		field("/stages/output/projection/rename/*/to", OutputFieldTypeString),
		field("/stages/output/projection/render", OutputFieldTypeString),
		array("/stages/output/projection/select", OutputFieldTypeString),
		field("/surface_origin", OutputFieldTypeString),
		array("/transformed_argv", OutputFieldTypeString),
		field("/wrapper_kind", OutputFieldTypeString),
	}
	for index := range fields {
		switch fields[index].Path {
		case "/processor", "/specification_entry", "/stages/output":
			fields[index].Nullable = true
		case "/specification_entry/wrapper/output", "/specification_entry/wrapper/output/optimizer", "/specification_entry/wrapper/output/projection", "/stages/output/optimizer", "/stages/output/projection":
			fields[index].Required = false
		}
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Path < fields[j].Path })
	return &OutputSchema{ID: "wrapper-plan", Version: tailoringplan.SchemaVersion, Fields: fields}
}

func freshPlanResultModes() []PlanResultModeContract {
	success := func(
		disposition PlanResultDisposition,
		stdout, stderr PlanResultStream,
		exitStatus PlanResultExitStatus,
		framing PlanResultFraming,
		projection PlanResultProjection,
		crossStreamOrder PlanResultCrossStreamOrder,
		stdoutLimitBytes, stderrLimitBytes, sourceAttempts, processorAttempts int,
	) PlanResultSuccessContract {
		return PlanResultSuccessContract{
			Disposition: disposition, Stdout: stdout, Stderr: stderr, ExitStatus: exitStatus,
			Framing: framing, Projection: projection, Delivery: PlanResultDeliveryBufferedAfterCompletion,
			CrossStreamOrder: crossStreamOrder, StdoutLimitBytes: stdoutLimitBytes, StderrLimitBytes: stderrLimitBytes,
			SourceProcessAttempts: sourceAttempts, ProcessorProcessAttempts: processorAttempts,
		}
	}
	return []PlanResultModeContract{
		{
			Mode: tailoringplan.ResultModeTransformedJSON,
			SuccessVariants: []PlanResultSuccessContract{success(
				PlanResultDispositionNotApplicable,
				PlanResultStreamCompactJSON, PlanResultStreamEmpty, PlanResultExitStatusZero,
				PlanResultFramingOneValueLF, PlanResultProjectionVisibleJSON, PlanResultCrossStreamOrderNotApplicable,
				maxBundleOutputBytes, 0, 1, 0,
			)},
		},
		{
			Mode: tailoringplan.ResultModeSourceStreamPassthrough,
			SuccessVariants: []PlanResultSuccessContract{success(
				PlanResultDispositionNotApplicable,
				PlanResultStreamExactSourceBytes, PlanResultStreamExactSourceBytes, PlanResultExitStatusSourceConventional,
				PlanResultFramingNone, PlanResultProjectionNone, PlanResultCrossStreamOrderNotPreserved,
				sourceprocess.MaxStdoutBytes, sourceprocess.MaxStderrBytes, 1, 0,
			)},
		},
		{
			Mode: tailoringplan.ResultModeOriginalPreservingOptimizer,
			SuccessVariants: []PlanResultSuccessContract{
				success(
					PlanResultDispositionPreservedBeforeProcessor,
					PlanResultStreamExactSourceBytes, PlanResultStreamExactSourceBytes, PlanResultExitStatusSourceConventional,
					PlanResultFramingNone, PlanResultProjectionNone, PlanResultCrossStreamOrderNotPreserved,
					sourceprocess.MaxStdoutBytes, sourceprocess.MaxStderrBytes, 1, 0,
				),
				success(
					PlanResultDispositionPreservedAfterProcessor,
					PlanResultStreamExactAdmittedInputBytes, PlanResultStreamEmpty, PlanResultExitStatusZero,
					PlanResultFramingNone, PlanResultProjectionNone, PlanResultCrossStreamOrderNotApplicable,
					sourceprocess.MaxStdoutBytes, 0, 1, 1,
				),
				success(
					PlanResultDispositionOptimized,
					PlanResultStreamValidatedOptimizerSummary, PlanResultStreamEmpty, PlanResultExitStatusZero,
					PlanResultFramingNone, PlanResultProjectionNone, PlanResultCrossStreamOrderNotApplicable,
					processorprocess.MaxStdoutBytes, 0, 1, 1,
				),
			},
		},
	}
}

// freshWrapperPlanAuthoritativeOutput declares the finite dynamic result union
// governed by the freshly rebuilt plan. The static catalog publishes result
// modes and their byte/status contracts without duplicating plan fields.
func freshWrapperPlanAuthoritativeOutput() CommandOutput {
	planSchema := wrapperPlanOutputSchema()
	return CommandOutput{
		Authority:          OutputAuthorityFreshWrapperPlan,
		Formats:            []OutputFormat{OutputFormatPlanResult},
		DefaultFormat:      OutputFormatPlanResult,
		Fields:             []OutputField{},
		Delivery:           OutputDeliveryComplete,
		CollectionCoverage: CollectionCoverageNotApplicable,
		PlanSchema: &OutputSchemaReference{
			Command: "bundle preview",
			Field:   "plan",
			ID:      planSchema.ID,
			Version: planSchema.Version,
		},
		PlanResultModes: freshPlanResultModes(),
	}
}

func legacyMigrationCommand(path, summary, args, outcome, recovery string, inputs []CommandInput, handler commandHandler) CommandSpec {
	return CommandSpec{
		Path: path, Summary: summary, Args: args, Effect: operation.EffectRead, Role: RoleUtility,
		Agent: AgentContract{
			CapabilityID: "tailoring.schema.migrate", Outcome: outcome, Inputs: inputs,
			Output:        CommandOutput{Authority: OutputAuthorityCatalog, Formats: []OutputFormat{OutputFormatNone}, DefaultFormat: OutputFormatNone, Fields: []OutputField{}, Delivery: OutputDeliveryComplete, CollectionCoverage: CollectionCoverageNotApplicable},
			Prerequisites: []string{"This deprecated path exists only to return a deterministic migration diagnostic and never reads the retired file or starts a source process."},
			Errors: []CommandError{
				declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help "+path, "Use the deprecated command's exact historical syntax to obtain migration guidance."),
				declaredCommandError(fault.KindInvalidInput, "legacy_tailoring_schema", false, recovery, "Create or validate a schema-4 tailoring specification; automatic authorization-to-surface conversion is not available."),
				declaredCommandError(fault.KindCanceled, "operation_canceled", true, path, "Retry when the caller is ready."),
			},
		},
		handler: handler,
	}
}

// DefaultCatalog returns the public CLI contract.
func DefaultCatalog() Catalog {
	return NewCatalog(
		CommandSpec{
			Path:    "doctor",
			Summary: "Run local, read-only diagnostics",
			Args:    "[--format tsv|json]",
			Effect:  operation.EffectRead,
			Role:    RoleUtility,
			Agent: AgentContract{
				CapabilityID: "system.diagnostics",
				Outcome:      "Inspect the local runtime and receive a validated diagnostic report",
				Inputs: []CommandInput{
					{
						Name: "--format", Source: InputSourceFlag, Required: false,
						ValueKind: InputValueText, Cardinality: InputCardinalitySingle,
						Description: "Select the complete report representation.", AllowedValues: []string{"tsv", "json"},
						DefaultValue: stringPointer("tsv"),
					},
				},
				Output: CommandOutput{
					Authority:     OutputAuthorityCatalog,
					Formats:       []OutputFormat{OutputFormatTSV, OutputFormatJSON},
					DefaultFormat: OutputFormatTSV,
					Fields: []OutputField{
						{Name: "check", Type: OutputFieldTypeString, Description: "Stable diagnostic name with unsafe structural runes rendered as visible escapes."},
						{Name: "status", Type: OutputFieldTypeString, Description: "Diagnostic result: pass, warn, or fail."},
						{Name: "detail", Type: OutputFieldTypeString, Description: "Diagnostic detail with unsafe structural runes rendered as visible escapes."},
					},
					Delivery:           OutputDeliveryComplete,
					CollectionCoverage: CollectionCoverageExhaustive,
					JSONEnvelope:       "report",
					JSONSchemaVersion:  1,
				},
				Prerequisites: []string{},
				Errors: []CommandError{
					declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help doctor", "Correct the command arguments."),
					declaredCommandError(fault.KindRejected, "diagnostic_failed", false, "doctor", "Review the failed diagnostic and correct the local prerequisite."),
					declaredCommandError(fault.KindContract, "output_contract_exceeded", false, "doctor", "Review the bounded output contract and diagnostic adapter."),
					declaredCommandError(fault.KindContract, "output_encoding_failed", false, "doctor", "Repair the diagnostic JSON projection."),
					declaredCommandError(fault.KindInternal, "internal_error", false, "doctor", "Retry after investigating the local diagnostic adapter."),
					declaredCommandError(fault.KindInternal, "output_write_failed", true, "doctor", "Retry with a writable output stream."),
					declaredCommandError(fault.KindCanceled, "operation_canceled", true, "doctor", "Retry when the caller is ready."),
				},
			},
			handler: runDoctor,
		},
		CommandSpec{
			Path:    "help",
			Summary: "Show human help or the agent command specification",
			Args:    "[command] [--format text|agent]",
			Effect:  operation.EffectRead,
			Role:    RoleUtility,
			Agent: AgentContract{
				CapabilityID: "cli.discovery",
				Outcome:      "Discover command usage, contracts, workflows, and next actions without external I/O",
				Inputs: []CommandInput{
					{
						Name: "command", Source: InputSourceArgument, Required: false,
						ValueKind: InputValueText, Cardinality: InputCardinalityRepeatable,
						Description: "Select an exact command path or canonical command namespace as one or more path words.", AllowedValues: []string{},
					},
					{
						Name: "--format", Source: InputSourceFlag, Required: false,
						ValueKind: InputValueText, Cardinality: InputCardinalitySingle,
						Description: "Select human text or the machine-readable agent contract.", AllowedValues: []string{"text", "agent"},
						DefaultValue: stringPointer("text"),
					},
				},
				Output: CommandOutput{
					Authority:     OutputAuthorityCatalog,
					Formats:       []OutputFormat{OutputFormatText, OutputFormatJSON},
					DefaultFormat: OutputFormatText,
					Fields: []OutputField{
						{Name: "path", Type: OutputFieldTypeString, Description: "Exact command path accepted as a scoped help selector."},
						{Name: "namespace", Type: OutputFieldTypeString, Description: "Canonical top-level namespace accepted as a scoped help selector."},
						{Name: "summary", Type: OutputFieldTypeString, Description: "Concise description of the command task."},
						{Name: "capability_id", Type: OutputFieldTypeString, Description: "Stable product capability identifier."},
						{Name: "outcome", Type: OutputFieldTypeString, Description: "User outcome the command can achieve."},
						{Name: "effect", Type: OutputFieldTypeString, Description: "Declared read, execute, create, or write effect."},
						{Name: "role", Type: OutputFieldTypeString, Description: "Declared utility, discover, or act workflow role."},
					},
					Delivery:           OutputDeliveryComplete,
					CollectionCoverage: CollectionCoverageExhaustive,
					JSONEnvelope:       "commands",
					JSONSchemaVersion:  agentHelpSchemaVersion,
				},
				Prerequisites: []string{},
				Errors: []CommandError{
					declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help", "Use text or agent format and an exact catalog command path."),
					declaredCommandError(fault.KindContract, "output_encoding_failed", false, "help", "Repair the agent help JSON projection."),
					declaredCommandError(fault.KindInternal, "output_write_failed", true, "help", "Retry with a writable output stream."),
					declaredCommandError(fault.KindCanceled, "operation_canceled", true, "help", "Retry when the caller is ready."),
				},
			},
			handler: runHelp,
		},
		legacyMigrationCommand(
			"policy init",
			"Explain migration from the retired policy-init schema",
			"--catalog <path> --effect read|create|write -- <command>",
			"Return a stable zero-execution migration diagnostic for the retired schema-2 policy draft command",
			"help spec init",
			[]CommandInput{
				{Name: "--catalog", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Historical catalog path; the migration handler does not read it.", AllowedValues: []string{}},
				{Name: "--effect", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Historical source-effect value; it is not converted into surface semantics.", AllowedValues: []string{"read", "create", "write"}},
				{Name: "command", Source: InputSourceArgument, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalityRepeatable, Description: "Historical exact command path after the positional-only marker.", AllowedValues: []string{}},
			},
			runPolicyInit,
		),
		legacyMigrationCommand(
			"policy validate",
			"Explain migration from retired policy schemas",
			"--catalog <path> --policy <path>",
			"Return a stable zero-execution migration diagnostic for retired authorization-centered policy schemas",
			"help spec validate",
			[]CommandInput{
				{Name: "--catalog", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Historical catalog path; the migration handler does not read it.", AllowedValues: []string{}},
				{Name: "--policy", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Historical policy path; the migration handler does not read it.", AllowedValues: []string{}},
			},
			runPolicyValidate,
		),
		CommandSpec{
			Path:    "spec init",
			Summary: "Create a schema-4 identity or admitted optimizer baseline",
			Args:    "--catalog <path> [--processor <inspection.json>] -- <command>",
			Effect:  operation.EffectRead,
			Role:    RoleUtility,
			Agent: AgentContract{
				CapabilityID: "tailoring.spec.init",
				Outcome:      "Create an exclude-by-default schema-4 authoring baseline for one exact verified command: identity without processor evidence, or the finite registry-owned optimizer default when one explicit source-compatible processor observation is supplied",
				Inputs: []CommandInput{
					{Name: "--catalog", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Read the exact bounded JSON document emitted by source inspect.", AllowedValues: []string{}},
					{Name: "--processor", Source: InputSourceFlag, Required: false, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Optionally read one exact bounded JSON document emitted by processor inspect; no executable discovery or inspection occurs during authoring.", AllowedValues: []string{}},
					{Name: "command", Source: InputSourceArgument, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalityRepeatable, Description: "Select one exact verified source command path after the positional-only marker.", AllowedValues: []string{}},
				},
				Output: CommandOutput{
					Authority: OutputAuthorityCatalog,
					Formats:   []OutputFormat{OutputFormatText}, DefaultFormat: OutputFormatText,
					Fields: []OutputField{
						{Name: "specification", Type: OutputFieldTypeObject, Description: "Complete schema-4 YAML tailoring specification authoring baseline; processor evidence may select only a registry-owned finite optimizer default.", Schema: tailoringSpecificationOutputSchema()},
					},
					Delivery: OutputDeliveryComplete, CollectionCoverage: CollectionCoverageNotApplicable,
				},
				Prerequisites: []string{
					"A source inspect JSON document containing the exact command as verified_builtin evidence; use its versioned source-command-catalog inventory to select command paths, options, structured-output selectors, and fields.",
					"Without --processor, the emitted identity wrapper is an authoring baseline for review and preview, not an executable transform.",
					"With --processor, the exact observation must match a closed source-command, processor adapter, version, platform, artifact, and output-contract tuple; Atsura performs no PATH lookup, download, installation, inspection, or processor execution while authoring.",
					"The built-in projection grammar is kind=transform; explicit empty before and after arrays; invoke.append_args as exact argv elements; output.kind=projection; output.projection.input=json; a non-empty ordered output.projection.select drawn from the command's cataloged structured-output fields; optional output.projection.rename entries from selected fields to unique output names; and output.projection.render=compact_json. Optimizers require a separately admitted finite contract and exact processor evidence. Arbitrary shell, script, jq, plugin, external-transformer, and runtime-LLM actions are invalid.",
				},
				Errors: append([]CommandError{
					declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help spec init", "Pass a catalog, at most one processor observation, and one exact command path."),
					declaredCommandError(fault.KindNotFound, "catalog_command_not_found", false, "help spec init", "Select an exact command present in the catalog."),
					declaredCommandError(fault.KindRejected, "unverified_catalog_command", false, "source inspect", "Use only verified built-in command evidence."),
					declaredCommandError(fault.KindContract, "invalid_source_catalog", false, "source inspect", "Regenerate a valid source catalog."),
					declaredCommandError(fault.KindContract, "invalid_specification_draft", false, "help spec init", "Inspect schema-4 draft construction."),
					declaredCommandError(fault.KindNotFound, "catalog_file_not_found", false, "source inspect", "Generate and select a source inspection JSON file."),
					declaredCommandError(fault.KindPermission, "catalog_file_permission_denied", false, "source inspect", "Correct catalog file permissions."),
					declaredCommandError(fault.KindInvalidInput, "unsafe_catalog_file", false, "source inspect", "Use a stable regular source inspection file."),
					declaredCommandError(fault.KindInvalidInput, "catalog_file_too_large", false, "source inspect", "Regenerate a bounded source inspection file."),
					declaredCommandError(fault.KindUnavailable, "catalog_file_read_failed", true, "source inspect", "Retry after the catalog file is readable."),
					declaredCommandError(fault.KindInvalidInput, "invalid_catalog_file", false, "source inspect", "Regenerate strict source inspection JSON."),
					declaredCommandError(fault.KindRejected, "catalog_digest_mismatch", false, "source inspect", "Regenerate and review source inspection JSON."),
				}, append(processorObservationErrors("spec init"),
					declaredCommandError(fault.KindRejected, "processor_default_not_admitted", false, "processor inspect", "Generate current processor evidence and select an exact source command admitted by the finite compatibility registry."),
					declaredCommandError(fault.KindContract, "invalid_processor_default", false, "help spec init", "Repair the registry-owned optimizer authoring default."),
					declaredCommandError(fault.KindContract, "output_contract_exceeded", false, "help spec init", "Reduce the bounded draft output."),
					declaredCommandError(fault.KindContract, "output_encoding_failed", false, "help spec init", "Repair deterministic YAML projection."),
					declaredCommandError(fault.KindInternal, "internal_error", false, "help spec init", "Inspect catalog loading and draft construction."),
					declaredCommandError(fault.KindInternal, "output_write_failed", true, "spec init", "Retry with a writable output stream."),
					declaredCommandError(fault.KindCanceled, "operation_canceled", true, "spec init", "Retry when the caller is ready."),
				)...),
			},
			handler: runSpecInit,
		},
		CommandSpec{
			Path:    "spec validate",
			Summary: "Validate and normalize a catalog-bound schema-4 specification",
			Args:    "--catalog <path> --spec <path>",
			Effect:  operation.EffectRead,
			Role:    RoleUtility,
			Agent: AgentContract{
				CapabilityID: "tailoring.spec.validate",
				Outcome:      "Validate one strict schema-4 YAML tailoring specification against exact source catalog evidence and return its canonical digest and surface-wrapper counts",
				Inputs: []CommandInput{
					{Name: "--catalog", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Read the exact bounded JSON document emitted by source inspect.", AllowedValues: []string{}},
					{Name: "--spec", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Read one bounded strict schema-4 tailoring specification.", AllowedValues: []string{}},
				},
				Output: CommandOutput{
					Authority: OutputAuthorityCatalog,
					Formats:   []OutputFormat{OutputFormatJSON}, DefaultFormat: OutputFormatJSON,
					Fields: []OutputField{
						{Name: "valid", Type: OutputFieldTypeBoolean, Description: "True only after strict syntax and catalog-bound semantic validation."},
						{Name: "catalog_digest", Type: OutputFieldTypeString, Description: "Exact canonical catalog digest required by the specification."},
						{Name: "specification_digest", Type: OutputFieldTypeString, Description: "SHA-256 identity of normalized canonical specification JSON."},
						{Name: "command_count", Type: OutputFieldTypeInteger, Description: "Number of explicit command entries."},
						{Name: "included_count", Type: OutputFieldTypeInteger, Description: "Number of explicit included command entries."},
						{Name: "excluded_count", Type: OutputFieldTypeInteger, Description: "Number of explicit excluded command entries."},
						{Name: "identity_wrapper_count", Type: OutputFieldTypeInteger, Description: "Number of explicit identity wrappers."},
						{Name: "transform_wrapper_count", Type: OutputFieldTypeInteger, Description: "Number of explicit transforming wrappers."},
						{Name: "specification", Type: OutputFieldTypeObject, Description: "Normalized vendor-neutral schema-4 tailoring specification with the complete finite authoring inventory.", Schema: tailoringSpecificationOutputSchema()},
					},
					Delivery: OutputDeliveryComplete, CollectionCoverage: CollectionCoverageNotApplicable,
					JSONEnvelope: "validation", JSONSchemaVersion: 2,
				},
				Prerequisites: []string{
					"A reviewed source inspect JSON document and schema-4 YAML specification; validation does not adopt either artifact.",
					"Use the versioned tailoring-specification inventory published on this command's normalized specification output to author surface membership, option membership, identity wrappers, and the finite JSON transform grammar without inferring fields from prose.",
				},
				Errors: artifactInputErrors("spec validate", false),
			},
			handler: runSpecValidate,
		},
		CommandSpec{
			Path:    "bundle build",
			Summary: "Compile catalog and specification into one canonical bundle",
			Args:    "--catalog <path> --spec <path> [--processor <inspection.json>]",
			Effect:  operation.EffectRead,
			Role:    RoleUtility,
			Agent: AgentContract{
				CapabilityID: "tailoring.bundle.build",
				Outcome:      "Compile exact source evidence, a valid schema-4 surface-wrapper specification, and exactly required explicit processor evidence into one deterministic identity-bound bundle without adopting or executing it",
				Inputs: []CommandInput{
					{Name: "--catalog", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Read the exact bounded JSON document emitted by source inspect.", AllowedValues: []string{}},
					{Name: "--spec", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Read one bounded strict schema-4 tailoring specification.", AllowedValues: []string{}},
					{Name: "--processor", Source: InputSourceFlag, Required: false, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Supply one exact processor inspect JSON document only when the specification contains an optimizer stage.", AllowedValues: []string{}},
				},
				Output: CommandOutput{
					Authority: OutputAuthorityCatalog,
					Formats:   []OutputFormat{OutputFormatJSON}, DefaultFormat: OutputFormatJSON,
					Fields: []OutputField{
						{Name: "bundle_digest", Type: OutputFieldTypeString, Description: "SHA-256 identity of the canonical embedded bundle JSON."},
						{Name: "bundle", Type: OutputFieldTypeObject, Description: "Canonical catalog, normalized specification, recomputable digests, purpose-specific surface, wrappers, and any exact registry-admitted processor binding."},
					},
					Delivery: OutputDeliveryComplete, CollectionCoverage: CollectionCoverageNotApplicable,
					JSONEnvelope: "build", JSONSchemaVersion: 2,
				},
				Prerequisites: []string{
					"A source inspect JSON document and schema-4 specification that passes spec validate; build does not create an adoption receipt.",
					"An optimizer specification requires exactly one explicit --processor observation; a specification without an optimizer rejects that option before reading processor evidence. Bundle build performs no processor inspection or execution.",
				},
				Errors: append(artifactInputErrors("bundle build", true), append(processorObservationErrors("bundle build"),
					declaredCommandError(fault.KindInvalidInput, "processor_observation_required", false, "processor inspect", "Generate and supply exact processor evidence for the optimizer specification."),
					declaredCommandError(fault.KindInvalidInput, "processor_observation_not_used", false, "help bundle build", "Omit --processor when the specification has no optimizer stage."),
					declaredCommandError(fault.KindRejected, "processor_compatibility_not_admitted", false, "processor inspect", "Generate current evidence for an exact source and optimizer tuple admitted by the finite registry."),
				)...),
			},
			handler: runBundleBuild,
		},
		CommandSpec{
			Path:    "bundle status",
			Summary: "Inspect exact bundle adoption, source, and processor drift",
			Args:    "--bundle <path>",
			Effect:  operation.EffectRead,
			Role:    RoleUtility,
			Agent: AgentContract{
				CapabilityID: "tailoring.bundle.status",
				Outcome:      "Determine whether one exact purpose-specific bundle is user-adopted and report its current source and processor identities independently without starting either",
				Inputs: []CommandInput{
					{Name: "--bundle", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Read the exact bounded JSON document emitted by bundle build.", AllowedValues: []string{}},
				},
				Output: CommandOutput{
					Authority: OutputAuthorityCatalog,
					Formats:   []OutputFormat{OutputFormatJSON}, DefaultFormat: OutputFormatJSON,
					Fields: []OutputField{
						{Name: "bundle_digest", Type: OutputFieldTypeString, Description: "Exact canonical bundle identity."},
						{Name: "catalog_digest", Type: OutputFieldTypeString, Description: "Recomputed catalog identity embedded in the bundle."},
						{Name: "specification_digest", Type: OutputFieldTypeString, Description: "Recomputed normalized tailoring specification identity embedded in the bundle."},
						{Name: "adoption", Type: OutputFieldTypeString, Description: "User-local exact-digest adoption state: adopted, not_adopted, or invalid; it does not grant source-operation permission."},
						{Name: "source", Type: OutputFieldTypeString, Description: "Current executable state: current, drifted, or unavailable."},
						{Name: "adopted", Type: OutputFieldTypeBoolean, Description: "True only when the exact bundle digest has a valid user-local adoption receipt."},
						{Name: "source_path", Type: OutputFieldTypeString, Description: "Exact resolved source path embedded in the bundle."},
						{Name: "source_sha256", Type: OutputFieldTypeString, Description: "Exact source byte identity embedded in the bundle."},
						{Name: "source_version", Type: OutputFieldTypeString, Description: "Adapter-observed source version embedded in the bundle."},
						{Name: "processors", Type: OutputFieldTypeArray, Description: "Ordered exact bundle processor records; empty when the bundle binds no processor.", Schema: bundleProcessorStatusOutputSchema()},
						{Name: "source_process_attempts", Type: OutputFieldTypeInteger, Description: "Always zero; status starts no source process."},
						{Name: "processor_process_attempts", Type: OutputFieldTypeInteger, Description: "Always zero; status reads processor identity bytes but starts no processor process."},
					},
					Delivery: OutputDeliveryComplete, CollectionCoverage: CollectionCoverageNotApplicable,
					JSONEnvelope: "status", JSONSchemaVersion: 3,
				},
				Prerequisites: []string{"One bundle build JSON document; repository presence does not imply adoption or source-operation permission. Status reads exact source and processor file identities without starting either executable."},
				Errors: append(bundleFileErrors("bundle status"),
					declaredCommandError(fault.KindContract, "output_contract_exceeded", false, "help bundle status", "Repair the bounded status projection."),
					declaredCommandError(fault.KindContract, "output_encoding_failed", false, "help bundle status", "Repair deterministic status JSON."),
					declaredCommandError(fault.KindInternal, "internal_error", false, "bundle status", "Inspect bundle authority wiring."),
					declaredCommandError(fault.KindInternal, "output_write_failed", true, "bundle status", "Retry with a writable output stream."),
					declaredCommandError(fault.KindCanceled, "operation_canceled", true, "bundle status", "Retry when the caller is ready."),
				),
			},
			handler: runBundleStatus,
		},
		CommandSpec{
			Path:    "bundle preview",
			Summary: "Preview one adopted wrapper plan without source execution",
			Args:    "--bundle <path> -- <source-executable> <argv>",
			Effect:  operation.EffectRead,
			Role:    RoleUtility,
			Agent: AgentContract{
				CapabilityID: "tailoring.preview",
				Outcome:      "Resolve one exact attempted invocation against an adopted current bundle and return the complete deterministic tailored wrapper plan without starting the source",
				Inputs: []CommandInput{
					{Name: "--bundle", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Read the exact bounded JSON document emitted by bundle build.", AllowedValues: []string{}},
					{Name: "source-executable", Source: InputSourceArgument, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Use the exact requested executable spelling or resolved path recorded in the bundle after the positional-only marker.", AllowedValues: []string{}},
					{Name: "argv", Source: InputSourceArgument, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalityRepeatable, Description: "Pass the source command path, options, and positional values as separate argv elements; dash-prefixed values require the published positional-only or equals grammar.", AllowedValues: []string{}},
				},
				Output: CommandOutput{
					Authority: OutputAuthorityCatalog,
					Formats:   []OutputFormat{OutputFormatJSON}, DefaultFormat: OutputFormatJSON,
					Fields: []OutputField{
						{Name: "plan_digest", Type: OutputFieldTypeString, Description: "SHA-256 identity of the complete canonical wrapper plan."},
						{Name: "plan", Type: OutputFieldTypeObject, Description: "Complete schema-5 tailored plan binding source, artifacts, surface, specification entry, argv, stages, processor evidence when present, process framing, and runtime bounds.", Schema: wrapperPlanOutputSchema()},
						{Name: "source_process_attempts", Type: OutputFieldTypeInteger, Description: "Always zero; preview reads identity evidence but never starts the source process."},
					},
					Delivery: OutputDeliveryComplete, CollectionCoverage: CollectionCoverageNotApplicable,
					JSONEnvelope: "preview", JSONSchemaVersion: 2,
				},
				Prerequisites: []string{"One current schema-2 bundle whose exact digest is user-adopted; preview revalidates source path, SHA-256, and size and never treats adoption as source authorization."},
				Errors:        bundlePreviewErrors(),
			},
			handler: runBundlePreview,
		},
		CommandSpec{
			Path:    "bundle execute",
			Summary: "Execute one adopted adapter-proven JSON transform wrapper",
			Args:    "--bundle <path> -- <source-executable> <argv>",
			Effect:  operation.EffectExecute,
			Role:    RoleUtility,
			Agent: AgentContract{
				CapabilityID: "tailoring.execute",
				Outcome:      "Rebuild one adopted current wrapper plan, start its exact identity-bound source at most once, and return the complete declared typed JSON transformation without raw fallback",
				Inputs: []CommandInput{
					{Name: "--bundle", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Read the exact bounded JSON document emitted by bundle build.", AllowedValues: []string{}},
					{Name: "source-executable", Source: InputSourceArgument, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Use the exact requested executable spelling or resolved path recorded in the bundle after the positional-only marker.", AllowedValues: []string{}},
					{Name: "argv", Source: InputSourceArgument, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalityRepeatable, Description: "Pass the source command path, options, and positional values as separate argv elements; dash-prefixed values require the published positional-only or equals grammar.", AllowedValues: []string{}},
				},
				Output: CommandOutput{
					Authority: OutputAuthorityCatalog,
					Formats:   []OutputFormat{OutputFormatJSON}, DefaultFormat: OutputFormatJSON,
					Fields: []OutputField{
						{Name: "bundle_digest", Type: OutputFieldTypeString, Description: "Exact canonical bundle identity used to rebuild runtime authority."},
						{Name: "plan_digest", Type: OutputFieldTypeString, Description: "SHA-256 identity of the freshly rebuilt schema-5 wrapper plan; it equals preview for identical current inputs."},
						{Name: "matched_command", Type: OutputFieldTypeArray, Description: "Exact tailored command path selected from the complete embedded catalog."},
						{Name: "wrapper_kind", Type: OutputFieldTypeString, Description: "Always transform in this initial runtime slice."},
						{Name: "output", Type: OutputFieldTypeObject, Description: "Complete compact typed JSON selection; each record has exactly the declared fields in order and external structural text is visibly escaped.", Schema: tailoredJSONOutputSchema()},
						{Name: "source", Type: OutputFieldTypeObject, Description: "Bounded facts from the one successful source attempt; raw stdout and stderr are never included.", Schema: sourceExecutionOutputSchema()},
						{Name: "source_process_attempts", Type: OutputFieldTypeInteger, Description: "Exactly one on success; pre-start failures start zero and post-start failures never retry."},
					},
					Delivery: OutputDeliveryComplete, CollectionCoverage: CollectionCoverageNotApplicable,
					JSONEnvelope: "execution", JSONSchemaVersion: 2,
				},
				Prerequisites: []string{
					"One current schema-2 bundle whose exact digest is user-adopted; execution rebuilds rather than consumes a preview document.",
					"The current runtime accepts only source adapter atsura.source.github_cli contract 2, GitHub CLI major version 2, and exact command issue list or pr list.",
					"A projection wrapper must be kind=transform with output.kind=projection, output.projection.input=json, and output.projection.render=compact_json; it must append exactly one inline --json=<ordered-select> selector whose fields exactly equal output.projection.select in order.",
					"The attempted argv may use only the command-specific maintained GitHub CLI long-option grammar; positional arguments, unmodeled options, separated --json values, duplicate or reordered selectors, selectors after --, and competing --jq, --template, or --web modes fail before source start.",
					"A live GitHub CLI invocation requires source-owned authentication plus repository context from the inherited working directory or an admitted command-specific --repo option; Atsura accepts no credential input and starts the source with closed stdin, inherited working directory and environment, and no shell.",
					"Successful source stderr must be empty in this runtime slice; every post-start failure is non-retryable and raw stdout or stderr is never returned as fallback.",
				},
				Errors: bundleExecuteErrors(),
			},
			handler: runBundleExecute,
		},
		CommandSpec{
			Path:    "wrapper render",
			Summary: "Render one adopted bundle as a deterministic POSIX function",
			Args:    "--bundle <absolute-path> [--format text|json]",
			Effect:  operation.EffectRead,
			Role:    RoleUtility,
			Agent: AgentContract{
				CapabilityID: "tailoring.wrapper.materialize",
				Outcome:      "Produce fixed POSIX function bytes and a review digest bound to one adopted purpose bundle, every exact external processor, and the exact current Atsura runtime",
				Inputs: []CommandInput{
					{Name: "--bundle", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Use one exact absolute clean path to the bounded canonical JSON document emitted by bundle build.", AllowedValues: []string{}},
					{Name: "--format", Source: InputSourceFlag, Required: false, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Choose raw sourceable function text or its schema-2 JSON review envelope.", AllowedValues: []string{"text", "json"}, DefaultValue: stringPointer("text")},
				},
				Output: CommandOutput{
					Authority: OutputAuthorityCatalog,
					Formats:   []OutputFormat{OutputFormatText, OutputFormatJSON}, DefaultFormat: OutputFormatText,
					Fields: []OutputField{
						{Name: "source", Type: OutputFieldTypeString, Description: "Exact sourceable POSIX function bytes represented as UTF-8 text."},
						{Name: "source_sha256", Type: OutputFieldTypeString, Description: "SHA-256 review identity of the exact generated source bytes."},
						{Name: "command", Type: OutputFieldTypeString, Description: "Ordinary portable POSIX command name derived verbatim from the bundle's requested executable."},
						{Name: "contract", Type: OutputFieldTypeObject, Description: "Fixed generated-wrapper contract and shell grammar.", Schema: wrapperContractOutputSchema()},
						{Name: "bundle", Type: OutputFieldTypeObject, Description: "Exact bundle locator and canonical digest embedded by the renderer.", Schema: wrapperBundleBindingOutputSchema()},
						{Name: "runtime", Type: OutputFieldTypeObject, Description: "Exact current Atsura executable path, SHA-256, and bounded size embedded by the renderer.", Schema: wrapperRuntimeBindingOutputSchema()},
						{Name: "source_process_attempts", Type: OutputFieldTypeInteger, Description: "Always zero; rendering fingerprints files but starts no source CLI process."},
						{Name: "processor_process_attempts", Type: OutputFieldTypeInteger, Description: "Always zero; rendering may fingerprint bundle-bound processors but starts no processor process."},
					},
					Delivery: OutputDeliveryComplete, CollectionCoverage: CollectionCoverageNotApplicable,
					JSONEnvelope: "wrapper", JSONSchemaVersion: 2,
				},
				Prerequisites: []string{
					"Linux or macOS, one absolute-path current-schema bundle whose exact digest is user-adopted, and current source, processor, plus Atsura executable identities.",
					"The bundle requested executable must be one portable non-reserved POSIX Name; it is never derived from a path or basename.",
					"The complete included surface and any exact processor tuple must be admitted by the maintained source and processor runtime contracts, including every exposed option.",
					"Text output is fixed product source, not specification-authored code; activation and later modification of those bytes remain caller-owned.",
				},
				Errors: wrapperRenderErrors(),
			},
			handler: runWrapperRender,
		},
		CommandSpec{
			Path:    "wrapper run",
			Summary: "Apply one render-bound wrapper invocation through a fresh plan",
			Args:    "--contract-version=1 --bundle=<absolute-path> --bundle-digest=<sha256> --runtime-path=<absolute-path> --runtime-sha256=<sha256> --runtime-size=<bytes> -- [argv]",
			Effect:  operation.EffectExecute,
			Role:    RoleUtility,
			Agent: AgentContract{
				CapabilityID: "tailoring.wrapper.materialize",
				Outcome:      "Verify one render-produced bundle and runtime closure, rebuild its fresh plan, and emit only its declared transformed-JSON, source-stream, or original-preserving optimizer result after at most one exact source and one exact processor attempt",
				Inputs: []CommandInput{
					{Name: "--contract-version", Source: InputSourceFlag, Required: true, ValueKind: InputValueInteger, Cardinality: InputCardinalitySingle, Description: "Use the exact generated wrapper binding contract version.", AllowedValues: []string{"1"}, Minimum: int64Pointer(1), Maximum: int64Pointer(1)},
					{Name: "--bundle", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Pass the exact absolute clean bundle locator emitted by wrapper render.", AllowedValues: []string{}},
					{Name: "--bundle-digest", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Pass the exact lowercase SHA-256 bundle digest emitted by wrapper render.", AllowedValues: []string{}},
					{Name: "--runtime-path", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Pass the exact absolute clean Atsura executable path emitted by wrapper render.", AllowedValues: []string{}},
					{Name: "--runtime-sha256", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Pass the exact lowercase SHA-256 Atsura runtime digest emitted by wrapper render.", AllowedValues: []string{}},
					{Name: "--runtime-size", Source: InputSourceFlag, Required: true, ValueKind: InputValueInteger, Cardinality: InputCardinalitySingle, Description: "Pass the exact positive bounded Atsura executable size emitted by wrapper render.", AllowedValues: []string{}, Minimum: int64Pointer(1), Maximum: int64Pointer(sourceprocess.MaxExecutableBytes)},
					{Name: "argv", Source: InputSourceArgument, Required: false, ValueKind: InputValueText, Cardinality: InputCardinalityRepeatable, Description: "Forward zero or more source argv elements byte-for-byte after --; spaces, empty values, Unicode, duplicates, and dash-prefixed values remain distinct.", AllowedValues: []string{}},
				},
				Output: freshWrapperPlanAuthoritativeOutput(),
				Prerequisites: []string{
					"Use only the complete closure emitted by wrapper render; the source command spelling is derived from the one strictly loaded bundle and is not another input.",
					"Execution requires the exact bundle to remain adopted and the exact bundle, Atsura runtime, source and processor identities, included surface, invocation, and maintained runtime contracts to remain current.",
					"A transformed_json result is one compact object or array plus LF with empty stderr and status zero; source_stream_passthrough returns exact bounded source stdout/stderr and conventional status without framing, projection, timing, or interleaving claims.",
					"An original_preserving_optimizer result has exactly one declared disposition: preserved_before_processor returns the exact conventional source result with zero processor attempts; preserved_after_processor returns a byte-identical admitted input; optimized returns the validated newline-free optimizer summary. The latter two use status zero, empty stderr, and one processor attempt.",
					"The source and processor each start at most once without a shell; a processor fault never falls back to source bytes. Uncertain post-start and final output-write failures are non-retryable, expose no captured streams through a fault, and never recommend replay.",
				},
				Errors: wrapperRunErrors(),
			},
			handler: runWrapperRun,
		},
		CommandSpec{
			Path:    "bundle trust",
			Summary: "Interactively adopt one exact current tailoring bundle digest",
			Args:    "--bundle <path>",
			Effect:  operation.EffectWrite,
			Role:    RoleAct,
			Agent: AgentContract{
				CapabilityID: "tailoring.bundle.trust",
				Outcome:      "Display one current bundle's exact source, processors, surface, and wrapper summary on a controlling terminal and record its digest as user-adopted after exact confirmation",
				Inputs:       []CommandInput{{Name: "--bundle", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Read the exact bounded JSON document emitted by bundle build.", AllowedValues: []string{}}},
				Output: CommandOutput{
					Authority: OutputAuthorityCatalog,
					Formats:   []OutputFormat{OutputFormatJSON}, DefaultFormat: OutputFormatJSON,
					Fields: []OutputField{
						{Name: "bundle_digest", Type: OutputFieldTypeString, Description: "Exact digest whose user-local adoption was confirmed."},
						{Name: "adopted", Type: OutputFieldTypeBoolean, Description: "True after the exact adoption receipt is present."},
						{Name: "already_adopted", Type: OutputFieldTypeBoolean, Description: "True when no adoption-store mutation was needed."},
						{Name: "source", Type: OutputFieldTypeString, Description: "Source identity state; adoption succeeds only when current."},
						{Name: "processors", Type: OutputFieldTypeArray, Description: "Ordered exact bundle processor records; every processor is current on success and the array is empty for identity-only bundles.", Schema: bundleProcessorStatusOutputSchema()},
						{Name: "source_process_attempts", Type: OutputFieldTypeInteger, Description: "Always zero; adoption starts no source process."},
						{Name: "processor_process_attempts", Type: OutputFieldTypeInteger, Description: "Always zero; adoption reads processor identity bytes but starts no processor process."},
					},
					Delivery: OutputDeliveryComplete, CollectionCoverage: CollectionCoverageNotApplicable,
					JSONEnvelope: "trust", JSONSchemaVersion: 3,
				},
				Prerequisites: []string{"A canonical bundle whose exact source and every bound processor identity are current, plus an interactive controlling terminal; redirected stdin cannot adopt a bundle, and adoption is not source authorization."},
				FixedTarget:   &FixedTarget{Kind: "bundle-adoption-store", ID: "selected", Description: "This Atsura installation's user-local exact-digest bundle adoption store.", Scope: FixedTargetScopeToolLocal},
				Mutation:      &MutationContract{TargetKind: "bundle-adoption-store", TargetInputs: []string{}, Impact: operation.Impact{Cardinality: operation.CardinalityOne, Notification: operation.DeclarationNo, AccessChange: operation.DeclarationYes, Destructive: operation.DeclarationNo}},
				Errors: append(bundleFileErrors("bundle trust"),
					declaredCommandError(fault.KindRejected, "invalid_bundle_trust_store", false, "bundle status", "Repair or reconcile invalid user-local adoption state."),
					declaredCommandError(fault.KindRejected, "bundle_source_drift", false, "bundle status", "Inspect source drift before building and adopting new evidence."),
					declaredCommandError(fault.KindRejected, "bundle_processor_drift", false, "bundle status", "Inspect processor drift before building and adopting new evidence."),
					declaredCommandError(fault.KindUnavailable, "bundle_trust_store_failed", false, "bundle status", "Reconcile adoption state after a safe store failure."),
					declaredCommandError(fault.KindContract, "invalid_mutation_contract", false, "help bundle trust", "Repair the adoption mutation declaration."),
					declaredCommandError(fault.KindContract, "missing_mutation_action", false, "help bundle trust", "Configure the adoption-store mutation action."),
					declaredCommandError(fault.KindRejected, "missing_mutation_policy", false, "help bundle trust", "Configure interactive exact-digest confirmation."),
					declaredCommandError(fault.KindRejected, "mutation_rejected", false, "bundle status", "Review the surface-wrapper summary and confirm only the exact digest."),
					declaredCommandError(fault.KindContract, "unclassified_mutation_outcome", false, "bundle status", "Reconcile adoption state before another mutation."),
					declaredCommandError(fault.KindContract, "output_contract_exceeded", false, "help bundle trust", "Repair the bounded trust result projection."),
					declaredCommandError(fault.KindContract, "output_encoding_failed", false, "help bundle trust", "Repair deterministic trust JSON."),
					declaredCommandError(fault.KindInternal, "internal_error", false, "bundle status", "Inspect bundle authority wiring."),
					declaredCommandError(fault.KindInternal, "mutation_output_write_failed", false, "bundle status", "Reconcile confirmed adoption without repeating it."),
					declaredCommandError(fault.KindCanceled, "operation_canceled", true, "bundle trust", "Retry when the caller is ready."),
				),
			},
			handler: runBundleTrust,
		},
		legacyMigrationCommand(
			"plan preview",
			"Explain migration from retired configuration preview",
			"--config <path> -- <command>",
			"Return a stable zero-execution migration diagnostic for the retired authorization-plan preview",
			"help spec validate",
			[]CommandInput{
				{Name: "--config", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Historical schema-1 configuration path; the migration handler does not read it.", AllowedValues: []string{}},
				{Name: "command", Source: InputSourceArgument, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalityRepeatable, Description: "Historical source executable and argv after the positional-only marker; they are never started.", AllowedValues: []string{}},
			},
			runPlanPreview,
		),
		legacyMigrationCommand(
			"run",
			"Explain migration from retired policy execution",
			"--config <path> -- <command>",
			"Return a stable zero-execution migration diagnostic for the retired authorization-policy run command",
			"help spec validate",
			[]CommandInput{
				{Name: "--config", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Historical schema-1 configuration path; the migration handler does not read it.", AllowedValues: []string{}},
				{Name: "command", Source: InputSourceArgument, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalityRepeatable, Description: "Historical source executable and argv after the positional-only marker; they are never started.", AllowedValues: []string{}},
			},
			runRun,
		),
		CommandSpec{
			Path:    "sample list",
			Summary: "Discover offline samples and their opaque IDs",
			Args:    "[--format tsv|json]",
			Effect:  operation.EffectRead,
			Role:    RoleDiscover,
			Agent: AgentContract{
				CapabilityID: "sample.inspect",
				Outcome:      "Discover every offline sample and its stable opaque reference",
				Inputs: []CommandInput{
					{
						Name: "--format", Source: InputSourceFlag, Required: false,
						ValueKind: InputValueText, Cardinality: InputCardinalitySingle,
						Description: "Select the complete sample collection representation.", AllowedValues: []string{"tsv", "json"},
						DefaultValue: stringPointer("tsv"),
					},
				},
				Output: CommandOutput{
					Authority:     OutputAuthorityCatalog,
					Formats:       []OutputFormat{OutputFormatTSV, OutputFormatJSON},
					DefaultFormat: OutputFormatTSV,
					Fields: []OutputField{
						{Name: "id", Type: OutputFieldTypeString, Description: "Opaque sample reference accepted unchanged by sample read.", ReferenceKind: "sample"},
						{Name: "name", Type: OutputFieldTypeString, Description: "Human-readable label with unsafe structural runes visibly escaped; never use it as an identifier."},
					},
					Delivery:           OutputDeliveryComplete,
					CollectionCoverage: CollectionCoverageExhaustive,
					JSONEnvelope:       "items",
					JSONSchemaVersion:  1,
				},
				Prerequisites: []string{},
				Errors: []CommandError{
					declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help sample list", "Correct the command arguments."),
					declaredCommandError(fault.KindUnavailable, "page_fetch_failed", true, "sample list", "Retry after the sample source is available."),
					declaredCommandError(fault.KindContract, "invalid_page_contract", false, "sample list", "Inspect the sample adapter page contract."),
					declaredCommandError(fault.KindContract, "pagination_page_limit", false, "sample list", "Review the declared pagination page budget."),
					declaredCommandError(fault.KindContract, "pagination_item_limit", false, "sample list", "Review the declared pagination item budget."),
					declaredCommandError(fault.KindContract, "pagination_cursor_loop", false, "sample list", "Inspect the adapter cursor sequence."),
					declaredCommandError(fault.KindContract, "output_contract_exceeded", false, "sample list", "Review the bounded output contract and sample adapter."),
					declaredCommandError(fault.KindContract, "output_encoding_failed", false, "sample list", "Repair the sample list JSON projection."),
					declaredCommandError(fault.KindInternal, "internal_error", false, "sample list", "Inspect the sample adapter and returned items."),
					declaredCommandError(fault.KindInternal, "output_write_failed", true, "sample list", "Retry with a writable output stream."),
					declaredCommandError(fault.KindCanceled, "operation_canceled", true, "sample list", "Retry when the caller is ready."),
				},
			},
			handler: runSampleList,
		},
		CommandSpec{
			Path:    "sample read",
			Summary: "Read exactly one offline sample by opaque ID",
			Args:    "--id <sample-id> [--format tsv|json]",
			Effect:  operation.EffectRead,
			Role:    RoleAct,
			Agent: AgentContract{
				CapabilityID: "sample.inspect",
				Outcome:      "Read one uniquely identified offline sample without rediscovery",
				Inputs: []CommandInput{
					{
						Name: "--id", Source: InputSourceFlag, Required: true,
						ValueKind: InputValueText, Cardinality: InputCardinalitySingle,
						Description: "Pass an id from sample list byte-for-byte without parsing or transformation.", AllowedValues: []string{}, ReferenceKind: "sample",
					},
					{
						Name: "--format", Source: InputSourceFlag, Required: false,
						ValueKind: InputValueText, Cardinality: InputCardinalitySingle,
						Description: "Select the single sample representation.", AllowedValues: []string{"tsv", "json"},
						DefaultValue: stringPointer("tsv"),
					},
				},
				Output: CommandOutput{
					Authority:     OutputAuthorityCatalog,
					Formats:       []OutputFormat{OutputFormatTSV, OutputFormatJSON},
					DefaultFormat: OutputFormatTSV,
					Fields: []OutputField{
						{Name: "id", Type: OutputFieldTypeString, Description: "Exact opaque sample ID requested by the caller."},
						{Name: "name", Type: OutputFieldTypeString, Description: "Human-readable label with unsafe structural runes rendered as visible escapes."},
						{Name: "content", Type: OutputFieldTypeString, Description: "Complete content with unsafe structural runes rendered as visible escapes."},
					},
					Delivery:           OutputDeliveryComplete,
					CollectionCoverage: CollectionCoverageNotApplicable,
					JSONEnvelope:       "item",
					JSONSchemaVersion:  1,
				},
				Prerequisites: []string{},
				Errors: []CommandError{
					declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help sample read", "Pass exactly one opaque sample ID through --id and choose a supported format."),
					declaredCommandError(fault.KindNotFound, "sample_not_found", false, "sample list", "Discover a current opaque sample ID."),
					declaredCommandError(fault.KindContract, "output_contract_exceeded", false, "sample read", "Review the bounded output contract and sample adapter."),
					declaredCommandError(fault.KindContract, "output_encoding_failed", false, "sample read", "Repair the sample JSON projection."),
					declaredCommandError(fault.KindInternal, "internal_error", false, "sample read", "Inspect the sample adapter and returned item."),
					declaredCommandError(fault.KindInternal, "output_write_failed", true, "sample read", "Retry with a writable output stream."),
					declaredCommandError(fault.KindCanceled, "operation_canceled", true, "sample read", "Retry when the caller is ready."),
				},
			},
			handler: runSampleRead,
		},
		CommandSpec{
			Path:    "processor inspect",
			Summary: "Inspect one explicit output processor executable",
			Args:    "--adapter=rtk --executable <absolute-path>",
			Effect:  operation.EffectExecute,
			Role:    RoleUtility,
			Agent: AgentContract{
				CapabilityID: "tailoring.output.optimize",
				Outcome:      "Produce deterministic identity-bound evidence for one maintained RTK output processor by running only its isolated version probe",
				Inputs: []CommandInput{
					{Name: "--adapter", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Select the sole registered finite processor-inspection adapter.", AllowedValues: []string{"rtk"}},
					{Name: "--executable", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Inspect this exact absolute clean processor executable path without PATH discovery.", AllowedValues: []string{}},
				},
				Output: CommandOutput{
					Authority: OutputAuthorityCatalog,
					Formats:   []OutputFormat{OutputFormatJSON}, DefaultFormat: OutputFormatJSON,
					Fields: []OutputField{
						{Name: "observation_digest", Type: OutputFieldTypeString, Description: "SHA-256 identity of the canonical schema-1 processor observation bytes."},
						{Name: "observation", Type: OutputFieldTypeObject, Description: "Exact adapter, native platform, executable identity, version, isolated probe, and attempt evidence.", Schema: processorObservationOutputSchema()},
						{Name: "processor_process_attempts", Type: OutputFieldTypeInteger, Description: "Exact isolated processor probe attempts; successful inspection is always one."},
					},
					Delivery: OutputDeliveryComplete, CollectionCoverage: CollectionCoverageNotApplicable,
					JSONEnvelope: "inspection", JSONSchemaVersion: 1,
				},
				Prerequisites: []string{
					"An official RTK v0.43.0 executable at one explicit absolute path on a maintained Linux or Darwin architecture; Atsura does not discover, download, install, or configure it.",
					"Inspection starts exactly one no-shell --version probe in atsura.processor.rtk_isolated.v1 with no inherited credentials or coding-agent host configuration.",
				},
				Errors: []CommandError{
					declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help processor inspect", "Pass the exact rtk adapter and one absolute executable path."),
					declaredCommandError(fault.KindContract, "processor_adapter_contract", false, "help processor inspect", "Repair the finite processor adapter composition."),
					declaredCommandError(fault.KindInvalidInput, "unsupported_processor_adapter", false, "help processor inspect", "Select the maintained rtk adapter."),
					declaredCommandError(fault.KindInvalidInput, "invalid_processor_executable", false, "help processor inspect", "Pass one absolute clean processor executable path."),
					declaredCommandError(fault.KindUnsupported, "unsupported_processor_platform", false, "help processor inspect", "Inspect RTK only on a maintained Linux or Darwin architecture."),
					declaredCommandError(fault.KindInvalidInput, "unsupported_processor_version", false, "help processor inspect", "Install the exact maintained RTK v0.43.0 version before inspecting again."),
					declaredCommandError(fault.KindRejected, "unsupported_processor_artifact", false, "help processor inspect", "Select the official RTK v0.43.0 artifact for this native platform."),
					declaredCommandError(fault.KindContract, "invalid_processor_observation", false, "help processor inspect", "Inspect the adapter observation and canonical evidence contract."),
					declaredCommandError(fault.KindRejected, "processor_inspection_failed", false, "help processor inspect", "Review the exact isolated version-probe result before inspecting again."),
					declaredCommandError(fault.KindContract, "invalid_processor_process_request", false, "help processor inspect", "Repair the exact processor process request."),
					declaredCommandError(fault.KindUnavailable, "processor_identity_unavailable", true, "processor inspect", "Retry after the explicit executable identity is readable and stable."),
					declaredCommandError(fault.KindInvalidInput, "unsafe_processor_executable", false, "help processor inspect", "Use a supported regular executable rather than a link or special file."),
					declaredCommandError(fault.KindRejected, "processor_identity_changed", false, "help processor inspect", "Review the executable identity before deciding whether to inspect again."),
					declaredCommandError(fault.KindContract, "invalid_processor_identity", false, "help processor inspect", "Repair the processor identity adapter contract."),
					declaredCommandError(fault.KindUnavailable, "processor_environment_setup_failed", true, "processor inspect", "Retry after a private isolated processor environment can be created."),
					declaredCommandError(fault.KindUnavailable, "processor_process_start_failed", true, "processor inspect", "Retry only because the result proves no processor process started."),
					declaredCommandError(fault.KindContract, "processor_stdout_too_large", false, "help processor inspect", "Use processor version evidence within the declared stdout bound."),
					declaredCommandError(fault.KindContract, "processor_stderr_too_large", false, "help processor inspect", "Use processor version evidence within the declared stderr bound."),
					declaredCommandError(fault.KindCanceled, "processor_execution_canceled", false, "help processor inspect", "Review the uncertain post-start processor outcome before deciding whether to inspect again."),
					declaredCommandError(fault.KindUnavailable, "processor_timeout", false, "help processor inspect", "Review the uncertain timed-out processor outcome before deciding whether to inspect again."),
					declaredCommandError(fault.KindRejected, "processor_command_failed", false, "help processor inspect", "Correct the exact version-probe failure before inspecting again."),
					declaredCommandError(fault.KindUnavailable, "processor_process_wait_failed", false, "help processor inspect", "Review the uncertain processor wait outcome before deciding whether to inspect again."),
					declaredCommandError(fault.KindUnavailable, "processor_cleanup_failed", false, "help processor inspect", "Review and remove the isolated temporary processor state before another inspection."),
					declaredCommandError(fault.KindContract, "output_contract_exceeded", false, "help processor inspect", "Reduce the bounded processor observation output."),
					declaredCommandError(fault.KindContract, "output_encoding_failed", false, "help processor inspect", "Repair deterministic processor observation JSON projection."),
					declaredCommandError(fault.KindInternal, "internal_error", false, "help processor inspect", "Inspect processor adapter and orchestration wiring."),
					declaredCommandError(fault.KindInternal, "execute_output_write_failed", false, "help processor inspect", "The processor probe completed; review its outcome before considering replay."),
					declaredCommandError(fault.KindCanceled, "operation_canceled", true, "processor inspect", "Retry only because cancellation occurred before a processor attempt."),
				},
			},
			handler: runProcessorInspect,
		},
		CommandSpec{
			Path:    "source inspect",
			Summary: "Inspect one installed CLI through a bounded source adapter",
			Args:    "--adapter=github-cli|go-cli --executable <path-or-name>",
			Effect:  operation.EffectExecute,
			Role:    RoleUtility,
			Agent: AgentContract{
				CapabilityID: "tailoring.catalog.inspect",
				Outcome:      "Produce a deterministic provenance-bearing catalog for one supported installed source CLI by requesting only the adapter's declared offline probes",
				Inputs: []CommandInput{
					{Name: "--adapter", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Select one registered source-inspection adapter.", AllowedValues: []string{"github-cli", "go-cli"}},
					{Name: "--executable", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Resolve and inspect this source executable path or PATH name.", AllowedValues: []string{}},
				},
				Output: CommandOutput{
					Authority: OutputAuthorityCatalog,
					Formats:   []OutputFormat{OutputFormatJSON}, DefaultFormat: OutputFormatJSON,
					Fields: []OutputField{
						{Name: "catalog_digest", Type: OutputFieldTypeString, Description: "SHA-256 identity of the canonical catalog bytes."},
						{Name: "catalog", Type: OutputFieldTypeObject, Description: "Vendor-neutral source identity, adapter, provenance, probe, command, option, and structured-output evidence with a complete versioned field inventory.", Schema: sourceCatalogOutputSchema()},
						{Name: "source_process_attempts", Type: OutputFieldTypeInteger, Description: "Exact bounded offline probe attempts: four for github-cli contract 2 and three for go-cli contract 2."},
					},
					Delivery: OutputDeliveryComplete, CollectionCoverage: CollectionCoverageExhaustive,
					JSONEnvelope: "inspection", JSONSchemaVersion: 1,
				},
				Prerequisites: []string{"A supported source adapter and installed executable; inspection may start only the adapter's declared offline probes."},
				Errors: []CommandError{
					declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help source inspect", "Correct the source adapter and executable inputs."),
					declaredCommandError(fault.KindInvalidInput, "unsupported_source_version", false, "help source inspect", "Install a version covered by the adapter compatibility contract."),
					declaredCommandError(fault.KindRejected, "source_inspection_failed", false, "help source inspect", "Review malformed or unsupported source inspection evidence."),
					declaredCommandError(fault.KindContract, "invalid_source_catalog", false, "help source inspect", "Inspect the adapter's vendor-neutral catalog mapping."),
					declaredCommandError(fault.KindContract, "invalid_source_process_request", false, "help source inspect", "Repair the adapter probe request."),
					declaredCommandError(fault.KindContract, "invalid_source_process_result", false, "help source inspect", "Inspect the bounded source-process adapter."),
					declaredCommandError(fault.KindNotFound, "source_executable_not_found", false, "help source inspect", "Install or select the source executable."),
					declaredCommandError(fault.KindUnavailable, "source_identity_unavailable", true, "help source inspect", "Retry after the executable identity is readable."),
					declaredCommandError(fault.KindInvalidInput, "unsafe_source_executable", false, "help source inspect", "Use a supported regular executable."),
					declaredCommandError(fault.KindRejected, "source_identity_changed", false, "help source inspect", "Review the executable before inspecting again."),
					declaredCommandError(fault.KindContract, "invalid_source_identity", false, "help source inspect", "Inspect the executable identity adapter."),
					declaredCommandError(fault.KindUnavailable, "source_process_start_failed", true, "help source inspect", "Retry after the executable can be started."),
					declaredCommandError(fault.KindContract, "source_stdout_too_large", false, "help source inspect", "Use source evidence within the adapter byte budget."),
					declaredCommandError(fault.KindContract, "source_stderr_too_large", false, "help source inspect", "Use source evidence within the adapter byte budget."),
					declaredCommandError(fault.KindUnavailable, "source_command_timeout", false, "help source inspect", "Review the uncertain post-start outcome before deciding whether to inspect again."),
					declaredCommandError(fault.KindRejected, "source_command_failed", false, "help source inspect", "Correct the source probe failure before retrying."),
					declaredCommandError(fault.KindUnavailable, "source_process_wait_failed", false, "help source inspect", "Review the uncertain post-start outcome before deciding whether to inspect again."),
					declaredCommandError(fault.KindCanceled, "source_execution_canceled", false, "help source inspect", "Review the uncertain post-start outcome before deciding whether to inspect again."),
					declaredCommandError(fault.KindContract, "output_contract_exceeded", false, "help source inspect", "Reduce the bounded catalog output."),
					declaredCommandError(fault.KindContract, "output_encoding_failed", false, "help source inspect", "Repair the source catalog JSON projection."),
					declaredCommandError(fault.KindInternal, "internal_error", false, "help source inspect", "Inspect the source adapter and orchestration."),
					declaredCommandError(fault.KindInternal, "execute_output_write_failed", false, "help source inspect", "Inspect the catalog output destination; do not assume source-process replay is safe."),
					declaredCommandError(fault.KindCanceled, "operation_canceled", true, "source inspect", "Retry when the caller is ready."),
				},
			},
			handler: runSourceInspect,
		},
		CommandSpec{
			Path:    "version",
			Summary: "Print version information",
			Effect:  operation.EffectRead,
			Role:    RoleUtility,
			Agent: AgentContract{
				CapabilityID: "cli.version",
				Outcome:      "Read the executable version and optional source commit identity",
				Inputs:       []CommandInput{},
				Output: CommandOutput{
					Authority:     OutputAuthorityCatalog,
					Formats:       []OutputFormat{OutputFormatText},
					DefaultFormat: OutputFormatText,
					Fields: []OutputField{
						{Name: "version", Type: OutputFieldTypeString, Description: "Release version embedded in the executable."},
						{Name: "commit", Type: OutputFieldTypeString, Description: "Optional source commit embedded in the executable."},
					},
					Delivery:           OutputDeliveryComplete,
					CollectionCoverage: CollectionCoverageNotApplicable,
				},
				Prerequisites: []string{},
				Errors: []CommandError{
					declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, "help version", "Run version without command arguments."),
					declaredCommandError(fault.KindInternal, "output_write_failed", true, "version", "Retry with a writable output stream."),
					declaredCommandError(fault.KindCanceled, "operation_canceled", true, "version", "Retry when the caller is ready."),
				},
			},
			handler: runVersion,
		},
	)
}

// Validate rejects incomplete command declarations before any handler runs.
func (c Catalog) Validate() error {
	if len(c.commands) == 0 {
		return fmt.Errorf("command catalog is empty")
	}
	seen := make(map[string]struct{}, len(c.commands))
	producedKinds := make(map[string][]string)
	consumedKinds := make(map[string][]string)
	paginationKindOwners := make(map[string]string)
	faultSignatures := make(map[string]catalogFaultSignature)
	for _, declaredError := range defaultAgentErrorContract().GlobalErrors {
		faultSignatures[declaredError.Code] = catalogFaultSignature{
			command:   "agent-help global errors",
			kind:      declaredError.Kind,
			retryable: declaredError.Retryable,
		}
	}
	for index, command := range c.commands {
		if err := operation.ValidateCommandPath(command.Path); err != nil {
			return fmt.Errorf("catalog command %d: %w", index, err)
		}
		if err := validateContractText("command summary", command.Summary); err != nil {
			return fmt.Errorf("catalog command %q has an invalid summary", command.Path)
		}
		if !utf8.ValidString(command.Args) || strings.TrimSpace(command.Args) != command.Args ||
			strings.IndexFunc(command.Args, isUnsafeContractRune) >= 0 {
			return fmt.Errorf("catalog command %q has invalid argument syntax", command.Path)
		}
		if err := command.Effect.Validate(); err != nil {
			return fmt.Errorf("catalog command %q: %w", command.Path, err)
		}
		if err := command.Role.validate(); err != nil {
			return fmt.Errorf("catalog command %q: %w", command.Path, err)
		}
		if err := validateAgentContract(command); err != nil {
			return fmt.Errorf("catalog command %q: %w", command.Path, err)
		}
		if err := validateAgentIndexEntry(command); err != nil {
			return fmt.Errorf("catalog command %q: %w", command.Path, err)
		}
		if err := validateCommandReferenceRole(command); err != nil {
			return fmt.Errorf("catalog command %q: %w", command.Path, err)
		}
		if command.handler == nil {
			return fmt.Errorf("catalog command %q has no handler", command.Path)
		}
		for existing := range seen {
			if strings.HasPrefix(command.Path, existing+" ") || strings.HasPrefix(existing, command.Path+" ") {
				return fmt.Errorf("catalog command paths %q and %q collide at a command/namespace boundary", existing, command.Path)
			}
		}
		if _, exists := seen[command.Path]; exists {
			return fmt.Errorf("catalog contains duplicate command %q", command.Path)
		}
		seen[command.Path] = struct{}{}
		for _, declaredError := range command.Agent.Errors {
			got := catalogFaultSignature{
				command:   command.Path,
				kind:      declaredError.Kind,
				retryable: declaredError.Retryable,
			}
			if previous, exists := faultSignatures[declaredError.Code]; exists &&
				(previous.kind != got.kind || previous.retryable != got.retryable) {
				return fmt.Errorf(
					"catalog fault code %q has conflicting signatures: command %q declares kind %q retryable=%t; command %q declares kind %q retryable=%t",
					declaredError.Code,
					previous.command,
					previous.kind,
					previous.retryable,
					got.command,
					got.kind,
					got.retryable,
				)
			}
			faultSignatures[declaredError.Code] = got
		}
		for _, produced := range command.ProducedRefs() {
			producedKinds[produced.Kind] = append(producedKinds[produced.Kind], command.Path)
		}
		for _, consumed := range command.ConsumedRefs() {
			consumedKinds[consumed.Kind] = append(consumedKinds[consumed.Kind], command.Path)
		}
		if command.Agent.Pagination != nil {
			paginationKindOwners[command.Agent.Pagination.CursorOutput.ReferenceKind] = command.Path
		}
	}
	if err := validateOutputAuthorityReferences(c.commands); err != nil {
		return err
	}
	for kind, owner := range paginationKindOwners {
		producers := producedKinds[kind]
		consumers := consumedKinds[kind]
		if len(producers) != 1 || producers[0] != owner || len(consumers) != 1 || consumers[0] != owner {
			return fmt.Errorf("pagination reference kind %q must be dedicated to command %q", kind, owner)
		}
	}
	for kind, producers := range producedKinds {
		if len(consumedKinds[kind]) == 0 {
			return fmt.Errorf("reference kind %q is produced by %s but has no consumer", kind, strings.Join(producers, ", "))
		}
	}
	for kind, consumers := range consumedKinds {
		if len(producedKinds[kind]) == 0 {
			return fmt.Errorf("reference kind %q is consumed by %s but has no producer", kind, strings.Join(consumers, ", "))
		}
	}
	if err := validateReferenceReachability(c.commands); err != nil {
		return err
	}
	for _, command := range c.commands {
		for _, declaredError := range command.Agent.Errors {
			for _, action := range declaredError.NextActions {
				nextCommand, err := c.resolveRecoveryCommand(action.Command)
				if err != nil {
					return fmt.Errorf("catalog command %q error %q: %w", command.Path, declaredError.Code, err)
				}
				requiresReadOnlyRecovery := declaredError.Code == "unclassified_mutation_outcome" ||
					declaredError.Code == "mutation_output_write_failed" ||
					declaredError.Code == "execute_output_write_failed" ||
					(isMutationEffect(command.Effect) && declaredError.Kind == fault.KindRateLimited && !declaredError.Retryable)
				if requiresReadOnlyRecovery && nextCommand.Effect != operation.EffectRead {
					return fmt.Errorf("catalog command %q error %q must point to a read-only reconciliation command", command.Path, declaredError.Code)
				}
			}
		}
	}
	return nil
}

func validateOutputAuthorityReferences(commands []CommandSpec) error {
	byPath := make(map[string]CommandSpec, len(commands))
	for _, command := range commands {
		byPath[command.Path] = command
	}
	for _, command := range commands {
		output := command.Agent.Output
		if output.Authority != OutputAuthorityFreshWrapperPlan {
			continue
		}
		reference := output.PlanSchema
		if reference == nil {
			continue // Per-command validation reports the governing error.
		}
		source, exists := byPath[reference.Command]
		if !exists {
			return fmt.Errorf("catalog command %q output authority references missing command %q", command.Path, reference.Command)
		}
		var schema *OutputSchema
		for _, field := range source.Agent.Output.Fields {
			if field.Name == reference.Field {
				schema = field.Schema
				break
			}
		}
		if schema == nil {
			return fmt.Errorf("catalog command %q output authority references missing structured field %q on command %q", command.Path, reference.Field, reference.Command)
		}
		if schema.ID != reference.ID || schema.Version != reference.Version {
			return fmt.Errorf("catalog command %q output authority schema %q version %d does not match command %q field %q schema %q version %d", command.Path, reference.ID, reference.Version, reference.Command, reference.Field, schema.ID, schema.Version)
		}
	}
	return nil
}

// resolveRecoveryCommand validates the deliberately small recovery grammar:
// either one exact command path, or help followed by one exact command path or
// canonical namespace. Argument-bearing recovery needs a future typed contract
// rather than an unchecked prose suffix.
func (c Catalog) resolveRecoveryCommand(value string) (CommandSpec, error) {
	words := strings.Fields(value)
	if len(words) == 0 || strings.Join(words, " ") != value {
		return CommandSpec{}, fmt.Errorf("next command %q is not canonical", value)
	}
	if command, found := c.Lookup(value); found {
		return command, nil
	}
	if words[0] != "help" || len(words) == 1 {
		return CommandSpec{}, fmt.Errorf("next command %q is not an exact catalog path", value)
	}
	help, found := c.Lookup("help")
	if !found {
		return CommandSpec{}, fmt.Errorf("next command %q requires the catalog help command", value)
	}
	hasSelector := false
	for _, input := range help.Agent.Inputs {
		if input.Name == "command" && input.Source == InputSourceArgument && !input.Required {
			hasSelector = true
			break
		}
	}
	if !hasSelector {
		return CommandSpec{}, fmt.Errorf("next command %q requires help to declare its optional command selector", value)
	}
	selector := strings.Join(words[1:], " ")
	selected, _ := c.Select(selector)
	if len(selected) == 0 {
		return CommandSpec{}, fmt.Errorf("next command %q has an unknown help selector", value)
	}
	return help, nil
}

func validateAgentContract(command CommandSpec) error {
	contract := command.Agent
	if err := validateCapabilityID(contract.CapabilityID); err != nil {
		return err
	}
	if err := validateContractText("outcome", contract.Outcome); err != nil {
		return err
	}
	if contract.FixedTarget != nil {
		if err := validateFixedTarget(*contract.FixedTarget); err != nil {
			return err
		}
	}
	if contract.Inputs == nil {
		return fmt.Errorf("agent inputs are unknown; use an explicit empty list when there are none")
	}
	seenInputs := make(map[string]struct{}, len(contract.Inputs))
	inputsByName := make(map[string]CommandInput, len(contract.Inputs))
	commandLineInputs := make(map[string]struct{})
	repeatableArgumentSeen := false
	for index, input := range contract.Inputs {
		if err := input.Source.validate(); err != nil {
			return fmt.Errorf("agent input %d: %w", index, err)
		}
		if err := input.ValueKind.validate(); err != nil {
			return fmt.Errorf("agent input %d: %w", index, err)
		}
		if err := input.Cardinality.validate(); err != nil {
			return fmt.Errorf("agent input %d: %w", index, err)
		}
		if err := validateInputName(input); err != nil {
			return fmt.Errorf("agent input %d: %w", index, err)
		}
		if err := validateContractText("input description", input.Description); err != nil {
			return fmt.Errorf("agent input %q: %w", input.Name, err)
		}
		if input.AllowedValues == nil {
			return fmt.Errorf("agent input %q allowed values are unknown; use an explicit empty list for free-form values", input.Name)
		}
		seenValues := make(map[string]struct{}, len(input.AllowedValues))
		for _, value := range input.AllowedValues {
			if err := validateContractText("input allowed value", value); err != nil {
				return fmt.Errorf("agent input %q: %w", input.Name, err)
			}
			if err := validateStableInputLiteral(value); err != nil {
				return fmt.Errorf("agent input %q allowed value: %w", input.Name, err)
			}
			if _, exists := seenValues[value]; exists {
				return fmt.Errorf("agent input %q allowed value %q is declared more than once", input.Name, value)
			}
			seenValues[value] = struct{}{}
		}
		if input.Required && input.DefaultValue != nil {
			return fmt.Errorf("agent input %q cannot be required and declare a default", input.Name)
		}
		if input.Cardinality == InputCardinalityRepeatable && input.DefaultValue != nil {
			return fmt.Errorf("agent repeatable input %q cannot declare one scalar default", input.Name)
		}
		if input.Source != InputSourceArgument && input.Source != InputSourceFlag && input.Cardinality != InputCardinalitySingle {
			return fmt.Errorf("agent non-command-line input %q must use single cardinality", input.Name)
		}
		if input.ValueKind == InputValueBoolean && input.Cardinality == InputCardinalityRepeatable {
			return fmt.Errorf("agent boolean input %q cannot be repeatable", input.Name)
		}
		if input.Source == InputSourceArgument {
			if repeatableArgumentSeen {
				return fmt.Errorf("agent argument input %q follows a repeatable positional input", input.Name)
			}
			repeatableArgumentSeen = input.Cardinality == InputCardinalityRepeatable
		}
		if input.ValueKind != InputValueInteger && (input.Minimum != nil || input.Maximum != nil) {
			return fmt.Errorf("agent non-integer input %q cannot declare numeric bounds", input.Name)
		}
		if input.Minimum != nil && input.Maximum != nil && *input.Minimum > *input.Maximum {
			return fmt.Errorf("agent integer input %q minimum exceeds maximum", input.Name)
		}
		if input.ValueKind == InputValueBoolean && len(input.AllowedValues) != 0 {
			return fmt.Errorf("agent boolean input %q uses the fixed true/false grammar rather than allowed values", input.Name)
		}
		for _, value := range input.AllowedValues {
			if err := validateInputScalar(input, value); err != nil {
				return fmt.Errorf("agent input %q has invalid allowed value %q: %w", input.Name, value, err)
			}
		}
		if input.DefaultValue != nil {
			if err := validateStableInputLiteral(*input.DefaultValue); err != nil {
				return fmt.Errorf("agent input %q has invalid default: %w", input.Name, err)
			}
			if err := validateInputValue(input, *input.DefaultValue); err != nil {
				return fmt.Errorf("agent input %q has invalid default: %w", input.Name, err)
			}
		}
		if _, exists := seenInputs[input.Name]; exists {
			return fmt.Errorf("agent input %q is declared more than once", input.Name)
		}
		seenInputs[input.Name] = struct{}{}
		inputsByName[input.Name] = input
		if input.ReferenceKind != "" {
			if err := validateReferenceName(input.ReferenceKind); err != nil {
				return fmt.Errorf("agent input %q reference kind: %w", input.Name, err)
			}
			if len(input.AllowedValues) != 0 {
				return fmt.Errorf("agent reference input %q must accept opaque values rather than an enumeration", input.Name)
			}
			if input.ValueKind != InputValueText {
				return fmt.Errorf("agent reference input %q must use text values", input.Name)
			}
		}
		if input.Source == InputSourceArgument || input.Source == InputSourceFlag {
			commandLineInputs[input.Name] = struct{}{}
		}
	}
	for _, input := range contract.Inputs {
		if err := validateInputRelations(input, inputsByName); err != nil {
			return err
		}
	}
	if err := validateInputRelationSatisfiability(inputsByName); err != nil {
		return err
	}
	syntaxInputs, syntaxPositionals, err := parseArgumentSyntaxInputs(command.Args)
	if err != nil {
		return err
	}
	declaredPositionals := make([]string, 0)
	for _, input := range contract.Inputs {
		if input.Source == InputSourceArgument {
			declaredPositionals = append(declaredPositionals, input.Name)
		}
	}
	if !equalStrings(declaredPositionals, syntaxPositionals) {
		return fmt.Errorf("agent positional input order %v does not match argument syntax order %v", declaredPositionals, syntaxPositionals)
	}
	for input := range commandLineInputs {
		syntax, exists := syntaxInputs[input]
		if !exists {
			return fmt.Errorf("agent input %q is not present in argument syntax %q", input, command.Args)
		}
		declared := inputsByName[input]
		if declared.Required != syntax.Required {
			return fmt.Errorf("agent input %q required=%t does not match argument syntax required=%t", input, declared.Required, syntax.Required)
		}
		if !equalStrings(declared.AllowedValues, syntax.AllowedValues) {
			return fmt.Errorf("agent input %q allowed values %v do not match argument syntax values %v", input, declared.AllowedValues, syntax.AllowedValues)
		}
		if declared.Source == InputSourceFlag {
			declaredTakesValue := declared.ValueKind != InputValueBoolean
			if declaredTakesValue != syntax.TakesValue {
				return fmt.Errorf("agent input %q value kind %q does not match whether argument syntax takes a value", input, declared.ValueKind)
			}
		}
	}
	for input := range syntaxInputs {
		if _, exists := commandLineInputs[input]; !exists {
			return fmt.Errorf("argument syntax input %q is not described by the agent contract", input)
		}
	}

	if err := contract.Output.Authority.validate(); err != nil {
		return err
	}
	if contract.Output.Formats == nil || len(contract.Output.Formats) == 0 {
		return fmt.Errorf("agent output formats are unknown")
	}
	seenFormats := make(map[OutputFormat]struct{}, len(contract.Output.Formats))
	for _, format := range contract.Output.Formats {
		if err := format.validate(); err != nil {
			return err
		}
		if _, exists := seenFormats[format]; exists {
			return fmt.Errorf("agent output format %q is declared more than once", format)
		}
		seenFormats[format] = struct{}{}
	}
	if err := contract.Output.DefaultFormat.validate(); err != nil {
		return fmt.Errorf("agent default output format: %w", err)
	}
	if _, exists := seenFormats[contract.Output.DefaultFormat]; !exists {
		return fmt.Errorf("agent default output format %q is not supported", contract.Output.DefaultFormat)
	}
	if _, none := seenFormats[OutputFormatNone]; none && len(seenFormats) != 1 {
		return fmt.Errorf("none output format cannot be combined with another format")
	}
	if contract.Output.Fields == nil {
		return fmt.Errorf("agent output fields are unknown; use an explicit empty list when there are none")
	}
	if _, none := seenFormats[OutputFormatNone]; none {
		if len(contract.Output.Fields) != 0 {
			return fmt.Errorf("none output format must not declare fields")
		}
	} else if contract.Output.Authority == OutputAuthorityCatalog && len(contract.Output.Fields) == 0 {
		return fmt.Errorf("agent output must declare at least one field")
	}
	seenFields := make(map[string]struct{}, len(contract.Output.Fields))
	for index, field := range contract.Output.Fields {
		if err := validateOutputFieldName(field.Name); err != nil {
			return fmt.Errorf("agent output field %d: %w", index, err)
		}
		if err := field.Type.validate(); err != nil {
			return fmt.Errorf("agent output field %q: %w", field.Name, err)
		}
		if err := validateContractText("output field description", field.Description); err != nil {
			return fmt.Errorf("agent output field %q: %w", field.Name, err)
		}
		if _, exists := seenFields[field.Name]; exists {
			return fmt.Errorf("agent output field %q is declared more than once", field.Name)
		}
		seenFields[field.Name] = struct{}{}
		if field.ReferenceKind != "" {
			if err := validateReferenceName(field.ReferenceKind); err != nil {
				return fmt.Errorf("agent output field %q reference kind: %w", field.Name, err)
			}
			if field.Type != OutputFieldTypeString {
				return fmt.Errorf("agent output reference field %q must have string type", field.Name)
			}
		}
		if err := validateOutputSchema(field); err != nil {
			return fmt.Errorf("agent output field %q schema: %w", field.Name, err)
		}
	}
	if err := contract.Output.Delivery.validate(); err != nil {
		return err
	}
	if err := contract.Output.CollectionCoverage.validate(); err != nil {
		return err
	}
	if _, none := seenFormats[OutputFormatNone]; none && contract.Output.CollectionCoverage != CollectionCoverageNotApplicable {
		return fmt.Errorf("none output format requires collection coverage %q", CollectionCoverageNotApplicable)
	}
	if err := validateOutputAuthority(contract.Output, seenFormats); err != nil {
		return err
	}
	if err := validatePaginationContract(contract.Output, contract.Pagination, inputsByName); err != nil {
		return err
	}

	if contract.Prerequisites == nil {
		return fmt.Errorf("agent prerequisites are unknown; use an explicit empty list when there are none")
	}
	seenPrerequisites := make(map[string]struct{}, len(contract.Prerequisites))
	for index, prerequisite := range contract.Prerequisites {
		if err := validateContractText(fmt.Sprintf("prerequisite %d", index), prerequisite); err != nil {
			return err
		}
		if _, exists := seenPrerequisites[prerequisite]; exists {
			return fmt.Errorf("agent prerequisite %q is declared more than once", prerequisite)
		}
		seenPrerequisites[prerequisite] = struct{}{}
	}
	if contract.Authentication != nil {
		if err := contract.Authentication.Validate(); err != nil {
			return fmt.Errorf("agent authentication requirement: %w", err)
		}
	}

	if contract.Errors == nil || len(contract.Errors) == 0 {
		return fmt.Errorf("agent error contract is unknown")
	}
	seenErrors := make(map[string]CommandError, len(contract.Errors))
	for index, declaredError := range contract.Errors {
		if declaredError.NextActions == nil || len(declaredError.NextActions) == 0 {
			return fmt.Errorf("agent error %q next actions are unknown", declaredError.Code)
		}
		candidate := fault.New(
			declaredError.Kind,
			declaredError.Code,
			"catalog-declared failure",
			declaredError.Retryable,
			declaredError.NextActions...,
		)
		if err := candidate.Validate(); err != nil {
			return fmt.Errorf("agent error %d: %w", index, err)
		}
		for _, action := range declaredError.NextActions {
			if err := validateContractText("error next command", action.Command); err != nil {
				return fmt.Errorf("agent error %q: %w", declaredError.Code, err)
			}
			if err := validateContractText("error next reason", action.Reason); err != nil {
				return fmt.Errorf("agent error %q: %w", declaredError.Code, err)
			}
		}
		if _, exists := seenErrors[declaredError.Code]; exists {
			return fmt.Errorf("agent error code %q is declared more than once", declaredError.Code)
		}
		seenErrors[declaredError.Code] = declaredError
	}
	if err := requireAgentError(seenErrors, "operation_canceled", fault.KindCanceled, true); err != nil {
		return err
	}
	if err := requireAgentError(seenErrors, "invalid_arguments", fault.KindInvalidInput, false); err != nil {
		return err
	}
	_, noOutput := seenFormats[OutputFormatNone]
	_, hasReadOutputFailure := seenErrors["output_write_failed"]
	_, hasExecuteOutputFailure := seenErrors["execute_output_write_failed"]
	_, hasMutationOutputFailure := seenErrors["mutation_output_write_failed"]
	if !isMutationEffect(command.Effect) && hasMutationOutputFailure {
		return fmt.Errorf("non-mutation command must not declare mutation_output_write_failed")
	}
	if isMutationEffect(command.Effect) && hasReadOutputFailure {
		return fmt.Errorf("mutating command must not declare retryable output_write_failed")
	}
	if command.Effect != operation.EffectExecute && hasExecuteOutputFailure {
		return fmt.Errorf("non-execute command must not declare execute_output_write_failed")
	}
	if noOutput && (hasReadOutputFailure || hasExecuteOutputFailure || hasMutationOutputFailure) {
		return fmt.Errorf("command without output must not declare an output write failure")
	}
	if !noOutput {
		switch command.Effect {
		case operation.EffectRead:
			if err := requireAgentError(seenErrors, "output_write_failed", fault.KindInternal, true); err != nil {
				return err
			}
		case operation.EffectExecute:
			if err := requireAgentError(seenErrors, "execute_output_write_failed", fault.KindInternal, false); err != nil {
				return err
			}
		}
	}
	if contract.Authentication != nil {
		for _, required := range []struct {
			code      string
			kind      fault.Kind
			retryable bool
		}{
			{code: "missing_authentication_context", kind: fault.KindContract},
			{code: "missing_authenticated_action", kind: fault.KindContract},
			{code: "invalid_authentication_requirement", kind: fault.KindContract},
			{code: "missing_authenticator", kind: fault.KindAuthentication},
			{code: "missing_authentication_clock", kind: fault.KindContract},
			{code: "invalid_authentication_session", kind: fault.KindAuthentication},
			{code: "authentication_evaluation_failed", kind: fault.KindContract},
			{code: "insufficient_authentication_capability", kind: fault.KindPermission},
			{code: "authentication_expired", kind: fault.KindAuthentication},
			{code: "authentication_context_mismatch", kind: fault.KindAuthentication},
			{code: "authentication_failed", kind: fault.KindAuthentication},
			{code: "authentication_canceled", kind: fault.KindCanceled},
			{code: "unclassified_authenticated_action_error", kind: fault.KindInternal},
		} {
			if err := requireAgentError(seenErrors, required.code, required.kind, required.retryable); err != nil {
				return err
			}
		}
	}

	if !isMutationEffect(command.Effect) {
		if contract.Mutation != nil {
			return fmt.Errorf("read and execute commands must not declare a mutation contract")
		}
		return nil
	}
	if contract.Mutation == nil {
		return fmt.Errorf("mutating command must declare a mutation contract")
	}
	for _, required := range []struct {
		code      string
		kind      fault.Kind
		retryable bool
	}{
		{code: "invalid_mutation_contract", kind: fault.KindContract},
		{code: "missing_mutation_action", kind: fault.KindContract},
		{code: "missing_mutation_policy", kind: fault.KindRejected},
		{code: "mutation_rejected", kind: fault.KindRejected},
		{code: "unclassified_mutation_outcome", kind: fault.KindContract},
	} {
		if err := requireAgentError(seenErrors, required.code, required.kind, required.retryable); err != nil {
			return err
		}
	}
	if !noOutput {
		if err := requireAgentError(seenErrors, "mutation_output_write_failed", fault.KindInternal, false); err != nil {
			return err
		}
	}
	mutation := contract.Mutation
	if err := validateReferenceName(mutation.TargetKind); err != nil {
		return fmt.Errorf("mutation target kind: %w", err)
	}
	if contract.FixedTarget != nil {
		if mutation.TargetKind != contract.FixedTarget.Kind {
			return fmt.Errorf("mutation target kind must match fixed target kind %q", contract.FixedTarget.Kind)
		}
		if mutation.TargetInputs == nil {
			return fmt.Errorf("fixed-target mutation target_inputs must be an explicit empty list")
		}
		if len(mutation.TargetInputs) != 0 {
			return fmt.Errorf("fixed-target mutation target_inputs must be empty")
		}
		if mutation.ParentInput != "" || mutation.TargetIDInput != "" {
			return fmt.Errorf("fixed-target mutation must not declare parent_input or target_id_input")
		}
		if err := mutation.Impact.Validate(); err != nil {
			return fmt.Errorf("mutation impact: %w", err)
		}
		return nil
	}
	if mutation.TargetInputs == nil || len(mutation.TargetInputs) == 0 {
		return fmt.Errorf("mutation target inputs are unknown")
	}
	seenTargets := make(map[string]struct{}, len(mutation.TargetInputs))
	for _, name := range mutation.TargetInputs {
		if _, exists := seenInputs[name]; !exists {
			return fmt.Errorf("mutation target input %q is not a structured input", name)
		}
		if _, exists := seenTargets[name]; exists {
			return fmt.Errorf("mutation target input %q is declared more than once", name)
		}
		seenTargets[name] = struct{}{}
	}
	if err := mutation.Impact.Validate(); err != nil {
		return fmt.Errorf("mutation impact: %w", err)
	}
	if command.Effect == operation.EffectCreate {
		if mutation.ParentInput == "" || mutation.TargetIDInput != "" {
			return fmt.Errorf("create mutation requires parent_input and must not declare target_id_input")
		}
		parent, err := validateMutationBinding(mutation.ParentInput, mutation.TargetInputs, inputsByName)
		if err != nil {
			return fmt.Errorf("create mutation parent: %w", err)
		}
		if parent.ReferenceKind == "" {
			return fmt.Errorf("create mutation parent input must consume an opaque reference")
		}
		if len(mutation.TargetInputs) != 1 {
			return fmt.Errorf("create mutation target_inputs must contain only parent_input")
		}
	}
	if command.Effect == operation.EffectWrite {
		if mutation.TargetIDInput == "" {
			return fmt.Errorf("write mutation requires target_id_input")
		}
		target, err := validateMutationBinding(mutation.TargetIDInput, mutation.TargetInputs, inputsByName)
		if err != nil {
			return fmt.Errorf("write mutation target ID: %w", err)
		}
		if target.ReferenceKind == "" || target.ReferenceKind != mutation.TargetKind {
			return fmt.Errorf("write mutation target ID must consume the opaque %q reference", mutation.TargetKind)
		}
		expectedTargetInputs := 1
		if mutation.ParentInput != "" {
			if mutation.ParentInput == mutation.TargetIDInput {
				return fmt.Errorf("write mutation parent_input and target_id_input must be distinct")
			}
			parent, err := validateMutationBinding(mutation.ParentInput, mutation.TargetInputs, inputsByName)
			if err != nil {
				return fmt.Errorf("write mutation parent: %w", err)
			}
			if parent.ReferenceKind == "" {
				return fmt.Errorf("write mutation parent input must consume an opaque reference")
			}
			expectedTargetInputs++
		}
		if len(mutation.TargetInputs) != expectedTargetInputs {
			return fmt.Errorf("write mutation target_inputs must contain only target_id_input and optional parent_input")
		}
	}
	return nil
}

func validateMutationBinding(name string, targetInputs []string, inputs map[string]CommandInput) (CommandInput, error) {
	input, exists := inputs[name]
	if !exists {
		return CommandInput{}, fmt.Errorf("input %q is not a structured input", name)
	}
	if input.Source != InputSourceArgument && input.Source != InputSourceFlag {
		return CommandInput{}, fmt.Errorf("input %q must be a command argument or flag", name)
	}
	if !input.Required {
		return CommandInput{}, fmt.Errorf("input %q must be required", name)
	}
	for _, target := range targetInputs {
		if target == name {
			return input, nil
		}
	}
	return CommandInput{}, fmt.Errorf("input %q is not included in target_inputs", name)
}

func validateOutputAuthority(output CommandOutput, formats map[OutputFormat]struct{}) error {
	switch output.Authority {
	case OutputAuthorityCatalog:
		if output.PlanSchema != nil || output.JSONShape != OutputJSONShapeUnknown || output.JSONRendering != OutputJSONRenderingUnknown || output.JSONFraming != OutputJSONFramingUnknown || len(output.PlanResultModes) != 0 {
			return fmt.Errorf("catalog-authoritative output must not declare dynamic output metadata")
		}
		_, supportsJSON := formats[OutputFormatJSON]
		if supportsJSON {
			if err := validateOutputFieldName(output.JSONEnvelope); err != nil {
				return fmt.Errorf("agent JSON envelope: %w", err)
			}
			if output.JSONSchemaVersion <= 0 {
				return fmt.Errorf("agent JSON schema version must be positive")
			}
		} else if output.JSONEnvelope != "" || output.JSONSchemaVersion != 0 {
			return fmt.Errorf("agent JSON metadata requires JSON output support")
		}
		return nil
	case OutputAuthorityFreshWrapperPlan:
		if len(output.Formats) != 1 || output.Formats[0] != OutputFormatPlanResult || output.DefaultFormat != OutputFormatPlanResult {
			return fmt.Errorf("fresh-wrapper-plan-authoritative output must support only plan_result and use it as its default format")
		}
		if len(output.Fields) != 0 {
			return fmt.Errorf("fresh-wrapper-plan-authoritative output must not declare static fields")
		}
		if output.Delivery != OutputDeliveryComplete {
			return fmt.Errorf("fresh-wrapper-plan-authoritative output must use complete delivery")
		}
		if output.CollectionCoverage != CollectionCoverageNotApplicable {
			return fmt.Errorf("fresh-wrapper-plan-authoritative output requires collection coverage %q", CollectionCoverageNotApplicable)
		}
		if output.JSONEnvelope != "" || output.JSONSchemaVersion != 0 || output.JSONShape != OutputJSONShapeUnknown || output.JSONRendering != OutputJSONRenderingUnknown || output.JSONFraming != OutputJSONFramingUnknown {
			return fmt.Errorf("fresh-wrapper-plan-authoritative output must not declare static JSON metadata")
		}
		if output.PlanSchema == nil {
			return fmt.Errorf("fresh-wrapper-plan-authoritative output must reference the bundle preview wrapper-plan schema")
		}
		if output.PlanSchema.Command != "bundle preview" || output.PlanSchema.Field != "plan" || output.PlanSchema.ID != "wrapper-plan" || output.PlanSchema.Version <= 0 {
			return fmt.Errorf("fresh-wrapper-plan-authoritative output must reference bundle preview field plan with a positive wrapper-plan schema version")
		}
		if err := validatePlanResultModes(output.PlanResultModes); err != nil {
			return err
		}
		return nil
	default:
		return nil // Authority validation reports the governing error.
	}
}

func validatePlanResultModes(got []PlanResultModeContract) error {
	want := freshPlanResultModes()
	if len(got) != len(want) {
		return fmt.Errorf("fresh-wrapper-plan-authoritative output must declare exactly %d plan result modes", len(want))
	}
	for index := range want {
		if !reflect.DeepEqual(got[index], want[index]) {
			return fmt.Errorf("fresh-wrapper-plan-authoritative result mode %d is incomplete or out of order", index)
		}
	}
	return nil
}

func validatePaginationContract(output CommandOutput, pagination *PaginationContract, inputs map[string]CommandInput) error {
	switch output.Delivery {
	case OutputDeliveryComplete:
		if pagination != nil {
			return fmt.Errorf("complete output must not declare a pagination binding")
		}
		return nil
	case OutputDeliveryPaged:
		if pagination == nil {
			return fmt.Errorf("paged output must declare a pagination binding")
		}
		if output.CollectionCoverage == CollectionCoverageNotApplicable {
			return fmt.Errorf("paged output requires collection coverage")
		}
	default:
		return nil // Delivery validation reports the governing error.
	}
	if len(output.Formats) != 1 || output.Formats[0] != OutputFormatJSON || output.DefaultFormat != OutputFormatJSON {
		return fmt.Errorf("paged output must support only JSON and use JSON as its default format")
	}

	cursorInput, exists := inputs[pagination.CursorInput]
	if !exists {
		return fmt.Errorf("pagination cursor input %q is not a structured input", pagination.CursorInput)
	}
	if cursorInput.Required {
		return fmt.Errorf("pagination cursor input %q must be optional", pagination.CursorInput)
	}
	if cursorInput.Source != InputSourceArgument && cursorInput.Source != InputSourceFlag {
		return fmt.Errorf("pagination cursor input %q must be a command argument or flag", pagination.CursorInput)
	}
	if cursorInput.ReferenceKind == "" {
		return fmt.Errorf("pagination cursor input %q must consume an opaque reference", pagination.CursorInput)
	}
	if err := validateOutputFieldName(pagination.CursorOutput.Name); err != nil {
		return fmt.Errorf("pagination cursor output: %w", err)
	}
	if pagination.CursorOutput.Name == "schema_version" || pagination.CursorOutput.Name == output.JSONEnvelope {
		return fmt.Errorf("pagination cursor output %q collides with top-level JSON metadata", pagination.CursorOutput.Name)
	}
	if pagination.CursorOutput.Type != OutputFieldTypeString {
		return fmt.Errorf("pagination cursor output %q must have string type", pagination.CursorOutput.Name)
	}
	if err := validateContractText("pagination cursor output description", pagination.CursorOutput.Description); err != nil {
		return err
	}
	if err := validateReferenceName(pagination.CursorOutput.ReferenceKind); err != nil {
		return fmt.Errorf("pagination cursor output %q reference kind: %w", pagination.CursorOutput.Name, err)
	}
	if cursorInput.ReferenceKind != pagination.CursorOutput.ReferenceKind {
		return fmt.Errorf("pagination cursor input and output must use the same reference kind")
	}
	if err := pagination.Completion.validate(); err != nil {
		return err
	}

	for name, input := range inputs {
		if name != pagination.CursorInput && input.ReferenceKind == cursorInput.ReferenceKind {
			return fmt.Errorf("pagination reference kind %q has an extra cursor input %q", cursorInput.ReferenceKind, name)
		}
	}
	for _, field := range output.Fields {
		if field.ReferenceKind == cursorInput.ReferenceKind {
			return fmt.Errorf("pagination reference kind %q has an extra cursor output %q", cursorInput.ReferenceKind, field.Name)
		}
	}
	return nil
}

func requireAgentError(declared map[string]CommandError, code string, kind fault.Kind, retryable bool) error {
	contract, exists := declared[code]
	if !exists {
		return fmt.Errorf("agent error contract must declare runtime error %q", code)
	}
	if contract.Kind != kind || contract.Retryable != retryable {
		return fmt.Errorf("agent runtime error %q must declare kind %q and retryable=%t", code, kind, retryable)
	}
	return nil
}

func validateCapabilityID(value string) error {
	if value == "" || strings.Trim(value, ".") != value {
		return fmt.Errorf("agent capability ID is missing or invalid: %q", value)
	}
	parts := strings.Split(value, ".")
	if len(parts) < 2 {
		return fmt.Errorf("agent capability ID must contain lowercase dot-separated segments: %q", value)
	}
	for _, part := range parts {
		if err := validateReferenceName(part); err != nil {
			return fmt.Errorf("agent capability ID %q: %w", value, err)
		}
	}
	return nil
}

func validateInputName(input CommandInput) error {
	if input.Name == "" || len(input.Name) > 4096 || !utf8.ValidString(input.Name) || strings.TrimSpace(input.Name) != input.Name ||
		strings.IndexFunc(input.Name, func(r rune) bool { return unicode.IsSpace(r) || isUnsafeContractRune(r) }) >= 0 {
		return fmt.Errorf("input name is missing or invalid: %q", input.Name)
	}
	switch input.Source {
	case InputSourceFlag:
		if !strings.HasPrefix(input.Name, "--") {
			return fmt.Errorf("flag input %q must be a long flag", input.Name)
		}
		if err := validateReferenceName(strings.TrimPrefix(input.Name, "--")); err != nil {
			return fmt.Errorf("flag input: %w", err)
		}
	case InputSourceArgument:
		if err := validateReferenceName(input.Name); err != nil {
			return fmt.Errorf("argument input: %w", err)
		}
	case InputSourceStdin:
		if err := validateOutputFieldName(input.Name); err != nil {
			return fmt.Errorf("stdin input: %w", err)
		}
	case InputSourceEnvironment:
		if err := validateEnvironmentInputName(input.Name); err != nil {
			return err
		}
	case InputSourceConfiguration:
		for _, segment := range strings.Split(input.Name, ".") {
			if err := validateOutputFieldName(segment); err != nil {
				return fmt.Errorf("configuration input name is invalid: %q", input.Name)
			}
		}
	}
	return nil
}

func validateEnvironmentInputName(value string) error {
	if value == "" {
		return fmt.Errorf("environment input name is empty")
	}
	for index, character := range value {
		switch {
		case character >= 'A' && character <= 'Z':
		case index > 0 && character >= '0' && character <= '9':
		case index > 0 && character == '_':
		default:
			return fmt.Errorf("environment input name is invalid: %q", value)
		}
	}
	return nil
}

func validateInputValue(input CommandInput, value string) error {
	if len(input.AllowedValues) != 0 {
		allowed := false
		for _, candidate := range input.AllowedValues {
			if value == candidate {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("value must be one of %s", strings.Join(input.AllowedValues, ", "))
		}
	}
	return validateInputScalar(input, value)
}

func validateInputScalar(input CommandInput, value string) error {
	if !utf8.ValidString(value) || strings.IndexFunc(value, isUnsafeContractRune) >= 0 {
		return fmt.Errorf("value contains invalid structural text")
	}
	switch input.ValueKind {
	case InputValueText:
		return nil
	case InputValueInteger:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("value must be a base-10 integer")
		}
		if input.Minimum != nil && parsed < *input.Minimum {
			return fmt.Errorf("value must be at least %d", *input.Minimum)
		}
		if input.Maximum != nil && parsed > *input.Maximum {
			return fmt.Errorf("value must be at most %d", *input.Maximum)
		}
		return nil
	case InputValueBoolean:
		if value != "true" && value != "false" {
			return fmt.Errorf("value must be true or false")
		}
		return nil
	default:
		return fmt.Errorf("value kind is invalid")
	}
}

func validateStableInputLiteral(value string) error {
	for _, character := range []byte(value) {
		if character < 0x20 || character > 0x7e {
			return fmt.Errorf("catalog-owned values must use stable printable ASCII bytes")
		}
	}
	return nil
}

func validateInputRelations(input CommandInput, inputs map[string]CommandInput) error {
	requires := make(map[string]struct{}, len(input.Requires))
	for _, name := range input.Requires {
		if name == input.Name {
			return fmt.Errorf("agent input %q cannot require itself", input.Name)
		}
		if _, duplicate := requires[name]; duplicate {
			return fmt.Errorf("agent input %q requires %q more than once", input.Name, name)
		}
		target, exists := inputs[name]
		if !exists {
			return fmt.Errorf("agent input %q requires unknown input %q", input.Name, name)
		}
		if !isCommandLineInput(input) || !isCommandLineInput(target) {
			return fmt.Errorf("agent input %q relation to %q is not enforceable by the command-line parser", input.Name, name)
		}
		if input.Required && !target.Required {
			return fmt.Errorf("required agent input %q makes optional required input %q effectively mandatory", input.Name, name)
		}
		requires[name] = struct{}{}
	}
	conflicts := make(map[string]struct{}, len(input.ConflictsWith))
	for _, name := range input.ConflictsWith {
		if name == input.Name {
			return fmt.Errorf("agent input %q cannot conflict with itself", input.Name)
		}
		if _, duplicate := conflicts[name]; duplicate {
			return fmt.Errorf("agent input %q conflicts with %q more than once", input.Name, name)
		}
		target, exists := inputs[name]
		if !exists {
			return fmt.Errorf("agent input %q conflicts with unknown input %q", input.Name, name)
		}
		if !isCommandLineInput(input) || !isCommandLineInput(target) {
			return fmt.Errorf("agent input %q relation to %q is not enforceable by the command-line parser", input.Name, name)
		}
		if _, required := requires[name]; required {
			return fmt.Errorf("agent input %q both requires and conflicts with %q", input.Name, name)
		}
		if input.Required && target.Required {
			return fmt.Errorf("required agent inputs %q and %q cannot conflict", input.Name, name)
		}
		conflicts[name] = struct{}{}
	}
	return nil
}

// validateInputRelationSatisfiability proves that the required invocation and
// every optional input can appear in at least one valid invocation after the
// transitive requires closure is applied. Conflicts are symmetric presence
// constraints even when declared on only one endpoint.
func validateInputRelationSatisfiability(inputs map[string]CommandInput) error {
	names := make([]string, 0, len(inputs))
	required := make(map[string]bool, len(inputs))
	for name, input := range inputs {
		names = append(names, name)
		if input.Required {
			required[name] = true
		}
	}
	sort.Strings(names)
	if left, right, conflict := inputPresenceConflict(required, inputs); conflict {
		return fmt.Errorf("required agent inputs %q and %q conflict after dependency expansion", left, right)
	}
	for _, name := range names {
		if inputs[name].Required {
			continue
		}
		selected := make(map[string]bool, len(required)+1)
		for requiredName := range required {
			selected[requiredName] = true
		}
		selected[name] = true
		if left, right, conflict := inputPresenceConflict(selected, inputs); conflict {
			return fmt.Errorf("optional agent input %q is unusable because %q and %q conflict after dependency expansion", name, left, right)
		}
	}
	return nil
}

func inputPresenceConflict(selected map[string]bool, inputs map[string]CommandInput) (string, string, bool) {
	queue := make([]string, 0, len(selected))
	for name := range selected {
		queue = append(queue, name)
	}
	sort.Strings(queue)
	for index := 0; index < len(queue); index++ {
		name := queue[index]
		for _, required := range inputs[name].Requires {
			if !selected[required] {
				selected[required] = true
				queue = append(queue, required)
			}
		}
	}
	selectedNames := make([]string, 0, len(selected))
	for name := range selected {
		selectedNames = append(selectedNames, name)
	}
	sort.Strings(selectedNames)
	for _, name := range selectedNames {
		for _, conflict := range inputs[name].ConflictsWith {
			if selected[conflict] {
				return name, conflict, true
			}
		}
	}
	return "", "", false
}

func isCommandLineInput(input CommandInput) bool {
	return input.Source == InputSourceArgument || input.Source == InputSourceFlag
}

type argumentSyntaxInput struct {
	Required      bool
	AllowedValues []string
	TakesValue    bool
}

type argumentSyntaxToken struct {
	Value    string
	Optional bool
}

func parseArgumentSyntaxInputs(syntax string) (map[string]argumentSyntaxInput, []string, error) {
	inputs := make(map[string]argumentSyntaxInput)
	positionals := make([]string, 0)
	rawTokens := strings.Fields(syntax)
	tokens := make([]argumentSyntaxToken, 0, len(rawTokens))
	inOptional := false
	for _, raw := range rawTokens {
		opens := strings.HasPrefix(raw, "[")
		closes := strings.HasSuffix(raw, "]")
		if opens {
			if inOptional {
				return nil, nil, fmt.Errorf("argument syntax contains nested optional groups")
			}
			inOptional = true
		}
		if closes && !inOptional {
			return nil, nil, fmt.Errorf("argument syntax contains an unmatched closing bracket")
		}
		value := strings.Trim(raw, "[]()")
		if value == "" {
			return nil, nil, fmt.Errorf("argument syntax contains an empty token")
		}
		tokens = append(tokens, argumentSyntaxToken{Value: value, Optional: inOptional})
		if closes {
			inOptional = false
		}
	}
	if inOptional {
		return nil, nil, fmt.Errorf("argument syntax contains an unclosed optional group")
	}

	optionalPositionalSeen := false
	positionalOnlySeen := false
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if token.Value == "--" {
			if token.Optional || positionalOnlySeen || index == len(tokens)-1 {
				return nil, nil, fmt.Errorf("argument syntax has an invalid positional-only marker")
			}
			positionalOnlySeen = true
			continue
		}
		if strings.HasPrefix(token.Value, "--") {
			if positionalOnlySeen {
				return nil, nil, fmt.Errorf("argument syntax flag %q follows the positional-only marker", token.Value)
			}
			parts := strings.SplitN(token.Value, "=", 2)
			name := parts[0]
			if err := validateInputName(CommandInput{Name: name, Source: InputSourceFlag}); err != nil {
				return nil, nil, fmt.Errorf("argument syntax: %w", err)
			}
			valueSyntax := ""
			if len(parts) == 2 {
				valueSyntax = parts[1]
			} else if index+1 < len(tokens) && tokens[index+1].Optional == token.Optional && isArgumentValueSyntax(tokens[index+1].Value) {
				index++
				valueSyntax = tokens[index].Value
			}
			allowed, err := argumentSyntaxAllowedValues(valueSyntax)
			if err != nil {
				return nil, nil, err
			}
			if _, exists := inputs[name]; exists {
				return nil, nil, fmt.Errorf("argument syntax input %q is declared more than once", name)
			}
			inputs[name] = argumentSyntaxInput{Required: !token.Optional, AllowedValues: allowed, TakesValue: valueSyntax != ""}
			continue
		}

		if strings.HasPrefix(token.Value, "<") && strings.HasSuffix(token.Value, ">") {
			name := strings.Trim(token.Value, "<>")
			if err := validateInputName(CommandInput{Name: name, Source: InputSourceArgument}); err != nil {
				return nil, nil, fmt.Errorf("argument syntax: %w", err)
			}
			if _, exists := inputs[name]; exists {
				return nil, nil, fmt.Errorf("argument syntax input %q is declared more than once", name)
			}
			if !token.Optional && optionalPositionalSeen {
				return nil, nil, fmt.Errorf("required positional input %q follows an optional positional input", name)
			}
			optionalPositionalSeen = optionalPositionalSeen || token.Optional
			positionals = append(positionals, name)
			inputs[name] = argumentSyntaxInput{Required: !token.Optional, AllowedValues: []string{}, TakesValue: true}
			continue
		}

		if token.Optional && !strings.ContainsAny(token.Value, "|<>=") {
			if err := validateInputName(CommandInput{Name: token.Value, Source: InputSourceArgument}); err != nil {
				return nil, nil, fmt.Errorf("argument syntax: %w", err)
			}
			if _, exists := inputs[token.Value]; exists {
				return nil, nil, fmt.Errorf("argument syntax input %q is declared more than once", token.Value)
			}
			optionalPositionalSeen = true
			positionals = append(positionals, token.Value)
			inputs[token.Value] = argumentSyntaxInput{Required: false, AllowedValues: []string{}, TakesValue: true}
			continue
		}
		return nil, nil, fmt.Errorf("argument syntax token %q is outside the supported grammar", token.Value)
	}
	return inputs, positionals, nil
}

func isArgumentValueSyntax(value string) bool {
	return (strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">")) || strings.Contains(value, "|")
}

func argumentSyntaxAllowedValues(value string) ([]string, error) {
	if value == "" || (strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">")) {
		return []string{}, nil
	}
	values := strings.Split(value, "|")
	for _, candidate := range values {
		if err := validateContractText("argument syntax value", candidate); err != nil || strings.ContainsAny(candidate, "[]()<>|=") {
			return nil, fmt.Errorf("argument syntax value %q is invalid", candidate)
		}
	}
	return values, nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validateContractText(label, value string) error {
	if value == "" || len(value) > 4096 || !utf8.ValidString(value) || strings.TrimSpace(value) != value ||
		strings.IndexFunc(value, isUnsafeContractRune) >= 0 {
		return fmt.Errorf("agent %s is missing or invalid", label)
	}
	return nil
}

func isUnsafeContractRune(r rune) bool {
	return unicode.Is(unicode.C, r) || r == '\u2028' || r == '\u2029'
}

// ProducedRefs derives the opaque references exposed by structured output.
func (s CommandSpec) ProducedRefs() []ProducedRef {
	references := make([]ProducedRef, 0, len(s.Agent.Output.Fields)+1)
	for _, field := range s.Agent.Output.Fields {
		if field.ReferenceKind != "" {
			references = append(references, ProducedRef{Kind: field.ReferenceKind, Field: field.Name})
		}
	}
	if pagination := s.Agent.Pagination; pagination != nil && pagination.CursorOutput.ReferenceKind != "" {
		references = append(references, ProducedRef{
			Kind:  pagination.CursorOutput.ReferenceKind,
			Field: pagination.CursorOutput.Name,
		})
	}
	return references
}

// ConsumedRefs derives the opaque references accepted by structured input.
func (s CommandSpec) ConsumedRefs() []ConsumedRef {
	references := make([]ConsumedRef, 0)
	for _, input := range s.Agent.Inputs {
		if input.ReferenceKind != "" {
			references = append(references, ConsumedRef{Kind: input.ReferenceKind, Argument: input.Name})
		}
	}
	return references
}

func validateCommandReferenceRole(command CommandSpec) error {
	produced := command.ProducedRefs()
	for _, reference := range produced {
		if err := validateReferenceName(reference.Kind); err != nil {
			return fmt.Errorf("produced reference kind: %w", err)
		}
		if err := validateOutputFieldName(reference.Field); err != nil {
			return fmt.Errorf("produced reference field: %w", err)
		}
	}

	consumed := command.ConsumedRefs()
	for _, reference := range consumed {
		if err := validateReferenceName(reference.Kind); err != nil {
			return fmt.Errorf("consumed reference kind: %w", err)
		}
	}
	if isMutationEffect(command.Effect) && command.Role != RoleAct {
		return fmt.Errorf("mutating commands must use the act role")
	}

	switch command.Role {
	case RoleUtility:
		if command.Agent.FixedTarget != nil {
			return fmt.Errorf("only act commands may declare a fixed target")
		}
		if len(produced) != 0 || len(consumed) != 0 {
			return fmt.Errorf("utility commands must not produce or consume references")
		}
	case RoleDiscover:
		if command.Agent.FixedTarget != nil {
			return fmt.Errorf("only act commands may declare a fixed target")
		}
		if command.Effect != operation.EffectRead {
			return fmt.Errorf("discover commands must have read effect")
		}
		if len(produced) == 0 {
			return fmt.Errorf("discover commands must produce at least one reference")
		}
	case RoleAct:
		if command.Agent.FixedTarget != nil {
			if len(produced) != 0 || len(consumed) != 0 {
				return fmt.Errorf("fixed-target act commands must not produce or consume references")
			}
			return nil
		}
		if len(consumed) == 0 {
			return fmt.Errorf("act commands must consume at least one reference")
		}
		hasRequiredReference := false
		for _, input := range command.Agent.Inputs {
			if input.Required && input.ReferenceKind != "" {
				hasRequiredReference = true
				break
			}
		}
		if !hasRequiredReference {
			return fmt.Errorf("act commands must require at least one opaque reference")
		}
	}
	return nil
}

func validateFixedTarget(target FixedTarget) error {
	if err := validateReferenceName(target.Kind); err != nil {
		return fmt.Errorf("fixed target kind: %w", err)
	}
	if err := validateReferenceName(target.ID); err != nil {
		return fmt.Errorf("fixed target ID: %w", err)
	}
	if err := validateContractText("fixed target description", target.Description); err != nil {
		return err
	}
	if target.Scope != FixedTargetScopeToolLocal {
		return fmt.Errorf("fixed target scope must be %q", FixedTargetScopeToolLocal)
	}
	return nil
}

func validateAgentIndexEntry(command CommandSpec) error {
	encoded, err := json.Marshal(projectAgentIndexCommand(command))
	if err != nil {
		return fmt.Errorf("agent index entry cannot be encoded: %w", err)
	}
	if len(encoded) > maxAgentIndexEntryBytes {
		return fmt.Errorf("agent index entry is %d bytes; maximum is %d", len(encoded), maxAgentIndexEntryBytes)
	}
	return nil
}

// validateReferenceReachability rejects closed reference cycles. A kind is
// reachable only when some producer can run after all of its required opaque
// inputs are themselves reachable. Optional inputs, including a first-page
// cursor, do not prevent a command from seeding a workflow.
func validateReferenceReachability(commands []CommandSpec) error {
	reachable := make(map[string]struct{})
	for {
		progress := false
		for _, command := range commands {
			ready := true
			for _, input := range command.Agent.Inputs {
				if !input.Required || input.ReferenceKind == "" {
					continue
				}
				if _, exists := reachable[input.ReferenceKind]; !exists {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			for _, produced := range command.ProducedRefs() {
				if _, exists := reachable[produced.Kind]; exists {
					continue
				}
				reachable[produced.Kind] = struct{}{}
				progress = true
			}
		}
		if !progress {
			break
		}
	}

	for _, command := range commands {
		for _, produced := range command.ProducedRefs() {
			if _, exists := reachable[produced.Kind]; !exists {
				return fmt.Errorf("reference kind %q is trapped in a closed required-reference cycle", produced.Kind)
			}
		}
	}
	return nil
}

func validateReferenceName(value string) error {
	if value == "" {
		return fmt.Errorf("reference name is empty")
	}
	for index, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case index > 0 && r >= '0' && r <= '9':
		case index > 0 && r == '-':
		default:
			return fmt.Errorf("reference name is invalid: %q", value)
		}
	}
	return nil
}

func validateOutputFieldName(value string) error {
	if value == "" {
		return fmt.Errorf("output field name is empty")
	}
	for index, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case index > 0 && r >= '0' && r <= '9':
		case index > 0 && (r == '-' || r == '_'):
		default:
			return fmt.Errorf("output field name is invalid: %q", value)
		}
	}
	return nil
}

func validateOutputSchema(field OutputField) error {
	if field.Schema == nil {
		return nil
	}
	if field.Type != OutputFieldTypeObject && field.Type != OutputFieldTypeArray {
		return fmt.Errorf("only object fields and object-element array fields may publish a nested schema")
	}
	if err := validateReferenceName(field.Schema.ID); err != nil {
		return fmt.Errorf("id: %w", err)
	}
	if field.Schema.Version <= 0 || field.Schema.Fields == nil || len(field.Schema.Fields) == 0 || len(field.Schema.Fields) > 128 {
		return fmt.Errorf("version and a non-empty bounded field inventory are required")
	}
	previous := ""
	for index, nested := range field.Schema.Fields {
		if err := validateOutputSchemaPath(nested.Path); err != nil {
			return fmt.Errorf("field %d: %w", index, err)
		}
		if nested.Path <= previous {
			return fmt.Errorf("field paths must be sorted and unique")
		}
		previous = nested.Path
		if err := nested.Type.validate(); err != nil {
			return fmt.Errorf("field %q: %w", nested.Path, err)
		}
		if nested.Type == OutputFieldTypeArray {
			if err := nested.ElementType.validate(); err != nil {
				return fmt.Errorf("array field %q element type: %w", nested.Path, err)
			}
		} else if nested.ElementType != OutputFieldTypeUnknown {
			return fmt.Errorf("non-array field %q cannot declare an element type", nested.Path)
		}
		if nested.Nullable && nested.Type != OutputFieldTypeObject {
			return fmt.Errorf("nullable field %q must be an object", nested.Path)
		}
	}
	return nil
}

func validateOutputSchemaPath(path string) error {
	if len(path) < 2 || len(path) > 512 || !strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return fmt.Errorf("schema path is invalid: %q", path)
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for index, part := range parts {
		if part == "*" {
			if index == 0 || index == len(parts)-1 {
				return fmt.Errorf("array wildcard must occur between named path segments: %q", path)
			}
			continue
		}
		if err := validateOutputFieldName(part); err != nil {
			return fmt.Errorf("schema path segment: %w", err)
		}
	}
	return nil
}

// Commands returns a copy in the curated display order.
func (c Catalog) Commands() []CommandSpec {
	commands := make([]CommandSpec, len(c.commands))
	for index, command := range c.commands {
		commands[index] = cloneCommandSpec(command)
	}
	return commands
}

// Lookup finds one exact command path.
func (c Catalog) Lookup(path string) (CommandSpec, bool) {
	for _, command := range c.commands {
		if command.Path == path {
			return cloneCommandSpec(command), true
		}
	}
	return CommandSpec{}, false
}

// Match selects the longest catalog path that prefixes args.
func (c Catalog) Match(args []string) (CommandSpec, []string, bool) {
	var (
		matched      CommandSpec
		matchedWords int
	)
	for _, command := range c.commands {
		words := strings.Split(command.Path, " ")
		if len(words) <= matchedWords || len(words) > len(args) {
			continue
		}
		match := true
		for index := range words {
			if args[index] != words[index] {
				match = false
				break
			}
		}
		if match {
			matched = command
			matchedWords = len(words)
		}
	}
	if matchedWords == 0 {
		return CommandSpec{}, nil, false
	}
	return cloneCommandSpec(matched), args[matchedWords:], true
}

func cloneCommandSpec(command CommandSpec) CommandSpec {
	command.Agent = cloneAgentContract(command.Agent)
	return command
}

func cloneAgentContract(contract AgentContract) AgentContract {
	contract.Inputs = cloneSlice(contract.Inputs)
	for index := range contract.Inputs {
		contract.Inputs[index].AllowedValues = cloneSlice(contract.Inputs[index].AllowedValues)
		contract.Inputs[index].Requires = cloneSlice(contract.Inputs[index].Requires)
		contract.Inputs[index].ConflictsWith = cloneSlice(contract.Inputs[index].ConflictsWith)
		if contract.Inputs[index].DefaultValue != nil {
			value := *contract.Inputs[index].DefaultValue
			contract.Inputs[index].DefaultValue = &value
		}
		if contract.Inputs[index].Minimum != nil {
			value := *contract.Inputs[index].Minimum
			contract.Inputs[index].Minimum = &value
		}
		if contract.Inputs[index].Maximum != nil {
			value := *contract.Inputs[index].Maximum
			contract.Inputs[index].Maximum = &value
		}
	}
	contract.Output.Formats = cloneSlice(contract.Output.Formats)
	contract.Output.Fields = cloneSlice(contract.Output.Fields)
	contract.Output.PlanResultModes = cloneSlice(contract.Output.PlanResultModes)
	for index := range contract.Output.PlanResultModes {
		contract.Output.PlanResultModes[index].SuccessVariants = cloneSlice(contract.Output.PlanResultModes[index].SuccessVariants)
	}
	for index := range contract.Output.Fields {
		if contract.Output.Fields[index].Schema != nil {
			schema := *contract.Output.Fields[index].Schema
			schema.Fields = cloneSlice(schema.Fields)
			contract.Output.Fields[index].Schema = &schema
		}
	}
	if contract.Output.PlanSchema != nil {
		reference := *contract.Output.PlanSchema
		contract.Output.PlanSchema = &reference
	}
	if contract.Pagination != nil {
		pagination := *contract.Pagination
		contract.Pagination = &pagination
	}
	contract.Prerequisites = cloneSlice(contract.Prerequisites)
	if contract.Authentication != nil {
		authentication := contract.Authentication.Clone()
		contract.Authentication = &authentication
	}
	if contract.FixedTarget != nil {
		fixedTarget := *contract.FixedTarget
		contract.FixedTarget = &fixedTarget
	}
	contract.Errors = cloneSlice(contract.Errors)
	for index := range contract.Errors {
		contract.Errors[index].NextActions = cloneSlice(contract.Errors[index].NextActions)
	}
	if contract.Mutation != nil {
		mutation := *contract.Mutation
		mutation.TargetInputs = cloneSlice(mutation.TargetInputs)
		contract.Mutation = &mutation
	}
	return contract
}

func cloneSlice[T any](values []T) []T {
	if values == nil {
		return nil
	}
	cloned := make([]T, len(values))
	copy(cloned, values)
	return cloned
}
