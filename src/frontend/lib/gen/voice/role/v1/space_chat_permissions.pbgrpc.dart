// This is a generated file - do not edit.
//
// Generated from voice/role/v1/space_chat_permissions.proto.

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

import 'space_chat_permissions.pb.dart' as $0;

export 'space_chat_permissions.pb.dart';

/// Dedicated capabilities at an exact Space-chat attachment. These decisions do
/// not grant visibility, ordinary sending or the independent invite permission.
@$pb.GrpcServiceName('voice.role.v1.RoleSpaceChatService')
class RoleSpaceChatServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  RoleSpaceChatServiceClient(super.channel,
      {super.options, super.interceptors});

  /// Protected service callers are exactly Chat, Messaging and Space. Gateway
  /// delegated users may query self; a manager target view also requires current
  /// qualified canonical Owner/Admin and ordinary roster visibility.
  $grpc.ResponseFuture<$0.GetChatPermissionStateResponse>
      getChatPermissionState(
    $0.GetChatPermissionStateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getChatPermissionState, request,
        options: options);
  }

  /// Gateway delegated user only. Actor comes exclusively from verified claims.
  $grpc.ResponseFuture<$0.SetSpaceChatAddMembersResponse>
      setSpaceChatAddMembers(
    $0.SetSpaceChatAddMembersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setSpaceChatAddMembers, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.SetSpaceChatPostAsChatGrantResponse>
      setSpaceChatPostAsChatGrant(
    $0.SetSpaceChatPostAsChatGrantRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setSpaceChatPostAsChatGrant, request,
        options: options);
  }

  // method descriptors

  static final _$getChatPermissionState = $grpc.ClientMethod<
          $0.GetChatPermissionStateRequest, $0.GetChatPermissionStateResponse>(
      '/voice.role.v1.RoleSpaceChatService/GetChatPermissionState',
      ($0.GetChatPermissionStateRequest value) => value.writeToBuffer(),
      $0.GetChatPermissionStateResponse.fromBuffer);
  static final _$setSpaceChatAddMembers = $grpc.ClientMethod<
          $0.SetSpaceChatAddMembersRequest, $0.SetSpaceChatAddMembersResponse>(
      '/voice.role.v1.RoleSpaceChatService/SetSpaceChatAddMembers',
      ($0.SetSpaceChatAddMembersRequest value) => value.writeToBuffer(),
      $0.SetSpaceChatAddMembersResponse.fromBuffer);
  static final _$setSpaceChatPostAsChatGrant = $grpc.ClientMethod<
          $0.SetSpaceChatPostAsChatGrantRequest,
          $0.SetSpaceChatPostAsChatGrantResponse>(
      '/voice.role.v1.RoleSpaceChatService/SetSpaceChatPostAsChatGrant',
      ($0.SetSpaceChatPostAsChatGrantRequest value) => value.writeToBuffer(),
      $0.SetSpaceChatPostAsChatGrantResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.role.v1.RoleSpaceChatService')
abstract class RoleSpaceChatServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.role.v1.RoleSpaceChatService';

  RoleSpaceChatServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.GetChatPermissionStateRequest,
            $0.GetChatPermissionStateResponse>(
        'GetChatPermissionState',
        getChatPermissionState_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetChatPermissionStateRequest.fromBuffer(value),
        ($0.GetChatPermissionStateResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SetSpaceChatAddMembersRequest,
            $0.SetSpaceChatAddMembersResponse>(
        'SetSpaceChatAddMembers',
        setSpaceChatAddMembers_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SetSpaceChatAddMembersRequest.fromBuffer(value),
        ($0.SetSpaceChatAddMembersResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SetSpaceChatPostAsChatGrantRequest,
            $0.SetSpaceChatPostAsChatGrantResponse>(
        'SetSpaceChatPostAsChatGrant',
        setSpaceChatPostAsChatGrant_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SetSpaceChatPostAsChatGrantRequest.fromBuffer(value),
        ($0.SetSpaceChatPostAsChatGrantResponse value) =>
            value.writeToBuffer()));
  }

  $async.Future<$0.GetChatPermissionStateResponse> getChatPermissionState_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetChatPermissionStateRequest> $request) async {
    return getChatPermissionState($call, await $request);
  }

  $async.Future<$0.GetChatPermissionStateResponse> getChatPermissionState(
      $grpc.ServiceCall call, $0.GetChatPermissionStateRequest request);

  $async.Future<$0.SetSpaceChatAddMembersResponse> setSpaceChatAddMembers_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetSpaceChatAddMembersRequest> $request) async {
    return setSpaceChatAddMembers($call, await $request);
  }

  $async.Future<$0.SetSpaceChatAddMembersResponse> setSpaceChatAddMembers(
      $grpc.ServiceCall call, $0.SetSpaceChatAddMembersRequest request);

  $async.Future<$0.SetSpaceChatPostAsChatGrantResponse>
      setSpaceChatPostAsChatGrant_Pre($grpc.ServiceCall $call,
          $async.Future<$0.SetSpaceChatPostAsChatGrantRequest> $request) async {
    return setSpaceChatPostAsChatGrant($call, await $request);
  }

  $async.Future<$0.SetSpaceChatPostAsChatGrantResponse>
      setSpaceChatPostAsChatGrant($grpc.ServiceCall call,
          $0.SetSpaceChatPostAsChatGrantRequest request);
}
