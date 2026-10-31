package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
	voicejwt "voice/backend/pkg/jwt"
)

type communityBootstrapTestStore struct {
	opened     *registry.OpenCommunityBootstrapInput
	approved   *[3]uuid.UUID
	ownerID    uuid.UUID
	openResult registry.CommunityBootstrapOperation
	completed  *[3]uuid.UUID
}

func (s *communityBootstrapTestStore) OpenCommunityBootstrap(_ context.Context, in registry.OpenCommunityBootstrapInput) (registry.CommunityBootstrapOperation, error) {
	s.opened = &in
	if s.openResult.Status != "" {
		return s.openResult, nil
	}
	return registry.CommunityBootstrapOperation{OperationID: in.OperationID, Status: "pending"}, nil
}

func (s *communityBootstrapTestStore) ApproveCommunityBootstrap(_ context.Context, appID, opID, operatorID uuid.UUID) (registry.CommunityBootstrapOperation, error) {
	if operatorID == s.ownerID {
		return registry.CommunityBootstrapOperation{}, registry.ErrInvalidApplication
	}
	s.approved = &[3]uuid.UUID{appID, opID, operatorID}
	return registry.CommunityBootstrapOperation{OperationID: opID, ApplicationID: appID, Status: "approved"}, nil
}

func (s *communityBootstrapTestStore) CompleteCommunityBootstrap(_ context.Context, opID, ownerID, spaceID uuid.UUID) (registry.CommunityBootstrapOperation, error) {
	s.completed = &[3]uuid.UUID{opID, ownerID, spaceID}
	return registry.CommunityBootstrapOperation{OperationID: opID, OwnerAccountID: ownerID, Status: "succeeded", SpaceID: &spaceID}, nil
}

type communitySpaceTestProvisioner struct {
	operationID uuid.UUID
	spaceID     uuid.UUID
	calls       int
	err         error
}

func (p *communitySpaceTestProvisioner) ProvisionCommunityBootstrap(_ context.Context, op registry.CommunityBootstrapOperation) (uuid.UUID, error) {
	p.calls++
	p.operationID = op.OperationID
	return p.spaceID, p.err
}

func TestCommunityBootstrapRequiresOwnerOperationAndSeparateOperatorApproval(t *testing.T) {
	ownerAccountID, ownerProfileID, appID, envID, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store := &communityBootstrapTestStore{}
	store.ownerID = ownerAccountID
	handler := NewHandler(testValidator{claims: voicejwt.Claims{
		UserID: ownerAccountID.String(), ProfileID: ownerProfileID.String(), AccountType: "regular",
	}}, &testApprovalStore{})
	handler.CommunityBootstraps = store
	handler.CommunityTemplates = map[string]struct{}{"guild-default": {}}
	request := httptest.NewRequest(http.MethodPost,
		"/api/v1/game-integrations/applications/"+appID.String()+"/community-bindings",
		strings.NewReader(`{"environment_id":"`+envID.String()+`","corporation_key":"corp-7","space_template_id":"guild-default","operation_id":"`+operationID.String()+`"}`))
	request.Header.Set("Authorization", "Bearer player-token")
	request.Header.Set("Idempotency-Key", operationID.String())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusAccepted, response.Code,
		"a verified human Owner must be able to open an exact, pending community operation")
	require.Equal(t, ownerAccountID, store.opened.OwnerAccountID)
	require.Equal(t, ownerProfileID, store.opened.OwnerProfileID)
	require.Equal(t, operationID, store.opened.OperationID)
}

