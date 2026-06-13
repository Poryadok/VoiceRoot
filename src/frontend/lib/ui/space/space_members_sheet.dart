import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/roles_client.dart';
import '../../backend/space_permissions.dart';
import '../../backend/spaces_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/space_providers.dart';
import '../../state/social_providers.dart';
import '../core/voice_avatar.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_state_panel.dart';

/// Space members list (embedded in side panel or bottom sheet).
class SpaceMembersContent extends ConsumerWidget {
  const SpaceMembersContent({
    super.key,
    required this.spaceId,
    this.showHeader = true,
  });

  final String spaceId;
  final bool showHeader;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final membersAsync = ref.watch(spaceMembersProvider(spaceId));
    final activeId = ref.watch(spaceViewerProfileIdProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (showHeader) ...[
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
        ],
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
              final canAssignRole = ref
                      .watch(
                        spacePermissionProvider((
                          spaceId: spaceId,
                          permission: SpacePermissions.memberAssignRoles,
                          chatId: null,
                          voiceRoomId: null,
                        )),
                      )
                      .valueOrNull ??
                  false;
              final canKick = ref
                      .watch(
                        spacePermissionProvider((
                          spaceId: spaceId,
                          permission: 'MEMBER_KICK',
                          chatId: null,
                          voiceRoomId: null,
                        )),
                      )
                      .valueOrNull ??
                  false;
              final canBan = ref
                      .watch(
                        spacePermissionProvider((
                          spaceId: spaceId,
                          permission: 'MEMBER_BAN',
                          chatId: null,
                          voiceRoomId: null,
                        )),
                      )
                      .valueOrNull ??
                  false;
              final canTimeout = ref
                      .watch(
                        spacePermissionProvider((
                          spaceId: spaceId,
                          permission: 'MODERATION_TIMEOUT_MEMBERS',
                          chatId: null,
                          voiceRoomId: null,
                        )),
                      )
                      .valueOrNull ??
                  false;
              return ListView.builder(
                itemCount: members.length,
                itemBuilder: (context, index) {
                  final member = members[index];
                  return _MemberTile(
                    spaceId: spaceId,
                    member: member,
                    isSelf: member.profileId == activeId,
                    canKick: canKick && !member.isOwner,
                    canBan: canBan && !member.isOwner,
                    canTimeout: canTimeout && !member.isOwner,
                    canAssignRole: canAssignRole && !member.isOwner,
                    kickKey: SpaceMembersSheet.kickMemberKey(member.profileId),
                    banKey: SpaceMembersSheet.banMemberKey(member.profileId),
                    timeoutKey:
                        SpaceMembersSheet.timeoutMemberKey(member.profileId),
                    assignRoleKey:
                        SpaceMembersSheet.assignRoleKey(member.profileId),
                  );
                },
              );
            },
          ),
        ),
      ],
    );
  }
}

/// Bottom sheet: space members with role badges; Owner/Admin can kick and assign roles.
class SpaceMembersSheet extends ConsumerWidget {
  const SpaceMembersSheet({super.key, required this.spaceId});

  static const Key sheetKey = Key('space_members_sheet');

  static Key kickMemberKey(String profileId) => Key('kick_member_$profileId');

  static Key banMemberKey(String profileId) => Key('ban_member_$profileId');

  static Key timeoutMemberKey(String profileId) =>
      Key('timeout_member_$profileId');

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
    return SafeArea(
      child: Padding(
        key: sheetKey,
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: SpaceMembersContent(spaceId: spaceId),
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
    required this.canBan,
    required this.canTimeout,
    required this.canAssignRole,
    required this.kickKey,
    required this.banKey,
    required this.timeoutKey,
    required this.assignRoleKey,
  });

