package planapply

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/app/processorcompat"
	"github.com/tasuku43/atsura/internal/domain/bundletrust"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
	"github.com/tasuku43/atsura/internal/domain/sourcecatalog"
	"github.com/tasuku43/atsura/internal/domain/sourceprocess"
	"github.com/tasuku43/atsura/internal/domain/tailoringbundle"
	"github.com/tasuku43/atsura/internal/domain/tailoringplan"
)

type optimizerIdentityStub struct {
	values []processorprocess.Identity
	errs   []error
	paths  []string
}

func (s *optimizerIdentityStub) Identify(_ context.Context, path string) (processorprocess.Identity, error) {
	s.paths = append(s.paths, path)
	index := len(s.paths) - 1
	var value processorprocess.Identity
	if index < len(s.values) {
		value = s.values[index]
	}
	if index < len(s.errs) {
		return value, s.errs[index]
	}
	return value, nil
}

type optimizerProcessStub struct {
	result  processorprocess.Result
	err     error
	calls   int
	request processorprocess.Request
	after   func()
}

func (s *optimizerProcessStub) Run(_ context.Context, request processorprocess.Request) (processorprocess.Result, error) {
	s.calls++
	s.request = request
	if s.after != nil {
		s.after()
	}
	return s.result, s.err
}

type optimizerCompatibilityStub struct {
	err   error
	calls int
}

func (s *optimizerCompatibilityStub) VerifyPlan(tailoringplan.Plan) error {
	s.calls++
	return s.err
}

type optimizerAdmissionStub struct {
	summary  string
	eligible bool
	mutate   bool
	calls    int
	input    []byte
}

func (s *optimizerAdmissionStub) ExpectedSummary(input []byte) (string, bool) {
	s.calls++
	s.input = append([]byte(nil), input...)
	if s.mutate && len(input) != 0 {
		input[0] ^= 0xff
	}
	return s.summary, s.eligible
}

type optimizerFixture struct {
	bundle            tailoringbundle.Bundle
	bundleDigest      string
	sourceIdentity    sourceprocess.Identity
	processorIdentity processorprocess.Identity
}

func newOptimizerFixture(t *testing.T) optimizerFixture {
	t.Helper()
	sourceIdentity := sourceprocess.Identity{
		ResolvedPath: filepath.Join(t.TempDir(), "go"), SHA256: strings.Repeat("b", 64), Size: 1024,
	}
	processorIdentity := processorprocess.Identity{
		ResolvedPath: filepath.Join(t.TempDir(), "rtk"),
		SHA256:       "2dab449f32ea744c30b02a3ef9806e3e7d3b356a145332f3f2aaabb5ea48edee",
		Size:         7763408,
	}
	catalog := sourcecatalog.Catalog{
		SchemaVersion: sourcecatalog.SchemaVersion,
		Adapter:       sourcecatalog.Adapter{Kind: processorcompat.SourceAdapterKind, ContractVersion: processorcompat.SourceContractVersion},
		Source: sourcecatalog.Source{
			RequestedExecutable: "go", ResolvedPath: sourceIdentity.ResolvedPath, SHA256: sourceIdentity.SHA256, Size: sourceIdentity.Size, Version: "go1.26.5",
		},
		Probe: sourcecatalog.Probe{IDs: []string{"help", "test_help", "version"}, Attempts: 3},
		Commands: []sourcecatalog.Command{{
			Path: []string{"test"}, Summary: "test packages", Provenance: sourcecatalog.ProvenanceVerifiedBuiltin,
			Options: []sourcecatalog.Option{},
			StructuredOutput: []sourcecatalog.StructuredOutput{{
				Format: processorcompat.InputFormat, SelectorFlag: "-json",
				Fields: []string{"Action", "Elapsed", "FailedBuild", "Output", "Package", "Test", "Time"},
			}},
		}},
	}
	observation := processorprocess.Observation{
		SchemaVersion: processorprocess.ObservationSchemaVersion,
		Adapter:       processorprocess.Adapter{Kind: processorcompat.ProcessorAdapterKind, ContractVersion: processorcompat.ProcessorContractVersion},
		Platform:      processorprocess.Platform{OS: "darwin", Arch: "arm64"},
		Identity:      processorIdentity,
		Version:       processorcompat.ProcessorVersion,
		Probe: processorprocess.Probe{
			Argv: []string{"--version"}, EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV1, Attempts: 1,
		},
	}
	registry := processorcompat.New()
	entry, err := registry.DefaultEntry(catalog, observation)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := registry.Binding(observation)
	if err != nil {
		t.Fatal(err)
	}
	catalogDigest, err := catalog.Digest()
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := tailoringbundle.Compile(catalog, tailoringbundle.Specification{
		SchemaVersion: tailoringbundle.SpecificationSchemaVersion,
		CatalogDigest: catalogDigest,
		Surface:       tailoringbundle.Surface{Default: tailoringbundle.SurfaceDefaultExclude},
		Commands:      []tailoringbundle.CommandEntry{entry},
	}, binding)
	if err != nil {
		t.Fatal(err)
	}
	bundleDigest, err := bundle.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return optimizerFixture{bundle: bundle, bundleDigest: bundleDigest, sourceIdentity: sourceIdentity, processorIdentity: processorIdentity}
}

