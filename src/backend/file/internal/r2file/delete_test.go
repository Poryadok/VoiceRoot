package r2file

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordingDeleter struct {
	deleted []string
	errKey  string
}

func (d *recordingDeleter) DeleteObject(_ context.Context, key string) error {
	if key == d.errKey {
		return errors.New("delete failed")
	}
	d.deleted = append(d.deleted, key)
	return nil
}

func TestDeleteKeysDedupesAndSkipsEmpty(t *testing.T) {
	t.Parallel()
	d := &recordingDeleter{}
	err := DeleteKeys(context.Background(), d,
		"attachments/a/original.png",
		"",
		"processed/a/full.webp",
		"attachments/a/original.png",
		"processed/a/thumb.webp",
	)
	require.NoError(t, err)
	require.Equal(t, []string{
		"attachments/a/original.png",
		"processed/a/full.webp",
		"processed/a/thumb.webp",
	}, d.deleted)
}

func TestDeleteKeysReturnsFirstError(t *testing.T) {
	t.Parallel()
	d := &recordingDeleter{errKey: "bad-key"}
	err := DeleteKeys(context.Background(), d, "good-key", "bad-key", "ignored")
	require.Error(t, err)
	require.Equal(t, []string{"good-key"}, d.deleted)
}

func TestDeleteKeysNilDeleterIsNoop(t *testing.T) {
	t.Parallel()
	require.NoError(t, DeleteKeys(context.Background(), nil, "any-key"))
}
