package grpcsvc

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	botv1 "voice.app/voice/bot/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/installprincipal"
	"voice/backend/bot/internal/store"
)

type MembershipJournal interface {
	PrepareInstall(context.Context, uuid.UUID, installprincipal.Administrator, *botv1.InstallBotInSpaceRequest) (store.MembershipRecord, error)
	BeginCancel(context.Context, uuid.UUID, installprincipal.Administrator, uuid.UUID, uuid.UUID) (store.MembershipRecord, error)
	ClaimMembershipAttempt(context.Context, uuid.UUID, store.MembershipRecord, time.Duration) (store.MembershipRecord, error)
	RecordAddAcknowledged(context.Context, uuid.UUID, store.MembershipRecord) (store.MembershipRecord, error)
	CompleteInstallation(context.Context, uuid.UUID, store.MembershipRecord) (uuid.UUID, error)
	RecordCancelAcknowledged(context.Context, uuid.UUID, store.MembershipRecord) (store.MembershipRecord, error)
	MembershipCleanupChatIDs(context.Context, store.MembershipRecord) ([]uuid.UUID, error)
	CompleteCancellation(context.Context, uuid.UUID, store.MembershipRecord) error
}
type MembershipTransport interface {
	CheckManageBots(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	Add(context.Context, *spacev1.AddBotMemberRequest) error
	Remove(context.Context, *spacev1.RemoveBotMemberRequest) error
}

// MembershipEffects must validate the exact Space-owned chat scope before
// publication and settle protected receiver cleanup before releasing a generation.
type MembershipEffects interface {
	ValidateChats(context.Context, store.MembershipRecord, *botv1.InstallBotInSpaceRequest) error
	Cleanup(context.Context, store.MembershipRecord, []uuid.UUID) error
}
type MembershipProducer struct {
	Journal   MembershipJournal
	Transport MembershipTransport
	Effects   MembershipEffects
	Source    uuid.UUID
}

func membershipError(err error) error {
	if errors.Is(err, store.ErrMembershipConflict) {
		return status.Error(codes.FailedPrecondition, "installation operation conflicts with current state")
	}
	return status.Error(codes.Unavailable, "protected installation dependency unavailable")
}
func (p *MembershipProducer) authorize(ctx context.Context, a installprincipal.Administrator, space string) (uuid.UUID, error) {
	if a.AccountID() == uuid.Nil || a.ProfileID() == uuid.Nil {
		return uuid.Nil, status.Error(codes.Unauthenticated, "verified administrator required")
	}
	id, err := uuid.Parse(space)
	if err != nil || id == uuid.Nil || id.String() != space {
		return uuid.Nil, status.Error(codes.InvalidArgument, "invalid space_id")
	}
	if p == nil || p.Journal == nil || p.Transport == nil || p.Effects == nil || p.Source == uuid.Nil {
		return uuid.Nil, membershipError(nil)
	}
	allowed, err := p.Transport.CheckManageBots(ctx, id, a.ProfileID())
	if err != nil {
		return uuid.Nil, membershipError(err)
	}
	if !allowed {
		return uuid.Nil, status.Error(codes.PermissionDenied, "SPACE_MANAGE_BOTS required")
	}
	return id, nil
}
func (p *MembershipProducer) Install(ctx context.Context, a installprincipal.Administrator, req *botv1.InstallBotInSpaceRequest) (*botv1.InstallBotInSpaceResponse, error) {
	if _, err := p.authorize(ctx, a, req.GetSpaceId()); err != nil {
		return nil, err
	}
	if _, _, err := store.CanonicalInstallInputs(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid installation request")
	}
	r, err := p.Journal.PrepareInstall(ctx, p.Source, a, req)
	if err != nil {
		return nil, membershipError(err)
	}
	if r.CompletionDone {
		return &botv1.InstallBotInSpaceResponse{InstallationId: r.InstallationID.String()}, nil
	}
	if err = p.Effects.ValidateChats(ctx, r, req); err != nil {
		return nil, membershipError(err)
	}
	r, err = p.Journal.ClaimMembershipAttempt(ctx, p.Source, r, 10*time.Second)
	if err != nil {
		return nil, membershipError(err)
	}
	add, err := r.AddRequest()
	if err != nil {
		return nil, membershipError(err)
	}
	if err = p.Transport.Add(ctx, add); err != nil {
		return nil, membershipError(err)
	}
	r, err = p.Journal.RecordAddAcknowledged(ctx, p.Source, r)
	if err != nil {
		return nil, membershipError(err)
	}
	id, err := p.Journal.CompleteInstallation(ctx, p.Source, r)
	if err != nil {
		return nil, membershipError(err)
	}
	return &botv1.InstallBotInSpaceResponse{InstallationId: id.String()}, nil
}
func (p *MembershipProducer) Uninstall(ctx context.Context, a installprincipal.Administrator, req *botv1.UninstallBotFromSpaceRequest) (*botv1.UninstallBotFromSpaceResponse, error) {
	space, err := p.authorize(ctx, a, req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	bot, err := uuid.Parse(req.GetBotId())
	if err != nil || bot == uuid.Nil || bot.String() != req.GetBotId() {
		return nil, status.Error(codes.InvalidArgument, "invalid bot_id")
	}
	r, err := p.Journal.BeginCancel(ctx, p.Source, a, space, bot)
	if err != nil {
		return nil, membershipError(err)
	}
	if r.CleanupSettled {
		return &botv1.UninstallBotFromSpaceResponse{}, nil
	}
	r, err = p.Journal.ClaimMembershipAttempt(ctx, p.Source, r, 10*time.Second)
	if err != nil {
		return nil, membershipError(err)
	}
	remove, err := r.RemoveRequest()
	if err != nil {
		return nil, membershipError(err)
	}
	if err = p.Transport.Remove(ctx, remove); err != nil {
		return nil, membershipError(err)
	}
	r, err = p.Journal.RecordCancelAcknowledged(ctx, p.Source, r)
	if err != nil {
		return nil, membershipError(err)
	}
	chats, err := p.Journal.MembershipCleanupChatIDs(ctx, r)
	if err != nil {
		return nil, membershipError(err)
	}
	if err = p.Effects.Cleanup(ctx, r, chats); err != nil {
		return nil, membershipError(err)
	}
	if err = p.Journal.CompleteCancellation(ctx, p.Source, r); err != nil {
		return nil, membershipError(err)
	}
	return &botv1.UninstallBotFromSpaceResponse{}, nil
}
