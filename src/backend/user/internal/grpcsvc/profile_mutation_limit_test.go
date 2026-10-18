package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/profilemutation"
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

type profileMutationLimiterFunc func(context.Context, uuid.UUID, string) (bool, error)

func (f profileMutationLimiterFunc) Allow(ctx context.Context, accountID uuid.UUID, operation string) (bool, error) {
	return f(ctx, accountID, operation)
}

func TestProfileMutationsDenyBeforeDatabaseAndPreserveCallerScope(t *testing.T) {
	account := uuid.New()
	for _, operation := range []string{profilemutation.Create, profilemutation.Delete} {
		for _, scenario := range []struct {
			name    string
			err     error
			allowed bool
			cancel  bool
			want    codes.Code
		}{
			{"exhausted", nil, false, false, codes.ResourceExhausted},
			{"redis error", errors.New("private redis configuration"), false, false, codes.Unavailable},
			{"owner canceled with live parent", context.Canceled, false, false, codes.Unavailable},
			{"parent canceled during admission", nil, true, true, codes.Canceled},
		} {
			t.Run(operation+"/"+scenario.name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(metadata.NewIncomingContext(context.Background(), metadata.Pairs(
					authctx.HeaderUserID, account.String(), authctx.HeaderProfileID, uuid.NewString())))
				defer cancel()
				calls := 0
				svc := &UserGRPC{ProfileMutations: profileMutationLimiterFunc(func(_ context.Context, got uuid.UUID, action string) (bool, error) {
					calls++
					require.Equal(t, account, got)
					require.Equal(t, operation, action)
					if scenario.cancel {
						cancel()
					}
					return scenario.allowed, scenario.err
				})}
				var err error
				require.NotPanics(t, func() {
					if operation == profilemutation.Create {
						response, callErr := svc.CreateProfile(ctx, &userv1.CreateProfileRequest{DisplayName: "Alt"})
						require.Nil(t, response)
						err = callErr
					} else {
						response, callErr := svc.DeleteProfile(ctx, &userv1.DeleteProfileRequest{ProfileId: uuid.NewString()})
						require.Nil(t, response)
						err = callErr
					}
				})
				require.Equal(t, scenario.want, status.Code(err))
				require.NotContains(t, err.Error(), "private redis configuration")
				require.Equal(t, 1, calls)
			})
		}
	}
}

func TestProfileMutationsAuthenticateBeforeQuotaAndHonorCanceledParent(t *testing.T) {
	for _, operation := range []string{profilemutation.Create, profilemutation.Delete} {
		for _, scenario := range []struct {
			name    string
			account string
			cancel  bool
			want    codes.Code
		}{
			{"missing account", "", false, codes.Unauthenticated},
			{"nil account", uuid.Nil.String(), false, codes.Unauthenticated},
			{"canceled parent", uuid.NewString(), true, codes.Canceled},
		} {
			t.Run(operation+"/"+scenario.name, func(t *testing.T) {
				ctx, cancel := context.WithCancel(metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderUserID, scenario.account)))
				defer cancel()
				if scenario.cancel {
					cancel()
				}
				svc := &UserGRPC{ProfileMutations: profileMutationLimiterFunc(func(context.Context, uuid.UUID, string) (bool, error) {
					t.Fatal("unauthenticated/canceled calls must not consume quota")
					return false, nil
				})}
				var err error
				if operation == profilemutation.Create {
					_, err = svc.CreateProfile(ctx, &userv1.CreateProfileRequest{})
				} else {
					_, err = svc.DeleteProfile(ctx, &userv1.DeleteProfileRequest{})
				}
				require.Equal(t, scenario.want, status.Code(err))
			})
		}
	}
}
