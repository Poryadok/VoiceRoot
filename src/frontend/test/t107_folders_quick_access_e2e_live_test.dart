import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/backend/chats_client.dart';

import 'support/live_gateway_harness.dart';

/// T-107 proves Chat-owned folder membership and Quick Access survive
/// independent Gateway clients and an Auth session refresh.
void main() {
  test(
    'folders and Quick Access persist across refreshed sessions',
    () async {
      final probe = await probeLiveGateway();
      expect(
        probe,
        isA<LiveGatewayReady>(),
        reason: probe is LiveGatewayUnavailable ? probe.reason : null,
      );
      final ctx = (probe as LiveGatewayReady).context;
      final owner = await ctx.registerUser('t107-owner');
      final chats = ctx.chatsClient();

      final createdA = await chats.createGroup(
        authorization: owner.authorizationHeader,
        name: 'T107 Group A',
      );
      expect(createdA, isA<ChatsApiOk<VoiceChat>>(), reason: '$createdA');
      final groupA = (createdA as ChatsApiOk<VoiceChat>).data;
      expect(groupA.isGroup, isTrue);
      expect(groupA.creatorProfileId, owner.activeProfileId);
      await _expectOwnerOnlyGroup(
        chats,
        owner.authorizationHeader,
        groupA.id,
        owner.activeProfileId,
      );

      final createdB = await chats.createGroup(
        authorization: owner.authorizationHeader,
        name: 'T107 Group B',
      );
      expect(createdB, isA<ChatsApiOk<VoiceChat>>(), reason: '$createdB');
      final groupB = (createdB as ChatsApiOk<VoiceChat>).data;
      expect(groupB.isGroup, isTrue);
      expect(groupB.creatorProfileId, owner.activeProfileId);
      await _expectOwnerOnlyGroup(
        chats,
        owner.authorizationHeader,
        groupB.id,
        owner.activeProfileId,
      );

      final initialQuickAccess = await chats.listQuickAccess(
        authorization: owner.authorizationHeader,
      );
      expect(
        initialQuickAccess,
        isA<ChatsApiOk<QuickAccessListData>>(),
        reason: '$initialQuickAccess',
      );
      expect(
        (initialQuickAccess as ChatsApiOk<QuickAccessListData>).data.items,
        isEmpty,
      );

      final createdFolder = await chats.createFolder(
        authorization: owner.authorizationHeader,
        name: 'T107 custom folder',
      );
      expect(
        createdFolder,
        isA<ChatsApiOk<VoiceFolder>>(),
        reason: '$createdFolder',
      );
      final folder = (createdFolder as ChatsApiOk<VoiceFolder>).data;
      expect(folder.isSystem, isFalse);

      final foldersFromFreshClient = await ctx.chatsClient().listFolders(
        authorization: owner.authorizationHeader,
      );
      expect(
        foldersFromFreshClient,
        isA<ChatsApiOk<FolderListData>>(),
        reason: '$foldersFromFreshClient',
      );
      expect(
        (foldersFromFreshClient as ChatsApiOk<FolderListData>).data.folders.any(
          (item) => item.id == folder.id && item.name == folder.name,
        ),
        isTrue,
      );

      final addedToFolder = await chats.addChatToFolder(
        authorization: owner.authorizationHeader,
        folderId: folder.id,
        chatId: groupA.id,
      );
      expect(addedToFolder, isA<ChatsApiOk<void>>(), reason: '$addedToFolder');

      final refreshedOwner = await ctx.refreshSession(owner);
      final refreshedChats = ctx.chatsClient();
      final folderContents = await refreshedChats.listChats(
        authorization: refreshedOwner.authorizationHeader,
        folderId: folder.id,
      );
      expect(
        folderContents,
        isA<ChatsApiOk<ChatListData>>(),
        reason: '$folderContents',
      );
      expect(
        (folderContents as ChatsApiOk<ChatListData>).data.items.map(
          (item) => item.chatId,
        ),
        [groupA.id],
      );

      final removedFromFolder = await refreshedChats.removeChatFromFolder(
        authorization: refreshedOwner.authorizationHeader,
        folderId: folder.id,
        chatId: groupA.id,
      );
      expect(
        removedFromFolder,
        isA<ChatsApiOk<void>>(),
        reason: '$removedFromFolder',
      );
      final folderAfterRemove = await ctx.chatsClient().listChats(
        authorization: refreshedOwner.authorizationHeader,
        folderId: folder.id,
      );
      expect(
        folderAfterRemove,
        isA<ChatsApiOk<ChatListData>>(),
        reason: '$folderAfterRemove',
      );
      expect(
        (folderAfterRemove as ChatsApiOk<ChatListData>).data.items,
        isEmpty,
      );

      final addedA = await refreshedChats.addQuickAccess(
        authorization: refreshedOwner.authorizationHeader,
        chatId: groupA.id,
      );
      expect(addedA, isA<ChatsApiOk<void>>(), reason: '$addedA');
      final addedB = await refreshedChats.addQuickAccess(
        authorization: refreshedOwner.authorizationHeader,
        chatId: groupB.id,
      );
      expect(addedB, isA<ChatsApiOk<void>>(), reason: '$addedB');
      await _expectQuickAccess(
        refreshedChats,
        refreshedOwner.authorizationHeader,
        [groupA.id, groupB.id],
      );

      final reordered = await refreshedChats.reorderQuickAccess(
        authorization: refreshedOwner.authorizationHeader,
        chatIds: [groupB.id, groupA.id],
      );
      expect(reordered, isA<ChatsApiOk<void>>(), reason: '$reordered');
      await _expectQuickAccess(
        refreshedChats,
        refreshedOwner.authorizationHeader,
        [groupB.id, groupA.id],
      );

      final nextSession = await ctx.refreshSession(refreshedOwner);
      await _expectQuickAccess(
        ctx.chatsClient(),
        nextSession.authorizationHeader,
        [groupB.id, groupA.id],
      );

      final removedB = await ctx.chatsClient().removeQuickAccess(
        authorization: nextSession.authorizationHeader,
        chatId: groupB.id,
      );
      expect(removedB, isA<ChatsApiOk<void>>(), reason: '$removedB');
      await _expectQuickAccess(
        ctx.chatsClient(),
        nextSession.authorizationHeader,
        [groupA.id],
      );
    },
    skip: runLiveIntegration
        ? null
        : 'Opt in with --dart-define=VOICE_RUN_LIVE_INTEGRATION=true',
  );
}

