// Code generated from owning migration function bodies; DO NOT EDIT.
package store

import "voice/backend/pkg/authoritysource"

var userAuthorityCatalog = authoritysource.Catalog{Version: 19, Tables: []string{"profiles", "user_account_lifecycle", "sdk_author_tombstones", "user_authority_revision"}, Triggers: []authoritysource.CatalogTrigger{
	{Table: "profiles", Name: "profiles_sdk_eligibility_revision", Function: "bump_sdk_eligibility_revision", Type: 19},
	{Table: "sdk_author_tombstones", Name: "sdk_author_tombstones_no_update_delete", Function: "reject_sdk_author_tombstone_mutation", Type: 27},
	{Table: "sdk_author_tombstones", Name: "sdk_author_tombstones_no_truncate", Function: "reject_sdk_author_tombstone_mutation", Type: 34},
	{Table: "user_authority_revision", Name: "user_authority_floor_change", Function: "user_authority_floor_guard", Type: 31},
	{Table: "user_authority_revision", Name: "user_authority_floor_truncate", Function: "user_authority_floor_guard", Type: 34},
	{Table: "profiles", Name: "user_authority_profiles", Function: "user_authority_changed", Type: 28},
	{Table: "user_account_lifecycle", Name: "user_authority_inactive", Function: "user_authority_changed", Type: 28},
	{Table: "sdk_author_tombstones", Name: "user_authority_tombstones", Function: "user_authority_changed", Type: 28},
	{Table: "profiles", Name: "user_authority_profiles_truncate", Function: "user_authority_floor_guard", Type: 34},
	{Table: "user_account_lifecycle", Name: "user_account_lifecycle_no_update_delete", Function: "reject_user_inactive_mutation", Type: 27},
	{Table: "user_account_lifecycle", Name: "user_account_lifecycle_no_truncate", Function: "reject_user_inactive_mutation", Type: 34},
}, Functions: []authoritysource.CatalogFunction{
	{Name: "bump_sdk_eligibility_revision", Arguments: 0, ReturnType: "trigger", BodySHA256: "f4ec491a2668192d3e192546705ee6c6e2ab192324b833193ba5fba5876cea36"},
	{Name: "reject_sdk_author_tombstone_mutation", Arguments: 0, ReturnType: "trigger", BodySHA256: "83dea7b72d755a134949c14ccb8ca852272ac95b797220e4f5bdc9a0194e529b"},
	{Name: "user_authority_floor_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "a69ba01bacff05f6b7363be53f5310f33af85e83f498805a15ff243751fe4567"},
	{Name: "user_authority_changed", Arguments: 0, ReturnType: "trigger", BodySHA256: "7e5d1cfb37caec3a57bdbab0ba763a2809aa1f331d18bb2e5587b1fe84a6b62f"},
	{Name: "reject_user_inactive_mutation", Arguments: 0, ReturnType: "trigger", BodySHA256: "f07424675de7e20eb12cde243d108df8aba5eb439085c9f35966f5d5b5ca8ab3"},
}}