func optimizerSourceBytes() []byte {
	return []byte("" +
		`{"Time":"2026-07-22T00:00:00Z","Action":"start","Package":"example.com/project"}` + "\n" +
		`{"Time":"2026-07-22T00:00:01Z","Action":"run","Package":"example.com/project","Test":"TestOne"}` + "\n" +
		`{"Time":"2026-07-22T00:00:02Z","Action":"pass","Package":"example.com/project","Test":"TestOne","Elapsed":0.01}` + "\n" +
		`{"Time":"2026-07-22T00:00:03Z","Action":"pass","Package":"example.com/project","Elapsed":0.02}` + "\n")
}

func optimizerRequest(allow bool) Request {
	return Request{
		BundlePath:                       "purpose.bundle.json",
		AllowOriginalPreservingOptimizer: allow,
		Attempt:                          tailoringplan.Attempt{Executable: "go", Args: []string{"test"}},
		Command:                          testCommandContext(),
	}
}

func optimizerService(fixture optimizerFixture, source *processStub, identity *optimizerIdentityStub, processor *optimizerProcessStub, compatibility *optimizerCompatibilityStub, admission *optimizerAdmissionStub) *Service {
	return New(
		&bundleStub{bundle: fixture.bundle, digest: fixture.bundleDigest},
		&adoptionStub{state: bundletrust.StateAdopted},
		&identityStub{value: fixture.sourceIdentity},
		&compatibilityStub{},
		source,
		&parserStub{},
		ProcessorSupport{Identity: identity, Processes: processor, Compatibility: compatibility, Admission: admission},
	)
}

func TestOptimizerRequiresExplicitFacadeOptInBeforeAnyProcess(t *testing.T) {
	fixture := newOptimizerFixture(t)
	source := &processStub{}
	identity := &optimizerIdentityStub{values: []processorprocess.Identity{fixture.processorIdentity}}
	processor := &optimizerProcessStub{}
	compatibility := &optimizerCompatibilityStub{}
	admission := &optimizerAdmissionStub{}

	result, err := optimizerService(fixture, source, identity, processor, compatibility, admission).Apply(context.Background(), optimizerRequest(false))
	assertOptimizerFault(t, result, err, "wrapper_runtime_not_supported", 0, 0)
	if source.calls != 0 || len(identity.paths) != 0 || processor.calls != 0 || compatibility.calls != 0 || admission.calls != 0 {
		t.Fatalf("calls source/identity/processor/compatibility/admission = %d/%d/%d/%d/%d", source.calls, len(identity.paths), processor.calls, compatibility.calls, admission.calls)
	}
}

