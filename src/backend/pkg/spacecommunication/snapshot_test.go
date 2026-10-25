package spacecommunication

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSnapshotRejectsDefaultAndInconsistentAuthority(t *testing.T) {
	space, profile := uuid.NewString(), uuid.NewString()
	now := time.Now()
	clear := Snapshot{Version: 1, SpaceID: space, ProfileID: profile, Epoch: 1, EvaluatedAt: timestamppb.New(now), State: Clear}
	_, err := clear.Validate(space, profile)
	require.NoError(t, err)
	for name, mutate := range map[string]func(*Snapshot){
		"version zero":                func(s *Snapshot) { s.Version = 0 },
		"future version":              func(s *Snapshot) { s.Version = 2 },
		"wrong space":                 func(s *Snapshot) { s.SpaceID = uuid.NewString() },
		"wrong profile":               func(s *Snapshot) { s.ProfileID = uuid.NewString() },
		"zero epoch":                  func(s *Snapshot) { s.Epoch = 0 },
		"missing clock":               func(s *Snapshot) { s.EvaluatedAt = nil },
		"invalid clock":               func(s *Snapshot) { s.EvaluatedAt = &timestamppb.Timestamp{Seconds: 1 << 62} },
		"zero clock":                  func(s *Snapshot) { s.EvaluatedAt = timestamppb.New(time.Time{}) },
		"unspecified":                 func(s *Snapshot) { s.State = 0 },
		"unknown enum":                func(s *Snapshot) { s.State = 3 },
		"clear with until":            func(s *Snapshot) { s.Until = timestamppb.New(now.Add(time.Hour)) },
		"timeout missing until":       func(s *Snapshot) { s.State = TimedOut },
		"timeout equal":               func(s *Snapshot) { s.State = TimedOut; s.Until = s.EvaluatedAt },
		"timeout expired at snapshot": func(s *Snapshot) { s.State = TimedOut; s.Until = timestamppb.New(now.Add(-time.Second)) },
	} {
		t.Run(name, func(t *testing.T) {
			snapshot := clear
			mutate(&snapshot)
			_, err := snapshot.Validate(space, profile)
			require.Error(t, err)
		})
	}
}

func TestSnapshotNeverGrantsByConsumerLocalExpiry(t *testing.T) {
	space, profile := uuid.NewString(), uuid.NewString()
	past := time.Now().Add(-time.Hour)
	snapshot := Snapshot{Version: 1, SpaceID: space, ProfileID: profile, Epoch: 1, EvaluatedAt: timestamppb.New(past), State: TimedOut, Until: timestamppb.New(past.Add(time.Minute))}
	denied, err := snapshot.Validate(space, profile)
	require.NoError(t, err)
	require.True(t, denied, "only a fresh Space decision may clear a timeout")
}
