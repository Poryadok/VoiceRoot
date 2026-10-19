// Package reporttargetclient is the only report validator owner transport.
package reporttargetclient

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	spacev1 "voice.app/voice/space/v1"
	storyv1 "voice.app/voice/story/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/reporttargetprincipal"
)

type Client struct {
	Issuer      *principal.Issuer
	Connections map[string]grpc.ClientConnInterface
}

func (c *Client) Resolve(ctx context.Context, reportID uuid.UUID, target string, id uuid.UUID) (string, uuid.UUID, error) {
	invalid := errors.New("report target owner unavailable")
	if c == nil || c.Issuer == nil || reportID == uuid.Nil || id == uuid.Nil || c.Connections[target] == nil {
		return "", uuid.Nil, invalid
	}
	var request, response proto.Message
	var state commonv1.ReportTargetState
	account := uuid.Nil
	switch target {
	case "user":
		request = &userv1.ResolveReportTargetRequest{ReportId: reportID.String(), ProfileId: id.String()}
		response = &userv1.ResolveReportTargetResponse{}
	case "message":
		request = &messagingv1.ResolveReportTargetRequest{ReportId: reportID.String(), MessageId: id.String()}
		response = &messagingv1.ResolveReportTargetResponse{}
	case "space":
		request = &spacev1.ResolveReportTargetRequest{ReportId: reportID.String(), SpaceId: id.String()}
		response = &spacev1.ResolveReportTargetResponse{}
	case "story":
		request = &storyv1.ResolveReportTargetRequest{ReportId: reportID.String(), StoryId: id.String()}
		response = &storyv1.ResolveReportTargetResponse{}
	default:
		return "", uuid.Nil, invalid
	}
	audience := target
	if target == "message" {
		audience = "messaging"
	}
	method := reporttargetprincipal.Method(audience)
	hash, err := principal.RequestHash(request)
	if err != nil {
		return "", uuid.Nil, invalid
	}
	requestID := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return "", uuid.Nil, invalid
	}
	callCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	if err = c.Connections[target].Invoke(callCtx, method, request, response, grpc.WaitForReady(false)); err != nil {
		return "", uuid.Nil, err
	}
	if len(response.ProtoReflect().GetUnknown()) != 0 {
		return "", uuid.Nil, invalid
	}
	switch out := response.(type) {
	case *userv1.ResolveReportTargetResponse:
		state = out.State
		if state == commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE {
			account, err = reporttargetprincipal.CanonicalID(out.AccountId)
			if err != nil {
				return "", uuid.Nil, invalid
			}
		} else if out.AccountId != "" {
			return "", uuid.Nil, invalid
		}
	case *messagingv1.ResolveReportTargetResponse:
		state = out.State
	case *spacev1.ResolveReportTargetResponse:
		state = out.State
	case *storyv1.ResolveReportTargetResponse:
		state = out.State
	}
	switch state {
	case commonv1.ReportTargetState_REPORT_TARGET_STATE_LIVE:
		return "LIVE", account, nil
	case commonv1.ReportTargetState_REPORT_TARGET_STATE_MISSING:
		return "MISSING", uuid.Nil, nil
	case commonv1.ReportTargetState_REPORT_TARGET_STATE_DELETED:
		return "DELETED", uuid.Nil, nil
	default:
		return "", uuid.Nil, invalid
	}
}
