import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../state/shell_providers.dart';

/// Desktop/web keyboard shortcuts (docs/features/accessibility.md).
class VoiceShortcuts extends ConsumerWidget {
  const VoiceShortcuts({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Shortcuts(
      shortcuts: const {
        SingleActivator(LogicalKeyboardKey.keyK, control: true): _FocusSearchIntent(),
        SingleActivator(LogicalKeyboardKey.comma, control: true): _OpenSettingsIntent(),
      },
      child: Actions(
        actions: {
          _FocusSearchIntent: CallbackAction<_FocusSearchIntent>(
            onInvoke: (_) {
              ref.read(navigationSectionProvider.notifier).state =
                  NavigationSection.chats;
              return null;
            },
          ),
          _OpenSettingsIntent: CallbackAction<_OpenSettingsIntent>(
            onInvoke: (_) {
              ref.read(settingsSheetRequestProvider.notifier).state = true;
              return null;
            },
          ),
        },
        child: Focus(autofocus: true, child: child),
      ),
    );
  }
}

class _FocusSearchIntent extends Intent {
  const _FocusSearchIntent();
}

class _OpenSettingsIntent extends Intent {
  const _OpenSettingsIntent();
}

/// Signals [VoiceApp] to open settings (Ctrl+,).
final settingsSheetRequestProvider = StateProvider<bool>((ref) => false);
