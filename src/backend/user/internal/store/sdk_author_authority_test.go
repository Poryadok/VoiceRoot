package store

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestJoinSdkAuthorRollbackError(t *testing.T) {
	primaryErr := errors.New("record tombstone failed")
	rollbackErr := errors.New("rollback failed")

	got := joinSdkAuthorRollbackError(primaryErr, rollbackErr)

	if !errors.Is(got, primaryErr) {
		t.Fatalf("joined error %v does not preserve primary error", got)
	}
	if !errors.Is(got, rollbackErr) {
		t.Fatalf("joined error %v does not expose rollback error", got)
	}
}

func TestJoinSdkAuthorRollbackErrorIgnoresClosedTransaction(t *testing.T) {
	primaryErr := errors.New("record tombstone failed")

	got := joinSdkAuthorRollbackError(primaryErr, pgx.ErrTxClosed)

	if got != primaryErr {
		t.Fatalf("got %v, want unchanged primary error %v", got, primaryErr)
	}
}
