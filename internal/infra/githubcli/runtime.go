package githubcli

import (
	"errors"
	"regexp"
	"strings"

	"github.com/tasuku43/atsura/internal/domain/runtimeadmission"
	"github.com/tasuku43/atsura/internal/domain/tailoring"
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

// VerifyRuntime proves that a plan produced from inspection contract 2 asks a
// supported GitHub CLI command for the exact selected JSON fields. It performs
// no I/O and grants no source-operation permission.
func VerifyRuntime(plan tailoringplan.Plan) error {
	if err := plan.Validate(); err != nil {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}
	if plan.Source.AdapterKind != AdapterKind || plan.Source.AdapterContractVersion != ContractVersion {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeAdapterContract, runtimeadmission.CategoryAdapterContract)
	}
	version := sourceVersionPattern.FindStringSubmatch(plan.Source.Version)
	if len(version) != 2 || version[1] != "2" {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeSourceVersion, runtimeadmission.CategorySourceVersion)
	}
	path := strings.Join(plan.MatchedCommand, " ")
	if path != "issue list" && path != "pr list" {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeCommand, runtimeadmission.CategoryCommand)
	}
	output, present, err := plan.OutputPlan()
	if err != nil || !present || output.Input != tailoring.InputJSON {
		return admissionError(ErrRuntimeUnsupported, ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}

	want := "--json=" + strings.Join(output.Select, ",")
	matches, err := verifyRuntimeArgs(path, plan.Stages.Invoke.Args[len(plan.MatchedCommand):], want)
	if err != nil {
		return err
	}
	if matches != 1 {
		return admissionError(ErrRuntimeSelector, ErrRuntimeSelectorConflict, runtimeadmission.CategorySelectorConflict)
	}
	return nil
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
