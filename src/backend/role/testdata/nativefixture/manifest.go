// Package nativefixture describes only the disposable Linux integration fixture.
// It is not a production authorization, membership or deployment protocol.
package nativefixture

import "time"

type Manifest struct {
	SchemaVersion         int       `json:"schema_version"`
	FixtureOnly           bool      `json:"fixture_only"`
	RoleSource            string    `json:"role_source"`
	SpaceSource           string    `json:"space_source"`
	RoleSchema            int64     `json:"role_schema"`
	SpaceSchema           int64     `json:"space_schema"`
	RoleAddress           string    `json:"role_address"`
	TLSServerName         string    `json:"tls_server_name"`
	TLSCAFile             string    `json:"tls_ca_file"`
	TLSClientCertFile     string    `json:"tls_client_cert_file"`
	TLSClientKeyFile      string    `json:"tls_client_key_file"`
	BotSignerKeyFile      string    `json:"bot_signer_key_file"`
	BotSignerKeyID        string    `json:"bot_signer_key_id"`
	BotJWKSURL            string    `json:"bot_jwks_url"`
	OwnerProfileID        string    `json:"owner_profile_id"`
	OwnerAccountID        string    `json:"owner_account_id"`
	SpaceID               string    `json:"space_id"`
	SpaceServingDSN       string    `json:"space_serving_dsn"`
	RoleDatabase          string    `json:"role_database"`
	SpaceDatabase         string    `json:"space_database"`
	RoleDatabaseSystemID  string    `json:"role_database_system_id"`
	SpaceDatabaseSystemID string    `json:"space_database_system_id"`
	ExpiresAt             time.Time `json:"expires_at"`
	StopFile              string    `json:"stop_file"`
}
