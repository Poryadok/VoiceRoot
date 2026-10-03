// Voice-owned adapter for the pinned LiveKit server. This file is copied into
// upstream pkg/service by prepare.py; authority logic comes from Federation.
package service

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/livekit/livekit-server/pkg/rtc"
	"github.com/livekit/livekit-server/pkg/rtc/types"
	"github.com/livekit/livekit-server/pkg/voiceauthority/mediaauthority"
	"github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/logger"
)

type voiceAuthorityTrust struct {
	Issuer      string            `json:"issuer"`
	Environment string            `json:"environment"`
	NodeID      string            `json:"node_id"`
	Keys        map[string]string `json:"keys"`
}

type voiceAdmissionRecord struct {
	admission mediaauthority.Admission
	cleanupAt time.Time
}

type voiceAuthorityRuntime struct {
	manager    *RoomManager
	registry   *mediaauthority.Registry
	heartbeat  *mediaauthority.BootHeartbeat
	directory  string
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex
	admissions map[livekit.ParticipantID]voiceAdmissionRecord
}

func newVoiceAuthority(manager *RoomManager) (*voiceAuthorityRuntime, error) {
	directory := os.Getenv("VOICE_SFU_AUTHORITY_DIR")
	if directory == "" {
		return nil, fmt.Errorf("Voice SFU requires its authority directory")
	}
	var trust voiceAuthorityTrust
	if readVoiceAuthorityJSON(os.Getenv("VOICE_SFU_TRUST_FILE"), &trust, 65536) != nil || trust.NodeID != string(manager.currentNode.NodeID()) {
		return nil, fmt.Errorf("invalid Voice SFU trust configuration")
	}
	keys := make(map[string]ed25519.PublicKey, len(trust.Keys))
	for id, encoded := range trust.Keys {
		key, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil || len(key) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid Voice SFU authority public key")
		}
		keys[id] = key
	}
	registry, err := mediaauthority.NewBootRegistry(mediaauthority.Verifier{Issuer: trust.Issuer, Environment: trust.Environment, NodeID: trust.NodeID, Keys: keys}, 250*time.Millisecond)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("Voice SFU authority directory unavailable")
	}
	heartbeat, err := mediaauthority.NewBootHeartbeat(registry, os.Getenv("VOICE_SFU_BOOT_REQUEST_DIR"))
	if err != nil || heartbeat.Pulse(time.Now()) != nil {
		return nil, fmt.Errorf("Voice SFU boot request directory unavailable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	runtime := &voiceAuthorityRuntime{manager: manager, registry: registry, heartbeat: heartbeat, directory: directory, cancel: cancel, admissions: make(map[livekit.ParticipantID]voiceAdmissionRecord)}
	runtime.load()
	runtime.wg.Add(2)
	go runtime.loadLoop(ctx)
	go runtime.watchLoop(ctx)
	return runtime, nil
}

func (v *voiceAuthorityRuntime) stop() {
	v.cancel()
	v.wg.Wait()
	_ = v.heartbeat.Close()
}

func (v *voiceAuthorityRuntime) remember(id livekit.ParticipantID, admission mediaauthority.Admission) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.admissions[id] = voiceAdmissionRecord{admission: admission, cleanupAt: time.Now().Add(5 * time.Second)}
}

func (v *voiceAuthorityRuntime) loadLoop(ctx context.Context) {
	defer v.wg.Done()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if v.heartbeat.Pulse(time.Now()) == nil {
				v.load()
			}
		}
	}
}

func (v *voiceAuthorityRuntime) load() {
	files, err := filepath.Glob(filepath.Join(v.directory, "*.json"))
	if err != nil {
		return
	}
	for _, name := range files {
		var bundle mediaauthority.Bundle
		if readVoiceAuthorityJSON(name, &bundle, 16<<20) == nil {
			// Invalid/stale/partial input never replaces a complete valid Space.
			// An absent or expired refresh naturally closes at the local deadline.
			_ = v.registry.Apply(bundle, time.Now())
		}
	}
}

func (v *voiceAuthorityRuntime) watchLoop(ctx context.Context) {
	defer v.wg.Done()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			v.sweep()
		}
	}
}

func (v *voiceAuthorityRuntime) sweep() {
	v.manager.lock.RLock()
	rooms := make([]*rtc.Room, 0, len(v.manager.rooms))
	for _, room := range v.manager.rooms {
		rooms = append(rooms, room)
	}
	v.manager.lock.RUnlock()
	seen := make(map[livekit.ParticipantID]bool)
	for _, room := range rooms {
		for _, participant := range room.GetParticipants() {
			id := participant.ID()
			seen[id] = true
			v.mu.RLock()
			record, ok := v.admissions[id]
			v.mu.RUnlock()
			if ok && v.registry.Check(record.admission, time.Now()) == nil {
				continue
			}
			// Exact SID protects a replacement session of the same identity.
			// Removal closes tracks and transports inside this SFU process.
			room.RemoveParticipant(participant.Identity(), id, types.ParticipantCloseReasonVerifyFailed)
			logger.Infow("Voice authority closed media session", "participant_id", id)
		}
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	for id, record := range v.admissions {
		if !seen[id] && !time.Now().Before(record.cleanupAt) {
			delete(v.admissions, id)
		}
	}
}

func readVoiceAuthorityJSON(name string, value any, limit int64) error {
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return mediaauthority.ErrDenied
	}
	file, err := os.Open(name)
	if err != nil {
		return mediaauthority.ErrDenied
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, limit+1))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil || decoder.Decode(new(any)) != io.EOF {
		return mediaauthority.ErrDenied
	}
	return nil
}
