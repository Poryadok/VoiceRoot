package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// This resolver is a UNIT fixture only. Production resolution must use both
// protected owner works, the sealed partition and fresh cut/resource proof.
type projectionFixture struct{ step LegacyProjectionStep }

func (f *projectionFixture) ResolveProjectionStep(context.Context, LegacyProjectionStepRef) (LegacyProjectionStep, error) {
	return f.step, nil
}
func stepRef(t *testing.T, s LegacyProjectionStep) LegacyProjectionStepRef {
	t.Helper()
	h, e := LegacyProjectionStepHash(s)
	require.NoError(t, e)
	return LegacyProjectionStepRef{s.ID, h}
}
func fixtureStep(t *testing.T, prefix, runID string) LegacyProjectionStep {
	t.Helper()
	return LegacyProjectionStep{ID: uuid.New(), CaptureID: uuid.New(), Ordinal: 1, ScopePrefix: prefix, EvidencePrefix: "maintenance-" + uuid.NewString() + ":", ExpectedRunID: runID, BindingBytes: []byte(`{"fixture":"not installed owner authority"}`)}
}
func rawRunID(t *testing.T, info string) string {
	t.Helper()
	for _, line := range strings.Split(info, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && k == "run_id" {
			return v
		}
	}
	t.Fatal("no Redis run identity")
	return ""
}

func TestLegacyProjectionSerialMixedOwnerStepsAndLostAck(t *testing.T) {
	_, c, prefix := newLegacyRedisFixture(t)
	ctx := context.Background()
	info, e := c.Info(ctx, "server").Result()
	require.NoError(t, e)
	before := `{"room_id":"room","states":{"native":{"profile_id":"native"},"legacy":{"profile_id":"legacy"}}}`
	middle := `{"room_id":"room","states":{"legacy":{"profile_id":"legacy"}}}`
	after := `{"room_id":"room","states":{}}`
	require.NoError(t, c.Set(ctx, prefix+"call:room", before, 0).Err())
	require.NoError(t, c.SAdd(ctx, prefix+"active_space_voice_rooms:space", "room", "foreign").Err())
	s := fixtureStep(t, prefix, rawRunID(t, info))
	s.Transitions = []LegacyKeyTransition{{Key: prefix + "call:room", Before: LegacyKeyImage{Kind: "string", Value: before}, After: LegacyKeyImage{Kind: "string", Value: middle}}}
	f := &projectionFixture{s}
	executor := NewLegacyProjectionExecutor(c, f)
	receipt, e := executor.Apply(ctx, stepRef(t, s))
	require.NoError(t, e)
	require.Equal(t, middle, c.Get(ctx, prefix+"call:room").Val())
	replayed, e := executor.Apply(ctx, stepRef(t, s))
	require.NoError(t, e)
	require.Equal(t, receipt, replayed)
	second := s
	second.ID = uuid.New()
	second.Ordinal = 2
	second.PreviousReceipt = receipt
	second.Transitions = []LegacyKeyTransition{
		{Key: prefix + "call:room", Before: LegacyKeyImage{Kind: "string", Value: middle}, After: LegacyKeyImage{Kind: "string", Value: after}},
		{Key: prefix + "active_space_voice_rooms:space", Before: LegacyKeyImage{Kind: "set", Members: []string{"foreign", "room"}}, After: LegacyKeyImage{Kind: "set", Members: []string{"foreign"}}},
	}
	f.step = second
	receipt2, e := executor.Apply(ctx, stepRef(t, second))
	require.NoError(t, e)
	require.NotEqual(t, receipt, receipt2)
	require.Equal(t, after, c.Get(ctx, prefix+"call:room").Val())
	require.Equal(t, []string{"foreign"}, c.SMembers(ctx, prefix+"active_space_voice_rooms:space").Val())
	// Captured input/intent remains immutable; the next owner uses S1 postimage.
	require.Equal(t, before, s.Transitions[0].Before.Value)
}

