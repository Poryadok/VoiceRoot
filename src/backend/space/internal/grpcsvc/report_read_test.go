package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/store"
)

type reportReadStore struct{ calls int }

func (s *reportReadStore) ReadProtectedSpaceReportReadContext(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*store.SpaceReportReadContext, error) {
	s.calls++
	return &store.SpaceReportReadContext{Eligible: true}, nil
}

func TestReportReadRejectsUnverifiedSubjectBeforeStore(t *testing.T) {
	st := &reportReadStore{}
	service := &ReportReadGRPC{Store: st}
	req := &spacev1.ResolveSpaceReportReadContextRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), SpaceId: uuid.NewString(), ViewerProfileId: uuid.NewString(), Chat: &chatv1.ChatRef{Id: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_GROUP.Enum()}}
	contexts := []context.Context{context.Background(), metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-profile-id", req.ViewerProfileId, "x-internal-caller", "messaging")), principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "messaging", Subject: "service:messaging", Audience: "space"})}
	for _, ctx := range contexts {
		response, err := service.ResolveSpaceReportReadContext(ctx, req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Nil(t, response)
	}
	require.Zero(t, st.calls, "unsigned/generic context cannot disclose even valid local facts")
}
