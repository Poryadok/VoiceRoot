import 'package:flutter/material.dart';

import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';

/// Static help / FAQ (docs/features/onboarding.md — no tutorial replay).
class HelpSheet extends StatelessWidget {
  const HelpSheet({super.key});

  static Future<void> show(BuildContext context) {
    return showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (_) => const HelpSheet(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(l10n.settingsHelpTitle, style: Theme.of(context).textTheme.titleLarge),
            const SizedBox(height: 16),
            _HelpItem(title: l10n.settingsHelpChatsTitle, body: l10n.settingsHelpChatsBody),
            _HelpItem(title: l10n.settingsHelpSpacesTitle, body: l10n.settingsHelpSpacesBody),
            _HelpItem(title: l10n.settingsHelpMatchmakingTitle, body: l10n.settingsHelpMatchmakingBody),
            _HelpItem(title: l10n.settingsHelpVoiceTitle, body: l10n.settingsHelpVoiceBody),
            const SizedBox(height: 8),
            Text(
              l10n.settingsHelpFooter,
              style: TextStyle(color: voice.textSecondary, fontSize: 13),
            ),
          ],
        ),
      ),
    );
  }
}

class _HelpItem extends StatelessWidget {
  const _HelpItem({required this.title, required this.body});

  final String title;
  final String body;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, style: Theme.of(context).textTheme.titleSmall),
          const SizedBox(height: 4),
          Text(body),
        ],
      ),
    );
  }
}