  final String spaceId;
  final SpaceMemberRosterEntry member;
  final bool isSelf;
  final bool canKick;
  final bool canBan;
  final bool canTimeout;
  final bool canAssignRole;
  final Key kickKey;
  final Key banKey;
  final Key timeoutKey;
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
      trailing: (canKick || canBan || canTimeout || canAssignRole)
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
                if (canTimeout)
                  IconButton(
                    key: timeoutKey,
                    tooltip: l10n.spaceTimeout,
                    icon: const Icon(Icons.timer_off_outlined),
                    onPressed: () => _confirmTimeout(context, ref, label),
                  ),
                if (canBan)
                  IconButton(
                    key: banKey,
                    tooltip: l10n.spaceBan,
                    icon: const Icon(Icons.block_outlined),
                    onPressed: () => _confirmBan(context, ref, label),
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

  Future<void> _confirmBan(
    BuildContext context,
    WidgetRef ref,
    String label,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final profile = ref.read(profileProvider(member.profileId)).valueOrNull;
    final accountId = profile?.accountId;
    if (accountId == null || accountId.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.spaceBanError('missing account'))),
      );
      return;
    }
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        final dialogL10n = AppLocalizations.of(ctx)!;
        return AlertDialog(
          title: Text(dialogL10n.spaceBanConfirmTitle),
          content: Text(dialogL10n.spaceBanConfirmMessage(label)),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(false),
              child: Text(dialogL10n.commonCancel),
            ),
            FilledButton(
              onPressed: () => Navigator.of(ctx).pop(true),
              child: Text(dialogL10n.spaceBan),
            ),
          ],
        );
      },
    );
    if (confirmed != true || !context.mounted) return;
    final err = await ref.read(spaceMemberActionsProvider).banMember(
      spaceId: spaceId,
      accountId: accountId,
      profileId: member.profileId,
    );
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.spaceBanError(err))),
      );
    }
  }

  Future<void> _confirmTimeout(
    BuildContext context,
    WidgetRef ref,
    String label,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    const durationSeconds = 600;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        final dialogL10n = AppLocalizations.of(ctx)!;
        return AlertDialog(
          title: Text(dialogL10n.spaceTimeoutConfirmTitle),
          content: Text(dialogL10n.spaceTimeoutConfirmMessage(label)),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(false),
              child: Text(dialogL10n.commonCancel),
            ),
            FilledButton(
              onPressed: () => Navigator.of(ctx).pop(true),
              child: Text(dialogL10n.spaceTimeout),
            ),
          ],
        );
      },
    );
    if (confirmed != true || !context.mounted) return;
    final err = await ref.read(spaceMemberActionsProvider).timeoutMember(
      spaceId: spaceId,
      profileId: member.profileId,
      durationSeconds: durationSeconds,
    );
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(l10n.spaceTimeoutError(err))),
      );
    }
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

    final memberRoles = await ref.read(voiceRolesClientProvider).getMemberRoles(
      authorization: ref.read(authorizationHeaderProvider)!,
      spaceId: spaceId,
      profileId: member.profileId,
    );
    final assignedIds = switch (memberRoles) {
      RolesApiOk(:final data) => data.map((r) => r.id).toSet(),
      RolesApiFailure() => <String>{},
    };

    final action = await showDialog<({String roleId, bool revoke})>(
      context: context,
      builder: (ctx) {
        final dialogL10n = AppLocalizations.of(ctx)!;
        return SimpleDialog(
          title: Text(dialogL10n.spaceAssignRoleTitle),
          children: [
            for (final role in assignable)
              if (!assignedIds.contains(role.id))
                SimpleDialogOption(
                  onPressed: () => Navigator.of(ctx).pop((roleId: role.id, revoke: false)),
                  child: Text('${dialogL10n.spaceAssignRole}: ${role.name}'),
                ),
            for (final role in assignable)
              if (assignedIds.contains(role.id) && role.name != kSpaceRoleOwner)
                SimpleDialogOption(
                  onPressed: () => Navigator.of(ctx).pop((roleId: role.id, revoke: true)),
                  child: Text('${dialogL10n.spaceRevokeRole}: ${role.name}'),
                ),
          ],
        );
      },
    );
    if (action == null || !context.mounted) return;

    final err = action.revoke
        ? await ref.read(spaceMemberActionsProvider).revokeRole(
            spaceId: spaceId,
            profileId: member.profileId,
            roleId: action.roleId,
          )
        : await ref.read(spaceMemberActionsProvider).assignRole(
            spaceId: spaceId,
            profileId: member.profileId,
            roleId: action.roleId,
          );
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            action.revoke
                ? l10n.spaceRevokeRoleError(err)
                : l10n.spaceAssignRoleError(err),
          ),
        ),
      );
    }
  }
}
