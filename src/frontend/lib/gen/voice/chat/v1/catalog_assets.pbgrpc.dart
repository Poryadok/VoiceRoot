// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/catalog_assets.proto.

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

import 'catalog_assets.pb.dart' as $0;

export 'catalog_assets.pb.dart';

/// Dedicated protected listener. Never register on the ordinary Chat listener.
@$pb.GrpcServiceName('voice.chat.v1.CatalogAssetService')
class CatalogAssetServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  CatalogAssetServiceClient(super.channel, {super.options, super.interceptors});

  /// @voice.security=protected;callers=delegated_user:gateway
  $grpc.ResponseFuture<$0.ResolveGifAssetResponse> resolveGifAsset(
    $0.ResolveGifAssetRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveGifAsset, request, options: options);
  }

  /// @voice.security=protected;callers=service:messaging
  $grpc.ResponseFuture<$0.ValidateGifAssetResponse> validateGifAsset(
    $0.ValidateGifAssetRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$validateGifAsset, request, options: options);
  }

  /// @voice.security=protected;callers=service:file
  $grpc.ResponseFuture<$0.AuthorizeCatalogDestinationResponse>
      authorizeCatalogDestination(
    $0.AuthorizeCatalogDestinationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$authorizeCatalogDestination, request,
        options: options);
  }

  // method descriptors

  static final _$resolveGifAsset =
      $grpc.ClientMethod<$0.ResolveGifAssetRequest, $0.ResolveGifAssetResponse>(
          '/voice.chat.v1.CatalogAssetService/ResolveGifAsset',
          ($0.ResolveGifAssetRequest value) => value.writeToBuffer(),
          $0.ResolveGifAssetResponse.fromBuffer);
  static final _$validateGifAsset = $grpc.ClientMethod<
          $0.ValidateGifAssetRequest, $0.ValidateGifAssetResponse>(
      '/voice.chat.v1.CatalogAssetService/ValidateGifAsset',
      ($0.ValidateGifAssetRequest value) => value.writeToBuffer(),
      $0.ValidateGifAssetResponse.fromBuffer);
  static final _$authorizeCatalogDestination = $grpc.ClientMethod<
          $0.AuthorizeCatalogDestinationRequest,
          $0.AuthorizeCatalogDestinationResponse>(
      '/voice.chat.v1.CatalogAssetService/AuthorizeCatalogDestination',
      ($0.AuthorizeCatalogDestinationRequest value) => value.writeToBuffer(),
      $0.AuthorizeCatalogDestinationResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.chat.v1.CatalogAssetService')
abstract class CatalogAssetServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.chat.v1.CatalogAssetService';

  CatalogAssetServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ResolveGifAssetRequest,
            $0.ResolveGifAssetResponse>(
        'ResolveGifAsset',
        resolveGifAsset_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ResolveGifAssetRequest.fromBuffer(value),
        ($0.ResolveGifAssetResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ValidateGifAssetRequest,
            $0.ValidateGifAssetResponse>(
        'ValidateGifAsset',
        validateGifAsset_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ValidateGifAssetRequest.fromBuffer(value),
        ($0.ValidateGifAssetResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AuthorizeCatalogDestinationRequest,
            $0.AuthorizeCatalogDestinationResponse>(
        'AuthorizeCatalogDestination',
        authorizeCatalogDestination_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.AuthorizeCatalogDestinationRequest.fromBuffer(value),
        ($0.AuthorizeCatalogDestinationResponse value) =>
            value.writeToBuffer()));
  }

  $async.Future<$0.ResolveGifAssetResponse> resolveGifAsset_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ResolveGifAssetRequest> $request) async {
    return resolveGifAsset($call, await $request);
  }

  $async.Future<$0.ResolveGifAssetResponse> resolveGifAsset(
      $grpc.ServiceCall call, $0.ResolveGifAssetRequest request);

  $async.Future<$0.ValidateGifAssetResponse> validateGifAsset_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ValidateGifAssetRequest> $request) async {
    return validateGifAsset($call, await $request);
  }

  $async.Future<$0.ValidateGifAssetResponse> validateGifAsset(
      $grpc.ServiceCall call, $0.ValidateGifAssetRequest request);

  $async.Future<$0.AuthorizeCatalogDestinationResponse>
      authorizeCatalogDestination_Pre($grpc.ServiceCall $call,
          $async.Future<$0.AuthorizeCatalogDestinationRequest> $request) async {
    return authorizeCatalogDestination($call, await $request);
  }

  $async.Future<$0.AuthorizeCatalogDestinationResponse>
      authorizeCatalogDestination($grpc.ServiceCall call,
          $0.AuthorizeCatalogDestinationRequest request);
}