Future<void> _expectOwnerOnlyGroup(
  VoiceChatsClient chats,
  String authorization,
  String chatId,
  String ownerProfileId,
) async {
  final result = await chats.listGroupMembers(
    authorization: authorization,
    chatId: chatId,
  );
  expect(result, isA<ChatsApiOk<MemberListData>>(), reason: '$result');
  final members = (result as ChatsApiOk<MemberListData>).data.members;
  expect(members, hasLength(1));
  expect(members.single.profileId, ownerProfileId);
  expect(members.single.isOwner, isTrue);
}

Future<void> _expectQuickAccess(
  VoiceChatsClient chats,
  String authorization,
  List<String> expectedChatIds,
) async {
  final result = await chats.listQuickAccess(authorization: authorization);
  expect(result, isA<ChatsApiOk<QuickAccessListData>>(), reason: '$result');
  final items = (result as ChatsApiOk<QuickAccessListData>).data.items;
  expect(items.map((item) => item.chatId), expectedChatIds);
  expect(
    items.map((item) => item.sortOrder),
    List<int>.generate(expectedChatIds.length, (index) => index),
  );
  expect(
    items.every((item) => item.chat != null && item.chat!.id == item.chatId),
    isTrue,
    reason: 'Quick Access must hydrate every persisted chat',
  );
}
