// Command sourcefixture is a credential- and network-free GitHub CLI fixture
// used to replay Atsura's native source-inspection and transform runtime.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	attemptLogEnvironment = "ATSURA_SOURCE_FIXTURE_ATTEMPT_LOG"
	modeEnvironment       = "ATSURA_SOURCE_FIXTURE_MODE"

	modeSuccess        = "success"
	modeCommandFailure = "command_failure"
	modeStderr         = "stderr"
	modeMalformed      = "malformed"
	modeMissingField   = "missing_field"

	identityStreamCommand   = "identity source stream"
	appendOnlyStreamCommand = "append-only source stream"
	identityStreamStdout    = "ID:\x00\xff\n"
	identityStreamStderr    = "IDERR:\xfe"
	appendOnlyStreamStdout  = "APP:\xff\x00"
	appendOnlyStreamStderr  = "APPERR:\n"

	stdoutCanary     = "ATSURA_SECRET_STDOUT_CANARY"
	stderrCanary     = "ATSURA_SECRET_STDERR_CANARY"
	unselectedCanary = "ATSURA_SECRET_UNSELECTED_CANARY"

	exitOK             = 0
	exitUsage          = 2
	exitAppendOnly     = 23
	exitCommandFailure = 42
)

type attemptRecord struct {
	SchemaVersion int      `json:"schema_version"`
	Kind          string   `json:"kind"`
	Mode          string   `json:"mode"`
	Argv          []string `json:"argv"`
}

type invocation struct {
	kind     string
	response string
	runtime  bool
	command  string
}

type runtimeRecord struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	Ignored string `json:"ignored"`
}

type missingFieldRecord struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Ignored string `json:"ignored"`
}

func main() {
	if sensitiveEnvironmentPresent(os.Environ()) {
		_, _ = fmt.Fprintln(os.Stderr, "sourcefixture: credential-bearing environment is not allowed")
		os.Exit(exitUsage)
	}
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func sensitiveEnvironmentPresent(environment []string) bool {
	for _, item := range environment {
		key, _, present := strings.Cut(item, "=")
		if !present {
			continue
		}
		upper := strings.ToUpper(key)
		for _, marker := range []string{"TOKEN", "SECRET", "PASSWORD", "CREDENTIAL", "ACCESS_KEY", "PRIVATE_KEY"} {
			if strings.Contains(upper, marker) {
				return true
			}
		}
	}
	return false
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if getenv == nil || stdout == nil || stderr == nil {
		return exitUsage
	}
	mode, err := selectedMode(getenv(modeEnvironment))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sourcefixture: %v\n", err)
		return exitUsage
	}
	call, ok := classifyInvocation(args)
	if !ok {
		call = invocation{kind: "unsupported"}
	}
	if path := getenv(attemptLogEnvironment); path != "" {
		record := attemptRecord{
			SchemaVersion: 1,
			Kind:          call.kind,
			Mode:          mode,
			Argv:          append([]string{}, args...),
		}
		if record.Argv == nil {
			record.Argv = []string{}
		}
		if err := appendAttempt(path, record); err != nil {
			_, _ = fmt.Fprintf(stderr, "sourcefixture: append attempt log: %v\n", err)
			return exitUsage
		}
	}
	if !ok {
		_, _ = fmt.Fprintln(stderr, "sourcefixture: unsupported argv")
		return exitUsage
	}
	if !call.runtime {
		_, _ = io.WriteString(stdout, call.response)
		return exitOK
	}
	return runRuntime(call.command, mode, stdout, stderr)
}

func selectedMode(value string) (string, error) {
	if value == "" {
		return modeSuccess, nil
	}
	switch value {
	case modeSuccess, modeCommandFailure, modeStderr, modeMalformed, modeMissingField:
		return value, nil
	default:
		return "", fmt.Errorf("unsupported %s value %q", modeEnvironment, value)
	}
}

func classifyInvocation(args []string) (invocation, bool) {
	key := strings.Join(args, "\x00")
	switch key {
	case "version":
		return invocation{kind: "probe", response: versionFixture}, true
	case "help\x00reference":
		return invocation{kind: "probe", response: referenceFixture}, true
	case "issue\x00list\x00--help":
		return invocation{kind: "probe", response: issueListHelpFixture}, true
	case "pr\x00list\x00--help":
		return invocation{kind: "probe", response: prListHelpFixture}, true
	case "issue\x00list\x00--limit=1\x00--json=number,title,state":
		return invocation{kind: "runtime", runtime: true, command: "issue list"}, true
	case "pr\x00list\x00--limit=30\x00--json=number,title,state":
		return invocation{kind: "runtime", runtime: true, command: "pr list"}, true
	case "pr\x00list\x00--limit=2\x00--json=number,title,state":
		return invocation{kind: "runtime", runtime: true, command: "pr list"}, true
	case "pr\x00list\x00--search=space value;$(touch atsura-artifact-injection)\x00--label=first\x00--label=Unicode 雪\x00--repo=-dash":
		return invocation{kind: "runtime", runtime: true, command: identityStreamCommand}, true
	case "issue\x00list\x00--search=append value\x00--label=one\x00--label=two\x00--limit=1":
		return invocation{kind: "runtime", runtime: true, command: appendOnlyStreamCommand}, true
	default:
		return invocation{}, false
	}
}

