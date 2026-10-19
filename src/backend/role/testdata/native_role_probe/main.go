// A real signed client for the disposable Role fixture. It never creates an
// installation witness or substitutes for Bot's verified initiating decision.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/testdata/nativefixture"
)

func run(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 64<<10))
	decoder.DisallowUnknownFields()
	var m nativefixture.Manifest
	if err = decoder.Decode(&m); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("one fixture manifest required")
	}
	if !m.FixtureOnly || m.SchemaVersion != 1 || m.RoleSchema != 25 || m.SpaceSchema != 44 || m.RoleDatabaseSystemID == "" || m.RoleDatabaseSystemID == m.SpaceDatabaseSystemID || !time.Now().Before(m.ExpiresAt) {
		return fmt.Errorf("qualified independent live fixture required")
	}
	ca, err := os.ReadFile(m.TLSCAFile)
	if err != nil {
		return err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return fmt.Errorf("fixture CA unavailable")
	}
	pair, err := tls.LoadX509KeyPair(m.TLSClientCertFile, m.TLSClientKeyFile)
	if err != nil {
		return err
	}
	key, err := principal.LoadRSAPrivateKeyFile(m.BotSignerKeyFile)
	if err != nil {
		return err
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: m.BotSignerKeyID, PrivateKey: key})
	if err != nil {
		return err
	}
	conn, err := grpc.NewClient(m.RoleAddress, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: m.TLSServerName, Certificates: []tls.Certificate{pair}})))
	if err != nil {
		return err
	}
	defer conn.Close()
	client := rolev1.NewRoleServiceClient(conn)
	signed := func(method, id string, req proto.Message) (context.Context, context.CancelFunc, error) {
		hash, e := principal.RequestHash(req)
		if e != nil {
			return nil, nil, e
		}
		token, e := issuer.IssueService(principal.ServiceInput{Audience: "role", RPC: method, RequestID: id, RequestHash: hash})
		if e != nil {
			return nil, nil, e
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id)), cancel, nil
	}
	request := &rolev1.CheckPermissionRequest{SpaceId: m.SpaceID, ProfileId: m.OwnerProfileID, PermissionName: "SPACE_MANAGE_BOTS"}
	ctx, cancel, err := signed(rolev1.RoleService_CheckPermission_FullMethodName, uuid.NewString(), request)
	if err != nil {
		return err
	}
	defer cancel()
	response, err := client.CheckPermission(ctx, request)
	if err != nil {
		return err
	}
	if !response.GetAllowed() {
		return fmt.Errorf("actual owner decision denied")
	}
	replayMD, _ := metadata.FromOutgoingContext(ctx)
	replayCtx, replayCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer replayCancel()
	_, err = client.CheckPermission(metadata.NewOutgoingContext(replayCtx, replayMD), request)
	if status.Code(err) != codes.Unauthenticated {
		return fmt.Errorf("replayed transport reached decision: %v", err)
	}
	freshCtx, freshCancel, err := signed(rolev1.RoleService_CheckPermission_FullMethodName, uuid.NewString(), request)
	if err != nil {
		return err
	}
	defer freshCancel()
	fresh, err := client.CheckPermission(freshCtx, request)
	if err != nil || !fresh.GetAllowed() {
		return fmt.Errorf("fresh signed owner retry unavailable: %v", err)
	}
	// Reach the actual cleanup validator without inventing native installation
	// evidence. The real Bot consumer supplies that separately from its journal.
	invalid := &rolev1.ApplyBotInstallationRemovalRequest{OperationId: uuid.NewString()}
	ctx2, cancel2, err := signed(rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName, invalid.OperationId, invalid)
	if err != nil {
		return err
	}
	defer cancel2()
	_, err = client.ApplyBotInstallationRemoval(ctx2, invalid)
	if status.Code(err) != codes.InvalidArgument {
		return fmt.Errorf("cleanup validation route unavailable: %v", err)
	}
	read := &rolev1.ListRolesRequest{SpaceId: m.SpaceID}
	ctx3, cancel3, err := signed(rolev1.RoleService_ListRoles_FullMethodName, uuid.NewString(), read)
	if err != nil {
		return err
	}
	defer cancel3()
	_, err = client.ListRoles(ctx3, read)
	if status.Code(err) != codes.Unauthenticated && status.Code(err) != codes.PermissionDenied {
		return fmt.Errorf("Bot fixture key escaped closed decision/cleanup authority")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"actual_owner_allowed": true, "replay_rejected": true, "cleanup_validator_reached": true, "generic_bot_read_rejected": true, "independent_databases": true})
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: native_role_probe /absolute/path/ready.json")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
