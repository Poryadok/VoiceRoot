package lifecyclecoord

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	notificationv1 "voice.app/voice/notification/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/spacecore"
)

// ManifestStore exposes the durable boundaries needed to start the freeze.
// Auth evidence is checked before contacting any participant, and the Chat
// binding is saved only after Messaging and File have accepted the exact root.
type ManifestStore interface {
	LifecycleDeletionProofRecorded(context.Context, uuid.UUID) (bool, error)
	BeginLifecycleFreeze(context.Context, uuid.UUID, *commonv1.ManifestBinding) (*spacecore.LifecycleAggregate, error)
}

// ManifestOwners are authenticated owner transports. Implementations must use
// the dedicated mTLS listeners and request-bound Space service principals.
type ManifestOwners interface {
	PrepareChatManifest(context.Context, *chatv1.PrepareSpaceDeletionManifestRequest) (*chatv1.PrepareSpaceDeletionManifestResponse, error)
	GetChatManifestPage(context.Context, *chatv1.GetSpacePurgeManifestPageRequest) (*chatv1.GetSpacePurgeManifestPageResponse, error)
	ImportMessagingManifestPage(context.Context, *messagingv1.ImportSpacePurgeManifestPageRequest) (*messagingv1.ImportSpacePurgeManifestPageResponse, error)
	ImportNotificationManifestPage(context.Context, *notificationv1.ImportSpacePurgeManifestPageRequest) (*notificationv1.ImportSpacePurgeManifestPageResponse, error)
	PrepareFileManifest(context.Context, *filev1.PrepareSpaceDeletionReferenceManifestRequest) (*filev1.PrepareSpaceDeletionReferenceManifestResponse, error)
	SealSpaceFileProducer(context.Context, spacecore.LifecycleSnapshot) error
}

