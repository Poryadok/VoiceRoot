import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../theme/voice_colors.dart';
import '../a11y/focus_trap.dart';

/// A completed match is a draft, never an automatic publication.
class StorySuggestionOverlay extends StatelessWidget {
  const StorySuggestionOverlay({
    super.key,
    required this.gameName,
    required this.teammateHandles,
    required this.onEdit,
    required this.onDismiss,
  });

  final String gameName;
  final List<String> teammateHandles;
  final VoidCallback onEdit;
  final VoidCallback onDismiss;

  static const editKey = Key('story_suggestion_edit');

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Material(
      color: voice.overlay,
      child: CallbackShortcuts(
        bindings: {const SingleActivator(LogicalKeyboardKey.escape): onDismiss},
        child: VoiceFocusTrap(
          onEscape: onDismiss,
          child: SafeArea(
            child: Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 480),
                  child: Semantics(
                    explicitChildNodes: true,
                    scopesRoute: true,
                    namesRoute: true,
                    label: 'Share your match',
                    child: Material(
                      color: voice.surface,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(16),
                        side: BorderSide(color: voice.borderDefault),
                      ),
                      clipBehavior: Clip.antiAlias,
                      child: SingleChildScrollView(
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            Padding(
                              padding: const EdgeInsets.fromLTRB(20, 12, 8, 12),
                              child: Row(
                                children: [
                                  const Expanded(
                                    child: Text(
                                      'Share your match',
                                      style: TextStyle(
                                        fontSize: 20,
                                        fontWeight: FontWeight.w600,
                                      ),
                                    ),
                                  ),
                                  IconButton(
                                    tooltip: 'Dismiss',
                                    onPressed: onDismiss,
                                    icon: const Icon(Icons.close),
                                  ),
                                ],
                              ),
                            ),
                            Divider(height: 1, color: voice.borderDefault),
                            Padding(
                              padding: const EdgeInsets.all(20),
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.stretch,
                                children: [
                                  Container(
                                    constraints: const BoxConstraints(
                                      minHeight: 260,
                                    ),
                                    padding: const EdgeInsets.all(18),
                                    decoration: BoxDecoration(
                                      borderRadius: BorderRadius.circular(12),
                                      gradient: LinearGradient(
                                        begin: Alignment.topRight,
                                        end: Alignment.bottomLeft,
                                        colors: [
                                          voice.profileAccent.withValues(
                                            alpha: .24,
                                          ),
                                          voice.muted,
                                        ],
                                      ),
                                    ),
                                    child: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: [
                                        Icon(
                                          Icons.sports_esports_outlined,
                                          size: 64,
                                          color: voice.profileAccent,
                                        ),
                                        const SizedBox(height: 48),
                                        Chip(label: Text(gameName)),
                                        const SizedBox(height: 8),
                                        const Text(
                                          'Match completed',
                                          style: TextStyle(
                                            fontSize: 24,
                                            fontWeight: FontWeight.w700,
                                          ),
                                        ),
                                        if (teammateHandles.isNotEmpty) ...[
                                          const SizedBox(height: 8),
                                          Text(
                                            teammateHandles
                                                .map((name) => '@$name')
                                                .join(' · '),
                                          ),
                                        ],
                                      ],
                                    ),
                                  ),
                                  const SizedBox(height: 16),
                                  FilledButton.icon(
                                    key: editKey,
                                    onPressed: onEdit,
                                    icon: const Icon(Icons.edit_outlined),
                                    label: const Text('Edit & publish'),
                                  ),
                                  TextButton(
                                    onPressed: onDismiss,
                                    child: const Text('Dismiss'),
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
              ),
            ),
          ),
        ),
      ),
    );
  }
}
