package main

import (
	"context"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	spacev1 "voice.app/voice/space/v1"
)

type recordingSpacesCreate struct {
	spacev1.UnimplementedSpaceServiceServer
	last *spacev1.CreateSpaceRequest
}

func (s *recordingSpacesCreate) CreateSpace(_ context.Context, req *spacev1.CreateSpaceRequest) (*spacev1.CreateSpaceResponse, error) {
	s.last = req
	now := timestamppb.Now()
	return &spacev1.CreateSpaceResponse{
		Space: &spacev1.Space{
			Id:               "space-1",
			Name:             req.GetName(),
			Description:      req.GetDescription(),
			Visibility:       "private",
			OwnerProfileId:   "profile-1",
			MemberCount:      1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}, nil
}

type recordingSpacesUpdate struct {
	spacev1.UnimplementedSpaceServiceServer
	last *spacev1.UpdateSpaceRequest
}

func (s *recordingSpacesUpdate) UpdateSpace(_ context.Context, req *spacev1.UpdateSpaceRequest) (*spacev1.UpdateSpaceResponse, error) {
	s.last = req
	now := timestamppb.Now()
	desc := req.GetDescription()
	return &spacev1.UpdateSpaceResponse{
		Space: &spacev1.Space{
			Id:             req.GetSpaceId(),
			Name:           "space-1",
			IconUrl:        req.IconUrl,
			Description:    desc,
			OwnerProfileId: "profile-1",
			MemberCount:    1,
			Visibility:     "private",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}, nil
}

func startBufconnSpaceConn(t *testing.T, impl spacev1.SpaceServiceServer) (grpc.ClientConnInterface, func()) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	spacev1.RegisterSpaceServiceServer(srv, impl)
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, func() {
		_ = conn.Close()
		srv.Stop()
		_ = lis.Close()
	}
}

// TestTranscodeSpacesCreate documents spaces.md: POST /api/v1/spaces with name + description.
func TestTranscodeSpacesCreate(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingSpacesCreate{}
	conn, cleanup := startBufconnSpaceConn(t, grpcRec)
	t.Cleanup(cleanup)

	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{space: spacev1.NewSpaceServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"spaces": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalled = true
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})

	body := `{"name":"Friday squad","description":"We raid on Fridays"}`
	resp := performRequest(h, http.MethodPost, "/api/v1/spaces", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if proxyCalled {
		t.Fatal("REST proxy must not run when gRPC transcoder handles POST /api/v1/spaces")
	}
	if grpcRec.last == nil {
		t.Fatal("CreateSpace request was not forwarded to Space Service")
	}
	if grpcRec.last.GetName() != "Friday squad" {
		t.Fatalf("CreateSpace name = %q", grpcRec.last.GetName())
	}
	if grpcRec.last.GetDescription() != "We raid on Fridays" {
		t.Fatalf("CreateSpace description = %q", grpcRec.last.GetDescription())
	}
}

// TestTranscodeSpacesUpdateIcon documents PATCH /api/v1/spaces/{spaceId} with icon_url.
func TestTranscodeSpacesUpdateIcon(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingSpacesUpdate{}
	conn, cleanup := startBufconnSpaceConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{space: spacev1.NewSpaceServiceClient(conn)}},
	})

	body := `{"icon_url":"https://cdn.voice.gg/spaces/party.webp"}`
	resp := performRequest(h, http.MethodPatch, "/api/v1/spaces/space-1", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if grpcRec.last == nil || grpcRec.last.GetSpaceId() != "space-1" {
		t.Fatalf("UpdateSpace space_id = %+v", grpcRec.last)
	}
	if grpcRec.last.GetIconUrl() != "https://cdn.voice.gg/spaces/party.webp" {
		t.Fatalf("UpdateSpace icon_url = %q", grpcRec.last.GetIconUrl())
	}
}

type recordingSpacesList struct {
	spacev1.UnimplementedSpaceServiceServer
	last *spacev1.ListMySpacesRequest
}

func (s *recordingSpacesList) ListMySpaces(_ context.Context, req *spacev1.ListMySpacesRequest) (*spacev1.ListMySpacesResponse, error) {
	s.last = req
	now := timestamppb.Now()
	return &spacev1.ListMySpacesResponse{
		SpaceList: &spacev1.SpaceList{
			Spaces: []*spacev1.Space{
				{
					Id:             "space-1",
					Name:           "Listed",
					Description:    "Shows up",
					Visibility:     "private",
					OwnerProfileId: "profile-1",
					MemberCount:    1,
					CreatedAt:      now,
					UpdatedAt:      now,
				},
			},
		},
	}, nil
}

type recordingSpacesGet struct {
	spacev1.UnimplementedSpaceServiceServer
	lastSpaceID string
}