// PrepareFreeze captures Chat's immutable work set, imports every exact page
// into Messaging, asks File to seal its three producer declarations, and only
// then persists the common binding in Space. Every remote request is replayable
// from the saved operation and generation after timeout or process restart.
func (c *Coordinator) PrepareFreeze(ctx context.Context, spaceID uuid.UUID) (*spacecore.LifecycleAggregate, error) {
	if c == nil || c.dependencies.Store == nil || c.dependencies.Manifests == nil || spaceID == uuid.Nil {
		return nil, ErrCoordinatorNotConfigured
	}
	store, ok := c.dependencies.Store.(ManifestStore)
	if !ok {
		return nil, ErrCoordinatorNotConfigured
	}
	proofSaved, err := store.LifecycleDeletionProofRecorded(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	if !proofSaved {
		return nil, errors.New("Auth deletion proof receipt is not durable")
	}
	aggregate, err := c.dependencies.Store.LoadLifecycle(ctx, spaceID)
	if err != nil {
		return nil, err
	}
	snapshot := aggregate.Snapshot()
	if snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING &&
		snapshot.Phase != spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_FREEZE_PENDING {
		return nil, fmt.Errorf("manifest preparation is not pending in phase %s", snapshot.Phase)
	}
	deadline := c.dependencies.RPCDeadline
	if deadline <= 0 {
		deadline = participantRPCDeadline
	}
	callCtx := func() (context.Context, context.CancelFunc) { return context.WithTimeout(ctx, deadline) }

	prepareRequest := &chatv1.PrepareSpaceDeletionManifestRequest{
		ProtocolVersion: 1, SpaceId: snapshot.SpaceID,
		DeletionOperationId: snapshot.DeletionOperationID, ScheduleGeneration: snapshot.Generation,
	}
	chatCtx, cancel := callCtx()
	prepared, err := c.dependencies.Manifests.PrepareChatManifest(chatCtx, prepareRequest)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("prepare Chat deletion manifest: %w", err)
	}
	chatReceipt := prepared.GetReceipt()
	manifest := chatReceipt.GetChatManifest()
	if chatReceipt == nil || chatReceipt.GetProtocolVersion() != 1 || chatReceipt.GetReceiptId() == "" ||
		chatReceipt.GetSpaceId() != snapshot.SpaceID || chatReceipt.GetDeletionOperationId() != snapshot.DeletionOperationID ||
		chatReceipt.GetScheduleGeneration() != snapshot.Generation ||
		chatReceipt.GetAppliedState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN ||
		!manifestReceiptMatches(prepareRequest, chatReceipt.GetRequestSha256()) || chatReceipt.GetAppliedAt() == nil || chatReceipt.GetAppliedAt().CheckValid() != nil ||
		!validManifestBinding(manifest) {
		return nil, errors.New("Chat returned an invalid deletion manifest receipt")
	}
	if snapshot.Manifest != nil && !proto.Equal(snapshot.Manifest, manifest) {
		return nil, errors.New("Chat deletion manifest changed during lifecycle replay")
	}

	var imported uint64
	pageToken := ""
	seenTokens := map[string]struct{}{}
	lastItemID := ""
	for pageIndex := uint64(0); ; pageIndex++ {
		pageCtx, pageCancel := callCtx()
		pageResponse, pageErr := c.dependencies.Manifests.GetChatManifestPage(pageCtx, &chatv1.GetSpacePurgeManifestPageRequest{
			ProtocolVersion: 1, SpaceId: snapshot.SpaceID, DeletionOperationId: snapshot.DeletionOperationID,
			Generation: snapshot.Generation, ManifestId: manifest.GetManifestId(), PageToken: pageToken,
		})
		pageCancel()
		if pageErr != nil {
			return nil, fmt.Errorf("read Chat manifest page %d: %w", pageIndex, pageErr)
		}
		page := pageResponse.GetPage()
		if page == nil || page.GetProtocolVersion() != 1 || page.GetPageIndex() != pageIndex ||
			!proto.Equal(page.GetManifest(), manifest) || len(page.GetPageSha256()) != 32 {
			return nil, fmt.Errorf("Chat returned an invalid manifest page %d", pageIndex)
		}
		for _, itemID := range page.GetItemIds() {
			if itemID == "" || (lastItemID != "" && itemID <= lastItemID) {
				return nil, fmt.Errorf("Chat manifest page %d is not globally sorted and unique", pageIndex)
			}
			lastItemID = itemID
		}
		if len(page.GetItemIds()) == 0 && manifest.GetItemCount() != 0 {
			return nil, fmt.Errorf("Chat manifest page %d is empty before the root is complete", pageIndex)
		}
		if uint64(len(page.GetItemIds())) > manifest.GetItemCount()-imported {
			return nil, errors.New("Chat manifest pages exceed the declared item count")
		}
		seals := page.GetNextPageToken() == ""
		if manifest.GetItemCount() == 0 && (!seals || len(page.GetItemIds()) != 0) {
			return nil, errors.New("empty Chat manifest must be represented by one final empty page")
		}
		if !seals && pageIndex+1 >= manifest.GetItemCount() {
			return nil, errors.New("Chat manifest page count exceeds the declared item count")
		}
		importCtx, importCancel := callCtx()
		messagingRequest := &messagingv1.ImportSpacePurgeManifestPageRequest{
			ProtocolVersion: 1, SpaceId: snapshot.SpaceID, DeletionOperationId: snapshot.DeletionOperationID,
			ScheduleGeneration: snapshot.Generation, Page: page, SealsManifest: seals,
		}
		importResponse, importErr := c.dependencies.Manifests.ImportMessagingManifestPage(importCtx, messagingRequest)
		importCancel()
		if importErr != nil {
			return nil, fmt.Errorf("import Messaging manifest page %d: %w", pageIndex, importErr)
		}
		importReceipt := importResponse.GetReceipt()
		if importReceipt == nil || importReceipt.GetProtocolVersion() != 1 || importReceipt.GetReceiptId() == "" ||
			importReceipt.GetSpaceId() != snapshot.SpaceID || importReceipt.GetDeletionOperationId() != snapshot.DeletionOperationID ||
			importReceipt.GetGeneration() != snapshot.Generation || !proto.Equal(importReceipt.GetManifest(), manifest) ||
			importReceipt.GetPageIndex() != pageIndex || importReceipt.GetAcceptedCount() != uint64(len(page.GetItemIds())) ||
			!bytes.Equal(importReceipt.GetPageSha256(), page.GetPageSha256()) || importReceipt.GetManifestSealed() != seals ||
			!manifestReceiptMatches(messagingRequest, importReceipt.GetRequestSha256()) || importReceipt.GetCompletedAt() == nil || importReceipt.GetCompletedAt().CheckValid() != nil {
			return nil, fmt.Errorf("Messaging returned an invalid receipt for manifest page %d", pageIndex)
		}
		notificationRequest := &notificationv1.ImportSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: snapshot.SpaceID, DeletionOperationId: snapshot.DeletionOperationID, ScheduleGeneration: snapshot.Generation, Page: page, SealsManifest: seals}
		notificationCtx, notificationCancel := callCtx()
		notificationResponse, notificationErr := c.dependencies.Manifests.ImportNotificationManifestPage(notificationCtx, notificationRequest)
		notificationCancel()
		if notificationErr != nil {
			return nil, fmt.Errorf("import Notification manifest page %d: %w", pageIndex, notificationErr)
		}
		notificationReceipt := notificationResponse.GetReceipt()
		requestWire, marshalErr := (proto.MarshalOptions{Deterministic: true}).Marshal(notificationRequest)
		requestDigest := sha256.Sum256(append(append([]byte(notificationRequest.ProtoReflect().Descriptor().FullName()), 0), requestWire...))
		if marshalErr != nil || notificationReceipt == nil || notificationReceipt.GetProtocolVersion() != 1 || notificationReceipt.GetReceiptId() == "" || notificationReceipt.GetSpaceId() != snapshot.SpaceID || notificationReceipt.GetDeletionOperationId() != snapshot.DeletionOperationID || notificationReceipt.GetGeneration() != snapshot.Generation || !proto.Equal(notificationReceipt.GetManifest(), manifest) || notificationReceipt.GetPageIndex() != pageIndex || notificationReceipt.GetAcceptedCount() != uint64(len(page.GetItemIds())) || !bytes.Equal(notificationReceipt.GetPageSha256(), page.GetPageSha256()) || notificationReceipt.GetManifestSealed() != seals || !bytes.Equal(notificationReceipt.GetRequestSha256(), requestDigest[:]) || notificationReceipt.GetCompletedAt() == nil || notificationReceipt.GetCompletedAt().CheckValid() != nil {
			return nil, fmt.Errorf("Notification returned an invalid receipt for manifest page %d", pageIndex)
		}
		imported += uint64(len(page.GetItemIds()))
		if seals {
			break
		}
		nextToken := page.GetNextPageToken()
		if _, exists := seenTokens[nextToken]; exists {
			return nil, errors.New("Chat manifest page token repeated")
		}
		seenTokens[nextToken] = struct{}{}
		pageToken = nextToken
	}
	if imported != manifest.GetItemCount() {
		return nil, fmt.Errorf("Messaging imported %d manifest items; Chat declared %d", imported, manifest.GetItemCount())
	}

	fileCtx, fileCancel := callCtx()
	fileRequest := &filev1.PrepareSpaceDeletionReferenceManifestRequest{
		ProtocolVersion: 1, SpaceId: snapshot.SpaceID, DeletionOperationId: snapshot.DeletionOperationID,
		ScheduleGeneration: snapshot.Generation, ChatManifest: manifest,
	}
	fileResponse, err := c.dependencies.Manifests.PrepareFileManifest(fileCtx, fileRequest)
	fileCancel()
	if err != nil {
		return nil, fmt.Errorf("prepare File reference manifest: %w", err)
	}
	fileReceipt := fileResponse.GetReceipt()
	if fileReceipt == nil || fileReceipt.GetProtocolVersion() != 1 || fileReceipt.GetReceiptId() == "" ||
		fileReceipt.GetSpaceId() != snapshot.SpaceID || fileReceipt.GetDeletionOperationId() != snapshot.DeletionOperationID ||
		fileReceipt.GetScheduleGeneration() != snapshot.Generation ||
		fileReceipt.GetAppliedState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN ||
		!manifestReceiptMatches(fileRequest, fileReceipt.GetRequestSha256()) || fileReceipt.GetAppliedAt() == nil || fileReceipt.GetAppliedAt().CheckValid() != nil {
		return nil, errors.New("File returned an invalid reference manifest receipt")
	}
	sealCtx, sealCancel := callCtx()
	err = c.dependencies.Manifests.SealSpaceFileProducer(sealCtx, snapshot)
	sealCancel()
	if err != nil {
		return nil, fmt.Errorf("seal Space File producer: %w", err)
	}
	return store.BeginLifecycleFreeze(ctx, spaceID, manifest)
}

func validManifestBinding(manifest *commonv1.ManifestBinding) bool {
	return manifest != nil && manifest.GetManifestId() != "" && len(manifest.GetManifestSha256()) == 32
}

func manifestReceiptMatches(request proto.Message, digest []byte) bool {
	wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return false
	}
	expected := sha256.Sum256(append(append([]byte(request.ProtoReflect().Descriptor().FullName()), 0), wire...))
	return bytes.Equal(expected[:], digest)
}
