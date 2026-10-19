package releasepolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// OwnerRuntime holds one immutable startup policy and its private output target.
// A snapshot proves this owner's binding only, never another service's guards.
type OwnerRuntime struct {
	policy       *Policy
	snapshotPath string
}

func LoadOwnerRuntime(lookup func(string) (string, bool), source string, readFile func(string) ([]byte, error)) (*OwnerRuntime, error) {
	policy, err := Load(lookup, source, readFile)
	if err != nil {
		return nil, err
	}
	path, set := lookup("VOICE_RELEASE_SNAPSHOT_FILE")
	if policy.Profile() == Alpha && (!set || !filepath.IsAbs(path)) {
		return nil, errPolicy
	}
	if policy.Profile() == Development && set {
		return nil, errPolicy
	}
	return &OwnerRuntime{policy: policy, snapshotPath: path}, nil
}

func (r *OwnerRuntime) Policy() *Policy {
	if r == nil {
		return nil
	}
	return r.policy
}

// WriteSnapshot requires the actual serving object's policy identity. The
// caller supplies only resolved nonsecret config and implemented route/guard IDs.
// Development has no release evidence and writes no snapshot.
func (r *OwnerRuntime) WriteSnapshot(service, instance string, serving *Policy, guards, routes []string, adapters map[string]string) error {
	if r == nil || r.policy == nil || serving != r.policy {
		return errPolicy
	}
	if r.policy.Profile() == Development {
		return nil
	}
	valid := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)
	if !valid.MatchString(service) || !valid.MatchString(instance) || len(guards) == 0 || len(routes) == 0 || len(adapters) == 0 {
		return errPolicy
	}
	routes = append([]string(nil), routes...)
	sort.Strings(routes)
	guards = append([]string(nil), guards...)
	sort.Strings(guards)
	projection, err := json.Marshal(adapters)
	if err != nil {
		return errPolicy
	}
	routeBytes, err := json.Marshal(routes)
	if err != nil {
		return errPolicy
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	snapshot := struct {
		SchemaVersion          int               `json:"schema_version"`
		ServiceID              string            `json:"service_id"`
		InstanceID             string            `json:"instance_id"`
		BuildSourceSHA         string            `json:"build_source_sha"`
		PolicySHA256           string            `json:"policy_sha256"`
		Profile                string            `json:"profile"`
		ConfigProjectionSHA256 string            `json:"config_projection_sha256"`
		CapabilityStates       map[string]string `json:"capability_states"`
		GuardIDs               []string          `json:"guard_ids"`
		RouteInventorySHA256   string            `json:"route_inventory_sha256"`
		AdapterModes           map[string]string `json:"adapter_modes"`
	}{2, service, instance, r.policy.SourceSHA(), r.policy.SHA256(), r.policy.Profile(), digest(projection), r.policy.CapabilityStates(), guards, digest(routeBytes), adapters}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return errPolicy
	}
	file, err := os.OpenFile(r.snapshotPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errPolicy
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		return errPolicy
	}
	return nil
}
