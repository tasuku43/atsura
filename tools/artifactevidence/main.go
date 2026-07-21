// Command artifactevidence validates and aggregates the five native artifact
// journey reports for one exact release identity.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

const toolName = "artifactevidence"

type options struct {
	directory string
	archives  string
	tag       string
	revision  string
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", toolName, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	configuration, err := parseOptions(args)
	if err != nil {
		return err
	}
	aggregate, err := collectEvidence(configuration)
	if err != nil {
		return err
	}
	encoded, err := encodeAggregate(aggregate)
	if err != nil {
		return err
	}
	written, err := output.Write(encoded)
	if err != nil || written != len(encoded) {
		return fmt.Errorf("aggregate write failed")
	}
	return nil
}

func parseOptions(args []string) (options, error) {
	var result options
	flags := flag.NewFlagSet(toolName, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&result.directory, "directory", "", "directory containing native artifact evidence")
	flags.StringVar(&result.archives, "archives", "", "directory containing the five candidate release archives")
	flags.StringVar(&result.tag, "tag", "", "release tag")
	flags.StringVar(&result.revision, "revision", "", "full release revision")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return options{}, fmt.Errorf("invalid arguments")
	}
	if result.directory == "" || result.archives == "" || result.tag == "" || result.revision == "" {
		return options{}, fmt.Errorf("all four named arguments are required")
	}
	if !validReleaseTag(result.tag) {
		return options{}, fmt.Errorf("--tag is invalid")
	}
	if !lowercaseHex(result.revision, revisionLength) {
		return options{}, fmt.Errorf("--revision must be a full lowercase commit SHA")
	}
	return result, nil
}
