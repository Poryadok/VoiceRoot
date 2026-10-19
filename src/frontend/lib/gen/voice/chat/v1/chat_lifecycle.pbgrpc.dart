// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/chat_lifecycle.proto.

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

import 'chat_lifecycle.pb.dart' as $0;

export 'chat_lifecycle.pb.dart';

/// Structural lifecycle enrollment only; it never grants recipient authority.
/// Register only on Chat's dedicated Realtime mTLS listener.
@$pb.GrpcServiceName('voice.chat.v1.ChatRealtimeLifecycleService')
class ChatRealtimeLifecycleServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  ChatRealtimeLifecycleServiceClient(super.channel,
      {super.options, super.interceptors});

  /// @voice.security=protected;callers=service:realtime
  $grpc.ResponseFuture<$0.ResolveChatLifecycleResponse> resolveChatLifecycle(
    $0.ResolveChatLifecycleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveChatLifecycle, request, options: options);
  }

  // method descriptors

  static final _$resolveChatLifecycle = $grpc.ClientMethod<
          $0.ResolveChatLifecycleRequest, $0.ResolveChatLifecycleResponse>(
      '/voice.chat.v1.ChatRealtimeLifecycleService/ResolveChatLifecycle',
      ($0.ResolveChatLifecycleRequest value) => value.writeToBuffer(),
      $0.ResolveChatLifecycleResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.chat.v1.ChatRealtimeLifecycleService')
abstract class ChatRealtimeLifecycleServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.chat.v1.ChatRealtimeLifecycleService';

  ChatRealtimeLifecycleServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ResolveChatLifecycleRequest,
            $0.ResolveChatLifecycleResponse>(
        'ResolveChatLifecycle',
        resolveChatLifecycle_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ResolveChatLifecycleRequest.fromBuffer(value),
        ($0.ResolveChatLifecycleResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.ResolveChatLifecycleResponse> resolveChatLifecycle_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ResolveChatLifecycleRequest> $request) async {
    return resolveChatLifecycle($call, await $request);
  }

  $async.Future<$0.ResolveChatLifecycleResponse> resolveChatLifecycle(
      $grpc.ServiceCall call, $0.ResolveChatLifecycleRequest request);
}
