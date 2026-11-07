package authprofileauthority

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"
	userv1 "voice.app/voice/user/v1"
)

func matchRequest() *userv1.MatchAuthProfileReferencesRequest {
	return &userv1.MatchAuthProfileReferencesRequest{ProtocolVersion: 1, RequestId: uuid.NewString(), InventoryId: uuid.NewString(), InventoryRevision: 1, AccountId: uuid.NewString(), OwnerHandle: uuid.NewString(), DeletionCycleId: uuid.NewString(), ProfileIds: []string{uuid.NewString()}}
}

func TestAuthProfileResponseRejectsChangedContextCoverageAndExpiredSource(t *testing.T) {
	r := matchRequest()
	now := time.Now()
	out := &userv1.MatchAuthProfileReferencesResponse{ProtocolVersion: 1, RequestId: r.RequestId, InventoryId: r.InventoryId, InventoryRevision: 1, AccountId: r.AccountId, OwnerHandle: r.OwnerHandle, DeletionCycleId: r.DeletionCycleId, RequestDigest: Digest(r), Results: []*userv1.AuthProfileReferenceMatch{{ProfileId: r.ProfileIds[0], Outcome: userv1.AuthProfileMatchOutcome_AUTH_PROFILE_MATCH_OUTCOME_INCOMPLETE}}, SourceCut: &userv1.AuthProfileSourceCut{Source: &userv1.AuthProfileSourcePin{UserSourceGeneration: uuid.NewString(), HistoryGeneration: uuid.NewString(), KeysetRevision: 1}, UserAuthorityRevision: 1, ThroughRevision: 1, HeadDigest: make([]byte, 32), ObservedAt: timestamppb.New(now)}}
	require.NoError(t, ValidateMatchResponse(r, out, now))
	out.SourceCut.UnresolvedErasureIssued = 4
	out.SourceCut.VerifiedErasureCovered = 4
	require.NoError(t, ValidateMatchResponse(r, out, now))
	for _, change := range []func(*userv1.MatchAuthProfileReferencesResponse){
		func(o *userv1.MatchAuthProfileReferencesResponse) { o.SourceCut.UnresolvedErasureIssued++ },
		func(o *userv1.MatchAuthProfileReferencesResponse) { o.DeletionCycleId = uuid.NewString() },
		func(o *userv1.MatchAuthProfileReferencesResponse) { o.Results[0].ProfileId = uuid.NewString() },
		func(o *userv1.MatchAuthProfileReferencesResponse) { o.Results[0].AssociationId = uuid.NewString() },
		func(o *userv1.MatchAuthProfileReferencesResponse) { o.AllRequestedResolved = true },
		func(o *userv1.MatchAuthProfileReferencesResponse) {
			o.SourceCut.ObservedAt = timestamppb.New(now.Add(-2 * time.Second))
		},
		func(o *userv1.MatchAuthProfileReferencesResponse) {
			o.SourceCut.ObservedAt = timestamppb.New(now.Add(2 * time.Second))
		},
		func(o *userv1.MatchAuthProfileReferencesResponse) {
			o.SourceCut.Source.ProtoReflect().SetUnknown([]byte{0xf8, 0x07, 0x01})
		},
	} {
		bad := proto.Clone(out).(*userv1.MatchAuthProfileReferencesResponse)
		change(bad)
		require.Error(t, ValidateMatchResponse(r, bad, now))
	}
}
func TestAuthProfileRequestRejectsUnboundOrUnboundedInventory(t *testing.T) {
	good := matchRequest()
	require.NoError(t, ValidateMatchRequest(good))
	for _, change := range []func(*userv1.MatchAuthProfileReferencesRequest){
		func(r *userv1.MatchAuthProfileReferencesRequest) { r.InventoryRevision = 0 },
		func(r *userv1.MatchAuthProfileReferencesRequest) { r.OwnerHandle = "" },
		func(r *userv1.MatchAuthProfileReferencesRequest) { r.ProfileIds = nil },
		func(r *userv1.MatchAuthProfileReferencesRequest) {
			r.ProfileIds = append(r.ProfileIds, r.ProfileIds[0])
		},
		func(r *userv1.MatchAuthProfileReferencesRequest) { r.ExpectedSource = &userv1.AuthProfileSourcePin{} },
		func(r *userv1.MatchAuthProfileReferencesRequest) {
			r.ProtoReflect().SetUnknown([]byte{0xf8, 0x07, 0x01})
		},
	} {
		r := proto.Clone(good).(*userv1.MatchAuthProfileReferencesRequest)
		change(r)
		require.Error(t, ValidateMatchRequest(r))
	}
}
