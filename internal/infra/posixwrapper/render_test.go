//go:build !windows

package posixwrapper

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
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
		Help: wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
			Path:    []string{"issue", "list"},
			Summary: "List issues",
			Reason:  "Keep issue inventory",
			Options: []wrapperbinding.HelpOption{{Name: "--json", TakesValue: true}, {Name: "--web", TakesValue: false}},
		}}},
	}

	first, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	want := "\\unalias gh 2>/dev/null || \\:\n" +
		"gh() (\n" +
		"  \\unset -f command return 2>/dev/null || \\exit 125\n" +
		"  if \\command test \"$#\" -eq 1 && \\command test \"${1}\" = '--help'; then\n" +
		"    \\command printf '%s\\n' 'Atsura tailored help' 'Bundle digest: " + strings.Repeat("a", 64) + "' 'Commands:' '  issue list'\n" +
		"    \\return $?\n" +
		"  fi\n" +
		"  if \\command test \"$#\" -eq 2 && \\command test \"${1}\" = 'issue' && \\command test \"${2}\" = '--help'; then\n" +
		"    \\command printf '%s\\n' 'Atsura tailored help' 'Bundle digest: " + strings.Repeat("a", 64) + "' 'Commands:' '  issue list'\n" +
		"    \\return $?\n" +
		"  fi\n" +
		"  if \\command test \"$#\" -eq 3 && \\command test \"${1}\" = 'issue' && \\command test \"${2}\" = 'list' && \\command test \"${3}\" = '--help'; then\n" +
		"    \\command printf '%s\\n' 'Atsura tailored help' 'Bundle digest: " + strings.Repeat("a", 64) + "' 'Command: issue list' 'Source summary: List issues' 'Tailoring reason: Keep issue inventory' 'Options:' '  --json=<value> (value required)' '  --web (no value)'\n" +
		"    \\return $?\n" +
		"  fi\n" +
		"  '/opt/Atsura'\\''s runtime/bin/atr' --error-format=json wrapper run --contract-version=2" +
		" --bundle='/tmp/-purpose bundle;`literal`/$(still-literal)-\u96ea.json'" +
		" --bundle-digest=" + strings.Repeat("a", 64) +
		" --runtime-path='/opt/Atsura'\\''s runtime/bin/atr'" +
		" --runtime-sha256=" + strings.Repeat("b", 64) +
		" --runtime-size=4242 -- \"$@\"\n)\n"
	if string(first.Source) != want || string(second.Source) != want {
		t.Fatalf("rendered source:\n%s\nwant:\n%s", first.Source, want)
	}
	wantDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(want)))
	if first.SHA256 != wantDigest || second.SHA256 != wantDigest {
		t.Fatalf("digests = %q, %q, want %q", first.SHA256, second.SHA256, wantDigest)
	}
	for _, forbidden := range []string{"--command=", "eval", "sh -c", "$10"} {
		if strings.Contains(string(first.Source), forbidden) {
			t.Fatalf("rendered source contains forbidden runtime material %q:\n%s", forbidden, first.Source)
		}
	}
}

func TestRenderedFunctionPrintsExactRootNamespaceExactAndCombinedHelp(t *testing.T) {
	binding := helpBinding(t, "fixture", wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{
		{
			Path:    []string{"issue"},
			Summary: "Work with issues",
			Reason:  "Expose the issue namespace",
			Options: []wrapperbinding.HelpOption{{Name: "--web", TakesValue: false}},
		},
		{
			Path:    []string{"issue", "list"},
			Summary: "List issues",
			Reason:  "Return a compact issue inventory",
			Options: []wrapperbinding.HelpOption{{Name: "--json", TakesValue: true}, {Name: "--state", TakesValue: true}},
		},
		{
			Path:    []string{"pr", "list"},
			Summary: "List pull requests",
			Reason:  "Return a compact pull request inventory",
			Options: []wrapperbinding.HelpOption{},
		},
	}})
	rendered, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	header := "Atsura tailored help\nBundle digest: " + binding.BundleDigest + "\n"
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "root",
			args: []string{"--help"},
			want: header + "Commands:\n  issue\n  issue list\n  pr list\n",
		},
		{
			name: "combined exact and namespace",
			args: []string{"issue", "--help"},
			want: header + "Commands:\n  issue list\n" +
				"Command: issue\nSource summary: Work with issues\n" +
				"Tailoring reason: Expose the issue namespace\n" +
				"Options:\n  --web (no value)\n",
		},
		{
			name: "exact",
			args: []string{"issue", "list", "--help"},
			want: header + "Command: issue list\nSource summary: List issues\n" +
				"Tailoring reason: Return a compact issue inventory\n" +
				"Options:\n  --json=<value> (value required)\n  --state=<value> (value required)\n",
		},
		{
			name: "namespace",
			args: []string{"pr", "--help"},
			want: header + "Commands:\n  pr list\n",
		},
		{
			name: "exact without options",
			args: []string{"pr", "list", "--help"},
			want: header + "Command: pr list\nSource summary: List pull requests\n" +
				"Tailoring reason: Return a compact pull request inventory\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := invokeRendered(rendered.Source, binding.CommandName, "", test.args...)
			if err != nil {
				t.Fatal(err)
			}
			if string(output) != test.want {
				t.Fatalf("help bytes:\n%q\nwant:\n%q", output, test.want)
			}
		})
	}
}

