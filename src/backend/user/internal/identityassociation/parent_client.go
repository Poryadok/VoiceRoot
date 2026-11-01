package identityassociation

import (
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	identityv1 "voice.app/voice/identityerasure/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
)

type parentRPC interface {
	ResolveAccountHandle(context.Context, *identityv1.ResolveAccountHandleRequest, ...grpc.CallOption) (*identityv1.ResolveAccountHandleResponse, error)
}

// AuthParentClient validates a protected online observation. Transport and a
// complete owner cut do not establish Auth source continuity or recovery.
type AuthParentClient struct {
	rpc    parentRPC
	issuer *principal.Issuer
}

// AuthParentTransport contains caller-owned mTLS material, with no fallback
// trust or identity configuration. The caller owns the returned connection.
type AuthParentTransport struct {
	Address, ServerName string
	Roots               *x509.CertPool
	Certificate         tls.Certificate
}

func NewAuthParentClient(config AuthParentTransport, issuer *principal.Issuer) (*AuthParentClient, *grpc.ClientConn, error) {
	if issuer == nil || config.Address == "" || config.Address != strings.TrimSpace(config.Address) || config.ServerName == "" || config.ServerName != strings.TrimSpace(config.ServerName) || config.Roots == nil || config.Roots.Equal(x509.NewCertPool()) || len(config.Certificate.Certificate) == 0 || config.Certificate.PrivateKey == nil {
		return nil, nil, ErrIncomplete
	}
	signer, ok := config.Certificate.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, nil, ErrIncomplete
	}
	leaf, err := x509.ParseCertificate(config.Certificate.Certificate[0])
	if err != nil {
		return nil, nil, ErrIncomplete
	}
	public, err := x509.MarshalPKIXPublicKey(leaf.PublicKey)
	if err != nil {
		return nil, nil, ErrIncomplete
	}
	privatePublic, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil || !bytes.Equal(public, privatePublic) {
		return nil, nil, ErrIncomplete
	}
	certificate := config.Certificate
	certificate.Certificate = make([][]byte, len(config.Certificate.Certificate))
	for i, der := range config.Certificate.Certificate {
		certificate.Certificate[i] = append([]byte(nil), der...)
	}
	certificate.Leaf = nil
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: config.Roots.Clone(), ServerName: config.ServerName, Certificates: []tls.Certificate{certificate}}
	conn, err := grpc.NewClient(grpcclient.DialTarget(config.Address), grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), grpc.WithDisableRetry(), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(64<<10), grpc.MaxCallSendMsgSize(4<<10)))
	if err != nil {
		return nil, nil, ErrIncomplete
	}
	return &AuthParentClient{rpc: identityv1.NewAuthUserIdentityAuthorityServiceClient(conn), issuer: issuer}, conn, nil
}

