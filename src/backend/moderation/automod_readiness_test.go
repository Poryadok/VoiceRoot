package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"net/http/httptest"
	"testing"
	"voice/backend/moderation/internal/automod"
)

func TestAutoModReadiness_LivenessAndPrivateDiscoveryDoNotOpenPublicEffects(t *testing.T) {
	g := automod.NewBootGate()
	handler := healthHandlerWithGate(serviceName, g)
	get := func(path string) int {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		return response.Code
	}
	require.Equal(t, http.StatusOK, get("/health"))
	require.Equal(t, http.StatusServiceUnavailable, get("/ready"))
	called := false
	next := func(context.Context, any) (any, error) { called = true; return "response", nil }
	_, err := autoModPublicGate(g)(context.Background(), nil, &grpc.UnaryServerInfo{}, next)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.False(t, called)
	id, generation := g.Identity()
	for phase := automod.BootLocalIssuer; phase <= automod.BootEffectsReady; phase++ {
		require.NoError(t, g.Advance(automod.BootReceipt{BootID: id, Generation: generation, Phase: phase, ProofID: uuid.New()}))
	}
	require.Equal(t, http.StatusOK, get("/ready"))
	response, err := autoModPublicGate(g)(context.Background(), nil, &grpc.UnaryServerInfo{}, next)
	require.NoError(t, err)
	require.Equal(t, "response", response)
	_, err = autoModPublicGate(g)(context.Background(), nil, &grpc.UnaryServerInfo{}, func(context.Context, any) (any, error) { g.Close(); return "stale", nil })
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, http.StatusServiceUnavailable, get("/ready"))
	require.Equal(t, http.StatusOK, get("/health"))
}
