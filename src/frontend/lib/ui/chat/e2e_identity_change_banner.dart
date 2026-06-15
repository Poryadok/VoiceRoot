import 'package:flutter/material.dart';

import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';

/// In-chat banner when a peer's identity key changes (docs/features/encryption.md).
class E2eIdentityChangeBanner extends StatelessWidget {
  const E2eIdentityChangeBanner({
    super.key,
    required this.peerDisplayName,
    required this.onContinue,
    required this.onDistrust,
  });

  static const Key bannerKey = Key('e2e_identity_change_banner');
  static const Key continueButtonKey = Key('e2e_identity_change_continue');
  static const Key distrustButtonKey = Key('e2e_identity_change_distrust');

  final String peerDisplayName;
  final VoidCallback onContinue;
  final VoidCallback onDistrust;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final body = l10n.e2eIdentityKeyChangedBody;

    return Material(
      key: bannerKey,
      color: voice.surface,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              l10n.e2eIdentityKeyChangedTitle(peerDisplayName),
              style: Theme.of(context).textTheme.titleSmall,
            ),
            const SizedBox(height: 8),
            Text(
              body,
              style: TextStyle(color: voice.textSecondary),
            ),
            const SizedBox(height: 12),
            Wrap(
              spacing: 8,
              children: [
                FilledButton(
                  key: continueButtonKey,
                  onPressed: onContinue,
                  child: Text(l10n.e2eIdentityKeyChangedContinue),
                ),
                TextButton(
                  key: distrustButtonKey,
                  onPressed: onDistrust,
                  child: Text(l10n.e2eIdentityKeyChangedDistrust),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
