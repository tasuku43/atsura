package terminalconfirm

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/domain/bundletrust"
)

type terminalStub struct{ *bytes.Buffer }

func (terminalStub) Close() error { return nil }

func TestConfirmerRequiresFullDigestAndShowsAuthoritySummary(t *testing.T) {
	digest := strings.Repeat("a", 64)
	stream := &terminalStub{bytes.NewBufferString(digest + "\n")}
	confirmer := &Confirmer{open: func() (readWriteCloser, error) { return stream, nil }}
	summary := bundletrust.Summary{BundleDigest: digest, CatalogDigest: strings.Repeat("b", 64), SpecificationDigest: strings.Repeat("c", 64), SourcePath: "/tool", SourceSHA256: strings.Repeat("d", 64), SourceVersion: "1.0", SurfaceDefault: "exclude", IncludedCommandCount: 2, ExcludedCommandCount: 1, IdentityWrapperCount: 1, TransformWrapperCount: 1, OptionOverrideCount: 2, ArgvTransformationCount: 1, OutputTransformationCount: 1, SourceStreamResultCount: 1, OptimizerResultCount: 1, Processors: []bundletrust.ProcessorSummary{{Contract: "atsura.output.rtk_go_test_pass.v1", AdapterKind: "atsura.processor.rtk", Version: "0.43.0", ResolvedPath: "/rtk", SHA256: strings.Repeat("e", 64), Size: 42, InputFormat: "go_test_jsonl", OutputFormat: "go_test_pass_summary"}}}
	if err := confirmer.Confirm(context.Background(), summary); err != nil {
		t.Fatal(err)
	}
	if output, _ := io.ReadAll(stream); !bytes.Contains(output, []byte("surface default: exclude")) || !bytes.Contains(output, []byte("identity=1 transform=1")) || !bytes.Contains(output, []byte("option-overrides=2")) || !bytes.Contains(output, []byte("source-stream-passthrough=1 optimizer=1")) || !bytes.Contains(output, []byte("processor 1: contract=atsura.output.rtk_go_test_pass.v1")) || !bytes.Contains(output, []byte("exact transformed go test -json")) || !bytes.Contains(output, []byte("may contain controls or secrets")) {
		t.Fatalf("prompt = %q", output)
	}

	denied := &terminalStub{bytes.NewBufferString("yes\n")}
	confirmer.open = func() (readWriteCloser, error) { return denied, nil }
	if err := confirmer.Confirm(context.Background(), summary); err == nil {
		t.Fatal("non-digest confirmation succeeded")
	}
}

func TestConfirmerOmitsSourceStreamWarningWhenNoWrapperExposesIt(t *testing.T) {
	digest := strings.Repeat("a", 64)
	stream := &terminalStub{bytes.NewBufferString(digest + "\n")}
	confirmer := &Confirmer{open: func() (readWriteCloser, error) { return stream, nil }}
	summary := bundletrust.Summary{BundleDigest: digest, CatalogDigest: strings.Repeat("b", 64), SpecificationDigest: strings.Repeat("c", 64), SourcePath: "/tool", SourceSHA256: strings.Repeat("d", 64), SourceVersion: "1.0", SurfaceDefault: "exclude"}
	if err := confirmer.Confirm(context.Background(), summary); err != nil {
		t.Fatal(err)
	}
	output, _ := io.ReadAll(stream)
	if !bytes.Contains(output, []byte("source-stream-passthrough=0 optimizer=0")) || bytes.Contains(output, []byte("may contain controls or secrets")) {
		t.Fatalf("prompt = %q", output)
	}
}
