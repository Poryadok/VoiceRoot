// This is a generated file - do not edit.
//
// Generated from voice/identityerasure/v1/identity_erasure.proto.

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

import 'identity_erasure.pb.dart' as $0;

export 'identity_erasure.pb.dart';

/// Private Role-purpose authority. No ordinary Auth/User listener exposes it.
@$pb.GrpcServiceName(
    'voice.identityerasure.v1.AuthRoleIdentityAuthorityService')
class AuthRoleIdentityAuthorityServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  AuthRoleIdentityAuthorityServiceClient(super.channel,
      {super.options, super.interceptors});

  /// Only the verified service:role principal may call these methods.
  $grpc.ResponseFuture<$0.ObserveAccountsResponse> observeAccounts(
    $0.ObserveAccountsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$observeAccounts, request, options: options);
  }

  // method descriptors

  static final _$observeAccounts = $grpc.ClientMethod<$0.ObserveAccountsRequest,
          $0.ObserveAccountsResponse>(
      '/voice.identityerasure.v1.AuthRoleIdentityAuthorityService/ObserveAccounts',
      ($0.ObserveAccountsRequest value) => value.writeToBuffer(),
      $0.ObserveAccountsResponse.fromBuffer);
}

@$pb.GrpcServiceName(
    'voice.identityerasure.v1.AuthRoleIdentityAuthorityService')
abstract class AuthRoleIdentityAuthorityServiceBase extends $grpc.Service {
  $core.String get $name =>
      'voice.identityerasure.v1.AuthRoleIdentityAuthorityService';

  AuthRoleIdentityAuthorityServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ObserveAccountsRequest,
            $0.ObserveAccountsResponse>(
        'ObserveAccounts',
        observeAccounts_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ObserveAccountsRequest.fromBuffer(value),
        ($0.ObserveAccountsResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.ObserveAccountsResponse> observeAccounts_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ObserveAccountsRequest> $request) async {
    return observeAccounts($call, await $request);
  }

  $async.Future<$0.ObserveAccountsResponse> observeAccounts(
      $grpc.ServiceCall call, $0.ObserveAccountsRequest request);
}

@$pb.GrpcServiceName(
    'voice.identityerasure.v1.UserRoleIdentityAuthorityService')
class UserRoleIdentityAuthorityServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  UserRoleIdentityAuthorityServiceClient(super.channel,
      {super.options, super.interceptors});

  /// Only service:role. Current ineligibility is separate from irreversible proof.
  $grpc.ResponseFuture<$0.ObserveProfilesResponse> observeProfiles(
    $0.ObserveProfilesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$observeProfiles, request, options: options);
  }

  /// Matches the complete finite Role actor AND target inventory, including
  /// archived and previously removed profiles. It never enumerates raw IDs.
  $grpc.ResponseFuture<$0.ResolveHistoricalCoverageResponse>
      resolveHistoricalCoverage(
    $0.ResolveHistoricalCoverageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveHistoricalCoverage, request,
        options: options);
  }

  // method descriptors

  static final _$observeProfiles = $grpc.ClientMethod<$0.ObserveProfilesRequest,
          $0.ObserveProfilesResponse>(
      '/voice.identityerasure.v1.UserRoleIdentityAuthorityService/ObserveProfiles',
      ($0.ObserveProfilesRequest value) => value.writeToBuffer(),
      $0.ObserveProfilesResponse.fromBuffer);
  static final _$resolveHistoricalCoverage = $grpc.ClientMethod<
          $0.ResolveHistoricalCoverageRequest,
          $0.ResolveHistoricalCoverageResponse>(
      '/voice.identityerasure.v1.UserRoleIdentityAuthorityService/ResolveHistoricalCoverage',
      ($0.ResolveHistoricalCoverageRequest value) => value.writeToBuffer(),
      $0.ResolveHistoricalCoverageResponse.fromBuffer);
}

@$pb.GrpcServiceName(
    'voice.identityerasure.v1.UserRoleIdentityAuthorityService')
abstract class UserRoleIdentityAuthorityServiceBase extends $grpc.Service {
  $core.String get $name =>
      'voice.identityerasure.v1.UserRoleIdentityAuthorityService';

  UserRoleIdentityAuthorityServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ObserveProfilesRequest,
            $0.ObserveProfilesResponse>(
        'ObserveProfiles',
        observeProfiles_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ObserveProfilesRequest.fromBuffer(value),
        ($0.ObserveProfilesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ResolveHistoricalCoverageRequest,
            $0.ResolveHistoricalCoverageResponse>(
        'ResolveHistoricalCoverage',
        resolveHistoricalCoverage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ResolveHistoricalCoverageRequest.fromBuffer(value),
        ($0.ResolveHistoricalCoverageResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.ObserveProfilesResponse> observeProfiles_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ObserveProfilesRequest> $request) async {
    return observeProfiles($call, await $request);
  }

  $async.Future<$0.ObserveProfilesResponse> observeProfiles(
      $grpc.ServiceCall call, $0.ObserveProfilesRequest request);

  $async.Future<$0.ResolveHistoricalCoverageResponse>
      resolveHistoricalCoverage_Pre($grpc.ServiceCall $call,
          $async.Future<$0.ResolveHistoricalCoverageRequest> $request) async {
    return resolveHistoricalCoverage($call, await $request);
  }

  $async.Future<$0.ResolveHistoricalCoverageResponse> resolveHistoricalCoverage(
      $grpc.ServiceCall call, $0.ResolveHistoricalCoverageRequest request);
}

/// Distinct User-purpose capability: it does not grant User any Role method.
@$pb.GrpcServiceName(
    'voice.identityerasure.v1.AuthUserIdentityAuthorityService')
class AuthUserIdentityAuthorityServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  AuthUserIdentityAuthorityServiceClient(super.channel,
      {super.options, super.interceptors});

  /// Called before User opens its birth transaction and outside Auth caller locks.
  $grpc.ResponseFuture<$0.ResolveAccountHandleResponse> resolveAccountHandle(
    $0.ResolveAccountHandleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveAccountHandle, request, options: options);
  }

  $grpc.ResponseFuture<$0.ObserveAccountHandlesResponse> observeAccountHandles(
    $0.ObserveAccountHandlesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$observeAccountHandles, request, options: options);
  }

  // method descriptors

  static final _$resolveAccountHandle = $grpc.ClientMethod<
          $0.ResolveAccountHandleRequest, $0.ResolveAccountHandleResponse>(
      '/voice.identityerasure.v1.AuthUserIdentityAuthorityService/ResolveAccountHandle',
      ($0.ResolveAccountHandleRequest value) => value.writeToBuffer(),
      $0.ResolveAccountHandleResponse.fromBuffer);
  static final _$observeAccountHandles = $grpc.ClientMethod<
          $0.ObserveAccountHandlesRequest, $0.ObserveAccountHandlesResponse>(
      '/voice.identityerasure.v1.AuthUserIdentityAuthorityService/ObserveAccountHandles',
      ($0.ObserveAccountHandlesRequest value) => value.writeToBuffer(),
      $0.ObserveAccountHandlesResponse.fromBuffer);
}

@$pb.GrpcServiceName(
    'voice.identityerasure.v1.AuthUserIdentityAuthorityService')
abstract class AuthUserIdentityAuthorityServiceBase extends $grpc.Service {
  $core.String get $name =>
      'voice.identityerasure.v1.AuthUserIdentityAuthorityService';

  AuthUserIdentityAuthorityServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ResolveAccountHandleRequest,
            $0.ResolveAccountHandleResponse>(
        'ResolveAccountHandle',
        resolveAccountHandle_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ResolveAccountHandleRequest.fromBuffer(value),
        ($0.ResolveAccountHandleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ObserveAccountHandlesRequest,
            $0.ObserveAccountHandlesResponse>(
        'ObserveAccountHandles',
        observeAccountHandles_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ObserveAccountHandlesRequest.fromBuffer(value),
        ($0.ObserveAccountHandlesResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.ResolveAccountHandleResponse> resolveAccountHandle_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ResolveAccountHandleRequest> $request) async {
    return resolveAccountHandle($call, await $request);
  }

  $async.Future<$0.ResolveAccountHandleResponse> resolveAccountHandle(
      $grpc.ServiceCall call, $0.ResolveAccountHandleRequest request);

  $async.Future<$0.ObserveAccountHandlesResponse> observeAccountHandles_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ObserveAccountHandlesRequest> $request) async {
    return observeAccountHandles($call, await $request);
  }

  $async.Future<$0.ObserveAccountHandlesResponse> observeAccountHandles(
      $grpc.ServiceCall call, $0.ObserveAccountHandlesRequest request);
}