func TestRenderedHelpKeepsHostilePrintableTextLiteralAndBypassesPrintfFunction(t *testing.T) {
	help := wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
		Path:    []string{"issue", "list"},
		Summary: "Literal $(printf injected); `tick` %s 'quote' \\backslash \u96ea",
		Reason:  "Preserve [brackets] * ? ! # & | < > (parentheses)",
		Options: []wrapperbinding.HelpOption{{Name: "--json", TakesValue: true}},
	}}}
	binding := helpBinding(t, "fixture", help)
	rendered, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "printf() { command printf '%s\\n' 'INTERCEPTED'; }\n" +
		"test() { return 1; }\n"
	output, err := invokeRendered(rendered.Source, binding.CommandName, prefix, "issue", "list", "--help")
	if err != nil {
		t.Fatal(err)
	}
	want := "Atsura tailored help\nBundle digest: " + binding.BundleDigest + "\n" +
		"Command: issue list\n" +
		"Source summary: Literal $(printf injected); `tick` %s 'quote' \\backslash \u96ea\n" +
		"Tailoring reason: Preserve [brackets] * ? ! # & | < > (parentheses)\n" +
		"Options:\n  --json=<value> (value required)\n"
	if string(output) != want {
		t.Fatalf("help bytes:\n%q\nwant:\n%q", output, want)
	}
	if strings.Contains(string(output), "INTERCEPTED") {
		t.Fatalf("caller-defined printf intercepted fixed help: %q", output)
	}

	printfBinding := helpBinding(t, "printf", help)
	printfRendered, err := Render(printfBinding)
	if err != nil {
		t.Fatal(err)
	}
	output, err = invokeRendered(printfRendered.Source, printfBinding.CommandName, "", "issue", "list", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != want {
		t.Fatalf("wrapper named printf did not reach the shell utility through command: %q", output)
	}
}

func TestRenderedHelpCannotBeHijackedByCallerCommandFunction(t *testing.T) {
	help := wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
		Path: []string{"issue", "list"}, Summary: "List issues", Reason: "Keep issue inventory", Options: []wrapperbinding.HelpOption{},
	}}}
	binding := helpBinding(t, "fixture", help)
	rendered, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}

	script := "command() { printf 'CALLER_COMMAND:%s\\n' \"$*\"; }\n" + string(rendered.Source) +
		"\nfixture issue list --help\ncommand preserved\n"
	output, err := exec.Command("/bin/sh", "-c", script).Output()
	if err != nil {
		t.Fatal(err)
	}
	want := "Atsura tailored help\nBundle digest: " + binding.BundleDigest + "\n" +
		"Command: issue list\nSource summary: List issues\nTailoring reason: Keep issue inventory\n" +
		"CALLER_COMMAND:preserved\n"
	if string(output) != want {
		t.Fatalf("help or caller command function isolation failed:\n%q\nwant:\n%q", output, want)
	}
}

func TestRenderedHelpCannotBeHijackedByCallerReturnFunctionWhenSupported(t *testing.T) {
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("bash is not installed")
	}
	help := wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
		Path: []string{"issue", "list"}, Summary: "List issues", Reason: "Keep issue inventory", Options: []wrapperbinding.HelpOption{},
	}}}
	binding := helpBinding(t, "fixture", help)
	rendered, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}

	script := "return() { printf 'CALLER_RETURN:%s\\n' \"$*\"; }\n" + string(rendered.Source) +
		"\nfixture issue list --help\nreturn preserved\n"
	output, err := exec.Command("/bin/bash", "-c", script).Output()
	if err != nil {
		t.Fatal(err)
	}
	want := "Atsura tailored help\nBundle digest: " + binding.BundleDigest + "\n" +
		"Command: issue list\nSource summary: List issues\nTailoring reason: Keep issue inventory\n" +
		"CALLER_RETURN:preserved\n"
	if string(output) != want {
		t.Fatalf("help or caller return function isolation failed:\n%q\nwant:\n%q", output, want)
	}
}

