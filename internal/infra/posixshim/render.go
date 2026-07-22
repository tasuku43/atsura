// Package posixshim renders one fixed executable POSIX wrapper shim from an
// exact host-neutral wrapper binding.
package posixshim

import (
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
	"github.com/tasuku43/atsura/internal/domain/wrappershim"
)

// MaxQuotedValueBytes covers any one validated help line or bound path.
const MaxQuotedValueBytes = wrapperbinding.MaxCompiledHelpLineBytes

var (
	ErrInvalidQuotedValue = errors.New("invalid POSIX shim quoted value")
	ErrInvalidRender      = errors.New("invalid POSIX shim render")
)

// Renderer adapts the fixed renderer to an application-owned port.
type Renderer struct{}

func New() *Renderer { return &Renderer{} }

func (*Renderer) Render(binding wrapperbinding.Binding) (wrapperbinding.RenderedMaterial, error) {
	return Render(binding)
}

// Render emits an executable #!/bin/sh program with contract-3 help semantics
// and one exact wrapper-run fallthrough. No shell body, executable, argv, or
// command-resolution instruction is supplied by configuration.
func Render(binding wrapperbinding.Binding) (wrapperbinding.RenderedMaterial, error) {
	if err := binding.Validate(); err != nil {
		return wrapperbinding.RenderedMaterial{}, invalidRender("binding: %v", err)
	}
	if !posixAbsoluteClean(binding.BundleLocator) || !posixAbsoluteClean(binding.Runtime.ResolvedPath) {
		return wrapperbinding.RenderedMaterial{}, invalidRender("bundle and runtime paths must use absolute clean POSIX syntax")
	}
	runtimePath, err := SingleQuote(binding.Runtime.ResolvedPath)
	if err != nil {
		return wrapperbinding.RenderedMaterial{}, invalidRender("runtime path: %v", err)
	}
	bundleLocator, err := SingleQuote(binding.BundleLocator)
	if err != nil {
		return wrapperbinding.RenderedMaterial{}, invalidRender("bundle locator: %v", err)
	}

	var source strings.Builder
	source.WriteString("#!/bin/sh\n")
	// The fixed program runs in its own shell process. Removing exported caller
	// functions prevents them from changing help classification or exit status.
	// POSIX special-builtin lookup keeps alias-safe unset authoritative.
	source.WriteString("\\unset -f command exit 2>/dev/null || \\exit 125\n")
	if err := renderHelpBranches(&source, binding); err != nil {
		return wrapperbinding.RenderedMaterial{}, invalidRender("help: %v", err)
	}
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
	source.WriteString(" -- \"$@\"\n")

	encoded := []byte(source.String())
	if len(encoded) > wrappershim.MaxShimBytes {
		return wrapperbinding.RenderedMaterial{}, invalidRender("source exceeds the %d-byte shim bound", wrappershim.MaxShimBytes)
	}
	material, err := wrapperbinding.NewRenderedMaterial(encoded)
	if err != nil {
		return wrapperbinding.RenderedMaterial{}, invalidRender("material: %v", err)
	}
	return material, nil
}

func renderHelpBranches(source *strings.Builder, binding wrapperbinding.Binding) error {
	views, err := binding.Help.Views()
	if err != nil {
		return err
	}
	for _, view := range views {
		source.WriteString("if \\command test \"$#\" -eq ")
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
		source.WriteString("  \\command printf '%s\\n'")
		lines, err := helpLines(binding.BundleDigest, view)
		if err != nil {
			return err
		}
		for _, line := range lines {
			quoted, err := SingleQuote(line)
			if err != nil {
				return err
			}
			source.WriteByte(' ')
			source.WriteString(quoted)
		}
		source.WriteString("\n  \\exit $?\nfi\n")
	}
	return nil
}

func helpLines(bundleDigest string, view wrapperbinding.HelpView) ([]string, error) {
	lines := []string{"Atsura tailored help", "Bundle digest: " + bundleDigest}
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
				line, err := wrapperbinding.FormatHelpOptionLine(option)
				if err != nil {
					return nil, err
				}
				lines = append(lines, line)
			}
		}
	}
	return lines, nil
}

// SingleQuote encodes one bounded UTF-8 value as exactly one POSIX shell word.
func SingleQuote(value string) (string, error) {
	if len(value) > MaxQuotedValueBytes || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return "", ErrInvalidQuotedValue
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'", nil
}

func posixAbsoluteClean(value string) bool {
	return path.IsAbs(value) && path.Clean(value) == value
}

func invalidRender(format string, values ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRender, fmt.Sprintf(format, values...))
}