func TestOptimizerRequiresCompleteOptionalProcessorSupportBeforeSource(t *testing.T) {
	fixture := newOptimizerFixture(t)
	source := &processStub{}
	service := New(
		&bundleStub{bundle: fixture.bundle, digest: fixture.bundleDigest},
		&adoptionStub{state: bundletrust.StateAdopted},
		&identityStub{value: fixture.sourceIdentity},
		&compatibilityStub{},
		source,
		&parserStub{},
	)
	result, err := service.Apply(context.Background(), optimizerRequest(true))
	assertOptimizerFault(t, result, err, "wrapper_runtime_not_supported", 0, 0)
	if source.calls != 0 {
		t.Fatalf("source calls = %d", source.calls)
	}
}

func TestOptimizerProcessorPreflightAndCompatibilityFailBeforeSource(t *testing.T) {
	fixture := newOptimizerFixture(t)
	t.Run("compatibility", func(t *testing.T) {
		source := &processStub{}
		identity := &optimizerIdentityStub{}
		processor := &optimizerProcessStub{}
		compatibility := &optimizerCompatibilityStub{err: errors.New("not admitted")}
		result, err := optimizerService(fixture, source, identity, processor, compatibility, &optimizerAdmissionStub{}).Apply(context.Background(), optimizerRequest(true))
		assertOptimizerFault(t, result, err, "wrapper_runtime_not_supported", 0, 0)
		if source.calls != 0 || len(identity.paths) != 0 || processor.calls != 0 || compatibility.calls != 1 {
			t.Fatalf("calls source/identity/processor/compatibility = %d/%d/%d/%d", source.calls, len(identity.paths), processor.calls, compatibility.calls)
		}
	})

	t.Run("identity drift", func(t *testing.T) {
		source := &processStub{}
		identity := &optimizerIdentityStub{values: []processorprocess.Identity{{
			ResolvedPath: fixture.processorIdentity.ResolvedPath, SHA256: strings.Repeat("c", 64), Size: fixture.processorIdentity.Size,
		}}}
		processor := &optimizerProcessStub{}
		compatibility := &optimizerCompatibilityStub{}
		result, err := optimizerService(fixture, source, identity, processor, compatibility, &optimizerAdmissionStub{}).Apply(context.Background(), optimizerRequest(true))
		assertOptimizerFault(t, result, err, "processor_identity_changed", 0, 0)
		if source.calls != 0 || len(identity.paths) != 1 || processor.calls != 0 || compatibility.calls != 1 {
			t.Fatalf("calls source/identity/processor/compatibility = %d/%d/%d/%d", source.calls, len(identity.paths), processor.calls, compatibility.calls)
		}
	})

	t.Run("identity unavailable", func(t *testing.T) {
		source := &processStub{}
		identity := &optimizerIdentityStub{errs: []error{errors.New("private path detail")}}
		processor := &optimizerProcessStub{}
		compatibility := &optimizerCompatibilityStub{}
		result, err := optimizerService(fixture, source, identity, processor, compatibility, &optimizerAdmissionStub{}).Apply(context.Background(), optimizerRequest(true))
		public, ok := fault.PublicCopy(err)
		if !ok || public.Code != "processor_identity_unavailable" || !public.Retryable || result.SourceProcessAttempts != 0 || result.ProcessorProcessAttempts != 0 {
			t.Fatalf("result/error/public = %+v/%v/%+v", result, err, public)
		}
		if source.calls != 0 || len(identity.paths) != 1 || processor.calls != 0 || compatibility.calls != 1 || strings.Contains(err.Error(), "private path") {
			t.Fatalf("result/error = %+v/%v, calls source/identity/processor/compatibility = %d/%d/%d/%d", result, err, source.calls, len(identity.paths), processor.calls, compatibility.calls)
		}
	})

	t.Run("pre-source identity cancellation", func(t *testing.T) {
		source := &processStub{}
		identity := &optimizerIdentityStub{errs: []error{context.Canceled}}
		processor := &optimizerProcessStub{}
		compatibility := &optimizerCompatibilityStub{}
		result, err := optimizerService(fixture, source, identity, processor, compatibility, &optimizerAdmissionStub{}).Apply(context.Background(), optimizerRequest(true))
		if !errors.Is(err, context.Canceled) || result.SourceProcessAttempts != 0 || result.ProcessorProcessAttempts != 0 || result.Optimizer != nil {
			t.Fatalf("result/error = %+v/%v", result, err)
		}
		if source.calls != 0 || len(identity.paths) != 1 || processor.calls != 0 || compatibility.calls != 1 {
			t.Fatalf("calls source/identity/processor/compatibility = %d/%d/%d/%d", source.calls, len(identity.paths), processor.calls, compatibility.calls)
		}
	})
}

