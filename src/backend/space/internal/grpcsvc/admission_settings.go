package grpcsvc

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
)

func (s *admissionServer) SetEntryPolicy(ctx context.Context, original *spacev1.SetEntryPolicyRequest) (*spacev1.SetEntryPolicyResponse, error) {
	if original == nil {
		return nil, status.Error(codes.InvalidArgument, "request required")
	}
	request := proto.Clone(original).(*spacev1.SetEntryPolicyRequest)
	if s == nil || s.store == nil {
		return nil, status.Error(codes.Unavailable, "settings storage unavailable")
	}
	p, err := s.store.PrepareEntryPolicySet(ctx, request)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	ack, done, err := s.store.ReplayEntryPolicySet(ctx, p)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	if done {
		return ack, nil
	}
	if s.deps.Subjects == nil || s.deps.Settings == nil {
		return nil, status.Error(codes.Unavailable, "settings authority unavailable")
	}
	expected, err := p.OwnerRequest()
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	subject, err := s.deps.Subjects.Observe(ctx, expected)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	settings, err := s.deps.Settings.Observe(ctx, subject, expected)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	decision, err := p.Seal(subject, settings)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	ack, err = s.store.CommitEntryPolicySet(ctx, decision)
	if err != nil {
		return nil, mapAdmissionError(err)
	}
	return ack, nil
}
