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

  $grpc.ResponseFuture<$0.GetBotResponse> getBotBySlug(
    $0.GetBotBySlugRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getBotBySlug, request, options: options);
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

  /// Manifest (Developer Portal).
  $grpc.ResponseFuture<$0.ValidateManifestResponse> validateManifest(
    $0.ValidateManifestRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$validateManifest, request, options: options);
  }

  $grpc.ResponseFuture<$0.ApplyManifestResponse> applyManifest(
    $0.ApplyManifestRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$applyManifest, request, options: options);
  }

  /// Space lifecycle.
  $grpc.ResponseFuture<$0.InstallBotInSpaceResponse> installBotInSpace(
    $0.InstallBotInSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$installBotInSpace, request, options: options);
  }

  $grpc.ResponseFuture<$0.UninstallBotFromSpaceResponse> uninstallBotFromSpace(
    $0.UninstallBotFromSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$uninstallBotFromSpace, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListInstalledBotsResponse> listInstalledBots(
    $0.ListInstalledBotsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listInstalledBots, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListBotsInChatResponse> listBotsInChat(
    $0.ListBotsInChatRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listBotsInChat, request, options: options);
  }

  $grpc.ResponseFuture<$0.SetBotChatEnabledResponse> setBotChatEnabled(
    $0.SetBotChatEnabledRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setBotChatEnabled, request, options: options);
  }

  /// Client slash interactions.
  $grpc.ResponseFuture<$0.ExecuteSlashInteractionResponse>
      executeSlashInteraction(
    $0.ExecuteSlashInteractionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$executeSlashInteraction, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.ListSlashCommandsForChatResponse>
      listSlashCommandsForChat(
    $0.ListSlashCommandsForChatRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listSlashCommandsForChat, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.CompleteInteractionResponse> completeInteraction(
    $0.CompleteInteractionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$completeInteraction, request, options: options);
  }

  $grpc.ResponseFuture<$0.AutocompleteSlashOptionResponse>
      autocompleteSlashOption(
    $0.AutocompleteSlashOptionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$autocompleteSlashOption, request,
        options: options);
  }

  /// BOT-C: presence, scopes runtime, history gate.
  $grpc.ResponseFuture<$0.TouchPresenceResponse> touchPresence(
    $0.TouchPresenceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$touchPresence, request, options: options);
  }

  $grpc.ResponseFuture<$0.AssignBotRoleResponse> assignBotRole(
    $0.AssignBotRoleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$assignBotRole, request, options: options);
  }

  $grpc.ResponseFuture<$0.RevokeBotRoleResponse> revokeBotRole(
    $0.RevokeBotRoleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$revokeBotRole, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListSpaceMembersForBotResponse>
      listSpaceMembersForBot(
    $0.ListSpaceMembersForBotRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listSpaceMembersForBot, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.CreateBotChatResponse> createBotChat(
    $0.CreateBotChatRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createBotChat, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetChatMessagesForBotResponse> getChatMessagesForBot(
    $0.GetChatMessagesForBotRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getChatMessagesForBot, request, options: options);
  }

  $grpc.ResponseFuture<$0.CreateBotRoleResponse> createBotRole(
    $0.CreateBotRoleRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createBotRole, request, options: options);
  }

  $grpc.ResponseFuture<$0.CompleteAutocompleteResponse> completeAutocomplete(
    $0.CompleteAutocompleteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$completeAutocomplete, request, options: options);
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
  static final _$getBotBySlug =
      $grpc.ClientMethod<$0.GetBotBySlugRequest, $0.GetBotResponse>(
          '/voice.bot.v1.BotService/GetBotBySlug',
          ($0.GetBotBySlugRequest value) => value.writeToBuffer(),
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
  static final _$validateManifest = $grpc.ClientMethod<
          $0.ValidateManifestRequest, $0.ValidateManifestResponse>(
      '/voice.bot.v1.BotService/ValidateManifest',
      ($0.ValidateManifestRequest value) => value.writeToBuffer(),
      $0.ValidateManifestResponse.fromBuffer);
  static final _$applyManifest =
      $grpc.ClientMethod<$0.ApplyManifestRequest, $0.ApplyManifestResponse>(
          '/voice.bot.v1.BotService/ApplyManifest',
          ($0.ApplyManifestRequest value) => value.writeToBuffer(),
          $0.ApplyManifestResponse.fromBuffer);
  static final _$installBotInSpace = $grpc.ClientMethod<
          $0.InstallBotInSpaceRequest, $0.InstallBotInSpaceResponse>(
      '/voice.bot.v1.BotService/InstallBotInSpace',
      ($0.InstallBotInSpaceRequest value) => value.writeToBuffer(),
      $0.InstallBotInSpaceResponse.fromBuffer);
  static final _$uninstallBotFromSpace = $grpc.ClientMethod<
          $0.UninstallBotFromSpaceRequest, $0.UninstallBotFromSpaceResponse>(
      '/voice.bot.v1.BotService/UninstallBotFromSpace',
      ($0.UninstallBotFromSpaceRequest value) => value.writeToBuffer(),
      $0.UninstallBotFromSpaceResponse.fromBuffer);
  static final _$listInstalledBots = $grpc.ClientMethod<
          $0.ListInstalledBotsRequest, $0.ListInstalledBotsResponse>(
      '/voice.bot.v1.BotService/ListInstalledBots',
      ($0.ListInstalledBotsRequest value) => value.writeToBuffer(),
      $0.ListInstalledBotsResponse.fromBuffer);
  static final _$listBotsInChat =
      $grpc.ClientMethod<$0.ListBotsInChatRequest, $0.ListBotsInChatResponse>(
          '/voice.bot.v1.BotService/ListBotsInChat',
          ($0.ListBotsInChatRequest value) => value.writeToBuffer(),
          $0.ListBotsInChatResponse.fromBuffer);
  static final _$setBotChatEnabled = $grpc.ClientMethod<
          $0.SetBotChatEnabledRequest, $0.SetBotChatEnabledResponse>(
      '/voice.bot.v1.BotService/SetBotChatEnabled',
      ($0.SetBotChatEnabledRequest value) => value.writeToBuffer(),
      $0.SetBotChatEnabledResponse.fromBuffer);
  static final _$executeSlashInteraction = $grpc.ClientMethod<
          $0.ExecuteSlashInteractionRequest,
          $0.ExecuteSlashInteractionResponse>(
      '/voice.bot.v1.BotService/ExecuteSlashInteraction',
      ($0.ExecuteSlashInteractionRequest value) => value.writeToBuffer(),
      $0.ExecuteSlashInteractionResponse.fromBuffer);
  static final _$listSlashCommandsForChat = $grpc.ClientMethod<
          $0.ListSlashCommandsForChatRequest,
          $0.ListSlashCommandsForChatResponse>(
      '/voice.bot.v1.BotService/ListSlashCommandsForChat',
      ($0.ListSlashCommandsForChatRequest value) => value.writeToBuffer(),
      $0.ListSlashCommandsForChatResponse.fromBuffer);
  static final _$completeInteraction = $grpc.ClientMethod<
          $0.CompleteInteractionRequest, $0.CompleteInteractionResponse>(
      '/voice.bot.v1.BotService/CompleteInteraction',
      ($0.CompleteInteractionRequest value) => value.writeToBuffer(),
      $0.CompleteInteractionResponse.fromBuffer);
  static final _$autocompleteSlashOption = $grpc.ClientMethod<
          $0.AutocompleteSlashOptionRequest,
          $0.AutocompleteSlashOptionResponse>(
      '/voice.bot.v1.BotService/AutocompleteSlashOption',
      ($0.AutocompleteSlashOptionRequest value) => value.writeToBuffer(),
      $0.AutocompleteSlashOptionResponse.fromBuffer);
  static final _$touchPresence =
      $grpc.ClientMethod<$0.TouchPresenceRequest, $0.TouchPresenceResponse>(
          '/voice.bot.v1.BotService/TouchPresence',
          ($0.TouchPresenceRequest value) => value.writeToBuffer(),
          $0.TouchPresenceResponse.fromBuffer);
  static final _$assignBotRole =
      $grpc.ClientMethod<$0.AssignBotRoleRequest, $0.AssignBotRoleResponse>(
          '/voice.bot.v1.BotService/AssignBotRole',
          ($0.AssignBotRoleRequest value) => value.writeToBuffer(),
          $0.AssignBotRoleResponse.fromBuffer);
  static final _$revokeBotRole =
      $grpc.ClientMethod<$0.RevokeBotRoleRequest, $0.RevokeBotRoleResponse>(
          '/voice.bot.v1.BotService/RevokeBotRole',
          ($0.RevokeBotRoleRequest value) => value.writeToBuffer(),
          $0.RevokeBotRoleResponse.fromBuffer);
  static final _$listSpaceMembersForBot = $grpc.ClientMethod<
          $0.ListSpaceMembersForBotRequest, $0.ListSpaceMembersForBotResponse>(
      '/voice.bot.v1.BotService/ListSpaceMembersForBot',
      ($0.ListSpaceMembersForBotRequest value) => value.writeToBuffer(),
      $0.ListSpaceMembersForBotResponse.fromBuffer);
  static final _$createBotChat =
      $grpc.ClientMethod<$0.CreateBotChatRequest, $0.CreateBotChatResponse>(
          '/voice.bot.v1.BotService/CreateBotChat',
          ($0.CreateBotChatRequest value) => value.writeToBuffer(),
          $0.CreateBotChatResponse.fromBuffer);
  static final _$getChatMessagesForBot = $grpc.ClientMethod<
          $0.GetChatMessagesForBotRequest, $0.GetChatMessagesForBotResponse>(
      '/voice.bot.v1.BotService/GetChatMessagesForBot',
      ($0.GetChatMessagesForBotRequest value) => value.writeToBuffer(),
      $0.GetChatMessagesForBotResponse.fromBuffer);
  static final _$createBotRole =
      $grpc.ClientMethod<$0.CreateBotRoleRequest, $0.CreateBotRoleResponse>(
          '/voice.bot.v1.BotService/CreateBotRole',
          ($0.CreateBotRoleRequest value) => value.writeToBuffer(),
          $0.CreateBotRoleResponse.fromBuffer);
  static final _$completeAutocomplete = $grpc.ClientMethod<
          $0.CompleteAutocompleteRequest, $0.CompleteAutocompleteResponse>(
      '/voice.bot.v1.BotService/CompleteAutocomplete',
      ($0.CompleteAutocompleteRequest value) => value.writeToBuffer(),
      $0.CompleteAutocompleteResponse.fromBuffer);
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
    $addMethod($grpc.ServiceMethod<$0.GetBotBySlugRequest, $0.GetBotResponse>(
        'GetBotBySlug',
        getBotBySlug_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetBotBySlugRequest.fromBuffer(value),
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
    $addMethod($grpc.ServiceMethod<$0.ValidateManifestRequest,
            $0.ValidateManifestResponse>(
        'ValidateManifest',
        validateManifest_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ValidateManifestRequest.fromBuffer(value),
        ($0.ValidateManifestResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ApplyManifestRequest, $0.ApplyManifestResponse>(
            'ApplyManifest',
            applyManifest_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ApplyManifestRequest.fromBuffer(value),
            ($0.ApplyManifestResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.InstallBotInSpaceRequest,
            $0.InstallBotInSpaceResponse>(
        'InstallBotInSpace',
        installBotInSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.InstallBotInSpaceRequest.fromBuffer(value),
        ($0.InstallBotInSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UninstallBotFromSpaceRequest,
            $0.UninstallBotFromSpaceResponse>(
        'UninstallBotFromSpace',
        uninstallBotFromSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UninstallBotFromSpaceRequest.fromBuffer(value),
        ($0.UninstallBotFromSpaceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListInstalledBotsRequest,
            $0.ListInstalledBotsResponse>(
        'ListInstalledBots',
        listInstalledBots_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListInstalledBotsRequest.fromBuffer(value),
        ($0.ListInstalledBotsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListBotsInChatRequest,
            $0.ListBotsInChatResponse>(
        'ListBotsInChat',
        listBotsInChat_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListBotsInChatRequest.fromBuffer(value),
        ($0.ListBotsInChatResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SetBotChatEnabledRequest,
            $0.SetBotChatEnabledResponse>(
        'SetBotChatEnabled',
        setBotChatEnabled_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SetBotChatEnabledRequest.fromBuffer(value),
        ($0.SetBotChatEnabledResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ExecuteSlashInteractionRequest,
            $0.ExecuteSlashInteractionResponse>(
        'ExecuteSlashInteraction',
        executeSlashInteraction_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ExecuteSlashInteractionRequest.fromBuffer(value),
        ($0.ExecuteSlashInteractionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListSlashCommandsForChatRequest,
            $0.ListSlashCommandsForChatResponse>(
        'ListSlashCommandsForChat',
        listSlashCommandsForChat_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListSlashCommandsForChatRequest.fromBuffer(value),
        ($0.ListSlashCommandsForChatResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CompleteInteractionRequest,
            $0.CompleteInteractionResponse>(
        'CompleteInteraction',
        completeInteraction_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CompleteInteractionRequest.fromBuffer(value),
        ($0.CompleteInteractionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AutocompleteSlashOptionRequest,
            $0.AutocompleteSlashOptionResponse>(
        'AutocompleteSlashOption',
        autocompleteSlashOption_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.AutocompleteSlashOptionRequest.fromBuffer(value),
        ($0.AutocompleteSlashOptionResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.TouchPresenceRequest, $0.TouchPresenceResponse>(
            'TouchPresence',
            touchPresence_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.TouchPresenceRequest.fromBuffer(value),
            ($0.TouchPresenceResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.AssignBotRoleRequest, $0.AssignBotRoleResponse>(
            'AssignBotRole',
            assignBotRole_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.AssignBotRoleRequest.fromBuffer(value),
            ($0.AssignBotRoleResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.RevokeBotRoleRequest, $0.RevokeBotRoleResponse>(
            'RevokeBotRole',
            revokeBotRole_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.RevokeBotRoleRequest.fromBuffer(value),
            ($0.RevokeBotRoleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListSpaceMembersForBotRequest,
            $0.ListSpaceMembersForBotResponse>(
        'ListSpaceMembersForBot',
        listSpaceMembersForBot_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListSpaceMembersForBotRequest.fromBuffer(value),
        ($0.ListSpaceMembersForBotResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.CreateBotChatRequest, $0.CreateBotChatResponse>(
            'CreateBotChat',
            createBotChat_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.CreateBotChatRequest.fromBuffer(value),
            ($0.CreateBotChatResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetChatMessagesForBotRequest,
            $0.GetChatMessagesForBotResponse>(
        'GetChatMessagesForBot',
        getChatMessagesForBot_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetChatMessagesForBotRequest.fromBuffer(value),
        ($0.GetChatMessagesForBotResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.CreateBotRoleRequest, $0.CreateBotRoleResponse>(
            'CreateBotRole',
            createBotRole_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.CreateBotRoleRequest.fromBuffer(value),
            ($0.CreateBotRoleResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CompleteAutocompleteRequest,
            $0.CompleteAutocompleteResponse>(
        'CompleteAutocomplete',
        completeAutocomplete_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CompleteAutocompleteRequest.fromBuffer(value),
        ($0.CompleteAutocompleteResponse value) => value.writeToBuffer()));
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

  $async.Future<$0.GetBotResponse> getBotBySlug_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetBotBySlugRequest> $request) async {
    return getBotBySlug($call, await $request);
  }

  $async.Future<$0.GetBotResponse> getBotBySlug(
      $grpc.ServiceCall call, $0.GetBotBySlugRequest request);

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

  $async.Future<$0.ValidateManifestResponse> validateManifest_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ValidateManifestRequest> $request) async {
    return validateManifest($call, await $request);
  }

  $async.Future<$0.ValidateManifestResponse> validateManifest(
      $grpc.ServiceCall call, $0.ValidateManifestRequest request);

  $async.Future<$0.ApplyManifestResponse> applyManifest_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ApplyManifestRequest> $request) async {
    return applyManifest($call, await $request);
  }

  $async.Future<$0.ApplyManifestResponse> applyManifest(
      $grpc.ServiceCall call, $0.ApplyManifestRequest request);

  $async.Future<$0.InstallBotInSpaceResponse> installBotInSpace_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.InstallBotInSpaceRequest> $request) async {
    return installBotInSpace($call, await $request);
  }

  $async.Future<$0.InstallBotInSpaceResponse> installBotInSpace(
      $grpc.ServiceCall call, $0.InstallBotInSpaceRequest request);

  $async.Future<$0.UninstallBotFromSpaceResponse> uninstallBotFromSpace_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UninstallBotFromSpaceRequest> $request) async {
    return uninstallBotFromSpace($call, await $request);
  }

  $async.Future<$0.UninstallBotFromSpaceResponse> uninstallBotFromSpace(
      $grpc.ServiceCall call, $0.UninstallBotFromSpaceRequest request);

  $async.Future<$0.ListInstalledBotsResponse> listInstalledBots_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListInstalledBotsRequest> $request) async {
    return listInstalledBots($call, await $request);
  }

  $async.Future<$0.ListInstalledBotsResponse> listInstalledBots(
      $grpc.ServiceCall call, $0.ListInstalledBotsRequest request);

  $async.Future<$0.ListBotsInChatResponse> listBotsInChat_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListBotsInChatRequest> $request) async {
    return listBotsInChat($call, await $request);
  }

  $async.Future<$0.ListBotsInChatResponse> listBotsInChat(
      $grpc.ServiceCall call, $0.ListBotsInChatRequest request);

  $async.Future<$0.SetBotChatEnabledResponse> setBotChatEnabled_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetBotChatEnabledRequest> $request) async {
    return setBotChatEnabled($call, await $request);
  }

  $async.Future<$0.SetBotChatEnabledResponse> setBotChatEnabled(
      $grpc.ServiceCall call, $0.SetBotChatEnabledRequest request);

  $async.Future<$0.ExecuteSlashInteractionResponse> executeSlashInteraction_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ExecuteSlashInteractionRequest> $request) async {
    return executeSlashInteraction($call, await $request);
  }

  $async.Future<$0.ExecuteSlashInteractionResponse> executeSlashInteraction(
      $grpc.ServiceCall call, $0.ExecuteSlashInteractionRequest request);

  $async.Future<$0.ListSlashCommandsForChatResponse>
      listSlashCommandsForChat_Pre($grpc.ServiceCall $call,
          $async.Future<$0.ListSlashCommandsForChatRequest> $request) async {
    return listSlashCommandsForChat($call, await $request);
  }

  $async.Future<$0.ListSlashCommandsForChatResponse> listSlashCommandsForChat(
      $grpc.ServiceCall call, $0.ListSlashCommandsForChatRequest request);

  $async.Future<$0.CompleteInteractionResponse> completeInteraction_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CompleteInteractionRequest> $request) async {
    return completeInteraction($call, await $request);
  }

  $async.Future<$0.CompleteInteractionResponse> completeInteraction(
      $grpc.ServiceCall call, $0.CompleteInteractionRequest request);

  $async.Future<$0.AutocompleteSlashOptionResponse> autocompleteSlashOption_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.AutocompleteSlashOptionRequest> $request) async {
    return autocompleteSlashOption($call, await $request);
  }

  $async.Future<$0.AutocompleteSlashOptionResponse> autocompleteSlashOption(
      $grpc.ServiceCall call, $0.AutocompleteSlashOptionRequest request);

  $async.Future<$0.TouchPresenceResponse> touchPresence_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.TouchPresenceRequest> $request) async {
    return touchPresence($call, await $request);
  }

  $async.Future<$0.TouchPresenceResponse> touchPresence(
      $grpc.ServiceCall call, $0.TouchPresenceRequest request);

  $async.Future<$0.AssignBotRoleResponse> assignBotRole_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.AssignBotRoleRequest> $request) async {
    return assignBotRole($call, await $request);
  }

  $async.Future<$0.AssignBotRoleResponse> assignBotRole(
      $grpc.ServiceCall call, $0.AssignBotRoleRequest request);

  $async.Future<$0.RevokeBotRoleResponse> revokeBotRole_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RevokeBotRoleRequest> $request) async {
    return revokeBotRole($call, await $request);
  }

  $async.Future<$0.RevokeBotRoleResponse> revokeBotRole(
      $grpc.ServiceCall call, $0.RevokeBotRoleRequest request);

  $async.Future<$0.ListSpaceMembersForBotResponse> listSpaceMembersForBot_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListSpaceMembersForBotRequest> $request) async {
    return listSpaceMembersForBot($call, await $request);
  }

  $async.Future<$0.ListSpaceMembersForBotResponse> listSpaceMembersForBot(
      $grpc.ServiceCall call, $0.ListSpaceMembersForBotRequest request);

  $async.Future<$0.CreateBotChatResponse> createBotChat_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CreateBotChatRequest> $request) async {
    return createBotChat($call, await $request);
  }

  $async.Future<$0.CreateBotChatResponse> createBotChat(
      $grpc.ServiceCall call, $0.CreateBotChatRequest request);

  $async.Future<$0.GetChatMessagesForBotResponse> getChatMessagesForBot_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetChatMessagesForBotRequest> $request) async {
    return getChatMessagesForBot($call, await $request);
  }

  $async.Future<$0.GetChatMessagesForBotResponse> getChatMessagesForBot(
      $grpc.ServiceCall call, $0.GetChatMessagesForBotRequest request);

  $async.Future<$0.CreateBotRoleResponse> createBotRole_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CreateBotRoleRequest> $request) async {
    return createBotRole($call, await $request);
  }

  $async.Future<$0.CreateBotRoleResponse> createBotRole(
      $grpc.ServiceCall call, $0.CreateBotRoleRequest request);

  $async.Future<$0.CompleteAutocompleteResponse> completeAutocomplete_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CompleteAutocompleteRequest> $request) async {
    return completeAutocomplete($call, await $request);
  }

  $async.Future<$0.CompleteAutocompleteResponse> completeAutocomplete(
      $grpc.ServiceCall call, $0.CompleteAutocompleteRequest request);
}