func TestOptimizerPreservesEveryConventionalIneligibleSourceResultBeforeProcessor(t *testing.T) {
	fixture := newOptimizerFixture(t)
	valid := optimizerSourceBytes()
	tests := []struct {
		name              string
		stdout            []byte
		stderr            []byte
		exitCode          int
		sourceErr         error
		summary           string
		eligible          bool
		mutateAdmission   bool
		wantAdmissionCall int
	}{
		{name: "nonzero", stdout: []byte("failed source bytes\x00"), stderr: []byte("source stderr\n"), exitCode: 7, sourceErr: sourceCommandFailed(), wantAdmissionCall: 0},
		{name: "successful stderr", stdout: valid, stderr: []byte("warning\n"), exitCode: 0, wantAdmissionCall: 0},
		{name: "ineligible JSON", stdout: []byte("malformed\n"), exitCode: 0, eligible: false, mutateAdmission: true, wantAdmissionCall: 1},
		{name: "summary not smaller", stdout: valid, exitCode: 0, summary: strings.Repeat("x", len(valid)), eligible: true, wantAdmissionCall: 1},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			source := &processStub{result: sourceprocess.Result{
				Attempts: 1, ExitCode: test.exitCode, Stdout: append([]byte(nil), test.stdout...), Stderr: append([]byte(nil), test.stderr...), Identity: fixture.sourceIdentity,
			}, err: test.sourceErr}
			identity := &optimizerIdentityStub{values: []processorprocess.Identity{fixture.processorIdentity}}
			processor := &optimizerProcessStub{}
			compatibility := &optimizerCompatibilityStub{}
			admission := &optimizerAdmissionStub{summary: test.summary, eligible: test.eligible, mutate: test.mutateAdmission}

			result, err := optimizerService(fixture, source, identity, processor, compatibility, admission).Apply(context.Background(), optimizerRequest(true))
			if err != nil {
				t.Fatal(err)
			}
			if result.Optimizer == nil || result.Optimizer.Disposition != OptimizerPreservedBeforeProcessor || result.SourceProcessAttempts != 1 || result.ProcessorProcessAttempts != 0 ||
				result.Optimizer.ExitCode != test.exitCode || !bytes.Equal(result.Optimizer.Stdout, test.stdout) || !bytes.Equal(result.Optimizer.Stderr, test.stderr) {
				t.Fatalf("result = %+v", result)
			}
			if err := result.Validate(); err != nil {
				t.Fatal(err)
			}
			if source.calls != 1 || len(identity.paths) != 1 || processor.calls != 0 || compatibility.calls != 1 || admission.calls != test.wantAdmissionCall {
				t.Fatalf("calls source/identity/processor/compatibility/admission = %d/%d/%d/%d/%d", source.calls, len(identity.paths), processor.calls, compatibility.calls, admission.calls)
			}
			if len(test.stdout) != 0 {
				result.Optimizer.Stdout[0] ^= 0xff
				if bytes.Equal(result.Optimizer.Stdout, source.result.Stdout) {
					t.Fatal("optimizer result aliases source stdout")
				}
			}
		})
	}
}

