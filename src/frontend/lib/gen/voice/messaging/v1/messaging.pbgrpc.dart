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

import 'messaging.pb.dart' as $1;
import 'report_target.pb.dart' as $0;

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

  /// Protected service:moderation only; no viewer/content authority.
  $grpc.ResponseFuture<$0.ResolveReportTargetResponse> resolveReportTarget(
    $0.ResolveReportTargetRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveReportTarget, request, options: options);
  }

  /// @voice.security=protected;callers=service:bot
  $grpc.ResponseFuture<$1.SendGameEventMessageResponse> sendGameEventMessage(
    $1.SendGameEventMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$sendGameEventMessage, request, options: options);
  }

  $grpc.ResponseFuture<$1.SendMessageResponse> sendMessage(
    $1.SendMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$sendMessage, request, options: options);
  }

  /// @voice.security=protected;callers=service:gateway
  /// T15 internal ingress: the exact device-signed compact JWS and Auth
  /// assertion are forwarded unchanged; sender/profile/chat authority is
  /// derived by Messaging from Auth binding and chat policy.
  $grpc.ResponseFuture<$1.ApplyGameMessageResponse> applyGameMessage(
    $1.ApplyGameMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$applyGameMessage, request, options: options);
  }

  /// @voice.security=protected;callers=service:moderation
  $grpc.ResponseFuture<$1.TombstoneGameMessageResponse> tombstoneGameMessage(
    $1.TombstoneGameMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$tombstoneGameMessage, request, options: options);
  }

  $grpc.ResponseFuture<$1.EditMessageResponse> editMessage(
    $1.EditMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$editMessage, request, options: options);
  }

  $grpc.ResponseFuture<$1.DeleteMessageResponse> deleteMessage(
    $1.DeleteMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteMessage, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetMessagesResponse> getMessages(
    $1.GetMessagesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getMessages, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetMessageResponse> getMessage(
    $1.GetMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getMessage, request, options: options);
  }

  /// @voice.security=protected;callers=service:gameintegration
  /// Re-resolves immutable game card authority under Messaging chat ACL.
  $grpc.ResponseFuture<$1.ResolveGameActionResponse> resolveGameAction(
    $1.ResolveGameActionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveGameAction, request, options: options);
  }

  /// @voice.security=protected;callers=service:gameintegration
  /// Projects a committed immutable result onto the separately stored card status.
  $grpc.ResponseFuture<$1.ProjectGameActionResultResponse>
      projectGameActionResult(
    $1.ProjectGameActionResultRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$projectGameActionResult, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:gameintegration
  /// Starts or resumes the exact durable managed-chat retention purge operation.
  $grpc.ResponseFuture<$1.PurgeManagedChatContentResponse>
      purgeManagedChatContent(
    $1.PurgeManagedChatContentRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$purgeManagedChatContent, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.GetThreadMessagesResponse> getThreadMessages(
    $1.GetThreadMessagesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getThreadMessages, request, options: options);
  }

  $grpc.ResponseFuture<$1.ListThreadsResponse> listThreads(
    $1.ListThreadsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listThreads, request, options: options);
  }

  $grpc.ResponseFuture<$1.AddReactionResponse> addReaction(
    $1.AddReactionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$addReaction, request, options: options);
  }

  $grpc.ResponseFuture<$1.RemoveReactionResponse> removeReaction(
    $1.RemoveReactionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeReaction, request, options: options);
  }

  $grpc.ResponseFuture<$1.PinMessageResponse> pinMessage(
    $1.PinMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$pinMessage, request, options: options);
  }

  $grpc.ResponseFuture<$1.UnpinMessageResponse> unpinMessage(
    $1.UnpinMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$unpinMessage, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetPinnedMessagesResponse> getPinnedMessages(
    $1.GetPinnedMessagesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getPinnedMessages, request, options: options);
  }

  /// BOT-B: bulk unpin for bot uninstall cleanup (sender_profile_id + chat_ids).
  $grpc.ResponseFuture<$1.UnpinMessagesBySenderInChatsResponse>
      unpinMessagesBySenderInChats(
    $1.UnpinMessagesBySenderInChatsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$unpinMessagesBySenderInChats, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.ForwardMessageResponse> forwardMessage(
    $1.ForwardMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$forwardMessage, request, options: options);
  }

  $grpc.ResponseFuture<$1.MarkReadResponse> markRead(
    $1.MarkReadRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$markRead, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetReadStateResponse> getReadState(
    $1.GetReadStateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getReadState, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetBulkReadStateResponse> getBulkReadState(
    $1.GetBulkReadStateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getBulkReadState, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetChatListMetadataResponse> getChatListMetadata(
    $1.GetChatListMetadataRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getChatListMetadata, request, options: options);
  }

  $grpc.ResponseFuture<$1.ListSharedMediaResponse> listSharedMedia(
    $1.ListSharedMediaRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listSharedMedia, request, options: options);
  }

  /// Signal pre-key directory for DM E2E — docs/features/encryption.md.
  $grpc.ResponseFuture<$1.UploadPreKeyBundleResponse> uploadPreKeyBundle(
    $1.UploadPreKeyBundleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$uploadPreKeyBundle, request, options: options);
  }

  $grpc.ResponseFuture<$1.GetPreKeyBundleResponse> getPreKeyBundle(
    $1.GetPreKeyBundleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getPreKeyBundle, request, options: options);
  }

  /// Schedule lifecycle handlers are introduced in a later slice. These declarations
  /// establish the additive wire contract for callers and generated clients.
  $grpc.ResponseFuture<$1.ListScheduledMessagesResponse> listScheduledMessages(
    $1.ListScheduledMessagesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listScheduledMessages, request, options: options);
  }

  $grpc.ResponseFuture<$1.UpdateScheduledMessageResponse>
      updateScheduledMessage(
    $1.UpdateScheduledMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateScheduledMessage, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.CancelScheduledMessageResponse>
      cancelScheduledMessage(
    $1.CancelScheduledMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$cancelScheduledMessage, request,
        options: options);
  }

  $grpc.ResponseFuture<$1.SendScheduledMessageNowResponse>
      sendScheduledMessageNow(
    $1.SendScheduledMessageNowRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$sendScheduledMessageNow, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:space
  $grpc.ResponseFuture<$1.ApplySpaceLifecycleFenceResponse>
      applySpaceLifecycleFence(
    $1.ApplySpaceLifecycleFenceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$applySpaceLifecycleFence, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:space
  $grpc.ResponseFuture<$1.PurgeSpaceResponse> purgeSpace(
    $1.PurgeSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$purgeSpace, request, options: options);
  }

  /// Chat may only read Messaging's already committed participant receipt. This
  /// lookup never starts or advances Messaging purge.
  /// @voice.security=protected;callers=service:chat
  $grpc.ResponseFuture<$1.GetSpacePurgeReceiptResponse> getSpacePurgeReceipt(
    $1.GetSpacePurgeReceiptRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSpacePurgeReceipt, request, options: options);
  }

  /// @voice.security=protected;callers=service:space
  $grpc.ResponseFuture<$1.ImportSpacePurgeManifestPageResponse>
      importSpacePurgeManifestPage(
    $1.ImportSpacePurgeManifestPageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$importSpacePurgeManifestPage, request,
        options: options);
  }

  // method descriptors

  static final _$resolveReportTarget = $grpc.ClientMethod<
          $0.ResolveReportTargetRequest, $0.ResolveReportTargetResponse>(
      '/voice.messaging.v1.MessagingService/ResolveReportTarget',
      ($0.ResolveReportTargetRequest value) => value.writeToBuffer(),
      $0.ResolveReportTargetResponse.fromBuffer);
  static final _$sendGameEventMessage = $grpc.ClientMethod<
          $1.SendGameEventMessageRequest, $1.SendGameEventMessageResponse>(
      '/voice.messaging.v1.MessagingService/SendGameEventMessage',
      ($1.SendGameEventMessageRequest value) => value.writeToBuffer(),
      $1.SendGameEventMessageResponse.fromBuffer);
  static final _$sendMessage =
      $grpc.ClientMethod<$1.SendMessageRequest, $1.SendMessageResponse>(
          '/voice.messaging.v1.MessagingService/SendMessage',
          ($1.SendMessageRequest value) => value.writeToBuffer(),
          $1.SendMessageResponse.fromBuffer);
  static final _$applyGameMessage = $grpc.ClientMethod<
          $1.ApplyGameMessageRequest, $1.ApplyGameMessageResponse>(
      '/voice.messaging.v1.MessagingService/ApplyGameMessage',
      ($1.ApplyGameMessageRequest value) => value.writeToBuffer(),
      $1.ApplyGameMessageResponse.fromBuffer);
  static final _$tombstoneGameMessage = $grpc.ClientMethod<
          $1.TombstoneGameMessageRequest, $1.TombstoneGameMessageResponse>(
      '/voice.messaging.v1.MessagingService/TombstoneGameMessage',
      ($1.TombstoneGameMessageRequest value) => value.writeToBuffer(),
      $1.TombstoneGameMessageResponse.fromBuffer);
  static final _$editMessage =
      $grpc.ClientMethod<$1.EditMessageRequest, $1.EditMessageResponse>(
          '/voice.messaging.v1.MessagingService/EditMessage',
          ($1.EditMessageRequest value) => value.writeToBuffer(),
          $1.EditMessageResponse.fromBuffer);
  static final _$deleteMessage =
      $grpc.ClientMethod<$1.DeleteMessageRequest, $1.DeleteMessageResponse>(
          '/voice.messaging.v1.MessagingService/DeleteMessage',
          ($1.DeleteMessageRequest value) => value.writeToBuffer(),
          $1.DeleteMessageResponse.fromBuffer);
  static final _$getMessages =
      $grpc.ClientMethod<$1.GetMessagesRequest, $1.GetMessagesResponse>(
          '/voice.messaging.v1.MessagingService/GetMessages',
          ($1.GetMessagesRequest value) => value.writeToBuffer(),
          $1.GetMessagesResponse.fromBuffer);
  static final _$getMessage =
      $grpc.ClientMethod<$1.GetMessageRequest, $1.GetMessageResponse>(
          '/voice.messaging.v1.MessagingService/GetMessage',
          ($1.GetMessageRequest value) => value.writeToBuffer(),
          $1.GetMessageResponse.fromBuffer);
  static final _$resolveGameAction = $grpc.ClientMethod<
          $1.ResolveGameActionRequest, $1.ResolveGameActionResponse>(
      '/voice.messaging.v1.MessagingService/ResolveGameAction',
      ($1.ResolveGameActionRequest value) => value.writeToBuffer(),
      $1.ResolveGameActionResponse.fromBuffer);
  static final _$projectGameActionResult = $grpc.ClientMethod<
          $1.ProjectGameActionResultRequest,
          $1.ProjectGameActionResultResponse>(
      '/voice.messaging.v1.MessagingService/ProjectGameActionResult',
      ($1.ProjectGameActionResultRequest value) => value.writeToBuffer(),
      $1.ProjectGameActionResultResponse.fromBuffer);
  static final _$purgeManagedChatContent = $grpc.ClientMethod<
          $1.PurgeManagedChatContentRequest,
          $1.PurgeManagedChatContentResponse>(
      '/voice.messaging.v1.MessagingService/PurgeManagedChatContent',
      ($1.PurgeManagedChatContentRequest value) => value.writeToBuffer(),
      $1.PurgeManagedChatContentResponse.fromBuffer);
  static final _$getThreadMessages = $grpc.ClientMethod<
          $1.GetThreadMessagesRequest, $1.GetThreadMessagesResponse>(
      '/voice.messaging.v1.MessagingService/GetThreadMessages',
      ($1.GetThreadMessagesRequest value) => value.writeToBuffer(),
      $1.GetThreadMessagesResponse.fromBuffer);
  static final _$listThreads =
      $grpc.ClientMethod<$1.ListThreadsRequest, $1.ListThreadsResponse>(
          '/voice.messaging.v1.MessagingService/ListThreads',
          ($1.ListThreadsRequest value) => value.writeToBuffer(),
          $1.ListThreadsResponse.fromBuffer);
  static final _$addReaction =
      $grpc.ClientMethod<$1.AddReactionRequest, $1.AddReactionResponse>(
          '/voice.messaging.v1.MessagingService/AddReaction',
          ($1.AddReactionRequest value) => value.writeToBuffer(),
          $1.AddReactionResponse.fromBuffer);
  static final _$removeReaction =
      $grpc.ClientMethod<$1.RemoveReactionRequest, $1.RemoveReactionResponse>(
          '/voice.messaging.v1.MessagingService/RemoveReaction',
          ($1.RemoveReactionRequest value) => value.writeToBuffer(),
          $1.RemoveReactionResponse.fromBuffer);
  static final _$pinMessage =
      $grpc.ClientMethod<$1.PinMessageRequest, $1.PinMessageResponse>(
          '/voice.messaging.v1.MessagingService/PinMessage',
          ($1.PinMessageRequest value) => value.writeToBuffer(),
          $1.PinMessageResponse.fromBuffer);
  static final _$unpinMessage =
      $grpc.ClientMethod<$1.UnpinMessageRequest, $1.UnpinMessageResponse>(
          '/voice.messaging.v1.MessagingService/UnpinMessage',
          ($1.UnpinMessageRequest value) => value.writeToBuffer(),
          $1.UnpinMessageResponse.fromBuffer);
  static final _$getPinnedMessages = $grpc.ClientMethod<
          $1.GetPinnedMessagesRequest, $1.GetPinnedMessagesResponse>(
      '/voice.messaging.v1.MessagingService/GetPinnedMessages',
      ($1.GetPinnedMessagesRequest value) => value.writeToBuffer(),
      $1.GetPinnedMessagesResponse.fromBuffer);
  static final _$unpinMessagesBySenderInChats = $grpc.ClientMethod<
          $1.UnpinMessagesBySenderInChatsRequest,
          $1.UnpinMessagesBySenderInChatsResponse>(
      '/voice.messaging.v1.MessagingService/UnpinMessagesBySenderInChats',
      ($1.UnpinMessagesBySenderInChatsRequest value) => value.writeToBuffer(),
      $1.UnpinMessagesBySenderInChatsResponse.fromBuffer);
  static final _$forwardMessage =
      $grpc.ClientMethod<$1.ForwardMessageRequest, $1.ForwardMessageResponse>(
          '/voice.messaging.v1.MessagingService/ForwardMessage',
          ($1.ForwardMessageRequest value) => value.writeToBuffer(),
          $1.ForwardMessageResponse.fromBuffer);
  static final _$markRead =
      $grpc.ClientMethod<$1.MarkReadRequest, $1.MarkReadResponse>(
          '/voice.messaging.v1.MessagingService/MarkRead',
          ($1.MarkReadRequest value) => value.writeToBuffer(),
          $1.MarkReadResponse.fromBuffer);
  static final _$getReadState =
      $grpc.ClientMethod<$1.GetReadStateRequest, $1.GetReadStateResponse>(
          '/voice.messaging.v1.MessagingService/GetReadState',
          ($1.GetReadStateRequest value) => value.writeToBuffer(),
          $1.GetReadStateResponse.fromBuffer);
  static final _$getBulkReadState = $grpc.ClientMethod<
          $1.GetBulkReadStateRequest, $1.GetBulkReadStateResponse>(
      '/voice.messaging.v1.MessagingService/GetBulkReadState',
      ($1.GetBulkReadStateRequest value) => value.writeToBuffer(),
      $1.GetBulkReadStateResponse.fromBuffer);
  static final _$getChatListMetadata = $grpc.ClientMethod<
          $1.GetChatListMetadataRequest, $1.GetChatListMetadataResponse>(
      '/voice.messaging.v1.MessagingService/GetChatListMetadata',
      ($1.GetChatListMetadataRequest value) => value.writeToBuffer(),
      $1.GetChatListMetadataResponse.fromBuffer);
  static final _$listSharedMedia =
      $grpc.ClientMethod<$1.ListSharedMediaRequest, $1.ListSharedMediaResponse>(
          '/voice.messaging.v1.MessagingService/ListSharedMedia',
          ($1.ListSharedMediaRequest value) => value.writeToBuffer(),
          $1.ListSharedMediaResponse.fromBuffer);
  static final _$uploadPreKeyBundle = $grpc.ClientMethod<
          $1.UploadPreKeyBundleRequest, $1.UploadPreKeyBundleResponse>(
      '/voice.messaging.v1.MessagingService/UploadPreKeyBundle',
      ($1.UploadPreKeyBundleRequest value) => value.writeToBuffer(),
      $1.UploadPreKeyBundleResponse.fromBuffer);
  static final _$getPreKeyBundle =
      $grpc.ClientMethod<$1.GetPreKeyBundleRequest, $1.GetPreKeyBundleResponse>(
          '/voice.messaging.v1.MessagingService/GetPreKeyBundle',
          ($1.GetPreKeyBundleRequest value) => value.writeToBuffer(),
          $1.GetPreKeyBundleResponse.fromBuffer);
  static final _$listScheduledMessages = $grpc.ClientMethod<
          $1.ListScheduledMessagesRequest, $1.ListScheduledMessagesResponse>(
      '/voice.messaging.v1.MessagingService/ListScheduledMessages',
      ($1.ListScheduledMessagesRequest value) => value.writeToBuffer(),
      $1.ListScheduledMessagesResponse.fromBuffer);
  static final _$updateScheduledMessage = $grpc.ClientMethod<
          $1.UpdateScheduledMessageRequest, $1.UpdateScheduledMessageResponse>(
      '/voice.messaging.v1.MessagingService/UpdateScheduledMessage',
      ($1.UpdateScheduledMessageRequest value) => value.writeToBuffer(),
      $1.UpdateScheduledMessageResponse.fromBuffer);
  static final _$cancelScheduledMessage = $grpc.ClientMethod<
          $1.CancelScheduledMessageRequest, $1.CancelScheduledMessageResponse>(
      '/voice.messaging.v1.MessagingService/CancelScheduledMessage',
      ($1.CancelScheduledMessageRequest value) => value.writeToBuffer(),
      $1.CancelScheduledMessageResponse.fromBuffer);
  static final _$sendScheduledMessageNow = $grpc.ClientMethod<
          $1.SendScheduledMessageNowRequest,
          $1.SendScheduledMessageNowResponse>(
      '/voice.messaging.v1.MessagingService/SendScheduledMessageNow',
      ($1.SendScheduledMessageNowRequest value) => value.writeToBuffer(),
      $1.SendScheduledMessageNowResponse.fromBuffer);
  static final _$applySpaceLifecycleFence = $grpc.ClientMethod<
          $1.ApplySpaceLifecycleFenceRequest,
          $1.ApplySpaceLifecycleFenceResponse>(
      '/voice.messaging.v1.MessagingService/ApplySpaceLifecycleFence',
      ($1.ApplySpaceLifecycleFenceRequest value) => value.writeToBuffer(),
      $1.ApplySpaceLifecycleFenceResponse.fromBuffer);
  static final _$purgeSpace =
      $grpc.ClientMethod<$1.PurgeSpaceRequest, $1.PurgeSpaceResponse>(
          '/voice.messaging.v1.MessagingService/PurgeSpace',
          ($1.PurgeSpaceRequest value) => value.writeToBuffer(),
          $1.PurgeSpaceResponse.fromBuffer);
  static final _$getSpacePurgeReceipt = $grpc.ClientMethod<
          $1.GetSpacePurgeReceiptRequest, $1.GetSpacePurgeReceiptResponse>(
      '/voice.messaging.v1.MessagingService/GetSpacePurgeReceipt',
      ($1.GetSpacePurgeReceiptRequest value) => value.writeToBuffer(),
      $1.GetSpacePurgeReceiptResponse.fromBuffer);
  static final _$importSpacePurgeManifestPage = $grpc.ClientMethod<
          $1.ImportSpacePurgeManifestPageRequest,
          $1.ImportSpacePurgeManifestPageResponse>(
      '/voice.messaging.v1.MessagingService/ImportSpacePurgeManifestPage',
      ($1.ImportSpacePurgeManifestPageRequest value) => value.writeToBuffer(),
      $1.ImportSpacePurgeManifestPageResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.messaging.v1.MessagingService')
abstract class MessagingServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.messaging.v1.MessagingService';

  MessagingServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ResolveReportTargetRequest,
            $0.ResolveReportTargetResponse>(
        'ResolveReportTarget',
        resolveReportTarget_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ResolveReportTargetRequest.fromBuffer(value),
        ($0.ResolveReportTargetResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.SendGameEventMessageRequest,
            $1.SendGameEventMessageResponse>(
        'SendGameEventMessage',
        sendGameEventMessage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.SendGameEventMessageRequest.fromBuffer(value),
        ($1.SendGameEventMessageResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.SendMessageRequest, $1.SendMessageResponse>(
            'SendMessage',
            sendMessage_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.SendMessageRequest.fromBuffer(value),
            ($1.SendMessageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ApplyGameMessageRequest,
            $1.ApplyGameMessageResponse>(
        'ApplyGameMessage',
        applyGameMessage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ApplyGameMessageRequest.fromBuffer(value),
        ($1.ApplyGameMessageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.TombstoneGameMessageRequest,
            $1.TombstoneGameMessageResponse>(
        'TombstoneGameMessage',
        tombstoneGameMessage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.TombstoneGameMessageRequest.fromBuffer(value),
        ($1.TombstoneGameMessageResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.EditMessageRequest, $1.EditMessageResponse>(
            'EditMessage',
            editMessage_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.EditMessageRequest.fromBuffer(value),
            ($1.EditMessageResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.DeleteMessageRequest, $1.DeleteMessageResponse>(
            'DeleteMessage',
            deleteMessage_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.DeleteMessageRequest.fromBuffer(value),
            ($1.DeleteMessageResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.GetMessagesRequest, $1.GetMessagesResponse>(
            'GetMessages',
            getMessages_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.GetMessagesRequest.fromBuffer(value),
            ($1.GetMessagesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetMessageRequest, $1.GetMessageResponse>(
        'GetMessage',
        getMessage_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.GetMessageRequest.fromBuffer(value),
        ($1.GetMessageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ResolveGameActionRequest,
            $1.ResolveGameActionResponse>(
        'ResolveGameAction',
        resolveGameAction_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ResolveGameActionRequest.fromBuffer(value),
        ($1.ResolveGameActionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ProjectGameActionResultRequest,
            $1.ProjectGameActionResultResponse>(
        'ProjectGameActionResult',
        projectGameActionResult_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ProjectGameActionResultRequest.fromBuffer(value),
        ($1.ProjectGameActionResultResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.PurgeManagedChatContentRequest,
            $1.PurgeManagedChatContentResponse>(
        'PurgeManagedChatContent',
        purgeManagedChatContent_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.PurgeManagedChatContentRequest.fromBuffer(value),
        ($1.PurgeManagedChatContentResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetThreadMessagesRequest,
            $1.GetThreadMessagesResponse>(
        'GetThreadMessages',
        getThreadMessages_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetThreadMessagesRequest.fromBuffer(value),
        ($1.GetThreadMessagesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.ListThreadsRequest, $1.ListThreadsResponse>(
            'ListThreads',
            listThreads_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.ListThreadsRequest.fromBuffer(value),
            ($1.ListThreadsResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.AddReactionRequest, $1.AddReactionResponse>(
            'AddReaction',
            addReaction_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.AddReactionRequest.fromBuffer(value),
            ($1.AddReactionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.RemoveReactionRequest,
            $1.RemoveReactionResponse>(
        'RemoveReaction',
        removeReaction_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.RemoveReactionRequest.fromBuffer(value),
        ($1.RemoveReactionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.PinMessageRequest, $1.PinMessageResponse>(
        'PinMessage',
        pinMessage_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.PinMessageRequest.fromBuffer(value),
        ($1.PinMessageResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.UnpinMessageRequest, $1.UnpinMessageResponse>(
            'UnpinMessage',
            unpinMessage_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.UnpinMessageRequest.fromBuffer(value),
            ($1.UnpinMessageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetPinnedMessagesRequest,
            $1.GetPinnedMessagesResponse>(
        'GetPinnedMessages',
        getPinnedMessages_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetPinnedMessagesRequest.fromBuffer(value),
        ($1.GetPinnedMessagesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.UnpinMessagesBySenderInChatsRequest,
            $1.UnpinMessagesBySenderInChatsResponse>(
        'UnpinMessagesBySenderInChats',
        unpinMessagesBySenderInChats_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.UnpinMessagesBySenderInChatsRequest.fromBuffer(value),
        ($1.UnpinMessagesBySenderInChatsResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ForwardMessageRequest,
            $1.ForwardMessageResponse>(
        'ForwardMessage',
        forwardMessage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ForwardMessageRequest.fromBuffer(value),
        ($1.ForwardMessageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.MarkReadRequest, $1.MarkReadResponse>(
        'MarkRead',
        markRead_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.MarkReadRequest.fromBuffer(value),
        ($1.MarkReadResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$1.GetReadStateRequest, $1.GetReadStateResponse>(
            'GetReadState',
            getReadState_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $1.GetReadStateRequest.fromBuffer(value),
            ($1.GetReadStateResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetBulkReadStateRequest,
            $1.GetBulkReadStateResponse>(
        'GetBulkReadState',
        getBulkReadState_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetBulkReadStateRequest.fromBuffer(value),
        ($1.GetBulkReadStateResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetChatListMetadataRequest,
            $1.GetChatListMetadataResponse>(
        'GetChatListMetadata',
        getChatListMetadata_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetChatListMetadataRequest.fromBuffer(value),
        ($1.GetChatListMetadataResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ListSharedMediaRequest,
            $1.ListSharedMediaResponse>(
        'ListSharedMedia',
        listSharedMedia_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ListSharedMediaRequest.fromBuffer(value),
        ($1.ListSharedMediaResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.UploadPreKeyBundleRequest,
            $1.UploadPreKeyBundleResponse>(
        'UploadPreKeyBundle',
        uploadPreKeyBundle_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.UploadPreKeyBundleRequest.fromBuffer(value),
        ($1.UploadPreKeyBundleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetPreKeyBundleRequest,
            $1.GetPreKeyBundleResponse>(
        'GetPreKeyBundle',
        getPreKeyBundle_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetPreKeyBundleRequest.fromBuffer(value),
        ($1.GetPreKeyBundleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ListScheduledMessagesRequest,
            $1.ListScheduledMessagesResponse>(
        'ListScheduledMessages',
        listScheduledMessages_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ListScheduledMessagesRequest.fromBuffer(value),
        ($1.ListScheduledMessagesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.UpdateScheduledMessageRequest,
            $1.UpdateScheduledMessageResponse>(
        'UpdateScheduledMessage',
        updateScheduledMessage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.UpdateScheduledMessageRequest.fromBuffer(value),
        ($1.UpdateScheduledMessageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.CancelScheduledMessageRequest,
            $1.CancelScheduledMessageResponse>(
        'CancelScheduledMessage',
        cancelScheduledMessage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.CancelScheduledMessageRequest.fromBuffer(value),
        ($1.CancelScheduledMessageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.SendScheduledMessageNowRequest,
            $1.SendScheduledMessageNowResponse>(
        'SendScheduledMessageNow',
        sendScheduledMessageNow_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.SendScheduledMessageNowRequest.fromBuffer(value),
        ($1.SendScheduledMessageNowResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ApplySpaceLifecycleFenceRequest,
            $1.ApplySpaceLifecycleFenceResponse>(
        'ApplySpaceLifecycleFence',
        applySpaceLifecycleFence_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ApplySpaceLifecycleFenceRequest.fromBuffer(value),
        ($1.ApplySpaceLifecycleFenceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.PurgeSpaceRequest, $1.PurgeSpaceResponse>(
        'PurgeSpace',
        purgeSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $1.PurgeSpaceRequest.fromBuffer(value),
        ($1.PurgeSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.GetSpacePurgeReceiptRequest,
            $1.GetSpacePurgeReceiptResponse>(
        'GetSpacePurgeReceipt',
        getSpacePurgeReceipt_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.GetSpacePurgeReceiptRequest.fromBuffer(value),
        ($1.GetSpacePurgeReceiptResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$1.ImportSpacePurgeManifestPageRequest,
            $1.ImportSpacePurgeManifestPageResponse>(
        'ImportSpacePurgeManifestPage',
        importSpacePurgeManifestPage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $1.ImportSpacePurgeManifestPageRequest.fromBuffer(value),
        ($1.ImportSpacePurgeManifestPageResponse value) =>
            value.writeToBuffer()));
  }

  $async.Future<$0.ResolveReportTargetResponse> resolveReportTarget_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ResolveReportTargetRequest> $request) async {
    return resolveReportTarget($call, await $request);
  }

  $async.Future<$0.ResolveReportTargetResponse> resolveReportTarget(
      $grpc.ServiceCall call, $0.ResolveReportTargetRequest request);

  $async.Future<$1.SendGameEventMessageResponse> sendGameEventMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.SendGameEventMessageRequest> $request) async {
    return sendGameEventMessage($call, await $request);
  }

  $async.Future<$1.SendGameEventMessageResponse> sendGameEventMessage(
      $grpc.ServiceCall call, $1.SendGameEventMessageRequest request);

  $async.Future<$1.SendMessageResponse> sendMessage_Pre($grpc.ServiceCall $call,
      $async.Future<$1.SendMessageRequest> $request) async {
    return sendMessage($call, await $request);
  }

  $async.Future<$1.SendMessageResponse> sendMessage(
      $grpc.ServiceCall call, $1.SendMessageRequest request);

  $async.Future<$1.ApplyGameMessageResponse> applyGameMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ApplyGameMessageRequest> $request) async {
    return applyGameMessage($call, await $request);
  }

  $async.Future<$1.ApplyGameMessageResponse> applyGameMessage(
      $grpc.ServiceCall call, $1.ApplyGameMessageRequest request);

  $async.Future<$1.TombstoneGameMessageResponse> tombstoneGameMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.TombstoneGameMessageRequest> $request) async {
    return tombstoneGameMessage($call, await $request);
  }

  $async.Future<$1.TombstoneGameMessageResponse> tombstoneGameMessage(
      $grpc.ServiceCall call, $1.TombstoneGameMessageRequest request);

  $async.Future<$1.EditMessageResponse> editMessage_Pre($grpc.ServiceCall $call,
      $async.Future<$1.EditMessageRequest> $request) async {
    return editMessage($call, await $request);
  }

  $async.Future<$1.EditMessageResponse> editMessage(
      $grpc.ServiceCall call, $1.EditMessageRequest request);

  $async.Future<$1.DeleteMessageResponse> deleteMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.DeleteMessageRequest> $request) async {
    return deleteMessage($call, await $request);
  }

  $async.Future<$1.DeleteMessageResponse> deleteMessage(
      $grpc.ServiceCall call, $1.DeleteMessageRequest request);

  $async.Future<$1.GetMessagesResponse> getMessages_Pre($grpc.ServiceCall $call,
      $async.Future<$1.GetMessagesRequest> $request) async {
    return getMessages($call, await $request);
  }

  $async.Future<$1.GetMessagesResponse> getMessages(
      $grpc.ServiceCall call, $1.GetMessagesRequest request);

  $async.Future<$1.GetMessageResponse> getMessage_Pre($grpc.ServiceCall $call,
      $async.Future<$1.GetMessageRequest> $request) async {
    return getMessage($call, await $request);
  }

  $async.Future<$1.GetMessageResponse> getMessage(
      $grpc.ServiceCall call, $1.GetMessageRequest request);

  $async.Future<$1.ResolveGameActionResponse> resolveGameAction_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ResolveGameActionRequest> $request) async {
    return resolveGameAction($call, await $request);
  }

  $async.Future<$1.ResolveGameActionResponse> resolveGameAction(
      $grpc.ServiceCall call, $1.ResolveGameActionRequest request);

  $async.Future<$1.ProjectGameActionResultResponse> projectGameActionResult_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ProjectGameActionResultRequest> $request) async {
    return projectGameActionResult($call, await $request);
  }

  $async.Future<$1.ProjectGameActionResultResponse> projectGameActionResult(
      $grpc.ServiceCall call, $1.ProjectGameActionResultRequest request);

  $async.Future<$1.PurgeManagedChatContentResponse> purgeManagedChatContent_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.PurgeManagedChatContentRequest> $request) async {
    return purgeManagedChatContent($call, await $request);
  }

  $async.Future<$1.PurgeManagedChatContentResponse> purgeManagedChatContent(
      $grpc.ServiceCall call, $1.PurgeManagedChatContentRequest request);

  $async.Future<$1.GetThreadMessagesResponse> getThreadMessages_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetThreadMessagesRequest> $request) async {
    return getThreadMessages($call, await $request);
  }

  $async.Future<$1.GetThreadMessagesResponse> getThreadMessages(
      $grpc.ServiceCall call, $1.GetThreadMessagesRequest request);

  $async.Future<$1.ListThreadsResponse> listThreads_Pre($grpc.ServiceCall $call,
      $async.Future<$1.ListThreadsRequest> $request) async {
    return listThreads($call, await $request);
  }

  $async.Future<$1.ListThreadsResponse> listThreads(
      $grpc.ServiceCall call, $1.ListThreadsRequest request);

  $async.Future<$1.AddReactionResponse> addReaction_Pre($grpc.ServiceCall $call,
      $async.Future<$1.AddReactionRequest> $request) async {
    return addReaction($call, await $request);
  }

  $async.Future<$1.AddReactionResponse> addReaction(
      $grpc.ServiceCall call, $1.AddReactionRequest request);

  $async.Future<$1.RemoveReactionResponse> removeReaction_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.RemoveReactionRequest> $request) async {
    return removeReaction($call, await $request);
  }

  $async.Future<$1.RemoveReactionResponse> removeReaction(
      $grpc.ServiceCall call, $1.RemoveReactionRequest request);

  $async.Future<$1.PinMessageResponse> pinMessage_Pre($grpc.ServiceCall $call,
      $async.Future<$1.PinMessageRequest> $request) async {
    return pinMessage($call, await $request);
  }

  $async.Future<$1.PinMessageResponse> pinMessage(
      $grpc.ServiceCall call, $1.PinMessageRequest request);

  $async.Future<$1.UnpinMessageResponse> unpinMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UnpinMessageRequest> $request) async {
    return unpinMessage($call, await $request);
  }

  $async.Future<$1.UnpinMessageResponse> unpinMessage(
      $grpc.ServiceCall call, $1.UnpinMessageRequest request);

  $async.Future<$1.GetPinnedMessagesResponse> getPinnedMessages_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetPinnedMessagesRequest> $request) async {
    return getPinnedMessages($call, await $request);
  }

  $async.Future<$1.GetPinnedMessagesResponse> getPinnedMessages(
      $grpc.ServiceCall call, $1.GetPinnedMessagesRequest request);

  $async.Future<$1.UnpinMessagesBySenderInChatsResponse>
      unpinMessagesBySenderInChats_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$1.UnpinMessagesBySenderInChatsRequest>
              $request) async {
    return unpinMessagesBySenderInChats($call, await $request);
  }

  $async.Future<$1.UnpinMessagesBySenderInChatsResponse>
      unpinMessagesBySenderInChats($grpc.ServiceCall call,
          $1.UnpinMessagesBySenderInChatsRequest request);

  $async.Future<$1.ForwardMessageResponse> forwardMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ForwardMessageRequest> $request) async {
    return forwardMessage($call, await $request);
  }

  $async.Future<$1.ForwardMessageResponse> forwardMessage(
      $grpc.ServiceCall call, $1.ForwardMessageRequest request);

  $async.Future<$1.MarkReadResponse> markRead_Pre($grpc.ServiceCall $call,
      $async.Future<$1.MarkReadRequest> $request) async {
    return markRead($call, await $request);
  }

  $async.Future<$1.MarkReadResponse> markRead(
      $grpc.ServiceCall call, $1.MarkReadRequest request);

  $async.Future<$1.GetReadStateResponse> getReadState_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetReadStateRequest> $request) async {
    return getReadState($call, await $request);
  }

  $async.Future<$1.GetReadStateResponse> getReadState(
      $grpc.ServiceCall call, $1.GetReadStateRequest request);

  $async.Future<$1.GetBulkReadStateResponse> getBulkReadState_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetBulkReadStateRequest> $request) async {
    return getBulkReadState($call, await $request);
  }

  $async.Future<$1.GetBulkReadStateResponse> getBulkReadState(
      $grpc.ServiceCall call, $1.GetBulkReadStateRequest request);

  $async.Future<$1.GetChatListMetadataResponse> getChatListMetadata_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetChatListMetadataRequest> $request) async {
    return getChatListMetadata($call, await $request);
  }

  $async.Future<$1.GetChatListMetadataResponse> getChatListMetadata(
      $grpc.ServiceCall call, $1.GetChatListMetadataRequest request);

  $async.Future<$1.ListSharedMediaResponse> listSharedMedia_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ListSharedMediaRequest> $request) async {
    return listSharedMedia($call, await $request);
  }

  $async.Future<$1.ListSharedMediaResponse> listSharedMedia(
      $grpc.ServiceCall call, $1.ListSharedMediaRequest request);

  $async.Future<$1.UploadPreKeyBundleResponse> uploadPreKeyBundle_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UploadPreKeyBundleRequest> $request) async {
    return uploadPreKeyBundle($call, await $request);
  }

  $async.Future<$1.UploadPreKeyBundleResponse> uploadPreKeyBundle(
      $grpc.ServiceCall call, $1.UploadPreKeyBundleRequest request);

  $async.Future<$1.GetPreKeyBundleResponse> getPreKeyBundle_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetPreKeyBundleRequest> $request) async {
    return getPreKeyBundle($call, await $request);
  }

  $async.Future<$1.GetPreKeyBundleResponse> getPreKeyBundle(
      $grpc.ServiceCall call, $1.GetPreKeyBundleRequest request);

  $async.Future<$1.ListScheduledMessagesResponse> listScheduledMessages_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.ListScheduledMessagesRequest> $request) async {
    return listScheduledMessages($call, await $request);
  }

  $async.Future<$1.ListScheduledMessagesResponse> listScheduledMessages(
      $grpc.ServiceCall call, $1.ListScheduledMessagesRequest request);

  $async.Future<$1.UpdateScheduledMessageResponse> updateScheduledMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.UpdateScheduledMessageRequest> $request) async {
    return updateScheduledMessage($call, await $request);
  }

  $async.Future<$1.UpdateScheduledMessageResponse> updateScheduledMessage(
      $grpc.ServiceCall call, $1.UpdateScheduledMessageRequest request);

  $async.Future<$1.CancelScheduledMessageResponse> cancelScheduledMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.CancelScheduledMessageRequest> $request) async {
    return cancelScheduledMessage($call, await $request);
  }

  $async.Future<$1.CancelScheduledMessageResponse> cancelScheduledMessage(
      $grpc.ServiceCall call, $1.CancelScheduledMessageRequest request);

  $async.Future<$1.SendScheduledMessageNowResponse> sendScheduledMessageNow_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.SendScheduledMessageNowRequest> $request) async {
    return sendScheduledMessageNow($call, await $request);
  }

  $async.Future<$1.SendScheduledMessageNowResponse> sendScheduledMessageNow(
      $grpc.ServiceCall call, $1.SendScheduledMessageNowRequest request);

  $async.Future<$1.ApplySpaceLifecycleFenceResponse>
      applySpaceLifecycleFence_Pre($grpc.ServiceCall $call,
          $async.Future<$1.ApplySpaceLifecycleFenceRequest> $request) async {
    return applySpaceLifecycleFence($call, await $request);
  }

  $async.Future<$1.ApplySpaceLifecycleFenceResponse> applySpaceLifecycleFence(
      $grpc.ServiceCall call, $1.ApplySpaceLifecycleFenceRequest request);

  $async.Future<$1.PurgeSpaceResponse> purgeSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$1.PurgeSpaceRequest> $request) async {
    return purgeSpace($call, await $request);
  }

  $async.Future<$1.PurgeSpaceResponse> purgeSpace(
      $grpc.ServiceCall call, $1.PurgeSpaceRequest request);

  $async.Future<$1.GetSpacePurgeReceiptResponse> getSpacePurgeReceipt_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$1.GetSpacePurgeReceiptRequest> $request) async {
    return getSpacePurgeReceipt($call, await $request);
  }

  $async.Future<$1.GetSpacePurgeReceiptResponse> getSpacePurgeReceipt(
      $grpc.ServiceCall call, $1.GetSpacePurgeReceiptRequest request);

  $async.Future<$1.ImportSpacePurgeManifestPageResponse>
      importSpacePurgeManifestPage_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$1.ImportSpacePurgeManifestPageRequest>
              $request) async {
    return importSpacePurgeManifestPage($call, await $request);
  }

  $async.Future<$1.ImportSpacePurgeManifestPageResponse>
      importSpacePurgeManifestPage($grpc.ServiceCall call,
          $1.ImportSpacePurgeManifestPageRequest request);
}
