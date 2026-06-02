import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../app.dart';
import '../state/auth_providers.dart';

/// Restores persisted session (refresh) before showing [VoiceApp].
class VoiceAppBootstrap extends ConsumerStatefulWidget {
  const VoiceAppBootstrap({super.key, this.locale});

  final Locale? locale;

  @override
  ConsumerState<VoiceAppBootstrap> createState() => _VoiceAppBootstrapState();
}

class _VoiceAppBootstrapState extends ConsumerState<VoiceAppBootstrap> {
  @override
  void initState() {
    super.initState();
    Future.microtask(() => ref.read(authControllerProvider.notifier).restore());
  }

  @override
  Widget build(BuildContext context) {
    final auth = ref.watch(authControllerProvider);
    if (auth.isRestoring) {
      return MaterialApp(
        locale: widget.locale,
        home: const Scaffold(body: Center(child: CircularProgressIndicator())),
      );
    }
    return VoiceApp(locale: widget.locale);
  }
}