func TestOptimizerAcceptsOnlyExactProcessorPostconditions(t *testing.T) {
	fixture := newOptimizerFixture(t)
	input := optimizerSourceBytes()
	summary := "Go test: 1 passed in 1 packages"
	tests := []struct {
		name        string
		stdout      []byte
		disposition OptimizerDisposition
	}{
		{name: "byte-identical input", stdout: input, disposition: OptimizerPreservedAfterProcessor},
		{name: "independent summary", stdout: []byte(summary), disposition: OptimizerOptimized},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			source := &processStub{result: sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: append([]byte(nil), input...), Identity: fixture.sourceIdentity}}
			identity := &optimizerIdentityStub{values: []processorprocess.Identity{fixture.processorIdentity, fixture.processorIdentity}}
			processor := &optimizerProcessStub{result: processorprocess.Result{
				Attempts: 1, ExitCode: 0, Stdout: append([]byte(nil), test.stdout...), Identity: fixture.processorIdentity,
			}}
			compatibility := &optimizerCompatibilityStub{}
			admission := &optimizerAdmissionStub{summary: summary, eligible: true}

			result, err := optimizerService(fixture, source, identity, processor, compatibility, admission).Apply(context.Background(), optimizerRequest(true))
			if err != nil {
				t.Fatal(err)
			}
			if result.Optimizer == nil || result.Optimizer.Disposition != test.disposition || result.SourceProcessAttempts != 1 || result.ProcessorProcessAttempts != 1 ||
				!bytes.Equal(result.Optimizer.Stdout, test.stdout) || len(result.Optimizer.Stderr) != 0 || result.Optimizer.ExitCode != 0 {
				t.Fatalf("result = %+v", result)
			}
			if err := result.Validate(); err != nil {
				t.Fatal(err)
			}
			if source.calls != 1 || len(identity.paths) != 2 || processor.calls != 1 || compatibility.calls != 1 || admission.calls != 1 {
				t.Fatalf("calls source/identity/processor/compatibility/admission = %d/%d/%d/%d/%d", source.calls, len(identity.paths), processor.calls, compatibility.calls, admission.calls)
			}
			if !bytes.Equal(processor.request.Input, input) || !bytes.Equal(admission.input, input) || strings.Join(processor.request.Args, " ") != "pipe --filter=go-test" {
				t.Fatalf("processor request = %+v, admission input = %q", processor.request, admission.input)
			}
			result.Optimizer.Stdout[0] ^= 0xff
			if bytes.Equal(result.Optimizer.Stdout, processor.result.Stdout) {
				t.Fatal("optimizer result aliases processor stdout")
			}
		})
	}
}

func TestOptimizerRechecksIdentityAfterEligibleSourceBeforeProcessor(t *testing.T) {
	fixture := newOptimizerFixture(t)
	input := optimizerSourceBytes()
	source := &processStub{result: sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: input, Identity: fixture.sourceIdentity}}
	drifted := fixture.processorIdentity
	drifted.SHA256 = strings.Repeat("c", 64)
	identity := &optimizerIdentityStub{values: []processorprocess.Identity{fixture.processorIdentity, drifted}}
	processor := &optimizerProcessStub{}
	admission := &optimizerAdmissionStub{summary: "Go test: 1 passed in 1 packages", eligible: true}

	result, err := optimizerService(fixture, source, identity, processor, &optimizerCompatibilityStub{}, admission).Apply(context.Background(), optimizerRequest(true))
	assertOptimizerFault(t, result, err, "processor_identity_changed", 1, 0)
	if source.calls != 1 || len(identity.paths) != 2 || processor.calls != 0 || admission.calls != 1 || result.Optimizer != nil || result.SourceStream != nil || result.TransformedJSON != nil {
		t.Fatalf("result = %+v, calls source/identity/processor/admission = %d/%d/%d/%d", result, source.calls, len(identity.paths), processor.calls, admission.calls)
	}
	if strings.Contains(err.Error(), string(input)) {
		t.Fatal("fault exposes source input")
	}
}

