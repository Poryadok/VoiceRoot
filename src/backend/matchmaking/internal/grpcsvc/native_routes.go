package grpcsvc

type freshRoutePolicy uint8

const (
	preCutFreshRoutes freshRoutePolicy = iota
	nativeVoiceFreshRoutes
)

// NewNativeRouteHandler binds the closed native route policy. It constructs a
// handler only; installation admission and complete serving dependencies must
// be validated separately by the fixed native root before registration.
func NewNativeRouteHandler(service MatchmakingGRPC) *MatchmakingGRPC {
	service.freshRoutes = nativeVoiceFreshRoutes
	return &service
}
