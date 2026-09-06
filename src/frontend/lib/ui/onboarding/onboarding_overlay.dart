import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/onboarding_controller.dart';
import '../../state/shell_providers.dart';
import '../../state/social_providers.dart';
import '../profile/profile_edit_sheet.dart';
import 'onboarding_anchor_keys.dart';
import 'onboarding_coach_mark.dart';

/// Contextual onboarding hints (docs/features/onboarding.md).
class OnboardingOverlay extends ConsumerStatefulWidget {
  const OnboardingOverlay({super.key, required this.child});

  final Widget child;

  @override
  ConsumerState<OnboardingOverlay> createState() => _OnboardingOverlayState();
}

class _OnboardingOverlayState extends ConsumerState<OnboardingOverlay> {
  var _loaded = false;
  var _saveAccountModalOpen = false;
  OverlayEntry? _coachMark;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadOnboarding());
  }

  @override
  void dispose() {
    _clearCoachMark();
    super.dispose();
  }

  Future<void> _loadOnboarding() async {
    if (_loaded) return;
    _loaded = true;
    await ref.read(onboardingControllerProvider.notifier).load();
    if (!mounted) return;
    _maybeShowStep();
  }

  void _clearCoachMark() {
    final entry = _coachMark;
    _coachMark = null;
    if (entry != null && entry.mounted) {
      entry.remove();
    }
  }

  Future<void> _maybeShowStep() async {
    final onboarding = ref.read(onboardingControllerProvider);
    if (!onboarding.shouldShowHints) return;
    final step = onboarding.currentStep;
    if (step == null) return;

    final l10n = AppLocalizations.of(context)!;

    if (step == OnboardingStep.saveAccount &&
        ref.read(authControllerProvider).isGuest) {
      await _completeCurrentStepAndShowNext(retryOnFailure: false);
      return;
    }

    switch (step) {
      case OnboardingStep.saveAccount:
        _showSaveAccountModal(l10n);
      case OnboardingStep.chatsNav:
        _showCoachMark(
          anchorKey: OnboardingAnchorKeys.chatsNav,
          title: l10n.onboardingChatsNavTitle,
          body: l10n.onboardingChatsNavBody,
          continueLabel: l10n.onboardingGotIt,
          onContinue: _completeCurrentStepAndShowNext,
        );
      case OnboardingStep.spaces:
        _showCoachMark(
          anchorKey: OnboardingAnchorKeys.spaces,
          title: l10n.onboardingSpacesTitle,
          body: l10n.onboardingSpacesBody,
          continueLabel: l10n.onboardingLater,
          secondaryLabel: l10n.onboardingSpacesFind,
          onSecondary: () async {
            ref.read(shellNavigationProvider).setNavigationSection(
              NavigationSection.chats,
            );
            ref.read(globalSearchFocusRequestProvider.notifier).state++;
            await _completeCurrentStepAndShowNext();
          },
          onContinue: _completeCurrentStepAndShowNext,
        );
      case OnboardingStep.matchmaking:
        ref.read(shellNavigationProvider).setNavigationSection(
          NavigationSection.social,
        );
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) return;
          _showCoachMark(
            anchorKey: OnboardingAnchorKeys.matchmaking,
            title: l10n.onboardingMatchmakingTitle,
            body: l10n.onboardingMatchmakingBody,
            continueLabel: l10n.onboardingLater,
            secondaryLabel: l10n.onboardingMatchmakingTry,
            onSecondary: _completeCurrentStepAndShowNext,
            onContinue: _completeCurrentStepAndShowNext,
          );
        });
      case OnboardingStep.wrapUp:
        _showHintDialog(
          title: l10n.onboardingWrapUpTitle,
          body: l10n.onboardingWrapUpBody,
          onContinue: _completeCurrentStepAndShowNext,
          continueLabel: l10n.onboardingWrapUpStart,
        );
    }
  }

  Future<void> _completeCurrentStepAndShowNext({
    bool retryOnFailure = true,
  }) async {
    final currentStep = ref.read(onboardingControllerProvider).currentStep;
    if (currentStep == null) return;

    await ref.read(onboardingControllerProvider.notifier).completeCurrentStep();
    if (!mounted) return;

    final nextStep = ref.read(onboardingControllerProvider).currentStep;
    if (!retryOnFailure && nextStep == currentStep) return;
    await _maybeShowStep();
  }

  void _showCoachMark({
    required GlobalKey anchorKey,
    required String title,
    required String body,
    required Future<void> Function() onContinue,
    String? continueLabel,
    String? secondaryLabel,
    Future<void> Function()? onSecondary,
  }) {
    final l10n = AppLocalizations.of(context)!;
    _clearCoachMark();
    if (!mounted) return;
    _coachMark = OnboardingCoachMark.show(
      context: context,
      anchorKey: anchorKey,
      title: title,
      body: body,
      onContinue: () async {
        _clearCoachMark();
        await onContinue();
      },
      onSkip: () {
        _clearCoachMark();
        ref.read(onboardingControllerProvider.notifier).dismiss();
      },
      continueLabel: continueLabel ?? l10n.onboardingGotIt,
      skipLabel: l10n.onboardingSkip,
      secondaryLabel: secondaryLabel,
      onSecondary: onSecondary == null
          ? null
          : () async {
              _clearCoachMark();
              await onSecondary();
            },
    );
  }

  Future<void> _showSaveAccountModal(AppLocalizations l10n) async {
    if (_saveAccountModalOpen) return;
    final profile = ref.read(activeProfileProvider).valueOrNull;
    if (profile == null) return;
    if (!mounted) return;
    _saveAccountModalOpen = true;
    try {
      await showDialog<void>(
        context: context,
        barrierDismissible: false,
        builder: (ctx) => AlertDialog(
          title: Text(l10n.onboardingSaveAccountTitle),
          content: Text(l10n.onboardingSaveAccountBody),
          actions: [
            TextButton(
              onPressed: () async {
                await ref.read(onboardingControllerProvider.notifier).dismiss();
                if (ctx.mounted) Navigator.of(ctx).pop();
              },
              child: Text(l10n.onboardingSkip),
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
              child: Text(l10n.commonSave),
            ),
          ],
        ),
      );
    } finally {
      _saveAccountModalOpen = false;
    }
    if (mounted) await _maybeShowStep();
  }

  Future<void> _showHintDialog({
    required String title,
    required String body,
    required Future<void> Function() onContinue,
    required String continueLabel,
  }) async {
    final l10n = AppLocalizations.of(context)!;
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
            child: Text(l10n.onboardingSkip),
          ),
          FilledButton(
          onPressed: () async {
              await onContinue();
              if (!ctx.mounted) return;
              Navigator.of(ctx).pop();
            },
            child: Text(continueLabel),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(activeProfileProvider, (previous, next) {
      if (next.valueOrNull == null) return;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        _maybeShowStep();
      });
    });
    return widget.child;
  }
}
