import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../backend/subscription_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_primary_button.dart';
import '../core/voice_secondary_button.dart';
import '../core/voice_state_panel.dart';

enum _SubscriptionPlanDisplay {
  free,
  premiumActive,
  gracePeriod,
  premiumUntilEnd,
}

/// Localized plan title for settings entry points and plan cards.
String subscriptionPlanLabel(
  AppLocalizations l10n,
  VoiceSubscription? subscription,
) {
  return switch (_resolveSubscriptionPlanDisplay(subscription)) {
    _SubscriptionPlanDisplay.free => l10n.subscriptionStatusFree,
    _SubscriptionPlanDisplay.premiumActive ||
    _SubscriptionPlanDisplay.gracePeriod ||
    _SubscriptionPlanDisplay.premiumUntilEnd =>
      l10n.subscriptionStatusPremium,
  };
}

_SubscriptionPlanDisplay _resolveSubscriptionPlanDisplay(
  VoiceSubscription? subscription,
) {
  if (subscription == null || subscription.plan != 'premium') {
    return _SubscriptionPlanDisplay.free;
  }
  if (subscription.status == 'grace_period') {
    return _SubscriptionPlanDisplay.gracePeriod;
  }
  if (subscription.status == 'active') {
    return _SubscriptionPlanDisplay.premiumActive;
  }
  final periodEnd = subscription.currentPeriodEnd;
  if (periodEnd != null && periodEnd.isAfter(DateTime.now())) {
    return _SubscriptionPlanDisplay.premiumUntilEnd;
  }
  return _SubscriptionPlanDisplay.free;
}

String subscriptionStatusHeadline(
  AppLocalizations l10n,
  VoiceSubscription? subscription,
) {
  final display = _resolveSubscriptionPlanDisplay(subscription);
  final periodEnd = subscription?.currentPeriodEnd;
  final formattedEnd = periodEnd != null
      ? DateFormat.yMMMd(l10n.localeName).format(periodEnd.toLocal())
      : null;

  return switch (display) {
    _SubscriptionPlanDisplay.free => l10n.subscriptionStatusFree,
    _SubscriptionPlanDisplay.premiumActive => l10n.subscriptionStatusPremium,
    _SubscriptionPlanDisplay.gracePeriod => l10n.subscriptionStatusGracePeriod,
    _SubscriptionPlanDisplay.premiumUntilEnd when formattedEnd != null =>
      l10n.subscriptionStatusPremiumUntil(formattedEnd),
    _SubscriptionPlanDisplay.premiumUntilEnd => l10n.subscriptionStatusPremium,
  };
}

String? subscriptionStatusHint(
  AppLocalizations l10n,
  VoiceSubscription? subscription,
) {
  return switch (_resolveSubscriptionPlanDisplay(subscription)) {
    _SubscriptionPlanDisplay.gracePeriod => l10n.subscriptionGracePeriodHint,
    _SubscriptionPlanDisplay.premiumUntilEnd =>
      l10n.subscriptionPremiumUntilHint,
    _ => null,
  };
}

String subscriptionBillingPeriodLabel(
  AppLocalizations l10n,
  String billingPeriod,
) {
  return switch (billingPeriod) {
    'yearly' => l10n.subscriptionBillingPeriodYearly,
    'monthly' => l10n.subscriptionBillingPeriodMonthly,
    _ => billingPeriod,
  };
}

/// Account subscription status, upgrade checkout, and billing management.
class SubscriptionSettingsScreen extends ConsumerStatefulWidget {
  const SubscriptionSettingsScreen({super.key});

  static const Key screenKey = Key('subscription_settings_screen');
  static const Key planStateKey = Key('subscription_plan_state');
  static const Key freeTierNoteKey = Key('subscription_free_tier_note');
  static const Key upgradeSectionKey = Key('subscription_upgrade_section');

  @override
  ConsumerState<SubscriptionSettingsScreen> createState() =>
      _SubscriptionSettingsScreenState();
}

