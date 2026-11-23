import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/chats_client.dart';
import 'auth_providers.dart';
import 'chat_providers.dart';

/// Resolves management access only after locating the active profile in the
/// complete, paginated member list for a standalone group chat. Owner and
/// admins may edit group settings (docs/features/text-chat.md §Group Rights,
/// docs/design/screen-controls.md §10.1c entry #21 "GRP owner/admin"); members
/// never do. Space chats keep their settings in the Space/Role Service and are
/// intentionally excluded here.
final groupSettingsAuthorityProvider = FutureProvider.family<bool, String>((
  ref,
  chatId,
) async {
  final authorization = ref.watch(authorizationHeaderProvider);
  final authState = ref.watch(authControllerProvider);
  final profileId = authState.activeProfileId;
  final accountId = authState.session?.accountId;
  if (authorization == null ||
      profileId == null ||
      profileId.isEmpty ||
      accountId == null ||
      accountId.isEmpty) {
    return false;
  }

  final chatList = ref.watch(chatListControllerProvider);
  if (chatList.isLoading || chatList.profileId != profileId) return false;
  final chat = chatList.items
      .where((item) => item.chat.id == chatId)
      .map((item) => item.chat)
      .firstOrNull;
  final isSpaceChat = chat?.spaceId != null && chat!.spaceId!.isNotEmpty;
  if (chat == null || !chat.isGroup || isSpaceChat) return false;

  final seenCursors = <String>{};
  String? cursor;
  while (true) {
    if (!_isCurrentAuthority(ref, authorization, accountId, profileId)) {
      return false;
    }
    final result = await ref
        .read(voiceChatsClientProvider)
        .listGroupMembers(
          authorization: authorization,
          chatId: chatId,
          cursor: cursor,
        );
    if (!_isCurrentAuthority(ref, authorization, accountId, profileId)) {
      return false;
    }
    switch (result) {
      case ChatsApiFailure():
        return false;
      case ChatsApiOk(:final data):
        final member = data.members
            .where((candidate) => candidate.profileId == profileId)
            .firstOrNull;
        if (member != null) {
          return member.role == kChatRoleOwner || member.role == kChatRoleAdmin;
        }
        final next = data.nextCursor;
        if (next == null || next.isEmpty || !seenCursors.add(next)) {
          return false;
        }
        cursor = next;
    }
  }
});

bool _isCurrentAuthority(
  Ref ref,
  String authorization,
  String accountId,
  String profileId,
) {
  return ref.read(authorizationHeaderProvider) == authorization &&
      ref.read(authControllerProvider).session?.accountId == accountId &&
      ref.read(authControllerProvider).activeProfileId == profileId;
}
