// Package memberadmissionrole consumes inert default-role receipts through the
// protected Space-only Role surface. A receipt alone grants no membership.
package memberadmissionrole

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/pkg/principal"
)

var ErrUnavailable = errors.New("default member role preparation unavailable")

type RoleClient interface {
	EnsureDefaultMemberRole(context.Context, *rolev1.EnsureDefaultMemberRoleRequest, ...grpc.CallOption) (*rolev1.EnsureDefaultMemberRoleResponse, error)
}

type Client struct {
	Role   RoleClient
	Issuer *principal.Issuer
	Clock  func() time.Time
}

type Receipt struct {
	binding memberadmission.EnsureBinding
	role    uuid.UUID
	digest  [32]byte
}

func (r *Receipt) For(binding memberadmission.EnsureBinding) (uuid.UUID, []byte, error) {
	if r == nil || r.role == uuid.Nil || memberadmission.ValidateEnsure(binding) != nil || binding.ProtocolVersion != r.binding.ProtocolVersion || binding.SpaceID != r.binding.SpaceID || binding.ProfileID != r.binding.ProfileID || binding.AdmissionOperationID != r.binding.AdmissionOperationID || binding.MembershipGeneration != r.binding.MembershipGeneration || !bytes.Equal(binding.AdmissionBindingSHA256, r.binding.AdmissionBindingSHA256) {
		return uuid.Nil, nil, ErrUnavailable
	}
	expected, err := memberadmission.EnsureReceiptFingerprint(binding, r.role.String())
	if err != nil || expected != r.digest {
		return uuid.Nil, nil, ErrUnavailable
	}
	return r.role, append([]byte(nil), r.digest[:]...), nil
}

func (c *Client) Ensure(ctx context.Context, binding memberadmission.EnsureBinding) (*Receipt, error) {
	if memberadmission.ValidateEnsure(binding) != nil || c == nil || c.Role == nil || c.Issuer == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrUnavailable
	}
	binding.AdmissionBindingSHA256 = append([]byte(nil), binding.AdmissionBindingSHA256...)
	request := &rolev1.EnsureDefaultMemberRoleRequest{ProtocolVersion: binding.ProtocolVersion, SpaceId: binding.SpaceID, ProfileId: binding.ProfileID, AdmissionOperationId: binding.AdmissionOperationID, MembershipGeneration: binding.MembershipGeneration, AdmissionBindingSha256: append([]byte(nil), binding.AdmissionBindingSHA256...)}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, ErrUnavailable
	}
	id := uuid.NewString()
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "role", RPC: rolev1.RoleService_EnsureDefaultMemberRole_FullMethodName, RequestID: id, RequestHash: hash})
	if err != nil {
		return nil, ErrUnavailable
	}
	clock := c.Clock
	if clock == nil {
		clock = time.Now
	}
	started := clock()
	ownerCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ownerCtx = metadata.NewOutgoingContext(ownerCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	wire := proto.Clone(request).(*rolev1.EnsureDefaultMemberRoleRequest)
	response, err := c.Role.EnsureDefaultMemberRole(ownerCtx, wire)
	now := clock()
	if err != nil || ownerCtx.Err() != nil || now.Before(started) || !now.Before(started.Add(2*time.Second)) || !proto.Equal(wire, request) || response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.Receipt == nil || len(response.Receipt.ProtoReflect().GetUnknown()) != 0 {
		return nil, ErrUnavailable
	}
	r := response.Receipt
	if r.ProtocolVersion != binding.ProtocolVersion || r.SpaceId != binding.SpaceID || r.ProfileId != binding.ProfileID || r.AdmissionOperationId != binding.AdmissionOperationID || r.MembershipGeneration != binding.MembershipGeneration || !bytes.Equal(r.AdmissionBindingSha256, binding.AdmissionBindingSHA256) {
		return nil, ErrUnavailable
	}
	digest, err := memberadmission.EnsureReceiptFingerprint(binding, r.RoleId)
	if err != nil || !bytes.Equal(digest[:], r.ReceiptSha256) {
		return nil, ErrUnavailable
	}
	return &Receipt{binding: binding, role: uuid.MustParse(r.RoleId), digest: digest}, nil
}
