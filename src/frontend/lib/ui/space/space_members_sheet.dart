import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/roles_client.dart';
import '../../backend/spaces_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/space_providers.dart';
import '../../state/social_providers.dart';
import '../core/voice_avatar.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_state_panel.dart';

/// Bottom sheet: space members with role badges; Owner/Admin can kick and assign roles.
class SpaceMembersSheet extends ConsumerWidget {
  const SpaceMembersSheet({super.key, required this.spaceId});

  static const Key sheetKey = Key('space_members_sheet');

  static Key kickMemberKey(String profileId) => Key('kick_member_$profileId');

  static Key assignRoleKey(String profileId) => Key('assign_role_$profileId');

  final String spaceId;

  static Future<void> show(BuildContext context, {required String spaceId}) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: UncontrolledProviderScope(
        container: container,
        child: SpaceMembersSheet(spaceId: spaceId),
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final membersAsync = ref.watch(spaceMembersProvider(spaceId));
    final activeId = ref.watch(spaceViewerProfileIdProvider);

    return SafeArea(
      child: Padding(
        key: sheetKey,
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              l10n.spaceMembersTitle,
              style: theme.textTheme.titleMedium,
            ),
            const SizedBox(height: 4),
            Text(
              l10n.spaceMembersSubtitle,
              style: theme.textTheme.bodySmall,
            ),
            const SizedBox(height: 12),
            Expanded(
              child: membersAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (error, _) => VoiceStatePanel(
                  title: l10n.spaceMembersLoadError,
                  message: '$error',
                  icon: Icons.cloud_off_outlined,
                  actionLabel: l10n.commonRetry,
                  onAction: () => ref.invalidate(spaceMembersProvider(spaceId)),
                ),
                data: (members) {
                  final canManage = viewerCanManageSpaceMembers(
                    members,
                    activeId,
                  );
                  return ListView.builder(
                    itemCount: members.length,
                    itemBuilder: (context, index) {
                      final member = members[index];
                      return _MemberTile(
                        spaceId: spaceId,
                        member: member,
                        isSelf: member.profileId == activeId,
                        canKick: canManage && !member.isOwner,
                        canAssignRole: canManage && !member.isOwner,
                        kickKey: kickMemberKey(member.profileId),
                        assignRoleKey: assignRoleKey(member.profileId),
                      );
                    },
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _MemberTile extends ConsumerWidget {
  const _MemberTile({
    required this.spaceId,
    required this.member,
    required this.isSelf,
    required this.canKick,
    required this.canAssignRole,
    required this.kickKey,
    required this.assignRoleKey,
  });

  final String spaceId;
  final SpaceMemberRosterEntry member;
  final bool isSelf;
  final bool canKick;
  final bool canAssignRole;
  final Key kickKey;
  final Key assignRoleKey;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final profileAsync = ref.watch(profileProvider(member.profileId));
    final profile = profileAsync.valueOrNull;
    final label =
        profile?.displayName ?? profile?.handle ?? member.profileId;

    return ListTile(
      leading: VoiceAvatar(
        imageUrl: profile?.avatarUrl,
        label: label,
        radius: 20,
      ),
      title: Text(isSelf ? l10n.spaceMemberYou(label) : label),
      subtitle: Wrap(
        spacing: 6,
        runSpacing: 4,
        children: [
          for (final roleName in member.roleNames)
            Chip(
              label: Text(roleName),
              visualDensity: VisualDensity.compact,
              materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
              padding: EdgeInsets.zero,
            ),
        ],
      ),
      trailing: (canKick || canAssignRole)
          ? Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                if (canAssignRole)
                  IconButton(
                    key: assignRoleKey,
                    tooltip: l10n.spaceAssignRole,
                    icon: const Icon(Icons.badge_outlined),
                    onPressed: () => _pickRole(context, ref),
                  ),
                if (canKick)
                  IconButton(
                    key: kickKey,
                    tooltip: l10n.spaceKick,
                    icon: const Icon(Icons.person_remove_outlined),
                    onPressed: () => _confirmKick(context, ref, label),
                  ),
              ],
            )
          : null,
    );
  }

  Future<void> _confirmKick(
    BuildContext context,
    WidgetRef ref,
    String label,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        final dialogL10n = AppLocalizations.of(ctx)!;
        return AlertDialog(
          title: Text(dialogL10n.spaceKickConfirmTitle),
          content: Text(dialogL10n.spaceKickConfirmMessage(label)),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(false),
              child: Text(dialogL10n.commonCancel),
            ),
            FilledButton(
              onPressed: () => Navigator.of(ctx).pop(true),
              child: Text(dialogL10n.spaceKick),
            ),
          ],
        );
      },
    );
    if (confirmed != true || !context.mounted) return;
    final err = await ref.read(spaceMemberActionsProvider).kickMember(
      spaceId: spaceId,
      profileId: member.profileId,
    );
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.spaceKickError(err))),
      );
    }
  }

  Future<void> _pickRole(BuildContext context, WidgetRef ref) async {
    final l10n = AppLocalizations.of(context)!;
    final roles = await ref.read(spaceRolesProvider(spaceId).future);
    final assignable = roles
        .where((role) => role.name != kSpaceRoleOwner)
        .toList(growable: false);
    if (!context.mounted) return;
    if (assignable.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.spaceAssignRoleEmpty)),
      );
      return;
    }

    final picked = await showDialog<SpaceRole>(
      context: context,
      builder: (ctx) {
        final dialogL10n = AppLocalizations.of(ctx)!;
        return SimpleDialog(
          title: Text(dialogL10n.spaceAssignRoleTitle),
          children: [
            for (final role in assignable)
              SimpleDialogOption(
                onPressed: () => Navigator.of(ctx).pop(role),
                child: Text(role.name),
              ),
          ],
        );
      },
    );
    if (picked == null || !context.mounted) return;

    final err = await ref.read(spaceMemberActionsProvider).assignRole(
      spaceId: spaceId,
      profileId: member.profileId,
      roleId: picked.id,
    );
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.spaceAssignRoleError(err))),
      );
    }
  }
}
