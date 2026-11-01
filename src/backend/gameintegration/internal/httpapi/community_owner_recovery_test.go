package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/communityspace"
	"voice/backend/gameintegration/internal/registry"
	voicejwt "voice/backend/pkg/jwt"
)

type ownerRecoveryHTTPStore struct {
	opened    *registry.OpenOwnerRecoveryInput
	approved  *[3]uuid.UUID
	cancelled *[3]uuid.UUID
	completed *[5]any
	result    registry.OwnerRecoveryOperation
}

func (s *ownerRecoveryHTTPStore) OpenCommunityOwnerRecovery(_ context.Context, in registry.OpenOwnerRecoveryInput) (registry.OwnerRecoveryOperation, error) {
	s.opened = &in
	return s.result, nil
}
func (s *ownerRecoveryHTTPStore) ApproveCommunityOwnerRecovery(_ context.Context, app, op, operator uuid.UUID) (registry.OwnerRecoveryOperation, error) {
	s.approved = &[3]uuid.UUID{app, op, operator}
	return registry.OwnerRecoveryOperation{OperationID: op, ApplicationID: app, Status: "approved", ExpectedGeneration: 1, OperatorID: &operator, ExpiresAt: ptrCommunityTime(time.Now().Add(time.Minute))}, nil
}
func (s *ownerRecoveryHTTPStore) CancelCommunityOwnerRecovery(_ context.Context, app, op, operator uuid.UUID) (registry.OwnerRecoveryOperation, error) {
	s.cancelled = &[3]uuid.UUID{app, op, operator}
	return registry.OwnerRecoveryOperation{OperationID: op, ApplicationID: app, Status: "cancelled", ExpectedGeneration: 1, CancelledBy: &operator}, nil
}
func (s *ownerRecoveryHTTPStore) CompleteCommunityOwnerRecovery(_ context.Context, op, account, profile, space uuid.UUID, generation int64) (registry.OwnerRecoveryOperation, error) {
	s.completed = &[5]any{op, account, profile, space, generation}
	return registry.OwnerRecoveryOperation{OperationID: op, ApplicationID: uuid.New(), Status: "succeeded", SpaceID: &space, ExpectedGeneration: 1, ResultGeneration: &generation}, nil
}

type ownerRecoveryHTTPProvisioner struct {
	receipt     communityspace.OwnerRecoveryReceipt
	operationID uuid.UUID
	calls       int
}

func (p *ownerRecoveryHTTPProvisioner) ProvisionCommunityOwnerRecovery(_ context.Context, op registry.OwnerRecoveryOperation) (communityspace.OwnerRecoveryReceipt, error) {
	p.calls++
	p.operationID = op.OperationID
	return p.receipt, nil
}

func TestCommunityOwnerRecoveryUsesBearerIdentityApprovalAndSpaceReceipt(t *testing.T) {
	owner, replacementAccount, replacementProfile, operator, app, env, opID, spaceID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	digest := strings.Repeat("ab", 32)
	store := &ownerRecoveryHTTPStore{result: registry.OwnerRecoveryOperation{OperationID: opID, ApplicationID: app, EnvironmentID: env, CorporationKey: "corp-7", PreviousOwnerAccountID: owner, PreviousOwnerProfileID: uuid.New(), ReplacementAccountID: replacementAccount, ReplacementProfileID: replacementProfile, ExpectedGeneration: 1, ReasonCode: "owner_lost", EvidenceSHA256: []byte(strings.Repeat("x", 32)), Status: "approved", OperatorID: &operator, ExpiresAt: ptrCommunityTime(time.Now().Add(time.Minute)), TargetSpaceID: spaceID}}
	provisioner := &ownerRecoveryHTTPProvisioner{receipt: communityspace.OwnerRecoveryReceipt{SpaceID: spaceID, Generation: 2}}
	h := NewHandler(testValidator{claims: voicejwt.Claims{UserID: replacementAccount.String(), ProfileID: replacementProfile.String(), AccountType: "regular"}}, &testApprovalStore{})
	h.CommunityRecoveries = store
	h.CommunityRecoveryProvisioner = provisioner
	body := `{"environment_id":"` + env.String() + `","corporation_key":"corp-7","operation_id":"` + opID.String() + `","expected_generation":1,"reason_code":"owner_lost","evidence_sha256":"` + digest + `"}`
	r := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications/"+app.String()+"/community-bindings/owner-recovery", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer player-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotNil(t, store.opened)
	require.Equal(t, replacementAccount, store.opened.ReplacementAccountID)
	require.Equal(t, replacementProfile, store.opened.ReplacementProfileID)
	require.Equal(t, []byte{0xab, 0xab, 0xab, 0xab}, store.opened.EvidenceSHA256[:4])
	require.Equal(t, 1, provisioner.calls)
	require.Equal(t, opID, provisioner.operationID)
	require.Equal(t, [5]any{opID, replacementAccount, replacementProfile, spaceID, int64(2)}, *store.completed)

	h.Tokens = testValidator{claims: voicejwt.Claims{UserID: operator.String(), ProfileID: uuid.NewString(), AccountType: "regular"}}
	h.OperatorAccounts = map[uuid.UUID]struct{}{operator: {}}
	r = httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications/"+app.String()+"/community-bindings/owner-recoveries/"+opID.String()+"/approve", nil)
	r.Header.Set("Authorization", "Bearer player-token")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, [3]uuid.UUID{app, opID, operator}, *store.approved)
	r = httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications/"+app.String()+"/community-bindings/owner-recoveries/"+opID.String()+"/cancel", nil)
	r.Header.Set("Authorization", "Bearer player-token")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, [3]uuid.UUID{app, opID, operator}, *store.cancelled)
}
