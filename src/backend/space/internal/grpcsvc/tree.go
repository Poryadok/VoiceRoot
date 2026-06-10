package grpcsvc

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
)

func (s *SpaceGRPC) requireSpaceMember(ctx context.Context, spaceID uuid.UUID) error {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing profile")
	}
	member, err := s.Store.IsSpaceMember(ctx, spaceID, caller)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if !member {
		return status.Error(codes.PermissionDenied, "not a space member")
	}
	return nil
}

func (s *SpaceGRPC) requireSpaceOwner(ctx context.Context, spaceID uuid.UUID) error {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing profile")
	}
	row, err := s.Store.GetSpace(ctx, spaceID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return status.Error(codes.NotFound, "space not found")
	}
	if row.OwnerProfileID != caller {
		return status.Error(codes.PermissionDenied, "only the space owner can modify the tree")
	}
	return nil
}

func (s *SpaceGRPC) ListSpaceTree(ctx context.Context, req *spacev1.ListSpaceTreeRequest) (*spacev1.ListSpaceTreeResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpaceMember(ctx, spaceID); err != nil {
		return nil, err
	}
	data, err := s.Store.ListSpaceTree(ctx, spaceID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return spaceTreeDataToProto(data), nil
}

func (s *SpaceGRPC) CreateCategory(ctx context.Context, req *spacev1.CreateCategoryRequest) (*spacev1.CreateCategoryResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpaceOwner(ctx, spaceID); err != nil {
		return nil, err
	}
	row, err := s.Store.CreateCategory(ctx, spaceID, req.GetName(), req.GetSortOrder())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &spacev1.CreateCategoryResponse{Category: categoryRowToProto(row)}, nil
}

func (s *SpaceGRPC) UpdateCategory(ctx context.Context, req *spacev1.UpdateCategoryRequest) (*spacev1.UpdateCategoryResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	categoryID, err := parseUUIDField("category_id", req.GetCategoryId())
	if err != nil {
		return nil, err
	}
	spaceID, err := s.storeCategorySpaceID(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	if err := s.requireSpaceOwner(ctx, spaceID); err != nil {
		return nil, err
	}
	row, err := s.Store.UpdateCategory(ctx, categoryID, req.Name, req.SortOrder)
	if errors.Is(err, store.ErrCategoryNotFound) {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &spacev1.UpdateCategoryResponse{Category: categoryRowToProto(row)}, nil
}

func (s *SpaceGRPC) DeleteCategory(ctx context.Context, req *spacev1.DeleteCategoryRequest) (*spacev1.DeleteCategoryResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	categoryID, err := parseUUIDField("category_id", req.GetCategoryId())
	if err != nil {
		return nil, err
	}
	// Need space_id for auth - fetch categories is inefficient; use a helper
	spaceID, err := s.storeCategorySpaceID(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	if err := s.requireSpaceOwner(ctx, spaceID); err != nil {
		return nil, err
	}
	if err := s.Store.DeleteCategory(ctx, categoryID); err != nil {
		if errors.Is(err, store.ErrCategoryNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &spacev1.DeleteCategoryResponse{}, nil
}

func (s *SpaceGRPC) storeCategorySpaceID(ctx context.Context, categoryID uuid.UUID) (uuid.UUID, error) {
	var spaceID uuid.UUID
	err := s.Store.Pool.QueryRow(ctx, `SELECT space_id FROM categories WHERE id = $1`, categoryID).Scan(&spaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, status.Error(codes.NotFound, "category not found")
	}
	if err != nil {
		return uuid.Nil, status.Error(codes.Internal, err.Error())
	}
	return spaceID, nil
}

func (s *SpaceGRPC) CreateVoiceRoom(ctx context.Context, req *spacev1.CreateVoiceRoomRequest) (*spacev1.CreateVoiceRoomResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpaceOwner(ctx, spaceID); err != nil {
		return nil, err
	}
	room, node, err := s.Store.CreateVoiceRoom(ctx, spaceID, req.GetName(), nil)
	if errors.Is(err, store.ErrTreeNodeLimit) {
		return nil, status.Error(codes.ResourceExhausted, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	s.publishTreeUpserted(ctx, spaceID, node)
	if s.SpaceEvents != nil {
		_ = s.SpaceEvents.PublishVoiceRoomCreated(ctx, spaceID.String(), room.ID.String())
	}
	return &spacev1.CreateVoiceRoomResponse{VoiceRoom: voiceRoomRowToProto(room)}, nil
}

func (s *SpaceGRPC) UpdateVoiceRoom(ctx context.Context, req *spacev1.UpdateVoiceRoomRequest) (*spacev1.UpdateVoiceRoomResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	voiceRoomID, err := parseUUIDField("voice_room_id", req.GetVoiceRoomId())
	if err != nil {
		return nil, err
	}
	var spaceID uuid.UUID
	err = s.Store.Pool.QueryRow(ctx, `SELECT space_id FROM voice_rooms WHERE id = $1`, voiceRoomID).Scan(&spaceID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "voice room not found")
	}
	if err := s.requireSpaceOwner(ctx, spaceID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.GetName())
	if req.Name == nil || name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	room, err := s.Store.UpdateVoiceRoom(ctx, voiceRoomID, name)
	if errors.Is(err, store.ErrVoiceRoomNotFound) {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &spacev1.UpdateVoiceRoomResponse{VoiceRoom: voiceRoomRowToProto(room)}, nil
}

func (s *SpaceGRPC) DeleteVoiceRoom(ctx context.Context, req *spacev1.DeleteVoiceRoomRequest) (*spacev1.DeleteVoiceRoomResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	voiceRoomID, err := parseUUIDField("voice_room_id", req.GetVoiceRoomId())
	if err != nil {
		return nil, err
	}
	var spaceID uuid.UUID
	var nodeID sql.NullString
	err = s.Store.Pool.QueryRow(ctx, `
SELECT vr.space_id, n.id::text FROM voice_rooms vr
LEFT JOIN space_tree_nodes n ON n.voice_room_id = vr.id
WHERE vr.id = $1
`, voiceRoomID).Scan(&spaceID, &nodeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "voice room not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := s.requireSpaceOwner(ctx, spaceID); err != nil {
		return nil, err
	}
	if err := s.Store.DeleteVoiceRoom(ctx, voiceRoomID); err != nil {
		if errors.Is(err, store.ErrVoiceRoomNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if nodeID.Valid {
		if nid, parseErr := uuid.Parse(nodeID.String); parseErr == nil {
			s.publishTreeRemoved(ctx, spaceID, nid)
		}
	}
	if s.SpaceEvents != nil {
		_ = s.SpaceEvents.PublishVoiceRoomDeleted(ctx, spaceID.String(), voiceRoomID.String())
	}
	return &spacev1.DeleteVoiceRoomResponse{}, nil
}

func (s *SpaceGRPC) UpsertTreeNode(ctx context.Context, req *spacev1.UpsertTreeNodeRequest) (*spacev1.UpsertTreeNodeResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpaceOwner(ctx, spaceID); err != nil {
		return nil, err
	}

	in := store.UpsertTreeNodeInput{
		SpaceID: spaceID,
		Kind:    req.GetKind(),
	}
	if req.NodeId != nil && req.GetNodeId() != "" {
		nid, parseErr := parseUUIDField("node_id", req.GetNodeId())
		if parseErr != nil {
			return nil, parseErr
		}
		in.NodeID = &nid
	}
	if req.CategoryId != nil && req.GetCategoryId() != "" {
		cid, parseErr := parseUUIDField("category_id", req.GetCategoryId())
		if parseErr != nil {
			return nil, parseErr
		}
		in.CategoryID = &cid
	}
	if req.SortOrder != nil {
		so := req.GetSortOrder()
		in.SortOrder = &so
	}
	if req.IsSystem != nil {
		v := req.GetIsSystem()
		in.IsSystem = &v
	}
	if lc := req.GetLinkedChat(); lc != nil && lc.GetId() != "" {
		cid, parseErr := parseUUIDField("chat_id", lc.GetId())
		if parseErr != nil {
			return nil, parseErr
		}
		in.ChatID = &cid
	}
	if req.VoiceRoomId != nil && req.GetVoiceRoomId() != "" {
		vid, parseErr := parseUUIDField("voice_room_id", req.GetVoiceRoomId())
		if parseErr != nil {
			return nil, parseErr
		}
		in.VoiceRoomID = &vid
	}

	node, err := s.Store.UpsertTreeNode(ctx, in)
	if errors.Is(err, store.ErrTreeNodeLimit) {
		return nil, status.Error(codes.ResourceExhausted, err.Error())
	}
	if errors.Is(err, store.ErrInvalidTreeKind) || errors.Is(err, store.ErrTreeNodeNotFound) {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.publishTreeUpserted(ctx, spaceID, node)
	return &spacev1.UpsertTreeNodeResponse{SpaceTreeNode: treeNodeRowToProto(node)}, nil
}

func (s *SpaceGRPC) RemoveTreeNode(ctx context.Context, req *spacev1.RemoveTreeNodeRequest) (*spacev1.RemoveTreeNodeResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	nodeID, err := parseUUIDField("node_id", req.GetNodeId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpaceOwner(ctx, spaceID); err != nil {
		return nil, err
	}
	if err := s.Store.RemoveTreeNode(ctx, spaceID, nodeID); err != nil {
		if errors.Is(err, store.ErrTreeNodeNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.publishTreeRemoved(ctx, spaceID, nodeID)
	return &spacev1.RemoveTreeNodeResponse{}, nil
}

func (s *SpaceGRPC) ReorderSpaceTree(ctx context.Context, req *spacev1.ReorderSpaceTreeRequest) (*spacev1.ReorderSpaceTreeResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "space persistence not configured")
	}
	spaceID, err := parseUUIDField("space_id", req.GetSpaceId())
	if err != nil {
		return nil, err
	}
	if err := s.requireSpaceOwner(ctx, spaceID); err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(req.GetOrderedNodeIds()))
	for _, raw := range req.GetOrderedNodeIds() {
		id, parseErr := parseUUIDField("ordered_node_ids", raw)
		if parseErr != nil {
			return nil, parseErr
		}
		ids = append(ids, id)
	}
	if err := s.Store.ReorderSpaceTree(ctx, spaceID, ids); err != nil {
		if errors.Is(err, store.ErrInvalidReorder) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &spacev1.ReorderSpaceTreeResponse{}, nil
}

func (s *SpaceGRPC) publishTreeUpserted(ctx context.Context, spaceID uuid.UUID, node *store.TreeNodeRow) {
	if s.SpaceEvents == nil || node == nil {
		return
	}
	var chatID, voiceRoomID string
	if node.ChatID != nil {
		chatID = node.ChatID.String()
	}
	if node.VoiceRoomID != nil {
		voiceRoomID = node.VoiceRoomID.String()
	}
	if err := s.SpaceEvents.PublishTreeNodeUpserted(ctx, spaceID.String(), node.ID.String(), node.Kind, chatID, voiceRoomID); err != nil {
		log.Printf("space: publish tree_node_upserted: %v", err)
	}
}

func (s *SpaceGRPC) publishTreeRemoved(ctx context.Context, spaceID, nodeID uuid.UUID) {
	if s.SpaceEvents == nil {
		return
	}
	if err := s.SpaceEvents.PublishTreeNodeRemoved(ctx, spaceID.String(), nodeID.String()); err != nil {
		log.Printf("space: publish tree_node_removed: %v", err)
	}
}

func spaceTreeDataToProto(data *store.SpaceTreeData) *spacev1.ListSpaceTreeResponse {
	if data == nil {
		return &spacev1.ListSpaceTreeResponse{}
	}
	out := &spacev1.ListSpaceTreeResponse{
		Categories: make([]*spacev1.Category, 0, len(data.Categories)),
		Nodes:      make([]*spacev1.SpaceTreeNode, 0, len(data.Nodes)),
		VoiceRooms: make([]*spacev1.VoiceRoom, 0, len(data.VoiceRooms)),
	}
	for _, c := range data.Categories {
		out.Categories = append(out.Categories, categoryRowToProto(c))
	}
	for _, n := range data.Nodes {
		out.Nodes = append(out.Nodes, treeNodeRowToProto(n))
	}
	for _, r := range data.VoiceRooms {
		out.VoiceRooms = append(out.VoiceRooms, voiceRoomRowToProto(r))
	}
	return out
}

func categoryRowToProto(r *store.CategoryRow) *spacev1.Category {
	if r == nil {
		return nil
	}
	return &spacev1.Category{
		Id:        r.ID.String(),
		SpaceId:   r.SpaceID.String(),
		Name:      r.Name,
		SortOrder: r.SortOrder,
	}
}

func voiceRoomRowToProto(r *store.VoiceRoomRow) *spacev1.VoiceRoom {
	if r == nil {
		return nil
	}
	return &spacev1.VoiceRoom{
		Id:        r.ID.String(),
		SpaceId:   r.SpaceID.String(),
		Name:      r.Name,
		CreatedAt: timestamppb.New(r.CreatedAt),
	}
}

func treeNodeRowToProto(r *store.TreeNodeRow) *spacev1.SpaceTreeNode {
	if r == nil {
		return nil
	}
	out := &spacev1.SpaceTreeNode{
		Id:        r.ID.String(),
		SpaceId:   r.SpaceID.String(),
		Kind:      r.Kind,
		SortOrder: r.SortOrder,
		IsSystem:  r.IsSystem,
	}
	if r.CategoryID != nil {
		cid := r.CategoryID.String()
		out.CategoryId = &cid
	}
	if r.ChatID != nil {
		out.LinkedChat = &chatv1.ChatRef{Id: r.ChatID.String()}
	}
	if r.VoiceRoomID != nil {
		vid := r.VoiceRoomID.String()
		out.VoiceRoomId = &vid
	}
	return out
}
