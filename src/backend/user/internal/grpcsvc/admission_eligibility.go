package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"math"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/user/internal/store"
)

type admissionProfileReader interface {
	GetSdkProfileEligibility(context.Context, uuid.UUID, uuid.UUID) (store.SdkProfileEligibility, error)
}

// AdmissionEligibilityGRPC has no mutation or SDK conversion capability. It is
// registered only on the dedicated Auth admission mutual-TLS listener.
type AdmissionEligibilityGRPC struct {
	userv1.UnimplementedUserServiceServer
	Profiles admissionProfileReader
}

func (s *AdmissionEligibilityGRPC) GetSpaceAdmissionProfileEligibility(ctx context.Context, req *userv1.GetSpaceAdmissionProfileEligibilityRequest) (*userv1.GetSpaceAdmissionProfileEligibilityResponse, error) {
	p, ok := principal.FromContext(ctx)
	hash, hashErr := principal.RequestHash(req)
	if !ok || hashErr != nil || p.Kind != "service" || p.Issuer != "auth" || p.Subject != "service:auth" ||
		p.Audience != "user" || p.RPC != userv1.UserService_GetSpaceAdmissionProfileEligibility_FullMethodName || p.RequestHash != hash ||
		p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return nil, status.Error(codes.Unauthenticated, "invalid admission principal")
	}
	account, err := canonicalSDKUUID(req.GetAccountId())
	if err != nil {
		return nil, err
	}
	profile, err := canonicalSDKUUID(req.GetProfileId())
	if err != nil {
		return nil, err
	}
	if len(req.GetObservationBindingSha256()) != 32 {
		return nil, status.Error(codes.InvalidArgument, "invalid observation binding")
	}
	if s == nil || s.Profiles == nil {
		return nil, status.Error(codes.Unavailable, "profile authority unavailable")
	}
	current, err := s.Profiles.GetSdkProfileEligibility(ctx, account, profile)
	if errors.Is(err, store.ErrSdkProfileMissing) {
		return nil, status.Error(codes.PermissionDenied, "profile ineligible")
	}
	if err != nil || current.Revision == 0 || current.Revision > math.MaxInt64 || current.AccountID != account || current.ProfileID != profile {
		return nil, status.Error(codes.Unavailable, "profile authority unavailable")
	}
	return &userv1.GetSpaceAdmissionProfileEligibilityResponse{
		AccountId: account.String(), ProfileId: profile.String(), ObservationBindingSha256: append([]byte(nil), req.GetObservationBindingSha256()...),
		ProfileRevision: current.Revision, Deleted: current.Deleted, Frozen: current.Frozen,
	}, nil
}

func RegisterAdmissionEligibilityServer(server grpc.ServiceRegistrar, profiles *store.ProfileStore) {
	desc := userv1.UserService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range userv1.UserService_ServiceDesc.Methods {
		if method.MethodName == "GetSpaceAdmissionProfileEligibility" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, &AdmissionEligibilityGRPC{Profiles: profiles})
}

// Compile-time guard: this endpoint consumes only protobuf requests, as the
// principal hash helper also rejects unknown fields recursively.
var _ proto.Message = (*userv1.GetSpaceAdmissionProfileEligibilityRequest)(nil)
