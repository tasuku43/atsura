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

// Render emits exactly one function whose fixed body forwards argv to the
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
	source.WriteString(binding.CommandName)
	source.WriteString("() {\n  ")
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
	source.WriteString(" -- \"$@\"\n}\n")

	encoded := []byte(source.String())
	material, err := wrapperbinding.NewRenderedMaterial(encoded)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrInvalidRender, err)
	}
	return material, nil
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
