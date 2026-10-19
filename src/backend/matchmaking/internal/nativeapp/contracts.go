package nativeapp

import (
	"crypto/sha256"
	"encoding/json"
	"slices"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	w "voice/backend/pkg/mmadmissionwire"
)

type routeContract struct {
	Method, Ordinary, Protected string
}

// nativeCodeContracts binds every method in the linked descriptor, including
// explicit refusals. A future method cannot silently inherit a legacy route.
// These code-owned policy identities describe this binary; they do not assert
// that any dependency or installation has become available.
func nativeCodeContracts() (factories, routes w.Hash) {
	var policy []routeContract
	service := matchmakingv1.File_voice_matchmaking_v1_matchmaking_proto.Services().ByName("MatchmakingService")
	for i := 0; i < service.Methods().Len(); i++ {
		method := service.Methods().Get(i)
		route := routeContract{Method: "/" + string(service.FullName()) + "/" + string(method.Name()), Ordinary: "DENY", Protected: "DENY"}
		switch string(method.Name()) {
		case "GetGame", "ListGames", "SearchGames":
			route.Ordinary = "PUBLIC_CATALOGUE"
		case "StartSearch":
			route.Protected = "GATEWAY_EXACT_SEARCH_OP_C16_ORIGIN_V1"
		case "RespondToMatch":
			route.Protected = "GATEWAY_EXACT_RESPONSE_OP_ATOMIC_RECEIPT_V1"
		case "GetMatch", "GetSearchStatus", "GetMatchHistory", "CancelSearch":
			route.Protected = "GATEWAY_EXACT_TERMINAL_V1"
		case "ApplySpaceLifecycleFence", "PurgeSpace":
			route.Protected = "SPACE_EXACT_LIFECYCLE_V1"
		case "CompleteMatch":
			route.Protected = "GATEWAY_EXACT_COMPLETION_V1"
		}
		policy = append(policy, route)
	}
	slices.SortFunc(policy, func(a, b routeContract) int {
		if a.Method < b.Method {
			return -1
		}
		if a.Method > b.Method {
			return 1
		}
		return 0
	})
	descriptor, _ := proto.MarshalOptions{Deterministic: true}.Marshal(protodesc.ToFileDescriptorProto(matchmakingv1.File_voice_matchmaking_v1_matchmaking_proto))
	routes = codeContractHash("voice.mm.native.route-catalogue.v1", struct {
		DescriptorSHA256 [32]byte
		Routes           []routeContract
	}{sha256.Sum256(descriptor), policy})
	factories = codeContractHash("voice.mm.native.factory-catalogue.v1", []string{
		"binding:private-FD3-once-root-driver-noexec-v1",
		"database:physical-identity-MM024-MM025-MM026-MM027-privileges-v1",
		"gateway:exact-search-response-terminal-guest-regular-live-epoch-v1",
		"source:protected-voice-C16-v1-origin-v1-candidate-v2",
		"provider:protected-chat-voice-private-cut-FIRST-explicit-lifetime-v1",
		"queue:wholeparty-projector-private-cut-FIRST-witness-v2",
		"matcher:wholeparty-validated-proposal-v1",
		"timeout:native-wholeparty-expiry-MM027-frozen-facts-recovery-v1",
		"roster:prepared-durable-inbox-captured-cancellation-v1",
		"events:required-JetStream-JWT-mTLS-TLS13-v1",
		"jwks:public-key-only-bootstrap-v1",
		"control:private-authenticated-workload-close-drain-v1",
	})
	return factories, routes
}

func codeContractHash(domain string, value any) w.Hash {
	raw, err := json.Marshal(value)
	if err != nil {
		return w.Hash{}
	}
	h := sha256.New()
	_, _ = h.Write([]byte(domain + "\x00"))
	_, _ = h.Write(raw)
	var result w.Hash
	copy(result[:], h.Sum(nil))
	return result
}

// checkCodeContracts is deliberately a rejection check, not preparation or an
// admission issuer. The actual factories still have to construct and qualify.
func checkCodeContracts(cfg nativeConfig) error {
	factories, routes := nativeCodeContracts()
	if cfg.descriptor.Validate() != nil || factories == (w.Hash{}) || routes == (w.Hash{}) || cfg.descriptor.Binding.SourceFactoryManifestHash != factories || cfg.descriptor.Binding.FreshRouteManifestHash != routes {
		return errUnavailable
	}
	return nil
}
