import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/guest_save_account_reminder.dart';
import '../../theme/voice_layout.dart';
import 'guest_convert_sheet.dart';
import '../onboarding/onboarding_anchor_keys.dart';

/// Non-blocking reminder for returning guests to register their account.
class GuestSaveAccountReminderBanner extends ConsumerWidget {
  const GuestSaveAccountReminderBanner({super.key});

  static const Key bannerKey = Key('guest_save_account_reminder');
  static const Key ctaKey = Key('guest_save_account_reminder_cta');
  static const Key dismissKey = Key('guest_save_account_reminder_dismiss');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authControllerProvider);
    if (!auth.isGuest || auth.needsGuestNickname) {
      return const SizedBox.shrink();
    }
    final dismissed = ref.watch(guestSaveAccountDismissedProvider);
    final visibleAsync = ref.watch(guestSaveAccountReminderVisibleProvider);
    return visibleAsync.when(
      data: (visible) {
        if (!visible || dismissed) return const SizedBox.shrink();
        final l10n = AppLocalizations.of(context)!;
        Widget reminderActionsRow() => Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextButton(
              key: ctaKey,
              onPressed: () async {
                if (!context.mounted) return;
                await GuestConvertSheet.show(context);
              },
              child: Text(l10n.guestSaveAccountReminderCta),
            ),
            const SizedBox(width: 8),
            TextButton(
              key: dismissKey,
              onPressed: () =>
                  ref.read(guestSaveAccountDismissedProvider.notifier).state =
                      true,
              // Mockup renders a compact `×` text glyph in the ghost
              // button (§2.3 #2 Dismiss). Semantics carries the localized
              // a11y name; a text glyph keeps the actions row height intact.
              child: Semantics(
                label: l10n.commonClose,
                child: const Text('\u00d7'),
              ),
            ),
          ],
        );
        return Container(
          key: OnboardingAnchorKeys.saveAccountStep,
          child: Container(
            key: bannerKey,
            width: double.infinity,
            color: Theme.of(context).colorScheme.surfaceContainerHighest,
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            child: VoiceLayout.isNarrow(MediaQuery.sizeOf(context).width)
                // V adaptive: text block above, §2.3 actions row below, so the
                // CTA and × never squeeze the reminder text into a sliver.
                ? Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(l10n.guestSaveAccountReminder),
                      const SizedBox(height: 8),
                      reminderActionsRow(),
                    ],
                  )
                : Row(
                    children: [
                      // Mockup banner: growable text block, then primary CTA +
                      // ghost × on the right (#1, #2).
                      Expanded(child: Text(l10n.guestSaveAccountReminder)),
                      reminderActionsRow(),
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
