package wrappershim

import "errors"

// Store and lifecycle errors are domain-owned categories so application code
// can classify an implementation without importing a filesystem adapter.
var (
	ErrInvalidInput = errors.New("invalid wrapper shim input")
	ErrUnsupported  = errors.New("wrapper shim operation is unsupported")
	ErrUnsafeStore  = errors.New("wrapper shim store is unsafe")
	ErrCapacity     = errors.New("wrapper shim store capacity reached")
	ErrNotFound     = errors.New("wrapper shim artifact not found")
	ErrConflict     = errors.New("wrapper shim conflicts with existing state")
	ErrTampered     = errors.New("wrapper shim artifact is tampered")
	ErrUncertain    = errors.New("wrapper shim mutation outcome is uncertain")
)
