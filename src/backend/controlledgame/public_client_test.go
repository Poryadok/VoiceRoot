package controlledgame

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicSessionClientUsesFrozenRoutesAndSeparateGameAndPlayerCredentials(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.HasSuffix(r.URL.Path, "/consent") || strings.HasSuffix(r.URL.Path, "/keep-group") {
			require.Equal(t, "Bearer player-voice-token", r.Header.Get("Authorization"))
			require.Equal(t, http.MethodPost, r.Method)
			if strings.HasSuffix(r.URL.Path, "/keep-group") {
				_, _ = w.Write([]byte(`{"operation_id":"keep-group-operation"}`))
			} else {
				_, _ = w.Write([]byte(`{"status":"accepted"}`))
			}
			return
		}
		require.Equal(t, "Bearer scoped-game-credential", r.Header.Get("Authorization"))
		switch r.Method {
		case http.MethodPost:
			require.True(t, r.URL.Path == "/api/v1/sessions" || strings.HasSuffix(r.URL.Path, "/close"))
		case http.MethodGet:
			require.True(t, strings.HasPrefix(r.URL.Path, "/api/v1/operations/"))
		case http.MethodPut:
			require.True(t, strings.HasSuffix(r.URL.Path, "/roster"))
		default:
			t.Fatalf("unexpected public client method %s", r.Method)
		}
		status := http.StatusAccepted
		if r.Method == http.MethodGet {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{"operation_id":"operation-%d","status":"pending"}`, calls)
	}))
	defer server.Close()

	client, err := NewPublicSessionClient(server.URL, server.Client())
	require.NoError(t, err)
	ctx := context.Background()
	party, err := client.CreateSession(ctx, "scoped-game-credential", PublicSessionRequest{
		OperationID: "party-operation", Kind: "party", ExternalKey: "party-1", DisplayName: "Raid",
		RosterRevision: 1, RosterComplete: true, Members: []string{"profile-a", "profile-b"},
	})
	require.NoError(t, err)
	require.Equal(t, "operation-1", party.OperationID)
	_, err = client.GetOperation(ctx, "scoped-game-credential", party.OperationID)
	require.NoError(t, err)
	_, err = client.UpdateRoster(ctx, "scoped-game-credential", "session-1", PublicRosterRequest{
		OperationID: "roster-operation", RosterRevision: 2, RosterComplete: true, Members: []string{"profile-a"},
	})
	require.NoError(t, err)
	require.NoError(t, client.ConsentToKeepGroup(ctx, "player-voice-token", "session-1", PublicConsentRequest{
		OperationID: "consent-operation", RosterRevision: 2,
	}))
	keepGroup, err := client.KeepGroup(ctx, "player-voice-token", "session-1", PublicConsentRequest{
		OperationID: "keep-group-operation", RosterRevision: 2,
	})
	require.NoError(t, err)
	require.Equal(t, "keep-group-operation", keepGroup.OperationID)
	require.NoError(t, client.CloseSession(ctx, "scoped-game-credential", "session-1", "close-operation"))
	require.Equal(t, 6, calls)
}

func TestPublicSessionClientRejectsRedirectsAndUnexpectedStatuses(t *testing.T) {
	redirected := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/target" {
			redirected = true
		}
		http.Redirect(w, r, "/target", http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	client, err := NewPublicSessionClient(server.URL, server.Client())
	require.NoError(t, err)
	_, err = client.CreateSession(context.Background(), "scoped-game-credential", PublicSessionRequest{
		OperationID: "operation", Kind: "party", ExternalKey: "party", RosterRevision: 1,
		RosterComplete: true, Members: []string{"profile"},
	})
	require.Error(t, err)
	require.False(t, redirected, "the public test client never follows a credential-bearing redirect")

	_, err = NewPublicSessionClient("file:///tmp/game-api", server.Client())
	require.Error(t, err, "only HTTP(S) endpoints are accepted")
}

func TestPublicSessionClientDrivesCommunityRosterWithRosterCredential(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/api/v1/community-bindings/corp-1/roster", r.URL.Path)
		require.Equal(t, "Bearer game-roster-credential", r.Header.Get("Authorization"))
		if r.Method == http.MethodPut {
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			if calls == 1 {
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"operation_id":"roster-op","outcome":"accepted","member_count":1}`))
			} else {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"operation_id":"roster-op","outcome":"accepted","member_count":1,"replayed":true}`))
			}
			return
		}
		require.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"corporation_key":"corp-1","source_revision":2,"members":[{"binding_id":"binding-1","character_key":"char-1","source_rank":"officer","grant_reasons":["membership","officer_rank"]}]}`))
	}))
	defer server.Close()
	client, err := NewPublicSessionClient(server.URL, server.Client())
	require.NoError(t, err)

	request := PublicCommunityRosterRequest{
		OperationID: "roster-op", SourceRevision: 2, OwnerGeneration: 1, SnapshotSHA256: strings.Repeat("a", 64),
		Pages: []PublicCommunityRosterPage{{Index: 0, Count: 1, SHA256: strings.Repeat("b", 64), Members: []PublicCommunityRosterSnapshotMember{{
			BindingID: "binding-1", CharacterKey: "char-1", SourceMembershipRevision: 4,
			SourceRank: "officer", GrantReasons: []string{"membership", "officer_rank"},
		}}}},
	}
	result, err := client.ApplyCommunityRosterSnapshot(context.Background(), "game-roster-credential", "corp-1", request)
	require.NoError(t, err)
	require.Equal(t, "accepted", result.Outcome)
	view, err := client.GetCommunityRosterSnapshot(context.Background(), "game-roster-credential", "corp-1")
	require.NoError(t, err)
	require.EqualValues(t, 2, view.SourceRevision)
	require.Equal(t, "officer", view.Members[0].SourceRank)
	require.Equal(t, []string{"membership", "officer_rank"}, view.Members[0].GrantReasons)
	replay, err := client.ApplyCommunityRosterSnapshot(context.Background(), "game-roster-credential", "corp-1", request)
	require.NoError(t, err, "exact public-route replay returns the saved 200 receipt")
	require.True(t, replay.Replayed)
	require.Equal(t, 3, calls)
}

func TestPublicSessionClientReportsOnlyValidatedServerErrorCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error_code":"KEEP_GROUP_UNAVAILABLE","access_token":"do-not-report"}`))
	}))
	defer server.Close()
	client, err := NewPublicSessionClient(server.URL, server.Client())
	require.NoError(t, err)
	_, err = client.GetOperation(context.Background(), "scoped-game-credential", "operation")
	require.ErrorContains(t, err, "error_code=KEEP_GROUP_UNAVAILABLE")
	require.NotContains(t, err.Error(), "do-not-report")
}
