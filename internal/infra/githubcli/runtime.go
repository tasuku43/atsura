package githubcli

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

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
)

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
		return fmt.Errorf("%w: invalid wrapper plan", ErrRuntimeUnsupported)
	}
	if plan.Source.AdapterKind != AdapterKind || plan.Source.AdapterContractVersion != ContractVersion {
		return fmt.Errorf("%w: adapter contract is not covered", ErrRuntimeUnsupported)
	}
	version := sourceVersionPattern.FindStringSubmatch(plan.Source.Version)
	if len(version) != 2 || version[1] != "2" {
		return fmt.Errorf("%w: source version is not covered", ErrRuntimeUnsupported)
	}
	path := strings.Join(plan.MatchedCommand, " ")
	if path != "issue list" && path != "pr list" {
		return fmt.Errorf("%w: command is not covered", ErrRuntimeUnsupported)
	}
	output, present, err := plan.OutputPlan()
	if err != nil || !present || output.Input != tailoring.InputJSON {
		return fmt.Errorf("%w: output mode is not covered", ErrRuntimeUnsupported)
	}

	want := "--json=" + strings.Join(output.Select, ",")
	matches := 0
	for _, argument := range plan.Stages.Invoke.Args[len(plan.MatchedCommand):] {
		if argument == "--" {
			break
		}
		if argument == "--json" || strings.HasPrefix(argument, "--json=") {
			matches++
			if argument != want {
				return fmt.Errorf("%w: selector value does not match the output plan", ErrRuntimeSelector)
			}
		}
	}
	if matches != 1 {
		return fmt.Errorf("%w: expected exactly one active selector", ErrRuntimeSelector)
	}
	return nil
}
