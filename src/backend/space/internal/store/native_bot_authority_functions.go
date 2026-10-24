package store

import "voice/backend/pkg/authoritysource"

// Exact native42 owner function bodies; serving readiness is separate.
var botInstallationAuthorityFunctions = []authoritysource.CatalogFunction{
	{Name: "space_bot_installation_liveness_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "900ac652f9d824bce8d6eb9b47fb611d3520bf94c3cb8fa95742af7b30773120"},
	{Name: "space_bot_installation_attempt_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "7442060ff1f6c2637b8d9637fe33f2d52138c1a109dfd481dcea431103bfc28d"},
	{Name: "space_bot_installation_head_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "a03915f5203889d73483a07c88522976773439d7170f62bab8a210945f9f057d"},
	{Name: "space_bot_installation_cancellation_guard", Arguments: 0, ReturnType: "trigger", BodySHA256: "acab3507397acd82d9e05f872b214766800a3d9fdbd8ce00bad2f139a6103cf5"},
	{Name: "space_bot_installation_terminal_cleanup", Arguments: 0, ReturnType: "trigger", BodySHA256: "07ce0407d4210da16b190373accef4fcc865a7c329c20d0d642a2ffb6306c3b8"},
}
