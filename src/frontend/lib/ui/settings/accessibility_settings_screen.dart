import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../settings/reduced_motion.dart';
import '../../settings/voice_input_settings.dart';
import '../../theme/voice_colors.dart';

/// Motion and voice input preferences for Settings → Accessibility.
class AccessibilitySettingsScreen extends ConsumerWidget {
  const AccessibilitySettingsScreen({super.key});

  static const Key screenKey = Key('accessibility_settings_screen');
  static const Key backKey = Key('accessibility_back');
  static const Key reducedMotionKey = Key('accessibility_reduced_motion');
  static const Key defaultModeKey = Key('accessibility_ptt_default_mode');
  static const Key pttModeKey = Key('accessibility_ptt_mode_ptt');
  static const Key vadModeKey = Key('accessibility_ptt_mode_vad');
  static const Key pttKeybindKey = Key('accessibility_ptt_keybind');

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final colors = VoiceColors.of(context);
    final reducedMotion = ref.watch(reducedMotionEnabledProvider);
    final voiceInput = ref.watch(voiceInputSettingsProvider);

    return Scaffold(
      key: screenKey,
      backgroundColor: colors.canvas,
      appBar: AppBar(
        backgroundColor: colors.surface,
        leading: IconButton(
          key: backKey,
          tooltip: MaterialLocalizations.of(context).backButtonTooltip,
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.of(context).maybePop(),
        ),
        title: Text(l10n.settingsAccessibilityTitle),
      ),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Align(
            alignment: Alignment.topCenter,
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 480),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Semantics(
                    header: true,
                    child: Text(
                      l10n.settingsAccessibilityMotionText,
                      style: TextStyle(color: colors.textSecondary),
                    ),
                  ),
                  const SizedBox(height: 8),
                  SwitchListTile(
                    key: reducedMotionKey,
                    contentPadding: EdgeInsets.zero,
                    title: Text(l10n.settingsReducedMotion),
                    value: reducedMotion,
                    onChanged: (value) => ref
                        .read(reducedMotionEnabledProvider.notifier)
                        .setEnabled(value),
                  ),
                  const SizedBox(height: 24),
                  Semantics(
                    header: true,
                    child: Text(
                      l10n.settingsAccessibilityVoiceCalls,
                      style: TextStyle(color: colors.textSecondary),
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    l10n.settingsAccessibilityDefaultVoiceMode,
                    style: Theme.of(context).textTheme.bodyLarge,
                  ),
                  const SizedBox(height: 8),
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: [
                      ChoiceChip(
                        key: pttModeKey,
                        label: Text(l10n.callPttMode),
                        selected: voiceInput.mode == VoiceInputMode.ptt,
                        onSelected: (_) => ref
                            .read(voiceInputSettingsProvider.notifier)
                            .setMode(VoiceInputMode.ptt),
                      ),
                      ChoiceChip(
                        key: vadModeKey,
                        label: Text(l10n.callVadMode),
                        selected: voiceInput.mode == VoiceInputMode.vad,
                        onSelected: (_) => ref
                            .read(voiceInputSettingsProvider.notifier)
                            .setMode(VoiceInputMode.vad),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  _PttKeybindTile(key: pttKeybindKey),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _PttKeybindTile extends ConsumerStatefulWidget {
  const _PttKeybindTile({super.key});

  @override
  ConsumerState<_PttKeybindTile> createState() => _PttKeybindTileState();
}

class _PttKeybindTileState extends ConsumerState<_PttKeybindTile> {
  final _focusNode = FocusNode();
  var _capturing = false;

  @override
  void dispose() {
    _focusNode.dispose();
    super.dispose();
  }

  bool _isModifier(LogicalKeyboardKey key) {
    return key == LogicalKeyboardKey.control ||
        key == LogicalKeyboardKey.controlLeft ||
        key == LogicalKeyboardKey.controlRight ||
        key == LogicalKeyboardKey.shift ||
        key == LogicalKeyboardKey.shiftLeft ||
        key == LogicalKeyboardKey.shiftRight ||
        key == LogicalKeyboardKey.alt ||
        key == LogicalKeyboardKey.altLeft ||
        key == LogicalKeyboardKey.altRight ||
        key == LogicalKeyboardKey.meta ||
        key == LogicalKeyboardKey.metaLeft ||
        key == LogicalKeyboardKey.metaRight;
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final colors = VoiceColors.of(context);
    final key = ref.watch(voiceInputSettingsProvider).pttKey;
    final keyLabel = key.keyLabel.isNotEmpty
        ? key.keyLabel
        : key.debugName ?? key.keyId.toString();

    return Focus(
      focusNode: _focusNode,
      onKeyEvent: (node, event) {
        if (!_capturing || event is! KeyDownEvent) {
          return KeyEventResult.ignored;
        }
        if (_isModifier(event.logicalKey)) {
          return KeyEventResult.ignored;
        }
        ref
            .read(voiceInputSettingsProvider.notifier)
            .setPttKey(event.logicalKey);
        setState(() => _capturing = false);
        return KeyEventResult.handled;
      },
      child: ListTile(
        contentPadding: EdgeInsets.zero,
        title: Text(l10n.settingsPttKeybind),
        subtitle: Text(
          _capturing
              ? l10n.settingsPttKeybindHint
              : l10n.settingsPttKeybindDesktopHint,
        ),
        trailing: DecoratedBox(
          decoration: BoxDecoration(
            color: colors.surface,
            borderRadius: BorderRadius.circular(8),
            border: Border.all(color: colors.borderDefault),
          ),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            child: Text(
              keyLabel,
              style: Theme.of(context).textTheme.labelMedium,
            ),
          ),
        ),
        onTap: () {
          setState(() => _capturing = true);
          _focusNode.requestFocus();
        },
      ),
    );
  }
}
