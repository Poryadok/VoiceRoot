import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/roles_client.dart';
import '../../backend/space_permissions.dart';
import '../../backend/spaces_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/social_providers.dart';
import '../../state/space_providers.dart';
import '../api_error_messages.dart';
import '../core/voice_avatar.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_skeleton.dart';
import '../core/voice_state_panel.dart';
import '../social/profile_detail_sheet.dart';

/// §24 Members/Bans tab state and the in-panel member/ban search filter.
/// One instance per space; the side panel and the bottom sheet share it.
enum SpaceMembersTabKind { members, bans }

final spaceMembersTabProvider =
    StateProvider.family<SpaceMembersTabKind, String>(
      (ref, spaceId) => SpaceMembersTabKind.members,
    );

final spaceMembersSearchQueryProvider = StateProvider.family<String, String>(
  (ref, spaceId) => '',
);

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
    final tab = ref.watch(spaceMembersTabProvider(spaceId));
    final query = ref.watch(spaceMembersSearchQueryProvider(spaceId));

    final canAssignRole =
        ref
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
    final canKick =
        ref
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
    final canBan =
        ref
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
    final canTimeout =
        ref
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

    // Bans are watched only while the audit tab exists (MEMBER_BAN); hidden tab
    // never fetches, per §24 row 8.
    final bansAsync = canBan ? ref.watch(spaceBansProvider(spaceId)) : null;

    final membersCount = membersAsync.hasValue
        ? '${membersAsync.value!.length}'
        : null;
    final bansCount =
        tab == SpaceMembersTabKind.bans && bansAsync?.hasValue == true
        ? l10n.spaceMembersBanCount(bansAsync!.value!.length)
        : null;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (showHeader) ...[
          Text(l10n.spaceMembersTitle, style: theme.textTheme.titleMedium),
          const SizedBox(height: 4),
          if (membersCount != null || bansCount != null)
            Text(bansCount ?? membersCount!, style: theme.textTheme.bodySmall)
          else
            Text(l10n.spaceMembersSubtitle, style: theme.textTheme.bodySmall),
          const SizedBox(height: 12),
        ],
        Row(
          key: const Key('space_members_sheet_tabs'),
          children: [
            TextButton(
              key: const Key('space_members_sheet_members_tab'),
              onPressed: () =>
                  ref.read(spaceMembersTabProvider(spaceId).notifier).state =
                      SpaceMembersTabKind.members,
              child: Text(l10n.spaceMembersTitle),
            ),
            const SizedBox(width: 8),
            if (canBan)
              TextButton(
                key: const Key('space_members_sheet_bans_tab'),
                onPressed: () =>
                    ref.read(spaceMembersTabProvider(spaceId).notifier).state =
                        SpaceMembersTabKind.bans,
                child: Text(l10n.spaceMembersBansTab),
              ),
          ],
        ),
        const SizedBox(height: 8),
        TextField(
          key: SpaceMembersSheet.searchKey,
          onChanged: (text) =>
              ref
                  .read(spaceMembersSearchQueryProvider(spaceId).notifier)
                  .state = text
                  .trim(),
          decoration: InputDecoration(
            labelText: tab == SpaceMembersTabKind.bans
                ? l10n.spaceMembersBansSearchHint
                : l10n.spaceMembersSearchHint,
            prefixIcon: const Icon(Icons.search),
            isDense: true,
          ),
        ),
        const SizedBox(height: 8),
        if (tab == SpaceMembersTabKind.bans && canBan)
          Expanded(
            child: bansAsync!.when(
              loading: () => const VoiceListSkeleton(rowCount: 3),
              error: (error, stack) => VoiceStatePanel(
                key: const Key('space_members_sheet_bans_error'),
                title: l10n.spaceMembersBansLoadError,
                message: spaceBansErrorMessage(l10n, error),
                icon: Icons.cloud_off_outlined,
                actionLabel: l10n.commonRetry,
                onAction: () => ref.invalidate(spaceBansProvider(spaceId)),
              ),
              data: (bans) {
                final filtered = query.isEmpty
                    ? bans
                    : bans
                          .where((ban) => _banMatchesQuery(ban, query))
                          .toList(growable: false);
                if (filtered.isEmpty && query.isNotEmpty) {
                  return VoiceStatePanel(
                    key: const Key('space_members_sheet_bans_no_matches'),
                    title: l10n.spaceMembersBansSearchNoMatches,
                    icon: Icons.search,
                  );
                }
                if (filtered.isEmpty) {
                  return VoiceStatePanel(
                    key: const Key('space_members_sheet_bans_empty'),
                    title: l10n.spaceMembersBansEmpty,
                    icon: Icons.verified_user_outlined,
                  );
                }
                return ListView.builder(
                  key: const Key('space_members_sheet_bans_body'),
                  itemCount: filtered.length + 1,
                  itemBuilder: (context, index) {
                    if (index == filtered.length) {
                      return Semantics(
                        liveRegion: true,
                        child: Text(
                          l10n.spaceMembersUnbanNotice,
                          style: theme.textTheme.bodySmall,
                        ),
                      );
                    }
                    final ban = filtered[index];
                    return _BanTile(
                      spaceId: spaceId,
                      ban: ban,
                      unbanKey: SpaceMembersSheet.unbanMemberKey(ban.accountId),
                    );
                  },
                );
              },
            ),
          )
        else
          Expanded(
            child: membersAsync.when(
              loading: () => const VoiceListSkeleton(rowCount: 5),
              error: (error, _) => VoiceStatePanel(
                title: l10n.spaceMembersLoadError,
                message: spaceMembersErrorMessage(l10n, error),
                icon: Icons.cloud_off_outlined,
                actionLabel: l10n.commonRetry,
                onAction: () => ref.invalidate(spaceMembersProvider(spaceId)),
              ),
              data: (members) {
                final filtered = query.isEmpty
                    ? members
                    : members
                          .where((member) => _memberMatchesQuery(member, query))
                          .toList(growable: false);
                if (filtered.isEmpty && query.isNotEmpty) {
                  return VoiceStatePanel(
                    key: const Key('space_members_sheet_search_no_matches'),
                    title: l10n.spaceMembersSearchNoMatches,
                    icon: Icons.search,
                  );
                }
                return ListView.builder(
                  itemCount: filtered.length,
                  itemBuilder: (context, index) {
                    final member = filtered[index];
                    return _MemberTile(
                      spaceId: spaceId,
                      member: member,
                      isSelf: member.profileId == activeId,
                      canKick: canKick && !member.isOwner,
                      canBan: canBan && !member.isOwner,
                      canTimeout: canTimeout && !member.isOwner,
                      canAssignRole: canAssignRole && !member.isOwner,
                      kickKey: SpaceMembersSheet.kickMemberKey(
                        member.profileId,
                      ),
                      banKey: SpaceMembersSheet.banMemberKey(member.profileId),
                      timeoutKey: SpaceMembersSheet.timeoutMemberKey(
                        member.profileId,
                      ),
                      assignRoleKey: SpaceMembersSheet.assignRoleKey(
                        member.profileId,
                      ),
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

bool _memberMatchesQuery(SpaceMemberRosterEntry member, String query) {
  final needle = query.toLowerCase();
  if (member.profileId.toLowerCase().contains(needle)) return true;
  if (member.nickname?.toLowerCase().contains(needle) ?? false) return true;
  return member.roleNames.any((role) => role.toLowerCase().contains(needle));
}

bool _banMatchesQuery(SpaceBanData ban, String query) {
  final needle = query.toLowerCase();
  if (ban.accountId.toLowerCase().contains(needle)) return true;
  if (ban.reason?.toLowerCase().contains(needle) ?? false) return true;
  return ban.bannedByProfileId?.toLowerCase().contains(needle) ?? false;
}

/// Bottom sheet: space members with role badges; Owner/Admin can kick and assign roles.
class SpaceMembersSheet extends ConsumerWidget {
  const SpaceMembersSheet({super.key, required this.spaceId});

  static const Key sheetKey = Key('space_members_sheet');

  static const Key searchKey = Key('space_members_sheet_search');

  static Key unbanMemberKey(String accountId) => Key('unban_member_$accountId');

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
      key: sheetKey,
      child: Padding(
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
    final label = profile?.displayName ?? profile?.handle ?? member.profileId;

    return ListTile(
      leading: VoiceAvatar(
        imageUrl: profile?.avatarUrl,
        label: label,
        radius: 20,
      ),
      title: InkWell(
        key: Key('member_profile_open_${member.profileId}'),
        onTap: () => _openProfile(context, ref, member.profileId),
        child: Text(isSelf ? l10n.spaceMemberYou(label) : label),
      ),
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

  /// Row tap opens the profile destination (§24); presence and the rest of the
  /// profile facts live there, not on the roster row.
  void _openProfile(BuildContext context, WidgetRef ref, String profileId) {
    final container = ProviderScope.containerOf(context);
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => UncontrolledProviderScope(
        container: container,
        child: ProfileDetailSheet(profileId: profileId),
      ),
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
    final reasonController = TextEditingController();
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) {
        final dialogL10n = AppLocalizations.of(ctx)!;
        return AlertDialog(
          title: Text(dialogL10n.spaceBanConfirmTitle),
          content: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(dialogL10n.spaceBanConfirmMessage(label)),
              TextField(
                key: const Key('space_ban_dialog_reason'),
                controller: reasonController,
                decoration: InputDecoration(
                  labelText: dialogL10n.spaceBanReasonHint,
                  isDense: true,
                ),
              ),
            ],
          ),
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
    final reason = reasonController.text.trim();
    final err = await ref
        .read(spaceMemberActionsProvider)
        .banMember(
          spaceId: spaceId,
          accountId: accountId,
          profileId: member.profileId,
          reason: reason.isEmpty ? null : reason,
        );
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            err == 'not_authenticated'
                ? l10n.spaceBanError(commonActionErrorMessage(l10n))
                : commonActionErrorMessage(l10n),
          ),
        ),
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
    final err = await ref
        .read(spaceMemberActionsProvider)
        .timeoutMember(
          spaceId: spaceId,
          profileId: member.profileId,
          durationSeconds: durationSeconds,
        );
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            err == 'not_authenticated'
                ? l10n.spaceTimeoutError(commonActionErrorMessage(l10n))
                : commonActionErrorMessage(l10n),
          ),
        ),
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
    final err = await ref
        .read(spaceMemberActionsProvider)
        .kickMember(spaceId: spaceId, profileId: member.profileId);
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            err == 'not_authenticated'
                ? l10n.spaceKickError(commonActionErrorMessage(l10n))
                : commonActionErrorMessage(l10n),
          ),
        ),
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
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(l10n.spaceAssignRoleEmpty)));
      return;
    }

    final memberRoles = await ref
        .read(voiceRolesClientProvider)
        .getMemberRoles(
          authorization: ref.read(authorizationHeaderProvider)!,
          spaceId: spaceId,
          profileId: member.profileId,
        );
    final assignedIds = switch (memberRoles) {
      RolesApiOk(:final data) => data.map((r) => r.id).toSet(),
      RolesApiFailure() => <String>{},
    };

    if (!context.mounted) return;

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
                  onPressed: () =>
                      Navigator.of(ctx).pop((roleId: role.id, revoke: false)),
                  child: Text('${dialogL10n.spaceAssignRole}: ${role.name}'),
                ),
            for (final role in assignable)
              if (assignedIds.contains(role.id) && role.name != kSpaceRoleOwner)
                SimpleDialogOption(
                  onPressed: () =>
                      Navigator.of(ctx).pop((roleId: role.id, revoke: true)),
                  child: Text('${dialogL10n.spaceRevokeRole}: ${role.name}'),
                ),
          ],
        );
      },
    );
    if (action == null || !context.mounted) return;

    final err = action.revoke
        ? await ref
              .read(spaceMemberActionsProvider)
              .revokeRole(
                spaceId: spaceId,
                profileId: member.profileId,
                roleId: action.roleId,
              )
        : await ref
              .read(spaceMemberActionsProvider)
              .assignRole(
                spaceId: spaceId,
                profileId: member.profileId,
                roleId: action.roleId,
              );
    if (!context.mounted) return;
    if (err != null) {
      final message = err == 'not_authenticated'
          ? action.revoke
                ? l10n.spaceRevokeRoleError(commonActionErrorMessage(l10n))
                : l10n.spaceAssignRoleError(commonActionErrorMessage(l10n))
          : commonActionErrorMessage(l10n);
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(message)));
    }
  }
}

