import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../theme/voice_colors.dart';
import '../../theme/voice_metrics.dart';

/// A recoverable error belonging to the adjacent composer or attachment.
/// Callers retain ownership of retries, draft state and error classification.
class ComposerErrorNotice extends StatelessWidget {
  const ComposerErrorNotice({
    super.key,
    required this.message,
    this.attachmentName,
    this.actionLabel,
    this.onAction,
    this.actionKey,
    this.dismissLabel,
    this.onDismiss,
    this.dismissKey,
  });

  final String message;
  final String? attachmentName;
  final String? actionLabel;
  final VoidCallback? onAction;
  final Key? actionKey;
  final String? dismissLabel;
  final VoidCallback? onDismiss;
  final Key? dismissKey;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final metrics = context.voiceMetrics;
    return LayoutBuilder(
      builder: (context, constraints) => Align(
        alignment: AlignmentDirectional.centerEnd,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 620),
          child: Semantics(
            liveRegion: true,
            child: Container(
              width: double.infinity,
              padding: const EdgeInsets.fromLTRB(4, 8, 8, 8),
              decoration: BoxDecoration(
                color: voice.error.withValues(alpha: 0.08),
                border: Border.all(color: voice.error.withValues(alpha: 0.35)),
                borderRadius: BorderRadius.circular(
                  metrics.corner('md', fallback: 6),
                ),
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  IconButton(
                    tooltip: MaterialLocalizations.of(context).copyButtonLabel,
                    onPressed: () =>
                        Clipboard.setData(ClipboardData(text: message)),
                    icon: const Icon(Icons.content_copy_outlined, size: 18),
                  ),
                  Expanded(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        if (attachmentName != null)
                          Padding(
                            padding: const EdgeInsets.only(bottom: 4),
                            child: Text(
                              attachmentName!,
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(
                                color: voice.textPrimary,
                                fontSize: 14,
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                          ),
                        SelectableText(
                          message,
                          style: TextStyle(
                            color: voice.error,
                            fontSize: 13,
                            height: 18 / 13,
                          ),
                        ),
                        if (actionLabel != null ||
                            dismissLabel != null ||
                            onDismiss != null)
                          Wrap(
                            spacing: 8,
                            crossAxisAlignment: WrapCrossAlignment.center,
                            children: [
                              if (actionLabel != null)
                                TextButton(
                                  key: actionKey,
                                  onPressed: onAction,
                                  style: TextButton.styleFrom(
                                    padding: const EdgeInsets.symmetric(
                                      horizontal: 2,
                                    ),
                                    minimumSize: const Size(48, 48),
                                    foregroundColor: voice.profileAccent,
                                  ),
                                  child: Text(actionLabel!),
                                ),
                              if (dismissLabel != null || onDismiss != null)
                                TextButton(
                                  key: dismissKey,
                                  onPressed: onDismiss,
                                  style: TextButton.styleFrom(
                                    padding: const EdgeInsets.symmetric(
                                      horizontal: 2,
                                    ),
                                    minimumSize: const Size(48, 48),
                                    foregroundColor: voice.error,
                                  ),
                                  child: Text(
                                    dismissLabel ??
                                        MaterialLocalizations.of(
                                          context,
                                        ).closeButtonTooltip,
                                  ),
                                ),
                            ],
                          ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
