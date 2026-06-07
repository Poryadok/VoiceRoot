import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../state/version_policy_providers.dart';

/// Force / soft update UI per [docs/features/updates.md] (skipped on web).
class VersionPolicyOverlay extends ConsumerWidget {
  const VersionPolicyOverlay({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final policy = ref.watch(versionPolicyProvider);
    return Stack(
      children: [
        child,
        if (policy.phase == VersionPolicyPhase.forceUpdate)
          _ForceUpdateBarrier(policy: policy),
        if (policy.phase == VersionPolicyPhase.softUpdate)
          _SoftUpdateBanner(
            policy: policy,
            onDismiss: () =>
                ref.read(versionPolicyProvider.notifier).dismissSoftUpdate(),
          ),
      ],
    );
  }
}

class _ForceUpdateBarrier extends StatelessWidget {
  const _ForceUpdateBarrier({required this.policy});

  final VersionPolicyState policy;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.black87,
      child: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  'Update required',
                  style: Theme.of(context).textTheme.headlineSmall,
                ),
                if (policy.releaseNotes != null) ...[
                  const SizedBox(height: 12),
                  Text(policy.releaseNotes!),
                ],
                if (policy.updateUrl != null) ...[
                  const SizedBox(height: 12),
                  SelectableText(policy.updateUrl!),
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
  const _SoftUpdateBanner({required this.policy, required this.onDismiss});

  final VersionPolicyState policy;
  final VoidCallback onDismiss;

  @override
  Widget build(BuildContext context) {
    return Align(
      alignment: Alignment.bottomCenter,
      child: Material(
        elevation: 8,
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  policy.latestVersion != null
                      ? 'Update ${policy.latestVersion} available'
                      : 'Update available',
                ),
              ),
              TextButton(onPressed: onDismiss, child: const Text('Later')),
            ],
          ),
        ),
      ),
    );
  }
}
