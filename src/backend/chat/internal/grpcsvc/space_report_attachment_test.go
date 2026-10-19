package grpcsvc

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type reportAttachmentStub struct {
	scopeAuthorityStub
	reads    int
	response *chatv1.ReadSpaceReportAttachmentResponse
	readErr  error
}

func (s *reportAttachmentStub) ReadSpaceReportAttachment(_ context.Context, chat, space uuid.UUID, saved uint64) (*chatv1.ReadSpaceReportAttachmentResponse, error) {
	s.reads++
	if s.response != nil {
		return proto.Clone(s.response).(*chatv1.ReadSpaceReportAttachmentResponse), s.readErr
	}
	return &chatv1.ReadSpaceReportAttachmentResponse{ProtocolVersion: 1, ChatId: chat.String(), ExpectedSpaceId: space.String(), SavedAttachmentGeneration: saved, Disposition: chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNKNOWN}, s.readErr
}

func TestSpaceReportAttachmentBoundary(t *testing.T) {
	req := &chatv1.ReadSpaceReportAttachmentRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), ChatId: uuid.NewString(), ExpectedSpaceId: uuid.NewString(), SavedAttachmentGeneration: 7}
	st := &reportAttachmentStub{}
	svc := &SpaceChatScopeAuthorityGRPC{Store: st}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	p := principal.Principal{Kind: "service", Issuer: "messaging", Subject: "service:messaging", Audience: "chat", RPC: chatv1.ChatScopeAuthorityService_ReadSpaceReportAttachment_FullMethodName, RequestID: req.OperationId, RequestHash: hash}
	call := func(p principal.Principal, r *chatv1.ReadSpaceReportAttachmentRequest) (*chatv1.ReadSpaceReportAttachmentResponse, error) {
		return svc.ReadSpaceReportAttachment(principal.WithVerified(context.Background(), p), r)
	}
	response, err := call(p, req)
	require.NoError(t, err)
	require.Equal(t, req.OperationId, response.OperationId)
	require.Equal(t, chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNKNOWN, response.Disposition)
	before := st.reads
	for name, mutate := range map[string]func(*principal.Principal){
		"role":     func(p *principal.Principal) { p.Issuer = "role"; p.Subject = "service:role" },
		"space":    func(p *principal.Principal) { p.Issuer = "space"; p.Subject = "service:space" },
		"subject":  func(p *principal.Principal) { p.Subject = "service:role" },
		"profile":  func(p *principal.Principal) { p.ProfileID = uuid.NewString() },
		"account":  func(p *principal.Principal) { p.AccountID = uuid.NewString() },
		"epoch":    func(p *principal.Principal) { p.SessionEpoch = 1 },
		"audience": func(p *principal.Principal) { p.Audience = "messaging" },
		"rpc": func(p *principal.Principal) {
			p.RPC = chatv1.ChatScopeAuthorityService_GetSpaceChatScopeAuthority_FullMethodName
		},
		"request": func(p *principal.Principal) { p.RequestID = uuid.NewString() },
		"hash":    func(p *principal.Principal) { p.RequestHash = "bad" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := p
			mutate(&changed)
			_, err := call(changed, req)
			require.Error(t, err)
		})
	}
	_, err = svc.ReadSpaceReportAttachment(context.Background(), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	for _, mutate := range []func(*chatv1.ReadSpaceReportAttachmentRequest){
		func(r *chatv1.ReadSpaceReportAttachmentRequest) { r.ProtocolVersion = 2 },
		func(r *chatv1.ReadSpaceReportAttachmentRequest) { r.ChatId = "bad" },
		func(r *chatv1.ReadSpaceReportAttachmentRequest) { r.ExpectedSpaceId = uuid.Nil.String() },
		func(r *chatv1.ReadSpaceReportAttachmentRequest) { r.OperationId = "bad" },
		func(r *chatv1.ReadSpaceReportAttachmentRequest) { r.SavedAttachmentGeneration = 0 },
		func(r *chatv1.ReadSpaceReportAttachmentRequest) { r.SavedAttachmentGeneration = math.MaxUint64 },
		func(r *chatv1.ReadSpaceReportAttachmentRequest) { r.ProtoReflect().SetUnknown([]byte{0x78, 1}) },
	} {
		changed := proto.Clone(req).(*chatv1.ReadSpaceReportAttachmentRequest)
		mutate(changed)
		_, err = call(p, changed)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	changed := proto.Clone(req).(*chatv1.ReadSpaceReportAttachmentRequest)
	changed.SavedAttachmentGeneration++
	_, err = call(p, changed)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Equal(t, before, st.reads)
	st.readErr = errors.New("private SQL detail")
	_, err = call(p, req)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.NotContains(t, err.Error(), "private SQL detail")
	st.readErr = nil
	for _, disposition := range []chatv1.SpaceReportAttachmentDisposition{chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNKNOWN, chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED, chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_RETIRED} {
		st.response = &chatv1.ReadSpaceReportAttachmentResponse{ProtocolVersion: 1, ChatId: req.ChatId, ExpectedSpaceId: req.ExpectedSpaceId, SavedAttachmentGeneration: 7, Disposition: disposition}
		if disposition == chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_RETIRED {
			st.response.Reason = chatv1.SpaceReportAttachmentRetirementReason_SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_MOVED
			st.response.OwnerEventId = uuid.NewString()
			st.response.OwnerRevision = 9
		}
		_, err = call(p, req)
		require.NoError(t, err)
		valid := proto.Clone(st.response).(*chatv1.ReadSpaceReportAttachmentResponse)
		for _, mutate := range []func(*chatv1.ReadSpaceReportAttachmentResponse){func(r *chatv1.ReadSpaceReportAttachmentResponse) { r.ChatId = uuid.NewString() }, func(r *chatv1.ReadSpaceReportAttachmentResponse) { r.Disposition = 99 }, func(r *chatv1.ReadSpaceReportAttachmentResponse) {
			r.OwnerRevision = 0
			r.OwnerEventId = "not-an-event"
		}} {
			st.response = proto.Clone(valid).(*chatv1.ReadSpaceReportAttachmentResponse)
			mutate(st.response)
			_, err = call(p, req)
			require.Equal(t, codes.Unavailable, status.Code(err))
		}
	}
}
