package grpcsvc

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/pkg/principal"
)

type catalogFileFake struct {
	snapshot                   *filev1.ValidateCatalogAssetResponse
	ordinary                   bool
	lookupCalls, validateCalls int
	fail                       error
	mutateFinal                bool
}

func (f *catalogFileFake) LookupCatalogAsset(ctx context.Context, req *filev1.LookupCatalogAssetRequest, _ ...grpc.CallOption) (*filev1.LookupCatalogAssetResponse, error) {
	f.lookupCalls++
	md, _ := metadata.FromOutgoingContext(ctx)
	if len(md.Get("authorization")) != 1 || len(md.Get("x-request-id")) != 1 || len(md.Get("x-voice-profile-id")) != 0 {
		return nil, status.Error(codes.Unauthenticated, "unsigned call")
	}
	if f.fail != nil {
		return nil, f.fail
	}
	if f.ordinary {
		return &filev1.LookupCatalogAssetResponse{Classification: &filev1.LookupCatalogAssetResponse_OrdinaryFile{OrdinaryFile: true}}, nil
	}
	return &filev1.LookupCatalogAssetResponse{Classification: &filev1.LookupCatalogAssetResponse_Catalog{Catalog: proto.Clone(f.snapshot).(*filev1.ValidateCatalogAssetResponse)}}, nil
}
func (f *catalogFileFake) ValidateCatalogAsset(_ context.Context, _ *filev1.ValidateCatalogAssetRequest, _ ...grpc.CallOption) (*filev1.ValidateCatalogAssetResponse, error) {
	f.validateCalls++
	r := proto.Clone(f.snapshot).(*filev1.ValidateCatalogAssetResponse)
	if f.mutateFinal {
		r.SourceSha256 = bytes.Repeat([]byte{9}, 32)
	}
	return r, nil
}

type catalogChatFake struct {
	calls           int
	denyDestination bool
	wrongSource     bool
	destination     string
}

func (f *catalogChatFake) ValidateGifAsset(_ context.Context, r *chatv1.ValidateGifAssetRequest, _ ...grpc.CallOption) (*chatv1.ValidateGifAssetResponse, error) {
	f.calls++
	f.destination = r.GetDestinationChat().GetId()
	if f.denyDestination {
		return nil, status.Error(codes.PermissionDenied, "destination denied")
	}
	source := "00000000-0000-4000-8000-000000000001"
	if f.wrongSource {
		source = uuid.NewString()
	}
	return &chatv1.ValidateGifAssetResponse{SourceId: source, ActorAccountId: r.ActorAccountId, ActorProfileId: r.ActorProfileId, Provider: r.Provider, ProviderId: r.ProviderId, FileId: r.FileId}, nil
}

