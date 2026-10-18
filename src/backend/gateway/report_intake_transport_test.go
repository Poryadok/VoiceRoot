package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	moderationv1 "voice.app/voice/moderation/v1"
	"voice/backend/pkg/principal"
)

type reportIntakeCapture struct {
	moderationv1.ModerationServiceClient
	call func(context.Context, *moderationv1.CreateReportRequest) (*moderationv1.CreateReportResponse, error)
}

func (c reportIntakeCapture) CreateReport(ctx context.Context, req *moderationv1.CreateReportRequest, _ ...grpc.CallOption) (*moderationv1.CreateReportResponse, error) {
	return c.call(ctx, req)
}

func TestReportIntakeRouteUsesVerifiedSignedNormalizedRequest(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	claims := tokenClaims{UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: "guest", ExpiresAt: time.Now().Add(time.Minute)}
	receipt := uuid.NewString()
	calls := 0
	capture := reportIntakeCapture{call: func(ctx context.Context, req *moderationv1.CreateReportRequest) (*moderationv1.CreateReportResponse, error) {
		calls++
		require.Equal(t, "cheating", req.Category)
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Len(t, md, 2)
		require.Len(t, md.Get("authorization"), 1)
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		p, err := principal.VerifyDelegatedUser(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "moderation", ExpectedRPC: moderationv1.ModerationService_CreateReport_FullMethodName, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, SessionEpochChecker: func(_ context.Context, id string, epoch int64) error {
			require.Equal(t, claims.UserID, id)
			require.Equal(t, claims.SessionEpoch, epoch)
			return nil
		}})
		require.NoError(t, err)
		require.Equal(t, claims.ProfileID, p.ProfileID)
		require.Equal(t, claims.AccountType, p.AccountType)
		return &moderationv1.CreateReportResponse{Report: &moderationv1.Report{Id: receipt}}, nil
	}}
	tr := &transcoder{lifecycleIssuer: issuer, clients: grpcClients{reportIntake: capture}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/moderation/reports", strings.NewReader(`{"targetType":"user","targetId":"`+uuid.NewString()+`","category":"mm_toxic"}`))
	request.Header.Set("X-Voice-User-Id", uuid.NewString())
	request.Header.Set("X-Voice-Profile-Id", uuid.NewString())
	request = request.WithContext(context.WithValue(request.Context(), verifiedUserClaimsKey{}, claims))
	response := httptest.NewRecorder()
	require.True(t, tr.serveNamespace(response, request, "moderation"))
	require.Equal(t, http.StatusAccepted, response.Code)
	require.JSONEq(t, `{"report":{"id":"`+receipt+`"}}`, response.Body.String())
	require.Equal(t, 1, calls)
}

func TestReportIntakeRouteHasNoRawOrPlainFallback(t *testing.T) {
	calls := 0
	plain := reportIntakeCapture{call: func(context.Context, *moderationv1.CreateReportRequest) (*moderationv1.CreateReportResponse, error) {
		calls++
		return nil, nil
	}}
	for _, trusted := range []bool{false, true} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/moderation/reports", strings.NewReader(`{"targetType":"story","targetId":"`+uuid.NewString()+`","category":"spam"}`))
		request.Header.Set("X-Voice-User-Id", uuid.NewString())
		request.Header.Set("X-Voice-Profile-Id", uuid.NewString())
		if trusted {
			request = request.WithContext(context.WithValue(request.Context(), verifiedUserClaimsKey{}, tokenClaims{UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 1, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}))
		}
		response := httptest.NewRecorder()
		tr := &transcoder{clients: grpcClients{moderation: plain}}
		require.True(t, tr.serveNamespace(response, request, "moderation"))
		if trusted {
			require.Equal(t, http.StatusServiceUnavailable, response.Code)
		} else {
			require.Equal(t, http.StatusUnauthorized, response.Code)
		}
	}
	require.Zero(t, calls)
}
