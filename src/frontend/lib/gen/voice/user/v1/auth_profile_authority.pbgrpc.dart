// This is a generated file - do not edit.
//
// Generated from voice/user/v1/auth_profile_authority.proto.

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

import 'auth_profile_authority.pb.dart' as $0;

export 'auth_profile_authority.pb.dart';

/// Only the separate auth-profile-authority capability exposes these methods.
/// Neither response authorizes independent profile admission or raw enumeration.
@$pb.GrpcServiceName('voice.user.v1.AuthProfileAuthorityService')
class AuthProfileAuthorityServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  AuthProfileAuthorityServiceClient(super.channel,
      {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.MatchAuthProfileReferencesResponse>
      matchAuthProfileReferences(
    $0.MatchAuthProfileReferencesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$matchAuthProfileReferences, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.ResolveAuthOwnershipTargetResponse>
      resolveAuthOwnershipTarget(
    $0.ResolveAuthOwnershipTargetRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveAuthOwnershipTarget, request,
        options: options);
  }

  // method descriptors

  static final _$matchAuthProfileReferences = $grpc.ClientMethod<
          $0.MatchAuthProfileReferencesRequest,
          $0.MatchAuthProfileReferencesResponse>(
      '/voice.user.v1.AuthProfileAuthorityService/MatchAuthProfileReferences',
      ($0.MatchAuthProfileReferencesRequest value) => value.writeToBuffer(),
      $0.MatchAuthProfileReferencesResponse.fromBuffer);
  static final _$resolveAuthOwnershipTarget = $grpc.ClientMethod<
          $0.ResolveAuthOwnershipTargetRequest,
          $0.ResolveAuthOwnershipTargetResponse>(
      '/voice.user.v1.AuthProfileAuthorityService/ResolveAuthOwnershipTarget',
      ($0.ResolveAuthOwnershipTargetRequest value) => value.writeToBuffer(),
      $0.ResolveAuthOwnershipTargetResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.user.v1.AuthProfileAuthorityService')
abstract class AuthProfileAuthorityServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.user.v1.AuthProfileAuthorityService';

  AuthProfileAuthorityServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.MatchAuthProfileReferencesRequest,
            $0.MatchAuthProfileReferencesResponse>(
        'MatchAuthProfileReferences',
        matchAuthProfileReferences_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.MatchAuthProfileReferencesRequest.fromBuffer(value),
        ($0.MatchAuthProfileReferencesResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ResolveAuthOwnershipTargetRequest,
            $0.ResolveAuthOwnershipTargetResponse>(
        'ResolveAuthOwnershipTarget',
        resolveAuthOwnershipTarget_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ResolveAuthOwnershipTargetRequest.fromBuffer(value),
        ($0.ResolveAuthOwnershipTargetResponse value) =>
            value.writeToBuffer()));
  }

  $async.Future<$0.MatchAuthProfileReferencesResponse>
      matchAuthProfileReferences_Pre($grpc.ServiceCall $call,
          $async.Future<$0.MatchAuthProfileReferencesRequest> $request) async {
    return matchAuthProfileReferences($call, await $request);
  }

  $async.Future<$0.MatchAuthProfileReferencesResponse>
      matchAuthProfileReferences(
          $grpc.ServiceCall call, $0.MatchAuthProfileReferencesRequest request);

  $async.Future<$0.ResolveAuthOwnershipTargetResponse>
      resolveAuthOwnershipTarget_Pre($grpc.ServiceCall $call,
          $async.Future<$0.ResolveAuthOwnershipTargetRequest> $request) async {
    return resolveAuthOwnershipTarget($call, await $request);
  }

  $async.Future<$0.ResolveAuthOwnershipTargetResponse>
      resolveAuthOwnershipTarget(
          $grpc.ServiceCall call, $0.ResolveAuthOwnershipTargetRequest request);
}
