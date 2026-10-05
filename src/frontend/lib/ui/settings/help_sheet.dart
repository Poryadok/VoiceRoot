import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../settings/voice_input_settings.dart';
import '../../theme/voice_colors.dart';

/// Static help / FAQ (docs/features/onboarding.md — no tutorial replay).
class HelpSheet extends ConsumerWidget {
  const HelpSheet({super.key});

  static Future<void> show(BuildContext context) {
    final previousFocus = FocusScope.of(context).focusedChild;
    return showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (_) => const HelpSheet(),
    ).whenComplete(() {
      if (previousFocus?.context?.mounted ?? false) {
        previousFocus!.requestFocus();
      }
    });
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final inputSettings = ref.watch(voiceInputSettingsProvider);
    final shortcuts = <_ShortcutItem>[
      _ShortcutItem('Ctrl+K', l10n.settingsHelpShortcutSearch),
      _ShortcutItem('Ctrl+,', l10n.settingsHelpShortcutSettings),
      _ShortcutItem('Alt+↑ / Alt+↓', l10n.settingsHelpShortcutUnreadChats),
      _ShortcutItem('Escape', l10n.settingsHelpShortcutFocusComposer),
      _ShortcutItem('↑ / ↓', l10n.settingsHelpShortcutSelectMessage),
      _ShortcutItem('Enter', l10n.settingsHelpShortcutMessageActions),
      _ShortcutItem('R', l10n.settingsHelpShortcutReply),
      _ShortcutItem('E', l10n.settingsHelpShortcutReact),
    ];
    if (inputSettings.mode == VoiceInputMode.ptt) {
      final key = inputSettings.pttKey;
      shortcuts.add(
        _ShortcutItem(
          key.keyLabel.isNotEmpty ? key.keyLabel : key.debugName ?? '',
          l10n.settingsHelpShortcutPushToTalk,
        ),
      );
    }

    return SafeArea(
      child: ConstrainedBox(
        constraints: BoxConstraints(
          maxHeight: MediaQuery.sizeOf(context).height * .9,
        ),
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      l10n.settingsHelpTitle,
                      style: Theme.of(context).textTheme.titleLarge,
                    ),
                  ),
                  IconButton(
                    tooltip: l10n.settingsHelpCloseLabel,
                    onPressed: () => Navigator.of(context).maybePop(),
                    icon: const Icon(Icons.close),
                  ),
                ],
              ),
              const SizedBox(height: 16),
              _HelpItem(
                title: l10n.settingsHelpChatsTitle,
                body: l10n.settingsHelpChatsBody,
              ),
              _HelpItem(
                title: l10n.settingsHelpSpacesTitle,
                body: l10n.settingsHelpSpacesBody,
              ),
              _HelpItem(
                title: l10n.settingsHelpMatchmakingTitle,
                body: l10n.settingsHelpMatchmakingBody,
              ),
              _HelpItem(
                title: l10n.settingsHelpVoiceTitle,
                body: l10n.settingsHelpVoiceBody,
              ),
              const SizedBox(height: 8),
              Text(
                l10n.settingsHelpShortcutsTitle,
                style: Theme.of(context).textTheme.titleSmall,
              ),
              const SizedBox(height: 8),
              for (final shortcut in shortcuts)
                _ShortcutRow(shortcut: shortcut),
              const SizedBox(height: 8),
              Text(
                l10n.settingsHelpFooter,
                style: TextStyle(color: voice.textSecondary, fontSize: 13),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ShortcutItem {
  const _ShortcutItem(this.keys, this.description);

  final String keys;
  final String description;
}

class _ShortcutRow extends StatelessWidget {
  const _ShortcutRow({required this.shortcut});

  final _ShortcutItem shortcut;

  @override
  Widget build(BuildContext context) {
    final textTheme = Theme.of(context).textTheme;
    return Semantics(
      container: true,
      label: '${shortcut.keys}, ${shortcut.description}',
      child: ExcludeSemantics(
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 4),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              SizedBox(
                width: 104,
                child: Text(
                  shortcut.keys,
                  style: textTheme.labelLarge?.copyWith(
                    fontFeatures: const [FontFeature.tabularFigures()],
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(shortcut.description, style: textTheme.bodyMedium),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _HelpItem extends StatelessWidget {
  const _HelpItem({required this.title, required this.body});

  final String title;
  final String body;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, style: Theme.of(context).textTheme.titleSmall),
          const SizedBox(height: 4),
          Text(body),
        ],
      ),
    );
  }
}
