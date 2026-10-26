package authlifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	authv1 "voice.app/voice/auth/v1"
)

// Private wire fixtures test rejection rules, not a live Auth authority.
func captureFixture() *authv1.ObserveSubscriptionAccountResponse {
	now := time.Now()
	scheduled := now.Add(-time.Hour)
	account, cycle := uuid.NewString(), uuid.NewString()
	fact := &authv1.SubscriptionAccountLifecycleFact{AccountId: account, SourceRevision: 1, EventId: cycle, DeletionCycleId: cycle, Kind: authv1.SubscriptionAccountLifecycleKind_SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_DELETE_SCHEDULED, SessionEpoch: 2, ScheduledAt: timestamppb.New(scheduled), PurgeAt: timestamppb.New(scheduled.Add(720 * time.Hour)), EffectiveAt: timestamppb.New(scheduled), CapturedAt: timestamppb.New(scheduled)}
	return &authv1.ObserveSubscriptionAccountResponse{ProtocolVersion: 1, AccountId: account, AccountType: "regular", AccountStatus: "deleted", SessionEpoch: 2, SecurityRevision: 2, DeletedAt: timestamppb.New(scheduled), ObservedAt: timestamppb.New(now), CaptureCoverage: authv1.SubscriptionAccountCaptureCoverage_SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_CAPTURED, Head: fact, Facts: []*authv1.SubscriptionAccountLifecycleFact{fact}}
}

func TestFullSnapshotRejectsForeignLowerPartialAndChangedSourceFacts(t *testing.T) {
	for name, mutate := range map[string]func(*authv1.ObserveSubscriptionAccountResponse){
		"foreign_account":  func(s *authv1.ObserveSubscriptionAccountResponse) { s.AccountId = uuid.NewString() },
		"unknown_protocol": func(s *authv1.ObserveSubscriptionAccountResponse) { s.ProtocolVersion = 2 },
		"missing_head":     func(s *authv1.ObserveSubscriptionAccountResponse) { s.Head = nil },
		"missing_facts":    func(s *authv1.ObserveSubscriptionAccountResponse) { s.Facts = nil },
		"gap":              func(s *authv1.ObserveSubscriptionAccountResponse) { s.Head.SourceRevision = 2 },
		"changed_deadline": func(s *authv1.ObserveSubscriptionAccountResponse) {
			s.Head.PurgeAt = timestamppb.New(s.Head.PurgeAt.AsTime().Add(time.Hour))
		},
		"missing_deleted_state": func(s *authv1.ObserveSubscriptionAccountResponse) { s.DeletedAt = nil },
		"unknown_fields": func(s *authv1.ObserveSubscriptionAccountResponse) {
			s.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
		},
		"future_observation": func(s *authv1.ObserveSubscriptionAccountResponse) {
			s.ObservedAt = timestamppb.New(time.Now().Add(time.Minute))
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := captureFixture()
			account := s.AccountId
			now := time.Now()
			require.True(t, validate(s, account, 1, now, now))
			mutate(s)
			require.False(t, validate(s, account, 1, now, now))
		})
	}
	s := captureFixture()
	now := time.Now()
	require.False(t, validate(s, s.AccountId, 2, now, now))
}

func TestUnrecordedAndLateRestoreRemainInputsWithoutPaidEligibility(t *testing.T) {
	s := captureFixture()
	now := time.Now()
	s.AccountStatus = "active"
	s.DeletedAt = nil
	s.CaptureCoverage = authv1.SubscriptionAccountCaptureCoverage_SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_UNRECORDED
	s.Head = nil
	s.Facts = nil
	require.True(t, validate(s, s.AccountId, 0, now, now))
	require.False(t, validate(s, s.AccountId, 1, now, now))
	s = captureFixture()
	past := now.Add(-721 * time.Hour)
	s.Head.ScheduledAt = timestamppb.New(past)
	s.Head.EffectiveAt = timestamppb.New(past)
	s.Head.CapturedAt = timestamppb.New(past)
	s.Head.PurgeAt = timestamppb.New(past.Add(720 * time.Hour))
	restore := proto.Clone(s.Head).(*authv1.SubscriptionAccountLifecycleFact)
	restore.SourceRevision = 2
	restore.EventId = uuid.NewString()
	restore.Kind = authv1.SubscriptionAccountLifecycleKind_SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_RESTORE_REQUESTED
	// The owner must preserve this truthful late input. The future paid reducer
	// must reject a restore at/after the original fixed deadline independently.
	restore.EffectiveAt = timestamppb.New(restore.PurgeAt.AsTime().Add(time.Second))
	restore.CapturedAt = restore.EffectiveAt
	s.Head = restore
	s.Facts = append(s.Facts, restore)
	s.AccountStatus = "active"
	s.DeletedAt = nil
	require.True(t, validate(s, s.AccountId, 1, now, now))
	restore.DeletionCycleId = uuid.NewString()
	require.False(t, validate(s, s.AccountId, 1, now, now))
}

func TestNoClientOrExpiredCallerCannotCreateObservation(t *testing.T) {
	_, err := (*Client)(nil).Observe(context.Background(), uuid.NewString(), 0)
	require.ErrorIs(t, err, ErrUnavailable)
}