func TestOptimizerPostSourceIdentityUnavailableUsesDistinctNonRetryableCode(t *testing.T) {
	fixture := newOptimizerFixture(t)
	input := optimizerSourceBytes()
	source := &processStub{result: sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: input, Identity: fixture.sourceIdentity}}
	identity := &optimizerIdentityStub{
		values: []processorprocess.Identity{fixture.processorIdentity, fixture.processorIdentity},
		errs: []error{nil, fault.New(
			fault.KindUnavailable, "processor_identity_unavailable", "private identity detail", true,
			testCommandContext().RuntimeHelpAction,
		)},
	}
	processor := &optimizerProcessStub{}
	admission := &optimizerAdmissionStub{summary: "Go test: 1 passed in 1 packages", eligible: true}

	result, err := optimizerService(fixture, source, identity, processor, &optimizerCompatibilityStub{}, admission).Apply(context.Background(), optimizerRequest(true))
	assertOptimizerFault(t, result, err, "processor_identity_unavailable_after_source", 1, 0)
	assertNoOptimizerBytes(t, result, err)
	if source.calls != 1 || len(identity.paths) != 2 || processor.calls != 0 || admission.calls != 1 || strings.Contains(err.Error(), "private identity detail") {
		t.Fatalf("result/error = %+v/%v, calls source/identity/processor/admission = %d/%d/%d/%d", result, err, source.calls, len(identity.paths), processor.calls, admission.calls)
	}
}

func TestOptimizerPostSourceIdentityCancellationIsNonRetryableAndSuppressesBytes(t *testing.T) {
	fixture := newOptimizerFixture(t)
	input := optimizerSourceBytes()
	source := &processStub{result: sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: input, Identity: fixture.sourceIdentity}}
	identity := &optimizerIdentityStub{
		values: []processorprocess.Identity{fixture.processorIdentity, fixture.processorIdentity},
		errs:   []error{nil, context.DeadlineExceeded},
	}
	processor := &optimizerProcessStub{}
	admission := &optimizerAdmissionStub{summary: "Go test: 1 passed in 1 packages", eligible: true}

	result, err := optimizerService(fixture, source, identity, processor, &optimizerCompatibilityStub{}, admission).Apply(context.Background(), optimizerRequest(true))
	assertOptimizerFault(t, result, err, "processor_execution_canceled", 1, 0)
	public, _ := fault.PublicCopy(err)
	if public.Kind != fault.KindCanceled || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error/public = %v/%+v", err, public)
	}
	assertNoOptimizerBytes(t, result, err)
	if source.calls != 1 || len(identity.paths) != 2 || processor.calls != 0 || admission.calls != 1 {
		t.Fatalf("calls source/identity/processor/admission = %d/%d/%d/%d", source.calls, len(identity.paths), processor.calls, admission.calls)
	}
}

