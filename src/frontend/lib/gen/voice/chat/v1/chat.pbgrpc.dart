// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/chat.proto.

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

import 'chat.pb.dart' as $0;

export 'chat.pb.dart';

/// Chats — DM, groups, channels. HTTP: /api/v1/chats/**.
@$pb.GrpcServiceName('voice.chat.v1.ChatService')
class ChatServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  ChatServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.CreateDMResponse> createDM(
    $0.CreateDMRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createDM, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetDMResponse> getDM(
    $0.GetDMRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getDM, request, options: options);
  }

  $grpc.ResponseFuture<$0.CreateChatResponse> createChat(
    $0.CreateChatRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createChat, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdateChatResponse> updateChat(
    $0.UpdateChatRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateChat, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteChatResponse> deleteChat(
    $0.DeleteChatRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteChat, request, options: options);
  }

  $grpc.ResponseFuture<$0.AddMembersResponse> addMembers(
    $0.AddMembersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$addMembers, request, options: options);
  }

  $grpc.ResponseFuture<$0.RemoveMemberResponse> removeMember(
    $0.RemoveMemberRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$removeMember, request, options: options);
  }

  $grpc.ResponseFuture<$0.LeaveChatResponse> leaveChat(
    $0.LeaveChatRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$leaveChat, request, options: options);
  }

  $grpc.ResponseFuture<$0.TransferGroupOwnershipResponse>
      transferGroupOwnership(
    $0.TransferGroupOwnershipRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$transferGroupOwnership, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.ListMembersResponse> listMembers(
    $0.ListMembersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listMembers, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListChatsResponse> listChats(
    $0.ListChatsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listChats, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetChatResponse> getChat(
    $0.GetChatRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getChat, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListFoldersResponse> listFolders(
    $0.ListFoldersRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listFolders, request, options: options);
  }

  $grpc.ResponseFuture<$0.CreateFolderResponse> createFolder(
    $0.CreateFolderRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createFolder, request, options: options);
  }

  $grpc.ResponseFuture<$0.UpdateFolderResponse> updateFolder(
    $0.UpdateFolderRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateFolder, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteFolderResponse> deleteFolder(
    $0.DeleteFolderRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteFolder, request, options: options);
  }

  $grpc.ResponseFuture<$0.AcceptDMRequestResponse> acceptDMRequest(
    $0.AcceptDMRequestRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$acceptDMRequest, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeclineDMRequestResponse> declineDMRequest(
    $0.DeclineDMRequestRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$declineDMRequest, request, options: options);
  }

  $grpc.ResponseFuture<$0.MuteChatResponse> muteChat(
    $0.MuteChatRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$muteChat, request, options: options);
  }

  $grpc.ResponseFuture<$0.ArchiveChatResponse> archiveChat(
    $0.ArchiveChatRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$archiveChat, request, options: options);
  }

  // method descriptors

  static final _$createDM =
      $grpc.ClientMethod<$0.CreateDMRequest, $0.CreateDMResponse>(
          '/voice.chat.v1.ChatService/CreateDM',
          ($0.CreateDMRequest value) => value.writeToBuffer(),
          $0.CreateDMResponse.fromBuffer);
  static final _$getDM = $grpc.ClientMethod<$0.GetDMRequest, $0.GetDMResponse>(
      '/voice.chat.v1.ChatService/GetDM',
      ($0.GetDMRequest value) => value.writeToBuffer(),
      $0.GetDMResponse.fromBuffer);
  static final _$createChat =
      $grpc.ClientMethod<$0.CreateChatRequest, $0.CreateChatResponse>(
          '/voice.chat.v1.ChatService/CreateChat',
          ($0.CreateChatRequest value) => value.writeToBuffer(),
          $0.CreateChatResponse.fromBuffer);
  static final _$updateChat =
      $grpc.ClientMethod<$0.UpdateChatRequest, $0.UpdateChatResponse>(
          '/voice.chat.v1.ChatService/UpdateChat',
          ($0.UpdateChatRequest value) => value.writeToBuffer(),
          $0.UpdateChatResponse.fromBuffer);
  static final _$deleteChat =
      $grpc.ClientMethod<$0.DeleteChatRequest, $0.DeleteChatResponse>(
          '/voice.chat.v1.ChatService/DeleteChat',
          ($0.DeleteChatRequest value) => value.writeToBuffer(),
          $0.DeleteChatResponse.fromBuffer);
  static final _$addMembers =
      $grpc.ClientMethod<$0.AddMembersRequest, $0.AddMembersResponse>(
          '/voice.chat.v1.ChatService/AddMembers',
          ($0.AddMembersRequest value) => value.writeToBuffer(),
          $0.AddMembersResponse.fromBuffer);
  static final _$removeMember =
      $grpc.ClientMethod<$0.RemoveMemberRequest, $0.RemoveMemberResponse>(
          '/voice.chat.v1.ChatService/RemoveMember',
          ($0.RemoveMemberRequest value) => value.writeToBuffer(),
          $0.RemoveMemberResponse.fromBuffer);
  static final _$leaveChat =
      $grpc.ClientMethod<$0.LeaveChatRequest, $0.LeaveChatResponse>(
          '/voice.chat.v1.ChatService/LeaveChat',
          ($0.LeaveChatRequest value) => value.writeToBuffer(),
          $0.LeaveChatResponse.fromBuffer);
  static final _$transferGroupOwnership = $grpc.ClientMethod<
          $0.TransferGroupOwnershipRequest, $0.TransferGroupOwnershipResponse>(
      '/voice.chat.v1.ChatService/TransferGroupOwnership',
      ($0.TransferGroupOwnershipRequest value) => value.writeToBuffer(),
      $0.TransferGroupOwnershipResponse.fromBuffer);
  static final _$listMembers =
      $grpc.ClientMethod<$0.ListMembersRequest, $0.ListMembersResponse>(
          '/voice.chat.v1.ChatService/ListMembers',
          ($0.ListMembersRequest value) => value.writeToBuffer(),
          $0.ListMembersResponse.fromBuffer);
  static final _$listChats =
      $grpc.ClientMethod<$0.ListChatsRequest, $0.ListChatsResponse>(
          '/voice.chat.v1.ChatService/ListChats',
          ($0.ListChatsRequest value) => value.writeToBuffer(),
          $0.ListChatsResponse.fromBuffer);
  static final _$getChat =
      $grpc.ClientMethod<$0.GetChatRequest, $0.GetChatResponse>(
          '/voice.chat.v1.ChatService/GetChat',
          ($0.GetChatRequest value) => value.writeToBuffer(),
          $0.GetChatResponse.fromBuffer);
  static final _$listFolders =
      $grpc.ClientMethod<$0.ListFoldersRequest, $0.ListFoldersResponse>(
          '/voice.chat.v1.ChatService/ListFolders',
          ($0.ListFoldersRequest value) => value.writeToBuffer(),
          $0.ListFoldersResponse.fromBuffer);
  static final _$createFolder =
      $grpc.ClientMethod<$0.CreateFolderRequest, $0.CreateFolderResponse>(
          '/voice.chat.v1.ChatService/CreateFolder',
          ($0.CreateFolderRequest value) => value.writeToBuffer(),
          $0.CreateFolderResponse.fromBuffer);
  static final _$updateFolder =
      $grpc.ClientMethod<$0.UpdateFolderRequest, $0.UpdateFolderResponse>(
          '/voice.chat.v1.ChatService/UpdateFolder',
          ($0.UpdateFolderRequest value) => value.writeToBuffer(),
          $0.UpdateFolderResponse.fromBuffer);
  static final _$deleteFolder =
      $grpc.ClientMethod<$0.DeleteFolderRequest, $0.DeleteFolderResponse>(
          '/voice.chat.v1.ChatService/DeleteFolder',
          ($0.DeleteFolderRequest value) => value.writeToBuffer(),
          $0.DeleteFolderResponse.fromBuffer);
  static final _$acceptDMRequest =
      $grpc.ClientMethod<$0.AcceptDMRequestRequest, $0.AcceptDMRequestResponse>(
          '/voice.chat.v1.ChatService/AcceptDMRequest',
          ($0.AcceptDMRequestRequest value) => value.writeToBuffer(),
          $0.AcceptDMRequestResponse.fromBuffer);
  static final _$declineDMRequest = $grpc.ClientMethod<
          $0.DeclineDMRequestRequest, $0.DeclineDMRequestResponse>(
      '/voice.chat.v1.ChatService/DeclineDMRequest',
      ($0.DeclineDMRequestRequest value) => value.writeToBuffer(),
      $0.DeclineDMRequestResponse.fromBuffer);
  static final _$muteChat =
      $grpc.ClientMethod<$0.MuteChatRequest, $0.MuteChatResponse>(
          '/voice.chat.v1.ChatService/MuteChat',
          ($0.MuteChatRequest value) => value.writeToBuffer(),
          $0.MuteChatResponse.fromBuffer);
  static final _$archiveChat =
      $grpc.ClientMethod<$0.ArchiveChatRequest, $0.ArchiveChatResponse>(
          '/voice.chat.v1.ChatService/ArchiveChat',
          ($0.ArchiveChatRequest value) => value.writeToBuffer(),
          $0.ArchiveChatResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.chat.v1.ChatService')
abstract class ChatServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.chat.v1.ChatService';

  ChatServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.CreateDMRequest, $0.CreateDMResponse>(
        'CreateDM',
        createDM_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.CreateDMRequest.fromBuffer(value),
        ($0.CreateDMResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetDMRequest, $0.GetDMResponse>(
        'GetDM',
        getDM_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetDMRequest.fromBuffer(value),
        ($0.GetDMResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CreateChatRequest, $0.CreateChatResponse>(
        'CreateChat',
        createChat_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.CreateChatRequest.fromBuffer(value),
        ($0.CreateChatResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpdateChatRequest, $0.UpdateChatResponse>(
        'UpdateChat',
        updateChat_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.UpdateChatRequest.fromBuffer(value),
        ($0.UpdateChatResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeleteChatRequest, $0.DeleteChatResponse>(
        'DeleteChat',
        deleteChat_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.DeleteChatRequest.fromBuffer(value),
        ($0.DeleteChatResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AddMembersRequest, $0.AddMembersResponse>(
        'AddMembers',
        addMembers_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.AddMembersRequest.fromBuffer(value),
        ($0.AddMembersResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.RemoveMemberRequest, $0.RemoveMemberResponse>(
            'RemoveMember',
            removeMember_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.RemoveMemberRequest.fromBuffer(value),
            ($0.RemoveMemberResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.LeaveChatRequest, $0.LeaveChatResponse>(
        'LeaveChat',
        leaveChat_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.LeaveChatRequest.fromBuffer(value),
        ($0.LeaveChatResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.TransferGroupOwnershipRequest,
            $0.TransferGroupOwnershipResponse>(
        'TransferGroupOwnership',
        transferGroupOwnership_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.TransferGroupOwnershipRequest.fromBuffer(value),
        ($0.TransferGroupOwnershipResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListMembersRequest, $0.ListMembersResponse>(
            'ListMembers',
            listMembers_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListMembersRequest.fromBuffer(value),
            ($0.ListMembersResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListChatsRequest, $0.ListChatsResponse>(
        'ListChats',
        listChats_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ListChatsRequest.fromBuffer(value),
        ($0.ListChatsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetChatRequest, $0.GetChatResponse>(
        'GetChat',
        getChat_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetChatRequest.fromBuffer(value),
        ($0.GetChatResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListFoldersRequest, $0.ListFoldersResponse>(
            'ListFolders',
            listFolders_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListFoldersRequest.fromBuffer(value),
            ($0.ListFoldersResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.CreateFolderRequest, $0.CreateFolderResponse>(
            'CreateFolder',
            createFolder_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.CreateFolderRequest.fromBuffer(value),
            ($0.CreateFolderResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.UpdateFolderRequest, $0.UpdateFolderResponse>(
            'UpdateFolder',
            updateFolder_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.UpdateFolderRequest.fromBuffer(value),
            ($0.UpdateFolderResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.DeleteFolderRequest, $0.DeleteFolderResponse>(
            'DeleteFolder',
            deleteFolder_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.DeleteFolderRequest.fromBuffer(value),
            ($0.DeleteFolderResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.AcceptDMRequestRequest,
            $0.AcceptDMRequestResponse>(
        'AcceptDMRequest',
        acceptDMRequest_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.AcceptDMRequestRequest.fromBuffer(value),
        ($0.AcceptDMRequestResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeclineDMRequestRequest,
            $0.DeclineDMRequestResponse>(
        'DeclineDMRequest',
        declineDMRequest_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DeclineDMRequestRequest.fromBuffer(value),
        ($0.DeclineDMRequestResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.MuteChatRequest, $0.MuteChatResponse>(
        'MuteChat',
        muteChat_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.MuteChatRequest.fromBuffer(value),
        ($0.MuteChatResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ArchiveChatRequest, $0.ArchiveChatResponse>(
            'ArchiveChat',
            archiveChat_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ArchiveChatRequest.fromBuffer(value),
            ($0.ArchiveChatResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.CreateDMResponse> createDM_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CreateDMRequest> $request) async {
    return createDM($call, await $request);
  }

  $async.Future<$0.CreateDMResponse> createDM(
      $grpc.ServiceCall call, $0.CreateDMRequest request);

  $async.Future<$0.GetDMResponse> getDM_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.GetDMRequest> $request) async {
    return getDM($call, await $request);
  }

  $async.Future<$0.GetDMResponse> getDM(
      $grpc.ServiceCall call, $0.GetDMRequest request);

  $async.Future<$0.CreateChatResponse> createChat_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CreateChatRequest> $request) async {
    return createChat($call, await $request);
  }

  $async.Future<$0.CreateChatResponse> createChat(
      $grpc.ServiceCall call, $0.CreateChatRequest request);

  $async.Future<$0.UpdateChatResponse> updateChat_Pre($grpc.ServiceCall $call,
      $async.Future<$0.UpdateChatRequest> $request) async {
    return updateChat($call, await $request);
  }

  $async.Future<$0.UpdateChatResponse> updateChat(
      $grpc.ServiceCall call, $0.UpdateChatRequest request);

  $async.Future<$0.DeleteChatResponse> deleteChat_Pre($grpc.ServiceCall $call,
      $async.Future<$0.DeleteChatRequest> $request) async {
    return deleteChat($call, await $request);
  }

  $async.Future<$0.DeleteChatResponse> deleteChat(
      $grpc.ServiceCall call, $0.DeleteChatRequest request);

  $async.Future<$0.AddMembersResponse> addMembers_Pre($grpc.ServiceCall $call,
      $async.Future<$0.AddMembersRequest> $request) async {
    return addMembers($call, await $request);
  }

  $async.Future<$0.AddMembersResponse> addMembers(
      $grpc.ServiceCall call, $0.AddMembersRequest request);

  $async.Future<$0.RemoveMemberResponse> removeMember_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RemoveMemberRequest> $request) async {
    return removeMember($call, await $request);
  }

  $async.Future<$0.RemoveMemberResponse> removeMember(
      $grpc.ServiceCall call, $0.RemoveMemberRequest request);

  $async.Future<$0.LeaveChatResponse> leaveChat_Pre($grpc.ServiceCall $call,
      $async.Future<$0.LeaveChatRequest> $request) async {
    return leaveChat($call, await $request);
  }

  $async.Future<$0.LeaveChatResponse> leaveChat(
      $grpc.ServiceCall call, $0.LeaveChatRequest request);

  $async.Future<$0.TransferGroupOwnershipResponse> transferGroupOwnership_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.TransferGroupOwnershipRequest> $request) async {
    return transferGroupOwnership($call, await $request);
  }

  $async.Future<$0.TransferGroupOwnershipResponse> transferGroupOwnership(
      $grpc.ServiceCall call, $0.TransferGroupOwnershipRequest request);

  $async.Future<$0.ListMembersResponse> listMembers_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListMembersRequest> $request) async {
    return listMembers($call, await $request);
  }

  $async.Future<$0.ListMembersResponse> listMembers(
      $grpc.ServiceCall call, $0.ListMembersRequest request);

  $async.Future<$0.ListChatsResponse> listChats_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListChatsRequest> $request) async {
    return listChats($call, await $request);
  }

  $async.Future<$0.ListChatsResponse> listChats(
      $grpc.ServiceCall call, $0.ListChatsRequest request);

  $async.Future<$0.GetChatResponse> getChat_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetChatRequest> $request) async {
    return getChat($call, await $request);
  }

  $async.Future<$0.GetChatResponse> getChat(
      $grpc.ServiceCall call, $0.GetChatRequest request);

  $async.Future<$0.ListFoldersResponse> listFolders_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListFoldersRequest> $request) async {
    return listFolders($call, await $request);
  }

  $async.Future<$0.ListFoldersResponse> listFolders(
      $grpc.ServiceCall call, $0.ListFoldersRequest request);

  $async.Future<$0.CreateFolderResponse> createFolder_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CreateFolderRequest> $request) async {
    return createFolder($call, await $request);
  }

  $async.Future<$0.CreateFolderResponse> createFolder(
      $grpc.ServiceCall call, $0.CreateFolderRequest request);

  $async.Future<$0.UpdateFolderResponse> updateFolder_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UpdateFolderRequest> $request) async {
    return updateFolder($call, await $request);
  }

  $async.Future<$0.UpdateFolderResponse> updateFolder(
      $grpc.ServiceCall call, $0.UpdateFolderRequest request);

  $async.Future<$0.DeleteFolderResponse> deleteFolder_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DeleteFolderRequest> $request) async {
    return deleteFolder($call, await $request);
  }

  $async.Future<$0.DeleteFolderResponse> deleteFolder(
      $grpc.ServiceCall call, $0.DeleteFolderRequest request);

  $async.Future<$0.AcceptDMRequestResponse> acceptDMRequest_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.AcceptDMRequestRequest> $request) async {
    return acceptDMRequest($call, await $request);
  }

  $async.Future<$0.AcceptDMRequestResponse> acceptDMRequest(
      $grpc.ServiceCall call, $0.AcceptDMRequestRequest request);

  $async.Future<$0.DeclineDMRequestResponse> declineDMRequest_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DeclineDMRequestRequest> $request) async {
    return declineDMRequest($call, await $request);
  }

  $async.Future<$0.DeclineDMRequestResponse> declineDMRequest(
      $grpc.ServiceCall call, $0.DeclineDMRequestRequest request);

  $async.Future<$0.MuteChatResponse> muteChat_Pre($grpc.ServiceCall $call,
      $async.Future<$0.MuteChatRequest> $request) async {
    return muteChat($call, await $request);
  }

  $async.Future<$0.MuteChatResponse> muteChat(
      $grpc.ServiceCall call, $0.MuteChatRequest request);

  $async.Future<$0.ArchiveChatResponse> archiveChat_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ArchiveChatRequest> $request) async {
    return archiveChat($call, await $request);
  }

  $async.Future<$0.ArchiveChatResponse> archiveChat(
      $grpc.ServiceCall call, $0.ArchiveChatRequest request);
}
