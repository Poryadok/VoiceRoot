package grpcsvc

import (
	"context"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/bot/internal/installprincipal"
)

// SpaceMembershipCoordinator is an owned producer. Implementations must commit
// immutable intent before transport, and never hold SQL locks across an RPC.
type SpaceMembershipCoordinator interface {
	Install(context.Context, installprincipal.Administrator, *botv1.InstallBotInSpaceRequest) (*botv1.InstallBotInSpaceResponse, error)
	Uninstall(context.Context, installprincipal.Administrator, *botv1.UninstallBotFromSpaceRequest) (*botv1.UninstallBotFromSpaceResponse, error)
}
