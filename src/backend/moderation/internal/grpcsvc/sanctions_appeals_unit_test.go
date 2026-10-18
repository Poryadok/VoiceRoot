package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	moderationv1 "voice.app/voice/moderation/v1"
	"voice/backend/moderation/internal/store"
)

// These tests exercise handler preconditions without PostgreSQL. Empty stores
// deliberately have no pool: reaching persistence cannot satisfy an expected
// authentication or validation error. Successful mutations remain covered by
// the PostgreSQL-backed sanctions/appeals integration tests.
type moderationHandlerCall func(context.Context, *ModerationGRPC) (any, error)

func TestSanctionsAppeals_StoresRequired(t *testing.T) {
	calls := map[string]moderationHandlerCall{
		"apply": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.ApplySanction(ctx, nil)
		},
		"revoke": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.RevokeSanction(ctx, nil)
		},
		"account sanctions": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.GetAccountSanctions(ctx, nil)
		},
		"active sanction": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.GetActiveSanction(ctx, nil)
		},
		"submit appeal": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.SubmitAppeal(ctx, nil)
		},
		"review appeal": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.ReviewAppeal(ctx, nil)
		},
		"get appeal": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.GetAppeal(ctx, nil)
		},
		"list appeals": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.ListAppeals(ctx, nil)
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			for _, service := range []*ModerationGRPC{nil, {}} {
				response, err := call(context.Background(), service)
				require.Equal(t, codes.FailedPrecondition, status.Code(err))
				require.Nil(t, response)
			}
		})
	}
	for _, service := range []*ModerationGRPC{
		{Appeals: &store.AppealStore{}},
		{Sanctions: &store.SanctionStore{}},
	} {
		_, err := service.SubmitAppeal(context.Background(), nil)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
		_, err = service.ReviewAppeal(context.Background(), nil)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	}
}

func TestSanctionsAppeals_ModeratorRequiredBeforePersistence(t *testing.T) {
	calls := map[string]moderationHandlerCall{
		"apply": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.ApplySanction(ctx, &moderationv1.ApplySanctionRequest{})
		},
		"revoke": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.RevokeSanction(ctx, &moderationv1.RevokeSanctionRequest{})
		},
		"account sanctions": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.GetAccountSanctions(ctx, &moderationv1.GetAccountSanctionsRequest{})
		},
		"active sanction": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.GetActiveSanction(ctx, &moderationv1.GetActiveSanctionRequest{})
		},
		"review appeal": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.ReviewAppeal(ctx, &moderationv1.ReviewAppealRequest{})
		},
		"get appeal": func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.GetAppeal(ctx, &moderationv1.GetAppealRequest{})
		},
	}
	contexts := []struct {
		name string
		md   metadata.MD
		code codes.Code
	}{
		{"external", nil, codes.PermissionDenied},
		{"profile alone", metadata.Pairs("x-voice-profile-id", uuid.NewString()), codes.PermissionDenied},
		{"internal without profile", metadata.Pairs("x-voice-internal", "true"), codes.Unauthenticated},
		{"internal invalid profile", metadata.Pairs("x-voice-internal", "true", "x-voice-profile-id", "invalid"), codes.Unauthenticated},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			for _, tc := range contexts {
				t.Run(tc.name, func(t *testing.T) {
					service := &ModerationGRPC{Sanctions: &store.SanctionStore{}, Appeals: &store.AppealStore{}}
					response, err := call(metadata.NewIncomingContext(context.Background(), tc.md), service)
					require.Equal(t, tc.code, status.Code(err))
					require.Nil(t, response)
				})
			}
		})
	}
}

