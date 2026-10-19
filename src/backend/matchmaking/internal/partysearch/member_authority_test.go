package partysearch

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/partyauthority"
)

func TestSourcePartyPrivateAuthorityPersistsExactTupleManifestAndOpaqueCut(t *testing.T) {
	pool := partyDB(t)
	p := partyParams(t, pool)
	parties := &store.PartyStore{Pool: pool}
	created, err := parties.CreateSourceParty(context.Background(), p)
	require.NoError(t, err)
	require.Equal(t, p.MemberAuthorities, created.Party.MemberAuthorities)
	require.Equal(t, p.MemberAuthorityVersion, created.Party.MemberAuthorityVersion)
	require.Equal(t, p.SourceAuthorityHash, created.Party.SourceAuthorityHash)
	raw, hash, err := partyauthority.Encode(p.MemberAuthorities)
	require.NoError(t, err)
	require.Equal(t, hash, created.Party.MemberAuthorityHash)
	fresh, err := parties.GetSourceParty(context.Background(), created.Party.ID)
	require.NoError(t, err)
	require.Equal(t, created.Party, fresh)
	var persisted []byte
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT voice_member_authority FROM parties WHERE id=$1`, fresh.ID).Scan(&persisted))
	require.Equal(t, raw, persisted)
	p.SourceAuthorityHash[0]++
	_, err = parties.CreateSourceParty(context.Background(), p)
	require.ErrorIs(t, err, store.ErrSourcePartyChanged)
	_, err = pool.Exec(context.Background(), `UPDATE parties SET voice_source_authority_sha256=$2 WHERE id=$1`, fresh.ID, p.SourceAuthorityHash[:])
	require.Error(t, err, "the existing immutable trigger must cover additive private columns")
}

func TestSourcePartyMalformedOrMissingPrivateAuthorityCannotEnqueue(t *testing.T) {
	pool := partyDB(t)
	for _, tc := range []struct {
		name   string
		mutate func(*store.CreateSourcePartyParams)
	}{
		{"version missing", func(p *store.CreateSourcePartyParams) { p.MemberAuthorityVersion = 0 }},
		{"version unsupported", func(p *store.CreateSourcePartyParams) { p.MemberAuthorityVersion = 2 }},
		{"missing tuples", func(p *store.CreateSourcePartyParams) { p.MemberAuthorities = nil }},
		{"missing source cut", func(p *store.CreateSourcePartyParams) { p.SourceAuthorityHash = [32]byte{} }},
		{"different profile", func(p *store.CreateSourcePartyParams) { p.MemberAuthorities[1].ProfileID = uuid.New() }},
		{"duplicate account", func(p *store.CreateSourcePartyParams) { p.MemberAuthorities[1].AccountID = p.InitiatorAccountID }},
		{"zero account", func(p *store.CreateSourcePartyParams) { p.MemberAuthorities[1].AccountID = uuid.Nil }},
		{"unknown epoch", func(p *store.CreateSourcePartyParams) { p.MemberAuthorities[1].SessionEpoch = 0 }},
		{"initiator account mismatch", func(p *store.CreateSourcePartyParams) { p.MemberAuthorities[0].AccountID = uuid.New() }},
		{"solo device epoch mismatch", func(p *store.CreateSourcePartyParams) {
			p.SourceKind = "SOLO"
			p.SourceRoomID = nil
			p.SourceRosterVersion = 0
			p.MemberProfileIDs = p.MemberProfileIDs[:1]
			p.MemberAuthorities = p.MemberAuthorities[:1]
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := partyParams(t, pool)
			tc.mutate(&p)
			_, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(context.Background(), p)
			require.ErrorIs(t, err, store.ErrSourcePartyInvalid)
		})
	}
	var count int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM parties`).Scan(&count))
	require.Zero(t, count)
}

func TestSourcePartyBannedPeerRejectsWholeEnqueueWithoutPartialRows(t *testing.T) {
	pool := partyDB(t)
	p := partyParams(t, pool)
	require.NoError(t, (&store.BanStore{Pool: pool}).InsertPlatformMMBan(context.Background(), store.InsertPlatformMMBanParams{AccountID: p.MemberAuthorities[1].AccountID, BannedByProfileID: uuid.New()}))
	_, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(context.Background(), p)
	require.ErrorIs(t, err, store.ErrPlatformBanDenied)
	var parties, sessions, intents int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM parties),(SELECT count(*) FROM search_sessions),(SELECT count(*) FROM matchmaking_party_search_queue_intents)`).Scan(&parties, &sessions, &intents))
	require.Zero(t, parties)
	require.Zero(t, sessions)
	require.Zero(t, intents)
}

func TestSourcePartyHistoricalOrCorruptPrivateCaptureIsUnavailable(t *testing.T) {
	pool := partyDB(t)
	ctx := context.Background()
	p := partyParams(t, pool)
	parties := &store.PartyStore{Pool: pool}
	created, err := parties.CreateSourceParty(ctx, p)
	require.NoError(t, err)
	raw, hash, err := partyauthority.Encode(p.MemberAuthorities)
	require.NoError(t, err)
	// Deliberate privileged corruption is confined to this disposable fixture DB.
	_, err = pool.Exec(ctx, `ALTER TABLE parties DISABLE TRIGGER matchmaking_party_source_immutable`)
	require.NoError(t, err)
	for _, mutation := range []string{
		`voice_member_authority_version=NULL,voice_member_authority=NULL,voice_member_authority_sha256=NULL,voice_source_authority_sha256=NULL`,
		`voice_member_authority=decode(repeat('00',48),'hex')`,
		`voice_member_authority_sha256=decode(repeat('00',32),'hex')`,
		`voice_source_authority_sha256=decode(repeat('00',32),'hex')`,
	} {
		_, err = pool.Exec(ctx, `UPDATE parties SET `+mutation+` WHERE id=$1`, created.Party.ID)
		require.NoError(t, err)
		_, err = parties.GetSourceParty(ctx, created.Party.ID)
		require.ErrorIs(t, err, store.ErrSourcePartyUnavailable)
		_, err = pool.Exec(ctx, `UPDATE parties SET voice_member_authority_version=1,voice_member_authority=$2,voice_member_authority_sha256=$3,voice_source_authority_sha256=$4 WHERE id=$1`, created.Party.ID, raw, hash[:], p.SourceAuthorityHash[:])
		require.NoError(t, err)
	}
	_, err = pool.Exec(ctx, `ALTER TABLE parties ENABLE TRIGGER matchmaking_party_source_immutable`)
	require.NoError(t, err)
	_, err = parties.GetSourceParty(ctx, created.Party.ID)
	require.NoError(t, err)
	body, err := os.ReadFile("../../../migrations/matchmaking_db/000022_party_member_authority.down.sql")
	require.NoError(t, err)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, string(body))
	require.ErrorContains(t, err, "must be retired")
	_, err = conn.Exec(ctx, `ROLLBACK`)
	require.NoError(t, err)
}
