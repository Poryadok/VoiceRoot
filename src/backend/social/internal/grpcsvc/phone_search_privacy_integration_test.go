package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	socialv1 "voice.app/voice/social/v1"
	"voice/backend/pkg/privacy"
)

type phoneSearchPrivacyStub struct {
	friendsOnly map[uuid.UUID]bool
}

func (s phoneSearchPrivacyStub) AllowPhoneSearchAudience(_ context.Context, profileID uuid.UUID) (privacy.Audience, error) {
	if s.friendsOnly[profileID] {
		return privacy.FriendsOnly(), nil
	}
	return privacy.EveryoneWithGuests(), nil
}

type phoneHashLookupStub struct {
	byHash map[string]uuid.UUID
}

func (s phoneHashLookupStub) ProfileIDsByPhoneHashes(_ context.Context, hashes []string) (map[string]uuid.UUID, error) {
	out := make(map[string]uuid.UUID, len(hashes))
	for _, h := range hashes {
		if id, ok := s.byHash[h]; ok {
			out[h] = id
		}
	}
	return out, nil
}

// TestSyncPhoneContacts_FriendsOnlyPrivacy_StrangerExcluded documents privacy.md: friends-only allow_phone_search excludes non-friend profiles from sync matches.
func TestSyncPhoneContacts_FriendsOnlyPrivacy_StrangerExcluded(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSocialPostgresForTest(t, ctx)
	applySocialMigration(t, ctx, pool)

	syncer := uuid.New()
	target := uuid.New()
	hash := "sha256-deadbeef-phone-hash"

	client, cleanup := startSocialGRPCTestServer(t, pool,
		withPhoneSearchPrivacy(phoneSearchPrivacyStub{friendsOnly: map[uuid.UUID]bool{target: true}}),
		withPhoneHashLookup(phoneHashLookupStub{byHash: map[string]uuid.UUID{hash: target}}),
	)
	t.Cleanup(cleanup)

	resp, err := client.SyncPhoneContacts(withProfileCtx(ctx, syncer), &socialv1.SyncPhoneContactsRequest{
		HashedPhoneNumbers: []string{hash},
	})
	require.NoError(t, err)
	require.NotContains(t, resp.GetMatchedProfileIds(), target.String())
}
