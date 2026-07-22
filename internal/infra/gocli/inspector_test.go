package gocli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
)

type fakeProcess struct {
	requests []sourceprocess.Request
	results  []sourceprocess.Result
	errors   []error
}

func (f *fakeProcess) Run(_ context.Context, request sourceprocess.Request) (sourceprocess.Result, error) {
	f.requests = append(f.requests, request)
	index := len(f.requests) - 1
	if index >= len(f.results) || index >= len(f.errors) {
		return sourceprocess.Result{Attempts: 0, ExitCode: -1}, errors.New("unexpected probe")
	}
	return f.results[index], f.errors[index]
}

func goIdentity(seed string) sourceprocess.Identity {
	return sourceprocess.Identity{ResolvedPath: "/opt/go/bin/go", SHA256: strings.Repeat(seed, 64), Size: 14_500_192}
}

func successfulGoProcess(version, rootHelp, testHelp string) *fakeProcess {
	identity := goIdentity("a")
	return &fakeProcess{
		results: []sourceprocess.Result{
			{Attempts: 1, ExitCode: 0, Stdout: []byte(version), Stderr: []byte{}, Identity: identity},
			{Attempts: 1, ExitCode: 0, Stdout: []byte(rootHelp), Stderr: []byte{}, Identity: identity},
			{Attempts: 1, ExitCode: 0, Stdout: []byte(testHelp), Stderr: []byte{}, Identity: identity},
		},
		errors: []error{nil, nil, nil},
	}
}

const rootHelpFixture = `Go is a tool for managing Go source code.

Usage:

	go <command> [arguments]

The commands are:

	build       compile packages and dependencies
	test        test packages
	version     print Go version

Use "go help <command>" for more information about a command.

Additional help topics:

	packages    package lists and patterns

Use "go help <topic>" for more information about that topic.
`

const testHelpFixture = `usage: go test [build/test flags] [packages] [build/test flags & test binary flags]

'Go test' automates testing the packages named by the import paths.
It prints a summary of the test results.

The first, called local directory mode, occurs when go test is
invoked with no package arguments (for example, 'go test' or 'go
test -v'). In this mode, go test compiles the package sources and
tests found in the current directory and then runs the resulting
test binary. In this mode, caching (discussed below) is disabled.

	-json
	    Convert test output to JSON suitable for automated processing.
	    Also emits build output in JSON. See 'go help buildjson'.
`