func TestRenderedHelpAndDefinitionCannotBeHijackedByCallerAliases(t *testing.T) {
	help := wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
		Path: []string{"issue", "list"}, Summary: "List issues", Reason: "Keep issue inventory", Options: []wrapperbinding.HelpOption{},
	}}}
	binding := helpBinding(t, "fixture", help)
	rendered, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	wrapperPath := filepath.Join(t.TempDir(), "wrapper.sh")
	if err := os.WriteFile(wrapperPath, rendered.Source, 0o600); err != nil {
		t.Fatal(err)
	}
	quotedPath, err := SingleQuote(wrapperPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "Atsura tailored help\nBundle digest: " + binding.BundleDigest + "\n" +
		"Command: issue list\nSource summary: List issues\nTailoring reason: Keep issue inventory\n"
	tests := []struct {
		name   string
		prefix string
	}{
		{name: "same name and return", prefix: "alias fixture='printf ALIAS_WRAPPER\\n'\nalias return='printf ALIAS_RETURN\\n'\n"},
		{name: "no-op utility", prefix: "alias :='printf ALIAS_COLON\\n'\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			script := test.prefix + ". " + quotedPath + "\nfixture issue list --help\n"
			output, err := exec.Command("/bin/sh", "-c", script).Output()
			if err != nil {
				t.Fatal(err)
			}
			if string(output) != want {
				t.Fatalf("caller alias intercepted generated material: %q", output)
			}
		})
	}
}

func TestRenderedWrapperParsesAcrossAvailablePOSIXShellFamilies(t *testing.T) {
	help := wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
		Path: []string{"issue", "list"}, Summary: "List issues", Reason: "Keep issue inventory", Options: []wrapperbinding.HelpOption{},
	}}}
	binding := helpBinding(t, "printf", help)
	rendered, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	wrapperPath := filepath.Join(t.TempDir(), "wrapper.sh")
	if err := os.WriteFile(wrapperPath, rendered.Source, 0o600); err != nil {
		t.Fatal(err)
	}
	shells := []struct {
		name string
		path string
		args []string
	}{
		{name: "sh", path: "/bin/sh"},
		{name: "dash", path: "/bin/dash"},
		{name: "bash_posix", path: "/bin/bash", args: []string{"--posix"}},
		{name: "zsh_sh", path: "/bin/zsh", args: []string{"--emulate", "sh"}},
	}
	for _, shell := range shells {
		t.Run(shell.name, func(t *testing.T) {
			if _, err := os.Stat(shell.path); err != nil {
				t.Skip("shell is not installed")
			}
			args := append(append([]string{}, shell.args...), "-n", wrapperPath)
			if output, err := exec.Command(shell.path, args...).CombinedOutput(); err != nil {
				t.Fatalf("generated wrapper did not parse: %v: %s", err, output)
			}
		})
	}
}

func TestRenderedHelpUsesBracedPositionalReferencesPastNine(t *testing.T) {
	path := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	binding := helpBinding(t, "fixture", wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
		Path: path, Summary: "Deep command", Reason: "Exercise exact segment matching", Options: []wrapperbinding.HelpOption{},
	}}})
	rendered, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rendered.Source), `\command test "${10}" = 'j'`) || strings.Contains(string(rendered.Source), `$10`) {
		t.Fatalf("deep selector does not use an unambiguous braced positional reference:\n%s", rendered.Source)
	}
	argv := append(append([]string{}, path...), "--help")
	output, err := invokeRendered(rendered.Source, binding.CommandName, "", argv...)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(path, " ")
	want := "Atsura tailored help\nBundle digest: " + binding.BundleDigest + "\n" +
		"Command: " + joined + "\nSource summary: Deep command\n" +
		"Tailoring reason: Exercise exact segment matching\n"
	if string(output) != want {
		t.Fatalf("deep help bytes:\n%q\nwant:\n%q", output, want)
	}
}