class _SubscriptionSettingsScreenState
    extends ConsumerState<SubscriptionSettingsScreen> {
  var _busy = false;
  String? _errorKey;

  Future<void> _startCheckout(String billingPeriod) async {
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;
    setState(() {
      _busy = true;
      _errorKey = null;
    });
    final origin = Uri.base.origin;
    final result = await ref
        .read(voiceSubscriptionClientProvider)
        .createCheckoutSession(
          authorization: session.authorizationHeader,
          plan: 'premium',
          billingPeriod: billingPeriod,
          successUrl: '$origin/subscription/success',
          cancelUrl: '$origin/subscription/cancel',
        );
    if (!mounted) return;
    switch (result) {
      case SubscriptionApiOk(:final data):
        final uri = Uri.tryParse(data.checkoutUrl);
        if (uri == null) {
          setState(() {
            _busy = false;
            _errorKey = 'invalid_checkout_url';
          });
          return;
        }
        final launched = await launchUrl(
          uri,
          mode: LaunchMode.externalApplication,
        );
        setState(() {
          _busy = false;
          if (!launched) _errorKey = 'checkout_launch_failed';
        });
      case SubscriptionApiFailure(:final message):
        setState(() {
          _busy = false;
          _errorKey = message;
        });
    }
  }

  Future<void> _openManageBilling(VoiceSubscription subscription) async {
    final provider = subscription.providerSubscriptionId;
    if (provider == null || provider.isEmpty) return;
    final uri = Uri.parse(
      'https://customer-portal.paddle.com/subscriptions/$provider',
    );
    await launchUrl(uri, mode: LaunchMode.externalApplication);
  }

  Future<void> _cancelSubscription(VoiceSubscription subscription) async {
    final session = ref.read(authControllerProvider).session;
    if (session == null || subscription.id.isEmpty) return;
    setState(() {
      _busy = true;
      _errorKey = null;
    });
    final result = await ref
        .read(voiceSubscriptionClientProvider)
        .cancelSubscription(
          authorization: session.authorizationHeader,
          subscriptionId: subscription.id,
        );
    if (!mounted) return;
    switch (result) {
      case SubscriptionApiOk():
        ref.invalidate(subscriptionProvider);
        setState(() => _busy = false);
      case SubscriptionApiFailure(:final message):
        setState(() {
          _busy = false;
          _errorKey = message;
        });
    }
  }

  String _checkoutErrorMessage(AppLocalizations l10n, String errorKey) {
    return switch (errorKey) {
      'invalid_checkout_url' => l10n.subscriptionInvalidCheckoutUrl,
      'checkout_launch_failed' => l10n.subscriptionCheckoutLaunchFailed,
      _ => errorKey,
    };
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final subscriptionAsync = ref.watch(subscriptionProvider);

    return Scaffold(
      key: SubscriptionSettingsScreen.screenKey,
      appBar: AppBar(title: Text(l10n.subscriptionSettingsTitle)),
      body: subscriptionAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (_, _) => VoiceStatePanel(
          title: l10n.subscriptionLoadError,
          icon: Icons.cloud_off_outlined,
          actionLabel: l10n.subscriptionRetry,
          onAction: () => ref.invalidate(subscriptionProvider),
        ),
        data: (subscription) => _SubscriptionBody(
          l10n: l10n,
          voice: voice,
          subscription: subscription,
          busy: _busy,
          errorKey: _errorKey,
          checkoutErrorMessage: _checkoutErrorMessage,
          onUpgradeMonthly: () => _startCheckout('monthly'),
          onUpgradeYearly: () => _startCheckout('yearly'),
          onManageBilling: subscription == null
              ? null
              : () => _openManageBilling(subscription),
          onCancel: subscription == null
              ? null
              : () => _cancelSubscription(subscription),
        ),
      ),
    );
  }
}

class _SubscriptionBody extends StatelessWidget {
  const _SubscriptionBody({
    required this.l10n,
    required this.voice,
    required this.subscription,
    required this.busy,
    required this.errorKey,
    required this.checkoutErrorMessage,
    required this.onUpgradeMonthly,
    required this.onUpgradeYearly,
    this.onManageBilling,
    this.onCancel,
  });

  final AppLocalizations l10n;
  final VoiceColors voice;
  final VoiceSubscription? subscription;
  final bool busy;
  final String? errorKey;
  final String Function(AppLocalizations l10n, String errorKey)
  checkoutErrorMessage;
  final VoidCallback onUpgradeMonthly;
  final VoidCallback onUpgradeYearly;
  final VoidCallback? onManageBilling;
  final VoidCallback? onCancel;

