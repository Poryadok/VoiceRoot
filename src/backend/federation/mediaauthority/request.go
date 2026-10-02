package mediaauthority

import "voice/backend/federation/protocol"

// Request carries the already-authorized Voice caller tuple and expected
// canonical room/route. Master derives signing identity and node from its store;
// this request cannot choose issuer, node, authority epoch, TTL or signing key.
type Request struct {
	AccountID         string `json:"account_id"`
	ProfileID         string `json:"profile_id"`
	SpaceID           string `json:"space_id"`
	ResourceID        string `json:"resource_id"`
	RoomName          string `json:"room_name"`
	RoutingGeneration int64  `json:"routing_generation"`
	SessionEpoch      int64  `json:"session_epoch"`
	CanPublish        bool   `json:"can_publish"`
	ApplicationID     string `json:"application_id,omitempty"`
	EnvironmentID     string `json:"environment_id,omitempty"`
	BindingID         string `json:"binding_id,omitempty"`
	InstallationID    string `json:"installation_id,omitempty"`
}

func (r Request) Validate() error {
	if !canonicalID(r.AccountID) || !canonicalID(r.ProfileID) || !canonicalID(r.SpaceID) || !canonicalID(r.ResourceID) || !protocol.ValidRoomName(r.RoomName) || r.RoutingGeneration < 1 || r.SessionEpoch < 1 || !protocol.ValidApplicationScope(r.ApplicationID, r.EnvironmentID, r.BindingID, r.InstallationID) {
		return ErrDenied
	}
	return nil
}

type Result struct {
	Credential        string `json:"credential"`
	NodeID            string `json:"node_id"`
	NodeEndpoint      string `json:"node_endpoint"`
	SpaceID           string `json:"space_id"`
	ResourceID        string `json:"resource_id"`
	RoomName          string `json:"room_name"`
	RoutingGeneration int64  `json:"routing_generation"`
	ExpiresAt         int64  `json:"expires_at"`
}

// RouteRequest is accepted only from the trusted master Voice role after its
// canonical membership/Role checks. Application scope and routing generation
// are resolved from one unambiguous current master permission, never a client.
type RouteRequest struct {
	AccountID    string `json:"account_id"`
	ProfileID    string `json:"profile_id"`
	SpaceID      string `json:"space_id"`
	ResourceID   string `json:"resource_id"`
	RoomName     string `json:"room_name"`
	SessionEpoch int64  `json:"session_epoch"`
	CanPublish   bool   `json:"can_publish"`
}

func (r RouteRequest) Validate() error {
	return r.Request().Validate()
}

func (r RouteRequest) Request() Request {
	return Request{AccountID: r.AccountID, ProfileID: r.ProfileID, SpaceID: r.SpaceID, ResourceID: r.ResourceID, RoomName: r.RoomName, SessionEpoch: r.SessionEpoch, CanPublish: r.CanPublish, RoutingGeneration: 1}
}

type RouteResult struct {
	Version int      `json:"version"`
	Hosted  bool     `json:"hosted"`
	Request *Request `json:"request,omitempty"`
}
