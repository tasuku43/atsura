package planapply

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

func sourceStreamFixture(t *testing.T, kind tailoringbundle.WrapperKind, appendedArgs []string) (tailoringbundle.Bundle, string, sourceprocess.Identity) {
	t.Helper()
	identity := sourceprocess.Identity{
		ResolvedPath: "/opt/bin/fixture",
		SHA256:       strings.Repeat("a", 64),
		Size:         42,
	}
	catalog := sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: "atsura.source.alternate", ContractVersion: 1},
		Source: sourcecatalog.Source{
			RequestedExecutable: "fixture",
			ResolvedPath:        identity.ResolvedPath,
			SHA256:              identity.SHA256,
			Size:                identity.Size,
			Version:             "1.0.0",
		},
		Probe: sourcecatalog.Probe{IDs: []string{"help"}, Attempts: 1},
		Commands: []sourcecatalog.Command{{
			Path:             []string{"item", "list"},
			Summary:          "List items",
			Provenance:       sourcecatalog.ProvenanceVerifiedBuiltin,
			Options:          []sourcecatalog.Option{{Name: "--format", TakesValue: true}},
			StructuredOutput: []sourcecatalog.StructuredOutput{},
		}},
	}
	catalogDigest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := tailoringbundle.Compile(catalog, tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: catalogDigest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands: []tailoringbundle.CommandEntry{{
			Command:  []string{"item", "list"},
			Presence: tailoringbundle.PresenceInclude,
			Reason:   "Preserve the source command stream.",
			Options: &tailoringbundle.OptionSurface{
				Default: tailoringbundle.SurfaceDefaultInherit,
				Include: []string{},
				Exclude: []string{},
			},
			Wrapper: &tailoringbundle.Wrapper{
				Kind:   kind,
				Before: []tailoringbundle.StageAction{},
				Invoke: tailoringbundle.Invocation{AppendArgs: append([]string{}, appendedArgs...)},
				After:  []tailoringbundle.StageAction{},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return bundle, digest, identity
}

func sourceStreamService(t *testing.T, kind tailoringbundle.WrapperKind, appendedArgs []string, process *processStub, parser *parserStub) (*Service, Request, *compatibilityStub) {
	t.Helper()
	bundle, digest, identity := sourceStreamFixture(t, kind, appendedArgs)
	compatibility := &compatibilityStub{}
	service := New(
		&bundleStub{bundle: bundle, digest: digest},
		&adoptionStub{state: bundletrust.StateAdopted},
		&identityStub{value: identity},
		compatibility,
		process,
		parser,
	)
	return service, Request{
		BundlePath:                   "purpose.bundle.json",
		ExpectedBundleDigest:         digest,
		AllowSourceStreamPassthrough: true,
		Attempt: tailoringplan.Attempt{
			Executable: "fixture",
			Args:       []string{"item", "list"},
		},
		Command: testCommandContext(),
	}, compatibility
}

func TestApplyReturnsIdentitySourceStreamWithExactDetachedBytes(t *testing.T) {
	stdout := []byte{0x00, 0xff, 'o', 'u', 't', '\n'}
	stderr := []byte{0xfe, 'w', 'a', 'r', 'n', '\n'}
	identity := sourceprocess.Identity{ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42}
	process := &processStub{result: sourceprocess.Result{
		Attempts: 1,
		ExitCode: 0,
		Stdout:   append([]byte{}, stdout...),
		Stderr:   append([]byte{}, stderr...),
		Identity: identity,
	}}
	parser := &parserStub{}
	service, request, compatibility := sourceStreamService(t, tailoringbundle.WrapperIdentity, []string{}, process, parser)

	result, err := service.Apply(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.ResultMode != tailoringplan.ResultModeSourceStreamPassthrough || result.WrapperKind != tailoringbundle.WrapperIdentity || result.SourceStream == nil || result.TransformedJSON != nil || result.SourceStream.ExitCode != 0 {
		t.Fatalf("result=%+v", result)
	}
	if !bytes.Equal(result.SourceStream.Stdout, stdout) || !bytes.Equal(result.SourceStream.Stderr, stderr) {
		t.Fatalf("stream=%+v", result.SourceStream)
	}
	if compatibility.calls != 1 || process.calls != 1 || parser.calls != 0 {
		t.Fatalf("compatibility/process/parser calls=%d/%d/%d", compatibility.calls, process.calls, parser.calls)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result validation: %v", err)
	}
	result.SourceStream.Stdout[0] = 'x'
	result.SourceStream.Stderr[0] = 'y'
	if process.result.Stdout[0] != stdout[0] || process.result.Stderr[0] != stderr[0] {
		t.Fatalf("returned stream aliases process storage: result=%+v process=%+v", result.SourceStream, process.result)
	}
}

func TestApplyReturnsAppendOnlyNonzeroSourceStreamAndExactInvocation(t *testing.T) {
	identity := sourceprocess.Identity{ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42}
	stdout := []byte("partial source stdout\n")
	stderr := []byte("source diagnostic\n")
	process := &processStub{
		result: sourceprocess.Result{Attempts: 1, ExitCode: 23, Stdout: stdout, Stderr: stderr, Identity: identity},
		err:    fault.New(fault.KindRejected, "source_command_failed", "untrusted adapter detail", false),
	}
	parser := &parserStub{}
	service, request, compatibility := sourceStreamService(t, tailoringbundle.WrapperTransform, []string{"--format=json"}, process, parser)

	result, err := service.Apply(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.ResultMode != tailoringplan.ResultModeSourceStreamPassthrough || result.WrapperKind != tailoringbundle.WrapperTransform || result.SourceStream == nil || result.TransformedJSON != nil || result.SourceStream.ExitCode != 23 {
		t.Fatalf("result=%+v", result)
	}
	if !bytes.Equal(result.SourceStream.Stdout, stdout) || !bytes.Equal(result.SourceStream.Stderr, stderr) {
		t.Fatalf("stream=%+v", result.SourceStream)
	}
	if got, want := process.request.Process.Args, []string{"item", "list", "--format=json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("source argv=%#v want %#v", got, want)
	}
	if compatibility.calls != 1 || process.calls != 1 || parser.calls != 0 {
		t.Fatalf("compatibility/process/parser calls=%d/%d/%d", compatibility.calls, process.calls, parser.calls)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result validation: %v", err)
	}
}

func TestApplySuppressesSourceBytesForUnknownOrInconsistentStreamErrors(t *testing.T) {
	identity := sourceprocess.Identity{ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42}
	tests := []struct {
		name   string
		result sourceprocess.Result
		err    error
		code   string
	}{
		{
			name:   "unknown error",
			result: sourceprocess.Result{Attempts: 1, ExitCode: 9, Stdout: []byte("secret stdout"), Stderr: []byte("secret stderr"), Identity: identity},
			err:    errors.New("secret adapter cause"),
			code:   "unclassified_source_execution_outcome",
		},
		{
			name:   "nil error with nonzero exit",
			result: sourceprocess.Result{Attempts: 1, ExitCode: 9, Stdout: []byte("secret stdout"), Stderr: []byte("secret stderr"), Identity: identity},
			code:   "unclassified_source_execution_outcome",
		},
		{
			name:   "retryable command failure",
			result: sourceprocess.Result{Attempts: 1, ExitCode: 9, Stdout: []byte("secret stdout"), Stderr: []byte("secret stderr"), Identity: identity},
			err:    fault.New(fault.KindRejected, "source_command_failed", "secret adapter cause", true),
			code:   "source_command_failed",
		},
		{
			name:   "wrong-kind command failure",
			result: sourceprocess.Result{Attempts: 1, ExitCode: 9, Stdout: []byte("secret stdout"), Stderr: []byte("secret stderr"), Identity: identity},
			err:    fault.New(fault.KindUnavailable, "source_command_failed", "secret adapter cause", false),
			code:   "source_command_failed",
		},
		{
			name:   "command failure with zero exit",
			result: sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: []byte("secret stdout"), Stderr: []byte("secret stderr"), Identity: identity},
			err:    fault.New(fault.KindRejected, "source_command_failed", "secret adapter cause", false),
			code:   "source_command_failed",
		},
		{
			name:   "unconventional completion",
			result: sourceprocess.Result{Attempts: 1, ExitCode: -1, Stdout: []byte("secret stdout"), Stderr: []byte("secret stderr"), Identity: identity},
			err:    fault.New(fault.KindRejected, "source_command_failed", "secret adapter cause", false),
			code:   "source_command_failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			process := &processStub{result: test.result, err: test.err}
			parser := &parserStub{}
			service, request, _ := sourceStreamService(t, tailoringbundle.WrapperIdentity, []string{}, process, parser)

			result, err := service.Apply(context.Background(), request)
			public, ok := fault.PublicCopy(err)
			if !ok || public.Code != test.code || public.Retryable {
				t.Fatalf("result=%+v error=%v public=%+v", result, err, public)
			}
			if result.SourceProcessAttempts != 0 || result.SourceStream != nil || result.TransformedJSON != nil || strings.Contains(public.Message, "secret") || strings.Contains(err.Error(), "secret") {
				t.Fatalf("source data escaped failure boundary: result=%+v error=%v public=%+v", result, err, public)
			}
			if process.calls != 1 || parser.calls != 0 {
				t.Fatalf("process/parser calls=%d/%d", process.calls, parser.calls)
			}
		})
	}
}

func TestApplySuppressesSourceStreamWhenContextIsCanceledAfterProcess(t *testing.T) {
	identity := sourceprocess.Identity{ResolvedPath: "/opt/bin/fixture", SHA256: strings.Repeat("a", 64), Size: 42}
	ctx, cancel := context.WithCancel(context.Background())
	process := &processStub{
		result: sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: []byte("secret stdout"), Stderr: []byte("secret stderr"), Identity: identity},
		after:  cancel,
	}
	parser := &parserStub{}
	service, request, _ := sourceStreamService(t, tailoringbundle.WrapperIdentity, []string{}, process, parser)

	result, err := service.Apply(ctx, request)
	public, ok := fault.PublicCopy(err)
	if !ok || public.Code != "source_output_processing_canceled" || public.Retryable {
		t.Fatalf("result=%+v error=%v public=%+v", result, err, public)
	}
	if result.SourceProcessAttempts != 0 || result.SourceStream != nil || result.TransformedJSON != nil || strings.Contains(public.Message, "secret") || process.calls != 1 || parser.calls != 0 {
		t.Fatalf("result=%+v public=%+v process/parser calls=%d/%d", result, public, process.calls, parser.calls)
	}
}
