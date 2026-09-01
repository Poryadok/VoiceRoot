import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/chats_client.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_navigation_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/folder_pin_providers.dart';
import 'quick_access_replace_sheet.dart';
/// Add [chatId] to Quick Access; opens replace picker at 15/15 limit.
Future<void> addChatToQuickAccess(
  BuildContext context,
  WidgetRef ref, {
  required String chatId,
}) async {
  final auth = ref.read(authorizationHeaderProvider);
  if (auth == null) return;

  final client = ref.read(voiceChatsClientProvider);
  QuickAccessListData qaList;
  try {
    qaList = await ref.read(quickAccessListProvider.future);
  } catch (_) {
    return;
  }

  if (qaList.items.any((item) => item.chatId == chatId)) return;

  Future<bool> addAfterRemove(String? removeChatId) async {
    if (removeChatId != null) {
      final removeResult = await client.removeQuickAccess(
        authorization: auth,
        chatId: removeChatId,
      );
      if (!context.mounted) return false;
      if (removeResult case ChatsApiFailure(:final message)) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(message)),
        );
        return false;
      }
    }

    final addResult = await client.addQuickAccess(
      authorization: auth,
      chatId: chatId,
    );
    if (!context.mounted) return false;
    switch (addResult) {
      case ChatsApiOk<void>():
        return true;
      case ChatsApiFailure(:final message):
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(message)),
        );
        return false;
    }
  }

  if (qaList.items.length >= kQuickAccessLimit) {
    if (!context.mounted) return;
    final replaceId = await QuickAccessReplaceSheet.show(
      context,
      items: qaList.items,
    );
    if (replaceId == null || !context.mounted) return;
    if (await addAfterRemove(replaceId)) {
      invalidateChatNavigationData(ref);
    }
    return;
  }

  final addResult = await client.addQuickAccess(
    authorization: auth,
    chatId: chatId,
  );
  if (!context.mounted) return;
  switch (addResult) {
    case ChatsApiOk<void>():
      invalidateChatNavigationData(ref);
    case ChatsApiFailure(:final errorCode):
      if (errorCode == 'failed_precondition') {
        final refreshed = await ref.read(quickAccessListProvider.future);
        if (!context.mounted) return;
        final replaceId = await QuickAccessReplaceSheet.show(
          context,
          items: refreshed.items,
        );
        if (replaceId == null || !context.mounted) return;
        if (await addAfterRemove(replaceId)) {
          invalidateChatNavigationData(ref);
        }
      } else if (addResult case ChatsApiFailure(:final message)) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(message)),
        );
      }
  }
}
