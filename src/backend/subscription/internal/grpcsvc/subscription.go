package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc"
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
		// SpaceEntitlements optional; syncs space_db entitlement cache after Space Pro webhook.
		SpaceEntitlements SpaceProSyncClient
		Analytics    interface {
			Publish(ctx context.Context, subject, sourceService, eventType string, props map[string]any) error
			PublishWithAccount(ctx context.Context, subject, sourceService, eventType, accountID string, props map[string]any) error
		}
		DomainEvents interface {
			PublishPlanStarted(ctx context.Context, accountID, plan string) error
			PublishPlanCancelled(ctx context.Context, accountID, plan string) error
			PublishPlanExpired(ctx context.Context, accountID, plan string) error
			PublishDowngrade(ctx context.Context, accountID, plan string) error
			PublishPaymentSuccess(ctx context.Context, accountID, provider string) error
			PublishPaymentFailed(ctx context.Context, accountID, provider string) error
			PublishSpaceProStarted(ctx context.Context, spaceID, purchaserAccountID string) error
			PublishSpaceProExpired(ctx context.Context, spaceID string) error
			PublishGraceReminder(ctx context.Context, accountID, plan string, day int32) error
		}
	}

	// SpaceProSyncClient mirrors SpaceService.SyncSpaceProSubscription for entitlement cache updates.
	type SpaceProSyncClient interface {
		SyncSpaceProSubscription(ctx context.Context, in *spacev1.SyncSpaceProSubscriptionRequest, opts ...grpc.CallOption) (*spacev1.SyncSpaceProSubscriptionResponse, error)
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
	subID, err := parseUUIDField("subscription_id", req.GetSubscriptionId())
	if err != nil {
		return nil, err
	}
	accountID, ok := accountIDFromIncomingMetadata(ctx)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "account_id required")
	}
	row, err := s.Store.CancelSubscriptionByID(ctx, subID, accountID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrSubscriptionNotFound):
			return nil, status.Error(codes.NotFound, "subscription not found")
		case errors.Is(err, store.ErrSubscriptionOwner):
			return nil, status.Error(codes.PermissionDenied, err.Error())
		case errors.Is(err, store.ErrSubscriptionState):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	s.publishDomainPlanCancelled(ctx, row.AccountID.String(), row.Plan)
	return &subscriptionv1.CancelSubscriptionResponse{Subscription: subscriptionToProto(row)}, nil
}

