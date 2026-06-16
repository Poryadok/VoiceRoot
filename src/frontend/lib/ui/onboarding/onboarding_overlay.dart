import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../state/onboarding_controller.dart';
import '../../state/social_providers.dart';
import '../profile/profile_edit_sheet.dart';

/// Contextual onboarding hints (docs/features/onboarding.md).
class OnboardingOverlay extends ConsumerStatefulWidget {
  const OnboardingOverlay({super.key, required this.child});

  final Widget child;

  @override
  ConsumerState<OnboardingOverlay> createState() => _OnboardingOverlayState();
}

class _OnboardingOverlayState extends ConsumerState<OnboardingOverlay> {
  var _loaded = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadOnboarding());
  }

  Future<void> _loadOnboarding() async {
    if (_loaded) return;
    _loaded = true;
    await ref.read(onboardingControllerProvider.notifier).load();
    if (!mounted) return;
    _maybeShowStep();
  }

  void _maybeShowStep() {
    final onboarding = ref.read(onboardingControllerProvider);
    if (!onboarding.shouldShowHints) return;
    final step = onboarding.currentStep;
    if (step == null) return;

    switch (step) {
      case OnboardingStep.saveAccount:
        _showSaveAccountModal();
      case OnboardingStep.chatsNav:
        _showHintDialog(
          title: 'Chats and navigation',
          body: 'All your chats live here — DMs, groups, channels, and spaces.',
          onContinue: () => ref.read(onboardingControllerProvider.notifier).completeCurrentStep(),
        );
      case OnboardingStep.spaces:
        _showHintDialog(
          title: 'Spaces',
          body: 'Spaces are communities with channels and voice rooms. Find one for your game or create your own.',
          onContinue: () => ref.read(onboardingControllerProvider.notifier).completeCurrentStep(),
        );
      case OnboardingStep.matchmaking:
        _showHintDialog(
          title: 'Matchmaking',
          body: 'Looking for a squad? We match you with people who fit your criteria.',
          onContinue: () => ref.read(onboardingControllerProvider.notifier).completeCurrentStep(),
        );
      case OnboardingStep.wrapUp:
        _showHintDialog(
          title: 'You are all set',
          body: 'You know the basics! Help is always available in Settings.',
          onContinue: () => ref.read(onboardingControllerProvider.notifier).completeCurrentStep(),
          continueLabel: 'Start',
        );
    }
  }

  Future<void> _showSaveAccountModal() async {
    final profile = ref.read(activeProfileProvider).valueOrNull;
    if (profile == null) return;
    if (!mounted) return;
    await showDialog<void>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => AlertDialog(
        title: const Text('Save your account'),
        content: const Text(
          'Set a username and optional email so you do not lose access.',
        ),
        actions: [
          TextButton(
            onPressed: () async {
              await ref.read(onboardingControllerProvider.notifier).dismiss();
              if (ctx.mounted) Navigator.of(ctx).pop();
            },
            child: const Text('Skip'),
          ),
          FilledButton(
            onPressed: () async {
              Navigator.of(ctx).pop();
              if (!mounted) return;
              await showModalBottomSheet<void>(
                context: context,
                isScrollControlled: true,
                builder: (_) => ProfileEditSheet(profile: profile),
              );
              await ref.read(onboardingControllerProvider.notifier).completeCurrentStep();
              _maybeShowStep();
            },
            child: const Text('Save'),
          ),
        ],
      ),
    );
    if (mounted) _maybeShowStep();
  }

  Future<void> _showHintDialog({
    required String title,
    required String body,
    required VoidCallback onContinue,
    String continueLabel = 'Got it',
  }) async {
    if (!mounted) return;
    await showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(title),
        content: Text(body),
        actions: [
          TextButton(
            onPressed: () async {
              await ref.read(onboardingControllerProvider.notifier).dismiss();
              if (ctx.mounted) Navigator.of(ctx).pop();
            },
            child: const Text('Skip'),
          ),
          FilledButton(
            onPressed: () {
              onContinue();
              Navigator.of(ctx).pop();
              _maybeShowStep();
            },
            child: Text(continueLabel),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) => widget.child;
}
