import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/shell_providers.dart';
import '../../backend/space_permissions.dart';
import '../../state/space_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';
import 'space_invites_sheet.dart';
import 'space_bots_sheet.dart';
import 'space_members_sheet.dart';
import 'space_roles_sheet.dart';
import 'space_tree_panel.dart';
import '../report/report_sheet.dart';

/// Middle column: space tree with header actions (members, invites).
class SpaceTreeColumn extends ConsumerWidget {
  const SpaceTreeColumn({
    super.key,
    required this.spaceId,
    this.selectedChatId,
    required this.onTextChatSelected,
  });

  final String spaceId;
  final String? selectedChatId;
  final ValueChanged<String> onTextChatSelected;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final spaceAsync = ref.watch(spaceProvider(spaceId));
    final shellNav = ref.read(shellNavigationProvider);
    final narrow = VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width);
    final canManageBots = ref
            .watch(
              spacePermissionProvider((
                spaceId: spaceId,
                permission: SpacePermissions.spaceManageBots,
                chatId: null,
                voiceRoomId: null,
              )),
            )
            .valueOrNull ??
        false;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Material(
          color: voice.surface,
          child: Padding(
            padding: const EdgeInsets.fromLTRB(8, 8, 4, 8),
            child: Row(
              children: [
                Expanded(
                  child: spaceAsync.when(
                    data: (space) => Text(
                      space.name,
                      style: Theme.of(context).textTheme.titleMedium,
                      overflow: TextOverflow.ellipsis,
                    ),
                    loading: () => Text(
                      l10n.spaceTreeTitle,
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                    error: (_, _) => Text(
                      l10n.spaceTreeTitle,
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                  ),
                ),
                IconButton(
                  key: const Key('space_members_action'),
                  icon: const Icon(Icons.groups_outlined),
                  tooltip: l10n.spaceMembersTooltip,
                  onPressed: () {
                    if (narrow) {
                      SpaceMembersSheet.show(context, spaceId: spaceId);
                    } else {
                      shellNav.toggleSidePanel(ShellSidePanel.members);
                    }
                  },
                ),
                IconButton(
                  key: const Key('space_roles_action'),
                  icon: const Icon(Icons.badge_outlined),
                  tooltip: l10n.spaceRolesTooltip,
                  onPressed: () => SpaceRolesSheet.show(context, spaceId: spaceId),
                ),
                IconButton(
                  key: const Key('space_bots_action'),
                  icon: const Icon(Icons.smart_toy_outlined),
                  tooltip: l10n.spaceBotsTitle,
                  onPressed: canManageBots
                      ? () => SpaceBotsSheet.show(context, spaceId: spaceId)
                      : null,
                ),
                IconButton(
                  key: const Key('space_invites_action'),
                  icon: const Icon(Icons.person_add_outlined),
                  tooltip: l10n.spaceInvitesTooltip,
                  onPressed: () =>
                      SpaceInvitesSheet.show(context, spaceId: spaceId),
                ),
                IconButton(
                  key: const Key('space_report_action'),
                  icon: const Icon(Icons.flag_outlined),
                  tooltip: l10n.reportAction,
                  onPressed: () => ReportSheet.show(
                    context,
                    target: ReportSpaceTarget(spaceId: spaceId),
                  ),
                ),
              ],
            ),
          ),
        ),
        Divider(height: 1, color: voice.borderDefault),
        Expanded(
          child: SpaceTreePanel(
            spaceId: spaceId,
            selectedChatId: selectedChatId,
            onTextChatSelected: onTextChatSelected,
          ),
        ),
      ],
    );
  }
}
