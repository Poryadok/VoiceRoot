import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/shell_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';
import '../../theme/voice_metrics.dart';
import '../core/voice_avatar.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_shell_icons.dart';
import '../matchmaking/game_catalog_screen.dart';
import '../profile/create_profile_sheet.dart';
import '../profile/profile_edit_sheet.dart';

enum ShellRailDestination { chats, friends, matchmaking, settings }

/// Penpot `Screen/Shell/Desktop` icon rail (`layout.railWidth` = 56).
class DesktopShellRail extends ConsumerWidget {
  const DesktopShellRail({
    super.key,
    required this.onOpenSettings,
  });

  static const railKey = Key('desktop_shell_rail');

  final VoidCallback onOpenSettings;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final section = ref.watch(navigationSectionProvider);
    final shellNav = ref.read(shellNavigationProvider);
    final socialBadge =
        ref.watch(friendRequestsProvider).valueOrNull?.incoming.length ?? 0;
    final activeProfile = ref.watch(activeProfileProvider).valueOrNull;
    final railWidth = context.voiceMetrics.layoutSize(
      'railWidth',
      fallback: VoiceLayout.desktopRailWidth,
    );

    ShellRailDestination? selected;
    if (section == NavigationSection.social) {
      selected = ShellRailDestination.friends;
    } else {
      selected = ShellRailDestination.chats;
    }

    void goChats() {
      shellNav.exitSpace();
      shellNav.setNavigationSection(NavigationSection.chats);
    }

    void goFriends() {
      shellNav.exitSpace();
      shellNav.setNavigationSection(NavigationSection.social);
    }

    void goMatchmaking() {
      shellNav.exitSpace();
      Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => const GameCatalogScreen(),
        ),
      );
    }

    return ColoredBox(
      key: railKey,
      color: voice.muted,
      child: SizedBox(
        width: railWidth,
        child: Column(
          children: [
            const SizedBox(height: 8),
            _RailIconButton(
              key: const Key('shell_rail_chats'),
              tooltip: l10n.chatListTitle,
              icon: VoiceShellRailIcon.chats,
              selected: selected == ShellRailDestination.chats,
              onPressed: goChats,
            ),
            _RailIconButton(
              key: const Key('shell_rail_friends'),
              tooltip: l10n.socialTabFriends,
              icon: VoiceShellRailIcon.friends,
              selected: selected == ShellRailDestination.friends,
              badgeCount: socialBadge,
              onPressed: goFriends,
            ),
            _RailIconButton(
              key: const Key('shell_rail_matchmaking'),
              tooltip: l10n.gameCatalogTitle,
              icon: VoiceShellRailIcon.matchmaking,
              selected: false,
              onPressed: goMatchmaking,
            ),
            const Spacer(),
            _RailIconButton(
              key: const Key('shell_rail_settings'),
              tooltip: l10n.settingsTooltip,
              icon: VoiceShellRailIcon.settings,
              selected: false,
              onPressed: onOpenSettings,
            ),
            _ProfileRailButton(
              profile: activeProfile,
              tooltip: l10n.profileEditTooltip,
              onOpenEdit: activeProfile == null
                  ? null
                  : () => showVoiceBottomSheet<void>(
                        context: context,
                        child: ProfileEditSheet(profile: activeProfile),
                      ),
              onCreateProfile: () => showCreateProfileSheet(context),
            ),
            const SizedBox(height: 8),
          ],
        ),
      ),
    );
  }
}

class _RailIconButton extends StatelessWidget {
  const _RailIconButton({
    super.key,
    required this.tooltip,
    required this.icon,
    required this.selected,
    required this.onPressed,
    this.badgeCount = 0,
  });

  final String tooltip;
  final VoiceShellRailIcon icon;
  final bool selected;
  final VoidCallback onPressed;
  final int badgeCount;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final barWidth = context.voiceMetrics.spacing('4', fallback: 4);
    return Stack(
      clipBehavior: Clip.none,
      children: [
        Tooltip(
          message: tooltip,
          child: Material(
            color: selected ? voice.elevated : Colors.transparent,
            child: InkWell(
              onTap: onPressed,
              child: Semantics(
                button: true,
                label: tooltip,
                selected: selected,
                child: SizedBox(
                  width: double.infinity,
                  height: 48,
                  child: Row(
                    children: [
                      if (selected)
                        Container(width: barWidth, color: voice.profileAccent),
                      Expanded(
                        child: Center(
                          child: VoiceShellRailIconWidget(
                            icon: icon,
                            selected: selected,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
        if (badgeCount > 0)
          Positioned(
            right: 8,
            top: 6,
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

class _ProfileRailButton extends StatelessWidget {
  const _ProfileRailButton({
    required this.profile,
    required this.tooltip,
    required this.onOpenEdit,
    required this.onCreateProfile,
  });

  final VoiceProfile? profile;
  final String tooltip;
  final VoidCallback? onOpenEdit;
  final VoidCallback onCreateProfile;

  @override
  Widget build(BuildContext context) {
    final label = profile?.displayName ?? '?';
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: PopupMenuButton<String>(
        key: const Key('shell_rail_profile'),
        tooltip: tooltip,
        offset: const Offset(56, 0),
        itemBuilder: (context) => [
          if (onOpenEdit != null)
            PopupMenuItem(
              value: 'edit',
              child: Text(AppLocalizations.of(context)!.profileEditTooltip),
            ),
          PopupMenuItem(
            value: 'create',
            child: Text(AppLocalizations.of(context)!.createProfileSubmit),
          ),
        ],
        onSelected: (value) {
          switch (value) {
            case 'edit':
              onOpenEdit?.call();
            case 'create':
              onCreateProfile();
          }
        },
        child: VoiceAvatar(
          imageUrl: profile?.avatarUrl,
          label: label,
          radius: 16,
        ),
      ),
    );
  }
}
