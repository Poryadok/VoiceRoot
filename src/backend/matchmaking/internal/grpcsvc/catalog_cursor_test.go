package grpcsvc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	commonv1 "voice.app/voice/common/v1"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/store"
)

func TestListGames_InvalidCursorRejectedBeforeDatabase(t *testing.T) {
	for _, cursor := range []string{"bad|cursor", " ", "gpop2." + strings.Repeat("a", 64) + ".1", strings.Repeat("x", 129)} {
		t.Run(cursor[:min(len(cursor), 16)], func(t *testing.T) {
			svc := &MatchmakingGRPC{Games: &store.GameStore{}}
			_, err := svc.ListGames(context.Background(), &matchmakingv1.ListGamesRequest{
				Page: &commonv1.CursorPageRequest{Cursor: cursor},
			})
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.Equal(t, "invalid games cursor", status.Convert(err).Message())
		})
	}
}

func TestPublicGamesErrorMapping(t *testing.T) {
	require.Equal(t, codes.FailedPrecondition, status.Code(publicGamesError(store.ErrGamesCursorStale)))
	require.Equal(t, "games_cursor_stale", status.Convert(publicGamesError(store.ErrGamesCursorStale)).Message())
	err := publicGamesError(errors.New("private database diagnostic"))
	require.Equal(t, codes.Internal, status.Code(err))
	require.Equal(t, "list games unavailable", status.Convert(err).Message())
}
