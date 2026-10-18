package partycoverage

import (
	"context"
	"encoding/json"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/google/uuid"
	"sort"
	"strings"
)

const environmentLabel = "voice.party.cutover.environment"
const cohortLabel = "voice.party.cutover.cohort"
const resourceLabel = "voice.party.cutover.resource"

type dockerTarget struct{ ID, ImageID, Resource string }
type dockerScope struct {
	EnvironmentID, CohortID uuid.UUID
	Targets                 []dockerTarget
}

// This adapter supports explicitly owned isolated Docker execution/media
// cohorts. It grants no authority over shared Compose/Kubernetes providers.
func retireDockerCohort(ctx context.Context, c client.APIClient, scope dockerScope) ([]byte, error) {
	if c == nil || scope.EnvironmentID == uuid.Nil || scope.CohortID == uuid.Nil {
		return nil, ErrUnavailable
	}
	targets := append([]dockerTarget(nil), scope.Targets...)
	sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })
	expected := map[string]dockerTarget{}
	for _, target := range targets {
		if len(target.ID) != 64 || len(strings.TrimPrefix(target.ImageID, "sha256:")) != 64 || (target.Resource != "media" && target.Resource != "writer") {
			return nil, ErrUnavailable
		}
		if _, duplicate := expected[target.ID]; duplicate {
			return nil, ErrUnavailable
		}
		expected[target.ID] = target
	}
	selector := filters.NewArgs(filters.Arg("label", environmentLabel+"="+scope.EnvironmentID.String()), filters.Arg("label", cohortLabel+"="+scope.CohortID.String()))
	live, err := c.ContainerList(ctx, container.ListOptions{All: true, Filters: selector})
	if err != nil {
		return nil, err
	}
	// Inspect the complete scope before stopping anything. A missing producer
	// makes this a partial package, not an invitation to kill the known subset.
	for _, item := range live {
		if _, ok := expected[item.ID]; !ok {
			return nil, ErrConflict
		}
	}
	for _, target := range targets {
		info, err := c.ContainerInspect(ctx, target.ID)
		if errdefs.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.Config == nil || info.State == nil || info.HostConfig == nil || info.Image != target.ImageID ||
			info.Config.Labels[environmentLabel] != scope.EnvironmentID.String() || info.Config.Labels[cohortLabel] != scope.CohortID.String() || info.Config.Labels[resourceLabel] != target.Resource ||
			(info.HostConfig.RestartPolicy.Name != "" && info.HostConfig.RestartPolicy.Name != "no") {
			return nil, ErrConflict
		}
	}
	// Whole old-realm retirement ends established sessions and makes the exact
	// old endpoint/container impossible to restart. Removing participants or
	// rotating signing keys alone would not establish those two postconditions.
	for _, target := range targets {
		info, err := c.ContainerInspect(ctx, target.ID)
		if errdefs.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.State.Running {
			seconds := 10
			if err = c.ContainerStop(ctx, target.ID, container.StopOptions{Timeout: &seconds}); err != nil {
				return nil, err
			}
		}
		if err = c.ContainerRemove(ctx, target.ID, container.RemoveOptions{}); err != nil && !errdefs.IsNotFound(err) {
			return nil, err
		}
	}
	for _, target := range targets {
		_, err = c.ContainerInspect(ctx, target.ID)
		if !errdefs.IsNotFound(err) {
			if err != nil {
				return nil, err
			}
			return nil, ErrUnavailable
		}
	}
	remaining, err := c.ContainerList(ctx, container.ListOptions{All: true, Filters: selector})
	if err != nil {
		return nil, err
	}
	if len(remaining) != 0 {
		return nil, ErrUnavailable
	}
	scope.Targets = targets
	// Exact plan identity is stable across a crash after removal/lost ack. This
	// proves only this isolated resource adapter; native drain/validation follows.
	return json.Marshal(struct {
		Protocol     int
		RetiredScope dockerScope
	}{1, scope})
}
