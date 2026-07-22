package posixshim

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/wrapperbinding"
	"github.com/tasuku43/atsura/internal/domain/wrappershim"
)

func TestSingleQuoteIsOneBoundedLiteralWord(t *testing.T) {
	values := map[string]string{
		"":                           "''",
		"plain":                      "'plain'",
		"space value":                "'space value'",
		"$(not-executed); `literal`": "'$(not-executed); `literal`'",
		"Atsura's shim":              "'Atsura'\\''s shim'",
		"Unicode 雪":                  "'Unicode 雪'",
	}
	for value, want := range values {
		got, err := SingleQuote(value)
		if err != nil || got != want {
			t.Errorf("SingleQuote(%q) = %q, %v; want %q", value, got, err, want)
		}
	}
	for _, value := range []string{string([]byte{0xff}), "nul\x00byte", strings.Repeat("x", MaxQuotedValueBytes+1)} {
		if got, err := SingleQuote(value); got != "" || !errors.Is(err, ErrInvalidQuotedValue) {
			t.Errorf("SingleQuote(invalid) = %q, %v", got, err)
		}
	}
}

func TestRenderIsDeterministicFixedExecutable(t *testing.T) {
	runtimePath := "/opt/Atsura's runtime/bin/atr"
	bundlePath := "/tmp/-purpose bundle;`literal`/$(still-literal)-雪.json"
	binding := testBinding(t, runtimePath, bundlePath, wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
		Path:    []string{"issue", "list"},
		Summary: "List issues",
		Reason:  "Keep issue inventory",
		Options: []wrapperbinding.HelpOption{{Name: "--json", TakesValue: true, DefaultValue: stringPointer("30")}, {Name: "--web", TakesValue: false}},
	}}})

	first, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	second, err := New().Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	want := "#!/bin/sh\n" +
		"\\unset -f command exit 2>/dev/null || \\exit 125\n" +
		"if \\command test \"$#\" -eq 1 && \\command test \"${1}\" = '--help'; then\n" +
		"  \\command printf '%s\\n' 'Atsura tailored help' 'Bundle digest: " + strings.Repeat("a", 64) + "' 'Commands:' '  issue list'\n" +
		"  \\exit $?\nfi\n" +
		"if \\command test \"$#\" -eq 2 && \\command test \"${1}\" = 'issue' && \\command test \"${2}\" = '--help'; then\n" +
		"  \\command printf '%s\\n' 'Atsura tailored help' 'Bundle digest: " + strings.Repeat("a", 64) + "' 'Commands:' '  issue list'\n" +
		"  \\exit $?\nfi\n" +
		"if \\command test \"$#\" -eq 3 && \\command test \"${1}\" = 'issue' && \\command test \"${2}\" = 'list' && \\command test \"${3}\" = '--help'; then\n" +
		"  \\command printf '%s\\n' 'Atsura tailored help' 'Bundle digest: " + strings.Repeat("a", 64) + "' 'Command: issue list' 'Source summary: List issues' 'Tailoring reason: Keep issue inventory' 'Options:' '  --json=<value> (value required; default when omitted: \"30\")' '  --web (no value)'\n" +
		"  \\exit $?\nfi\n" +
		"\\exec '/opt/Atsura'\\''s runtime/bin/atr' --error-format=json wrapper run --contract-version=3" +
		" --bundle='/tmp/-purpose bundle;`literal`/$(still-literal)-雪.json'" +
		" --bundle-digest=" + strings.Repeat("a", 64) +
		" --runtime-path='/opt/Atsura'\\''s runtime/bin/atr'" +
		" --runtime-sha256=" + strings.Repeat("b", 64) +
		" --runtime-size=4242 -- \"$@\"\n"
	if string(first.Source) != want || string(second.Source) != want {
		t.Fatalf("rendered source:\n%s\nwant:\n%s", first.Source, want)
	}
	wantDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(want)))
	if first.SHA256 != wantDigest || second.SHA256 != wantDigest {
		t.Fatalf("digests = %q, %q, want %q", first.SHA256, second.SHA256, wantDigest)
	}
	if len(first.Source) > wrappershim.MaxShimBytes {
		t.Fatalf("rendered source = %d bytes", len(first.Source))
	}
	for _, forbidden := range []string{"eval", "sh -c", "command -v", "type -P", "--command=", "${PATH", "$PATH", "source "} {
		if strings.Contains(string(first.Source), forbidden) {
			t.Fatalf("fixed shim contains forbidden material %q:\n%s", forbidden, first.Source)
		}
	}
}

