package communityspace

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/gameintegration/internal/registry"
	"voice/backend/pkg/principal"
)

type Client struct {
	conn    *grpc.ClientConn
	service spacev1.SpaceServiceClient
	token   string
	issuer  *principal.Issuer
}

func New(target, token string) (*Client, error) {
	target, token = strings.TrimSpace(target), strings.TrimSpace(token)
	if target == "" || token == "" {
		return nil, fmt.Errorf("space bootstrap target and service token are required")
	}
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, service: spacev1.NewSpaceServiceClient(conn), token: token}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) SetPrincipalIssuer(issuer *principal.Issuer) {
	if c != nil {
		c.issuer = issuer
	}
}

// AreCoMembers rechecks service-owned Space membership and account bans for an
// app-scoped profile read. The request is signed for the exact RPC and bytes.
func (c *Client) AreCoMembers(ctx context.Context, profileA, profileB uuid.UUID, spaceIDs []uuid.UUID) (bool, error) {
	if c == nil || c.service == nil || c.issuer == nil || profileA == uuid.Nil || profileB == uuid.Nil || len(spaceIDs) == 0 {
		return false, fmt.Errorf("space co-membership client unavailable")
	}
	ids := make([]string, len(spaceIDs))
	for i, id := range spaceIDs {
		if id == uuid.Nil {
			return false, fmt.Errorf("space co-membership scope is invalid")
		}
		ids[i] = id.String()
	}
	req := &spacev1.AreCoMembersRequest{ProfileIdA: profileA.String(), ProfileIdB: profileB.String(), SpaceIds: ids}
	requestHash, err := principal.RequestHash(proto.Message(req))
	if err != nil {
		return false, fmt.Errorf("hash Space co-membership request: %w", err)
	}
	requestID := uuid.NewString()
	token, err := c.issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_AreCoMembers_FullMethodName,
		RequestID: requestID, RequestHash: requestHash})
	if err != nil {
		return false, fmt.Errorf("sign Space co-membership request: %w", err)
	}
	callCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	response, err := c.service.AreCoMembers(callCtx, req)
	if err != nil {
		return false, err
	}
	if response == nil {
		return false, fmt.Errorf("space co-membership response is empty")
	}
	return response.GetCoMembers(), nil
}

func (c *Client) ProvisionCommunityBootstrap(ctx context.Context, op registry.CommunityBootstrapOperation) (uuid.UUID, error) {
	if c == nil || c.service == nil || c.token == "" {
		return uuid.Nil, fmt.Errorf("space bootstrap client unavailable")
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.token)
	response, err := c.service.CreateCommunityBootstrap(ctx, &spacev1.CreateCommunityBootstrapRequest{
		OperationId: op.OperationID.String(), ApplicationId: op.ApplicationID.String(), EnvironmentId: op.EnvironmentID.String(),
		OwnerProfileId: op.OwnerProfileID.String(), OwnerAccountId: op.OwnerAccountID.String(), CorporationKey: op.CorporationKey, TemplateId: op.TemplateID,
	})
	if err != nil {
		return uuid.Nil, err
	}
	if response.GetSpace() == nil {
		return uuid.Nil, fmt.Errorf("space bootstrap returned no Space")
	}
	spaceID, err := uuid.Parse(response.GetSpace().GetId())
	if err != nil || spaceID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("space bootstrap returned invalid Space identity")
	}
	return spaceID, nil
}

type OwnerRecoveryReceipt struct {
	SpaceID    uuid.UUID
	Generation int64
}

func (c *Client) ProvisionCommunityOwnerRecovery(ctx context.Context, op registry.OwnerRecoveryOperation) (OwnerRecoveryReceipt, error) {
	if c == nil || c.service == nil || c.token == "" || op.TargetSpaceID == uuid.Nil {
		return OwnerRecoveryReceipt{}, fmt.Errorf("space owner recovery client unavailable")
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.token)
	response, err := c.service.RecoverCommunityOwner(ctx, &spacev1.RecoverCommunityOwnerRequest{
		OperationId: op.OperationID.String(), ApplicationId: op.ApplicationID.String(), EnvironmentId: op.EnvironmentID.String(),
		CorporationKey: op.CorporationKey, SpaceId: op.TargetSpaceID.String(), PreviousOwnerAccountId: op.PreviousOwnerAccountID.String(),
		PreviousOwnerProfileId: op.PreviousOwnerProfileID.String(), ReplacementAccountId: op.ReplacementAccountID.String(),
		ReplacementProfileId: op.ReplacementProfileID.String(), ExpectedGeneration: op.ExpectedGeneration,
		ReasonCode: op.ReasonCode, EvidenceSha256: append([]byte(nil), op.EvidenceSHA256...),
	})
	if err != nil {
		return OwnerRecoveryReceipt{}, err
	}
	spaceID, err := uuid.Parse(response.GetSpaceId())
	if err != nil || spaceID == uuid.Nil || spaceID != op.TargetSpaceID || response.GetOwnerGeneration() != op.ExpectedGeneration+1 {
		return OwnerRecoveryReceipt{}, fmt.Errorf("space owner recovery returned a mismatched receipt")
	}
	return OwnerRecoveryReceipt{SpaceID: spaceID, Generation: response.GetOwnerGeneration()}, nil
}

func (c *Client) ProjectCommunityRoster(ctx context.Context, in registry.CommunityRosterProjection) (registry.CommunityRosterSpaceReceipt, error) {
	if c == nil || c.service == nil || c.token == "" || in.OperationID == uuid.Nil || in.SpaceID == uuid.Nil ||
		in.LeaseExpiresAt.IsZero() || len(in.SnapshotSHA256) != 32 {
		return registry.CommunityRosterSpaceReceipt{}, fmt.Errorf("space community roster client unavailable")
	}
	profileIDs := make([]string, len(in.ProfileIDs))
	for i, id := range in.ProfileIDs {
		profileIDs[i] = id.String()
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.token)
	response, err := c.service.ApplyCommunityRoster(ctx, &spacev1.ApplyCommunityRosterRequest{
		OperationId: in.OperationID.String(), ApplicationId: in.ApplicationID.String(), EnvironmentId: in.EnvironmentID.String(),
		CorporationKey: in.CorporationKey, SpaceId: in.SpaceID.String(), OwnerGeneration: in.OwnerGeneration,
		SourceRevision: in.SourceRevision, SnapshotSha256: append([]byte(nil), in.SnapshotSHA256...),
		ProfileIds: profileIDs, LeaseExpiresAt: timestamppb.New(in.LeaseExpiresAt.UTC()),
	})
	if err != nil {
		return registry.CommunityRosterSpaceReceipt{}, err
	}
	receiptID, parseErr := uuid.Parse(response.GetReceiptId())
	if parseErr != nil || receiptID == uuid.Nil || response.GetOperationId() != in.OperationID.String() ||
		response.GetSpaceId() != in.SpaceID.String() || response.GetOwnerGeneration() != in.OwnerGeneration ||
		response.GetSourceRevision() != in.SourceRevision || string(response.GetSnapshotSha256()) != string(in.SnapshotSHA256) ||
		response.GetMemberCount() != int32(len(in.ProfileIDs)) || len(response.GetRequestSha256()) != 32 {
		return registry.CommunityRosterSpaceReceipt{}, fmt.Errorf("space community roster returned a mismatched receipt")
	}
	return registry.CommunityRosterSpaceReceipt{OperationID: in.OperationID, SpaceID: in.SpaceID,
		ReceiptID: receiptID, RequestHash: append([]byte(nil), response.GetRequestSha256()...), MemberCount: response.GetMemberCount()}, nil
}
