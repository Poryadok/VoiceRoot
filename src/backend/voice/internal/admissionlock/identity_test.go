package admissionlock

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

type hashRow int32

func (r hashRow) Scan(dest ...any) error { *dest[0].(*int32) = int32(r); return nil }

type lockRecorder struct {
	hashes  map[string]int32
	keys    []int32
	classes []int32
}

func (r *lockRecorder) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	return hashRow(r.hashes[args[0].(string)])
}
func (r *lockRecorder) Exec(_ context.Context, _ string, args ...any) (pgconn.CommandTag, error) {
	r.classes = append(r.classes, args[0].(int32))
	r.keys = append(r.keys, args[1].(int32))
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func TestIdentityLocksSortSignedHashesAndDeduplicateCollisions(t *testing.T) {
	a, b, p, q := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	r := &lockRecorder{hashes: map[string]int32{
		"account:" + a.String(): 11, "profile:" + p.String(): -5,
		"account:" + b.String(): -5, "profile:" + q.String(): 2,
	}}
	err := Lock(context.Background(), r, []Identity{{b, q}, {a, p}, {b, q}})
	require.NoError(t, err)
	require.Equal(t, []int32{-5, 2, 11}, r.keys)
	require.Equal(t, []int32{Namespace, Namespace, Namespace}, r.classes)
}

func TestInvalidIdentityDoesNotAcquireAnyLocks(t *testing.T) {
	r := &lockRecorder{}
	require.Error(t, Lock(context.Background(), r, []Identity{{uuid.New(), uuid.Nil}}))
	require.Empty(t, r.keys)
}
