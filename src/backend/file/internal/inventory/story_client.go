package inventory

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
	inv "voice/backend/pkg/referenceinventory"
)

// StorySource owns a trusted mTLS connection. Its issuer must be File's own
// configured signer; incoming/forwarded metadata is always replaced.
type StorySource struct {
	client storyv1.StoryReferenceInventoryServiceClient
	issuer *principal.Issuer
	conn   *grpc.ClientConn
}

func DialStorySource(target string, trust *tls.Config, issuer *principal.Issuer) (*StorySource, error) {
	if target == "" || issuer == nil || trust == nil || trust.InsecureSkipVerify || trust.RootCAs == nil || trust.ServerName == "" || len(trust.Certificates) == 0 || trust.MinVersion < tls.VersionTLS12 {
		return nil, inv.ErrUnavailable
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(target), grpc.WithDisableRetry(), grpc.WithTransportCredentials(credentials.NewTLS(trust.Clone())))
	if err != nil {
		return nil, err
	}
	return &StorySource{client: storyv1.NewStoryReferenceInventoryServiceClient(conn), issuer: issuer, conn: conn}, nil
}
func (c *StorySource) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *StorySource) signed(ctx context.Context, rpc string, request proto.Message) (context.Context, context.CancelFunc, error) {
	if c == nil || c.client == nil || c.issuer == nil {
		return nil, nil, inv.ErrUnavailable
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, nil, err
	}
	id := uuid.NewString()
	token, err := c.issuer.IssueService(principal.ServiceInput{Audience: "story", RPC: rpc, RequestID: id, RequestHash: hash})
	if err != nil {
		return nil, nil, err
	}
	// Issuer is a configured dependency. Verify its fixed identity locally too,
	// rather than permitting accidental injection of another service's signer.
	parts := strings.Split(token, ".")
	var claims struct {
		Issuer  string `json:"iss"`
		Subject string `json:"sub"`
		Type    string `json:"principal_type"`
	}
	if len(parts) != 3 {
		return nil, nil, errors.New("invalid File principal")
	}
	payload, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil || json.Unmarshal(payload, &claims) != nil || claims.Issuer != "file" || claims.Subject != "service:file" || claims.Type != "service" {
		return nil, nil, errors.New("file issuer required for inventory")
	}
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	return metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), cancel, nil
}
func (c *StorySource) BeginReferenceInventory(ctx context.Context, r *commonv1.BeginReferenceInventoryRequest) (*commonv1.BeginReferenceInventoryResponse, error) {
	call, cancel, err := c.signed(ctx, storyv1.StoryReferenceInventoryService_BeginReferenceInventory_FullMethodName, r)
	if err != nil {
		return nil, err
	}
	defer cancel()
	return c.client.BeginReferenceInventory(call, r, grpc.WaitForReady(false))
}
func (c *StorySource) ReadReferenceInventoryPage(ctx context.Context, r *commonv1.ReadReferenceInventoryPageRequest) (*commonv1.ReadReferenceInventoryPageResponse, error) {
	call, cancel, err := c.signed(ctx, storyv1.StoryReferenceInventoryService_ReadReferenceInventoryPage_FullMethodName, r)
	if err != nil {
		return nil, err
	}
	defer cancel()
	return c.client.ReadReferenceInventoryPage(call, r, grpc.WaitForReady(false))
}
func (c *StorySource) ReadReferenceInventoryChanges(ctx context.Context, r *commonv1.ReadReferenceInventoryChangesRequest) (*commonv1.ReadReferenceInventoryChangesResponse, error) {
	call, cancel, err := c.signed(ctx, storyv1.StoryReferenceInventoryService_ReadReferenceInventoryChanges_FullMethodName, r)
	if err != nil {
		return nil, err
	}
	defer cancel()
	return c.client.ReadReferenceInventoryChanges(call, r, grpc.WaitForReady(false))
}
