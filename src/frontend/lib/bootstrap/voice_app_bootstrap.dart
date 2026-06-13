import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../app.dart';
import '../l10n/app_localizations.dart';
import '../state/auth_providers.dart';
import '../theme/voice_theme_providers.dart';
import '../ui/core/voice_state_panel.dart';

/// Restores persisted session (refresh) before showing [VoiceApp].
class VoiceAppBootstrap extends ConsumerStatefulWidget {
  const VoiceAppBootstrap({super.key, this.locale});

  final Locale? locale;

  @override
  ConsumerState<VoiceAppBootstrap> createState() => _VoiceAppBootstrapState();
}

class _VoiceAppBootstrapState extends ConsumerState<VoiceAppBootstrap> {
  var _restoreComplete = false;

  @override
  void initState() {
    super.initState();
    Future.microtask(_restoreSession);
  }

  Future<void> _restoreSession() async {
    await ref.read(authControllerProvider.notifier).restore();
    if (mounted) {
      setState(() => _restoreComplete = true);
    }
  }

  @override
  Widget build(BuildContext context) {
    final auth = ref.watch(authControllerProvider);
    final themeAsync = ref.watch(voiceMaterialThemeProvider);
    if (!_restoreComplete || auth.isRestoring) {
      return themeAsync.when(
        data: (theme) => MaterialApp(
          locale: widget.locale,
          theme: theme,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Builder(
            builder: (ctx) {
              final l10n = AppLocalizations.of(ctx)!;
              return Scaffold(
                body: Center(
                  child: VoiceStatePanel(
                    title: l10n.bootstrapRestoring,
                    icon: Icons.hourglass_empty,
                  ),
                ),
              );
            },
          ),
        ),
        loading: () => MaterialApp(
          locale: widget.locale,
          theme: ThemeData(
            useMaterial3: true,
            brightness: Brightness.dark,
            colorScheme: const ColorScheme.dark(primary: Color(0xFF7EC8E3)),
          ),
          home: const Scaffold(
            body: Center(child: CircularProgressIndicator()),
          ),
        ),
        error: (_, _) => MaterialApp(
          locale: widget.locale,
          theme: ThemeData(
            useMaterial3: true,
            brightness: Brightness.dark,
            colorScheme: const ColorScheme.dark(primary: Color(0xFF7EC8E3)),
          ),
          home: const Scaffold(
            body: Center(child: CircularProgressIndicator()),
          ),
        ),
      );
    }
    return VoiceApp(locale: widget.locale);
  }
}
