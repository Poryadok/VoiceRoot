// This is a generated file - do not edit.
//
// Generated from voice/matchmaking/v1/matchmaking.proto.

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

import 'matchmaking.pb.dart' as $0;

export 'matchmaking.pb.dart';

/// Matchmaking. HTTP: /api/v1/matchmaking/**.
@$pb.GrpcServiceName('voice.matchmaking.v1.MatchmakingService')
class MatchmakingServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  MatchmakingServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.ListGamesResponse> listGames(
    $0.ListGamesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listGames, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetGameResponse> getGame(
    $0.GetGameRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getGame, request, options: options);
  }

  $grpc.ResponseFuture<$0.CreateGameResponse> createGame(
    $0.CreateGameRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createGame, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdateGameResponse> updateGame(
    $0.UpdateGameRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateGame, request, options: options);
  }

  $grpc.ResponseFuture<$0.SearchGamesResponse> searchGames(
    $0.SearchGamesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$searchGames, request, options: options);
  }

  $grpc.ResponseFuture<$0.StartSearchResponse> startSearch(
    $0.StartSearchRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$startSearch, request, options: options);
  }

  $grpc.ResponseFuture<$0.CancelSearchResponse> cancelSearch(
    $0.CancelSearchRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$cancelSearch, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetSearchStatusResponse> getSearchStatus(
    $0.GetSearchStatusRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSearchStatus, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetMatchResponse> getMatch(
    $0.GetMatchRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getMatch, request, options: options);
  }

  $grpc.ResponseFuture<$0.RespondToMatchResponse> respondToMatch(
    $0.RespondToMatchRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$respondToMatch, request, options: options);
  }

  $grpc.ResponseFuture<$0.CompleteMatchResponse> completeMatch(
    $0.CompleteMatchRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$completeMatch, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetMatchHistoryResponse> getMatchHistory(
    $0.GetMatchHistoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getMatchHistory, request, options: options);
  }

  $grpc.ResponseFuture<$0.RateMatchResponse> rateMatch(
    $0.RateMatchRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$rateMatch, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetPlayerRatingResponse> getPlayerRating(
    $0.GetPlayerRatingRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getPlayerRating, request, options: options);
  }

  $grpc.ResponseFuture<$0.BanFromMMResponse> banFromMM(
    $0.BanFromMMRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$banFromMM, request, options: options);
  }

  $grpc.ResponseFuture<$0.UnbanFromMMResponse> unbanFromMM(
    $0.UnbanFromMMRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$unbanFromMM, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetMMBanStatusResponse> getMMBanStatus(
    $0.GetMMBanStatusRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getMMBanStatus, request, options: options);
  }

  /// Player profile (per-game MM settings)
  $grpc.ResponseFuture<$0.GetMyPlayerProfileResponse> getMyPlayerProfile(
    $0.GetMyPlayerProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getMyPlayerProfile, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetPlayerProfileResponse> getPlayerProfile(
    $0.GetPlayerProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getPlayerProfile, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpsertPlayerGameEntryResponse> upsertPlayerGameEntry(
    $0.UpsertPlayerGameEntryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$upsertPlayerGameEntry, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeletePlayerGameEntryResponse> deletePlayerGameEntry(
    $0.DeletePlayerGameEntryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deletePlayerGameEntry, request, options: options);
  }

  // method descriptors

  static final _$listGames =
      $grpc.ClientMethod<$0.ListGamesRequest, $0.ListGamesResponse>(
          '/voice.matchmaking.v1.MatchmakingService/ListGames',
          ($0.ListGamesRequest value) => value.writeToBuffer(),
          $0.ListGamesResponse.fromBuffer);
  static final _$getGame =
      $grpc.ClientMethod<$0.GetGameRequest, $0.GetGameResponse>(
          '/voice.matchmaking.v1.MatchmakingService/GetGame',
          ($0.GetGameRequest value) => value.writeToBuffer(),
          $0.GetGameResponse.fromBuffer);
  static final _$createGame =
      $grpc.ClientMethod<$0.CreateGameRequest, $0.CreateGameResponse>(
          '/voice.matchmaking.v1.MatchmakingService/CreateGame',
          ($0.CreateGameRequest value) => value.writeToBuffer(),
          $0.CreateGameResponse.fromBuffer);
  static final _$updateGame =
      $grpc.ClientMethod<$0.UpdateGameRequest, $0.UpdateGameResponse>(
          '/voice.matchmaking.v1.MatchmakingService/UpdateGame',
          ($0.UpdateGameRequest value) => value.writeToBuffer(),
          $0.UpdateGameResponse.fromBuffer);
  static final _$searchGames =
      $grpc.ClientMethod<$0.SearchGamesRequest, $0.SearchGamesResponse>(
          '/voice.matchmaking.v1.MatchmakingService/SearchGames',
          ($0.SearchGamesRequest value) => value.writeToBuffer(),
          $0.SearchGamesResponse.fromBuffer);
  static final _$startSearch =
      $grpc.ClientMethod<$0.StartSearchRequest, $0.StartSearchResponse>(
          '/voice.matchmaking.v1.MatchmakingService/StartSearch',
          ($0.StartSearchRequest value) => value.writeToBuffer(),
          $0.StartSearchResponse.fromBuffer);
  static final _$cancelSearch =
      $grpc.ClientMethod<$0.CancelSearchRequest, $0.CancelSearchResponse>(
          '/voice.matchmaking.v1.MatchmakingService/CancelSearch',
          ($0.CancelSearchRequest value) => value.writeToBuffer(),
          $0.CancelSearchResponse.fromBuffer);
  static final _$getSearchStatus =
      $grpc.ClientMethod<$0.GetSearchStatusRequest, $0.GetSearchStatusResponse>(
          '/voice.matchmaking.v1.MatchmakingService/GetSearchStatus',
          ($0.GetSearchStatusRequest value) => value.writeToBuffer(),
          $0.GetSearchStatusResponse.fromBuffer);
  static final _$getMatch =
      $grpc.ClientMethod<$0.GetMatchRequest, $0.GetMatchResponse>(
          '/voice.matchmaking.v1.MatchmakingService/GetMatch',
          ($0.GetMatchRequest value) => value.writeToBuffer(),
          $0.GetMatchResponse.fromBuffer);
  static final _$respondToMatch =
      $grpc.ClientMethod<$0.RespondToMatchRequest, $0.RespondToMatchResponse>(
          '/voice.matchmaking.v1.MatchmakingService/RespondToMatch',
          ($0.RespondToMatchRequest value) => value.writeToBuffer(),
          $0.RespondToMatchResponse.fromBuffer);
  static final _$completeMatch =
      $grpc.ClientMethod<$0.CompleteMatchRequest, $0.CompleteMatchResponse>(
          '/voice.matchmaking.v1.MatchmakingService/CompleteMatch',
          ($0.CompleteMatchRequest value) => value.writeToBuffer(),
          $0.CompleteMatchResponse.fromBuffer);
  static final _$getMatchHistory =
      $grpc.ClientMethod<$0.GetMatchHistoryRequest, $0.GetMatchHistoryResponse>(
          '/voice.matchmaking.v1.MatchmakingService/GetMatchHistory',
          ($0.GetMatchHistoryRequest value) => value.writeToBuffer(),
          $0.GetMatchHistoryResponse.fromBuffer);
  static final _$rateMatch =
      $grpc.ClientMethod<$0.RateMatchRequest, $0.RateMatchResponse>(
          '/voice.matchmaking.v1.MatchmakingService/RateMatch',
          ($0.RateMatchRequest value) => value.writeToBuffer(),
          $0.RateMatchResponse.fromBuffer);
  static final _$getPlayerRating =
      $grpc.ClientMethod<$0.GetPlayerRatingRequest, $0.GetPlayerRatingResponse>(
          '/voice.matchmaking.v1.MatchmakingService/GetPlayerRating',
          ($0.GetPlayerRatingRequest value) => value.writeToBuffer(),
          $0.GetPlayerRatingResponse.fromBuffer);
  static final _$banFromMM =
      $grpc.ClientMethod<$0.BanFromMMRequest, $0.BanFromMMResponse>(
          '/voice.matchmaking.v1.MatchmakingService/BanFromMM',
          ($0.BanFromMMRequest value) => value.writeToBuffer(),
          $0.BanFromMMResponse.fromBuffer);
  static final _$unbanFromMM =
      $grpc.ClientMethod<$0.UnbanFromMMRequest, $0.UnbanFromMMResponse>(
          '/voice.matchmaking.v1.MatchmakingService/UnbanFromMM',
          ($0.UnbanFromMMRequest value) => value.writeToBuffer(),
          $0.UnbanFromMMResponse.fromBuffer);
  static final _$getMMBanStatus =
      $grpc.ClientMethod<$0.GetMMBanStatusRequest, $0.GetMMBanStatusResponse>(
          '/voice.matchmaking.v1.MatchmakingService/GetMMBanStatus',
          ($0.GetMMBanStatusRequest value) => value.writeToBuffer(),
          $0.GetMMBanStatusResponse.fromBuffer);
  static final _$getMyPlayerProfile = $grpc.ClientMethod<
          $0.GetMyPlayerProfileRequest, $0.GetMyPlayerProfileResponse>(
      '/voice.matchmaking.v1.MatchmakingService/GetMyPlayerProfile',
      ($0.GetMyPlayerProfileRequest value) => value.writeToBuffer(),
      $0.GetMyPlayerProfileResponse.fromBuffer);
  static final _$getPlayerProfile = $grpc.ClientMethod<
          $0.GetPlayerProfileRequest, $0.GetPlayerProfileResponse>(
      '/voice.matchmaking.v1.MatchmakingService/GetPlayerProfile',
      ($0.GetPlayerProfileRequest value) => value.writeToBuffer(),
      $0.GetPlayerProfileResponse.fromBuffer);
  static final _$upsertPlayerGameEntry = $grpc.ClientMethod<
          $0.UpsertPlayerGameEntryRequest, $0.UpsertPlayerGameEntryResponse>(
      '/voice.matchmaking.v1.MatchmakingService/UpsertPlayerGameEntry',
      ($0.UpsertPlayerGameEntryRequest value) => value.writeToBuffer(),
      $0.UpsertPlayerGameEntryResponse.fromBuffer);
  static final _$deletePlayerGameEntry = $grpc.ClientMethod<
          $0.DeletePlayerGameEntryRequest, $0.DeletePlayerGameEntryResponse>(
      '/voice.matchmaking.v1.MatchmakingService/DeletePlayerGameEntry',
      ($0.DeletePlayerGameEntryRequest value) => value.writeToBuffer(),
      $0.DeletePlayerGameEntryResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.matchmaking.v1.MatchmakingService')
abstract class MatchmakingServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.matchmaking.v1.MatchmakingService';

  MatchmakingServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ListGamesRequest, $0.ListGamesResponse>(
        'ListGames',
        listGames_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ListGamesRequest.fromBuffer(value),
        ($0.ListGamesResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetGameRequest, $0.GetGameResponse>(
        'GetGame',
        getGame_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetGameRequest.fromBuffer(value),
        ($0.GetGameResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CreateGameRequest, $0.CreateGameResponse>(
        'CreateGame',
        createGame_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.CreateGameRequest.fromBuffer(value),
        ($0.CreateGameResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpdateGameRequest, $0.UpdateGameResponse>(
        'UpdateGame',
        updateGame_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.UpdateGameRequest.fromBuffer(value),
        ($0.UpdateGameResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SearchGamesRequest, $0.SearchGamesResponse>(
            'SearchGames',
            searchGames_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SearchGamesRequest.fromBuffer(value),
            ($0.SearchGamesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.StartSearchRequest, $0.StartSearchResponse>(
            'StartSearch',
            startSearch_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.StartSearchRequest.fromBuffer(value),
            ($0.StartSearchResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.CancelSearchRequest, $0.CancelSearchResponse>(
            'CancelSearch',
            cancelSearch_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.CancelSearchRequest.fromBuffer(value),
            ($0.CancelSearchResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetSearchStatusRequest,
            $0.GetSearchStatusResponse>(
        'GetSearchStatus',
        getSearchStatus_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetSearchStatusRequest.fromBuffer(value),
        ($0.GetSearchStatusResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetMatchRequest, $0.GetMatchResponse>(
        'GetMatch',
        getMatch_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetMatchRequest.fromBuffer(value),
        ($0.GetMatchResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RespondToMatchRequest,
            $0.RespondToMatchResponse>(
        'RespondToMatch',
        respondToMatch_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RespondToMatchRequest.fromBuffer(value),
        ($0.RespondToMatchResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.CompleteMatchRequest, $0.CompleteMatchResponse>(
            'CompleteMatch',
            completeMatch_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.CompleteMatchRequest.fromBuffer(value),
            ($0.CompleteMatchResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetMatchHistoryRequest,
            $0.GetMatchHistoryResponse>(
        'GetMatchHistory',
        getMatchHistory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetMatchHistoryRequest.fromBuffer(value),
        ($0.GetMatchHistoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RateMatchRequest, $0.RateMatchResponse>(
        'RateMatch',
        rateMatch_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.RateMatchRequest.fromBuffer(value),
        ($0.RateMatchResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetPlayerRatingRequest,
            $0.GetPlayerRatingResponse>(
        'GetPlayerRating',
        getPlayerRating_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetPlayerRatingRequest.fromBuffer(value),
        ($0.GetPlayerRatingResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.BanFromMMRequest, $0.BanFromMMResponse>(
        'BanFromMM',
        banFromMM_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.BanFromMMRequest.fromBuffer(value),
        ($0.BanFromMMResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.UnbanFromMMRequest, $0.UnbanFromMMResponse>(
            'UnbanFromMM',
            unbanFromMM_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.UnbanFromMMRequest.fromBuffer(value),
            ($0.UnbanFromMMResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetMMBanStatusRequest,
            $0.GetMMBanStatusResponse>(
        'GetMMBanStatus',
        getMMBanStatus_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetMMBanStatusRequest.fromBuffer(value),
        ($0.GetMMBanStatusResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetMyPlayerProfileRequest,
            $0.GetMyPlayerProfileResponse>(
        'GetMyPlayerProfile',
        getMyPlayerProfile_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetMyPlayerProfileRequest.fromBuffer(value),
        ($0.GetMyPlayerProfileResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetPlayerProfileRequest,
            $0.GetPlayerProfileResponse>(
        'GetPlayerProfile',
        getPlayerProfile_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetPlayerProfileRequest.fromBuffer(value),
        ($0.GetPlayerProfileResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpsertPlayerGameEntryRequest,
            $0.UpsertPlayerGameEntryResponse>(
        'UpsertPlayerGameEntry',
        upsertPlayerGameEntry_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UpsertPlayerGameEntryRequest.fromBuffer(value),
        ($0.UpsertPlayerGameEntryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeletePlayerGameEntryRequest,
            $0.DeletePlayerGameEntryResponse>(
        'DeletePlayerGameEntry',
        deletePlayerGameEntry_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DeletePlayerGameEntryRequest.fromBuffer(value),
        ($0.DeletePlayerGameEntryResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.ListGamesResponse> listGames_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListGamesRequest> $request) async {
    return listGames($call, await $request);
  }

  $async.Future<$0.ListGamesResponse> listGames(
      $grpc.ServiceCall call, $0.ListGamesRequest request);

  $async.Future<$0.GetGameResponse> getGame_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetGameRequest> $request) async {
    return getGame($call, await $request);
  }

  $async.Future<$0.GetGameResponse> getGame(
      $grpc.ServiceCall call, $0.GetGameRequest request);

  $async.Future<$0.CreateGameResponse> createGame_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CreateGameRequest> $request) async {
    return createGame($call, await $request);
  }

  $async.Future<$0.CreateGameResponse> createGame(
      $grpc.ServiceCall call, $0.CreateGameRequest request);

  $async.Future<$0.UpdateGameResponse> updateGame_Pre($grpc.ServiceCall $call,
      $async.Future<$0.UpdateGameRequest> $request) async {
    return updateGame($call, await $request);
  }

  $async.Future<$0.UpdateGameResponse> updateGame(
      $grpc.ServiceCall call, $0.UpdateGameRequest request);

  $async.Future<$0.SearchGamesResponse> searchGames_Pre($grpc.ServiceCall $call,
      $async.Future<$0.SearchGamesRequest> $request) async {
    return searchGames($call, await $request);
  }

  $async.Future<$0.SearchGamesResponse> searchGames(
      $grpc.ServiceCall call, $0.SearchGamesRequest request);

  $async.Future<$0.StartSearchResponse> startSearch_Pre($grpc.ServiceCall $call,
      $async.Future<$0.StartSearchRequest> $request) async {
    return startSearch($call, await $request);
  }

  $async.Future<$0.StartSearchResponse> startSearch(
      $grpc.ServiceCall call, $0.StartSearchRequest request);

  $async.Future<$0.CancelSearchResponse> cancelSearch_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CancelSearchRequest> $request) async {
    return cancelSearch($call, await $request);
  }

  $async.Future<$0.CancelSearchResponse> cancelSearch(
      $grpc.ServiceCall call, $0.CancelSearchRequest request);

  $async.Future<$0.GetSearchStatusResponse> getSearchStatus_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetSearchStatusRequest> $request) async {
    return getSearchStatus($call, await $request);
  }

  $async.Future<$0.GetSearchStatusResponse> getSearchStatus(
      $grpc.ServiceCall call, $0.GetSearchStatusRequest request);

  $async.Future<$0.GetMatchResponse> getMatch_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetMatchRequest> $request) async {
    return getMatch($call, await $request);
  }

  $async.Future<$0.GetMatchResponse> getMatch(
      $grpc.ServiceCall call, $0.GetMatchRequest request);

  $async.Future<$0.RespondToMatchResponse> respondToMatch_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RespondToMatchRequest> $request) async {
    return respondToMatch($call, await $request);
  }

  $async.Future<$0.RespondToMatchResponse> respondToMatch(
      $grpc.ServiceCall call, $0.RespondToMatchRequest request);

  $async.Future<$0.CompleteMatchResponse> completeMatch_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CompleteMatchRequest> $request) async {
    return completeMatch($call, await $request);
  }

  $async.Future<$0.CompleteMatchResponse> completeMatch(
      $grpc.ServiceCall call, $0.CompleteMatchRequest request);

  $async.Future<$0.GetMatchHistoryResponse> getMatchHistory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetMatchHistoryRequest> $request) async {
    return getMatchHistory($call, await $request);
  }

  $async.Future<$0.GetMatchHistoryResponse> getMatchHistory(
      $grpc.ServiceCall call, $0.GetMatchHistoryRequest request);

  $async.Future<$0.RateMatchResponse> rateMatch_Pre($grpc.ServiceCall $call,
      $async.Future<$0.RateMatchRequest> $request) async {
    return rateMatch($call, await $request);
  }

  $async.Future<$0.RateMatchResponse> rateMatch(
      $grpc.ServiceCall call, $0.RateMatchRequest request);

  $async.Future<$0.GetPlayerRatingResponse> getPlayerRating_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetPlayerRatingRequest> $request) async {
    return getPlayerRating($call, await $request);
  }

  $async.Future<$0.GetPlayerRatingResponse> getPlayerRating(
      $grpc.ServiceCall call, $0.GetPlayerRatingRequest request);

  $async.Future<$0.BanFromMMResponse> banFromMM_Pre($grpc.ServiceCall $call,
      $async.Future<$0.BanFromMMRequest> $request) async {
    return banFromMM($call, await $request);
  }

  $async.Future<$0.BanFromMMResponse> banFromMM(
      $grpc.ServiceCall call, $0.BanFromMMRequest request);

  $async.Future<$0.UnbanFromMMResponse> unbanFromMM_Pre($grpc.ServiceCall $call,
      $async.Future<$0.UnbanFromMMRequest> $request) async {
    return unbanFromMM($call, await $request);
  }

  $async.Future<$0.UnbanFromMMResponse> unbanFromMM(
      $grpc.ServiceCall call, $0.UnbanFromMMRequest request);

  $async.Future<$0.GetMMBanStatusResponse> getMMBanStatus_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetMMBanStatusRequest> $request) async {
    return getMMBanStatus($call, await $request);
  }

  $async.Future<$0.GetMMBanStatusResponse> getMMBanStatus(
      $grpc.ServiceCall call, $0.GetMMBanStatusRequest request);

  $async.Future<$0.GetMyPlayerProfileResponse> getMyPlayerProfile_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetMyPlayerProfileRequest> $request) async {
    return getMyPlayerProfile($call, await $request);
  }

  $async.Future<$0.GetMyPlayerProfileResponse> getMyPlayerProfile(
      $grpc.ServiceCall call, $0.GetMyPlayerProfileRequest request);

  $async.Future<$0.GetPlayerProfileResponse> getPlayerProfile_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetPlayerProfileRequest> $request) async {
    return getPlayerProfile($call, await $request);
  }

  $async.Future<$0.GetPlayerProfileResponse> getPlayerProfile(
      $grpc.ServiceCall call, $0.GetPlayerProfileRequest request);

  $async.Future<$0.UpsertPlayerGameEntryResponse> upsertPlayerGameEntry_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UpsertPlayerGameEntryRequest> $request) async {
    return upsertPlayerGameEntry($call, await $request);
  }

  $async.Future<$0.UpsertPlayerGameEntryResponse> upsertPlayerGameEntry(
      $grpc.ServiceCall call, $0.UpsertPlayerGameEntryRequest request);

  $async.Future<$0.DeletePlayerGameEntryResponse> deletePlayerGameEntry_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DeletePlayerGameEntryRequest> $request) async {
    return deletePlayerGameEntry($call, await $request);
  }

  $async.Future<$0.DeletePlayerGameEntryResponse> deletePlayerGameEntry(
      $grpc.ServiceCall call, $0.DeletePlayerGameEntryRequest request);
}