func TestOptimizerSuppressesBytesForUncertainSourceAndProcessorFailures(t *testing.T) {
	fixture := newOptimizerFixture(t)
	input := optimizerSourceBytes()
	summary := "Go test: 1 passed in 1 packages"

	t.Run("uncertain source", func(t *testing.T) {
		source := &processStub{result: sourceprocess.Result{
			Attempts: 1, ExitCode: -1, Stdout: []byte("secret source stdout"), Stderr: []byte("secret source stderr"), Identity: fixture.sourceIdentity,
		}, err: fault.New(fault.KindUnavailable, "source_command_timeout", "The source timed out.", false, testCommandContext().RuntimeHelpAction)}
		identity := &optimizerIdentityStub{values: []processorprocess.Identity{fixture.processorIdentity}}
		processor := &optimizerProcessStub{}
		result, err := optimizerService(fixture, source, identity, processor, &optimizerCompatibilityStub{}, &optimizerAdmissionStub{}).Apply(context.Background(), optimizerRequest(true))
		assertOptimizerFault(t, result, err, "source_command_timeout", 1, 0)
		assertNoOptimizerBytes(t, result, err)
		if processor.calls != 0 || len(identity.paths) != 1 {
			t.Fatalf("processor/identity calls = %d/%d", processor.calls, len(identity.paths))
		}
	})

	tests := []struct {
		name         string
		result       processorprocess.Result
		err          error
		code         string
		wantAttempts int
	}{
		{
			name: "processor nonzero", wantAttempts: 1, code: "processor_command_failed",
			result: processorprocess.Result{Attempts: 1, ExitCode: 9, Stdout: []byte("secret processor stdout"), Stderr: []byte("secret processor stderr"), Identity: fixture.processorIdentity},
			err:    fault.New(fault.KindRejected, "processor_command_failed", "private processor detail", false, testCommandContext().RuntimeHelpAction),
		},
		{
			name: "processor start failure", wantAttempts: 0, code: "processor_process_start_failed_after_source",
			result: processorprocess.Result{ExitCode: -1},
			err:    fault.New(fault.KindUnavailable, "processor_process_start_failed", "private processor detail", true, testCommandContext().RuntimeHelpAction),
		},
		{
			name: "processor environment setup failure", wantAttempts: 0, code: "processor_environment_setup_failed_after_source",
			result: processorprocess.Result{ExitCode: -1},
			err:    fault.New(fault.KindUnavailable, "processor_environment_setup_failed", "private processor detail", true, testCommandContext().RuntimeHelpAction),
		},
		{
			name: "processor identity unavailable during start", wantAttempts: 0, code: "processor_identity_unavailable_after_source",
			result: processorprocess.Result{ExitCode: -1},
			err:    fault.New(fault.KindUnavailable, "processor_identity_unavailable", "private processor detail", true, testCommandContext().RuntimeHelpAction),
		},
		{
			name: "cleanup failure after processor completion", wantAttempts: 1, code: "processor_cleanup_failed",
			result: processorprocess.Result{Attempts: 1, ExitCode: 0, Stdout: []byte("secret processor stdout"), Identity: fixture.processorIdentity},
			err:    fault.New(fault.KindUnavailable, "processor_cleanup_failed", "private processor detail", false, testCommandContext().RuntimeHelpAction),
		},
		{
			name: "successful stderr", wantAttempts: 1, code: "processor_output_not_admitted",
			result: processorprocess.Result{Attempts: 1, ExitCode: 0, Stdout: []byte(summary), Stderr: []byte("secret processor stderr"), Identity: fixture.processorIdentity},
		},
		{
			name: "unexpected stdout", wantAttempts: 1, code: "processor_output_not_admitted",
			result: processorprocess.Result{Attempts: 1, ExitCode: 0, Stdout: []byte("misleading but smaller"), Identity: fixture.processorIdentity},
		},
		{
			name: "inconsistent success", wantAttempts: 0, code: "unclassified_processor_execution_outcome",
			result: processorprocess.Result{ExitCode: -1},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			source := &processStub{result: sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: append([]byte(nil), input...), Identity: fixture.sourceIdentity}}
			identity := &optimizerIdentityStub{values: []processorprocess.Identity{fixture.processorIdentity, fixture.processorIdentity}}
			processor := &optimizerProcessStub{result: test.result, err: test.err}
			result, err := optimizerService(fixture, source, identity, processor, &optimizerCompatibilityStub{}, &optimizerAdmissionStub{summary: summary, eligible: true}).Apply(context.Background(), optimizerRequest(true))
			assertOptimizerFault(t, result, err, test.code, 1, test.wantAttempts)
			assertNoOptimizerBytes(t, result, err)
			if source.calls != 1 || processor.calls != 1 || len(identity.paths) != 2 {
				t.Fatalf("calls source/processor/identity = %d/%d/%d", source.calls, processor.calls, len(identity.paths))
			}
		})
	}
}

func TestOptimizerCancellationAfterSourceReportsOneZeroAndNoBytes(t *testing.T) {
	fixture := newOptimizerFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	source := &processStub{
		result: sourceprocess.Result{Attempts: 1, ExitCode: 0, Stdout: []byte("secret source bytes"), Identity: fixture.sourceIdentity},
		after:  cancel,
	}
	identity := &optimizerIdentityStub{values: []processorprocess.Identity{fixture.processorIdentity}}
	processor := &optimizerProcessStub{}
	result, err := optimizerService(fixture, source, identity, processor, &optimizerCompatibilityStub{}, &optimizerAdmissionStub{}).Apply(ctx, optimizerRequest(true))
	assertOptimizerFault(t, result, err, "source_output_processing_canceled", 1, 0)
	assertNoOptimizerBytes(t, result, err)
	if processor.calls != 0 || len(identity.paths) != 1 {
		t.Fatalf("processor/identity calls = %d/%d", processor.calls, len(identity.paths))
	}
}

