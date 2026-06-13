// This is a generated file - do not edit.
//
// Generated from voice/search/v1/search.proto.

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

import 'search.pb.dart' as $0;

export 'search.pb.dart';

/// Full-text and global search. HTTP: /api/v1/search/**.
@$pb.GrpcServiceName('voice.search.v1.SearchService')
class SearchServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  SearchServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.SearchInChatResponse> searchInChat(
    $0.SearchInChatRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$searchInChat, request, options: options);
  }

  $grpc.ResponseFuture<$0.SearchGlobalResponse> searchGlobal(
    $0.SearchGlobalRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$searchGlobal, request, options: options);
  }

  $grpc.ResponseFuture<$0.SearchUsersResponse> searchUsers(
    $0.SearchUsersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$searchUsers, request, options: options);
  }

  $grpc.ResponseFuture<$0.SearchSpacesResponse> searchSpaces(
    $0.SearchSpacesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$searchSpaces, request, options: options);
  }

  $grpc.ResponseFuture<$0.ReindexChatResponse> reindexChat(
    $0.ReindexChatRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$reindexChat, request, options: options);
  }

  // method descriptors

  static final _$searchInChat =
      $grpc.ClientMethod<$0.SearchInChatRequest, $0.SearchInChatResponse>(
          '/voice.search.v1.SearchService/SearchInChat',
          ($0.SearchInChatRequest value) => value.writeToBuffer(),
          $0.SearchInChatResponse.fromBuffer);
  static final _$searchGlobal =
      $grpc.ClientMethod<$0.SearchGlobalRequest, $0.SearchGlobalResponse>(
          '/voice.search.v1.SearchService/SearchGlobal',
          ($0.SearchGlobalRequest value) => value.writeToBuffer(),
          $0.SearchGlobalResponse.fromBuffer);
  static final _$searchUsers =
      $grpc.ClientMethod<$0.SearchUsersRequest, $0.SearchUsersResponse>(
          '/voice.search.v1.SearchService/SearchUsers',
          ($0.SearchUsersRequest value) => value.writeToBuffer(),
          $0.SearchUsersResponse.fromBuffer);
  static final _$searchSpaces =
      $grpc.ClientMethod<$0.SearchSpacesRequest, $0.SearchSpacesResponse>(
          '/voice.search.v1.SearchService/SearchSpaces',
          ($0.SearchSpacesRequest value) => value.writeToBuffer(),
          $0.SearchSpacesResponse.fromBuffer);
  static final _$reindexChat =
      $grpc.ClientMethod<$0.ReindexChatRequest, $0.ReindexChatResponse>(
          '/voice.search.v1.SearchService/ReindexChat',
          ($0.ReindexChatRequest value) => value.writeToBuffer(),
          $0.ReindexChatResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.search.v1.SearchService')
abstract class SearchServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.search.v1.SearchService';

  SearchServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.SearchInChatRequest, $0.SearchInChatResponse>(
            'SearchInChat',
            searchInChat_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SearchInChatRequest.fromBuffer(value),
            ($0.SearchInChatResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SearchGlobalRequest, $0.SearchGlobalResponse>(
            'SearchGlobal',
            searchGlobal_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SearchGlobalRequest.fromBuffer(value),
            ($0.SearchGlobalResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SearchUsersRequest, $0.SearchUsersResponse>(
            'SearchUsers',
            searchUsers_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SearchUsersRequest.fromBuffer(value),
            ($0.SearchUsersResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SearchSpacesRequest, $0.SearchSpacesResponse>(
            'SearchSpaces',
            searchSpaces_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SearchSpacesRequest.fromBuffer(value),
            ($0.SearchSpacesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ReindexChatRequest, $0.ReindexChatResponse>(
            'ReindexChat',
            reindexChat_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ReindexChatRequest.fromBuffer(value),
            ($0.ReindexChatResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.SearchInChatResponse> searchInChat_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SearchInChatRequest> $request) async {
    return searchInChat($call, await $request);
  }

  $async.Future<$0.SearchInChatResponse> searchInChat(
      $grpc.ServiceCall call, $0.SearchInChatRequest request);

  $async.Future<$0.SearchGlobalResponse> searchGlobal_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SearchGlobalRequest> $request) async {
    return searchGlobal($call, await $request);
  }

  $async.Future<$0.SearchGlobalResponse> searchGlobal(
      $grpc.ServiceCall call, $0.SearchGlobalRequest request);

  $async.Future<$0.SearchUsersResponse> searchUsers_Pre($grpc.ServiceCall $call,
      $async.Future<$0.SearchUsersRequest> $request) async {
    return searchUsers($call, await $request);
  }

  $async.Future<$0.SearchUsersResponse> searchUsers(
      $grpc.ServiceCall call, $0.SearchUsersRequest request);

  $async.Future<$0.SearchSpacesResponse> searchSpaces_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SearchSpacesRequest> $request) async {
    return searchSpaces($call, await $request);
  }

  $async.Future<$0.SearchSpacesResponse> searchSpaces(
      $grpc.ServiceCall call, $0.SearchSpacesRequest request);

  $async.Future<$0.ReindexChatResponse> reindexChat_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ReindexChatRequest> $request) async {
    return reindexChat($call, await $request);
  }

  $async.Future<$0.ReindexChatResponse> reindexChat(
      $grpc.ServiceCall call, $0.ReindexChatRequest request);
}
