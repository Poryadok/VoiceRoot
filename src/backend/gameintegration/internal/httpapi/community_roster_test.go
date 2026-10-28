package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/gameintegration/internal/registry"
)

type communityRosterRouteStub struct {
	sessionOrchestratorStub
	input     registry.CommunityRosterSnapshotInput
	principal registry.SessionPrincipal
	calls     int
	view      registry.CommunityRosterSnapshotView
}

func (s *communityRosterRouteStub) GetCommunityRosterSnapshot(_ context.Context, principal registry.SessionPrincipal, corporationKey string) (registry.CommunityRosterSnapshotView, error) {
	s.calls++
	s.principal = principal
	s.view.CorporationKey = corporationKey
	return s.view, nil
}

func (s *communityRosterRouteStub) ApplyCommunityRosterSnapshot(_ context.Context, principal registry.SessionPrincipal, input registry.CommunityRosterSnapshotInput) (registry.CommunityRosterResult, error) {
	s.calls++
	s.principal, s.input = principal, input
	return registry.CommunityRosterResult{OperationID: input.OperationID, Outcome: "accepted"}, nil
}

func TestCommunityRosterRouteRequiresRosterScopeAndForwardsOnlyOpaqueBindingRefs(t *testing.T) {
	appID, envID, bindingID := uuid.New(), uuid.New(), uuid.New()
	principal := registry.SessionPrincipal{ApplicationID: appID, EnvironmentID: envID, Scopes: []string{"game.roster.write"}}
	orchestrator := &communityRosterRouteStub{}
	handler := NewSessionHandler(sessionServerValidator{principal: principal}, orchestrator)
	opID := uuid.New()
	body := `{"operation_id":"` + opID.String() + `","source_revision":9,"owner_generation":3,"snapshot_sha256":"` + strings.Repeat("a", 64) + `","pages":[{"index":0,"count":1,"sha256":"` + strings.Repeat("b", 64) + `","members":[{"binding_id":"` + bindingID.String() + `","character_key":"char-1","source_membership_revision":4,"source_rank":"member","grant_reasons":["membership"]}]}]}`
	response := sessionRequest(handler, http.MethodPut, "/api/v1/community-bindings/corp-1/roster", body)
	require.Equal(t, http.StatusAccepted, response.Code, response.Body.String())
	require.Equal(t, 1, orchestrator.calls)
	require.Equal(t, principal, orchestrator.principal)
	require.Equal(t, "corp-1", orchestrator.input.CorporationKey)
	require.Equal(t, bindingID, orchestrator.input.Pages[0].Members[0].BindingID)
	require.Equal(t, strings.Repeat("a", 64), fmtHex(orchestrator.input.SnapshotSHA256))

	profileID := uuid.New()
	orchestrator.view = registry.CommunityRosterSnapshotView{ApplicationID: appID, EnvironmentID: envID,
		CorporationKey: "corp-1", SourceRevision: 9, OwnerGeneration: 3, SnapshotSHA256: make([]byte, 32),
		Members: []registry.CommunityRosterGrant{{BindingID: bindingID, CharacterKey: "char-1", SourceRank: "member", GrantReasons: []string{"membership"}}}}
	read := sessionRequest(handler, http.MethodGet, "/api/v1/community-bindings/corp-1/roster", "")
	require.Equal(t, http.StatusOK, read.Code, read.Body.String())
	require.NotContains(t, read.Body.String(), profileID.String())
	require.NotContains(t, read.Body.String(), "profile_id")
	require.NotContains(t, read.Body.String(), "account_id")
	require.Contains(t, read.Body.String(), bindingID.String(), "only the app-scoped opaque binding reference is exposed")

	principal.Scopes = []string{"game.sessions.manage"}
	orchestrator = &communityRosterRouteStub{}
	handler = NewSessionHandler(sessionServerValidator{principal: principal}, orchestrator)
	response = sessionRequest(handler, http.MethodPut, "/api/v1/community-bindings/corp-1/roster", body)
	require.Equal(t, http.StatusForbidden, response.Code)
	require.Zero(t, orchestrator.calls, "session-management scope cannot write corporation grants")
}

func fmtHex(value []byte) string {
	const digits = "0123456789abcdef"
	result := make([]byte, len(value)*2)
	for i, b := range value {
		result[i*2], result[i*2+1] = digits[b>>4], digits[b&0x0f]
	}
	return string(result)
}
