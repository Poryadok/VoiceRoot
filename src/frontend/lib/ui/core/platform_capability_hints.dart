import 'package:flutter/material.dart';

import '../../backend/platform_capabilities.dart';
import '../../l10n/app_localizations.dart';

/// Tooltip text for web-only platform limitations (docs/features/platforms.md).
String? platformWebVoiceLimitationsTooltip(AppLocalizations l10n) {
  final hints = <String>[
    if (!canCaptureSystemAudioWithScreenShare)
      l10n.platformWebSystemAudioUnavailable,
    if (!canUseGlobalPushToTalkHotkey) l10n.platformWebGlobalPttUnavailable,
  ];
  if (hints.isEmpty) return null;
  return hints.join('\n');
}

/// Info affordance for web voice limitations (system audio, global PTT).
class PlatformWebVoiceLimitationsButton extends StatelessWidget {
  const PlatformWebVoiceLimitationsButton({super.key});

  static const Key buttonKey = Key('platform_web_voice_limitations');

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final tooltip = platformWebVoiceLimitationsTooltip(l10n);
    if (tooltip == null) return const SizedBox.shrink();
    return IconButton(
      key: buttonKey,
      tooltip: tooltip,
      onPressed: null,
      icon: const Icon(Icons.info_outline),
    );
  }
}
