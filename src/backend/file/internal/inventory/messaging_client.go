package inventory

import (
	"context"
	"crypto/tls"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/grpcclient"
	"voice/backend/pkg/principal"
	inv "voice/backend/pkg/referenceinventory"
)

type MessagingSource struct {
	client messagingv1.MessagingReferenceInventoryServiceClient
	issuer *principal.Issuer
	conn   *grpc.ClientConn
}

func DialMessagingSource(target string, trust *tls.Config, issuer *principal.Issuer) (*MessagingSource, error) {
	if target == "" || issuer == nil || trust == nil || trust.InsecureSkipVerify || trust.RootCAs == nil || trust.ServerName == "" || len(trust.Certificates) == 0 || trust.MinVersion < tls.VersionTLS12 {
		return nil, inv.ErrUnavailable
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(target), grpc.WithDisableRetry(), grpc.WithTransportCredentials(credentials.NewTLS(trust.Clone())))
	if err != nil {
		return nil, err
	}
	return &MessagingSource{client: messagingv1.NewMessagingReferenceInventoryServiceClient(conn), issuer: issuer, conn: conn}, nil
}
func (c *MessagingSource) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *MessagingSource) BeginReferenceInventory(ctx context.Context, r *commonv1.BeginReferenceInventoryRequest) (*commonv1.BeginReferenceInventoryResponse, error) {
	if c == nil || c.client == nil || inv.Strict(r) != nil {
		return nil, inv.ErrInvalid
	}
	w := &messagingv1.BeginReferenceInventoryRequest{Inventory: r}
	call, cancel, err := signedInventoryRequest(ctx, c.issuer, "messaging", messagingv1.MessagingReferenceInventoryService_BeginReferenceInventory_FullMethodName, w)
	if err != nil {
		return nil, err
	}
	defer cancel()
	response, err := c.client.BeginReferenceInventory(call, w, grpc.WaitForReady(false))
	if err != nil {
		return nil, err
	}
	if inv.Strict(response) != nil || response.Inventory == nil {
		return nil, inv.ErrInvalid
	}
	return response.Inventory, nil
}
func (c *MessagingSource) ReadReferenceInventoryPage(ctx context.Context, r *commonv1.ReadReferenceInventoryPageRequest) (*commonv1.ReadReferenceInventoryPageResponse, error) {
	if c == nil || c.client == nil || inv.Strict(r) != nil {
		return nil, inv.ErrInvalid
	}
	w := &messagingv1.ReadReferenceInventoryPageRequest{Inventory: r}
	call, cancel, err := signedInventoryRequest(ctx, c.issuer, "messaging", messagingv1.MessagingReferenceInventoryService_ReadReferenceInventoryPage_FullMethodName, w)
	if err != nil {
		return nil, err
	}
	defer cancel()
	response, err := c.client.ReadReferenceInventoryPage(call, w, grpc.WaitForReady(false))
	if err != nil {
		return nil, err
	}
	if inv.Strict(response) != nil || response.Inventory == nil {
		return nil, inv.ErrInvalid
	}
	return response.Inventory, nil
}
func (c *MessagingSource) ReadReferenceInventoryChanges(ctx context.Context, r *commonv1.ReadReferenceInventoryChangesRequest) (*commonv1.ReadReferenceInventoryChangesResponse, error) {
	if c == nil || c.client == nil || inv.Strict(r) != nil {
		return nil, inv.ErrInvalid
	}
	w := &messagingv1.ReadReferenceInventoryChangesRequest{Inventory: r}
	call, cancel, err := signedInventoryRequest(ctx, c.issuer, "messaging", messagingv1.MessagingReferenceInventoryService_ReadReferenceInventoryChanges_FullMethodName, w)
	if err != nil {
		return nil, err
	}
	defer cancel()
	response, err := c.client.ReadReferenceInventoryChanges(call, w, grpc.WaitForReady(false))
	if err != nil {
		return nil, err
	}
	if inv.Strict(response) != nil || response.Inventory == nil {
		return nil, inv.ErrInvalid
	}
	return response.Inventory, nil
}
