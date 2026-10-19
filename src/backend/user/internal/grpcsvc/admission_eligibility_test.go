package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"testing"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/user/internal/store"
)

type admissionReaderFunc func(context.Context, uuid.UUID, uuid.UUID) (store.SdkProfileEligibility, error)

func (f admissionReaderFunc) GetSdkProfileEligibility(ctx context.Context, a, p uuid.UUID) (store.SdkProfileEligibility, error) {
	return f(ctx, a, p)
}

func TestAdmissionEligibility_ExactOwnerBindingAndGuestWithoutSdkConsent(t *testing.T) {
	a, p := uuid.New(), uuid.New()
	req := &userv1.GetSpaceAdmissionProfileEligibilityRequest{AccountId: a.String(), ProfileId: p.String(), ObservationBindingSha256: make([]byte, 32)}
	service := &AdmissionEligibilityGRPC{Profiles: admissionReaderFunc(func(_ context.Context, account, profile uuid.UUID) (store.SdkProfileEligibility, error) {
		require.Equal(t, a, account)
		require.Equal(t, p, profile)
		return store.SdkProfileEligibility{AccountID: a, ProfileID: p, Revision: 3}, nil
	})}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	valid := principal.Principal{Kind: "service", Issuer: "auth", Subject: "service:auth", Audience: "user", RPC: userv1.UserService_GetSpaceAdmissionProfileEligibility_FullMethodName, RequestHash: hash}
	ctx := principal.WithVerified(context.Background(), valid)
	result, err := service.GetSpaceAdmissionProfileEligibility(ctx, req)
	require.NoError(t, err)
	require.Equal(t, uint64(3), result.ProfileRevision)
	require.Equal(t, req.ObservationBindingSha256, result.ObservationBindingSha256)
	for _, change := range []func(*principal.Principal){func(p *principal.Principal) { p.Issuer = "space" }, func(p *principal.Principal) { p.RPC = userv1.UserService_GetSdkProfileEligibility_FullMethodName }, func(p *principal.Principal) { p.RequestHash = "wrong" }} {
		bad := valid
		change(&bad)
		_, err = service.GetSpaceAdmissionProfileEligibility(principal.WithVerified(context.Background(), bad), req)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	}
	altered := proto.Clone(req).(*userv1.GetSpaceAdmissionProfileEligibilityRequest)
	altered.ObservationBindingSha256[0] = 1
	_, err = service.GetSpaceAdmissionProfileEligibility(ctx, altered)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = service.GetSpaceAdmissionProfileEligibility(context.Background(), req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestAdmissionEligibility_NoMissingForeignOrInvalidRevisionPositive(t *testing.T) {
	a, p := uuid.New(), uuid.New()
	req := &userv1.GetSpaceAdmissionProfileEligibilityRequest{AccountId: a.String(), ProfileId: p.String(), ObservationBindingSha256: make([]byte, 32)}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	ctx := principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "auth", Subject: "service:auth", Audience: "user", RPC: userv1.UserService_GetSpaceAdmissionProfileEligibility_FullMethodName, RequestHash: hash})
	for _, tc := range []struct {
		row  store.SdkProfileEligibility
		err  error
		want codes.Code
	}{
		{err: store.ErrSdkProfileMissing, want: codes.PermissionDenied},
		{row: store.SdkProfileEligibility{AccountID: a, ProfileID: p}, want: codes.Unavailable},
		{row: store.SdkProfileEligibility{AccountID: uuid.New(), ProfileID: p, Revision: 2}, want: codes.Unavailable},
	} {
		service := &AdmissionEligibilityGRPC{Profiles: admissionReaderFunc(func(context.Context, uuid.UUID, uuid.UUID) (store.SdkProfileEligibility, error) {
			return tc.row, tc.err
		})}
		_, err = service.GetSpaceAdmissionProfileEligibility(ctx, req)
		require.Equal(t, tc.want, status.Code(err))
	}
}
