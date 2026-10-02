// Code generated from the owner migration function bodies; DO NOT EDIT.
package store

import "voice/backend/pkg/authoritysource"

var roleAuthorityCatalog = authoritysource.Catalog{
	Version: 16,
	Tables:  []string{"chat_overrides", "member_roles", "ownership_transfer_v2", "role_space_deletion_fences", "role_space_lifecycle", "role_voice_policy_epochs", "role_voice_policy_outbox", "roles", "voice_room_overrides", "game_session_grant_sessions", "game_session_grants", "role_sdk_authority_revision"},
	Triggers: []authoritysource.CatalogTrigger{
		{Table: "roles", Name: "role_authority_revision_roles", Function: "role_authority_revision_changed", Type: 29, Argument: "space_id"},
		{Table: "roles", Name: "role_authority_revision_roles_truncate", Function: "role_authority_revision_floor_guard", Type: 34},
		{Table: "member_roles", Name: "role_authority_revision_member_roles", Function: "role_authority_revision_changed", Type: 29, Argument: "space_id"},
		{Table: "member_roles", Name: "role_authority_revision_member_roles_truncate", Function: "role_authority_revision_floor_guard", Type: 34},
		{Table: "ownership_transfer_v2", Name: "role_authority_revision_ownership_transfer_v2", Function: "role_authority_revision_changed", Type: 29, Argument: "space_id"},
		{Table: "ownership_transfer_v2", Name: "role_authority_revision_ownership_transfer_v2_truncate", Function: "role_authority_revision_floor_guard", Type: 34},
		{Table: "role_space_lifecycle", Name: "role_authority_revision_role_space_lifecycle", Function: "role_authority_revision_changed", Type: 29, Argument: "space_id"},
		{Table: "role_space_lifecycle", Name: "role_authority_revision_role_space_lifecycle_truncate", Function: "role_authority_revision_floor_guard", Type: 34},
		{Table: "role_space_deletion_fences", Name: "role_authority_revision_role_space_deletion_fences", Function: "role_authority_revision_changed", Type: 29, Argument: "space_id"},
		{Table: "role_space_deletion_fences", Name: "role_authority_revision_role_space_deletion_fences_truncate", Function: "role_authority_revision_floor_guard", Type: 34},
		{Table: "chat_overrides", Name: "role_authority_revision_chat_overrides", Function: "role_authority_revision_changed", Type: 29, Argument: "role_id"},
		{Table: "chat_overrides", Name: "role_authority_revision_chat_overrides_truncate", Function: "role_authority_revision_floor_guard", Type: 34},
		{Table: "voice_room_overrides", Name: "role_authority_revision_voice_room_overrides", Function: "role_authority_revision_changed", Type: 29, Argument: "role_id"},
		{Table: "voice_room_overrides", Name: "role_authority_revision_voice_room_overrides_truncate", Function: "role_authority_revision_floor_guard", Type: 34},
		{Table: "role_voice_policy_epochs", Name: "role_authority_revision_floor_change", Function: "role_authority_revision_floor_guard", Type: 31},
		{Table: "role_voice_policy_epochs", Name: "role_authority_revision_floor_truncate", Function: "role_authority_revision_floor_guard", Type: 34},
		{Table: "role_voice_policy_outbox", Name: "role_authority_revision_outbox_truncate", Function: "role_authority_revision_floor_guard", Type: 34},
		{Table: "role_sdk_authority_revision", Name: "role_sdk_authority_floor_change", Function: "role_sdk_authority_floor_guard", Type: 31},
		{Table: "role_sdk_authority_revision", Name: "role_sdk_authority_floor_truncate", Function: "role_sdk_authority_floor_guard", Type: 34},
		{Table: "game_session_grant_sessions", Name: "role_sdk_authority_sessions", Function: "role_sdk_authority_changed", Type: 28},
		{Table: "game_session_grants", Name: "role_sdk_authority_grants", Function: "role_sdk_authority_changed", Type: 28},
		{Table: "game_session_grant_sessions", Name: "role_sdk_authority_sessions_truncate", Function: "role_sdk_authority_floor_guard", Type: 34},
		{Table: "game_session_grants", Name: "role_sdk_authority_grants_truncate", Function: "role_sdk_authority_floor_guard", Type: 34},
	}, Functions: []authoritysource.CatalogFunction{
		{Name: "role_voice_policy_bump", Arguments: 3, ReturnType: "bigint", BodySHA256: "cfa8b3d1a003580fd28738ad5c404dff002f4fe0d7f775fc3a057fec1313d55a"},
		{Name: "role_authority_revision_floor_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "8c9dc242cd302fdf434075cd9b793d28ac126f2416c7d82125c820de7671c327"},
		{Name: "role_authority_revision_changed", Arguments: 0, ReturnType: "trigger", BodySHA256: "fc408928b9e427013dcc973e227b84f73d9ae04be3e4aba9bde2fee11b2999d4"},
		{Name: "role_sdk_authority_floor_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "9a8f5f22fef761b6d6a318f08cd758556ef74fccb2ebfb08ca22dc7ecaa6df58"},
		{Name: "role_sdk_authority_changed", Arguments: 0, ReturnType: "trigger", BodySHA256: "5506ca2bf08faa9c260fad3eff5d24e481b2eec3066fb8089d695b8341156383"},
	}}
