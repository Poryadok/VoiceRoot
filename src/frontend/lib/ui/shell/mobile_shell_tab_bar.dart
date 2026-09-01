import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/shell_providers.dart';
import '../../state/social_providers.dart';
import '../matchmaking/game_catalog_screen.dart';

/// Mobile bottom tab bar — normative §1.1 / §1.6a incremental step.
///
/// Full drawer IA (folders, QA, settings) remains deferred; this ships the
/// primary Chats / Social / Matchmaking tabs when no chat is open.
class MobileShellTabBar extends ConsumerWidget {
  const MobileShellTabBar({super.key});

  static const barKey = Key('mobile_shell_tab_bar');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final section = ref.watch(navigationSectionProvider);
    final shellNav = ref.read(shellNavigationProvider);
    final socialBadge =
        ref.watch(friendRequestsProvider).valueOrNull?.incoming.length ?? 0;

    var selectedIndex = 0;
    if (section == NavigationSection.social) {
      selectedIndex = 1;
    }

    return NavigationBar(
      key: barKey,
      selectedIndex: selectedIndex,
      onDestinationSelected: (index) {
        switch (index) {
          case 0:
            shellNav.exitSpace();
            shellNav.setNavigationSection(NavigationSection.chats);
          case 1:
            shellNav.exitSpace();
            shellNav.setNavigationSection(NavigationSection.social);
          case 2:
            shellNav.exitSpace();
            Navigator.of(context).push(
              MaterialPageRoute<void>(
                builder: (_) => const GameCatalogScreen(),
              ),
            );
        }
      },
      destinations: [
        NavigationDestination(
          icon: const Icon(Icons.chat_bubble_outline),
          selectedIcon: const Icon(Icons.chat_bubble),
          label: l10n.chatListTitle,
        ),
        NavigationDestination(
          icon: Badge(
            isLabelVisible: socialBadge > 0,
            label: Text(socialBadge > 9 ? '9+' : '$socialBadge'),
            child: const Icon(Icons.people_outline),
          ),
          selectedIcon: Badge(
            isLabelVisible: socialBadge > 0,
            label: Text(socialBadge > 9 ? '9+' : '$socialBadge'),
            child: const Icon(Icons.people),
          ),
          label: l10n.socialTabFriends,
        ),
        NavigationDestination(
          icon: const Icon(Icons.sports_esports_outlined),
          selectedIcon: const Icon(Icons.sports_esports),
          label: l10n.gameCatalogTitle,
        ),
      ],
    );
  }
}