func (s *SubscriptionGRPC) ResumeSubscription(ctx context.Context, req *subscriptionv1.ResumeSubscriptionRequest) (*subscriptionv1.ResumeSubscriptionResponse, error) {
	subID, err := parseUUIDField("subscription_id", req.GetSubscriptionId())
	if err != nil {
		return nil, err
	}
	accountID, ok := accountIDFromIncomingMetadata(ctx)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "account_id required")
	}
	row, err := s.Store.ResumeSubscriptionByID(ctx, subID, accountID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrSubscriptionNotFound):
			return nil, status.Error(codes.NotFound, "subscription not found")
		case errors.Is(err, store.ErrSubscriptionOwner):
			return nil, status.Error(codes.PermissionDenied, err.Error())
		case errors.Is(err, store.ErrSubscriptionState):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &subscriptionv1.ResumeSubscriptionResponse{Subscription: subscriptionToProto(row)}, nil
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
	if req.GetScopeSpace() != nil && strings.TrimSpace(req.GetScopeSpace().GetId()) != "" {
		spaceID, err := parseUUIDField("scope_space.id", req.GetScopeSpace().GetId())
		if err != nil {
			return nil, err
		}
		hasPro, err := s.Store.HasActiveSpaceProForSpace(ctx, spaceID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		_ = accountID
		return &subscriptionv1.GetLimitsResponse{
			Limits: &subscriptionv1.Limits{LimitsJson: limits.ForSpace(hasPro)},
		}, nil
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
	_, err := parseAccountID(req.GetAccountId())
	if err != nil {
		return nil, err
	}
	switch strings.TrimSpace(req.GetLimitName()) {
	case "space_member_count":
		if req.GetScopeSpace() == nil || strings.TrimSpace(req.GetScopeSpace().GetId()) == "" {
			return nil, status.Error(codes.InvalidArgument, "scope_space is required for space_member_count")
		}
		spaceID, err := parseUUIDField("scope_space.id", req.GetScopeSpace().GetId())
		if err != nil {
			return nil, err
		}
		hasPro, err := s.Store.HasActiveSpaceProForSpace(ctx, spaceID)
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
			s.publishDomainPlanStarted(ctx, accountID.String(), plan)
			s.publishDomainPaymentSuccess(ctx, accountID.String(), "paddle")
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
			s.publishDomainSpaceProStarted(ctx, spaceID.String(), purchaserID.String())
			s.syncSpaceProCache(ctx, spaceID.String(), purchaserID.String(), "active")
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
		s.publishDomainPaymentFailed(ctx, accountID.String(), "paddle")
	case "subscription.renewed":
		if err := s.handlePremiumLifecycleWebhook(ctx, ev, details, func(accountID uuid.UUID) error {
			_, err := s.Store.RenewPremium(ctx, accountID, ev.EventID, details, billing.PeriodEndFromEvent(ev.Data))
			return err
		}); err != nil {
			return nil, err
		}
	case "subscription.cancelled":
		plan := billing.PlanFromCustomData(ev.Data.CustomData)
		if plan == "space_pro" {
			spaceID, _, err := billing.SpaceProFromCustomData(ev.Data.CustomData)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			row, err := s.Store.MarkSpaceProCancelled(ctx, spaceID, ev.EventID, details)
			if err != nil {
				if errors.Is(err, store.ErrDuplicateBillingEvent) {
					return &subscriptionv1.HandlePaddleWebhookResponse{}, nil
				}
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, status.Error(codes.NotFound, "space subscription not found")
				}
				return nil, status.Error(codes.Internal, err.Error())
			}
			if row != nil && row.Status == "cancelled" {
				s.publishDomainSpaceProExpired(ctx, spaceID.String())
				s.syncSpaceProCache(ctx, spaceID.String(), row.PurchaserAccountID.String(), "cancelled")
			}
		} else if err := s.handlePremiumLifecycleWebhook(ctx, ev, details, func(accountID uuid.UUID) error {
			_, err := s.Store.MarkSubscriptionCancelled(ctx, accountID, ev.EventID, details)
			return err
		}); err != nil {
			return nil, err
		} else if accountID, err := billing.AccountIDFromCustomData(ev.Data.CustomData); err == nil {
			s.publishDomainPlanCancelled(ctx, accountID.String(), "premium")
		}
	case "subscription.paused":
		if err := s.handlePremiumLifecycleWebhook(ctx, ev, details, func(accountID uuid.UUID) error {
			_, err := s.Store.PauseSubscription(ctx, accountID, ev.EventID, details)
			return err
		}); err != nil {
			return nil, err
		}
	case "subscription.updated":
		if strings.EqualFold(strings.TrimSpace(ev.Data.Status), "cancelled") {
			plan := billing.PlanFromCustomData(ev.Data.CustomData)
			if plan == "space_pro" {
				spaceID, _, err := billing.SpaceProFromCustomData(ev.Data.CustomData)
				if err != nil {
					return nil, status.Error(codes.InvalidArgument, err.Error())
				}
				row, err := s.Store.GetSpaceSubscriptionBySpaceID(ctx, spaceID)
				if err != nil {
					return nil, status.Error(codes.Internal, err.Error())
				}
				if row != nil {
					if _, err := s.Store.FinalizeSpaceProCancellation(ctx, row.ID); err != nil {
						return nil, status.Error(codes.Internal, err.Error())
					}
					s.publishDomainSpaceProExpired(ctx, spaceID.String())
					s.syncSpaceProCache(ctx, spaceID.String(), row.PurchaserAccountID.String(), "cancelled")
				}
			} else if err := s.handlePremiumLifecycleWebhook(ctx, ev, details, func(accountID uuid.UUID) error {
				sub, err := s.Store.GetSubscriptionByAccountID(ctx, accountID)
				if err != nil || sub == nil {
					return err
				}
				_, err = s.Store.FinalizeSubscriptionCancellation(ctx, sub.ID)
				if err != nil {
					return err
				}
				s.publishDomainPlanExpired(ctx, accountID.String(), sub.Plan)
				s.publishDomainDowngrade(ctx, accountID.String(), sub.Plan)
				return nil
			}); err != nil {
				return nil, err
			}
		}
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
		"provider_event_id": providerEventID,
	}
	if plan != "" {
		props["plan"] = plan
	}
	_ = s.Analytics.PublishWithAccount(ctx, subject, "subscription", eventType, accountID, props)
}

func (s *SubscriptionGRPC) publishDomainPlanStarted(ctx context.Context, accountID, plan string) {
	if s == nil || s.DomainEvents == nil {
		return
	}
	_ = s.DomainEvents.PublishPlanStarted(ctx, accountID, plan)
}

func (s *SubscriptionGRPC) publishDomainPaymentSuccess(ctx context.Context, accountID, provider string) {
	if s == nil || s.DomainEvents == nil {
		return
	}
	_ = s.DomainEvents.PublishPaymentSuccess(ctx, accountID, provider)
}

