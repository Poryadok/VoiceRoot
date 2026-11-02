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
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';
import '../api_error_messages.dart';
import '../core/platform_capability_hints.dart';
import 'screen_share_picker.dart';
import 'stream_viewer_picker.dart';

class ScreenSharePanel extends ConsumerStatefulWidget {
  const ScreenSharePanel({super.key, this.bottomInset = 96});

  /// Leaves room for the active call's overlaid controls. Standalone hosts can
  /// set this to zero when their controls occupy a separate layout slot.
  final double bottomInset;

  static const Key panelKey = Key('screen_share_panel');
  static const Key shareButtonKey = Key('active_call_screen_share');
  static const Key streamPickerKey = Key('screen_share_stream_picker');
  static const Key localPreviewKey = Key('screen_share_local_preview');

  @override
  ConsumerState<ScreenSharePanel> createState() => _ScreenSharePanelState();
}

class _ScreenSharePanelState extends ConsumerState<ScreenSharePanel> {
  bool _viewerClosed = false;
  String? _closedRoomId;

  @override
  Widget build(BuildContext context) {
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

    bool isLocal(ActiveScreenShare stream) =>
        share.isSharing &&
        (stream.streamId == share.localStreamId ||
            (selfId != null && stream.profileId == selfId));
    livekit.VideoTrack? trackFor(ActiveScreenShare stream) {
      if (room == null) return null;
      if (isLocal(stream)) {
        final isMatchSquad =
            session.matchId != null && session.mediaEpoch != null;
        if (!isMatchSquad ||
            (selfId != null &&
                room is MatchSquadVoiceLiveKitRoom &&
                room.matchesLocalParticipantIdentity(
                  selfId,
                  session.mediaEpoch,
                ))) {
          return room.localScreenShareTrack();
        }
      } else {
        final isMatchSquad =
            session.matchId != null && session.mediaEpoch != null;
        final identity = isMatchSquad
            ? room is MatchSquadVoiceLiveKitRoom
                  ? room.resolveRemoteParticipantIdentity(stream.profileId)
                  : null
            : stream.profileId;
        if (identity != null) {
          final tracks = room.remoteScreenShareTracks(
            participantIdentity: identity,
          );
          return tracks.isNotEmpty ? tracks.first : null;
        }
      }
      return null;
    }

    final track = selected == null ? null : trackFor(selected);
    final isLocalSelf = selected != null && isLocal(selected);

    final hasStreams = share.streams.isNotEmpty || share.isSharing;
    final viewerClosed = _viewerClosed && _closedRoomId == session.roomId;
    final paused = (isLocalSelf && share.isPaused) || (track?.muted ?? false);
    String nameFor(String profileId) => ref
        .watch(profileProvider(profileId))
        .maybeWhen(
          data: (profile) => profile?.displayName.isNotEmpty == true
              ? profile!.displayName
              : profileId,
          orElse: () => profileId,
        );
    final names = {
      for (final stream in share.streams)
        stream.profileId: nameFor(stream.profileId),
    };
    final error = share.errorMessage == null
        ? null
        : share.errorMessage == 'screen_share_limit'
        ? l10n.screenShareLimitReached
        : share.errorMessage == 'screen_share_source_unavailable'
        ? l10n.screenShareSourceUnavailable
        : commonActionErrorMessage(l10n);
    void closeViewer() => setState(() {
      _viewerClosed = true;
      _closedRoomId = session.roomId;
    });

    if (!hasStreams || viewerClosed) {
      return Column(
        key: ScreenSharePanel.panelKey,
        mainAxisSize: MainAxisSize.min,
        children: [
          if (hasStreams)
            TextButton.icon(
              onPressed: () => setState(() => _viewerClosed = false),
              icon: const Icon(Icons.screen_share_outlined),
              label: const Text('Watch stream'),
            ),
          if (error != null) _ShareNotice(message: error),
        ],
      );
    }
    return LayoutBuilder(
      builder: (context, constraints) {
        final narrow = VoiceLayout.isNarrow(constraints.maxWidth);
        final height = constraints.hasBoundedHeight
            ? (constraints.maxHeight - widget.bottomInset).clamp(160.0, 1200.0)
            : 420.0;
        final viewer = ColoredBox(
          color: const Color(0xFF1E1E1E),
          child: Stack(
            fit: StackFit.expand,
            children: [
              if (track != null)
                KeyedSubtree(
                  key: isLocalSelf
                      ? ScreenSharePanel.localPreviewKey
                      : Key('screen_share_remote_${selected!.profileId}'),
                  child: livekit.VideoTrackRenderer(
                    track,
                    fit: livekit.VideoViewFit.contain,
                  ),
                )
              else
                Align(
                  alignment: narrow
                      ? const Alignment(0, -.65)
                      : Alignment.center,
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const Icon(
                          Icons.cast_connected,
                          color: Colors.white70,
                          size: 48,
                        ),
                        const SizedBox(height: 16),
                        Semantics(
                          liveRegion: true,
                          child: const Text(
                            'Waiting for screen video…',
                            textAlign: TextAlign.center,
                            style: TextStyle(
                              color: Colors.white,
                              fontSize: 20,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                        if (selected != null)
                          Text(
                            names[selected.profileId] ?? selected.profileId,
                            style: const TextStyle(color: Colors.white70),
                          ),
                        TextButton(
                          onPressed: closeViewer,
                          style: TextButton.styleFrom(
                            foregroundColor: Colors.white,
                          ),
                          child: const Text('Close viewer'),
                        ),
                      ],
                    ),
                  ),
                ),
              if (paused)
                Align(
                  alignment: narrow
                      ? const Alignment(0, -.65)
                      : Alignment.center,
                  child: Semantics(
                    label: 'Stream paused',
                    child: const Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.pause_circle_outline,
                          color: Colors.white,
                          size: 48,
                        ),
                        Text('Paused', style: TextStyle(color: Colors.white)),
                      ],
                    ),
                  ),
                ),
              if (selected != null)
                Positioned(
                  left: 16,
                  bottom: 16,
                  right: 16,
                  child: Text(
                    '${names[selected.profileId] ?? selected.profileId} · Screen share',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(color: Colors.white, fontSize: 12),
                  ),
                ),
            ],
          ),
        );
        final sidebar = Material(
          color: voice.canvas,
          borderRadius: narrow ? BorderRadius.circular(12) : null,
          clipBehavior: Clip.antiAlias,
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Semantics(
                        header: true,
                        child: const Text(
                          'Screen shares',
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                    ),
                    IconButton(
                      tooltip: 'Close viewer',
                      onPressed: closeViewer,
                      icon: const Icon(Icons.close, size: 20),
                    ),
                  ],
                ),
                Column(
                  key: ScreenSharePanel.streamPickerKey,
                  children: [
                    for (final stream in share.streams)
                      _ShareThumbnail(
                        profileId: stream.profileId,
                        label: names[stream.profileId] ?? stream.profileId,
                        selected: selected?.profileId == stream.profileId,
                        preview: trackFor(stream),
                        paused:
                            (stream.streamId == share.localStreamId &&
                                share.isPaused) ||
                            (trackFor(stream)?.muted ?? false),
                        onTap: () => ref
                            .read(screenShareControllerProvider.notifier)
                            .selectStream(stream.profileId),
                      ),
                  ],
                ),
                if (share.streams.length > 1)
                  TextButton(
                    onPressed: () => showDialog<void>(
                      context: context,
                      builder: (dialogContext) => Dialog(
                        insetPadding: const EdgeInsets.all(16),
                        child: Consumer(
                          builder: (context, dialogRef, _) {
                            final current = dialogRef.watch(
                              screenShareControllerProvider,
                            );
                            final currentCall = dialogRef.watch(
                              callControllerProvider,
                            );
                            final sameSession = identical(
                              currentCall.session,
                              session,
                            );
                            return StreamViewerPicker(
                              streams: [
                                for (final stream in current.streams)
                                  if (sameSession)
                                    StreamViewerItem(
                                      profileId: stream.profileId,
                                      label:
                                          names[stream.profileId] ??
                                          stream.profileId,
                                      preview: trackFor(stream) == null
                                          ? null
                                          : livekit.VideoTrackRenderer(
                                              trackFor(stream)!,
                                              fit: livekit.VideoViewFit.contain,
                                            ),
                                      isPaused:
                                          (stream.streamId ==
                                                  current.localStreamId &&
                                              current.isPaused) ||
                                          (trackFor(stream)?.muted ?? false),
                                    ),
                              ],
                              selectedProfileId: current.selectedProfileId,
                              onSelected: (id) {
                                dialogRef
                                    .read(
                                      screenShareControllerProvider.notifier,
                                    )
                                    .selectStream(id);
                                Navigator.of(dialogContext).pop();
                              },
                            );
                          },
                        ),
                      ),
                    ),
                    child: const Text('Choose a stream'),
                  ),
                if (!narrow && canStartScreenShare && share.isSharing)
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      OutlinedButton.icon(
                        onPressed: () => ref
                            .read(callControllerProvider.notifier)
                            .pauseScreenShare(!share.isPaused),
                        icon: Icon(
                          share.isPaused ? Icons.play_arrow : Icons.pause,
                        ),
                        label: Text(share.isPaused ? 'Resume' : 'Pause'),
                      ),
                      FilledButton(
                        onPressed: () => ref
                            .read(callControllerProvider.notifier)
                            .stopScreenShare(),
                        style: FilledButton.styleFrom(
                          backgroundColor: voice.error,
                          foregroundColor: Colors.white,
                        ),
                        child: const Text('Stop sharing'),
                      ),
                    ],
                  ),
                if (share.streams.length >= 3)
                  const _ShareNotice(
                    message: 'Limit reached: up to 3 screen shares at once.',
                  ),
                if (error != null) _ShareNotice(message: error),
              ],
            ),
          ),
        );
        return SizedBox(
          key: ScreenSharePanel.panelKey,
          height: height,
          width: double.infinity,
          child: narrow
              ? Stack(
                  fit: StackFit.expand,
                  children: [
                    viewer,
                    Positioned(
                      left: 16,
                      right: 16,
                      bottom: 16,
                      child: ConstrainedBox(
                        constraints: BoxConstraints(maxHeight: height * .48),
                        child: sidebar,
                      ),
                    ),
                  ],
                )
              : Row(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Expanded(child: viewer),
                    SizedBox(width: 290, child: sidebar),
                  ],
                ),
        );
      },
    );
  }
}

