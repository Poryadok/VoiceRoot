package main

import (
	"context"
	"slices"

	"github.com/jackc/pgx/v5"
)

// Called with the node and placement locked, matching resource registration.
// Immutable latest routes are read in one bounded query, not once per actor.
func validateRoutedPolicy(ctx context.Context, tx pgx.Tx, node, space string, snapshot Snapshot) error {
	ids := []string{}
	seen := map[string]bool{}
	for _, p := range snapshot.Permissions {
		if !seen[p.ResourceID] {
			ids = append(ids, p.ResourceID)
			seen[p.ResourceID] = true
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT ON(r.resource_id) r.resource_id::text,r.space_id::text,r.home_node_id::text,
		r.routing_generation,r.lifecycle_state,r.resource_type,r.room_name,r.capabilities,
		EXISTS(SELECT 1 FROM federation_hosted_resources f WHERE f.resource_id=r.resource_id AND f.lifecycle_state IN('purging','tombstoned'))
		FROM federation_hosted_resources r WHERE r.resource_id=ANY($1::uuid[]) ORDER BY r.resource_id,r.routing_generation DESC`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	routes := map[string]HostedResourceMapping{}
	for rows.Next() {
		var r HostedResourceMapping
		var retired bool
		if err = rows.Scan(&r.ResourceID, &r.SpaceID, &r.HomeNodeID, &r.RoutingGeneration, &r.LifecycleState, &r.ResourceType, &r.RoomName, &r.Capabilities, &retired); err != nil {
			return err
		}
		if retired {
			return errForbidden
		}
		routes[r.ResourceID] = r
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, p := range snapshot.Permissions {
		if p.RoutingGeneration == 0 {
			continue
		}
		r, ok := routes[p.ResourceID]
		if !ok || r.SpaceID != space || r.HomeNodeID != node || r.RoutingGeneration != p.RoutingGeneration || r.LifecycleState != "active" {
			return errForbidden
		}
		if slices.Contains(p.Actions, "media") && (r.ResourceType != "voice_room" || !slices.Contains(r.Capabilities, "voice") || p.RoomName != r.RoomName) {
			return errForbidden
		}
	}
	return nil
}

// Retirement is permanent even when pre-repair append-only history contains a
// later active row. Never rewrite that evidence to make the latest row safe.
func resourcePurgeFence(ctx context.Context, tx pgx.Tx, resource string) (bool, bool, error) {
	var purging, tombstoned bool
	err := tx.QueryRow(ctx, `SELECT coalesce(bool_or(lifecycle_state='purging'),false),coalesce(bool_or(lifecycle_state='tombstoned'),false) FROM federation_hosted_resources WHERE resource_id=$1`, resource).Scan(&purging, &tombstoned)
	return purging, tombstoned, err
}
