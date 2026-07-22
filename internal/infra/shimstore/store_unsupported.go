//go:build !linux && !darwin

package shimstore

import (
	"context"

	"github.com/tasuku43/atsura/internal/domain/wrappershim"
)

const binDirectoryName = "bin"

func (s *Store) Install(context.Context, wrappershim.Manifest, []byte) (wrappershim.Record, bool, error) {
	return wrappershim.Record{}, false, ErrUnsupported
}

func (s *Store) Status(context.Context) (wrappershim.Inventory, error) {
	return wrappershim.Inventory{}, ErrUnsupported
}

func (s *Store) Remove(context.Context, wrappershim.Reference) (wrappershim.Record, error) {
	return wrappershim.Record{}, ErrUnsupported
}
