package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/app/processorinspect"
	"github.com/tasuku43/atsura/internal/domain/fault"
	"github.com/tasuku43/atsura/internal/domain/operation"
	"github.com/tasuku43/atsura/internal/domain/processorprocess"
)

type cliProcessorInspectionService struct {
	result     processorinspect.Result
	err        error
	calls      int
	intent     operation.Intent
	selector   string
	executable string
}

func (s *cliProcessorInspectionService) Inspect(_ context.Context, intent operation.Intent, selector, executable string) (processorinspect.Result, error) {
	s.calls++
	s.intent = intent
	s.selector = selector
	s.executable = executable
	return s.result, s.err
}

func testProcessorInspectionResult(t *testing.T) processorinspect.Result {
	t.Helper()
	observation := processorprocess.Observation{
		SchemaVersion: processorprocess.ObservationSchemaVersion,
		Adapter:       processorprocess.Adapter{Kind: "atsura.processor.rtk", ContractVersion: 1},
		Platform:      processorprocess.Platform{OS: "darwin", Arch: "arm64"},
		Identity: processorprocess.Identity{
			ResolvedPath: filepath.Join(t.TempDir(), "rtk"),
			SHA256:       strings.Repeat("a", 64),
			Size:         7763408,
		},
		Version: "0.43.0",
		Probe: processorprocess.Probe{
			Argv:                []string{"--version"},
			EnvironmentContract: processorprocess.EnvironmentRTKIsolatedV1,
			Attempts:            1,
		},
	}
	digest, err := observation.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return processorinspect.Result{Observation: observation, Digest: digest}
}

func TestProcessorInspectEmitsCanonicalObservationEnvelope(t *testing.T) {
	result := testProcessorInspectionResult(t)
	service := &cliProcessorInspectionService{result: result}
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	command.processors = service

	code := runCLI(command, []string{
		"processor", "inspect", "--adapter=rtk", "--executable", result.Observation.Identity.ResolvedPath,
	})
	if code != ExitOK || stderr.Len() != 0 {
		t.Fatalf("Run() code=%d stderr=%q", code, stderr.String())
	}
	if service.calls != 1 || service.intent != (operation.Intent{Command: "processor inspect", Effect: operation.EffectExecute}) ||
		service.selector != "rtk" || service.executable != result.Observation.Identity.ResolvedPath {
		t.Fatalf("service calls=%d intent=%+v selector=%q executable=%q", service.calls, service.intent, service.selector, service.executable)
	}

	var document processorInspectionDocument
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if document.SchemaVersion != 1 || document.Inspection.ObservationDigest != result.Digest ||
		document.Inspection.ProcessorProcessAttempts != 1 || !reflect.DeepEqual(document.Inspection.Observation, result.Observation) {
		t.Fatalf("document=%+v", document)
	}
	want, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Fatalf("output is not canonical:\n got %q\nwant %q", stdout.Bytes(), want)
	}
}

func TestProcessorInspectInvalidArgumentsDoNotReachService(t *testing.T) {
	service := &cliProcessorInspectionService{result: testProcessorInspectionResult(t)}
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	command.processors = service

	if code := runCLI(command, []string{"processor", "inspect", "--adapter=rtk"}); code != ExitUsage {
		t.Fatalf("Run() code=%d stderr=%q", code, stderr.String())
	}
	if service.calls != 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "invalid_arguments") {
		t.Fatalf("calls=%d stdout=%q stderr=%q", service.calls, stdout.String(), stderr.String())
	}
}

func TestProcessorInspectHumanHelpIsCatalogDerived(t *testing.T) {
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	if code := runCLI(command, []string{"processor", "inspect", "--help"}); code != ExitOK {
		t.Fatalf("Run() code=%d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		"Usage:\n  atr processor inspect --adapter=rtk --executable <absolute-path>",
		"Effect: execute",
		"Role: utility",
		"--adapter\n    source: flag; required: true; value: text; cardinality: single",
		"allowed: rtk",
		"--executable\n    source: flag; required: true; value: text; cardinality: single",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("help lacks %q:\n%s", want, stdout.String())
		}
	}
}

func TestProcessorInspectPreservesStructuredServiceFault(t *testing.T) {
	service := &cliProcessorInspectionService{err: fault.Wrap(
		fault.KindRejected,
		"unsupported_processor_artifact",
		"The processor does not match the maintained artifact.",
		false,
		errors.New("private path detail"),
		fault.NextAction{Command: "help processor inspect", Reason: "Select the official artifact."},
	)}
	var stdout, stderr bytes.Buffer
	command := New(strings.NewReader(""), &stdout, &stderr)
	command.processors = service
	path := filepath.Join(t.TempDir(), "rtk")

	if code := runCLI(command, []string{"processor", "inspect", "--adapter=rtk", "--executable", path}); code != ExitRejected {
		t.Fatalf("Run() code=%d stderr=%q", code, stderr.String())
	}
	if service.calls != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unsupported_processor_artifact") || strings.Contains(stderr.String(), "private path detail") {
		t.Fatalf("calls=%d stdout=%q stderr=%q", service.calls, stdout.String(), stderr.String())
	}
}

func TestRenderProcessorInspectionRejectsDigestOrAttemptDrift(t *testing.T) {
	result := testProcessorInspectionResult(t)
	result.Digest = strings.Repeat("b", 64)
	if _, err := renderProcessorInspection(result); publicFaultCode(err) != "invalid_processor_observation" {
		t.Fatalf("digest error=%v", err)
	}

	result = testProcessorInspectionResult(t)
	result.Observation.Probe.Attempts = 0
	if _, err := renderProcessorInspection(result); publicFaultCode(err) != "invalid_processor_observation" {
		t.Fatalf("attempt error=%v", err)
	}
}

func TestProcessorInspectOutputFailureIsNonRetryableAfterProbe(t *testing.T) {
	result := testProcessorInspectionResult(t)
	service := &cliProcessorInspectionService{result: result}
	var stderr bytes.Buffer
	command := New(strings.NewReader(""), shortWriter{}, &stderr)
	command.processors = service

	if code := runCLI(command, []string{"--error-format=json", "processor", "inspect", "--adapter=rtk", "--executable", result.Observation.Identity.ResolvedPath}); code != ExitInternal {
		t.Fatalf("Run() code=%d stderr=%q", code, stderr.String())
	}
	if service.calls != 1 || !strings.Contains(stderr.String(), `"code":"execute_output_write_failed"`) ||
		!strings.Contains(stderr.String(), `"retryable":false`) {
		t.Fatalf("calls=%d stderr=%q", service.calls, stderr.String())
	}
}

func publicFaultCode(err error) string {
	public, ok := fault.PublicCopy(err)
	if !ok {
		return ""
	}
	return public.Code
}
