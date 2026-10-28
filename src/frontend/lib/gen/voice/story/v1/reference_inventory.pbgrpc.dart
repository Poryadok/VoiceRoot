// This is a generated file - do not edit.
//
// Generated from voice/story/v1/reference_inventory.proto.

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

import '../../common/v1/reference_inventory.pb.dart' as $0;

export 'reference_inventory.pb.dart';

/// Private migration endpoint. It does not attest global File activation.
@$pb.GrpcServiceName('voice.story.v1.StoryReferenceInventoryService')
class StoryReferenceInventoryServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  StoryReferenceInventoryServiceClient(super.channel,
      {super.options, super.interceptors});

  /// @voice.security=protected;callers=service:file
  $grpc.ResponseFuture<$0.BeginReferenceInventoryResponse>
      beginReferenceInventory(
    $0.BeginReferenceInventoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$beginReferenceInventory, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:file
  $grpc.ResponseFuture<$0.ReadReferenceInventoryPageResponse>
      readReferenceInventoryPage(
    $0.ReadReferenceInventoryPageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$readReferenceInventoryPage, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:file
  $grpc.ResponseFuture<$0.ReadReferenceInventoryChangesResponse>
      readReferenceInventoryChanges(
    $0.ReadReferenceInventoryChangesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$readReferenceInventoryChanges, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:file
  $grpc.ResponseFuture<$0.SealReferenceInventoryResponse>
      sealReferenceInventory(
    $0.SealReferenceInventoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$sealReferenceInventory, request,
        options: options);
  }

  // method descriptors

  static final _$beginReferenceInventory = $grpc.ClientMethod<
          $0.BeginReferenceInventoryRequest,
          $0.BeginReferenceInventoryResponse>(
      '/voice.story.v1.StoryReferenceInventoryService/BeginReferenceInventory',
      ($0.BeginReferenceInventoryRequest value) => value.writeToBuffer(),
      $0.BeginReferenceInventoryResponse.fromBuffer);
  static final _$readReferenceInventoryPage = $grpc.ClientMethod<
          $0.ReadReferenceInventoryPageRequest,
          $0.ReadReferenceInventoryPageResponse>(
      '/voice.story.v1.StoryReferenceInventoryService/ReadReferenceInventoryPage',
      ($0.ReadReferenceInventoryPageRequest value) => value.writeToBuffer(),
      $0.ReadReferenceInventoryPageResponse.fromBuffer);
  static final _$readReferenceInventoryChanges = $grpc.ClientMethod<
          $0.ReadReferenceInventoryChangesRequest,
          $0.ReadReferenceInventoryChangesResponse>(
      '/voice.story.v1.StoryReferenceInventoryService/ReadReferenceInventoryChanges',
      ($0.ReadReferenceInventoryChangesRequest value) => value.writeToBuffer(),
      $0.ReadReferenceInventoryChangesResponse.fromBuffer);
  static final _$sealReferenceInventory = $grpc.ClientMethod<
          $0.SealReferenceInventoryRequest, $0.SealReferenceInventoryResponse>(
      '/voice.story.v1.StoryReferenceInventoryService/SealReferenceInventory',
      ($0.SealReferenceInventoryRequest value) => value.writeToBuffer(),
      $0.SealReferenceInventoryResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.story.v1.StoryReferenceInventoryService')
abstract class StoryReferenceInventoryServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.story.v1.StoryReferenceInventoryService';

  StoryReferenceInventoryServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.BeginReferenceInventoryRequest,
            $0.BeginReferenceInventoryResponse>(
        'BeginReferenceInventory',
        beginReferenceInventory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.BeginReferenceInventoryRequest.fromBuffer(value),
        ($0.BeginReferenceInventoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ReadReferenceInventoryPageRequest,
            $0.ReadReferenceInventoryPageResponse>(
        'ReadReferenceInventoryPage',
        readReferenceInventoryPage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ReadReferenceInventoryPageRequest.fromBuffer(value),
        ($0.ReadReferenceInventoryPageResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ReadReferenceInventoryChangesRequest,
            $0.ReadReferenceInventoryChangesResponse>(
        'ReadReferenceInventoryChanges',
        readReferenceInventoryChanges_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ReadReferenceInventoryChangesRequest.fromBuffer(value),
        ($0.ReadReferenceInventoryChangesResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SealReferenceInventoryRequest,
            $0.SealReferenceInventoryResponse>(
        'SealReferenceInventory',
        sealReferenceInventory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SealReferenceInventoryRequest.fromBuffer(value),
        ($0.SealReferenceInventoryResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.BeginReferenceInventoryResponse> beginReferenceInventory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.BeginReferenceInventoryRequest> $request) async {
    return beginReferenceInventory($call, await $request);
  }

  $async.Future<$0.BeginReferenceInventoryResponse> beginReferenceInventory(
      $grpc.ServiceCall call, $0.BeginReferenceInventoryRequest request);

  $async.Future<$0.ReadReferenceInventoryPageResponse>
      readReferenceInventoryPage_Pre($grpc.ServiceCall $call,
          $async.Future<$0.ReadReferenceInventoryPageRequest> $request) async {
    return readReferenceInventoryPage($call, await $request);
  }

  $async.Future<$0.ReadReferenceInventoryPageResponse>
      readReferenceInventoryPage(
          $grpc.ServiceCall call, $0.ReadReferenceInventoryPageRequest request);

  $async.Future<$0.ReadReferenceInventoryChangesResponse>
      readReferenceInventoryChanges_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$0.ReadReferenceInventoryChangesRequest>
              $request) async {
    return readReferenceInventoryChanges($call, await $request);
  }

  $async.Future<$0.ReadReferenceInventoryChangesResponse>
      readReferenceInventoryChanges($grpc.ServiceCall call,
          $0.ReadReferenceInventoryChangesRequest request);

  $async.Future<$0.SealReferenceInventoryResponse> sealReferenceInventory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SealReferenceInventoryRequest> $request) async {
    return sealReferenceInventory($call, await $request);
  }

  $async.Future<$0.SealReferenceInventoryResponse> sealReferenceInventory(
      $grpc.ServiceCall call, $0.SealReferenceInventoryRequest request);
}
