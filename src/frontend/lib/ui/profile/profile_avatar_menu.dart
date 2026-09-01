import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/users_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/social_providers.dart';
import '../../state/subscription_providers.dart';
import '../core/voice_avatar.dart';
import 'create_profile_sheet.dart';

/// Profile avatar context menu per [screen-controls.md] §1.1a.
class ProfileAvatarMenuButton extends ConsumerWidget {
  const ProfileAvatarMenuButton({
    super.key,
    required this.profile,
    this.menuKey = const Key('profile_avatar_menu'),
    this.avatarRadius = 16,
    this.tooltip,
    this.offset,
  });

  static const railKey = Key('shell_rail_profile');

  final VoiceProfile? profile;
  final Key menuKey;
  final double avatarRadius;
  final String? tooltip;
  final Offset? offset;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final label = profile?.displayName ?? '?';

    return PopupMenuButton<String>(
      key: menuKey,
      tooltip: tooltip ?? l10n.profileEditTooltip,
      offset: offset ?? const Offset(56, 0),
      onSelected: (value) => _handleSelection(context, ref, value),
      itemBuilder: (context) => buildProfileAvatarMenuItems(context, ref),
      child: VoiceAvatar(
        imageUrl: profile?.avatarUrl,
        label: label,
        radius: avatarRadius,
      ),
    );
  }
}

/// Opens the §1.1a menu anchored below [anchorContext] (mobile tap).
Future<void> showProfileAvatarMenu({
  required BuildContext context,
  required WidgetRef ref,
  required BuildContext anchorContext,
}) async {
  final box = anchorContext.findRenderObject() as RenderBox?;
  if (box == null || !box.hasSize) return;
  final offset = box.localToGlobal(box.size.bottomLeft(Offset.zero));
  final selected = await showMenu<String>(
    context: context,
    position: RelativeRect.fromLTRB(
      offset.dx,
      offset.dy,
      offset.dx + 1,
      offset.dy + 1,
    ),
    items: buildProfileAvatarMenuItems(context, ref),
  );
  if (selected != null && context.mounted) {
    await _handleSelection(context, ref, selected);
  }
}

List<PopupMenuEntry<String>> buildProfileAvatarMenuItems(
  BuildContext context,
  WidgetRef ref,
) {
  final l10n = AppLocalizations.of(context)!;
  final auth = ref.watch(authControllerProvider);
  final activeId = auth.activeProfileId;
  final profiles = ref.watch(myProfilesProvider).valueOrNull ?? const <VoiceProfile>[];
  final tier = ref.watch(subscriptionTierProvider);
  final maxProfiles = tier == 'premium' ? 5 : 2;
  final switching = ref.watch(profileSwitchInProgressProvider);
  final items = <PopupMenuEntry<String>>[];

  if (profiles.length > 1 && activeId != null) {
    for (final profile in profiles) {
      final isActive = profile.id == activeId;
      items.add(
        PopupMenuItem<String>(
          value: 'profile:${profile.id}',
          enabled: !switching && !isActive,
          child: Row(
            children: [
              if (isActive)
                const Icon(Icons.check, size: 18)
              else
                const SizedBox(width: 18),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  profile.isPrimary
                      ? '${profile.displayName} (${l10n.downgradeProfilePrimary})'
                      : profile.displayName,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
        ),
      );
    }
    items.add(const PopupMenuDivider());
  }

  if (profiles.length < maxProfiles) {
    items.add(
      PopupMenuItem<String>(
        value: 'create',
        child: Text(l10n.createProfileSubmit),
      ),
    );
  }

  items.add(
    PopupMenuItem<String>(
      value: 'presence_header',
      enabled: false,
      child: Text(
        l10n.profileMenuPresence,
        style: TextStyle(color: Theme.of(context).disabledColor),
      ),
    ),
  );
  for (final entry in _presenceMenuEntries(l10n)) {
    items.add(
      PopupMenuItem<String>(
        value: 'presence:${entry.$1}',
        child: Padding(
          padding: const EdgeInsets.only(left: 16),
          child: Text(entry.$2),
        ),
      ),
    );
  }

  items.add(const PopupMenuDivider());
  items.add(
    PopupMenuItem<String>(
      value: 'archive',
      child: Text(l10n.chatListArchive),
    ),
  );

  return items;
}

List<(String, String)> _presenceMenuEntries(AppLocalizations l10n) => [
  ('online', l10n.socialPresenceOnline),
  ('idle', l10n.socialPresenceIdle),
  ('dnd', l10n.socialPresenceDnd),
  ('invisible', l10n.socialPresenceInvisible),
];

Future<void> _handleSelection(
  BuildContext context,
  WidgetRef ref,
  String value,
) async {
  final l10n = AppLocalizations.of(context)!;
  if (value.startsWith('profile:')) {
    final profileId = value.substring('profile:'.length);
    await _switchProfile(ref, profileId);
    return;
  }
  if (value == 'create') {
    await showCreateProfileSheet(context);
    return;
  }
  if (value == 'presence_header') {
    return;
  }
  if (value.startsWith('presence:')) {
    final status = value.substring('presence:'.length);
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    final result = await ref.read(voiceUsersClientProvider).updatePresence(
      authorization: auth,
      status: status,
    );
    if (!context.mounted) return;
    if (result is UsersApiFailure) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(result.message)),
      );
    }
    return;
  }
  if (value == 'archive') {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(l10n.profileMenuArchiveUnavailable)),
    );
  }
}

Future<void> _switchProfile(WidgetRef ref, String profileId) async {
  ref.read(profileSwitchInProgressProvider.notifier).state = true;
  try {
    final err = await ref
        .read(authControllerProvider.notifier)
        .switchActiveProfile(profileId);
    if (err == null) {
      ref.invalidate(activeProfileProvider);
      ref.invalidate(profileProvider(profileId));
      ref.invalidate(myProfilesProvider);
      ref.invalidate(subscriptionProvider);
    }
  } finally {
    ref.read(profileSwitchInProgressProvider.notifier).state = false;
  }
}
