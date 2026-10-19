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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type auditReadClient struct {
	spacev1.SpaceServiceClient
	call func(context.Context, *spacev1.GetAuditLogRequest) (*spacev1.GetAuditLogResponse, error)
}

func (c auditReadClient) GetAuditLog(ctx context.Context, req *spacev1.GetAuditLogRequest, _ ...grpc.CallOption) (*spacev1.GetAuditLogResponse, error) {
	return c.call(ctx, req)
}

func TestSpaceAuditReadStrictQueryAndProtectedBinding(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	space, actor, account, viewer := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, tc := range []struct {
		name, query string
		code        int
	}{
		{"defaults", "", 200},
		{"filters", "actor_profile_id=" + actor + "&action=unknown_action&from=2026-10-18T12:00:00.000000001Z&to=2026-10-18T13:00:00Z&page_size=2&cursor=signed-cursor", 200},
		{"duplicate", "page_size=1&page_size=2", 400},
		{"unknown", "actor_id=" + actor, 400},
		{"empty actor", "actor_profile_id=", 400},
		{"invalid actor", "actor_profile_id=bad", 400},
		{"empty action", "action=", 400},
		{"invalid from", "from=yesterday", 400},
		{"empty from", "from=", 400},
		{"inverted", "from=2026-10-18T13:00:00Z&to=2026-10-18T12:00:00Z", 400},
		{"equal", "from=2026-10-18T12:00:00Z&to=2026-10-18T12:00:00Z", 400},
		{"zero", "page_size=0", 400},
		{"negative", "page_size=-1", 400},
		{"large", "page_size=101", 400},
		{"fraction", "page_size=1.5", 400},
		{"malformed encoding", "action=%zz", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := auditReadClient{call: func(ctx context.Context, req *spacev1.GetAuditLogRequest) (*spacev1.GetAuditLogResponse, error) {
				calls++
				require.Equal(t, space, req.SpaceId)
				if tc.name == "filters" {
					require.NotNil(t, req.ActorProfileId)
					require.Equal(t, actor, req.GetActorProfileId())
					require.Equal(t, "unknown_action", req.GetAction())
					require.Equal(t, int32(2), req.GetPage().GetPageSize())
					require.Equal(t, "signed-cursor", req.GetPage().GetCursor())
					require.Equal(t, int32(1), req.GetFrom().GetNanos())
				} else {
					require.Nil(t, req.ActorProfileId)
					require.Nil(t, req.Action)
					require.Equal(t, int32(50), req.GetPage().GetPageSize())
				}
				md, _ := metadata.FromOutgoingContext(ctx)
				require.Empty(t, md.Get("x-voice-profile-id"))
				require.Empty(t, md.Get("x-voice-user-id"))
				require.Len(t, md.Get("authorization"), 1)
				hash, err := principal.RequestHash(req)
				require.NoError(t, err)
				p, err := principal.VerifyDelegatedUser(context.Background(), strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_GetAuditLog_FullMethodName, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(_ context.Context, a string, e int64) error {
					require.Equal(t, account, a)
					require.Equal(t, int64(7), e)
					return nil
				}})
				require.NoError(t, err)
				require.Equal(t, viewer, p.ProfileID)
				return &spacev1.GetAuditLogResponse{AuditLogList: &spacev1.AuditLogList{}}, nil
			}}
			h := newGateway(gatewayConfig{tokenClaims: map[string]tokenClaims{"user": {UserID: account, ProfileID: viewer, SessionEpoch: 7, ExpiresAt: time.Now().Add(time.Minute)}}, transcoder: &transcoder{clients: grpcClients{spaceLifecycle: client}, lifecycleIssuer: issuer}})
			r := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+space+"/audit-log?"+tc.query, nil)
			r.Header.Set("Authorization", "Bearer user")
			r.Header.Set("X-Voice-Profile-Id", uuid.NewString())
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, tc.code, w.Code, w.Body.String())
			if tc.code == 200 {
				require.Equal(t, 1, calls)
				require.JSONEq(t, `{"audit_log_list":{"entries":[],"next_cursor":""}}`, w.Body.String())
				require.Contains(t, w.Header().Get("Cache-Control"), "no-store")
			} else {
				require.Zero(t, calls)
			}
		})
	}
}

func TestSpaceAuditReadDependencyAndSafeErrors(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	for _, tc := range []struct {
		name               string
		code               codes.Code
		missing, anonymous bool
		want               int
	}{
		{"missing signed transport", codes.OK, true, false, 503},
		{"anonymous", codes.OK, false, true, 401},
		{"hidden", codes.NotFound, false, false, 404},
		{"policy deny", codes.PermissionDenied, false, false, 403},
		{"frozen owner", codes.FailedPrecondition, false, false, 409},
		{"dependency", codes.Unavailable, false, false, 503},
		{"deadline", codes.DeadlineExceeded, false, false, 503},
		{"internal", codes.Internal, false, false, 500},
		{"missing response", codes.OK, false, false, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := auditReadClient{call: func(context.Context, *spacev1.GetAuditLogRequest) (*spacev1.GetAuditLogResponse, error) {
				calls++
				if tc.code == codes.OK {
					return nil, nil
				}
				return nil, status.Error(tc.code, "private SQL and actor details")
			}}
			// A signed transport cannot be replaced by an ordinary Space client.
			clients := grpcClients{space: client, spaceLifecycle: client}
			if tc.missing {
				clients.spaceLifecycle = nil
			}
			h := newGateway(gatewayConfig{tokenClaims: map[string]tokenClaims{"user": {UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, ExpiresAt: time.Now().Add(time.Minute)}}, transcoder: &transcoder{clients: clients, lifecycleIssuer: issuer}})
			r := httptest.NewRequest(http.MethodGet, "/api/v1/spaces/"+uuid.NewString()+"/audit-log?page_size=50", nil)
			if !tc.anonymous {
				r.Header.Set("Authorization", "Bearer user")
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, tc.want, w.Code, w.Body.String())
			require.NotContains(t, w.Body.String(), "private SQL")
			if tc.missing || tc.anonymous {
				require.Zero(t, calls)
			} else {
				require.Equal(t, 1, calls)
			}
		})
	}
}
