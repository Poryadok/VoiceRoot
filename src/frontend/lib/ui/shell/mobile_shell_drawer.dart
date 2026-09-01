import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/chat_navigation_providers.dart';
import '../../state/chat_providers.dart';
import '../../state/shell_providers.dart';
import '../../theme/voice_colors.dart';
import 'chat_rail_sections.dart';

/// Mobile drawer stub (R2-A04 incremental): folders, Quick Access, settings entry.
class MobileShellDrawer extends ConsumerWidget {
  const MobileShellDrawer({
    super.key,
    required this.onOpenSettings,
  });

  static const drawerKey = Key('mobile_shell_drawer');

  final VoidCallback onOpenSettings;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final foldersAsync = ref.watch(chatFoldersProvider);
    final qaAsync = ref.watch(quickAccessListProvider);
    final selectedFolderId = ref.watch(selectedChatFolderIdProvider);
    final shellNav = ref.read(shellNavigationProvider);

    return Drawer(
      key: drawerKey,
      child: SafeArea(
        child: ListView(
          padding: const EdgeInsets.symmetric(vertical: 8),
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
              child: Text(
                l10n.chatFoldersTitle,
                style: Theme.of(context).textTheme.titleSmall?.copyWith(
                      color: voice.textSecondary,
                    ),
              ),
            ),
            foldersAsync.when(
              data: (data) => Column(
                children: [
                  for (final folder in data.folders)
                    ListTile(
                      key: ChatRailFoldersSection.folderKey(folder.id),
                      leading: const Icon(Icons.folder_outlined),
                      title: Text(folder.name),
                      selected: selectedFolderId == folder.id,
                      onTap: () {
                        final current = ref.read(selectedChatFolderIdProvider);
                        ref.read(selectedChatFolderIdProvider.notifier).state =
                            current == folder.id ? null : folder.id;
                        ref.read(chatListControllerProvider.notifier).loadInitial();
                        Navigator.of(context).pop();
                      },
                    ),
                ],
              ),
              loading: () => const LinearProgressIndicator(minHeight: 2),
              error: (_, _) => ListTile(
                title: Text(l10n.backendUnavailable),
              ),
            ),
            const Divider(),
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
              child: Text(
                l10n.chatQuickAccessTitle,
                style: Theme.of(context).textTheme.titleSmall?.copyWith(
                      color: voice.textSecondary,
                    ),
              ),
            ),
            qaAsync.when(
              data: (data) {
                if (data.items.isEmpty) {
                  return ListTile(
                    title: Text(
                      l10n.chatQuickAccessEmpty,
                      style: TextStyle(color: voice.textSecondary),
                    ),
                  );
                }
                return Column(
                  children: [
                    for (final item in data.items)
                      ListTile(
                        key: ChatRailQuickAccessSection.itemKey(item.chatId),
                        leading: const Icon(Icons.star_outline),
                        title: Text(item.chat?.name ?? item.chatId),
                        onTap: () {
                          shellNav.selectChatFromHome(item.chatId);
                          Navigator.of(context).pop();
                        },
                      ),
                  ],
                );
              },
              loading: () => const LinearProgressIndicator(minHeight: 2),
              error: (_, _) => const SizedBox.shrink(),
            ),
            const Divider(),
            ListTile(
              key: const Key('mobile_drawer_settings'),
              leading: const Icon(Icons.settings_outlined),
              title: Text(l10n.settingsTooltip),
              onTap: () {
                Navigator.of(context).pop();
                onOpenSettings();
              },
            ),
          ],
        ),
      ),
    );
  }
}