func TestRenderedFunctionForwardsExactNonHelpAndUnknownHelpArgv(t *testing.T) {
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
		Help: wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
			Path: []string{"issue", "list"}, Summary: "List issues", Reason: "Keep issue inventory", Options: []wrapperbinding.HelpOption{},
		}}},
	}
	rendered, err := Render(binding)
	if err != nil {
		t.Fatal(err)
	}
	tests := [][]string{
		{"", "space value", "-dash", "$(literal)", "single'quote", "back\\slash", "line\nbreak", "Unicode \u96ea"},
		{"hidden", "--help"},
		{"unknown", "path", "--help"},
		{"issue", "list", "--help", "trailing"},
		{"issue", "list", "--", "--help"},
		{"--help", "trailing"},
		{"-h"},
	}
	for _, argv := range tests {
		prefix := "command() { printf 'INTERCEPTED_COMMAND:%s\\n' \"$*\"; }\n"
		output, err := invokeRendered(rendered.Source, binding.CommandName, prefix, argv...)
		if err != nil {
			t.Fatal(err)
		}
		wantArgs := append([]string{
			"--error-format=json",
			"wrapper",
			"run",
			"--contract-version=2",
			"--bundle=" + binding.BundleLocator,
			"--bundle-digest=" + binding.BundleDigest,
			"--runtime-path=" + runtimePath,
			"--runtime-sha256=" + binding.Runtime.SHA256,
			"--runtime-size=42",
			"--",
		}, argv...)
		want := strings.Join(wantArgs, "\n") + "\n"
		if string(output) != want {
			t.Fatalf("recorded argv for %q:\n%q\nwant:\n%q", argv, output, want)
		}
	}
}

func TestRenderRejectsInvalidOrOversizedBindingWithoutPartialSource(t *testing.T) {
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
		Help: wrapperbinding.CompiledHelp{Commands: []wrapperbinding.HelpCommand{{
			Path: []string{"issue", "list"}, Summary: "List issues", Reason: "Keep issue inventory", Options: []wrapperbinding.HelpOption{},
		}}},
	}
	invalid := valid
	invalid.CommandName = "if"
	if result, err := Render(invalid); len(result.Source) != 0 || !errors.Is(err, ErrInvalidRender) {
		t.Fatalf("invalid binding render = %+v, %v", result, err)
	}

	invalidHelp := valid
	invalidHelp.Help = invalidHelp.Help.Clone()
	invalidHelp.Help.Commands[0].Summary = "structural\ntext"
	if result, err := Render(invalidHelp); len(result.Source) != 0 || !errors.Is(err, ErrInvalidRender) {
		t.Fatalf("invalid help render = %+v, %v", result, err)
	}

	commands := make([]wrapperbinding.HelpCommand, 240)
	for index := range commands {
		commands[index] = wrapperbinding.HelpCommand{
			Path:    []string{fmt.Sprintf("cmd%03d", index)},
			Summary: "s",
			Reason:  "r",
			Options: []wrapperbinding.HelpOption{},
		}
	}
	oversized := valid
	oversized.Help = wrapperbinding.CompiledHelp{Commands: commands}
	if err := oversized.Validate(); err != nil {
		t.Fatalf("oversized fixture must cross only the rendered-material bound: %v", err)
	}
	if result, err := Render(oversized); len(result.Source) != 0 || !errors.Is(err, ErrInvalidRender) {
		t.Fatalf("oversized render = %d bytes, %v", len(result.Source), err)
	}
}

func helpBinding(t *testing.T, commandName string, help wrapperbinding.CompiledHelp) wrapperbinding.Binding {
	t.Helper()
	temporary := t.TempDir()
	binding := wrapperbinding.Binding{
		ContractVersion: wrapperbinding.ContractVersion,
		BundleLocator:   filepath.Join(temporary, "bundle.json"),
		BundleDigest:    strings.Repeat("a", 64),
		CommandName:     commandName,
		Runtime: wrapperbinding.RuntimeIdentity{
			ResolvedPath: filepath.Join(temporary, "runtime-not-invoked"),
			SHA256:       strings.Repeat("b", 64),
			Size:         42,
		},
		Help: help,
	}
	if err := binding.Validate(); err != nil {
		t.Fatalf("help binding: %v", err)
	}
	return binding
}

func invokeRendered(source []byte, commandName, prefix string, argv ...string) ([]byte, error) {
	script := prefix + string(source) + "\n" + commandName + " \"$@\"\n"
	command := exec.Command("/bin/sh", "-c", script, "wrapper-test")
	command.Args = append(command.Args, argv...)
	return command.Output()
}
