import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/chat_navigation_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/folder_pin_providers.dart';
import '../api_error_messages.dart';
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
  } catch (error) {
    if (error is StateError && error.message == 'not_authenticated') return;
    if (!context.mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(commonActionErrorMessage(AppLocalizations.of(context)!)),
      ),
    );
    return;
  }

  if (qaList.items.any((item) => item.chatId == chatId)) return;

  Future<bool> addAfterReplace(String replaceChatId) async {
    final addResult = await client.addQuickAccess(
      authorization: auth,
      chatId: chatId,
      replaceChatId: replaceChatId,
    );
    if (!context.mounted) return false;
    switch (addResult) {
      case ChatsApiOk<void>():
        return true;
      case ChatsApiFailure(:final statusCode):
        ref.invalidate(quickAccessListProvider);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              commonActionErrorMessage(
                AppLocalizations.of(context)!,
                statusCode: statusCode,
              ),
            ),
          ),
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
    if (await addAfterReplace(replaceId)) {
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
    case ChatsApiFailure(:final errorCode, :final statusCode):
      if (errorCode == 'failed_precondition') {
        QuickAccessListData refreshed;
        try {
          ref.invalidate(quickAccessListProvider);
          refreshed = await ref.read(quickAccessListProvider.future);
        } catch (_) {
          if (!context.mounted) return;
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text(
                commonActionErrorMessage(
                  AppLocalizations.of(context)!,
                  statusCode: statusCode,
                ),
              ),
            ),
          );
          return;
        }
        if (!context.mounted) return;
        final replaceId = await QuickAccessReplaceSheet.show(
          context,
          items: refreshed.items,
        );
        if (replaceId == null || !context.mounted) return;
        if (await addAfterReplace(replaceId)) {
          invalidateChatNavigationData(ref);
        }
      } else {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              commonActionErrorMessage(
                AppLocalizations.of(context)!,
                statusCode: statusCode,
              ),
            ),
          ),
        );
      }
  }
}
