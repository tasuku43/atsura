package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/authn"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

func noOpHandler(context.Context, *CLI, CommandSpec, operation.Intent, ParsedInputs) int {
	return ExitOK
}

func utilitySpec(path string) CommandSpec {
	return CommandSpec{
		Path:    path,
		Summary: "Complete a test outcome",
		Effect:  operation.EffectRead,
		Role:    RoleUtility,
		Agent: AgentContract{
			CapabilityID: "test.complete",
			Outcome:      "Complete a bounded test outcome",
			Inputs:       []CommandInput{},
			Output: CommandOutput{
				Authority:          OutputAuthorityCatalog,
				Formats:            []OutputFormat{OutputFormatText},
				DefaultFormat:      OutputFormatText,
				Fields:             []OutputField{{Name: "result", Type: OutputFieldTypeString, Description: "Stable test result."}},
				Delivery:           OutputDeliveryComplete,
				CollectionCoverage: CollectionCoverageNotApplicable,
			},
			Prerequisites: []string{},
			Errors: []CommandError{
				declaredCommandError(fault.KindInvalidInput, "invalid_arguments", false, path, "Correct the command arguments."),
				{
					Code:        "test_failed",
					Kind:        fault.KindInternal,
					Retryable:   false,
					NextActions: []fault.NextAction{{Command: path, Reason: "Inspect the test fixture and correct it."}},
				},
				declaredCommandError(fault.KindInternal, "output_write_failed", true, path, "Retry with a writable output stream."),
				declaredCommandError(fault.KindCanceled, "operation_canceled", true, path, "Retry when the caller is ready."),
			},
		},
		handler: noOpHandler,
	}
}

func mutationRuntimeErrors(path string) []CommandError {
	namespace := strings.Fields(path)[0]
	return []CommandError{
		declaredCommandError(fault.KindContract, "invalid_mutation_contract", false, path, "Repair the mutation declaration."),
		declaredCommandError(fault.KindContract, "missing_mutation_action", false, path, "Configure the mutation action."),
		declaredCommandError(fault.KindRejected, "missing_mutation_policy", false, path, "Configure the project mutation policy."),
		declaredCommandError(fault.KindRejected, "mutation_rejected", false, path, "Review the project mutation policy."),
		declaredCommandError(fault.KindContract, "unclassified_mutation_outcome", false, namespace+" list", "Reconcile the target before deciding whether another mutation is safe."),
		declaredCommandError(fault.KindInternal, "mutation_output_write_failed", false, namespace+" list", "Reconcile the confirmed mutation result without repeating the mutation."),
	}
}

func mutationErrors(base []CommandError, path string) []CommandError {
	errors := make([]CommandError, 0, len(base)+len(mutationRuntimeErrors(path)))
	for _, declared := range base {
		if declared.Code != "output_write_failed" {
			errors = append(errors, declared)
		}
	}
	return append(errors, mutationRuntimeErrors(path)...)
}

func authenticationGateRuntimeErrors(path string) []CommandError {
	return []CommandError{
		declaredCommandError(fault.KindContract, "missing_authentication_context", false, path, "Repair the authenticated use case wiring."),
		declaredCommandError(fault.KindContract, "missing_authenticated_action", false, path, "Configure the authenticated action."),
		declaredCommandError(fault.KindContract, "invalid_authentication_requirement", false, path, "Repair the authentication requirement."),
		declaredCommandError(fault.KindAuthentication, "missing_authenticator", false, path, "Configure a supported authentication method."),
		declaredCommandError(fault.KindContract, "missing_authentication_clock", false, path, "Configure the authentication clock."),
		declaredCommandError(fault.KindAuthentication, "invalid_authentication_session", false, path, "Repair the authentication adapter contract."),
		declaredCommandError(fault.KindContract, "authentication_evaluation_failed", false, path, "Repair the authentication evaluation contract."),
		declaredCommandError(fault.KindPermission, "insufficient_authentication_capability", false, path, "Obtain the required capability."),
		declaredCommandError(fault.KindAuthentication, "authentication_expired", false, path, "Reacquire authentication according to project policy."),
		declaredCommandError(fault.KindAuthentication, "authentication_context_mismatch", false, path, "Select the required account and authentication context."),
		declaredCommandError(fault.KindAuthentication, "authentication_failed", false, path, "Establish authentication with a supported method."),
		declaredCommandError(fault.KindCanceled, "authentication_canceled", false, path, "Start a new attempt when the caller is ready."),
		declaredCommandError(fault.KindInternal, "unclassified_authenticated_action_error", false, path, "Repair the adapter fault classification."),
	}
}

func discoverSpec(path, kind string) CommandSpec {
	spec := utilitySpec(path)
	spec.Summary = "Discover test items"
	spec.Role = RoleDiscover
	spec.Agent.Outcome = "Discover stable test item references"
	spec.Agent.Output.Formats = []OutputFormat{OutputFormatTSV, OutputFormatJSON}
	spec.Agent.Output.DefaultFormat = OutputFormatTSV
	spec.Agent.Output.Fields = []OutputField{
		{Name: "id", Type: OutputFieldTypeString, Description: "Opaque test item ID.", ReferenceKind: kind},
		{Name: "name", Type: OutputFieldTypeString, Description: "Test item name."},
	}
	spec.Agent.Output.JSONEnvelope = "items"
	spec.Agent.Output.JSONSchemaVersion = 1
	spec.Agent.Output.CollectionCoverage = CollectionCoverageExhaustive
	return spec
}

func actSpec(path, kind string, inputs ...string) CommandSpec {
	spec := utilitySpec(path)
	spec.Summary = "Read test items"
	spec.Role = RoleAct
	spec.Agent.Outcome = "Read the selected test items"
	spec.Agent.Inputs = make([]CommandInput, 0, len(inputs))
	parts := make([]string, 0, len(inputs)*2)
	for _, input := range inputs {
		spec.Agent.Inputs = append(spec.Agent.Inputs, CommandInput{
			Name: input, Source: InputSourceFlag, Required: true,
			ValueKind: InputValueText, Cardinality: InputCardinalitySingle,
			Description: "Opaque test item ID.", AllowedValues: []string{}, ReferenceKind: kind,
		})
		parts = append(parts, input, "<"+kind+"-id>")
	}
	spec.Args = strings.Join(parts, " ")
	return spec
}

func fixedTargetActSpec(path string) CommandSpec {
	spec := utilitySpec(path)
	spec.Role = RoleAct
	spec.Agent.FixedTarget = &FixedTarget{
		Kind: "auth-config", ID: "selected", Description: "This CLI installation's selected authentication configuration.",
		Scope: FixedTargetScopeToolLocal,
	}
	return spec
}

func TestDefaultCatalogIsValidAndUnique(t *testing.T) {
	catalog := DefaultCatalog()
	if err := catalog.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	seen := map[string]bool{}
	for _, command := range catalog.Commands() {
		if seen[command.Path] {
			t.Fatalf("duplicate command path %q", command.Path)
		}
		seen[command.Path] = true
		if err := command.Effect.Validate(); err != nil {
			t.Errorf("%s effect: %v", command.Path, err)
		}
		if err := command.Role.validate(); err != nil {
			t.Errorf("%s role: %v", command.Path, err)
		}
	}
	for _, required := range []string{"doctor", "help", "version"} {
		if !seen[required] {
			t.Errorf("catalog is missing %q", required)
		}
	}
}

func TestDefaultCatalogContainsNoRetiredCodingAgentHostSurface(t *testing.T) {
	forbiddenPaths := map[string]struct{}{
		"hook claude-code":        {},
		"hook codex":              {},
		"integration claude-code": {},
		"integration codex":       {},
	}
	forbiddenCapabilities := map[string]struct{}{
		"integration.claude-code.lifecycle": {},
		"integration.claude-code.transport": {},
		"integration.codex.lifecycle":       {},
		"integration.codex.transport":       {},
	}
	for _, command := range DefaultCatalog().Commands() {
		for path := range forbiddenPaths {
			if command.Path == path || strings.HasPrefix(command.Path, path+" ") {
				t.Errorf("retired coding-agent-host command entered product catalog: %q", command.Path)
			}
		}
		if _, forbidden := forbiddenCapabilities[command.Agent.CapabilityID]; forbidden {
			t.Errorf("retired coding-agent-host capability entered product catalog: %q", command.Agent.CapabilityID)
		}
	}
}

func TestDefaultCatalogSeparatesDeliveryFromCollectionCoverage(t *testing.T) {
	wantCoverage := map[string]CollectionCoverage{
		"doctor":      CollectionCoverageExhaustive,
		"help":        CollectionCoverageExhaustive,
		"sample list": CollectionCoverageExhaustive,
		"sample read": CollectionCoverageNotApplicable,
		"version":     CollectionCoverageNotApplicable,
	}
	for path, coverage := range wantCoverage {
		command, found := DefaultCatalog().Lookup(path)
		if !found {
			t.Fatalf("default catalog lacks %q", path)
		}
		if command.Agent.Output.Delivery != OutputDeliveryComplete ||
			command.Agent.Output.CollectionCoverage != coverage {
			t.Errorf("%s output = %+v, want delivery complete and coverage %q", path, command.Agent.Output, coverage)
		}
	}
}

func TestDefaultCatalogOutputsDeclareOneExclusiveAuthority(t *testing.T) {
	for _, command := range DefaultCatalog().Commands() {
		output := command.Agent.Output
		if command.Path == "wrapper run" {
			if output.Authority != OutputAuthorityFreshWrapperPlan {
				t.Errorf("%s output authority = %q, want %q", command.Path, output.Authority, OutputAuthorityFreshWrapperPlan)
			}
			continue
		}
		if output.Authority != OutputAuthorityCatalog {
			t.Errorf("%s output authority = %q, want %q", command.Path, output.Authority, OutputAuthorityCatalog)
		}
		if output.PlanSchema != nil || output.JSONShape != OutputJSONShapeUnknown || output.JSONRendering != OutputJSONRenderingUnknown || output.JSONFraming != OutputJSONFramingUnknown || len(output.PlanResultModes) != 0 {
			t.Errorf("%s catalog-authoritative output has dynamic metadata: %+v", command.Path, output)
		}
	}
}

