package githubcli

import (
	"context"
	"errors"
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
	return f.results[index], f.errors[index]
}

func identity(seed string) sourceprocess.Identity {
	return sourceprocess.Identity{ResolvedPath: "/opt/bin/gh", SHA256: strings.Repeat(seed, 64), Size: 2048}
}

func successfulProcess(version, help string) *fakeProcess {
	id := identity("a")
	return &fakeProcess{
		results: []sourceprocess.Result{
			{Attempts: 1, ExitCode: 0, Stdout: []byte(version), Stderr: []byte{}, Identity: id},
			{Attempts: 1, ExitCode: 0, Stdout: []byte(help), Stderr: []byte{}, Identity: id},
		},
		errors: []error{nil, nil},
	}
}

const referenceFixture = `# gh reference

## gh api <endpoint> [flags]

Make an API request

  -X, --method string  HTTP method

## gh issue <command>

Manage issues

### gh issue list [flags]

List issues

      --json fields   Output JSON with the specified fields
      --web           List issues in the browser
  -R, --repo string   Select another repository
`

func TestInspectProducesVendorNeutralCatalogWithFixedProbes(t *testing.T) {
	process := successfulProcess("gh version 2.72.0 (2025-04-30)\n", referenceFixture)
	catalog, err := New(process).Inspect(context.Background(), "gh")
	if err != nil {
		t.Fatal(err)
	}
	if len(process.requests) != 2 || strings.Join(process.requests[0].Args, " ") != "version" || strings.Join(process.requests[1].Args, " ") != "help reference" {
		t.Fatalf("requests = %+v", process.requests)
	}
	if process.requests[0].Timeout != probeTimeout || process.requests[1].Timeout != probeTimeout ||
		process.requests[0].StdoutLimit != versionByteLimit || process.requests[1].StdoutLimit != helpByteLimit ||
		process.requests[0].StderrLimit != 64*1024 || process.requests[1].StderrLimit != 64*1024 {
		t.Fatalf("probe bounds = %+v", process.requests)
	}
	if catalog.Adapter.Kind != AdapterKind || catalog.Source.Version != "2.72.0" || catalog.Probe.Attempts != 2 || len(catalog.Commands) != 2 {
		t.Fatalf("catalog = %+v", catalog)
	}
	issue := catalog.Commands[1]
	if strings.Join(issue.Path, " ") != "issue list" || len(issue.StructuredOutput) != 1 || issue.StructuredOutput[0].Format != "json" {
		t.Fatalf("issue command = %+v", issue)
	}
	options := map[string]bool{}
	for _, option := range issue.Options {
		options[option.Name] = option.TakesValue
	}
	if !options["--json"] || !options["--repo"] || options["--web"] {
		t.Fatalf("option value grammar = %+v", options)
	}
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestInspectRejectsUnsupportedMalformedAndDriftingEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fakeProcess)
		want   error
	}{
		{name: "major", mutate: func(p *fakeProcess) { p.results[0].Stdout = []byte("gh version 3.0.0\n") }, want: sourcecatalog.ErrUnsupportedVersion},
		{name: "version", mutate: func(p *fakeProcess) { p.results[0].Stdout = []byte("hostile\n") }, want: sourcecatalog.ErrInspectionFailed},
		{name: "help", mutate: func(p *fakeProcess) { p.results[1].Stdout = []byte("# other reference\n") }, want: sourcecatalog.ErrInspectionFailed},
		{name: "drift", mutate: func(p *fakeProcess) { p.results[1].Identity = identity("b") }, want: sourcecatalog.ErrInspectionFailed},
		{name: "stderr", mutate: func(p *fakeProcess) { p.results[0].Stderr = []byte("warning") }, want: sourcecatalog.ErrInspectionFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process := successfulProcess("gh version 2.72.0\n", referenceFixture)
			test.mutate(process)
			_, err := New(process).Inspect(context.Background(), "gh")
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestInspectStopsAfterFailedFirstProbe(t *testing.T) {
	process := &fakeProcess{
		results: []sourceprocess.Result{{Attempts: 0, ExitCode: -1}},
		errors:  []error{errors.New("missing")},
	}
	_, err := New(process).Inspect(context.Background(), "gh")
	if err == nil || len(process.requests) != 1 {
		t.Fatalf("requests = %d, error = %v", len(process.requests), err)
	}
}
