import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:livekit_client/livekit_client.dart' as livekit;

import '../../backend/voice_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/call_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/screen_share_providers.dart';
import '../../theme/voice_colors.dart';
import 'screen_share_panel.dart';

class ActiveCallPanel extends ConsumerStatefulWidget {
  const ActiveCallPanel({super.key});

  static const Key panelKey = Key('active_call_panel');
  static const Key unlockAudioKey = Key('active_call_unlock_audio');
  static const Key muteKey = Key('active_call_mute');
  static const Key speakerKey = Key('active_call_speaker');
  static const Key videoKey = Key('active_call_video');
  static const Key hangupKey = Key('active_call_hangup');
  static const Key videoPlaceholderKey = Key('active_call_video_placeholder');
  static const Key fullscreenKey = Key('active_call_fullscreen');
  static const Key minimizeBarKey = Key('active_call_minimize_bar');
  static const Key minimizeKey = Key('active_call_minimize');

  @override
  ConsumerState<ActiveCallPanel> createState() => _ActiveCallPanelState();
}

class _ActiveCallPanelState extends ConsumerState<ActiveCallPanel> {
  var _minimized = false;

  @override
  Widget build(BuildContext context) {
    if (!ref.watch(gatewayConfigProvider).canPlaceVoiceCalls) {
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
    final share = ref.watch(screenShareControllerProvider);
    final hasScreenShare = share.isSharing || share.streams.isNotEmpty;
    final isVideoCall =
        session.mediaKind == VoiceCallMediaKind.video && call.isVideoEnabled;
    final useExpandedMedia = isVideoCall || hasScreenShare;
    // Rebuild when LiveKit camera tracks are published/subscribed.
    final _ = call.mediaTracksVersion;

    final controls = _CallControlsRow(
      connecting: connecting,
      l10n: l10n,
      session: session,
      call: call,
      showMinimize: useExpandedMedia && !_minimized,
      onMinimize: useExpandedMedia ? () => setState(() => _minimized = true) : null,
      onExpand: useExpandedMedia && _minimized
          ? () => setState(() => _minimized = false)
          : null,
    );

    if (useExpandedMedia && !_minimized) {
      return SizedBox.expand(
        child: Material(
          key: ActiveCallPanel.panelKey,
          color: voice.elevated,
          child: Stack(
            fit: StackFit.expand,
            children: [
              KeyedSubtree(
                key: ActiveCallPanel.fullscreenKey,
                child: _CallVideoPreview(
                  key: ActiveCallPanel.videoPlaceholderKey,
                  isVideoEnabled: call.isVideoEnabled,
                  placeholder: l10n.callVideoPlaceholder,
                  mutedColor: voice.muted,
                  textColor: voice.textSecondary,
                  expanded: true,
                ),
              ),
              const Align(
                alignment: Alignment.topCenter,
                child: ScreenSharePanel(),
              ),
              if (call.needsAudioPlaybackUnlock)
                Align(
                  alignment: Alignment.topCenter,
                  child: Material(
                    key: ActiveCallPanel.unlockAudioKey,
                    color: voice.focusRing.withValues(alpha: 0.12),
                    child: InkWell(
                      onTap: () => ref
                          .read(callControllerProvider.notifier)
                          .unlockAudioPlayback(),
                      child: Padding(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 16,
                          vertical: 8,
                        ),
                        child: Row(
                          children: [
                            Icon(Icons.volume_up, color: voice.focusRing, size: 18),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Text(
                                l10n.callTapToEnableAudio,
                                style: Theme.of(context).textTheme.bodySmall
                                    ?.copyWith(color: voice.textSecondary),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
              Align(
                alignment: Alignment.bottomCenter,
                child: Material(
                  key: ActiveCallPanel.minimizeBarKey,
                  color: voice.elevated,
                  elevation: 4,
                  child: Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 12,
                    ),
                    decoration: BoxDecoration(
                      border: Border(top: BorderSide(color: voice.borderDefault)),
                    ),
                    child: controls,
                  ),
                ),
              ),
            ],
          ),
        ),
      );
    }

    if (useExpandedMedia && _minimized) {
      return Align(
        alignment: Alignment.bottomCenter,
        child: Material(
          key: ActiveCallPanel.minimizeBarKey,
          color: voice.elevated,
          elevation: 4,
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            decoration: BoxDecoration(
              border: Border(top: BorderSide(color: voice.borderDefault)),
            ),
            child: controls,
          ),
        ),
      );
    }

    return Align(
      alignment: Alignment.bottomCenter,
      child: Material(
        key: ActiveCallPanel.panelKey,
        color: voice.elevated,
        elevation: 4,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const ScreenSharePanel(),
            if (call.needsAudioPlaybackUnlock)
              Material(
                key: ActiveCallPanel.unlockAudioKey,
                color: voice.focusRing.withValues(alpha: 0.12),
                child: InkWell(
                  onTap: () => ref
                      .read(callControllerProvider.notifier)
                      .unlockAudioPlayback(),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 8,
                    ),
                    child: Row(
                      children: [
                        Icon(Icons.volume_up, color: voice.focusRing, size: 18),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            l10n.callTapToEnableAudio,
                            style: Theme.of(context).textTheme.bodySmall
                                ?.copyWith(color: voice.textSecondary),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
              decoration: BoxDecoration(
                border: Border(top: BorderSide(color: voice.borderDefault)),
              ),
              child: controls,
            ),
          ],
        ),
      ),
    );
  }
}

class _CallControlsRow extends ConsumerWidget {
  const _CallControlsRow({
    required this.connecting,
    required this.l10n,
    required this.session,
    required this.call,
    this.showMinimize = false,
    this.onMinimize,
    this.onExpand,
  });

  final bool connecting;
  final AppLocalizations l10n;
  final VoiceCallSession session;
  final CallState call;
  final bool showMinimize;
  final VoidCallback? onMinimize;
  final VoidCallback? onExpand;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Row(
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
          connecting
              ? l10n.callConnecting
              : session.isGroupVoice
              ? l10n.callGroupVoiceActive
              : l10n.callActive,
          style: Theme.of(context).textTheme.labelLarge,
        ),
        const SizedBox(width: 16),
        if (onExpand != null)
          IconButton.filledTonal(
            key: ActiveCallPanel.minimizeKey,
            tooltip: l10n.callVideoOn,
            onPressed: onExpand,
            icon: const Icon(Icons.open_in_full),
          ),
        IconButton.filledTonal(
          key: ActiveCallPanel.muteKey,
          tooltip: call.isMuted ? l10n.callUnmute : l10n.callMute,
          onPressed: () => ref
              .read(callControllerProvider.notifier)
              .setMuted(!call.isMuted),
          icon: Icon(call.isMuted ? Icons.mic_off : Icons.mic),
        ),
        IconButton.filledTonal(
          key: ActiveCallPanel.speakerKey,
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
          key: ActiveCallPanel.videoKey,
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
        const ScreenShareCallButton(),
        const ScreenSharePauseButton(),
        if (showMinimize && onMinimize != null) ...[
          IconButton.filledTonal(
            key: ActiveCallPanel.minimizeKey,
            tooltip: 'Minimize',
            onPressed: onMinimize,
            icon: const Icon(Icons.close_fullscreen),
          ),
        ],
        const SizedBox(width: 8),
        IconButton.filled(
          key: ActiveCallPanel.hangupKey,
          tooltip: l10n.callHangup,
          style: IconButton.styleFrom(
            backgroundColor: VoiceColors.of(context).error,
          ),
          onPressed: () => ref.read(callControllerProvider.notifier).hangUp(),
          icon: const Icon(Icons.call_end),
        ),
      ],
    );
  }
}

class _CallVideoPreview extends ConsumerWidget {
  const _CallVideoPreview({
    required super.key,
    required this.isVideoEnabled,
    required this.placeholder,
    required this.mutedColor,
    required this.textColor,
    this.expanded = false,
  });

  final bool isVideoEnabled;
  final String placeholder;
  final Color mutedColor;
  final Color textColor;
  final bool expanded;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final room = ref.read(callControllerProvider.notifier).liveKitRoom;
    livekit.VideoTrack? track;
    if (room != null && isVideoEnabled) {
      track = room.remoteCameraTrack() ?? room.localCameraTrack();
    }

    return Container(
      width: double.infinity,
      height: expanded ? double.infinity : 160,
      color: mutedColor,
      alignment: Alignment.center,
      child: track != null
          ? livekit.VideoTrackRenderer(
              track,
              fit: livekit.VideoViewFit.cover,
            )
          : Text(
              placeholder,
              style: TextStyle(color: textColor),
            ),
    );
  }
}