class _ShareThumbnail extends StatelessWidget {
  const _ShareThumbnail({
    required this.profileId,
    required this.label,
    required this.selected,
    required this.paused,
    required this.onTap,
    this.preview,
  });
  final String profileId;
  final String label;
  final bool selected;
  final bool paused;
  final VoidCallback onTap;
  final livekit.VideoTrack? preview;
  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Semantics(
      button: true,
      selected: selected,
      label: '$label${paused ? ', paused' : ''}',
      child: InkWell(
        key: Key('screen_share_thumbnail_$profileId'),
        onTap: onTap,
        borderRadius: BorderRadius.circular(6),
        child: Padding(
          padding: const EdgeInsets.only(bottom: 12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Container(
                height: 76,
                clipBehavior: Clip.antiAlias,
                decoration: BoxDecoration(
                  color: voice.muted,
                  borderRadius: BorderRadius.circular(6),
                  border: Border.all(
                    color: selected ? voice.focusRing : voice.borderDefault,
                    width: selected ? 2 : 1,
                  ),
                ),
                child: preview != null
                    ? livekit.VideoTrackRenderer(
                        preview!,
                        fit: livekit.VideoViewFit.contain,
                      )
                    : Center(
                        child: Icon(
                          paused ? Icons.pause : Icons.screen_share_outlined,
                          color: voice.textSecondary,
                        ),
                      ),
              ),
              const SizedBox(height: 4),
              Text(
                label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontWeight: FontWeight.w600),
              ),
              if (selected || paused)
                Text(
                  paused ? 'Paused' : 'Watching',
                  style: TextStyle(color: voice.textSecondary, fontSize: 12),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ShareNotice extends StatelessWidget {
  const _ShareNotice({required this.message});
  final String message;
  @override
  Widget build(BuildContext context) => Semantics(
    liveRegion: true,
    child: Padding(
      padding: const EdgeInsets.symmetric(vertical: 8, horizontal: 16),
      child: Text(
        message,
        style: TextStyle(color: VoiceColors.of(context).error),
      ),
    ),
  );
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
      onPressed: !share.isSharing && share.streams.length >= 3
          ? null
          : () async {
              if (share.isSharing) {
                await notifier.stopScreenShare();
                return;
              }
              final startingCall = ref.read(callControllerProvider);
              final startingSession = startingCall.session;
              if (!startingCall.isActive || startingSession == null) return;
              bool stillCurrentCall() {
                final current = ref.read(callControllerProvider);
                return context.mounted &&
                    current.isActive &&
                    identical(current.session, startingSession);
              }

              var sources = <ScreenShareSource>[];
              if (canSelectDesktopScreenShareSource) {
                try {
                  sources = await ref
                      .read(screenShareSourceCatalogProvider)
                      .listSources();
                } catch (_) {
                  if (stillCurrentCall()) {
                    ref
                        .read(screenShareControllerProvider.notifier)
                        .setError('screen_share_source_unavailable');
                  }
                  return;
                }
                if (!stillCurrentCall()) return;
                if (sources.isEmpty) {
                  ref
                      .read(screenShareControllerProvider.notifier)
                      .setError('screen_share_source_unavailable');
                  return;
                }
              }
              if (!context.mounted) return;
              final options = await showScreenSharePicker(
                context,
                sources: sources,
                captureSystemAudioSupported:
                    canCaptureSystemAudioWithScreenShare,
                useSystemPicker: !canSelectDesktopScreenShareSource,
              );
              if (options == null || !stillCurrentCall()) return;
              await notifier.startScreenShare(
                maxFrameRate: options.maxFrameRate,
                captureSystemAudio: options.captureSystemAudio,
                sourceId: options.sourceId,
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
