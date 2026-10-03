import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/space_permissions.dart';
import '../../l10n/app_localizations.dart';
import '../../state/space_providers.dart';
import 'voice_list_row.dart';

/// Localized explanation when a space permission is missing.
String spacePermissionDeniedReason(AppLocalizations l10n, String permission) {
  return switch (permission) {
    SpacePermissions.spaceManageRoles => l10n.spacePermissionDeniedManageRoles,
    SpacePermissions.textChatSetSlowMode => l10n.spacePermissionDeniedSetSlowMode,
    SpacePermissions.voiceJoin => l10n.spacePermissionDeniedVoiceJoin,
    SpacePermissions.textChatSendMessages => l10n.spacePermissionDeniedSendMessages,
    _ => l10n.spacePermissionDeniedGeneric(_permissionLabel(permission)),
  };
}

String _permissionLabel(String permission) {
  return permission
      .toLowerCase()
      .replaceAll('_', ' ')
      .split(' ')
      .map((part) => part.isEmpty ? part : '${part[0].toUpperCase()}${part.substring(1)}')
      .join(' ');
}

/// Resolved permission state from [spacePermissionProvider].
({bool allowed, String? deniedReason}) resolveSpacePermission(
  AppLocalizations l10n,
  AsyncValue<bool> asyncAllowed,
  String permission,
) {
  return asyncAllowed.when(
    data: (allowed) => allowed
        ? (allowed: true, deniedReason: null)
        : (
            allowed: false,
            deniedReason: spacePermissionDeniedReason(l10n, permission),
          ),
    loading: () => (allowed: true, deniedReason: null),
    error: (_, _) => (
      allowed: false,
      deniedReason: spacePermissionDeniedReason(l10n, permission),
    ),
  );
}

/// Icon button that keeps a tooltip with the denial reason when disabled.
class VoiceDisabledIconButton extends StatelessWidget {
  const VoiceDisabledIconButton({
    super.key,
    required this.tooltip,
    required this.icon,
    this.onPressed,
    this.disabledReason,
    this.iconSize,
  });

  final String tooltip;
  final Widget icon;
  final VoidCallback? onPressed;
  final String? disabledReason;
  final double? iconSize;

  bool get _enabled => onPressed != null && disabledReason == null;

  @override
  Widget build(BuildContext context) {
    final message = disabledReason ?? tooltip;
    return Tooltip(
      message: message,
      child: IconButton(
        tooltip: '',
        onPressed: _enabled ? onPressed : null,
        icon: icon,
        iconSize: iconSize,
      ),
    );
  }
}

/// Wraps [child] with a tooltip when [disabledReason] is set.
class VoiceDisabledAction extends StatelessWidget {
  const VoiceDisabledAction({
    super.key,
    required this.child,
    this.disabledReason,
  });

  final Widget child;
  final String? disabledReason;

  @override
  Widget build(BuildContext context) {
    if (disabledReason == null) return child;
    return Tooltip(
      message: disabledReason!,
      child: child,
    );
  }
}

/// [VoiceListRow] that disables tap and surfaces [disabledReason] in the subtitle.
class VoicePermissionListRow extends StatelessWidget {
  const VoicePermissionListRow({
    super.key,
    required this.title,
    required this.baseSubtitle,
    this.leading,
    this.trailing,
    this.selected = false,
    this.onTap,
    this.disabledReason,
  });

  final String title;
  final String? baseSubtitle;
  final Widget? leading;
  final Widget? trailing;
  final bool selected;
  final VoidCallback? onTap;
  final String? disabledReason;

  @override
  Widget build(BuildContext context) {
    final subtitle = disabledReason ?? baseSubtitle;
    final row = VoiceListRow(
      title: title,
      subtitle: subtitle,
      leading: leading,
      trailing: trailing,
      selected: selected,
      onTap: disabledReason == null ? onTap : null,
    );
    return VoiceDisabledAction(
      disabledReason: disabledReason,
      child: Opacity(
        opacity: disabledReason == null ? 1 : 0.55,
        child: row,
      ),
    );
  }
}
