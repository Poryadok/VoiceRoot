package spaceaccountcapture

import (
	"bytes"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	userv1 "voice.app/voice/user/v1"
)

const id = "11111111-1111-4111-8111-111111111111"

func request(t *testing.T, count int) *userv1.CaptureSpaceMemberAccountBindingsRequest {
	t.Helper()
	r := &userv1.CaptureSpaceMemberAccountBindingsRequest{Protocol: 1, OperationId: id, SpaceId: id,
		Purpose: userv1.SpaceAccountCapturePurpose_SPACE_ACCOUNT_CAPTURE_PURPOSE_BAN_PREFLIGHT,
		Cohort:  &userv1.SpaceAccountCaptureCohort{}}
	for i := 0; i < count; i++ {
		r.Cohort.Entries = append(r.Cohort.Entries, &userv1.SpaceAccountCaptureEntry{
			ProfileId: fmt.Sprintf("%08x-1111-4111-8111-111111111111", i+1),
			Source:    &userv1.SpaceAccountCaptureEntry_Ordinary{Ordinary: &userv1.SpaceOrdinaryPublicationIdentity{PublicationId: id}},
		})
	}
	sort.Slice(r.Cohort.Entries, func(i, j int) bool {
		a, err := proto.MarshalOptions{Deterministic: true}.Marshal(r.Cohort.Entries[i])
		require.NoError(t, err)
		b, err := proto.MarshalOptions{Deterministic: true}.Marshal(r.Cohort.Entries[j])
		require.NoError(t, err)
		return bytes.Compare(a, b) < 0
	})
	r.CohortDigest = CohortDigest(r.Cohort)
	return r
}

func response(t *testing.T, r *userv1.CaptureSpaceMemberAccountBindingsRequest) *userv1.CaptureSpaceMemberAccountBindingsResponse {
	t.Helper()
	out := &userv1.CaptureSpaceMemberAccountBindingsResponse{Protocol: r.Protocol, OperationId: r.OperationId,
		Purpose: r.Purpose, SpaceId: r.SpaceId, CohortDigest: append([]byte(nil), r.CohortDigest...),
		RequestDigest: RequestDigest(r), OwnerSourceRevision: 17, UserSourceGeneration: id,
		IdentityCut: &userv1.SpaceAccountIdentityCut{Owner: "user", Stream: "identity-association", HistoryGeneration: id, ThroughRevision: 5, KeysetRevision: 1, Digest: bytes.Repeat([]byte{3}, 32)}}
	for _, e := range r.Cohort.Entries {
		out.Results = append(out.Results, &userv1.SpaceAccountCaptureResult{Entry: proto.Clone(e).(*userv1.SpaceAccountCaptureEntry),
			Outcome:   userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_ASSOCIATED,
			AccountId: id, AssociationId: id, AssociationRevision: 1, ProfileRevision: 1,
			Origin:            userv1.SpaceAccountCaptureOrigin_SPACE_ACCOUNT_CAPTURE_ORIGIN_EXISTING_CURRENT_ROW,
			AssociationOrigin: userv1.SpaceAccountAssociationOrigin_SPACE_ACCOUNT_ASSOCIATION_ORIGIN_RETAINED_NOW,
			OwnerReceiptId:    id, OwnerReceiptDigest: bytes.Repeat([]byte{1}, 32), CaptureCut: timestamppb.New(time.Unix(100, 0)),
		})
	}
	return out
}

func TestCompleteCohortBeyondFirstHundred(t *testing.T) {
	r := request(t, 301)
	require.NoError(t, ValidateRequest(r))
	out := response(t, r)
	require.NoError(t, ValidateResponse(r, out))
	out.Results = out.Results[:100]
	require.Error(t, ValidateResponse(r, out))
}

func TestRequestRejectsMutatedOrAmbiguousCohorts(t *testing.T) {
	for name, mutate := range map[string]func(*userv1.CaptureSpaceMemberAccountBindingsRequest){
		"wrong protocol":     func(r *userv1.CaptureSpaceMemberAccountBindingsRequest) { r.Protocol = 2 },
		"unknown purpose":    func(r *userv1.CaptureSpaceMemberAccountBindingsRequest) { r.Purpose = 99 },
		"noncanonical space": func(r *userv1.CaptureSpaceMemberAccountBindingsRequest) { r.SpaceId = "{" + id + "}" },
		"missing source":     func(r *userv1.CaptureSpaceMemberAccountBindingsRequest) { r.Cohort.Entries[0].Source = nil },
		"missing publication": func(r *userv1.CaptureSpaceMemberAccountBindingsRequest) {
			r.Cohort.Entries[0].GetOrdinary().PublicationId = ""
		},
		"digest changed": func(r *userv1.CaptureSpaceMemberAccountBindingsRequest) { r.CohortDigest[0] ^= 1 },
		"partial expected association": func(r *userv1.CaptureSpaceMemberAccountBindingsRequest) {
			r.Cohort.Entries[0].ExpectedAssociationId = id
		},
		"unknown field": func(r *userv1.CaptureSpaceMemberAccountBindingsRequest) {
			r.Cohort.Entries[0].ProtoReflect().SetUnknown([]byte{0x78, 0x01})
		},
		"duplicate source different expectations": func(r *userv1.CaptureSpaceMemberAccountBindingsRequest) {
			e := proto.Clone(r.Cohort.Entries[0]).(*userv1.SpaceAccountCaptureEntry)
			e.ExpectedAssociationId, e.ExpectedAssociationRevision = id, 1
			r.Cohort.Entries = []*userv1.SpaceAccountCaptureEntry{r.Cohort.Entries[0], e}
		},
	} {
		t.Run(name, func(t *testing.T) { r := request(t, 2); mutate(r); require.Error(t, ValidateRequest(r)) })
	}
}

