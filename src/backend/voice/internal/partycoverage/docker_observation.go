package partycoverage

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/google/uuid"
	"voice/backend/voice/internal/retirementproof"
)

type dockerFenceEvidence struct {
	Protocol     uint32
	DaemonID     string
	RetiredScope dockerScope
}

// observeDockerFence is a read-only component check against the daemon pinned
// in the protected plan. It cannot prove complete deployment/credential custody
// or the absence of external recreating controllers; the issuer must establish
// those separately. Empty scope and a replacement daemon are unavailable.
func observeDockerFence(ctx context.Context, api client.APIClient, daemonID string, scope dockerScope) ([]byte, [32]byte, error) {
	if api == nil || daemonID == "" || scope.EnvironmentID == uuid.Nil || scope.CohortID == uuid.Nil || len(scope.Targets) == 0 {
		return nil, [32]byte{}, ErrUnavailable
	}
	info, err := api.Info(ctx)
	if err != nil || info.ID == "" || info.ID != daemonID {
		return nil, [32]byte{}, ErrUnavailable
	}
	scope.Targets = append([]dockerTarget(nil), scope.Targets...)
	sort.Slice(scope.Targets, func(i, j int) bool { return scope.Targets[i].ID < scope.Targets[j].ID })
	for i, target := range scope.Targets {
		if len(target.ID) != 64 || len(strings.TrimPrefix(target.ImageID, "sha256:")) != 64 || (target.Resource != "media" && target.Resource != "writer") || (i > 0 && scope.Targets[i-1].ID == target.ID) {
			return nil, [32]byte{}, ErrUnavailable
		}
		if _, err := api.ContainerInspect(ctx, target.ID); !errdefs.IsNotFound(err) {
			return nil, [32]byte{}, ErrUnavailable
		}
	}
	selector := filters.NewArgs(filters.Arg("label", environmentLabel+"="+scope.EnvironmentID.String()), filters.Arg("label", cohortLabel+"="+scope.CohortID.String()))
	remaining, err := api.ContainerList(ctx, container.ListOptions{All: true, Filters: selector})
	if err != nil || len(remaining) != 0 {
		return nil, [32]byte{}, ErrUnavailable
	}
	final, err := api.Info(ctx)
	if err != nil || final.ID != daemonID {
		return nil, [32]byte{}, ErrUnavailable
	}
	raw, err := json.Marshal(dockerFenceEvidence{Protocol: 1, DaemonID: daemonID, RetiredScope: scope})
	if err != nil {
		return nil, [32]byte{}, err
	}
	return raw, retirementproof.Hash("voice.party.docker-fence-observation.v1", raw), nil
}
