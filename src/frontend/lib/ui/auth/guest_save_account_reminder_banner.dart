import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/guest_save_account_reminder.dart';
import 'guest_convert_sheet.dart';
import '../onboarding/onboarding_anchor_keys.dart';

/// Non-blocking reminder for returning guests to register their account.
class GuestSaveAccountReminderBanner extends ConsumerWidget {
  const GuestSaveAccountReminderBanner({super.key});

  static const Key bannerKey = Key('guest_save_account_reminder');
  static const Key ctaKey = Key('guest_save_account_reminder_cta');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authControllerProvider);
    if (!auth.isGuest || auth.needsGuestNickname) {
      return const SizedBox.shrink();
    }
    final visibleAsync = ref.watch(guestSaveAccountReminderVisibleProvider);
    return visibleAsync.when(
      data: (visible) {
        if (!visible) return const SizedBox.shrink();
        final l10n = AppLocalizations.of(context)!;
        return Container(
          key: OnboardingAnchorKeys.saveAccountStep,
          child: Container(
            key: bannerKey,
            width: double.infinity,
            color: Theme.of(context).colorScheme.surfaceContainerHighest,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            child: Row(
              children: [
                Expanded(child: Text(l10n.guestSaveAccountReminder)),
                TextButton(
                  key: ctaKey,
                  onPressed: () async {
                    if (!context.mounted) return;
                    await GuestConvertSheet.show(context);
                    final accountId = auth.session?.accountId;
                    if (accountId != null) {
                      await ref
                          .read(guestSaveAccountReminderProvider)
                          .markShown(accountId);
                    }
                  },
                  child: Text(l10n.guestSaveAccountReminderCta),
                ),
              ],
            ),
          ),
        );
      },
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
    );
  }
}
