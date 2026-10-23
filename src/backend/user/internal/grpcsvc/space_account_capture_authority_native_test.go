package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/spaceaccountcapture"
	"voice/backend/user/internal/identityassociation"
	"voice/backend/user/internal/testsupport/associationfixture"
)

func nativeCaptureAuthority(t *testing.T, count int) (associationfixture.Database, *identityassociation.Store, *identityassociation.Keyset, *CanonicalSpaceAccountCaptureAuthority) {
	t.Helper()
	db := associationfixture.New(t, count)
	secret := bytes.Repeat([]byte{7}, 32)
	keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: 1, Versions: map[uint32][32]byte{1: sha256.Sum256(secret)}}, map[uint32][]byte{1: secret})
	require.NoError(t, err)
	var generation uuid.UUID
	require.NoError(t, db.Pool.QueryRow(context.Background(), `SELECT history_generation FROM identity_meta WHERE singleton`).Scan(&generation))
	store, err := identityassociation.NewStore(db.Pool, keys, generation)
	require.NoError(t, err)
	require.NoError(t, store.Bootstrap(context.Background()))
	require.NoError(t, store.ActivateGuards(context.Background()))
	authority, err := NewCanonicalSpaceAccountCaptureAuthority(store)
	require.NoError(t, err)
	return db, store, keys, authority
}
func nativeCaptureRequest(t *testing.T, profiles []uuid.UUID) *userv1.CaptureSpaceMemberAccountBindingsRequest {
	t.Helper()
	request := captureTestRequest()
	for _, id := range profiles {
		request.Cohort.Entries = append(request.Cohort.Entries, &userv1.SpaceAccountCaptureEntry{ProfileId: id.String(), Source: &userv1.SpaceAccountCaptureEntry_Ordinary{Ordinary: &userv1.SpaceOrdinaryPublicationIdentity{PublicationId: uuid.NewString()}}})
	}
	canonicalizeNativeCohort(t, request)
	return request
}
func canonicalizeNativeCohort(t *testing.T, request *userv1.CaptureSpaceMemberAccountBindingsRequest) {
	t.Helper()
	sort.Slice(request.Cohort.Entries, func(i, j int) bool {
		a, err := proto.MarshalOptions{Deterministic: true}.Marshal(request.Cohort.Entries[i])
		require.NoError(t, err)
		b, err := proto.MarshalOptions{Deterministic: true}.Marshal(request.Cohort.Entries[j])
		require.NoError(t, err)
		return bytes.Compare(a, b) < 0
	})
	request.CohortDigest = spaceaccountcapture.CohortDigest(request.Cohort)
}

func TestSpaceCaptureCanonicalProducerNativeCompleteCohort(t *testing.T) {
	db, _, _, authority := nativeCaptureAuthority(t, 301)
	request := nativeCaptureRequest(t, db.Profiles)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	response, err := authority.CaptureSpaceMemberAccountBindings(ctx, request)
	require.NoError(t, err)
	require.NoError(t, spaceaccountcapture.ValidateResponse(request, response))
	require.Len(t, response.Results, 301)
	for _, result := range response.Results {
		require.Equal(t, userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_ASSOCIATED, result.Outcome)
		require.Equal(t, db.Accounts[uuid.MustParse(result.Entry.ProfileId)].String(), result.AccountId)
	}
	retry, err := authority.CaptureSpaceMemberAccountBindings(context.Background(), request)
	require.NoError(t, err)
	require.True(t, proto.Equal(response, retry), "lost reply recapture returns stable facts and cuts")
	// The owner never invents a receipt for missing history, even in a mixed
	// complete cohort containing otherwise proved retained profiles.
	unknown := uuid.New()
	request.Cohort.Entries = append(request.Cohort.Entries, &userv1.SpaceAccountCaptureEntry{ProfileId: unknown.String(), Source: &userv1.SpaceAccountCaptureEntry_Ordinary{Ordinary: &userv1.SpaceOrdinaryPublicationIdentity{PublicationId: uuid.NewString()}}})
	canonicalizeNativeCohort(t, request)
	response, err = authority.CaptureSpaceMemberAccountBindings(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, response.Results, 302)
	for _, result := range response.Results {
		if result.Entry.ProfileId == unknown.String() {
			require.Equal(t, userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_INCOMPLETE, result.Outcome)
			require.Empty(t, result.OwnerReceiptId)
			require.Nil(t, result.CaptureCut)
			require.Empty(t, result.AccountId)
		}
	}
	// An expected association mismatch cannot move the profile to another owner.
	var entry *userv1.SpaceAccountCaptureEntry
	for _, candidate := range request.Cohort.Entries {
		if candidate.ProfileId == db.Profiles[0].String() {
			entry = candidate
			break
		}
	}
	require.NotNil(t, entry)
	entry.ExpectedAssociationId = uuid.NewString()
	entry.ExpectedAssociationRevision = 1
	canonicalizeNativeCohort(t, request)
	response, err = authority.CaptureSpaceMemberAccountBindings(context.Background(), request)
	require.NoError(t, err)
	for _, result := range response.Results {
		if result.Entry.ProfileId == entry.ProfileId {
			require.Equal(t, userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_INCOMPLETE, result.Outcome)
			require.Empty(t, result.AccountId)
		}
	}
}

