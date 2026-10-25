import 'package:flutter/material.dart';

import '../../backend/proto_mappers.dart' show protoTimestampToDateTime;
import '../../gen/voice/messaging/v1/messaging.pb.dart';
import '../../l10n/app_localizations.dart';
import '../../theme/voice_colors.dart';

/// Sender-owned pending schedules. The host owns API mutations and recovery;
/// this component only keeps the local expanded/collapsed presentation state.
class ScheduledMessagesStrip extends StatefulWidget {
  const ScheduledMessagesStrip({
    super.key,
    required this.messages,
    required this.onEdit,
    required this.onCancel,
    required this.onSendNow,
    this.busyIds = const {},
    this.failedIds = const {},
    this.onRetry,
    this.onDismissError,
    this.loading = false,
    this.hasMore = false,
    this.loadFailed = false,
    this.onLoadMore,
    this.onRetryLoad,
    this.editingId,
  });

  static const toggleKey = Key('scheduled_strip_toggle');
  static const listKey = Key('scheduled_strip_list');
  final List<ScheduledMessage> messages;
  final ValueChanged<ScheduledMessage> onEdit;
  final ValueChanged<ScheduledMessage> onCancel;
  final ValueChanged<ScheduledMessage> onSendNow;
  final Set<String> busyIds;
  final Set<String> failedIds;
  final ValueChanged<String>? onRetry;
  final ValueChanged<String>? onDismissError;
  final bool loading;
  final bool hasMore;
  final bool loadFailed;
  final VoidCallback? onLoadMore;
  final VoidCallback? onRetryLoad;
  final String? editingId;

  @override
  State<ScheduledMessagesStrip> createState() => _ScheduledMessagesStripState();
}

