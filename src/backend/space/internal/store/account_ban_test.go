package store

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sort"
	"testing"
	"time"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/spaceaccountcapture"
)

func accountBanFixture(t *testing.T, count int) (AccountBanInput, *accountBanSnapshot, *userv1.CaptureSpaceMemberAccountBindingsResponse) {
	t.Helper()
	in := AccountBanInput{OperationID: uuid.New(), SpaceID: uuid.New(), AccountID: uuid.New(), ActorProfileID: uuid.New()}
	snapshot := &accountBanSnapshot{owner: uuid.New(), revision: 1}
	for i := 0; i < count; i++ {
		profile := uuid.New()
		if i == 0 {
			profile = snapshot.owner
		}
		snapshot.sources = append(snapshot.sources, accountBanSource{entry: &userv1.SpaceAccountCaptureEntry{ProfileId: profile.String(), Source: &userv1.SpaceAccountCaptureEntry_Ordinary{Ordinary: &userv1.SpaceOrdinaryPublicationIdentity{PublicationId: uuid.NewString()}}}})
	}
	sort.Slice(snapshot.sources, func(i, j int) bool {
		return bytes.Compare(deterministicBanBytes(snapshot.sources[i].entry), deterministicBanBytes(snapshot.sources[j].entry)) < 0
	})
	cohort := &userv1.SpaceAccountCaptureCohort{}
	for _, source := range snapshot.sources {
		cohort.Entries = append(cohort.Entries, source.entry)
	}
	snapshot.request = &userv1.CaptureSpaceMemberAccountBindingsRequest{Protocol: 1, OperationId: in.OperationID.String(), SpaceId: in.SpaceID.String(), Purpose: 2, Cohort: cohort, CohortDigest: spaceaccountcapture.CohortDigest(cohort)}
	out := &userv1.CaptureSpaceMemberAccountBindingsResponse{Protocol: 1, OperationId: in.OperationID.String(), SpaceId: in.SpaceID.String(), Purpose: 2, CohortDigest: cohortDigestCopy(cohort), RequestDigest: spaceaccountcapture.RequestDigest(snapshot.request), OwnerSourceRevision: 901, UserSourceGeneration: uuid.NewString(), IdentityCut: &userv1.SpaceAccountIdentityCut{Owner: "user", Stream: "identity-association", HistoryGeneration: uuid.NewString(), ThroughRevision: 3, KeysetRevision: 1, Digest: bytes.Repeat([]byte{1}, 32)}}
	for _, entry := range cohort.Entries {
		account := in.AccountID.String()
		if entry.ProfileId == snapshot.owner.String() {
			account = uuid.NewString()
		}
		out.Results = append(out.Results, &userv1.SpaceAccountCaptureResult{Entry: proto.Clone(entry).(*userv1.SpaceAccountCaptureEntry), Outcome: 1, AccountId: account, AssociationId: uuid.NewString(), AssociationRevision: 1, ProfileRevision: 7, Origin: 1, AssociationOrigin: 2, OwnerReceiptId: uuid.NewString(), OwnerReceiptDigest: bytes.Repeat([]byte{2}, 32), CaptureCut: timestamppb.New(time.Now()), Frozen: true, Deleted: true})
	}
	require.NoError(t, spaceaccountcapture.ValidateResponse(snapshot.request, out))
	return in, snapshot, out
}

func cohortDigestCopy(c *userv1.SpaceAccountCaptureCohort) []byte {
	return append([]byte(nil), spaceaccountcapture.CohortDigest(c)...)
}

func TestAccountBanCoverageAll301FrozenAliasesAndDistinctIdentityCut(t *testing.T) {
	in, snapshot, out := accountBanFixture(t, 301)
	profiles, err := accountBanCoverage(in, snapshot, out)
	require.NoError(t, err)
	require.Len(t, profiles, 300, "every alias, including beyond page100, is selected; owner excluded")
	require.NotContains(t, profiles, snapshot.owner)
	require.NotEqual(t, out.OwnerSourceRevision, out.IdentityCut.ThroughRevision, "A is not identity H")
	out.Results = out.Results[:100]
	_, err = accountBanCoverage(in, snapshot, out)
	require.ErrorIs(t, err, ErrAccountBanUnavailable)
}

func TestAccountBanOwnerAliasCannotBeHiddenByHint(t *testing.T) {
	in, snapshot, out := accountBanFixture(t, 3)
	for _, r := range out.Results {
		if r.Entry.ProfileId == snapshot.owner.String() {
			r.AccountId = in.AccountID.String()
		}
	}
	for _, r := range out.Results {
		if r.Entry.ProfileId != snapshot.owner.String() {
			hint := uuid.MustParse(r.Entry.ProfileId)
			in.ProfileHint = &hint
			break
		}
	}
	_, err := accountBanCoverage(in, snapshot, out)
	require.ErrorIs(t, err, ErrCannotBanOwner)
}

func TestAccountBanWrongHintAndNativeDisagreementAreNotNoMatch(t *testing.T) {
	in, snapshot, out := accountBanFixture(t, 3)
	wrong := snapshot.owner
	in.ProfileHint = &wrong
	_, err := accountBanCoverage(in, snapshot, out)
	require.ErrorIs(t, err, ErrMemberAdmissionConflict)
	in.ProfileHint = nil
	snapshot.sources[0].nativeAccount = uuid.New()
	_, err = accountBanCoverage(in, snapshot, out)
	require.ErrorIs(t, err, ErrAccountBanUnavailable)
}

func TestAccountBanIncompleteAndTerminalNeverBecomeSuccessfulPartialBan(t *testing.T) {
	for _, outcome := range []userv1.SpaceAccountCaptureOutcome{2, 3} {
		t.Run(outcome.String(), func(t *testing.T) {
			in, snapshot, out := accountBanFixture(t, 3)
			old := out.Results[0]
			r := &userv1.SpaceAccountCaptureResult{Entry: old.Entry, Outcome: outcome}
			if outcome == 2 {
				r.OwnerReceiptId = uuid.NewString()
				r.OwnerReceiptDigest = bytes.Repeat([]byte{3}, 32)
				r.CaptureCut = old.CaptureCut
				r.TerminalOperationId = uuid.NewString()
				r.TerminalRevision = 1
				r.TerminalReceiptId = uuid.NewString()
				r.TerminalReceiptDigest = bytes.Repeat([]byte{4}, 32)
			}
			out.Results[0] = r
			require.NoError(t, spaceaccountcapture.ValidateResponse(snapshot.request, out), "genuine outcome remains structurally valid")
			_, err := accountBanCoverage(in, snapshot, out)
			require.ErrorIs(t, err, ErrAccountBanUnavailable)
		})
	}
}
