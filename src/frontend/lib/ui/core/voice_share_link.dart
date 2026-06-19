import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../l10n/app_localizations.dart';

/// Copies [link] to clipboard and shows a confirmation snackbar.
Future<void> copyVoiceShareLink(BuildContext context, String link) async {
  await Clipboard.setData(ClipboardData(text: link));
  if (!context.mounted) return;
  final l10n = AppLocalizations.of(context)!;
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(content: Text(l10n.shareLinkCopied)),
  );
}

/// Icon button that copies a pre-built share link.
class VoiceShareLinkButton extends StatelessWidget {
  const VoiceShareLinkButton({
    super.key,
    required this.link,
    this.tooltip,
  });

  final String link;
  final String? tooltip;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return IconButton(
      key: Key('share_link_${link.hashCode}'),
      tooltip: tooltip ?? l10n.shareLinkAction,
      icon: const Icon(Icons.link),
      onPressed: () => copyVoiceShareLink(context, link),
    );
  }
}
