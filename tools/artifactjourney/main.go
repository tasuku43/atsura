// Command artifactjourney verifies one exact Atsura release archive through a
// credential-free native source journey and an explicitly supplied pinned
// processor artifact. It makes no child-process, filesystem, or network-absence
// claim without an external observer.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/tasuku43/atsura/tools/internal/releaseversion"
)

const toolName = "artifactjourney"

type options struct {
	archive          string
	processorArchive string
	source           string
	tag              string
	revision         string
	goos             string
	goarch           string
	repositoryRoot   string
}

type singleOption struct {
	value string
	set   bool
}

func (o *singleOption) String() string { return o.value }

func (o *singleOption) Set(value string) error {
	if o.set {
		return fmt.Errorf("argument was provided more than once")
	}
	o.value = value
	o.set = true
	return nil
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", toolName, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	configuration, err := parseOptions(args)
	if err != nil {
		return err
	}
	repositoryRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("repository root is unavailable")
	}
	configuration.repositoryRoot = repositoryRoot
	if runtime.GOOS != configuration.goos || runtime.GOARCH != configuration.goarch {
		return fmt.Errorf("native host tuple does not match --goos/--goarch")
	}
	evidence, err := verifyArtifactJourney(ctx, configuration)
	if err != nil {
		return err
	}
	return writeEvidence(output, evidence)
}

func writeEvidence(output io.Writer, evidence evidenceDocument) error {
	encoded, err := json.Marshal(evidence)
	if err != nil || len(encoded)+1 > maxEvidenceBytes {
		return fmt.Errorf("evidence encoding failed")
	}
	encoded = append(encoded, '\n')
	written, err := output.Write(encoded)
	if err != nil || written != len(encoded) {
		return fmt.Errorf("evidence write failed")
	}
	return nil
}

func parseOptions(args []string) (options, error) {
	var result options
	var processorArchive singleOption
	flags := flag.NewFlagSet(toolName, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&result.archive, "archive", "", "exact release archive")
	flags.Var(&processorArchive, "processor-archive", "already-downloaded official processor archive")
	flags.StringVar(&result.source, "source", "", "native GitHub-compatible source fixture")
	flags.StringVar(&result.tag, "tag", "", "release tag")
	flags.StringVar(&result.revision, "revision", "", "full release revision")
	flags.StringVar(&result.goos, "goos", "", "release target operating system")
	flags.StringVar(&result.goarch, "goarch", "", "release target architecture")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return options{}, fmt.Errorf("invalid arguments")
	}
	if result.archive == "" || result.source == "" || result.tag == "" || result.revision == "" || result.goos == "" || result.goarch == "" {
		return options{}, fmt.Errorf("all six named arguments are required")
	}
	if _, err := releaseversion.ParseReleaseTag(result.tag); err != nil {
		return options{}, fmt.Errorf("--tag is invalid")
	}
	if !fullRevision(result.revision) {
		return options{}, fmt.Errorf("--revision must be a full lowercase commit SHA")
	}
	if !supportedTarget(result.goos, result.goarch) {
		return options{}, fmt.Errorf("--goos/--goarch is not a supported release target")
	}
	result.processorArchive = processorArchive.value
	if result.goos == "windows" {
		if processorArchive.set {
			return options{}, fmt.Errorf("--processor-archive is unsupported for Windows")
		}
	} else if !processorArchive.set || result.processorArchive == "" {
		return options{}, fmt.Errorf("--processor-archive is required for POSIX targets")
	}
	return result, nil
}

func fullRevision(value string) bool {
	return lowercaseHex(value, 40)
}

func lowercaseHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func supportedTarget(goos, goarch string) bool {
	switch goos + "/" + goarch {
	case "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64":
		return true
	default:
		return false
	}
}
