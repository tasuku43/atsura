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
	summary := bundletrust.Summary{BundleDigest: digest, CatalogDigest: strings.Repeat("b", 64), SpecificationDigest: strings.Repeat("c", 64), SourcePath: "/tool", SourceSHA256: strings.Repeat("d", 64), SourceVersion: "1.0", SurfaceDefault: "exclude", IncludedCommandCount: 2, ExcludedCommandCount: 1, IdentityWrapperCount: 1, TransformWrapperCount: 1, OptionOverrideCount: 2, ArgvTransformationCount: 1, OutputTransformationCount: 1}
	if err := confirmer.Confirm(context.Background(), summary); err != nil {
		t.Fatal(err)
	}
	if output, _ := io.ReadAll(stream); !bytes.Contains(output, []byte("surface default: exclude")) || !bytes.Contains(output, []byte("identity=1 transform=1")) || !bytes.Contains(output, []byte("option-overrides=2")) {
		t.Fatalf("prompt = %q", output)
	}

	denied := &terminalStub{bytes.NewBufferString("yes\n")}
	confirmer.open = func() (readWriteCloser, error) { return denied, nil }
	if err := confirmer.Confirm(context.Background(), summary); err == nil {
		t.Fatal("non-digest confirmation succeeded")
	}
}
