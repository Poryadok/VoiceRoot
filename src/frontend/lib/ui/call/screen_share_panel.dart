import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:livekit_client/livekit_client.dart' as livekit;

import '../../backend/screen_share_capabilities.dart';
import '../../backend/screen_share_sources.dart';
import '../../backend/livekit_room.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../state/call_providers.dart';
import '../../state/screen_share_providers.dart';
import '../../theme/voice_colors.dart';
import '../api_error_messages.dart';
import '../core/platform_capability_hints.dart';

class ScreenSharePanel extends ConsumerWidget {
  const ScreenSharePanel({super.key});

  static const Key panelKey = Key('screen_share_panel');
  static const Key shareButtonKey = Key('active_call_screen_share');
  static const Key streamPickerKey = Key('screen_share_stream_picker');
  static const Key localPreviewKey = Key('screen_share_local_preview');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final call = ref.watch(callControllerProvider);
    final share = ref.watch(screenShareControllerProvider);
    final session = call.session;
    if (!call.isActive || session == null) {
      return const SizedBox.shrink();
    }

    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final selfId = ref.watch(authControllerProvider).activeProfileId;
    final room = ref.read(callControllerProvider.notifier).liveKitRoom;
    final selected = share.selectedStream;
    // Rebuild when LiveKit screen-share tracks are published/subscribed.
    final _ = call.mediaTracksVersion;

    livekit.VideoTrack? track;
    final isLocalSelf =
        share.isSharing &&
        selected != null &&
        (selected.streamId == share.localStreamId ||
            (selfId != null && selected.profileId == selfId));

    if (room != null && selected != null) {
      if (isLocalSelf) {
        final isMatchSquad =
            session.matchId != null && session.mediaEpoch != null;
        if (!isMatchSquad ||
            (selfId != null &&
                room is MatchSquadVoiceLiveKitRoom &&
                room.matchesLocalParticipantIdentity(
                  selfId,
                  session.mediaEpoch,
                ))) {
          track = room.localScreenShareTrack();
        }
      } else {
        final isMatchSquad =
            session.matchId != null && session.mediaEpoch != null;
        final identity = isMatchSquad
            ? room is MatchSquadVoiceLiveKitRoom
                  ? room.resolveRemoteParticipantIdentity(selected.profileId)
                  : null
            : selected.profileId;
        if (identity != null) {
          final tracks = room.remoteScreenShareTracks(
            participantIdentity: identity,
          );
          track = tracks.isNotEmpty ? tracks.first : null;
        }
      }
    }

    final hasStreams = share.streams.isNotEmpty || share.isSharing;

    return Column(
      key: panelKey,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (hasStreams) ...[
          SizedBox(
            key: streamPickerKey,
            height: 40,
            child: ListView(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 12),
              children: [
                for (final stream in share.streams)
                  Padding(
                    padding: const EdgeInsets.only(right: 8),
                    child: ChoiceChip(
                      label: Text(
                        stream.profileId.length > 8
                            ? stream.profileId.substring(0, 8)
                            : stream.profileId,
                      ),
                      selected: share.selectedProfileId == stream.profileId,
                      onSelected: (_) => ref
                          .read(screenShareControllerProvider.notifier)
                          .selectStream(stream.profileId),
                    ),
                  ),
              ],
            ),
          ),
          Container(
            width: double.infinity,
            height: 200,
            color: voice.muted,
            child: Stack(
              alignment: Alignment.center,
              children: [
                for (final stream in share.streams)
                  if (stream.profileId != selected?.profileId)
                    KeyedSubtree(
                      key: Key('screen_share_remote_${stream.profileId}'),
                      child: const SizedBox.shrink(),
                    ),
                if (isLocalSelf)
                  KeyedSubtree(
                    key: localPreviewKey,
                    child: track != null
                        ? livekit.VideoTrackRenderer(
                            track,
                            fit: livekit.VideoViewFit.contain,
                          )
                        : const SizedBox.expand(),
                  )
                else if (selected != null)
                  KeyedSubtree(
                    key: Key('screen_share_remote_${selected.profileId}'),
                    child: track != null
                        ? livekit.VideoTrackRenderer(
                            track,
                            fit: livekit.VideoViewFit.contain,
                          )
                        : const SizedBox.shrink(),
                  )
                else
                  Text(
                    l10n.screenShareWaitingForVideo,
                    style: TextStyle(color: voice.textSecondary),
                  ),
                if (share.isPaused && share.localStreamId == selected?.streamId)
                  Icon(
                    Icons.pause_circle,
                    size: 48,
                    color: voice.textSecondary,
                  ),
              ],
            ),
          ),
        ],
        if (share.errorMessage != null)
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
            child: Text(
              share.errorMessage == 'screen_share_limit'
                  ? l10n.screenShareLimitReached
                  : share.errorMessage == 'screen_share_source_unavailable'
                  ? l10n.screenShareSourceUnavailable
                  : commonActionErrorMessage(l10n),
              style: TextStyle(color: voice.error),
            ),
          ),
      ],
    );
  }
}

class ScreenShareStartOptions {
  const ScreenShareStartOptions({
    required this.maxFrameRate,
    required this.captureSystemAudio,
  });

