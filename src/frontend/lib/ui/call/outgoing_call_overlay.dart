import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/call_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../call/call_modal_overlay.dart';

class OutgoingCallOverlay extends ConsumerWidget {
  const OutgoingCallOverlay({super.key});

  static const Key overlayKey = Key('outgoing_call_overlay');
  static const Key cancelKey = Key('outgoing_call_cancel');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (!ref.watch(gatewayConfigProvider).canPlaceVoiceCalls) {
      return const SizedBox.shrink();
    }
    final call = ref.watch(callControllerProvider);
    final session = call.session;
    if (session?.isGroupVoice == true) {
      return const SizedBox.shrink();
    }
    final calleeProfileId =
        session?.calleeProfileId ?? call.outgoingCalleeProfileId;
    if (!call.isOutgoing || calleeProfileId == null || calleeProfileId.isEmpty) {
      return const SizedBox.shrink();
    }

    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final callee = ref.watch(profileProvider(calleeProfileId)).valueOrNull;
    final title = callee?.displayName ?? calleeProfileId;

    return CallModalOverlay(
      overlayKey: overlayKey,
      title: l10n.callOutgoingTitle(title),
      subtitle: session?.mediaKind.name == 'video'
          ? l10n.callIncomingVideo
          : l10n.callIncomingAudio,
      avatarLabel: title,
      avatarUrl: callee?.avatarUrl,
      actions: [
        OutlinedButton(
          onPressed: () => ref
              .read(callControllerProvider.notifier)
              .setMuted(!call.isMuted),
          child: Text(call.isMuted ? l10n.callUnmute : l10n.callMute),
        ),
        OutlinedButton(
          key: cancelKey,
          onPressed: () => ref.read(callControllerProvider.notifier).hangUp(),
          style: OutlinedButton.styleFrom(
            foregroundColor: voice.error,
            side: BorderSide(color: voice.error),
          ),
          child: Text(l10n.callHangup),
        ),
      ],
    );
  }
}
