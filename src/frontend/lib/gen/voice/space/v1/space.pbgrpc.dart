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

import 'member_admission.pb.dart' as $0;
import 'space.pb.dart' as $1;

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

  /// Narrow native membership authority; own DB only, no Role callback.
  /// @voice.security=protected;callers=service:role
  $grpc.ResponseFuture<$0.GetMemberAdmissionAuthorityResponse>
      getMemberAdmissionAuthority(
    $0.GetMemberAdmissionAuthorityRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getMemberAdmissionAuthority, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.CreateSpaceResponse> createSpace(
    $1.CreateSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createSpace, request, options: options);
  }

  $grpc.ResponseFuture<$1.UpdateSpaceResponse> updateSpace(
    $1.UpdateSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateSpace, request, options: options);
  }

  /// Space-level matchmaking settings (mm_config jsonb; roadmap П.1).
  $grpc.ResponseFuture<$1.UpdateSpaceMmConfigResponse> updateSpaceMmConfig(
    $1.UpdateSpaceMmConfigRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateSpaceMmConfig, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$1.DeleteSpaceResponse> deleteSpace(
    $1.DeleteSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteSpace, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetSpaceResponse> getSpace(
    $1.GetSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSpace, request, options: options);
  }

  /// Narrow existence/lifecycle read. No ACL lookup or public Gateway route.
  /// @voice.security=protected;callers=service:role
  $grpc.ResponseFuture<$1.ValidateRoleOverrideTargetResponse>
      validateRoleOverrideTarget(
    $1.ValidateRoleOverrideTargetRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$validateRoleOverrideTarget, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.ListMySpacesResponse> listMySpaces(
    $1.ListMySpacesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listMySpaces, request, options: options);
  }

  $grpc.ResponseFuture<$1.SearchPublicSpacesResponse> searchPublicSpaces(
    $1.SearchPublicSpacesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$searchPublicSpaces, request, options: options);
  }

  $grpc.ResponseFuture<$1.CreateVoiceRoomResponse> createVoiceRoom(
    $1.CreateVoiceRoomRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createVoiceRoom, request, options: options);
  }

  $grpc.ResponseFuture<$1.UpdateVoiceRoomResponse> updateVoiceRoom(
    $1.UpdateVoiceRoomRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateVoiceRoom, request, options: options);
  }

  $grpc.ResponseFuture<$1.DeleteVoiceRoomResponse> deleteVoiceRoom(
    $1.DeleteVoiceRoomRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteVoiceRoom, request, options: options);
  }

  $grpc.ResponseFuture<$1.UpsertTreeNodeResponse> upsertTreeNode(
    $1.UpsertTreeNodeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$upsertTreeNode, request, options: options);
  }

  $grpc.ResponseFuture<$1.RemoveTreeNodeResponse> removeTreeNode(
    $1.RemoveTreeNodeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeTreeNode, request, options: options);
  }

  $grpc.ResponseFuture<$1.CreateCategoryResponse> createCategory(
    $1.CreateCategoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createCategory, request, options: options);
  }

  $grpc.ResponseFuture<$1.UpdateCategoryResponse> updateCategory(
    $1.UpdateCategoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateCategory, request, options: options);
  }

  $grpc.ResponseFuture<$1.DeleteCategoryResponse> deleteCategory(
    $1.DeleteCategoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteCategory, request, options: options);
  }

  $grpc.ResponseFuture<$1.ReorderSpaceTreeResponse> reorderSpaceTree(
    $1.ReorderSpaceTreeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$reorderSpaceTree, request, options: options);
  }

  $grpc.ResponseFuture<$1.PinTreeNodeResponse> pinTreeNode(
    $1.PinTreeNodeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$pinTreeNode, request, options: options);
  }

  $grpc.ResponseFuture<$1.UnpinTreeNodeResponse> unpinTreeNode(
    $1.UnpinTreeNodeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$unpinTreeNode, request, options: options);
  }

  $grpc.ResponseFuture<$1.ListSpaceTreeResponse> listSpaceTree(
    $1.ListSpaceTreeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listSpaceTree, request, options: options);
  }

  $grpc.ResponseFuture<$1.CreateInviteResponse> createInvite(
    $1.CreateInviteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createInvite, request, options: options);
  }

  $grpc.ResponseFuture<$1.RevokeInviteResponse> revokeInvite(
    $1.RevokeInviteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$revokeInvite, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetInviteResponse> getInvite(
    $1.GetInviteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getInvite, request, options: options);
  }

  $grpc.ResponseFuture<$1.ListInvitesResponse> listInvites(
    $1.ListInvitesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listInvites, request, options: options);
  }

  $grpc.ResponseFuture<$1.JoinByInviteResponse> joinByInvite(
    $1.JoinByInviteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$joinByInvite, request, options: options);
  }

  $grpc.ResponseFuture<$1.JoinSpaceResponse> joinSpace(
    $1.JoinSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$joinSpace, request, options: options);
  }

  /// Exact authenticated applicant; no ambient proof or latest-attempt lookup.
  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$1.StartJoinAttemptResponse> startJoinAttempt(
    $1.StartJoinAttemptRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$startJoinAttempt, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$1.GetJoinAttemptResponse> getJoinAttempt(
    $1.GetJoinAttemptRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getJoinAttempt, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$1.SubmitJoinAttemptResponse> submitJoinAttempt(
    $1.SubmitJoinAttemptRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$submitJoinAttempt, request, options: options);
  }

  /// Current member plus SPACE_APPROVE_JOIN_REQUESTS; not invite/settings ACL.
  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$1.ListJoinRequestsResponse> listJoinRequests(
    $1.ListJoinRequestsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listJoinRequests, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$1.GetJoinRequestResponse> getJoinRequest(
    $1.GetJoinRequestRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getJoinRequest, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$1.ApproveJoinRequestResponse> approveJoinRequest(
    $1.ApproveJoinRequestRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$approveJoinRequest, request, options: options);
  }

  $grpc.ResponseFuture<$1.LeaveSpaceResponse> leaveSpace(
    $1.LeaveSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$leaveSpace, request, options: options);
  }

  $grpc.ResponseFuture<$1.KickMemberResponse> kickMember(
    $1.KickMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$kickMember, request, options: options);
  }

  $grpc.ResponseFuture<$1.BanMemberResponse> banMember(
    $1.BanMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$banMember, request, options: options);
  }

  $grpc.ResponseFuture<$1.UnbanMemberResponse> unbanMember(
    $1.UnbanMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$unbanMember, request, options: options);
  }

  $grpc.ResponseFuture<$1.ListMembersResponse> listMembers(
    $1.ListMembersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listMembers, request, options: options);
  }

  /// BE-234-nickname-v1: verified actor, global MEMBER_MANAGE_NICKNAMES;
  /// local members only, Owner protected, serialized redacted audit/outbox.
  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$1.UpdateMemberNicknameResponse> updateMemberNickname(
    $1.UpdateMemberNicknameRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateMemberNickname, request, options: options);
  }

  $grpc.ResponseFuture<$1.ListBansResponse> listBans(
    $1.ListBansRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listBans, request, options: options);
  }

  $grpc.ResponseFuture<$1.TimeoutMemberResponse> timeoutMember(
    $1.TimeoutMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$timeoutMember, request, options: options);
  }

  $grpc.ResponseFuture<$1.RemoveMemberTimeoutResponse> removeMemberTimeout(
    $1.RemoveMemberTimeoutRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeMemberTimeout, request, options: options);
  }

  $grpc.ResponseFuture<$1.TransferOwnershipResponse> transferOwnership(
    $1.TransferOwnershipRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$transferOwnership, request, options: options);
  }

  /// S2S: Bot Service adds bot actor profile to space membership on install.
  $grpc.ResponseFuture<$1.AddBotMemberResponse> addBotMember(
    $1.AddBotMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$addBotMember, request, options: options);
  }

  $grpc.ResponseFuture<$1.RemoveBotMemberResponse> removeBotMember(
    $1.RemoveBotMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeBotMember, request, options: options);
  }

  $grpc.ResponseFuture<$1.ListTemplatesResponse> listTemplates(
    $1.ListTemplatesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listTemplates, request, options: options);
  }

  $grpc.ResponseFuture<$1.CreateFromTemplateResponse> createFromTemplate(
    $1.CreateFromTemplateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createFromTemplate, request, options: options);
  }

  /// GIS-only versioned complete roster projection. Membership remains source-scoped and expires fail-closed.
  /// @voice.security=protected;callers=service:gameintegration
  $grpc.ResponseFuture<$1.ApplyCommunityRosterResponse> applyCommunityRoster(
    $1.ApplyCommunityRosterRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$applyCommunityRoster, request, options: options);
  }

  /// GIS-only idempotent community bootstrap. Owner identity is asserted by the authenticated GIS workload.
  /// @voice.security=protected;callers=service:gameintegration
  $grpc.ResponseFuture<$1.CreateCommunityBootstrapResponse>
      createCommunityBootstrap(
    $1.CreateCommunityBootstrapRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createCommunityBootstrap, request,
        options: options);
  }

  /// GIS-only durable Owner succession after a distinct operator approval.
  /// @voice.security=protected;callers=service:gameintegration
  $grpc.ResponseFuture<$1.RecoverCommunityOwnerResponse> recoverCommunityOwner(
    $1.RecoverCommunityOwnerRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$recoverCommunityOwner, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetAuditLogResponse> getAuditLog(
    $1.GetAuditLogRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getAuditLog, request, options: options);
  }

  /// @voice.security=protected;callers=service:role,service:chat
  $grpc.ResponseFuture<$1.AppendAuditEventResponse> appendAuditEvent(
    $1.AppendAuditEventRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$appendAuditEvent, request, options: options);
  }

  /// Chat-owned durable create intent; not an ordinary user tree mutation.
  /// @voice.security=protected;callers=service:chat
  $grpc.ResponseFuture<$1.CommitSpaceChatCreationResponse>
      commitSpaceChatCreation(
    $1.CommitSpaceChatCreationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$commitSpaceChatCreation, request,
        options: options);
  }

  /// Exact immutable create outcome; caller authentication does not replace disclosure ACLs.
  /// Gateway uses a verified delegated-user principal and current disclosure ACL.
  /// @voice.security=protected;callers=service:chat,service:gateway
  $grpc.ResponseFuture<$1.GetSpaceChatCreationOutcomeResponse>
      getSpaceChatCreationOutcome(
    $1.GetSpaceChatCreationOutcomeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSpaceChatCreationOutcome, request,
        options: options);
  }

  /// S2S: privacy audience "space members" — shared membership between two profiles.
  $grpc.ResponseFuture<$1.AreCoMembersResponse> areCoMembers(
    $1.AreCoMembersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$areCoMembers, request, options: options);
  }

  /// S2S: canonical owner and exact membership evidence for Voice authorization.
  /// This endpoint is restricted to the verified Voice service identity.
  /// @voice.security=protected;callers=service:voice
  $grpc.ResponseFuture<$1.ResolveVoiceRoomAccessResponse>
      resolveVoiceRoomAccess(
    $1.ResolveVoiceRoomAccessRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveVoiceRoomAccess, request,
        options: options);
  }

  /// Fresh profile/Space timeout authority; no public or legacy transport.
  /// @voice.security=protected;callers=service:messaging
  $grpc.ResponseFuture<$1.ResolveMemberCommunicationStateResponse>
      resolveMemberCommunicationState(
    $1.ResolveMemberCommunicationStateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveMemberCommunicationState, request,
        options: options);
  }

  /// S2S: Subscription Service syncs Space Pro entitlement cache after webhook.
  $grpc.ResponseFuture<$1.SyncSpaceProSubscriptionResponse>
      syncSpaceProSubscription(
    $1.SyncSpaceProSubscriptionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$syncSpaceProSubscription, request,
        options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$1.RestoreSpaceResponse> restoreSpace(
    $1.RestoreSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$restoreSpace, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  /// @voice.activation=held
  /// @voice.authorization=current_owner
  $grpc.ResponseFuture<$1.GetSpaceDeletionCoordinatorStatusResponse>
      getSpaceDeletionCoordinatorStatus(
    $1.GetSpaceDeletionCoordinatorStatusRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSpaceDeletionCoordinatorStatus, request,
        options: options);
  }

  // method descriptors

  static final _$getMemberAdmissionAuthority = $grpc.ClientMethod<
          $0.GetMemberAdmissionAuthorityRequest,
          $0.GetMemberAdmissionAuthorityResponse>(
      '/voice.space.v1.SpaceService/GetMemberAdmissionAuthority',
      ($0.GetMemberAdmissionAuthorityRequest value) => value.writeToBuffer(),
      $0.GetMemberAdmissionAuthorityResponse.fromBuffer);
  static final _$createSpace =
      $grpc.ClientMethod<$1.CreateSpaceRequest, $1.CreateSpaceResponse>(
          '/voice.space.v1.SpaceService/CreateSpace',
          ($1.CreateSpaceRequest value) => value.writeToBuffer(),
          $1.CreateSpaceResponse.fromBuffer);
  static final _$updateSpace =
      $grpc.ClientMethod<$1.UpdateSpaceRequest, $1.UpdateSpaceResponse>(
          '/voice.space.v1.SpaceService/UpdateSpace',
          ($1.UpdateSpaceRequest value) => value.writeToBuffer(),
          $1.UpdateSpaceResponse.fromBuffer);
  static final _$updateSpaceMmConfig = $grpc.ClientMethod<
          $1.UpdateSpaceMmConfigRequest, $1.UpdateSpaceMmConfigResponse>(
      '/voice.space.v1.SpaceService/UpdateSpaceMmConfig',
      ($1.UpdateSpaceMmConfigRequest value) => value.writeToBuffer(),
      $1.UpdateSpaceMmConfigResponse.fromBuffer);
  static final _$deleteSpace =
      $grpc.ClientMethod<$1.DeleteSpaceRequest, $1.DeleteSpaceResponse>(
          '/voice.space.v1.SpaceService/DeleteSpace',
          ($1.DeleteSpaceRequest value) => value.writeToBuffer(),
          $1.DeleteSpaceResponse.fromBuffer);
  static final _$getSpace =
      $grpc.ClientMethod<$1.GetSpaceRequest, $1.GetSpaceResponse>(
          '/voice.space.v1.SpaceService/GetSpace',
          ($1.GetSpaceRequest value) => value.writeToBuffer(),
          $1.GetSpaceResponse.fromBuffer);
  static final _$validateRoleOverrideTarget = $grpc.ClientMethod<
          $1.ValidateRoleOverrideTargetRequest,
          $1.ValidateRoleOverrideTargetResponse>(
      '/voice.space.v1.SpaceService/ValidateRoleOverrideTarget',
      ($1.ValidateRoleOverrideTargetRequest value) => value.writeToBuffer(),
      $1.ValidateRoleOverrideTargetResponse.fromBuffer);
  static final _$listMySpaces =
      $grpc.ClientMethod<$1.ListMySpacesRequest, $1.ListMySpacesResponse>(
          '/voice.space.v1.SpaceService/ListMySpaces',
          ($1.ListMySpacesRequest value) => value.writeToBuffer(),
          $1.ListMySpacesResponse.fromBuffer);
  static final _$searchPublicSpaces = $grpc.ClientMethod<
          $1.SearchPublicSpacesRequest, $1.SearchPublicSpacesResponse>(
      '/voice.space.v1.SpaceService/SearchPublicSpaces',
      ($1.SearchPublicSpacesRequest value) => value.writeToBuffer(),
      $1.SearchPublicSpacesResponse.fromBuffer);
  static final _$createVoiceRoom =
      $grpc.ClientMethod<$1.CreateVoiceRoomRequest, $1.CreateVoiceRoomResponse>(
          '/voice.space.v1.SpaceService/CreateVoiceRoom',
          ($1.CreateVoiceRoomRequest value) => value.writeToBuffer(),
          $1.CreateVoiceRoomResponse.fromBuffer);
  static final _$updateVoiceRoom =
      $grpc.ClientMethod<$1.UpdateVoiceRoomRequest, $1.UpdateVoiceRoomResponse>(
          '/voice.space.v1.SpaceService/UpdateVoiceRoom',
          ($1.UpdateVoiceRoomRequest value) => value.writeToBuffer(),
          $1.UpdateVoiceRoomResponse.fromBuffer);
  static final _$deleteVoiceRoom =
      $grpc.ClientMethod<$1.DeleteVoiceRoomRequest, $1.DeleteVoiceRoomResponse>(
          '/voice.space.v1.SpaceService/DeleteVoiceRoom',
          ($1.DeleteVoiceRoomRequest value) => value.writeToBuffer(),
          $1.DeleteVoiceRoomResponse.fromBuffer);
  static final _$upsertTreeNode =
      $grpc.ClientMethod<$1.UpsertTreeNodeRequest, $1.UpsertTreeNodeResponse>(
          '/voice.space.v1.SpaceService/UpsertTreeNode',
          ($1.UpsertTreeNodeRequest value) => value.writeToBuffer(),
          $1.UpsertTreeNodeResponse.fromBuffer);
  static final _$removeTreeNode =
      $grpc.ClientMethod<$1.RemoveTreeNodeRequest, $1.RemoveTreeNodeResponse>(
          '/voice.space.v1.SpaceService/RemoveTreeNode',
          ($1.RemoveTreeNodeRequest value) => value.writeToBuffer(),
          $1.RemoveTreeNodeResponse.fromBuffer);
  static final _$createCategory =
      $grpc.ClientMethod<$1.CreateCategoryRequest, $1.CreateCategoryResponse>(
          '/voice.space.v1.SpaceService/CreateCategory',
          ($1.CreateCategoryRequest value) => value.writeToBuffer(),
          $1.CreateCategoryResponse.fromBuffer);
  static final _$updateCategory =
      $grpc.ClientMethod<$1.UpdateCategoryRequest, $1.UpdateCategoryResponse>(
          '/voice.space.v1.SpaceService/UpdateCategory',
          ($1.UpdateCategoryRequest value) => value.writeToBuffer(),
          $1.UpdateCategoryResponse.fromBuffer);
  static final _$deleteCategory =
      $grpc.ClientMethod<$1.DeleteCategoryRequest, $1.DeleteCategoryResponse>(
          '/voice.space.v1.SpaceService/DeleteCategory',
          ($1.DeleteCategoryRequest value) => value.writeToBuffer(),
          $1.DeleteCategoryResponse.fromBuffer);
  static final _$reorderSpaceTree = $grpc.ClientMethod<
          $1.ReorderSpaceTreeRequest, $1.ReorderSpaceTreeResponse>(
      '/voice.space.v1.SpaceService/ReorderSpaceTree',
      ($1.ReorderSpaceTreeRequest value) => value.writeToBuffer(),
      $1.ReorderSpaceTreeResponse.fromBuffer);
  static final _$pinTreeNode =
      $grpc.ClientMethod<$1.PinTreeNodeRequest, $1.PinTreeNodeResponse>(
          '/voice.space.v1.SpaceService/PinTreeNode',
          ($1.PinTreeNodeRequest value) => value.writeToBuffer(),
          $1.PinTreeNodeResponse.fromBuffer);
  static final _$unpinTreeNode =
      $grpc.ClientMethod<$1.UnpinTreeNodeRequest, $1.UnpinTreeNodeResponse>(
          '/voice.space.v1.SpaceService/UnpinTreeNode',
          ($1.UnpinTreeNodeRequest value) => value.writeToBuffer(),
          $1.UnpinTreeNodeResponse.fromBuffer);
  static final _$listSpaceTree =
      $grpc.ClientMethod<$1.ListSpaceTreeRequest, $1.ListSpaceTreeResponse>(
          '/voice.space.v1.SpaceService/ListSpaceTree',
          ($1.ListSpaceTreeRequest value) => value.writeToBuffer(),
          $1.ListSpaceTreeResponse.fromBuffer);
  static final _$createInvite =
      $grpc.ClientMethod<$1.CreateInviteRequest, $1.CreateInviteResponse>(
          '/voice.space.v1.SpaceService/CreateInvite',
          ($1.CreateInviteRequest value) => value.writeToBuffer(),
          $1.CreateInviteResponse.fromBuffer);
  static final _$revokeInvite =
      $grpc.ClientMethod<$1.RevokeInviteRequest, $1.RevokeInviteResponse>(
          '/voice.space.v1.SpaceService/RevokeInvite',
          ($1.RevokeInviteRequest value) => value.writeToBuffer(),
          $1.RevokeInviteResponse.fromBuffer);
  static final _$getInvite =
      $grpc.ClientMethod<$1.GetInviteRequest, $1.GetInviteResponse>(
          '/voice.space.v1.SpaceService/GetInvite',
          ($1.GetInviteRequest value) => value.writeToBuffer(),
          $1.GetInviteResponse.fromBuffer);
  static final _$listInvites =
      $grpc.ClientMethod<$1.ListInvitesRequest, $1.ListInvitesResponse>(
          '/voice.space.v1.SpaceService/ListInvites',
          ($1.ListInvitesRequest value) => value.writeToBuffer(),
          $1.ListInvitesResponse.fromBuffer);
  static final _$joinByInvite =
      $grpc.ClientMethod<$1.JoinByInviteRequest, $1.JoinByInviteResponse>(
          '/voice.space.v1.SpaceService/JoinByInvite',
          ($1.JoinByInviteRequest value) => value.writeToBuffer(),
          $1.JoinByInviteResponse.fromBuffer);
  static final _$joinSpace =
      $grpc.ClientMethod<$1.JoinSpaceRequest, $1.JoinSpaceResponse>(
          '/voice.space.v1.SpaceService/JoinSpace',
          ($1.JoinSpaceRequest value) => value.writeToBuffer(),
          $1.JoinSpaceResponse.fromBuffer);
  static final _$startJoinAttempt = $grpc.ClientMethod<
          $1.StartJoinAttemptRequest, $1.StartJoinAttemptResponse>(
      '/voice.space.v1.SpaceService/StartJoinAttempt',
      ($1.StartJoinAttemptRequest value) => value.writeToBuffer(),
      $1.StartJoinAttemptResponse.fromBuffer);
  static final _$getJoinAttempt =
      $grpc.ClientMethod<$1.GetJoinAttemptRequest, $1.GetJoinAttemptResponse>(
          '/voice.space.v1.SpaceService/GetJoinAttempt',
          ($1.GetJoinAttemptRequest value) => value.writeToBuffer(),
          $1.GetJoinAttemptResponse.fromBuffer);
  static final _$submitJoinAttempt = $grpc.ClientMethod<
          $1.SubmitJoinAttemptRequest, $1.SubmitJoinAttemptResponse>(
      '/voice.space.v1.SpaceService/SubmitJoinAttempt',
      ($1.SubmitJoinAttemptRequest value) => value.writeToBuffer(),
      $1.SubmitJoinAttemptResponse.fromBuffer);
  static final _$listJoinRequests = $grpc.ClientMethod<
          $1.ListJoinRequestsRequest, $1.ListJoinRequestsResponse>(
      '/voice.space.v1.SpaceService/ListJoinRequests',
      ($1.ListJoinRequestsRequest value) => value.writeToBuffer(),
      $1.ListJoinRequestsResponse.fromBuffer);
  static final _$getJoinRequest =
      $grpc.ClientMethod<$1.GetJoinRequestRequest, $1.GetJoinRequestResponse>(
          '/voice.space.v1.SpaceService/GetJoinRequest',
          ($1.GetJoinRequestRequest value) => value.writeToBuffer(),
          $1.GetJoinRequestResponse.fromBuffer);
  static final _$approveJoinRequest = $grpc.ClientMethod<
          $1.ApproveJoinRequestRequest, $1.ApproveJoinRequestResponse>(
      '/voice.space.v1.SpaceService/ApproveJoinRequest',
      ($1.ApproveJoinRequestRequest value) => value.writeToBuffer(),
      $1.ApproveJoinRequestResponse.fromBuffer);
  static final _$leaveSpace =
      $grpc.ClientMethod<$1.LeaveSpaceRequest, $1.LeaveSpaceResponse>(
          '/voice.space.v1.SpaceService/LeaveSpace',
          ($1.LeaveSpaceRequest value) => value.writeToBuffer(),
          $1.LeaveSpaceResponse.fromBuffer);
  static final _$kickMember =
      $grpc.ClientMethod<$1.KickMemberRequest, $1.KickMemberResponse>(
          '/voice.space.v1.SpaceService/KickMember',
          ($1.KickMemberRequest value) => value.writeToBuffer(),
          $1.KickMemberResponse.fromBuffer);
  static final _$banMember =
      $grpc.ClientMethod<$1.BanMemberRequest, $1.BanMemberResponse>(
          '/voice.space.v1.SpaceService/BanMember',
          ($1.BanMemberRequest value) => value.writeToBuffer(),
          $1.BanMemberResponse.fromBuffer);
  static final _$unbanMember =
      $grpc.ClientMethod<$1.UnbanMemberRequest, $1.UnbanMemberResponse>(
          '/voice.space.v1.SpaceService/UnbanMember',
          ($1.UnbanMemberRequest value) => value.writeToBuffer(),
          $1.UnbanMemberResponse.fromBuffer);
  static final _$listMembers =
      $grpc.ClientMethod<$1.ListMembersRequest, $1.ListMembersResponse>(
          '/voice.space.v1.SpaceService/ListMembers',
          ($1.ListMembersRequest value) => value.writeToBuffer(),
          $1.ListMembersResponse.fromBuffer);
  static final _$updateMemberNickname = $grpc.ClientMethod<
          $1.UpdateMemberNicknameRequest, $1.UpdateMemberNicknameResponse>(
      '/voice.space.v1.SpaceService/UpdateMemberNickname',
      ($1.UpdateMemberNicknameRequest value) => value.writeToBuffer(),
      $1.UpdateMemberNicknameResponse.fromBuffer);
  static final _$listBans =
      $grpc.ClientMethod<$1.ListBansRequest, $1.ListBansResponse>(
          '/voice.space.v1.SpaceService/ListBans',
          ($1.ListBansRequest value) => value.writeToBuffer(),
          $1.ListBansResponse.fromBuffer);
  static final _$timeoutMember =
      $grpc.ClientMethod<$1.TimeoutMemberRequest, $1.TimeoutMemberResponse>(
          '/voice.space.v1.SpaceService/TimeoutMember',
          ($1.TimeoutMemberRequest value) => value.writeToBuffer(),
          $1.TimeoutMemberResponse.fromBuffer);
  static final _$removeMemberTimeout = $grpc.ClientMethod<
          $1.RemoveMemberTimeoutRequest, $1.RemoveMemberTimeoutResponse>(
      '/voice.space.v1.SpaceService/RemoveMemberTimeout',
      ($1.RemoveMemberTimeoutRequest value) => value.writeToBuffer(),
      $1.RemoveMemberTimeoutResponse.fromBuffer);
  static final _$transferOwnership = $grpc.ClientMethod<
          $1.TransferOwnershipRequest, $1.TransferOwnershipResponse>(
      '/voice.space.v1.SpaceService/TransferOwnership',
      ($1.TransferOwnershipRequest value) => value.writeToBuffer(),
      $1.TransferOwnershipResponse.fromBuffer);
  static final _$addBotMember =
      $grpc.ClientMethod<$1.AddBotMemberRequest, $1.AddBotMemberResponse>(
          '/voice.space.v1.SpaceService/AddBotMember',
          ($1.AddBotMemberRequest value) => value.writeToBuffer(),
          $1.AddBotMemberResponse.fromBuffer);
  static final _$removeBotMember =
      $grpc.ClientMethod<$1.RemoveBotMemberRequest, $1.RemoveBotMemberResponse>(
          '/voice.space.v1.SpaceService/RemoveBotMember',
          ($1.RemoveBotMemberRequest value) => value.writeToBuffer(),
          $1.RemoveBotMemberResponse.fromBuffer);
  static final _$listTemplates =
      $grpc.ClientMethod<$1.ListTemplatesRequest, $1.ListTemplatesResponse>(
          '/voice.space.v1.SpaceService/ListTemplates',
          ($1.ListTemplatesRequest value) => value.writeToBuffer(),
          $1.ListTemplatesResponse.fromBuffer);
  static final _$createFromTemplate = $grpc.ClientMethod<
          $1.CreateFromTemplateRequest, $1.CreateFromTemplateResponse>(
      '/voice.space.v1.SpaceService/CreateFromTemplate',
      ($1.CreateFromTemplateRequest value) => value.writeToBuffer(),
      $1.CreateFromTemplateResponse.fromBuffer);
  static final _$applyCommunityRoster = $grpc.ClientMethod<
          $1.ApplyCommunityRosterRequest, $1.ApplyCommunityRosterResponse>(
      '/voice.space.v1.SpaceService/ApplyCommunityRoster',
      ($1.ApplyCommunityRosterRequest value) => value.writeToBuffer(),
      $1.ApplyCommunityRosterResponse.fromBuffer);
  static final _$createCommunityBootstrap = $grpc.ClientMethod<
          $1.CreateCommunityBootstrapRequest,
          $1.CreateCommunityBootstrapResponse>(
      '/voice.space.v1.SpaceService/CreateCommunityBootstrap',
      ($1.CreateCommunityBootstrapRequest value) => value.writeToBuffer(),
      $1.CreateCommunityBootstrapResponse.fromBuffer);
  static final _$recoverCommunityOwner = $grpc.ClientMethod<
          $1.RecoverCommunityOwnerRequest, $1.RecoverCommunityOwnerResponse>(
      '/voice.space.v1.SpaceService/RecoverCommunityOwner',
      ($1.RecoverCommunityOwnerRequest value) => value.writeToBuffer(),
      $1.RecoverCommunityOwnerResponse.fromBuffer);
  static final _$getAuditLog =
      $grpc.ClientMethod<$1.GetAuditLogRequest, $1.GetAuditLogResponse>(
          '/voice.space.v1.SpaceService/GetAuditLog',
          ($1.GetAuditLogRequest value) => value.writeToBuffer(),
          $1.GetAuditLogResponse.fromBuffer);
  static final _$appendAuditEvent = $grpc.ClientMethod<
          $1.AppendAuditEventRequest, $1.AppendAuditEventResponse>(
      '/voice.space.v1.SpaceService/AppendAuditEvent',
      ($1.AppendAuditEventRequest value) => value.writeToBuffer(),
      $1.AppendAuditEventResponse.fromBuffer);
  static final _$commitSpaceChatCreation = $grpc.ClientMethod<
          $1.CommitSpaceChatCreationRequest,
          $1.CommitSpaceChatCreationResponse>(
      '/voice.space.v1.SpaceService/CommitSpaceChatCreation',
      ($1.CommitSpaceChatCreationRequest value) => value.writeToBuffer(),
      $1.CommitSpaceChatCreationResponse.fromBuffer);
  static final _$getSpaceChatCreationOutcome = $grpc.ClientMethod<
          $1.GetSpaceChatCreationOutcomeRequest,
          $1.GetSpaceChatCreationOutcomeResponse>(
      '/voice.space.v1.SpaceService/GetSpaceChatCreationOutcome',
      ($1.GetSpaceChatCreationOutcomeRequest value) => value.writeToBuffer(),
      $1.GetSpaceChatCreationOutcomeResponse.fromBuffer);
  static final _$areCoMembers =
      $grpc.ClientMethod<$1.AreCoMembersRequest, $1.AreCoMembersResponse>(
          '/voice.space.v1.SpaceService/AreCoMembers',
          ($1.AreCoMembersRequest value) => value.writeToBuffer(),
          $1.AreCoMembersResponse.fromBuffer);
  static final _$resolveVoiceRoomAccess = $grpc.ClientMethod<
          $1.ResolveVoiceRoomAccessRequest, $1.ResolveVoiceRoomAccessResponse>(
      '/voice.space.v1.SpaceService/ResolveVoiceRoomAccess',
      ($1.ResolveVoiceRoomAccessRequest value) => value.writeToBuffer(),
      $1.ResolveVoiceRoomAccessResponse.fromBuffer);
  static final _$resolveMemberCommunicationState = $grpc.ClientMethod<
          $1.ResolveMemberCommunicationStateRequest,
          $1.ResolveMemberCommunicationStateResponse>(
      '/voice.space.v1.SpaceService/ResolveMemberCommunicationState',
      ($1.ResolveMemberCommunicationStateRequest value) =>
          value.writeToBuffer(),
      $1.ResolveMemberCommunicationStateResponse.fromBuffer);
  static final _$syncSpaceProSubscription = $grpc.ClientMethod<
          $1.SyncSpaceProSubscriptionRequest,
          $1.SyncSpaceProSubscriptionResponse>(
      '/voice.space.v1.SpaceService/SyncSpaceProSubscription',
      ($1.SyncSpaceProSubscriptionRequest value) => value.writeToBuffer(),
      $1.SyncSpaceProSubscriptionResponse.fromBuffer);
  static final _$restoreSpace =
      $grpc.ClientMethod<$1.RestoreSpaceRequest, $1.RestoreSpaceResponse>(
          '/voice.space.v1.SpaceService/RestoreSpace',
          ($1.RestoreSpaceRequest value) => value.writeToBuffer(),
          $1.RestoreSpaceResponse.fromBuffer);
  static final _$getSpaceDeletionCoordinatorStatus = $grpc.ClientMethod<
          $1.GetSpaceDeletionCoordinatorStatusRequest,
          $1.GetSpaceDeletionCoordinatorStatusResponse>(
      '/voice.space.v1.SpaceService/GetSpaceDeletionCoordinatorStatus',
      ($1.GetSpaceDeletionCoordinatorStatusRequest value) =>
          value.writeToBuffer(),
      $1.GetSpaceDeletionCoordinatorStatusResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.space.v1.SpaceService')
abstract class SpaceServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.space.v1.SpaceService';

  SpaceServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.GetMemberAdmissionAuthorityRequest,
            $0.GetMemberAdmissionAuthorityResponse>(
        'GetMemberAdmissionAuthority',
        getMemberAdmissionAuthority_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetMemberAdmissionAuthorityRequest.fromBuffer(value),
        ($0.GetMemberAdmissionAuthorityResponse value) =>
            value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.CreateSpaceRequest, $1.CreateSpaceResponse>(
            'CreateSpace',
            createSpace_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.CreateSpaceRequest.fromBuffer(value),
            ($1.CreateSpaceResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.UpdateSpaceRequest, $1.UpdateSpaceResponse>(
            'UpdateSpace',
            updateSpace_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.UpdateSpaceRequest.fromBuffer(value),
            ($1.UpdateSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.UpdateSpaceMmConfigRequest,
            $1.UpdateSpaceMmConfigResponse>(
        'UpdateSpaceMmConfig',
        updateSpaceMmConfig_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.UpdateSpaceMmConfigRequest.fromBuffer(value),
        ($1.UpdateSpaceMmConfigResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.DeleteSpaceRequest, $1.DeleteSpaceResponse>(
            'DeleteSpace',
            deleteSpace_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.DeleteSpaceRequest.fromBuffer(value),
            ($1.DeleteSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetSpaceRequest, $1.GetSpaceResponse>(
        'GetSpace',
        getSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.GetSpaceRequest.fromBuffer(value),
        ($1.GetSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ValidateRoleOverrideTargetRequest,
            $1.ValidateRoleOverrideTargetResponse>(
        'ValidateRoleOverrideTarget',
        validateRoleOverrideTarget_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ValidateRoleOverrideTargetRequest.fromBuffer(value),
        ($1.ValidateRoleOverrideTargetResponse value) =>
            value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.ListMySpacesRequest, $1.ListMySpacesResponse>(
            'ListMySpaces',
            listMySpaces_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.ListMySpacesRequest.fromBuffer(value),
            ($1.ListMySpacesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.SearchPublicSpacesRequest,
            $1.SearchPublicSpacesResponse>(
        'SearchPublicSpaces',
        searchPublicSpaces_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.SearchPublicSpacesRequest.fromBuffer(value),
        ($1.SearchPublicSpacesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.CreateVoiceRoomRequest,
            $1.CreateVoiceRoomResponse>(
        'CreateVoiceRoom',
        createVoiceRoom_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.CreateVoiceRoomRequest.fromBuffer(value),
        ($1.CreateVoiceRoomResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.UpdateVoiceRoomRequest,
            $1.UpdateVoiceRoomResponse>(
        'UpdateVoiceRoom',
        updateVoiceRoom_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.UpdateVoiceRoomRequest.fromBuffer(value),
        ($1.UpdateVoiceRoomResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.DeleteVoiceRoomRequest,
            $1.DeleteVoiceRoomResponse>(
        'DeleteVoiceRoom',
        deleteVoiceRoom_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.DeleteVoiceRoomRequest.fromBuffer(value),
        ($1.DeleteVoiceRoomResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.UpsertTreeNodeRequest,
            $1.UpsertTreeNodeResponse>(
        'UpsertTreeNode',
        upsertTreeNode_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.UpsertTreeNodeRequest.fromBuffer(value),
        ($1.UpsertTreeNodeResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.RemoveTreeNodeRequest,
            $1.RemoveTreeNodeResponse>(
        'RemoveTreeNode',
        removeTreeNode_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.RemoveTreeNodeRequest.fromBuffer(value),
        ($1.RemoveTreeNodeResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.CreateCategoryRequest,
            $1.CreateCategoryResponse>(
        'CreateCategory',
        createCategory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.CreateCategoryRequest.fromBuffer(value),
        ($1.CreateCategoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.UpdateCategoryRequest,
            $1.UpdateCategoryResponse>(
        'UpdateCategory',
        updateCategory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.UpdateCategoryRequest.fromBuffer(value),
        ($1.UpdateCategoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.DeleteCategoryRequest,
            $1.DeleteCategoryResponse>(
        'DeleteCategory',
        deleteCategory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.DeleteCategoryRequest.fromBuffer(value),
        ($1.DeleteCategoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ReorderSpaceTreeRequest,
            $1.ReorderSpaceTreeResponse>(
        'ReorderSpaceTree',
        reorderSpaceTree_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ReorderSpaceTreeRequest.fromBuffer(value),
        ($1.ReorderSpaceTreeResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.PinTreeNodeRequest, $1.PinTreeNodeResponse>(
            'PinTreeNode',
            pinTreeNode_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.PinTreeNodeRequest.fromBuffer(value),
            ($1.PinTreeNodeResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.UnpinTreeNodeRequest, $1.UnpinTreeNodeResponse>(
            'UnpinTreeNode',
            unpinTreeNode_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.UnpinTreeNodeRequest.fromBuffer(value),
            ($1.UnpinTreeNodeResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.ListSpaceTreeRequest, $1.ListSpaceTreeResponse>(
            'ListSpaceTree',
            listSpaceTree_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.ListSpaceTreeRequest.fromBuffer(value),
            ($1.ListSpaceTreeResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.CreateInviteRequest, $1.CreateInviteResponse>(
            'CreateInvite',
            createInvite_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.CreateInviteRequest.fromBuffer(value),
            ($1.CreateInviteResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.RevokeInviteRequest, $1.RevokeInviteResponse>(
            'RevokeInvite',
            revokeInvite_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.RevokeInviteRequest.fromBuffer(value),
            ($1.RevokeInviteResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetInviteRequest, $1.GetInviteResponse>(
        'GetInvite',
        getInvite_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.GetInviteRequest.fromBuffer(value),
        ($1.GetInviteResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.ListInvitesRequest, $1.ListInvitesResponse>(
            'ListInvites',
            listInvites_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.ListInvitesRequest.fromBuffer(value),
            ($1.ListInvitesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.JoinByInviteRequest, $1.JoinByInviteResponse>(
            'JoinByInvite',
            joinByInvite_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.JoinByInviteRequest.fromBuffer(value),
            ($1.JoinByInviteResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.JoinSpaceRequest, $1.JoinSpaceResponse>(
        'JoinSpace',
        joinSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.JoinSpaceRequest.fromBuffer(value),
        ($1.JoinSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.StartJoinAttemptRequest,
            $1.StartJoinAttemptResponse>(
        'StartJoinAttempt',
        startJoinAttempt_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.StartJoinAttemptRequest.fromBuffer(value),
        ($1.StartJoinAttemptResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetJoinAttemptRequest,
            $1.GetJoinAttemptResponse>(
        'GetJoinAttempt',
        getJoinAttempt_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetJoinAttemptRequest.fromBuffer(value),
        ($1.GetJoinAttemptResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.SubmitJoinAttemptRequest,
            $1.SubmitJoinAttemptResponse>(
        'SubmitJoinAttempt',
        submitJoinAttempt_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.SubmitJoinAttemptRequest.fromBuffer(value),
        ($1.SubmitJoinAttemptResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ListJoinRequestsRequest,
            $1.ListJoinRequestsResponse>(
        'ListJoinRequests',
        listJoinRequests_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ListJoinRequestsRequest.fromBuffer(value),
        ($1.ListJoinRequestsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetJoinRequestRequest,
            $1.GetJoinRequestResponse>(
        'GetJoinRequest',
        getJoinRequest_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetJoinRequestRequest.fromBuffer(value),
        ($1.GetJoinRequestResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ApproveJoinRequestRequest,
            $1.ApproveJoinRequestResponse>(
        'ApproveJoinRequest',
        approveJoinRequest_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ApproveJoinRequestRequest.fromBuffer(value),
        ($1.ApproveJoinRequestResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.LeaveSpaceRequest, $1.LeaveSpaceResponse>(
        'LeaveSpace',
        leaveSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.LeaveSpaceRequest.fromBuffer(value),
        ($1.LeaveSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.KickMemberRequest, $1.KickMemberResponse>(
        'KickMember',
        kickMember_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.KickMemberRequest.fromBuffer(value),
        ($1.KickMemberResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.BanMemberRequest, $1.BanMemberResponse>(
        'BanMember',
        banMember_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.BanMemberRequest.fromBuffer(value),
        ($1.BanMemberResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.UnbanMemberRequest, $1.UnbanMemberResponse>(
            'UnbanMember',
            unbanMember_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.UnbanMemberRequest.fromBuffer(value),
            ($1.UnbanMemberResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.ListMembersRequest, $1.ListMembersResponse>(
            'ListMembers',
            listMembers_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.ListMembersRequest.fromBuffer(value),
            ($1.ListMembersResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.UpdateMemberNicknameRequest,
            $1.UpdateMemberNicknameResponse>(
        'UpdateMemberNickname',
        updateMemberNickname_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.UpdateMemberNicknameRequest.fromBuffer(value),
        ($1.UpdateMemberNicknameResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ListBansRequest, $1.ListBansResponse>(
        'ListBans',
        listBans_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.ListBansRequest.fromBuffer(value),
        ($1.ListBansResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.TimeoutMemberRequest, $1.TimeoutMemberResponse>(
            'TimeoutMember',
            timeoutMember_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.TimeoutMemberRequest.fromBuffer(value),
            ($1.TimeoutMemberResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.RemoveMemberTimeoutRequest,
            $1.RemoveMemberTimeoutResponse>(
        'RemoveMemberTimeout',
        removeMemberTimeout_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.RemoveMemberTimeoutRequest.fromBuffer(value),
        ($1.RemoveMemberTimeoutResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.TransferOwnershipRequest,
            $1.TransferOwnershipResponse>(
        'TransferOwnership',
        transferOwnership_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.TransferOwnershipRequest.fromBuffer(value),
        ($1.TransferOwnershipResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.AddBotMemberRequest, $1.AddBotMemberResponse>(
            'AddBotMember',
            addBotMember_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.AddBotMemberRequest.fromBuffer(value),
            ($1.AddBotMemberResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.RemoveBotMemberRequest,
            $1.RemoveBotMemberResponse>(
        'RemoveBotMember',
        removeBotMember_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.RemoveBotMemberRequest.fromBuffer(value),
        ($1.RemoveBotMemberResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.ListTemplatesRequest, $1.ListTemplatesResponse>(
            'ListTemplates',
            listTemplates_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.ListTemplatesRequest.fromBuffer(value),
            ($1.ListTemplatesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.CreateFromTemplateRequest,
            $1.CreateFromTemplateResponse>(
        'CreateFromTemplate',
        createFromTemplate_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.CreateFromTemplateRequest.fromBuffer(value),
        ($1.CreateFromTemplateResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ApplyCommunityRosterRequest,
            $1.ApplyCommunityRosterResponse>(
        'ApplyCommunityRoster',
        applyCommunityRoster_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ApplyCommunityRosterRequest.fromBuffer(value),
        ($1.ApplyCommunityRosterResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.CreateCommunityBootstrapRequest,
            $1.CreateCommunityBootstrapResponse>(
        'CreateCommunityBootstrap',
        createCommunityBootstrap_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.CreateCommunityBootstrapRequest.fromBuffer(value),
        ($1.CreateCommunityBootstrapResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.RecoverCommunityOwnerRequest,
            $1.RecoverCommunityOwnerResponse>(
        'RecoverCommunityOwner',
        recoverCommunityOwner_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.RecoverCommunityOwnerRequest.fromBuffer(value),
        ($1.RecoverCommunityOwnerResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.GetAuditLogRequest, $1.GetAuditLogResponse>(
            'GetAuditLog',
            getAuditLog_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.GetAuditLogRequest.fromBuffer(value),
            ($1.GetAuditLogResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.AppendAuditEventRequest,
            $1.AppendAuditEventResponse>(
        'AppendAuditEvent',
        appendAuditEvent_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.AppendAuditEventRequest.fromBuffer(value),
        ($1.AppendAuditEventResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.CommitSpaceChatCreationRequest,
            $1.CommitSpaceChatCreationResponse>(
        'CommitSpaceChatCreation',
        commitSpaceChatCreation_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.CommitSpaceChatCreationRequest.fromBuffer(value),
        ($1.CommitSpaceChatCreationResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetSpaceChatCreationOutcomeRequest,
            $1.GetSpaceChatCreationOutcomeResponse>(
        'GetSpaceChatCreationOutcome',
        getSpaceChatCreationOutcome_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetSpaceChatCreationOutcomeRequest.fromBuffer(value),
        ($1.GetSpaceChatCreationOutcomeResponse value) =>
            value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.AreCoMembersRequest, $1.AreCoMembersResponse>(
            'AreCoMembers',
            areCoMembers_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.AreCoMembersRequest.fromBuffer(value),
            ($1.AreCoMembersResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ResolveVoiceRoomAccessRequest,
            $1.ResolveVoiceRoomAccessResponse>(
        'ResolveVoiceRoomAccess',
        resolveVoiceRoomAccess_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ResolveVoiceRoomAccessRequest.fromBuffer(value),
        ($1.ResolveVoiceRoomAccessResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ResolveMemberCommunicationStateRequest,
            $1.ResolveMemberCommunicationStateResponse>(
        'ResolveMemberCommunicationState',
        resolveMemberCommunicationState_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ResolveMemberCommunicationStateRequest.fromBuffer(value),
        ($1.ResolveMemberCommunicationStateResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.SyncSpaceProSubscriptionRequest,
            $1.SyncSpaceProSubscriptionResponse>(
        'SyncSpaceProSubscription',
        syncSpaceProSubscription_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.SyncSpaceProSubscriptionRequest.fromBuffer(value),
        ($1.SyncSpaceProSubscriptionResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.RestoreSpaceRequest, $1.RestoreSpaceResponse>(
            'RestoreSpace',
            restoreSpace_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.RestoreSpaceRequest.fromBuffer(value),
            ($1.RestoreSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetSpaceDeletionCoordinatorStatusRequest,
            $1.GetSpaceDeletionCoordinatorStatusResponse>(
        'GetSpaceDeletionCoordinatorStatus',
        getSpaceDeletionCoordinatorStatus_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetSpaceDeletionCoordinatorStatusRequest.fromBuffer(value),
        ($1.GetSpaceDeletionCoordinatorStatusResponse value) =>
            value.writeToBuffer()));
  }

  $async.Future<$0.GetMemberAdmissionAuthorityResponse>
      getMemberAdmissionAuthority_Pre($grpc.ServiceCall $call,
          $async.Future<$0.GetMemberAdmissionAuthorityRequest> $request) async {
    return getMemberAdmissionAuthority($call, await $request);
  }

  $async.Future<$0.GetMemberAdmissionAuthorityResponse>
      getMemberAdmissionAuthority($grpc.ServiceCall call,
          $0.GetMemberAdmissionAuthorityRequest request);

  $async.Future<$1.CreateSpaceResponse> createSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$1.CreateSpaceRequest> $request) async {
    return createSpace($call, await $request);
  }

  $async.Future<$1.CreateSpaceResponse> createSpace(
      $grpc.ServiceCall call, $1.CreateSpaceRequest request);

  $async.Future<$1.UpdateSpaceResponse> updateSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$1.UpdateSpaceRequest> $request) async {
    return updateSpace($call, await $request);
  }

  $async.Future<$1.UpdateSpaceResponse> updateSpace(
      $grpc.ServiceCall call, $1.UpdateSpaceRequest request);

  $async.Future<$1.UpdateSpaceMmConfigResponse> updateSpaceMmConfig_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UpdateSpaceMmConfigRequest> $request) async {
    return updateSpaceMmConfig($call, await $request);
  }

  $async.Future<$1.UpdateSpaceMmConfigResponse> updateSpaceMmConfig(
      $grpc.ServiceCall call, $1.UpdateSpaceMmConfigRequest request);

  $async.Future<$1.DeleteSpaceResponse> deleteSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$1.DeleteSpaceRequest> $request) async {
    return deleteSpace($call, await $request);
  }

  $async.Future<$1.DeleteSpaceResponse> deleteSpace(
      $grpc.ServiceCall call, $1.DeleteSpaceRequest request);

  $async.Future<$1.GetSpaceResponse> getSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$1.GetSpaceRequest> $request) async {
    return getSpace($call, await $request);
  }

  $async.Future<$1.GetSpaceResponse> getSpace(
      $grpc.ServiceCall call, $1.GetSpaceRequest request);

  $async.Future<$1.ValidateRoleOverrideTargetResponse>
      validateRoleOverrideTarget_Pre($grpc.ServiceCall $call,
          $async.Future<$1.ValidateRoleOverrideTargetRequest> $request) async {
    return validateRoleOverrideTarget($call, await $request);
  }

  $async.Future<$1.ValidateRoleOverrideTargetResponse>
      validateRoleOverrideTarget(
          $grpc.ServiceCall call, $1.ValidateRoleOverrideTargetRequest request);

  $async.Future<$1.ListMySpacesResponse> listMySpaces_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ListMySpacesRequest> $request) async {
    return listMySpaces($call, await $request);
  }

  $async.Future<$1.ListMySpacesResponse> listMySpaces(
      $grpc.ServiceCall call, $1.ListMySpacesRequest request);

  $async.Future<$1.SearchPublicSpacesResponse> searchPublicSpaces_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.SearchPublicSpacesRequest> $request) async {
    return searchPublicSpaces($call, await $request);
  }

  $async.Future<$1.SearchPublicSpacesResponse> searchPublicSpaces(
      $grpc.ServiceCall call, $1.SearchPublicSpacesRequest request);

  $async.Future<$1.CreateVoiceRoomResponse> createVoiceRoom_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.CreateVoiceRoomRequest> $request) async {
    return createVoiceRoom($call, await $request);
  }

  $async.Future<$1.CreateVoiceRoomResponse> createVoiceRoom(
      $grpc.ServiceCall call, $1.CreateVoiceRoomRequest request);

  $async.Future<$1.UpdateVoiceRoomResponse> updateVoiceRoom_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UpdateVoiceRoomRequest> $request) async {
    return updateVoiceRoom($call, await $request);
  }

  $async.Future<$1.UpdateVoiceRoomResponse> updateVoiceRoom(
      $grpc.ServiceCall call, $1.UpdateVoiceRoomRequest request);

  $async.Future<$1.DeleteVoiceRoomResponse> deleteVoiceRoom_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.DeleteVoiceRoomRequest> $request) async {
    return deleteVoiceRoom($call, await $request);
  }

  $async.Future<$1.DeleteVoiceRoomResponse> deleteVoiceRoom(
      $grpc.ServiceCall call, $1.DeleteVoiceRoomRequest request);

  $async.Future<$1.UpsertTreeNodeResponse> upsertTreeNode_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UpsertTreeNodeRequest> $request) async {
    return upsertTreeNode($call, await $request);
  }

  $async.Future<$1.UpsertTreeNodeResponse> upsertTreeNode(
      $grpc.ServiceCall call, $1.UpsertTreeNodeRequest request);

  $async.Future<$1.RemoveTreeNodeResponse> removeTreeNode_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.RemoveTreeNodeRequest> $request) async {
    return removeTreeNode($call, await $request);
  }

  $async.Future<$1.RemoveTreeNodeResponse> removeTreeNode(
      $grpc.ServiceCall call, $1.RemoveTreeNodeRequest request);

  $async.Future<$1.CreateCategoryResponse> createCategory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.CreateCategoryRequest> $request) async {
    return createCategory($call, await $request);
  }

  $async.Future<$1.CreateCategoryResponse> createCategory(
      $grpc.ServiceCall call, $1.CreateCategoryRequest request);

  $async.Future<$1.UpdateCategoryResponse> updateCategory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UpdateCategoryRequest> $request) async {
    return updateCategory($call, await $request);
  }

  $async.Future<$1.UpdateCategoryResponse> updateCategory(
      $grpc.ServiceCall call, $1.UpdateCategoryRequest request);

  $async.Future<$1.DeleteCategoryResponse> deleteCategory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.DeleteCategoryRequest> $request) async {
    return deleteCategory($call, await $request);
  }

  $async.Future<$1.DeleteCategoryResponse> deleteCategory(
      $grpc.ServiceCall call, $1.DeleteCategoryRequest request);

  $async.Future<$1.ReorderSpaceTreeResponse> reorderSpaceTree_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ReorderSpaceTreeRequest> $request) async {
    return reorderSpaceTree($call, await $request);
  }

  $async.Future<$1.ReorderSpaceTreeResponse> reorderSpaceTree(
      $grpc.ServiceCall call, $1.ReorderSpaceTreeRequest request);

  $async.Future<$1.PinTreeNodeResponse> pinTreeNode_Pre($grpc.ServiceCall $call,
      $async.Future<$1.PinTreeNodeRequest> $request) async {
    return pinTreeNode($call, await $request);
  }

  $async.Future<$1.PinTreeNodeResponse> pinTreeNode(
      $grpc.ServiceCall call, $1.PinTreeNodeRequest request);

  $async.Future<$1.UnpinTreeNodeResponse> unpinTreeNode_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UnpinTreeNodeRequest> $request) async {
    return unpinTreeNode($call, await $request);
  }

  $async.Future<$1.UnpinTreeNodeResponse> unpinTreeNode(
      $grpc.ServiceCall call, $1.UnpinTreeNodeRequest request);

  $async.Future<$1.ListSpaceTreeResponse> listSpaceTree_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ListSpaceTreeRequest> $request) async {
    return listSpaceTree($call, await $request);
  }

  $async.Future<$1.ListSpaceTreeResponse> listSpaceTree(
      $grpc.ServiceCall call, $1.ListSpaceTreeRequest request);

  $async.Future<$1.CreateInviteResponse> createInvite_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.CreateInviteRequest> $request) async {
    return createInvite($call, await $request);
  }

  $async.Future<$1.CreateInviteResponse> createInvite(
      $grpc.ServiceCall call, $1.CreateInviteRequest request);

  $async.Future<$1.RevokeInviteResponse> revokeInvite_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.RevokeInviteRequest> $request) async {
    return revokeInvite($call, await $request);
  }

  $async.Future<$1.RevokeInviteResponse> revokeInvite(
      $grpc.ServiceCall call, $1.RevokeInviteRequest request);

  $async.Future<$1.GetInviteResponse> getInvite_Pre($grpc.ServiceCall $call,
      $async.Future<$1.GetInviteRequest> $request) async {
    return getInvite($call, await $request);
  }

  $async.Future<$1.GetInviteResponse> getInvite(
      $grpc.ServiceCall call, $1.GetInviteRequest request);

  $async.Future<$1.ListInvitesResponse> listInvites_Pre($grpc.ServiceCall $call,
      $async.Future<$1.ListInvitesRequest> $request) async {
    return listInvites($call, await $request);
  }

  $async.Future<$1.ListInvitesResponse> listInvites(
      $grpc.ServiceCall call, $1.ListInvitesRequest request);

  $async.Future<$1.JoinByInviteResponse> joinByInvite_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.JoinByInviteRequest> $request) async {
    return joinByInvite($call, await $request);
  }

  $async.Future<$1.JoinByInviteResponse> joinByInvite(
      $grpc.ServiceCall call, $1.JoinByInviteRequest request);

  $async.Future<$1.JoinSpaceResponse> joinSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$1.JoinSpaceRequest> $request) async {
    return joinSpace($call, await $request);
  }

  $async.Future<$1.JoinSpaceResponse> joinSpace(
      $grpc.ServiceCall call, $1.JoinSpaceRequest request);

  $async.Future<$1.StartJoinAttemptResponse> startJoinAttempt_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.StartJoinAttemptRequest> $request) async {
    return startJoinAttempt($call, await $request);
  }

  $async.Future<$1.StartJoinAttemptResponse> startJoinAttempt(
      $grpc.ServiceCall call, $1.StartJoinAttemptRequest request);

  $async.Future<$1.GetJoinAttemptResponse> getJoinAttempt_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetJoinAttemptRequest> $request) async {
    return getJoinAttempt($call, await $request);
  }

  $async.Future<$1.GetJoinAttemptResponse> getJoinAttempt(
      $grpc.ServiceCall call, $1.GetJoinAttemptRequest request);

  $async.Future<$1.SubmitJoinAttemptResponse> submitJoinAttempt_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.SubmitJoinAttemptRequest> $request) async {
    return submitJoinAttempt($call, await $request);
  }

  $async.Future<$1.SubmitJoinAttemptResponse> submitJoinAttempt(
      $grpc.ServiceCall call, $1.SubmitJoinAttemptRequest request);

  $async.Future<$1.ListJoinRequestsResponse> listJoinRequests_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ListJoinRequestsRequest> $request) async {
    return listJoinRequests($call, await $request);
  }

  $async.Future<$1.ListJoinRequestsResponse> listJoinRequests(
      $grpc.ServiceCall call, $1.ListJoinRequestsRequest request);

  $async.Future<$1.GetJoinRequestResponse> getJoinRequest_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetJoinRequestRequest> $request) async {
    return getJoinRequest($call, await $request);
  }

  $async.Future<$1.GetJoinRequestResponse> getJoinRequest(
      $grpc.ServiceCall call, $1.GetJoinRequestRequest request);

  $async.Future<$1.ApproveJoinRequestResponse> approveJoinRequest_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ApproveJoinRequestRequest> $request) async {
    return approveJoinRequest($call, await $request);
  }

  $async.Future<$1.ApproveJoinRequestResponse> approveJoinRequest(
      $grpc.ServiceCall call, $1.ApproveJoinRequestRequest request);

  $async.Future<$1.LeaveSpaceResponse> leaveSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$1.LeaveSpaceRequest> $request) async {
    return leaveSpace($call, await $request);
  }

  $async.Future<$1.LeaveSpaceResponse> leaveSpace(
      $grpc.ServiceCall call, $1.LeaveSpaceRequest request);

  $async.Future<$1.KickMemberResponse> kickMember_Pre($grpc.ServiceCall $call,
      $async.Future<$1.KickMemberRequest> $request) async {
    return kickMember($call, await $request);
  }

  $async.Future<$1.KickMemberResponse> kickMember(
      $grpc.ServiceCall call, $1.KickMemberRequest request);

  $async.Future<$1.BanMemberResponse> banMember_Pre($grpc.ServiceCall $call,
      $async.Future<$1.BanMemberRequest> $request) async {
    return banMember($call, await $request);
  }

  $async.Future<$1.BanMemberResponse> banMember(
      $grpc.ServiceCall call, $1.BanMemberRequest request);

  $async.Future<$1.UnbanMemberResponse> unbanMember_Pre($grpc.ServiceCall $call,
      $async.Future<$1.UnbanMemberRequest> $request) async {
    return unbanMember($call, await $request);
  }

  $async.Future<$1.UnbanMemberResponse> unbanMember(
      $grpc.ServiceCall call, $1.UnbanMemberRequest request);

  $async.Future<$1.ListMembersResponse> listMembers_Pre($grpc.ServiceCall $call,
      $async.Future<$1.ListMembersRequest> $request) async {
    return listMembers($call, await $request);
  }

  $async.Future<$1.ListMembersResponse> listMembers(
      $grpc.ServiceCall call, $1.ListMembersRequest request);

  $async.Future<$1.UpdateMemberNicknameResponse> updateMemberNickname_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UpdateMemberNicknameRequest> $request) async {
    return updateMemberNickname($call, await $request);
  }

  $async.Future<$1.UpdateMemberNicknameResponse> updateMemberNickname(
      $grpc.ServiceCall call, $1.UpdateMemberNicknameRequest request);

  $async.Future<$1.ListBansResponse> listBans_Pre($grpc.ServiceCall $call,
      $async.Future<$1.ListBansRequest> $request) async {
    return listBans($call, await $request);
  }

  $async.Future<$1.ListBansResponse> listBans(
      $grpc.ServiceCall call, $1.ListBansRequest request);

  $async.Future<$1.TimeoutMemberResponse> timeoutMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.TimeoutMemberRequest> $request) async {
    return timeoutMember($call, await $request);
  }

  $async.Future<$1.TimeoutMemberResponse> timeoutMember(
      $grpc.ServiceCall call, $1.TimeoutMemberRequest request);

  $async.Future<$1.RemoveMemberTimeoutResponse> removeMemberTimeout_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.RemoveMemberTimeoutRequest> $request) async {
    return removeMemberTimeout($call, await $request);
  }

  $async.Future<$1.RemoveMemberTimeoutResponse> removeMemberTimeout(
      $grpc.ServiceCall call, $1.RemoveMemberTimeoutRequest request);

  $async.Future<$1.TransferOwnershipResponse> transferOwnership_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.TransferOwnershipRequest> $request) async {
    return transferOwnership($call, await $request);
  }

  $async.Future<$1.TransferOwnershipResponse> transferOwnership(
      $grpc.ServiceCall call, $1.TransferOwnershipRequest request);

  $async.Future<$1.AddBotMemberResponse> addBotMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.AddBotMemberRequest> $request) async {
    return addBotMember($call, await $request);
  }

  $async.Future<$1.AddBotMemberResponse> addBotMember(
      $grpc.ServiceCall call, $1.AddBotMemberRequest request);

  $async.Future<$1.RemoveBotMemberResponse> removeBotMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.RemoveBotMemberRequest> $request) async {
    return removeBotMember($call, await $request);
  }

  $async.Future<$1.RemoveBotMemberResponse> removeBotMember(
      $grpc.ServiceCall call, $1.RemoveBotMemberRequest request);

  $async.Future<$1.ListTemplatesResponse> listTemplates_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ListTemplatesRequest> $request) async {
    return listTemplates($call, await $request);
  }

  $async.Future<$1.ListTemplatesResponse> listTemplates(
      $grpc.ServiceCall call, $1.ListTemplatesRequest request);

  $async.Future<$1.CreateFromTemplateResponse> createFromTemplate_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.CreateFromTemplateRequest> $request) async {
    return createFromTemplate($call, await $request);
  }

  $async.Future<$1.CreateFromTemplateResponse> createFromTemplate(
      $grpc.ServiceCall call, $1.CreateFromTemplateRequest request);

  $async.Future<$1.ApplyCommunityRosterResponse> applyCommunityRoster_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ApplyCommunityRosterRequest> $request) async {
    return applyCommunityRoster($call, await $request);
  }

  $async.Future<$1.ApplyCommunityRosterResponse> applyCommunityRoster(
      $grpc.ServiceCall call, $1.ApplyCommunityRosterRequest request);

  $async.Future<$1.CreateCommunityBootstrapResponse>
      createCommunityBootstrap_Pre($grpc.ServiceCall $call,
          $async.Future<$1.CreateCommunityBootstrapRequest> $request) async {
    return createCommunityBootstrap($call, await $request);
  }

  $async.Future<$1.CreateCommunityBootstrapResponse> createCommunityBootstrap(
      $grpc.ServiceCall call, $1.CreateCommunityBootstrapRequest request);

  $async.Future<$1.RecoverCommunityOwnerResponse> recoverCommunityOwner_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.RecoverCommunityOwnerRequest> $request) async {
    return recoverCommunityOwner($call, await $request);
  }

  $async.Future<$1.RecoverCommunityOwnerResponse> recoverCommunityOwner(
      $grpc.ServiceCall call, $1.RecoverCommunityOwnerRequest request);

  $async.Future<$1.GetAuditLogResponse> getAuditLog_Pre($grpc.ServiceCall $call,
      $async.Future<$1.GetAuditLogRequest> $request) async {
    return getAuditLog($call, await $request);
  }

  $async.Future<$1.GetAuditLogResponse> getAuditLog(
      $grpc.ServiceCall call, $1.GetAuditLogRequest request);

  $async.Future<$1.AppendAuditEventResponse> appendAuditEvent_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.AppendAuditEventRequest> $request) async {
    return appendAuditEvent($call, await $request);
  }

  $async.Future<$1.AppendAuditEventResponse> appendAuditEvent(
      $grpc.ServiceCall call, $1.AppendAuditEventRequest request);

  $async.Future<$1.CommitSpaceChatCreationResponse> commitSpaceChatCreation_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.CommitSpaceChatCreationRequest> $request) async {
    return commitSpaceChatCreation($call, await $request);
  }

  $async.Future<$1.CommitSpaceChatCreationResponse> commitSpaceChatCreation(
      $grpc.ServiceCall call, $1.CommitSpaceChatCreationRequest request);

  $async.Future<$1.GetSpaceChatCreationOutcomeResponse>
      getSpaceChatCreationOutcome_Pre($grpc.ServiceCall $call,
          $async.Future<$1.GetSpaceChatCreationOutcomeRequest> $request) async {
    return getSpaceChatCreationOutcome($call, await $request);
  }

  $async.Future<$1.GetSpaceChatCreationOutcomeResponse>
      getSpaceChatCreationOutcome($grpc.ServiceCall call,
          $1.GetSpaceChatCreationOutcomeRequest request);

  $async.Future<$1.AreCoMembersResponse> areCoMembers_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.AreCoMembersRequest> $request) async {
    return areCoMembers($call, await $request);
  }

  $async.Future<$1.AreCoMembersResponse> areCoMembers(
      $grpc.ServiceCall call, $1.AreCoMembersRequest request);

  $async.Future<$1.ResolveVoiceRoomAccessResponse> resolveVoiceRoomAccess_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ResolveVoiceRoomAccessRequest> $request) async {
    return resolveVoiceRoomAccess($call, await $request);
  }

  $async.Future<$1.ResolveVoiceRoomAccessResponse> resolveVoiceRoomAccess(
      $grpc.ServiceCall call, $1.ResolveVoiceRoomAccessRequest request);

  $async.Future<$1.ResolveMemberCommunicationStateResponse>
      resolveMemberCommunicationState_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$1.ResolveMemberCommunicationStateRequest>
              $request) async {
    return resolveMemberCommunicationState($call, await $request);
  }

  $async.Future<$1.ResolveMemberCommunicationStateResponse>
      resolveMemberCommunicationState($grpc.ServiceCall call,
          $1.ResolveMemberCommunicationStateRequest request);

  $async.Future<$1.SyncSpaceProSubscriptionResponse>
      syncSpaceProSubscription_Pre($grpc.ServiceCall $call,
          $async.Future<$1.SyncSpaceProSubscriptionRequest> $request) async {
    return syncSpaceProSubscription($call, await $request);
  }

  $async.Future<$1.SyncSpaceProSubscriptionResponse> syncSpaceProSubscription(
      $grpc.ServiceCall call, $1.SyncSpaceProSubscriptionRequest request);

  $async.Future<$1.RestoreSpaceResponse> restoreSpace_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.RestoreSpaceRequest> $request) async {
    return restoreSpace($call, await $request);
  }

  $async.Future<$1.RestoreSpaceResponse> restoreSpace(
      $grpc.ServiceCall call, $1.RestoreSpaceRequest request);

  $async.Future<$1.GetSpaceDeletionCoordinatorStatusResponse>
      getSpaceDeletionCoordinatorStatus_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$1.GetSpaceDeletionCoordinatorStatusRequest>
              $request) async {
    return getSpaceDeletionCoordinatorStatus($call, await $request);
  }

  $async.Future<$1.GetSpaceDeletionCoordinatorStatusResponse>
      getSpaceDeletionCoordinatorStatus($grpc.ServiceCall call,
          $1.GetSpaceDeletionCoordinatorStatusRequest request);
}