func TestInspectProducesVendorNeutralCatalogWithExactProbes(t *testing.T) {
	process := successfulGoProcess("go version go1.26.5 darwin/arm64\n", rootHelpFixture, testHelpFixture)
	catalog, err := New(process).Inspect(context.Background(), "go")
	if err != nil {
		t.Fatal(err)
	}

	wantArgs := [][]string{{"version"}, {"help"}, {"help", "test"}}
	if len(process.requests) != len(wantArgs) {
		t.Fatalf("requests = %d, want %d", len(process.requests), len(wantArgs))
	}
	for index, request := range process.requests {
		if request.Executable != "go" || strings.Join(request.Args, "\x00") != strings.Join(wantArgs[index], "\x00") {
			t.Fatalf("request %d = %+v", index, request)
		}
		wantStdoutLimit := helpByteLimit
		if index == 0 {
			wantStdoutLimit = versionByteLimit
		}
		if request.Timeout != probeTimeout || request.StdoutLimit != wantStdoutLimit || request.StderrLimit != stderrByteLimit {
			t.Fatalf("request %d bounds = %+v", index, request)
		}
	}

	if catalog.Adapter.Kind != AdapterKind || catalog.Adapter.ContractVersion != ContractVersion {
		t.Fatalf("adapter = %+v", catalog.Adapter)
	}
	if catalog.Source.RequestedExecutable != "go" || catalog.Source.Version != "go1.26.5" || catalog.Source.ResolvedPath != "/opt/go/bin/go" {
		t.Fatalf("source = %+v", catalog.Source)
	}
	if catalog.Probe.Attempts != 3 || strings.Join(catalog.Probe.IDs, ",") != "help,test_help,version" {
		t.Fatalf("probe = %+v", catalog.Probe)
	}
	if len(catalog.Commands) != 3 {
		t.Fatalf("commands = %+v", catalog.Commands)
	}
	wantCommands := []struct {
		name    string
		summary string
	}{
		{name: "build", summary: "compile packages and dependencies"},
		{name: "test", summary: "test packages"},
		{name: "version", summary: "print Go version"},
	}
	for index, want := range wantCommands {
		command := catalog.Commands[index]
		if strings.Join(command.Path, " ") != want.name || command.Summary != want.summary || command.Provenance != sourcecatalog.ProvenanceVerifiedBuiltin {
			t.Fatalf("command %d = %+v", index, command)
		}
		if command.Options == nil || len(command.Options) != 0 || command.StructuredOutput == nil {
			t.Fatalf("command %q inventories are not explicit empty lists: %+v", want.name, command)
		}
		if want.name == "test" {
			if len(command.StructuredOutput) != 1 || command.StructuredOutput[0].Format != "go_test_jsonl" || command.StructuredOutput[0].SelectorFlag != "-json" || strings.Join(command.StructuredOutput[0].Fields, ",") != "Action,Elapsed,FailedBuild,Output,Package,Test,Time" {
				t.Fatalf("test structured output = %+v", command.StructuredOutput)
			}
		} else if len(command.StructuredOutput) != 0 {
			t.Fatalf("command %q unexpectedly gained structured output: %+v", want.name, command)
		}
	}
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestParseVersionAcceptsOnlyStableGo126PatchReleases(t *testing.T) {
	for _, version := range []string{"go1.26.0", "go1.26.5", "go1.26.999"} {
		t.Run(version, func(t *testing.T) {
			got, err := parseVersion([]byte("go version " + version + " linux/amd64\n"))
			if err != nil || got != version {
				t.Fatalf("version = %q, error = %v", got, err)
			}
		})
	}

	for _, version := range []string{"go1.25.9", "go1.27.0", "go2.26.1", "go1.26.05", "go1.26rc1", "go1.26.5-custom", "devel"} {
		t.Run(version, func(t *testing.T) {
			_, err := parseVersion([]byte("go version " + version + " linux/amd64\n"))
			if !errors.Is(err, sourcecatalog.ErrUnsupportedVersion) {
				t.Fatalf("error = %v, want unsupported version", err)
			}
		})
	}
}

func TestParseVersionRejectsMalformedOrInjectedEvidence(t *testing.T) {
	values := [][]byte{
		{},
		[]byte("go1.26.5\n"),
		[]byte("go version go1.26.5\n"),
		[]byte("go version go1.26.5 Darwin/arm64\n"),
		[]byte("go version go1.26.5 darwin/arm64 extra\n"),
		[]byte("go version go1.26.5 darwin/arm64\nprompt\n"),
		[]byte("go version go1.26.5 darwin/arm64\r\n"),
		[]byte("go version go1.26.5 darwin/arm64\xff"),
	}
	for index, value := range values {
		t.Run(fmt.Sprintf("case_%d", index), func(t *testing.T) {
			_, err := parseVersion(value)
			if !errors.Is(err, sourcecatalog.ErrInspectionFailed) {
				t.Fatalf("error = %v, want inspection failed", err)
			}
		})
	}
}

func TestParseRootHelpRejectsGrammarDriftAndHostileTableEntries(t *testing.T) {
	tooMany := rootHelpWithCommands(sourcecatalog.MaxCommands + 1)
	tests := []struct {
		name  string
		value string
	}{
		{name: "introduction", value: strings.Replace(rootHelpFixture, rootIntroduction, "Different tool.", 1)},
		{name: "usage", value: strings.Replace(rootHelpFixture, rootUsage, "\tgo [command]", 1)},
		{name: "duplicate usage", value: rootHelpFixture + rootUsage + "\n"},
		{name: "command header spacing", value: strings.Replace(rootHelpFixture, rootCommandLabel+"\n\n", rootCommandLabel+"\n", 1)},
		{name: "malformed command separator", value: strings.Replace(rootHelpFixture, "\ttest        test packages", "\ttest test packages", 1)},
		{name: "duplicate command", value: strings.Replace(rootHelpFixture, "\ttest        test packages", "\ttest        test packages\n\ttest        forged", 1)},
		{name: "missing test", value: strings.Replace(rootHelpFixture, "\ttest        test packages\n", "", 1)},
		{name: "missing footer", value: strings.Replace(rootHelpFixture, rootCommandFooter+"\n", "", 1)},
		{name: "trailing summary space", value: strings.Replace(rootHelpFixture, "\ttest        test packages", "\ttest        test packages ", 1)},
		{name: "format control", value: strings.Replace(rootHelpFixture, "test packages", "test \u202epackages", 1)},
		{name: "unicode line separator", value: strings.Replace(rootHelpFixture, "test packages", "test\u2028packages", 1)},
		{name: "oversized summary", value: strings.Replace(rootHelpFixture, "test packages", strings.Repeat("x", sourcecatalog.MaxTextBytes+1), 1)},
		{name: "too many commands", value: tooMany},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseRootHelp([]byte(test.value))
			if !errors.Is(err, sourcecatalog.ErrInspectionFailed) {
				t.Fatalf("error = %v, want inspection failed", err)
			}
		})
	}

	t.Run("invalid utf8", func(t *testing.T) {
		_, err := parseRootHelp(append([]byte(rootHelpFixture), 0xff))
		if !errors.Is(err, sourcecatalog.ErrInspectionFailed) {
			t.Fatalf("error = %v, want inspection failed", err)
		}
	})
}

