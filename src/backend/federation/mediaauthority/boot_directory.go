package mediaauthority

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"voice/backend/federation/protocol"
)

const bootHeartbeatTTL = time.Second
const maxBootRequestFiles = 4096
const maxBootRequestBytes = 16 << 10

type bootRequest struct {
	Version     int    `json:"version"`
	Issuer      string `json:"issuer"`
	Environment string `json:"environment"`
	NodeID      string `json:"node_id"`
	Nonce       string `json:"receiver_boot_nonce"`
	IssuedAt    int64  `json:"issued_at"`
	ExpiresAt   int64  `json:"expires_at"`
}

// BootHeartbeat holds no master credential. Its UUID was generated inside the
// receiver process; the unsigned request only asks the controller for a fresh
// master lease. It cannot establish authority without that signed round trip.
type BootHeartbeat struct {
	mu        sync.Mutex
	registry  *Registry
	directory string
	closed    bool
}

func bootDirectory(directory string) bool {
	if !filepath.IsAbs(directory) {
		return false
	}
	info, err := os.Lstat(directory)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0
}

func NewBootHeartbeat(registry *Registry, directory string) (*BootHeartbeat, error) {
	if registry == nil || !canonicalID(registry.BootNonce()) || !bootDirectory(directory) {
		return nil, ErrDenied
	}
	return &BootHeartbeat{registry: registry, directory: directory}, nil
}

func (h *BootHeartbeat) Pulse(now time.Time) error {
	if h == nil {
		return ErrDenied
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return ErrDenied
	}
	v := h.registry.verifier
	raw, err := json.Marshal(bootRequest{Version: 1, Issuer: v.Issuer, Environment: v.Environment, NodeID: v.NodeID, Nonce: h.registry.BootNonce(), IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(bootHeartbeatTTL).UnixMilli()})
	if err != nil {
		return ErrDenied
	}
	file, err := os.CreateTemp(h.directory, ".boot-*.tmp")
	if err != nil {
		return ErrDenied
	}
	defer os.Remove(file.Name())
	_, err = file.Write(raw)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return ErrDenied
	}
	if os.Rename(file.Name(), filepath.Join(h.directory, h.registry.BootNonce()+".json")) != nil {
		return ErrDenied
	}
	// This ephemeral request is not restored authority. Losing it on a crash
	// merely stops fresh ACKs; the next live pulse replaces it atomically.
	return nil
}

func (h *BootHeartbeat) Close() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	err := os.Remove(filepath.Join(h.directory, h.registry.BootNonce()+".json"))
	if err != nil && !os.IsNotExist(err) {
		return ErrDenied
	}
	return nil
}

func ActiveBootNonces(directory, issuer, environment, node string, now time.Time) ([]string, error) {
	if !bootDirectory(directory) {
		return nil, ErrDenied
	}
	files, err := filepath.Glob(filepath.Join(directory, "*.json"))
	if err != nil || len(files) > maxBootRequestFiles {
		return nil, ErrDenied
	}
	nonces := []string{}
	for _, path := range files {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxBootRequestBytes {
			return nil, ErrDenied
		}
		raw, err := os.ReadFile(path)
		var request bootRequest
		if err != nil || !canonicalJSON(raw, &request) || request.Version != 1 || !canonicalID(request.Nonce) || filepath.Base(path) != request.Nonce+".json" || request.Issuer != issuer || request.Environment != environment || request.NodeID != node || request.IssuedAt <= 0 || request.ExpiresAt <= request.IssuedAt || request.ExpiresAt-request.IssuedAt > bootHeartbeatTTL.Milliseconds() {
			return nil, ErrDenied
		}
		if request.IssuedAt > now.UnixMilli() || request.ExpiresAt <= now.UnixMilli() {
			continue
		}
		nonces = append(nonces, request.Nonce)
		if len(nonces) > protocol.MaxReceiverBootNonces {
			return nil, ErrDenied
		}
	}
	slices.Sort(nonces)
	if !protocol.ValidReceiverBootNonces(nonces) {
		return nil, ErrDenied
	}
	return nonces, nil
}
