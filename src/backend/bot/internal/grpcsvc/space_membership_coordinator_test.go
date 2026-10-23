package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

// An absent Pool is an intentional forbidden-effect tripwire, not a fixture.
// Unverified input must be refused before touching either DB or a remote client.
func TestInstallationRejectsUnverifiedAdministratorBeforeEffects(t *testing.T) {
	for _, method := range []string{"install", "uninstall"} {
		for _, contextKind := range []string{"raw_headers", "injected_principal"} {
			t.Run(method+"/"+contextKind, func(t *testing.T) {
				ctx := context.Background()
				account, profile := uuid.NewString(), uuid.NewString()
				if contextKind == "raw_headers" {
					ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-user-id", account, "x-voice-profile-id", profile))
				} else {
					ctx = principal.WithVerified(ctx, principal.Principal{Kind: "delegated_user", Issuer: "gateway", Audience: "bot", Subject: account, AccountID: account, ProfileID: profile, SessionEpoch: 1})
				}
				defer func() {
					if recover() != nil {
						t.Fatal("unverified administrator reached the forbidden storage effect")
					}
				}()
				svc := NewBotGRPC(&store.BotStore{}, nil)
				var err error
				if method == "install" {
					_, err = svc.InstallBotInSpace(ctx, &botv1.InstallBotInSpaceRequest{BotId: uuid.NewString(), SpaceId: uuid.NewString()})
				} else {
					_, err = svc.UninstallBotFromSpace(ctx, &botv1.UninstallBotFromSpaceRequest{BotId: uuid.NewString(), SpaceId: uuid.NewString()})
				}
				if status.Code(err) != codes.Unauthenticated {
					t.Fatalf("want unauthenticated before effects, got %v", err)
				}
			})
		}
	}
}
