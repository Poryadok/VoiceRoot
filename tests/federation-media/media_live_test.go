package federationmedia

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
	"voice/backend/federation/mediaauthority"
	"voice/backend/federation/protocol"
)

type fixtureParameters struct {
	NodeID     string `json:"node_id"`
	APIKey     string `json:"api_key"`
	APISecret  string `json:"api_secret"`
	PrivateKey string `json:"private_key"`
}

type projectionPublisher struct {
	mu        sync.Mutex
	private   ed25519.PrivateKey
	directory string
	grants    [2][]mediaauthority.Grant
	allow     [2]bool
	revision  [2]int64
	sink      func(string, mediaauthority.Bundle) error
}

func (p *projectionPublisher) publish() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for space := range p.grants {
		now := time.Now()
		grant := p.grants[space][0]
		permissions := []protocol.Permission{}
		if p.allow[space] {
			for _, actor := range p.grants[space] {
				permissions = append(permissions, protocol.Permission{AccountID: actor.AccountID, ProfileID: actor.ProfileID, ResourceID: actor.ResourceID, SessionEpoch: actor.SessionEpoch, Actions: []string{"media"}})
			}
		}
		p.revision[space]++
		policy := protocol.Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: p.revision[space], ValidUntil: now.Add(1500 * time.Millisecond).UnixMilli(), Permissions: permissions}
		manifest, err := protocol.SignSnapshotManifest(p.private, "fixture-1", grant.Scope(), protocol.ManifestFor(policy), now)
		if err != nil {
			return err
		}
		pages, err := protocol.SignSnapshotPages(p.private, "fixture-1", grant.Scope(), policy, now)
		if err != nil {
			return err
		}
		lease, err := protocol.SignEnvelope(p.private, "fixture-1", protocol.Claims{Version: 1, Kind: "lease", Issuer: grant.Issuer, Audience: "voice-node", Environment: grant.Environment,
			NodeID: grant.NodeID, SpaceID: grant.SpaceID, Generation: grant.Generation, Epoch: grant.AuthorityEpoch, Revision: policy.Revision, IssuedAt: now.UnixMilli(), ExpiresAt: policy.ValidUntil, Hash: protocol.SnapshotDigest(policy)})
		if err != nil {
			return err
		}
		bundle := mediaauthority.Bundle{Scope: grant.Scope(), Manifest: manifest, Pages: pages, Lease: lease}
		if p.sink != nil {
			if err := p.sink(grant.SpaceID, bundle); err != nil {
				return err
			}
			continue
		}
		wire, err := json.Marshal(bundle)
		if err != nil {
			return err
		}
		name := filepath.Join(p.directory, grant.SpaceID+".json")
		if os.WriteFile(name+".new", wire, 0644) != nil || os.Rename(name+".new", name) != nil {
			return fmt.Errorf("projection write failed")
		}
	}
	return nil
}

type mediaPeer struct {
	room         *lksdk.Room
	packets      atomic.Int64
	disconnected chan time.Time
}

func connectPeer(ctx context.Context, url, token string) (*mediaPeer, error) {
	peer := &mediaPeer{disconnected: make(chan time.Time, 1)}
	callback := &lksdk.RoomCallback{
		OnDisconnected: func() {
			select {
			case peer.disconnected <- time.Now():
			default:
			}
		},
		ParticipantCallback: lksdk.ParticipantCallback{OnTrackSubscribed: func(track *webrtc.TrackRemote, _ *lksdk.RemoteTrackPublication, _ *lksdk.RemoteParticipant) {
			go func() {
				for {
					packet, _, err := track.ReadRTP()
					if err != nil {
						return
					}
					if len(packet.Payload) != 0 {
						peer.packets.Add(1)
					}
				}
			}()
		}},
	}
	room, err := lksdk.ConnectToRoomWithToken(url, token, callback, lksdk.WithAutoSubscribe(true))
	if err != nil {
		return nil, fmt.Errorf("media admission failed")
	}
	peer.room = room
	track, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, "authority-audio", "authority-stream")
	if err != nil {
		room.Disconnect()
		return nil, err
	}
	if _, err = room.LocalParticipant.PublishTrack(track, &lksdk.TrackPublicationOptions{Name: "authority-audio", Source: livekit.TrackSource_MICROPHONE}); err != nil {
		room.Disconnect()
		return nil, fmt.Errorf("media publication failed")
	}
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		var sequence uint16
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sequence++
				_ = track.WriteRTP(&rtp.Packet{Header: rtp.Header{Version: 2, SequenceNumber: sequence, Timestamp: uint32(sequence) * 960}, Payload: []byte{0xf8, 0xff, 0xfe}})
			}
		}
	}()
	return peer, nil
}

