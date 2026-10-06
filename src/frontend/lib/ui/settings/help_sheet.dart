import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../l10n/app_localizations.dart';
import '../../settings/voice_input_settings.dart';
import '../../theme/voice_colors.dart';
import '../../theme/voice_layout.dart';

/// Static searchable help guide (docs/features/onboarding.md — no tutorial replay).
class HelpSheet extends ConsumerStatefulWidget {
  const HelpSheet({super.key});

  static const Key searchKey = Key('settings_help_search');
  static const Key clearSearchKey = Key('settings_help_clear_search');
  static const Key docsKey = Key('settings_help_docs');
  static const Key supportKey = Key('settings_help_support');
  static const Key noResultsKey = Key('settings_help_no_results');

  static final Uri _docsUri = Uri(
    scheme: 'https',
    host: 'github.com',
    path: '/Poryadok/VoiceRoot/blob/master/README.md',
  );
  static final Uri _supportUri = Uri(
    scheme: 'https',
    host: 'github.com',
    path: '/Poryadok/VoiceRoot/issues',
  );

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
  ConsumerState<HelpSheet> createState() => _HelpSheetState();
}

class _HelpSheetState extends ConsumerState<HelpSheet> {
  final _searchController = TextEditingController();
  String _searchQuery = '';
  Uri? _failedUri;

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _openExternal(Uri uri) async {
    var opened = false;
    try {
      opened = await launchUrl(uri, mode: LaunchMode.externalApplication);
    } catch (_) {
      // The platform may not have a handler for external links.
    }
    if (!mounted) return;
    setState(() => _failedUri = opened ? null : uri);
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final inputSettings = ref.watch(voiceInputSettingsProvider);
    final showShortcuts = !VoiceLayout.isNarrow(
      MediaQuery.sizeOf(context).width,
    );
    final shortcuts = <_ShortcutItem>[
      _ShortcutItem('Ctrl+K', l10n.settingsHelpShortcutSearch),
      _ShortcutItem('Ctrl+,', l10n.settingsHelpShortcutSettings),
      _ShortcutItem('Alt+Up / Alt+Down', l10n.settingsHelpShortcutUnreadChats),
      _ShortcutItem('Escape', l10n.settingsHelpShortcutFocusComposer),
      _ShortcutItem('Up / Down', l10n.settingsHelpShortcutSelectMessage),
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

    final entries = <_HelpEntry>[
      _HelpEntry(l10n.settingsHelpChatsTitle, l10n.settingsHelpChatsBody),
      _HelpEntry(l10n.settingsHelpSpacesTitle, l10n.settingsHelpSpacesBody),
      _HelpEntry(
        l10n.settingsHelpMatchmakingTitle,
        l10n.settingsHelpMatchmakingBody,
      ),
      _HelpEntry(l10n.settingsHelpVoiceTitle, l10n.settingsHelpVoiceBody),
    ];
    final query = _searchQuery.trim().toLowerCase();
    final matchingEntries = entries
        .where((entry) {
          if (query.isEmpty) return true;
          return '${entry.title} ${entry.body}'.toLowerCase().contains(query);
        })
        .toList(growable: false);

    return Shortcuts(
      shortcuts: {
        SingleActivator(LogicalKeyboardKey.escape): const _CloseHelpIntent(),
      },
      child: Actions(
        actions: {
          _CloseHelpIntent: CallbackAction<_CloseHelpIntent>(
            onInvoke: (_) {
              Navigator.of(context).maybePop();
              return null;
            },
          ),
        },
        child: SafeArea(
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
                      Focus(
                        autofocus: true,
                        child: IconButton(
                          tooltip: l10n.settingsHelpCloseLabel,
                          onPressed: () => Navigator.of(context).maybePop(),
                          icon: const Icon(Icons.close),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 16),
                  TextField(
                    key: HelpSheet.searchKey,
                    controller: _searchController,
                    textInputAction: TextInputAction.search,
                    decoration: InputDecoration(
                      hintText: l10n.settingsHelpSearchHint,
                      prefixIcon: const Icon(Icons.search),
                      suffixIcon: _searchQuery.isEmpty
                          ? null
                          : IconButton(
                              key: HelpSheet.clearSearchKey,
                              tooltip: l10n.storyCreateGameTagClear,
                              onPressed: () {
                                _searchController.clear();
                                setState(() => _searchQuery = '');
                              },
                              icon: const Icon(Icons.close),
                            ),
                    ),
                    onChanged: (value) => setState(() => _searchQuery = value),
                  ),
                  const SizedBox(height: 12),
                  for (final entry in matchingEntries)
                    _HelpItem(title: entry.title, body: entry.body),
                  if (matchingEntries.isEmpty)
                    Padding(
                      key: HelpSheet.noResultsKey,
                      padding: const EdgeInsets.symmetric(vertical: 16),
                      child: Text(
                        l10n.settingsHelpNoResults,
                        style: TextStyle(color: voice.textSecondary),
                      ),
                    ),
                  const SizedBox(height: 8),
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      OutlinedButton.icon(
                        key: HelpSheet.docsKey,
                        onPressed: () =>
                            unawaited(_openExternal(HelpSheet._docsUri)),
                        icon: const Icon(Icons.menu_book_outlined),
                        label: Text(l10n.settingsHelpDocsLabel),
                      ),
                      OutlinedButton.icon(
                        key: HelpSheet.supportKey,
                        onPressed: () =>
                            unawaited(_openExternal(HelpSheet._supportUri)),
                        icon: const Icon(Icons.support_agent_outlined),
                        label: Text(l10n.settingsHelpSupportLabel),
                      ),
                    ],
                  ),
                  if (_failedUri != null) ...[
                    const SizedBox(height: 8),
                    Row(
                      children: [
                        Icon(Icons.error_outline, color: voice.textSecondary),
                        const SizedBox(width: 8),
                        Expanded(child: Text(l10n.settingsHelpLaunchError)),
                        TextButton(
                          onPressed: () =>
                              unawaited(_openExternal(_failedUri!)),
                          child: Text(l10n.commonRetry),
                        ),
                      ],
                    ),
                  ],
                  if (showShortcuts) ...[
                    const SizedBox(height: 16),
                    Text(
                      l10n.settingsHelpShortcutsTitle,
                      style: Theme.of(context).textTheme.titleSmall,
                    ),
                    const SizedBox(height: 8),
                    for (final shortcut in shortcuts)
                      _ShortcutRow(shortcut: shortcut),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _CloseHelpIntent extends Intent {
  const _CloseHelpIntent();
}

class _HelpEntry {
  const _HelpEntry(this.title, this.body);

  final String title;
  final String body;
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
