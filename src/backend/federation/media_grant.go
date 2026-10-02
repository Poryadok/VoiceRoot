package main

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/federation/mediaauthority"
)

var errMediaNotHosted = errors.New("media resource is not hosted")

func (s *authorityStore) issueMediaGrant(ctx context.Context, request mediaauthority.Request) (mediaauthority.Result, error) {
	return s.issueMediaGrantWithRoute(ctx, request, nil)
}

func (s *authorityStore) resolveMediaRoute(ctx context.Context, discovery mediaauthority.RouteRequest) (mediaauthority.RouteResult, error) {
	if discovery.Validate() != nil {
		return mediaauthority.RouteResult{}, errInvalid
	}
	var resolved mediaauthority.Request
	_, err := s.issueMediaGrantWithRoute(ctx, discovery.Request(), &resolved)
	if errors.Is(err, errMediaNotHosted) {
		return mediaauthority.RouteResult{Version: 1, Hosted: false}, nil
	}
	if err != nil {
		return mediaauthority.RouteResult{}, err
	}
	return mediaauthority.RouteResult{Version: 1, Hosted: true, Request: &resolved}, nil
}

func (s *authorityStore) issueMediaGrantWithRoute(ctx context.Context, request mediaauthority.Request, resolved *mediaauthority.Request) (mediaauthority.Result, error) {
	if request.Validate() != nil {
		return mediaauthority.Result{}, errInvalid
	}
	var home, mappedSpace string
	// This read selects a lock target only. All authority is reread under the
	// same node -> placement lock order as publication/revocation/registration.
	err := s.Pool.QueryRow(ctx, `SELECT home_node_id::text,space_id::text FROM federation_hosted_resources
		WHERE resource_id=$1 ORDER BY routing_generation DESC LIMIT 1`, request.ResourceID).Scan(&home, &mappedSpace)
	if errors.Is(err, pgx.ErrNoRows) {
		if resolved != nil {
			return mediaauthority.Result{}, errMediaNotHosted
		}
		return mediaauthority.Result{}, errForbidden
	}
	if err != nil {
		return mediaauthority.Result{}, err
	}
	if mappedSpace != request.SpaceID {
		return mediaauthority.Result{}, errForbidden
	}
	var result mediaauthority.Result
	err = s.transaction(ctx, func(tx pgx.Tx) error {
		node, err := lockNode(ctx, tx, home, s.Environment)
		if err != nil {
			return err
		}
		var now time.Time
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return err
		}
		if node.Status != "active" || !now.Before(node.Expiry) {
			return errForbidden
		}
		var generation, revision int64
		var raw []byte
		var hash *string
		var until *time.Time
		if err = tx.QueryRow(ctx, `SELECT generation,revision,snapshot,snapshot_hash,valid_until
			FROM federation_placements WHERE space_id=$1 AND node_id=$2 FOR UPDATE`, request.SpaceID, home).
			Scan(&generation, &revision, &raw, &hash, &until); errors.Is(err, pgx.ErrNoRows) {
			return errForbidden
		} else if err != nil {
			return err
		}
		if until == nil || hash == nil || generation < 1 || revision < 1 || !now.Before(*until) || digest(raw) != *hash {
			return errForbidden
		}
		var route int64
		var kind, state, room, placedHome string
		var capabilities []string
		if err = tx.QueryRow(ctx, `SELECT routing_generation,resource_type,lifecycle_state,room_name,home_node_id::text,capabilities
			FROM federation_hosted_resources WHERE resource_id=$1 AND space_id=$2 ORDER BY routing_generation DESC LIMIT 1 FOR UPDATE`, request.ResourceID, request.SpaceID).
			Scan(&route, &kind, &state, &room, &placedHome, &capabilities); errors.Is(err, pgx.ErrNoRows) {
			return errForbidden
		} else if err != nil {
			return err
		}
		if (resolved == nil && route != request.RoutingGeneration) || kind != "voice_room" || state != "active" || room != request.RoomName || placedHome != home || !slices.Contains(capabilities, "voice") {
			return errForbidden
		}
		var bound bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM federation_voice_room_bindings WHERE resource_id=$1 AND node_id=$2 AND space_id=$3 AND room_name=$4)`, request.ResourceID, home, request.SpaceID, room).Scan(&bound); err != nil {
			return err
		}
		if !bound {
			return errForbidden
		}
		var snapshot Snapshot
		if strictJSON(raw, &snapshot) != nil || snapshot.Validate(now) != nil || snapshot.Revision != revision || snapshot.ValidUntil != until.UnixMilli() {
			return errForbidden
		}
		allowed := false
		for _, p := range snapshot.Permissions {
			if p.AccountID == request.AccountID && p.ProfileID == request.ProfileID && p.ResourceID == request.ResourceID && p.SessionEpoch == request.SessionEpoch && p.RoutingGeneration == route && p.RoomName == room &&
				(resolved != nil || (p.ApplicationID == request.ApplicationID && p.EnvironmentID == request.EnvironmentID && p.BindingID == request.BindingID && p.InstallationID == request.InstallationID)) && slices.Contains(p.Actions, "media") && (!request.CanPublish || slices.Contains(p.Actions, "media_publish")) {
				if resolved != nil && allowed {
					return errForbidden
				}
				allowed = true
				if resolved == nil {
					break
				}
				request.ApplicationID, request.EnvironmentID, request.BindingID, request.InstallationID = p.ApplicationID, p.EnvironmentID, p.BindingID, p.InstallationID
				request.RoutingGeneration = route
			}
		}
		if !allowed {
			return errForbidden
		}
		var endpoint string
		if err = tx.QueryRow(ctx, `SELECT endpoint FROM federation_nodes WHERE id=$1 AND environment=$2`, home, s.Environment).Scan(&endpoint); err != nil {
			return err
		}
		// Re-sample after every potential lock/query wait. A request cannot mint
		// from a source or node credential that elapsed while it was blocked.
		if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
			return err
		}
		if snapshot.Validate(now) != nil || !now.Before(*until) || !now.Before(node.Expiry) {
			return errForbidden
		}
		if resolved != nil {
			*resolved = request
			return nil
		}
		expires := min(now.Add(mediaauthority.MaxGrantValidity).UnixMilli(), node.Expiry.UnixMilli())
		grant := mediaauthority.Grant{Version: 1, Issuer: s.Issuer, Audience: "voice-node-media", Environment: s.Environment, NodeID: home, SpaceID: request.SpaceID, Generation: generation, AuthorityEpoch: node.Epoch,
			AccountID: request.AccountID, ProfileID: request.ProfileID, ResourceID: request.ResourceID, SessionEpoch: request.SessionEpoch, RoomName: room, CanPublish: request.CanPublish, RoutingGeneration: route, Nonce: uuid.NewString(),
			ApplicationID: request.ApplicationID, EnvironmentID: request.EnvironmentID, BindingID: request.BindingID, InstallationID: request.InstallationID, IssuedAt: now.UnixMilli(), ExpiresAt: expires}
		credential, err := mediaauthority.Sign(s.Key, s.KeyID, grant, now)
		if err != nil {
			return err
		}
		result = mediaauthority.Result{Credential: credential, NodeID: home, NodeEndpoint: endpoint, SpaceID: request.SpaceID, ResourceID: request.ResourceID, RoomName: room, RoutingGeneration: route, ExpiresAt: expires}
		return nil
	})
	return result, err
}

func (s *authorityStore) mediaIssuerPinAvailable(ctx context.Context, pin string) (bool, error) {
	var node bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM federation_nodes WHERE certificate_sha256=$1)`, pin).Scan(&node)
	return !node, err
}
