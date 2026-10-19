// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/chat_scope_authority.proto.

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

import 'chat_scope_authority.pb.dart' as $0;

export 'chat_scope_authority.pb.dart';

/// Narrow own-DB resource facts. This service grants no user permissions and is
/// served only on caller-specific signed mTLS listeners.
@$pb.GrpcServiceName('voice.chat.v1.ChatScopeAuthorityService')
class ChatScopeAuthorityServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  ChatScopeAuthorityServiceClient(super.channel,
      {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.GetSpaceChatScopeAuthorityResponse>
      getSpaceChatScopeAuthority(
    $0.GetSpaceChatScopeAuthorityRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSpaceChatScopeAuthority, request,
        options: options);
  }

  /// Bot-only source observation, including qualified retained retired heads.
  /// This read grants no membership or cleanup completion authority.
  $grpc.ResponseFuture<$0.GetBotChatScopeAuthorityResponse>
      getBotChatScopeAuthority(
    $0.GetBotChatScopeAuthorityRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getBotChatScopeAuthority, request,
        options: options);
  }

  // method descriptors

  static final _$getSpaceChatScopeAuthority = $grpc.ClientMethod<
          $0.GetSpaceChatScopeAuthorityRequest,
          $0.GetSpaceChatScopeAuthorityResponse>(
      '/voice.chat.v1.ChatScopeAuthorityService/GetSpaceChatScopeAuthority',
      ($0.GetSpaceChatScopeAuthorityRequest value) => value.writeToBuffer(),
      $0.GetSpaceChatScopeAuthorityResponse.fromBuffer);
  static final _$getBotChatScopeAuthority = $grpc.ClientMethod<
          $0.GetBotChatScopeAuthorityRequest,
          $0.GetBotChatScopeAuthorityResponse>(
      '/voice.chat.v1.ChatScopeAuthorityService/GetBotChatScopeAuthority',
      ($0.GetBotChatScopeAuthorityRequest value) => value.writeToBuffer(),
      $0.GetBotChatScopeAuthorityResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.chat.v1.ChatScopeAuthorityService')
abstract class ChatScopeAuthorityServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.chat.v1.ChatScopeAuthorityService';

  ChatScopeAuthorityServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.GetSpaceChatScopeAuthorityRequest,
            $0.GetSpaceChatScopeAuthorityResponse>(
        'GetSpaceChatScopeAuthority',
        getSpaceChatScopeAuthority_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetSpaceChatScopeAuthorityRequest.fromBuffer(value),
        ($0.GetSpaceChatScopeAuthorityResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetBotChatScopeAuthorityRequest,
            $0.GetBotChatScopeAuthorityResponse>(
        'GetBotChatScopeAuthority',
        getBotChatScopeAuthority_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetBotChatScopeAuthorityRequest.fromBuffer(value),
        ($0.GetBotChatScopeAuthorityResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.GetSpaceChatScopeAuthorityResponse>
      getSpaceChatScopeAuthority_Pre($grpc.ServiceCall $call,
          $async.Future<$0.GetSpaceChatScopeAuthorityRequest> $request) async {
    return getSpaceChatScopeAuthority($call, await $request);
  }

  $async.Future<$0.GetSpaceChatScopeAuthorityResponse>
      getSpaceChatScopeAuthority(
          $grpc.ServiceCall call, $0.GetSpaceChatScopeAuthorityRequest request);

  $async.Future<$0.GetBotChatScopeAuthorityResponse>
      getBotChatScopeAuthority_Pre($grpc.ServiceCall $call,
          $async.Future<$0.GetBotChatScopeAuthorityRequest> $request) async {
    return getBotChatScopeAuthority($call, await $request);
  }

  $async.Future<$0.GetBotChatScopeAuthorityResponse> getBotChatScopeAuthority(
      $grpc.ServiceCall call, $0.GetBotChatScopeAuthorityRequest request);
}
