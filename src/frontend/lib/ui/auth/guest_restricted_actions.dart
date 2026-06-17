import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';

/// Defense-in-depth: guest-restricted primary actions shown disabled in shell.
class GuestRestrictedActions extends ConsumerWidget {
  const GuestRestrictedActions({super.key});

  static const Key dmComposeKey = Key('dm_compose_button');
  static const Key startCallKey = Key('start_call_button');
  static const Key friendInviteKey = Key('friend_invite_button');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isGuest = ref.watch(authControllerProvider).isGuest;
    if (!isGuest) return const SizedBox.shrink();

    final l10n = AppLocalizations.of(context)!;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      child: Wrap(
        spacing: 8,
        runSpacing: 4,
        alignment: WrapAlignment.end,
        children: [
          ElevatedButton(
            key: dmComposeKey,
            onPressed: null,
            child: Text(l10n.profileMessage),
          ),
          ElevatedButton(
            key: startCallKey,
            onPressed: null,
            child: Text(l10n.callStartAudio),
          ),
          ElevatedButton(
            key: friendInviteKey,
            onPressed: null,
            child: Text(l10n.socialAddFriend),
          ),
        ],
      ),
    );
  }
}
