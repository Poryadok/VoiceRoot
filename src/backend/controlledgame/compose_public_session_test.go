package controlledgame

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestT40ComposePublicClientPartyThreeMatchesAndChatMembership(t *testing.T) {
	if os.Getenv("VOICE_T40_COMPOSE_PUBLIC_SESSIONS") != "1" {
		t.Skip("set VOICE_T40_COMPOSE_PUBLIC_SESSIONS=1 for the isolated public-client + service-owner acceptance")
	}
	baseURL := requireT40Env(t, "T40_PUBLIC_GATEWAY_BASE_URL")
	gameCredential := requireT40Env(t, "T40_GAME_SERVER_CREDENTIAL")
	playerOneToken := requireT40Env(t, "T40_PLAYER_ONE_TOKEN")
	playerTwoToken := requireT40Env(t, "T40_PLAYER_TWO_TOKEN")
	profileOne := requireT40UUID(t, "T40_PLAYER_ONE_PROFILE_ID")
	profileTwo := requireT40UUID(t, "T40_PLAYER_TWO_PROFILE_ID")
	chatDatabaseURL := requireT40Env(t, "T40_CHAT_DATABASE_URL")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client, err := NewPublicSessionClient(baseURL, nil)
	require.NoError(t, err)
	chatDB, err := pgxpool.New(ctx, chatDatabaseURL)
	require.NoError(t, err)
	defer chatDB.Close()
	require.NoError(t, chatDB.Ping(ctx))

	profiles := []string{profileOne.String(), profileTwo.String()}
	sort.Strings(profiles)
	partyKey := "t40-party-" + uuid.NewString()
	partyOpID := uuid.NewString()
	_, err = client.CreateSession(ctx, gameCredential, PublicSessionRequest{
		OperationID: partyOpID, Kind: "party", ExternalKey: partyKey, DisplayName: "T40 acceptance party",
		RosterRevision: 1, RosterComplete: true, Members: profiles,
	})
	require.NoError(t, err)
	party := waitT40Operation(t, ctx, client, gameCredential, partyOpID, "active")
	require.NotEmpty(t, party.ChatID)
	require.NotEmpty(t, party.ChatCreateReceiptID)
	require.NotEmpty(t, party.ChatRosterReceiptID)
	require.NotEmpty(t, party.VoiceRoomID)
	require.NotEmpty(t, party.VoiceProvisionReceiptID)
	require.NotEmpty(t, party.RoleGrantReceiptID)

	seenRooms := make(map[string]struct{}, 3)
	firstChildSessionID := ""
	for i := 1; i <= 3; i++ {
		operationID := uuid.NewString()
		_, createErr := client.CreateSession(ctx, gameCredential, PublicSessionRequest{
			OperationID: operationID, Kind: "match", ExternalKey: fmt.Sprintf("t40-match-%d-%s", i, uuid.NewString()),
			ParentPartyKey: partyKey, RosterRevision: 1, RosterComplete: true, Members: profiles,
		})
		require.NoError(t, createErr)
		child := waitT40Operation(t, ctx, client, gameCredential, operationID, "active")
		require.Equal(t, party.ChatID, child.ChatID)
		require.Equal(t, party.ChatOwnerSessionID, child.ChatOwnerSessionID)
		require.Equal(t, party.ChatCreateReceiptID, child.ChatCreateReceiptID)
		require.Equal(t, party.ChatRosterReceiptID, child.ChatRosterReceiptID)
		require.NotEmpty(t, child.VoiceRoomID)
		require.NotEmpty(t, child.VoiceProvisionReceiptID)
		require.NotEmpty(t, child.RoleGrantReceiptID)
		if _, duplicate := seenRooms[child.VoiceRoomID]; duplicate {
			t.Fatalf("two matches reused Voice room %s", child.VoiceRoomID)
		}
		seenRooms[child.VoiceRoomID] = struct{}{}
		switch i {
		case 1:
			firstChildSessionID = child.SessionID
			consentErr := client.ConsentToKeepGroup(ctx, playerOneToken, child.SessionID, PublicConsentRequest{
				OperationID: uuid.NewString(), RosterRevision: 1,
			})
			require.NoError(t, consentErr, "an authenticated participant can consent to the party destination")
			keepGroup, keepErr := client.KeepGroup(ctx, playerOneToken, child.SessionID, PublicConsentRequest{
				OperationID: uuid.NewString(), RosterRevision: 1,
			})
			require.NoError(t, keepErr, "a consenting participant can preserve the group")
			require.Equal(t, child.SessionID, keepGroup.SessionID)
			require.Equal(t, party.ChatID, keepGroup.ChatID)
			require.Equal(t, []string{profileOne.String()}, keepGroup.AddedProfiles, "only the participant who consented is newly authorized")
			require.False(t, keepGroup.Replayed)
		case 2:
			require.NoError(t, client.ConsentToKeepGroup(ctx, playerTwoToken, child.SessionID, PublicConsentRequest{
				OperationID: uuid.NewString(), RosterRevision: 1,
			}), "a different authenticated participant can independently consent")
			keepGroup, keepErr := client.KeepGroup(ctx, playerTwoToken, child.SessionID, PublicConsentRequest{
				OperationID: uuid.NewString(), RosterRevision: 1,
			})
			require.NoError(t, keepErr, "a second consenting participant can independently preserve the group")
			require.Equal(t, party.ChatID, keepGroup.ChatID)
			require.Equal(t, []string{profileTwo.String()}, keepGroup.AddedProfiles, "the second operation adds only its consenting participant")
			require.False(t, keepGroup.Replayed)
		}
	}
	require.Len(t, seenRooms, 3)

	var memberCount int
	require.NoError(t, chatDB.QueryRow(ctx, `SELECT count(*) FROM chat_members WHERE chat_id=$1 AND profile_id=ANY($2::uuid[])`, party.ChatID, profiles).Scan(&memberCount))
	require.Equal(t, 2, memberCount, "actual Chat state contains the party roster")
	var partyMemberCount int
	require.NoError(t, chatDB.QueryRow(ctx, `SELECT count(*) FROM chat_members WHERE chat_id=$1`, party.ChatID).Scan(&partyMemberCount))
	require.Equal(t, 2, partyMemberCount, "keep-group preserves the party roster without adding the nonconsenting participant twice or removing either member")

	_, offlineHistory := t40JSONRequest(t, ctx, baseURL, http.MethodGet, "/api/v1/messages?chat_id="+party.ChatID, playerTwoToken, nil, http.StatusOK)
	require.NotNil(t, offlineHistory["message_list"], "a current Chat member can fetch party history with Voice authentication while no Realtime connection is present")

	closeOperationID := uuid.NewString()
	require.NotEmpty(t, firstChildSessionID)
	require.NoError(t, client.CloseSession(ctx, gameCredential, firstChildSessionID, closeOperationID))
	closed := waitT40Operation(t, ctx, client, gameCredential, closeOperationID, "closed")
	require.Equal(t, party.ChatID, closed.ChatID)
	partyAfterClose := waitT40Operation(t, ctx, client, gameCredential, partyOpID, "active")
	require.Equal(t, party.ChatID, partyAfterClose.ChatID)
	require.Len(t, seenRooms, 3)

	communitySpaceID := exerciseT40CommunityRoster(t, ctx, client, profileOne, profileTwo)
	require.NotEqual(t, uuid.Nil, communitySpaceID)
}

