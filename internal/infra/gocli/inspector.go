// Package gocli adapts bounded Go command help to the vendor-neutral source
// catalog contract.
package gocli

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
)

const (
	// AdapterKind is the stable namespaced identifier for Go CLI inspection.
	AdapterKind = "atsura.source.go_cli"
	// ContractVersion adds exact go_test_jsonl selector evidence while retaining
	// the same three bounded probes and inspection-time version boundary.
	ContractVersion = 2

	probeTimeout     = 5 * time.Second
	versionByteLimit = 64 * 1024
	helpByteLimit    = 256 * 1024
	stderrByteLimit  = 64 * 1024
)

const (
	rootIntroduction  = "Go is a tool for managing Go source code."
	rootUsageLabel    = "Usage:"
	rootUsage         = "\tgo <command> [arguments]"
	rootCommandLabel  = "The commands are:"
	rootCommandFooter = `Use "go help <command>" for more information about a command.`

	testUsage = "usage: go test [build/test flags] [packages] [build/test flags & test binary flags]"
)

var stableGoVersionPattern = regexp.MustCompile(`^go(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var goPlatformPattern = regexp.MustCompile(`^[a-z0-9]+/[a-z0-9]+$`)

// ProcessPort starts one direct bounded source process.
type ProcessPort interface {
	Run(context.Context, sourceprocess.Request) (sourceprocess.Result, error)
}

// Inspector executes the fixed offline Go CLI probe set.
type Inspector struct {
	processes ProcessPort
}

// New creates a Go CLI source adapter.
func New(processes ProcessPort) *Inspector { return &Inspector{processes: processes} }

// Inspect runs the exact version, root-help, and test-help probes. It does not
// run a caller-selected Go task.
func (i *Inspector) Inspect(ctx context.Context, executable string) (sourcecatalog.Catalog, error) {
	if ctx == nil {
		return sourcecatalog.Catalog{}, fmt.Errorf("go cli inspection context is nil")
	}
	if err := ctx.Err(); err != nil {
		return sourcecatalog.Catalog{}, err
	}
	if i == nil || i.processes == nil {
		return sourcecatalog.Catalog{}, fmt.Errorf("go cli process adapter is not configured")
	}

	versionResult, err := i.runProbe(ctx, executable, []string{"version"}, versionByteLimit)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}
	// This is the effective version reported under the inspection cwd and
	// environment. The separately bound executable identity names the direct
	// launcher; later toolchain selection remains source-owned behavior.
	version, err := parseVersion(versionResult.Stdout)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}

	helpResult, err := i.runProbe(ctx, executable, []string{"help"}, helpByteLimit)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}
	if helpResult.Identity != versionResult.Identity {
		return sourcecatalog.Catalog{}, fmt.Errorf("%w: executable identity changed between probes", sourcecatalog.ErrInspectionFailed)
	}
	commands, err := parseRootHelp(helpResult.Stdout)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}

	testHelpResult, err := i.runProbe(ctx, executable, []string{"help", "test"}, helpByteLimit)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}
	if testHelpResult.Identity != versionResult.Identity {
		return sourcecatalog.Catalog{}, fmt.Errorf("%w: executable identity changed between probes", sourcecatalog.ErrInspectionFailed)
	}
	if err := verifyTestHelp(testHelpResult.Stdout); err != nil {
		return sourcecatalog.Catalog{}, err
	}
	commands, err = attachTestJSONOutput(commands)
	if err != nil {
		return sourcecatalog.Catalog{}, err
	}

	catalog := sourcecatalog.Sort(sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter: sourcecatalog.Adapter{
			Kind:            AdapterKind,
			ContractVersion: ContractVersion,
		},
		Source: sourcecatalog.Source{
			RequestedExecutable: executable,
			ResolvedPath:        versionResult.Identity.ResolvedPath,
			SHA256:              versionResult.Identity.SHA256,
			Size:                versionResult.Identity.Size,
			Version:             version,
		},
		Probe: sourcecatalog.Probe{
			IDs:      []string{"version", "help", "test_help"},
			Attempts: 3,
		},
		Commands: commands,
	})
	if err := catalog.Validate(); err != nil {
		return sourcecatalog.Catalog{}, fmt.Errorf("%w: %v", sourcecatalog.ErrInvalidCatalog, err)
	}
	return catalog, nil
}

func (i *Inspector) runProbe(ctx context.Context, executable string, args []string, stdoutLimit int) (sourceprocess.Result, error) {
	request := sourceprocess.Request{
		Executable:  executable,
		Args:        args,
		Timeout:     probeTimeout,
		StdoutLimit: stdoutLimit,
		StderrLimit: stderrByteLimit,
	}
	if err := request.Validate(); err != nil {
		return sourceprocess.Result{}, fmt.Errorf("%w: invalid probe request", sourcecatalog.ErrInspectionFailed)
	}
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

func parseVersion(value []byte) (string, error) {
	if len(value) == 0 || len(value) > versionByteLimit || !utf8.Valid(value) || strings.ContainsRune(string(value), '\r') {
		return "", fmt.Errorf("%w: version output does not match the go cli contract", sourcecatalog.ErrInspectionFailed)
	}
	text := strings.TrimSuffix(string(value), "\n")
	if strings.ContainsRune(text, '\n') {
		return "", fmt.Errorf("%w: version output does not match the go cli contract", sourcecatalog.ErrInspectionFailed)
	}
	fields := strings.Fields(text)
	if len(fields) != 4 || fields[0] != "go" || fields[1] != "version" || !goPlatformPattern.MatchString(fields[3]) {
		return "", fmt.Errorf("%w: version output does not match the go cli contract", sourcecatalog.ErrInspectionFailed)
	}
	match := stableGoVersionPattern.FindStringSubmatch(fields[2])
	if match == nil {
		return "", fmt.Errorf("%w: go cli version is not a stable release", sourcecatalog.ErrUnsupportedVersion)
	}
	major, majorErr := strconv.Atoi(match[1])
	minor, minorErr := strconv.Atoi(match[2])
	if majorErr != nil || minorErr != nil || major != 1 || minor != 26 {
		return "", fmt.Errorf("%w: go cli version is outside go1.26.x", sourcecatalog.ErrUnsupportedVersion)
	}
	return fields[2], nil
}

func parseRootHelp(value []byte) ([]sourcecatalog.Command, error) {
	lines, err := boundedHelpLines(value, "root help")
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 || lines[0] != rootIntroduction {
		return nil, inspectionFailure("root help introduction is missing or changed")
	}
	usageLabel := uniqueLine(lines, rootUsageLabel)
	usage := uniqueLine(lines, rootUsage)
	commandLabel := uniqueLine(lines, rootCommandLabel)
	commandFooter := uniqueLine(lines, rootCommandFooter)
	if usageLabel < 0 || usage < 0 || commandLabel < 0 || commandFooter < 0 ||
		usage != usageLabel+2 || commandLabel <= usage || commandLabel+2 >= len(lines) {
		return nil, inspectionFailure("root help does not match the documented command grammar")
	}
	if lines[usageLabel+1] != "" || lines[commandLabel+1] != "" {
		return nil, inspectionFailure("root help command grammar spacing changed")
	}

	commands := make([]sourcecatalog.Command, 0, 32)
	seen := make(map[string]struct{}, 32)
	index := commandLabel + 2
	for ; index < len(lines) && lines[index] != ""; index++ {
		name, summary, ok := parseCommandLine(lines[index])
		if !ok {
			return nil, inspectionFailure("root command table contains a malformed entry")
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, inspectionFailure("root command table contains a duplicate command")
		}
		seen[name] = struct{}{}
		commands = append(commands, sourcecatalog.Command{
			Path:             []string{name},
			Summary:          summary,
			Provenance:       sourcecatalog.ProvenanceVerifiedBuiltin,
			Options:          []sourcecatalog.Option{},
			StructuredOutput: []sourcecatalog.StructuredOutput{},
		})
		if len(commands) > sourcecatalog.MaxCommands {
			return nil, inspectionFailure("root command table exceeds its bound")
		}
	}
	if len(commands) == 0 || index+1 >= len(lines) || index+1 != commandFooter {
		return nil, inspectionFailure("root command table is empty or lacks its exact footer")
	}
	if _, found := seen["test"]; !found {
		return nil, inspectionFailure("root command table does not contain test")
	}
	return commands, nil
}

func parseCommandLine(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "\t\t") {
		return "", "", false
	}
	body := strings.TrimPrefix(line, "\t")
	separator := strings.Index(body, "  ")
	if separator <= 0 {
		return "", "", false
	}
	name := body[:separator]
	summary := strings.TrimLeft(body[separator:], " ")
	if !stableCommandName(name) || !safeSummary(summary) || summary != strings.TrimSpace(summary) {
		return "", "", false
	}
	return name, summary, true
}

func verifyTestHelp(value []byte) error {
	lines, err := boundedHelpLines(value, "test help")
	if err != nil {
		return err
	}
	required := []string{
		testUsage,
		"'Go test' automates testing the packages named by the import paths.",
		"The first, called local directory mode, occurs when go test is",
		"invoked with no package arguments (for example, 'go test' or 'go",
		"test -v'). In this mode, go test compiles the package sources and",
		"tests found in the current directory and then runs the resulting",
		"test binary. In this mode, caching (discussed below) is disabled.",
		"\t-json",
		"\t    Convert test output to JSON suitable for automated processing.",
		"\t    Also emits build output in JSON. See 'go help buildjson'.",
	}
	previous := -1
	for _, anchor := range required {
		index := uniqueLine(lines, anchor)
		if index < 0 || index <= previous {
			return inspectionFailure("test help does not match the no-argument test contract")
		}
		previous = index
	}
	if len(lines) == 0 || lines[0] != testUsage {
		return inspectionFailure("test help usage is missing or changed")
	}
	return nil
}

func attachTestJSONOutput(commands []sourcecatalog.Command) ([]sourcecatalog.Command, error) {
	matched := -1
	for index := range commands {
		if len(commands[index].Path) != 1 || commands[index].Path[0] != "test" {
			continue
		}
		if matched >= 0 {
			return nil, inspectionFailure("root command table contains duplicate test entries")
		}
		matched = index
	}
	if matched < 0 {
		return nil, inspectionFailure("root command table does not contain test")
	}
	commands[matched].StructuredOutput = []sourcecatalog.StructuredOutput{{
		Format:       "go_test_jsonl",
		SelectorFlag: "-json",
		Fields:       []string{"Action", "Elapsed", "FailedBuild", "Output", "Package", "Test", "Time"},
	}}
	return commands, nil
}

func boundedHelpLines(value []byte, description string) ([]string, error) {
	if len(value) == 0 || len(value) > helpByteLimit || !utf8.Valid(value) || strings.ContainsRune(string(value), '\r') {
		return nil, inspectionFailure(description + " is not bounded UTF-8 with canonical newlines")
	}
	text := strings.TrimSuffix(string(value), "\n")
	return strings.Split(text, "\n"), nil
}

func uniqueLine(lines []string, wanted string) int {
	found := -1
	for index, line := range lines {
		if line != wanted {
			continue
		}
		if found >= 0 {
			return -1
		}
		found = index
	}
	return found
}

func stableCommandName(value string) bool {
	if value == "" || len(value) > 128 {
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

func safeSummary(value string) bool {
	if value == "" || len(value) > sourcecatalog.MaxTextBytes || !utf8.ValidString(value) {
		return false
	}
	return strings.IndexFunc(value, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '\u2028' || r == '\u2029'
	}) < 0
}

func inspectionFailure(message string) error {
	return fmt.Errorf("%w: %s", sourcecatalog.ErrInspectionFailed, message)
}
