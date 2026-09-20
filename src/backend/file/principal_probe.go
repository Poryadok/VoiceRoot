package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"voice/backend/pkg/principal"

	userv1 "voice.app/voice/user/v1"
)

// runFilePrincipalProbe is an explicit, short-lived deployment probe. It is
// never enabled by normal service configuration. The first call proves that a
// File-owned key can reach User's isolated resolver; the identical second
// bearer proves User's replay admission persists for that request.
func runFilePrincipalProbe(ctx context.Context, runtime *filePrincipalRuntime) error {
	if runtime == nil || runtime.Issuer == nil || runtime.conn == nil {
		return fmt.Errorf("file principal probe runtime unavailable")
	}
	profileID, operationID := uuid.New(), uuid.New()
	req := &userv1.ResolveAccountIDForProfileRequest{ProfileId: profileID.String(), ActorProfileId: profileID.String(), OperationId: operationID.String()}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return err
	}
	requestID := uuid.NewString()
	issuer := runtime.Issuer
	if override := os.Getenv("FILE_PRINCIPAL_PROBE_ISSUER"); override != "" {
		issuer, err = principal.NewIssuer(principal.IssuerConfig{Issuer: override, KeyID: runtime.keyID, PrivateKey: runtime.privateKey})
		if err != nil {
			return err
		}
	}
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "file", RPC: userv1.UserService_ResolveAccountIDForProfile_FullMethodName, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return err
	}
	callCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	client := userv1.NewUserServiceClient(runtime.conn)
	_, err = client.ResolveAccountIDForProfile(callCtx, req, grpc.WaitForReady(false))
	if status.Code(err) != codes.NotFound {
		return fmt.Errorf("file principal first resolver call = %s (%v), want NotFound", status.Code(err), err)
	}
	_, err = client.ResolveAccountIDForProfile(callCtx, req, grpc.WaitForReady(false))
	if status.Code(err) != codes.Unauthenticated {
		return fmt.Errorf("file principal replay call = %s, want Unauthenticated", status.Code(err))
	}
	return nil
}