func catalogAdmissionFixture(t *testing.T) (*MessagingGRPC, context.Context, uuid.UUID, messageAttachment, *catalogFileFake, *catalogChatFake) {
	t.Helper()
	account, profile, chat, file := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "messaging", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	f := &catalogFileFake{snapshot: &filev1.ValidateCatalogAssetResponse{SourceId: "00000000-0000-4000-8000-000000000001", ActorAccountId: account.String(), ActorProfileId: profile.String(), Provider: "fixture", ProviderId: "asset-1", FileId: file.String(), ObjectRevision: 1, Intent: "gif", ScanResult: "clean", FileType: "video", SourceSha256: bytes.Repeat([]byte{1}, 32), DeliveredSha256: bytes.Repeat([]byte{2}, 32), DeliveredSizeBytes: 42, DeliveredMimeType: "video/mp4"}}
	c := &catalogChatFake{}
	s := &MessagingGRPC{CatalogAssets: &CatalogAssetCoordinator{Files: f, Chat: c, Issuer: issuer}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-user-id", account.String(), "x-voice-profile-id", profile.String()))
	ctx = withCatalogDestination(ctx, chat, chatv1.ChatType_CHAT_TYPE_GROUP, false)
	return s, ctx, chat, messageAttachment{FileID: file.String(), Type: "gif"}, f, c
}

func TestCatalogSendClassifiesOmittedAttributionAndAuthorizesEachDestination(t *testing.T) {
	s, ctx, chat, attachment, f, c := catalogAdmissionFixture(t)
	remaining, err := s.validateCatalogAttachments(ctx, &chat, []messageAttachment{attachment}, "gif")
	require.NoError(t, err)
	require.Empty(t, remaining)
	require.Equal(t, 1, f.lookupCalls)
	require.Equal(t, 1, c.calls)
	require.Equal(t, 1, f.validateCalls)
	require.Equal(t, chat.String(), c.destination)
	second := uuid.New()
	secondCtx := withCatalogDestination(ctx, second, chatv1.ChatType_CHAT_TYPE_GROUP, false)
	_, err = s.validateCatalogAttachments(secondCtx, &second, []messageAttachment{attachment}, "gif")
	require.NoError(t, err)
	require.Equal(t, second.String(), c.destination)
	c.denyDestination = true
	_, err = s.validateCatalogAttachments(secondCtx, &second, []messageAttachment{attachment}, "gif")
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Equal(t, 2, f.validateCalls, "denied target must not reach final File attestation")
}

func TestCatalogSendRejectsForgedSourceAndNoClassificationFallback(t *testing.T) {
	for _, name := range []string{"lookup-unavailable", "wrong-actor", "wrong-source", "wrong-provider", "wrong-provider-id", "wrong-kind", "wrong-content", "wrong-intent", "stale-file-snapshot", "forward"} {
		t.Run(name, func(t *testing.T) {
			s, ctx, chat, a, f, c := catalogAdmissionFixture(t)
			target := &chat
			switch name {
			case "lookup-unavailable":
				f.fail = status.Error(codes.Unavailable, "unavailable")
			case "wrong-actor":
				f.snapshot.ActorProfileId = uuid.NewString()
			case "wrong-source":
				c.wrongSource = true
			case "wrong-provider":
				a.Provider = "forged"
			case "wrong-provider-id":
				a.ProviderID = "forged"
			case "wrong-kind":
				a.Type = "video"
			case "wrong-intent":
				f.snapshot.Intent = ""
			case "stale-file-snapshot":
				f.mutateFinal = true
			case "forward":
				target = nil
			}
			content := "gif"
			if name == "wrong-content" {
				content = "video"
			}
			remaining, err := s.validateCatalogAttachments(ctx, target, []messageAttachment{a}, content)
			require.Error(t, err)
			require.Nil(t, remaining, "catalog errors must never fall back to ordinary ACL")
		})
	}
}

func TestCatalogSendPreservesOrdinaryPathAndNeverDisclosesE2ESource(t *testing.T) {
	s, ctx, chat, a, f, c := catalogAdmissionFixture(t)
	f.ordinary = true
	remaining, err := s.validateCatalogAttachments(ctx, &chat, []messageAttachment{a}, "gif")
	require.NoError(t, err)
	require.Equal(t, []messageAttachment{a}, remaining)
	require.Zero(t, c.calls)
	f.ordinary = false
	before := f.lookupCalls
	ctx = withCatalogDestination(ctx, chat, chatv1.ChatType_CHAT_TYPE_DM, true)
	remaining, err = s.validateCatalogAttachments(ctx, &chat, []messageAttachment{a}, "gif")
	require.NoError(t, err)
	require.Equal(t, []messageAttachment{a}, remaining)
	require.Equal(t, before, f.lookupCalls)
	require.Zero(t, c.calls)
}

func TestCatalogSendUsesExistingParsedAttachmentAdmission(t *testing.T) {
	s, ctx, chat, a, _, _ := catalogAdmissionFixture(t)
	count, err := s.validateParsedAttachments(ctx, &chat, []messageAttachment{a}, "gif")
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestCatalogOrdinaryClassificationPreservesOriginChatACLAndRejectsForgedAttribution(t *testing.T) {
	s, ctx, chat, a, f, c := catalogAdmissionFixture(t)
	f.ordinary = true
	a.Type = "image"
	foreign := uuid.New()
	s.Files = stubFiles{byID: map[string]*filev1.FileMetadata{a.FileID: {Id: a.FileID, Status: "ready", FileType: "image", ScanResult: "clean", Chat: &chatv1.ChatRef{Id: foreign.String()}}}}
	_, err := s.validateParsedAttachments(ctx, &chat, []messageAttachment{a}, "")
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Zero(t, c.calls)
	a.Provider = "forged"
	_, err = s.validateCatalogAttachments(ctx, &chat, []messageAttachment{a}, "")
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCatalogMissingTypedAuthorityFailsClosed(t *testing.T) {
	s, ctx, chat, a, _, _ := catalogAdmissionFixture(t)
	var absent *catalogFileFake
	s.CatalogAssets.Files = absent
	_, err := s.validateCatalogAttachments(ctx, &chat, []messageAttachment{a}, "gif")
	require.Equal(t, codes.Unavailable, status.Code(err))
}
