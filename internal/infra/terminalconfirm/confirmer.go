// Package terminalconfirm requires explicit exact-digest confirmation on a
// controlling terminal. Redirected stdin is never consulted.
package terminalconfirm

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/tasuku43/atsura/internal/domain/bundletrust"
)

type readWriteCloser interface {
	io.Reader
	io.Writer
	io.Closer
}

type Confirmer struct {
	open func() (readWriteCloser, error)
}

func New() *Confirmer { return &Confirmer{open: openTerminal} }

func (c *Confirmer) Confirm(ctx context.Context, summary bundletrust.Summary) error {
	if ctx == nil || c == nil || c.open == nil || len(summary.BundleDigest) != 64 {
		return fmt.Errorf("interactive confirmation is not configured")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	terminal, err := c.open()
	if err != nil {
		return fmt.Errorf("controlling terminal is unavailable: %w", err)
	}
	defer terminal.Close()
	prompt := fmt.Sprintf(
		"Adopt this exact Atsura tailoring bundle?\n  bundle: %s\n  catalog: %s\n  specification: %s\n  source: %s\n  source sha256: %s\n  source version: %s\n  surface default: %s\n  commands: included=%d explicitly-excluded=%d\n  wrappers: identity=%d transform=%d\n  transforms: option-overrides=%d argv=%d before=%d after=%d output=%d\nType the full bundle digest to adopt it:\n> ",
		summary.BundleDigest, summary.CatalogDigest, summary.SpecificationDigest, summary.SourcePath,
		summary.SourceSHA256, summary.SourceVersion, summary.SurfaceDefault,
		summary.IncludedCommandCount, summary.ExcludedCommandCount,
		summary.IdentityWrapperCount, summary.TransformWrapperCount,
		summary.OptionOverrideCount, summary.ArgvTransformationCount,
		summary.BeforeActionCount, summary.AfterActionCount, summary.OutputTransformationCount,
	)
	if _, err := io.WriteString(terminal, prompt); err != nil {
		return fmt.Errorf("write confirmation prompt: %w", err)
	}
	reader := bufio.NewReader(io.LimitReader(terminal, 256))
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("read confirmation: %w", err)
	}
	if strings.TrimSpace(line) != summary.BundleDigest {
		return fmt.Errorf("exact bundle digest was not confirmed")
	}
	return nil
}
