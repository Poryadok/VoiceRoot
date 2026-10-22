package usercapture

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spaceaccountcapture"
)

const testID = "11111111-1111-4111-8111-111111111111"

type callFunc func(context.Context, *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error)

func (f callFunc) CaptureSpaceMemberAccountBindings(ctx context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest, _ ...grpc.CallOption) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
	return f(ctx, r)
}
func testRequest() *userv1.CaptureSpaceMemberAccountBindingsRequest {
	r := &userv1.CaptureSpaceMemberAccountBindingsRequest{Protocol: 1, OperationId: testID, SpaceId: testID, Purpose: userv1.SpaceAccountCapturePurpose_SPACE_ACCOUNT_CAPTURE_PURPOSE_BAN_PREFLIGHT, Cohort: &userv1.SpaceAccountCaptureCohort{Entries: []*userv1.SpaceAccountCaptureEntry{{ProfileId: testID, Source: &userv1.SpaceAccountCaptureEntry_Ordinary{Ordinary: &userv1.SpaceOrdinaryPublicationIdentity{PublicationId: testID}}}}}}
	r.CohortDigest = spaceaccountcapture.CohortDigest(r.Cohort)
	return r
}

// Structural wire fixtures are not owner/provider receipts or database proof.
func testResponse(r *userv1.CaptureSpaceMemberAccountBindingsRequest, outcome userv1.SpaceAccountCaptureOutcome) *userv1.CaptureSpaceMemberAccountBindingsResponse {
	result := &userv1.SpaceAccountCaptureResult{Entry: proto.Clone(r.Cohort.Entries[0]).(*userv1.SpaceAccountCaptureEntry), Outcome: outcome}
	if outcome != 3 {
		result.OwnerReceiptId = testID
		result.OwnerReceiptDigest = bytes.Repeat([]byte{1}, 32)
		result.CaptureCut = timestamppb.Now()
	}
	if outcome == 1 {
		result.AccountId = testID
		result.AssociationId = testID
		result.AssociationRevision = 1
		result.ProfileRevision = 1
		result.Origin = 1
		result.AssociationOrigin = 2
	}
	if outcome == 2 {
		result.TerminalOperationId = testID
		result.TerminalRevision = 1
		result.TerminalReceiptId = testID
		result.TerminalReceiptDigest = bytes.Repeat([]byte{2}, 32)
	}
	return &userv1.CaptureSpaceMemberAccountBindingsResponse{Protocol: r.Protocol, OperationId: r.OperationId, Purpose: r.Purpose, SpaceId: r.SpaceId, CohortDigest: append([]byte(nil), r.CohortDigest...), RequestDigest: spaceaccountcapture.RequestDigest(r), OwnerSourceRevision: 17, UserSourceGeneration: testID, IdentityCut: &userv1.SpaceAccountIdentityCut{Owner: "user", Stream: "identity-association", HistoryGeneration: testID, ThroughRevision: 5, KeysetRevision: 1, Digest: bytes.Repeat([]byte{3}, 32)}, Results: []*userv1.SpaceAccountCaptureResult{result}}
}
func testIssuer(t *testing.T, name string) (*principal.Issuer, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: name, KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	return issuer, &key.PublicKey
}
func verifyCall(ctx context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest, key *rsa.PublicKey) (principal.Principal, error) {
	return verifyCallWithReplay(ctx, r, key, nil)
}
func verifyCallWithReplay(ctx context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest, key *rsa.PublicKey, replay principal.ReplayGuard) (principal.Principal, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	auth := md.Get("authorization")
	ids := md.Get("x-request-id")
	if len(auth) != 1 || len(ids) != 1 || !strings.HasPrefix(auth[0], "Bearer ") {
		return principal.Principal{}, errors.New("missing exact metadata")
	}
	hash, err := principal.RequestHash(r)
	if err != nil {
		return principal.Principal{}, err
	}
	return principal.VerifyService(ctx, strings.TrimPrefix(auth[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "user", ExpectedRPC: userv1.UserService_CaptureSpaceMemberAccountBindings_FullMethodName, ExpectedRequestID: ids[0], ExpectedRequestHash: hash, ReplayGuard: replay, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return key, nil }})
}

func TestClientBindsPureServiceCredentialAndPreservesOutcomes(t *testing.T) {
	issuer, key := testIssuer(t, "space")
	var prior principal.Principal
	for _, outcome := range []userv1.SpaceAccountCaptureOutcome{1, 2, 3} {
		r := testRequest()
		original := proto.Clone(r)
		calls := 0
		var ownerReply *userv1.CaptureSpaceMemberAccountBindingsResponse
		c := &Client{issuer: issuer, rpc: callFunc(func(ctx context.Context, wire *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
			calls++
			require.True(t, proto.Equal(original, wire))
			require.NotSame(t, r, wire)
			p, err := verifyCall(ctx, wire, key)
			require.NoError(t, err)
			require.Equal(t, "service:space", p.Subject)
			require.Empty(t, p.AccountID)
			require.Empty(t, p.ProfileID)
			require.Zero(t, p.SessionEpoch)
			require.NotEqual(t, prior.RequestID, p.RequestID)
			require.NotEqual(t, prior.JWTID, p.JWTID)
			require.NotEqual(t, r.OperationId, p.RequestID)
			prior = p
			deadline, ok := ctx.Deadline()
			require.True(t, ok)
			require.Positive(t, time.Until(deadline))
			require.LessOrEqual(t, time.Until(deadline), 2*time.Second)
			ownerReply = testResponse(wire, outcome)
			require.NoError(t, spaceaccountcapture.ValidateResponse(wire, ownerReply))
			return ownerReply, nil
		})}
		ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer inherited-human", "x-request-id", "inherited"))
		out, err := c.CaptureSpaceMemberAccountBindings(ctx, r)
		require.NoError(t, err)
		require.Equal(t, 1, calls)
		require.Equal(t, outcome, out.Results[0].Outcome)
		require.True(t, proto.Equal(original, r))
		require.NotSame(t, ownerReply, out)
	}
}

