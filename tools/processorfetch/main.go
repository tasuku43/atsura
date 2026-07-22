// Command processorfetch downloads one pinned external processor archive for
// native CI. It is intentionally absent from deterministic local gates.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/tasuku43/atsura/tools/internal/processormanifest"
)

const toolName = "processorfetch"

type options struct {
	target          string
	outputDirectory string
}

type dependencies struct {
	repositoryRoot string
	client         *http.Client
	fetchArchive   func(context.Context, string, processormanifest.TargetMetadata, *http.Client) (string, error)
}

type singleValue struct {
	value string
	set   bool
}

func (v *singleValue) String() string { return v.value }

func (v *singleValue) Set(value string) error {
	if v.set {
		return fmt.Errorf("argument was provided more than once")
	}
	v.value = value
	v.set = true
	return nil
}

func main() {
	repositoryRoot, err := os.Getwd()
	if err == nil {
		err = run(context.Background(), os.Args[1:], os.Stdout, dependencies{
			repositoryRoot: repositoryRoot,
			client:         newHTTPClient(nil),
			fetchArchive:   fetch,
		})
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", toolName, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer, deps dependencies) error {
	configuration, err := parseOptions(args)
	if err != nil {
		return err
	}
	manifest, err := processormanifest.LoadPinned(deps.repositoryRoot)
	if err != nil {
		return fmt.Errorf("processor manifest rejected")
	}
	metadata, err := manifest.Target(configuration.target)
	if err != nil {
		return fmt.Errorf("processor target rejected")
	}
	if deps.fetchArchive == nil {
		return fmt.Errorf("processor archive fetch dependency is unavailable")
	}
	path, err := deps.fetchArchive(ctx, configuration.outputDirectory, metadata, deps.client)
	if err != nil {
		return err
	}
	encoded := []byte(path + "\n")
	written, err := output.Write(encoded)
	if err != nil || written != len(encoded) {
		return fmt.Errorf("processor archive path output failed")
	}
	return nil
}

func parseOptions(args []string) (options, error) {
	var target, outputDirectory singleValue
	flags := flag.NewFlagSet(toolName, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Var(&target, "target", "exact supported processor target")
	flags.Var(&outputDirectory, "output-dir", "existing private absolute output directory")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !target.set || !outputDirectory.set {
		return options{}, fmt.Errorf("exactly one --target and --output-dir are required")
	}
	if !processormanifest.SupportedTarget(target.value) {
		return options{}, fmt.Errorf("--target is not a supported POSIX processor target")
	}
	if target.value != strings.TrimSpace(target.value) || outputDirectory.value == "" {
		return options{}, fmt.Errorf("invalid arguments")
	}
	return options{target: target.value, outputDirectory: outputDirectory.value}, nil
}