func (c *AuthParentClient) ObserveParent(ctx context.Context, account uuid.UUID) (ParentObservation, error) {
	var empty ParentObservation
	if c == nil || c.rpc == nil || c.issuer == nil || ctx == nil || ctx.Err() != nil || account == uuid.Nil {
		return empty, ErrIncomplete
	}
	id := uuid.NewString()
	request := &identityv1.ResolveAccountHandleRequest{ProtocolVersion: 1, RequestId: id, AccountId: account.String()}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return empty, ErrIncomplete
	}
	token, err := c.issuer.IssueService(principal.ServiceInput{Audience: "auth", RPC: identityv1.AuthUserIdentityAuthorityService_ResolveAccountHandle_FullMethodName, RequestID: id, RequestHash: hash})
	if err != nil {
		return empty, ErrIncomplete
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	bounded = metadata.NewOutgoingContext(bounded, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	wire := proto.Clone(request).(*identityv1.ResolveAccountHandleRequest)
	response, err := c.rpc.ResolveAccountHandle(bounded, wire)
	if err != nil || bounded.Err() != nil || !proto.Equal(request, wire) || response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.ProtocolVersion != 1 || response.RequestId != id || response.AccountId != account.String() || response.Window == nil || response.Lifecycle == nil || len(response.Window.ProtoReflect().GetUnknown()) != 0 || len(response.Lifecycle.ProtoReflect().GetUnknown()) != 0 {
		return empty, ErrIncomplete
	}
	window, fact := response.Window, response.Lifecycle
	cut := fact.OwnerCut
	if window.ObservedAt == nil || window.ValidUntil == nil || window.ObservedAt.CheckValid() != nil || window.ValidUntil.CheckValid() != nil || len(window.ObservedAt.ProtoReflect().GetUnknown()) != 0 || len(window.ValidUntil.ProtoReflect().GetUnknown()) != 0 || cut == nil || len(cut.ProtoReflect().GetUnknown()) != 0 || cut.Owner != "auth" || !cut.Complete || cut.FactCount == 0 || len(cut.Digest) != 32 || len(cut.SelectorKeyVersions) == 0 || len(cut.SelectorKeyVersions) > 256 {
		return empty, ErrIncomplete
	}
	canonicalID := func(value string) bool {
		id, err := uuid.Parse(value)
		return err == nil && id != uuid.Nil && id.String() == value
	}
	if !canonicalID(fact.OwnerHandle) || !canonicalID(cut.StreamId) || !canonicalID(cut.HistoryGeneration) {
		return empty, ErrIncomplete
	}
	out := ParentObservation{AccountID: account, OwnerHandle: fact.OwnerHandle, LifecycleRevision: fact.LifecycleRevision, ObservedAt: window.ObservedAt.AsTime(), ValidUntil: window.ValidUntil.AsTime(), Cut: ParentCut{Owner: cut.Owner, Stream: cut.StreamId, HistoryGeneration: cut.HistoryGeneration, ThroughRevision: cut.ThroughRevision, KeysetRevision: cut.SelectorKeysetRevision, Complete: cut.Complete}}
	copy(out.Cut.Digest[:], cut.Digest)
	for _, value := range cut.SelectorKeyVersions {
		version, parseErr := strconv.ParseUint(value, 10, 32)
		if parseErr != nil || version == 0 || strconv.FormatUint(version, 10) != value || (len(out.Cut.RequiredVersions) > 0 && uint32(version) <= out.Cut.RequiredVersions[len(out.Cut.RequiredVersions)-1]) {
			return empty, ErrIncomplete
		}
		out.Cut.RequiredVersions = append(out.Cut.RequiredVersions, uint32(version))
	}
	switch fact.State {
	case identityv1.AccountLifecycleState_ACCOUNT_LIFECYCLE_STATE_ACTIVE:
		out.State = ParentActive
	case identityv1.AccountLifecycleState_ACCOUNT_LIFECYCLE_STATE_RECOVERABLE:
		out.State = ParentRecoverable
	case identityv1.AccountLifecycleState_ACCOUNT_LIFECYCLE_STATE_TERMINAL:
		out.State = ParentTerminal
	default:
		return empty, ErrIncomplete
	}
	switch fact.Eligibility {
	case identityv1.Eligibility_ELIGIBILITY_ELIGIBLE:
		out.Eligibility = ParentEligible
	case identityv1.Eligibility_ELIGIBILITY_INELIGIBLE:
		out.Eligibility = ParentIneligible
	default:
		return empty, ErrIncomplete
	}
	switch fact.Disposition {
	case identityv1.ErasureDisposition_ERASURE_DISPOSITION_NOT_IRREVERSIBLE:
		out.Disposition = ParentNotIrreversible
	case identityv1.ErasureDisposition_ERASURE_DISPOSITION_IRREVERSIBLE:
		out.Disposition = ParentIrreversible
	default:
		return empty, ErrIncomplete
	}
	if fact.DeletionCycle != "" {
		if !canonicalID(fact.DeletionCycle) {
			return empty, ErrIncomplete
		}
		out.CycleID = uuid.MustParse(fact.DeletionCycle)
	}
	if fact.EraseAt != nil {
		if fact.EraseAt.CheckValid() != nil || len(fact.EraseAt.ProtoReflect().GetUnknown()) != 0 {
			return empty, ErrIncomplete
		}
		out.EraseAt = fact.EraseAt.AsTime()
	}
	if !validParentObservation(out, account, time.Now()) {
		return empty, ErrIncomplete
	}
	return out, nil
}
