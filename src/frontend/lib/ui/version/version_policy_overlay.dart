import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/version_policy_providers.dart';
import '../../state/version_update_launcher.dart';
import '../../theme/voice_colors.dart';

final versionUpdateLauncherProvider = Provider<VersionUpdateLauncher>((ref) {
  return createVersionUpdateLauncher();
});

/// Force / soft update UI per [docs/features/updates.md] (skipped on web).
class VersionPolicyOverlay extends ConsumerWidget {
  const VersionPolicyOverlay({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final policy = ref.watch(versionPolicyProvider);
    final launcher = ref.read(versionUpdateLauncherProvider);
    return Stack(
      children: [
        child,
        if (policy.phase == VersionPolicyPhase.forceUpdate)
          _ForceUpdateBarrier(
            policy: policy,
            onUpdate: () => launcher.launchUpdate(
              updateUrl: policy.updateUrl ?? '',
              immediate: true,
            ),
          ),
        if (policy.phase == VersionPolicyPhase.softUpdate)
          _SoftUpdateBanner(
            policy: policy,
            onDismiss: () =>
                ref.read(versionPolicyProvider.notifier).dismissSoftUpdate(),
            onUpdate: () => launcher.launchUpdate(
              updateUrl: policy.updateUrl ?? '',
              immediate: false,
            ),
          ),
      ],
    );
  }
}

class _ForceUpdateBarrier extends StatelessWidget {
  const _ForceUpdateBarrier({
    required this.policy,
    required this.onUpdate,
  });

  final VersionPolicyState policy;
  final VoidCallback onUpdate;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    return Material(
      key: const Key('version_force_update_barrier'),
      color: voice.canvas.withValues(alpha: 0.92),
      child: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  l10n.versionUpdateRequired,
                  style: Theme.of(context).textTheme.headlineSmall,
                ),
                if (policy.releaseNotes != null) ...[
                  const SizedBox(height: 12),
                  Text(policy.releaseNotes!),
                ],
                if (policy.updateUrl != null) ...[
                  const SizedBox(height: 16),
                  FilledButton(
                    key: const Key('version_force_update_button'),
                    onPressed: onUpdate,
                    child: Text(l10n.versionUpdateNow),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _SoftUpdateBanner extends StatelessWidget {
  const _SoftUpdateBanner({
    required this.policy,
    required this.onDismiss,
    required this.onUpdate,
  });

  final VersionPolicyState policy;
  final VoidCallback onDismiss;
  final VoidCallback onUpdate;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final message = policy.latestVersion != null
        ? l10n.versionUpdateAvailable(policy.latestVersion!)
        : l10n.versionUpdateAvailableGeneric;
    return Align(
      alignment: Alignment.bottomCenter,
      child: Material(
        key: const Key('version_soft_update_banner'),
        color: voice.elevated,
        elevation: 4,
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Row(
            children: [
              Expanded(child: Text(message)),
              if (policy.updateUrl != null)
                TextButton(
                  key: const Key('version_soft_update_button'),
                  onPressed: onUpdate,
                  child: Text(l10n.versionUpdateNow),
                ),
              TextButton(
                key: const Key('version_soft_update_dismiss'),
                onPressed: onDismiss,
                child: Text(l10n.versionUpdateLater),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
