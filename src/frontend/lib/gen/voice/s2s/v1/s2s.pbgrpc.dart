// This is a generated file - do not edit.
//
// Generated from voice/s2s/v1/s2s.proto.

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

import 's2s.pb.dart' as $0;

export 's2s.pb.dart';

/// S2S federation API between a federated node and the master server.
/// The node is the gRPC client; the master is the server.
/// Account UUID fields use account_id (not user_id); see docs/REPOSITORIES.md (Protobuf).
@$pb.GrpcServiceName('voice.s2s.v1.FederationService')
class FederationServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  FederationServiceClient(super.channel, {super.options, super.interceptors});

  /// Persistent bidirectional stream: the node subscribes to events for its hosted spaces.
  /// The node sends EventStreamRequest frames (SubscribeRequest / Heartbeat / Ack).
  /// The master sends EventStreamResponse on each change (buf STANDARD stream frame names).
  $grpc.ResponseStream<$0.EventStreamResponse> eventStream(
    $async.Stream<$0.EventStreamRequest> request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(_$eventStream, request, options: options);
  }

  /// Snapshot sync on reconnect: the node requests current roles and bans for a given space.
  /// Called after EventStream reconnects.
  $grpc.ResponseFuture<$0.SyncSnapshotResponse> syncSnapshot(
    $0.SyncSnapshotRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$syncSnapshot, request, options: options);
  }

  /// The node asks the master to deliver a push notification via FCM/APNs.
  /// Used when the node cannot deliver the push itself (no certificates).
  $grpc.ResponseFuture<$0.NotifyUserResponse> notifyUser(
    $0.NotifyUserRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$notifyUser, request, options: options);
  }

  /// The node verifies a federated auth token for the user.
  /// Called when the user enters a space on the node.
  $grpc.ResponseFuture<$0.AuthenticateUserResponse> authenticateUser(
    $0.AuthenticateUserRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$authenticateUser, request, options: options);
  }

  // method descriptors

  static final _$eventStream =
      $grpc.ClientMethod<$0.EventStreamRequest, $0.EventStreamResponse>(
          '/voice.s2s.v1.FederationService/EventStream',
          ($0.EventStreamRequest value) => value.writeToBuffer(),
          $0.EventStreamResponse.fromBuffer);
  static final _$syncSnapshot =
      $grpc.ClientMethod<$0.SyncSnapshotRequest, $0.SyncSnapshotResponse>(
          '/voice.s2s.v1.FederationService/SyncSnapshot',
          ($0.SyncSnapshotRequest value) => value.writeToBuffer(),
          $0.SyncSnapshotResponse.fromBuffer);
  static final _$notifyUser =
      $grpc.ClientMethod<$0.NotifyUserRequest, $0.NotifyUserResponse>(
          '/voice.s2s.v1.FederationService/NotifyUser',
          ($0.NotifyUserRequest value) => value.writeToBuffer(),
          $0.NotifyUserResponse.fromBuffer);
  static final _$authenticateUser = $grpc.ClientMethod<
          $0.AuthenticateUserRequest, $0.AuthenticateUserResponse>(
      '/voice.s2s.v1.FederationService/AuthenticateUser',
      ($0.AuthenticateUserRequest value) => value.writeToBuffer(),
      $0.AuthenticateUserResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.s2s.v1.FederationService')
abstract class FederationServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.s2s.v1.FederationService';

  FederationServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.EventStreamRequest, $0.EventStreamResponse>(
            'EventStream',
            eventStream,
            true,
            true,
            ($core.List<$core.int> value) =>
                $0.EventStreamRequest.fromBuffer(value),
            ($0.EventStreamResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SyncSnapshotRequest, $0.SyncSnapshotResponse>(
            'SyncSnapshot',
            syncSnapshot_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SyncSnapshotRequest.fromBuffer(value),
            ($0.SyncSnapshotResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.NotifyUserRequest, $0.NotifyUserResponse>(
        'NotifyUser',
        notifyUser_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.NotifyUserRequest.fromBuffer(value),
        ($0.NotifyUserResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AuthenticateUserRequest,
            $0.AuthenticateUserResponse>(
        'AuthenticateUser',
        authenticateUser_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.AuthenticateUserRequest.fromBuffer(value),
        ($0.AuthenticateUserResponse value) => value.writeToBuffer()));
  }

  $async.Stream<$0.EventStreamResponse> eventStream(
      $grpc.ServiceCall call, $async.Stream<$0.EventStreamRequest> request);

  $async.Future<$0.SyncSnapshotResponse> syncSnapshot_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SyncSnapshotRequest> $request) async {
    return syncSnapshot($call, await $request);
  }

  $async.Future<$0.SyncSnapshotResponse> syncSnapshot(
      $grpc.ServiceCall call, $0.SyncSnapshotRequest request);

  $async.Future<$0.NotifyUserResponse> notifyUser_Pre($grpc.ServiceCall $call,
      $async.Future<$0.NotifyUserRequest> $request) async {
    return notifyUser($call, await $request);
  }

  $async.Future<$0.NotifyUserResponse> notifyUser(
      $grpc.ServiceCall call, $0.NotifyUserRequest request);

  $async.Future<$0.AuthenticateUserResponse> authenticateUser_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.AuthenticateUserRequest> $request) async {
    return authenticateUser($call, await $request);
  }

  $async.Future<$0.AuthenticateUserResponse> authenticateUser(
      $grpc.ServiceCall call, $0.AuthenticateUserRequest request);
}
