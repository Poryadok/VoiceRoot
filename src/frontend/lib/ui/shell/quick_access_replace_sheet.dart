import 'package:flutter/material.dart';

import '../../backend/chats_client.dart';
import '../../l10n/app_localizations.dart';

/// Replace picker when Quick Access is at 15/15 (navigation.md § Quick Access).
class QuickAccessReplaceSheet extends StatelessWidget {
  const QuickAccessReplaceSheet({super.key, required this.items});

  static const sheetKey = Key('quick_access_replace_sheet');

  final List<VoiceQuickAccessItem> items;

  static Future<String?> show(
    BuildContext context, {
    required List<VoiceQuickAccessItem> items,
  }) {
    return showModalBottomSheet<String>(
      context: context,
      builder: (_) => QuickAccessReplaceSheet(items: items),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return KeyedSubtree(
      key: sheetKey,
      child: SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
              child: Text(
                l10n.chatQuickAccessReplaceTitle,
                style: Theme.of(context).textTheme.titleMedium,
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
              child: Text(
                l10n.chatQuickAccessReplaceHint,
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ),
            for (final item in items)
              ListTile(
                key: Key('quick_access_replace_${item.chatId}'),
                leading: const Icon(Icons.star_outline),
                title: Text(item.chat?.name ?? item.chatId),
                onTap: () => Navigator.pop(context, item.chatId),
              ),
          ],
        ),
      ),
    );
  }
}