type nativeCaptureParentFixture struct {
	fact identityassociation.ParentObservation
}

func (f nativeCaptureParentFixture) ObserveParent(context.Context, uuid.UUID) (identityassociation.ParentObservation, error) {
	return f.fact, nil
}

// The Auth observation is an explicit trusted-adapter fixture. All User capture,
// terminal preservation, raw deletion and response projection use real SQL.
func TestSpaceCaptureCanonicalProducerNativeArchiveAndRawErasure(t *testing.T) {
	db, store, keys, authority := nativeCaptureAuthority(t, 1)
	ctx := context.Background()
	profile := db.Profiles[0]
	account := db.Accounts[profile]
	request := nativeCaptureRequest(t, db.Profiles)
	initial, err := authority.CaptureSpaceMemberAccountBindings(ctx, request)
	require.NoError(t, err)
	first := initial.Results[0]
	_, err = db.Pool.Exec(ctx, `UPDATE profiles SET deleted_at=clock_timestamp(),frozen_at=clock_timestamp() WHERE id=$1`, profile)
	require.NoError(t, err)
	archived, err := authority.CaptureSpaceMemberAccountBindings(ctx, request)
	require.NoError(t, err)
	current := archived.Results[0]
	require.Equal(t, userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_ASSOCIATED, current.Outcome)
	require.True(t, current.Deleted)
	require.True(t, current.Frozen)
	require.Equal(t, first.OwnerReceiptId, current.OwnerReceiptId)
	require.Equal(t, first.OwnerReceiptDigest, current.OwnerReceiptDigest)
	require.Greater(t, current.AssociationRevision, first.AssociationRevision)
	now := time.Now().UTC()
	parent := identityassociation.ParentObservation{AccountID: account, OwnerHandle: "fixture-auth-terminal", LifecycleRevision: 1, State: identityassociation.ParentTerminal, Eligibility: identityassociation.ParentIneligible, Disposition: identityassociation.ParentIrreversible, CycleID: uuid.New(), EraseAt: now.Add(-2 * time.Second), ObservedAt: now.Add(-time.Second), ValidUntil: now.Add(time.Minute), Cut: identityassociation.ParentCut{Owner: "auth", Stream: "fixture-lifetime", HistoryGeneration: "fixture-generation", ThroughRevision: 1, Digest: [32]byte{1}, KeysetRevision: 1, RequiredVersions: []uint32{1}, Complete: true}}
	verifier, err := identityassociation.NewParentVerifier(nativeCaptureParentFixture{parent})
	require.NoError(t, err)
	proof, err := verifier.Verify(ctx, account)
	require.NoError(t, err)
	prepared, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	tx, err := store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	terminal, err := store.AttachParentTx(ctx, tx, prepared, current.AssociationRevision, proof)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	tx, err = store.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, store.PrepareTerminalDeleteTx(ctx, tx, prepared, terminal.Revision))
	_, err = tx.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, profile)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	erased, err := authority.CaptureSpaceMemberAccountBindings(ctx, request)
	require.NoError(t, err)
	require.NoError(t, spaceaccountcapture.ValidateResponse(request, erased))
	result := erased.Results[0]
	require.Equal(t, userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_TERMINAL_PROFILE, result.Outcome)
	require.Equal(t, first.OwnerReceiptId, result.OwnerReceiptId)
	require.Equal(t, terminal.Terminal.ReceiptID.String(), result.TerminalReceiptId)
	require.Empty(t, result.AccountId)
	require.Empty(t, result.AssociationId)
	require.Zero(t, result.ProfileRevision)
	require.Zero(t, result.AssociationRevision)
	require.Zero(t, result.Origin)
	require.Zero(t, result.AssociationOrigin)
	retry, err := authority.CaptureSpaceMemberAccountBindings(ctx, request)
	require.NoError(t, err)
	require.True(t, proto.Equal(erased, retry), "raw erasure never replays the old raw-account response")
}
