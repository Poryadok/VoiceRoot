import 'dart:async';
import 'dart:typed_data';
import 'package:flutter/material.dart';
import '../../backend/messages_client.dart';
import '../../l10n/app_localizations.dart';
import 'music_format.dart';
import 'music_playback.dart';

class MusicAttachmentPlayer extends StatefulWidget {
  const MusicAttachmentPlayer({
    super.key,
    required this.attachment,
    required this.loadBytes,
    this.createPlayback = _create,
  });
  final MessageAttachment attachment;
  final Future<Uint8List> Function() loadBytes;
  final MusicPlayback Function() createPlayback;
  static MusicPlayback _create() => PullMusicPlayback();
  @override
  State<MusicAttachmentPlayer> createState() => _MusicAttachmentPlayerState();
}

class _MusicAttachmentPlayerState extends State<MusicAttachmentPlayer> {
  MusicPlayback? _playback;
  bool _loading = false;
  bool _failed = false;
  int _generation = 0;

  @override
  void didUpdateWidget(MusicAttachmentPlayer oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.attachment.fileId != widget.attachment.fileId ||
        oldWidget.attachment.e2eKeyWire != widget.attachment.e2eKeyWire)
      _retire();
  }

  void _changed() {
    if (!mounted) return;
    if (_playback?.closed == true) {
      final old = _playback;
      _failed = old?.failed == true;
      _playback = null;
      old?.removeListener(_changed);
      if (old != null) scheduleMicrotask(old.dispose);
    }
    setState(() {});
  }

  void _retire() {
    _generation++;
    final playback = _playback;
    _playback = null;
    playback?.removeListener(_changed);
    playback?.dispose();
    _loading = false;
    _failed = false;
  }

  @override
  void dispose() {
    _retire();
    super.dispose();
  }

  Future<void> _toggle() async {
    if (_loading) return;
    final current = _playback;
    if (current != null) {
      try {
        current.playing ? current.pause() : current.resume();
      } catch (_) {
        _retire();
        if (mounted) setState(() => _failed = true);
      }
      return;
    }
    final generation = ++_generation;
    setState(() {
      _loading = true;
      _failed = false;
    });
    Uint8List? bytes;
    MusicPlayback? playback;
    try {
      bytes = await widget.loadBytes();
      if (!mounted || generation != _generation) return;
      if (!hasSupportedMusicSignature(bytes))
        throw const FormatException('music_format_invalid');
      playback = widget.createPlayback()..addListener(_changed);
      _playback = playback;
      await playback.open(bytes);
      if (!mounted || generation != _generation) return;
      setState(() => _loading = false);
    } catch (_) {
      if (mounted && generation == _generation) {
        _retire();
        setState(() => _failed = true);
      }
    } finally {
      if (bytes != null) bytes.fillRange(0, bytes.length, 0);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final attachment = widget.attachment;
    final playback = _playback;
    final encrypted = attachment.isE2eEncrypted;
    final title =
        (encrypted ? playback?.title : attachment.musicTitle) ??
        (encrypted ? null : attachment.name) ??
        l10n.chatPinnedTypeMusic;
    final artist = encrypted ? playback?.artist : attachment.musicArtist;
    final album = encrypted ? playback?.album : attachment.musicAlbum;
    final duration =
        playback?.duration ??
        Duration(seconds: encrypted ? 0 : attachment.durationSeconds ?? 0);
    final position = playback?.position ?? Duration.zero;
    final total = duration.inMilliseconds.toDouble();
    return Container(
      key: ValueKey('music_attachment_${attachment.fileId}'),
      constraints: const BoxConstraints(maxWidth: 300),
      padding: const EdgeInsets.all(8),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, maxLines: 2, overflow: TextOverflow.ellipsis),
          if (artist != null && artist.isNotEmpty)
            Text(artist, maxLines: 1, overflow: TextOverflow.ellipsis),
          if (album != null && album.isNotEmpty)
            Text(album, maxLines: 1, overflow: TextOverflow.ellipsis),
          Row(
            children: [
              IconButton(
                key: ValueKey('music_toggle_${attachment.fileId}'),
                tooltip: playback?.playing == true
                    ? l10n.chatMusicPause
                    : l10n.chatMusicPlay,
                onPressed: _loading ? null : _toggle,
                icon: _loading
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : Icon(
                        playback?.playing == true
                            ? Icons.pause
                            : Icons.play_arrow,
                      ),
              ),
              Expanded(
                child: Semantics(
                  label: l10n.chatMusicSeek,
                  child: Slider(
                    key: ValueKey('music_seek_${attachment.fileId}'),
                    min: 0,
                    max: total > 0 ? total : 1,
                    value: position.inMilliseconds.toDouble().clamp(
                      0,
                      total > 0 ? total : 1,
                    ),
                    onChanged: !_loading && playback != null && total > 0
                        ? (value) {
                            try {
                              playback.seek(
                                Duration(milliseconds: value.round()),
                              );
                            } catch (_) {
                              _retire();
                              setState(() => _failed = true);
                            }
                          }
                        : null,
                  ),
                ),
              ),
              Text(
                '${duration.inMinutes}:${(duration.inSeconds % 60).toString().padLeft(2, '0')}',
              ),
            ],
          ),
          if (_failed)
            Text(
              l10n.chatAttachmentDownloadFailed,
              key: ValueKey('music_error_${attachment.fileId}'),
            ),
        ],
      ),
    );
  }
}
