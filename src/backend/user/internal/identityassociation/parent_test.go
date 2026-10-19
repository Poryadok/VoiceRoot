package identityassociation

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type parentAuthorityFunc func(context.Context, uuid.UUID) (ParentObservation, error)

func (f parentAuthorityFunc) ObserveParent(ctx context.Context, account uuid.UUID) (ParentObservation, error) {
	return f(ctx, account)
}

func parentForTest(account uuid.UUID) ParentObservation {
	return ParentObservation{AccountID: account, OwnerHandle: "opaque-auth-lifetime", LifecycleRevision: 1, State: ParentActive,
		Eligibility: ParentEligible, Disposition: ParentNotIrreversible, ObservedAt: time.Now().Add(-time.Second), ValidUntil: time.Now().Add(time.Minute),
		Cut: ParentCut{Owner: "auth", Stream: "identity", HistoryGeneration: uuid.NewString(), ThroughRevision: 1, Digest: [32]byte{1}, KeysetRevision: 1, RequiredVersions: []uint32{1}, Complete: true}}
}

func TestSealedParentProofRequiresExactCompleteFreshOwnerFact(t *testing.T) {
	account := uuid.New()
	for name, change := range map[string]func(*ParentObservation){
		"valid":                     func(*ParentObservation) {},
		"wrong account":             func(p *ParentObservation) { p.AccountID = uuid.New() },
		"missing handle":            func(p *ParentObservation) { p.OwnerHandle = "" },
		"zero revision":             func(p *ParentObservation) { p.LifecycleRevision = 0 },
		"expired":                   func(p *ParentObservation) { p.ValidUntil = time.Now().Add(-time.Second) },
		"incomplete cut":            func(p *ParentObservation) { p.Cut.Complete = false },
		"missing retained keys":     func(p *ParentObservation) { p.Cut.RequiredVersions = nil },
		"wrong owner":               func(p *ParentObservation) { p.Cut.Owner = "user" },
		"terminal without proof":    func(p *ParentObservation) { p.State = ParentTerminal; p.Disposition = ParentIrreversible },
		"recoverable without cycle": func(p *ParentObservation) { p.State = ParentRecoverable },
	} {
		t.Run(name, func(t *testing.T) {
			v, err := NewParentVerifier(parentAuthorityFunc(func(ctx context.Context, got uuid.UUID) (ParentObservation, error) {
				require.Equal(t, account, got)
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				require.LessOrEqual(t, time.Until(deadline), 2*time.Second)
				p := parentForTest(account)
				change(&p)
				return p, nil
			}))
			require.NoError(t, err)
			proof, err := v.Verify(context.Background(), account)
			if name == "valid" {
				require.NoError(t, err)
				require.True(t, proof.validForBirth(time.Now()))
			} else {
				require.Error(t, err)
			}
		})
	}
	require.False(t, (VerifiedParentProof{}).validForBirth(time.Now()))
}
