package partysource

import (
	"context"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/partyauthority"
)

type readerFunc func(context.Context, Subject) (Snapshot, error)

func (f readerFunc) Snapshot(ctx context.Context, s Subject) (Snapshot, error) { return f(ctx, s) }
func captured() store.SourceParty {
	id := uuid.MustParse("10000000-0000-4000-8000-000000000001")
	p := store.SourceParty{Party: store.Party{ID: uuid.New(), LeaderProfileID: id, MemberProfileIDs: []uuid.UUID{id}}, SearchOperationID: uuid.New(), SearchRequestHash: [32]byte{1}, InitiatorAccountID: uuid.New(), InitiatorSessionEpoch: 7, ProtocolVersion: 1, SourceKind: "SOLO", MemberManifestHash: sha256.Sum256(id[:]), MemberAuthorityVersion: 1, SourceAuthorityHash: [32]byte{2}}
	p.MemberAuthorities = []partyauthority.Member{{ProfileID: id, AccountID: p.InitiatorAccountID, SessionEpoch: 7}}
	_, p.MemberAuthorityHash, _ = partyauthority.Encode(p.MemberAuthorities)
	return p
}
func TestRevalidateUsesPersistedInitiatorAndDistinguishesChangedFromUnknown(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Snapshot)
		err    error
		want   Validation
	}{
		{name: "current", want: ValidationCurrent},
		{name: "same profiles changed native cut", mutate: func(s *Snapshot) { s.SourceAuthorityHash[0]++ }, want: ValidationChanged},
		{name: "private authority unsupported", mutate: func(s *Snapshot) { s.MemberAuthorityVersion = 2 }, want: ValidationUnknown},
		{name: "solo native epoch mismatch", mutate: func(s *Snapshot) { s.MemberAuthorities[0].SessionEpoch++ }, want: ValidationUnknown},
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
				snap := Snapshot{ProtocolVersion: 1, Kind: "SOLO", MemberProfileIDs: append([]uuid.UUID(nil), p.MemberProfileIDs...), MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: append([]partyauthority.Member(nil), p.MemberAuthorities...), SourceAuthorityHash: p.SourceAuthorityHash}
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
	for _, mutate := range []func(*store.SourceParty){func(p *store.SourceParty) { p.InitiatorSessionEpoch = 0 }, func(p *store.SourceParty) { p.MemberManifestHash = [32]byte{2} }, func(p *store.SourceParty) { p.SearchOperationID = uuid.Nil }, func(p *store.SourceParty) { p.SourceKind = "" }, func(p *store.SourceParty) { p.MemberAuthorityVersion = 0 }, func(p *store.SourceParty) { p.MemberAuthorities = nil }, func(p *store.SourceParty) { p.MemberAuthorityHash[0]++ }, func(p *store.SourceParty) { p.SourceAuthorityHash = [32]byte{} }} {
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
