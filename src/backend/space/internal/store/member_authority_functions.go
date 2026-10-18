// Owner migration33 function body fingerprints. Regenerate after changing its SQL.
package store

import "voice/backend/pkg/authoritysource"

var memberAuthorityFunctions = []authoritysource.CatalogFunction{
	{Name: "space_membership_ownership_changed", Arguments: 0, ReturnType: "trigger", BodySHA256: "7a7fc9225f4c9e88299947c363bb60e8b282786c979b6a2221b9d72c68b12a9f"},
	{Name: "space_membership_ownership_head_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "f422bca5669ebc2f0abdf6c69b63fcc7b3ca25cae9fce5e50ace87aebfd812ab"},
	{Name: "space_membership_terminal_cleanup_allowed", Arguments: 1, ReturnType: "boolean", BodySHA256: "34f62a88777b36e9c762cacdfd3cb869941179b2254d08e57741dc653cff3be7"},
	{Name: "space_member_head_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "e2f059482b172def5293b41d56036faa55a022e04e607d6f0d8c84d604223f15"},
	{Name: "space_ordinary_publication_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "c98febaa293e68388551ca226c7f49a31bb6ea7f821ed62636e51be82404514b"},
	{Name: "space_community_episode_changed", Arguments: 0, ReturnType: "trigger", BodySHA256: "e339e3487508e424a07937281530a177ead533d0bfda0492250021bfbc18d1a0"},
	{Name: "space_community_owner_episode_changed", Arguments: 0, ReturnType: "trigger", BodySHA256: "8519c2323273a7d9750d01baf55c8fe4fa431c2dfb71a08183af4234770eef97"},
	{Name: "space_membership_terminal_cleanup", Arguments: 0, ReturnType: "trigger", BodySHA256: "98f5f7bb949c65b078f914d61fb848817ac2d2866b90b232d59bf9ca79902a32"},
	{Name: "space_member_admission_event_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "d5f42f4a50453c5c3f04c78cb9a5734c0ec3b1062b8175248e975fcc3568232e"},
	{Name: "space_native_member_publication_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "032faf0d8e0b171f164515d4a557f2fd4a2e182316f8489e741d895c4d556260"},
}
