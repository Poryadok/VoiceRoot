package store

import (
	"bytes"
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	spacev1 "voice.app/voice/space/v1"
)

func legacyBasisGolden(kind, origin uint8, method string) JoinLegacyDecisionBasis {
	v1 := joinBasisGolden()
	c := v1.Context
	b := JoinLegacyDecisionBasis{Context: JoinLegacyDecisionContext{Kind: kind, OperationOrigin: origin, SpaceID: c.SpaceID, AttemptID: c.AttemptID, AttemptBindingSHA256: c.AttemptBindingSHA256, SubmissionSHA256: c.SubmissionSHA256, ActorAccountID: c.ActorAccountID, ActorProfileID: c.ActorProfileID, OperationID: c.OperationID, RequestMethod: method, RequestSHA256: c.RequestSHA256}, ApplicantObservationSHA256: v1.ApplicantObservationSHA256}
	if kind != JoinLegacyPending {
		b.Context.ReadyEvidenceSHA256 = bytes.Repeat([]byte{3}, 32)
	}
	if kind == JoinApproveLegacyReady {
		b.Context.PendingBasisSHA256 = bytes.Repeat([]byte{7}, 32)
		b.ReviewerObservationSHA256 = bytes.Repeat([]byte{8}, 32)
	}
	return b
}

func TestJoinLegacyDecisionBasis_IndependentBinaryGolden(t *testing.T) {
	// Independently framed using Python UUID.bytes and struct.pack('>I'); v1
	// codec, unchanged35 evidence and opaque FIRST_PUBLISH framing stay frozen.
	for _, tc := range []struct {
		kind, origin           uint8
		method, context, basis string
	}{
		{4, 1, spacev1.SpaceService_JoinSpace_FullMethodName, "47f76e25df3ecd03fac4fb9e69b614fe32e97f9647176e0323bef960457ec9e3", "1412e4d3a2294b29a536ad1e02fb2c4bece1073f609fa1081d808b42a6732a15"},
		{4, 2, spacev1.SpaceService_JoinByInvite_FullMethodName, "ce4d6cc76d0d1501d922641a952c7f89cc99bdea43678bdbd8734583c69b54bf", "44e8b637049f04241ad336b30eb8dce875345db0dcf184ad65167f004973ba83"},
		{5, 1, spacev1.SpaceService_JoinSpace_FullMethodName, "6088cf7937360999b4f1019a8a2065a03e56f58eef5f08b4c911a3bc86228bb8", "14ca4e94c991228a3a91f9493eeb3469c2a5a61ad503aaf03b12ba12d70f4eae"},
		{6, 1, JoinOperationApprove, "d63bc67cb7fd8ba61699f66e184eaf42305a790e92783cb42dd6ebbc580a3ec2", "9c991d58e5c618fc1ab5cc0dc9b7e4e39464c573858c0217d484d320ee95fb37"},
	} {
		b := legacyBasisGolden(tc.kind, tc.origin, tc.method)
		contextSHA, err := b.Context.SHA256()
		require.NoError(t, err)
		require.Equal(t, tc.context, hex.EncodeToString(contextSHA[:]))
		basisSHA, err := b.SHA256()
		require.NoError(t, err)
		require.Equal(t, tc.basis, hex.EncodeToString(basisSHA[:]))
	}
}

func TestJoinLegacyDecisionBasis_RejectsCrossFamilyAndAmbiguousOrigin(t *testing.T) {
	for name, change := range map[string]func(*JoinLegacyDecisionBasis){
		"v1 kind":                  func(b *JoinLegacyDecisionBasis) { b.Context.Kind = 2 },
		"unknown origin":           func(b *JoinLegacyDecisionBasis) { b.Context.OperationOrigin = 0 },
		"unknown kind":             func(b *JoinLegacyDecisionBasis) { b.Context.Kind = 7 },
		"fake Submit":              func(b *JoinLegacyDecisionBasis) { b.Context.RequestMethod = JoinOperationSubmit },
		"wrong method":             func(b *JoinLegacyDecisionBasis) { b.Context.RequestMethod = JoinOperationApprove },
		"nil actor":                func(b *JoinLegacyDecisionBasis) { b.Context.ActorAccountID = uuid.Nil },
		"short digest":             func(b *JoinLegacyDecisionBasis) { b.Context.SubmissionSHA256 = []byte{1} },
		"zero digest":              func(b *JoinLegacyDecisionBasis) { b.Context.RequestSHA256 = make([]byte, 32) },
		"ready no evidence":        func(b *JoinLegacyDecisionBasis) { b.Context.ReadyEvidenceSHA256 = nil },
		"empty parent not absence": func(b *JoinLegacyDecisionBasis) { b.Context.PendingBasisSHA256 = []byte{} },
		"automatic parent":         func(b *JoinLegacyDecisionBasis) { b.Context.PendingBasisSHA256 = bytes.Repeat([]byte{7}, 32) },
		"automatic reviewer":       func(b *JoinLegacyDecisionBasis) { b.ReviewerObservationSHA256 = bytes.Repeat([]byte{8}, 32) },
		"pending evidence":         func(b *JoinLegacyDecisionBasis) { b.Context.Kind = JoinLegacyPending },
		"missing applicant":        func(b *JoinLegacyDecisionBasis) { b.ApplicantObservationSHA256 = nil },
	} {
		t.Run(name, func(t *testing.T) {
			b := legacyBasisGolden(5, 1, spacev1.SpaceService_JoinSpace_FullMethodName)
			change(&b)
			_, err := b.SHA256()
			require.ErrorIs(t, err, ErrJoinDecisionInvalid)
		})
	}
	b := legacyBasisGolden(6, 2, JoinOperationApprove)
	_, err := b.SHA256()
	require.ErrorIs(t, err, ErrJoinDecisionInvalid, "approval cannot invent a server no-key origin")
	b = legacyBasisGolden(6, 1, JoinOperationApprove)
	b.ReviewerObservationSHA256 = nil
	_, err = b.SHA256()
	require.ErrorIs(t, err, ErrJoinDecisionInvalid)
	for _, kind := range []uint8{4, 5, 6} {
		v1 := joinBasisGolden()
		v1.Context.Kind = kind
		_, err = v1.SHA256()
		require.ErrorIs(t, err, ErrJoinDecisionInvalid, "v1 predicates must not widen")
	}
}
