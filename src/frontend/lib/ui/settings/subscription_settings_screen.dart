import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../backend/subscription_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/subscription_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_primary_button.dart';
import '../core/voice_secondary_button.dart';

/// Account subscription status, upgrade checkout, and billing management.
class SubscriptionSettingsScreen extends ConsumerStatefulWidget {
  const SubscriptionSettingsScreen({super.key});

  static const Key screenKey = Key('subscription_settings_screen');

  @override
  ConsumerState<SubscriptionSettingsScreen> createState() =>
      _SubscriptionSettingsScreenState();
}

class _SubscriptionSettingsScreenState
    extends ConsumerState<SubscriptionSettingsScreen> {
  var _busy = false;
  String? _error;

  Future<void> _startCheckout(String billingPeriod) async {
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;
    setState(() {
      _busy = true;
      _error = null;
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
            _error = 'invalid_checkout_url';
          });
          return;
        }
        final launched = await launchUrl(
          uri,
          mode: LaunchMode.externalApplication,
        );
        setState(() {
          _busy = false;
          if (!launched) _error = 'checkout_launch_failed';
        });
      case SubscriptionApiFailure(:final message):
        setState(() {
          _busy = false;
          _error = message;
        });
    }
  }

  Future<void> _openManageBilling(VoiceSubscription subscription) async {
    final provider = subscription.providerSubscriptionId;
    if (provider == null || provider.isEmpty) return;
    final uri = Uri.parse('https://customer-portal.paddle.com/subscriptions/$provider');
    await launchUrl(uri, mode: LaunchMode.externalApplication);
  }

  Future<void> _cancelSubscription(VoiceSubscription subscription) async {
    final session = ref.read(authControllerProvider).session;
    if (session == null || subscription.id.isEmpty) return;
    setState(() {
      _busy = true;
      _error = null;
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
          _error = message;
        });
    }
  }

  String _statusLabel(AppLocalizations l10n, VoiceSubscription? subscription) {
    final plan = subscription?.plan ?? 'free';
    final status = subscription?.status ?? 'cancelled';
    if (plan == 'premium' && status == 'active') {
      return l10n.subscriptionStatusPremium;
    }
    if (status == 'cancelled' || plan == 'free') {
      return l10n.subscriptionStatusFree;
    }
    return '$plan · $status';
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final subscriptionAsync = ref.watch(subscriptionProvider);
    final subscription = subscriptionAsync.valueOrNull;
    final isPremium = subscription?.isPremium ?? false;
    final canManage = subscription?.providerSubscriptionId?.isNotEmpty == true;

    return Scaffold(
      key: SubscriptionSettingsScreen.screenKey,
      appBar: AppBar(title: Text(l10n.subscriptionSettingsTitle)),
      body: ListView(
        padding: const EdgeInsets.all(24),
        children: [
          Text(
            l10n.subscriptionCurrentPlan,
            style: TextStyle(color: voice.textSecondary),
          ),
          const SizedBox(height: 8),
          Text(
            _statusLabel(l10n, subscription),
            style: Theme.of(context).textTheme.titleMedium,
          ),
          if (subscription?.billingPeriod.isNotEmpty == true) ...[
            const SizedBox(height: 4),
            Text(
              l10n.subscriptionBillingPeriod(subscription!.billingPeriod),
              style: TextStyle(color: voice.textSecondary),
            ),
          ],
          const SizedBox(height: 24),
          if (!isPremium) ...[
            Text(
              l10n.subscriptionUpgradeTitle,
              style: TextStyle(color: voice.textSecondary),
            ),
            const SizedBox(height: 12),
            VoicePrimaryButton(
              onPressed: _busy ? null : () => _startCheckout('monthly'),
              isLoading: _busy,
              child: Text(l10n.subscriptionUpgradeMonthly),
            ),
            const SizedBox(height: 8),
            VoiceSecondaryButton(
              onPressed: _busy ? null : () => _startCheckout('yearly'),
              child: Text(l10n.subscriptionUpgradeYearly),
            ),
          ],
          if (canManage) ...[
            const SizedBox(height: 16),
            TextButton(
              onPressed: _busy ? null : () => _openManageBilling(subscription!),
              child: Text(l10n.subscriptionManageBilling),
            ),
          ],
          if (isPremium && subscription?.id.isNotEmpty == true) ...[
            const SizedBox(height: 8),
            TextButton(
              onPressed: _busy ? null : () => _cancelSubscription(subscription!),
              child: Text(l10n.subscriptionCancel),
            ),
          ],
          if (_error != null) ...[
            const SizedBox(height: 16),
            Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
          ],
        ],
      ),
    );
  }
}
