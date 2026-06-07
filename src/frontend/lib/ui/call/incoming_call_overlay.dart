import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/call_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import 'call_modal_overlay.dart';

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

    return CallModalOverlay(
      overlayKey: overlayKey,
      title: l10n.callIncomingTitle(title),
      subtitle: session.mediaKind.name == 'video'
          ? l10n.callIncomingVideo
          : l10n.callIncomingAudio,
      avatarLabel: title,
      avatarUrl: caller?.avatarUrl,
      actions: [
        FilledButton.icon(
          key: acceptKey,
          onPressed: () => ref.read(callControllerProvider.notifier).acceptCall(),
          icon: const Icon(Icons.call),
          label: Text(l10n.callAccept),
        ),
        OutlinedButton.icon(
          key: declineKey,
          onPressed: () =>
              ref.read(callControllerProvider.notifier).declineCall(),
          icon: Icon(Icons.call_end, color: voice.error),
          label: Text(l10n.callDecline),
        ),
      ],
    );
  }
}
