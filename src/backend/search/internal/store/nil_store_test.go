package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMessageSearchStore_nilPoolErrors(t *testing.T) {
	t.Parallel()
	st := &MessageSearchStore{}
	ctx := context.Background()
	require.Error(t, st.Upsert(ctx, MessageDocument{}))
	require.Error(t, st.Delete(ctx, uuid.New()))
	_, _, err := st.SearchInChat(ctx, uuid.New(), "q", nil, 20)
	require.Error(t, err)
}

func TestProfileSpaceSearchStore_nilPoolErrors(t *testing.T) {
	t.Parallel()
	st := &ProfileSpaceSearchStore{}
	ctx := context.Background()
	require.Error(t, st.UpsertProfile(ctx, ProfileDocument{}))
	_, err := st.SearchProfiles(ctx, uuid.Nil, "q", nil, 20)
	require.Error(t, err)
	_, _, err = st.SearchSpaces(ctx, "q", nil, 20)
	require.Error(t, err)
}
