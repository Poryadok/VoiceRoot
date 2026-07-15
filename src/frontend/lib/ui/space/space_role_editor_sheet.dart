import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/roles_client.dart';
import '../../backend/space_permissions.dart';
import '../../l10n/app_localizations.dart';
import '../../state/space_providers.dart';
import '../core/voice_bottom_sheet.dart';
import '../core/voice_compact_banner.dart';
import '../core/voice_disabled_action.dart';

class SpaceRoleEditorSheet extends ConsumerStatefulWidget {
  const SpaceRoleEditorSheet({
    super.key,
    required this.spaceId,
    this.role,
  });

  final String spaceId;
  final SpaceRole? role;

  static Future<void> show(
    BuildContext context, {
    required String spaceId,
    SpaceRole? role,
  }) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      child: UncontrolledProviderScope(
        container: container,
        child: SpaceRoleEditorSheet(spaceId: spaceId, role: role),
      ),
    );
  }

  @override
  ConsumerState<SpaceRoleEditorSheet> createState() => _SpaceRoleEditorSheetState();
}

class _SpaceRoleEditorSheetState extends ConsumerState<SpaceRoleEditorSheet> {
  late final TextEditingController _nameController;
  late int _mask;
  var _saving = false;

  @override
  void initState() {
    super.initState();
    _nameController = TextEditingController(text: widget.role?.name ?? '');
    _mask = widget.role?.permissionsMask ?? 0;
  }

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final isEdit = widget.role != null;
    final manageRoles = resolveSpacePermission(
      l10n,
      ref.watch(
        spacePermissionProvider((
          spaceId: widget.spaceId,
          permission: SpacePermissions.spaceManageRoles,
          chatId: null,
          voiceRoomId: null,
        )),
      ),
      SpacePermissions.spaceManageRoles,
    );
    final canEdit = manageRoles.allowed;

    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              isEdit ? l10n.spaceRoleEditTitle : l10n.spaceRoleCreateTitle,
              style: theme.textTheme.titleMedium,
            ),
            if (manageRoles.deniedReason != null) ...[
              const SizedBox(height: 8),
              VoiceCompactBanner(
                key: const Key('space_role_editor_denied'),
                message: manageRoles.deniedReason!,
                icon: Icons.lock_outline,
                tone: VoiceBannerTone.warning,
              ),
            ],
            const SizedBox(height: 12),
            TextField(
              controller: _nameController,
              readOnly: !canEdit,
              decoration: InputDecoration(labelText: l10n.spaceRoleNameLabel),
            ),
            const SizedBox(height: 12),
            Expanded(
              child: ListView(
                children: [
                  for (final entry in SpacePermissions.editableGroups.entries) ...[
                    Text(entry.key, style: theme.textTheme.titleSmall),
                    for (final perm in entry.value)
                      VoiceDisabledAction(
                        disabledReason: manageRoles.deniedReason,
                        child: CheckboxListTile(
                          value: SpacePermissions.hasPermission(_mask, perm),
                          onChanged: canEdit
                              ? (value) {
                                  setState(() {
                                    _mask = SpacePermissions.setPermission(
                                      _mask,
                                      perm,
                                      value ?? false,
                                    );
                                  });
                                }
                              : null,
                          title: Text(perm, style: theme.textTheme.bodySmall),
                          dense: true,
                          controlAffinity: ListTileControlAffinity.leading,
                        ),
                      ),
                    const SizedBox(height: 8),
                  ],
                ],
              ),
            ),
            VoiceDisabledAction(
              disabledReason: manageRoles.deniedReason,
              child: FilledButton(
                onPressed: _saving || !canEdit ? null : _save,
                child: Text(l10n.commonSave),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _save() async {
    final name = _nameController.text.trim();
    if (name.isEmpty) return;
    setState(() => _saving = true);
    final actions = ref.read(spaceRoleActionsProvider);
    final error = widget.role == null
        ? await actions.createRole(
            spaceId: widget.spaceId,
            name: name,
            permissionsMask: _mask,
          )
        : await actions.updateRole(
            spaceId: widget.spaceId,
            roleId: widget.role!.id,
            name: name,
            permissionsMask: _mask,
          );
    if (!mounted) return;
    setState(() => _saving = false);
    if (error != null) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(error)));
      return;
    }
    Navigator.of(context).pop();
  }
}
