package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/authbinding"
	"voice/backend/gameintegration/internal/registry"
	voicejwt "voice/backend/pkg/jwt"
)

type bindingRevocationStoreStub struct {
	authority   registry.PlayerBindingAuthority
	operationID uuid.UUID
	err         error
	accountID   uuid.UUID
	calls       int
}

func (s *bindingRevocationStoreStub) RevokePlayerBindingForOwner(_ context.Context, id, account uuid.UUID,
	revision int64, operation uuid.UUID) (registry.PlayerBindingAuthority, error) {
	s.calls++
	s.accountID = account
	if s.err != nil {
		return registry.PlayerBindingAuthority{}, s.err
	}
	if id != s.authority.BindingID || revision != 1 || operation == uuid.Nil {
		return registry.PlayerBindingAuthority{}, registry.ErrPlayerBindingConflict
	}
	return s.authority, nil
}
func (s *bindingRevocationStoreStub) LoadPlayerBindingOperationID(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	if id != s.authority.BindingID {
		return uuid.Nil, registry.ErrPlayerBindingNotFound
	}
	return s.operationID, nil
}

type bindingRevocationAuthStub struct {
	receipt     authbinding.RevokeReceipt
	err         error
	calls       int
	operationID uuid.UUID
}

func (s *bindingRevocationAuthStub) Revoke(_ context.Context, operation uuid.UUID) (authbinding.RevokeReceipt, error) {
	s.calls++
	s.operationID = operation
	return s.receipt, s.err
}

func TestBindingRevocationRequiresOwnerAndBothDrainsBeforeSuccess(t *testing.T) {
	account, bindingID, grantOperation, revokeOperation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store := &bindingRevocationStoreStub{authority: registry.PlayerBindingAuthority{BindingID: bindingID,
		Status: "revoked", BindingRevision: 2}, operationID: grantOperation}
	auth := &bindingRevocationAuthStub{receipt: authbinding.RevokeReceipt{OperationID: grantOperation,
		Status: "revoking", AuthorityRevision: 4}}
	validator := testValidator{claims: voicejwt.Claims{UserID: account.String(), AccountType: "regular"}}
	handler := NewHandler(validator, &testInstallationStore{})
	handler.BindingRevocations = &BindingRevocationHandler{Tokens: validator, Store: store, Auth: auth}
	newRequest := func(user string) *http.Request {
		r := httptest.NewRequest(http.MethodDelete, "/api/v1/game-integrations/bindings/"+bindingID.String(),
			strings.NewReader(`{"expected_revision":1}`))
		r.Header.Set("Authorization", "Bearer player-token")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", revokeOperation.String())
		return r
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newRequest(account.String()))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Contains(t, w.Body.String(), "BINDING_REVOCATION_PENDING")
	require.Equal(t, grantOperation, auth.operationID)
	require.Equal(t, 1, store.calls)

	auth.receipt.Status = "revoked"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, newRequest(account.String()))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), bindingID.String())
	require.Equal(t, 2, store.calls, "same GIS idempotency key retries a pending cross-service revoke")

	store.err = registry.ErrPlayerBindingNotFound
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, newRequest(uuid.NewString()))
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Equal(t, 2, auth.calls, "non-owner cannot trigger Auth revocation")
}
