package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/subscription/internal/billing"
	"voice/backend/subscription/internal/catalog"
	"voice/backend/subscription/internal/limits"
	"voice/backend/subscription/internal/store"

	subscriptionv1 "voice.app/voice/subscription/v1"
	spacev1 "voice.app/voice/space/v1"
)

// SubscriptionGRPC implements voice.subscription.v1.SubscriptionService.
type SubscriptionGRPC struct {
	subscriptionv1.UnimplementedSubscriptionServiceServer
	Store   *store.SubscriptionStore
	Catalog *catalog.ProductCatalog
	// UserProfiles optional; when set, downgrade delegates profile freeze to User service.
	UserProfiles UserProfileDowngradeClient
	Analytics    interface {
		Publish(ctx context.Context, subject, sourceService, eventType string, props map[string]any) error
	}
}

// UserProfileDowngradeClient applies profile selection on subscription downgrade.
type UserProfileDowngradeClient interface {
	ApplyDowngradeProfiles(ctx context.Context, accountID uuid.UUID, keptProfileIDs []uuid.UUID) error
}

// NewSubscriptionGRPC constructs the gRPC service.
func NewSubscriptionGRPC(st *store.SubscriptionStore) *SubscriptionGRPC {
	cat := catalog.Default()
	return &SubscriptionGRPC{Store: st, Catalog: cat}
}

func (s *SubscriptionGRPC) GetSubscription(ctx context.Context, req *subscriptionv1.GetSubscriptionRequest) (*subscriptionv1.GetSubscriptionResponse, error) {
	accountID, err := parseAccountID(req.GetAccountId())
	if err != nil {
		return nil, err
	}
	row, err := s.Store.GetSubscriptionByAccountID(ctx, accountID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return &subscriptionv1.GetSubscriptionResponse{}, nil
	}
	return &subscriptionv1.GetSubscriptionResponse{Subscription: subscriptionToProto(row)}, nil
}

func (s *SubscriptionGRPC) CreateCheckoutSession(ctx context.Context, req *subscriptionv1.CreateCheckoutSessionRequest) (*subscriptionv1.CreateCheckoutSessionResponse, error) {
	plan := strings.TrimSpace(req.GetPlan())
	period := strings.TrimSpace(req.GetBillingPeriod())
	if _, ok := s.Catalog.PriceCents(plan, period); !ok {
		return nil, status.Error(codes.InvalidArgument, "unknown plan or billing period")
	}
	sessionID := uuid.NewString()
	checkoutURL := strings.TrimSpace(req.GetSuccessUrl())
	if checkoutURL == "" {
		checkoutURL = "https://checkout.paddle.test/session/" + sessionID
	}
	return &subscriptionv1.CreateCheckoutSessionResponse{
		CheckoutResponse: &subscriptionv1.CheckoutResponse{
			CheckoutUrl: checkoutURL,
			SessionId:   sessionID,
		},
	}, nil
}

func (s *SubscriptionGRPC) CancelSubscription(ctx context.Context, req *subscriptionv1.CancelSubscriptionRequest) (*subscriptionv1.CancelSubscriptionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "cancel subscription not implemented")
}

func (s *SubscriptionGRPC) ResumeSubscription(ctx context.Context, req *subscriptionv1.ResumeSubscriptionRequest) (*subscriptionv1.ResumeSubscriptionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "resume subscription not implemented")
}

func (s *SubscriptionGRPC) GetSpaceSubscription(ctx context.Context, req *subscriptionv1.GetSpaceSubscriptionRequest) (*subscriptionv1.GetSpaceSubscriptionResponse, error) {
	spaceID, err := parseUUIDField("space.id", req.GetSpace().GetId())
	if err != nil {
		return nil, err
	}
	row, err := s.Store.GetSpaceSubscriptionBySpaceID(ctx, spaceID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return &subscriptionv1.GetSpaceSubscriptionResponse{}, nil
	}
	return &subscriptionv1.GetSpaceSubscriptionResponse{SpaceSubscription: spaceSubscriptionToProto(row)}, nil
}