func (s *recordingSpacesGet) GetSpace(_ context.Context, req *spacev1.GetSpaceRequest) (*spacev1.GetSpaceResponse, error) {
	s.lastSpaceID = req.GetSpaceId()
	now := timestamppb.Now()
	return &spacev1.GetSpaceResponse{
		Space: &spacev1.Space{
			Id:             req.GetSpaceId(),
			Name:           "Readable",
			Description:    "About us",
			IconUrl:        strPtr("https://cdn.voice.gg/spaces/readable.webp"),
			Visibility:     "private",
			OwnerProfileId: "profile-1",
			MemberCount:    1,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}, nil
}

type denySpacesUpdateGRPC struct {
	spacev1.UnimplementedSpaceServiceServer
}

func (denySpacesUpdateGRPC) UpdateSpace(context.Context, *spacev1.UpdateSpaceRequest) (*spacev1.UpdateSpaceResponse, error) {
	return nil, status.Error(codes.PermissionDenied, "only the space owner can update the space")
}

func strPtr(s string) *string { return &s }

// TestTranscodeSpacesListMySpaces documents GET /api/v1/spaces lists caller spaces.
func TestTranscodeSpacesListMySpaces(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingSpacesList{}
	conn, cleanup := startBufconnSpaceConn(t, grpcRec)
	t.Cleanup(cleanup)

	proxyCalled := false
	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{space: spacev1.NewSpaceServiceClient(conn)}},
		restUpstreams: map[string]http.Handler{
			"spaces": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalled = true
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/spaces?page_size=10", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if proxyCalled {
		t.Fatal("REST proxy must not run when gRPC transcoder handles GET /api/v1/spaces")
	}
	if grpcRec.last == nil || grpcRec.last.GetPage().GetPageSize() != 10 {
		t.Fatalf("ListMySpaces page = %+v", grpcRec.last)
	}

	var body struct {
		SpaceList struct {
			Spaces []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"spaces"`
		} `json:"space_list"`
	}
	decodeJSON(t, resp.Body, &body)
	if len(body.SpaceList.Spaces) != 1 || body.SpaceList.Spaces[0].ID != "space-1" {
		t.Fatalf("response body = %+v", body)
	}
}

// TestTranscodeSpacesGetByID documents GET /api/v1/spaces/{spaceId}.
func TestTranscodeSpacesGetByID(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingSpacesGet{}
	conn, cleanup := startBufconnSpaceConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{space: spacev1.NewSpaceServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/spaces/space-42", "", map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if grpcRec.lastSpaceID != "space-42" {
		t.Fatalf("GetSpace space_id = %q", grpcRec.lastSpaceID)
	}

	var body struct {
		Space struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			IconURL     string `json:"icon_url"`
		} `json:"space"`
	}
	decodeJSON(t, resp.Body, &body)
	if body.Space.ID != "space-42" || body.Space.Description != "About us" {
		t.Fatalf("response body = %+v", body)
	}
}

// TestTranscodeSpacesCreateUnauthorized documents JWT required on POST /api/v1/spaces.
func TestTranscodeSpacesCreateUnauthorized(t *testing.T) {
	t.Parallel()

	conn, cleanup := startBufconnSpaceConn(t, &recordingSpacesCreate{})
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		transcoder: &transcoder{clients: grpcClients{space: spacev1.NewSpaceServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodPost, "/api/v1/spaces", `{"name":"No auth"}`, nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusUnauthorized, resp.Body.String())
	}
}

// TestTranscodeSpacesUpdatePermissionDenied documents gRPC PermissionDenied → HTTP 403.
func TestTranscodeSpacesUpdatePermissionDenied(t *testing.T) {
	t.Parallel()

	conn, cleanup := startBufconnSpaceConn(t, denySpacesUpdateGRPC{})
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{space: spacev1.NewSpaceServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodPatch, "/api/v1/spaces/space-1", `{"description":"nope"}`, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusForbidden, resp.Body.String())
	}
	var got struct {
		ErrorCode string `json:"error_code"`
	}
	decodeJSON(t, resp.Body, &got)
	if got.ErrorCode != "permission_denied" {
		t.Fatalf("error_code = %q", got.ErrorCode)
	}
}

// TestTranscodeSpacesListUnauthorized documents JWT required on GET /api/v1/spaces.
func TestTranscodeSpacesListUnauthorized(t *testing.T) {
	t.Parallel()

	conn, cleanup := startBufconnSpaceConn(t, &recordingSpacesList{})
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		transcoder: &transcoder{clients: grpcClients{space: spacev1.NewSpaceServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/spaces", "", nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusUnauthorized, resp.Body.String())
	}
}

// TestTranscodeSpacesGetUnauthorized documents JWT required on GET /api/v1/spaces/{spaceId}.
func TestTranscodeSpacesGetUnauthorized(t *testing.T) {
	t.Parallel()

	conn, cleanup := startBufconnSpaceConn(t, &recordingSpacesGet{})
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		transcoder: &transcoder{clients: grpcClients{space: spacev1.NewSpaceServiceClient(conn)}},
	})

	resp := performRequest(h, http.MethodGet, "/api/v1/spaces/space-1", "", nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusUnauthorized, resp.Body.String())
	}
}

// TestTranscodeSpacesUpdateDescription documents PATCH description-only body.
func TestTranscodeSpacesUpdateDescription(t *testing.T) {
	t.Parallel()

	grpcRec := &recordingSpacesUpdate{}
	conn, cleanup := startBufconnSpaceConn(t, grpcRec)
	t.Cleanup(cleanup)

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"valid-user-token": {UserID: "account-1", ProfileID: "profile-1"},
		},
		transcoder: &transcoder{clients: grpcClients{space: spacev1.NewSpaceServiceClient(conn)}},
	})

	body := `{"description":"Updated about"}`
	resp := performRequest(h, http.MethodPatch, "/api/v1/spaces/space-1", body, map[string]string{
		"Authorization": "Bearer valid-user-token",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", resp.Code, http.StatusOK, resp.Body.String())
	}
	if grpcRec.last == nil || grpcRec.last.GetDescription() != "Updated about" {
		t.Fatalf("UpdateSpace description = %+v", grpcRec.last)
	}
}

