package main

import (
	"errors"
	"google.golang.org/grpc"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/messaging/internal/s2s"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spacecommunication"
)

func newMessageViewDeletionClient(lookup func(string) (string, bool), issuer *principal.Issuer) (*s2s.UserGRPCProfileDeletion, *grpc.ClientConn, error) {
	config, enabled, err := spacecommunication.LoadClientConfig(lookup, "MESSAGING_USER_DELETION_")
	if err != nil {
		return nil, nil, err
	}
	if !enabled {
		return nil, nil, nil
	}
	if issuer == nil {
		return nil, nil, errors.New("profile view compaction requires Messaging signing keys and protected JWKS publisher")
	}
	conn, err := config.Dial()
	if err != nil {
		return nil, nil, err
	}
	return &s2s.UserGRPCProfileDeletion{Client: userv1.NewUserServiceClient(conn), Issuer: issuer}, conn, nil
}