func TestOptimizerResultValidationRejectsContradictoryDispositions(t *testing.T) {
	validBefore := Result{
		WrapperKind: tailoringbundle.WrapperTransform, ResultMode: tailoringplan.ResultModeOriginalPreservingOptimizer,
		Optimizer:             &OptimizerResult{Stdout: []byte("source"), Stderr: []byte("stderr"), ExitCode: 9, Disposition: OptimizerPreservedBeforeProcessor},
		SourceProcessAttempts: 1,
	}
	if err := validBefore.Validate(); err != nil {
		t.Fatal(err)
	}
	validOptimized := Result{
		WrapperKind: tailoringbundle.WrapperTransform, ResultMode: tailoringplan.ResultModeOriginalPreservingOptimizer,
		Optimizer:             &OptimizerResult{Stdout: []byte("summary"), ExitCode: 0, Disposition: OptimizerOptimized},
		SourceProcessAttempts: 1, ProcessorProcessAttempts: 1,
	}
	if err := validOptimized.Validate(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*Result)
	}{
		{name: "before has processor attempt", mutate: func(value *Result) { value.ProcessorProcessAttempts = 1 }},
		{name: "before has source stream payload", mutate: func(value *Result) { value.SourceStream = &SourceStreamResult{} }},
		{name: "before uses identity wrapper", mutate: func(value *Result) { value.WrapperKind = tailoringbundle.WrapperIdentity }},
		{name: "missing disposition", mutate: func(value *Result) { value.Optimizer.Disposition = "" }},
		{name: "missing source attempt", mutate: func(value *Result) { value.SourceProcessAttempts = 0 }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			value := validBefore
			optimizer := *validBefore.Optimizer
			value.Optimizer = &optimizer
			test.mutate(&value)
			if err := value.Validate(); err == nil {
				t.Fatalf("Validate() accepted %+v", value)
			}
		})
	}
	for _, test := range []struct {
		name   string
		mutate func(*Result)
	}{
		{name: "optimized has no processor attempt", mutate: func(value *Result) { value.ProcessorProcessAttempts = 0 }},
		{name: "optimized has stderr", mutate: func(value *Result) { value.Optimizer.Stderr = []byte("stderr") }},
		{name: "optimized has newline", mutate: func(value *Result) { value.Optimizer.Stdout = []byte("summary\n") }},
		{name: "optimized has nonzero status", mutate: func(value *Result) { value.Optimizer.ExitCode = 1 }},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			value := validOptimized
			optimizer := *validOptimized.Optimizer
			value.Optimizer = &optimizer
			test.mutate(&value)
			if err := value.Validate(); err == nil {
				t.Fatalf("Validate() accepted %+v", value)
			}
		})
	}
}

func sourceCommandFailed() error {
	return fault.New(fault.KindRejected, "source_command_failed", "The source command failed.", false, testCommandContext().RuntimeHelpAction)
}

func assertOptimizerFault(t *testing.T, result Result, err error, code string, sourceAttempts, processorAttempts int) {
	t.Helper()
	public, ok := fault.PublicCopy(err)
	if !ok || public.Code != code || public.Retryable {
		t.Fatalf("result = %+v, error = %v, public = %+v, want code %q", result, err, public, code)
	}
	if result.SourceProcessAttempts != sourceAttempts || result.ProcessorProcessAttempts != processorAttempts {
		t.Fatalf("attempts = %d/%d, want %d/%d", result.SourceProcessAttempts, result.ProcessorProcessAttempts, sourceAttempts, processorAttempts)
	}
}

func assertNoOptimizerBytes(t *testing.T, result Result, err error) {
	t.Helper()
	if result.Optimizer != nil || result.SourceStream != nil || result.TransformedJSON != nil {
		t.Fatalf("fault result contains a payload: %+v", result)
	}
	public, _ := fault.PublicCopy(err)
	for _, secret := range []string{"secret source stdout", "secret source stderr", "secret processor stdout", "secret processor stderr"} {
		if strings.Contains(err.Error(), secret) || (public != nil && strings.Contains(public.Message, secret)) {
			t.Fatalf("fault exposes %q: error=%v public=%+v", secret, err, public)
		}
	}
}