func TestResponseNeverAcceptsMissingForeignOrInventedAuthority(t *testing.T) {
	for name, mutate := range map[string]func(*userv1.CaptureSpaceMemberAccountBindingsResponse){
		"extra entry": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.Results = append(r.Results, r.Results[0]) },
		"duplicate entry": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) {
			r.Results[1].Entry = proto.Clone(r.Results[0].Entry).(*userv1.SpaceAccountCaptureEntry)
		},
		"wrong purpose": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.Purpose = 1 },
		"wrong space": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) {
			r.SpaceId = "22222222-2222-4222-8222-222222222222"
		},
		"wrong request":               func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.RequestDigest[0] ^= 1 },
		"zero source revision":        func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.OwnerSourceRevision = 0 },
		"missing identity cut":        func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.IdentityCut = nil },
		"wrong identity owner":        func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.IdentityCut.Owner = "auth" },
		"missing identity generation": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.IdentityCut.HistoryGeneration = "" },
		"future association": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) {
			r.Results[0].AssociationRevision = r.IdentityCut.ThroughRevision + 1
		},
		"identity floor above head": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) {
			r.IdentityCut.RetainedFloor = r.IdentityCut.ThroughRevision + 1
		},
		"missing provenance":  func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.Results[0].AssociationOrigin = 0 },
		"missing association": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.Results[0].AssociationId = "" },
		"unknown outcome":     func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.Results[0].Outcome = 99 },
		"false terminal": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) {
			r.Results[0].Outcome = 2
			r.Results[0].Deleted = true
		},
		"incomplete positive owner": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.Results[0].Outcome = 3 },
		"unknown nested field": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) {
			r.Results[0].CaptureCut.ProtoReflect().SetUnknown([]byte{0x18, 0x01})
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := request(t, 2)
			out := response(t, r)
			mutate(out)
			require.Error(t, ValidateResponse(r, out))
		})
	}
}

func TestRetainedSoftDeletedFrozenAssociationRemainsAssociated(t *testing.T) {
	r := request(t, 1)
	out := response(t, r)
	out.Results[0].Deleted, out.Results[0].Frozen = true, true
	require.NoError(t, ValidateResponse(r, out))
}

func TestResponseRejectsContradictoryOwnerForTwoSourcesOfOneProfile(t *testing.T) {
	r := request(t, 1)
	e := proto.Clone(r.Cohort.Entries[0]).(*userv1.SpaceAccountCaptureEntry)
	e.Source = &userv1.SpaceAccountCaptureEntry_Community{Community: &userv1.SpaceCommunityPublicationIdentity{OwnerGeneration: 1, MemberGeneration: 1, EpisodeId: id}}
	r.Cohort.Entries = append(r.Cohort.Entries, e)
	r.CohortDigest = CohortDigest(r.Cohort)
	require.NoError(t, ValidateRequest(r))
	out := response(t, r)
	require.NoError(t, ValidateResponse(r, out))
	out.Results[1].AccountId = "22222222-2222-4222-8222-222222222222"
	require.Error(t, ValidateResponse(r, out))
}

func TestIncompleteIsExplicitAndTerminalRequiresIndependentReceipt(t *testing.T) {
	r := request(t, 1)
	for _, outcome := range []userv1.SpaceAccountCaptureOutcome{2, 3} {
		t.Run(outcome.String(), func(t *testing.T) {
			out := response(t, r)
			original := out.Results[0]
			out.Results[0] = &userv1.SpaceAccountCaptureResult{Entry: original.Entry, Outcome: outcome,
				OwnerReceiptId: id, OwnerReceiptDigest: bytes.Repeat([]byte{1}, 32), CaptureCut: original.CaptureCut}
			if outcome == 2 {
				// Terminal evidence has an actual owner receipt; unknown history
				// below has no fabricated receipt or capture timestamp.
				out.Results[0].TerminalOperationId, out.Results[0].TerminalRevision = id, 1
				out.Results[0].TerminalReceiptId, out.Results[0].TerminalReceiptDigest = id, bytes.Repeat([]byte{2}, 32)
			} else {
				out.Results[0].OwnerReceiptId = ""
				out.Results[0].OwnerReceiptDigest = nil
				out.Results[0].CaptureCut = nil
			}
			require.NoError(t, ValidateResponse(r, out))
		})
	}
}
