package nativeinstaller

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type legacy24RollbackFixture struct {
	pgx.Tx
	failure  error
	deadline bool
}

func (f *legacy24RollbackFixture) Rollback(ctx context.Context) error {
	_, f.deadline = ctx.Deadline()
	return f.failure
}

func TestFinishLegacy24ReadPropagatesRollbackFailure(t *testing.T) {
	prior := errors.New("earlier failure")
	f := &legacy24RollbackFixture{failure: errors.New("driver details stay private")}
	err := prior
	finishLegacy24Read(f, &err)
	require.ErrorIs(t, err, prior)
	require.ErrorContains(t, err, "read snapshot cleanup failed")
	require.NotContains(t, err.Error(), "driver details")
	require.True(t, f.deadline)
	var success error
	f.failure = pgx.ErrTxClosed
	finishLegacy24Read(f, &success)
	require.NoError(t, success, "successful Commit already closed the transaction")
}
