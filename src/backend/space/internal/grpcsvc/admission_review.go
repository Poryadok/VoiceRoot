package grpcsvc

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/store"
)

func (s *admissionServer) authorizeJoinReviewRead(ctx context.Context, request proto.Message) (*store.VerifiedJoinReviewRead, error) {
	p, err := store.PrepareJoinReviewRead(ctx, request)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	if s == nil || s.store == nil || s.deps.Subjects == nil || s.deps.Reviewers == nil {
		return nil, status.Error(codes.Unavailable, "review authority unavailable")
	}
	expected, err := p.OwnerRequest()
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	subject, err := s.deps.Subjects.Observe(ctx, expected)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	permission, err := s.deps.Reviewers.Observe(ctx, subject, expected)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	decision, err := p.Seal(subject, permission)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	return decision, nil
}
func (s *admissionServer) ListJoinRequests(ctx context.Context, request *spacev1.ListJoinRequestsRequest) (*spacev1.ListJoinRequestsResponse, error) {
	d, err := s.authorizeJoinReviewRead(ctx, request)
	if err != nil {
		return nil, err
	}
	result, err := s.store.ListJoinRequestsVerified(ctx, d)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	return result, nil
}
func (s *admissionServer) GetJoinRequest(ctx context.Context, request *spacev1.GetJoinRequestRequest) (*spacev1.GetJoinRequestResponse, error) {
	d, err := s.authorizeJoinReviewRead(ctx, request)
	if err != nil {
		return nil, err
	}
	result, err := s.store.GetJoinRequestVerified(ctx, d)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	return result, nil
}
