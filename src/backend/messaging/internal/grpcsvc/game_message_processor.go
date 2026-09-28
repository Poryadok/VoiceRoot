package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"

	"voice/backend/messaging/internal/gameprotocol"
	"voice/backend/messaging/internal/store"
)

type GameAuthKeySnapshot = gameprotocol.AuthKeySnapshot

type GameAuthKeySource interface {
	Snapshot(context.Context) (GameAuthKeySnapshot, error)
}

// GameBindingAuthority checks the T30/T31 app/environment/binding/chat
// relationship and current membership after Auth issues the execution permit.
type GameBindingAuthority interface {
	AuthorizeMessageResource(context.Context, gameprotocol.DeviceAuthority, gameprotocol.Message) error
	AuthorizeMessageChat(context.Context, uuid.UUID, gameprotocol.DeviceAuthority, gameprotocol.Message) error
}

type GameMessageExecutionPermitIssuer interface {
	Issue(context.Context, gameprotocol.DeviceAuthority, uuid.UUID, []byte) (string, error)
	Complete(context.Context, uuid.UUID, uuid.UUID, string) error
}

// GameAppChatResourceAuthority is supplied by the T30/T31 resource-mapping
// owner. Chat membership alone does not prove that a chat belongs to the
// asserted application, environment, and binding.
type GameAppChatResourceAuthority interface {
	AuthorizeAppBindingChat(context.Context, gameprotocol.DeviceAuthority, gameprotocol.Message) error
}

// AuthBackedGameBindingAuthority checks the T30/T31 app resource mapping
// before permit issuance, then checks membership against Auth's permit profile.
type AuthBackedGameBindingAuthority struct {
	Chats            ChatGuard
	ResourceMappings GameAppChatResourceAuthority
}

func (a *AuthBackedGameBindingAuthority) AuthorizeMessageResource(ctx context.Context, authority gameprotocol.DeviceAuthority, message gameprotocol.Message) error {
	if a == nil || a.ResourceMappings == nil {
		return errors.New("game app/environment/binding/chat resource mapping is unavailable")
	}
	if err := a.ResourceMappings.AuthorizeAppBindingChat(ctx, authority, message); err != nil {
		return errors.New("current game app/environment/binding is not linked to the target chat")
	}
	return nil
}

func (a *AuthBackedGameBindingAuthority) AuthorizeMessageChat(ctx context.Context, profileID uuid.UUID, authority gameprotocol.DeviceAuthority, message gameprotocol.Message) error {
	if a == nil || a.Chats == nil || profileID == uuid.Nil {
		return errors.New("chat membership authority is unavailable")
	}
	if err := a.Chats.EnsureMember(ctx, message.ChatID, profileID); err != nil {
		return errors.New("current bound profile is not a member of the target chat")
	}
	return nil
}

type GameAttachmentManifestVerifier interface {
	VerifyGameAttachmentManifest(context.Context, uuid.UUID, uuid.UUID, []gameprotocol.Attachment) error
}

// VerifiedGameMessageProcessor implements receipt-first delivery followed by
// Auth assertion, device JWS, T16 binding/chat, and File provenance checks.
// Every dependency except storage is required only for a new operation, so an
// exact retained receipt remains available during outages and after expiry.
type VerifiedGameMessageProcessor struct {
	Store    *store.MessagesStore
	AuthKeys GameAuthKeySource
	Permits  GameMessageExecutionPermitIssuer
	Bindings GameBindingAuthority
	Files    GameAttachmentManifestVerifier
	Clock    func() time.Time
}

