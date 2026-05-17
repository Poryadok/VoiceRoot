package store

import "errors"

var (
	// ErrSelfBlock is returned when blocker and blocked account IDs are equal.
	ErrSelfBlock = errors.New("cannot block self")
	// ErrBlockNotFound is returned when no block row exists for unblock.
	ErrBlockNotFound = errors.New("block not found")
)