func runRuntime(command, mode string, stdout, stderr io.Writer) int {
	if mode == modeSuccess {
		switch command {
		case identityStreamCommand:
			_, _ = io.WriteString(stdout, identityStreamStdout)
			_, _ = io.WriteString(stderr, identityStreamStderr)
			return exitOK
		case appendOnlyStreamCommand:
			_, _ = io.WriteString(stdout, appendOnlyStreamStdout)
			_, _ = io.WriteString(stderr, appendOnlyStreamStderr)
			return exitAppendOnly
		}
	}
	if mode == modeCommandFailure {
		_, _ = io.WriteString(stdout, stdoutCanary)
		_, _ = io.WriteString(stderr, stderrCanary)
		return exitCommandFailure
	}
	if mode == modeMalformed {
		_, _ = io.WriteString(stdout, `[{`+stdoutCanary)
		return exitOK
	}
	record := fixtureRecord(command)
	if mode == modeMissingField {
		value := []missingFieldRecord{{Number: record.Number, Title: record.Title, Ignored: record.Ignored}}
		if err := json.NewEncoder(stdout).Encode(value); err != nil {
			return exitCommandFailure
		}
	} else if err := json.NewEncoder(stdout).Encode([]runtimeRecord{record}); err != nil {
		return exitCommandFailure
	}
	if mode == modeStderr {
		_, _ = fmt.Fprintln(stderr, stderrCanary)
	}
	return exitOK
}

func fixtureRecord(command string) runtimeRecord {
	if command == "issue list" {
		return runtimeRecord{
			Number: 202, Title: "Fix deterministic wrapper", State: "OPEN", Ignored: unselectedCanary,
		}
	}
	return runtimeRecord{
		Number: 101, Title: "Review policy", State: "OPEN", Ignored: unselectedCanary,
	}
}

func appendAttempt(path string, record attemptRecord) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("%s must be an absolute clean path", attemptLogEnvironment)
	}
	parent, name := filepath.Split(path)
	if name == "" {
		return fmt.Errorf("%s must name a file", attemptLogEnvironment)
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer root.Close()

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	for attempt := 0; attempt < 2; attempt++ {
		info, inspectErr := root.Lstat(name)
		if inspectErr == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return fmt.Errorf("attempt log must be a regular file")
			}
			file, openErr := root.OpenFile(name, os.O_WRONLY|os.O_APPEND, 0)
			if openErr != nil {
				return openErr
			}
			opened, statErr := file.Stat()
			if statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
				_ = file.Close()
				return fmt.Errorf("attempt log changed while opening: %w", statErr)
			}
			return appendRecord(file, data)
		}
		if !errors.Is(inspectErr, fs.ErrNotExist) {
			return inspectErr
		}
		file, createErr := root.OpenFile(name, os.O_WRONLY|os.O_APPEND|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(createErr, fs.ErrExist) {
			continue
		}
		if createErr != nil {
			return createErr
		}
		return appendRecord(file, data)
	}
	return fmt.Errorf("attempt log changed while opening")
}

func appendRecord(file *os.File, data []byte) error {
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

const versionFixture = "gh version 2.72.0 (Atsura synthetic source fixture)\n"

const referenceFixture = `# gh reference

## gh api <endpoint> [flags]

Make an API request

  -X, --method string  HTTP method

## gh issue <command>

Work with issues

### gh issue list [flags]

List issues

      --app string          Filter by app
      --assignee string     Filter by assignee
      --author string       Filter by author
      --jq expression       Filter JSON output
      --json fields         Output JSON with the specified fields
      --label strings       Filter by label
  -L, --limit int           Maximum number of issues
  -R, --repo repository     Select another repository
      --search query        Search issues
      --state string        Filter by state
      --template string     Format JSON output
      --web                 Open in a browser

## gh pr <command>

Work with pull requests

### gh pr list [flags]

List pull requests

      --app string          Filter by app
      --assignee string     Filter by assignee
      --author string       Filter by author
      --base string         Filter by base branch
      --draft               Filter by draft state
      --head string         Filter by head branch
      --jq expression       Filter JSON output
      --json fields         Output JSON with the specified fields
      --label strings       Filter by label
  -L, --limit int           Maximum number of pull requests
  -R, --repo repository     Select another repository
      --search query        Search pull requests
      --state string        Filter by state
      --template string     Format JSON output
      --web                 Open in a browser
`

const issueListHelpFixture = `List issues.

USAGE
  gh issue list [flags]

FLAGS
      --json fields   Output JSON with the specified fields

JSON FIELDS
  assignees, author, body, id, number, state, title, updatedAt, url

EXAMPLES
  $ gh issue list
`

const prListHelpFixture = `List pull requests.

USAGE
  gh pr list [flags]

FLAGS
      --json fields   Output JSON with the specified fields

JSON FIELDS
  additions, author, changedFiles, id, isDraft, number, state, title, updatedAt, url

EXAMPLES
  $ gh pr list
`
