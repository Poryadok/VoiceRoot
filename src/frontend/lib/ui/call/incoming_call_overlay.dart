import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/call_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';

class IncomingCallOverlay extends ConsumerWidget {
  const IncomingCallOverlay({super.key});

  static const Key overlayKey = Key('incoming_call_overlay');
  static const Key acceptKey = Key('incoming_call_accept');
  static const Key declineKey = Key('incoming_call_decline');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (!ref.watch(gatewayConfigProvider).hasLivekitUrl) {
      return const SizedBox.shrink();
    }
    final call = ref.watch(callControllerProvider);
    final session = call.session;
    if (!call.isIncoming || session == null) {
      return const SizedBox.shrink();
    }

    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final caller = ref
        .watch(profileProvider(session.initiatorProfileId))
        .valueOrNull;
    final title = caller?.displayName ?? session.initiatorProfileId;

    return Positioned(
      key: overlayKey,
      top: 16,
      right: 16,
      child: Material(
        color: voice.elevated,
        borderRadius: BorderRadius.circular(8),
        elevation: 6,
        child: Container(
          width: 320,
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            border: Border.all(color: voice.borderDefault),
            borderRadius: BorderRadius.circular(8),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                l10n.callIncomingTitle(title),
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 4),
              Text(
                session.mediaKind.name == 'video'
                    ? l10n.callIncomingVideo
                    : l10n.callIncomingAudio,
                style: TextStyle(color: voice.textSecondary),
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  FilledButton.icon(
                    key: acceptKey,
                    onPressed: () =>
                        ref.read(callControllerProvider.notifier).acceptCall(),
                    icon: const Icon(Icons.call),
                    label: Text(l10n.callAccept),
                  ),
                  const SizedBox(width: 8),
                  OutlinedButton.icon(
                    key: declineKey,
                    onPressed: () =>
                        ref.read(callControllerProvider.notifier).declineCall(),
                    icon: Icon(Icons.call_end, color: voice.error),
                    label: Text(l10n.callDecline),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