func fixtureJWT(t *testing.T, parameters fixtureParameters, private ed25519.PrivateKey, grant mediaauthority.Grant, validity time.Duration) string {
	t.Helper()
	now := time.Now()
	grant.IssuedAt, grant.ExpiresAt = now.UnixMilli(), now.Add(validity).UnixMilli()
	credential, err := mediaauthority.Sign(private, "fixture-1", grant, now)
	require.NoError(t, err)
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{"iss": parameters.APIKey, "sub": grant.ProfileID, "iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(time.Hour).Unix(),
		"video": map[string]any{"roomJoin": true, "room": grant.RoomName}, mediaauthority.GrantClaim: credential})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, []byte(parameters.APISecret))
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// This controlled signer fixture verifies the SFU mechanism on real RTP. It is
// deliberately separate from full Federation/master/Gateway acceptance.
func TestSFUEnforcesSignedAuthorityForRealMedia_live(t *testing.T) {
	runMediaAuthorityFixture(t, false)
}

func TestSFUEnforcesControllerProcessDeathForRealMedia_live(t *testing.T) {
	runMediaAuthorityFixture(t, true)
}

func runMediaAuthorityFixture(t *testing.T, controllerProcess bool) {
	directory := os.Getenv("VOICE_SFU_FIXTURE_DIR")
	url := os.Getenv("VOICE_SFU_URL")
	if directory == "" || url == "" {
		t.Skip("owned SFU fixture required")
	}
	var parameters fixtureParameters
	raw, err := os.ReadFile(filepath.Join(directory, "parameters.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &parameters))
	private, err := base64.RawURLEncoding.DecodeString(parameters.PrivateKey)
	require.NoError(t, err)
	publisher := &projectionPublisher{private: private, directory: filepath.Join(directory, "authority"), allow: [2]bool{true, true}}
	for space := range publisher.grants {
		spaceID, resourceID, room := uuid.NewString(), uuid.NewString(), "authority-"+uuid.NewString()
		for range 2 {
			publisher.grants[space] = append(publisher.grants[space], mediaauthority.Grant{Version: 1, Issuer: "fixture-master", Audience: "voice-node-media", Environment: "sandbox", NodeID: parameters.NodeID,
				SpaceID: spaceID, Generation: 1, AuthorityEpoch: 1, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), ResourceID: resourceID, SessionEpoch: 1, RoomName: room})
		}
	}
	var stopController func() time.Time
	if controllerProcess {
		stopController = startControllerProcess(t, directory, parameters, publisher)
	} else {
		require.NoError(t, publisher.publish())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	refreshCtx, stopRefresh := context.WithCancel(ctx)
	defer stopRefresh()
	refreshDone := make(chan struct{})
	refreshErrors := make(chan error, 1)
	go func() {
		defer close(refreshDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-refreshCtx.Done():
				return
			case <-ticker.C:
				if publisher.publish() != nil {
					refreshErrors <- fmt.Errorf("projection refresh failed")
					return
				}
			}
		}
	}()
	time.Sleep(300 * time.Millisecond)
	var peers [2][]*mediaPeer
	var tokens [2][]string
	for space := range publisher.grants {
		for _, grant := range publisher.grants[space] {
			validity := 30 * time.Second
			if space == 1 {
				validity = 3 * time.Second
			}
			token := fixtureJWT(t, parameters, private, grant, validity)
			peer, err := connectPeer(ctx, url, token)
			require.True(t, err == nil, "real media admission must succeed")
			defer peer.room.Disconnect()
			peers[space] = append(peers[space], peer)
			tokens[space] = append(tokens[space], token)
		}
	}
	for _, group := range peers {
		for _, peer := range group {
			require.Eventually(t, func() bool { return peer.packets.Load() >= 10 }, 15*time.Second, 20*time.Millisecond, "bidirectional nonempty RTP required")
		}
	}
	for _, group := range peers {
		for _, peer := range group {
			for _, remote := range peer.room.GetRemoteParticipants() {
				require.Empty(t, remote.Metadata())
				require.Empty(t, remote.Attributes(), "authority credential must never be exposed to room participants")
			}
		}
	}
	// Active media remains authorized by fresh policy after the short admission
	// credential expires; its bearer still cannot open a new session afterward.
	time.Sleep(4 * time.Second)
	before := peers[1][0].packets.Load()
	require.Eventually(t, func() bool { return peers[1][0].packets.Load() > before+10 }, 2*time.Second, 20*time.Millisecond)
	publisher.mu.Lock()
	publisher.allow[0] = false
	publisher.mu.Unlock()
	changed := time.Now()
	require.NoError(t, publisher.publish())
	for _, peer := range peers[0] {
		select {
		case closed := <-peer.disconnected:
			t.Logf("revoked Space media eject_ms=%d", closed.Sub(changed).Milliseconds())
			require.Less(t, closed.Sub(changed), 5*time.Second)
		case <-time.After(5 * time.Second):
			t.Fatal("revoked media remained connected")
		}
	}
	_, err = connectPeer(ctx, url, tokens[0][0])
	require.True(t, err != nil, "unexpired stale JWT/grant cannot reconnect after revocation")
	before = peers[1][0].packets.Load()
	require.Eventually(t, func() bool { return peers[1][0].packets.Load() > before+10 }, 2*time.Second, 20*time.Millisecond, "other Space must continue after revocation")
	freshBearer := fixtureJWT(t, parameters, private, publisher.grants[1][0], 30*time.Second)
	partitioned := time.Now()
	var signerRevision [2]int64
	if controllerProcess {
		partitioned = stopController()
		publisher.mu.Lock()
		signerRevision = publisher.revision
		publisher.mu.Unlock()
		// The signed projection source stays live. Only the independent
		// production controller process has died; the SFU must expire itself.
	} else {
		stopRefresh()
		<-refreshDone
	}
	for _, peer := range peers[1] {
		select {
		case closed := <-peer.disconnected:
			t.Logf("authority unavailable controller_process=%t media eject_ms=%d", controllerProcess, closed.Sub(partitioned).Milliseconds())
			require.Less(t, closed.Sub(partitioned), 5*time.Second)
		case <-time.After(5 * time.Second):
			t.Fatal("media survived authority lease expiry")
		}
	}
	_, err = connectPeer(ctx, url, freshBearer)
	require.True(t, err != nil, "fresh unexpired bearer cannot reconnect while authority is expired")
	if controllerProcess {
		publisher.mu.Lock()
		current := publisher.revision
		publisher.mu.Unlock()
		for space := range current {
			require.Greater(t, current[space], signerRevision[space], "signed authority source must continue after controller death")
		}
	}
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(strings.Replace(url, "ws://", "http://", 1))
	require.True(t, err == nil, "SFU must remain reachable after expired-authority ejection")
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	select {
	case <-refreshErrors:
		t.Fatal("controlled projection publisher failed")
	default:
	}
	t.Log("two Spaces exchanged bidirectional RTP; stale media admissions denied; SFU independently enforced expired authority")
}
