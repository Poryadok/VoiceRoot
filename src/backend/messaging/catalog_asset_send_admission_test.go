package main

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/grpcsvc"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/integrationtest"
)

type catalogAdmissionStop struct{ reached bool }

func (s *catalogAdmissionStop) CheckMessageAllowed(context.Context, uuid.UUID, uuid.UUID, string) error {
	s.reached = true
	return errors.New("test stop after catalog admission")
}
func (*catalogAdmissionStop) IsShadowBanned(context.Context, uuid.UUID) (bool, error) {
	return false, nil
}

// This proves the existing authenticated send path reaches the stage after
// catalog admission through the runtime-created clients. It deliberately stops
// before the write: File byte settlement/reference acquisition are95's scope.
func TestCatalogRuntimeAuthenticatedSendAdmission(t *testing.T) {
	if testing.Short() {
		t.Skip("isolated PostgreSQL send policy")
	}
	f := newCatalogRuntimeFixture(t)
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "catalog_runtime", "")
	_, source, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
	integrationtest.ApplySQLFile(t, ctx, pool, root, "src/backend/migrations/chat_db/000001_init.up.sql")
	snapshot := f.file.snapshot
	chat := uuid.MustParse(f.chat.destination)
	actor := uuid.MustParse(snapshot.ActorProfileId)
	_, err := pool.Exec(ctx, `INSERT INTO chats(id,type,creator_profile_id,allow_user_main_feed) VALUES($1,'group',$2,true)`, chat, actor)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO chat_members(chat_id,profile_id,role) VALUES($1,$2,'member')`, chat, actor)
	require.NoError(t, err)
	stop := &catalogAdmissionStop{}
	svc := &grpcsvc.MessagingGRPC{Messages: &store.MessagesStore{Pool: pool}, ChatGuard: &store.SQLChatGuard{Pool: pool}, ChatTypeResolver: &store.SQLChatTypeResolver{Pool: pool}, ChatThreadPolicy: &store.SQLChatThreadPolicy{Pool: pool}, CatalogAssets: f.runtime.Coordinator, PlatformMod: stop}
	kind := chatv1.ChatType_CHAT_TYPE_GROUP
	contentType := messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_GIF
	request := &messagingv1.SendMessageRequest{Chat: &chatv1.ChatRef{Id: chat.String(), Type: &kind}, AttachmentsJson: `[{"file_id":"` + snapshot.FileId + `","type":"gif"}]`, ContentType: &contentType}
	actorContext := metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-user-id", snapshot.ActorAccountId, "x-voice-profile-id", snapshot.ActorProfileId))
	// Incoming/outgoing spoofed identity metadata must not survive into s2s calls.
	actorContext = metadata.NewOutgoingContext(actorContext, metadata.Pairs("authorization", "forged", "x-voice-user-id", "forged", "x-voice-profile-id", "forged"))
	_, err = svc.SendMessage(actorContext, request)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, "test stop after catalog admission", status.Convert(err).Message())
	require.True(t, stop.reached)
	f.mu.Lock()
	calls := append([]string(nil), f.calls...)
	f.mu.Unlock()
	require.Equal(t, []string{"/voice.file.v1.CatalogAssetService/LookupCatalogAsset", "/voice.chat.v1.CatalogAssetService/ValidateGifAsset", "/voice.file.v1.CatalogAssetService/ValidateCatalogAsset"}, calls)
	for _, scenario := range []string{"wrong-actor", "forged-provider", "unknown-file", "wrong-destination", "wrong-source", "not-ready", "legacy-zero-proof", "superseded-file", "mixed-chat-proof"} {
		t.Run(scenario, func(t *testing.T) {
			candidate := proto.Clone(request).(*messagingv1.SendMessageRequest)
			callContext := actorContext
			stop.reached = false
			oldSource, oldIntent := snapshot.SourceId, snapshot.Intent
			oldSelection, oldCurrent := snapshot.SelectionRevision, snapshot.CurrentSelectionRevision
			oldDestination := f.chat.destination
			defer func() {
				snapshot.SourceId = oldSource
				snapshot.Intent = oldIntent
				snapshot.SelectionRevision, snapshot.CurrentSelectionRevision = oldSelection, oldCurrent
				f.chat.destination = oldDestination
			}()
			switch scenario {
			case "wrong-actor":
				callContext = metadata.NewIncomingContext(ctx, metadata.Pairs("x-voice-user-id", uuid.NewString(), "x-voice-profile-id", snapshot.ActorProfileId))
			case "forged-provider":
				candidate.AttachmentsJson = `[{"file_id":"` + snapshot.FileId + `","type":"gif","provider":"forged"}]`
			case "unknown-file":
				candidate.AttachmentsJson = `[{"file_id":"` + uuid.NewString() + `","type":"gif"}]`
			case "wrong-destination":
				f.chat.destination = uuid.NewString()
			case "wrong-source":
				f.chat.snapshot = proto.Clone(snapshot).(*filev1.ValidateCatalogAssetResponse)
				f.chat.snapshot.SourceId = uuid.NewString()
				defer func() { f.chat.snapshot = snapshot }()
			case "not-ready":
				snapshot.Intent = ""
			case "legacy-zero-proof":
				snapshot.SelectionRevision, snapshot.CurrentSelectionRevision = 0, 0
			case "superseded-file":
				snapshot.CurrentSelectionRevision++
			case "mixed-chat-proof":
				f.chat.snapshot = proto.Clone(snapshot).(*filev1.ValidateCatalogAssetResponse)
				f.chat.snapshot.SelectionRevision++
				f.chat.snapshot.CurrentSelectionRevision++
				defer func() { f.chat.snapshot = snapshot }()
			}
			_, err := svc.SendMessage(callContext, candidate)
			require.Error(t, err)
			require.False(t, stop.reached, "invalid source must not reach post-admission stage")
		})
	}
}
