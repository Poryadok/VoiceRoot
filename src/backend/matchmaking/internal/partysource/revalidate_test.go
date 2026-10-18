package partysource

import (
	"context"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"voice/backend/matchmaking/internal/store"
)

type readerFunc func(context.Context, Subject) (Snapshot, error)

func (f readerFunc) Snapshot(ctx context.Context, s Subject) (Snapshot, error) { return f(ctx, s) }
func captured() store.SourceParty {
	id := uuid.MustParse("10000000-0000-4000-8000-000000000001")
	return store.SourceParty{Party: store.Party{ID: uuid.New(), LeaderProfileID: id, MemberProfileIDs: []uuid.UUID{id}}, SearchOperationID: uuid.New(), SearchRequestHash: [32]byte{1}, InitiatorAccountID: uuid.New(), InitiatorSessionEpoch: 7, ProtocolVersion: 1, SourceKind: "SOLO", MemberManifestHash: sha256.Sum256(id[:])}
}
func TestRevalidateUsesPersistedInitiatorAndDistinguishesChangedFromUnknown(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Snapshot)
		err    error
		want   Validation
	}{
		{name: "current", want: ValidationCurrent},
		{name: "changed room", mutate: func(s *Snapshot) { id := uuid.New(); s.Kind = "VOICE_ROSTER"; s.RoomID = &id; s.RosterVersion = 1 }, want: ValidationChanged},
		{name: "revoked", err: ErrIneligible, want: ValidationChanged},
		{name: "outage", err: errors.New("transport unavailable"), want: ValidationUnknown},
		{name: "malformed", mutate: func(s *Snapshot) { s.ProtocolVersion = 0 }, want: ValidationUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := captured()
			calls := 0
			r := &Revalidator{Snapshots: readerFunc(func(_ context.Context, s Subject) (Snapshot, error) {
				calls++
				require.Equal(t, Subject{p.SearchOperationID, p.InitiatorAccountID, p.LeaderProfileID, p.InitiatorSessionEpoch}, s)
				snap := Snapshot{ProtocolVersion: 1, Kind: "SOLO", MemberProfileIDs: append([]uuid.UUID(nil), p.MemberProfileIDs...)}
				if tc.mutate != nil {
					tc.mutate(&snap)
				}
				return snap, tc.err
			})}
			got, err := r.RevalidateCapturedParty(context.Background(), p)
			require.Equal(t, tc.want, got)
			require.Equal(t, 1, calls)
			if tc.want == ValidationUnknown {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
func TestInvalidPersistedCaptureNeverContactsAuthority(t *testing.T) {
	for _, mutate := range []func(*store.SourceParty){func(p *store.SourceParty) { p.InitiatorSessionEpoch = 0 }, func(p *store.SourceParty) { p.MemberManifestHash = [32]byte{2} }, func(p *store.SourceParty) { p.SearchOperationID = uuid.Nil }, func(p *store.SourceParty) { p.SourceKind = "" }} {
		p := captured()
		mutate(&p)
		r := &Revalidator{Snapshots: readerFunc(func(context.Context, Subject) (Snapshot, error) {
			t.Fatal("invalid capture reached authority")
			return Snapshot{}, nil
		})}
		v, err := r.RevalidateCapturedParty(context.Background(), p)
		require.Equal(t, ValidationUnknown, v)
		require.Error(t, err)
	}
}
