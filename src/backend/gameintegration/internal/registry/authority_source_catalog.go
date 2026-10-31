package registry

import "voice/backend/pkg/authoritysource"

var gisAuthorityTables = []string{"applications", "environments", "player_bindings", "installations", "game_resource_mappings", "game_resource_binding_chats", "gis_sessions", "gis_app_notification_consents", "service_credentials"}
var gisAuthorityCatalog = func() authoritysource.Catalog {
	c := authoritysource.Catalog{Version: 33, Tables: append(append([]string{}, gisAuthorityTables...), "gis_authority_revision"), Triggers: []authoritysource.CatalogTrigger{
		{Table: "gis_authority_revision", Name: "gis_authority_floor_change", Function: "gis_authority_floor_guard", Type: 31},
		{Table: "gis_authority_revision", Name: "gis_authority_floor_truncate", Function: "gis_authority_floor_guard", Type: 34},
	}, Functions: []authoritysource.CatalogFunction{
		{Name: "gis_authority_floor_guard", ReturnType: "trigger", BodySHA256: "6ee87591548481e6e663df34b2ace732564c07cd129b439c31c37055fc4ad70b"},
		{Name: "gis_authority_changed", ReturnType: "trigger", BodySHA256: "2df3536b5e8688cf132121f4ae5e1d5901bbcb7809e4fa82710cb93f63846bd3"},
	}}
	for _, table := range gisAuthorityTables {
		c.Triggers = append(c.Triggers,
			authoritysource.CatalogTrigger{Table: table, Name: "gis_authority_changed", Function: "gis_authority_changed", Type: 28},
			authoritysource.CatalogTrigger{Table: table, Name: "gis_authority_no_truncate", Function: "gis_authority_floor_guard", Type: 34})
	}
	return c
}()