func TestLegacyProjectionRejectsNewValuesOrderAndDenyMutation(t *testing.T) {
	_, c, prefix := newLegacyRedisFixture(t)
	ctx := context.Background()
	info, e := c.Info(ctx, "server").Result()
	require.NoError(t, e)
	s := fixtureStep(t, prefix, rawRunID(t, info))
	s.Transitions = []LegacyKeyTransition{{Key: prefix + "session:profile", Before: LegacyKeyImage{Kind: "string", Value: "old"}, After: LegacyKeyImage{Kind: "none"}}}
	f := &projectionFixture{s}
	executor := NewLegacyProjectionExecutor(c, f)
	require.NoError(t, c.Set(ctx, prefix+"session:profile", "newer", 0).Err())
	_, e = executor.Apply(ctx, stepRef(t, s))
	require.Error(t, e)
	require.Equal(t, "newer", c.Get(ctx, prefix+"session:profile").Val())
	s.Transitions[0].Key = prefix + "space_lifecycle:space"
	f.step = s
	_, e = executor.Apply(ctx, stepRef(t, s))
	require.Error(t, e)
	s.Transitions[0].Key = prefix + "session:profile"
	s.Ordinal = 2
	f.step = s
	_, e = executor.Apply(ctx, stepRef(t, s))
	require.Error(t, e)
	changed := stepRef(t, s)
	changed.Hash[0] ^= 1
	_, e = executor.Apply(ctx, changed)
	require.Error(t, e)
}

func TestLegacyProjectionReceiptContainsBoundObservation(t *testing.T) {
	_, c, prefix := newLegacyRedisFixture(t)
	ctx := context.Background()
	info, e := c.Info(ctx, "server").Result()
	require.NoError(t, e)
	s := fixtureStep(t, prefix, rawRunID(t, info))
	s.Transitions = []LegacyKeyTransition{{Key: prefix + "session:profile", Before: LegacyKeyImage{Kind: "none"}, After: LegacyKeyImage{Kind: "none"}}}
	f := &projectionFixture{s}
	receipt, e := NewLegacyProjectionExecutor(c, f).Apply(ctx, stepRef(t, s))
	require.NoError(t, e)
	require.Contains(t, string(receipt), s.ID.String())
	require.Contains(t, string(receipt), s.ExpectedRunID)
	_, e = json.Marshal(s)
	require.NoError(t, e)
}

func TestLegacyProjectionMalformedReceiptCannotAdvanceCursor(t *testing.T) {
	_, c, prefix := newLegacyRedisFixture(t)
	ctx := context.Background()
	info, e := c.Info(ctx, "server").Result()
	require.NoError(t, e)
	s := fixtureStep(t, prefix, rawRunID(t, info))
	s.Transitions = []LegacyKeyTransition{{Key: prefix + "session:p", Before: LegacyKeyImage{Kind: "none"}, After: LegacyKeyImage{Kind: "none"}}}
	f := &projectionFixture{s}
	executor := NewLegacyProjectionExecutor(c, f)
	receipt, e := executor.Apply(ctx, stepRef(t, s))
	require.NoError(t, e)
	lines := strings.Split(string(receipt), "\n")
	require.Len(t, lines, 8)
	forged := strings.Join(lines[:6], "\n") + "\nnot-a-time\nAPPLIED"
	base := s.EvidencePrefix + s.CaptureID.String() + ":"
	require.NoError(t, c.Set(ctx, base+"step:"+s.ID.String(), forged, 0).Err())
	require.NoError(t, c.Del(ctx, base+"cursor").Err())
	_, e = executor.Apply(ctx, stepRef(t, s))
	require.Error(t, e)
	require.EqualValues(t, 0, c.Exists(ctx, base+"cursor").Val(), "invalid evidence must not change the resource cursor")
}

