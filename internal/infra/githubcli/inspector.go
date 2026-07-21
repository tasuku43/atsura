// Package githubcli adapts bounded GitHub CLI reference help to the
// vendor-neutral source catalog contract.
package githubcli

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
)

const (
	AdapterKind      = "atsura.source.github_cli"
	ContractVersion  = 2
	probeTimeout     = 5 * time.Second
	versionByteLimit = 64 * 1024
	helpByteLimit    = 256 * 1024
)

var versionPattern = regexp.MustCompile(`(?m)^gh version ([0-9]+)\.([0-9]+)\.([0-9]+)([-+][0-9A-Za-z.-]+)?(?: \(|$)`)

// ProcessPort starts one direct bounded source process.
type ProcessPort interface {
	Run(context.Context, sourceprocess.Request) (sourceprocess.Result, error)
}

// Inspector executes the fixed offline GitHub CLI probe set.
type Inspector struct {
	processes ProcessPort
}

// New creates a GitHub CLI source adapter.
func New(processes ProcessPort) *Inspector { return &Inspector{processes: processes} }

// Inspect runs exactly four offline probes and performs no provider task.
func (i *Inspector) Inspect(ctx context.Context, executable string) (sourcecatalog.Catalog, error) {
	if ctx == nil {
		return sourcecatalog.Catalog{}, fmt.Errorf("github cli inspection context is nil")
	}
	if err := ctx.Err(); err != nil {
		return sourcecatalog.Catalog{}, err
	}
	if i == nil || i.processes == nil {
		return sourcecatalog.Catalog{}, fmt.Errorf("github cli process adapter is not configured")
	}
	versionResult, err := i.runProbe(ctx, executable, []string{"version"}, versionByteLimit)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}
	version, major, err := parseVersion(versionResult.Stdout)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}
	if major != 2 {
		return sourcecatalog.Catalog{}, fmt.Errorf("%w: github cli major version %d", sourcecatalog.ErrUnsupportedVersion, major)
	}
	helpResult, err := i.runProbe(ctx, executable, []string{"help", "reference"}, helpByteLimit)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}
	if helpResult.Identity != versionResult.Identity {
		return sourcecatalog.Catalog{}, fmt.Errorf("%w: executable identity changed between probes", sourcecatalog.ErrInspectionFailed)
	}
	commands, err := parseReference(helpResult.Stdout)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}
	issueResult, err := i.runProbe(ctx, executable, []string{"issue", "list", "--help"}, helpByteLimit)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}
	if issueResult.Identity != versionResult.Identity {
		return sourcecatalog.Catalog{}, fmt.Errorf("%w: executable identity changed between probes", sourcecatalog.ErrInspectionFailed)
	}
	issueFields, err := parseJSONFields(issueResult.Stdout, []string{"issue", "list"})
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}
	commands, err = attachJSONFields(commands, []string{"issue", "list"}, issueFields)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}
	prResult, err := i.runProbe(ctx, executable, []string{"pr", "list", "--help"}, helpByteLimit)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}
	if prResult.Identity != versionResult.Identity {
		return sourcecatalog.Catalog{}, fmt.Errorf("%w: executable identity changed between probes", sourcecatalog.ErrInspectionFailed)
	}
	prFields, err := parseJSONFields(prResult.Stdout, []string{"pr", "list"})
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}
	commands, err = attachJSONFields(commands, []string{"pr", "list"}, prFields)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}
	catalog := sourcecatalog.Sort(sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: AdapterKind, ContractVersion: ContractVersion},
		Source: sourcecatalog.Source{
			RequestedExecutable: executable,
			ResolvedPath:        versionResult.Identity.ResolvedPath,
			SHA256:              versionResult.Identity.SHA256,
			Size:                versionResult.Identity.Size,
			Version:             version,
		},
		Probe: sourcecatalog.Probe{
			IDs:      []string{"version", "help_reference", "issue_list_help", "pr_list_help"},
			Attempts: 4,
		},
		Commands: commands,
	})
	if err := catalog.Validate(); err != nil {
		return sourcecatalog.Catalog{}, fmt.Errorf("%w: %v", sourcecatalog.ErrInvalidCatalog, err)
	}
	return catalog, nil
}