  final double maxFrameRate;
  final bool captureSystemAudio;
}

Future<ScreenShareStartOptions?> showScreenShareQualityDialog(
  BuildContext context,
) {
  final l10n = AppLocalizations.of(context)!;
  final webHint = platformWebVoiceLimitationsTooltip(l10n);
  var captureSystemAudio = canCaptureSystemAudioWithScreenShare;
  return showDialog<ScreenShareStartOptions>(
    context: context,
    builder: (context) => StatefulBuilder(
      builder: (context, setDialogState) => AlertDialog(
        title: Text(l10n.screenShareQualityTitle),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            ListTile(
              title: Text(l10n.screenShareQuality720p15),
              onTap: () => Navigator.pop(
                context,
                ScreenShareStartOptions(
                  maxFrameRate: 15.0,
                  captureSystemAudio: captureSystemAudio,
                ),
              ),
            ),
            ListTile(
              title: Text(l10n.screenShareQuality720p30),
              onTap: () => Navigator.pop(
                context,
                ScreenShareStartOptions(
                  maxFrameRate: 30.0,
                  captureSystemAudio: captureSystemAudio,
                ),
              ),
            ),
            SwitchListTile(
              value: captureSystemAudio,
              title: Text(l10n.screenShareSystemAudio),
              onChanged: canCaptureSystemAudioWithScreenShare
                  ? (enabled) =>
                        setDialogState(() => captureSystemAudio = enabled)
                  : null,
            ),
            if (webHint != null) ...[
              const SizedBox(height: 8),
              Text(webHint, style: Theme.of(context).textTheme.bodySmall),
            ],
          ],
        ),
      ),
    ),
  );
}

Future<String?> showScreenShareSourceDialog(
  BuildContext context,
  List<ScreenShareSource> sources,
) {
  final l10n = AppLocalizations.of(context)!;
  return showDialog<String>(
    context: context,
    builder: (context) => AlertDialog(
      title: Text(l10n.screenShareSourceTitle),
      content: SizedBox(
        width: 420,
        child: ListView(
          shrinkWrap: true,
          children: [
            for (final source in sources)
              ListTile(
                key: Key('screen_share_source_${source.id}'),
                leading: Icon(
                  source.type == ScreenShareSourceType.screen
                      ? Icons.desktop_windows
                      : Icons.web_asset,
                ),
                title: Text(source.name),
                subtitle: Text(
                  source.type == ScreenShareSourceType.screen
                      ? l10n.screenShareEntireScreen
                      : l10n.screenShareWindow,
                ),
                onTap: () => Navigator.pop(context, source.id),
              ),
          ],
        ),
      ),
    ),
  );
}

class ScreenShareCallButton extends ConsumerWidget {
  const ScreenShareCallButton({super.key, this.style});
  final ButtonStyle? style;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (!canStartScreenShare) {
      return const SizedBox.shrink();
    }
    final l10n = AppLocalizations.of(context)!;
    final share = ref.watch(screenShareControllerProvider);
    final notifier = ref.read(callControllerProvider.notifier);

    return IconButton.filledTonal(
      key: ScreenSharePanel.shareButtonKey,
      style: style,
      tooltip: share.isSharing
          ? l10n.screenShareStop
          : _screenShareStartTooltip(l10n),
      onPressed: () async {
        if (share.isSharing) {
          await notifier.stopScreenShare();
          return;
        }
        String? sourceId;
        if (canSelectDesktopScreenShareSource) {
          final sources = await ref
              .read(screenShareSourceCatalogProvider)
              .listSources();
          if (!context.mounted) return;
          if (sources.isEmpty) {
            ref
                .read(screenShareControllerProvider.notifier)
                .setError('screen_share_source_unavailable');
            return;
          }
          sourceId = await showScreenShareSourceDialog(context, sources);
          if (!context.mounted || sourceId == null) return;
        }
        final options = await showScreenShareQualityDialog(context);
        if (options == null) return;
        await notifier.startScreenShare(
          maxFrameRate: options.maxFrameRate,
          captureSystemAudio: options.captureSystemAudio,
          sourceId: sourceId,
        );
      },
      icon: Icon(
        share.isSharing ? Icons.stop_screen_share : Icons.screen_share,
      ),
    );
  }
}

class ScreenSharePauseButton extends ConsumerWidget {
  const ScreenSharePauseButton({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final share = ref.watch(screenShareControllerProvider);
    if (!share.isSharing) return const SizedBox.shrink();
    final l10n = AppLocalizations.of(context)!;
    return IconButton.filledTonal(
      tooltip: share.isPaused ? l10n.screenShareResume : l10n.screenSharePause,
      onPressed: () => ref
          .read(callControllerProvider.notifier)
          .pauseScreenShare(!share.isPaused),
      icon: Icon(share.isPaused ? Icons.play_arrow : Icons.pause),
    );
  }
}

String _screenShareStartTooltip(AppLocalizations l10n) {
  if (canCaptureSystemAudioWithScreenShare) return l10n.screenShareStart;
  return '${l10n.screenShareStart} — ${l10n.platformWebSystemAudioUnavailable}';
}
