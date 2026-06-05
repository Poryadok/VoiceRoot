import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/call_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';

class OutgoingCallOverlay extends ConsumerWidget {
  const OutgoingCallOverlay({super.key});

  static const Key overlayKey = Key('outgoing_call_overlay');
  static const Key cancelKey = Key('outgoing_call_cancel');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (!ref.watch(gatewayConfigProvider).hasLivekitUrl) {
      return const SizedBox.shrink();
    }
    final call = ref.watch(callControllerProvider);
    final session = call.session;
    if (!call.isOutgoing || session == null) {
      return const SizedBox.shrink();
    }

    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final callee = ref
        .watch(profileProvider(session.calleeProfileId))
        .valueOrNull;
    final title = callee?.displayName ?? session.calleeProfileId;

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
                l10n.callOutgoingTitle(title),
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
                  const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
                  const SizedBox(width: 12),
                  OutlinedButton.icon(
                    key: cancelKey,
                    onPressed: () =>
                        ref.read(callControllerProvider.notifier).hangUp(),
                    icon: Icon(Icons.call_end, color: voice.error),
                    label: Text(l10n.callHangup),
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
