package gocli

import (
	"errors"
	"regexp"
	"strings"

	"github.com/tasuku43/atsura/internal/domain/runtimeadmission"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

var (
	// ErrRuntimeUnsupported means this adapter cannot prove the supplied Go CLI
	// plan or complete surface under contract 1.
	ErrRuntimeUnsupported = errors.New("go cli runtime is not supported")

	// ErrRuntimeAdapterContract classifies an adapter kind or inspection
	// contract outside the one maintained Go CLI boundary.
	ErrRuntimeAdapterContract = errors.New("go cli adapter contract is not admitted")
	// ErrRuntimeSourceVersion classifies the catalog's inspection-time effective
	// version observation outside stable go1.26.x releases. It is distinct from
	// the separately bound direct-launcher file identity.
	ErrRuntimeSourceVersion = errors.New("go cli source version is not admitted")
	// ErrRuntimeCommand classifies a source command other than exact test.
	ErrRuntimeCommand = errors.New("go cli command is not admitted")
	// ErrRuntimeWrapperOutput classifies any non-identity wrapper or result mode.
	ErrRuntimeWrapperOutput = errors.New("go cli wrapper output is not admitted")
	// ErrRuntimeArgvGrammar classifies an option, package, marker, test-binary
	// argument, or observed option/output grammar outside contract 1.
	ErrRuntimeArgvGrammar = errors.New("go cli argv grammar is not admitted")
)

var runtimeSourceVersionPattern = regexp.MustCompile(`^go1\.26\.(?:0|[1-9][0-9]*)$`)

// runtimeAdmissionError preserves the package's broad sentinel while exposing
// one finite vendor-neutral category. It retains no source text or parser
// cause.
type runtimeAdmissionError struct {
	category error
	name     runtimeadmission.Category
}

func (e *runtimeAdmissionError) Error() string {
	return ErrRuntimeUnsupported.Error() + ": " + e.category.Error()
}

func (e *runtimeAdmissionError) Unwrap() []error {
	return []error{ErrRuntimeUnsupported, e.category}
}

func (e *runtimeAdmissionError) RuntimeAdmissionCategory() runtimeadmission.Category {
	return e.name
}

func admissionError(category error, name runtimeadmission.Category) error {
	return &runtimeAdmissionError{category: category, name: name}
}

// RuntimeVerifier is the zero-state Go CLI runtime proof adapter.
type RuntimeVerifier struct{}

// NewRuntimeVerifier creates a Go CLI runtime proof adapter.
func NewRuntimeVerifier() *RuntimeVerifier { return &RuntimeVerifier{} }

// VerifyRuntime proves the same contract as the package function.
func (*RuntimeVerifier) VerifyRuntime(plan tailoringplan.Plan) error { return VerifyRuntime(plan) }

// VerifySurface proves the same complete-surface contract as the package
// function.
func (*RuntimeVerifier) VerifySurface(bundle tailoringbundle.Bundle) error {
	return VerifySurface(bundle)
}

// VerifyRuntime accepts only a contract-2 catalog carrying a stable Go 1.26.x
// inspection-time version observation and an exact no-argument test invocation.
// It admits either identity source-stream delivery or the exact -json append
// with a typed optimizer input. It contains no processor identity or filter
// policy, performs no I/O, does not claim runtime toolchain closure, and grants
// no permission to the downstream test process.
func VerifyRuntime(plan tailoringplan.Plan) error {
	if err := plan.Validate(); err != nil {
		return admissionError(ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}
	if err := verifySourceContract(plan.Source.AdapterKind, plan.Source.AdapterContractVersion, plan.Source.Version); err != nil {
		return err
	}
	if !exactTestCommand(plan.MatchedCommand) {
		return admissionError(ErrRuntimeCommand, runtimeadmission.CategoryCommand)
	}
	if len(plan.OriginalArgv) != 2 || plan.OriginalArgv[1] != "test" {
		return admissionError(ErrRuntimeArgvGrammar, runtimeadmission.CategoryArgvGrammar)
	}
	switch plan.ResultMode {
	case tailoringplan.ResultModeSourceStreamPassthrough:
		if !identityWrapper(plan.WrapperKind, plan.Stages.Before, plan.Stages.Invoke.OptionDefaults, plan.Stages.Invoke.AppendedArgs, plan.Stages.Output, plan.Stages.After) || len(plan.Stages.Invoke.Args) != 1 || plan.Stages.Invoke.Args[0] != "test" {
			return admissionError(ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
		}
	case tailoringplan.ResultModeOriginalPreservingOptimizer:
		optimizer, present, err := plan.OptimizerPlan()
		if err != nil || !present || !optimizerWrapper(plan.WrapperKind, plan.Stages.Before, plan.Stages.Invoke.OptionDefaults, plan.Stages.Invoke.AppendedArgs, plan.Stages.Output, plan.Stages.After) || optimizer.Input != "go_test_jsonl" || !optimizer.AllowOriginalOutput {
			return admissionError(ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
		}
		if len(plan.Stages.Invoke.Args) != 2 || plan.Stages.Invoke.Args[0] != "test" || plan.Stages.Invoke.Args[1] != "-json" {
			return admissionError(ErrRuntimeArgvGrammar, runtimeadmission.CategoryArgvGrammar)
		}
	default:
		return admissionError(ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}
	return nil
}

// VerifySurface accepts only a complete included surface containing exact
// command test, no observed caller option grammar, one exact go_test_jsonl
// selector observation, and either the identity or optimizer wrapper shape.
// Other cataloged root commands may remain excluded.
func VerifySurface(bundle tailoringbundle.Bundle) error {
	if err := bundle.Validate(); err != nil {
		return admissionError(ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}
	if err := verifySourceContract(bundle.Catalog.Adapter.Kind, bundle.Catalog.Adapter.ContractVersion, bundle.Catalog.Source.Version); err != nil {
		return err
	}
	if len(bundle.Surface) != 1 {
		return admissionError(ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}
	entry := bundle.Surface[0]
	if !exactTestCommand(entry.Command) {
		return admissionError(ErrRuntimeCommand, runtimeadmission.CategoryCommand)
	}
	command, found := catalogCommand(bundle.Catalog, entry.Command)
	if !found {
		return admissionError(ErrRuntimeCommand, runtimeadmission.CategoryCommand)
	}
	if len(command.Options) != 0 || len(entry.Options.Include) != 0 || len(entry.Options.Exclude) != 0 {
		return admissionError(ErrRuntimeArgvGrammar, runtimeadmission.CategoryArgvGrammar)
	}
	if !exactTestJSONOutput(command.StructuredOutput) {
		return admissionError(ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}
	if !identityWrapper(entry.Wrapper.Kind, entry.Wrapper.Before, entry.Wrapper.Invoke.OptionDefaults, entry.Wrapper.Invoke.AppendArgs, entry.Wrapper.Output, entry.Wrapper.After) &&
		!optimizerWrapper(entry.Wrapper.Kind, entry.Wrapper.Before, entry.Wrapper.Invoke.OptionDefaults, entry.Wrapper.Invoke.AppendArgs, entry.Wrapper.Output, entry.Wrapper.After) {
		return admissionError(ErrRuntimeWrapperOutput, runtimeadmission.CategoryWrapperOutput)
	}
	return nil
}

func verifySourceContract(adapterKind string, adapterContract int, sourceVersion string) error {
	if adapterKind != AdapterKind || adapterContract != ContractVersion {
		return admissionError(ErrRuntimeAdapterContract, runtimeadmission.CategoryAdapterContract)
	}
	if !runtimeSourceVersionPattern.MatchString(sourceVersion) {
		return admissionError(ErrRuntimeSourceVersion, runtimeadmission.CategorySourceVersion)
	}
	return nil
}

func exactTestCommand(path []string) bool {
	return len(path) == 1 && path[0] == "test"
}

func identityWrapper(kind tailoringbundle.WrapperKind, before []tailoringbundle.StageAction, defaults []tailoringbundle.OptionDefault, appendArgs []string, output *tailoringbundle.Output, after []tailoringbundle.StageAction) bool {
	return kind == tailoringbundle.WrapperIdentity && len(before) == 0 && len(defaults) == 0 && len(appendArgs) == 0 && output == nil && len(after) == 0
}

func optimizerWrapper(kind tailoringbundle.WrapperKind, before []tailoringbundle.StageAction, defaults []tailoringbundle.OptionDefault, appendArgs []string, output *tailoringbundle.Output, after []tailoringbundle.StageAction) bool {
	return kind == tailoringbundle.WrapperTransform && len(before) == 0 && len(after) == 0 && len(defaults) == 0 && len(appendArgs) == 1 && appendArgs[0] == "-json" && output != nil && output.Kind == tailoringbundle.OutputKindOptimizer && output.Projection == nil && output.Optimizer != nil && output.Optimizer.Input == "go_test_jsonl" && output.Optimizer.AllowOriginalOutput
}

func exactTestJSONOutput(values []sourcecatalog.StructuredOutput) bool {
	wantFields := []string{"Action", "Elapsed", "FailedBuild", "Output", "Package", "Test", "Time"}
	return len(values) == 1 && values[0].Format == "go_test_jsonl" && values[0].SelectorFlag == "-json" && slicesEqual(values[0].Fields, wantFields)
}

func slicesEqual(left, right []string) bool {
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

func catalogCommand(catalog sourcecatalog.Catalog, path []string) (sourcecatalog.Command, bool) {
	wanted := strings.Join(path, "\x00")
	for _, command := range catalog.Commands {
		if strings.Join(command.Path, "\x00") == wanted {
			return command, true
		}
	}
	return sourcecatalog.Command{}, false
}
