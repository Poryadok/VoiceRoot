package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	authv1 "voice.app/voice/auth/v1"
)

type deletionProofClient struct {
	authv1.AuthServiceClient
	request *authv1.IssueSpaceDeletionProofRequest
	md      metadata.MD
	err     error
}

func (c *deletionProofClient) IssueSpaceDeletionProof(ctx context.Context, req *authv1.IssueSpaceDeletionProofRequest, _ ...grpc.CallOption) (*authv1.IssueSpaceDeletionProofResponse, error) {
	c.request = req
	c.md, _ = metadata.FromOutgoingContext(ctx)
	return &authv1.IssueSpaceDeletionProofResponse{Proof: "opaque-proof", ExpiresAt: timestamppb.New(time.Unix(1790920000, 0))}, c.err
}

func TestPublicSpaceDeletionProof(t *testing.T) {
	valid := `{"space_id":"10000000-0000-0000-0000-000000000001","confirmation_name":" Exact Name ","operation_id":"10000000-0000-0000-0000-000000000002","password":"factor-value"}`
	for _, tc := range []struct {
		name, body, token, accountType string
		code                           int
		upstreamErr                    error
	}{
		{"valid", valid, "user", "regular", 200, nil},
		{"camel", strings.NewReplacer("space_id", "spaceId", "confirmation_name", "confirmationName", "operation_id", "operationId").Replace(valid), "user", "regular", 200, nil},
		{"unknown", strings.TrimSuffix(valid, "}") + `,"actor_id":"forged"}`, "user", "regular", 400, nil},
		{"alias", strings.TrimSuffix(valid, "}") + `,"operationId":"10000000-0000-0000-0000-000000000002"}`, "user", "regular", 400, nil},
		{"bad uuid", strings.Replace(valid, "10000000-0000-0000-0000-000000000001", "bad", 1), "user", "regular", 400, nil},
		{"two factors", strings.TrimSuffix(valid, "}") + `,"totp_code":"123456","backup_code":"unused"}`, "user", "regular", 400, nil},
		{"empty", `{}`, "user", "regular", 400, nil},
		{"unauthenticated", valid, "", "regular", 401, nil},
		{"sdk", valid, "user", "sdk-account", 403, nil},
		{"safe factor denial", valid, "user", "regular", 403, status.Error(codes.PermissionDenied, "password factor-value is wrong")},
		{"auth outage", valid, "user", "regular", 503, status.Error(codes.Unavailable, "internal address and factors")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &deletionProofClient{err: tc.upstreamErr}
			h := newGateway(gatewayConfig{tokenClaims: map[string]tokenClaims{"user": {UserID: "10000000-0000-0000-0000-000000000003", ProfileID: "10000000-0000-0000-0000-000000000004", SessionEpoch: 7, AccountType: tc.accountType}}, transcoder: &transcoder{clients: grpcClients{auth: client}}})
			r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/space-deletion-proof", strings.NewReader(tc.body))
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			r.Header.Set("X-Voice-User-Id", "forged")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, tc.code, w.Code, w.Body.String())
			require.Contains(t, w.Header().Get("Cache-Control"), "no-store")
			require.NotContains(t, w.Body.String(), "factor-value")
			if tc.code == 200 {
				require.Equal(t, " Exact Name ", client.request.ConfirmationName)
				require.Equal(t, []string{"Bearer user"}, client.md.Get("authorization"))
				require.NotContains(t, client.md.Get("x-voice-user-id"), "forged")
				require.Contains(t, w.Body.String(), `"expires_at"`)
			} else if tc.upstreamErr == nil {
				require.Nil(t, client.request)
			}
		})
	}
}