func (p *VerifiedGameMessageProcessor) ProcessGameMessage(ctx context.Context, compactJWS, authorityJWS string) (*store.MessageRow, error) {
	if p == nil || p.Store == nil {
		return nil, errors.New("game message storage unavailable")
	}
	receiptKey, err := gameprotocol.ExtractReceiptKey(compactJWS)
	if err != nil {
		return nil, err
	}
	row, found, err := p.Store.LookupGameMessageReceipt(ctx, receiptKey)
	if err != nil || found {
		return row, err
	}
	if authorityJWS == "" {
		return nil, errors.New("fresh Auth device authority is required")
	}
	if p.AuthKeys == nil || p.Permits == nil || p.Bindings == nil {
		return nil, errors.New("current Auth assertion, execution permit, or game chat authority is unavailable")
	}
	expected, err := gameprotocol.ExtractDeviceAuthorityExpected(authorityJWS)
	if err != nil {
		return nil, err
	}
	expected.ChatID = receiptKey.ChatID.String()
	snapshot, err := p.AuthKeys.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	monotonicNow := time.Now()
	now := monotonicNow.UTC()
	if p.Clock != nil {
		now = p.Clock().UTC()
	}
	if snapshot.RefreshedAt.IsZero() || snapshot.RefreshedAt.After(now.Add(250*time.Millisecond)) || now.Sub(snapshot.RefreshedAt) > 5*time.Second {
		return nil, errors.New("auth principal JWKS is stale")
	}
	message, authority, err := gameprotocol.VerifyRequest(compactJWS, authorityJWS, snapshot.Keys,
		now, snapshot.ClockUncertainty, expected)
	if err != nil {
		return nil, err
	}
	if message.ApplicationID != receiptKey.ApplicationID || message.EnvironmentID != receiptKey.EnvironmentID ||
		message.OperationID != receiptKey.OperationID || message.ChatID != receiptKey.ChatID ||
		message.MessageID != receiptKey.MessageID || message.Revision != receiptKey.Revision {
		return nil, errors.New("message receipt key changed during verification")
	}
	digest := sha256.Sum256(message.RawPayload)
	requestDigest := hex.EncodeToString(digest[:])
	priorCompletion, _, completionFound, err := p.Store.GameMessagePermitCompletionByOperation(ctx, message.OperationID)
	if err != nil {
		return nil, err
	}
	if completionFound {
		if priorCompletion.RequestSHA256 != requestDigest {
			return nil, store.ErrGameOperationConflict
		}
		if priorCompletion.Outcome == "aborted" {
			return nil, errors.New("game message operation was already aborted")
		}
		return nil, errors.New("committed execution permit has no matching durable message receipt")
	}
	if err := p.Bindings.AuthorizeMessageResource(ctx, authority, message); err != nil {
		return nil, errors.New("game message app-scoped resource authorization denied")
	}
	permitJWS, err := p.Permits.Issue(ctx, authority, message.OperationID, message.RawPayload)
	if err != nil {
		return nil, errors.New("auth execution permit issuance denied")
	}
	permitMonotonicNow := time.Now()
	permitNow := permitMonotonicNow.UTC()
	if p.Clock != nil {
		permitNow = p.Clock().UTC()
	}
	permit, err := gameprotocol.VerifyExecutionPermit(permitJWS, snapshot.Keys, permitNow, snapshot.ClockUncertainty, gameprotocol.ExecutionPermitExpected{
		Device: authority, OperationID: message.OperationID, RequestSHA256: requestDigest, MutationBytes: message.RawPayload,
	})
	if err != nil {
		return nil, errors.New("auth execution permit verification denied")
	}
	profileID := permit.ProfileID
	completion := store.GameMessagePermitCompletion{
		PermitID: permit.ID, GISPermitID: permit.GISPermitID, OperationID: permit.OperationID,
		RequestSHA256: permit.RequestSHA256,
	}
	abortPermit := func() {
		completion.Outcome = "aborted"
		if err := p.Store.RecordAbortedGameMessagePermit(ctx, completion); err != nil {
			// The permit is short lived; if the local outbox cannot be committed,
			// try to abort remotely and let GIS expiry remain the final fence.
			_ = p.Permits.Complete(ctx, permit.ID, permit.OperationID, "aborted")
			return
		}
		_ = p.dispatchGamePermitCompletion(ctx, completion)
	}
	if err := p.Bindings.AuthorizeMessageChat(ctx, profileID, authority, message); err != nil {
		abortPermit()
		return nil, errors.New("game message chat authorization denied")
	}
	if len(message.Attachments) > 0 {
		if p.Files == nil {
			abortPermit()
			return nil, errors.New("file attachment provenance is unavailable")
		}
		if err := p.Files.VerifyGameAttachmentManifest(ctx, profileID, message.ChatID, message.Attachments); err != nil {
			abortPermit()
			return nil, errors.New("file attachment provenance denied")
		}
	}
	remaining := permit.ExpiresAt.Sub(permitNow) - 250*time.Millisecond
	if remaining <= 0 {
		abortPermit()
		return nil, errors.New("auth execution permit expires before bounded message transaction")
	}
	transactionWindow := min(remaining, 250*time.Millisecond)
	deadline := permitMonotonicNow.Add(transactionWindow)
	transactionCtx, cancel := context.WithTimeout(ctx, transactionWindow)
	defer cancel()
	completion.Outcome = "committed"
	row, err = p.Store.ApplyGameMessageWithExecutionPermit(transactionCtx, message, profileID, deadline, completion)
	if err != nil {
		return nil, err
	}
	_ = p.dispatchGamePermitCompletion(ctx, completion)
	return row, nil
}

func (p *VerifiedGameMessageProcessor) dispatchGamePermitCompletion(ctx context.Context, completion store.GameMessagePermitCompletion) error {
	if p == nil || p.Store == nil || p.Permits == nil {
		return errors.New("game execution permit completion dispatcher unavailable")
	}
	if err := p.Permits.Complete(ctx, completion.PermitID, completion.OperationID, completion.Outcome); err != nil {
		return err
	}
	return p.Store.MarkGameMessagePermitCompletion(ctx, completion)
}

func (p *VerifiedGameMessageProcessor) dispatchPendingGamePermitCompletions(ctx context.Context) error {
	rows, err := p.Store.PendingGameMessagePermitCompletions(ctx, 64)
	if err != nil {
		return err
	}
	for _, completion := range rows {
		if err := p.dispatchGamePermitCompletion(ctx, completion); err != nil {
			return err
		}
	}
	return nil
}

// RunGameMessagePermitCompletionDispatcher retries durable Auth/GIS completion
// receipts until acknowledged. Unknown completion remains pending and is never
// interpreted as success.
func (p *VerifiedGameMessageProcessor) RunGameMessagePermitCompletionDispatcher(ctx context.Context, logger interface{ Error(string, ...any) }) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if p != nil && p.Store != nil && p.Permits != nil {
			if err := p.dispatchPendingGamePermitCompletions(ctx); err != nil {
				if logger != nil {
					logger.Error("query game execution permit completion outbox", "error", err)
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
