package githubcli

import (
	"errors"
	"regexp"
	"strings"

	"github.com/tasuku43/atsura/internal/domain/runtimeadmission"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

var (
	// ErrRuntimeUnsupported means this adapter contract cannot prove the plan's
	// source, command, or output mode without another inspection contract.
	ErrRuntimeUnsupported = errors.New("github cli runtime is not supported")
	// ErrRuntimeSelector means the plan does not carry the one exact selector
	// value required to make the declared output transform truthful.
	ErrRuntimeSelector = errors.New("github cli output selector is not proven")

	// ErrRuntimeAdapterContract classifies a source adapter kind or inspection
	// contract that this runtime does not admit.
	ErrRuntimeAdapterContract = errors.New("github cli adapter contract is not admitted")
	// ErrRuntimeSourceVersion classifies source version evidence outside the
	// maintained runtime range.
	ErrRuntimeSourceVersion = errors.New("github cli source version is not admitted")
	// ErrRuntimeCommand classifies a source command outside the finite runtime
	// command set.
	ErrRuntimeCommand = errors.New("github cli command is not admitted")
	// ErrRuntimeWrapperOutput classifies a wrapper or output stage outside the
	// typed JSON transform boundary.
	ErrRuntimeWrapperOutput = errors.New("github cli wrapper output is not admitted")
	// ErrRuntimeArgvGrammar classifies source argv outside the finite grammar
	// maintained for an admitted command.
	ErrRuntimeArgvGrammar = errors.New("github cli argv grammar is not admitted")
	// ErrRuntimeSelectorConflict classifies a missing, competing, duplicate, or
	// mismatched JSON selector.
	ErrRuntimeSelectorConflict = errors.New("github cli output selector conflicts with the wrapper plan")
)

// runtimeAdmissionError keeps errors.Is compatibility with the original
// broad sentinels while exposing one finite, secret-free diagnostic category
// to the application boundary. It never retains source text or a parser cause.
type runtimeAdmissionError struct {
	legacy   error
	category error
	name     runtimeadmission.Category
}

func (e *runtimeAdmissionError) Error() string {
	return e.legacy.Error() + ": " + e.category.Error()
}

func (e *runtimeAdmissionError) Unwrap() []error {
	return []error{e.legacy, e.category}
}

func (e *runtimeAdmissionError) RuntimeAdmissionCategory() runtimeadmission.Category {
	return e.name
}

func admissionError(legacy, category error, name runtimeadmission.Category) error {
	return &runtimeAdmissionError{legacy: legacy, category: category, name: name}
}

var sourceVersionPattern = regexp.MustCompile(`^([0-9]+)\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)

// RuntimeVerifier is the zero-state GitHub CLI runtime proof adapter.
type RuntimeVerifier struct{}

// NewRuntimeVerifier creates a runtime proof adapter.
func NewRuntimeVerifier() *RuntimeVerifier { return &RuntimeVerifier{} }

// VerifyRuntime proves the same contract as the package function.
func (*RuntimeVerifier) VerifyRuntime(plan tailoringplan.Plan) error { return VerifyRuntime(plan) }

// VerifySurface proves that every command and option exposed by one compiled
// surface belongs to this finite runtime contract. It is stricter than
// VerifyRuntime because rendering one ordinary-command wrapper would otherwise
// advertise invocations that the runtime can only reject later.
func (*RuntimeVerifier) VerifySurface(bundle tailoringbundle.Bundle) error {
	return VerifySurface(bundle)
}

// VerifyRuntime proves that a plan produced from inspection contract 2 uses one
// maintained GitHub CLI command grammar and one admitted result contract. It
// performs no I/O and grants no source-operation permission.
func VerifyRuntime(plan tailoringplan.Plan) error {
	if err := plan.Validate(); err != nil {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}
	if err := verifySourceContract(plan.Source.AdapterKind, plan.Source.AdapterContractVersion, plan.Source.Version); err != nil {
		return err
	}
	path := strings.Join(plan.MatchedCommand, " ")
	if _, ok := runtimeArgContracts[path]; !ok {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeCommand, runtimeadmission.CategoryCommand)
	}
	output, present, err := plan.OutputPlan()
	if err != nil {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}
	want := ""
	wantMatches := 0
	switch plan.ResultMode {
	case tailoringplan.ResultModeTransformedJSON:
		if !present || plan.WrapperKind != tailoringbundle.WrapperTransform || output.Input != tailoring.InputJSON {
			return admissionError(ErrRuntimeUnsupported, ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
		}
		want = "--json=" + strings.Join(output.Select, ",")
		wantMatches = 1
	case tailoringplan.ResultModeSourceStreamPassthrough:
		if present || !admittedSourceStreamWrapper(plan.WrapperKind, plan.Stages.Before, plan.Stages.Invoke.AppendedArgs, plan.Stages.Output, plan.Stages.After) {
			return admissionError(ErrRuntimeUnsupported, ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
		}
	default:
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}
	matches, err := verifyRuntimeArgs(path, plan.Stages.Invoke.Args[len(plan.MatchedCommand):], want)
	if err != nil {
		return err
	}
	if matches != wantMatches {
		return admissionError(ErrRuntimeSelector, ErrRuntimeSelectorConflict, runtimeadmission.CategorySelectorConflict)
	}
	return nil
}

// VerifySurface proves that the complete included bundle surface can enter the
// maintained wrapper runtime. The initial materialization contract exposes
// exactly one command using either the admitted JSON transform or an identity /
// append-argv-only source-stream wrapper. Mixed or partially admitted surfaces
// are rejected before wrapper bytes are produced.
func VerifySurface(bundle tailoringbundle.Bundle) error {
	if err := bundle.Validate(); err != nil {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}
	if err := verifySourceContract(bundle.Catalog.Adapter.Kind, bundle.Catalog.Adapter.ContractVersion, bundle.Catalog.Source.Version); err != nil {
		return err
	}
	if len(bundle.Surface) != 1 {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}
	entry := bundle.Surface[0]
	path := strings.Join(entry.Command, " ")
	contract, ok := runtimeArgContracts[path]
	if !ok {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeCommand, runtimeadmission.CategoryCommand)
	}
	wantedSelector := ""
	wantedMatches := 0
	if entry.Wrapper.Output != nil {
		if entry.Wrapper.Kind != tailoringbundle.WrapperTransform || entry.Wrapper.Output.Input != string(tailoring.InputJSON) {
			return admissionError(ErrRuntimeUnsupported, ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
		}
		wantedSelector = "--json=" + strings.Join(entry.Wrapper.Output.Select, ",")
		wantedMatches = 1
	} else if !admittedSourceStreamWrapper(entry.Wrapper.Kind, entry.Wrapper.Before, entry.Wrapper.Invoke.AppendArgs, entry.Wrapper.Output, entry.Wrapper.After) {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}
	matches, err := verifyRuntimeArgs(path, entry.Wrapper.Invoke.AppendArgs, wantedSelector)
	if err != nil {
		return err
	}
	if matches != wantedMatches {
		return admissionError(ErrRuntimeSelector, ErrRuntimeSelectorConflict, runtimeadmission.CategorySelectorConflict)
	}

	command, found := catalogCommand(bundle.Catalog, entry.Command)
	if !found {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeCommand, runtimeadmission.CategoryCommand)
	}
	for _, option := range command.Options {
		if !surfaceOptionIncluded(entry.Options, option.Name) {
			continue
		}
		switch option.Name {
		case "--json", "--jq", "--template", "--web":
			return admissionError(ErrRuntimeSelector, ErrRuntimeSelectorConflict, runtimeadmission.CategorySelectorConflict)
		}
		if _, allowed := contract.values[option.Name]; allowed {
			if !option.TakesValue {
				return admissionError(ErrRuntimeUnsupported, ErrRuntimeArgvGrammar, runtimeadmission.CategoryArgvGrammar)
			}
			continue
		}
		if _, allowed := contract.booleans[option.Name]; allowed {
			if option.TakesValue {
				return admissionError(ErrRuntimeUnsupported, ErrRuntimeArgvGrammar, runtimeadmission.CategoryArgvGrammar)
			}
			continue
		}
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeArgvGrammar, runtimeadmission.CategoryArgvGrammar)
	}
	return nil
}

func admittedSourceStreamWrapper(kind tailoringbundle.WrapperKind, before []tailoringbundle.StageAction, appendArgs []string, output *tailoringbundle.Output, after []tailoringbundle.StageAction) bool {
	if len(before) != 0 || len(after) != 0 || output != nil {
		return false
	}
	switch kind {
	case tailoringbundle.WrapperIdentity:
		return len(appendArgs) == 0
	case tailoringbundle.WrapperTransform:
		return len(appendArgs) != 0
	default:
		return false
	}
}

func verifySourceContract(adapterKind string, adapterContract int, sourceVersion string) error {
	if adapterKind != AdapterKind || adapterContract != ContractVersion {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeAdapterContract, runtimeadmission.CategoryAdapterContract)
	}
	version := sourceVersionPattern.FindStringSubmatch(sourceVersion)
	if len(version) != 2 || version[1] != "2" {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeSourceVersion, runtimeadmission.CategorySourceVersion)
	}
	return nil
}

func catalogCommand(catalog sourcecatalog.Catalog, path []string) (sourcecatalog.Command, bool) {
	wanted := strings.Join(path, "\x00")
	for _, command := range catalog.Commands {
		if strings.Join(command.Path, "\x00") == wanted {
			return command, true
		}
	}
	return sourcecatalog.Command{}, false
}

func surfaceOptionIncluded(surface tailoringbundle.OptionSurface, name string) bool {
	values := surface.Include
	wantPresent := true
	if surface.Default == tailoringbundle.SurfaceDefaultInherit {
		values = surface.Exclude
		wantPresent = false
	}
	for _, value := range values {
		if value == name {
			return wantPresent
		}
	}
	return surface.Default == tailoringbundle.SurfaceDefaultInherit
}

type runtimeArgContract struct {
	values   map[string]struct{}
	booleans map[string]struct{}
}

var runtimeArgContracts = map[string]runtimeArgContract{
	"issue list": {
		values: stringSet("--app", "--assignee", "--author", "--label", "--limit", "--mention", "--milestone", "--repo", "--search", "--state"),
	},
	"pr list": {
		values:   stringSet("--app", "--assignee", "--author", "--base", "--head", "--label", "--limit", "--repo", "--search", "--state"),
		booleans: stringSet("--draft"),
	},
}

func verifyRuntimeArgs(path string, arguments []string, wantedSelector string) (int, error) {
	contract, ok := runtimeArgContracts[path]
	if !ok {
		return 0, admissionError(ErrRuntimeUnsupported, ErrRuntimeCommand, runtimeadmission.CategoryCommand)
	}
	matches := 0
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--" {
			if index != len(arguments)-1 {
				return 0, admissionError(ErrRuntimeUnsupported, ErrRuntimeArgvGrammar, runtimeadmission.CategoryArgvGrammar)
			}
			continue
		}
		name, value, inline := strings.Cut(argument, "=")
		switch name {
		case "--json":
			matches++
			if !inline || argument != wantedSelector {
				return 0, admissionError(ErrRuntimeSelector, ErrRuntimeSelectorConflict, runtimeadmission.CategorySelectorConflict)
			}
			continue
		case "--jq", "--template", "--web":
			return 0, admissionError(ErrRuntimeSelector, ErrRuntimeSelectorConflict, runtimeadmission.CategorySelectorConflict)
		}
		if _, allowed := contract.booleans[name]; allowed {
			if inline {
				return 0, admissionError(ErrRuntimeUnsupported, ErrRuntimeArgvGrammar, runtimeadmission.CategoryArgvGrammar)
			}
			continue
		}
		if _, allowed := contract.values[name]; !allowed {
			return 0, admissionError(ErrRuntimeUnsupported, ErrRuntimeArgvGrammar, runtimeadmission.CategoryArgvGrammar)
		}
		if inline {
			if value == "" {
				return 0, admissionError(ErrRuntimeUnsupported, ErrRuntimeArgvGrammar, runtimeadmission.CategoryArgvGrammar)
			}
			continue
		}
		if index+1 >= len(arguments) || strings.HasPrefix(arguments[index+1], "-") {
			return 0, admissionError(ErrRuntimeUnsupported, ErrRuntimeArgvGrammar, runtimeadmission.CategoryArgvGrammar)
		}
		index++
	}
	return matches, nil
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