func TestExecutableShimPrintsExactCompiledHelpWithoutRuntimeAttempt(t *testing.T) {
	temporary := t.TempDir()
	marker := filepath.Join(temporary, "runtime-attempted")
	runtimePath := filepath.Join(temporary, "runtime")
	writeExecutable(t, runtimePath, []byte("#!/bin/sh\nprintf attempted > '"+marker+"'\nexit 99\n"))
	help := wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{
		{Path: []string{"issue"}, Summary: "Work with issues", Reason: "Expose the issue namespace", Options: []wrapperbinding.HelpOption{{Name: "--web", TakesValue: false}}},
		{Path: []string{"issue", "list"}, Summary: "List $(literal) `%s` 'issues' 雪", Reason: "Preserve [brackets] * ? ! # & | < >", Options: []wrapperbinding.HelpOption{{Name: "--state", TakesValue: true, DefaultValue: stringPointer("$(not-executed); 'open'")}}},
		{Path: []string{"pr", "list"}, Summary: "List pull requests", Reason: "Keep PR inventory", Options: []wrapperbinding.HelpOption{}},
	}}
	binding := testBinding(t, runtimePath, filepath.Join(temporary, "bundle.json"), help)
	material, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"--help"}, want: "Commands:\n  issue\n  issue list\n  pr list\n"},
		{args: []string{"issue", "--help"}, want: "Commands:\n  issue list\nCommand: issue\nSource summary: Work with issues\nTailoring reason: Expose the issue namespace\nOptions:\n  --web (no value)\n"},
		{args: []string{"issue", "list", "--help"}, want: "Command: issue list\nSource summary: List $(literal) `%s` 'issues' 雪\nTailoring reason: Preserve [brackets] * ? ! # & | < >\nOptions:\n  --state=<value> (value required; default when omitted: \"$(not-executed); 'open'\")\n"},
		{args: []string{"pr", "--help"}, want: "Commands:\n  pr list\n"},
	}
	for _, test := range tests {
		output, status := executeMaterial(t, material.Source, test.args...)
		if status != 0 {
			t.Fatalf("help %q status = %d, output=%q", test.args, status, output)
		}
		header := "Atsura tailored help\nBundle digest: " + binding.BundleDigest + "\n"
		if string(output) != header+test.want {
			t.Fatalf("help %q = %q, want %q", test.args, output, header+test.want)
		}
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("help reached runtime marker: %v", err)
	}
}

func TestExecutableShimForwardsExactArgvOnlyToBoundRuntime(t *testing.T) {
	temporary := t.TempDir()
	runtimePath := filepath.Join(temporary, "Atsura's runtime")
	writeExecutable(t, runtimePath, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"))
	bundlePath := filepath.Join(temporary, "purpose bundle;$(literal).json")
	binding := testBinding(t, runtimePath, bundlePath, oneHelp())
	material, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	argv := []string{"", "space value", "-dash", "$(literal)", "single'quote", "back\\slash", "line\nbreak", "Unicode 雪", "*"}
	output, status := executeMaterial(t, material.Source, argv...)
	if status != 0 {
		t.Fatalf("shim status=%d output=%q", status, output)
	}
	wantArgs := append([]string{
		"--error-format=json", "wrapper", "run", "--contract-version=3",
		"--bundle=" + bundlePath,
		"--bundle-digest=" + binding.BundleDigest,
		"--runtime-path=" + runtimePath,
		"--runtime-sha256=" + binding.Runtime.SHA256,
		"--runtime-size=" + strconv.FormatInt(binding.Runtime.Size, 10),
		"--",
	}, argv...)
	want := strings.Join(wantArgs, "\n") + "\n"
	if string(output) != want {
		t.Fatalf("recorded argv:\n%q\nwant:\n%q", output, want)
	}
}

func TestExecutableShimPreservesBoundRuntimeStatusAndStreams(t *testing.T) {
	temporary := t.TempDir()
	runtimePath := filepath.Join(temporary, "runtime")
	writeExecutable(t, runtimePath, []byte("#!/bin/sh\nprintf 'runtime stdout\\n'\nprintf 'runtime stderr\\n' >&2\nexit 17\n"))
	binding := testBinding(t, runtimePath, filepath.Join(temporary, "bundle.json"), oneHelp())
	material, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	shimPath := filepath.Join(t.TempDir(), "gh")
	writeExecutable(t, shimPath, material.Source)
	command := exec.Command(shimPath, "issue", "list")
	stdout, err := command.Output()
	if string(stdout) != "runtime stdout\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 17 || string(exitError.Stderr) != "runtime stderr\n" {
		t.Fatalf("runtime result = %v, stderr=%q", err, exitError.Stderr)
	}
}

