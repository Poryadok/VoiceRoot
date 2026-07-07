import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/roles_client.dart';
import '../../backend/space_permissions.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/space_providers.dart';
import '../core/voice_bottom_sheet.dart';

/// Per-voice-room role deny overrides (app stack0 custom roles).
class SpaceVoiceRoomOverrideSheet extends ConsumerStatefulWidget {
  const SpaceVoiceRoomOverrideSheet({
    super.key,
    required this.spaceId,
    required this.voiceRoomId,
  });

  final String spaceId;
  final String voiceRoomId;

  static Future<void> show(
    BuildContext context, {
    required String spaceId,
    required String voiceRoomId,
  }) {
    final container = ProviderScope.containerOf(context);
    return showVoiceBottomSheet<void>(
      context: context,
      child: UncontrolledProviderScope(
        container: container,
        child: SpaceVoiceRoomOverrideSheet(
          spaceId: spaceId,
          voiceRoomId: voiceRoomId,
        ),
      ),
    );
  }

  @override
  ConsumerState<SpaceVoiceRoomOverrideSheet> createState() =>
      _SpaceVoiceRoomOverrideSheetState();
}

class _SpaceVoiceRoomOverrideSheetState
    extends ConsumerState<SpaceVoiceRoomOverrideSheet> {
  final Set<String> _denyJoinRoleIds = {};
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
              l10n.spaceVoiceOverrideTitle,
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 8),
            Text(
              l10n.spaceVoiceOverrideHint,
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
                      if (!role.managed)
                        SwitchListTile(
                          title: Text(role.name),
                          subtitle: Text(l10n.spaceVoiceOverrideDenyJoin),
                          value: _denyJoinRoleIds.contains(role.id),
                          onChanged: _busy
                              ? null
                              : (enabled) => _setDenyJoin(role.id, enabled),
                        ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _setDenyJoin(String roleId, bool deny) async {
    final auth = ref.read(authorizationHeaderProvider);
    if (auth == null) return;
    setState(() {
      _busy = true;
      if (deny) {
        _denyJoinRoleIds.add(roleId);
      } else {
        _denyJoinRoleIds.remove(roleId);
      }
    });
    final denyMask = deny
        ? SpacePermissions.setPermission(0, SpacePermissions.voiceJoin, true)
        : 0;
    final result = await ref.read(voiceRolesClientProvider).setVoiceRoomOverride(
          authorization: auth,
          spaceId: widget.spaceId,
          voiceRoomId: widget.voiceRoomId,
          roleId: roleId,
          denyMask: denyMask,
        );
    if (!mounted) return;
    setState(() => _busy = false);
    if (result is RolesApiFailure) {
      setState(() {
        if (deny) {
          _denyJoinRoleIds.remove(roleId);
        } else {
          _denyJoinRoleIds.add(roleId);
        }
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(result.message)),
      );
    }
  }
}