  @override
  Widget build(BuildContext context) {
    final display = _resolveSubscriptionPlanDisplay(subscription);
    final isPremium = subscription?.isPremium ?? false;
    final showUpsell = display == _SubscriptionPlanDisplay.free;
    final canManage =
        subscription?.providerSubscriptionId?.isNotEmpty == true;
    final statusHint = subscriptionStatusHint(l10n, subscription);

    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        _PlanStateCard(
          key: SubscriptionSettingsScreen.planStateKey,
          l10n: l10n,
          voice: voice,
          subscription: subscription,
          display: display,
        ),
        if (statusHint != null) ...[
          const SizedBox(height: 12),
          Text(
            statusHint,
            style: TextStyle(color: voice.textSecondary),
          ),
        ],
        if (subscription?.billingPeriod.isNotEmpty == true &&
            display != _SubscriptionPlanDisplay.free) ...[
          const SizedBox(height: 8),
          Text(
            l10n.subscriptionBillingPeriod(
              subscriptionBillingPeriodLabel(
                l10n,
                subscription!.billingPeriod,
              ),
            ),
            style: TextStyle(color: voice.textSecondary),
          ),
        ],
        if (showUpsell) ...[
          const SizedBox(height: 24),
          Text(
            l10n.subscriptionFreeTierNote,
            key: SubscriptionSettingsScreen.freeTierNoteKey,
            style: TextStyle(color: voice.textSecondary),
          ),
          const SizedBox(height: 24),
          _UpgradeSection(
            key: SubscriptionSettingsScreen.upgradeSectionKey,
            l10n: l10n,
            voice: voice,
            busy: busy,
            onUpgradeMonthly: onUpgradeMonthly,
            onUpgradeYearly: onUpgradeYearly,
          ),
        ],
        if (canManage) ...[
          const SizedBox(height: 16),
          TextButton(
            onPressed: busy ? null : onManageBilling,
            child: Text(l10n.subscriptionManageBilling),
          ),
        ],
        if (isPremium && subscription?.id.isNotEmpty == true) ...[
          const SizedBox(height: 8),
          TextButton(
            onPressed: busy ? null : onCancel,
            child: Text(l10n.subscriptionCancel),
          ),
        ],
        if (errorKey != null) ...[
          const SizedBox(height: 16),
          Text(
            checkoutErrorMessage(l10n, errorKey!),
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
        ],
      ],
    );
  }
}

class _PlanStateCard extends StatelessWidget {
  const _PlanStateCard({
    super.key,
    required this.l10n,
    required this.voice,
    required this.subscription,
    required this.display,
  });

  final AppLocalizations l10n;
  final VoiceColors voice;
  final VoiceSubscription? subscription;
  final _SubscriptionPlanDisplay display;

  @override
  Widget build(BuildContext context) {
    final headline = subscriptionStatusHeadline(l10n, subscription);
    final isPremiumDisplay = display != _SubscriptionPlanDisplay.free;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: voice.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: voice.borderDefault),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            l10n.subscriptionCurrentPlan,
            style: TextStyle(color: voice.textSecondary),
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: Text(
                  headline,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
              ),
              _PlanStatusChip(
                voice: voice,
                label: subscriptionPlanLabel(l10n, subscription),
                isPremium: isPremiumDisplay,
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _PlanStatusChip extends StatelessWidget {
  const _PlanStatusChip({
    required this.voice,
    required this.label,
    required this.isPremium,
  });

  final VoiceColors voice;
  final String label;
  final bool isPremium;

  @override
  Widget build(BuildContext context) {
    final accent = voice.profileAccent;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: isPremium ? accent.withValues(alpha: 0.15) : voice.muted,
        borderRadius: BorderRadius.circular(999),
        border: Border.all(
          color: isPremium ? accent.withValues(alpha: 0.45) : voice.borderDefault,
        ),
      ),
      child: Text(
        label,
        style: Theme.of(context).textTheme.labelMedium?.copyWith(
          color: isPremium ? accent : voice.textSecondary,
        ),
      ),
    );
  }
}

class _UpgradeSection extends StatelessWidget {
  const _UpgradeSection({
    super.key,
    required this.l10n,
    required this.voice,
    required this.busy,
    required this.onUpgradeMonthly,
    required this.onUpgradeYearly,
  });

  final AppLocalizations l10n;
  final VoiceColors voice;
  final bool busy;
  final VoidCallback onUpgradeMonthly;
  final VoidCallback onUpgradeYearly;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.subscriptionUpgradeTitle,
          style: Theme.of(context).textTheme.titleSmall,
        ),
        const SizedBox(height: 8),
        Text(
          l10n.subscriptionUpgradeSubtitle,
          style: TextStyle(color: voice.textSecondary),
        ),
        const SizedBox(height: 12),
        for (final benefit in [
          l10n.subscriptionBenefitBadge,
          l10n.subscriptionBenefitUploads,
          l10n.subscriptionBenefitProfiles,
        ])
          Padding(
            padding: const EdgeInsets.only(bottom: 6),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(Icons.check, size: 18, color: voice.textSecondary),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    benefit,
                    style: TextStyle(color: voice.textSecondary),
                  ),
                ),
              ],
            ),
          ),
        const SizedBox(height: 16),
        VoicePrimaryButton(
          onPressed: busy ? null : onUpgradeMonthly,
          isLoading: busy,
          child: Text(l10n.subscriptionUpgradeMonthly),
        ),
        const SizedBox(height: 8),
        VoiceSecondaryButton(
          onPressed: busy ? null : onUpgradeYearly,
          child: Text(l10n.subscriptionUpgradeYearly),
        ),
      ],
    );
  }
}
