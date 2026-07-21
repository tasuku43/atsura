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

func successfulProcess(version, help, issueHelp, prHelp string) *fakeProcess {
	id := identity("a")
	return &fakeProcess{
		results: []sourceprocess.Result{
			{Attempts: 1, ExitCode: 0, Stdout: []byte(version), Stderr: []byte{}, Identity: id},
			{Attempts: 1, ExitCode: 0, Stdout: []byte(help), Stderr: []byte{}, Identity: id},
			{Attempts: 1, ExitCode: 0, Stdout: []byte(issueHelp), Stderr: []byte{}, Identity: id},
			{Attempts: 1, ExitCode: 0, Stdout: []byte(prHelp), Stderr: []byte{}, Identity: id},
		},
		errors: []error{nil, nil, nil, nil},
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

## gh pr <command>

Manage pull requests

### gh pr list [flags]

List pull requests

      --json fields   Output JSON with the specified fields
  -R, --repo string   Select another repository
`

const issueListHelpFixture = `List issues.

USAGE
  gh issue list [flags]

FLAGS
      --json fields   Output JSON with the specified fields

JSON FIELDS
  assignees, author, body, closed, closedAt, comments, createdAt, id, isPinned,
  labels, milestone, number, projectCards, projectItems, reactionGroups, state,
  stateReason, title, updatedAt, url

EXAMPLES
  $ gh issue list
`

const prListHelpFixture = `List pull requests.

USAGE
  gh pr list [flags]

FLAGS
      --json fields   Output JSON with the specified fields

JSON FIELDS
  additions, assignees, author, autoMergeRequest, baseRefName, body, changedFiles,
  id, isDraft, number, reviewDecision, state, title, updatedAt, url

EXAMPLES
  $ gh pr list
`

func TestInspectProducesVendorNeutralCatalogWithFixedProbes(t *testing.T) {
	process := successfulProcess("gh version 2.72.0 (2025-04-30)\n", referenceFixture, issueListHelpFixture, prListHelpFixture)
	catalog, err := New(process).Inspect(context.Background(), "gh")
	if err != nil {
		t.Fatal(err)
	}
	if len(process.requests) != 4 || strings.Join(process.requests[0].Args, " ") != "version" || strings.Join(process.requests[1].Args, " ") != "help reference" ||
		strings.Join(process.requests[2].Args, " ") != "issue list --help" || strings.Join(process.requests[3].Args, " ") != "pr list --help" {
		t.Fatalf("requests = %+v", process.requests)
	}
	for index, request := range process.requests {
		wantStdoutLimit := helpByteLimit
		if index == 0 {
			wantStdoutLimit = versionByteLimit
		}
		if request.Timeout != probeTimeout || request.StdoutLimit != wantStdoutLimit || request.StderrLimit != 64*1024 {
			t.Fatalf("probe %d bounds = %+v", index, request)
		}
	}
	if catalog.Adapter.Kind != AdapterKind || catalog.Adapter.ContractVersion != 2 || catalog.Source.Version != "2.72.0" || catalog.Probe.Attempts != 4 || len(catalog.Commands) != 3 {
		t.Fatalf("catalog = %+v", catalog)
	}
	issue := catalog.Commands[1]
	if strings.Join(issue.Path, " ") != "issue list" || len(issue.StructuredOutput) != 1 || issue.StructuredOutput[0].Format != "json" {
		t.Fatalf("issue command = %+v", issue)
	}
	if got := strings.Join(issue.StructuredOutput[0].Fields, ","); got != "assignees,author,body,closed,closedAt,comments,createdAt,id,isPinned,labels,milestone,number,projectCards,projectItems,reactionGroups,state,stateReason,title,updatedAt,url" {
		t.Fatalf("issue fields = %q", got)
	}
	pr := catalog.Commands[2]
	if strings.Join(pr.Path, " ") != "pr list" || len(pr.StructuredOutput) != 1 {
		t.Fatalf("pr command = %+v", pr)
	}
	if got := strings.Join(pr.StructuredOutput[0].Fields, ","); got != "additions,assignees,author,autoMergeRequest,baseRefName,body,changedFiles,id,isDraft,number,reviewDecision,state,title,updatedAt,url" {
		t.Fatalf("pr fields = %q", got)
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
		{name: "reference drift", mutate: func(p *fakeProcess) { p.results[1].Identity = identity("b") }, want: sourcecatalog.ErrInspectionFailed},
		{name: "issue drift", mutate: func(p *fakeProcess) { p.results[2].Identity = identity("b") }, want: sourcecatalog.ErrInspectionFailed},
		{name: "pr drift", mutate: func(p *fakeProcess) { p.results[3].Identity = identity("b") }, want: sourcecatalog.ErrInspectionFailed},
		{name: "stderr", mutate: func(p *fakeProcess) { p.results[0].Stderr = []byte("warning") }, want: sourcecatalog.ErrInspectionFailed},
		{name: "missing pr command", mutate: func(p *fakeProcess) {
			p.results[1].Stdout = []byte(strings.Replace(referenceFixture, "### gh pr list [flags]", "### gh pr view [flags]", 1))
		}, want: sourcecatalog.ErrInspectionFailed},
		{name: "missing issue selector", mutate: func(p *fakeProcess) {
			p.results[1].Stdout = []byte(strings.Replace(referenceFixture, "      --json fields   Output JSON with the specified fields", "      --web           List issues in the browser", 1))
		}, want: sourcecatalog.ErrInspectionFailed},
		{name: "issue fields", mutate: func(p *fakeProcess) {
			p.results[2].Stdout = []byte(strings.Replace(issueListHelpFixture, "number,", "number, number,", 1))
		}, want: sourcecatalog.ErrInspectionFailed},
		{name: "pr usage", mutate: func(p *fakeProcess) {
			p.results[3].Stdout = []byte(strings.Replace(prListHelpFixture, "gh pr list [flags]", "gh issue list [flags]", 1))
		}, want: sourcecatalog.ErrInspectionFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process := successfulProcess("gh version 2.72.0\n", referenceFixture, issueListHelpFixture, prListHelpFixture)
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

func TestInspectStopsAfterFailedFieldProbe(t *testing.T) {
	process := successfulProcess("gh version 2.72.0\n", referenceFixture, issueListHelpFixture, prListHelpFixture)
	process.results[2] = sourceprocess.Result{Attempts: 0, ExitCode: -1}
	process.errors[2] = errors.New("help unavailable")
	_, err := New(process).Inspect(context.Background(), "gh")
	if err == nil || len(process.requests) != 3 {
		t.Fatalf("requests = %d, error = %v", len(process.requests), err)
	}
}