func exerciseT40CommunityRoster(t *testing.T, ctx context.Context, client *PublicSessionClient, profileOne, profileTwo uuid.UUID) uuid.UUID {
	t.Helper()
	ownerToken := requireT40Env(t, "T40_PLAYER_ONE_TOKEN")
	operatorToken := requireT40Env(t, "T40_PLAYER_TWO_TOKEN")
	baseURL := requireT40Env(t, "T40_PUBLIC_GATEWAY_BASE_URL")
	appID := requireT40UUID(t, "T40_APPLICATION_ID")
	envID := requireT40UUID(t, "T40_ENVIRONMENT_ID")
	rosterCredential := requireT40Env(t, "T40_GAME_ROSTER_CREDENTIAL")
	gameDB, err := pgxpool.New(ctx, requireT40Env(t, "T40_GAME_INTEGRATION_DATABASE_URL"))
	require.NoError(t, err)
	defer gameDB.Close()
	spaceDB, err := pgxpool.New(ctx, requireT40Env(t, "T40_SPACE_DATABASE_URL"))
	require.NoError(t, err)
	defer spaceDB.Close()
	require.NoError(t, gameDB.Ping(ctx))
	require.NoError(t, spaceDB.Ping(ctx))

	corpKey := "t40-corp-" + uuid.NewString()
	bootstrapOperationID := uuid.NewString()
	bootstrapPath := "/api/v1/game-integrations/applications/" + appID.String() + "/community-bindings"
	bootstrapBody := map[string]any{"environment_id": envID.String(), "corporation_key": corpKey, "space_template_id": "guild-default", "operation_id": bootstrapOperationID}
	status, _ := t40JSONRequest(t, ctx, baseURL, http.MethodPost, bootstrapPath, ownerToken, bootstrapBody, http.StatusAccepted)
	require.Equal(t, http.StatusAccepted, status)
	_, _ = t40JSONRequest(t, ctx, baseURL, http.MethodPost, bootstrapPath+"/"+bootstrapOperationID+"/approve", operatorToken, nil, http.StatusOK)
	createdStatus, bootstrapResult := t40JSONRequest(t, ctx, baseURL, http.MethodPost, bootstrapPath, ownerToken, bootstrapBody, http.StatusCreated)
	require.Equal(t, http.StatusCreated, createdStatus)
	spaceID := uuid.MustParse(bootstrapResult["space_id"].(string))

	var bindingOne, bindingTwo uuid.UUID
	require.NoError(t, gameDB.QueryRow(ctx, `SELECT binding_id FROM player_bindings WHERE application_id=$1 AND environment_id=$2 AND profile_id=$3 AND status='active'`, appID, envID, profileOne).Scan(&bindingOne))
	require.NoError(t, gameDB.QueryRow(ctx, `SELECT binding_id FROM player_bindings WHERE application_id=$1 AND environment_id=$2 AND profile_id=$3 AND status='active'`, appID, envID, profileTwo).Scan(&bindingTwo))
	members := []PublicCommunityRosterSnapshotMember{
		{BindingID: bindingOne.String(), CharacterKey: "t40-character-1", SourceMembershipRevision: 4, SourceRank: "officer", GrantReasons: []string{"membership", "officer_rank"}},
		{BindingID: bindingOne.String(), CharacterKey: "t40-character-2", SourceMembershipRevision: 4, SourceRank: "member", GrantReasons: []string{"membership"}},
		{BindingID: bindingTwo.String(), CharacterKey: "t40-character-1", SourceMembershipRevision: 2, SourceRank: "member", GrantReasons: []string{"membership"}},
	}
	first := t40CommunityRosterRequest(t, corpKey, 1, members)
	accepted, err := client.ApplyCommunityRosterSnapshot(ctx, rosterCredential, corpKey, first)
	require.NoError(t, err)
	require.Equal(t, "accepted", accepted.Outcome)
	require.Equal(t, 3, accepted.MemberCount)
	view, err := client.GetCommunityRosterSnapshot(ctx, rosterCredential, corpKey)
	require.NoError(t, err)
	require.Len(t, view.Members, 3)
	var officerVisible bool
	for _, member := range view.Members {
		if member.BindingID == bindingOne.String() && member.CharacterKey == "t40-character-1" {
			officerVisible = member.SourceRank == "officer" && len(member.GrantReasons) == 2
		}
	}
	require.True(t, officerVisible, "the officer rank and its independent reason are visible in the accepted snapshot")

	// The same bound account now loses the officer rank for its first character
	// while its second character and corporation membership remain active.
	downgraded := append([]PublicCommunityRosterSnapshotMember(nil), members...)
	downgraded[0].SourceMembershipRevision = 5
	downgraded[0].SourceRank = "member"
	downgraded[0].GrantReasons = []string{"membership"}
	second := t40CommunityRosterRequest(t, corpKey, 2, downgraded)
	result, err := client.ApplyCommunityRosterSnapshot(ctx, rosterCredential, corpKey, second)
	require.NoError(t, err)
	require.Equal(t, "accepted", result.Outcome)
	view, err = client.GetCommunityRosterSnapshot(ctx, rosterCredential, corpKey)
	require.NoError(t, err)
	require.Len(t, view.Members, 3, "two character references for one player remain separate from the second player")
	for _, member := range view.Members {
		require.Equal(t, "member", member.SourceRank)
		require.Equal(t, []string{"membership"}, member.GrantReasons)
	}
	stale := t40CommunityRosterRequest(t, corpKey, 1, members)
	_, err = client.ApplyCommunityRosterSnapshot(ctx, rosterCredential, corpKey, stale)
	require.ErrorContains(t, err, "error_code=STALE_REVISION", "the public path rejects an older owner snapshot")
	view, err = client.GetCommunityRosterSnapshot(ctx, rosterCredential, corpKey)
	require.NoError(t, err)
	require.Equal(t, int64(2), view.SourceRevision, "a rejected stale snapshot cannot roll back the current roster")
	require.Len(t, view.Members, 3)
	var activeOfficer, revokedOfficer, currentGISGrants int
	require.NoError(t, gameDB.QueryRow(ctx, `SELECT count(*) FILTER (WHERE source_reason='officer_rank' AND revoked_at IS NULL), count(*) FILTER (WHERE source_reason='officer_rank' AND revoked_at IS NOT NULL), count(*) FILTER (WHERE source_reason='membership' AND revoked_at IS NULL) FROM gis_community_grants WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3`, appID, envID, corpKey).Scan(&activeOfficer, &revokedOfficer, &currentGISGrants))
	require.Zero(t, activeOfficer)
	require.Equal(t, 1, revokedOfficer)
	require.Equal(t, 3, currentGISGrants, "the rank reason is revoked without removing membership grants")
	var projectedMembers int
	var projectedRevision int64
	require.NoError(t, spaceDB.QueryRow(ctx, `SELECT count(*), min(source_revision) FROM community_roster_members WHERE space_id=$1 AND revoked_at IS NULL AND lease_expires_at>clock_timestamp()`, spaceID).Scan(&projectedMembers, &projectedRevision))
	require.Equal(t, 2, projectedMembers, "multiple characters for one profile project to one Voice community member")
	require.Equal(t, int64(2), projectedRevision, "Space has applied the accepted rank-downgrade snapshot")
	var activeCommunityHead int
	require.NoError(t, spaceDB.QueryRow(ctx, `SELECT count(*) FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='ACTIVE'`, spaceID, profileTwo).Scan(&activeCommunityHead))
	require.Equal(t, 1, activeCommunityHead, "the target has an active managed membership before Ban")
	_, bannedAccount := t40JSONRequest(t, ctx, baseURL, http.MethodPost, "/api/v1/auth/validate", operatorToken, nil, http.StatusOK)
	accountID, ok := bannedAccount["user_id"].(string)
	require.True(t, ok)
	_, _ = t40JSONRequest(t, ctx, baseURL, http.MethodPost, "/api/v1/spaces/"+spaceID.String()+"/bans", ownerToken, map[string]any{
		"operation_id": uuid.NewString(), "account_id": accountID, "profile_id": profileTwo.String(), "reason": "T40 banned-user acceptance",
	}, http.StatusNoContent)
	var activeSpaceMember, retiredProjection, spaceBan, currentOwnerGrant int
	require.NoError(t, spaceDB.QueryRow(ctx, `SELECT count(*) FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='ACTIVE'`, spaceID, profileTwo).Scan(&activeSpaceMember))
	require.NoError(t, spaceDB.QueryRow(ctx, `SELECT count(*) FROM space_bans WHERE space_id=$1 AND account_id=$2`, spaceID, accountID).Scan(&spaceBan))
	require.NoError(t, spaceDB.QueryRow(ctx, `SELECT count(*) FROM community_roster_members WHERE space_id=$1 AND profile_id=$2 AND revoked_at IS NOT NULL`, spaceID, profileTwo).Scan(&retiredProjection))
	require.NoError(t, gameDB.QueryRow(ctx, `SELECT count(*) FROM gis_community_grants WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 AND binding_id=$4 AND source_reason='membership' AND revoked_at IS NULL`, appID, envID, corpKey, bindingTwo).Scan(&currentOwnerGrant))
	ownerAfterBan, err := client.GetCommunityRosterSnapshot(ctx, rosterCredential, corpKey)
	require.NoError(t, err)
	require.Zero(t, activeSpaceMember, "Space ban evicts the selected profile from live community membership")
	require.Equal(t, 1, spaceBan)
	require.Equal(t, 1, retiredProjection, "Space retires its local access projection")
	require.Equal(t, 1, currentOwnerGrant, "the independent GIS membership grant remains active")
	require.Equal(t, view.SourceRevision, ownerAfterBan.SourceRevision)
	require.ElementsMatch(t, view.Members, ownerAfterBan.Members, "the independent GIS roster snapshot remains unchanged")
	return spaceID
}

