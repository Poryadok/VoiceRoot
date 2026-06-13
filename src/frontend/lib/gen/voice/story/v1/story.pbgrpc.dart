// This is a generated file - do not edit.
//
// Generated from voice/story/v1/story.proto.

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

import 'story.pb.dart' as $0;

export 'story.pb.dart';

/// Stories and highlights. HTTP: /api/v1/stories/**.
@$pb.GrpcServiceName('voice.story.v1.StoryService')
class StoryServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  StoryServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.CreateStoryResponse> createStory(
    $0.CreateStoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createStory, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteStoryResponse> deleteStory(
    $0.DeleteStoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteStory, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetStoryResponse> getStory(
    $0.GetStoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getStory, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetStoryFeedResponse> getStoryFeed(
    $0.GetStoryFeedRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getStoryFeed, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetProfileStoriesResponse> getProfileStories(
    $0.GetProfileStoriesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getProfileStories, request, options: options);
  }

  $grpc.ResponseFuture<$0.MarkViewedResponse> markViewed(
    $0.MarkViewedRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$markViewed, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetViewersResponse> getViewers(
    $0.GetViewersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getViewers, request, options: options);
  }

  $grpc.ResponseFuture<$0.ReactToStoryResponse> reactToStory(
    $0.ReactToStoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$reactToStory, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetArchiveResponse> getArchive(
    $0.GetArchiveRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getArchive, request, options: options);
  }

  $grpc.ResponseFuture<$0.CreateHighlightResponse> createHighlight(
    $0.CreateHighlightRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createHighlight, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdateHighlightResponse> updateHighlight(
    $0.UpdateHighlightRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateHighlight, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteHighlightResponse> deleteHighlight(
    $0.DeleteHighlightRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteHighlight, request, options: options);
  }

  $grpc.ResponseFuture<$0.AddToHighlightResponse> addToHighlight(
    $0.AddToHighlightRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$addToHighlight, request, options: options);
  }

  $grpc.ResponseFuture<$0.RemoveFromHighlightResponse> removeFromHighlight(
    $0.RemoveFromHighlightRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeFromHighlight, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetHighlightsResponse> getHighlights(
    $0.GetHighlightsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getHighlights, request, options: options);
  }

  $grpc.ResponseFuture<$0.CreateLookingForPartyResponse> createLookingForParty(
    $0.CreateLookingForPartyRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createLookingForParty, request, options: options);
  }

  // method descriptors

  static final _$createStory =
      $grpc.ClientMethod<$0.CreateStoryRequest, $0.CreateStoryResponse>(
          '/voice.story.v1.StoryService/CreateStory',
          ($0.CreateStoryRequest value) => value.writeToBuffer(),
          $0.CreateStoryResponse.fromBuffer);
  static final _$deleteStory =
      $grpc.ClientMethod<$0.DeleteStoryRequest, $0.DeleteStoryResponse>(
          '/voice.story.v1.StoryService/DeleteStory',
          ($0.DeleteStoryRequest value) => value.writeToBuffer(),
          $0.DeleteStoryResponse.fromBuffer);
  static final _$getStory =
      $grpc.ClientMethod<$0.GetStoryRequest, $0.GetStoryResponse>(
          '/voice.story.v1.StoryService/GetStory',
          ($0.GetStoryRequest value) => value.writeToBuffer(),
          $0.GetStoryResponse.fromBuffer);
  static final _$getStoryFeed =
      $grpc.ClientMethod<$0.GetStoryFeedRequest, $0.GetStoryFeedResponse>(
          '/voice.story.v1.StoryService/GetStoryFeed',
          ($0.GetStoryFeedRequest value) => value.writeToBuffer(),
          $0.GetStoryFeedResponse.fromBuffer);
  static final _$getProfileStories = $grpc.ClientMethod<
          $0.GetProfileStoriesRequest, $0.GetProfileStoriesResponse>(
      '/voice.story.v1.StoryService/GetProfileStories',
      ($0.GetProfileStoriesRequest value) => value.writeToBuffer(),
      $0.GetProfileStoriesResponse.fromBuffer);
  static final _$markViewed =
      $grpc.ClientMethod<$0.MarkViewedRequest, $0.MarkViewedResponse>(
          '/voice.story.v1.StoryService/MarkViewed',
          ($0.MarkViewedRequest value) => value.writeToBuffer(),
          $0.MarkViewedResponse.fromBuffer);
  static final _$getViewers =
      $grpc.ClientMethod<$0.GetViewersRequest, $0.GetViewersResponse>(
          '/voice.story.v1.StoryService/GetViewers',
          ($0.GetViewersRequest value) => value.writeToBuffer(),
          $0.GetViewersResponse.fromBuffer);
  static final _$reactToStory =
      $grpc.ClientMethod<$0.ReactToStoryRequest, $0.ReactToStoryResponse>(
          '/voice.story.v1.StoryService/ReactToStory',
          ($0.ReactToStoryRequest value) => value.writeToBuffer(),
          $0.ReactToStoryResponse.fromBuffer);
  static final _$getArchive =
      $grpc.ClientMethod<$0.GetArchiveRequest, $0.GetArchiveResponse>(
          '/voice.story.v1.StoryService/GetArchive',
          ($0.GetArchiveRequest value) => value.writeToBuffer(),
          $0.GetArchiveResponse.fromBuffer);
  static final _$createHighlight =
      $grpc.ClientMethod<$0.CreateHighlightRequest, $0.CreateHighlightResponse>(
          '/voice.story.v1.StoryService/CreateHighlight',
          ($0.CreateHighlightRequest value) => value.writeToBuffer(),
          $0.CreateHighlightResponse.fromBuffer);
  static final _$updateHighlight =
      $grpc.ClientMethod<$0.UpdateHighlightRequest, $0.UpdateHighlightResponse>(
          '/voice.story.v1.StoryService/UpdateHighlight',
          ($0.UpdateHighlightRequest value) => value.writeToBuffer(),
          $0.UpdateHighlightResponse.fromBuffer);
  static final _$deleteHighlight =
      $grpc.ClientMethod<$0.DeleteHighlightRequest, $0.DeleteHighlightResponse>(
          '/voice.story.v1.StoryService/DeleteHighlight',
          ($0.DeleteHighlightRequest value) => value.writeToBuffer(),
          $0.DeleteHighlightResponse.fromBuffer);
  static final _$addToHighlight =
      $grpc.ClientMethod<$0.AddToHighlightRequest, $0.AddToHighlightResponse>(
          '/voice.story.v1.StoryService/AddToHighlight',
          ($0.AddToHighlightRequest value) => value.writeToBuffer(),
          $0.AddToHighlightResponse.fromBuffer);
  static final _$removeFromHighlight = $grpc.ClientMethod<
          $0.RemoveFromHighlightRequest, $0.RemoveFromHighlightResponse>(
      '/voice.story.v1.StoryService/RemoveFromHighlight',
      ($0.RemoveFromHighlightRequest value) => value.writeToBuffer(),
      $0.RemoveFromHighlightResponse.fromBuffer);
  static final _$getHighlights =
      $grpc.ClientMethod<$0.GetHighlightsRequest, $0.GetHighlightsResponse>(
          '/voice.story.v1.StoryService/GetHighlights',
          ($0.GetHighlightsRequest value) => value.writeToBuffer(),
          $0.GetHighlightsResponse.fromBuffer);
  static final _$createLookingForParty = $grpc.ClientMethod<
          $0.CreateLookingForPartyRequest, $0.CreateLookingForPartyResponse>(
      '/voice.story.v1.StoryService/CreateLookingForParty',
      ($0.CreateLookingForPartyRequest value) => value.writeToBuffer(),
      $0.CreateLookingForPartyResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.story.v1.StoryService')
abstract class StoryServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.story.v1.StoryService';

  StoryServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.CreateStoryRequest, $0.CreateStoryResponse>(
            'CreateStory',
            createStory_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.CreateStoryRequest.fromBuffer(value),
            ($0.CreateStoryResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.DeleteStoryRequest, $0.DeleteStoryResponse>(
            'DeleteStory',
            deleteStory_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.DeleteStoryRequest.fromBuffer(value),
            ($0.DeleteStoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetStoryRequest, $0.GetStoryResponse>(
        'GetStory',
        getStory_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetStoryRequest.fromBuffer(value),
        ($0.GetStoryResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetStoryFeedRequest, $0.GetStoryFeedResponse>(
            'GetStoryFeed',
            getStoryFeed_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetStoryFeedRequest.fromBuffer(value),
            ($0.GetStoryFeedResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetProfileStoriesRequest,
            $0.GetProfileStoriesResponse>(
        'GetProfileStories',
        getProfileStories_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetProfileStoriesRequest.fromBuffer(value),
        ($0.GetProfileStoriesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.MarkViewedRequest, $0.MarkViewedResponse>(
        'MarkViewed',
        markViewed_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.MarkViewedRequest.fromBuffer(value),
        ($0.MarkViewedResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetViewersRequest, $0.GetViewersResponse>(
        'GetViewers',
        getViewers_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetViewersRequest.fromBuffer(value),
        ($0.GetViewersResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ReactToStoryRequest, $0.ReactToStoryResponse>(
            'ReactToStory',
            reactToStory_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ReactToStoryRequest.fromBuffer(value),
            ($0.ReactToStoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetArchiveRequest, $0.GetArchiveResponse>(
        'GetArchive',
        getArchive_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetArchiveRequest.fromBuffer(value),
        ($0.GetArchiveResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CreateHighlightRequest,
            $0.CreateHighlightResponse>(
        'CreateHighlight',
        createHighlight_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CreateHighlightRequest.fromBuffer(value),
        ($0.CreateHighlightResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpdateHighlightRequest,
            $0.UpdateHighlightResponse>(
        'UpdateHighlight',
        updateHighlight_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UpdateHighlightRequest.fromBuffer(value),
        ($0.UpdateHighlightResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeleteHighlightRequest,
            $0.DeleteHighlightResponse>(
        'DeleteHighlight',
        deleteHighlight_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DeleteHighlightRequest.fromBuffer(value),
        ($0.DeleteHighlightResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AddToHighlightRequest,
            $0.AddToHighlightResponse>(
        'AddToHighlight',
        addToHighlight_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.AddToHighlightRequest.fromBuffer(value),
        ($0.AddToHighlightResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RemoveFromHighlightRequest,
            $0.RemoveFromHighlightResponse>(
        'RemoveFromHighlight',
        removeFromHighlight_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RemoveFromHighlightRequest.fromBuffer(value),
        ($0.RemoveFromHighlightResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetHighlightsRequest, $0.GetHighlightsResponse>(
            'GetHighlights',
            getHighlights_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetHighlightsRequest.fromBuffer(value),
            ($0.GetHighlightsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CreateLookingForPartyRequest,
            $0.CreateLookingForPartyResponse>(
        'CreateLookingForParty',
        createLookingForParty_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CreateLookingForPartyRequest.fromBuffer(value),
        ($0.CreateLookingForPartyResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.CreateStoryResponse> createStory_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CreateStoryRequest> $request) async {
    return createStory($call, await $request);
  }

  $async.Future<$0.CreateStoryResponse> createStory(
      $grpc.ServiceCall call, $0.CreateStoryRequest request);

  $async.Future<$0.DeleteStoryResponse> deleteStory_Pre($grpc.ServiceCall $call,
      $async.Future<$0.DeleteStoryRequest> $request) async {
    return deleteStory($call, await $request);
  }

  $async.Future<$0.DeleteStoryResponse> deleteStory(
      $grpc.ServiceCall call, $0.DeleteStoryRequest request);

  $async.Future<$0.GetStoryResponse> getStory_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetStoryRequest> $request) async {
    return getStory($call, await $request);
  }

  $async.Future<$0.GetStoryResponse> getStory(
      $grpc.ServiceCall call, $0.GetStoryRequest request);

  $async.Future<$0.GetStoryFeedResponse> getStoryFeed_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetStoryFeedRequest> $request) async {
    return getStoryFeed($call, await $request);
  }

  $async.Future<$0.GetStoryFeedResponse> getStoryFeed(
      $grpc.ServiceCall call, $0.GetStoryFeedRequest request);

  $async.Future<$0.GetProfileStoriesResponse> getProfileStories_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetProfileStoriesRequest> $request) async {
    return getProfileStories($call, await $request);
  }

  $async.Future<$0.GetProfileStoriesResponse> getProfileStories(
      $grpc.ServiceCall call, $0.GetProfileStoriesRequest request);

  $async.Future<$0.MarkViewedResponse> markViewed_Pre($grpc.ServiceCall $call,
      $async.Future<$0.MarkViewedRequest> $request) async {
    return markViewed($call, await $request);
  }

  $async.Future<$0.MarkViewedResponse> markViewed(
      $grpc.ServiceCall call, $0.MarkViewedRequest request);

  $async.Future<$0.GetViewersResponse> getViewers_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetViewersRequest> $request) async {
    return getViewers($call, await $request);
  }

  $async.Future<$0.GetViewersResponse> getViewers(
      $grpc.ServiceCall call, $0.GetViewersRequest request);

  $async.Future<$0.ReactToStoryResponse> reactToStory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ReactToStoryRequest> $request) async {
    return reactToStory($call, await $request);
  }

  $async.Future<$0.ReactToStoryResponse> reactToStory(
      $grpc.ServiceCall call, $0.ReactToStoryRequest request);

  $async.Future<$0.GetArchiveResponse> getArchive_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetArchiveRequest> $request) async {
    return getArchive($call, await $request);
  }

  $async.Future<$0.GetArchiveResponse> getArchive(
      $grpc.ServiceCall call, $0.GetArchiveRequest request);

  $async.Future<$0.CreateHighlightResponse> createHighlight_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CreateHighlightRequest> $request) async {
    return createHighlight($call, await $request);
  }

  $async.Future<$0.CreateHighlightResponse> createHighlight(
      $grpc.ServiceCall call, $0.CreateHighlightRequest request);

  $async.Future<$0.UpdateHighlightResponse> updateHighlight_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UpdateHighlightRequest> $request) async {
    return updateHighlight($call, await $request);
  }

  $async.Future<$0.UpdateHighlightResponse> updateHighlight(
      $grpc.ServiceCall call, $0.UpdateHighlightRequest request);

  $async.Future<$0.DeleteHighlightResponse> deleteHighlight_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DeleteHighlightRequest> $request) async {
    return deleteHighlight($call, await $request);
  }

  $async.Future<$0.DeleteHighlightResponse> deleteHighlight(
      $grpc.ServiceCall call, $0.DeleteHighlightRequest request);

  $async.Future<$0.AddToHighlightResponse> addToHighlight_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.AddToHighlightRequest> $request) async {
    return addToHighlight($call, await $request);
  }

  $async.Future<$0.AddToHighlightResponse> addToHighlight(
      $grpc.ServiceCall call, $0.AddToHighlightRequest request);

  $async.Future<$0.RemoveFromHighlightResponse> removeFromHighlight_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RemoveFromHighlightRequest> $request) async {
    return removeFromHighlight($call, await $request);
  }

  $async.Future<$0.RemoveFromHighlightResponse> removeFromHighlight(
      $grpc.ServiceCall call, $0.RemoveFromHighlightRequest request);

  $async.Future<$0.GetHighlightsResponse> getHighlights_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetHighlightsRequest> $request) async {
    return getHighlights($call, await $request);
  }

  $async.Future<$0.GetHighlightsResponse> getHighlights(
      $grpc.ServiceCall call, $0.GetHighlightsRequest request);

  $async.Future<$0.CreateLookingForPartyResponse> createLookingForParty_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CreateLookingForPartyRequest> $request) async {
    return createLookingForParty($call, await $request);
  }

  $async.Future<$0.CreateLookingForPartyResponse> createLookingForParty(
      $grpc.ServiceCall call, $0.CreateLookingForPartyRequest request);
}
