package main

import (
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/sessionfloor"
)

func loadRoleSessionFloor(issuer *principal.Issuer) (principal.SessionEpochChecker, func(), error) {
	config, enabled, err := sessionfloor.ConfigFromEnv()
	noop := func() {}
	if err != nil || !enabled {
		return nil, noop, err
	}
	client, connection, err := config.Dial(issuer)
	if err != nil {
		return nil, noop, err
	}
	return client.RequireCurrent, func() { _ = connection.Close() }, nil
}
