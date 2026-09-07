package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/user/internal/store"
)

func TestFilterDeletedAccountProfiles_MissingCheckerFailsClosed(t *testing.T) {
	svc := &UserGRPC{}
	rows := []*store.ProfileRow{{AccountID: uuid.New()}}

	visible, err := svc.filterDeletedAccountProfiles(context.Background(), rows)

	require.Error(t, err)
	require.Nil(t, visible)
	require.Contains(t, err.Error(), "checker is not configured")
}
