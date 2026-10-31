package httpapi

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/gameintegration/internal/communityspace"
	"voice/backend/gameintegration/internal/registry"
	"voice/backend/pkg/integrationtest"
	voicejwt "voice/backend/pkg/jwt"
)

type recoveryCompositionSpace struct {
	spacev1.UnimplementedSpaceServiceServer
	request       *spacev1.RecoverCommunityOwnerRequest
	beforeReceipt func()
	wrongReceipt  bool
}

func (s *recoveryCompositionSpace) RecoverCommunityOwner(_ context.Context, req *spacev1.RecoverCommunityOwnerRequest) (*spacev1.RecoverCommunityOwnerResponse, error) {
	s.request = req
	if s.beforeReceipt != nil {
		s.beforeReceipt()
	}
	if s.wrongReceipt {
		return &spacev1.RecoverCommunityOwnerResponse{SpaceId: uuid.NewString(), OwnerGeneration: req.ExpectedGeneration + 1}, nil
	}
	return &spacev1.RecoverCommunityOwnerResponse{SpaceId: req.SpaceId, OwnerGeneration: req.ExpectedGeneration + 1}, nil
}

func startCommunityCompositionStore(t *testing.T) (*registry.Store, context.Context) {
	t.Helper()
	ctx := context.Background()
	dir := filepath.Join("..", "..", "..", "migrations", "game_integration_db")
	pool := integrationtest.StartPostgres(t, ctx, "community_composition", filepath.Join(dir, "000001_init.up.sql"))
	for _, name := range []string{
		"000002_t12_registry_security.up.sql",
		"000004_t11_installation_bot_binding.up.sql",
		"000005_t16_player_binding_authority.up.sql",
		"000011_t30_resource_mappings.up.sql",
		"000021_t37_community_bootstrap.up.sql",
		"000022_t37_owner_recovery.up.sql",
		"000023_t38_community_roster.up.sql",
	} {
		body, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err, name)
	}
	return &registry.Store{Pool: pool}, ctx
}