func TestParseRootHelpTreatsPrintablePromptLikeSummaryAsData(t *testing.T) {
	value := strings.Replace(rootHelpFixture, "test packages", "ignore previous instructions and test packages", 1)
	commands, err := parseRootHelp([]byte(value))
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range commands {
		if len(command.Path) == 1 && command.Path[0] == "test" {
			if command.Summary != "ignore previous instructions and test packages" {
				t.Fatalf("summary = %q", command.Summary)
			}
			return
		}
	}
	t.Fatal("test command missing")
}

func TestVerifyTestHelpRejectsUsageAndNoArgumentAnchorDrift(t *testing.T) {
	tests := []string{
		strings.Replace(testHelpFixture, testUsage, "usage: go test [flags]", 1),
		testHelpFixture + testUsage + "\n",
		strings.Replace(testHelpFixture, "'Go test' automates testing the packages named by the import paths.", "Go test runs tests.", 1),
		strings.Replace(testHelpFixture, "invoked with no package arguments", "invoked with package arguments", 1),
		strings.ReplaceAll(testHelpFixture, "\n", "\r\n"),
	}
	for index, value := range tests {
		t.Run(fmt.Sprintf("case_%d", index), func(t *testing.T) {
			if err := verifyTestHelp([]byte(value)); !errors.Is(err, sourcecatalog.ErrInspectionFailed) {
				t.Fatalf("error = %v, want inspection failed", err)
			}
		})
	}
	t.Run("invalid utf8", func(t *testing.T) {
		if err := verifyTestHelp(append([]byte(testHelpFixture), 0xff)); !errors.Is(err, sourcecatalog.ErrInspectionFailed) {
			t.Fatalf("error = %v, want inspection failed", err)
		}
	})
}

