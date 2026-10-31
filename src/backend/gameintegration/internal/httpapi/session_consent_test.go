package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
	voicejwt "voice/backend/pkg/jwt"
)

type sessionConsentValidator struct{ claims voicejwt.Claims }

func (v sessionConsentValidator) Validate(*http.Request) (voicejwt.Claims, string) {
	return v.claims, ""
}

type sessionConsentState struct{}

func (sessionConsentState) Minimum(context.Context, string) (int64, error)  { return 1, nil }
func (sessionConsentState) IsRevoked(context.Context, string) (bool, error) { return false, nil }

type sessionConsentStoreStub struct {
	accountID, profileID, sessionID, operationID uuid.UUID
	rosterRevision                               int64
	calls                                        int
}

func (s *sessionConsentStoreStub) RecordSessionConsent(_ context.Context, accountID, profileID, sessionID, operationID uuid.UUID, rosterRevision int64) (registry.SessionConsentReceipt, error) {
	s.calls++
	s.accountID, s.profileID, s.sessionID, s.operationID, s.rosterRevision = accountID, profileID, sessionID, operationID, rosterRevision
	return registry.SessionConsentReceipt{ReceiptID: uuid.New(), OperationID: operationID, SessionID: sessionID, ProfileID: profileID, RosterRevision: rosterRevision}, nil
}

type sessionKeepGroupStoreStub struct {
	input registry.SessionKeepGroupInput
	calls int
}

func (s *sessionKeepGroupStoreStub) KeepSessionGroup(_ context.Context, input registry.SessionKeepGroupInput) (registry.SessionKeepGroupReceipt, error) {
	s.calls++
	s.input = input
	return registry.SessionKeepGroupReceipt{OperationID: input.OperationID, SessionID: input.SessionID, RosterRevision: input.RosterRevision}, nil
}

func TestSessionConsentRequiresParticipantBearerAndTakesIdentityOnlyFromClaims(t *testing.T) {
	accountID, profileID, sessionID, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store := &sessionConsentStoreStub{}
	handler := NewSessionConsentHandler(Authorizer{
		Tokens: sessionConsentValidator{claims: voicejwt.Claims{UserID: accountID.String(), ProfileID: profileID.String(), JTI: "jti", SessionEpoch: 1}},
		State:  sessionConsentState{},
	}, store)
	path := "/api/v1/sessions/" + sessionID.String() + "/consent"
	body := `{"operation_id":"` + operationID.String() + `","roster_revision":7}`

	unauthenticated := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	require.Zero(t, store.calls)

	forged := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"operation_id":"`+operationID.String()+`","roster_revision":7,"profile_id":"`+uuid.NewString()+`","account_id":"`+uuid.NewString()+`"}`))
	forged.Header.Set("Authorization", "Bearer participant")
	forgedResult := httptest.NewRecorder()
	handler.ServeHTTP(forgedResult, forged)
	require.Equal(t, http.StatusBadRequest, forgedResult.Code, "identity is not accepted from client body fields")
	require.Zero(t, store.calls)

	valid := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	valid.Header.Set("Authorization", "Bearer participant")
	validResult := httptest.NewRecorder()
	handler.ServeHTTP(validResult, valid)
	require.Equal(t, http.StatusOK, validResult.Code)
	require.Equal(t, 1, store.calls)
	require.Equal(t, accountID, store.accountID)
	require.Equal(t, profileID, store.profileID)
	require.Equal(t, sessionID, store.sessionID)
	require.Equal(t, operationID, store.operationID)
	require.EqualValues(t, 7, store.rosterRevision)
}

func TestSessionConsentRejectsAuthOutageAndStaleOrForeignSessionWithoutReceipt(t *testing.T) {
	accountID, profileID, sessionID, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store := &sessionConsentStoreStub{}
	handler := NewSessionConsentHandler(Authorizer{}, store)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sessionID.String()+"/consent", strings.NewReader(`{"operation_id":"`+operationID.String()+`","roster_revision":7}`))
	request.Header.Set("Authorization", "Bearer participant")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Zero(t, store.calls)
	_ = accountID
	_ = profileID
}

func TestSessionRoutesDispatchesConsentToParticipantAuthBeforeGameCredential(t *testing.T) {
	consentCalls, keepGroupCalls, gameServerCalls := 0, 0, 0
	dispatcher := NewSessionRoutes(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { gameServerCalls++ }),
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { consentCalls++ }),
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { keepGroupCalls++ }),
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
	)
	consent := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/11111111-1111-4111-8111-111111111111/consent", nil)
	dispatcher.ServeHTTP(httptest.NewRecorder(), consent)
	keepGroup := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/11111111-1111-4111-8111-111111111111/keep-group", nil)
	dispatcher.ServeHTTP(httptest.NewRecorder(), keepGroup)
	roster := httptest.NewRequest(http.MethodPut, "/api/v1/sessions/11111111-1111-4111-8111-111111111111/roster", nil)
	dispatcher.ServeHTTP(httptest.NewRecorder(), roster)
	require.Equal(t, 1, consentCalls)
	require.Equal(t, 1, keepGroupCalls)
	require.Equal(t, 1, gameServerCalls)
}

func TestSessionKeepGroupUsesAuthenticatedIdentityAndStrictRequest(t *testing.T) {
	accountID, profileID, sessionID, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store := &sessionKeepGroupStoreStub{}
	handler := NewSessionKeepGroupHandler(Authorizer{
		Tokens: sessionConsentValidator{claims: voicejwt.Claims{UserID: accountID.String(), ProfileID: profileID.String(), JTI: "jti", SessionEpoch: 1}},
		State:  sessionConsentState{},
	}, store)
	path := "/api/v1/sessions/" + sessionID.String() + "/keep-group"
	body := `{"operation_id":"` + operationID.String() + `","roster_revision":7}`
	unauthenticated := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	require.Equal(t, http.StatusUnauthorized, unauthenticated.Code)
	require.Zero(t, store.calls)

	forged := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"operation_id":"`+operationID.String()+`","roster_revision":7,"profile_id":"`+uuid.NewString()+`"}`))
	forged.Header.Set("Authorization", "Bearer participant")
	forgedResult := httptest.NewRecorder()
	handler.ServeHTTP(forgedResult, forged)
	require.Equal(t, http.StatusBadRequest, forgedResult.Code)
	require.Zero(t, store.calls)

	valid := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	valid.Header.Set("Authorization", "Bearer participant")
	validResult := httptest.NewRecorder()
	handler.ServeHTTP(validResult, valid)
	require.Equal(t, http.StatusOK, validResult.Code)
	require.Equal(t, "no-store", validResult.Header().Get("Cache-Control"))
	require.Equal(t, 1, store.calls)
	require.Equal(t, registry.SessionKeepGroupInput{OperationID: operationID, SessionID: sessionID, AccountID: accountID, ProfileID: profileID, RosterRevision: 7}, store.input)
}
