package sessionowners

import (
	"voice/backend/gameintegration/internal/registry"
	"voice/backend/pkg/gisowner"
)

func classifyOwnerError(err error, request registry.SessionOwnerRequest, domain, rpc string) error {
	category, ok := gisowner.Match(err, domain, rpc, request.OperationID.String(), request.RequestHash)
	if !ok {
		return err
	}
	return &registry.PermanentOwnerRejection{Category: category}
}