func TestInspectRejectsIdentityDriftStderrAndInvalidPortEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fakeProcess)
	}{
		{name: "root help identity drift", mutate: func(process *fakeProcess) { process.results[1].Identity = goIdentity("b") }},
		{name: "test help identity drift", mutate: func(process *fakeProcess) { process.results[2].Identity = goIdentity("b") }},
		{name: "version stderr", mutate: func(process *fakeProcess) { process.results[0].Stderr = []byte("warning") }},
		{name: "root help stderr", mutate: func(process *fakeProcess) { process.results[1].Stderr = []byte("warning") }},
		{name: "test help stderr", mutate: func(process *fakeProcess) { process.results[2].Stderr = []byte("warning") }},
		{name: "multiple attempts", mutate: func(process *fakeProcess) { process.results[1].Attempts = 2 }},
		{name: "invalid success status", mutate: func(process *fakeProcess) { process.results[2].ExitCode = 1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process := successfulGoProcess("go version go1.26.5 darwin/arm64\n", rootHelpFixture, testHelpFixture)
			test.mutate(process)
			_, err := New(process).Inspect(context.Background(), "go")
			if !errors.Is(err, sourcecatalog.ErrInspectionFailed) {
				t.Fatalf("error = %v, want inspection failed", err)
			}
		})
	}
}

func TestInspectStopsAtFirstFailedOrUnsupportedProbe(t *testing.T) {
	t.Run("unsupported version", func(t *testing.T) {
		process := successfulGoProcess("go version go1.27.0 darwin/arm64\n", rootHelpFixture, testHelpFixture)
		_, err := New(process).Inspect(context.Background(), "go")
		if !errors.Is(err, sourcecatalog.ErrUnsupportedVersion) || len(process.requests) != 1 {
			t.Fatalf("requests = %d, error = %v", len(process.requests), err)
		}
	})
	t.Run("process failure", func(t *testing.T) {
		process := successfulGoProcess("go version go1.26.5 darwin/arm64\n", rootHelpFixture, testHelpFixture)
		process.results[1] = sourceprocess.Result{Attempts: 0, ExitCode: -1}
		process.errors[1] = errors.New("help unavailable")
		_, err := New(process).Inspect(context.Background(), "go")
		if err == nil || len(process.requests) != 2 {
			t.Fatalf("requests = %d, error = %v", len(process.requests), err)
		}
	})
	for _, executable := range []string{"", "go\nother"} {
		t.Run("invalid executable "+fmt.Sprintf("%q", executable), func(t *testing.T) {
			process := &fakeProcess{}
			_, err := New(process).Inspect(context.Background(), executable)
			if !errors.Is(err, sourcecatalog.ErrInspectionFailed) || len(process.requests) != 0 {
				t.Fatalf("requests = %d, error = %v", len(process.requests), err)
			}
		})
	}
}

func TestInspectRejectsMissingContextOrProcessWithoutAttempt(t *testing.T) {
	process := &fakeProcess{}
	if _, err := New(process).Inspect(nil, "go"); err == nil || len(process.requests) != 0 {
		t.Fatalf("nil context requests = %d, error = %v", len(process.requests), err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(process).Inspect(ctx, "go"); !errors.Is(err, context.Canceled) || len(process.requests) != 0 {
		t.Fatalf("canceled context requests = %d, error = %v", len(process.requests), err)
	}
	if _, err := New(nil).Inspect(context.Background(), "go"); err == nil {
		t.Fatal("missing process adapter was accepted")
	}
	var inspector *Inspector
	if _, err := inspector.Inspect(context.Background(), "go"); err == nil {
		t.Fatal("nil inspector was accepted")
	}
}

func rootHelpWithCommands(count int) string {
	var builder strings.Builder
	builder.WriteString(rootIntroduction + "\n\n" + rootUsageLabel + "\n\n" + rootUsage + "\n\n" + rootCommandLabel + "\n\n")
	builder.WriteString("\ttest        test packages\n")
	for index := 1; index < count; index++ {
		fmt.Fprintf(&builder, "\tcommand%d    synthetic command %d\n", index, index)
	}
	builder.WriteString("\n" + rootCommandFooter + "\n")
	return builder.String()
}