func TestCommunityBootstrapApprovalRequiresDistinctAllowlistedRegularOperator(t *testing.T) {
	ownerID, operatorID, appID, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store := &communityBootstrapTestStore{}
	store.ownerID = ownerID
	handler := NewHandler(testValidator{claims: voicejwt.Claims{UserID: operatorID.String(), ProfileID: uuid.NewString(), AccountType: "regular"}}, &testApprovalStore{})
	handler.CommunityBootstraps = store
	handler.OperatorAccounts = map[uuid.UUID]struct{}{operatorID: {}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications/"+appID.String()+"/community-bindings/"+operationID.String()+"/approve", nil)
	request.Header.Set("Authorization", "Bearer player-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, [3]uuid.UUID{appID, operationID, operatorID}, *store.approved)

	handler.Tokens = testValidator{claims: voicejwt.Claims{UserID: ownerID.String(), ProfileID: uuid.NewString(), AccountType: "regular"}}
	handler.OperatorAccounts = map[uuid.UUID]struct{}{ownerID: {}}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusForbidden, response.Code, "an allowlisted application owner cannot approve their own operation")
}

func TestCommunityBootstrapRejectsUnconfiguredTemplate(t *testing.T) {
	appID, ownerID, profileID := uuid.New(), uuid.New(), uuid.New()
	handler := NewHandler(testValidator{claims: voicejwt.Claims{UserID: ownerID.String(), ProfileID: profileID.String(), AccountType: "regular"}}, &testApprovalStore{})
	handler.CommunityBootstraps = &communityBootstrapTestStore{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications/"+appID.String()+"/community-bindings", strings.NewReader(`{"environment_id":"`+uuid.NewString()+`","corporation_key":"corp","space_template_id":"unlisted","operation_id":"`+uuid.NewString()+`"}`))
	request.Header.Set("Authorization", "Bearer player-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusForbidden, response.Code)
}

func TestCommunityBootstrapOwnerRetryProvisionsAndPersistsSpaceReceipt(t *testing.T) {
	ownerID, profileID, appID, envID, operationID, spaceID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store := &communityBootstrapTestStore{openResult: registry.CommunityBootstrapOperation{
		OperationID: operationID, ApplicationID: appID, EnvironmentID: envID,
		OwnerAccountID: ownerID, OwnerProfileID: profileID, CorporationKey: "corp-7", TemplateID: "guild-default",
		Status: "approved", ExpiresAt: ptrCommunityTime(time.Now().Add(time.Minute)),
	}}
	provisioner := &communitySpaceTestProvisioner{spaceID: spaceID}
	handler := NewHandler(testValidator{claims: voicejwt.Claims{UserID: ownerID.String(), ProfileID: profileID.String(), AccountType: "regular"}}, &testApprovalStore{})
	handler.CommunityBootstraps = store
	handler.CommunityTemplates = map[string]struct{}{"guild-default": {}}
	handler.CommunityProvisioner = provisioner
	request := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications/"+appID.String()+"/community-bindings", strings.NewReader(`{"environment_id":"`+envID.String()+`","corporation_key":"corp-7","space_template_id":"guild-default","operation_id":"`+operationID.String()+`"}`))
	request.Header.Set("Authorization", "Bearer player-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusCreated, response.Code)
	require.Equal(t, 1, provisioner.calls)
	require.Equal(t, operationID, provisioner.operationID)
	require.Equal(t, [3]uuid.UUID{operationID, ownerID, spaceID}, *store.completed)
}

func TestCommunityBootstrapDoesNotWriteSpaceWithoutCurrentApprovalOrSpaceAuthority(t *testing.T) {
	ownerID, profileID, appID, envID, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	newRequest := func() *http.Request {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/game-integrations/applications/"+appID.String()+"/community-bindings", strings.NewReader(`{"environment_id":"`+envID.String()+`","corporation_key":"corp-7","space_template_id":"guild-default","operation_id":"`+operationID.String()+`"}`))
		request.Header.Set("Authorization", "Bearer player-token")
		return request
	}
	for _, tc := range []struct {
		name        string
		expiry      time.Time
		provisioner *communitySpaceTestProvisioner
		wantStatus  int
	}{
		{name: "expired operator approval", expiry: time.Now().Add(-time.Second), provisioner: &communitySpaceTestProvisioner{spaceID: uuid.New()}, wantStatus: http.StatusConflict},
		{name: "space client unavailable", expiry: time.Now().Add(time.Minute), wantStatus: http.StatusServiceUnavailable},
		{name: "space authority outage", expiry: time.Now().Add(time.Minute), provisioner: &communitySpaceTestProvisioner{err: errors.New("Space unavailable")}, wantStatus: http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &communityBootstrapTestStore{openResult: registry.CommunityBootstrapOperation{
				OperationID: operationID, ApplicationID: appID, EnvironmentID: envID,
				OwnerAccountID: ownerID, OwnerProfileID: profileID, CorporationKey: "corp-7", TemplateID: "guild-default",
				Status: "approved", ExpiresAt: ptrCommunityTime(tc.expiry),
			}}
			handler := NewHandler(testValidator{claims: voicejwt.Claims{UserID: ownerID.String(), ProfileID: profileID.String(), AccountType: "regular"}}, &testApprovalStore{})
			handler.CommunityBootstraps = store
			handler.CommunityTemplates = map[string]struct{}{"guild-default": {}}
			if tc.provisioner != nil {
				handler.CommunityProvisioner = tc.provisioner
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, newRequest())
			require.Equal(t, tc.wantStatus, response.Code)
			require.Nil(t, store.completed, "no Space receipt is persisted without a valid authority result")
			if tc.name == "expired operator approval" {
				require.Equal(t, 0, tc.provisioner.calls, "expired approval must stop before the Space call")
			}
			if tc.name == "space authority outage" {
				require.Equal(t, 1, tc.provisioner.calls, "the owner can retry the same operation after a Space outage")
			}
		})
	}
}

func ptrCommunityTime(value time.Time) *time.Time { return &value }