func attachJSONFields(commands []sourcecatalog.Command, path []string, fields []string) ([]sourcecatalog.Command, error) {
	matched := -1
	for index := range commands {
		if strings.Join(commands[index].Path, "\x00") != strings.Join(path, "\x00") {
			continue
		}
		if matched >= 0 {
			return nil, fmt.Errorf("%w: duplicate command %q in reference help", sourcecatalog.ErrInspectionFailed, strings.Join(path, " "))
		}
		matched = index
	}
	if matched < 0 {
		return nil, fmt.Errorf("%w: command %q is missing from reference help", sourcecatalog.ErrInspectionFailed, strings.Join(path, " "))
	}
	jsonOption := false
	for _, option := range commands[matched].Options {
		if option.Name == "--json" && option.TakesValue {
			jsonOption = true
		}
	}
	if !jsonOption {
		return nil, fmt.Errorf("%w: command %q does not declare a value-taking --json option", sourcecatalog.ErrInspectionFailed, strings.Join(path, " "))
	}
	commands[matched].StructuredOutput = []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: fields}}
	return commands, nil
}

func parseJSONFields(value []byte, expectedPath []string) ([]string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(value)))
	scanner.Buffer(make([]byte, 4096), helpByteLimit)
	wantUsage := "  gh " + strings.Join(expectedPath, " ") + " [flags]"
	usageMatches := 0
	jsonOptions := 0
	sectionMatches := 0
	collecting := false
	fieldText := strings.Builder{}
	for scanner.Scan() {
		line := scanner.Text()
		if line == wantUsage {
			usageMatches++
		}
		if strings.Contains(line, "--json fields") && strings.Contains(line, "Output JSON with the specified fields") {
			jsonOptions++
		}
		if line == "JSON FIELDS" {
			sectionMatches++
			collecting = true
			continue
		}
		if !collecting {
			continue
		}
		if strings.TrimSpace(line) == "" {
			collecting = false
			continue
		}
		if !strings.HasPrefix(line, "  ") {
			return nil, fmt.Errorf("%w: JSON FIELDS for %q is malformed", sourcecatalog.ErrInspectionFailed, strings.Join(expectedPath, " "))
		}
		if fieldText.Len() > 0 {
			fieldText.WriteByte(' ')
		}
		fieldText.WriteString(strings.TrimSpace(line))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: command help exceeds line bounds", sourcecatalog.ErrInspectionFailed)
	}
	if usageMatches != 1 || jsonOptions != 1 || sectionMatches != 1 {
		return nil, fmt.Errorf("%w: command help for %q does not match the JSON field contract", sourcecatalog.ErrInspectionFailed, strings.Join(expectedPath, " "))
	}
	text := fieldText.String()
	if text == "" || strings.HasPrefix(text, ",") || strings.HasSuffix(text, ",") {
		return nil, fmt.Errorf("%w: JSON FIELDS for %q is empty or malformed", sourcecatalog.ErrInspectionFailed, strings.Join(expectedPath, " "))
	}
	parts := strings.Split(text, ",")
	if len(parts) == 0 || len(parts) > 256 {
		return nil, fmt.Errorf("%w: JSON FIELDS for %q is empty or exceeds bounds", sourcecatalog.ErrInspectionFailed, strings.Join(expectedPath, " "))
	}
	fields := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		field := strings.TrimSpace(part)
		if !stableJSONField(field) {
			return nil, fmt.Errorf("%w: JSON field %q for %q is invalid", sourcecatalog.ErrInspectionFailed, field, strings.Join(expectedPath, " "))
		}
		if _, duplicate := seen[field]; duplicate {
			return nil, fmt.Errorf("%w: JSON field %q for %q is duplicated", sourcecatalog.ErrInspectionFailed, field, strings.Join(expectedPath, " "))
		}
		seen[field] = struct{}{}
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields, nil
}

func stableJSONField(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' ||
			(index > 0 && r >= '0' && r <= '9') || (index > 0 && (r == '-' || r == '.')) {
			continue
		}
		return false
	}
	return true
}

func (i *Inspector) runProbe(ctx context.Context, executable string, args []string, stdoutLimit int) (sourceprocess.Result, error) {
	request := sourceprocess.Request{Executable: executable, Args: args, Timeout: probeTimeout, StdoutLimit: stdoutLimit, StderrLimit: 64 * 1024}
	result, err := i.processes.Run(ctx, request)
	if validateErr := result.Validate(request, err == nil); validateErr != nil {
		return sourceprocess.Result{}, fmt.Errorf("%w: invalid process result: %v", sourcecatalog.ErrInspectionFailed, validateErr)
	}
	if err != nil {
		return sourceprocess.Result{}, err
	}
	if result.Attempts != 1 || len(result.Stderr) != 0 {
		return sourceprocess.Result{}, fmt.Errorf("%w: probe must succeed once without stderr", sourcecatalog.ErrInspectionFailed)
	}
	return result, nil
}

func parseVersion(value []byte) (string, int, error) {
	match := versionPattern.FindSubmatch(value)
	if match == nil {
		return "", 0, fmt.Errorf("%w: version output does not match the github cli contract", sourcecatalog.ErrInspectionFailed)
	}
	major, err := strconv.Atoi(string(match[1]))
	if err != nil {
		return "", 0, fmt.Errorf("%w: invalid major version", sourcecatalog.ErrInspectionFailed)
	}
	return string(match[1]) + "." + string(match[2]) + "." + string(match[3]) + string(match[4]), major, nil
}