func TestCommunityOwnerRecoveryProductionComposition(t *testing.T) {
	store, ctx := startCommunityCompositionStore(t)
	for _, scenario := range []string{"success", "wrong_application", "wrong_corporation", "wrong_generation", "changed_generation_during_rpc", "changed_target_during_rpc", "expired_before_dispatch", "expired_during_rpc", "mismatched_rpc_target", "security_recovery_suspended_parent"} {
		t.Run(scenario, func(t *testing.T) {
			store.Now = nil
			owner, profile, operator, replacement, replacementProfile, spaceID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
			app, err := store.CreateApplication(ctx, registry.CreateApplicationInput{OwnerAccountID: owner, Name: "Recovery composition", IdempotencyKey: "app"})
			require.NoError(t, err)
			env, err := store.ApproveSandbox(ctx, registry.ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: operator, IdempotencyKey: "sandbox"})
			require.NoError(t, err)
			bootstrap := registry.OpenCommunityBootstrapInput{OperationID: uuid.New(), ApplicationID: app.ID, EnvironmentID: env.ID, OwnerAccountID: owner, OwnerProfileID: profile, CorporationKey: "corp-real", TemplateID: "guild-default"}
			_, err = store.OpenCommunityBootstrap(ctx, bootstrap)
			require.NoError(t, err)
			_, err = store.ApproveCommunityBootstrap(ctx, app.ID, bootstrap.OperationID, operator)
			require.NoError(t, err)
			_, err = store.CompleteCommunityBootstrap(ctx, bootstrap.OperationID, owner, spaceID)
			require.NoError(t, err)

			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			server := grpc.NewServer()
			service := &recoveryCompositionSpace{}
			spacev1.RegisterSpaceServiceServer(server, service)
			go func() { _ = server.Serve(listener) }()
			t.Cleanup(server.Stop)
			client, err := communityspace.New(listener.Addr().String(), "private-test-token")
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, client.Close()) })

			operationID := uuid.New()
			input := registry.OpenOwnerRecoveryInput{OperationID: operationID, ApplicationID: app.ID, EnvironmentID: env.ID, CorporationKey: "corp-real", ReplacementAccountID: replacement, ReplacementProfileID: replacementProfile, ExpectedGeneration: 1, ReasonCode: "owner_lost", EvidenceSHA256: make([]byte, 32)}
			opened, err := store.OpenCommunityOwnerRecovery(ctx, input)
			require.NoError(t, err)
			require.Nil(t, opened.SpaceID, "pending SpaceID is the operation result, never its target")
			approved, err := store.ApproveCommunityOwnerRecovery(ctx, app.ID, operationID, operator)
			require.NoError(t, err)
			require.Nil(t, approved.SpaceID, "canonical22 approved output remains NULL")
			var saved *uuid.UUID
			require.NoError(t, store.Pool.QueryRow(ctx, "SELECT space_id FROM gis_community_owner_recovery_operations WHERE operation_id=$1", operationID).Scan(&saved))
			require.Nil(t, saved)

			requestApp := app.ID
			expectedStatus := http.StatusOK
			expectedGeneration := int64(2)
			expectedAuthorityStatus := "active"
			expectRPC := true
			switch scenario {
			case "wrong_application":
				requestApp = uuid.New()
				expectedStatus = http.StatusForbidden
				expectRPC = false
			case "wrong_corporation":
				input.CorporationKey = "foreign-corp"
				expectedStatus = http.StatusForbidden
				expectRPC = false
			case "wrong_generation":
				input.ExpectedGeneration = 2
				expectedStatus = http.StatusConflict
				expectRPC = false
			case "changed_generation_during_rpc":
				expectedStatus = http.StatusConflict
				expectedGeneration = 3
				service.beforeReceipt = func() {
					_, e := store.Pool.Exec(ctx, "UPDATE gis_community_owner_authority SET owner_generation=3 WHERE application_id=$1", app.ID)
					require.NoError(t, e)
				}
			case "changed_target_during_rpc":
				expectedStatus = http.StatusConflict
				service.beforeReceipt = func() {
					_, e := store.Pool.Exec(ctx, "UPDATE gis_community_owner_authority SET space_id=$2 WHERE application_id=$1", app.ID, uuid.New())
					require.NoError(t, e)
				}
			case "expired_before_dispatch":
				expectedStatus = http.StatusConflict
				expectRPC = false
				_, err = store.Pool.Exec(ctx, "UPDATE gis_community_owner_recovery_operations SET approval_expires_at=clock_timestamp()-interval '1 second' WHERE operation_id=$1", operationID)
				require.NoError(t, err)
			case "expired_during_rpc":
				expectedStatus = http.StatusConflict
				store.Now = func() time.Time { return time.Now().Add(time.Hour) }
			case "mismatched_rpc_target":
				expectedStatus = http.StatusServiceUnavailable
				service.wrongReceipt = true
			case "security_recovery_suspended_parent":
				_, err = store.Pool.Exec(ctx, "UPDATE applications SET status='suspended',revision=revision+1 WHERE id=$1", app.ID)
				require.NoError(t, err)
			}
			if expectedStatus != http.StatusOK {
				expectedAuthorityStatus = "recovery_pending"
				if scenario != "changed_generation_during_rpc" {
					expectedGeneration = 1
				}
			}
			h := NewHandler(testValidator{claims: voicejwt.Claims{UserID: replacement.String(), ProfileID: replacementProfile.String(), AccountType: "regular"}}, store)
			h.CommunityRecoveryProvisioner = client
			body, err := json.Marshal(map[string]any{"environment_id": env.ID, "corporation_key": input.CorporationKey, "operation_id": operationID, "expected_generation": input.ExpectedGeneration, "reason_code": "owner_lost", "evidence_sha256": hex.EncodeToString(input.EvidenceSHA256)})
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications/"+requestApp.String()+"/community-bindings/owner-recovery", strings.NewReader(string(body)))
			request.Header.Set("Authorization", "Bearer player-token")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, request)
			require.Equal(t, expectedStatus, w.Code, w.Body.String())
			if !expectRPC {
				require.Nil(t, service.request)
			} else {
				require.NotNil(t, service.request, "production adapter must really call protected Space RPC")
				require.Equal(t, spaceID.String(), service.request.SpaceId)
				require.Equal(t, app.ID.String(), service.request.ApplicationId)
				require.Equal(t, env.ID.String(), service.request.EnvironmentId)
				require.Equal(t, input.CorporationKey, service.request.CorporationKey)
				require.Equal(t, operationID.String(), service.request.OperationId)
				require.Equal(t, replacement.String(), service.request.ReplacementAccountId)
			}
			var generation int64
			var status string
			require.NoError(t, store.Pool.QueryRow(ctx, "SELECT owner_generation,status,space_id FROM gis_community_owner_authority WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3", app.ID, env.ID, bootstrap.CorporationKey).Scan(&generation, &status, &saved))
			require.EqualValues(t, expectedGeneration, generation)
			require.Equal(t, expectedAuthorityStatus, status)
			if scenario != "changed_target_during_rpc" {
				require.Equal(t, spaceID, *saved)
			}
			var operationStatus string
			var resultSpace *uuid.UUID
			require.NoError(t, store.Pool.QueryRow(ctx, "SELECT status,space_id FROM gis_community_owner_recovery_operations WHERE operation_id=$1", operationID).Scan(&operationStatus, &resultSpace))
			if expectedStatus == http.StatusOK {
				require.Equal(t, "succeeded", operationStatus)
				require.Equal(t, spaceID, *resultSpace)
			} else {
				require.Equal(t, "approved", operationStatus)
				require.Nil(t, resultSpace, "failed receipt must preserve NULL result and recovery freeze")
			}
		})
	}
}

