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

import 'member_admission.pb.dart' as $1;
import 'report_read.pb.dart' as $0;
import 'space.pb.dart' as $2;

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

  /// Current exact Space/tree/effective-history facts; no Role/content grant.
  /// @voice.security=protected;callers=service:messaging
  $grpc.ResponseFuture<$0.ResolveSpaceReportReadContextResponse>
      resolveSpaceReportReadContext(
    $0.ResolveSpaceReportReadContextRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveSpaceReportReadContext, request,
        options: options);
  }

  /// Narrow native membership authority; own DB only, no Role callback.
  /// @voice.security=protected;callers=service:role
  $grpc.ResponseFuture<$1.GetMemberAdmissionAuthorityResponse>
      getMemberAdmissionAuthority(
    $1.GetMemberAdmissionAuthorityRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getMemberAdmissionAuthority, request,
        options: options);
  }

  $grpc.ResponseFuture<$2.CreateSpaceResponse> createSpace(
    $2.CreateSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createSpace, request, options: options);
  }

  $grpc.ResponseFuture<$2.UpdateSpaceResponse> updateSpace(
    $2.UpdateSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateSpace, request, options: options);
  }

  /// Space-level matchmaking settings (mm_config jsonb; roadmap П.1).
  $grpc.ResponseFuture<$2.UpdateSpaceMmConfigResponse> updateSpaceMmConfig(
    $2.UpdateSpaceMmConfigRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateSpaceMmConfig, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$2.DeleteSpaceResponse> deleteSpace(
    $2.DeleteSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteSpace, request, options: options);
  }

  $grpc.ResponseFuture<$2.GetSpaceResponse> getSpace(
    $2.GetSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSpace, request, options: options);
  }

  /// Narrow existence/lifecycle read. No ACL lookup or public Gateway route.
  /// @voice.security=protected;callers=service:role
  $grpc.ResponseFuture<$2.ValidateRoleOverrideTargetResponse>
      validateRoleOverrideTarget(
    $2.ValidateRoleOverrideTargetRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$validateRoleOverrideTarget, request,
        options: options);
  }

  $grpc.ResponseFuture<$2.ListMySpacesResponse> listMySpaces(
    $2.ListMySpacesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listMySpaces, request, options: options);
  }

  $grpc.ResponseFuture<$2.SearchPublicSpacesResponse> searchPublicSpaces(
    $2.SearchPublicSpacesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$searchPublicSpaces, request, options: options);
  }

  $grpc.ResponseFuture<$2.CreateVoiceRoomResponse> createVoiceRoom(
    $2.CreateVoiceRoomRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createVoiceRoom, request, options: options);
  }

  $grpc.ResponseFuture<$2.UpdateVoiceRoomResponse> updateVoiceRoom(
    $2.UpdateVoiceRoomRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateVoiceRoom, request, options: options);
  }

  $grpc.ResponseFuture<$2.DeleteVoiceRoomResponse> deleteVoiceRoom(
    $2.DeleteVoiceRoomRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteVoiceRoom, request, options: options);
  }

  $grpc.ResponseFuture<$2.UpsertTreeNodeResponse> upsertTreeNode(
    $2.UpsertTreeNodeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$upsertTreeNode, request, options: options);
  }

  $grpc.ResponseFuture<$2.RemoveTreeNodeResponse> removeTreeNode(
    $2.RemoveTreeNodeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeTreeNode, request, options: options);
  }

  $grpc.ResponseFuture<$2.CreateCategoryResponse> createCategory(
    $2.CreateCategoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createCategory, request, options: options);
  }

  $grpc.ResponseFuture<$2.UpdateCategoryResponse> updateCategory(
    $2.UpdateCategoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateCategory, request, options: options);
  }

  $grpc.ResponseFuture<$2.DeleteCategoryResponse> deleteCategory(
    $2.DeleteCategoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteCategory, request, options: options);
  }

  $grpc.ResponseFuture<$2.ReorderSpaceTreeResponse> reorderSpaceTree(
    $2.ReorderSpaceTreeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$reorderSpaceTree, request, options: options);
  }

  $grpc.ResponseFuture<$2.PinTreeNodeResponse> pinTreeNode(
    $2.PinTreeNodeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$pinTreeNode, request, options: options);
  }

  $grpc.ResponseFuture<$2.UnpinTreeNodeResponse> unpinTreeNode(
    $2.UnpinTreeNodeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$unpinTreeNode, request, options: options);
  }

  $grpc.ResponseFuture<$2.ListSpaceTreeResponse> listSpaceTree(
    $2.ListSpaceTreeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listSpaceTree, request, options: options);
  }

  $grpc.ResponseFuture<$2.CreateInviteResponse> createInvite(
    $2.CreateInviteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createInvite, request, options: options);
  }

  $grpc.ResponseFuture<$2.RevokeInviteResponse> revokeInvite(
    $2.RevokeInviteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$revokeInvite, request, options: options);
  }

  $grpc.ResponseFuture<$2.GetInviteResponse> getInvite(
    $2.GetInviteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getInvite, request, options: options);
  }

  $grpc.ResponseFuture<$2.ListInvitesResponse> listInvites(
    $2.ListInvitesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listInvites, request, options: options);
  }

  $grpc.ResponseFuture<$2.JoinByInviteResponse> joinByInvite(
    $2.JoinByInviteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$joinByInvite, request, options: options);
  }

  $grpc.ResponseFuture<$2.JoinSpaceResponse> joinSpace(
    $2.JoinSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$joinSpace, request, options: options);
  }

  /// Exact authenticated applicant; no ambient proof or latest-attempt lookup.
  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$2.StartJoinAttemptResponse> startJoinAttempt(
    $2.StartJoinAttemptRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$startJoinAttempt, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$2.GetJoinAttemptResponse> getJoinAttempt(
    $2.GetJoinAttemptRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getJoinAttempt, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$2.SubmitJoinAttemptResponse> submitJoinAttempt(
    $2.SubmitJoinAttemptRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$submitJoinAttempt, request, options: options);
  }

  /// Current member plus SPACE_APPROVE_JOIN_REQUESTS; not invite/settings ACL.
  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$2.ListJoinRequestsResponse> listJoinRequests(
    $2.ListJoinRequestsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listJoinRequests, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$2.GetJoinRequestResponse> getJoinRequest(
    $2.GetJoinRequestRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getJoinRequest, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$2.ApproveJoinRequestResponse> approveJoinRequest(
    $2.ApproveJoinRequestRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$approveJoinRequest, request, options: options);
  }

  $grpc.ResponseFuture<$2.LeaveSpaceResponse> leaveSpace(
    $2.LeaveSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$leaveSpace, request, options: options);
  }

  $grpc.ResponseFuture<$2.KickMemberResponse> kickMember(
    $2.KickMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$kickMember, request, options: options);
  }

  $grpc.ResponseFuture<$2.BanMemberResponse> banMember(
    $2.BanMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$banMember, request, options: options);
  }

  $grpc.ResponseFuture<$2.UnbanMemberResponse> unbanMember(
    $2.UnbanMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$unbanMember, request, options: options);
  }

  $grpc.ResponseFuture<$2.ListMembersResponse> listMembers(
    $2.ListMembersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listMembers, request, options: options);
  }

  /// BE-234-nickname-v1: verified actor, global MEMBER_MANAGE_NICKNAMES;
  /// local members only, Owner protected, serialized redacted audit/outbox.
  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$2.UpdateMemberNicknameResponse> updateMemberNickname(
    $2.UpdateMemberNicknameRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateMemberNickname, request, options: options);
  }

  $grpc.ResponseFuture<$2.ListBansResponse> listBans(
    $2.ListBansRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listBans, request, options: options);
  }

  $grpc.ResponseFuture<$2.TimeoutMemberResponse> timeoutMember(
    $2.TimeoutMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$timeoutMember, request, options: options);
  }

  $grpc.ResponseFuture<$2.RemoveMemberTimeoutResponse> removeMemberTimeout(
    $2.RemoveMemberTimeoutRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeMemberTimeout, request, options: options);
  }

  $grpc.ResponseFuture<$2.TransferOwnershipResponse> transferOwnership(
    $2.TransferOwnershipRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$transferOwnership, request, options: options);
  }

  /// S2S: Bot Service adds bot actor profile to space membership on install.
  $grpc.ResponseFuture<$2.AddBotMemberResponse> addBotMember(
    $2.AddBotMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$addBotMember, request, options: options);
  }

  $grpc.ResponseFuture<$2.RemoveBotMemberResponse> removeBotMember(
    $2.RemoveBotMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeBotMember, request, options: options);
  }

  $grpc.ResponseFuture<$2.ListTemplatesResponse> listTemplates(
    $2.ListTemplatesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listTemplates, request, options: options);
  }

  $grpc.ResponseFuture<$2.CreateFromTemplateResponse> createFromTemplate(
    $2.CreateFromTemplateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createFromTemplate, request, options: options);
  }

  /// GIS-only versioned complete roster projection. Membership remains source-scoped and expires fail-closed.
  /// @voice.security=protected;callers=service:gameintegration
  $grpc.ResponseFuture<$2.ApplyCommunityRosterResponse> applyCommunityRoster(
    $2.ApplyCommunityRosterRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$applyCommunityRoster, request, options: options);
  }

  /// GIS-only idempotent community bootstrap. Owner identity is asserted by the authenticated GIS workload.
  /// @voice.security=protected;callers=service:gameintegration
  $grpc.ResponseFuture<$2.CreateCommunityBootstrapResponse>
      createCommunityBootstrap(
    $2.CreateCommunityBootstrapRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createCommunityBootstrap, request,
        options: options);
  }

  /// GIS-only durable Owner succession after a distinct operator approval.
  /// @voice.security=protected;callers=service:gameintegration
  $grpc.ResponseFuture<$2.RecoverCommunityOwnerResponse> recoverCommunityOwner(
    $2.RecoverCommunityOwnerRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$recoverCommunityOwner, request, options: options);
  }

  $grpc.ResponseFuture<$2.GetAuditLogResponse> getAuditLog(
    $2.GetAuditLogRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getAuditLog, request, options: options);
  }

  /// @voice.security=protected;callers=service:role,service:chat
  $grpc.ResponseFuture<$2.AppendAuditEventResponse> appendAuditEvent(
    $2.AppendAuditEventRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$appendAuditEvent, request, options: options);
  }

  /// Chat-owned durable create intent; not an ordinary user tree mutation.
  /// @voice.security=protected;callers=service:chat
  $grpc.ResponseFuture<$2.CommitSpaceChatCreationResponse>
      commitSpaceChatCreation(
    $2.CommitSpaceChatCreationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$commitSpaceChatCreation, request,
        options: options);
  }

  /// Exact immutable create outcome; caller authentication does not replace disclosure ACLs.
  /// Gateway uses a verified delegated-user principal and current disclosure ACL.
  /// @voice.security=protected;callers=service:chat,service:gateway
  $grpc.ResponseFuture<$2.GetSpaceChatCreationOutcomeResponse>
      getSpaceChatCreationOutcome(
    $2.GetSpaceChatCreationOutcomeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSpaceChatCreationOutcome, request,
        options: options);
  }

  /// S2S: privacy audience "space members" — shared membership between two profiles.
  $grpc.ResponseFuture<$2.AreCoMembersResponse> areCoMembers(
    $2.AreCoMembersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$areCoMembers, request, options: options);
  }

  /// S2S: canonical owner and exact membership evidence for Voice authorization.
  /// This endpoint is restricted to the verified Voice service identity.
  /// @voice.security=protected;callers=service:voice
  $grpc.ResponseFuture<$2.ResolveVoiceRoomAccessResponse>
      resolveVoiceRoomAccess(
    $2.ResolveVoiceRoomAccessRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveVoiceRoomAccess, request,
        options: options);
  }

  /// Fresh profile/Space timeout authority; no public or legacy transport.
  /// @voice.security=protected;callers=service:messaging
  $grpc.ResponseFuture<$2.ResolveMemberCommunicationStateResponse>
      resolveMemberCommunicationState(
    $2.ResolveMemberCommunicationStateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveMemberCommunicationState, request,
        options: options);
  }

  /// S2S: Subscription Service syncs Space Pro entitlement cache after webhook.
  $grpc.ResponseFuture<$2.SyncSpaceProSubscriptionResponse>
      syncSpaceProSubscription(
    $2.SyncSpaceProSubscriptionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$syncSpaceProSubscription, request,
        options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  $grpc.ResponseFuture<$2.RestoreSpaceResponse> restoreSpace(
    $2.RestoreSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$restoreSpace, request, options: options);
  }

  /// @voice.security=public_gateway;callers=service:gateway
  /// @voice.activation=held
  /// @voice.authorization=current_owner
  $grpc.ResponseFuture<$2.GetSpaceDeletionCoordinatorStatusResponse>
      getSpaceDeletionCoordinatorStatus(
    $2.GetSpaceDeletionCoordinatorStatusRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSpaceDeletionCoordinatorStatus, request,
        options: options);
  }

  // method descriptors

  static final _$resolveSpaceReportReadContext = $grpc.ClientMethod<
          $0.ResolveSpaceReportReadContextRequest,
          $0.ResolveSpaceReportReadContextResponse>(
      '/voice.space.v1.SpaceService/ResolveSpaceReportReadContext',
      ($0.ResolveSpaceReportReadContextRequest value) => value.writeToBuffer(),
      $0.ResolveSpaceReportReadContextResponse.fromBuffer);
  static final _$getMemberAdmissionAuthority = $grpc.ClientMethod<
          $1.GetMemberAdmissionAuthorityRequest,
          $1.GetMemberAdmissionAuthorityResponse>(
      '/voice.space.v1.SpaceService/GetMemberAdmissionAuthority',
      ($1.GetMemberAdmissionAuthorityRequest value) => value.writeToBuffer(),
      $1.GetMemberAdmissionAuthorityResponse.fromBuffer);
  static final _$createSpace =
      $grpc.ClientMethod<$2.CreateSpaceRequest, $2.CreateSpaceResponse>(
          '/voice.space.v1.SpaceService/CreateSpace',
          ($2.CreateSpaceRequest value) => value.writeToBuffer(),
          $2.CreateSpaceResponse.fromBuffer);
  static final _$updateSpace =
      $grpc.ClientMethod<$2.UpdateSpaceRequest, $2.UpdateSpaceResponse>(
          '/voice.space.v1.SpaceService/UpdateSpace',
          ($2.UpdateSpaceRequest value) => value.writeToBuffer(),
          $2.UpdateSpaceResponse.fromBuffer);
  static final _$updateSpaceMmConfig = $grpc.ClientMethod<
          $2.UpdateSpaceMmConfigRequest, $2.UpdateSpaceMmConfigResponse>(
      '/voice.space.v1.SpaceService/UpdateSpaceMmConfig',
      ($2.UpdateSpaceMmConfigRequest value) => value.writeToBuffer(),
      $2.UpdateSpaceMmConfigResponse.fromBuffer);
  static final _$deleteSpace =
      $grpc.ClientMethod<$2.DeleteSpaceRequest, $2.DeleteSpaceResponse>(
          '/voice.space.v1.SpaceService/DeleteSpace',
          ($2.DeleteSpaceRequest value) => value.writeToBuffer(),
          $2.DeleteSpaceResponse.fromBuffer);
  static final _$getSpace =
      $grpc.ClientMethod<$2.GetSpaceRequest, $2.GetSpaceResponse>(
          '/voice.space.v1.SpaceService/GetSpace',
          ($2.GetSpaceRequest value) => value.writeToBuffer(),
          $2.GetSpaceResponse.fromBuffer);
  static final _$validateRoleOverrideTarget = $grpc.ClientMethod<
          $2.ValidateRoleOverrideTargetRequest,
          $2.ValidateRoleOverrideTargetResponse>(
      '/voice.space.v1.SpaceService/ValidateRoleOverrideTarget',
      ($2.ValidateRoleOverrideTargetRequest value) => value.writeToBuffer(),
      $2.ValidateRoleOverrideTargetResponse.fromBuffer);
  static final _$listMySpaces =
      $grpc.ClientMethod<$2.ListMySpacesRequest, $2.ListMySpacesResponse>(
          '/voice.space.v1.SpaceService/ListMySpaces',
          ($2.ListMySpacesRequest value) => value.writeToBuffer(),
          $2.ListMySpacesResponse.fromBuffer);
  static final _$searchPublicSpaces = $grpc.ClientMethod<
          $2.SearchPublicSpacesRequest, $2.SearchPublicSpacesResponse>(
      '/voice.space.v1.SpaceService/SearchPublicSpaces',
      ($2.SearchPublicSpacesRequest value) => value.writeToBuffer(),
      $2.SearchPublicSpacesResponse.fromBuffer);
  static final _$createVoiceRoom =
      $grpc.ClientMethod<$2.CreateVoiceRoomRequest, $2.CreateVoiceRoomResponse>(
          '/voice.space.v1.SpaceService/CreateVoiceRoom',
          ($2.CreateVoiceRoomRequest value) => value.writeToBuffer(),
          $2.CreateVoiceRoomResponse.fromBuffer);
  static final _$updateVoiceRoom =
      $grpc.ClientMethod<$2.UpdateVoiceRoomRequest, $2.UpdateVoiceRoomResponse>(
          '/voice.space.v1.SpaceService/UpdateVoiceRoom',
          ($2.UpdateVoiceRoomRequest value) => value.writeToBuffer(),
          $2.UpdateVoiceRoomResponse.fromBuffer);
  static final _$deleteVoiceRoom =
      $grpc.ClientMethod<$2.DeleteVoiceRoomRequest, $2.DeleteVoiceRoomResponse>(
          '/voice.space.v1.SpaceService/DeleteVoiceRoom',
          ($2.DeleteVoiceRoomRequest value) => value.writeToBuffer(),
          $2.DeleteVoiceRoomResponse.fromBuffer);
  static final _$upsertTreeNode =
      $grpc.ClientMethod<$2.UpsertTreeNodeRequest, $2.UpsertTreeNodeResponse>(
          '/voice.space.v1.SpaceService/UpsertTreeNode',
          ($2.UpsertTreeNodeRequest value) => value.writeToBuffer(),
          $2.UpsertTreeNodeResponse.fromBuffer);
  static final _$removeTreeNode =
      $grpc.ClientMethod<$2.RemoveTreeNodeRequest, $2.RemoveTreeNodeResponse>(
          '/voice.space.v1.SpaceService/RemoveTreeNode',
          ($2.RemoveTreeNodeRequest value) => value.writeToBuffer(),
          $2.RemoveTreeNodeResponse.fromBuffer);
  static final _$createCategory =
      $grpc.ClientMethod<$2.CreateCategoryRequest, $2.CreateCategoryResponse>(
          '/voice.space.v1.SpaceService/CreateCategory',
          ($2.CreateCategoryRequest value) => value.writeToBuffer(),
          $2.CreateCategoryResponse.fromBuffer);
  static final _$updateCategory =
      $grpc.ClientMethod<$2.UpdateCategoryRequest, $2.UpdateCategoryResponse>(
          '/voice.space.v1.SpaceService/UpdateCategory',
          ($2.UpdateCategoryRequest value) => value.writeToBuffer(),
          $2.UpdateCategoryResponse.fromBuffer);
  static final _$deleteCategory =
      $grpc.ClientMethod<$2.DeleteCategoryRequest, $2.DeleteCategoryResponse>(
          '/voice.space.v1.SpaceService/DeleteCategory',
          ($2.DeleteCategoryRequest value) => value.writeToBuffer(),
          $2.DeleteCategoryResponse.fromBuffer);
  static final _$reorderSpaceTree = $grpc.ClientMethod<
          $2.ReorderSpaceTreeRequest, $2.ReorderSpaceTreeResponse>(
      '/voice.space.v1.SpaceService/ReorderSpaceTree',
      ($2.ReorderSpaceTreeRequest value) => value.writeToBuffer(),
      $2.ReorderSpaceTreeResponse.fromBuffer);
  static final _$pinTreeNode =
      $grpc.ClientMethod<$2.PinTreeNodeRequest, $2.PinTreeNodeResponse>(
          '/voice.space.v1.SpaceService/PinTreeNode',
          ($2.PinTreeNodeRequest value) => value.writeToBuffer(),
          $2.PinTreeNodeResponse.fromBuffer);
  static final _$unpinTreeNode =
      $grpc.ClientMethod<$2.UnpinTreeNodeRequest, $2.UnpinTreeNodeResponse>(
          '/voice.space.v1.SpaceService/UnpinTreeNode',
          ($2.UnpinTreeNodeRequest value) => value.writeToBuffer(),
          $2.UnpinTreeNodeResponse.fromBuffer);
  static final _$listSpaceTree =
      $grpc.ClientMethod<$2.ListSpaceTreeRequest, $2.ListSpaceTreeResponse>(
          '/voice.space.v1.SpaceService/ListSpaceTree',
          ($2.ListSpaceTreeRequest value) => value.writeToBuffer(),
          $2.ListSpaceTreeResponse.fromBuffer);
  static final _$createInvite =
      $grpc.ClientMethod<$2.CreateInviteRequest, $2.CreateInviteResponse>(
          '/voice.space.v1.SpaceService/CreateInvite',
          ($2.CreateInviteRequest value) => value.writeToBuffer(),
          $2.CreateInviteResponse.fromBuffer);
  static final _$revokeInvite =
      $grpc.ClientMethod<$2.RevokeInviteRequest, $2.RevokeInviteResponse>(
          '/voice.space.v1.SpaceService/RevokeInvite',
          ($2.RevokeInviteRequest value) => value.writeToBuffer(),
          $2.RevokeInviteResponse.fromBuffer);
  static final _$getInvite =
      $grpc.ClientMethod<$2.GetInviteRequest, $2.GetInviteResponse>(
          '/voice.space.v1.SpaceService/GetInvite',
          ($2.GetInviteRequest value) => value.writeToBuffer(),
          $2.GetInviteResponse.fromBuffer);
  static final _$listInvites =
      $grpc.ClientMethod<$2.ListInvitesRequest, $2.ListInvitesResponse>(
          '/voice.space.v1.SpaceService/ListInvites',
          ($2.ListInvitesRequest value) => value.writeToBuffer(),
          $2.ListInvitesResponse.fromBuffer);
  static final _$joinByInvite =
      $grpc.ClientMethod<$2.JoinByInviteRequest, $2.JoinByInviteResponse>(
          '/voice.space.v1.SpaceService/JoinByInvite',
          ($2.JoinByInviteRequest value) => value.writeToBuffer(),
          $2.JoinByInviteResponse.fromBuffer);
  static final _$joinSpace =
      $grpc.ClientMethod<$2.JoinSpaceRequest, $2.JoinSpaceResponse>(
          '/voice.space.v1.SpaceService/JoinSpace',
          ($2.JoinSpaceRequest value) => value.writeToBuffer(),
          $2.JoinSpaceResponse.fromBuffer);
  static final _$startJoinAttempt = $grpc.ClientMethod<
          $2.StartJoinAttemptRequest, $2.StartJoinAttemptResponse>(
      '/voice.space.v1.SpaceService/StartJoinAttempt',
      ($2.StartJoinAttemptRequest value) => value.writeToBuffer(),
      $2.StartJoinAttemptResponse.fromBuffer);
  static final _$getJoinAttempt =
      $grpc.ClientMethod<$2.GetJoinAttemptRequest, $2.GetJoinAttemptResponse>(
          '/voice.space.v1.SpaceService/GetJoinAttempt',
          ($2.GetJoinAttemptRequest value) => value.writeToBuffer(),
          $2.GetJoinAttemptResponse.fromBuffer);
  static final _$submitJoinAttempt = $grpc.ClientMethod<
          $2.SubmitJoinAttemptRequest, $2.SubmitJoinAttemptResponse>(
      '/voice.space.v1.SpaceService/SubmitJoinAttempt',
      ($2.SubmitJoinAttemptRequest value) => value.writeToBuffer(),
      $2.SubmitJoinAttemptResponse.fromBuffer);
  static final _$listJoinRequests = $grpc.ClientMethod<
          $2.ListJoinRequestsRequest, $2.ListJoinRequestsResponse>(
      '/voice.space.v1.SpaceService/ListJoinRequests',
      ($2.ListJoinRequestsRequest value) => value.writeToBuffer(),
      $2.ListJoinRequestsResponse.fromBuffer);
  static final _$getJoinRequest =
      $grpc.ClientMethod<$2.GetJoinRequestRequest, $2.GetJoinRequestResponse>(
          '/voice.space.v1.SpaceService/GetJoinRequest',
          ($2.GetJoinRequestRequest value) => value.writeToBuffer(),
          $2.GetJoinRequestResponse.fromBuffer);
  static final _$approveJoinRequest = $grpc.ClientMethod<
          $2.ApproveJoinRequestRequest, $2.ApproveJoinRequestResponse>(
      '/voice.space.v1.SpaceService/ApproveJoinRequest',
      ($2.ApproveJoinRequestRequest value) => value.writeToBuffer(),
      $2.ApproveJoinRequestResponse.fromBuffer);
  static final _$leaveSpace =
      $grpc.ClientMethod<$2.LeaveSpaceRequest, $2.LeaveSpaceResponse>(
          '/voice.space.v1.SpaceService/LeaveSpace',
          ($2.LeaveSpaceRequest value) => value.writeToBuffer(),
          $2.LeaveSpaceResponse.fromBuffer);
  static final _$kickMember =
      $grpc.ClientMethod<$2.KickMemberRequest, $2.KickMemberResponse>(
          '/voice.space.v1.SpaceService/KickMember',
          ($2.KickMemberRequest value) => value.writeToBuffer(),
          $2.KickMemberResponse.fromBuffer);
  static final _$banMember =
      $grpc.ClientMethod<$2.BanMemberRequest, $2.BanMemberResponse>(
          '/voice.space.v1.SpaceService/BanMember',
          ($2.BanMemberRequest value) => value.writeToBuffer(),
          $2.BanMemberResponse.fromBuffer);
  static final _$unbanMember =
      $grpc.ClientMethod<$2.UnbanMemberRequest, $2.UnbanMemberResponse>(
          '/voice.space.v1.SpaceService/UnbanMember',
          ($2.UnbanMemberRequest value) => value.writeToBuffer(),
          $2.UnbanMemberResponse.fromBuffer);
  static final _$listMembers =
      $grpc.ClientMethod<$2.ListMembersRequest, $2.ListMembersResponse>(
          '/voice.space.v1.SpaceService/ListMembers',
          ($2.ListMembersRequest value) => value.writeToBuffer(),
          $2.ListMembersResponse.fromBuffer);
  static final _$updateMemberNickname = $grpc.ClientMethod<
          $2.UpdateMemberNicknameRequest, $2.UpdateMemberNicknameResponse>(
      '/voice.space.v1.SpaceService/UpdateMemberNickname',
      ($2.UpdateMemberNicknameRequest value) => value.writeToBuffer(),
      $2.UpdateMemberNicknameResponse.fromBuffer);
  static final _$listBans =
      $grpc.ClientMethod<$2.ListBansRequest, $2.ListBansResponse>(
          '/voice.space.v1.SpaceService/ListBans',
          ($2.ListBansRequest value) => value.writeToBuffer(),
          $2.ListBansResponse.fromBuffer);
  static final _$timeoutMember =
      $grpc.ClientMethod<$2.TimeoutMemberRequest, $2.TimeoutMemberResponse>(
          '/voice.space.v1.SpaceService/TimeoutMember',
          ($2.TimeoutMemberRequest value) => value.writeToBuffer(),
          $2.TimeoutMemberResponse.fromBuffer);
  static final _$removeMemberTimeout = $grpc.ClientMethod<
          $2.RemoveMemberTimeoutRequest, $2.RemoveMemberTimeoutResponse>(
      '/voice.space.v1.SpaceService/RemoveMemberTimeout',
      ($2.RemoveMemberTimeoutRequest value) => value.writeToBuffer(),
      $2.RemoveMemberTimeoutResponse.fromBuffer);
  static final _$transferOwnership = $grpc.ClientMethod<
          $2.TransferOwnershipRequest, $2.TransferOwnershipResponse>(
      '/voice.space.v1.SpaceService/TransferOwnership',
      ($2.TransferOwnershipRequest value) => value.writeToBuffer(),
      $2.TransferOwnershipResponse.fromBuffer);
  static final _$addBotMember =
      $grpc.ClientMethod<$2.AddBotMemberRequest, $2.AddBotMemberResponse>(
          '/voice.space.v1.SpaceService/AddBotMember',
          ($2.AddBotMemberRequest value) => value.writeToBuffer(),
          $2.AddBotMemberResponse.fromBuffer);
  static final _$removeBotMember =
      $grpc.ClientMethod<$2.RemoveBotMemberRequest, $2.RemoveBotMemberResponse>(
          '/voice.space.v1.SpaceService/RemoveBotMember',
          ($2.RemoveBotMemberRequest value) => value.writeToBuffer(),
          $2.RemoveBotMemberResponse.fromBuffer);
  static final _$listTemplates =
      $grpc.ClientMethod<$2.ListTemplatesRequest, $2.ListTemplatesResponse>(
          '/voice.space.v1.SpaceService/ListTemplates',
          ($2.ListTemplatesRequest value) => value.writeToBuffer(),
          $2.ListTemplatesResponse.fromBuffer);
  static final _$createFromTemplate = $grpc.ClientMethod<
          $2.CreateFromTemplateRequest, $2.CreateFromTemplateResponse>(
      '/voice.space.v1.SpaceService/CreateFromTemplate',
      ($2.CreateFromTemplateRequest value) => value.writeToBuffer(),
      $2.CreateFromTemplateResponse.fromBuffer);
  static final _$applyCommunityRoster = $grpc.ClientMethod<
          $2.ApplyCommunityRosterRequest, $2.ApplyCommunityRosterResponse>(
      '/voice.space.v1.SpaceService/ApplyCommunityRoster',
      ($2.ApplyCommunityRosterRequest value) => value.writeToBuffer(),
      $2.ApplyCommunityRosterResponse.fromBuffer);
  static final _$createCommunityBootstrap = $grpc.ClientMethod<
          $2.CreateCommunityBootstrapRequest,
          $2.CreateCommunityBootstrapResponse>(
      '/voice.space.v1.SpaceService/CreateCommunityBootstrap',
      ($2.CreateCommunityBootstrapRequest value) => value.writeToBuffer(),
      $2.CreateCommunityBootstrapResponse.fromBuffer);
  static final _$recoverCommunityOwner = $grpc.ClientMethod<
          $2.RecoverCommunityOwnerRequest, $2.RecoverCommunityOwnerResponse>(
      '/voice.space.v1.SpaceService/RecoverCommunityOwner',
      ($2.RecoverCommunityOwnerRequest value) => value.writeToBuffer(),
      $2.RecoverCommunityOwnerResponse.fromBuffer);
  static final _$getAuditLog =
      $grpc.ClientMethod<$2.GetAuditLogRequest, $2.GetAuditLogResponse>(
          '/voice.space.v1.SpaceService/GetAuditLog',
          ($2.GetAuditLogRequest value) => value.writeToBuffer(),
          $2.GetAuditLogResponse.fromBuffer);
  static final _$appendAuditEvent = $grpc.ClientMethod<
          $2.AppendAuditEventRequest, $2.AppendAuditEventResponse>(
      '/voice.space.v1.SpaceService/AppendAuditEvent',
      ($2.AppendAuditEventRequest value) => value.writeToBuffer(),
      $2.AppendAuditEventResponse.fromBuffer);
  static final _$commitSpaceChatCreation = $grpc.ClientMethod<
          $2.CommitSpaceChatCreationRequest,
          $2.CommitSpaceChatCreationResponse>(
      '/voice.space.v1.SpaceService/CommitSpaceChatCreation',
      ($2.CommitSpaceChatCreationRequest value) => value.writeToBuffer(),
      $2.CommitSpaceChatCreationResponse.fromBuffer);
  static final _$getSpaceChatCreationOutcome = $grpc.ClientMethod<
          $2.GetSpaceChatCreationOutcomeRequest,
          $2.GetSpaceChatCreationOutcomeResponse>(
      '/voice.space.v1.SpaceService/GetSpaceChatCreationOutcome',
      ($2.GetSpaceChatCreationOutcomeRequest value) => value.writeToBuffer(),
      $2.GetSpaceChatCreationOutcomeResponse.fromBuffer);
  static final _$areCoMembers =
      $grpc.ClientMethod<$2.AreCoMembersRequest, $2.AreCoMembersResponse>(
          '/voice.space.v1.SpaceService/AreCoMembers',
          ($2.AreCoMembersRequest value) => value.writeToBuffer(),
          $2.AreCoMembersResponse.fromBuffer);
  static final _$resolveVoiceRoomAccess = $grpc.ClientMethod<
          $2.ResolveVoiceRoomAccessRequest, $2.ResolveVoiceRoomAccessResponse>(
      '/voice.space.v1.SpaceService/ResolveVoiceRoomAccess',
      ($2.ResolveVoiceRoomAccessRequest value) => value.writeToBuffer(),
      $2.ResolveVoiceRoomAccessResponse.fromBuffer);
  static final _$resolveMemberCommunicationState = $grpc.ClientMethod<
          $2.ResolveMemberCommunicationStateRequest,
          $2.ResolveMemberCommunicationStateResponse>(
      '/voice.space.v1.SpaceService/ResolveMemberCommunicationState',
      ($2.ResolveMemberCommunicationStateRequest value) =>
          value.writeToBuffer(),
      $2.ResolveMemberCommunicationStateResponse.fromBuffer);
  static final _$syncSpaceProSubscription = $grpc.ClientMethod<
          $2.SyncSpaceProSubscriptionRequest,
          $2.SyncSpaceProSubscriptionResponse>(
      '/voice.space.v1.SpaceService/SyncSpaceProSubscription',
      ($2.SyncSpaceProSubscriptionRequest value) => value.writeToBuffer(),
      $2.SyncSpaceProSubscriptionResponse.fromBuffer);
  static final _$restoreSpace =
      $grpc.ClientMethod<$2.RestoreSpaceRequest, $2.RestoreSpaceResponse>(
          '/voice.space.v1.SpaceService/RestoreSpace',
          ($2.RestoreSpaceRequest value) => value.writeToBuffer(),
          $2.RestoreSpaceResponse.fromBuffer);
  static final _$getSpaceDeletionCoordinatorStatus = $grpc.ClientMethod<
          $2.GetSpaceDeletionCoordinatorStatusRequest,
          $2.GetSpaceDeletionCoordinatorStatusResponse>(
      '/voice.space.v1.SpaceService/GetSpaceDeletionCoordinatorStatus',
      ($2.GetSpaceDeletionCoordinatorStatusRequest value) =>
          value.writeToBuffer(),
      $2.GetSpaceDeletionCoordinatorStatusResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.space.v1.SpaceService')
abstract class SpaceServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.space.v1.SpaceService';

  SpaceServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ResolveSpaceReportReadContextRequest,
            $0.ResolveSpaceReportReadContextResponse>(
        'ResolveSpaceReportReadContext',
        resolveSpaceReportReadContext_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ResolveSpaceReportReadContextRequest.fromBuffer(value),
        ($0.ResolveSpaceReportReadContextResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetMemberAdmissionAuthorityRequest,
            $1.GetMemberAdmissionAuthorityResponse>(
        'GetMemberAdmissionAuthority',
        getMemberAdmissionAuthority_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetMemberAdmissionAuthorityRequest.fromBuffer(value),
        ($1.GetMemberAdmissionAuthorityResponse value) =>
            value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.CreateSpaceRequest, $2.CreateSpaceResponse>(
            'CreateSpace',
            createSpace_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.CreateSpaceRequest.fromBuffer(value),
            ($2.CreateSpaceResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.UpdateSpaceRequest, $2.UpdateSpaceResponse>(
            'UpdateSpace',
            updateSpace_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.UpdateSpaceRequest.fromBuffer(value),
            ($2.UpdateSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.UpdateSpaceMmConfigRequest,
            $2.UpdateSpaceMmConfigResponse>(
        'UpdateSpaceMmConfig',
        updateSpaceMmConfig_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.UpdateSpaceMmConfigRequest.fromBuffer(value),
        ($2.UpdateSpaceMmConfigResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.DeleteSpaceRequest, $2.DeleteSpaceResponse>(
            'DeleteSpace',
            deleteSpace_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.DeleteSpaceRequest.fromBuffer(value),
            ($2.DeleteSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.GetSpaceRequest, $2.GetSpaceResponse>(
        'GetSpace',
        getSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $2.GetSpaceRequest.fromBuffer(value),
        ($2.GetSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.ValidateRoleOverrideTargetRequest,
            $2.ValidateRoleOverrideTargetResponse>(
        'ValidateRoleOverrideTarget',
        validateRoleOverrideTarget_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.ValidateRoleOverrideTargetRequest.fromBuffer(value),
        ($2.ValidateRoleOverrideTargetResponse value) =>
            value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.ListMySpacesRequest, $2.ListMySpacesResponse>(
            'ListMySpaces',
            listMySpaces_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.ListMySpacesRequest.fromBuffer(value),
            ($2.ListMySpacesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.SearchPublicSpacesRequest,
            $2.SearchPublicSpacesResponse>(
        'SearchPublicSpaces',
        searchPublicSpaces_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.SearchPublicSpacesRequest.fromBuffer(value),
        ($2.SearchPublicSpacesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.CreateVoiceRoomRequest,
            $2.CreateVoiceRoomResponse>(
        'CreateVoiceRoom',
        createVoiceRoom_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.CreateVoiceRoomRequest.fromBuffer(value),
        ($2.CreateVoiceRoomResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.UpdateVoiceRoomRequest,
            $2.UpdateVoiceRoomResponse>(
        'UpdateVoiceRoom',
        updateVoiceRoom_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.UpdateVoiceRoomRequest.fromBuffer(value),
        ($2.UpdateVoiceRoomResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.DeleteVoiceRoomRequest,
            $2.DeleteVoiceRoomResponse>(
        'DeleteVoiceRoom',
        deleteVoiceRoom_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.DeleteVoiceRoomRequest.fromBuffer(value),
        ($2.DeleteVoiceRoomResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.UpsertTreeNodeRequest,
            $2.UpsertTreeNodeResponse>(
        'UpsertTreeNode',
        upsertTreeNode_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.UpsertTreeNodeRequest.fromBuffer(value),
        ($2.UpsertTreeNodeResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.RemoveTreeNodeRequest,
            $2.RemoveTreeNodeResponse>(
        'RemoveTreeNode',
        removeTreeNode_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.RemoveTreeNodeRequest.fromBuffer(value),
        ($2.RemoveTreeNodeResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.CreateCategoryRequest,
            $2.CreateCategoryResponse>(
        'CreateCategory',
        createCategory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.CreateCategoryRequest.fromBuffer(value),
        ($2.CreateCategoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.UpdateCategoryRequest,
            $2.UpdateCategoryResponse>(
        'UpdateCategory',
        updateCategory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.UpdateCategoryRequest.fromBuffer(value),
        ($2.UpdateCategoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.DeleteCategoryRequest,
            $2.DeleteCategoryResponse>(
        'DeleteCategory',
        deleteCategory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.DeleteCategoryRequest.fromBuffer(value),
        ($2.DeleteCategoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.ReorderSpaceTreeRequest,
            $2.ReorderSpaceTreeResponse>(
        'ReorderSpaceTree',
        reorderSpaceTree_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.ReorderSpaceTreeRequest.fromBuffer(value),
        ($2.ReorderSpaceTreeResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.PinTreeNodeRequest, $2.PinTreeNodeResponse>(
            'PinTreeNode',
            pinTreeNode_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.PinTreeNodeRequest.fromBuffer(value),
            ($2.PinTreeNodeResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.UnpinTreeNodeRequest, $2.UnpinTreeNodeResponse>(
            'UnpinTreeNode',
            unpinTreeNode_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.UnpinTreeNodeRequest.fromBuffer(value),
            ($2.UnpinTreeNodeResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.ListSpaceTreeRequest, $2.ListSpaceTreeResponse>(
            'ListSpaceTree',
            listSpaceTree_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.ListSpaceTreeRequest.fromBuffer(value),
            ($2.ListSpaceTreeResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.CreateInviteRequest, $2.CreateInviteResponse>(
            'CreateInvite',
            createInvite_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.CreateInviteRequest.fromBuffer(value),
            ($2.CreateInviteResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.RevokeInviteRequest, $2.RevokeInviteResponse>(
            'RevokeInvite',
            revokeInvite_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.RevokeInviteRequest.fromBuffer(value),
            ($2.RevokeInviteResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.GetInviteRequest, $2.GetInviteResponse>(
        'GetInvite',
        getInvite_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $2.GetInviteRequest.fromBuffer(value),
        ($2.GetInviteResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.ListInvitesRequest, $2.ListInvitesResponse>(
            'ListInvites',
            listInvites_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.ListInvitesRequest.fromBuffer(value),
            ($2.ListInvitesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.JoinByInviteRequest, $2.JoinByInviteResponse>(
            'JoinByInvite',
            joinByInvite_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.JoinByInviteRequest.fromBuffer(value),
            ($2.JoinByInviteResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.JoinSpaceRequest, $2.JoinSpaceResponse>(
        'JoinSpace',
        joinSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $2.JoinSpaceRequest.fromBuffer(value),
        ($2.JoinSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.StartJoinAttemptRequest,
            $2.StartJoinAttemptResponse>(
        'StartJoinAttempt',
        startJoinAttempt_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.StartJoinAttemptRequest.fromBuffer(value),
        ($2.StartJoinAttemptResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.GetJoinAttemptRequest,
            $2.GetJoinAttemptResponse>(
        'GetJoinAttempt',
        getJoinAttempt_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.GetJoinAttemptRequest.fromBuffer(value),
        ($2.GetJoinAttemptResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.SubmitJoinAttemptRequest,
            $2.SubmitJoinAttemptResponse>(
        'SubmitJoinAttempt',
        submitJoinAttempt_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.SubmitJoinAttemptRequest.fromBuffer(value),
        ($2.SubmitJoinAttemptResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.ListJoinRequestsRequest,
            $2.ListJoinRequestsResponse>(
        'ListJoinRequests',
        listJoinRequests_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.ListJoinRequestsRequest.fromBuffer(value),
        ($2.ListJoinRequestsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.GetJoinRequestRequest,
            $2.GetJoinRequestResponse>(
        'GetJoinRequest',
        getJoinRequest_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.GetJoinRequestRequest.fromBuffer(value),
        ($2.GetJoinRequestResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.ApproveJoinRequestRequest,
            $2.ApproveJoinRequestResponse>(
        'ApproveJoinRequest',
        approveJoinRequest_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.ApproveJoinRequestRequest.fromBuffer(value),
        ($2.ApproveJoinRequestResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.LeaveSpaceRequest, $2.LeaveSpaceResponse>(
        'LeaveSpace',
        leaveSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $2.LeaveSpaceRequest.fromBuffer(value),
        ($2.LeaveSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.KickMemberRequest, $2.KickMemberResponse>(
        'KickMember',
        kickMember_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $2.KickMemberRequest.fromBuffer(value),
        ($2.KickMemberResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.BanMemberRequest, $2.BanMemberResponse>(
        'BanMember',
        banMember_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $2.BanMemberRequest.fromBuffer(value),
        ($2.BanMemberResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.UnbanMemberRequest, $2.UnbanMemberResponse>(
            'UnbanMember',
            unbanMember_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.UnbanMemberRequest.fromBuffer(value),
            ($2.UnbanMemberResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.ListMembersRequest, $2.ListMembersResponse>(
            'ListMembers',
            listMembers_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.ListMembersRequest.fromBuffer(value),
            ($2.ListMembersResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.UpdateMemberNicknameRequest,
            $2.UpdateMemberNicknameResponse>(
        'UpdateMemberNickname',
        updateMemberNickname_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.UpdateMemberNicknameRequest.fromBuffer(value),
        ($2.UpdateMemberNicknameResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.ListBansRequest, $2.ListBansResponse>(
        'ListBans',
        listBans_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $2.ListBansRequest.fromBuffer(value),
        ($2.ListBansResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.TimeoutMemberRequest, $2.TimeoutMemberResponse>(
            'TimeoutMember',
            timeoutMember_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.TimeoutMemberRequest.fromBuffer(value),
            ($2.TimeoutMemberResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.RemoveMemberTimeoutRequest,
            $2.RemoveMemberTimeoutResponse>(
        'RemoveMemberTimeout',
        removeMemberTimeout_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.RemoveMemberTimeoutRequest.fromBuffer(value),
        ($2.RemoveMemberTimeoutResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.TransferOwnershipRequest,
            $2.TransferOwnershipResponse>(
        'TransferOwnership',
        transferOwnership_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.TransferOwnershipRequest.fromBuffer(value),
        ($2.TransferOwnershipResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.AddBotMemberRequest, $2.AddBotMemberResponse>(
            'AddBotMember',
            addBotMember_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.AddBotMemberRequest.fromBuffer(value),
            ($2.AddBotMemberResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.RemoveBotMemberRequest,
            $2.RemoveBotMemberResponse>(
        'RemoveBotMember',
        removeBotMember_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.RemoveBotMemberRequest.fromBuffer(value),
        ($2.RemoveBotMemberResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.ListTemplatesRequest, $2.ListTemplatesResponse>(
            'ListTemplates',
            listTemplates_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.ListTemplatesRequest.fromBuffer(value),
            ($2.ListTemplatesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.CreateFromTemplateRequest,
            $2.CreateFromTemplateResponse>(
        'CreateFromTemplate',
        createFromTemplate_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.CreateFromTemplateRequest.fromBuffer(value),
        ($2.CreateFromTemplateResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.ApplyCommunityRosterRequest,
            $2.ApplyCommunityRosterResponse>(
        'ApplyCommunityRoster',
        applyCommunityRoster_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.ApplyCommunityRosterRequest.fromBuffer(value),
        ($2.ApplyCommunityRosterResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.CreateCommunityBootstrapRequest,
            $2.CreateCommunityBootstrapResponse>(
        'CreateCommunityBootstrap',
        createCommunityBootstrap_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.CreateCommunityBootstrapRequest.fromBuffer(value),
        ($2.CreateCommunityBootstrapResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.RecoverCommunityOwnerRequest,
            $2.RecoverCommunityOwnerResponse>(
        'RecoverCommunityOwner',
        recoverCommunityOwner_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.RecoverCommunityOwnerRequest.fromBuffer(value),
        ($2.RecoverCommunityOwnerResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.GetAuditLogRequest, $2.GetAuditLogResponse>(
            'GetAuditLog',
            getAuditLog_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.GetAuditLogRequest.fromBuffer(value),
            ($2.GetAuditLogResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.AppendAuditEventRequest,
            $2.AppendAuditEventResponse>(
        'AppendAuditEvent',
        appendAuditEvent_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.AppendAuditEventRequest.fromBuffer(value),
        ($2.AppendAuditEventResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.CommitSpaceChatCreationRequest,
            $2.CommitSpaceChatCreationResponse>(
        'CommitSpaceChatCreation',
        commitSpaceChatCreation_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.CommitSpaceChatCreationRequest.fromBuffer(value),
        ($2.CommitSpaceChatCreationResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.GetSpaceChatCreationOutcomeRequest,
            $2.GetSpaceChatCreationOutcomeResponse>(
        'GetSpaceChatCreationOutcome',
        getSpaceChatCreationOutcome_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.GetSpaceChatCreationOutcomeRequest.fromBuffer(value),
        ($2.GetSpaceChatCreationOutcomeResponse value) =>
            value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.AreCoMembersRequest, $2.AreCoMembersResponse>(
            'AreCoMembers',
            areCoMembers_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.AreCoMembersRequest.fromBuffer(value),
            ($2.AreCoMembersResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.ResolveVoiceRoomAccessRequest,
            $2.ResolveVoiceRoomAccessResponse>(
        'ResolveVoiceRoomAccess',
        resolveVoiceRoomAccess_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.ResolveVoiceRoomAccessRequest.fromBuffer(value),
        ($2.ResolveVoiceRoomAccessResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.ResolveMemberCommunicationStateRequest,
            $2.ResolveMemberCommunicationStateResponse>(
        'ResolveMemberCommunicationState',
        resolveMemberCommunicationState_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.ResolveMemberCommunicationStateRequest.fromBuffer(value),
        ($2.ResolveMemberCommunicationStateResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.SyncSpaceProSubscriptionRequest,
            $2.SyncSpaceProSubscriptionResponse>(
        'SyncSpaceProSubscription',
        syncSpaceProSubscription_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.SyncSpaceProSubscriptionRequest.fromBuffer(value),
        ($2.SyncSpaceProSubscriptionResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$2.RestoreSpaceRequest, $2.RestoreSpaceResponse>(
            'RestoreSpace',
            restoreSpace_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $2.RestoreSpaceRequest.fromBuffer(value),
            ($2.RestoreSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$2.GetSpaceDeletionCoordinatorStatusRequest,
            $2.GetSpaceDeletionCoordinatorStatusResponse>(
        'GetSpaceDeletionCoordinatorStatus',
        getSpaceDeletionCoordinatorStatus_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $2.GetSpaceDeletionCoordinatorStatusRequest.fromBuffer(value),
        ($2.GetSpaceDeletionCoordinatorStatusResponse value) =>
            value.writeToBuffer()));
  }

  $async.Future<$0.ResolveSpaceReportReadContextResponse>
      resolveSpaceReportReadContext_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$0.ResolveSpaceReportReadContextRequest>
              $request) async {
    return resolveSpaceReportReadContext($call, await $request);
  }

  $async.Future<$0.ResolveSpaceReportReadContextResponse>
      resolveSpaceReportReadContext($grpc.ServiceCall call,
          $0.ResolveSpaceReportReadContextRequest request);

  $async.Future<$1.GetMemberAdmissionAuthorityResponse>
      getMemberAdmissionAuthority_Pre($grpc.ServiceCall $call,
          $async.Future<$1.GetMemberAdmissionAuthorityRequest> $request) async {
    return getMemberAdmissionAuthority($call, await $request);
  }

  $async.Future<$1.GetMemberAdmissionAuthorityResponse>
      getMemberAdmissionAuthority($grpc.ServiceCall call,
          $1.GetMemberAdmissionAuthorityRequest request);

  $async.Future<$2.CreateSpaceResponse> createSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$2.CreateSpaceRequest> $request) async {
    return createSpace($call, await $request);
  }

  $async.Future<$2.CreateSpaceResponse> createSpace(
      $grpc.ServiceCall call, $2.CreateSpaceRequest request);

  $async.Future<$2.UpdateSpaceResponse> updateSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$2.UpdateSpaceRequest> $request) async {
    return updateSpace($call, await $request);
  }

  $async.Future<$2.UpdateSpaceResponse> updateSpace(
      $grpc.ServiceCall call, $2.UpdateSpaceRequest request);

  $async.Future<$2.UpdateSpaceMmConfigResponse> updateSpaceMmConfig_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.UpdateSpaceMmConfigRequest> $request) async {
    return updateSpaceMmConfig($call, await $request);
  }

  $async.Future<$2.UpdateSpaceMmConfigResponse> updateSpaceMmConfig(
      $grpc.ServiceCall call, $2.UpdateSpaceMmConfigRequest request);

  $async.Future<$2.DeleteSpaceResponse> deleteSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$2.DeleteSpaceRequest> $request) async {
    return deleteSpace($call, await $request);
  }

  $async.Future<$2.DeleteSpaceResponse> deleteSpace(
      $grpc.ServiceCall call, $2.DeleteSpaceRequest request);

  $async.Future<$2.GetSpaceResponse> getSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$2.GetSpaceRequest> $request) async {
    return getSpace($call, await $request);
  }

  $async.Future<$2.GetSpaceResponse> getSpace(
      $grpc.ServiceCall call, $2.GetSpaceRequest request);

  $async.Future<$2.ValidateRoleOverrideTargetResponse>
      validateRoleOverrideTarget_Pre($grpc.ServiceCall $call,
          $async.Future<$2.ValidateRoleOverrideTargetRequest> $request) async {
    return validateRoleOverrideTarget($call, await $request);
  }

  $async.Future<$2.ValidateRoleOverrideTargetResponse>
      validateRoleOverrideTarget(
          $grpc.ServiceCall call, $2.ValidateRoleOverrideTargetRequest request);

  $async.Future<$2.ListMySpacesResponse> listMySpaces_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.ListMySpacesRequest> $request) async {
    return listMySpaces($call, await $request);
  }

  $async.Future<$2.ListMySpacesResponse> listMySpaces(
      $grpc.ServiceCall call, $2.ListMySpacesRequest request);

  $async.Future<$2.SearchPublicSpacesResponse> searchPublicSpaces_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.SearchPublicSpacesRequest> $request) async {
    return searchPublicSpaces($call, await $request);
  }

  $async.Future<$2.SearchPublicSpacesResponse> searchPublicSpaces(
      $grpc.ServiceCall call, $2.SearchPublicSpacesRequest request);

  $async.Future<$2.CreateVoiceRoomResponse> createVoiceRoom_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.CreateVoiceRoomRequest> $request) async {
    return createVoiceRoom($call, await $request);
  }

  $async.Future<$2.CreateVoiceRoomResponse> createVoiceRoom(
      $grpc.ServiceCall call, $2.CreateVoiceRoomRequest request);

  $async.Future<$2.UpdateVoiceRoomResponse> updateVoiceRoom_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.UpdateVoiceRoomRequest> $request) async {
    return updateVoiceRoom($call, await $request);
  }

  $async.Future<$2.UpdateVoiceRoomResponse> updateVoiceRoom(
      $grpc.ServiceCall call, $2.UpdateVoiceRoomRequest request);

  $async.Future<$2.DeleteVoiceRoomResponse> deleteVoiceRoom_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.DeleteVoiceRoomRequest> $request) async {
    return deleteVoiceRoom($call, await $request);
  }

  $async.Future<$2.DeleteVoiceRoomResponse> deleteVoiceRoom(
      $grpc.ServiceCall call, $2.DeleteVoiceRoomRequest request);

  $async.Future<$2.UpsertTreeNodeResponse> upsertTreeNode_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.UpsertTreeNodeRequest> $request) async {
    return upsertTreeNode($call, await $request);
  }

  $async.Future<$2.UpsertTreeNodeResponse> upsertTreeNode(
      $grpc.ServiceCall call, $2.UpsertTreeNodeRequest request);

  $async.Future<$2.RemoveTreeNodeResponse> removeTreeNode_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.RemoveTreeNodeRequest> $request) async {
    return removeTreeNode($call, await $request);
  }

  $async.Future<$2.RemoveTreeNodeResponse> removeTreeNode(
      $grpc.ServiceCall call, $2.RemoveTreeNodeRequest request);

  $async.Future<$2.CreateCategoryResponse> createCategory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.CreateCategoryRequest> $request) async {
    return createCategory($call, await $request);
  }

  $async.Future<$2.CreateCategoryResponse> createCategory(
      $grpc.ServiceCall call, $2.CreateCategoryRequest request);

  $async.Future<$2.UpdateCategoryResponse> updateCategory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.UpdateCategoryRequest> $request) async {
    return updateCategory($call, await $request);
  }

  $async.Future<$2.UpdateCategoryResponse> updateCategory(
      $grpc.ServiceCall call, $2.UpdateCategoryRequest request);

  $async.Future<$2.DeleteCategoryResponse> deleteCategory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.DeleteCategoryRequest> $request) async {
    return deleteCategory($call, await $request);
  }

  $async.Future<$2.DeleteCategoryResponse> deleteCategory(
      $grpc.ServiceCall call, $2.DeleteCategoryRequest request);

  $async.Future<$2.ReorderSpaceTreeResponse> reorderSpaceTree_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.ReorderSpaceTreeRequest> $request) async {
    return reorderSpaceTree($call, await $request);
  }

  $async.Future<$2.ReorderSpaceTreeResponse> reorderSpaceTree(
      $grpc.ServiceCall call, $2.ReorderSpaceTreeRequest request);

  $async.Future<$2.PinTreeNodeResponse> pinTreeNode_Pre($grpc.ServiceCall $call,
      $async.Future<$2.PinTreeNodeRequest> $request) async {
    return pinTreeNode($call, await $request);
  }

  $async.Future<$2.PinTreeNodeResponse> pinTreeNode(
      $grpc.ServiceCall call, $2.PinTreeNodeRequest request);

  $async.Future<$2.UnpinTreeNodeResponse> unpinTreeNode_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.UnpinTreeNodeRequest> $request) async {
    return unpinTreeNode($call, await $request);
  }

  $async.Future<$2.UnpinTreeNodeResponse> unpinTreeNode(
      $grpc.ServiceCall call, $2.UnpinTreeNodeRequest request);

  $async.Future<$2.ListSpaceTreeResponse> listSpaceTree_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.ListSpaceTreeRequest> $request) async {
    return listSpaceTree($call, await $request);
  }

  $async.Future<$2.ListSpaceTreeResponse> listSpaceTree(
      $grpc.ServiceCall call, $2.ListSpaceTreeRequest request);

  $async.Future<$2.CreateInviteResponse> createInvite_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.CreateInviteRequest> $request) async {
    return createInvite($call, await $request);
  }

  $async.Future<$2.CreateInviteResponse> createInvite(
      $grpc.ServiceCall call, $2.CreateInviteRequest request);

  $async.Future<$2.RevokeInviteResponse> revokeInvite_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.RevokeInviteRequest> $request) async {
    return revokeInvite($call, await $request);
  }

  $async.Future<$2.RevokeInviteResponse> revokeInvite(
      $grpc.ServiceCall call, $2.RevokeInviteRequest request);

  $async.Future<$2.GetInviteResponse> getInvite_Pre($grpc.ServiceCall $call,
      $async.Future<$2.GetInviteRequest> $request) async {
    return getInvite($call, await $request);
  }

  $async.Future<$2.GetInviteResponse> getInvite(
      $grpc.ServiceCall call, $2.GetInviteRequest request);

  $async.Future<$2.ListInvitesResponse> listInvites_Pre($grpc.ServiceCall $call,
      $async.Future<$2.ListInvitesRequest> $request) async {
    return listInvites($call, await $request);
  }

  $async.Future<$2.ListInvitesResponse> listInvites(
      $grpc.ServiceCall call, $2.ListInvitesRequest request);

  $async.Future<$2.JoinByInviteResponse> joinByInvite_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.JoinByInviteRequest> $request) async {
    return joinByInvite($call, await $request);
  }

  $async.Future<$2.JoinByInviteResponse> joinByInvite(
      $grpc.ServiceCall call, $2.JoinByInviteRequest request);

  $async.Future<$2.JoinSpaceResponse> joinSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$2.JoinSpaceRequest> $request) async {
    return joinSpace($call, await $request);
  }

  $async.Future<$2.JoinSpaceResponse> joinSpace(
      $grpc.ServiceCall call, $2.JoinSpaceRequest request);

  $async.Future<$2.StartJoinAttemptResponse> startJoinAttempt_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.StartJoinAttemptRequest> $request) async {
    return startJoinAttempt($call, await $request);
  }

  $async.Future<$2.StartJoinAttemptResponse> startJoinAttempt(
      $grpc.ServiceCall call, $2.StartJoinAttemptRequest request);

  $async.Future<$2.GetJoinAttemptResponse> getJoinAttempt_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.GetJoinAttemptRequest> $request) async {
    return getJoinAttempt($call, await $request);
  }

  $async.Future<$2.GetJoinAttemptResponse> getJoinAttempt(
      $grpc.ServiceCall call, $2.GetJoinAttemptRequest request);

  $async.Future<$2.SubmitJoinAttemptResponse> submitJoinAttempt_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.SubmitJoinAttemptRequest> $request) async {
    return submitJoinAttempt($call, await $request);
  }

  $async.Future<$2.SubmitJoinAttemptResponse> submitJoinAttempt(
      $grpc.ServiceCall call, $2.SubmitJoinAttemptRequest request);

  $async.Future<$2.ListJoinRequestsResponse> listJoinRequests_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.ListJoinRequestsRequest> $request) async {
    return listJoinRequests($call, await $request);
  }

  $async.Future<$2.ListJoinRequestsResponse> listJoinRequests(
      $grpc.ServiceCall call, $2.ListJoinRequestsRequest request);

  $async.Future<$2.GetJoinRequestResponse> getJoinRequest_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.GetJoinRequestRequest> $request) async {
    return getJoinRequest($call, await $request);
  }

  $async.Future<$2.GetJoinRequestResponse> getJoinRequest(
      $grpc.ServiceCall call, $2.GetJoinRequestRequest request);

  $async.Future<$2.ApproveJoinRequestResponse> approveJoinRequest_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.ApproveJoinRequestRequest> $request) async {
    return approveJoinRequest($call, await $request);
  }

  $async.Future<$2.ApproveJoinRequestResponse> approveJoinRequest(
      $grpc.ServiceCall call, $2.ApproveJoinRequestRequest request);

  $async.Future<$2.LeaveSpaceResponse> leaveSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$2.LeaveSpaceRequest> $request) async {
    return leaveSpace($call, await $request);
  }

  $async.Future<$2.LeaveSpaceResponse> leaveSpace(
      $grpc.ServiceCall call, $2.LeaveSpaceRequest request);

  $async.Future<$2.KickMemberResponse> kickMember_Pre($grpc.ServiceCall $call,
      $async.Future<$2.KickMemberRequest> $request) async {
    return kickMember($call, await $request);
  }

  $async.Future<$2.KickMemberResponse> kickMember(
      $grpc.ServiceCall call, $2.KickMemberRequest request);

  $async.Future<$2.BanMemberResponse> banMember_Pre($grpc.ServiceCall $call,
      $async.Future<$2.BanMemberRequest> $request) async {
    return banMember($call, await $request);
  }

  $async.Future<$2.BanMemberResponse> banMember(
      $grpc.ServiceCall call, $2.BanMemberRequest request);

  $async.Future<$2.UnbanMemberResponse> unbanMember_Pre($grpc.ServiceCall $call,
      $async.Future<$2.UnbanMemberRequest> $request) async {
    return unbanMember($call, await $request);
  }

  $async.Future<$2.UnbanMemberResponse> unbanMember(
      $grpc.ServiceCall call, $2.UnbanMemberRequest request);

  $async.Future<$2.ListMembersResponse> listMembers_Pre($grpc.ServiceCall $call,
      $async.Future<$2.ListMembersRequest> $request) async {
    return listMembers($call, await $request);
  }

  $async.Future<$2.ListMembersResponse> listMembers(
      $grpc.ServiceCall call, $2.ListMembersRequest request);

  $async.Future<$2.UpdateMemberNicknameResponse> updateMemberNickname_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.UpdateMemberNicknameRequest> $request) async {
    return updateMemberNickname($call, await $request);
  }

  $async.Future<$2.UpdateMemberNicknameResponse> updateMemberNickname(
      $grpc.ServiceCall call, $2.UpdateMemberNicknameRequest request);

  $async.Future<$2.ListBansResponse> listBans_Pre($grpc.ServiceCall $call,
      $async.Future<$2.ListBansRequest> $request) async {
    return listBans($call, await $request);
  }

  $async.Future<$2.ListBansResponse> listBans(
      $grpc.ServiceCall call, $2.ListBansRequest request);

  $async.Future<$2.TimeoutMemberResponse> timeoutMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.TimeoutMemberRequest> $request) async {
    return timeoutMember($call, await $request);
  }

  $async.Future<$2.TimeoutMemberResponse> timeoutMember(
      $grpc.ServiceCall call, $2.TimeoutMemberRequest request);

  $async.Future<$2.RemoveMemberTimeoutResponse> removeMemberTimeout_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.RemoveMemberTimeoutRequest> $request) async {
    return removeMemberTimeout($call, await $request);
  }

  $async.Future<$2.RemoveMemberTimeoutResponse> removeMemberTimeout(
      $grpc.ServiceCall call, $2.RemoveMemberTimeoutRequest request);

  $async.Future<$2.TransferOwnershipResponse> transferOwnership_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.TransferOwnershipRequest> $request) async {
    return transferOwnership($call, await $request);
  }

  $async.Future<$2.TransferOwnershipResponse> transferOwnership(
      $grpc.ServiceCall call, $2.TransferOwnershipRequest request);

  $async.Future<$2.AddBotMemberResponse> addBotMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.AddBotMemberRequest> $request) async {
    return addBotMember($call, await $request);
  }

  $async.Future<$2.AddBotMemberResponse> addBotMember(
      $grpc.ServiceCall call, $2.AddBotMemberRequest request);

  $async.Future<$2.RemoveBotMemberResponse> removeBotMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.RemoveBotMemberRequest> $request) async {
    return removeBotMember($call, await $request);
  }

  $async.Future<$2.RemoveBotMemberResponse> removeBotMember(
      $grpc.ServiceCall call, $2.RemoveBotMemberRequest request);

  $async.Future<$2.ListTemplatesResponse> listTemplates_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.ListTemplatesRequest> $request) async {
    return listTemplates($call, await $request);
  }

  $async.Future<$2.ListTemplatesResponse> listTemplates(
      $grpc.ServiceCall call, $2.ListTemplatesRequest request);

  $async.Future<$2.CreateFromTemplateResponse> createFromTemplate_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.CreateFromTemplateRequest> $request) async {
    return createFromTemplate($call, await $request);
  }

  $async.Future<$2.CreateFromTemplateResponse> createFromTemplate(
      $grpc.ServiceCall call, $2.CreateFromTemplateRequest request);

  $async.Future<$2.ApplyCommunityRosterResponse> applyCommunityRoster_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.ApplyCommunityRosterRequest> $request) async {
    return applyCommunityRoster($call, await $request);
  }

  $async.Future<$2.ApplyCommunityRosterResponse> applyCommunityRoster(
      $grpc.ServiceCall call, $2.ApplyCommunityRosterRequest request);

  $async.Future<$2.CreateCommunityBootstrapResponse>
      createCommunityBootstrap_Pre($grpc.ServiceCall $call,
          $async.Future<$2.CreateCommunityBootstrapRequest> $request) async {
    return createCommunityBootstrap($call, await $request);
  }

  $async.Future<$2.CreateCommunityBootstrapResponse> createCommunityBootstrap(
      $grpc.ServiceCall call, $2.CreateCommunityBootstrapRequest request);

  $async.Future<$2.RecoverCommunityOwnerResponse> recoverCommunityOwner_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.RecoverCommunityOwnerRequest> $request) async {
    return recoverCommunityOwner($call, await $request);
  }

  $async.Future<$2.RecoverCommunityOwnerResponse> recoverCommunityOwner(
      $grpc.ServiceCall call, $2.RecoverCommunityOwnerRequest request);

  $async.Future<$2.GetAuditLogResponse> getAuditLog_Pre($grpc.ServiceCall $call,
      $async.Future<$2.GetAuditLogRequest> $request) async {
    return getAuditLog($call, await $request);
  }

  $async.Future<$2.GetAuditLogResponse> getAuditLog(
      $grpc.ServiceCall call, $2.GetAuditLogRequest request);

  $async.Future<$2.AppendAuditEventResponse> appendAuditEvent_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.AppendAuditEventRequest> $request) async {
    return appendAuditEvent($call, await $request);
  }

  $async.Future<$2.AppendAuditEventResponse> appendAuditEvent(
      $grpc.ServiceCall call, $2.AppendAuditEventRequest request);

  $async.Future<$2.CommitSpaceChatCreationResponse> commitSpaceChatCreation_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.CommitSpaceChatCreationRequest> $request) async {
    return commitSpaceChatCreation($call, await $request);
  }

  $async.Future<$2.CommitSpaceChatCreationResponse> commitSpaceChatCreation(
      $grpc.ServiceCall call, $2.CommitSpaceChatCreationRequest request);

  $async.Future<$2.GetSpaceChatCreationOutcomeResponse>
      getSpaceChatCreationOutcome_Pre($grpc.ServiceCall $call,
          $async.Future<$2.GetSpaceChatCreationOutcomeRequest> $request) async {
    return getSpaceChatCreationOutcome($call, await $request);
  }

  $async.Future<$2.GetSpaceChatCreationOutcomeResponse>
      getSpaceChatCreationOutcome($grpc.ServiceCall call,
          $2.GetSpaceChatCreationOutcomeRequest request);

  $async.Future<$2.AreCoMembersResponse> areCoMembers_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.AreCoMembersRequest> $request) async {
    return areCoMembers($call, await $request);
  }

  $async.Future<$2.AreCoMembersResponse> areCoMembers(
      $grpc.ServiceCall call, $2.AreCoMembersRequest request);

  $async.Future<$2.ResolveVoiceRoomAccessResponse> resolveVoiceRoomAccess_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.ResolveVoiceRoomAccessRequest> $request) async {
    return resolveVoiceRoomAccess($call, await $request);
  }

  $async.Future<$2.ResolveVoiceRoomAccessResponse> resolveVoiceRoomAccess(
      $grpc.ServiceCall call, $2.ResolveVoiceRoomAccessRequest request);

  $async.Future<$2.ResolveMemberCommunicationStateResponse>
      resolveMemberCommunicationState_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$2.ResolveMemberCommunicationStateRequest>
              $request) async {
    return resolveMemberCommunicationState($call, await $request);
  }

  $async.Future<$2.ResolveMemberCommunicationStateResponse>
      resolveMemberCommunicationState($grpc.ServiceCall call,
          $2.ResolveMemberCommunicationStateRequest request);

  $async.Future<$2.SyncSpaceProSubscriptionResponse>
      syncSpaceProSubscription_Pre($grpc.ServiceCall $call,
          $async.Future<$2.SyncSpaceProSubscriptionRequest> $request) async {
    return syncSpaceProSubscription($call, await $request);
  }

  $async.Future<$2.SyncSpaceProSubscriptionResponse> syncSpaceProSubscription(
      $grpc.ServiceCall call, $2.SyncSpaceProSubscriptionRequest request);

  $async.Future<$2.RestoreSpaceResponse> restoreSpace_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$2.RestoreSpaceRequest> $request) async {
    return restoreSpace($call, await $request);
  }

  $async.Future<$2.RestoreSpaceResponse> restoreSpace(
      $grpc.ServiceCall call, $2.RestoreSpaceRequest request);

  $async.Future<$2.GetSpaceDeletionCoordinatorStatusResponse>
      getSpaceDeletionCoordinatorStatus_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$2.GetSpaceDeletionCoordinatorStatusRequest>
              $request) async {
    return getSpaceDeletionCoordinatorStatus($call, await $request);
  }

  $async.Future<$2.GetSpaceDeletionCoordinatorStatusResponse>
      getSpaceDeletionCoordinatorStatus($grpc.ServiceCall call,
          $2.GetSpaceDeletionCoordinatorStatusRequest request);
}
