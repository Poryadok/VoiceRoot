import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/shell_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../social/social_panel.dart';
import '../space/space_rail.dart';
import 'chat_list_body.dart';

/// Left navigation column: chats, spaces, and inline social.
class NavigationPanel extends ConsumerWidget {
  const NavigationPanel({super.key, required this.collapsed});

  static const Key panelKey = Key('navigation_panel');
  static const Key homeButtonKey = Key('navigation_home');
  static const Key socialToggleKey = Key('navigation_social_toggle');
  static const Key chatsToggleKey = Key('navigation_chats_toggle');

  final bool collapsed;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (collapsed) {
      return _CollapsedNavigation(panelKey: panelKey);
    }
    return const _ExpandedNavigation(key: panelKey);
  }
}

class _CollapsedNavigation extends ConsumerWidget {
  const _CollapsedNavigation({required this.panelKey});

  final Key panelKey;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final shellNav = ref.read(shellNavigationProvider);
    final socialBadge =
        ref.watch(friendRequestsProvider).valueOrNull?.incoming.length ?? 0;
    final socialOpen =
        ref.watch(navigationSectionProvider) == NavigationSection.social;

    return ColoredBox(
      key: panelKey,
      color: voice.muted,
      child: Column(
        children: [
          const SizedBox(height: 8),
          IconButton(
            key: NavigationPanel.homeButtonKey,
            tooltip: l10n.chatListTitle,
            onPressed: shellNav.exitSpace,
            icon: Icon(Icons.home_outlined, color: voice.textSecondary),
          ),
          Expanded(
            child: SpaceRail(
              onSpaceSelected: (id) => shellNav.selectSpace(id),
            ),
          ),
          _SocialToggleButton(
            buttonKey: NavigationPanel.socialToggleKey,
            active: socialOpen,
            badgeCount: socialBadge,
            onPressed: () {
              shellNav.exitSpace();
              shellNav.setNavigationSection(NavigationSection.social);
            },
          ),
          const SizedBox(height: 8),
        ],
      ),
    );
  }
}

class _ExpandedNavigation extends ConsumerStatefulWidget {
  const _ExpandedNavigation({super.key});

  @override
  ConsumerState<_ExpandedNavigation> createState() =>
      _ExpandedNavigationState();
}

class _ExpandedNavigationState extends ConsumerState<_ExpandedNavigation> {
  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final section = ref.watch(navigationSectionProvider);
    final shellNav = ref.read(shellNavigationProvider);
    final socialBadge =
        ref.watch(friendRequestsProvider).valueOrNull?.incoming.length ?? 0;

    return ColoredBox(
      color: voice.muted,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(8, 8, 8, 4),
            child: SegmentedButton<NavigationSection>(
              segments: [
                ButtonSegment(
                  value: NavigationSection.chats,
                  label: Text(l10n.chatListTitle),
                  icon: const Icon(Icons.chat_bubble_outline, size: 18),
                ),
                ButtonSegment(
                  value: NavigationSection.social,
                  label: Text(l10n.socialTabFriends),
                  icon: Badge(
                    isLabelVisible: socialBadge > 0,
                    label: Text(
                      socialBadge > 9 ? '9+' : '$socialBadge',
                      style: const TextStyle(fontSize: 10),
                    ),
                    child: const Icon(Icons.people_outline, size: 18),
                  ),
                ),
              ],
              selected: {section},
              onSelectionChanged: (next) =>
                  shellNav.setNavigationSection(next.single),
            ),
          ),
          Expanded(
            child: section == NavigationSection.chats
                ? const ChatListBody(showHeader: true)
                : const SocialPanel(),
          ),
        ],
      ),
    );
  }
}

class _SocialToggleButton extends StatelessWidget {
  const _SocialToggleButton({
    required this.buttonKey,
    required this.onPressed,
    this.active = false,
    this.badgeCount = 0,
  });

  final Key buttonKey;
  final VoidCallback onPressed;
  final bool active;
  final int badgeCount;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final iconColor = active ? voice.profileAccent : voice.textSecondary;
    return Stack(
      clipBehavior: Clip.none,
      children: [
        IconButton(
          key: buttonKey,
          onPressed: onPressed,
          style: active
              ? IconButton.styleFrom(backgroundColor: voice.elevated)
              : null,
          icon: Icon(Icons.people_outline, color: iconColor),
        ),
        if (badgeCount > 0)
          Positioned(
            right: 4,
            top: 4,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 1),
              decoration: BoxDecoration(
                color: voice.profileAccent,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Text(
                badgeCount > 9 ? '9+' : '$badgeCount',
                style: TextStyle(
                  fontSize: 10,
                  color: Theme.of(context).colorScheme.onPrimary,
                ),
              ),
            ),
          ),
      ],
    );
  }
}
