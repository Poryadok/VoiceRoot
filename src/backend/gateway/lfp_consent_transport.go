package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/principal"
)

func storyConsentClientConfigFromEnv() (lifecycleClientConfig, bool, error) {
	c := lifecycleClientConfig{os.Getenv("GATEWAY_STORY_CONSENT_GRPC_ADDR"), os.Getenv("GATEWAY_STORY_CONSENT_TLS_CA_FILE"), os.Getenv("GATEWAY_STORY_CONSENT_TLS_SERVER_NAME"), os.Getenv("GATEWAY_STORY_CONSENT_CLIENT_CERT_FILE"), os.Getenv("GATEWAY_STORY_CONSENT_CLIENT_KEY_FILE")}
	active := false
	for _, value := range []string{c.address, c.ca, c.serverName, c.cert, c.key} {
		if value != "" {
			active = true
		}
	}
	if !active {
		return c, false, nil
	}
	for _, value := range []string{c.address, c.ca, c.serverName, c.cert, c.key} {
		if value == "" || value != strings.TrimSpace(value) {
			return c, true, errors.New("Gateway Story consent TLS configuration is incomplete")
		}
	}
	if _, e := c.tlsConfig(); e != nil {
		return c, true, e
	}
	return c, true, nil
}

func isStoryConsentRequest(r *http.Request) bool {
	if r == nil || !strings.HasPrefix(r.URL.Path, "/api/v1/stories/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/stories/"), "/")
	return len(parts) == 2 && (parts[1] == "lfp-consent-offer" || parts[1] == "lfp-consent-response")
}

func (t *transcoder) storyConsentContext(r *http.Request, request proto.Message, method string) (context.Context, error) {
	claims, ok := r.Context().Value(verifiedUserClaimsKey{}).(tokenClaims)
	if !ok || !canonicalLifecycleUUID(claims.UserID) || !canonicalLifecycleUUID(claims.ProfileID) || claims.SessionEpoch <= 0 || !claims.ExpiresAt.After(time.Now()) {
		return nil, status.Error(codes.Unauthenticated, "verified LFP actor required")
	}
	// The compatibility AccountType/header may already default an absent signed
	// claim to regular. Only the exact authenticated claim can mint this path.
	if claims.AuthenticatedAccountType != "guest" && claims.AuthenticatedAccountType != "regular" {
		return nil, status.Error(codes.PermissionDenied, "explicit human LFP actor required")
	}
	if t == nil || t.lifecycleIssuer == nil {
		return nil, status.Error(codes.Unavailable, "LFP consent transport unavailable")
	}
	if !storyConsentRequest(request, method) {
		return nil, status.Error(codes.InvalidArgument, "invalid LFP action")
	}
	hash, e := principal.RequestHash(request)
	if e != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid LFP action")
	}
	requestID := uuid.NewString()
	token, e := t.lifecycleIssuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "story", RPC: method, RequestID: requestID, RequestHash: hash, AccountID: claims.UserID, ProfileID: claims.ProfileID, SessionEpoch: claims.SessionEpoch, AccountType: claims.AuthenticatedAccountType, ClientExpiresAt: claims.ExpiresAt})
	if e != nil {
		return nil, status.Error(codes.Unavailable, "LFP consent transport unavailable")
	}
	return metadata.NewOutgoingContext(r.Context(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}

func storyConsentRequest(r proto.Message, method string) bool {
	if r == nil || !r.ProtoReflect().IsValid() || len(r.ProtoReflect().GetUnknown()) != 0 {
		return false
	}
	switch method {
	case storyv1.LfpConsentActionService_PublishLfpConsentOffer_FullMethodName:
		_, ok := r.(*storyv1.PublishLfpConsentOfferRequest)
		return ok
	case storyv1.LfpConsentActionService_ObserveLfpConsentOffer_FullMethodName:
		_, ok := r.(*storyv1.ObserveLfpConsentOfferRequest)
		return ok
	case storyv1.LfpConsentActionService_RespondWithLfpConsent_FullMethodName:
		request, ok := r.(*storyv1.RespondWithLfpConsentRequest)
		return ok && request.Offer != nil && len(request.Offer.ProtoReflect().GetUnknown()) == 0
	default:
		return false
	}
}
