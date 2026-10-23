package store

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/role/internal/memberauthority"
	"voice/backend/role/permissions"
)

type delayedMemberQueryKey struct{}
type expireAfterMemberQuery struct {
	space, profile uuid.UUID
	query          string
	fresh          atomic.Bool
	calls          atomic.Int64
}

func (d *expireAfterMemberQuery) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, delayedMemberQueryKey{}, strings.TrimSpace(data.SQL))
}
func (d *expireAfterMemberQuery) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	sql, _ := ctx.Value(delayedMemberQueryKey{}).(string)
	if !strings.HasPrefix(sql, d.query) {
		return
	}
	o, ok := memberauthority.FromContext(ctx, d.space.String(), d.profile.String())
	if !ok {
		return
	}
	d.calls.Add(1)
	d.fresh.Store(o.Available(time.Now()))
	for o.Available(time.Now()) {
		time.Sleep(time.Millisecond)
	}
}

func TestQualifiedDecisionsRejectExpiryAfterLastSQL(t *testing.T) {
	ctx, st := memberAdmissionStore(t)
	space, profile, owner, room := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	binding := ensureMemberBinding(space, profile, 1)
	receipt, err := st.PrepareDefaultMemberRole(ctx, binding)
	require.NoError(t, err)
	for _, stage := range []string{"override", "voice_epoch", "mask_commit", "voice_commit"} {
		t.Run(stage, func(t *testing.T) {
			observed := qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
			baseline, err := st.ResolveVoiceRoomGrants(observed, space, room, profile)
			require.NoError(t, err)
			require.True(t, baseline.CanJoin)
			query := "SELECT allow, deny FROM voice_room_overrides"
			if stage == "voice_epoch" {
				query = "SELECT policy_epoch\nFROM role_voice_policy_epochs"
			}
			if strings.HasSuffix(stage, "_commit") {
				query = "commit"
			}
			delay := &expireAfterMemberQuery{space: space, profile: profile, query: query}
			config := st.Pool.Config().Copy()
			config.ConnConfig.Tracer = delay
			pool, err := pgxpool.NewWithConfig(ctx, config)
			require.NoError(t, err)
			defer pool.Close()
			traced := &RoleStore{Pool: pool}
			observed = qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
			if stage == "override" || stage == "mask_commit" {
				mask, err := traced.GetQualifiedEffectiveMask(observed, space, profile, nil, &room)
				require.EqualValues(t, 1, delay.calls.Load())
				require.True(t, delay.fresh.Load(), "observation was fresh when the final override query completed")
				require.ErrorIs(t, err, ErrScopeUnavailable)
				join, e := permissions.MaskFor(permissions.VoiceJoin)
				require.NoError(t, e)
				require.Zero(t, mask&join)
			} else {
				decision, err := traced.ResolveVoiceRoomGrants(observed, space, room, profile)
				require.EqualValues(t, 1, delay.calls.Load())
				require.True(t, delay.fresh.Load(), "observation was fresh when the final epoch query completed")
				require.ErrorIs(t, err, ErrScopeUnavailable)
				require.False(t, decision.CanJoin)
			}
			require.EqualValues(t, 1, delay.calls.Load())
			require.True(t, delay.fresh.Load(), "only expiry AFTER the final real query is the discriminator")
		})
	}
}
