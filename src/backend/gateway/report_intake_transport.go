package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	moderationv1 "voice.app/voice/moderation/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

type reportIntakeClientConfig struct{ tls lifecycleClientConfig }

func reportIntakeClientConfigFromEnv() (reportIntakeClientConfig, bool, error) {
	var values [5]string
	enabled := false
	for i, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
		value, present := os.LookupEnv("GATEWAY_MODERATION_INTAKE_" + suffix)
		enabled = enabled || present
		values[i] = value
	}
	cfg := reportIntakeClientConfig{lifecycleClientConfig{values[0], values[1], values[2], values[3], values[4]}}
	if !enabled {
		return cfg, false, nil
	}
	for _, value := range values {
		if value == "" || value != strings.TrimSpace(value) {
			return cfg, true, errors.New("Gateway report intake TLS configuration incomplete")
		}
	}
	_, err := cfg.tls.tlsConfig()
	return cfg, true, err
}
func (c reportIntakeClientConfig) dial() (*grpc.ClientConn, error) {
	transport, err := c.tls.tlsConfig()
	if err != nil {
		return nil, err
	}
	return grpc.NewClient(grpcclient.DialTarget(c.tls.address), grpc.WithTransportCredentials(credentials.NewTLS(transport)), grpc.WithDisableRetry())
}
func (c *grpcClients) configureReportIntake() {
	cfg, enabled, err := reportIntakeClientConfigFromEnv()
	c.reportIntakeRequired = enabled
	c.reportIntakeErr = err
	if !enabled || err != nil {
		return
	}
	conn, err := cfg.dial()
	c.reportIntakeErr = err
	if err != nil {
		return
	}
	c.connections = append(c.connections, conn)
	c.reportIntakeConn = conn
	c.reportIntake = moderationv1.NewModerationServiceClient(conn)
}
func (c *grpcClients) waitReportIntakeReady(ctx context.Context) error {
	if !c.reportIntakeRequired {
		return nil
	}
	if c.reportIntakeErr != nil {
		return c.reportIntakeErr
	}
	if c.reportIntakeConn == nil {
		return errors.New("report intake connection unavailable")
	}
	c.reportIntakeConn.Connect()
	return grpcclient.WaitForReady(ctx, c.reportIntakeConn)
}
func (t *transcoder) reportIntakeContext(r *http.Request, req *moderationv1.CreateReportRequest) (context.Context, error) {
	claims, ok := r.Context().Value(verifiedUserClaimsKey{}).(tokenClaims)
	if !ok || !canonicalLifecycleUUID(claims.UserID) || !canonicalLifecycleUUID(claims.ProfileID) || claims.SessionEpoch <= 0 || !claims.ExpiresAt.After(time.Now()) || (claims.AccountType != "regular" && claims.AccountType != "guest" && claims.AccountType != "sdk-account") {
		return nil, status.Error(codes.Unauthenticated, "verified report actor required")
	}
	if t == nil || t.lifecycleIssuer == nil || t.clients.reportIntake == nil {
		return nil, status.Error(codes.Unavailable, "report intake unavailable")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid report request")
	}
	requestID := uuid.NewString()
	token, err := t.lifecycleIssuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "moderation", RPC: moderationv1.ModerationService_CreateReport_FullMethodName, RequestID: requestID, RequestHash: hash, AccountID: claims.UserID, ProfileID: claims.ProfileID, SessionEpoch: claims.SessionEpoch, AccountType: claims.AccountType, ClientExpiresAt: claims.ExpiresAt})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "report credential unavailable")
	}
	return metadata.NewOutgoingContext(r.Context(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}
