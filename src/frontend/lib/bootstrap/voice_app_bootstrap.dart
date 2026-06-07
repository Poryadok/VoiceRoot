import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../app.dart';
import '../l10n/app_localizations.dart';
import '../state/auth_providers.dart';
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
    if (!_restoreComplete || auth.isRestoring) {
      return MaterialApp(
        locale: widget.locale,
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
      );
    }
    return VoiceApp(locale: widget.locale);
  }
}