class _ScheduledMessagesStripState extends State<ScheduledMessagesStrip> {
  bool _expanded = true;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final pending = widget.messages
        .where(
          (row) =>
              row.status ==
              ScheduledMessageStatus.SCHEDULED_MESSAGE_STATUS_PENDING,
        )
        .toList(growable: false);
    if (pending.isEmpty && !widget.loading && !widget.loadFailed) {
      return const SizedBox.shrink();
    }
    return Material(
      color: voice.surface,
      child: DecoratedBox(
        decoration: BoxDecoration(
          border: Border.symmetric(
            horizontal: BorderSide(color: voice.borderDefault),
          ),
        ),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          child: FocusTraversalGroup(
            policy: ReadingOrderTraversalPolicy(),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        l10n.chatScheduledCount(pending.length),
                        style: TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                          color: voice.textPrimary,
                        ),
                      ),
                    ),
                    IconButton(
                      key: ScheduledMessagesStrip.toggleKey,
                      tooltip: _expanded
                          ? l10n.chatScheduledCollapse
                          : l10n.chatScheduledExpand,
                      onPressed: () => setState(() => _expanded = !_expanded),
                      icon: Icon(
                        _expanded
                            ? Icons.keyboard_arrow_up
                            : Icons.keyboard_arrow_down,
                      ),
                    ),
                  ],
                ),
                if (widget.loading) const LinearProgressIndicator(minHeight: 2),
                if (_expanded)
                  ConstrainedBox(
                    constraints: BoxConstraints(
                      maxHeight:
                          (MediaQuery.sizeOf(context).height -
                                  MediaQuery.viewInsetsOf(context).bottom -
                                  260)
                              .clamp(0.0, 280.0),
                    ),
                    child: SingleChildScrollView(
                      key: ScheduledMessagesStrip.listKey,
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          if (widget.loadFailed)
                            _failure(
                              context,
                              keySuffix: 'load',
                              onRetry: widget.loading
                                  ? null
                                  : widget.onRetryLoad,
                            ),
                          for (final row in pending) _row(context, row),
                          if (widget.hasMore)
                            TextButton(
                              onPressed: widget.loading
                                  ? null
                                  : widget.onLoadMore,
                              child: Text(l10n.chatScheduleLoadMore),
                            ),
                        ],
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _row(BuildContext context, ScheduledMessage row) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final busy = widget.busyIds.contains(row.id);
    final encrypted = row.hasPayload() && row.payload.isE2e;
    final date = protoTimestampToDateTime(
      row.hasScheduledAt() ? row.scheduledAt : null,
    )?.toLocal();
    final material = MaterialLocalizations.of(context);
    final schedule = row.sendWhenOnline
        ? l10n.chatSendWhenOnline
        : date == null
        ? l10n.chatSchedulePending
        : '${material.formatMediumDate(date)} ${material.formatTimeOfDay(TimeOfDay.fromDateTime(date))}';
    final title = encrypted
        ? l10n.chatScheduledEncrypted
        : row.payload.content.isNotEmpty
        ? row.payload.content
        : l10n.chatScheduleMessage;
    return Container(
      key: ValueKey('scheduled_row_${row.id}'),
      margin: const EdgeInsets.only(top: 6),
      padding: const EdgeInsets.fromLTRB(8, 7, 8, 0),
      decoration: BoxDecoration(
        color: voice.canvas,
        border: Border.all(color: voice.borderDefault),
        borderRadius: BorderRadius.circular(6),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          LayoutBuilder(
            builder: (context, constraints) {
              final titleWidget = Text(
                title,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  color: voice.textPrimary,
                ),
              );
              final dateWidget = Text(
                schedule,
                style: TextStyle(fontSize: 12, color: voice.textSecondary),
              );
              if (constraints.maxWidth < 300 ||
                  MediaQuery.textScalerOf(context).scale(1) > 1.2) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [titleWidget, dateWidget],
                );
              }
              return Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(child: titleWidget),
                  const SizedBox(width: 12),
                  dateWidget,
                ],
              );
            },
          ),
          if (busy) const LinearProgressIndicator(minHeight: 2),
          Wrap(
            alignment: WrapAlignment.spaceBetween,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              Wrap(
                children: [
                  if (!encrypted && row.hasPayload())
                    TextButton(
                      key: ValueKey('scheduled_edit_${row.id}'),
                      onPressed: busy || widget.editingId != null
                          ? null
                          : () => widget.onEdit(row),
                      child: Text(l10n.commonEdit),
                    ),
                  TextButton(
                    key: ValueKey('scheduled_cancel_${row.id}'),
                    onPressed: busy || widget.editingId == row.id
                        ? null
                        : () => widget.onCancel(row),
                    child: Text(l10n.commonCancel),
                  ),
                ],
              ),
              TextButton(
                key: ValueKey('scheduled_send_${row.id}'),
                onPressed: busy || widget.editingId == row.id
                    ? null
                    : () => widget.onSendNow(row),
                child: Text(
                  l10n.chatScheduleSendNow,
                  style: const TextStyle(fontWeight: FontWeight.w600),
                ),
              ),
            ],
          ),
          if (widget.failedIds.contains(row.id))
            _failure(
              context,
              keySuffix: row.id,
              onRetry: busy || widget.onRetry == null
                  ? null
                  : () => widget.onRetry!(row.id),
              onDismiss: busy || widget.onDismissError == null
                  ? null
                  : () => widget.onDismissError!(row.id),
            ),
        ],
      ),
    );
  }

  Widget _failure(
    BuildContext context, {
    required String keySuffix,
    VoidCallback? onRetry,
    VoidCallback? onDismiss,
  }) {
    final l10n = AppLocalizations.of(context)!;
    return Semantics(
      liveRegion: true,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            l10n.chatScheduleFailure,
            style: TextStyle(
              fontSize: 12,
              color: VoiceColors.of(context).error,
            ),
          ),
          Wrap(
            children: [
              TextButton(
                key: ValueKey('scheduled_retry_$keySuffix'),
                onPressed: onRetry,
                child: Text(l10n.chatScheduleRetry),
              ),
              if (onDismiss != null)
                TextButton(
                  key: ValueKey('scheduled_dismiss_$keySuffix'),
                  onPressed: onDismiss,
                  child: Text(l10n.chatScheduledDismiss),
                ),
            ],
          ),
        ],
      ),
    );
  }
}
