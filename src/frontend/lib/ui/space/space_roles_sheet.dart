import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/space_permissions.dart';
import '../../l10n/app_localizations.dart';
import '../../state/space_providers.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_state_panel.dart';
import 'space_role_editor_sheet.dart';

class SpaceRolesSheet extends ConsumerWidget {
  const SpaceRolesSheet({super.key, required this.spaceId});

  static const Key sheetKey = Key('space_roles_sheet');

  final String spaceId;

  static Future<void> show(BuildContext context, {required String spaceId}) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: UncontrolledProviderScope(
        container: container,
        child: SpaceRolesSheet(spaceId: spaceId),
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final rolesAsync = ref.watch(spaceRolesProvider(spaceId));
    final defaultJoinAsync = ref.watch(defaultJoinRoleProvider(spaceId));
    final canManage = ref
            .watch(
              spacePermissionProvider((
                spaceId: spaceId,
                permission: SpacePermissions.spaceManageRoles,
                chatId: null,
                voiceRoomId: null,
              )),
            )
            .valueOrNull ??
        false;

    return SafeArea(
      child: Padding(
        key: sheetKey,
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(l10n.spaceRolesTitle, style: theme.textTheme.titleMedium),
                ),
                if (canManage)
                  IconButton(
                    key: const Key('create_space_role'),
                    tooltip: l10n.spaceRoleCreateTitle,
                    onPressed: () => SpaceRoleEditorSheet.show(context, spaceId: spaceId),
                    icon: const Icon(Icons.add),
                  ),
              ],
            ),
            defaultJoinAsync.when(
              loading: () => const SizedBox.shrink(),
              error: (_, __) => const SizedBox.shrink(),
              data: (role) {
                if (role == null) return const SizedBox.shrink();
                return Padding(
                  padding: const EdgeInsets.only(bottom: 8),
                  child: Text(
                    l10n.spaceDefaultJoinRole(role.name),
                    style: theme.textTheme.bodySmall,
                  ),
                );
              },
            ),
            Expanded(
              child: rolesAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (error, _) => VoiceStatePanel(
                  title: l10n.spaceRolesLoadError,
                  message: '$error',
                  icon: Icons.badge_outlined,
                  actionLabel: l10n.commonRetry,
                  onAction: () => ref.invalidate(spaceRolesProvider(spaceId)),
                ),
                data: (roles) => ListView.builder(
                  itemCount: roles.length,
                  itemBuilder: (context, index) {
                    final role = roles[index];
                    final isDefault = defaultJoinAsync.valueOrNull?.id == role.id;
                    return ListTile(
                      key: Key('space_role_${role.id}'),
                      title: Text(role.name),
                      subtitle: Text(
                        role.managed ? l10n.spaceRoleManaged : l10n.spaceRoleCustom,
                      ),
                      trailing: canManage && !role.managed
                          ? PopupMenuButton<String>(
                              onSelected: (value) async {
                                final actions = ref.read(spaceRoleActionsProvider);
                                if (value == 'edit') {
                                  await SpaceRoleEditorSheet.show(
                                    context,
                                    spaceId: spaceId,
                                    role: role,
                                  );
                                } else if (value == 'default') {
                                  final err = await actions.setDefaultJoinRole(
                                    spaceId: spaceId,
                                    roleId: role.id,
                                  );
                                  if (context.mounted && err != null) {
                                    ScaffoldMessenger.of(context).showSnackBar(
                                      SnackBar(content: Text(err)),
                                    );
                                  }
                                } else if (value == 'delete') {
                                  final err = await actions.deleteRole(
                                    spaceId: spaceId,
                                    roleId: role.id,
                                  );
                                  if (context.mounted && err != null) {
                                    ScaffoldMessenger.of(context).showSnackBar(
                                      SnackBar(content: Text(err)),
                                    );
                                  }
                                }
                              },
                              itemBuilder: (context) => [
                                PopupMenuItem(value: 'edit', child: Text(l10n.commonEdit)),
                                if (!isDefault)
                                  PopupMenuItem(
                                    value: 'default',
                                    child: Text(l10n.spaceSetDefaultJoinRole),
                                  ),
                                PopupMenuItem(value: 'delete', child: Text(l10n.commonDelete)),
                              ],
                            )
                          : isDefault
                              ? const Icon(Icons.login, size: 18)
                              : null,
                      onTap: canManage && !role.managed
                          ? () => SpaceRoleEditorSheet.show(
                                context,
                                spaceId: spaceId,
                                role: role,
                              )
                          : null,
                    );
                  },
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