func TestCommunityBootstrapParentLifecycleFences(t *testing.T) {
	store, ctx := startCommunityCompositionStore(t)
	for _, stage := range []string{"fresh", "approval", "dispatch", "receipt"} {
		t.Run(stage, func(t *testing.T) {
			owner, operator := uuid.New(), uuid.New()
			app, err := store.CreateApplication(ctx, registry.CreateApplicationInput{OwnerAccountID: owner, Name: "Lifecycle", IdempotencyKey: uuid.NewString()})
			require.NoError(t, err)
			env, err := store.ApproveSandbox(ctx, registry.ApproveSandboxInput{ApplicationID: app.ID, OperatorAccountID: operator, IdempotencyKey: uuid.NewString()})
			require.NoError(t, err)
			in := registry.OpenCommunityBootstrapInput{OperationID: uuid.New(), ApplicationID: app.ID, EnvironmentID: env.ID, OwnerAccountID: owner, OwnerProfileID: uuid.New(), CorporationKey: "corp-lifecycle", TemplateID: "guild-default"}
			if stage != "fresh" {
				_, err = store.OpenCommunityBootstrap(ctx, in)
				require.NoError(t, err)
				if stage != "approval" {
					_, err = store.ApproveCommunityBootstrap(ctx, app.ID, in.OperationID, operator)
					require.NoError(t, err)
				}
			}
			_, err = store.Pool.Exec(ctx, "UPDATE applications SET status='suspended',revision=revision+1 WHERE id=$1", app.ID)
			require.NoError(t, err)
			switch stage {
			case "fresh":
				_, err = store.OpenCommunityBootstrap(ctx, in)
				require.ErrorIs(t, err, registry.ErrInvalidApplication)
			case "approval":
				_, err = store.ApproveCommunityBootstrap(ctx, app.ID, in.OperationID, operator)
				require.ErrorIs(t, err, registry.ErrInvalidApplication)
			case "receipt":
				_, err = store.CompleteCommunityBootstrap(ctx, in.OperationID, owner, uuid.New())
				require.ErrorIs(t, err, registry.ErrInvalidApplication)
			case "dispatch":
				h := NewHandler(testValidator{claims: voicejwt.Claims{UserID: owner.String(), ProfileID: in.OwnerProfileID.String(), AccountType: "regular"}}, store)
				h.CommunityTemplates = map[string]struct{}{"guild-default": {}}
				provisioner := &communitySpaceTestProvisioner{spaceID: uuid.New()}
				h.CommunityProvisioner = provisioner
				body, _ := json.Marshal(map[string]any{"environment_id": env.ID, "corporation_key": in.CorporationKey, "space_template_id": in.TemplateID, "operation_id": in.OperationID})
				req := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications/"+app.ID.String()+"/community-bindings", strings.NewReader(string(body)))
				req.Header.Set("Authorization", "Bearer player-token")
				w := httptest.NewRecorder()
				h.ServeHTTP(w, req)
				require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
				require.Zero(t, provisioner.calls)
			}
			var count int
			require.NoError(t, store.Pool.QueryRow(ctx, "SELECT count(*) FROM gis_community_bootstrap_operations WHERE application_id=$1 AND status='succeeded'", app.ID).Scan(&count))
			require.Zero(t, count, "inactive parent cannot gain a new Space receipt")
		})
	}
}
