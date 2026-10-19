//go:build role_space_native

package main

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
)

type fixtureBotSeed struct {
	SchemaVersion int `json:"schema_version"`
	BotID string `json:"bot_id"`
	ActorProfileID string `json:"actor_profile_id"`
	SpaceID string `json:"space_id"`
	InstallationID string `json:"installation_id"`
	InstallOperationID string `json:"install_operation_id"`
	Generation uint64 `json:"generation"`
}

type fixtureBotPostcondition struct {
	fixtureBotSeed
	OperationID string `json:"operation_id"`
	ReceiptID string `json:"receipt_id"`
	DecisionID string `json:"decision_id"`
}

func fixtureWriteJSON(dir, name string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil { return err }
	path := filepath.Join(dir, name)
	if err = os.WriteFile(path+".tmp", raw, 0600); err != nil { return err }
	return os.Rename(path+".tmp", path)
}

func fixtureWaitFile(ctx context.Context, dir, name string) error {
	timer := time.NewTimer(3*time.Second)
	defer timer.Stop()
	for {
		if _, err := os.Stat(filepath.Join(dir,name)); err == nil { return nil } else if !os.IsNotExist(err) { return err }
		select { case <-ctx.Done(): return ctx.Err(); case <-timer.C: return fmt.Errorf("fixture handshake missing %s",name); case <-time.After(25*time.Millisecond): }
	}
}

// The provider self-test uses the real protected signed client; it does not
// qualify the independent Bot journal/target selector owned by consumer247.
func startFixtureBotControl(t *testing.T, ctx context.Context, client rolev1.RoleServiceClient, key *rsa.PrivateKey, space string) <-chan error {
	if os.Getenv("ROLE_NATIVE_FIXTURE_BOT_SELFTEST") != "1" { return nil }
	dir := os.Getenv("ROLE_NATIVE_FIXTURE_EXPORT_DIR")
	done := make(chan error,1)
	go func() {
		err := func() error {
			if err := fixtureWaitFile(ctx,dir,"ready.json"); err != nil { return err }
			seed := fixtureBotSeed{1,uuid.NewString(),uuid.NewString(),space,uuid.NewString(),uuid.NewString(),1}
			if err := fixtureWriteJSON(dir,"seed.request.json",seed); err != nil { return err }
			if err := fixtureWaitFile(ctx,dir,"seed.ready.json"); err != nil { return err }
			req := &rolev1.ApplyBotInstallationRemovalRequest{ProtocolVersion:1,OperationId:uuid.NewString(),DecisionId:uuid.NewString(),DecisionSha256:bytes.Repeat([]byte{3},32),Purpose:"space_uninstall",BotId:seed.BotID,ActorProfileId:seed.ActorProfileID,SpaceId:space,FrozenScopeSha256:bytes.Repeat([]byte{4},32),Evidence:&rolev1.ApplyBotInstallationRemovalRequest_Native{Native:&rolev1.BotRemovalNativeEvidence{InstallationId:seed.InstallationID,InstallOperationId:seed.InstallOperationID,Generation:seed.Generation}}}
			call := func() (*rolev1.ApplyBotInstallationRemovalResponse,error) { return client.ApplyBotInstallationRemoval(sourceCallContext(t,key,"bot",rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName,req.OperationId,req),req) }
			first,err := call(); if err != nil { return err }
			if first.GetReceipt().GetDeletedRoleCount()!=1 || first.GetReceipt().GetRemovedAssignmentCount()!=2 { return fmt.Errorf("nonzero effects differ: roles=%d assignments=%d",first.GetReceipt().GetDeletedRoleCount(),first.GetReceipt().GetRemovedAssignmentCount()) }
			again,err := call(); if err != nil { return err }; if !proto.Equal(first,again) { return fmt.Errorf("replay changed receipt") }
			post := fixtureBotPostcondition{seed,req.OperationId,first.Receipt.ReceiptId,req.DecisionId}
			if err := fixtureWriteJSON(dir,"postcondition.request.json",post); err != nil { return err }
			return fixtureWaitFile(ctx,dir,"postcondition.ready.json")
		}()
		_ = os.WriteFile(filepath.Join(dir,"stop"),nil,0600)
		done <- err
	}()
	return done
}

func TestNativeFixtureBotCleanupNonzero(t *testing.T) {
	t.Setenv("ROLE_NATIVE_FIXTURE_EXPORT_DIR",filepath.Join(t.TempDir(),"export"))
	t.Setenv("ROLE_NATIVE_FIXTURE_ROLE_SOURCE",os.Getenv("ROLE_NATIVE_FIXTURE_ROLE_SOURCE"))
	t.Setenv("ROLE_NATIVE_FIXTURE_BOT_CLEANUP","1")
	t.Setenv("ROLE_NATIVE_FIXTURE_BOT_SELFTEST","1")
	runNativeMemberProtocol(t,false,"25","44")
}

func fixtureCheckControl(t *testing.T, done <-chan error) {
	if done != nil { require.NoError(t,<-done) }
}
