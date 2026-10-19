// Native39 and canonical participant11 migration34 function body fingerprints.
package store

import "voice/backend/pkg/authoritysource"

var memberAuthorityFunctions = []authoritysource.CatalogFunction{
	{Name: "space_terminal_purge_ready", Arguments: 1, ReturnType: "boolean", BodySHA256: "7521d5ff86a3598c19cab651792fc49042e06997b8415df3ab3ae6ba9a82a2f3"},
	{Name: "space_member_admission_intent_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "5d0447c25b32c2f58769d0c32b89cf21610a5ea8b4e7e39e9ca68d47f11e2511"},
	{Name: "space_membership_ownership_changed", Arguments: 0, ReturnType: "trigger", BodySHA256: "7a7fc9225f4c9e88299947c363bb60e8b282786c979b6a2221b9d72c68b12a9f"},
	{Name: "space_membership_ownership_head_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "f422bca5669ebc2f0abdf6c69b63fcc7b3ca25cae9fce5e50ace87aebfd812ab"},
	{Name: "space_membership_terminal_cleanup_allowed", Arguments: 1, ReturnType: "boolean", BodySHA256: "34f62a88777b36e9c762cacdfd3cb869941179b2254d08e57741dc653cff3be7"},
	{Name: "space_member_head_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "e2f059482b172def5293b41d56036faa55a022e04e607d6f0d8c84d604223f15"},
	{Name: "space_ordinary_publication_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "c98febaa293e68388551ca226c7f49a31bb6ea7f821ed62636e51be82404514b"},
	{Name: "space_community_episode_changed", Arguments: 0, ReturnType: "trigger", BodySHA256: "e339e3487508e424a07937281530a177ead533d0bfda0492250021bfbc18d1a0"},
	{Name: "space_community_owner_episode_changed", Arguments: 0, ReturnType: "trigger", BodySHA256: "8519c2323273a7d9750d01baf55c8fe4fa431c2dfb71a08183af4234770eef97"},
	{Name: "space_membership_terminal_cleanup", Arguments: 0, ReturnType: "trigger", BodySHA256: "a718c93ac15537dc22f8913ba03f2a48fff6ad089d7f82c30eb7a4229abfd745"},
	{Name: "space_member_admission_event_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "d5f42f4a50453c5c3f04c78cb9a5734c0ec3b1062b8175248e975fcc3568232e"},
	{Name: "space_native_member_publication_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "032faf0d8e0b171f164515d4a557f2fd4a2e182316f8489e741d895c4d556260"},
	{Name: "space_member_role_cleanup_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "1ea9af53c1c2026d0b4ad260b2edb9fc7b22756873250fd0f6055e40ed0c1f01"},
	{Name: "space_creator_principal_replay_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "40272feb45f11dfdbce41e02165e9bebbbfde4e19ce0dba9abfb1e2d8bd77d38"},
	{Name: "space_native_creator_operation_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "d2d4633d4c9453ba8733c5ba6c01676d291759b3e504974a4a5ee68a9b519129"},
}