func freshWrapperPlanAuthorityCatalog(t *testing.T) Catalog {
	t.Helper()
	preview := utilitySpec("bundle preview")
	preview.Agent.Output = CommandOutput{
		Authority:     OutputAuthorityCatalog,
		Formats:       []OutputFormat{OutputFormatJSON},
		DefaultFormat: OutputFormatJSON,
		Fields: []OutputField{{
			Name: "plan", Type: OutputFieldTypeObject,
			Description: "Fresh wrapper plan.", Schema: wrapperPlanOutputSchema(),
		}},
		Delivery: OutputDeliveryComplete, CollectionCoverage: CollectionCoverageNotApplicable,
		JSONEnvelope: "preview", JSONSchemaVersion: 2,
	}
	wrapper := utilitySpec("wrapper run")
	wrapper.Agent.Output = freshWrapperPlanAuthoritativeOutput()
	return NewCatalog(preview, wrapper)
}

func TestCatalogAcceptsFreshWrapperPlanAuthoritativeOutput(t *testing.T) {
	catalog := freshWrapperPlanAuthorityCatalog(t)
	if err := catalog.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	wrapper, found := catalog.Lookup("wrapper run")
	if !found {
		t.Fatal("wrapper run is missing")
	}
	output := wrapper.Agent.Output
	if output.Authority != OutputAuthorityFreshWrapperPlan ||
		!reflect.DeepEqual(output.Formats, []OutputFormat{OutputFormatPlanResult}) ||
		output.DefaultFormat != OutputFormatPlanResult || len(output.Fields) != 0 ||
		output.Delivery != OutputDeliveryComplete || output.CollectionCoverage != CollectionCoverageNotApplicable ||
		output.JSONEnvelope != "" || output.JSONSchemaVersion != 0 ||
		output.JSONShape != OutputJSONShapeUnknown || output.JSONRendering != OutputJSONRenderingUnknown || output.JSONFraming != OutputJSONFramingUnknown ||
		!reflect.DeepEqual(output.PlanResultModes, freshPlanResultModes()) {
		t.Fatalf("fresh wrapper output = %+v", output)
	}
	wantReference := &OutputSchemaReference{Command: "bundle preview", Field: "plan", ID: "wrapper-plan", Version: tailoringplan.SchemaVersion}
	if !reflect.DeepEqual(output.PlanSchema, wantReference) {
		t.Fatalf("plan schema = %+v, want %+v", output.PlanSchema, wantReference)
	}

	encoded, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"json_envelope", "json_schema_version"} {
		if _, exists := document[forbidden]; exists {
			t.Fatalf("dynamic output contains static metadata %q: %s", forbidden, encoded)
		}
	}
	var fields []OutputField
	if err := json.Unmarshal(document["fields"], &fields); err != nil || len(fields) != 0 {
		t.Fatalf("dynamic static fields = %+v, error = %v", fields, err)
	}

	output.PlanSchema.Version = 99
	fresh, found := catalog.Lookup("wrapper run")
	if !found || fresh.Agent.Output.PlanSchema.Version != tailoringplan.SchemaVersion {
		t.Fatal("catalog lookup returned an aliased output plan schema")
	}
	output.PlanResultModes[0].Stdout = PlanResultStreamEmpty
	fresh, found = catalog.Lookup("wrapper run")
	if !found || !reflect.DeepEqual(fresh.Agent.Output.PlanResultModes, freshPlanResultModes()) {
		t.Fatal("catalog lookup returned aliased plan result modes")
	}
}

func TestCatalogRejectsInvalidFreshWrapperPlanOutputAuthority(t *testing.T) {
	tests := map[string]func(*CommandOutput){
		"unknown authority": func(output *CommandOutput) { output.Authority = OutputAuthorityUnknown },
		"extra format":      func(output *CommandOutput) { output.Formats = []OutputFormat{OutputFormatPlanResult, OutputFormatJSON} },
		"non plan default":  func(output *CommandOutput) { output.DefaultFormat = OutputFormatJSON },
		"unknown fields":    func(output *CommandOutput) { output.Fields = nil },
		"static field": func(output *CommandOutput) {
			output.Fields = []OutputField{{Name: "result", Type: OutputFieldTypeObject, Description: "Static result."}}
		},
		"paged delivery":         func(output *CommandOutput) { output.Delivery = OutputDeliveryPaged },
		"collection coverage":    func(output *CommandOutput) { output.CollectionCoverage = CollectionCoverageExhaustive },
		"static envelope":        func(output *CommandOutput) { output.JSONEnvelope = "result" },
		"static schema version":  func(output *CommandOutput) { output.JSONSchemaVersion = 1 },
		"missing schema":         func(output *CommandOutput) { output.PlanSchema = nil },
		"wrong schema command":   func(output *CommandOutput) { output.PlanSchema.Command = "bundle execute" },
		"wrong schema field":     func(output *CommandOutput) { output.PlanSchema.Field = "preview" },
		"wrong schema ID":        func(output *CommandOutput) { output.PlanSchema.ID = "other-plan" },
		"invalid schema version": func(output *CommandOutput) { output.PlanSchema.Version = 0 },
		"static JSON shape":      func(output *CommandOutput) { output.JSONShape = OutputJSONShapeObjectOrArray },
		"static JSON rendering":  func(output *CommandOutput) { output.JSONRendering = OutputJSONRenderingCompact },
		"static JSON framing":    func(output *CommandOutput) { output.JSONFraming = OutputJSONFramingOneValueLF },
		"missing result modes":   func(output *CommandOutput) { output.PlanResultModes = nil },
		"reordered result modes": func(output *CommandOutput) {
			output.PlanResultModes[0], output.PlanResultModes[1] = output.PlanResultModes[1], output.PlanResultModes[0]
		},
		"changed result mode": func(output *CommandOutput) { output.PlanResultModes[1].Projection = PlanResultProjectionVisibleJSON },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			catalog := freshWrapperPlanAuthorityCatalog(t)
			commands := catalog.Commands()
			mutate(&commands[1].Agent.Output)
			if err := NewCatalog(commands...).Validate(); err == nil {
				t.Fatal("invalid fresh-wrapper-plan output authority passed validation")
			}
		})
	}
}

func TestCatalogRejectsPaginationOnFreshWrapperPlanOutput(t *testing.T) {
	catalog := freshWrapperPlanAuthorityCatalog(t)
	commands := catalog.Commands()
	commands[1].Agent.Pagination = &PaginationContract{}
	if err := NewCatalog(commands...).Validate(); err == nil || !strings.Contains(err.Error(), "complete output must not declare a pagination binding") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestCatalogResolvesFreshWrapperPlanSchemaExactly(t *testing.T) {
	catalog := freshWrapperPlanAuthorityCatalog(t)
	commands := catalog.Commands()
	commands[0].Agent.Output.Fields[0].Schema.Version++
	err := NewCatalog(commands...).Validate()
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("schema mismatch error = %v", err)
	}

	wrapperOnly := commands[1]
	err = NewCatalog(wrapperOnly).Validate()
	if err == nil || !strings.Contains(err.Error(), `references missing command "bundle preview"`) {
		t.Fatalf("missing schema source error = %v", err)
	}
}

