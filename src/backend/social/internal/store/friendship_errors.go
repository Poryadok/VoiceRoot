package store

import "errors"

var (
	// ErrAlreadyFriends is returned when an accepted friendship exists between two profiles (either direction).
	ErrAlreadyFriends = errors.New("already friends")
	// ErrIncomingPendingExists means the target already sent a pending invitation to the requester (accept instead).
	ErrIncomingPendingExists = errors.New("incoming friend request exists")
	// ErrSelfInvitation is returned when target equals requester.
	ErrSelfInvitation = errors.New("cannot invite self")
	// ErrFriendshipNotFound is returned when no matching row exists for an operation.
	ErrFriendshipNotFound = errors.New("friendship not found")
)