/// Ban audit row: banned account (labelled by accountId honestly — there is no
/// display-name API for banned accounts), reason · moderator · when, and unban.
class _BanTile extends ConsumerWidget {
  const _BanTile({
    required this.spaceId,
    required this.ban,
    required this.unbanKey,
  });

  final String spaceId;
  final SpaceBanData ban;
  final Key unbanKey;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final moderatorAsync = ban.bannedByProfileId == null
        ? null
        : ref.watch(profileProvider(ban.bannedByProfileId!));
    final moderator = moderatorAsync?.valueOrNull;
    final moderatorLabel =
        moderator?.displayName ??
        moderator?.handle ??
        ban.bannedByProfileId ??
        '';
    final parts = [
      if (ban.reason != null && ban.reason!.isNotEmpty) ban.reason!,
      if (moderatorLabel.isNotEmpty) moderatorLabel,
      profileLastSeenLabel(context, ban.bannedAt),
    ];

    return ListTile(
      leading: VoiceAvatar(label: ban.accountId, radius: 20),
      title: Text(ban.accountId),
      subtitle: Text(parts.join(' · ')),
      trailing: TextButton(
        key: unbanKey,
        onPressed: () => _unban(context, ref),
        child: Text(l10n.spaceMembersUnban),
      ),
    );
  }

  Future<void> _unban(BuildContext context, WidgetRef ref) async {
    final l10n = AppLocalizations.of(context)!;
    final err = await ref
        .read(spaceMemberActionsProvider)
        .unbanMember(spaceId: spaceId, accountId: ban.accountId);
    if (!context.mounted) return;
    if (err != null) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            err == 'not_authenticated'
                ? l10n.spaceMembersUnbanError(commonActionErrorMessage(l10n))
                : commonActionErrorMessage(l10n),
          ),
        ),
      );
    }
  }
}