func (s *SubscriptionGRPC) CreateSpaceCheckoutSession(ctx context.Context, req *subscriptionv1.CreateSpaceCheckoutSessionRequest) (*subscriptionv1.CreateSpaceCheckoutSessionResponse, error) {
	if req.GetSpace() == nil || strings.TrimSpace(req.GetSpace().GetId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "space is required")
	}
	sessionID := uuid.NewString()
	checkoutURL := strings.TrimSpace(req.GetSuccessUrl())
	if checkoutURL == "" {
		checkoutURL = "https://checkout.paddle.test/space/" + sessionID
	}
	return &subscriptionv1.CreateSpaceCheckoutSessionResponse{
		CheckoutResponse: &subscriptionv1.CheckoutResponse{
			CheckoutUrl: checkoutURL,
			SessionId:   sessionID,
		},
	}, nil
}

func (s *SubscriptionGRPC) GetLimits(ctx context.Context, req *subscriptionv1.GetLimitsRequest) (*subscriptionv1.GetLimitsResponse, error) {
	accountID, err := parseAccountID(req.GetAccountId())
	if err != nil {
		return nil, err
	}
	tier, err := s.Store.EffectiveAccountTier(ctx, accountID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &subscriptionv1.GetLimitsResponse{
		Limits: &subscriptionv1.Limits{LimitsJson: limits.ForAccount(tier)},
	}, nil
}

func (s *SubscriptionGRPC) CheckLimit(ctx context.Context, req *subscriptionv1.CheckLimitRequest) (*subscriptionv1.CheckLimitResponse, error) {
	accountID, err := parseAccountID(req.GetAccountId())
	if err != nil {
		return nil, err
	}
	switch strings.TrimSpace(req.GetLimitName()) {
	case "space_member_count":
		hasPro, err := s.Store.HasActiveSpaceProForPurchaser(ctx, accountID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if !hasPro {
			return &subscriptionv1.CheckLimitResponse{Allowed: false, Remaining: 0}, nil
		}
		cap := limits.SpaceMemberCap(true)
		return &subscriptionv1.CheckLimitResponse{Allowed: true, Remaining: cap}, nil
	default:
		return nil, status.Error(codes.InvalidArgument, "unknown limit_name")
	}
}

func (s *SubscriptionGRPC) HandlePaddleWebhook(ctx context.Context, req *subscriptionv1.HandlePaddleWebhookRequest) (*subscriptionv1.HandlePaddleWebhookResponse, error) {
	rawBody := req.GetRawBody()
	if err := billing.VerifySignature(rawBody, req.GetSignature()); err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	ev, err := billing.ParseWebhook(rawBody)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	details, _ := json.Marshal(ev)

	switch ev.EventType {
	case "subscription.activated":
		plan := strings.TrimSpace(ev.Data.CustomData["plan"])
		switch plan {
		case "premium":
			accountID, err := billing.AccountIDFromCustomData(ev.Data.CustomData)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			if _, err := s.Store.ActivatePremium(ctx, accountID, ev.EventID, details); err != nil {
				if errors.Is(err, store.ErrDuplicateBillingEvent) {
					return &subscriptionv1.HandlePaddleWebhookResponse{}, nil
				}
				return nil, status.Error(codes.Internal, err.Error())
			}
			s.publishPaymentEvent(ctx, "analytics.subscription.payment_success", "payment_success", accountID.String(), plan, ev.EventID)
		case "space_pro":
			spaceID, purchaserID, err := billing.SpaceProFromCustomData(ev.Data.CustomData)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			if _, err := s.Store.ActivateSpacePro(ctx, spaceID, purchaserID, ev.EventID, details); err != nil {
				if errors.Is(err, store.ErrDuplicateBillingEvent) {
					return &subscriptionv1.HandlePaddleWebhookResponse{}, nil
				}
				return nil, status.Error(codes.Internal, err.Error())
			}
			s.publishPaymentEvent(ctx, "analytics.subscription.payment_success", "payment_success", purchaserID.String(), plan, ev.EventID)
		default:
			return nil, status.Error(codes.InvalidArgument, "unknown plan in custom_data")
		}
	case "subscription.payment_failed":
		accountID, err := billing.AccountIDFromCustomData(ev.Data.CustomData)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if _, err := s.Store.MarkPaymentFailed(ctx, accountID, ev.EventID, details); err != nil {
			if errors.Is(err, store.ErrDuplicateBillingEvent) {
				return &subscriptionv1.HandlePaddleWebhookResponse{}, nil
			}
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, status.Error(codes.NotFound, "subscription not found")
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		s.publishPaymentEvent(ctx, "analytics.subscription.payment_failed", "payment_failed", accountID.String(), "", ev.EventID)
	default:
		return &subscriptionv1.HandlePaddleWebhookResponse{}, nil
	}
	return &subscriptionv1.HandlePaddleWebhookResponse{}, nil
}

func (s *SubscriptionGRPC) publishPaymentEvent(ctx context.Context, subject, eventType, accountID, plan, providerEventID string) {
	if s == nil || s.Analytics == nil {
		return
	}
	props := map[string]any{
		"account_id":        accountID,
		"provider_event_id": providerEventID,
	}
	if plan != "" {
		props["plan"] = plan
	}
	_ = s.Analytics.Publish(ctx, subject, "subscription", eventType, props)
}

func (s *SubscriptionGRPC) HandleCloudPaymentsWebhook(ctx context.Context, req *subscriptionv1.HandleCloudPaymentsWebhookRequest) (*subscriptionv1.HandleCloudPaymentsWebhookResponse, error) {
	return nil, status.Error(codes.Unimplemented, "cloudpayments webhook not implemented")
}

func (s *SubscriptionGRPC) GetBillingHistory(ctx context.Context, req *subscriptionv1.GetBillingHistoryRequest) (*subscriptionv1.GetBillingHistoryResponse, error) {
	return &subscriptionv1.GetBillingHistoryResponse{
		BillingHistoryList: &subscriptionv1.BillingHistoryList{},
	}, nil
}

func (s *SubscriptionGRPC) ApplyDowngradeProfiles(ctx context.Context, req *subscriptionv1.ApplyDowngradeProfilesRequest) (*subscriptionv1.ApplyDowngradeProfilesResponse, error) {
	accountID, err := parseAccountID(req.GetAccountId())
	if err != nil {
		return nil, err
	}
	kept := make([]uuid.UUID, 0, len(req.GetKeptProfileIds()))
	for _, raw := range req.GetKeptProfileIds() {
		id, err := parseUUIDField("kept_profile_id", raw)
		if err != nil {
			return nil, err
		}
		kept = append(kept, id)
	}
	if s.UserProfiles != nil {
		if err := s.UserProfiles.ApplyDowngradeProfiles(ctx, accountID, kept); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &subscriptionv1.ApplyDowngradeProfilesResponse{KeptProfileIds: req.GetKeptProfileIds()}, nil
}

func subscriptionToProto(row *store.SubscriptionRow) *subscriptionv1.Subscription {
	out := &subscriptionv1.Subscription{
		Id:                     row.ID.String(),
		AccountId:              row.AccountID.String(),
		Plan:                   row.Plan,
		BillingPeriod:          row.BillingPeriod,
		Status:                 row.Status,
		Provider:               row.Provider,
		ProviderSubscriptionId: row.ProviderSubscriptionID,
		CurrentPeriodStart:     timestamppb.New(row.CurrentPeriodStart),
		CurrentPeriodEnd:       timestamppb.New(row.CurrentPeriodEnd),
	}
	if row.GracePeriodEnd != nil {
		out.GracePeriodEnd = timestamppb.New(*row.GracePeriodEnd)
	}
	if row.CancelledAt != nil {
		out.CancelledAt = timestamppb.New(*row.CancelledAt)
	}
	return out
}

func spaceSubscriptionToProto(row *store.SpaceSubscriptionRow) *subscriptionv1.SpaceSubscription {
	return &subscriptionv1.SpaceSubscription{
		Id:                 row.ID.String(),
		Space:              &spacev1.SpaceRef{Id: row.SpaceID.String()},
		PurchaserAccountId: row.PurchaserAccountID.String(),
		Plan:               row.Plan,
		BillingPeriod:      row.BillingPeriod,
		Status:             row.Status,
		Provider:           row.Provider,
		CurrentPeriodStart: timestamppb.New(row.CurrentPeriodStart),
		CurrentPeriodEnd:   timestamppb.New(row.CurrentPeriodEnd),
	}
}

func parseAccountID(raw string) (uuid.UUID, error) {
	id, err := parseUUIDField("account_id", raw)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func parseUUIDField(field, raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s is required", field)
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid %s", field)
	}
	return id, nil
}
