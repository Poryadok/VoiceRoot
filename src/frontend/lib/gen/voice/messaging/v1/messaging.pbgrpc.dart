// This is a generated file - do not edit.
//
// Generated from voice/messaging/v1/messaging.proto.

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

import 'messaging.pb.dart' as $0;

export 'messaging.pb.dart';

/// Messages, reactions, pins, read receipts. HTTP: /api/v1/messages/**.
///
/// Idempotency (SendMessage): optional client_message_id (UUID), unique per (chat, sender_profile_id).
/// Retry with the same key MUST NOT create a second row; response is gRPC OK with the same Message
/// body as the first successful attempt. Details: docs/microservices/messaging-service.md.
@$pb.GrpcServiceName('voice.messaging.v1.MessagingService')
class MessagingServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  MessagingServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.SendMessageResponse> sendMessage(
    $0.SendMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$sendMessage, request, options: options);
  }

  $grpc.ResponseFuture<$0.EditMessageResponse> editMessage(
    $0.EditMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$editMessage, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteMessageResponse> deleteMessage(
    $0.DeleteMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteMessage, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetMessagesResponse> getMessages(
    $0.GetMessagesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getMessages, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetMessageResponse> getMessage(
    $0.GetMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getMessage, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetThreadMessagesResponse> getThreadMessages(
    $0.GetThreadMessagesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getThreadMessages, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListThreadsResponse> listThreads(
    $0.ListThreadsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listThreads, request, options: options);
  }

  $grpc.ResponseFuture<$0.AddReactionResponse> addReaction(
    $0.AddReactionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$addReaction, request, options: options);
  }

  $grpc.ResponseFuture<$0.RemoveReactionResponse> removeReaction(
    $0.RemoveReactionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeReaction, request, options: options);
  }

  $grpc.ResponseFuture<$0.PinMessageResponse> pinMessage(
    $0.PinMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$pinMessage, request, options: options);
  }

  $grpc.ResponseFuture<$0.UnpinMessageResponse> unpinMessage(
    $0.UnpinMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$unpinMessage, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetPinnedMessagesResponse> getPinnedMessages(
    $0.GetPinnedMessagesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getPinnedMessages, request, options: options);
  }

  $grpc.ResponseFuture<$0.ForwardMessageResponse> forwardMessage(
    $0.ForwardMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$forwardMessage, request, options: options);
  }

  $grpc.ResponseFuture<$0.MarkReadResponse> markRead(
    $0.MarkReadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$markRead, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetReadStateResponse> getReadState(
    $0.GetReadStateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getReadState, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetBulkReadStateResponse> getBulkReadState(
    $0.GetBulkReadStateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getBulkReadState, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetChatListMetadataResponse> getChatListMetadata(
    $0.GetChatListMetadataRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getChatListMetadata, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListSharedMediaResponse> listSharedMedia(
    $0.ListSharedMediaRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listSharedMedia, request, options: options);
  }

  /// Phase 15: Signal pre-key directory for DM E2E (docs/features/encryption.md).
  $grpc.ResponseFuture<$0.UploadPreKeyBundleResponse> uploadPreKeyBundle(
    $0.UploadPreKeyBundleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$uploadPreKeyBundle, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetPreKeyBundleResponse> getPreKeyBundle(
    $0.GetPreKeyBundleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getPreKeyBundle, request, options: options);
  }

  // method descriptors

  static final _$sendMessage =
      $grpc.ClientMethod<$0.SendMessageRequest, $0.SendMessageResponse>(
          '/voice.messaging.v1.MessagingService/SendMessage',
          ($0.SendMessageRequest value) => value.writeToBuffer(),
          $0.SendMessageResponse.fromBuffer);
  static final _$editMessage =
      $grpc.ClientMethod<$0.EditMessageRequest, $0.EditMessageResponse>(
          '/voice.messaging.v1.MessagingService/EditMessage',
          ($0.EditMessageRequest value) => value.writeToBuffer(),
          $0.EditMessageResponse.fromBuffer);
  static final _$deleteMessage =
      $grpc.ClientMethod<$0.DeleteMessageRequest, $0.DeleteMessageResponse>(
          '/voice.messaging.v1.MessagingService/DeleteMessage',
          ($0.DeleteMessageRequest value) => value.writeToBuffer(),
          $0.DeleteMessageResponse.fromBuffer);
  static final _$getMessages =
      $grpc.ClientMethod<$0.GetMessagesRequest, $0.GetMessagesResponse>(
          '/voice.messaging.v1.MessagingService/GetMessages',
          ($0.GetMessagesRequest value) => value.writeToBuffer(),
          $0.GetMessagesResponse.fromBuffer);
  static final _$getMessage =
      $grpc.ClientMethod<$0.GetMessageRequest, $0.GetMessageResponse>(
          '/voice.messaging.v1.MessagingService/GetMessage',
          ($0.GetMessageRequest value) => value.writeToBuffer(),
          $0.GetMessageResponse.fromBuffer);
  static final _$getThreadMessages = $grpc.ClientMethod<
          $0.GetThreadMessagesRequest, $0.GetThreadMessagesResponse>(
      '/voice.messaging.v1.MessagingService/GetThreadMessages',
      ($0.GetThreadMessagesRequest value) => value.writeToBuffer(),
      $0.GetThreadMessagesResponse.fromBuffer);
  static final _$listThreads =
      $grpc.ClientMethod<$0.ListThreadsRequest, $0.ListThreadsResponse>(
          '/voice.messaging.v1.MessagingService/ListThreads',
          ($0.ListThreadsRequest value) => value.writeToBuffer(),
          $0.ListThreadsResponse.fromBuffer);
  static final _$addReaction =
      $grpc.ClientMethod<$0.AddReactionRequest, $0.AddReactionResponse>(
          '/voice.messaging.v1.MessagingService/AddReaction',
          ($0.AddReactionRequest value) => value.writeToBuffer(),
          $0.AddReactionResponse.fromBuffer);
  static final _$removeReaction =
      $grpc.ClientMethod<$0.RemoveReactionRequest, $0.RemoveReactionResponse>(
          '/voice.messaging.v1.MessagingService/RemoveReaction',
          ($0.RemoveReactionRequest value) => value.writeToBuffer(),
          $0.RemoveReactionResponse.fromBuffer);
  static final _$pinMessage =
      $grpc.ClientMethod<$0.PinMessageRequest, $0.PinMessageResponse>(
          '/voice.messaging.v1.MessagingService/PinMessage',
          ($0.PinMessageRequest value) => value.writeToBuffer(),
          $0.PinMessageResponse.fromBuffer);
  static final _$unpinMessage =
      $grpc.ClientMethod<$0.UnpinMessageRequest, $0.UnpinMessageResponse>(
          '/voice.messaging.v1.MessagingService/UnpinMessage',
          ($0.UnpinMessageRequest value) => value.writeToBuffer(),
          $0.UnpinMessageResponse.fromBuffer);
  static final _$getPinnedMessages = $grpc.ClientMethod<
          $0.GetPinnedMessagesRequest, $0.GetPinnedMessagesResponse>(
      '/voice.messaging.v1.MessagingService/GetPinnedMessages',
      ($0.GetPinnedMessagesRequest value) => value.writeToBuffer(),
      $0.GetPinnedMessagesResponse.fromBuffer);
  static final _$forwardMessage =
      $grpc.ClientMethod<$0.ForwardMessageRequest, $0.ForwardMessageResponse>(
          '/voice.messaging.v1.MessagingService/ForwardMessage',
          ($0.ForwardMessageRequest value) => value.writeToBuffer(),
          $0.ForwardMessageResponse.fromBuffer);
  static final _$markRead =
      $grpc.ClientMethod<$0.MarkReadRequest, $0.MarkReadResponse>(
          '/voice.messaging.v1.MessagingService/MarkRead',
          ($0.MarkReadRequest value) => value.writeToBuffer(),
          $0.MarkReadResponse.fromBuffer);
  static final _$getReadState =
      $grpc.ClientMethod<$0.GetReadStateRequest, $0.GetReadStateResponse>(
          '/voice.messaging.v1.MessagingService/GetReadState',
          ($0.GetReadStateRequest value) => value.writeToBuffer(),
          $0.GetReadStateResponse.fromBuffer);
  static final _$getBulkReadState = $grpc.ClientMethod<
          $0.GetBulkReadStateRequest, $0.GetBulkReadStateResponse>(
      '/voice.messaging.v1.MessagingService/GetBulkReadState',
      ($0.GetBulkReadStateRequest value) => value.writeToBuffer(),
      $0.GetBulkReadStateResponse.fromBuffer);
  static final _$getChatListMetadata = $grpc.ClientMethod<
          $0.GetChatListMetadataRequest, $0.GetChatListMetadataResponse>(
      '/voice.messaging.v1.MessagingService/GetChatListMetadata',
      ($0.GetChatListMetadataRequest value) => value.writeToBuffer(),
      $0.GetChatListMetadataResponse.fromBuffer);
  static final _$listSharedMedia =
      $grpc.ClientMethod<$0.ListSharedMediaRequest, $0.ListSharedMediaResponse>(
          '/voice.messaging.v1.MessagingService/ListSharedMedia',
          ($0.ListSharedMediaRequest value) => value.writeToBuffer(),
          $0.ListSharedMediaResponse.fromBuffer);
  static final _$uploadPreKeyBundle = $grpc.ClientMethod<
          $0.UploadPreKeyBundleRequest, $0.UploadPreKeyBundleResponse>(
      '/voice.messaging.v1.MessagingService/UploadPreKeyBundle',
      ($0.UploadPreKeyBundleRequest value) => value.writeToBuffer(),
      $0.UploadPreKeyBundleResponse.fromBuffer);
  static final _$getPreKeyBundle =
      $grpc.ClientMethod<$0.GetPreKeyBundleRequest, $0.GetPreKeyBundleResponse>(
          '/voice.messaging.v1.MessagingService/GetPreKeyBundle',
          ($0.GetPreKeyBundleRequest value) => value.writeToBuffer(),
          $0.GetPreKeyBundleResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.messaging.v1.MessagingService')
abstract class MessagingServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.messaging.v1.MessagingService';

  MessagingServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.SendMessageRequest, $0.SendMessageResponse>(
            'SendMessage',
            sendMessage_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SendMessageRequest.fromBuffer(value),
            ($0.SendMessageResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.EditMessageRequest, $0.EditMessageResponse>(
            'EditMessage',
            editMessage_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.EditMessageRequest.fromBuffer(value),
            ($0.EditMessageResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.DeleteMessageRequest, $0.DeleteMessageResponse>(
            'DeleteMessage',
            deleteMessage_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.DeleteMessageRequest.fromBuffer(value),
            ($0.DeleteMessageResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetMessagesRequest, $0.GetMessagesResponse>(
            'GetMessages',
            getMessages_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetMessagesRequest.fromBuffer(value),
            ($0.GetMessagesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetMessageRequest, $0.GetMessageResponse>(
        'GetMessage',
        getMessage_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetMessageRequest.fromBuffer(value),
        ($0.GetMessageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetThreadMessagesRequest,
            $0.GetThreadMessagesResponse>(
        'GetThreadMessages',
        getThreadMessages_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetThreadMessagesRequest.fromBuffer(value),
        ($0.GetThreadMessagesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListThreadsRequest, $0.ListThreadsResponse>(
            'ListThreads',
            listThreads_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListThreadsRequest.fromBuffer(value),
            ($0.ListThreadsResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.AddReactionRequest, $0.AddReactionResponse>(
            'AddReaction',
            addReaction_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.AddReactionRequest.fromBuffer(value),
            ($0.AddReactionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RemoveReactionRequest,
            $0.RemoveReactionResponse>(
        'RemoveReaction',
        removeReaction_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RemoveReactionRequest.fromBuffer(value),
        ($0.RemoveReactionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.PinMessageRequest, $0.PinMessageResponse>(
        'PinMessage',
        pinMessage_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.PinMessageRequest.fromBuffer(value),
        ($0.PinMessageResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.UnpinMessageRequest, $0.UnpinMessageResponse>(
            'UnpinMessage',
            unpinMessage_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.UnpinMessageRequest.fromBuffer(value),
            ($0.UnpinMessageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetPinnedMessagesRequest,
            $0.GetPinnedMessagesResponse>(
        'GetPinnedMessages',
        getPinnedMessages_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetPinnedMessagesRequest.fromBuffer(value),
        ($0.GetPinnedMessagesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ForwardMessageRequest,
            $0.ForwardMessageResponse>(
        'ForwardMessage',
        forwardMessage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ForwardMessageRequest.fromBuffer(value),
        ($0.ForwardMessageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.MarkReadRequest, $0.MarkReadResponse>(
        'MarkRead',
        markRead_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.MarkReadRequest.fromBuffer(value),
        ($0.MarkReadResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetReadStateRequest, $0.GetReadStateResponse>(
            'GetReadState',
            getReadState_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetReadStateRequest.fromBuffer(value),
            ($0.GetReadStateResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetBulkReadStateRequest,
            $0.GetBulkReadStateResponse>(
        'GetBulkReadState',
        getBulkReadState_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetBulkReadStateRequest.fromBuffer(value),
        ($0.GetBulkReadStateResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetChatListMetadataRequest,
            $0.GetChatListMetadataResponse>(
        'GetChatListMetadata',
        getChatListMetadata_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetChatListMetadataRequest.fromBuffer(value),
        ($0.GetChatListMetadataResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListSharedMediaRequest,
            $0.ListSharedMediaResponse>(
        'ListSharedMedia',
        listSharedMedia_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListSharedMediaRequest.fromBuffer(value),
        ($0.ListSharedMediaResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UploadPreKeyBundleRequest,
            $0.UploadPreKeyBundleResponse>(
        'UploadPreKeyBundle',
        uploadPreKeyBundle_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UploadPreKeyBundleRequest.fromBuffer(value),
        ($0.UploadPreKeyBundleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetPreKeyBundleRequest,
            $0.GetPreKeyBundleResponse>(
        'GetPreKeyBundle',
        getPreKeyBundle_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetPreKeyBundleRequest.fromBuffer(value),
        ($0.GetPreKeyBundleResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.SendMessageResponse> sendMessage_Pre($grpc.ServiceCall $call,
      $async.Future<$0.SendMessageRequest> $request) async {
    return sendMessage($call, await $request);
  }

  $async.Future<$0.SendMessageResponse> sendMessage(
      $grpc.ServiceCall call, $0.SendMessageRequest request);

  $async.Future<$0.EditMessageResponse> editMessage_Pre($grpc.ServiceCall $call,
      $async.Future<$0.EditMessageRequest> $request) async {
    return editMessage($call, await $request);
  }

  $async.Future<$0.EditMessageResponse> editMessage(
      $grpc.ServiceCall call, $0.EditMessageRequest request);

  $async.Future<$0.DeleteMessageResponse> deleteMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DeleteMessageRequest> $request) async {
    return deleteMessage($call, await $request);
  }

  $async.Future<$0.DeleteMessageResponse> deleteMessage(
      $grpc.ServiceCall call, $0.DeleteMessageRequest request);

  $async.Future<$0.GetMessagesResponse> getMessages_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetMessagesRequest> $request) async {
    return getMessages($call, await $request);
  }

  $async.Future<$0.GetMessagesResponse> getMessages(
      $grpc.ServiceCall call, $0.GetMessagesRequest request);

  $async.Future<$0.GetMessageResponse> getMessage_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetMessageRequest> $request) async {
    return getMessage($call, await $request);
  }

  $async.Future<$0.GetMessageResponse> getMessage(
      $grpc.ServiceCall call, $0.GetMessageRequest request);

  $async.Future<$0.GetThreadMessagesResponse> getThreadMessages_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetThreadMessagesRequest> $request) async {
    return getThreadMessages($call, await $request);
  }

  $async.Future<$0.GetThreadMessagesResponse> getThreadMessages(
      $grpc.ServiceCall call, $0.GetThreadMessagesRequest request);

  $async.Future<$0.ListThreadsResponse> listThreads_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListThreadsRequest> $request) async {
    return listThreads($call, await $request);
  }

  $async.Future<$0.ListThreadsResponse> listThreads(
      $grpc.ServiceCall call, $0.ListThreadsRequest request);

  $async.Future<$0.AddReactionResponse> addReaction_Pre($grpc.ServiceCall $call,
      $async.Future<$0.AddReactionRequest> $request) async {
    return addReaction($call, await $request);
  }

  $async.Future<$0.AddReactionResponse> addReaction(
      $grpc.ServiceCall call, $0.AddReactionRequest request);

  $async.Future<$0.RemoveReactionResponse> removeReaction_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RemoveReactionRequest> $request) async {
    return removeReaction($call, await $request);
  }

  $async.Future<$0.RemoveReactionResponse> removeReaction(
      $grpc.ServiceCall call, $0.RemoveReactionRequest request);

  $async.Future<$0.PinMessageResponse> pinMessage_Pre($grpc.ServiceCall $call,
      $async.Future<$0.PinMessageRequest> $request) async {
    return pinMessage($call, await $request);
  }

  $async.Future<$0.PinMessageResponse> pinMessage(
      $grpc.ServiceCall call, $0.PinMessageRequest request);

  $async.Future<$0.UnpinMessageResponse> unpinMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UnpinMessageRequest> $request) async {
    return unpinMessage($call, await $request);
  }

  $async.Future<$0.UnpinMessageResponse> unpinMessage(
      $grpc.ServiceCall call, $0.UnpinMessageRequest request);

  $async.Future<$0.GetPinnedMessagesResponse> getPinnedMessages_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetPinnedMessagesRequest> $request) async {
    return getPinnedMessages($call, await $request);
  }

  $async.Future<$0.GetPinnedMessagesResponse> getPinnedMessages(
      $grpc.ServiceCall call, $0.GetPinnedMessagesRequest request);

  $async.Future<$0.ForwardMessageResponse> forwardMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ForwardMessageRequest> $request) async {
    return forwardMessage($call, await $request);
  }

  $async.Future<$0.ForwardMessageResponse> forwardMessage(
      $grpc.ServiceCall call, $0.ForwardMessageRequest request);

  $async.Future<$0.MarkReadResponse> markRead_Pre($grpc.ServiceCall $call,
      $async.Future<$0.MarkReadRequest> $request) async {
    return markRead($call, await $request);
  }

  $async.Future<$0.MarkReadResponse> markRead(
      $grpc.ServiceCall call, $0.MarkReadRequest request);

  $async.Future<$0.GetReadStateResponse> getReadState_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetReadStateRequest> $request) async {
    return getReadState($call, await $request);
  }

  $async.Future<$0.GetReadStateResponse> getReadState(
      $grpc.ServiceCall call, $0.GetReadStateRequest request);

  $async.Future<$0.GetBulkReadStateResponse> getBulkReadState_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetBulkReadStateRequest> $request) async {
    return getBulkReadState($call, await $request);
  }

  $async.Future<$0.GetBulkReadStateResponse> getBulkReadState(
      $grpc.ServiceCall call, $0.GetBulkReadStateRequest request);

  $async.Future<$0.GetChatListMetadataResponse> getChatListMetadata_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetChatListMetadataRequest> $request) async {
    return getChatListMetadata($call, await $request);
  }

  $async.Future<$0.GetChatListMetadataResponse> getChatListMetadata(
      $grpc.ServiceCall call, $0.GetChatListMetadataRequest request);

  $async.Future<$0.ListSharedMediaResponse> listSharedMedia_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListSharedMediaRequest> $request) async {
    return listSharedMedia($call, await $request);
  }

  $async.Future<$0.ListSharedMediaResponse> listSharedMedia(
      $grpc.ServiceCall call, $0.ListSharedMediaRequest request);

  $async.Future<$0.UploadPreKeyBundleResponse> uploadPreKeyBundle_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UploadPreKeyBundleRequest> $request) async {
    return uploadPreKeyBundle($call, await $request);
  }

  $async.Future<$0.UploadPreKeyBundleResponse> uploadPreKeyBundle(
      $grpc.ServiceCall call, $0.UploadPreKeyBundleRequest request);

  $async.Future<$0.GetPreKeyBundleResponse> getPreKeyBundle_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetPreKeyBundleRequest> $request) async {
    return getPreKeyBundle($call, await $request);
  }

  $async.Future<$0.GetPreKeyBundleResponse> getPreKeyBundle(
      $grpc.ServiceCall call, $0.GetPreKeyBundleRequest request);
}
