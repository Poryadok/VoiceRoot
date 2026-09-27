package controlledgame

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	errPermitDenied        = errors.New("execution permit denied")
	errPermitUnavailable   = errors.New("execution permit authority unavailable")
	errAuthorizationDenied = errors.New("controlled game authorization denied")
	errPermitWindowExpired = errors.New("execution permit window expired")
)

// PermitRequest carries the command and receiver-owned binding inputs needed
// by the internal authority seam. Actor proof remains opaque to this module.
type PermitRequest struct {
	CommandID       string
	OperationID     string
	InvocationID    string
	ActionID        string
	MessageID       string
	AppID           string
	EnvironmentID   string
	InstallationID  string
	ProfileID       string
	ProofID         string
	BindingRevision int64
}

// ExecutionPermit is the stable result of the first committed admission for a
// command. Retries reuse this identity and epoch.
type ExecutionPermit struct {
	PermitID       string
	PermitIssuedAt int64
}

// PermitAuthority is an internal seam; T07a does not expose it as an HTTP
// client or connect it to a production GIS registry.
type PermitAuthority interface {
	Admit(context.Context, PermitRequest) (ExecutionPermit, error)
}

type BindingAuthorizer func(context.Context, pgx.Tx, callbackCommand) error

const (
	permitStartWindow      = 10 * time.Second
	permitCompletionWindow = 60 * time.Second
)
