package main

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/federation/protocol"
)

var errInvalidHostedResource = errors.New("invalid hosted resource mapping")

// HostedResourceMapping keeps the canonical resource UUID stable while a
// Space's owner records append-only route generations for it.
type HostedResourceMapping struct {
	ResourceID        string    `json:"resource_id"`
	ResourceType      string    `json:"resource_type"`
	SpaceID           string    `json:"space_id"`
	HomeNodeID        string    `json:"home_node_id"`
	RoutingGeneration int64     `json:"routing_generation"`
	LifecycleState    string    `json:"lifecycle_state"`
	Capabilities      []string  `json:"capabilities"`
	RoomName          string    `json:"room_name,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

func (s *authorityStore) registerHostedResource(ctx context.Context, input HostedResourceMapping) (HostedResourceMapping, error) {
	mapping, err := normalizeHostedResource(input)
	if err != nil {
		return HostedResourceMapping{}, err
	}
	err = s.transaction(ctx, func(tx pgx.Tx) error {
		node, err := lockNode(ctx, tx, mapping.HomeNodeID, s.Environment)
		if err != nil {
			return err
		}
		if node.Status != "active" {
			return errForbidden
		}
		var placedNode string
		if err := tx.QueryRow(ctx, `SELECT node_id::text FROM federation_placements
			WHERE space_id=$1 AND node_id=$2 FOR UPDATE`, mapping.SpaceID, mapping.HomeNodeID).Scan(&placedNode); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errForbidden
			}
			return err
		}
		var current HostedResourceMapping
		var currentCapabilities []string
		lookupErr := tx.QueryRow(ctx, `SELECT resource_id::text,resource_type,space_id::text,home_node_id::text,
			routing_generation,lifecycle_state,capabilities,room_name,created_at
			FROM federation_hosted_resources WHERE resource_id=$1
			ORDER BY routing_generation DESC LIMIT 1 FOR UPDATE`, mapping.ResourceID).
			Scan(&current.ResourceID, &current.ResourceType, &current.SpaceID, &current.HomeNodeID,
				&current.RoutingGeneration, &current.LifecycleState, &currentCapabilities, &current.RoomName, &current.CreatedAt)
		if lookupErr == nil {
			current.Capabilities = currentCapabilities
			if mapping.RoutingGeneration == current.RoutingGeneration {
				if sameHostedResource(current, mapping) {
					mapping = current
					return nil
				}
				return errConflict
			}
			if mapping.RoutingGeneration != current.RoutingGeneration+1 ||
				mapping.ResourceType != current.ResourceType || mapping.SpaceID != current.SpaceID ||
				mapping.HomeNodeID != current.HomeNodeID || (current.RoomName != "" && mapping.RoomName != current.RoomName) {
				return errConflict
			}
		} else if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return lookupErr
		} else if mapping.RoutingGeneration != 1 {
			return errConflict
		}

		if mapping.ResourceType == "voice_room" {
			if _, err := tx.Exec(ctx, `INSERT INTO federation_voice_room_bindings
				(resource_id,node_id,space_id,room_name) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
				mapping.ResourceID, mapping.HomeNodeID, mapping.SpaceID, mapping.RoomName); err != nil {
				return err
			}
			var boundNode, boundSpace, boundRoom string
			if err := tx.QueryRow(ctx, `SELECT node_id::text,space_id::text,room_name
				FROM federation_voice_room_bindings WHERE resource_id=$1`, mapping.ResourceID).
				Scan(&boundNode, &boundSpace, &boundRoom); errors.Is(err, pgx.ErrNoRows) {
				return errConflict
			} else if err != nil {
				return err
			}
			if boundNode != mapping.HomeNodeID || boundSpace != mapping.SpaceID || boundRoom != mapping.RoomName {
				return errConflict
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO federation_hosted_resources
			(resource_id,routing_generation,resource_type,space_id,home_node_id,lifecycle_state,capabilities,room_name)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, mapping.ResourceID, mapping.RoutingGeneration,
			mapping.ResourceType, mapping.SpaceID, mapping.HomeNodeID, mapping.LifecycleState, mapping.Capabilities, mapping.RoomName)
		if err != nil {
			if isUniqueViolation(err) {
				return errConflict
			}
			return err
		}
		return tx.QueryRow(ctx, `SELECT created_at FROM federation_hosted_resources WHERE resource_id=$1 AND routing_generation=$2`,
			mapping.ResourceID, mapping.RoutingGeneration).Scan(&mapping.CreatedAt)
	})
	return mapping, err
}

func normalizeHostedResource(input HostedResourceMapping) (HostedResourceMapping, error) {
	for _, value := range []string{input.ResourceID, input.SpaceID, input.HomeNodeID} {
		parsed, err := uuid.Parse(value)
		if err != nil || parsed == uuid.Nil || parsed.String() != value {
			return HostedResourceMapping{}, errInvalidHostedResource
		}
	}
	if input.RoutingGeneration <= 0 || !oneOf(input.ResourceType, "space_content", "chat", "file", "search_index", "voice_room") ||
		!oneOf(input.LifecycleState, "active", "frozen", "purging", "tombstoned") || len(input.Capabilities) == 0 || len(input.Capabilities) > 4 {
		return HostedResourceMapping{}, errInvalidHostedResource
	}
	if input.ResourceType == "voice_room" {
		if !protocol.ValidRoomName(input.RoomName) || !slices.Contains(input.Capabilities, "voice") {
			return HostedResourceMapping{}, errInvalidHostedResource
		}
	} else if input.RoomName != "" {
		return HostedResourceMapping{}, errInvalidHostedResource
	}
	capabilities := slices.Clone(input.Capabilities)
	for _, capability := range capabilities {
		if !oneOf(capability, "messaging", "file", "search", "voice") {
			return HostedResourceMapping{}, errInvalidHostedResource
		}
	}
	slices.Sort(capabilities)
	for i := 1; i < len(capabilities); i++ {
		if capabilities[i] == capabilities[i-1] {
			return HostedResourceMapping{}, errInvalidHostedResource
		}
	}
	input.Capabilities = capabilities
	input.CreatedAt = time.Time{}
	return input, nil
}

func sameHostedResource(a, b HostedResourceMapping) bool {
	left, _ := json.Marshal(a.Capabilities)
	right, _ := json.Marshal(b.Capabilities)
	return a.ResourceID == b.ResourceID && a.ResourceType == b.ResourceType && a.SpaceID == b.SpaceID &&
		a.HomeNodeID == b.HomeNodeID && a.RoutingGeneration == b.RoutingGeneration &&
		a.LifecycleState == b.LifecycleState && a.RoomName == b.RoomName && string(left) == string(right)
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
