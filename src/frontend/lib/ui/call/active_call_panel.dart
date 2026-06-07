import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/call_providers.dart';
import '../../state/gateway_providers.dart';
import '../../theme/voice_colors.dart';

class ActiveCallPanel extends ConsumerWidget {
  const ActiveCallPanel({super.key});

  static const Key panelKey = Key('active_call_panel');
  static const Key muteKey = Key('active_call_mute');
  static const Key speakerKey = Key('active_call_speaker');
  static const Key videoKey = Key('active_call_video');
  static const Key hangupKey = Key('active_call_hangup');
  static const Key videoPlaceholderKey = Key('active_call_video_placeholder');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (!ref.watch(gatewayConfigProvider).hasLivekitUrl) {
      return const SizedBox.shrink();
    }
    final call = ref.watch(callControllerProvider);
    final session = call.session;
    if (!call.isActive || session == null) {
      return const SizedBox.shrink();
    }

    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final connecting = call.phase == CallPhase.connecting;
    final showVideo = session.mediaKind.name == 'video';

    return Align(
      alignment: Alignment.bottomCenter,
      child: Material(
        key: panelKey,
        color: voice.elevated,
        elevation: 4,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (showVideo)
              Container(
                key: videoPlaceholderKey,
                width: double.infinity,
                height: 160,
                color: voice.muted,
                alignment: Alignment.center,
                child: Text(
                  l10n.callVideoPlaceholder,
                  style: TextStyle(color: voice.textSecondary),
                ),
              ),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
              decoration: BoxDecoration(
                border: Border(top: BorderSide(color: voice.borderDefault)),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (connecting) ...[
                    const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    ),
                    const SizedBox(width: 8),
                  ],
                  Text(
                    connecting ? l10n.callConnecting : l10n.callActive,
                    style: Theme.of(context).textTheme.labelLarge,
                  ),
                  const SizedBox(width: 16),
                  IconButton.filledTonal(
                    key: muteKey,
                    tooltip: call.isMuted ? l10n.callUnmute : l10n.callMute,
                    onPressed: () => ref
                        .read(callControllerProvider.notifier)
                        .setMuted(!call.isMuted),
                    icon: Icon(call.isMuted ? Icons.mic_off : Icons.mic),
                  ),
                  IconButton.filledTonal(
                    key: speakerKey,
                    tooltip: call.isSpeakerMuted
                        ? l10n.callSpeakerOn
                        : l10n.callSpeakerOff,
                    onPressed: () => ref
                        .read(callControllerProvider.notifier)
                        .setSpeakerMuted(!call.isSpeakerMuted),
                    icon: Icon(
                      call.isSpeakerMuted ? Icons.volume_off : Icons.volume_up,
                    ),
                  ),
                  IconButton.filledTonal(
                    key: videoKey,
                    tooltip: call.isVideoEnabled
                        ? l10n.callVideoOff
                        : l10n.callVideoOn,
                    onPressed: () => ref
                        .read(callControllerProvider.notifier)
                        .setVideoEnabled(!call.isVideoEnabled),
                    icon: Icon(
                      call.isVideoEnabled ? Icons.videocam : Icons.videocam_off,
                    ),
                  ),
                  const SizedBox(width: 8),
                  IconButton.filled(
                    key: hangupKey,
                    tooltip: l10n.callHangup,
                    style: IconButton.styleFrom(backgroundColor: voice.error),
                    onPressed: () =>
                        ref.read(callControllerProvider.notifier).hangUp(),
                    icon: const Icon(Icons.call_end),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
