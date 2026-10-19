package store

import (
	"bytes"
	"encoding/hex"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func joinBasisGolden() JoinDecisionBasis {
	return JoinDecisionBasis{Context: JoinDecisionContext{
		Kind:                 JoinDecisionSubmitReady,
		SpaceID:              uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		AttemptID:            uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		AttemptBindingSHA256: bytes.Repeat([]byte{1}, 32), SubmissionSHA256: bytes.Repeat([]byte{2}, 32), ReadyEvidenceSHA256: bytes.Repeat([]byte{3}, 32),
		ActorAccountID: uuid.MustParse("33333333-3333-4333-8333-333333333333"), ActorProfileID: uuid.MustParse("44444444-4444-4444-8444-444444444444"), OperationID: uuid.MustParse("55555555-5555-4555-8555-555555555555"),
		RequestMethod: JoinOperationSubmit, RequestSHA256: bytes.Repeat([]byte{4}, 32),
	}, ApplicantObservationSHA256: bytes.Repeat([]byte{5}, 32)}
}

func TestJoinDecisionBasis_IndependentBinaryGolden(t *testing.T) {
	// Expected values were independently framed with Python UUID.bytes and
	// struct.pack('>I'/' >Q'), not calculated by the implementation under test.
	basis := joinBasisGolden()
	contextHash, err := basis.Context.SHA256()
	require.NoError(t, err)
	require.Equal(t, "329aadcd0f643b8aec2ee2de8826dd7a80bfaf57c39cb0ca67af43d6e6e391f6", hex.EncodeToString(contextHash[:]))
	basisHash, err := basis.SHA256()
	require.NoError(t, err)
	require.Equal(t, "d25dce7c2b5870b600703f7526f103e32ccf2cc971f5e112d0321c975bcd28ae", hex.EncodeToString(basisHash[:]))
	publicationHash, err := JoinFirstPublishContextSHA256(basisHash[:], uuid.MustParse("66666666-6666-4666-8666-666666666666"), 7, bytes.Repeat([]byte{6}, 32))
	require.NoError(t, err)
	require.Equal(t, "6794253006569b8fc4afd89cf96e2e19c8103a387d7809331823e5a105f4ffb9", hex.EncodeToString(publicationHash[:]))
}

func TestJoinDecisionBasis_RejectsAmbiguousKindsAndOptionals(t *testing.T) {
	cases := map[string]func(*JoinDecisionBasis){
		"unknown kind":                 func(b *JoinDecisionBasis) { b.Context.Kind = 4 },
		"wrong method":                 func(b *JoinDecisionBasis) { b.Context.RequestMethod = JoinOperationApprove },
		"nil account":                  func(b *JoinDecisionBasis) { b.Context.ActorAccountID = uuid.Nil },
		"short digest":                 func(b *JoinDecisionBasis) { b.Context.SubmissionSHA256 = []byte{2} },
		"zero digest":                  func(b *JoinDecisionBasis) { b.Context.RequestSHA256 = make([]byte, 32) },
		"ready missing evidence":       func(b *JoinDecisionBasis) { b.Context.ReadyEvidenceSHA256 = nil },
		"empty optional is not absent": func(b *JoinDecisionBasis) { b.Context.SubmitBasisSHA256 = []byte{} },
		"auto has parent":              func(b *JoinDecisionBasis) { b.Context.SubmitBasisSHA256 = bytes.Repeat([]byte{7}, 32) },
		"auto has reviewer":            func(b *JoinDecisionBasis) { b.ReviewerObservationSHA256 = bytes.Repeat([]byte{8}, 32) },
		"pending has ready evidence":   func(b *JoinDecisionBasis) { b.Context.Kind = JoinDecisionSubmitPending },
		"missing applicant":            func(b *JoinDecisionBasis) { b.ApplicantObservationSHA256 = nil },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			b := joinBasisGolden()
			change(&b)
			_, err := b.SHA256()
			require.ErrorIs(t, err, ErrJoinDecisionInvalid)
		})
	}
	b := joinBasisGolden()
	b.Context.Kind = JoinDecisionSubmitPending
	b.Context.ReadyEvidenceSHA256 = nil
	_, err := b.SHA256()
	require.NoError(t, err)
	b = joinBasisGolden()
	b.Context.Kind = JoinDecisionApproveReady
	b.Context.RequestMethod = JoinOperationApprove
	b.Context.SubmitBasisSHA256 = bytes.Repeat([]byte{7}, 32)
	b.ReviewerObservationSHA256 = bytes.Repeat([]byte{8}, 32)
	_, err = b.SHA256()
	require.NoError(t, err)
	b.ReviewerObservationSHA256 = nil
	_, err = b.SHA256()
	require.ErrorIs(t, err, ErrJoinDecisionInvalid)
	_, err = JoinFirstPublishContextSHA256(bytes.Repeat([]byte{1}, 32), uuid.New(), uint64(math.MaxInt64)+1, bytes.Repeat([]byte{2}, 32))
	require.ErrorIs(t, err, ErrJoinDecisionInvalid)
}