func (s *SubscriptionGRPC) publishDomainPaymentFailed(ctx context.Context, accountID, provider string) {
	if s == nil || s.DomainEvents == nil {
		return
	}
	_ = s.DomainEvents.PublishPaymentFailed(ctx, accountID, provider)
}

func (s *SubscriptionGRPC) publishDomainPlanCancelled(ctx context.Context, accountID, plan string) {
	if s == nil || s.DomainEvents == nil {
		return
	}
	_ = s.DomainEvents.PublishPlanCancelled(ctx, accountID, plan)
}

func (s *SubscriptionGRPC) publishDomainPlanExpired(ctx context.Context, accountID, plan string) {
	if s == nil || s.DomainEvents == nil {
		return
	}
	_ = s.DomainEvents.PublishPlanExpired(ctx, accountID, plan)
}

func (s *SubscriptionGRPC) publishDomainDowngrade(ctx context.Context, accountID, plan string) {
	if s == nil || s.DomainEvents == nil {
		return
	}
	_ = s.DomainEvents.PublishDowngrade(ctx, accountID, plan)
}

func (s *SubscriptionGRPC) publishDomainSpaceProStarted(ctx context.Context, spaceID, purchaserAccountID string) {
	if s == nil || s.DomainEvents == nil {
		return
	}
	_ = s.DomainEvents.PublishSpaceProStarted(ctx, spaceID, purchaserAccountID)
}

func (s *SubscriptionGRPC) publishDomainSpaceProExpired(ctx context.Context, spaceID string) {
	if s == nil || s.DomainEvents == nil {
		return
	}
	_ = s.DomainEvents.PublishSpaceProExpired(ctx, spaceID)
}

func (s *SubscriptionGRPC) syncSpaceProCache(ctx context.Context, spaceID, purchaserAccountID, status string) {
	if s == nil || s.SpaceEntitlements == nil {
		return
	}
	_, _ = s.SpaceEntitlements.SyncSpaceProSubscription(ctx, &spacev1.SyncSpaceProSubscriptionRequest{
		SpaceId:            spaceID,
		PurchaserAccountId: purchaserAccountID,
		Status:             status,
	})
}

func (s *SubscriptionGRPC) handlePremiumLifecycleWebhook(ctx context.Context, ev *billing.PaddleEvent, details json.RawMessage, fn func(uuid.UUID) error) error {
	accountID, err := billing.AccountIDFromCustomData(ev.Data.CustomData)
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	if err := fn(accountID); err != nil {
		if errors.Is(err, store.ErrDuplicateBillingEvent) {
			return nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return status.Error(codes.NotFound, "subscription not found")
		}
		return status.Error(codes.Internal, err.Error())
	}
	_ = ctx
	_ = details
	return nil
}

func (s *SubscriptionGRPC) HandleCloudPaymentsWebhook(ctx context.Context, req *subscriptionv1.HandleCloudPaymentsWebhookRequest) (*subscriptionv1.HandleCloudPaymentsWebhookResponse, error) {
	return nil, status.Error(codes.Unimplemented, "cloudpayments webhook not implemented")
}

func (s *SubscriptionGRPC) GetBillingHistory(ctx context.Context, req *subscriptionv1.GetBillingHistoryRequest) (*subscriptionv1.GetBillingHistoryResponse, error) {
	accountID, err := parseAccountID(req.GetAccountId())
	if err != nil {
		return nil, err
	}
	limit := 50
	var after *time.Time
	if req.GetPage() != nil && strings.TrimSpace(req.GetPage().GetCursor()) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(req.GetPage().GetCursor()))
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid cursor")
		}
		after = &parsed
	}
	rows, err := s.Store.ListBillingHistory(ctx, accountID, limit, after)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	events := make([]*subscriptionv1.BillingEvent, 0, len(rows))
	var nextCursor string
	for i, row := range rows {
		payload := string(row.Details)
		if payload == "" {
			payload = "{}"
		}
		events = append(events, &subscriptionv1.BillingEvent{
			Id:          row.ID.String(),
			Type:        row.Type,
			OccurredAt:  timestamppb.New(row.OccurredAt),
			PayloadJson: payload,
		})
		if i == len(rows)-1 && len(rows) == limit {
			nextCursor = row.OccurredAt.UTC().Format(time.RFC3339Nano)
		}
	}
	return &subscriptionv1.GetBillingHistoryResponse{
		BillingHistoryList: &subscriptionv1.BillingHistoryList{
			Events:     events,
			NextCursor: nextCursor,
		},
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
