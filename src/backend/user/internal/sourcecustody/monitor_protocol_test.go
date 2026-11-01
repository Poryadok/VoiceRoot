package sourcecustody

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"math"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func monitorResponseFixture() MonitorResponse {
	var challenge, decision, nonce [32]byte
	for i := range challenge {
		challenge[i], decision[i], nonce[i] = 1, 2, 3
	}
	return MonitorResponse{
		Challenge: challenge, DecisionDigest: decision,
		Incarnation: uuid.MustParse("00112233-4455-6677-8899-aabbccddeeff"),
		Sequence:    7, Nonce: nonce, Kind: MonitorArmed,
	}
}

func TestMonitorResponseCanonicalSignatureAndParsing(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	response := monitorResponseFixture()
	payload, err := MonitorResponsePayload(response)
	require.NoError(t, err)
	expected := hex.EncodeToString([]byte("voice.user.source-custody.monitor.v1\x00")) + "0001" +
		strings.Repeat("01", 32) + strings.Repeat("02", 32) +
		"00112233445566778899aabbccddeeff" + "0000000000000007" + strings.Repeat("03", 32) + "01"
	require.Equal(t, expected, hex.EncodeToString(payload))
	signed, err := SignMonitorResponse(response, private)
	require.NoError(t, err)
	require.NoError(t, VerifyMonitorResponse(signed, public))
	parsed, err := ParseMonitorResponse(payload, signed.Signature, public)
	require.NoError(t, err)
	require.Equal(t, signed, parsed)

	changed := append([]byte(nil), payload...)
	changed[len(changed)-1] = byte(MonitorHeartbeat)
	_, err = ParseMonitorResponse(changed, signed.Signature, public)
	require.Error(t, err, "a different response kind must not reuse the signature")
	otherPublic, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_, err = ParseMonitorResponse(payload, signed.Signature, otherPublic)
	require.Error(t, err)
	_, err = ParseMonitorResponse(payload[:len(payload)-1], signed.Signature, public)
	require.Error(t, err)
	_, err = ParseMonitorResponse(append(payload, 0), signed.Signature, public)
	require.Error(t, err)
	_, err = ParseMonitorResponse(payload, signed.Signature[:63], public)
	require.Error(t, err)
}

func TestMonitorResponseRejectsInvalidSignedShapes(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	for name, mutate := range map[string]func(*MonitorResponse){
		"empty challenge":   func(v *MonitorResponse) { v.Challenge = [32]byte{} },
		"empty decision":    func(v *MonitorResponse) { v.DecisionDigest = [32]byte{} },
		"empty incarnation": func(v *MonitorResponse) { v.Incarnation = uuid.Nil },
		"zero sequence":     func(v *MonitorResponse) { v.Sequence = 0 },
		"sequence overflow": func(v *MonitorResponse) { v.Sequence = uint64(math.MaxInt64) + 1 },
		"empty nonce":       func(v *MonitorResponse) { v.Nonce = [32]byte{} },
		"unknown kind":      func(v *MonitorResponse) { v.Kind = MonitorResponseKind(4) },
	} {
		t.Run(name, func(t *testing.T) {
			response := monitorResponseFixture()
			mutate(&response)
			_, err := SignMonitorResponse(response, private)
			require.Error(t, err)
		})
	}
	valid := monitorResponseFixture()
	payload, err := MonitorResponsePayload(valid)
	require.NoError(t, err)
	versionOffset := len("voice.user.source-custody.monitor.v1\x00")
	payload[versionOffset+1] = 2
	signature := ed25519.Sign(private, payload)
	_, err = ParseMonitorResponse(payload, signature, public)
	require.Error(t, err, "even a signed unknown version must be rejected")
}

func TestMonitorResponseParserRejectsAuthenticMalformedWire(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	original, err := MonitorResponsePayload(monitorResponseFixture())
	require.NoError(t, err)
	base := len(monitorDomain) + 2
	zero := func(begin, count int) func([]byte) {
		return func(payload []byte) { clear(payload[begin : begin+count]) }
	}
	for name, mutate := range map[string]func([]byte){
		"domain":      func(payload []byte) { payload[0] ^= 1 },
		"challenge":   zero(base, 32),
		"decision":    zero(base+32, 32),
		"incarnation": zero(base+64, 16),
		"sequence":    zero(base+80, 8),
		"overflow":    func(payload []byte) { payload[base+80] = 0x80 },
		"nonce":       zero(base+88, 32),
		"kind zero":   func(payload []byte) { payload[base+120] = 0 },
		"kind four":   func(payload []byte) { payload[base+120] = 4 },
	} {
		t.Run(name, func(t *testing.T) {
			payload := append([]byte(nil), original...)
			mutate(payload)
			_, err := ParseMonitorResponse(payload, ed25519.Sign(private, payload), public)
			require.Error(t, err)
		})
	}
	signed, err := SignMonitorResponse(monitorResponseFixture(), private)
	require.NoError(t, err)
	_, err = SignMonitorResponse(signed, private)
	require.Error(t, err)
	_, err = ParseMonitorResponse(original, signed.Signature, public[:31])
	require.Error(t, err)
	_, err = ParseMonitorResponse(original, nil, public)
	require.Error(t, err)
	_, err = ParseMonitorResponse(original, append(append([]byte(nil), signed.Signature...), 0), public)
	require.Error(t, err)
	for _, kind := range []MonitorResponseKind{MonitorHeartbeat, MonitorStop} {
		response := monitorResponseFixture()
		response.Kind = kind
		response.Sequence = maxMonitorProbeSequence
		signed, err := SignMonitorResponse(response, private)
		require.NoError(t, err)
		payload, err := MonitorResponsePayload(signed)
		require.NoError(t, err)
		parsed, err := ParseMonitorResponse(payload, signed.Signature, public)
		require.NoError(t, err)
		require.Equal(t, signed, parsed)
		signed.Signature[0] ^= 1
		require.NoError(t, VerifyMonitorResponse(parsed, public), "parsed signature must not alias the input")
	}
}
