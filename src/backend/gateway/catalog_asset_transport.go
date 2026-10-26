package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	chatv1 "voice.app/voice/chat/v1"
)

func isGIFSelectionRequest(r *http.Request) bool {
	return r != nil && r.Method == http.MethodPost && r.URL.Path == "/api/v1/gifs/resolve"
}
func (t *transcoder) serveGIFSelection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	claims, ok := r.Context().Value(verifiedUserClaimsKey{}).(tokenClaims)
	if !ok || (claims.AuthenticatedAccountType != "regular" && claims.AuthenticatedAccountType != "guest" && claims.AuthenticatedAccountType != "sdk-account") {
		writeGRPCError(w, status.Error(codes.Unauthenticated, "verified explicit actor required"))
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 65537))
	req := &chatv1.ResolveGifAssetRequest{}
	if err != nil || len(data) > 65536 || protojson.Unmarshal(data, req) != nil || req.ProtocolVersion != 1 || !canonicalLifecycleUUID(req.OperationId) || req.Provider == "" || req.Provider != strings.TrimSpace(req.Provider) || req.ProviderId == "" || req.ProviderId != strings.TrimSpace(req.ProviderId) || !validGIFDestination(req.DestinationChat) {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid GIF selection"))
		return
	}
	ctx, err := t.auditInitiatingContext(r, req, chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName, uuid.NewString())
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	response, err := t.clients.chatCatalog.ResolveGifAsset(ctx, req)
	if err != nil {
		code := status.Code(err)
		if code == codes.Unknown {
			code = codes.Unavailable
		}
		writeGRPCError(w, status.Error(code, "GIF selection unavailable"))
		return
	}
	if response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.Provider != req.Provider || response.ProviderId != req.ProviderId || !canonicalLifecycleUUID(response.SourceId) || !canonicalLifecycleUUID(response.FileId) || !validGIFImportStatus(response.Status) {
		writeGRPCError(w, status.Error(codes.Unavailable, "invalid GIF selection response"))
		return
	}
	writeProtoJSON(w, http.StatusOK, response)
}
func validGIFDestination(ref *chatv1.ChatRef) bool {
	if ref == nil || !canonicalLifecycleUUID(ref.Id) {
		return false
	}
	switch ref.GetType() {
	case chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, chatv1.ChatType_CHAT_TYPE_DM, chatv1.ChatType_CHAT_TYPE_GROUP, chatv1.ChatType_CHAT_TYPE_CHANNEL:
		return true
	default:
		return false
	}
}
func validGIFImportStatus(value string) bool {
	switch value {
	case "pending_upload", "processing", "ready", "failed", "deleted", "expired":
		return true
	default:
		return false
	}
}

func catalogAssetClientConfigFromEnv() (lifecycleClientConfig, bool, error) {
	values := [5]string{}
	active := false
	for i, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
		value, present := os.LookupEnv("GATEWAY_CHAT_CATALOG_ASSET_" + suffix)
		active = active || present
		values[i] = value
	}
	c := lifecycleClientConfig{values[0], values[1], values[2], values[3], values[4]}
	if !active {
		return c, false, nil
	}
	for _, value := range values {
		if value == "" || value != strings.TrimSpace(value) {
			return c, true, status.Error(codes.FailedPrecondition, "complete Gateway catalog mTLS configuration required")
		}
	}
	_, err := c.tlsConfig()
	return c, true, err
}
func (c *grpcClients) configureCatalogAssetTransport() {
	cfg, enabled, err := catalogAssetClientConfigFromEnv()
	if !enabled && err == nil {
		return
	}
	transport := auditInitiatingTransport{service: "CHAT_CATALOG", err: err}
	if err == nil {
		transport.conn, transport.err = cfg.dial()
		if transport.err == nil {
			c.connections = append(c.connections, transport.conn)
			c.chatCatalog = chatv1.NewCatalogAssetServiceClient(transport.conn)
		}
	}
	c.auditInitiatingTransports = append(c.auditInitiatingTransports, transport)
}