func TestCatalogRejectsDynamicMetadataOnCatalogAuthoritativeOutput(t *testing.T) {
	for name, mutate := range map[string]func(*CommandOutput){
		"authority schema": func(output *CommandOutput) {
			output.PlanSchema = &OutputSchemaReference{Command: "bundle preview", Field: "plan", ID: "wrapper-plan", Version: 3}
		},
		"JSON shape":        func(output *CommandOutput) { output.JSONShape = OutputJSONShapeObjectOrArray },
		"JSON rendering":    func(output *CommandOutput) { output.JSONRendering = OutputJSONRenderingCompact },
		"JSON framing":      func(output *CommandOutput) { output.JSONFraming = OutputJSONFramingOneValueLF },
		"plan result modes": func(output *CommandOutput) { output.PlanResultModes = freshPlanResultModes() },
	} {
		t.Run(name, func(t *testing.T) {
			spec := utilitySpec("test")
			mutate(&spec.Agent.Output)
			if err := NewCatalog(spec).Validate(); err == nil || !strings.Contains(err.Error(), "must not declare dynamic output metadata") {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestCatalogRejectsIncompleteDeclarations(t *testing.T) {
	valid := utilitySpec("valid")
	missingEffect := utilitySpec("missing-effect")
	missingEffect.Effect = operation.EffectUnknown
	missingRole := utilitySpec("missing-role")
	missingRole.Role = RoleUnknown
	badPath := utilitySpec("Bad Path")
	missingSummary := utilitySpec("missing-summary")
	missingSummary.Summary = ""
	missingHandler := utilitySpec("missing-handler")
	missingHandler.handler = nil

	tests := []Catalog{
		{},
		NewCatalog(missingEffect),
		NewCatalog(missingRole),
		NewCatalog(badPath),
		NewCatalog(missingSummary),
		NewCatalog(missingHandler),
		NewCatalog(valid, valid),
	}
	for index, catalog := range tests {
		if err := catalog.Validate(); err == nil {
			t.Errorf("invalid catalog %d passed validation", index)
		}
	}
}

func TestCatalogRejectsCommandPathNamespaceCollision(t *testing.T) {
	catalog := NewCatalog(utilitySpec("foo"), utilitySpec("foo bar"))
	if err := catalog.Validate(); err == nil || !strings.Contains(err.Error(), "command/namespace boundary") {
		t.Fatalf("Validate() error = %v, want command/namespace collision", err)
	}
}

func TestCatalogRejectsStructuralLineSeparators(t *testing.T) {
	for _, separator := range []rune{'\u2028', '\u2029'} {
		t.Run(strings.ToUpper(strconv.FormatInt(int64(separator), 16)), func(t *testing.T) {
			if err := validateContractText("test value", "before"+string(separator)+"after"); err == nil {
				t.Fatal("structural separator passed contract text validation")
			}

			spec := utilitySpec("test")
			spec.Args = "[label" + string(separator) + "]"
			if err := NewCatalog(spec).Validate(); err == nil || !strings.Contains(err.Error(), "invalid argument syntax") {
				t.Fatalf("Validate() error = %v, want invalid argument syntax", err)
			}
		})
	}
}

func TestArgumentSyntaxRequiredAndAllowedValuesMatchAgentInputs(t *testing.T) {
	valid := utilitySpec("configure")
	valid.Args = "[--mode fast|safe] <target> [label]"
	valid.Agent.Inputs = []CommandInput{
		{Name: "--mode", Source: InputSourceFlag, Required: false, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Select the operating mode.", AllowedValues: []string{"fast", "safe"}},
		{Name: "target", Source: InputSourceArgument, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Target value.", AllowedValues: []string{}},
		{Name: "label", Source: InputSourceArgument, Required: false, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Optional display label.", AllowedValues: []string{}},
		{Name: "CLI_PROFILE", Source: InputSourceEnvironment, Required: false, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Optional environment profile.", AllowedValues: []string{}},
	}
	if err := NewCatalog(valid).Validate(); err != nil {
		t.Fatalf("valid small argument grammar: %v", err)
	}

	tests := map[string]func(*CommandSpec){
		"optional flag declared required":       func(spec *CommandSpec) { spec.Agent.Inputs[0].Required = true },
		"required positional declared optional": func(spec *CommandSpec) { spec.Agent.Inputs[1].Required = false },
		"optional positional declared required": func(spec *CommandSpec) { spec.Agent.Inputs[2].Required = true },
		"enum order differs":                    func(spec *CommandSpec) { spec.Agent.Inputs[0].AllowedValues = []string{"safe", "fast"} },
		"enum set differs":                      func(spec *CommandSpec) { spec.Agent.Inputs[0].AllowedValues = []string{"fast"} },
		"free form claims enumeration":          func(spec *CommandSpec) { spec.Agent.Inputs[1].AllowedValues = []string{"fixed"} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			spec := cloneCommandSpec(valid)
			mutate(&spec)
			if err := NewCatalog(spec).Validate(); err == nil {
				t.Fatal("argument syntax mismatch passed validation")
			}
		})
	}
}

func TestArgumentSyntaxPublishesPositionalOnlyMarker(t *testing.T) {
	valid := utilitySpec("preview")
	valid.Args = "--config <path> -- <command>"
	valid.Agent.Inputs = []CommandInput{
		{Name: "--config", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Configuration path.", AllowedValues: []string{}},
		{Name: "command", Source: InputSourceArgument, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalityRepeatable, Description: "Source command and argv.", AllowedValues: []string{}},
	}
	if err := NewCatalog(valid).Validate(); err != nil {
		t.Fatalf("valid positional-only grammar: %v", err)
	}

	for _, args := range []string{"-- <command> --", "[--] <command>", "--config <path> --"} {
		candidate := cloneCommandSpec(valid)
		candidate.Args = args
		if err := NewCatalog(candidate).Validate(); err == nil {
			t.Errorf("invalid positional-only grammar %q passed validation", args)
		}
	}
}

func TestArgumentSyntaxAllowsOneExactFixedFlagValue(t *testing.T) {
	valid := utilitySpec("items apply")
	valid.Args = "--confirm=destructive"
	valid.Agent.Inputs = []CommandInput{{
		Name: "--confirm", Source: InputSourceFlag, Required: true,
		ValueKind: InputValueText, Cardinality: InputCardinalitySingle,
		Description: "Confirm the exact mutation class.", AllowedValues: []string{"destructive"},
	}}
	if err := NewCatalog(valid).Validate(); err != nil {
		t.Fatalf("valid exact literal flag grammar: %v", err)
	}

	for name, allowed := range map[string][]string{
		"free form":     {},
		"wrong literal": {"access-change"},
		"extra literal": {"destructive", "access-change"},
	} {
		t.Run(name, func(t *testing.T) {
			spec := cloneCommandSpec(valid)
			spec.Agent.Inputs[0].AllowedValues = allowed
			if err := NewCatalog(spec).Validate(); err == nil {
				t.Fatal("exact literal syntax mismatch passed validation")
			}
		})
	}

	malformed := cloneCommandSpec(valid)
	malformed.Args = "--confirm=destructive=unexpected"
	if err := NewCatalog(malformed).Validate(); err == nil {
		t.Fatal("malformed exact literal syntax passed validation")
	}
}

func TestCatalogRequiresOneFaultSignatureAcrossCommands(t *testing.T) {
	first := utilitySpec("first")
	second := utilitySpec("second")
	if err := NewCatalog(first, second).Validate(); err != nil {
		t.Fatalf("matching fault signatures: %v", err)
	}

	for name, mutate := range map[string]func(*CommandError){
		"kind":         func(declared *CommandError) { declared.Kind = fault.KindUnavailable },
		"retryability": func(declared *CommandError) { declared.Retryable = true },
	} {
		t.Run(name, func(t *testing.T) {
			conflicting := cloneCommandSpec(second)
			for index := range conflicting.Agent.Errors {
				if conflicting.Agent.Errors[index].Code == "test_failed" {
					mutate(&conflicting.Agent.Errors[index])
				}
			}
			err := NewCatalog(first, conflicting).Validate()
			if err == nil || !strings.Contains(err.Error(), `fault code "test_failed" has conflicting signatures`) {
				t.Fatalf("Validate() error = %v, want conflicting fault signature", err)
			}
		})
	}
}

func TestCatalogFaultSignaturesIncludeAgentHelpGlobalErrors(t *testing.T) {
	matching := utilitySpec("matching")
	matching.Agent.Errors = append(matching.Agent.Errors, declaredCommandError(
		fault.KindContract,
		"invalid_catalog",
		false,
		matching.Path,
		"Repair the command-specific catalog observation.",
	))
	if err := NewCatalog(matching).Validate(); err != nil {
		t.Fatalf("matching global fault signature with command-local recovery: %v", err)
	}

	for name, declared := range map[string]CommandError{
		"kind": declaredCommandError(
			fault.KindUnavailable,
			"invalid_catalog",
			false,
			"test",
			"Retry after the unavailable dependency recovers.",
		),
		"retryability": declaredCommandError(
			fault.KindContract,
			"invalid_catalog",
			true,
			"test",
			"Retry after repairing the catalog.",
		),
	} {
		t.Run(name, func(t *testing.T) {
			conflicting := utilitySpec("test")
			conflicting.Agent.Errors = append(conflicting.Agent.Errors, declared)
			err := NewCatalog(conflicting).Validate()
			if err == nil || !strings.Contains(err.Error(), `fault code "invalid_catalog" has conflicting signatures`) {
				t.Fatalf("Validate() error = %v, want conflict with agent-help global error", err)
			}
		})
	}
}

func TestCatalogRequiresCommonRuntimeFailures(t *testing.T) {
	removeError := func(spec *CommandSpec, code string) {
		filtered := make([]CommandError, 0, len(spec.Agent.Errors))
		for _, declared := range spec.Agent.Errors {
			if declared.Code != code {
				filtered = append(filtered, declared)
			}
		}
		spec.Agent.Errors = filtered
	}
	for _, code := range []string{"operation_canceled", "output_write_failed"} {
		t.Run("missing_"+code, func(t *testing.T) {
			spec := utilitySpec("test")
			removeError(&spec, code)
			if err := NewCatalog(spec).Validate(); err == nil {
				t.Fatalf("catalog without %q passed validation", code)
			}
		})
	}

	wrong := utilitySpec("test")
	for index := range wrong.Agent.Errors {
		if wrong.Agent.Errors[index].Code == "operation_canceled" {
			wrong.Agent.Errors[index].Retryable = false
		}
	}
	if err := NewCatalog(wrong).Validate(); err == nil {
		t.Fatal("catalog with inconsistent common runtime failure passed")
	}

	noOutput := utilitySpec("test")
	noOutput.Agent.Output.Formats = []OutputFormat{OutputFormatNone}
	noOutput.Agent.Output.DefaultFormat = OutputFormatNone
	noOutput.Agent.Output.Fields = []OutputField{}
	removeError(&noOutput, "output_write_failed")
	if err := NewCatalog(noOutput).Validate(); err != nil {
		t.Fatalf("no-output command unnecessarily requires output_write_failed: %v", err)
	}
	noOutput.Agent.Output.CollectionCoverage = CollectionCoverageExhaustive
	if err := NewCatalog(noOutput).Validate(); err == nil || !strings.Contains(err.Error(), "none output format requires collection coverage") {
		t.Fatalf("no-output command with collection coverage error = %v", err)
	}

	readWithMutationFailure := utilitySpec("read")
	readWithMutationFailure.Agent.Errors = append(readWithMutationFailure.Agent.Errors, declaredCommandError(
		fault.KindInternal, "mutation_output_write_failed", false, "read", "Reconcile without mutation replay.",
	))
	if err := NewCatalog(readWithMutationFailure).Validate(); err == nil || !strings.Contains(err.Error(), "must not declare mutation_output_write_failed") {
		t.Fatalf("read command with mutation output failure error = %v", err)
	}

	noOutputWithWriteFailure := cloneCommandSpec(noOutput)
	noOutputWithWriteFailure.Agent.Output.CollectionCoverage = CollectionCoverageNotApplicable
	noOutputWithWriteFailure.Agent.Errors = append(noOutputWithWriteFailure.Agent.Errors, declaredCommandError(
		fault.KindInternal, "output_write_failed", true, "read", "Retry with a writable stream.",
	))
	if err := NewCatalog(noOutputWithWriteFailure).Validate(); err == nil || !strings.Contains(err.Error(), "without output") {
		t.Fatalf("no-output command with write failure error = %v", err)
	}
}

func TestExecuteEffectIsNonMutationAndRequiresNonRetryableOutputRecovery(t *testing.T) {
	reconcile := utilitySpec("inspect")
	execute := utilitySpec("execute")
	execute.Effect = operation.EffectExecute
	filtered := make([]CommandError, 0, len(execute.Agent.Errors))
	for _, declared := range execute.Agent.Errors {
		if declared.Code != "output_write_failed" {
			filtered = append(filtered, declared)
		}
	}
	execute.Agent.Errors = append(filtered, declaredCommandError(
		fault.KindInternal,
		"execute_output_write_failed",
		false,
		"inspect",
		"Inspect the output boundary without replaying the source process.",
	))
	if err := NewCatalog(reconcile, execute).Validate(); err != nil {
		t.Fatalf("valid execute utility: %v", err)
	}

	withMutation := cloneCommandSpec(execute)
	withMutation.Agent.Mutation = &MutationContract{TargetKind: "source", TargetInputs: []string{}}
	if err := NewCatalog(reconcile, withMutation).Validate(); err == nil || !strings.Contains(err.Error(), "must not declare a mutation contract") {
		t.Fatalf("execute mutation contract error = %v", err)
	}

	retryable := cloneCommandSpec(execute)
	for index := range retryable.Agent.Errors {
		if retryable.Agent.Errors[index].Code == "execute_output_write_failed" {
			retryable.Agent.Errors[index].Retryable = true
		}
	}
	if err := NewCatalog(reconcile, retryable).Validate(); err == nil || !strings.Contains(err.Error(), "execute_output_write_failed") {
		t.Fatalf("retryable execute output error = %v", err)
	}
}

func TestBundlePreviewCatalogDeclaresPurePlanOutcome(t *testing.T) {
	spec, found := DefaultCatalog().Lookup("bundle preview")
	if !found {
		t.Fatal("bundle preview is missing from the catalog")
	}
	if spec.Effect != operation.EffectRead || spec.Role != RoleUtility || spec.Agent.CapabilityID != "tailoring.preview" || spec.Agent.Mutation != nil || spec.Agent.FixedTarget != nil {
		t.Fatalf("bundle preview contract=%+v", spec)
	}
	if spec.Args != "--bundle <path> -- <source-executable> <argv>" || len(spec.Agent.Inputs) != 3 {
		t.Fatalf("bundle preview grammar=%q inputs=%+v", spec.Args, spec.Agent.Inputs)
	}
	wantInputs := []struct {
		name        string
		source      InputSource
		cardinality InputCardinality
	}{
		{name: "--bundle", source: InputSourceFlag, cardinality: InputCardinalitySingle},
		{name: "source-executable", source: InputSourceArgument, cardinality: InputCardinalitySingle},
		{name: "argv", source: InputSourceArgument, cardinality: InputCardinalityRepeatable},
	}
	for index, want := range wantInputs {
		got := spec.Agent.Inputs[index]
		if got.Name != want.name || got.Source != want.source || got.Cardinality != want.cardinality || !got.Required {
			t.Fatalf("input %d=%+v want=%+v", index, got, want)
		}
	}
	if spec.Agent.Output.JSONEnvelope != "preview" || spec.Agent.Output.JSONSchemaVersion != 2 || spec.Agent.Output.Delivery != OutputDeliveryComplete || spec.Agent.Output.CollectionCoverage != CollectionCoverageNotApplicable {
		t.Fatalf("output=%+v", spec.Agent.Output)
	}
	wantFields := []string{"plan_digest", "plan", "source_process_attempts"}
	for index, want := range wantFields {
		if spec.Agent.Output.Fields[index].Name != want {
			t.Fatalf("output field %d=%q want=%q", index, spec.Agent.Output.Fields[index].Name, want)
		}
	}
	planSchema := spec.Agent.Output.Fields[1].Schema
	if planSchema == nil || planSchema.ID != "wrapper-plan" || planSchema.Version != tailoringplan.SchemaVersion || len(planSchema.Fields) < 64 {
		t.Fatalf("plan schema=%+v", planSchema)
	}
	paths := make(map[string]OutputSchemaField, len(planSchema.Fields))
	for _, field := range planSchema.Fields {
		paths[field.Path] = field
	}
	for path, fieldType := range map[string]OutputFieldType{
		"/schema_version":                         OutputFieldTypeInteger,
		"/processor":                              OutputFieldTypeObject,
		"/processor/contract":                     OutputFieldTypeString,
		"/processor/observation/identity/sha256":  OutputFieldTypeString,
		"/processor/execution/max_attempts":       OutputFieldTypeInteger,
		"/source/sha256":                          OutputFieldTypeString,
		"/specification_entry":                    OutputFieldTypeObject,
		"/stages/invoke/max_attempts":             OutputFieldTypeInteger,
		"/stages/invoke/args":                     OutputFieldTypeArray,
		"/stages/invoke/stdin_mode":               OutputFieldTypeString,
		"/stages/invoke/environment_mode":         OutputFieldTypeString,
		"/stages/invoke/working_directory_mode":   OutputFieldTypeString,
		"/stages/output/kind":                     OutputFieldTypeString,
		"/stages/output/projection":               OutputFieldTypeObject,
		"/stages/output/projection/rename/*/from": OutputFieldTypeString,
		"/stages/output/optimizer":                OutputFieldTypeObject,
		"/stages/output/optimizer/contract":       OutputFieldTypeString,
		"/specification_entry/options/include":    OutputFieldTypeArray,
	} {
		declared, exists := paths[path]
		if !exists || declared.Type != fieldType {
			t.Errorf("schema field %q=%+v", path, declared)
		}
	}
	if !paths["/specification_entry"].Nullable || !paths["/processor"].Required || !paths["/processor"].Nullable || !paths["/stages/output"].Nullable ||
		paths["/specification_entry/wrapper/output"].Required || paths["/specification_entry/wrapper/output/projection"].Required ||
		paths["/specification_entry/wrapper/output/optimizer"].Required || paths["/stages/output/projection"].Required || paths["/stages/output/optimizer"].Required {
		t.Fatalf("nullable/conditional schema fields=%+v %+v %+v %+v", paths["/specification_entry"], paths["/processor"], paths["/stages/output"], paths["/specification_entry/wrapper/output"])
	}
	spec.Agent.Output.Fields[1].Schema.Fields[0].Path = "/changed"
	fresh, found := DefaultCatalog().Lookup("bundle preview")
	if !found || fresh.Agent.Output.Fields[1].Schema.Fields[0].Path == "/changed" {
		t.Fatal("catalog lookup returned an aliased nested output schema")
	}
	codes := make(map[string]CommandError, len(spec.Agent.Errors))
	for _, declared := range spec.Agent.Errors {
		codes[declared.Code] = declared
	}
	for code, kind := range map[string]fault.Kind{
		"bundle_not_adopted":         fault.KindRejected,
		"bundle_source_drift":        fault.KindRejected,
		"source_executable_mismatch": fault.KindInvalidInput,
		"invalid_invocation":         fault.KindInvalidInput,
		"command_not_in_surface":     fault.KindNotFound,
		"option_not_in_surface":      fault.KindNotFound,
		"invalid_wrapper_plan":       fault.KindContract,
	} {
		declared, exists := codes[code]
		if !exists || declared.Kind != kind || declared.Retryable {
			t.Errorf("fault %q=%+v", code, declared)
		}
	}
	if err := DefaultCatalog().Validate(); err != nil {
		t.Fatalf("default catalog validation: %v", err)
	}
}

func TestBundleExecuteCatalogDeclaresTransformOnlySourceExecution(t *testing.T) {
	spec, found := DefaultCatalog().Lookup("bundle execute")
	if !found {
		t.Fatal("bundle execute is missing from the catalog")
	}
	if spec.Effect != operation.EffectExecute || spec.Role != RoleUtility || spec.Agent.CapabilityID != "tailoring.execute" || spec.Agent.Mutation != nil || spec.Agent.FixedTarget != nil || spec.Agent.Authentication != nil {
		t.Fatalf("bundle execute contract=%+v", spec)
	}
	if spec.Args != "--bundle <path> -- <source-executable> <argv>" || len(spec.Agent.Inputs) != 3 || spec.Agent.Output.JSONEnvelope != "execution" || spec.Agent.Output.JSONSchemaVersion != 2 || spec.Agent.Output.Delivery != OutputDeliveryComplete || spec.Agent.Output.CollectionCoverage != CollectionCoverageNotApplicable {
		t.Fatalf("grammar/output=%q %+v", spec.Args, spec.Agent.Output)
	}
	wantFields := []string{"bundle_digest", "plan_digest", "matched_command", "wrapper_kind", "output", "source", "source_process_attempts"}
	for index, want := range wantFields {
		if spec.Agent.Output.Fields[index].Name != want {
			t.Fatalf("output field %d=%q want=%q", index, spec.Agent.Output.Fields[index].Name, want)
		}
	}
	if schema := spec.Agent.Output.Fields[4].Schema; schema == nil || schema.ID != "tailored-json-result" || schema.Version != 2 || len(schema.Fields) != 4 {
		t.Fatalf("tailored output schema=%+v", schema)
	}
	prerequisites := strings.Join(spec.Agent.Prerequisites, "\n")
	for _, want := range []string{
		"atsura.source.github_cli contract 2",
		"GitHub CLI major version 2",
		"issue list or pr list",
		"inline --json=<ordered-select>",
		"--jq, --template, or --web",
		"source-owned authentication",
		"repository context from the inherited working directory or an admitted command-specific --repo option",
		"Successful source stderr must be empty",
		"raw stdout or stderr is never returned",
	} {
		if !strings.Contains(prerequisites, want) {
			t.Errorf("bundle execute prerequisites lack %q: %s", want, prerequisites)
		}
	}
	codes := make(map[string]CommandError, len(spec.Agent.Errors))
	for _, declared := range spec.Agent.Errors {
		codes[declared.Code] = declared
	}
	for code, want := range map[string]struct {
		kind      fault.Kind
		retryable bool
	}{
		"wrapper_runtime_not_supported":         {kind: fault.KindUnsupported},
		"source_process_start_failed":           {kind: fault.KindUnavailable, retryable: true},
		"source_execution_canceled":             {kind: fault.KindCanceled},
		"source_output_processing_canceled":     {kind: fault.KindCanceled},
		"source_json_invalid":                   {kind: fault.KindContract},
		"output_transform_failed":               {kind: fault.KindContract},
		"unclassified_source_execution_outcome": {kind: fault.KindContract},
		"execute_output_write_failed":           {kind: fault.KindInternal},
	} {
		got, exists := codes[code]
		if !exists || got.Kind != want.kind || got.Retryable != want.retryable {
			t.Errorf("fault %q=%+v", code, got)
		}
	}
	encoded, _ := json.Marshal(spec.Agent)
	for _, forbidden := range []string{`"decision"`, `"confirmation_required"`, `"source_effect"`, `"target"`, `"impact"`} {
		if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
			t.Fatalf("execute contract contains forbidden field %s: %s", forbidden, encoded)
		}
	}
	if err := DefaultCatalog().Validate(); err != nil {
		t.Fatalf("default catalog validation: %v", err)
	}
}

func TestSourceInspectCatalogPublishesExactAdapterAndNestedCatalogSchema(t *testing.T) {
	spec, found := DefaultCatalog().Lookup("source inspect")
	if !found {
		t.Fatal("source inspect is missing from the catalog")
	}
	if spec.Args != "--adapter=github-cli|go-cli --executable <path-or-name>" || len(spec.Agent.Inputs) != 2 {
		t.Fatalf("source inspect grammar=%q inputs=%+v", spec.Args, spec.Agent.Inputs)
	}
	adapter := spec.Agent.Inputs[0]
	if adapter.Name != "--adapter" || !adapter.Required || !equalStrings(adapter.AllowedValues, []string{"github-cli", "go-cli"}) {
		t.Fatalf("adapter input=%+v", adapter)
	}
	if len(spec.Agent.Output.Fields) != 3 || spec.Agent.Output.Fields[2].Name != "source_process_attempts" ||
		spec.Agent.Output.Fields[2].Description != "Exact bounded offline probe attempts: four for github-cli contract 2 and three for go-cli contract 2." {
		t.Fatalf("source inspection attempt contract=%+v", spec.Agent.Output.Fields)
	}
	var catalogField OutputField
	for _, field := range spec.Agent.Output.Fields {
		if field.Name == "catalog" {
			catalogField = field
			break
		}
	}
	schema := catalogField.Schema
	if schema == nil || schema.ID != "source-command-catalog" || schema.Version != sourcecatalog.SchemaVersion {
		t.Fatalf("source catalog schema=%+v", schema)
	}
	paths := make(map[string]OutputSchemaField, len(schema.Fields))
	for _, field := range schema.Fields {
		paths[field.Path] = field
	}
	want := map[string]struct {
		fieldType   OutputFieldType
		elementType OutputFieldType
	}{
		"/adapter/kind":                                 {fieldType: OutputFieldTypeString},
		"/adapter/contract_version":                     {fieldType: OutputFieldTypeInteger},
		"/source/resolved_path":                         {fieldType: OutputFieldTypeString},
		"/source/sha256":                                {fieldType: OutputFieldTypeString},
		"/probe/attempts":                               {fieldType: OutputFieldTypeInteger},
		"/commands":                                     {fieldType: OutputFieldTypeArray, elementType: OutputFieldTypeObject},
		"/commands/*/path":                              {fieldType: OutputFieldTypeArray, elementType: OutputFieldTypeString},
		"/commands/*/provenance":                        {fieldType: OutputFieldTypeString},
		"/commands/*/options/*/takes_value":             {fieldType: OutputFieldTypeBoolean},
		"/commands/*/structured_output/*/selector_flag": {fieldType: OutputFieldTypeString},
		"/commands/*/structured_output/*/fields":        {fieldType: OutputFieldTypeArray, elementType: OutputFieldTypeString},
	}
	for path, expected := range want {
		got, exists := paths[path]
		if !exists || got.Type != expected.fieldType || got.ElementType != expected.elementType || !got.Required {
			t.Errorf("source catalog schema field %q=%+v want=%+v", path, got, expected)
		}
	}
}

func TestSpecificationCatalogPublishesFiniteAuthoringGrammar(t *testing.T) {
	for _, path := range []string{"spec init", "spec validate"} {
		t.Run(strings.ReplaceAll(path, " ", "_"), func(t *testing.T) {
			spec, found := DefaultCatalog().Lookup(path)
			if !found {
				t.Fatalf("%s is missing from the catalog", path)
			}
			var specification OutputField
			for _, field := range spec.Agent.Output.Fields {
				if field.Name == "specification" {
					specification = field
					break
				}
			}
			schema := specification.Schema
			if specification.Type != OutputFieldTypeObject || schema == nil || schema.ID != "tailoring-specification" || schema.Version != tailoringbundle.SpecificationSchemaVersion {
				t.Fatalf("%s specification field=%+v", path, specification)
			}
			paths := make(map[string]OutputSchemaField, len(schema.Fields))
			for _, field := range schema.Fields {
				paths[field.Path] = field
			}
			for schemaPath, fieldType := range map[string]OutputFieldType{
				"/surface/default":                                           OutputFieldTypeString,
				"/commands/*/command":                                        OutputFieldTypeArray,
				"/commands/*/presence":                                       OutputFieldTypeString,
				"/commands/*/options/default":                                OutputFieldTypeString,
				"/commands/*/wrapper/kind":                                   OutputFieldTypeString,
				"/commands/*/wrapper/invoke/append_args":                     OutputFieldTypeArray,
				"/commands/*/wrapper/output/kind":                            OutputFieldTypeString,
				"/commands/*/wrapper/output/projection":                      OutputFieldTypeObject,
				"/commands/*/wrapper/output/projection/input":                OutputFieldTypeString,
				"/commands/*/wrapper/output/projection/select":               OutputFieldTypeArray,
				"/commands/*/wrapper/output/projection/rename/*/from":        OutputFieldTypeString,
				"/commands/*/wrapper/output/projection/rename/*/to":          OutputFieldTypeString,
				"/commands/*/wrapper/output/projection/render":               OutputFieldTypeString,
				"/commands/*/wrapper/output/optimizer":                       OutputFieldTypeObject,
				"/commands/*/wrapper/output/optimizer/input":                 OutputFieldTypeString,
				"/commands/*/wrapper/output/optimizer/contract":              OutputFieldTypeString,
				"/commands/*/wrapper/output/optimizer/allow_original_output": OutputFieldTypeBoolean,
			} {
				got, exists := paths[schemaPath]
				if !exists || got.Type != fieldType {
					t.Errorf("%s schema field %q=%+v", path, schemaPath, got)
				}
			}
			if paths["/commands/*/options"].Required || paths["/commands/*/wrapper"].Required || paths["/commands/*/wrapper/output"].Required ||
				paths["/commands/*/wrapper/output/projection"].Required || paths["/commands/*/wrapper/output/optimizer"].Required {
				t.Fatalf("%s conditional authoring fields=%+v %+v %+v", path, paths["/commands/*/options"], paths["/commands/*/wrapper"], paths["/commands/*/wrapper/output"])
			}
		})
	}

	init, _ := DefaultCatalog().Lookup("spec init")
	authoring := init.Summary + "\n" + init.Agent.Outcome + "\n" + strings.Join(init.Agent.Prerequisites, "\n")
	for _, want := range []string{
		"identity-wrapper authoring baseline",
		"not an executable transform",
		"kind=transform",
		"output.kind=projection",
		"output.projection.input=json",
		"output.projection.select",
		"output.projection.rename",
		"output.projection.render=compact_json",
		"Optimizers require a separately admitted finite contract and exact processor evidence",
		"Arbitrary shell, script, jq, plugin, external-transformer, and runtime-LLM actions are invalid",
	} {
		if !strings.Contains(authoring, want) {
			t.Errorf("spec init authoring contract lacks %q: %s", want, authoring)
		}
	}
}

func TestCatalogRejectsMalformedNestedOutputSchema(t *testing.T) {
	base := utilitySpec("inspect")
	base.Agent.Output.Fields[0] = OutputField{
		Name: "result", Type: OutputFieldTypeObject, Description: "Structured test result.",
		Schema: &OutputSchema{ID: "test-schema", Version: 1, Fields: []OutputSchemaField{
			{Path: "/values", Type: OutputFieldTypeArray, Required: true},
		}},
	}
	if err := NewCatalog(base).Validate(); err == nil || !strings.Contains(err.Error(), "element type") {
		t.Fatalf("missing array element type error=%v", err)
	}
	base.Agent.Output.Fields[0].Schema.Fields = []OutputSchemaField{
		{Path: "/z", Type: OutputFieldTypeString, Required: true},
		{Path: "/a", Type: OutputFieldTypeString, Required: true},
	}
	if err := NewCatalog(base).Validate(); err == nil || !strings.Contains(err.Error(), "sorted and unique") {
		t.Fatalf("unsorted schema fields error=%v", err)
	}
}

func TestInputRelationsMustLeaveEveryDeclaredInputUsable(t *testing.T) {
	base := utilitySpec("inspect")
	base.Args = "[--a <value>] [--b <value>] [--c <value>]"
	base.Agent.Inputs = []CommandInput{
		{Name: "--a", Source: InputSourceFlag, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Optional A.", AllowedValues: []string{}},
		{Name: "--b", Source: InputSourceFlag, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Optional B.", AllowedValues: []string{}},
		{Name: "--c", Source: InputSourceFlag, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Optional C.", AllowedValues: []string{}},
	}
	if err := NewCatalog(base).Validate(); err != nil {
		t.Fatalf("valid independent optional inputs: %v", err)
	}

	optionalConflictsRequired := cloneCommandSpec(base)
	optionalConflictsRequired.Args = "--b <value> [--a <value>] [--c <value>]"
	optionalConflictsRequired.Agent.Inputs[1].Required = true
	optionalConflictsRequired.Agent.Inputs[0].ConflictsWith = []string{"--b"}
	if err := NewCatalog(optionalConflictsRequired).Validate(); err == nil || !strings.Contains(err.Error(), "unusable") {
		t.Fatalf("optional input conflicting with required input error = %v", err)
	}

	requiredRequiresOptional := cloneCommandSpec(base)
	requiredRequiresOptional.Args = "--a <value> [--b <value>] [--c <value>]"
	requiredRequiresOptional.Agent.Inputs[0].Required = true
	requiredRequiresOptional.Agent.Inputs[0].Requires = []string{"--b"}
	if err := NewCatalog(requiredRequiresOptional).Validate(); err == nil || !strings.Contains(err.Error(), "effectively mandatory") {
		t.Fatalf("required input requiring optional input error = %v", err)
	}

	transitiveConflict := cloneCommandSpec(base)
	transitiveConflict.Agent.Inputs[0].Requires = []string{"--b"}
	transitiveConflict.Agent.Inputs[1].Requires = []string{"--c"}
	transitiveConflict.Agent.Inputs[0].ConflictsWith = []string{"--c"}
	if err := NewCatalog(transitiveConflict).Validate(); err == nil || !strings.Contains(err.Error(), "unusable") {
		t.Fatalf("transitive dependency conflict error = %v", err)
	}
}

func TestAgentContractValidationFailsClosed(t *testing.T) {
	tests := map[string]func(*CommandSpec){
		"missing capability": func(spec *CommandSpec) { spec.Agent.CapabilityID = "" },
		"missing outcome":    func(spec *CommandSpec) { spec.Agent.Outcome = "" },
		"unknown inputs":     func(spec *CommandSpec) { spec.Agent.Inputs = nil },
		"unknown input source": func(spec *CommandSpec) {
			spec.Args = "--id <item-id>"
			spec.Agent.Inputs = []CommandInput{{Name: "--id", Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Item ID.", AllowedValues: []string{}}}
		},
		"undocumented argument": func(spec *CommandSpec) {
			spec.Args = "--id <item-id>"
		},
		"input absent from syntax": func(spec *CommandSpec) {
			spec.Agent.Inputs = []CommandInput{{Name: "--id", Source: InputSourceFlag, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Item ID.", AllowedValues: []string{}}}
		},
		"missing input description": func(spec *CommandSpec) {
			spec.Args = "--id <item-id>"
			spec.Agent.Inputs = []CommandInput{{Name: "--id", Source: InputSourceFlag, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, AllowedValues: []string{}}}
		},
		"unknown allowed values": func(spec *CommandSpec) {
			spec.Args = "--id <item-id>"
			spec.Agent.Inputs = []CommandInput{{Name: "--id", Source: InputSourceFlag, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Item ID."}}
		},
		"unknown formats":        func(spec *CommandSpec) { spec.Agent.Output.Formats = nil },
		"unknown default format": func(spec *CommandSpec) { spec.Agent.Output.DefaultFormat = OutputFormatUnknown },
		"unknown fields":         func(spec *CommandSpec) { spec.Agent.Output.Fields = nil },
		"missing field description": func(spec *CommandSpec) {
			spec.Agent.Output.Fields[0].Description = ""
		},
		"unknown delivery": func(spec *CommandSpec) {
			spec.Agent.Output.Delivery = OutputDeliveryUnknown
		},
		"unknown collection coverage": func(spec *CommandSpec) {
			spec.Agent.Output.CollectionCoverage = CollectionCoverageUnknown
		},
		"unknown prerequisites": func(spec *CommandSpec) { spec.Agent.Prerequisites = nil },
		"unknown errors":        func(spec *CommandSpec) { spec.Agent.Errors = nil },
		"missing next action":   func(spec *CommandSpec) { spec.Agent.Errors[0].NextActions = nil },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			spec := utilitySpec("test")
			mutate(&spec)
			if err := NewCatalog(spec).Validate(); err == nil {
				t.Fatal("incomplete agent contract passed validation")
			}
		})
	}
}

func TestCatalogMatchUsesLongestDeclarativePath(t *testing.T) {
	catalog := NewCatalog(utilitySpec("items"), utilitySpec("items list"))
	command, rest, found := catalog.Match([]string{"items", "list", "--limit", "2"})
	if !found {
		t.Fatal("Match() did not find a command")
	}
	if command.Path != "items list" {
		t.Fatalf("Match() path = %q, want items list", command.Path)
	}
	if got := strings.Join(rest, " "); got != "--limit 2" {
		t.Fatalf("Match() rest = %q", got)
	}
}

func TestCatalogEnforcesRoleAndReferenceFlowContracts(t *testing.T) {
	discover := discoverSpec("items list", "item")
	act := actSpec("items read", "item", "--id")
	if err := NewCatalog(discover, act).Validate(); err != nil {
		t.Fatalf("valid reference flow: %v", err)
	}

	utilityWithRef := discoverSpec("utility", "item")
	utilityWithRef.Role = RoleUtility
	mutatingDiscovery := discoverSpec("items list", "item")
	mutatingDiscovery.Effect = operation.EffectWrite
	emptyDiscovery := utilitySpec("items list")
	emptyDiscovery.Role = RoleDiscover
	emptyAct := utilitySpec("items read")
	emptyAct.Role = RoleAct
	optionalAct := actSpec("items inspect", "item", "--id")
	optionalAct.Args = "[--id <item-id>]"
	optionalAct.Agent.Inputs[0].Required = false
	invalidProducer := discoverSpec("items list", "Item")
	invalidConsumer := actSpec("items read", "Item", "--id")

	invalid := []Catalog{
		NewCatalog(utilityWithRef, act),
		NewCatalog(mutatingDiscovery, act),
		NewCatalog(emptyDiscovery),
		NewCatalog(emptyAct),
		NewCatalog(discover, optionalAct),
		NewCatalog(discover),
		NewCatalog(act),
		NewCatalog(invalidProducer, act),
		NewCatalog(discover, invalidConsumer),
	}
	for index, catalog := range invalid {
		if err := catalog.Validate(); err == nil {
			t.Errorf("invalid role/reference catalog %d passed validation", index)
		}
	}
}

func TestCatalogValidatesCommandBoundToolLocalFixedTargets(t *testing.T) {
	valid := fixedTargetActSpec("auth status")
	if err := NewCatalog(valid).Validate(); err != nil {
		t.Fatalf("valid fixed target: %v", err)
	}

	for name, mutate := range map[string]func(*CommandSpec){
		"missing kind":        func(spec *CommandSpec) { spec.Agent.FixedTarget.Kind = "" },
		"missing ID":          func(spec *CommandSpec) { spec.Agent.FixedTarget.ID = "" },
		"missing description": func(spec *CommandSpec) { spec.Agent.FixedTarget.Description = "" },
		"missing scope":       func(spec *CommandSpec) { spec.Agent.FixedTarget.Scope = FixedTargetScopeUnknown },
		"wrong scope":         func(spec *CommandSpec) { spec.Agent.FixedTarget.Scope = "provider" },
		"non-act role":        func(spec *CommandSpec) { spec.Role = RoleUtility },
		"consumed reference": func(spec *CommandSpec) {
			spec.Args = "--id <auth-config-id>"
			spec.Agent.Inputs = []CommandInput{{Name: "--id", Source: InputSourceFlag, Required: true, ValueKind: InputValueText, Cardinality: InputCardinalitySingle, Description: "Opaque config ID.", AllowedValues: []string{}, ReferenceKind: "auth-config"}}
		},
		"produced reference": func(spec *CommandSpec) {
			spec.Agent.Output.Fields[0].ReferenceKind = "auth-config"
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneCommandSpec(valid)
			mutate(&candidate)
			if err := NewCatalog(candidate).Validate(); err == nil {
				t.Fatal("invalid fixed target passed validation")
			}
		})
	}

	clone := cloneCommandSpec(valid)
	clone.Agent.FixedTarget.ID = "changed"
	if valid.Agent.FixedTarget.ID != "selected" {
		t.Fatal("fixed target pointer was not deep-copied")
	}
	encoded, err := json.Marshal(valid.Agent)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"kind":"auth-config"`, `"id":"selected"`, `"scope":"tool_local"`, `"description":"This CLI installation's selected authentication configuration."`} {
		if !bytes.Contains(encoded, []byte(want)) {
			t.Errorf("fixed target JSON lacks %s: %s", want, encoded)
		}
	}
}

func TestFixedTargetMutationPreservesMutationSafetyContract(t *testing.T) {
	status := fixedTargetActSpec("auth status")
	write := fixedTargetActSpec("auth reset")
	write.Effect = operation.EffectWrite
	write.Agent.Errors = mutationErrors(write.Agent.Errors, write.Path)
	for index := range write.Agent.Errors {
		if write.Agent.Errors[index].Code == "unclassified_mutation_outcome" ||
			write.Agent.Errors[index].Code == "mutation_output_write_failed" {
			write.Agent.Errors[index].NextActions[0].Command = status.Path
		}
	}
	write.Agent.Mutation = &MutationContract{
		TargetKind: "auth-config", TargetInputs: []string{},
		Impact: operation.Impact{Cardinality: operation.CardinalityOne, Notification: operation.DeclarationNo, AccessChange: operation.DeclarationNo, Destructive: operation.DeclarationYes},
	}
	if err := NewCatalog(status, write).Validate(); err != nil {
		t.Fatalf("valid fixed-target mutation: %v", err)
	}

	for name, mutate := range map[string]func(*CommandSpec){
		"target kind mismatch": func(spec *CommandSpec) { spec.Agent.Mutation.TargetKind = "other" },
		"nil target inputs":    func(spec *CommandSpec) { spec.Agent.Mutation.TargetInputs = nil },
		"unknown target input": func(spec *CommandSpec) { spec.Agent.Mutation.TargetInputs = []string{"--missing"} },
		"nonempty target input": func(spec *CommandSpec) {
			spec.Agent.Mutation.TargetInputs = []string{"--id"}
		},
		"parent input":    func(spec *CommandSpec) { spec.Agent.Mutation.ParentInput = "--parent-id" },
		"target ID input": func(spec *CommandSpec) { spec.Agent.Mutation.TargetIDInput = "--id" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneCommandSpec(write)
			mutate(&candidate)
			if err := validateAgentContract(candidate); err == nil {
				t.Fatal("invalid fixed-target mutation passed validation")
			}
		})
	}

	create := cloneCommandSpec(write)
	create.Path = "auth initialize"
	create.Effect = operation.EffectCreate
	if err := validateAgentContract(create); err != nil {
		t.Fatalf("fixed target as create scope: %v", err)
	}
}

func TestReferenceGraphRejectsClosedCyclesAndAcceptsReachableChains(t *testing.T) {
	selfCycle := actSpec("items rotate", "item", "--id")
	selfCycle.Agent.Output.Fields[0] = OutputField{
		Name: "id", Type: OutputFieldTypeString, Description: "Rotated item ID.", ReferenceKind: "item",
	}
	if err := NewCatalog(selfCycle).Validate(); err == nil || !strings.Contains(err.Error(), "closed required-reference cycle") {
		t.Fatalf("self-contained reference cycle error = %v", err)
	}

	alpha := actSpec("alpha derive", "beta", "--beta-id")
	alpha.Agent.Output.Fields[0] = OutputField{
		Name: "alpha_id", Type: OutputFieldTypeString, Description: "Derived alpha ID.", ReferenceKind: "alpha",
	}
	beta := actSpec("beta derive", "alpha", "--alpha-id")
	beta.Agent.Output.Fields[0] = OutputField{
		Name: "beta_id", Type: OutputFieldTypeString, Description: "Derived beta ID.", ReferenceKind: "beta",
	}
	if err := NewCatalog(alpha, beta).Validate(); err == nil || !strings.Contains(err.Error(), "closed required-reference cycle") {
		t.Fatalf("multi-kind reference cycle error = %v", err)
	}

	workspaces := discoverSpec("workspaces list", "workspace")
	items := discoverSpec("items list", "item")
	items.Args = "--workspace-id <workspace-id>"
	items.Agent.Inputs = []CommandInput{{
		Name: "--workspace-id", Source: InputSourceFlag, Required: true,
		ValueKind: InputValueText, Cardinality: InputCardinalitySingle,
		Description: "Opaque workspace ID.", AllowedValues: []string{}, ReferenceKind: "workspace",
	}}
	read := actSpec("items read", "item", "--id")
	if err := NewCatalog(workspaces, items, read).Validate(); err != nil {
		t.Fatalf("reachable reference chain failed validation: %v", err)
	}
}

func TestReferenceGraphAllowsMultipleInputsOfTheSameKind(t *testing.T) {
	discover := discoverSpec("items list", "item")
	act := actSpec("items compare", "item", "--left-id", "--right-id")
	catalog := NewCatalog(discover, act)
	if err := catalog.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	consumed := act.ConsumedRefs()
	if len(consumed) != 2 || consumed[0].Argument != "--left-id" || consumed[1].Argument != "--right-id" {
		t.Fatalf("ConsumedRefs() = %+v", consumed)
	}
	workflows := catalog.referenceWorkflows()
	if len(workflows) != 1 || len(workflows[0].Producers) != 1 || len(workflows[0].Consumers) != 2 ||
		workflows[0].Consumers[0].Input != "--left-id" || workflows[0].Consumers[1].Input != "--right-id" {
		t.Fatalf("reference workflows = %+v, want one grouped kind with both inputs", workflows)
	}
}

func TestInvalidCatalogFailsBeforeDispatch(t *testing.T) {
	called := false
	bad := utilitySpec("unsafe")
	bad.Effect = operation.EffectUnknown
	bad.handler = func(context.Context, *CLI, CommandSpec, operation.Intent, ParsedInputs) int {
		called = true
		return ExitOK
	}
	var stdout, stderr bytes.Buffer
	command := newCLI(strings.NewReader(""), &stdout, &stderr, NewCatalog(bad), nil)
	if code := runCLI(command, []string{"unsafe"}); code != ExitContract {
		t.Fatalf("Run() code = %d, want %d", code, ExitContract)
	}
	if called {
		t.Fatal("handler ran for an invalid catalog")
	}
	if !strings.Contains(stderr.String(), "code: invalid_catalog") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestCatalogCommandsReturnsDeepCopy(t *testing.T) {
	catalog := DefaultCatalog()
	commands := catalog.Commands()
	commands[0].Path = "changed"
	commands[0].Agent.Outcome = "changed"
	commands[0].Agent.Output.Formats[0] = OutputFormatNone
	commands[0].Agent.Output.Fields[0].Name = "changed"
	commands[0].Agent.Inputs[0].AllowedValues[0] = "changed"
	commands[0].Agent.Prerequisites = append(commands[0].Agent.Prerequisites, "changed")
	commands[0].Agent.Errors[0].Code = "changed"
	commands[0].Agent.Errors[0].NextActions[0].Command = "changed"

	doctor, found := catalog.Lookup("doctor")
	if !found {
		t.Fatal("mutating Commands() changed the catalog")
	}
	want, _ := DefaultCatalog().Lookup("doctor")
	if !reflect.DeepEqual(doctor.Agent, want.Agent) {
		t.Fatalf("nested agent contract was mutated: %+v", doctor.Agent)
	}
}

func TestMutationContractFailsClosedAndDeepCopies(t *testing.T) {
	spec := utilitySpec("items update")
	spec.Effect = operation.EffectWrite
	spec.Role = RoleAct
	spec.Agent.Errors = mutationErrors(spec.Agent.Errors, spec.Path)
	spec.Args = "--id <item-id>"
	spec.Agent.Inputs = []CommandInput{{
		Name: "--id", Source: InputSourceFlag, Required: true,
		ValueKind: InputValueText, Cardinality: InputCardinalitySingle,
		Description: "Target item ID.", AllowedValues: []string{}, ReferenceKind: "item",
	}}
	spec.Agent.Mutation = &MutationContract{
		TargetKind: "item", TargetInputs: []string{"--id"}, TargetIDInput: "--id",
		Impact: operation.Impact{
			Cardinality: operation.CardinalityOne, Notification: operation.DeclarationNo,
			AccessChange: operation.DeclarationNo, Destructive: operation.DeclarationNo,
		},
	}
	if err := validateAgentContract(spec); err != nil {
		t.Fatalf("valid mutation contract: %v", err)
	}
	if err := NewCatalog(discoverSpec("items list", "item"), spec).Validate(); err != nil {
		t.Fatalf("valid act mutation catalog: %v", err)
	}
	withReadOutputFailure := cloneCommandSpec(spec)
	withReadOutputFailure.Agent.Errors = append(withReadOutputFailure.Agent.Errors, declaredCommandError(
		fault.KindInternal,
		"output_write_failed",
		true,
		withReadOutputFailure.Path,
		"Retry the mutation with a writable output stream.",
	))
	if err := NewCatalog(discoverSpec("items list", "item"), withReadOutputFailure).Validate(); err == nil ||
		!strings.Contains(err.Error(), "must not declare retryable output_write_failed") {
		t.Fatalf("mutation with read output failure error = %v", err)
	}
	for _, code := range []string{"unclassified_mutation_outcome", "mutation_output_write_failed"} {
		unsafeRecovery := cloneCommandSpec(spec)
		for index := range unsafeRecovery.Agent.Errors {
			if unsafeRecovery.Agent.Errors[index].Code == code {
				unsafeRecovery.Agent.Errors[index].NextActions[0].Command = unsafeRecovery.Path
			}
		}
		if err := NewCatalog(discoverSpec("items list", "item"), unsafeRecovery).Validate(); err == nil ||
			!strings.Contains(err.Error(), "read-only reconciliation") {
			t.Fatalf("unsafe %s recovery error = %v", code, err)
		}
	}

	rateLimited := cloneCommandSpec(spec)
	rateLimited.Agent.Errors = append(rateLimited.Agent.Errors, declaredCommandError(
		fault.KindRateLimited,
		"mutation_rate_limited",
		false,
		rateLimited.Path,
		"Wait for the provider window, then reconcile before deciding on another mutation.",
	))
	if err := NewCatalog(discoverSpec("items list", "item"), rateLimited).Validate(); err == nil ||
		!strings.Contains(err.Error(), "read-only reconciliation") {
		t.Fatalf("unsafe non-retryable rate-limit recovery error = %v", err)
	}
	for index := range rateLimited.Agent.Errors {
		if rateLimited.Agent.Errors[index].Code == "mutation_rate_limited" {
			rateLimited.Agent.Errors[index].NextActions[0].Command = "items list"
		}
	}
	if err := NewCatalog(discoverSpec("items list", "item"), rateLimited).Validate(); err != nil {
		t.Fatalf("read-only rate-limit recovery rejected: %v", err)
	}

	missing := cloneCommandSpec(spec)
	missing.Agent.Mutation = nil
	if err := validateAgentContract(missing); err == nil {
		t.Fatal("mutation without declaration passed")
	}
	wrongInput := cloneCommandSpec(spec)
	wrongInput.Agent.Mutation.TargetInputs[0] = "--missing"
	if err := validateAgentContract(wrongInput); err == nil {
		t.Fatal("mutation with unknown target input passed")
	}
	clone := cloneCommandSpec(spec)
	clone.Agent.Mutation.TargetInputs[0] = "changed"
	if spec.Agent.Mutation.TargetInputs[0] != "--id" {
		t.Fatal("mutation target inputs share storage")
	}
	missingTargetBinding := cloneCommandSpec(spec)
	missingTargetBinding.Agent.Mutation.TargetIDInput = ""
	if err := validateAgentContract(missingTargetBinding); err == nil {
		t.Fatal("write mutation without target ID binding passed")
	}
	mismatchedTargetKind := cloneCommandSpec(spec)
	mismatchedTargetKind.Agent.Mutation.TargetKind = "other"
	if err := validateAgentContract(mismatchedTargetKind); err == nil {
		t.Fatal("write mutation with mismatched target reference kind passed")
	}
	optionalTarget := cloneCommandSpec(spec)
	optionalTarget.Args = "[--id <item-id>]"
	optionalTarget.Agent.Inputs[0].Required = false
	if err := validateAgentContract(optionalTarget); err == nil || !strings.Contains(err.Error(), "must be required") {
		t.Fatalf("optional mutation target error = %v", err)
	}
	configuredTarget := cloneCommandSpec(spec)
	configuredTarget.Args = ""
	configuredTarget.Agent.Inputs[0].Name = "id"
	configuredTarget.Agent.Inputs[0].Source = InputSourceConfiguration
	configuredTarget.Agent.Mutation.TargetInputs[0] = "id"
	configuredTarget.Agent.Mutation.TargetIDInput = "id"
	if err := validateAgentContract(configuredTarget); err == nil || !strings.Contains(err.Error(), "command argument or flag") {
		t.Fatalf("non-CLI mutation target error = %v", err)
	}
	withParent := cloneCommandSpec(spec)
	withParent.Args += " --collection-id <collection-id>"
	withParent.Agent.Inputs = append(withParent.Agent.Inputs, CommandInput{
		Name: "--collection-id", Source: InputSourceFlag, Required: true,
		ValueKind: InputValueText, Cardinality: InputCardinalitySingle,
		Description: "Parent collection ID.", AllowedValues: []string{}, ReferenceKind: "collection",
	})
	withParent.Agent.Mutation.ParentInput = "--collection-id"
	withParent.Agent.Mutation.TargetInputs = append(withParent.Agent.Mutation.TargetInputs, "--collection-id")
	if err := validateAgentContract(withParent); err != nil {
		t.Fatalf("write mutation with parent binding: %v", err)
	}
	ambiguousTargets := cloneCommandSpec(withParent)
	ambiguousTargets.Args += " --scope-id <scope-id>"
	ambiguousTargets.Agent.Inputs = append(ambiguousTargets.Agent.Inputs, CommandInput{
		Name: "--scope-id", Source: InputSourceFlag, Required: true,
		ValueKind: InputValueText, Cardinality: InputCardinalitySingle,
		Description: "Unbound scope ID.", AllowedValues: []string{}, ReferenceKind: "scope",
	})
	ambiguousTargets.Agent.Mutation.TargetInputs = append(ambiguousTargets.Agent.Mutation.TargetInputs, "--scope-id")
	if err := validateAgentContract(ambiguousTargets); err == nil {
		t.Fatal("write mutation with an unbound target input passed")
	}
	encoded, err := json.Marshal(spec.Agent)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"target_id_input":"--id"`, `"cardinality":"one"`, `"notification":"no"`, `"access_change":"no"`, `"destructive":"no"`} {
		if !bytes.Contains(encoded, []byte(want)) {
			t.Errorf("mutation JSON lacks %s: %s", want, encoded)
		}
	}
}

func TestMutationContractRequiresInvokerFailureSurface(t *testing.T) {
	spec := utilitySpec("items update")
	spec.Effect = operation.EffectWrite
	spec.Role = RoleAct
	spec.Agent.Errors = mutationErrors(spec.Agent.Errors, spec.Path)
	spec.Args = "--id <item-id>"
	spec.Agent.Inputs = []CommandInput{{
		Name: "--id", Source: InputSourceFlag, Required: true,
		ValueKind: InputValueText, Cardinality: InputCardinalitySingle,
		Description: "Target item ID.", AllowedValues: []string{}, ReferenceKind: "item",
	}}
	spec.Agent.Mutation = &MutationContract{
		TargetKind: "item", TargetInputs: []string{"--id"}, TargetIDInput: "--id",
		Impact: operation.Impact{
			Cardinality: operation.CardinalityOne, Notification: operation.DeclarationNo,
			AccessChange: operation.DeclarationNo, Destructive: operation.DeclarationNo,
		},
	}
	if err := validateAgentContract(spec); err != nil {
		t.Fatalf("valid mutation failure surface: %v", err)
	}
	for _, missing := range []string{"invalid_mutation_contract", "missing_mutation_action", "missing_mutation_policy", "mutation_rejected", "unclassified_mutation_outcome", "mutation_output_write_failed"} {
		t.Run(missing, func(t *testing.T) {
			candidate := cloneCommandSpec(spec)
			filtered := make([]CommandError, 0, len(candidate.Agent.Errors)-1)
			for _, declared := range candidate.Agent.Errors {
				if declared.Code != missing {
					filtered = append(filtered, declared)
				}
			}
			candidate.Agent.Errors = filtered
			if err := validateAgentContract(candidate); err == nil {
				t.Fatalf("mutation without %q passed", missing)
			}
		})
	}
}

func TestCreateMutationBindsOpaqueParentOnly(t *testing.T) {
	spec := utilitySpec("items create")
	spec.Effect = operation.EffectCreate
	spec.Role = RoleAct
	spec.Agent.Errors = mutationErrors(spec.Agent.Errors, spec.Path)
	spec.Args = "--collection-id <collection-id>"
	spec.Agent.Inputs = []CommandInput{{
		Name: "--collection-id", Source: InputSourceFlag, Required: true,
		ValueKind: InputValueText, Cardinality: InputCardinalitySingle,
		Description: "Parent collection ID.", AllowedValues: []string{}, ReferenceKind: "collection",
	}}
	spec.Agent.Mutation = &MutationContract{
		TargetKind: "item", TargetInputs: []string{"--collection-id"}, ParentInput: "--collection-id",
		Impact: operation.Impact{
			Cardinality: operation.CardinalityOne, Notification: operation.DeclarationNo,
			AccessChange: operation.DeclarationNo, Destructive: operation.DeclarationNo,
		},
	}
	if err := validateAgentContract(spec); err != nil {
		t.Fatalf("valid create mutation: %v", err)
	}

	missingParent := cloneCommandSpec(spec)
	missingParent.Agent.Mutation.ParentInput = ""
	if err := validateAgentContract(missingParent); err == nil {
		t.Fatal("create mutation without parent binding passed")
	}
	withTargetID := cloneCommandSpec(spec)
	withTargetID.Agent.Mutation.TargetIDInput = "--collection-id"
	if err := validateAgentContract(withTargetID); err == nil {
		t.Fatal("create mutation with an existing target ID passed")
	}
	parentOutsideTargets := cloneCommandSpec(spec)
	parentOutsideTargets.Agent.Mutation.TargetInputs = []string{"--missing"}
	if err := validateAgentContract(parentOutsideTargets); err == nil {
		t.Fatal("create mutation with unbound parent passed")
	}
}

func TestAuthenticationRequirementRequiresGateFailureSurfaceAndDeepCopies(t *testing.T) {
	spec := utilitySpec("items read")
	spec.Agent.Authentication = &authn.Requirement{
		Methods: []authn.Method{authn.MethodOAuth2, authn.MethodPAT}, Authority: "example",
		Audience: "items", RequiredCapabilities: []string{"items.read"},
	}
	if err := validateAgentContract(spec); err == nil {
		t.Fatal("authenticated command without gate errors passed")
	}
	required := authenticationGateRuntimeErrors(spec.Path)
	spec.Agent.Errors = append(spec.Agent.Errors, required...)
	if err := validateAgentContract(spec); err != nil {
		t.Fatalf("valid authenticated contract: %v", err)
	}
	for _, contract := range required {
		contract := contract
		t.Run("missing_"+contract.Code, func(t *testing.T) {
			candidate := cloneCommandSpec(spec)
			filtered := make([]CommandError, 0, len(candidate.Agent.Errors)-1)
			for _, declared := range candidate.Agent.Errors {
				if declared.Code != contract.Code {
					filtered = append(filtered, declared)
				}
			}
			candidate.Agent.Errors = filtered
			if err := validateAgentContract(candidate); err == nil || !strings.Contains(err.Error(), contract.Code) {
				t.Fatalf("authenticated command without %q error = %v", contract.Code, err)
			}
		})
		t.Run("wrong_kind_"+contract.Code, func(t *testing.T) {
			candidate := cloneCommandSpec(spec)
			for index := range candidate.Agent.Errors {
				if candidate.Agent.Errors[index].Code == contract.Code {
					candidate.Agent.Errors[index].Kind = fault.KindInternal
					if contract.Kind == fault.KindInternal {
						candidate.Agent.Errors[index].Kind = fault.KindContract
					}
				}
			}
			if err := validateAgentContract(candidate); err == nil || !strings.Contains(err.Error(), contract.Code) {
				t.Fatalf("authenticated command with wrong %q kind error = %v", contract.Code, err)
			}
		})
		t.Run("wrong_retryability_"+contract.Code, func(t *testing.T) {
			candidate := cloneCommandSpec(spec)
			for index := range candidate.Agent.Errors {
				if candidate.Agent.Errors[index].Code == contract.Code {
					candidate.Agent.Errors[index].Retryable = !contract.Retryable
				}
			}
			if err := validateAgentContract(candidate); err == nil || !strings.Contains(err.Error(), contract.Code) {
				t.Fatalf("authenticated command with wrong %q retryability error = %v", contract.Code, err)
			}
		})
	}
	withProviderFault := cloneCommandSpec(spec)
	withProviderFault.Agent.Errors = append(withProviderFault.Agent.Errors,
		declaredCommandError(fault.KindRateLimited, "identity_rate_limited", true, spec.Path, "Retry after the provider delay."),
	)
	if err := validateAgentContract(withProviderFault); err != nil {
		t.Fatalf("provider-specific authentication fault: %v", err)
	}
	clone := cloneCommandSpec(spec)
	clone.Agent.Authentication.Methods[0] = authn.MethodPAT
	clone.Agent.Authentication.RequiredCapabilities[0] = "changed"
	if spec.Agent.Authentication.Methods[0] != authn.MethodOAuth2 || spec.Agent.Authentication.RequiredCapabilities[0] != "items.read" {
		t.Fatal("authentication requirement shares slice storage")
	}
}

func TestCatalogValidatesExecutableRecoveryCommandGrammar(t *testing.T) {
	help, found := DefaultCatalog().Lookup("help")
	if !found {
		t.Fatal("default catalog lacks help")
	}
	sampleList := utilitySpec("sample list")
	for _, action := range []string{"help", "sample list", "help sample", "help sample list"} {
		t.Run("valid_"+strings.ReplaceAll(action, " ", "_"), func(t *testing.T) {
			spec := utilitySpec("test")
			spec.Agent.Errors[0].NextActions[0].Command = action
			if err := NewCatalog(help, sampleList, spec).Validate(); err != nil {
				t.Fatalf("valid recovery command %q: %v", action, err)
			}
		})
	}

	for _, action := range []string{
		"missing command",
		"sample list --bogus",
		"help nonexistent",
		"help sample list extra",
		"help sample --format agent",
		"sample  list",
	} {
		t.Run("invalid_"+strings.ReplaceAll(action, " ", "_"), func(t *testing.T) {
			spec := utilitySpec("test")
			spec.Agent.Errors[0].NextActions[0].Command = action
			if err := NewCatalog(help, sampleList, spec).Validate(); err == nil {
				t.Fatalf("invalid recovery command %q passed catalog validation", action)
			}
		})
	}
}
