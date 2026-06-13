// This is a generated file - do not edit.
//
// Generated from voice/role/v1/role.proto.

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

import 'role.pb.dart' as $0;

export 'role.pb.dart';

/// Roles and permission bitmask in space. HTTP: /api/v1/roles/**.
/// Permission names: docs/microservices/role-service.md (SCREAMING_SNAKE_CASE).
@$pb.GrpcServiceName('voice.role.v1.RoleService')
class RoleServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  RoleServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.CreateRoleResponse> createRole(
    $0.CreateRoleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createRole, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdateRoleResponse> updateRole(
    $0.UpdateRoleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateRole, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteRoleResponse> deleteRole(
    $0.DeleteRoleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteRole, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListRolesResponse> listRoles(
    $0.ListRolesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listRoles, request, options: options);
  }

  $grpc.ResponseFuture<$0.ReorderRolesResponse> reorderRoles(
    $0.ReorderRolesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$reorderRoles, request, options: options);
  }

  $grpc.ResponseFuture<$0.AssignRoleResponse> assignRole(
    $0.AssignRoleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$assignRole, request, options: options);
  }

  $grpc.ResponseFuture<$0.RevokeRoleResponse> revokeRole(
    $0.RevokeRoleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$revokeRole, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetMemberRolesResponse> getMemberRoles(
    $0.GetMemberRolesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getMemberRoles, request, options: options);
  }

  $grpc.ResponseFuture<$0.SetChatOverrideResponse> setChatOverride(
    $0.SetChatOverrideRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setChatOverride, request, options: options);
  }

  $grpc.ResponseFuture<$0.RemoveChatOverrideResponse> removeChatOverride(
    $0.RemoveChatOverrideRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeChatOverride, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetChatOverridesResponse> getChatOverrides(
    $0.GetChatOverridesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getChatOverrides, request, options: options);
  }

  $grpc.ResponseFuture<$0.SetVoiceRoomOverrideResponse> setVoiceRoomOverride(
    $0.SetVoiceRoomOverrideRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setVoiceRoomOverride, request, options: options);
  }

  $grpc.ResponseFuture<$0.RemoveVoiceRoomOverrideResponse>
      removeVoiceRoomOverride(
    $0.RemoveVoiceRoomOverrideRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeVoiceRoomOverride, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.GetVoiceRoomOverridesResponse> getVoiceRoomOverrides(
    $0.GetVoiceRoomOverridesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getVoiceRoomOverrides, request, options: options);
  }

  $grpc.ResponseFuture<$0.CheckPermissionResponse> checkPermission(
    $0.CheckPermissionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$checkPermission, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetEffectivePermissionsResponse>
      getEffectivePermissions(
    $0.GetEffectivePermissionsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getEffectivePermissions, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.SetDefaultJoinRoleResponse> setDefaultJoinRole(
    $0.SetDefaultJoinRoleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setDefaultJoinRole, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetDefaultJoinRoleResponse> getDefaultJoinRole(
    $0.GetDefaultJoinRoleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getDefaultJoinRole, request, options: options);
  }

  /// Called by Space Service after CreateSpace — seeds system roles and assigns Owner.
  $grpc.ResponseFuture<$0.BootstrapSpaceRolesResponse> bootstrapSpaceRoles(
    $0.BootstrapSpaceRolesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$bootstrapSpaceRoles, request, options: options);
  }

  // method descriptors

  static final _$createRole =
      $grpc.ClientMethod<$0.CreateRoleRequest, $0.CreateRoleResponse>(
          '/voice.role.v1.RoleService/CreateRole',
          ($0.CreateRoleRequest value) => value.writeToBuffer(),
          $0.CreateRoleResponse.fromBuffer);
  static final _$updateRole =
      $grpc.ClientMethod<$0.UpdateRoleRequest, $0.UpdateRoleResponse>(
          '/voice.role.v1.RoleService/UpdateRole',
          ($0.UpdateRoleRequest value) => value.writeToBuffer(),
          $0.UpdateRoleResponse.fromBuffer);
  static final _$deleteRole =
      $grpc.ClientMethod<$0.DeleteRoleRequest, $0.DeleteRoleResponse>(
          '/voice.role.v1.RoleService/DeleteRole',
          ($0.DeleteRoleRequest value) => value.writeToBuffer(),
          $0.DeleteRoleResponse.fromBuffer);
  static final _$listRoles =
      $grpc.ClientMethod<$0.ListRolesRequest, $0.ListRolesResponse>(
          '/voice.role.v1.RoleService/ListRoles',
          ($0.ListRolesRequest value) => value.writeToBuffer(),
          $0.ListRolesResponse.fromBuffer);
  static final _$reorderRoles =
      $grpc.ClientMethod<$0.ReorderRolesRequest, $0.ReorderRolesResponse>(
          '/voice.role.v1.RoleService/ReorderRoles',
          ($0.ReorderRolesRequest value) => value.writeToBuffer(),
          $0.ReorderRolesResponse.fromBuffer);
  static final _$assignRole =
      $grpc.ClientMethod<$0.AssignRoleRequest, $0.AssignRoleResponse>(
          '/voice.role.v1.RoleService/AssignRole',
          ($0.AssignRoleRequest value) => value.writeToBuffer(),
          $0.AssignRoleResponse.fromBuffer);
  static final _$revokeRole =
      $grpc.ClientMethod<$0.RevokeRoleRequest, $0.RevokeRoleResponse>(
          '/voice.role.v1.RoleService/RevokeRole',
          ($0.RevokeRoleRequest value) => value.writeToBuffer(),
          $0.RevokeRoleResponse.fromBuffer);
  static final _$getMemberRoles =
      $grpc.ClientMethod<$0.GetMemberRolesRequest, $0.GetMemberRolesResponse>(
          '/voice.role.v1.RoleService/GetMemberRoles',
          ($0.GetMemberRolesRequest value) => value.writeToBuffer(),
          $0.GetMemberRolesResponse.fromBuffer);
  static final _$setChatOverride =
      $grpc.ClientMethod<$0.SetChatOverrideRequest, $0.SetChatOverrideResponse>(
          '/voice.role.v1.RoleService/SetChatOverride',
          ($0.SetChatOverrideRequest value) => value.writeToBuffer(),
          $0.SetChatOverrideResponse.fromBuffer);
  static final _$removeChatOverride = $grpc.ClientMethod<
          $0.RemoveChatOverrideRequest, $0.RemoveChatOverrideResponse>(
      '/voice.role.v1.RoleService/RemoveChatOverride',
      ($0.RemoveChatOverrideRequest value) => value.writeToBuffer(),
      $0.RemoveChatOverrideResponse.fromBuffer);
  static final _$getChatOverrides = $grpc.ClientMethod<
          $0.GetChatOverridesRequest, $0.GetChatOverridesResponse>(
      '/voice.role.v1.RoleService/GetChatOverrides',
      ($0.GetChatOverridesRequest value) => value.writeToBuffer(),
      $0.GetChatOverridesResponse.fromBuffer);
  static final _$setVoiceRoomOverride = $grpc.ClientMethod<
          $0.SetVoiceRoomOverrideRequest, $0.SetVoiceRoomOverrideResponse>(
      '/voice.role.v1.RoleService/SetVoiceRoomOverride',
      ($0.SetVoiceRoomOverrideRequest value) => value.writeToBuffer(),
      $0.SetVoiceRoomOverrideResponse.fromBuffer);
  static final _$removeVoiceRoomOverride = $grpc.ClientMethod<
          $0.RemoveVoiceRoomOverrideRequest,
          $0.RemoveVoiceRoomOverrideResponse>(
      '/voice.role.v1.RoleService/RemoveVoiceRoomOverride',
      ($0.RemoveVoiceRoomOverrideRequest value) => value.writeToBuffer(),
      $0.RemoveVoiceRoomOverrideResponse.fromBuffer);
  static final _$getVoiceRoomOverrides = $grpc.ClientMethod<
          $0.GetVoiceRoomOverridesRequest, $0.GetVoiceRoomOverridesResponse>(
      '/voice.role.v1.RoleService/GetVoiceRoomOverrides',
      ($0.GetVoiceRoomOverridesRequest value) => value.writeToBuffer(),
      $0.GetVoiceRoomOverridesResponse.fromBuffer);
  static final _$checkPermission =
      $grpc.ClientMethod<$0.CheckPermissionRequest, $0.CheckPermissionResponse>(
          '/voice.role.v1.RoleService/CheckPermission',
          ($0.CheckPermissionRequest value) => value.writeToBuffer(),
          $0.CheckPermissionResponse.fromBuffer);
  static final _$getEffectivePermissions = $grpc.ClientMethod<
          $0.GetEffectivePermissionsRequest,
          $0.GetEffectivePermissionsResponse>(
      '/voice.role.v1.RoleService/GetEffectivePermissions',
      ($0.GetEffectivePermissionsRequest value) => value.writeToBuffer(),
      $0.GetEffectivePermissionsResponse.fromBuffer);
  static final _$setDefaultJoinRole = $grpc.ClientMethod<
          $0.SetDefaultJoinRoleRequest, $0.SetDefaultJoinRoleResponse>(
      '/voice.role.v1.RoleService/SetDefaultJoinRole',
      ($0.SetDefaultJoinRoleRequest value) => value.writeToBuffer(),
      $0.SetDefaultJoinRoleResponse.fromBuffer);
  static final _$getDefaultJoinRole = $grpc.ClientMethod<
          $0.GetDefaultJoinRoleRequest, $0.GetDefaultJoinRoleResponse>(
      '/voice.role.v1.RoleService/GetDefaultJoinRole',
      ($0.GetDefaultJoinRoleRequest value) => value.writeToBuffer(),
      $0.GetDefaultJoinRoleResponse.fromBuffer);
  static final _$bootstrapSpaceRoles = $grpc.ClientMethod<
          $0.BootstrapSpaceRolesRequest, $0.BootstrapSpaceRolesResponse>(
      '/voice.role.v1.RoleService/BootstrapSpaceRoles',
      ($0.BootstrapSpaceRolesRequest value) => value.writeToBuffer(),
      $0.BootstrapSpaceRolesResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.role.v1.RoleService')
abstract class RoleServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.role.v1.RoleService';

  RoleServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.CreateRoleRequest, $0.CreateRoleResponse>(
        'CreateRole',
        createRole_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.CreateRoleRequest.fromBuffer(value),
        ($0.CreateRoleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpdateRoleRequest, $0.UpdateRoleResponse>(
        'UpdateRole',
        updateRole_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.UpdateRoleRequest.fromBuffer(value),
        ($0.UpdateRoleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeleteRoleRequest, $0.DeleteRoleResponse>(
        'DeleteRole',
        deleteRole_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.DeleteRoleRequest.fromBuffer(value),
        ($0.DeleteRoleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListRolesRequest, $0.ListRolesResponse>(
        'ListRoles',
        listRoles_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ListRolesRequest.fromBuffer(value),
        ($0.ListRolesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ReorderRolesRequest, $0.ReorderRolesResponse>(
            'ReorderRoles',
            reorderRoles_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ReorderRolesRequest.fromBuffer(value),
            ($0.ReorderRolesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AssignRoleRequest, $0.AssignRoleResponse>(
        'AssignRole',
        assignRole_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.AssignRoleRequest.fromBuffer(value),
        ($0.AssignRoleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RevokeRoleRequest, $0.RevokeRoleResponse>(
        'RevokeRole',
        revokeRole_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.RevokeRoleRequest.fromBuffer(value),
        ($0.RevokeRoleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetMemberRolesRequest,
            $0.GetMemberRolesResponse>(
        'GetMemberRoles',
        getMemberRoles_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetMemberRolesRequest.fromBuffer(value),
        ($0.GetMemberRolesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SetChatOverrideRequest,
            $0.SetChatOverrideResponse>(
        'SetChatOverride',
        setChatOverride_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SetChatOverrideRequest.fromBuffer(value),
        ($0.SetChatOverrideResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RemoveChatOverrideRequest,
            $0.RemoveChatOverrideResponse>(
        'RemoveChatOverride',
        removeChatOverride_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RemoveChatOverrideRequest.fromBuffer(value),
        ($0.RemoveChatOverrideResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetChatOverridesRequest,
            $0.GetChatOverridesResponse>(
        'GetChatOverrides',
        getChatOverrides_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetChatOverridesRequest.fromBuffer(value),
        ($0.GetChatOverridesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SetVoiceRoomOverrideRequest,
            $0.SetVoiceRoomOverrideResponse>(
        'SetVoiceRoomOverride',
        setVoiceRoomOverride_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SetVoiceRoomOverrideRequest.fromBuffer(value),
        ($0.SetVoiceRoomOverrideResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RemoveVoiceRoomOverrideRequest,
            $0.RemoveVoiceRoomOverrideResponse>(
        'RemoveVoiceRoomOverride',
        removeVoiceRoomOverride_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RemoveVoiceRoomOverrideRequest.fromBuffer(value),
        ($0.RemoveVoiceRoomOverrideResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetVoiceRoomOverridesRequest,
            $0.GetVoiceRoomOverridesResponse>(
        'GetVoiceRoomOverrides',
        getVoiceRoomOverrides_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetVoiceRoomOverridesRequest.fromBuffer(value),
        ($0.GetVoiceRoomOverridesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CheckPermissionRequest,
            $0.CheckPermissionResponse>(
        'CheckPermission',
        checkPermission_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CheckPermissionRequest.fromBuffer(value),
        ($0.CheckPermissionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetEffectivePermissionsRequest,
            $0.GetEffectivePermissionsResponse>(
        'GetEffectivePermissions',
        getEffectivePermissions_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetEffectivePermissionsRequest.fromBuffer(value),
        ($0.GetEffectivePermissionsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SetDefaultJoinRoleRequest,
            $0.SetDefaultJoinRoleResponse>(
        'SetDefaultJoinRole',
        setDefaultJoinRole_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SetDefaultJoinRoleRequest.fromBuffer(value),
        ($0.SetDefaultJoinRoleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetDefaultJoinRoleRequest,
            $0.GetDefaultJoinRoleResponse>(
        'GetDefaultJoinRole',
        getDefaultJoinRole_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetDefaultJoinRoleRequest.fromBuffer(value),
        ($0.GetDefaultJoinRoleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.BootstrapSpaceRolesRequest,
            $0.BootstrapSpaceRolesResponse>(
        'BootstrapSpaceRoles',
        bootstrapSpaceRoles_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.BootstrapSpaceRolesRequest.fromBuffer(value),
        ($0.BootstrapSpaceRolesResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.CreateRoleResponse> createRole_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CreateRoleRequest> $request) async {
    return createRole($call, await $request);
  }

  $async.Future<$0.CreateRoleResponse> createRole(
      $grpc.ServiceCall call, $0.CreateRoleRequest request);

  $async.Future<$0.UpdateRoleResponse> updateRole_Pre($grpc.ServiceCall $call,
      $async.Future<$0.UpdateRoleRequest> $request) async {
    return updateRole($call, await $request);
  }

  $async.Future<$0.UpdateRoleResponse> updateRole(
      $grpc.ServiceCall call, $0.UpdateRoleRequest request);

  $async.Future<$0.DeleteRoleResponse> deleteRole_Pre($grpc.ServiceCall $call,
      $async.Future<$0.DeleteRoleRequest> $request) async {
    return deleteRole($call, await $request);
  }

  $async.Future<$0.DeleteRoleResponse> deleteRole(
      $grpc.ServiceCall call, $0.DeleteRoleRequest request);

  $async.Future<$0.ListRolesResponse> listRoles_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListRolesRequest> $request) async {
    return listRoles($call, await $request);
  }

  $async.Future<$0.ListRolesResponse> listRoles(
      $grpc.ServiceCall call, $0.ListRolesRequest request);

  $async.Future<$0.ReorderRolesResponse> reorderRoles_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ReorderRolesRequest> $request) async {
    return reorderRoles($call, await $request);
  }

  $async.Future<$0.ReorderRolesResponse> reorderRoles(
      $grpc.ServiceCall call, $0.ReorderRolesRequest request);

  $async.Future<$0.AssignRoleResponse> assignRole_Pre($grpc.ServiceCall $call,
      $async.Future<$0.AssignRoleRequest> $request) async {
    return assignRole($call, await $request);
  }

  $async.Future<$0.AssignRoleResponse> assignRole(
      $grpc.ServiceCall call, $0.AssignRoleRequest request);

  $async.Future<$0.RevokeRoleResponse> revokeRole_Pre($grpc.ServiceCall $call,
      $async.Future<$0.RevokeRoleRequest> $request) async {
    return revokeRole($call, await $request);
  }

  $async.Future<$0.RevokeRoleResponse> revokeRole(
      $grpc.ServiceCall call, $0.RevokeRoleRequest request);

  $async.Future<$0.GetMemberRolesResponse> getMemberRoles_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetMemberRolesRequest> $request) async {
    return getMemberRoles($call, await $request);
  }

  $async.Future<$0.GetMemberRolesResponse> getMemberRoles(
      $grpc.ServiceCall call, $0.GetMemberRolesRequest request);

  $async.Future<$0.SetChatOverrideResponse> setChatOverride_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetChatOverrideRequest> $request) async {
    return setChatOverride($call, await $request);
  }

  $async.Future<$0.SetChatOverrideResponse> setChatOverride(
      $grpc.ServiceCall call, $0.SetChatOverrideRequest request);

  $async.Future<$0.RemoveChatOverrideResponse> removeChatOverride_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RemoveChatOverrideRequest> $request) async {
    return removeChatOverride($call, await $request);
  }

  $async.Future<$0.RemoveChatOverrideResponse> removeChatOverride(
      $grpc.ServiceCall call, $0.RemoveChatOverrideRequest request);

  $async.Future<$0.GetChatOverridesResponse> getChatOverrides_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetChatOverridesRequest> $request) async {
    return getChatOverrides($call, await $request);
  }

  $async.Future<$0.GetChatOverridesResponse> getChatOverrides(
      $grpc.ServiceCall call, $0.GetChatOverridesRequest request);

  $async.Future<$0.SetVoiceRoomOverrideResponse> setVoiceRoomOverride_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetVoiceRoomOverrideRequest> $request) async {
    return setVoiceRoomOverride($call, await $request);
  }

  $async.Future<$0.SetVoiceRoomOverrideResponse> setVoiceRoomOverride(
      $grpc.ServiceCall call, $0.SetVoiceRoomOverrideRequest request);

  $async.Future<$0.RemoveVoiceRoomOverrideResponse> removeVoiceRoomOverride_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RemoveVoiceRoomOverrideRequest> $request) async {
    return removeVoiceRoomOverride($call, await $request);
  }

  $async.Future<$0.RemoveVoiceRoomOverrideResponse> removeVoiceRoomOverride(
      $grpc.ServiceCall call, $0.RemoveVoiceRoomOverrideRequest request);

  $async.Future<$0.GetVoiceRoomOverridesResponse> getVoiceRoomOverrides_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetVoiceRoomOverridesRequest> $request) async {
    return getVoiceRoomOverrides($call, await $request);
  }

  $async.Future<$0.GetVoiceRoomOverridesResponse> getVoiceRoomOverrides(
      $grpc.ServiceCall call, $0.GetVoiceRoomOverridesRequest request);

  $async.Future<$0.CheckPermissionResponse> checkPermission_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CheckPermissionRequest> $request) async {
    return checkPermission($call, await $request);
  }

  $async.Future<$0.CheckPermissionResponse> checkPermission(
      $grpc.ServiceCall call, $0.CheckPermissionRequest request);

  $async.Future<$0.GetEffectivePermissionsResponse> getEffectivePermissions_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetEffectivePermissionsRequest> $request) async {
    return getEffectivePermissions($call, await $request);
  }

  $async.Future<$0.GetEffectivePermissionsResponse> getEffectivePermissions(
      $grpc.ServiceCall call, $0.GetEffectivePermissionsRequest request);

  $async.Future<$0.SetDefaultJoinRoleResponse> setDefaultJoinRole_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetDefaultJoinRoleRequest> $request) async {
    return setDefaultJoinRole($call, await $request);
  }

  $async.Future<$0.SetDefaultJoinRoleResponse> setDefaultJoinRole(
      $grpc.ServiceCall call, $0.SetDefaultJoinRoleRequest request);

  $async.Future<$0.GetDefaultJoinRoleResponse> getDefaultJoinRole_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetDefaultJoinRoleRequest> $request) async {
    return getDefaultJoinRole($call, await $request);
  }

  $async.Future<$0.GetDefaultJoinRoleResponse> getDefaultJoinRole(
      $grpc.ServiceCall call, $0.GetDefaultJoinRoleRequest request);

  $async.Future<$0.BootstrapSpaceRolesResponse> bootstrapSpaceRoles_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.BootstrapSpaceRolesRequest> $request) async {
    return bootstrapSpaceRoles($call, await $request);
  }

  $async.Future<$0.BootstrapSpaceRolesResponse> bootstrapSpaceRoles(
      $grpc.ServiceCall call, $0.BootstrapSpaceRolesRequest request);
}
