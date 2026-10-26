// This is a generated file - do not edit.
//
// Generated from voice/file/v1/catalog_assets.proto.

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

/// Protected non-E2E catalog authority protocol 2. Ordinary File APIs retain
/// their ACL. Protocol 1 has no ordered-currentness proof and cannot admit sends.
@$pb.GrpcServiceName('voice.file.v1.CatalogAssetService')
class CatalogAssetServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  CatalogAssetServiceClient(super.channel, {super.options, super.interceptors});

  /// @voice.security=protected;callers=service:chat
  $grpc.ResponseFuture<$0.ImportCatalogAssetResponse> importCatalogAsset(
    $0.ImportCatalogAssetRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$importCatalogAsset, request, options: options);
  }

  /// @voice.security=protected;callers=service:messaging
  $grpc.ResponseFuture<$0.ValidateCatalogAssetResponse> validateCatalogAsset(
    $0.ValidateCatalogAssetRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$validateCatalogAsset, request, options: options);
  }

  /// @voice.security=protected;callers=service:messaging
  $grpc.ResponseFuture<$0.LookupCatalogAssetResponse> lookupCatalogAsset(
    $0.LookupCatalogAssetRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$lookupCatalogAsset, request, options: options);
  }

  /// @voice.security=protected;callers=service:chat
  $grpc.ResponseFuture<$0.GetCatalogAssetStatusResponse> getCatalogAssetStatus(
    $0.GetCatalogAssetStatusRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getCatalogAssetStatus, request, options: options);
  }

  // method descriptors

  static final _$importCatalogAsset = $grpc.ClientMethod<
          $0.ImportCatalogAssetRequest, $0.ImportCatalogAssetResponse>(
      '/voice.file.v1.CatalogAssetService/ImportCatalogAsset',
      ($0.ImportCatalogAssetRequest value) => value.writeToBuffer(),
      $0.ImportCatalogAssetResponse.fromBuffer);
  static final _$validateCatalogAsset = $grpc.ClientMethod<
          $0.ValidateCatalogAssetRequest, $0.ValidateCatalogAssetResponse>(
      '/voice.file.v1.CatalogAssetService/ValidateCatalogAsset',
      ($0.ValidateCatalogAssetRequest value) => value.writeToBuffer(),
      $0.ValidateCatalogAssetResponse.fromBuffer);
  static final _$lookupCatalogAsset = $grpc.ClientMethod<
          $0.LookupCatalogAssetRequest, $0.LookupCatalogAssetResponse>(
      '/voice.file.v1.CatalogAssetService/LookupCatalogAsset',
      ($0.LookupCatalogAssetRequest value) => value.writeToBuffer(),
      $0.LookupCatalogAssetResponse.fromBuffer);
  static final _$getCatalogAssetStatus = $grpc.ClientMethod<
          $0.GetCatalogAssetStatusRequest, $0.GetCatalogAssetStatusResponse>(
      '/voice.file.v1.CatalogAssetService/GetCatalogAssetStatus',
      ($0.GetCatalogAssetStatusRequest value) => value.writeToBuffer(),
      $0.GetCatalogAssetStatusResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.file.v1.CatalogAssetService')
abstract class CatalogAssetServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.file.v1.CatalogAssetService';

  CatalogAssetServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ImportCatalogAssetRequest,
            $0.ImportCatalogAssetResponse>(
        'ImportCatalogAsset',
        importCatalogAsset_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ImportCatalogAssetRequest.fromBuffer(value),
        ($0.ImportCatalogAssetResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ValidateCatalogAssetRequest,
            $0.ValidateCatalogAssetResponse>(
        'ValidateCatalogAsset',
        validateCatalogAsset_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ValidateCatalogAssetRequest.fromBuffer(value),
        ($0.ValidateCatalogAssetResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.LookupCatalogAssetRequest,
            $0.LookupCatalogAssetResponse>(
        'LookupCatalogAsset',
        lookupCatalogAsset_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.LookupCatalogAssetRequest.fromBuffer(value),
        ($0.LookupCatalogAssetResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetCatalogAssetStatusRequest,
            $0.GetCatalogAssetStatusResponse>(
        'GetCatalogAssetStatus',
        getCatalogAssetStatus_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetCatalogAssetStatusRequest.fromBuffer(value),
        ($0.GetCatalogAssetStatusResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.ImportCatalogAssetResponse> importCatalogAsset_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ImportCatalogAssetRequest> $request) async {
    return importCatalogAsset($call, await $request);
  }

  $async.Future<$0.ImportCatalogAssetResponse> importCatalogAsset(
      $grpc.ServiceCall call, $0.ImportCatalogAssetRequest request);

  $async.Future<$0.ValidateCatalogAssetResponse> validateCatalogAsset_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ValidateCatalogAssetRequest> $request) async {
    return validateCatalogAsset($call, await $request);
  }

  $async.Future<$0.ValidateCatalogAssetResponse> validateCatalogAsset(
      $grpc.ServiceCall call, $0.ValidateCatalogAssetRequest request);

  $async.Future<$0.LookupCatalogAssetResponse> lookupCatalogAsset_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.LookupCatalogAssetRequest> $request) async {
    return lookupCatalogAsset($call, await $request);
  }

  $async.Future<$0.LookupCatalogAssetResponse> lookupCatalogAsset(
      $grpc.ServiceCall call, $0.LookupCatalogAssetRequest request);

  $async.Future<$0.GetCatalogAssetStatusResponse> getCatalogAssetStatus_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetCatalogAssetStatusRequest> $request) async {
    return getCatalogAssetStatus($call, await $request);
  }

  $async.Future<$0.GetCatalogAssetStatusResponse> getCatalogAssetStatus(
      $grpc.ServiceCall call, $0.GetCatalogAssetStatusRequest request);
}
