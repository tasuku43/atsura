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
	summary := bundletrust.Summary{BundleDigest: digest, CatalogDigest: strings.Repeat("b", 64), PolicyDigest: strings.Repeat("c", 64), SourcePath: "/tool", SourceSHA256: strings.Repeat("d", 64), SourceVersion: "1.0", VisibleCount: 2, ReadCount: 1, WriteCount: 1, AllowCount: 1, ConfirmCount: 1}
	if err := confirmer.Confirm(context.Background(), summary); err != nil {
		t.Fatal(err)
	}
	if output, _ := io.ReadAll(stream); !bytes.Contains(output, []byte("visible commands: 2")) {
		t.Fatalf("prompt = %q", output)
	}

	denied := &terminalStub{bytes.NewBufferString("yes\n")}
	confirmer.open = func() (readWriteCloser, error) { return denied, nil }
	if err := confirmer.Confirm(context.Background(), summary); err == nil {
		t.Fatal("non-digest confirmation succeeded")
	}
}
