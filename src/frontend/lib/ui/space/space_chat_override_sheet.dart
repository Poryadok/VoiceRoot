import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/roles_client.dart';
import '../../backend/space_permissions.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/space_providers.dart';
import '../core/voice_bottom_sheet.dart';

/// Per-chat role deny overrides (app stack0 custom roles).
class SpaceChatOverrideSheet extends ConsumerStatefulWidget {
  const SpaceChatOverrideSheet({
    super.key,
    required this.spaceId,
    required this.chatId,
  });

  final String spaceId;
  final String chatId;

  static Future<void> show(
    BuildContext context, {
    required String spaceId,
    required String chatId,
  }) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      child: UncontrolledProviderScope(
        container: container,
        child: SpaceChatOverrideSheet(spaceId: spaceId, chatId: chatId),
      ),
    );
  }

  @override
  ConsumerState<SpaceChatOverrideSheet> createState() =>
      _SpaceChatOverrideSheetState();
}

class _SpaceChatOverrideSheetState extends ConsumerState<SpaceChatOverrideSheet> {
  final Set<String> _denyViewRoleIds = {};
  final Set<String> _denySendRoleIds = {};
  bool _busy = false;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final rolesAsync = ref.watch(spaceRolesProvider(widget.spaceId));
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              l10n.spaceChatOverrideTitle,
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 8),
            Text(
              l10n.spaceChatOverrideHint,
              style: Theme.of(context).textTheme.bodySmall,
            ),
            const SizedBox(height: 12),
            Flexible(
              child: rolesAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (e, _) => Text('$e'),
                data: (roles) => ListView(
                  shrinkWrap: true,
                  children: [
                    for (final role in roles)
                      if (!role.managed) ...[
                        SwitchListTile(
                          title: Text(role.name),
                          subtitle: Text(l10n.spaceChatOverrideDenyView),
                          value: _denyViewRoleIds.contains(role.id),
                          onChanged: _busy
                              ? null
                              : (enabled) => _setDeny(
                                    role.id,
                                    enabled,
                                    SpacePermissions.textChatView,
                                    _denyViewRoleIds,
                                  ),
                        ),
                        SwitchListTile(
                          title: Text(role.name),
                          subtitle: Text(l10n.spaceChatOverrideDenySend),
                          value: _denySendRoleIds.contains(role.id),
                          onChanged: _busy
                              ? null
                              : (enabled) => _setDeny(
                                    role.id,
                                    enabled,
                                    SpacePermissions.textChatSendMessages,
                                    _denySendRoleIds,
                                  ),
                        ),
                      ],
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _setDeny(
    String roleId,
    bool deny,
    String permissionName,
    Set<String> tracked,
  ) async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    setState(() {
      _busy = true;
      if (deny) {
        tracked.add(roleId);
      } else {
        tracked.remove(roleId);
      }
    });
    final denyMask = deny
        ? SpacePermissions.setPermission(0, permissionName, true)
        : 0;
    final result = await ref.read(voiceRolesClientProvider).setChatOverride(
          authorization: auth,
          spaceId: widget.spaceId,
          chatId: widget.chatId,
          roleId: roleId,
          denyMask: denyMask,
        );
    if (!mounted) return;
    setState(() => _busy = false);
    if (result is RolesApiFailure) {
      setState(() {
        if (deny) {
          tracked.remove(roleId);
        } else {
          tracked.add(roleId);
        }
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(result.message)),
      );
    }
  }
}