func TestClientRejectsInvalidInputWithoutDispatch(t *testing.T) {
	issuer, _ := testIssuer(t, "space")
	calls := 0
	c := &Client{issuer: issuer, rpc: callFunc(func(context.Context, *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		calls++
		return nil, nil
	})}
	for name, mutate := range map[string]func(*userv1.CaptureSpaceMemberAccountBindingsRequest){"unknown purpose": func(r *userv1.CaptureSpaceMemberAccountBindingsRequest) { r.Purpose = 99 }, "changed digest": func(r *userv1.CaptureSpaceMemberAccountBindingsRequest) { r.CohortDigest[0] ^= 1 }, "missing operation": func(r *userv1.CaptureSpaceMemberAccountBindingsRequest) { r.OperationId = "" }, "unknown field": func(r *userv1.CaptureSpaceMemberAccountBindingsRequest) { r.ProtoReflect().SetUnknown([]byte{0x78, 1}) }} {
		t.Run(name, func(t *testing.T) {
			r := testRequest()
			mutate(r)
			out, err := c.CaptureSpaceMemberAccountBindings(context.Background(), r)
			require.ErrorIs(t, err, ErrUnavailable)
			require.Nil(t, out)
		})
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, cancelled} {
		_, err := c.CaptureSpaceMemberAccountBindings(ctx, testRequest())
		require.ErrorIs(t, err, ErrUnavailable)
	}
	for _, client := range []*Client{nil, {}, {issuer: issuer}, {rpc: c.rpc}} {
		_, err := client.CaptureSpaceMemberAccountBindings(context.Background(), testRequest())
		require.ErrorIs(t, err, ErrUnavailable)
	}
	_, err := c.CaptureSpaceMemberAccountBindings(context.Background(), nil)
	require.ErrorIs(t, err, ErrUnavailable)
	require.Zero(t, calls)
}

func TestClientRejectsForeignIncompleteMalformedAndMutatedReplies(t *testing.T) {
	issuer, _ := testIssuer(t, "space")
	for name, mutate := range map[string]func(*userv1.CaptureSpaceMemberAccountBindingsResponse){"foreign operation": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) {
		r.OperationId = "22222222-2222-4222-8222-222222222222"
	}, "foreign space": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) {
		r.SpaceId = "22222222-2222-4222-8222-222222222222"
	}, "foreign purpose": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.Purpose = 3 }, "foreign request digest": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.RequestDigest[0] ^= 1 }, "omitted cohort result": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.Results = nil }, "invented incomplete receipt": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) { r.Results[0].OwnerReceiptId = testID }, "unknown response": func(r *userv1.CaptureSpaceMemberAccountBindingsResponse) {
		r.ProtoReflect().SetUnknown([]byte{0x78, 1})
	}} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			c := &Client{issuer: issuer, rpc: callFunc(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
				calls++
				out := testResponse(r, 3)
				mutate(out)
				return out, nil
			})}
			out, err := c.CaptureSpaceMemberAccountBindings(context.Background(), testRequest())
			require.ErrorIs(t, err, ErrUnavailable)
			require.Nil(t, out)
			require.Equal(t, 1, calls)
		})
	}
	t.Run("callee mutates request", func(t *testing.T) {
		r := testRequest()
		original := proto.Clone(r)
		c := &Client{issuer: issuer, rpc: callFunc(func(_ context.Context, wire *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
			out := testResponse(wire, 3)
			wire.Purpose = 3
			return out, nil
		})}
		_, err := c.CaptureSpaceMemberAccountBindings(context.Background(), r)
		require.ErrorIs(t, err, ErrUnavailable)
		require.True(t, proto.Equal(original, r))
	})
	t.Run("transport error never retries", func(t *testing.T) {
		calls := 0
		c := &Client{issuer: issuer, rpc: callFunc(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
			calls++
			return testResponse(r, 3), errors.New("owner failed")
		})}
		_, err := c.CaptureSpaceMemberAccountBindings(context.Background(), testRequest())
		require.ErrorIs(t, err, ErrUnavailable)
		require.Equal(t, 1, calls)
	})
}

func TestClientKeepsEarlierDeadlineAndRejectsLateSuccess(t *testing.T) {
	issuer, _ := testIssuer(t, "space")
	for _, ignoreCancellation := range []bool{false, true} {
		t.Run(map[bool]string{false: "cooperative", true: "late success"}[ignoreCancellation], func(t *testing.T) {
			parent, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			deadline, _ := parent.Deadline()
			calls := 0
			c := &Client{issuer: issuer, rpc: callFunc(func(ctx context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
				calls++
				actual, ok := ctx.Deadline()
				require.True(t, ok)
				require.Equal(t, deadline, actual)
				<-ctx.Done()
				if ignoreCancellation {
					return testResponse(r, 3), nil
				}
				return nil, ctx.Err()
			})}
			out, err := c.CaptureSpaceMemberAccountBindings(parent, testRequest())
			require.ErrorIs(t, err, ErrUnavailable)
			require.Nil(t, out)
			require.Equal(t, 1, calls)
		})
	}
}
