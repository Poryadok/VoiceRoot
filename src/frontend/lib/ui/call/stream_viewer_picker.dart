import 'package:flutter/material.dart';

import '../../theme/voice_colors.dart';

/// Presentation for a currently available share. The call surface supplies the
/// preview and pause state from its exact LiveKit participant/track binding.
@immutable
class StreamViewerItem {
  const StreamViewerItem({
    required this.profileId,
    required this.label,
    this.description,
    this.preview,
    this.isPaused = false,
  });

  final String profileId;
  final String label;
  final String? description;
  final Widget? preview;
  final bool isPaused;
}

/// Responsive content for the in-call stream chooser. The caller retains the
/// authoritative roster and selection, so disappearing shares update in place.
class StreamViewerPicker extends StatelessWidget {
  const StreamViewerPicker({
    super.key,
    required this.streams,
    required this.selectedProfileId,
    required this.onSelected,
    this.onClose,
  });

  static const limitKey = Key('stream_viewer_limit');

  final List<StreamViewerItem> streams;
  final String? selectedProfileId;
  final ValueChanged<String> onSelected;
  final VoidCallback? onClose;

  @override
  Widget build(BuildContext context) {
    final colors = VoiceColors.of(context);
    final theme = Theme.of(context);
    return ConstrainedBox(
      constraints: BoxConstraints(
        maxWidth: 480,
        maxHeight: MediaQuery.sizeOf(context).height - 48,
      ),
      child: Material(
        color: colors.surface,
        borderRadius: BorderRadius.circular(8),
        clipBehavior: Clip.antiAlias,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            DecoratedBox(
              decoration: BoxDecoration(
                border: Border(bottom: BorderSide(color: colors.borderDefault)),
              ),
              child: Padding(
                padding: const EdgeInsets.only(
                  left: 16,
                  right: 8,
                  top: 4,
                  bottom: 4,
                ),
                child: Row(
                  children: [
                    Expanded(
                      child: Semantics(
                        header: true,
                        child: Text(
                          'Choose a stream',
                          style: theme.textTheme.titleMedium?.copyWith(
                            fontSize: 16,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                    ),
                    IconButton(
                      tooltip: 'Close',
                      onPressed:
                          onClose ?? () => Navigator.of(context).maybePop(),
                      icon: const Icon(Icons.close, size: 20),
                    ),
                  ],
                ),
              ),
            ),
            Flexible(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(20),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    if (streams.isEmpty)
                      Padding(
                        padding: const EdgeInsets.symmetric(vertical: 24),
                        child: Text(
                          'No active streams',
                          textAlign: TextAlign.center,
                          style: TextStyle(color: colors.textSecondary),
                        ),
                      )
                    else
                      LayoutBuilder(
                        builder: (context, constraints) {
                          final columns = constraints.maxWidth >= 400 ? 3 : 2;
                          final width =
                              (constraints.maxWidth - 12 * (columns - 1)) /
                              columns;
                          return Wrap(
                            spacing: 12,
                            runSpacing: 12,
                            children: [
                              for (final stream in streams)
                                SizedBox(
                                  width: width,
                                  child: _StreamTile(
                                    item: stream,
                                    selected:
                                        stream.profileId == selectedProfileId,
                                    onSelected: () =>
                                        onSelected(stream.profileId),
                                  ),
                                ),
                            ],
                          );
                        },
                      ),
                    if (streams.length >= 3) ...[
                      const SizedBox(height: 14),
                      Semantics(
                        key: limitKey,
                        liveRegion: true,
                        child: DecoratedBox(
                          decoration: BoxDecoration(
                            color: colors.warning.withValues(alpha: .12),
                            border: Border.all(
                              color: colors.warning.withValues(alpha: .5),
                            ),
                            borderRadius: BorderRadius.circular(6),
                          ),
                          child: const Padding(
                            padding: EdgeInsets.all(12),
                            child: Text(
                              'Limit reached: up to 3 streams can be shared at once.',
                              style: TextStyle(fontSize: 13),
                            ),
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _StreamTile extends StatelessWidget {
  const _StreamTile({
    required this.item,
    required this.selected,
    required this.onSelected,
  });

  final StreamViewerItem item;
  final bool selected;
  final VoidCallback onSelected;

  @override
  Widget build(BuildContext context) {
    final colors = VoiceColors.of(context);
    return Semantics(
      key: Key('stream_viewer_semantics_${item.profileId}'),
      button: true,
      selected: selected,
      onTap: onSelected,
      label: [
        item.label,
        if (item.description != null) item.description!,
        if (selected) 'Watching',
        if (item.isPaused) 'Paused',
      ].join(', '),
      child: ExcludeSemantics(
        child: InkWell(
          key: Key('stream_viewer_${item.profileId}'),
          borderRadius: BorderRadius.circular(6),
          onTap: onSelected,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Container(
                height: 76,
                margin: const EdgeInsets.symmetric(vertical: 8),
                clipBehavior: Clip.antiAlias,
                decoration: BoxDecoration(
                  color: colors.muted,
                  border: Border.all(
                    color: selected
                        ? colors.profileAccent
                        : colors.borderDefault,
                    width: selected ? 2 : 1,
                  ),
                  borderRadius: BorderRadius.circular(6),
                ),
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    item.preview ??
                        (item.isPaused
                            ? const SizedBox.shrink()
                            : Icon(
                                Icons.screen_share_outlined,
                                color: colors.textSecondary,
                                size: 28,
                              )),
                    if (item.isPaused)
                      ColoredBox(
                        color: colors.overlay,
                        child: const Center(
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Icon(Icons.pause, color: Colors.white, size: 20),
                              Text(
                                'Paused',
                                style: TextStyle(
                                  color: Colors.white,
                                  fontSize: 12,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    if (selected)
                      Positioned(
                        top: 4,
                        right: 4,
                        child: Icon(
                          Icons.check_circle,
                          color: colors.profileAccent,
                          size: 18,
                        ),
                      ),
                  ],
                ),
              ),
              Text(
                item.label,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontWeight: FontWeight.w600),
              ),
              if (item.description != null)
                Text(
                  item.description!,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: colors.textSecondary, fontSize: 12),
                ),
              if (selected)
                Text(
                  'Watching',
                  style: TextStyle(
                    color: colors.textPrimary,
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