type commandBuilder struct {
	command sourcecatalog.Command
	options map[string]sourcecatalog.Option
}

func parseReference(value []byte) ([]sourcecatalog.Command, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(value)))
	scanner.Buffer(make([]byte, 4096), helpByteLimit)
	firstContent := ""
	var current *commandBuilder
	commands := make([]sourcecatalog.Command, 0, 128)
	flush := func() {
		if current == nil {
			return
		}
		current.command.Options = make([]sourcecatalog.Option, 0, len(current.options))
		for _, option := range current.options {
			current.command.Options = append(current.command.Options, option)
		}
		sort.Slice(current.command.Options, func(i, j int) bool { return current.command.Options[i].Name < current.command.Options[j].Name })
		if _, exists := current.options["--json"]; exists {
			current.command.StructuredOutput = []sourcecatalog.StructuredOutput{{Format: "json", SelectorFlag: "--json", Fields: []string{}}}
		}
		commands = append(commands, current.command)
		current = nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if firstContent == "" && strings.TrimSpace(line) != "" {
			firstContent = line
		}
		level, heading, isHeading := referenceHeading(line)
		if isHeading {
			flush()
			path, leaf := parseHeadingPath(level, heading)
			if leaf {
				current = &commandBuilder{command: sourcecatalog.Command{
					Path: path, Summary: strings.Join(path, " "), Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
					Options: []sourcecatalog.Option{}, StructuredOutput: []sourcecatalog.StructuredOutput{},
				}, options: map[string]sourcecatalog.Option{}}
			}
			continue
		}
		if current == nil {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if current.command.Summary == strings.Join(current.command.Path, " ") && trimmed != "" && !strings.HasPrefix(trimmed, "-") && trimmed != "Aliases" {
			current.command.Summary = trimmed
		}
		for _, option := range parseLongOptions(line) {
			current.options[option.Name] = option
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: reference help exceeds line bounds", sourcecatalog.ErrInspectionFailed)
	}
	if firstContent != "# gh reference" || len(commands) == 0 || len(commands) > sourcecatalog.MaxCommands {
		return nil, fmt.Errorf("%w: reference help does not match the github cli grammar", sourcecatalog.ErrInspectionFailed)
	}
	return commands, nil
}

func referenceHeading(line string) (int, string, bool) {
	if strings.HasPrefix(line, "### gh ") {
		return 3, strings.TrimPrefix(line, "### gh "), true
	}
	if strings.HasPrefix(line, "## gh ") {
		return 2, strings.TrimPrefix(line, "## gh "), true
	}
	return 0, "", false
}

func parseHeadingPath(level int, heading string) ([]string, bool) {
	words := strings.Fields(heading)
	path := make([]string, 0, 4)
	for _, word := range words {
		if strings.HasPrefix(word, "[") || strings.HasPrefix(word, "<") || strings.HasPrefix(word, "{") || strings.HasPrefix(word, "-") {
			break
		}
		if !stableWord(word) {
			break
		}
		path = append(path, word)
	}
	if len(path) == 0 {
		return nil, false
	}
	if level == 2 && (strings.Contains(heading, "<command>") || strings.Contains(heading, "[subcommand]")) {
		return nil, false
	}
	return path, true
}

func parseLongOptions(line string) []sourcecatalog.Option {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "-") {
		return nil
	}
	syntax := optionSyntax(trimmed)
	fields := strings.Fields(syntax)
	result := make([]sourcecatalog.Option, 0, 2)
	for index, field := range fields {
		name := strings.TrimRight(field, ",")
		parts := strings.SplitN(name, "=", 2)
		if strings.HasPrefix(parts[0], "--") && stableWord(strings.TrimPrefix(parts[0], "--")) {
			takesValue := len(parts) == 2
			if !takesValue && index+1 < len(fields) {
				next := strings.TrimRight(fields[index+1], ",")
				takesValue = !strings.HasPrefix(next, "-")
			}
			result = append(result, sourcecatalog.Option{Name: parts[0], TakesValue: takesValue})
		}
	}
	return result
}

func optionSyntax(line string) string {
	for index := 0; index < len(line)-1; index++ {
		if line[index] == ' ' && line[index+1] == ' ' {
			return strings.TrimSpace(line[:index])
		}
	}
	return line
}

func stableWord(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if (r >= 'a' && r <= 'z') || (index > 0 && r >= '0' && r <= '9') || (index > 0 && r == '-') {
			continue
		}
		return false
	}
	return true
}
