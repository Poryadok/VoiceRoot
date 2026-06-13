// This is a generated file - do not edit.
//
// Generated from voice/social/v1/social.proto.

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

import 'social.pb.dart' as $0;

export 'social.pb.dart';

/// Social graph — friends, contacts, blocks. HTTP: /api/v1/friends/**.
@$pb.GrpcServiceName('voice.social.v1.SocialService')
class SocialServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  SocialServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.SendFriendInvitationResponse> sendFriendInvitation(
    $0.SendFriendInvitationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$sendFriendInvitation, request, options: options);
  }

  $grpc.ResponseFuture<$0.AcceptFriendInvitationResponse>
      acceptFriendInvitation(
    $0.AcceptFriendInvitationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$acceptFriendInvitation, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.DeclineFriendInvitationResponse>
      declineFriendInvitation(
    $0.DeclineFriendInvitationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$declineFriendInvitation, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.RemoveFriendResponse> removeFriend(
    $0.RemoveFriendRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeFriend, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListFriendsResponse> listFriends(
    $0.ListFriendsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listFriends, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListFriendRequestsResponse> listFriendRequests(
    $0.ListFriendRequestsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listFriendRequests, request, options: options);
  }

  $grpc.ResponseFuture<$0.AddContactResponse> addContact(
    $0.AddContactRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$addContact, request, options: options);
  }

  $grpc.ResponseFuture<$0.RemoveContactResponse> removeContact(
    $0.RemoveContactRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeContact, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListContactsResponse> listContacts(
    $0.ListContactsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listContacts, request, options: options);
  }

  $grpc.ResponseFuture<$0.SyncPhoneContactsResponse> syncPhoneContacts(
    $0.SyncPhoneContactsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$syncPhoneContacts, request, options: options);
  }

  $grpc.ResponseFuture<$0.SetFavoriteResponse> setFavorite(
    $0.SetFavoriteRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setFavorite, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListFavoritesResponse> listFavorites(
    $0.ListFavoritesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listFavorites, request, options: options);
  }

  $grpc.ResponseFuture<$0.BlockAccountResponse> blockAccount(
    $0.BlockAccountRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$blockAccount, request, options: options);
  }

  $grpc.ResponseFuture<$0.UnblockAccountResponse> unblockAccount(
    $0.UnblockAccountRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$unblockAccount, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListBlockedResponse> listBlocked(
    $0.ListBlockedRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listBlocked, request, options: options);
  }

  $grpc.ResponseFuture<$0.IsBlockedResponse> isBlocked(
    $0.IsBlockedRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$isBlocked, request, options: options);
  }

  $grpc.ResponseFuture<$0.AreFriendsResponse> areFriends(
    $0.AreFriendsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$areFriends, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetFriendsOfFriendsResponse> getFriendsOfFriends(
    $0.GetFriendsOfFriendsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getFriendsOfFriends, request, options: options);
  }

  // method descriptors

  static final _$sendFriendInvitation = $grpc.ClientMethod<
          $0.SendFriendInvitationRequest, $0.SendFriendInvitationResponse>(
      '/voice.social.v1.SocialService/SendFriendInvitation',
      ($0.SendFriendInvitationRequest value) => value.writeToBuffer(),
      $0.SendFriendInvitationResponse.fromBuffer);
  static final _$acceptFriendInvitation = $grpc.ClientMethod<
          $0.AcceptFriendInvitationRequest, $0.AcceptFriendInvitationResponse>(
      '/voice.social.v1.SocialService/AcceptFriendInvitation',
      ($0.AcceptFriendInvitationRequest value) => value.writeToBuffer(),
      $0.AcceptFriendInvitationResponse.fromBuffer);
  static final _$declineFriendInvitation = $grpc.ClientMethod<
          $0.DeclineFriendInvitationRequest,
          $0.DeclineFriendInvitationResponse>(
      '/voice.social.v1.SocialService/DeclineFriendInvitation',
      ($0.DeclineFriendInvitationRequest value) => value.writeToBuffer(),
      $0.DeclineFriendInvitationResponse.fromBuffer);
  static final _$removeFriend =
      $grpc.ClientMethod<$0.RemoveFriendRequest, $0.RemoveFriendResponse>(
          '/voice.social.v1.SocialService/RemoveFriend',
          ($0.RemoveFriendRequest value) => value.writeToBuffer(),
          $0.RemoveFriendResponse.fromBuffer);
  static final _$listFriends =
      $grpc.ClientMethod<$0.ListFriendsRequest, $0.ListFriendsResponse>(
          '/voice.social.v1.SocialService/ListFriends',
          ($0.ListFriendsRequest value) => value.writeToBuffer(),
          $0.ListFriendsResponse.fromBuffer);
  static final _$listFriendRequests = $grpc.ClientMethod<
          $0.ListFriendRequestsRequest, $0.ListFriendRequestsResponse>(
      '/voice.social.v1.SocialService/ListFriendRequests',
      ($0.ListFriendRequestsRequest value) => value.writeToBuffer(),
      $0.ListFriendRequestsResponse.fromBuffer);
  static final _$addContact =
      $grpc.ClientMethod<$0.AddContactRequest, $0.AddContactResponse>(
          '/voice.social.v1.SocialService/AddContact',
          ($0.AddContactRequest value) => value.writeToBuffer(),
          $0.AddContactResponse.fromBuffer);
  static final _$removeContact =
      $grpc.ClientMethod<$0.RemoveContactRequest, $0.RemoveContactResponse>(
          '/voice.social.v1.SocialService/RemoveContact',
          ($0.RemoveContactRequest value) => value.writeToBuffer(),
          $0.RemoveContactResponse.fromBuffer);
  static final _$listContacts =
      $grpc.ClientMethod<$0.ListContactsRequest, $0.ListContactsResponse>(
          '/voice.social.v1.SocialService/ListContacts',
          ($0.ListContactsRequest value) => value.writeToBuffer(),
          $0.ListContactsResponse.fromBuffer);
  static final _$syncPhoneContacts = $grpc.ClientMethod<
          $0.SyncPhoneContactsRequest, $0.SyncPhoneContactsResponse>(
      '/voice.social.v1.SocialService/SyncPhoneContacts',
      ($0.SyncPhoneContactsRequest value) => value.writeToBuffer(),
      $0.SyncPhoneContactsResponse.fromBuffer);
  static final _$setFavorite =
      $grpc.ClientMethod<$0.SetFavoriteRequest, $0.SetFavoriteResponse>(
          '/voice.social.v1.SocialService/SetFavorite',
          ($0.SetFavoriteRequest value) => value.writeToBuffer(),
          $0.SetFavoriteResponse.fromBuffer);
  static final _$listFavorites =
      $grpc.ClientMethod<$0.ListFavoritesRequest, $0.ListFavoritesResponse>(
          '/voice.social.v1.SocialService/ListFavorites',
          ($0.ListFavoritesRequest value) => value.writeToBuffer(),
          $0.ListFavoritesResponse.fromBuffer);
  static final _$blockAccount =
      $grpc.ClientMethod<$0.BlockAccountRequest, $0.BlockAccountResponse>(
          '/voice.social.v1.SocialService/BlockAccount',
          ($0.BlockAccountRequest value) => value.writeToBuffer(),
          $0.BlockAccountResponse.fromBuffer);
  static final _$unblockAccount =
      $grpc.ClientMethod<$0.UnblockAccountRequest, $0.UnblockAccountResponse>(
          '/voice.social.v1.SocialService/UnblockAccount',
          ($0.UnblockAccountRequest value) => value.writeToBuffer(),
          $0.UnblockAccountResponse.fromBuffer);
  static final _$listBlocked =
      $grpc.ClientMethod<$0.ListBlockedRequest, $0.ListBlockedResponse>(
          '/voice.social.v1.SocialService/ListBlocked',
          ($0.ListBlockedRequest value) => value.writeToBuffer(),
          $0.ListBlockedResponse.fromBuffer);
  static final _$isBlocked =
      $grpc.ClientMethod<$0.IsBlockedRequest, $0.IsBlockedResponse>(
          '/voice.social.v1.SocialService/IsBlocked',
          ($0.IsBlockedRequest value) => value.writeToBuffer(),
          $0.IsBlockedResponse.fromBuffer);
  static final _$areFriends =
      $grpc.ClientMethod<$0.AreFriendsRequest, $0.AreFriendsResponse>(
          '/voice.social.v1.SocialService/AreFriends',
          ($0.AreFriendsRequest value) => value.writeToBuffer(),
          $0.AreFriendsResponse.fromBuffer);
  static final _$getFriendsOfFriends = $grpc.ClientMethod<
          $0.GetFriendsOfFriendsRequest, $0.GetFriendsOfFriendsResponse>(
      '/voice.social.v1.SocialService/GetFriendsOfFriends',
      ($0.GetFriendsOfFriendsRequest value) => value.writeToBuffer(),
      $0.GetFriendsOfFriendsResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.social.v1.SocialService')
abstract class SocialServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.social.v1.SocialService';

  SocialServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.SendFriendInvitationRequest,
            $0.SendFriendInvitationResponse>(
        'SendFriendInvitation',
        sendFriendInvitation_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SendFriendInvitationRequest.fromBuffer(value),
        ($0.SendFriendInvitationResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AcceptFriendInvitationRequest,
            $0.AcceptFriendInvitationResponse>(
        'AcceptFriendInvitation',
        acceptFriendInvitation_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.AcceptFriendInvitationRequest.fromBuffer(value),
        ($0.AcceptFriendInvitationResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeclineFriendInvitationRequest,
            $0.DeclineFriendInvitationResponse>(
        'DeclineFriendInvitation',
        declineFriendInvitation_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DeclineFriendInvitationRequest.fromBuffer(value),
        ($0.DeclineFriendInvitationResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.RemoveFriendRequest, $0.RemoveFriendResponse>(
            'RemoveFriend',
            removeFriend_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.RemoveFriendRequest.fromBuffer(value),
            ($0.RemoveFriendResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListFriendsRequest, $0.ListFriendsResponse>(
            'ListFriends',
            listFriends_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListFriendsRequest.fromBuffer(value),
            ($0.ListFriendsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListFriendRequestsRequest,
            $0.ListFriendRequestsResponse>(
        'ListFriendRequests',
        listFriendRequests_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ListFriendRequestsRequest.fromBuffer(value),
        ($0.ListFriendRequestsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AddContactRequest, $0.AddContactResponse>(
        'AddContact',
        addContact_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.AddContactRequest.fromBuffer(value),
        ($0.AddContactResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.RemoveContactRequest, $0.RemoveContactResponse>(
            'RemoveContact',
            removeContact_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.RemoveContactRequest.fromBuffer(value),
            ($0.RemoveContactResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListContactsRequest, $0.ListContactsResponse>(
            'ListContacts',
            listContacts_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListContactsRequest.fromBuffer(value),
            ($0.ListContactsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SyncPhoneContactsRequest,
            $0.SyncPhoneContactsResponse>(
        'SyncPhoneContacts',
        syncPhoneContacts_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SyncPhoneContactsRequest.fromBuffer(value),
        ($0.SyncPhoneContactsResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SetFavoriteRequest, $0.SetFavoriteResponse>(
            'SetFavorite',
            setFavorite_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SetFavoriteRequest.fromBuffer(value),
            ($0.SetFavoriteResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListFavoritesRequest, $0.ListFavoritesResponse>(
            'ListFavorites',
            listFavorites_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListFavoritesRequest.fromBuffer(value),
            ($0.ListFavoritesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.BlockAccountRequest, $0.BlockAccountResponse>(
            'BlockAccount',
            blockAccount_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.BlockAccountRequest.fromBuffer(value),
            ($0.BlockAccountResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UnblockAccountRequest,
            $0.UnblockAccountResponse>(
        'UnblockAccount',
        unblockAccount_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UnblockAccountRequest.fromBuffer(value),
        ($0.UnblockAccountResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListBlockedRequest, $0.ListBlockedResponse>(
            'ListBlocked',
            listBlocked_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListBlockedRequest.fromBuffer(value),
            ($0.ListBlockedResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.IsBlockedRequest, $0.IsBlockedResponse>(
        'IsBlocked',
        isBlocked_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.IsBlockedRequest.fromBuffer(value),
        ($0.IsBlockedResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AreFriendsRequest, $0.AreFriendsResponse>(
        'AreFriends',
        areFriends_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.AreFriendsRequest.fromBuffer(value),
        ($0.AreFriendsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetFriendsOfFriendsRequest,
            $0.GetFriendsOfFriendsResponse>(
        'GetFriendsOfFriends',
        getFriendsOfFriends_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetFriendsOfFriendsRequest.fromBuffer(value),
        ($0.GetFriendsOfFriendsResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.SendFriendInvitationResponse> sendFriendInvitation_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SendFriendInvitationRequest> $request) async {
    return sendFriendInvitation($call, await $request);
  }

  $async.Future<$0.SendFriendInvitationResponse> sendFriendInvitation(
      $grpc.ServiceCall call, $0.SendFriendInvitationRequest request);

  $async.Future<$0.AcceptFriendInvitationResponse> acceptFriendInvitation_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.AcceptFriendInvitationRequest> $request) async {
    return acceptFriendInvitation($call, await $request);
  }

  $async.Future<$0.AcceptFriendInvitationResponse> acceptFriendInvitation(
      $grpc.ServiceCall call, $0.AcceptFriendInvitationRequest request);

  $async.Future<$0.DeclineFriendInvitationResponse> declineFriendInvitation_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DeclineFriendInvitationRequest> $request) async {
    return declineFriendInvitation($call, await $request);
  }

  $async.Future<$0.DeclineFriendInvitationResponse> declineFriendInvitation(
      $grpc.ServiceCall call, $0.DeclineFriendInvitationRequest request);

  $async.Future<$0.RemoveFriendResponse> removeFriend_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RemoveFriendRequest> $request) async {
    return removeFriend($call, await $request);
  }

  $async.Future<$0.RemoveFriendResponse> removeFriend(
      $grpc.ServiceCall call, $0.RemoveFriendRequest request);

  $async.Future<$0.ListFriendsResponse> listFriends_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListFriendsRequest> $request) async {
    return listFriends($call, await $request);
  }

  $async.Future<$0.ListFriendsResponse> listFriends(
      $grpc.ServiceCall call, $0.ListFriendsRequest request);

  $async.Future<$0.ListFriendRequestsResponse> listFriendRequests_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListFriendRequestsRequest> $request) async {
    return listFriendRequests($call, await $request);
  }

  $async.Future<$0.ListFriendRequestsResponse> listFriendRequests(
      $grpc.ServiceCall call, $0.ListFriendRequestsRequest request);

  $async.Future<$0.AddContactResponse> addContact_Pre($grpc.ServiceCall $call,
      $async.Future<$0.AddContactRequest> $request) async {
    return addContact($call, await $request);
  }

  $async.Future<$0.AddContactResponse> addContact(
      $grpc.ServiceCall call, $0.AddContactRequest request);

  $async.Future<$0.RemoveContactResponse> removeContact_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RemoveContactRequest> $request) async {
    return removeContact($call, await $request);
  }

  $async.Future<$0.RemoveContactResponse> removeContact(
      $grpc.ServiceCall call, $0.RemoveContactRequest request);

  $async.Future<$0.ListContactsResponse> listContacts_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListContactsRequest> $request) async {
    return listContacts($call, await $request);
  }

  $async.Future<$0.ListContactsResponse> listContacts(
      $grpc.ServiceCall call, $0.ListContactsRequest request);

  $async.Future<$0.SyncPhoneContactsResponse> syncPhoneContacts_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SyncPhoneContactsRequest> $request) async {
    return syncPhoneContacts($call, await $request);
  }

  $async.Future<$0.SyncPhoneContactsResponse> syncPhoneContacts(
      $grpc.ServiceCall call, $0.SyncPhoneContactsRequest request);

  $async.Future<$0.SetFavoriteResponse> setFavorite_Pre($grpc.ServiceCall $call,
      $async.Future<$0.SetFavoriteRequest> $request) async {
    return setFavorite($call, await $request);
  }

  $async.Future<$0.SetFavoriteResponse> setFavorite(
      $grpc.ServiceCall call, $0.SetFavoriteRequest request);

  $async.Future<$0.ListFavoritesResponse> listFavorites_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ListFavoritesRequest> $request) async {
    return listFavorites($call, await $request);
  }

  $async.Future<$0.ListFavoritesResponse> listFavorites(
      $grpc.ServiceCall call, $0.ListFavoritesRequest request);

  $async.Future<$0.BlockAccountResponse> blockAccount_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.BlockAccountRequest> $request) async {
    return blockAccount($call, await $request);
  }

  $async.Future<$0.BlockAccountResponse> blockAccount(
      $grpc.ServiceCall call, $0.BlockAccountRequest request);

  $async.Future<$0.UnblockAccountResponse> unblockAccount_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UnblockAccountRequest> $request) async {
    return unblockAccount($call, await $request);
  }

  $async.Future<$0.UnblockAccountResponse> unblockAccount(
      $grpc.ServiceCall call, $0.UnblockAccountRequest request);

  $async.Future<$0.ListBlockedResponse> listBlocked_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListBlockedRequest> $request) async {
    return listBlocked($call, await $request);
  }

  $async.Future<$0.ListBlockedResponse> listBlocked(
      $grpc.ServiceCall call, $0.ListBlockedRequest request);

  $async.Future<$0.IsBlockedResponse> isBlocked_Pre($grpc.ServiceCall $call,
      $async.Future<$0.IsBlockedRequest> $request) async {
    return isBlocked($call, await $request);
  }

  $async.Future<$0.IsBlockedResponse> isBlocked(
      $grpc.ServiceCall call, $0.IsBlockedRequest request);

  $async.Future<$0.AreFriendsResponse> areFriends_Pre($grpc.ServiceCall $call,
      $async.Future<$0.AreFriendsRequest> $request) async {
    return areFriends($call, await $request);
  }

  $async.Future<$0.AreFriendsResponse> areFriends(
      $grpc.ServiceCall call, $0.AreFriendsRequest request);

  $async.Future<$0.GetFriendsOfFriendsResponse> getFriendsOfFriends_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetFriendsOfFriendsRequest> $request) async {
    return getFriendsOfFriends($call, await $request);
  }

  $async.Future<$0.GetFriendsOfFriendsResponse> getFriendsOfFriends(
      $grpc.ServiceCall call, $0.GetFriendsOfFriendsRequest request);
}
