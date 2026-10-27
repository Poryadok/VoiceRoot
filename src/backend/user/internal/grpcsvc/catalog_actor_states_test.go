package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	"testing"
	catalogv1 "voice/backend/pkg/pb/voice/catalogactor/v1"
	"voice/backend/pkg/principal"
)

func catalogContext(t *testing.T, issuer string, req *catalogv1.GetCatalogActorStatesRequest) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: issuer, Subject: "service:" + issuer, Audience: "user", RPC: catalogv1.CatalogActorAuthorityService_GetCatalogActorStates_FullMethodName, RequestHash: hash, RequestID: uuid.NewString()})
}

func catalogRequest() *catalogv1.GetCatalogActorStatesRequest {
	return &catalogv1.GetCatalogActorStatesRequest{ProtocolVersion: 1, Actors: []*catalogv1.CatalogActor{{AccountId: uuid.NewString(), ProfileId: uuid.NewString()}}}
}

func TestCatalogActorStatesRejectsInvalidBeforeStore(t *testing.T) {
	for _, name := range []string{"empty", "version", "nil actor", "nil uuid", "malformed", "uppercase", "duplicate", "same profile different owner", "oversize", "unknown request", "unknown actor"} {
		t.Run(name, func(t *testing.T) {
			req := catalogRequest()
			switch name {
			case "empty":
				req.Actors = nil
			case "version":
				req.ProtocolVersion = 2
			case "nil actor":
				req.Actors[0] = nil
			case "nil uuid":
				req.Actors[0].AccountId = uuid.Nil.String()
			case "malformed":
				req.Actors[0].ProfileId = "bad"
			case "uppercase":
				req.Actors[0].ProfileId = strings.ToUpper(req.Actors[0].ProfileId)
			case "duplicate":
				req.Actors = append(req.Actors, req.Actors[0])
			case "same profile different owner":
				req.Actors = append(req.Actors, &catalogv1.CatalogActor{AccountId: uuid.NewString(), ProfileId: req.Actors[0].ProfileId})
			case "oversize":
				for len(req.Actors) < 101 {
					req.Actors = append(req.Actors, catalogRequest().Actors[0])
				}
			case "unknown request":
				req.ProtoReflect().SetUnknown([]byte{0x78, 1})
			case "unknown actor":
				req.Actors[0].ProtoReflect().SetUnknown([]byte{0x78, 1})
			}
			_, err := (&CatalogActorGRPC{}).GetCatalogActorStates(catalogContext(t, "chat", req), req)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestCatalogActorStatesRequiresExactPrincipalAndStore(t *testing.T) {
	req := catalogRequest()
	svc := &CatalogActorGRPC{}
	for _, issuer := range []string{"chat", "file"} {
		_, err := svc.GetCatalogActorStates(catalogContext(t, issuer, req), req)
		require.Equal(t, codes.Unavailable, status.Code(err))
	}
	for _, issuer := range []string{"messaging", "social", "auth", "gateway"} {
		_, err := svc.GetCatalogActorStates(catalogContext(t, issuer, req), req)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
	}
	_, err := svc.GetCatalogActorStates(context.Background(), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	changed := catalogRequest()
	_, err = svc.GetCatalogActorStates(catalogContext(t, "chat", req), changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestCatalogActorStatesRejectsDelegatedAndMisboundAuthority(t *testing.T) {
	req := catalogRequest()
	base, _ := principal.FromContext(catalogContext(t, "chat", req))
	for _, field := range []string{"kind", "subject", "account", "profile", "epoch", "audience", "rpc", "hash", "request id"} {
		t.Run(field, func(t *testing.T) {
			p := base
			switch field {
			case "kind":
				p.Kind = "user"
			case "subject":
				p.Subject = "service:file"
			case "account":
				p.AccountID = uuid.NewString()
			case "profile":
				p.ProfileID = uuid.NewString()
			case "epoch":
				p.SessionEpoch = 1
			case "audience":
				p.Audience = "file"
			case "rpc":
				p.RPC = "/other"
			case "hash":
				p.RequestHash = "other"
			case "request id":
				p.RequestID = "request"
			}
			_, err := (&CatalogActorGRPC{}).GetCatalogActorStates(principal.WithVerified(context.Background(), p), req)
			require.Error(t, err)
			require.NotEqual(t, codes.Unavailable, status.Code(err))
		})
	}
}
