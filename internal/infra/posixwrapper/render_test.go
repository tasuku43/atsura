//go:build !windows

package posixwrapper

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
)

func TestSingleQuoteRoundTripsHostileValues(t *testing.T) {
	values := []string{
		"",
		"plain",
		"single'quote",
		"spaces between words",
		"line one\nline two",
		"tab\tseparated",
		"semi;colon",
		"back`tick",
		"$(printf injected)",
		"Unicode \u96ea\u3068\U0001F30A",
		"-dash-leading",
	}
	for _, value := range values {
		t.Run(fmt.Sprintf("%x", sha256.Sum256([]byte(value)))[:12], func(t *testing.T) {
			quoted, err := SingleQuote(value)
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command("/bin/sh", "-c", "printf %s "+quoted)
			output, err := command.Output()
			if err != nil {
				t.Fatalf("shell round trip: %v", err)
			}
			if string(output) != value {
				t.Fatalf("round trip = %q, want %q; encoded %q", output, value, quoted)
			}
		})
	}
}

func TestSingleQuoteRejectsNULInvalidUTF8AndUnboundedValues(t *testing.T) {
	values := []string{
		"nul\x00value",
		string([]byte{0xff}),
		strings.Repeat("x", MaxQuotedValueBytes+1),
	}
	for _, value := range values {
		if quoted, err := SingleQuote(value); quoted != "" || !errors.Is(err, ErrInvalidQuotedValue) {
			t.Errorf("SingleQuote() = %q, %v", quoted, err)
		}
	}
}

func TestRenderIsDeterministicFixedSource(t *testing.T) {
	runtimePath := "/opt/Atsura's runtime/bin/atr"
	bundlePath := "/tmp/-purpose bundle;`literal`/$(still-literal)-\u96ea.json"
	binding := wrapperbinding.Binding{
		ContractVersion: wrapperbinding.ContractVersion,
		BundleLocator:   bundlePath,
		BundleDigest:    strings.Repeat("a", 64),
		CommandName:     "gh",
		Runtime: wrapperbinding.RuntimeIdentity{
			ResolvedPath: runtimePath,
			SHA256:       strings.Repeat("b", 64),
			Size:         4242,
		},
	}

	first, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	want := "gh() {\n" +
		"  '/opt/Atsura'\\''s runtime/bin/atr' --error-format=json wrapper run --contract-version=1" +
		" --bundle='/tmp/-purpose bundle;`literal`/$(still-literal)-\u96ea.json'" +
		" --bundle-digest=" + strings.Repeat("a", 64) +
		" --runtime-path='/opt/Atsura'\\''s runtime/bin/atr'" +
		" --runtime-sha256=" + strings.Repeat("b", 64) +
		" --runtime-size=4242 -- \"$@\"\n}\n"
	if string(first.Source) != want || string(second.Source) != want {
		t.Fatalf("rendered source:\n%s\nwant:\n%s", first.Source, want)
	}
	wantDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(want)))
	if first.SHA256 != wantDigest || second.SHA256 != wantDigest {
		t.Fatalf("digests = %q, %q, want %q", first.SHA256, second.SHA256, wantDigest)
	}
	if strings.Contains(string(first.Source), "--command=") || strings.Contains(string(first.Source), "eval") || strings.Contains(string(first.Source), "sh -c") {
		t.Fatalf("rendered source contains forbidden runtime material:\n%s", first.Source)
	}
}

func TestRenderedFunctionForwardsExactArgv(t *testing.T) {
	temporary := t.TempDir()
	runtimePath := filepath.Join(temporary, "atr recorder")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\"\n"
	if err := writeExecutable(runtimePath, []byte(script)); err != nil {
		t.Fatal(err)
	}
	binding := wrapperbinding.Binding{
		ContractVersion: wrapperbinding.ContractVersion,
		BundleLocator:   filepath.Join(temporary, "bundle's purpose.json"),
		BundleDigest:    strings.Repeat("a", 64),
		CommandName:     "fixture",
		Runtime: wrapperbinding.RuntimeIdentity{
			ResolvedPath: runtimePath,
			SHA256:       strings.Repeat("b", 64),
			Size:         42,
		},
	}
	rendered, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	invocation := string(rendered.Source) + "fixture '' 'space value' '-dash' '$(literal)' 'Unicode \u96ea'"
	output, err := exec.Command("/bin/sh", "-c", invocation).Output()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"--error-format=json",
		"wrapper",
		"run",
		"--contract-version=1",
		"--bundle=" + binding.BundleLocator,
		"--bundle-digest=" + binding.BundleDigest,
		"--runtime-path=" + runtimePath,
		"--runtime-sha256=" + binding.Runtime.SHA256,
		"--runtime-size=42",
		"--",
		"",
		"space value",
		"-dash",
		"$(literal)",
		"Unicode \u96ea",
	}, "\n") + "\n"
	if string(output) != want {
		t.Fatalf("recorded argv:\n%q\nwant:\n%q", output, want)
	}
}

func TestRenderRejectsInvalidBindingWithoutPartialSource(t *testing.T) {
	valid := wrapperbinding.Binding{
		ContractVersion: wrapperbinding.ContractVersion,
		BundleLocator:   "/tmp/bundle.json",
		BundleDigest:    strings.Repeat("a", 64),
		CommandName:     "gh",
		Runtime: wrapperbinding.RuntimeIdentity{
			ResolvedPath: "/opt/bin/atr",
			SHA256:       strings.Repeat("b", 64),
			Size:         42,
		},
	}
	invalid := valid
	invalid.CommandName = "if"
	if result, err := Render(invalid); len(result.Source) != 0 || !errors.Is(err, ErrInvalidRender) {
		t.Fatalf("invalid binding render = %+v, %v", result, err)
	}
}
