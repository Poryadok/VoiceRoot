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

import 'report_target.pb.dart' as $0;
import 'story.pb.dart' as $1;

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

  /// Protected service:moderation only; no viewer/content authority.
  $grpc.ResponseFuture<$0.ResolveReportTargetResponse> resolveReportTarget(
    $0.ResolveReportTargetRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveReportTarget, request, options: options);
  }

  $grpc.ResponseFuture<$1.CreateStoryResponse> createStory(
    $1.CreateStoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createStory, request, options: options);
  }

  $grpc.ResponseFuture<$1.DeleteStoryResponse> deleteStory(
    $1.DeleteStoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteStory, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetStoryResponse> getStory(
    $1.GetStoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getStory, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetStoryFeedResponse> getStoryFeed(
    $1.GetStoryFeedRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getStoryFeed, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetProfileStoriesResponse> getProfileStories(
    $1.GetProfileStoriesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getProfileStories, request, options: options);
  }

  $grpc.ResponseFuture<$1.MarkViewedResponse> markViewed(
    $1.MarkViewedRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$markViewed, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetViewersResponse> getViewers(
    $1.GetViewersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getViewers, request, options: options);
  }

  $grpc.ResponseFuture<$1.ReactToStoryResponse> reactToStory(
    $1.ReactToStoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$reactToStory, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetStoryReactionsResponse> getStoryReactions(
    $1.GetStoryReactionsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getStoryReactions, request, options: options);
  }

  $grpc.ResponseFuture<$1.ReplyToStoryResponse> replyToStory(
    $1.ReplyToStoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$replyToStory, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetArchiveResponse> getArchive(
    $1.GetArchiveRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getArchive, request, options: options);
  }

  $grpc.ResponseFuture<$1.CreateHighlightResponse> createHighlight(
    $1.CreateHighlightRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createHighlight, request, options: options);
  }

  $grpc.ResponseFuture<$1.UpdateHighlightResponse> updateHighlight(
    $1.UpdateHighlightRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateHighlight, request, options: options);
  }

  $grpc.ResponseFuture<$1.DeleteHighlightResponse> deleteHighlight(
    $1.DeleteHighlightRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteHighlight, request, options: options);
  }

  $grpc.ResponseFuture<$1.AddToHighlightResponse> addToHighlight(
    $1.AddToHighlightRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$addToHighlight, request, options: options);
  }

  $grpc.ResponseFuture<$1.RemoveFromHighlightResponse> removeFromHighlight(
    $1.RemoveFromHighlightRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeFromHighlight, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetHighlightsResponse> getHighlights(
    $1.GetHighlightsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getHighlights, request, options: options);
  }

  $grpc.ResponseFuture<$1.CreateLookingForPartyResponse> createLookingForParty(
    $1.CreateLookingForPartyRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createLookingForParty, request, options: options);
  }

  $grpc.ResponseFuture<$1.RespondToLfpStoryResponse> respondToLfpStory(
    $1.RespondToLfpStoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$respondToLfpStory, request, options: options);
  }

  /// Staff/internal: soft-hide story from feeds and non-author GetStory (author retains access).
  $grpc.ResponseFuture<$1.HideStoryFromFeedResponse> hideStoryFromFeed(
    $1.HideStoryFromFeedRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$hideStoryFromFeed, request, options: options);
  }

  // method descriptors

  static final _$resolveReportTarget = $grpc.ClientMethod<
          $0.ResolveReportTargetRequest, $0.ResolveReportTargetResponse>(
      '/voice.story.v1.StoryService/ResolveReportTarget',
      ($0.ResolveReportTargetRequest value) => value.writeToBuffer(),
      $0.ResolveReportTargetResponse.fromBuffer);
  static final _$createStory =
      $grpc.ClientMethod<$1.CreateStoryRequest, $1.CreateStoryResponse>(
          '/voice.story.v1.StoryService/CreateStory',
          ($1.CreateStoryRequest value) => value.writeToBuffer(),
          $1.CreateStoryResponse.fromBuffer);
  static final _$deleteStory =
      $grpc.ClientMethod<$1.DeleteStoryRequest, $1.DeleteStoryResponse>(
          '/voice.story.v1.StoryService/DeleteStory',
          ($1.DeleteStoryRequest value) => value.writeToBuffer(),
          $1.DeleteStoryResponse.fromBuffer);
  static final _$getStory =
      $grpc.ClientMethod<$1.GetStoryRequest, $1.GetStoryResponse>(
          '/voice.story.v1.StoryService/GetStory',
          ($1.GetStoryRequest value) => value.writeToBuffer(),
          $1.GetStoryResponse.fromBuffer);
  static final _$getStoryFeed =
      $grpc.ClientMethod<$1.GetStoryFeedRequest, $1.GetStoryFeedResponse>(
          '/voice.story.v1.StoryService/GetStoryFeed',
          ($1.GetStoryFeedRequest value) => value.writeToBuffer(),
          $1.GetStoryFeedResponse.fromBuffer);
  static final _$getProfileStories = $grpc.ClientMethod<
          $1.GetProfileStoriesRequest, $1.GetProfileStoriesResponse>(
      '/voice.story.v1.StoryService/GetProfileStories',
      ($1.GetProfileStoriesRequest value) => value.writeToBuffer(),
      $1.GetProfileStoriesResponse.fromBuffer);
  static final _$markViewed =
      $grpc.ClientMethod<$1.MarkViewedRequest, $1.MarkViewedResponse>(
          '/voice.story.v1.StoryService/MarkViewed',
          ($1.MarkViewedRequest value) => value.writeToBuffer(),
          $1.MarkViewedResponse.fromBuffer);
  static final _$getViewers =
      $grpc.ClientMethod<$1.GetViewersRequest, $1.GetViewersResponse>(
          '/voice.story.v1.StoryService/GetViewers',
          ($1.GetViewersRequest value) => value.writeToBuffer(),
          $1.GetViewersResponse.fromBuffer);
  static final _$reactToStory =
      $grpc.ClientMethod<$1.ReactToStoryRequest, $1.ReactToStoryResponse>(
          '/voice.story.v1.StoryService/ReactToStory',
          ($1.ReactToStoryRequest value) => value.writeToBuffer(),
          $1.ReactToStoryResponse.fromBuffer);
  static final _$getStoryReactions = $grpc.ClientMethod<
          $1.GetStoryReactionsRequest, $1.GetStoryReactionsResponse>(
      '/voice.story.v1.StoryService/GetStoryReactions',
      ($1.GetStoryReactionsRequest value) => value.writeToBuffer(),
      $1.GetStoryReactionsResponse.fromBuffer);
  static final _$replyToStory =
      $grpc.ClientMethod<$1.ReplyToStoryRequest, $1.ReplyToStoryResponse>(
          '/voice.story.v1.StoryService/ReplyToStory',
          ($1.ReplyToStoryRequest value) => value.writeToBuffer(),
          $1.ReplyToStoryResponse.fromBuffer);
  static final _$getArchive =
      $grpc.ClientMethod<$1.GetArchiveRequest, $1.GetArchiveResponse>(
          '/voice.story.v1.StoryService/GetArchive',
          ($1.GetArchiveRequest value) => value.writeToBuffer(),
          $1.GetArchiveResponse.fromBuffer);
  static final _$createHighlight =
      $grpc.ClientMethod<$1.CreateHighlightRequest, $1.CreateHighlightResponse>(
          '/voice.story.v1.StoryService/CreateHighlight',
          ($1.CreateHighlightRequest value) => value.writeToBuffer(),
          $1.CreateHighlightResponse.fromBuffer);
  static final _$updateHighlight =
      $grpc.ClientMethod<$1.UpdateHighlightRequest, $1.UpdateHighlightResponse>(
          '/voice.story.v1.StoryService/UpdateHighlight',
          ($1.UpdateHighlightRequest value) => value.writeToBuffer(),
          $1.UpdateHighlightResponse.fromBuffer);
  static final _$deleteHighlight =
      $grpc.ClientMethod<$1.DeleteHighlightRequest, $1.DeleteHighlightResponse>(
          '/voice.story.v1.StoryService/DeleteHighlight',
          ($1.DeleteHighlightRequest value) => value.writeToBuffer(),
          $1.DeleteHighlightResponse.fromBuffer);
  static final _$addToHighlight =
      $grpc.ClientMethod<$1.AddToHighlightRequest, $1.AddToHighlightResponse>(
          '/voice.story.v1.StoryService/AddToHighlight',
          ($1.AddToHighlightRequest value) => value.writeToBuffer(),
          $1.AddToHighlightResponse.fromBuffer);
  static final _$removeFromHighlight = $grpc.ClientMethod<
          $1.RemoveFromHighlightRequest, $1.RemoveFromHighlightResponse>(
      '/voice.story.v1.StoryService/RemoveFromHighlight',
      ($1.RemoveFromHighlightRequest value) => value.writeToBuffer(),
      $1.RemoveFromHighlightResponse.fromBuffer);
  static final _$getHighlights =
      $grpc.ClientMethod<$1.GetHighlightsRequest, $1.GetHighlightsResponse>(
          '/voice.story.v1.StoryService/GetHighlights',
          ($1.GetHighlightsRequest value) => value.writeToBuffer(),
          $1.GetHighlightsResponse.fromBuffer);
  static final _$createLookingForParty = $grpc.ClientMethod<
          $1.CreateLookingForPartyRequest, $1.CreateLookingForPartyResponse>(
      '/voice.story.v1.StoryService/CreateLookingForParty',
      ($1.CreateLookingForPartyRequest value) => value.writeToBuffer(),
      $1.CreateLookingForPartyResponse.fromBuffer);
  static final _$respondToLfpStory = $grpc.ClientMethod<
          $1.RespondToLfpStoryRequest, $1.RespondToLfpStoryResponse>(
      '/voice.story.v1.StoryService/RespondToLfpStory',
      ($1.RespondToLfpStoryRequest value) => value.writeToBuffer(),
      $1.RespondToLfpStoryResponse.fromBuffer);
  static final _$hideStoryFromFeed = $grpc.ClientMethod<
          $1.HideStoryFromFeedRequest, $1.HideStoryFromFeedResponse>(
      '/voice.story.v1.StoryService/HideStoryFromFeed',
      ($1.HideStoryFromFeedRequest value) => value.writeToBuffer(),
      $1.HideStoryFromFeedResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.story.v1.StoryService')
abstract class StoryServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.story.v1.StoryService';

  StoryServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ResolveReportTargetRequest,
            $0.ResolveReportTargetResponse>(
        'ResolveReportTarget',
        resolveReportTarget_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ResolveReportTargetRequest.fromBuffer(value),
        ($0.ResolveReportTargetResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.CreateStoryRequest, $1.CreateStoryResponse>(
            'CreateStory',
            createStory_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.CreateStoryRequest.fromBuffer(value),
            ($1.CreateStoryResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.DeleteStoryRequest, $1.DeleteStoryResponse>(
            'DeleteStory',
            deleteStory_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.DeleteStoryRequest.fromBuffer(value),
            ($1.DeleteStoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetStoryRequest, $1.GetStoryResponse>(
        'GetStory',
        getStory_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.GetStoryRequest.fromBuffer(value),
        ($1.GetStoryResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.GetStoryFeedRequest, $1.GetStoryFeedResponse>(
            'GetStoryFeed',
            getStoryFeed_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.GetStoryFeedRequest.fromBuffer(value),
            ($1.GetStoryFeedResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetProfileStoriesRequest,
            $1.GetProfileStoriesResponse>(
        'GetProfileStories',
        getProfileStories_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetProfileStoriesRequest.fromBuffer(value),
        ($1.GetProfileStoriesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.MarkViewedRequest, $1.MarkViewedResponse>(
        'MarkViewed',
        markViewed_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.MarkViewedRequest.fromBuffer(value),
        ($1.MarkViewedResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetViewersRequest, $1.GetViewersResponse>(
        'GetViewers',
        getViewers_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.GetViewersRequest.fromBuffer(value),
        ($1.GetViewersResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.ReactToStoryRequest, $1.ReactToStoryResponse>(
            'ReactToStory',
            reactToStory_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.ReactToStoryRequest.fromBuffer(value),
            ($1.ReactToStoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetStoryReactionsRequest,
            $1.GetStoryReactionsResponse>(
        'GetStoryReactions',
        getStoryReactions_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetStoryReactionsRequest.fromBuffer(value),
        ($1.GetStoryReactionsResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.ReplyToStoryRequest, $1.ReplyToStoryResponse>(
            'ReplyToStory',
            replyToStory_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.ReplyToStoryRequest.fromBuffer(value),
            ($1.ReplyToStoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetArchiveRequest, $1.GetArchiveResponse>(
        'GetArchive',
        getArchive_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.GetArchiveRequest.fromBuffer(value),
        ($1.GetArchiveResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.CreateHighlightRequest,
            $1.CreateHighlightResponse>(
        'CreateHighlight',
        createHighlight_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.CreateHighlightRequest.fromBuffer(value),
        ($1.CreateHighlightResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.UpdateHighlightRequest,
            $1.UpdateHighlightResponse>(
        'UpdateHighlight',
        updateHighlight_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.UpdateHighlightRequest.fromBuffer(value),
        ($1.UpdateHighlightResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.DeleteHighlightRequest,
            $1.DeleteHighlightResponse>(
        'DeleteHighlight',
        deleteHighlight_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.DeleteHighlightRequest.fromBuffer(value),
        ($1.DeleteHighlightResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.AddToHighlightRequest,
            $1.AddToHighlightResponse>(
        'AddToHighlight',
        addToHighlight_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.AddToHighlightRequest.fromBuffer(value),
        ($1.AddToHighlightResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.RemoveFromHighlightRequest,
            $1.RemoveFromHighlightResponse>(
        'RemoveFromHighlight',
        removeFromHighlight_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.RemoveFromHighlightRequest.fromBuffer(value),
        ($1.RemoveFromHighlightResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.GetHighlightsRequest, $1.GetHighlightsResponse>(
            'GetHighlights',
            getHighlights_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.GetHighlightsRequest.fromBuffer(value),
            ($1.GetHighlightsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.CreateLookingForPartyRequest,
            $1.CreateLookingForPartyResponse>(
        'CreateLookingForParty',
        createLookingForParty_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.CreateLookingForPartyRequest.fromBuffer(value),
        ($1.CreateLookingForPartyResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.RespondToLfpStoryRequest,
            $1.RespondToLfpStoryResponse>(
        'RespondToLfpStory',
        respondToLfpStory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.RespondToLfpStoryRequest.fromBuffer(value),
        ($1.RespondToLfpStoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.HideStoryFromFeedRequest,
            $1.HideStoryFromFeedResponse>(
        'HideStoryFromFeed',
        hideStoryFromFeed_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.HideStoryFromFeedRequest.fromBuffer(value),
        ($1.HideStoryFromFeedResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.ResolveReportTargetResponse> resolveReportTarget_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ResolveReportTargetRequest> $request) async {
    return resolveReportTarget($call, await $request);
  }

  $async.Future<$0.ResolveReportTargetResponse> resolveReportTarget(
      $grpc.ServiceCall call, $0.ResolveReportTargetRequest request);

  $async.Future<$1.CreateStoryResponse> createStory_Pre($grpc.ServiceCall $call,
      $async.Future<$1.CreateStoryRequest> $request) async {
    return createStory($call, await $request);
  }

  $async.Future<$1.CreateStoryResponse> createStory(
      $grpc.ServiceCall call, $1.CreateStoryRequest request);

  $async.Future<$1.DeleteStoryResponse> deleteStory_Pre($grpc.ServiceCall $call,
      $async.Future<$1.DeleteStoryRequest> $request) async {
    return deleteStory($call, await $request);
  }

  $async.Future<$1.DeleteStoryResponse> deleteStory(
      $grpc.ServiceCall call, $1.DeleteStoryRequest request);

  $async.Future<$1.GetStoryResponse> getStory_Pre($grpc.ServiceCall $call,
      $async.Future<$1.GetStoryRequest> $request) async {
    return getStory($call, await $request);
  }

  $async.Future<$1.GetStoryResponse> getStory(
      $grpc.ServiceCall call, $1.GetStoryRequest request);

  $async.Future<$1.GetStoryFeedResponse> getStoryFeed_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetStoryFeedRequest> $request) async {
    return getStoryFeed($call, await $request);
  }

  $async.Future<$1.GetStoryFeedResponse> getStoryFeed(
      $grpc.ServiceCall call, $1.GetStoryFeedRequest request);

  $async.Future<$1.GetProfileStoriesResponse> getProfileStories_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetProfileStoriesRequest> $request) async {
    return getProfileStories($call, await $request);
  }

  $async.Future<$1.GetProfileStoriesResponse> getProfileStories(
      $grpc.ServiceCall call, $1.GetProfileStoriesRequest request);

  $async.Future<$1.MarkViewedResponse> markViewed_Pre($grpc.ServiceCall $call,
      $async.Future<$1.MarkViewedRequest> $request) async {
    return markViewed($call, await $request);
  }

  $async.Future<$1.MarkViewedResponse> markViewed(
      $grpc.ServiceCall call, $1.MarkViewedRequest request);

  $async.Future<$1.GetViewersResponse> getViewers_Pre($grpc.ServiceCall $call,
      $async.Future<$1.GetViewersRequest> $request) async {
    return getViewers($call, await $request);
  }

  $async.Future<$1.GetViewersResponse> getViewers(
      $grpc.ServiceCall call, $1.GetViewersRequest request);

  $async.Future<$1.ReactToStoryResponse> reactToStory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ReactToStoryRequest> $request) async {
    return reactToStory($call, await $request);
  }

  $async.Future<$1.ReactToStoryResponse> reactToStory(
      $grpc.ServiceCall call, $1.ReactToStoryRequest request);

  $async.Future<$1.GetStoryReactionsResponse> getStoryReactions_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetStoryReactionsRequest> $request) async {
    return getStoryReactions($call, await $request);
  }

  $async.Future<$1.GetStoryReactionsResponse> getStoryReactions(
      $grpc.ServiceCall call, $1.GetStoryReactionsRequest request);

  $async.Future<$1.ReplyToStoryResponse> replyToStory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ReplyToStoryRequest> $request) async {
    return replyToStory($call, await $request);
  }

  $async.Future<$1.ReplyToStoryResponse> replyToStory(
      $grpc.ServiceCall call, $1.ReplyToStoryRequest request);

  $async.Future<$1.GetArchiveResponse> getArchive_Pre($grpc.ServiceCall $call,
      $async.Future<$1.GetArchiveRequest> $request) async {
    return getArchive($call, await $request);
  }

  $async.Future<$1.GetArchiveResponse> getArchive(
      $grpc.ServiceCall call, $1.GetArchiveRequest request);

  $async.Future<$1.CreateHighlightResponse> createHighlight_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.CreateHighlightRequest> $request) async {
    return createHighlight($call, await $request);
  }

  $async.Future<$1.CreateHighlightResponse> createHighlight(
      $grpc.ServiceCall call, $1.CreateHighlightRequest request);

  $async.Future<$1.UpdateHighlightResponse> updateHighlight_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UpdateHighlightRequest> $request) async {
    return updateHighlight($call, await $request);
  }

  $async.Future<$1.UpdateHighlightResponse> updateHighlight(
      $grpc.ServiceCall call, $1.UpdateHighlightRequest request);

  $async.Future<$1.DeleteHighlightResponse> deleteHighlight_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.DeleteHighlightRequest> $request) async {
    return deleteHighlight($call, await $request);
  }

  $async.Future<$1.DeleteHighlightResponse> deleteHighlight(
      $grpc.ServiceCall call, $1.DeleteHighlightRequest request);

  $async.Future<$1.AddToHighlightResponse> addToHighlight_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.AddToHighlightRequest> $request) async {
    return addToHighlight($call, await $request);
  }

  $async.Future<$1.AddToHighlightResponse> addToHighlight(
      $grpc.ServiceCall call, $1.AddToHighlightRequest request);

  $async.Future<$1.RemoveFromHighlightResponse> removeFromHighlight_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.RemoveFromHighlightRequest> $request) async {
    return removeFromHighlight($call, await $request);
  }

  $async.Future<$1.RemoveFromHighlightResponse> removeFromHighlight(
      $grpc.ServiceCall call, $1.RemoveFromHighlightRequest request);

  $async.Future<$1.GetHighlightsResponse> getHighlights_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetHighlightsRequest> $request) async {
    return getHighlights($call, await $request);
  }

  $async.Future<$1.GetHighlightsResponse> getHighlights(
      $grpc.ServiceCall call, $1.GetHighlightsRequest request);

  $async.Future<$1.CreateLookingForPartyResponse> createLookingForParty_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.CreateLookingForPartyRequest> $request) async {
    return createLookingForParty($call, await $request);
  }

  $async.Future<$1.CreateLookingForPartyResponse> createLookingForParty(
      $grpc.ServiceCall call, $1.CreateLookingForPartyRequest request);

  $async.Future<$1.RespondToLfpStoryResponse> respondToLfpStory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.RespondToLfpStoryRequest> $request) async {
    return respondToLfpStory($call, await $request);
  }

  $async.Future<$1.RespondToLfpStoryResponse> respondToLfpStory(
      $grpc.ServiceCall call, $1.RespondToLfpStoryRequest request);

  $async.Future<$1.HideStoryFromFeedResponse> hideStoryFromFeed_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.HideStoryFromFeedRequest> $request) async {
    return hideStoryFromFeed($call, await $request);
  }

  $async.Future<$1.HideStoryFromFeedResponse> hideStoryFromFeed(
      $grpc.ServiceCall call, $1.HideStoryFromFeedRequest request);
}