func t40CommunityRosterRequest(t *testing.T, corporationKey string, revision int64, members []PublicCommunityRosterSnapshotMember) PublicCommunityRosterRequest {
	t.Helper()
	members = append([]PublicCommunityRosterSnapshotMember(nil), members...)
	sort.Slice(members, func(i, j int) bool {
		left := members[i].BindingID + "\x00" + members[i].CharacterKey
		right := members[j].BindingID + "\x00" + members[j].CharacterKey
		return left < right
	})
	canonicalMembers := make([]map[string]any, 0, len(members))
	for _, member := range members {
		canonicalMembers = append(canonicalMembers, map[string]any{
			"binding_id": member.BindingID, "character_key": member.CharacterKey,
			"source_membership_revision": member.SourceMembershipRevision, "source_rank": member.SourceRank,
			"grant_reasons": member.GrantReasons,
		})
	}
	pageValue := map[string]any{"index": int32(0), "count": int32(1), "members": canonicalMembers}
	pageBytes, err := json.Marshal(pageValue)
	require.NoError(t, err)
	pageHash := sha256.Sum256(pageBytes)
	pageHashHex := hex.EncodeToString(pageHash[:])
	operationID := uuid.NewString()
	snapshotValue := map[string]any{"operation_id": operationID, "corporation_key": corporationKey, "source_revision": revision, "owner_generation": int64(1), "pages": []string{pageHashHex}}
	snapshotBytes, err := json.Marshal(snapshotValue)
	require.NoError(t, err)
	snapshotHash := sha256.Sum256(snapshotBytes)
	return PublicCommunityRosterRequest{OperationID: operationID, SourceRevision: revision, OwnerGeneration: 1,
		SnapshotSHA256: hex.EncodeToString(snapshotHash[:]), Pages: []PublicCommunityRosterPage{{Index: 0, Count: 1,
			SHA256: pageHashHex, Members: members}}}
}

