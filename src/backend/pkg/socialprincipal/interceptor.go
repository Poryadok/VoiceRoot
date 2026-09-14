// Package socialprincipal secures the two Social privacy decision boundaries.
package socialprincipal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/principal"
)

func Method(target string) string {
	switch target {
	case "user":
		return "/voice.user.v1.UserService/GetPrivacySettings"
	case "space":
		return "/voice.space.v1.SpaceService/AreCoMembers"
	}
	return ""
}

type Verifier struct {
	Target  string
	Issuers map[string]bool
	Resolve principal.KeyResolver
	Replay  principal.ReplayGuard
}

func (v *Verifier) Verify(ctx context.Context, token, method, requestID, hash string) (principal.Principal, error) {
	if v == nil || v.Resolve == nil || v.Replay == nil {
		return principal.Principal{}, Unavailable(errors.New("principal verifier unavailable"))
	}
	// Untrusted issuer selects an explicitly configured verifier, never an URL.
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(token) > 16384 {
		return principal.Principal{}, errors.New("invalid credential")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return principal.Principal{}, err
	}
	var claim struct {
		Issuer string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &claim); err != nil {
		return principal.Principal{}, err
	}
	if !v.Issuers[claim.Issuer] {
		return principal.Principal{}, errors.New("untrusted issuer")
	}
	verified, err := principal.VerifyService(ctx, token, principal.VerifyConfig{ExpectedIssuer: claim.Issuer, ExpectedAudience: v.Target, ExpectedRPC: method, ExpectedRequestID: requestID, ExpectedRequestHash: hash, KeyResolver: v.Resolve, ReplayGuard: v.Replay})
	if err != nil {
		return principal.Principal{}, err
	}
	if verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return principal.Principal{}, errors.New("service credential contains user authority")
	}
	return verified, nil
}

type PrincipalVerifier interface {
	Verify(context.Context, string, string, string, string) (principal.Principal, error)
}

func StrictUnaryInterceptor(verifier PrincipalVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		message, ok := req.(proto.Message)
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		}
		if verifier == nil {
			return nil, status.Error(codes.Unavailable, "principal verifier unavailable")
		}
		verified, err := verifier.Verify(ctx, transport.BearerToken, info.FullMethod, transport.RequestID, hash)
		if err != nil {
			var unavailable unavailableError
			if errors.As(err, &unavailable) {
				return nil, status.Error(codes.Unavailable, "principal verifier unavailable")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid principal")
		}
		if Method(verified.Audience) == "" || info.FullMethod != Method(verified.Audience) || verified.Kind != "service" || verified.Issuer != "social" || verified.Subject != "service:social" {
			return nil, status.Error(codes.PermissionDenied, "Social principal required")
		}
		return handler(principal.WithVerified(ctx, verified), req)
	}
}

// RequireSocial is the domain defense for a protected listener. Its context
// value is server-owned and its request binding is checked again before storage.
func RequireSocial(ctx context.Context, target string, req proto.Message) error {
	verified, ok := principal.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "verified Social principal required")
	}
	if verified.Kind != "service" || verified.Issuer != "social" || verified.Subject != "service:social" {
		return status.Error(codes.PermissionDenied, "Social principal required")
	}
	hash, err := principal.RequestHash(req)
	if err != nil || verified.Audience != target || verified.RPC != Method(target) || verified.RequestHash != hash || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return status.Error(codes.Unauthenticated, "invalid principal binding")
	}
	return nil
}

// CheckDomain preserves separately migrated ordinary callers while enforcing
// verified Social authority whenever a principal or raw Social marker is present.
func CheckDomain(ctx context.Context, target string, req proto.Message) (bool, error) {
	if HasRawSocial(ctx) {
		return false, status.Error(codes.Unauthenticated, "raw Social identity is forbidden")
	}
	if _, ok := principal.FromContext(ctx); ok {
		return true, RequireSocial(ctx, target, req)
	}
	return false, nil
}

func HasRawSocial(ctx context.Context) bool {
	md, _ := metadata.FromIncomingContext(ctx)
	for _, value := range md.Get("x-voice-internal-caller") {
		if strings.EqualFold(strings.TrimSpace(value), "social") {
			return true
		}
	}
	return false
}

func OrdinaryUnaryInterceptor(target string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == Method(target) && HasRawSocial(ctx) {
			return nil, status.Error(codes.Unauthenticated, "raw Social identity is forbidden")
		}
		return handler(ctx, req)
	}
}

type unavailableError struct{ err error }

func (e unavailableError) Error() string { return e.err.Error() }
func (e unavailableError) Unwrap() error { return e.err }
func Unavailable(err error) error        { return unavailableError{err} }
