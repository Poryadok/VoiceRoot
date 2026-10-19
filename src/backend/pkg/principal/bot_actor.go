package principal

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

const botActorType = "bot_actor"

// BotActorInput is a narrow capability issued by Bot from its current verified
// bot/installation/scope state. It never carries human account authority.
type BotActorInput struct {
	Audience, RPC, RequestID, RequestHash                    string
	ActorProfileID, BotID, SpaceID, InstallationID, BotScope string
}

func (i *Issuer) IssueBotActor(input BotActorInput) (string, error) {
	if i == nil || i.issuer != "bot" || !validBotActorIdentity(input) {
		return "", errors.New("invalid bot actor issuer or capability")
	}
	if err := validateBinding(input.Audience, input.RPC, input.RequestID, input.RequestHash); err != nil {
		return "", err
	}
	return i.issueAt(rawClaims{Type: botActorType, Issuer: "bot", Subject: "bot:" + input.BotID, Audience: input.Audience, RPC: input.RPC, RequestID: input.RequestID, RequestHash: input.RequestHash, ActorProfileID: input.ActorProfileID, BotID: input.BotID, SpaceID: input.SpaceID, InstallationID: input.InstallationID, BotScope: input.BotScope}, i.clock().UTC(), maxCredentialTTL, time.Time{})
}

func VerifyBotActor(ctx context.Context, token string, config VerifyConfig) (Principal, error) {
	if config.ExpectedIssuer != "bot" || config.ReplayGuard == nil {
		return Principal{}, errors.New("bot actor requires Bot issuer and replay guard")
	}
	claims, err := verify(ctx, token, config, botActorType)
	if err != nil {
		return Principal{}, err
	}
	input := BotActorInput{Audience: claims.Audience, RPC: claims.RPC, ActorProfileID: claims.ActorProfileID, BotID: claims.BotID, SpaceID: claims.SpaceID, InstallationID: claims.InstallationID, BotScope: claims.BotScope}
	if !validBotActorIdentity(input) || claims.Subject != "bot:"+claims.BotID || claims.AccountID != "" || claims.ProfileID != "" || claims.SessionEpoch != 0 || claims.AccountType != "" {
		return Principal{}, errors.New("invalid or mixed bot actor authority")
	}
	return principalFromClaims(claims), nil
}

func validBotActorIdentity(input BotActorInput) bool {
	for _, value := range []string{input.ActorProfileID, input.BotID, input.SpaceID, input.InstallationID} {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return false
		}
	}
	switch input.Audience {
	case "role":
		switch input.RPC {
		case "/voice.role.v1.RoleService/CreateRole":
			return input.BotScope == "SPACE_MANAGE_ROLES"
		case "/voice.role.v1.RoleService/AssignRole", "/voice.role.v1.RoleService/RevokeRole":
			return input.BotScope == "MEMBER_ASSIGN_ROLES"
		}
	case "chat":
		return input.RPC == "/voice.chat.v1.ChatService/CreateChat" && input.BotScope == "TEXT_CHAT_CREATE_IN_SPACE"
	}
	return false
}

func hasBotActorClaims(claims rawClaims) bool {
	return claims.ActorProfileID != "" || claims.BotID != "" || claims.SpaceID != "" || claims.InstallationID != "" || claims.BotScope != ""
}

func validAccountType(value string) bool {
	return value == "" || value == "regular" || value == "guest" || value == "sdk-account"
}
