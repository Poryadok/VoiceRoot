import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/message_requests_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_badge.dart';

/// Virtual «Запросы» folder row — visible only when pending requests exist.
class MessageRequestsFolderRailButton extends ConsumerWidget {
  const MessageRequestsFolderRailButton({
    super.key,
    required this.selected,
    required this.onPressed,
  });

  static const keyId = Key('chat_rail_message_requests');

  final bool selected;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final summary = ref.watch(messageRequestsSummaryProvider).valueOrNull;
    if (summary == null || !summary.isVisible) {
      return const SizedBox.shrink();
    }
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    return Tooltip(
      message: l10n.chatInboxRequests,
      child: Material(
        key: keyId,
        color: selected ? voice.elevated : Colors.transparent,
        child: InkWell(
          onTap: onPressed,
          child: SizedBox(
            width: double.infinity,
            height: 40,
            child: Stack(
              alignment: Alignment.center,
              children: [
                Icon(
                  Icons.mark_email_unread_outlined,
                  size: 20,
                  color: selected ? voice.profileAccent : voice.textSecondary,
                ),
                if (summary.unreadCount > 0)
                  Positioned(
                    right: 8,
                    top: 6,
                    child: VoiceBadge(count: summary.unreadCount),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class MessageRequestsFolderDrawerTile extends ConsumerWidget {
  const MessageRequestsFolderDrawerTile({
    super.key,
    required this.selected,
    required this.onTap,
  });

  static const keyId = Key('mobile_drawer_message_requests');

  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final summary = ref.watch(messageRequestsSummaryProvider).valueOrNull;
    if (summary == null || !summary.isVisible) {
      return const SizedBox.shrink();
    }
    final l10n = AppLocalizations.of(context)!;
    return ListTile(
      key: keyId,
      leading: const Icon(Icons.mark_email_unread_outlined),
      title: Text(l10n.chatInboxRequests),
      selected: selected,
      trailing: summary.unreadCount > 0
          ? VoiceBadge(count: summary.unreadCount)
          : null,
      onTap: onTap,
    );
  }
}
