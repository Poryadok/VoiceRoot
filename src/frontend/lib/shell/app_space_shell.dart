import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../l10n/app_localizations.dart';
import '../state/chat_providers.dart';
import '../state/shell_providers.dart';
import '../state/space_providers.dart';
import '../theme/voice_colors.dart';
import '../ui/chat/chat_room_panel.dart';
import '../ui/space/space_rail.dart';
import '../ui/space/space_tree_panel.dart';

/// Authenticated Discord-style shell: space rail | channel tree | main content.
class AppSpaceShell extends ConsumerWidget {
  const AppSpaceShell({super.key});

  static const Key shellKey = Key('app_space_shell');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final voice = VoiceColors.of(context);
    final l10n = AppLocalizations.of(context)!;
    final selectedSpaceId = ref.watch(selectedSpaceIdProvider);
    final selectedChatId = ref.watch(selectedChatIdProvider);

    Widget listColumn() {
      if (selectedSpaceId == null) {
        return Center(
          child: Text(
            l10n.spaceSelectPrompt,
            style: TextStyle(color: voice.textSecondary),
          ),
        );
      }
      return SpaceTreePanel(
        spaceId: selectedSpaceId,
        selectedChatId: selectedChatId,
        onTextChatSelected: (chatId) {
          ref.read(shellNavigationProvider).selectChatInSpace(chatId);
        },
      );
    }

    Widget mainColumn() {
      if (selectedChatId == null) {
        return Center(
          child: Text(
            l10n.chatRoomSelectPrompt,
            style: TextStyle(color: voice.textSecondary),
          ),
        );
      }
      return ChatRoomPanel(chatId: selectedChatId);
    }

    return Row(
      key: shellKey,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(width: 72, child: SpaceRail()),
        VerticalDivider(width: 1, color: voice.borderDefault),
        Expanded(flex: 1, child: listColumn()),
        VerticalDivider(width: 1, color: voice.borderDefault),
        Expanded(flex: 2, child: mainColumn()),
      ],
    );
  }
}
