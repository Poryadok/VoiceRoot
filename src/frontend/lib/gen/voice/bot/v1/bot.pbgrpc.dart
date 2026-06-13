// This is a generated file - do not edit.
//
// Generated from voice/bot/v1/bot.proto.

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

import 'bot.pb.dart' as $0;

export 'bot.pb.dart';

/// Bot platform. HTTP: /api/v1/bots/**.
@$pb.GrpcServiceName('voice.bot.v1.BotService')
class BotServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  BotServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.RegisterBotResponse> registerBot(
    $0.RegisterBotRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$registerBot, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdateBotResponse> updateBot(
    $0.UpdateBotRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateBot, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteBotResponse> deleteBot(
    $0.DeleteBotRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteBot, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetBotResponse> getBot(
    $0.GetBotRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getBot, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListBotsResponse> listBots(
    $0.ListBotsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listBots, request, options: options);
  }

  $grpc.ResponseFuture<$0.RegenerateTokenResponse> regenerateToken(
    $0.RegenerateTokenRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$regenerateToken, request, options: options);
  }

  $grpc.ResponseFuture<$0.RegisterCommandsResponse> registerCommands(
    $0.RegisterCommandsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$registerCommands, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetCommandsResponse> getCommands(
    $0.GetCommandsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getCommands, request, options: options);
  }

  $grpc.ResponseFuture<$0.SetWebhookURLResponse> setWebhookURL(
    $0.SetWebhookURLRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setWebhookURL, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetWebhookURLResponse> getWebhookURL(
    $0.GetWebhookURLRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getWebhookURL, request, options: options);
  }

  $grpc.ResponseFuture<$0.SetChatWhitelistResponse> setChatWhitelist(
    $0.SetChatWhitelistRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setChatWhitelist, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetChatWhitelistResponse> getChatWhitelist(
    $0.GetChatWhitelistRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getChatWhitelist, request, options: options);
  }

  $grpc.ResponseFuture<$0.SendBotMessageResponse> sendBotMessage(
    $0.SendBotMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$sendBotMessage, request, options: options);
  }

  $grpc.ResponseFuture<$0.EditBotMessageResponse> editBotMessage(
    $0.EditBotMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$editBotMessage, request, options: options);
  }

  $grpc.ResponseFuture<$0.SendEphemeralResponse> sendEphemeral(
    $0.SendEphemeralRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$sendEphemeral, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeferResponseResponse> deferResponse(
    $0.DeferResponseRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deferResponse, request, options: options);
  }

  $grpc.ResponseStream<$0.PollEventsResponse> pollEvents(
    $0.PollEventsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(
        _$pollEvents, $async.Stream.fromIterable([request]),
        options: options);
  }

  // method descriptors

  static final _$registerBot =
      $grpc.ClientMethod<$0.RegisterBotRequest, $0.RegisterBotResponse>(
          '/voice.bot.v1.BotService/RegisterBot',
          ($0.RegisterBotRequest value) => value.writeToBuffer(),
          $0.RegisterBotResponse.fromBuffer);
  static final _$updateBot =
      $grpc.ClientMethod<$0.UpdateBotRequest, $0.UpdateBotResponse>(
          '/voice.bot.v1.BotService/UpdateBot',
          ($0.UpdateBotRequest value) => value.writeToBuffer(),
          $0.UpdateBotResponse.fromBuffer);
  static final _$deleteBot =
      $grpc.ClientMethod<$0.DeleteBotRequest, $0.DeleteBotResponse>(
          '/voice.bot.v1.BotService/DeleteBot',
          ($0.DeleteBotRequest value) => value.writeToBuffer(),
          $0.DeleteBotResponse.fromBuffer);
  static final _$getBot =
      $grpc.ClientMethod<$0.GetBotRequest, $0.GetBotResponse>(
          '/voice.bot.v1.BotService/GetBot',
          ($0.GetBotRequest value) => value.writeToBuffer(),
          $0.GetBotResponse.fromBuffer);
  static final _$listBots =
      $grpc.ClientMethod<$0.ListBotsRequest, $0.ListBotsResponse>(
          '/voice.bot.v1.BotService/ListBots',
          ($0.ListBotsRequest value) => value.writeToBuffer(),
          $0.ListBotsResponse.fromBuffer);
  static final _$regenerateToken =
      $grpc.ClientMethod<$0.RegenerateTokenRequest, $0.RegenerateTokenResponse>(
          '/voice.bot.v1.BotService/RegenerateToken',
          ($0.RegenerateTokenRequest value) => value.writeToBuffer(),
          $0.RegenerateTokenResponse.fromBuffer);
  static final _$registerCommands = $grpc.ClientMethod<
          $0.RegisterCommandsRequest, $0.RegisterCommandsResponse>(
      '/voice.bot.v1.BotService/RegisterCommands',
      ($0.RegisterCommandsRequest value) => value.writeToBuffer(),
      $0.RegisterCommandsResponse.fromBuffer);
  static final _$getCommands =
      $grpc.ClientMethod<$0.GetCommandsRequest, $0.GetCommandsResponse>(
          '/voice.bot.v1.BotService/GetCommands',
          ($0.GetCommandsRequest value) => value.writeToBuffer(),
          $0.GetCommandsResponse.fromBuffer);
  static final _$setWebhookURL =
      $grpc.ClientMethod<$0.SetWebhookURLRequest, $0.SetWebhookURLResponse>(
          '/voice.bot.v1.BotService/SetWebhookURL',
          ($0.SetWebhookURLRequest value) => value.writeToBuffer(),
          $0.SetWebhookURLResponse.fromBuffer);
  static final _$getWebhookURL =
      $grpc.ClientMethod<$0.GetWebhookURLRequest, $0.GetWebhookURLResponse>(
          '/voice.bot.v1.BotService/GetWebhookURL',
          ($0.GetWebhookURLRequest value) => value.writeToBuffer(),
          $0.GetWebhookURLResponse.fromBuffer);
  static final _$setChatWhitelist = $grpc.ClientMethod<
          $0.SetChatWhitelistRequest, $0.SetChatWhitelistResponse>(
      '/voice.bot.v1.BotService/SetChatWhitelist',
      ($0.SetChatWhitelistRequest value) => value.writeToBuffer(),
      $0.SetChatWhitelistResponse.fromBuffer);
  static final _$getChatWhitelist = $grpc.ClientMethod<
          $0.GetChatWhitelistRequest, $0.GetChatWhitelistResponse>(
      '/voice.bot.v1.BotService/GetChatWhitelist',
      ($0.GetChatWhitelistRequest value) => value.writeToBuffer(),
      $0.GetChatWhitelistResponse.fromBuffer);
  static final _$sendBotMessage =
      $grpc.ClientMethod<$0.SendBotMessageRequest, $0.SendBotMessageResponse>(
          '/voice.bot.v1.BotService/SendBotMessage',
          ($0.SendBotMessageRequest value) => value.writeToBuffer(),
          $0.SendBotMessageResponse.fromBuffer);
  static final _$editBotMessage =
      $grpc.ClientMethod<$0.EditBotMessageRequest, $0.EditBotMessageResponse>(
          '/voice.bot.v1.BotService/EditBotMessage',
          ($0.EditBotMessageRequest value) => value.writeToBuffer(),
          $0.EditBotMessageResponse.fromBuffer);
  static final _$sendEphemeral =
      $grpc.ClientMethod<$0.SendEphemeralRequest, $0.SendEphemeralResponse>(
          '/voice.bot.v1.BotService/SendEphemeral',
          ($0.SendEphemeralRequest value) => value.writeToBuffer(),
          $0.SendEphemeralResponse.fromBuffer);
  static final _$deferResponse =
      $grpc.ClientMethod<$0.DeferResponseRequest, $0.DeferResponseResponse>(
          '/voice.bot.v1.BotService/DeferResponse',
          ($0.DeferResponseRequest value) => value.writeToBuffer(),
          $0.DeferResponseResponse.fromBuffer);
  static final _$pollEvents =
      $grpc.ClientMethod<$0.PollEventsRequest, $0.PollEventsResponse>(
          '/voice.bot.v1.BotService/PollEvents',
          ($0.PollEventsRequest value) => value.writeToBuffer(),
          $0.PollEventsResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.bot.v1.BotService')
abstract class BotServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.bot.v1.BotService';

  BotServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.RegisterBotRequest, $0.RegisterBotResponse>(
            'RegisterBot',
            registerBot_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.RegisterBotRequest.fromBuffer(value),
            ($0.RegisterBotResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpdateBotRequest, $0.UpdateBotResponse>(
        'UpdateBot',
        updateBot_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.UpdateBotRequest.fromBuffer(value),
        ($0.UpdateBotResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeleteBotRequest, $0.DeleteBotResponse>(
        'DeleteBot',
        deleteBot_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.DeleteBotRequest.fromBuffer(value),
        ($0.DeleteBotResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetBotRequest, $0.GetBotResponse>(
        'GetBot',
        getBot_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetBotRequest.fromBuffer(value),
        ($0.GetBotResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListBotsRequest, $0.ListBotsResponse>(
        'ListBots',
        listBots_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ListBotsRequest.fromBuffer(value),
        ($0.ListBotsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RegenerateTokenRequest,
            $0.RegenerateTokenResponse>(
        'RegenerateToken',
        regenerateToken_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RegenerateTokenRequest.fromBuffer(value),
        ($0.RegenerateTokenResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RegisterCommandsRequest,
            $0.RegisterCommandsResponse>(
        'RegisterCommands',
        registerCommands_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RegisterCommandsRequest.fromBuffer(value),
        ($0.RegisterCommandsResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetCommandsRequest, $0.GetCommandsResponse>(
            'GetCommands',
            getCommands_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetCommandsRequest.fromBuffer(value),
            ($0.GetCommandsResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SetWebhookURLRequest, $0.SetWebhookURLResponse>(
            'SetWebhookURL',
            setWebhookURL_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SetWebhookURLRequest.fromBuffer(value),
            ($0.SetWebhookURLResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetWebhookURLRequest, $0.GetWebhookURLResponse>(
            'GetWebhookURL',
            getWebhookURL_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetWebhookURLRequest.fromBuffer(value),
            ($0.GetWebhookURLResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SetChatWhitelistRequest,
            $0.SetChatWhitelistResponse>(
        'SetChatWhitelist',
        setChatWhitelist_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SetChatWhitelistRequest.fromBuffer(value),
        ($0.SetChatWhitelistResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetChatWhitelistRequest,
            $0.GetChatWhitelistResponse>(
        'GetChatWhitelist',
        getChatWhitelist_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetChatWhitelistRequest.fromBuffer(value),
        ($0.GetChatWhitelistResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SendBotMessageRequest,
            $0.SendBotMessageResponse>(
        'SendBotMessage',
        sendBotMessage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SendBotMessageRequest.fromBuffer(value),
        ($0.SendBotMessageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.EditBotMessageRequest,
            $0.EditBotMessageResponse>(
        'EditBotMessage',
        editBotMessage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.EditBotMessageRequest.fromBuffer(value),
        ($0.EditBotMessageResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SendEphemeralRequest, $0.SendEphemeralResponse>(
            'SendEphemeral',
            sendEphemeral_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SendEphemeralRequest.fromBuffer(value),
            ($0.SendEphemeralResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.DeferResponseRequest, $0.DeferResponseResponse>(
            'DeferResponse',
            deferResponse_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.DeferResponseRequest.fromBuffer(value),
            ($0.DeferResponseResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.PollEventsRequest, $0.PollEventsResponse>(
        'PollEvents',
        pollEvents_Pre,
        false,
        true,
        ($core.List<$core.int> value) => $0.PollEventsRequest.fromBuffer(value),
        ($0.PollEventsResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.RegisterBotResponse> registerBot_Pre($grpc.ServiceCall $call,
      $async.Future<$0.RegisterBotRequest> $request) async {
    return registerBot($call, await $request);
  }

  $async.Future<$0.RegisterBotResponse> registerBot(
      $grpc.ServiceCall call, $0.RegisterBotRequest request);

  $async.Future<$0.UpdateBotResponse> updateBot_Pre($grpc.ServiceCall $call,
      $async.Future<$0.UpdateBotRequest> $request) async {
    return updateBot($call, await $request);
  }

  $async.Future<$0.UpdateBotResponse> updateBot(
      $grpc.ServiceCall call, $0.UpdateBotRequest request);

  $async.Future<$0.DeleteBotResponse> deleteBot_Pre($grpc.ServiceCall $call,
      $async.Future<$0.DeleteBotRequest> $request) async {
    return deleteBot($call, await $request);
  }

  $async.Future<$0.DeleteBotResponse> deleteBot(
      $grpc.ServiceCall call, $0.DeleteBotRequest request);

  $async.Future<$0.GetBotResponse> getBot_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.GetBotRequest> $request) async {
    return getBot($call, await $request);
  }

  $async.Future<$0.GetBotResponse> getBot(
      $grpc.ServiceCall call, $0.GetBotRequest request);

  $async.Future<$0.ListBotsResponse> listBots_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListBotsRequest> $request) async {
    return listBots($call, await $request);
  }

  $async.Future<$0.ListBotsResponse> listBots(
      $grpc.ServiceCall call, $0.ListBotsRequest request);

  $async.Future<$0.RegenerateTokenResponse> regenerateToken_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RegenerateTokenRequest> $request) async {
    return regenerateToken($call, await $request);
  }

  $async.Future<$0.RegenerateTokenResponse> regenerateToken(
      $grpc.ServiceCall call, $0.RegenerateTokenRequest request);

  $async.Future<$0.RegisterCommandsResponse> registerCommands_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RegisterCommandsRequest> $request) async {
    return registerCommands($call, await $request);
  }

  $async.Future<$0.RegisterCommandsResponse> registerCommands(
      $grpc.ServiceCall call, $0.RegisterCommandsRequest request);

  $async.Future<$0.GetCommandsResponse> getCommands_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetCommandsRequest> $request) async {
    return getCommands($call, await $request);
  }

  $async.Future<$0.GetCommandsResponse> getCommands(
      $grpc.ServiceCall call, $0.GetCommandsRequest request);

  $async.Future<$0.SetWebhookURLResponse> setWebhookURL_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetWebhookURLRequest> $request) async {
    return setWebhookURL($call, await $request);
  }

  $async.Future<$0.SetWebhookURLResponse> setWebhookURL(
      $grpc.ServiceCall call, $0.SetWebhookURLRequest request);

  $async.Future<$0.GetWebhookURLResponse> getWebhookURL_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetWebhookURLRequest> $request) async {
    return getWebhookURL($call, await $request);
  }

  $async.Future<$0.GetWebhookURLResponse> getWebhookURL(
      $grpc.ServiceCall call, $0.GetWebhookURLRequest request);

  $async.Future<$0.SetChatWhitelistResponse> setChatWhitelist_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetChatWhitelistRequest> $request) async {
    return setChatWhitelist($call, await $request);
  }

  $async.Future<$0.SetChatWhitelistResponse> setChatWhitelist(
      $grpc.ServiceCall call, $0.SetChatWhitelistRequest request);

  $async.Future<$0.GetChatWhitelistResponse> getChatWhitelist_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetChatWhitelistRequest> $request) async {
    return getChatWhitelist($call, await $request);
  }

  $async.Future<$0.GetChatWhitelistResponse> getChatWhitelist(
      $grpc.ServiceCall call, $0.GetChatWhitelistRequest request);

  $async.Future<$0.SendBotMessageResponse> sendBotMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SendBotMessageRequest> $request) async {
    return sendBotMessage($call, await $request);
  }

  $async.Future<$0.SendBotMessageResponse> sendBotMessage(
      $grpc.ServiceCall call, $0.SendBotMessageRequest request);

  $async.Future<$0.EditBotMessageResponse> editBotMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.EditBotMessageRequest> $request) async {
    return editBotMessage($call, await $request);
  }

  $async.Future<$0.EditBotMessageResponse> editBotMessage(
      $grpc.ServiceCall call, $0.EditBotMessageRequest request);

  $async.Future<$0.SendEphemeralResponse> sendEphemeral_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SendEphemeralRequest> $request) async {
    return sendEphemeral($call, await $request);
  }

  $async.Future<$0.SendEphemeralResponse> sendEphemeral(
      $grpc.ServiceCall call, $0.SendEphemeralRequest request);

  $async.Future<$0.DeferResponseResponse> deferResponse_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DeferResponseRequest> $request) async {
    return deferResponse($call, await $request);
  }

  $async.Future<$0.DeferResponseResponse> deferResponse(
      $grpc.ServiceCall call, $0.DeferResponseRequest request);

  $async.Stream<$0.PollEventsResponse> pollEvents_Pre($grpc.ServiceCall $call,
      $async.Future<$0.PollEventsRequest> $request) async* {
    yield* pollEvents($call, await $request);
  }

  $async.Stream<$0.PollEventsResponse> pollEvents(
      $grpc.ServiceCall call, $0.PollEventsRequest request);
}