func TestRenderedShimParsesAcrossAvailableMaintainedShells(t *testing.T) {
	material, err := Render(testBinding(t, "/opt/bin/atr", "/tmp/bundle.json", oneHelp()))
	if err != nil {
		t.Fatal(err)
	}
	shimPath := filepath.Join(t.TempDir(), "shim")
	writeExecutable(t, shimPath, material.Source)
	shells := []struct {
		path string
		args []string
	}{
		{path: "/bin/sh"},
		{path: "/bin/dash"},
		{path: "/bin/bash", args: []string{"--posix"}},
		{path: "/bin/zsh", args: []string{"--emulate", "sh"}},
	}
	for _, shell := range shells {
		if _, err := os.Stat(shell.path); err != nil {
			continue
		}
		args := append(append([]string{}, shell.args...), "-n", shimPath)
		if output, err := exec.Command(shell.path, args...).CombinedOutput(); err != nil {
			t.Errorf("%s parse: %v: %s", shell.path, err, output)
		}
	}
}

func TestRenderRejectsInvalidAndOversizedInputWithoutPartialMaterial(t *testing.T) {
	valid := testBinding(t, "/opt/bin/atr", "/tmp/bundle.json", oneHelp())
	tests := []struct {
		name   string
		mutate func(*wrapperbinding.Binding)
	}{
		{name: "invalid binding", mutate: func(value *wrapperbinding.Binding) { value.CommandName = "if" }},
		{name: "windows bundle syntax", mutate: func(value *wrapperbinding.Binding) { value.BundleLocator = `C:\purpose\bundle.json` }},
		{name: "unclean runtime", mutate: func(value *wrapperbinding.Binding) { value.Runtime.ResolvedPath = "/opt/../bin/atr" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid.Clone()
			test.mutate(&candidate)
			material, err := Render(candidate)
			if len(material.Source) != 0 || material.SHA256 != "" || !errors.Is(err, ErrInvalidRender) {
				t.Fatalf("Render() = %+v, %v", material, err)
			}
		})
	}

	commands := make([]wrapperbinding.HelpCommand, 240)
	for index := range commands {
		commands[index] = wrapperbinding.HelpCommand{Path: []string{fmt.Sprintf("cmd%03d", index)}, Summary: "s", Reason: "r", Options: []wrapperbinding.HelpOption{}}
	}
	oversized := valid.Clone()
	oversized.Help = wrapperbinding.CompiledHelp{Commands: commands}
	if err := oversized.Validate(); err != nil {
		t.Fatalf("fixture must cross only rendered-material bound: %v", err)
	}
	material, err := Render(oversized)
	if len(material.Source) != 0 || !errors.Is(err, ErrInvalidRender) {
		t.Fatalf("oversized Render() = %d bytes, %v", len(material.Source), err)
	}
}

func testBinding(t *testing.T, runtimePath, bundlePath string, help wrapperbinding.CompiledHelp) wrapperbinding.Binding {
	t.Helper()
	binding := wrapperbinding.Binding{
		ContractVersion: wrapperbinding.ContractVersion,
		BundleLocator:   bundlePath,
		BundleDigest:    strings.Repeat("a", 64),
		CommandName:     "gh",
		Help:            help,
		Runtime: wrapperbinding.RuntimeIdentity{
			ResolvedPath: runtimePath,
			SHA256:       strings.Repeat("b", 64),
			Size:         4242,
		},
	}
	if err := binding.Validate(); err != nil {
		t.Fatalf("binding: %v", err)
	}
	return binding
}

func oneHelp() wrapperbinding.CompiledHelp {
	return wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
		Path: []string{"issue", "list"}, Summary: "List issues", Reason: "Keep issue inventory", Options: []wrapperbinding.HelpOption{},
	}}}
}

func stringPointer(value string) *string { return &value }

func writeExecutable(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o700); err != nil {
		t.Fatal(err)
	}
}

func executeMaterial(t *testing.T, source []byte, argv ...string) ([]byte, int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "shim")
	writeExecutable(t, path, source)
	output, err := exec.Command(path, argv...).CombinedOutput()
	if err == nil {
		return output, 0
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		t.Fatalf("execute shim: %v", err)
	}
	return output, exitError.ExitCode()
}