func TestLegacyProjectionCommandFailureHasNoSuccessAndRecoversSameIntent(t *testing.T) {
	_, c, prefix := newLegacyRedisFixture(t)
	ctx := context.Background()
	info, e := c.Info(ctx, "server").Result()
	require.NoError(t, e)
	s := fixtureStep(t, prefix, rawRunID(t, info))
	s.Transitions = []LegacyKeyTransition{
		{Key: prefix + "active_space_voice_rooms:a", Before: LegacyKeyImage{Kind: "set", Members: []string{"old-a"}}, After: LegacyKeyImage{Kind: "none"}},
		{Key: prefix + "session:b", Before: LegacyKeyImage{Kind: "string", Value: "old-b"}, After: LegacyKeyImage{Kind: "none"}},
	}
	require.NoError(t, c.SAdd(ctx, prefix+"active_space_voice_rooms:a", "old-a").Err())
	require.NoError(t, c.Set(ctx, prefix+"session:b", "old-b", 0).Err())
	user := "legacy-partial-" + uuid.NewString()
	password := "isolated-fixture-only"
	require.NoError(t, c.Do(ctx, "ACL", "SETUSER", user, "on", ">"+password, "+@all", "-del", "~*").Err())
	t.Cleanup(func() { _ = c.Do(context.Background(), "ACL", "DELUSER", user).Err() })
	restricted := redis.NewClient(&redis.Options{Addr: c.Options().Addr, Username: user, Password: password})
	t.Cleanup(func() { _ = restricted.Close() })
	f := &projectionFixture{s}
	_, e = NewLegacyProjectionExecutor(restricted, f).Apply(ctx, stepRef(t, s))
	require.Error(t, e)
	base := s.EvidencePrefix + s.CaptureID.String() + ":"
	require.EqualValues(t, 0, c.Exists(ctx, base+"step:"+s.ID.String(), base+"cursor").Val())
	require.EqualValues(t, 1, c.Exists(ctx, base+"pending").Val())
	require.EqualValues(t, 0, c.Exists(ctx, prefix+"active_space_voice_rooms:a").Val())
	require.Equal(t, "old-b", c.Get(ctx, prefix+"session:b").Val())
	receipt, e := NewLegacyProjectionExecutor(c, f).Apply(ctx, stepRef(t, s))
	require.NoError(t, e)
	require.NotEmpty(t, receipt)
	require.EqualValues(t, 0, c.Exists(ctx, prefix+"active_space_voice_rooms:a", prefix+"session:b", base+"pending").Val())
}

func TestLegacyProjectionExpectedExpiryAndUnexpectedAbsence(t *testing.T) {
	_, c, prefix := newLegacyRedisFixture(t)
	ctx := context.Background()
	info, e := c.Info(ctx, "server").Result()
	require.NoError(t, e)
	require.NoError(t, c.Set(ctx, prefix+"call:expired", "old", time.Millisecond*10).Err())
	deadline, e := c.PExpireTime(ctx, prefix+"call:expired").Result()
	require.NoError(t, e)
	s := fixtureStep(t, prefix, rawRunID(t, info))
	s.Transitions = []LegacyKeyTransition{{Key: prefix + "call:expired", Before: LegacyKeyImage{Kind: "string", Value: "old", ExpiresAtUnixMilli: deadline.Milliseconds()}, After: LegacyKeyImage{Kind: "none"}, AllowDeadlineExpiry: true}}
	time.Sleep(time.Millisecond * 20)
	f := &projectionFixture{s}
	receipt, e := NewLegacyProjectionExecutor(c, f).Apply(ctx, stepRef(t, s))
	require.NoError(t, e)
	require.True(t, strings.HasSuffix(string(receipt), "EXPIRED"))
	s = fixtureStep(t, prefix, rawRunID(t, info))
	s.Transitions = []LegacyKeyTransition{{Key: prefix + "call:unexpected", Before: LegacyKeyImage{Kind: "string", Value: "old"}, After: LegacyKeyImage{Kind: "none"}}}
	f.step = s
	_, e = NewLegacyProjectionExecutor(c, f).Apply(ctx, stepRef(t, s))
	require.Error(t, e)
}
