// This is a generated file - do not edit.
//
// Generated from voice/space/v1/space.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:grpc/service_api.dart' as $grpc;
import 'package:protobuf/protobuf.dart' as $pb;

import 'space.pb.dart' as $0;

export 'space.pb.dart';

/// Spaces, tree, invites. HTTP: /api/v1/spaces/**.
@$pb.GrpcServiceName('voice.space.v1.SpaceService')
class SpaceServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  SpaceServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.CreateSpaceResponse> createSpace(
    $0.CreateSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createSpace, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdateSpaceResponse> updateSpace(
    $0.UpdateSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateSpace, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteSpaceResponse> deleteSpace(
    $0.DeleteSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteSpace, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetSpaceResponse> getSpace(
    $0.GetSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSpace, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListMySpacesResponse> listMySpaces(
    $0.ListMySpacesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listMySpaces, request, options: options);
  }

  $grpc.ResponseFuture<$0.SearchPublicSpacesResponse> searchPublicSpaces(
    $0.SearchPublicSpacesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$searchPublicSpaces, request, options: options);
  }

  $grpc.ResponseFuture<$0.CreateVoiceRoomResponse> createVoiceRoom(
    $0.CreateVoiceRoomRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createVoiceRoom, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdateVoiceRoomResponse> updateVoiceRoom(
    $0.UpdateVoiceRoomRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateVoiceRoom, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteVoiceRoomResponse> deleteVoiceRoom(
    $0.DeleteVoiceRoomRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteVoiceRoom, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpsertTreeNodeResponse> upsertTreeNode(
    $0.UpsertTreeNodeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$upsertTreeNode, request, options: options);
  }

  $grpc.ResponseFuture<$0.RemoveTreeNodeResponse> removeTreeNode(
    $0.RemoveTreeNodeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeTreeNode, request, options: options);
  }

  $grpc.ResponseFuture<$0.CreateCategoryResponse> createCategory(
    $0.CreateCategoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createCategory, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdateCategoryResponse> updateCategory(
    $0.UpdateCategoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateCategory, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteCategoryResponse> deleteCategory(
    $0.DeleteCategoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteCategory, request, options: options);
  }

  $grpc.ResponseFuture<$0.ReorderSpaceTreeResponse> reorderSpaceTree(
    $0.ReorderSpaceTreeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$reorderSpaceTree, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListSpaceTreeResponse> listSpaceTree(
    $0.ListSpaceTreeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listSpaceTree, request, options: options);
  }

  $grpc.ResponseFuture<$0.CreateInviteResponse> createInvite(
    $0.CreateInviteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createInvite, request, options: options);
  }

  $grpc.ResponseFuture<$0.RevokeInviteResponse> revokeInvite(
    $0.RevokeInviteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$revokeInvite, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetInviteResponse> getInvite(
    $0.GetInviteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getInvite, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListInvitesResponse> listInvites(
    $0.ListInvitesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listInvites, request, options: options);
  }

  $grpc.ResponseFuture<$0.JoinByInviteResponse> joinByInvite(
    $0.JoinByInviteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$joinByInvite, request, options: options);
  }

  $grpc.ResponseFuture<$0.JoinSpaceResponse> joinSpace(
    $0.JoinSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$joinSpace, request, options: options);
  }

  $grpc.ResponseFuture<$0.LeaveSpaceResponse> leaveSpace(
    $0.LeaveSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$leaveSpace, request, options: options);
  }

  $grpc.ResponseFuture<$0.KickMemberResponse> kickMember(
    $0.KickMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$kickMember, request, options: options);
  }

  $grpc.ResponseFuture<$0.BanMemberResponse> banMember(
    $0.BanMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$banMember, request, options: options);
  }

  $grpc.ResponseFuture<$0.UnbanMemberResponse> unbanMember(
    $0.UnbanMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$unbanMember, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListMembersResponse> listMembers(
    $0.ListMembersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listMembers, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListBansResponse> listBans(
    $0.ListBansRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listBans, request, options: options);
  }

  $grpc.ResponseFuture<$0.TimeoutMemberResponse> timeoutMember(
    $0.TimeoutMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$timeoutMember, request, options: options);
  }

  $grpc.ResponseFuture<$0.RemoveMemberTimeoutResponse> removeMemberTimeout(
    $0.RemoveMemberTimeoutRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeMemberTimeout, request, options: options);
  }

  $grpc.ResponseFuture<$0.TransferOwnershipResponse> transferOwnership(
    $0.TransferOwnershipRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$transferOwnership, request, options: options);
  }

  /// S2S: Bot Service adds bot actor profile to space membership on install.
  $grpc.ResponseFuture<$0.AddBotMemberResponse> addBotMember(
    $0.AddBotMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$addBotMember, request, options: options);
  }

  $grpc.ResponseFuture<$0.RemoveBotMemberResponse> removeBotMember(
    $0.RemoveBotMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeBotMember, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListTemplatesResponse> listTemplates(
    $0.ListTemplatesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listTemplates, request, options: options);
  }

  $grpc.ResponseFuture<$0.CreateFromTemplateResponse> createFromTemplate(
    $0.CreateFromTemplateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createFromTemplate, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetAuditLogResponse> getAuditLog(
    $0.GetAuditLogRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getAuditLog, request, options: options);
  }

  /// S2S: privacy audience "space members" — shared membership between two profiles.
  $grpc.ResponseFuture<$0.AreCoMembersResponse> areCoMembers(
    $0.AreCoMembersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$areCoMembers, request, options: options);
  }

  // method descriptors

  static final _$createSpace =
      $grpc.ClientMethod<$0.CreateSpaceRequest, $0.CreateSpaceResponse>(
          '/voice.space.v1.SpaceService/CreateSpace',
          ($0.CreateSpaceRequest value) => value.writeToBuffer(),
          $0.CreateSpaceResponse.fromBuffer);
  static final _$updateSpace =
      $grpc.ClientMethod<$0.UpdateSpaceRequest, $0.UpdateSpaceResponse>(
          '/voice.space.v1.SpaceService/UpdateSpace',
          ($0.UpdateSpaceRequest value) => value.writeToBuffer(),
          $0.UpdateSpaceResponse.fromBuffer);
  static final _$deleteSpace =
      $grpc.ClientMethod<$0.DeleteSpaceRequest, $0.DeleteSpaceResponse>(
          '/voice.space.v1.SpaceService/DeleteSpace',
          ($0.DeleteSpaceRequest value) => value.writeToBuffer(),
          $0.DeleteSpaceResponse.fromBuffer);
  static final _$getSpace =
      $grpc.ClientMethod<$0.GetSpaceRequest, $0.GetSpaceResponse>(
          '/voice.space.v1.SpaceService/GetSpace',
          ($0.GetSpaceRequest value) => value.writeToBuffer(),
          $0.GetSpaceResponse.fromBuffer);
  static final _$listMySpaces =
      $grpc.ClientMethod<$0.ListMySpacesRequest, $0.ListMySpacesResponse>(
          '/voice.space.v1.SpaceService/ListMySpaces',
          ($0.ListMySpacesRequest value) => value.writeToBuffer(),
          $0.ListMySpacesResponse.fromBuffer);
  static final _$searchPublicSpaces = $grpc.ClientMethod<
          $0.SearchPublicSpacesRequest, $0.SearchPublicSpacesResponse>(
      '/voice.space.v1.SpaceService/SearchPublicSpaces',
      ($0.SearchPublicSpacesRequest value) => value.writeToBuffer(),
      $0.SearchPublicSpacesResponse.fromBuffer);
  static final _$createVoiceRoom =
      $grpc.ClientMethod<$0.CreateVoiceRoomRequest, $0.CreateVoiceRoomResponse>(
          '/voice.space.v1.SpaceService/CreateVoiceRoom',
          ($0.CreateVoiceRoomRequest value) => value.writeToBuffer(),
          $0.CreateVoiceRoomResponse.fromBuffer);
  static final _$updateVoiceRoom =
      $grpc.ClientMethod<$0.UpdateVoiceRoomRequest, $0.UpdateVoiceRoomResponse>(
          '/voice.space.v1.SpaceService/UpdateVoiceRoom',
          ($0.UpdateVoiceRoomRequest value) => value.writeToBuffer(),
          $0.UpdateVoiceRoomResponse.fromBuffer);
  static final _$deleteVoiceRoom =
      $grpc.ClientMethod<$0.DeleteVoiceRoomRequest, $0.DeleteVoiceRoomResponse>(
          '/voice.space.v1.SpaceService/DeleteVoiceRoom',
          ($0.DeleteVoiceRoomRequest value) => value.writeToBuffer(),
          $0.DeleteVoiceRoomResponse.fromBuffer);
  static final _$upsertTreeNode =
      $grpc.ClientMethod<$0.UpsertTreeNodeRequest, $0.UpsertTreeNodeResponse>(
          '/voice.space.v1.SpaceService/UpsertTreeNode',
          ($0.UpsertTreeNodeRequest value) => value.writeToBuffer(),
          $0.UpsertTreeNodeResponse.fromBuffer);
  static final _$removeTreeNode =
      $grpc.ClientMethod<$0.RemoveTreeNodeRequest, $0.RemoveTreeNodeResponse>(
          '/voice.space.v1.SpaceService/RemoveTreeNode',
          ($0.RemoveTreeNodeRequest value) => value.writeToBuffer(),
          $0.RemoveTreeNodeResponse.fromBuffer);
  static final _$createCategory =
      $grpc.ClientMethod<$0.CreateCategoryRequest, $0.CreateCategoryResponse>(
          '/voice.space.v1.SpaceService/CreateCategory',
          ($0.CreateCategoryRequest value) => value.writeToBuffer(),
          $0.CreateCategoryResponse.fromBuffer);
  static final _$updateCategory =
      $grpc.ClientMethod<$0.UpdateCategoryRequest, $0.UpdateCategoryResponse>(
          '/voice.space.v1.SpaceService/UpdateCategory',
          ($0.UpdateCategoryRequest value) => value.writeToBuffer(),
          $0.UpdateCategoryResponse.fromBuffer);
  static final _$deleteCategory =
      $grpc.ClientMethod<$0.DeleteCategoryRequest, $0.DeleteCategoryResponse>(
          '/voice.space.v1.SpaceService/DeleteCategory',
          ($0.DeleteCategoryRequest value) => value.writeToBuffer(),
          $0.DeleteCategoryResponse.fromBuffer);
  static final _$reorderSpaceTree = $grpc.ClientMethod<
          $0.ReorderSpaceTreeRequest, $0.ReorderSpaceTreeResponse>(
      '/voice.space.v1.SpaceService/ReorderSpaceTree',
      ($0.ReorderSpaceTreeRequest value) => value.writeToBuffer(),
      $0.ReorderSpaceTreeResponse.fromBuffer);
  static final _$listSpaceTree =
      $grpc.ClientMethod<$0.ListSpaceTreeRequest, $0.ListSpaceTreeResponse>(
          '/voice.space.v1.SpaceService/ListSpaceTree',
          ($0.ListSpaceTreeRequest value) => value.writeToBuffer(),
          $0.ListSpaceTreeResponse.fromBuffer);
  static final _$createInvite =
      $grpc.ClientMethod<$0.CreateInviteRequest, $0.CreateInviteResponse>(
          '/voice.space.v1.SpaceService/CreateInvite',
          ($0.CreateInviteRequest value) => value.writeToBuffer(),
          $0.CreateInviteResponse.fromBuffer);
  static final _$revokeInvite =
      $grpc.ClientMethod<$0.RevokeInviteRequest, $0.RevokeInviteResponse>(
          '/voice.space.v1.SpaceService/RevokeInvite',
          ($0.RevokeInviteRequest value) => value.writeToBuffer(),
          $0.RevokeInviteResponse.fromBuffer);
  static final _$getInvite =
      $grpc.ClientMethod<$0.GetInviteRequest, $0.GetInviteResponse>(
          '/voice.space.v1.SpaceService/GetInvite',
          ($0.GetInviteRequest value) => value.writeToBuffer(),
          $0.GetInviteResponse.fromBuffer);
  static final _$listInvites =
      $grpc.ClientMethod<$0.ListInvitesRequest, $0.ListInvitesResponse>(
          '/voice.space.v1.SpaceService/ListInvites',
          ($0.ListInvitesRequest value) => value.writeToBuffer(),
          $0.ListInvitesResponse.fromBuffer);
  static final _$joinByInvite =
      $grpc.ClientMethod<$0.JoinByInviteRequest, $0.JoinByInviteResponse>(
          '/voice.space.v1.SpaceService/JoinByInvite',
          ($0.JoinByInviteRequest value) => value.writeToBuffer(),
          $0.JoinByInviteResponse.fromBuffer);
  static final _$joinSpace =
      $grpc.ClientMethod<$0.JoinSpaceRequest, $0.JoinSpaceResponse>(
          '/voice.space.v1.SpaceService/JoinSpace',
          ($0.JoinSpaceRequest value) => value.writeToBuffer(),
          $0.JoinSpaceResponse.fromBuffer);
  static final _$leaveSpace =
      $grpc.ClientMethod<$0.LeaveSpaceRequest, $0.LeaveSpaceResponse>(
          '/voice.space.v1.SpaceService/LeaveSpace',
          ($0.LeaveSpaceRequest value) => value.writeToBuffer(),
          $0.LeaveSpaceResponse.fromBuffer);
  static final _$kickMember =
      $grpc.ClientMethod<$0.KickMemberRequest, $0.KickMemberResponse>(
          '/voice.space.v1.SpaceService/KickMember',
          ($0.KickMemberRequest value) => value.writeToBuffer(),
          $0.KickMemberResponse.fromBuffer);
  static final _$banMember =
      $grpc.ClientMethod<$0.BanMemberRequest, $0.BanMemberResponse>(
          '/voice.space.v1.SpaceService/BanMember',
          ($0.BanMemberRequest value) => value.writeToBuffer(),
          $0.BanMemberResponse.fromBuffer);
  static final _$unbanMember =
      $grpc.ClientMethod<$0.UnbanMemberRequest, $0.UnbanMemberResponse>(
          '/voice.space.v1.SpaceService/UnbanMember',
          ($0.UnbanMemberRequest value) => value.writeToBuffer(),
          $0.UnbanMemberResponse.fromBuffer);
  static final _$listMembers =
      $grpc.ClientMethod<$0.ListMembersRequest, $0.ListMembersResponse>(
          '/voice.space.v1.SpaceService/ListMembers',
          ($0.ListMembersRequest value) => value.writeToBuffer(),
          $0.ListMembersResponse.fromBuffer);
  static final _$listBans =
      $grpc.ClientMethod<$0.ListBansRequest, $0.ListBansResponse>(
          '/voice.space.v1.SpaceService/ListBans',
          ($0.ListBansRequest value) => value.writeToBuffer(),
          $0.ListBansResponse.fromBuffer);
  static final _$timeoutMember =
      $grpc.ClientMethod<$0.TimeoutMemberRequest, $0.TimeoutMemberResponse>(
          '/voice.space.v1.SpaceService/TimeoutMember',
          ($0.TimeoutMemberRequest value) => value.writeToBuffer(),
          $0.TimeoutMemberResponse.fromBuffer);
  static final _$removeMemberTimeout = $grpc.ClientMethod<
          $0.RemoveMemberTimeoutRequest, $0.RemoveMemberTimeoutResponse>(
      '/voice.space.v1.SpaceService/RemoveMemberTimeout',
      ($0.RemoveMemberTimeoutRequest value) => value.writeToBuffer(),
      $0.RemoveMemberTimeoutResponse.fromBuffer);
  static final _$transferOwnership = $grpc.ClientMethod<
          $0.TransferOwnershipRequest, $0.TransferOwnershipResponse>(
      '/voice.space.v1.SpaceService/TransferOwnership',
      ($0.TransferOwnershipRequest value) => value.writeToBuffer(),
      $0.TransferOwnershipResponse.fromBuffer);
  static final _$addBotMember =
      $grpc.ClientMethod<$0.AddBotMemberRequest, $0.AddBotMemberResponse>(
          '/voice.space.v1.SpaceService/AddBotMember',
          ($0.AddBotMemberRequest value) => value.writeToBuffer(),
          $0.AddBotMemberResponse.fromBuffer);
  static final _$removeBotMember =
      $grpc.ClientMethod<$0.RemoveBotMemberRequest, $0.RemoveBotMemberResponse>(
          '/voice.space.v1.SpaceService/RemoveBotMember',
          ($0.RemoveBotMemberRequest value) => value.writeToBuffer(),
          $0.RemoveBotMemberResponse.fromBuffer);
  static final _$listTemplates =
      $grpc.ClientMethod<$0.ListTemplatesRequest, $0.ListTemplatesResponse>(
          '/voice.space.v1.SpaceService/ListTemplates',
          ($0.ListTemplatesRequest value) => value.writeToBuffer(),
          $0.ListTemplatesResponse.fromBuffer);
  static final _$createFromTemplate = $grpc.ClientMethod<
          $0.CreateFromTemplateRequest, $0.CreateFromTemplateResponse>(
      '/voice.space.v1.SpaceService/CreateFromTemplate',
      ($0.CreateFromTemplateRequest value) => value.writeToBuffer(),
      $0.CreateFromTemplateResponse.fromBuffer);
  static final _$getAuditLog =
      $grpc.ClientMethod<$0.GetAuditLogRequest, $0.GetAuditLogResponse>(
          '/voice.space.v1.SpaceService/GetAuditLog',
          ($0.GetAuditLogRequest value) => value.writeToBuffer(),
          $0.GetAuditLogResponse.fromBuffer);
  static final _$areCoMembers =
      $grpc.ClientMethod<$0.AreCoMembersRequest, $0.AreCoMembersResponse>(
          '/voice.space.v1.SpaceService/AreCoMembers',
          ($0.AreCoMembersRequest value) => value.writeToBuffer(),
          $0.AreCoMembersResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.space.v1.SpaceService')
abstract class SpaceServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.space.v1.SpaceService';

  SpaceServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.CreateSpaceRequest, $0.CreateSpaceResponse>(
            'CreateSpace',
            createSpace_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.CreateSpaceRequest.fromBuffer(value),
            ($0.CreateSpaceResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.UpdateSpaceRequest, $0.UpdateSpaceResponse>(
            'UpdateSpace',
            updateSpace_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.UpdateSpaceRequest.fromBuffer(value),
            ($0.UpdateSpaceResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.DeleteSpaceRequest, $0.DeleteSpaceResponse>(
            'DeleteSpace',
            deleteSpace_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.DeleteSpaceRequest.fromBuffer(value),
            ($0.DeleteSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetSpaceRequest, $0.GetSpaceResponse>(
        'GetSpace',
        getSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetSpaceRequest.fromBuffer(value),
        ($0.GetSpaceResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListMySpacesRequest, $0.ListMySpacesResponse>(
            'ListMySpaces',
            listMySpaces_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListMySpacesRequest.fromBuffer(value),
            ($0.ListMySpacesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SearchPublicSpacesRequest,
            $0.SearchPublicSpacesResponse>(
        'SearchPublicSpaces',
        searchPublicSpaces_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SearchPublicSpacesRequest.fromBuffer(value),
        ($0.SearchPublicSpacesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CreateVoiceRoomRequest,
            $0.CreateVoiceRoomResponse>(
        'CreateVoiceRoom',
        createVoiceRoom_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CreateVoiceRoomRequest.fromBuffer(value),
        ($0.CreateVoiceRoomResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpdateVoiceRoomRequest,
            $0.UpdateVoiceRoomResponse>(
        'UpdateVoiceRoom',
        updateVoiceRoom_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UpdateVoiceRoomRequest.fromBuffer(value),
        ($0.UpdateVoiceRoomResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeleteVoiceRoomRequest,
            $0.DeleteVoiceRoomResponse>(
        'DeleteVoiceRoom',
        deleteVoiceRoom_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DeleteVoiceRoomRequest.fromBuffer(value),
        ($0.DeleteVoiceRoomResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpsertTreeNodeRequest,
            $0.UpsertTreeNodeResponse>(
        'UpsertTreeNode',
        upsertTreeNode_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UpsertTreeNodeRequest.fromBuffer(value),
        ($0.UpsertTreeNodeResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RemoveTreeNodeRequest,
            $0.RemoveTreeNodeResponse>(
        'RemoveTreeNode',
        removeTreeNode_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RemoveTreeNodeRequest.fromBuffer(value),
        ($0.RemoveTreeNodeResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CreateCategoryRequest,
            $0.CreateCategoryResponse>(
        'CreateCategory',
        createCategory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CreateCategoryRequest.fromBuffer(value),
        ($0.CreateCategoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpdateCategoryRequest,
            $0.UpdateCategoryResponse>(
        'UpdateCategory',
        updateCategory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UpdateCategoryRequest.fromBuffer(value),
        ($0.UpdateCategoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeleteCategoryRequest,
            $0.DeleteCategoryResponse>(
        'DeleteCategory',
        deleteCategory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DeleteCategoryRequest.fromBuffer(value),
        ($0.DeleteCategoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ReorderSpaceTreeRequest,
            $0.ReorderSpaceTreeResponse>(
        'ReorderSpaceTree',
        reorderSpaceTree_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ReorderSpaceTreeRequest.fromBuffer(value),
        ($0.ReorderSpaceTreeResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListSpaceTreeRequest, $0.ListSpaceTreeResponse>(
            'ListSpaceTree',
            listSpaceTree_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListSpaceTreeRequest.fromBuffer(value),
            ($0.ListSpaceTreeResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.CreateInviteRequest, $0.CreateInviteResponse>(
            'CreateInvite',
            createInvite_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.CreateInviteRequest.fromBuffer(value),
            ($0.CreateInviteResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.RevokeInviteRequest, $0.RevokeInviteResponse>(
            'RevokeInvite',
            revokeInvite_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.RevokeInviteRequest.fromBuffer(value),
            ($0.RevokeInviteResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetInviteRequest, $0.GetInviteResponse>(
        'GetInvite',
        getInvite_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetInviteRequest.fromBuffer(value),
        ($0.GetInviteResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListInvitesRequest, $0.ListInvitesResponse>(
            'ListInvites',
            listInvites_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListInvitesRequest.fromBuffer(value),
            ($0.ListInvitesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.JoinByInviteRequest, $0.JoinByInviteResponse>(
            'JoinByInvite',
            joinByInvite_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.JoinByInviteRequest.fromBuffer(value),
            ($0.JoinByInviteResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.JoinSpaceRequest, $0.JoinSpaceResponse>(
        'JoinSpace',
        joinSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.JoinSpaceRequest.fromBuffer(value),
        ($0.JoinSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.LeaveSpaceRequest, $0.LeaveSpaceResponse>(
        'LeaveSpace',
        leaveSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.LeaveSpaceRequest.fromBuffer(value),
        ($0.LeaveSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.KickMemberRequest, $0.KickMemberResponse>(
        'KickMember',
        kickMember_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.KickMemberRequest.fromBuffer(value),
        ($0.KickMemberResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.BanMemberRequest, $0.BanMemberResponse>(
        'BanMember',
        banMember_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.BanMemberRequest.fromBuffer(value),
        ($0.BanMemberResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.UnbanMemberRequest, $0.UnbanMemberResponse>(
            'UnbanMember',
            unbanMember_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.UnbanMemberRequest.fromBuffer(value),
            ($0.UnbanMemberResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListMembersRequest, $0.ListMembersResponse>(
            'ListMembers',
            listMembers_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListMembersRequest.fromBuffer(value),
            ($0.ListMembersResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListBansRequest, $0.ListBansResponse>(
        'ListBans',
        listBans_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ListBansRequest.fromBuffer(value),
        ($0.ListBansResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.TimeoutMemberRequest, $0.TimeoutMemberResponse>(
            'TimeoutMember',
            timeoutMember_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.TimeoutMemberRequest.fromBuffer(value),
            ($0.TimeoutMemberResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RemoveMemberTimeoutRequest,
            $0.RemoveMemberTimeoutResponse>(
        'RemoveMemberTimeout',
        removeMemberTimeout_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RemoveMemberTimeoutRequest.fromBuffer(value),
        ($0.RemoveMemberTimeoutResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.TransferOwnershipRequest,
            $0.TransferOwnershipResponse>(
        'TransferOwnership',
        transferOwnership_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.TransferOwnershipRequest.fromBuffer(value),
        ($0.TransferOwnershipResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.AddBotMemberRequest, $0.AddBotMemberResponse>(
            'AddBotMember',
            addBotMember_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.AddBotMemberRequest.fromBuffer(value),
            ($0.AddBotMemberResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RemoveBotMemberRequest,
            $0.RemoveBotMemberResponse>(
        'RemoveBotMember',
        removeBotMember_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RemoveBotMemberRequest.fromBuffer(value),
        ($0.RemoveBotMemberResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListTemplatesRequest, $0.ListTemplatesResponse>(
            'ListTemplates',
            listTemplates_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListTemplatesRequest.fromBuffer(value),
            ($0.ListTemplatesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CreateFromTemplateRequest,
            $0.CreateFromTemplateResponse>(
        'CreateFromTemplate',
        createFromTemplate_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CreateFromTemplateRequest.fromBuffer(value),
        ($0.CreateFromTemplateResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetAuditLogRequest, $0.GetAuditLogResponse>(
            'GetAuditLog',
            getAuditLog_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetAuditLogRequest.fromBuffer(value),
            ($0.GetAuditLogResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.AreCoMembersRequest, $0.AreCoMembersResponse>(
            'AreCoMembers',
            areCoMembers_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.AreCoMembersRequest.fromBuffer(value),
            ($0.AreCoMembersResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.CreateSpaceResponse> createSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CreateSpaceRequest> $request) async {
    return createSpace($call, await $request);
  }

  $async.Future<$0.CreateSpaceResponse> createSpace(
      $grpc.ServiceCall call, $0.CreateSpaceRequest request);

  $async.Future<$0.UpdateSpaceResponse> updateSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$0.UpdateSpaceRequest> $request) async {
    return updateSpace($call, await $request);
  }

  $async.Future<$0.UpdateSpaceResponse> updateSpace(
      $grpc.ServiceCall call, $0.UpdateSpaceRequest request);

  $async.Future<$0.DeleteSpaceResponse> deleteSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$0.DeleteSpaceRequest> $request) async {
    return deleteSpace($call, await $request);
  }

  $async.Future<$0.DeleteSpaceResponse> deleteSpace(
      $grpc.ServiceCall call, $0.DeleteSpaceRequest request);

  $async.Future<$0.GetSpaceResponse> getSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetSpaceRequest> $request) async {
    return getSpace($call, await $request);
  }

  $async.Future<$0.GetSpaceResponse> getSpace(
      $grpc.ServiceCall call, $0.GetSpaceRequest request);

  $async.Future<$0.ListMySpacesResponse> listMySpaces_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListMySpacesRequest> $request) async {
    return listMySpaces($call, await $request);
  }

  $async.Future<$0.ListMySpacesResponse> listMySpaces(
      $grpc.ServiceCall call, $0.ListMySpacesRequest request);

  $async.Future<$0.SearchPublicSpacesResponse> searchPublicSpaces_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SearchPublicSpacesRequest> $request) async {
    return searchPublicSpaces($call, await $request);
  }

  $async.Future<$0.SearchPublicSpacesResponse> searchPublicSpaces(
      $grpc.ServiceCall call, $0.SearchPublicSpacesRequest request);

  $async.Future<$0.CreateVoiceRoomResponse> createVoiceRoom_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CreateVoiceRoomRequest> $request) async {
    return createVoiceRoom($call, await $request);
  }

  $async.Future<$0.CreateVoiceRoomResponse> createVoiceRoom(
      $grpc.ServiceCall call, $0.CreateVoiceRoomRequest request);

  $async.Future<$0.UpdateVoiceRoomResponse> updateVoiceRoom_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UpdateVoiceRoomRequest> $request) async {
    return updateVoiceRoom($call, await $request);
  }

  $async.Future<$0.UpdateVoiceRoomResponse> updateVoiceRoom(
      $grpc.ServiceCall call, $0.UpdateVoiceRoomRequest request);

  $async.Future<$0.DeleteVoiceRoomResponse> deleteVoiceRoom_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DeleteVoiceRoomRequest> $request) async {
    return deleteVoiceRoom($call, await $request);
  }

  $async.Future<$0.DeleteVoiceRoomResponse> deleteVoiceRoom(
      $grpc.ServiceCall call, $0.DeleteVoiceRoomRequest request);

  $async.Future<$0.UpsertTreeNodeResponse> upsertTreeNode_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UpsertTreeNodeRequest> $request) async {
    return upsertTreeNode($call, await $request);
  }

  $async.Future<$0.UpsertTreeNodeResponse> upsertTreeNode(
      $grpc.ServiceCall call, $0.UpsertTreeNodeRequest request);

  $async.Future<$0.RemoveTreeNodeResponse> removeTreeNode_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RemoveTreeNodeRequest> $request) async {
    return removeTreeNode($call, await $request);
  }

  $async.Future<$0.RemoveTreeNodeResponse> removeTreeNode(
      $grpc.ServiceCall call, $0.RemoveTreeNodeRequest request);

  $async.Future<$0.CreateCategoryResponse> createCategory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CreateCategoryRequest> $request) async {
    return createCategory($call, await $request);
  }

  $async.Future<$0.CreateCategoryResponse> createCategory(
      $grpc.ServiceCall call, $0.CreateCategoryRequest request);

  $async.Future<$0.UpdateCategoryResponse> updateCategory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UpdateCategoryRequest> $request) async {
    return updateCategory($call, await $request);
  }

  $async.Future<$0.UpdateCategoryResponse> updateCategory(
      $grpc.ServiceCall call, $0.UpdateCategoryRequest request);

  $async.Future<$0.DeleteCategoryResponse> deleteCategory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DeleteCategoryRequest> $request) async {
    return deleteCategory($call, await $request);
  }

  $async.Future<$0.DeleteCategoryResponse> deleteCategory(
      $grpc.ServiceCall call, $0.DeleteCategoryRequest request);

  $async.Future<$0.ReorderSpaceTreeResponse> reorderSpaceTree_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ReorderSpaceTreeRequest> $request) async {
    return reorderSpaceTree($call, await $request);
  }

  $async.Future<$0.ReorderSpaceTreeResponse> reorderSpaceTree(
      $grpc.ServiceCall call, $0.ReorderSpaceTreeRequest request);

  $async.Future<$0.ListSpaceTreeResponse> listSpaceTree_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListSpaceTreeRequest> $request) async {
    return listSpaceTree($call, await $request);
  }

  $async.Future<$0.ListSpaceTreeResponse> listSpaceTree(
      $grpc.ServiceCall call, $0.ListSpaceTreeRequest request);

  $async.Future<$0.CreateInviteResponse> createInvite_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CreateInviteRequest> $request) async {
    return createInvite($call, await $request);
  }

  $async.Future<$0.CreateInviteResponse> createInvite(
      $grpc.ServiceCall call, $0.CreateInviteRequest request);

  $async.Future<$0.RevokeInviteResponse> revokeInvite_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RevokeInviteRequest> $request) async {
    return revokeInvite($call, await $request);
  }

  $async.Future<$0.RevokeInviteResponse> revokeInvite(
      $grpc.ServiceCall call, $0.RevokeInviteRequest request);

  $async.Future<$0.GetInviteResponse> getInvite_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetInviteRequest> $request) async {
    return getInvite($call, await $request);
  }

  $async.Future<$0.GetInviteResponse> getInvite(
      $grpc.ServiceCall call, $0.GetInviteRequest request);

  $async.Future<$0.ListInvitesResponse> listInvites_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListInvitesRequest> $request) async {
    return listInvites($call, await $request);
  }

  $async.Future<$0.ListInvitesResponse> listInvites(
      $grpc.ServiceCall call, $0.ListInvitesRequest request);

  $async.Future<$0.JoinByInviteResponse> joinByInvite_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.JoinByInviteRequest> $request) async {
    return joinByInvite($call, await $request);
  }

  $async.Future<$0.JoinByInviteResponse> joinByInvite(
      $grpc.ServiceCall call, $0.JoinByInviteRequest request);

  $async.Future<$0.JoinSpaceResponse> joinSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$0.JoinSpaceRequest> $request) async {
    return joinSpace($call, await $request);
  }

  $async.Future<$0.JoinSpaceResponse> joinSpace(
      $grpc.ServiceCall call, $0.JoinSpaceRequest request);

  $async.Future<$0.LeaveSpaceResponse> leaveSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$0.LeaveSpaceRequest> $request) async {
    return leaveSpace($call, await $request);
  }

  $async.Future<$0.LeaveSpaceResponse> leaveSpace(
      $grpc.ServiceCall call, $0.LeaveSpaceRequest request);

  $async.Future<$0.KickMemberResponse> kickMember_Pre($grpc.ServiceCall $call,
      $async.Future<$0.KickMemberRequest> $request) async {
    return kickMember($call, await $request);
  }

  $async.Future<$0.KickMemberResponse> kickMember(
      $grpc.ServiceCall call, $0.KickMemberRequest request);

  $async.Future<$0.BanMemberResponse> banMember_Pre($grpc.ServiceCall $call,
      $async.Future<$0.BanMemberRequest> $request) async {
    return banMember($call, await $request);
  }

  $async.Future<$0.BanMemberResponse> banMember(
      $grpc.ServiceCall call, $0.BanMemberRequest request);

  $async.Future<$0.UnbanMemberResponse> unbanMember_Pre($grpc.ServiceCall $call,
      $async.Future<$0.UnbanMemberRequest> $request) async {
    return unbanMember($call, await $request);
  }

  $async.Future<$0.UnbanMemberResponse> unbanMember(
      $grpc.ServiceCall call, $0.UnbanMemberRequest request);

  $async.Future<$0.ListMembersResponse> listMembers_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListMembersRequest> $request) async {
    return listMembers($call, await $request);
  }

  $async.Future<$0.ListMembersResponse> listMembers(
      $grpc.ServiceCall call, $0.ListMembersRequest request);

  $async.Future<$0.ListBansResponse> listBans_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListBansRequest> $request) async {
    return listBans($call, await $request);
  }

  $async.Future<$0.ListBansResponse> listBans(
      $grpc.ServiceCall call, $0.ListBansRequest request);

  $async.Future<$0.TimeoutMemberResponse> timeoutMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.TimeoutMemberRequest> $request) async {
    return timeoutMember($call, await $request);
  }

  $async.Future<$0.TimeoutMemberResponse> timeoutMember(
      $grpc.ServiceCall call, $0.TimeoutMemberRequest request);

  $async.Future<$0.RemoveMemberTimeoutResponse> removeMemberTimeout_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RemoveMemberTimeoutRequest> $request) async {
    return removeMemberTimeout($call, await $request);
  }

  $async.Future<$0.RemoveMemberTimeoutResponse> removeMemberTimeout(
      $grpc.ServiceCall call, $0.RemoveMemberTimeoutRequest request);

  $async.Future<$0.TransferOwnershipResponse> transferOwnership_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.TransferOwnershipRequest> $request) async {
    return transferOwnership($call, await $request);
  }

  $async.Future<$0.TransferOwnershipResponse> transferOwnership(
      $grpc.ServiceCall call, $0.TransferOwnershipRequest request);

  $async.Future<$0.AddBotMemberResponse> addBotMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.AddBotMemberRequest> $request) async {
    return addBotMember($call, await $request);
  }

  $async.Future<$0.AddBotMemberResponse> addBotMember(
      $grpc.ServiceCall call, $0.AddBotMemberRequest request);

  $async.Future<$0.RemoveBotMemberResponse> removeBotMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RemoveBotMemberRequest> $request) async {
    return removeBotMember($call, await $request);
  }

  $async.Future<$0.RemoveBotMemberResponse> removeBotMember(
      $grpc.ServiceCall call, $0.RemoveBotMemberRequest request);

  $async.Future<$0.ListTemplatesResponse> listTemplates_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListTemplatesRequest> $request) async {
    return listTemplates($call, await $request);
  }

  $async.Future<$0.ListTemplatesResponse> listTemplates(
      $grpc.ServiceCall call, $0.ListTemplatesRequest request);

  $async.Future<$0.CreateFromTemplateResponse> createFromTemplate_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CreateFromTemplateRequest> $request) async {
    return createFromTemplate($call, await $request);
  }

  $async.Future<$0.CreateFromTemplateResponse> createFromTemplate(
      $grpc.ServiceCall call, $0.CreateFromTemplateRequest request);

  $async.Future<$0.GetAuditLogResponse> getAuditLog_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetAuditLogRequest> $request) async {
    return getAuditLog($call, await $request);
  }

  $async.Future<$0.GetAuditLogResponse> getAuditLog(
      $grpc.ServiceCall call, $0.GetAuditLogRequest request);

  $async.Future<$0.AreCoMembersResponse> areCoMembers_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.AreCoMembersRequest> $request) async {
    return areCoMembers($call, await $request);
  }

  $async.Future<$0.AreCoMembersResponse> areCoMembers(
      $grpc.ServiceCall call, $0.AreCoMembersRequest request);
}
