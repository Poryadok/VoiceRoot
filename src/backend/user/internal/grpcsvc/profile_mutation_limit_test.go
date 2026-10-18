package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"
)

func TestProfileMutationsRequireLimiterBeforeDatabase(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://u:p@127.0.0.1:1/unavailable?connect_timeout=1")
	require.NoError(t, err)
	pool.Close() // A denied mutation must not need any database connection.
	svc := &UserGRPC{Profiles: store.NewProfileStore(pool)}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderUserID, uuid.NewString()))
	t.Run("create", func(t *testing.T) {
		response, err := svc.CreateProfile(ctx, &userv1.CreateProfileRequest{DisplayName: "Alt"})
		require.Nil(t, response)
		require.Equal(t, codes.Unavailable, status.Code(err))
	})
	t.Run("delete", func(t *testing.T) {
		response, err := svc.DeleteProfile(ctx, &userv1.DeleteProfileRequest{ProfileId: uuid.NewString()})
		require.Nil(t, response)
		require.Equal(t, codes.Unavailable, status.Code(err))
	})
}
