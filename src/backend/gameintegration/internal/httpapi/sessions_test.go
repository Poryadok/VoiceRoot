package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

type sessionRequestVerifier struct {
	principal registry.SessionPrincipal
}

func (v sessionRequestVerifier) VerifyGameServer(*http.Request) (registry.SessionPrincipal, error) {
	return v.principal, nil
}

type sessionRequestOrchestrator struct {
	creates int
}

func (o *sessionRequestOrchestrator) CreateSession(_ context.Context, _ registry.SessionPrincipal, _ registry.CreateSessionInput) (registry.SessionOperation, error) {
	o.creates++
	return registry.SessionOperation{}, nil
}

func (*sessionRequestOrchestrator) GetOperation(context.Context, registry.SessionPrincipal, uuid.UUID) (registry.SessionOperation, error) {
	return registry.SessionOperation{}, nil
}

func (*sessionRequestOrchestrator) CloseSession(context.Context, registry.SessionPrincipal, uuid.UUID, uuid.UUID) (registry.SessionOperation, error) {
	return registry.SessionOperation{}, nil
}

func newSessionRequestHandler(orchestrator *sessionRequestOrchestrator) http.Handler {
	return NewSessionHandler(sessionRequestVerifier{principal: registry.SessionPrincipal{
		ApplicationID: uuid.MustParse("00000000-0000-4000-8000-000000000001"),
		EnvironmentID: uuid.MustParse("00000000-0000-4000-8000-000000000002"),
		Scopes:        []string{"game.sessions.manage"},
	}}, orchestrator)
}

func sessionCreateBody(revision, displayName string) []byte {
	return []byte(fmt.Sprintf(`{"operation_id":"00000000-0000-4000-8000-000000000003","kind":"match","external_key":"match-1","display_name":%q,"roster_revision":%s,"roster_complete":true,"members":[]}`, displayName, revision))
}

func TestCreateSessionHTTPRejectsUnsafeRosterRevisionsBeforeAcceptance(t *testing.T) {
	for _, test := range []struct {
		revision string
		want     int
		calls    int
	}{
		{revision: "9007199254740991", want: http.StatusAccepted, calls: 1},
		{revision: "9007199254740992", want: http.StatusBadRequest},
		{revision: "9007199254740993", want: http.StatusBadRequest},
	} {
		t.Run(test.revision, func(t *testing.T) {
			orchestrator := &sessionRequestOrchestrator{}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader(sessionCreateBody(test.revision, "Match")))
			response := httptest.NewRecorder()

			newSessionRequestHandler(orchestrator).ServeHTTP(response, request)

			if response.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.want, response.Body.String())
			}
			if orchestrator.creates != test.calls {
				t.Fatalf("create calls = %d, want %d", orchestrator.creates, test.calls)
			}
		})
	}
}

func TestCreateSessionHTTPRejectsInvalidUTF8AndOverlongDisplayName(t *testing.T) {
	orchestrator := &sessionRequestOrchestrator{}
	invalidUTF8 := sessionCreateBody("1", "Match")
	displayName := bytes.Index(invalidUTF8, []byte("Match"))
	if displayName < 0 {
		t.Fatal("test body does not contain display name")
	}
	invalidUTF8[displayName] = 0xff
	overlongName := sessionCreateBody("1", strings.Repeat("a", 129))

	for name, body := range map[string][]byte{"invalid_utf8": invalidUTF8, "display_name_over_128_scalars": overlongName} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader(body))
			response := httptest.NewRecorder()

			newSessionRequestHandler(orchestrator).ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
			}
		})
	}
	if orchestrator.creates != 0 {
		t.Fatalf("create calls = %d, want 0", orchestrator.creates)
	}
}
