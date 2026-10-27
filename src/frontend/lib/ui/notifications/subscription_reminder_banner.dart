import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/subscription_providers.dart';
import '../../state/subscription_reminder_hints.dart';
import '../settings/subscription_settings_screen.dart';

/// Payment failure presentation from screen-controls §5.4. The hint never
/// carries an external payment URL or grants Space management permissions.
class SubscriptionReminderBanner extends StatelessWidget {
  const SubscriptionReminderBanner({
    super.key,
    required this.hint,
    required this.onDismiss,
    this.onManage,
  });
  final SubscriptionReminderHint hint;
  final VoidCallback onDismiss;
  final VoidCallback? onManage;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return MaterialBanner(
      key: const Key('subscription_grace_reminder_banner'),
      leading: const Icon(Icons.payment),
      content: Text(
        hint.spaceId == null
            ? l10n.subscriptionGraceReminderPersonal
            : l10n.subscriptionGraceReminderSpace(hint.spaceId!),
      ),
      actions: [
        if (onManage != null)
          TextButton(
            key: const Key('subscription_grace_reminder_manage'),
            onPressed: onManage,
            child: Text(l10n.subscriptionManageBilling),
          ),
        TextButton(
          onPressed: onDismiss,
          child: Text(MaterialLocalizations.of(context).closeButtonLabel),
        ),
      ],
    );
  }
}

class SubscriptionReminderBannerHost extends ConsumerWidget {
  const SubscriptionReminderBannerHost({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final hints = ref.watch(subscriptionReminderHintsProvider);
    if (hints.isEmpty) return const SizedBox.shrink();
    final hint = hints.first;
    return SubscriptionReminderBanner(
      hint: hint,
      onDismiss: () =>
          ref.read(subscriptionReminderHintsProvider.notifier).dismiss(hint.id),
      // Current Flutter has no purchaser-authorized Space Pro billing management
      // route. Keep that accepted prerequisite explicit, never open personal billing.
      onManage: hint.spaceId != null
          ? null
          : () {
              ref.invalidate(subscriptionProvider);
              Navigator.of(context).push(
                MaterialPageRoute<void>(
                  builder: (_) => const SubscriptionSettingsScreen(),
                ),
              );
            },
    );
  }
}
