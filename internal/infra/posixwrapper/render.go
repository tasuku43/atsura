// Package posixwrapper renders a fixed POSIX-shell function for one exact
// host-neutral Atsura wrapper binding.
package posixwrapper

import (
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
)

const MaxQuotedValueBytes = 8192

var (
	ErrInvalidQuotedValue = errors.New("invalid POSIX quoted value")
	ErrInvalidRender      = errors.New("invalid POSIX wrapper render")
)

// Result is the domain-owned deterministic wrapper material contract.
type Result = wrapperbinding.RenderedMaterial

// Renderer adapts the fixed package renderer to the application-owned port.
type Renderer struct{}

// New creates the stateless fixed POSIX wrapper renderer.
func New() *Renderer { return &Renderer{} }

// Render delegates to the fixed package renderer.
func (*Renderer) Render(binding wrapperbinding.Binding) (wrapperbinding.RenderedMaterial, error) {
	return Render(binding)
}

// Render emits exactly one function whose fixed help branches describe the
// compiled tailored surface and whose fallthrough forwards argv to the
// host-neutral wrapper runtime. No configuration-authored shell body is an
// input to this API.
func Render(binding wrapperbinding.Binding) (Result, error) {
	if err := binding.Validate(); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrInvalidRender, err)
	}
	if !posixAbsoluteClean(binding.BundleLocator) || !posixAbsoluteClean(binding.Runtime.ResolvedPath) {
		return Result{}, fmt.Errorf("%w: bundle and runtime paths must use absolute clean POSIX syntax", ErrInvalidRender)
	}
	runtimePath, err := SingleQuote(binding.Runtime.ResolvedPath)
	if err != nil {
		return Result{}, fmt.Errorf("%w: runtime path: %v", ErrInvalidRender, err)
	}
	bundleLocator, err := SingleQuote(binding.BundleLocator)
	if err != nil {
		return Result{}, fmt.Errorf("%w: bundle locator: %v", ErrInvalidRender, err)
	}

	var source strings.Builder
	source.WriteString("\\unalias ")
	source.WriteString(binding.CommandName)
	source.WriteString(" 2>/dev/null || \\:\n")
	source.WriteString(binding.CommandName)
	// Run the fixed body in a subshell so removing caller-defined `command` and
	// `return` functions cannot mutate the caller's shell. POSIX special-builtin
	// lookup makes alias-safe `unset` authoritative; failure closes the wrapper
	// before it can misclassify argv or start the bound runtime.
	source.WriteString("() (\n")
	source.WriteString("  \\unset -f command return 2>/dev/null || \\exit 125\n")
	if err := renderHelpBranches(&source, binding); err != nil {
		return Result{}, fmt.Errorf("%w: help: %v", ErrInvalidRender, err)
	}
	source.WriteString("  ")
	source.WriteString(runtimePath)
	source.WriteString(" --error-format=json wrapper run --contract-version=")
	source.WriteString(strconv.Itoa(binding.ContractVersion))
	source.WriteString(" --bundle=")
	source.WriteString(bundleLocator)
	source.WriteString(" --bundle-digest=")
	source.WriteString(binding.BundleDigest)
	source.WriteString(" --runtime-path=")
	source.WriteString(runtimePath)
	source.WriteString(" --runtime-sha256=")
	source.WriteString(binding.Runtime.SHA256)
	source.WriteString(" --runtime-size=")
	source.WriteString(strconv.FormatInt(binding.Runtime.Size, 10))
	source.WriteString(" -- \"$@\"\n)\n")

	encoded := []byte(source.String())
	material, err := wrapperbinding.NewRenderedMaterial(encoded)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrInvalidRender, err)
	}
	return material, nil
}

func renderHelpBranches(source *strings.Builder, binding wrapperbinding.Binding) error {
	views, err := binding.Help.Views()
	if err != nil {
		return err
	}
	for _, view := range views {
		source.WriteString("  if \\command test \"$#\" -eq ")
		source.WriteString(strconv.Itoa(len(view.Selector) + 1))
		for index, segment := range view.Selector {
			quoted, err := SingleQuote(segment)
			if err != nil {
				return err
			}
			source.WriteString(" && \\command test \"${")
			source.WriteString(strconv.Itoa(index + 1))
			source.WriteString("}\" = ")
			source.WriteString(quoted)
		}
		source.WriteString(" && \\command test \"${")
		source.WriteString(strconv.Itoa(len(view.Selector) + 1))
		source.WriteString("}\" = '--help'; then\n")
		source.WriteString("    \\command printf '%s\\n'")
		for _, line := range helpLines(binding.BundleDigest, view) {
			quoted, err := SingleQuote(line)
			if err != nil {
				return err
			}
			source.WriteByte(' ')
			source.WriteString(quoted)
		}
		source.WriteString("\n    \\return $?\n  fi\n")
	}
	return nil
}

func helpLines(bundleDigest string, view wrapperbinding.HelpView) []string {
	lines := []string{
		"Atsura tailored help",
		"Bundle digest: " + bundleDigest,
	}
	if len(view.Descendants) > 0 {
		lines = append(lines, "Commands:")
		for _, command := range view.Descendants {
			lines = append(lines, "  "+strings.Join(command, " "))
		}
	}
	if view.Exact != nil {
		lines = append(lines,
			"Command: "+strings.Join(view.Exact.Path, " "),
			"Source summary: "+view.Exact.Summary,
			"Tailoring reason: "+view.Exact.Reason,
		)
		if len(view.Exact.Options) > 0 {
			lines = append(lines, "Options:")
			for _, option := range view.Exact.Options {
				if option.TakesValue {
					lines = append(lines, "  "+option.Name+"=<value> (value required)")
				} else {
					lines = append(lines, "  "+option.Name+" (no value)")
				}
			}
		}
	}
	return lines
}

// SingleQuote encodes one bounded UTF-8 value as exactly one POSIX shell word.
// It accepts structural characters because quoting must remain mechanically
// correct for arbitrary argv; Binding validation applies the stricter policy
// required for generated source fields.
func SingleQuote(value string) (string, error) {
	if len(value) > MaxQuotedValueBytes || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return "", ErrInvalidQuotedValue
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'", nil
}

func posixAbsoluteClean(value string) bool {
	return path.IsAbs(value) && path.Clean(value) == value
}