func t40JSONRequest(t *testing.T, ctx context.Context, baseURL, method, path, token string, body map[string]any, wantStatus int) (int, map[string]any) {
	t.Helper()
	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		require.NoError(t, err)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL, "/")+path, bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	transport := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := transport.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var result map[string]any
	if resp.Body != nil {
		_ = json.NewDecoder(resp.Body).Decode(&result)
	}
	require.Equal(t, wantStatus, resp.StatusCode, fmt.Sprintf("%s %s returned %v", method, path, result))
	return resp.StatusCode, result
}

func waitT40Operation(t *testing.T, ctx context.Context, client *PublicSessionClient, credential, operationID, sessionStatus string) PublicSessionOperation {
	t.Helper()
	for {
		operation, err := client.GetOperation(ctx, credential, operationID)
		if err == nil && operation.SessionStatus == sessionStatus {
			return operation
		}
		if err != nil && ctx.Err() == nil {
			t.Logf("poll operation %s: %v", operationID, err)
		}
		if ctx.Err() != nil {
			t.Fatalf("operation %s did not reach session status %q before timeout: %v", operationID, sessionStatus, ctx.Err())
		}
		select {
		case <-ctx.Done():
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func requireT40Env(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	require.NotEmpty(t, value, "%s is required for T40 Compose acceptance", name)
	return value
}

func requireT40UUID(t *testing.T, name string) uuid.UUID {
	t.Helper()
	value, err := uuid.Parse(requireT40Env(t, name))
	require.NoError(t, err, "%s must be a UUID", name)
	return value
}
