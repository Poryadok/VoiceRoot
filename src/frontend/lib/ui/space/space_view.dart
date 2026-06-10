import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/chat_providers.dart';
import '../../state/space_providers.dart';
import '../chat/chat_room_panel.dart';
import 'space_invites_sheet.dart';
import 'space_members_sheet.dart';
import 'space_tree_panel.dart';

/// Space shell: header + channel tree + optional open text chat.
class SpaceView extends ConsumerWidget {
  const SpaceView({super.key, required this.spaceId});

  static const Key viewKey = Key('space_view');

  final String spaceId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final spaceAsync = ref.watch(spaceProvider(spaceId));
    final selectedChatId = ref.watch(selectedChatIdProvider);
    final narrow = MediaQuery.sizeOf(context).width < 900;

    return Scaffold(
      key: viewKey,
      appBar: AppBar(
        title: spaceAsync.when(
          data: (space) => Text(space.name),
          loading: () => Text(l10n.spaceTreeTitle),
          error: (_, _) => Text(l10n.spaceTreeTitle),
        ),
        actions: [
          IconButton(
            key: const Key('space_members_action'),
            icon: const Icon(Icons.groups_outlined),
            tooltip: l10n.spaceMembersTooltip,
            onPressed: () => SpaceMembersSheet.show(context, spaceId: spaceId),
          ),
          IconButton(
            key: const Key('space_invites_action'),
            icon: const Icon(Icons.person_add_outlined),
            tooltip: l10n.spaceInvitesTooltip,
            onPressed: () => SpaceInvitesSheet.show(context, spaceId: spaceId),
          ),
        ],
      ),
      body: spaceAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(child: Text(e.toString())),
        data: (_) {
          if (narrow && selectedChatId != null) {
            return ChatRoomPanel(
              chatId: selectedChatId,
              onBack: () =>
                  ref.read(selectedChatIdProvider.notifier).state = null,
            );
          }
          return Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              SizedBox(
                width: narrow ? double.infinity : 260,
                child: SpaceTreePanel(
                  spaceId: spaceId,
                  selectedChatId: selectedChatId,
                  onTextChatSelected: (chatId) {
                    ref.read(selectedChatIdProvider.notifier).state = chatId;
                  },
                ),
              ),
              if (!narrow)
                Expanded(
                  child: selectedChatId == null
                      ? Center(child: Text(l10n.chatRoomSelectPrompt))
                      : ChatRoomPanel(chatId: selectedChatId),
                ),
            ],
          );
        },
      ),
    );
  }
}