func TestSanctionsAppeals_InvalidRequestBeforePersistence(t *testing.T) {
	id := uuid.NewString()
	cases := []struct {
		name string
		call moderationHandlerCall
	}{
		{"apply invalid account", func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.ApplySanction(ctx, &moderationv1.ApplySanctionRequest{TargetAccountId: "invalid", Type: "warning", Reason: "spam"})
		}},
		{"apply unknown type", func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.ApplySanction(ctx, &moderationv1.ApplySanctionRequest{TargetAccountId: id, Type: "unknown", Reason: "spam"})
		}},
		{"apply blank reason", func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.ApplySanction(ctx, &moderationv1.ApplySanctionRequest{TargetAccountId: id, Type: "perm_ban", Reason: " \t\n"})
		}},
		{"apply invalid report", func(ctx context.Context, s *ModerationGRPC) (any, error) {
			reportID := "invalid"
			return s.ApplySanction(ctx, &moderationv1.ApplySanctionRequest{TargetAccountId: id, Type: "warning", Reason: "spam", ReportId: &reportID})
		}},
		{"revoke invalid sanction", func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.RevokeSanction(ctx, &moderationv1.RevokeSanctionRequest{SanctionId: "invalid"})
		}},
		{"account sanctions invalid account", func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.GetAccountSanctions(ctx, &moderationv1.GetAccountSanctionsRequest{AccountId: "invalid"})
		}},
		{"active sanction invalid account", func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.GetActiveSanction(ctx, &moderationv1.GetActiveSanctionRequest{AccountId: "invalid"})
		}},
		{"review invalid appeal", func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.ReviewAppeal(ctx, &moderationv1.ReviewAppealRequest{AppealId: "invalid", Status: "approved"})
		}},
		{"review pending status", func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.ReviewAppeal(ctx, &moderationv1.ReviewAppealRequest{AppealId: id, Status: "pending"})
		}},
		{"get invalid appeal", func(ctx context.Context, s *ModerationGRPC) (any, error) {
			return s.GetAppeal(ctx, &moderationv1.GetAppealRequest{AppealId: "invalid"})
		}},
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-internal", "true", "x-voice-profile-id", uuid.NewString()))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := &ModerationGRPC{Sanctions: &store.SanctionStore{}, Appeals: &store.AppealStore{}}
			response, err := tc.call(ctx, service)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
			require.Nil(t, response)
		})
	}
}

func TestSubmitAppeal_AccountAndRequestRequiredBeforePersistence(t *testing.T) {
	cases := []struct {
		name string
		md   metadata.MD
		req  *moderationv1.SubmitAppealRequest
		code codes.Code
	}{
		{"missing account", nil, &moderationv1.SubmitAppealRequest{}, codes.Unauthenticated},
		{"profile is not account", metadata.Pairs("x-voice-profile-id", uuid.NewString()), &moderationv1.SubmitAppealRequest{}, codes.Unauthenticated},
		{"invalid account", metadata.Pairs("x-voice-user-id", "invalid"), &moderationv1.SubmitAppealRequest{}, codes.Unauthenticated},
		{"invalid sanction", metadata.Pairs("x-voice-user-id", uuid.NewString()), &moderationv1.SubmitAppealRequest{SanctionId: "invalid", Reason: "review"}, codes.InvalidArgument},
		{"blank reason", metadata.Pairs("x-voice-user-id", uuid.NewString()), &moderationv1.SubmitAppealRequest{SanctionId: uuid.NewString(), Reason: " \t\n"}, codes.InvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := &ModerationGRPC{Sanctions: &store.SanctionStore{}, Appeals: &store.AppealStore{}}
			response, err := service.SubmitAppeal(metadata.NewIncomingContext(context.Background(), tc.md), tc.req)
			require.Equal(t, tc.code, status.Code(err))
			require.Nil(t, response)
		})
	}
}

func TestListAppeals_ExternalDeniedBeforePersistence(t *testing.T) {
	service := &ModerationGRPC{Appeals: &store.AppealStore{}}
	response, err := service.ListAppeals(context.Background(), &moderationv1.ListAppealsRequest{})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Nil(t, response)
}
