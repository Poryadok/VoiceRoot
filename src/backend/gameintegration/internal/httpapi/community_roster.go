package httpapi

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"voice/backend/gameintegration/internal/registry"
)

type communityRosterSnapshotRequest struct {
	OperationID     string                       `json:"operation_id"`
	SourceRevision  int64                        `json:"source_revision"`
	OwnerGeneration int64                        `json:"owner_generation"`
	SnapshotSHA256  string                       `json:"snapshot_sha256"`
	Pages           []communityRosterPageRequest `json:"pages"`
}

type communityRosterPageRequest struct {
	Index   int32                            `json:"index"`
	Count   int32                            `json:"count"`
	SHA256  string                           `json:"sha256"`
	Members []registry.CommunityRosterMember `json:"members"`
}

func (r communityRosterSnapshotRequest) toInput(corporationKey string) (registry.CommunityRosterSnapshotInput, error) {
	operationID, parseErr := uuid.Parse(r.OperationID)
	if parseErr != nil || operationID == uuid.Nil || operationID.String() != r.OperationID ||
		r.SourceRevision <= 0 || r.OwnerGeneration <= 0 || len(r.Pages) == 0 {
		return registry.CommunityRosterSnapshotInput{}, errors.New("invalid community roster request")
	}
	snapshotHash, err := decodeRosterSHA256(r.SnapshotSHA256)
	if err != nil {
		return registry.CommunityRosterSnapshotInput{}, err
	}
	pages := make([]registry.CommunityRosterPage, len(r.Pages))
	for i, page := range r.Pages {
		hash, err := decodeRosterSHA256(page.SHA256)
		if err != nil {
			return registry.CommunityRosterSnapshotInput{}, err
		}
		pages[i] = registry.CommunityRosterPage{Index: page.Index, Count: page.Count,
			SHA256: hash, Members: page.Members}
	}
	return registry.CommunityRosterSnapshotInput{OperationID: operationID, CorporationKey: corporationKey,
		SourceRevision: r.SourceRevision, OwnerGeneration: r.OwnerGeneration,
		SnapshotSHA256: snapshotHash, Pages: pages}, nil
}

func communityRosterCorporationKey(u *url.URL) (string, bool) {
	const prefix = "/api/v1/community-bindings/"
	path := u.EscapedPath()
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, "/roster") || u.RawQuery != "" {
		return "", false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(path, prefix), "/roster")
	if raw == "" || strings.Contains(raw, "/") {
		return "", false
	}
	value, err := url.PathUnescape(raw)
	return value, err == nil && value != "" && len(value) <= 256 && !strings.Contains(value, "/")
}

func decodeRosterSHA256(value string) ([]byte, error) {
	if len(value) != 64 || value != strings.ToLower(value) {
		return nil, errors.New("invalid sha256")
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return nil, errors.New("invalid sha256")
	}
	return decoded, nil
}

func sessionPrincipalHasScope(p registry.SessionPrincipal, required string) bool {
	for _, scope := range p.Scopes {
		if scope == required {
			return true
		}
	}
	return false
}

func writeCommunityRosterError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, registry.ErrInvalidCommunityRoster):
		writeError(w, http.StatusUnprocessableEntity, "INVALID_COMMUNITY_ROSTER")
	case errors.Is(err, registry.ErrStaleCommunityRoster):
		writeError(w, http.StatusConflict, "STALE_REVISION")
	case errors.Is(err, registry.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "OPERATION_ID_CONFLICT")
	case errors.Is(err, registry.ErrRegistryUnavailable):
		writeError(w, http.StatusServiceUnavailable, "COMMUNITY_ROSTER_UNAVAILABLE")
	default:
		writeError(w, http.StatusServiceUnavailable, "AUTHORITY_UNAVAILABLE")
	}
}

var _ interface {
	ApplyCommunityRosterSnapshot(context.Context, registry.SessionPrincipal, registry.CommunityRosterSnapshotInput) (registry.CommunityRosterResult, error)
	GetCommunityRosterSnapshot(context.Context, registry.SessionPrincipal, string) (registry.CommunityRosterSnapshotView, error)
} = (*registry.SessionOrchestrator)(nil)
