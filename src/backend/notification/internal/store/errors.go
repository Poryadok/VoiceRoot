package store

import "errors"

var (
	// ErrNotImplemented marks Phase-6 stubs until production store logic lands.
	ErrNotImplemented = errors.New("notification store: not implemented")
	// ErrDeviceTokenNotFound is returned when unregister targets a missing row.
	ErrDeviceTokenNotFound = errors.New("device token not found")
)
