import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'backend/gateway_client.dart';
import 'l10n/app_localizations.dart';
import 'shell/three_column_shell.dart';
import 'state/auth_providers.dart';
import 'state/gateway_providers.dart';
import 'state/chat_providers.dart';
import 'state/social_providers.dart';
import 'theme/voice_colors.dart';
import 'theme/voice_theme_providers.dart';
import 'ui/auth/auth_screen.dart';
import 'ui/chat/chat_list_panel.dart';
import 'ui/chat/chat_room_panel.dart';
import 'ui/core/profile_accent_dot.dart';
import 'ui/social/social_panel.dart';

ThemeData _bootstrapTheme() {
  return ThemeData(
    useMaterial3: true,
    brightness: Brightness.dark,
    colorScheme: const ColorScheme.dark(primary: Color(0xFF7EC8E3)),
  );
}

class VoiceApp extends ConsumerWidget {
  const VoiceApp({super.key, this.locale});

  /// When non-null (e.g. in tests), forces [MaterialApp.locale] instead of the platform locale.
  final Locale? locale;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final themeAsync = ref.watch(voiceMaterialThemeProvider);
    final auth = ref.watch(authControllerProvider);

    return themeAsync.when(
      data: (theme) {
        if (!auth.isAuthenticated) {
          return MaterialApp(
            locale: locale,
            theme: theme,
            onGenerateTitle: (ctx) => AppLocalizations.of(ctx)!.appTitle,
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const AuthScreen(),
          );
        }
        return MaterialApp(
          locale: locale,
          theme: theme,
          onGenerateTitle: (ctx) => AppLocalizations.of(ctx)!.appTitle,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: _AuthenticatedShell(locale: locale),
        );
      },
      loading: () => MaterialApp(
        locale: locale,
        theme: _bootstrapTheme(),
        home: const Scaffold(
          body: Center(child: CircularProgressIndicator()),
        ),
      ),
      error: (e, _) => MaterialApp(
        locale: locale,
        theme: _bootstrapTheme(),
        home: Scaffold(body: Center(child: Text('Theme error: $e'))),
      ),
    );
  }
}

class _AuthenticatedShell extends ConsumerStatefulWidget {
  const _AuthenticatedShell({this.locale});

  final Locale? locale;

  @override
  ConsumerState<_AuthenticatedShell> createState() => _AuthenticatedShellState();
}

class _AuthenticatedShellState extends ConsumerState<_AuthenticatedShell> {
  var _discoverHintScheduled = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _scheduleDiscoverHintIfNeeded();
  }

  void _scheduleDiscoverHintIfNeeded() {
    if (_discoverHintScheduled) return;
    final auth = ref.read(authControllerProvider);
    if (!auth.pendingDiscoverHint) return;
    _discoverHintScheduled = true;
    WidgetsBinding.instance.addPostFrameCallback((_) => _maybeShowDiscoverHint());
  }

  Future<void> _maybeShowDiscoverHint() async {
    if (!mounted) return;
    final auth = ref.read(authControllerProvider);
    if (!auth.pendingDiscoverHint) return;

    final storage = ref.read(discoverHintStorageProvider);
    if (await storage.wasShown()) {
      ref.read(authControllerProvider.notifier).clearPendingDiscoverHint();
      return;
    }

    if (!mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    final l10n = AppLocalizations.of(context)!;
    messenger.showSnackBar(
      SnackBar(
        key: const Key('social_discover_hint'),
        content: Text(l10n.socialDiscoverHint),
      ),
    );
    await storage.markShown();
    if (!mounted) return;
    ref.read(authControllerProvider.notifier).clearPendingDiscoverHint();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final health = ref.watch(gatewayHealthProvider);
    final selectedChatId = ref.watch(selectedChatIdProvider);
    final profileAsync = ref.watch(activeProfileProvider);
    final voice = VoiceColors.of(context);

    final sessionLabel = profileAsync.when(
      data: (profile) => profile != null
          ? l10n.authSessionHandle(profile.handle)
          : l10n.authSessionProfile(
              ref.watch(authControllerProvider).activeProfileId!,
            ),
      loading: () => l10n.authSessionProfile(
        ref.watch(authControllerProvider).activeProfileId!,
      ),
      error: (_, _) => l10n.authSessionProfile(
        ref.watch(authControllerProvider).activeProfileId!,
      ),
    );

    return Scaffold(
      backgroundColor: voice.canvas,
      body: SafeArea(
        child: ThreeColumnShell(
          railChild: _SocialRail(
            onOpenSocial: () => _openSocialPanel(context),
          ),
          listChild: const ChatListPanel(),
          mainChild: selectedChatId == null
              ? Center(child: Text(l10n.chatRoomSelectPrompt))
              : ChatRoomPanel(chatId: selectedChatId),
          header: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              _SessionBar(
                onLogout: () =>
                    ref.read(authControllerProvider.notifier).logout(),
                sessionLabel: sessionLabel,
                logoutLabel: l10n.authLogout,
              ),
              _GatewayStatusBar(asyncHealth: health),
            ],
          ),
        ),
      ),
    );
  }
}

void _openSocialPanel(BuildContext context) {
  showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    builder: (ctx) => const SizedBox(
      height: 520,
      child: SocialPanel(),
    ),
  );
}

class _SocialRail extends StatelessWidget {
  const _SocialRail({required this.onOpenSocial});

  final VoidCallback onOpenSocial;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    return ColoredBox(
      color: voice.muted,
      child: Column(
        children: [
          const SizedBox(height: 8),
          IconButton(
            key: const Key('nav_open_social'),
            tooltip: l10n.socialRailTooltip,
            onPressed: onOpenSocial,
            icon: Icon(Icons.people_outline, color: voice.textSecondary),
          ),
        ],
      ),
    );
  }
}

class _SessionBar extends StatelessWidget {
  const _SessionBar({
    required this.onLogout,
    required this.sessionLabel,
    required this.logoutLabel,
  });

  final VoidCallback onLogout;
  final String sessionLabel;
  final String logoutLabel;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    return Material(
      color: voice.surface,
      elevation: 0,
      child: SizedBox(
        height: 40,
        width: double.infinity,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12),
          child: Row(
            children: [
              const ProfileAccentDot(),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  sessionLabel,
                  key: const Key('auth_session_profile'),
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: voice.textPrimary),
                ),
              ),
              TextButton(
                key: const Key('auth_logout'),
                onPressed: onLogout,
                child: Text(logoutLabel),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _GatewayStatusBar extends StatelessWidget {
  const _GatewayStatusBar({required this.asyncHealth});

  final AsyncValue<GatewayHealthResult> asyncHealth;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final text = asyncHealth.when(
      data: (r) => switch (r) {
        GatewayHealthOk() => l10n.gatewayStatusOk,
        GatewayHealthFailure(:final message) => message == kGatewayMissingBaseUrlDetail
            ? l10n.gatewayMissingBaseUrl
            : l10n.gatewayStatusFailure(message),
      },
      loading: () => l10n.gatewayStatusChecking,
      error: (e, _) => l10n.gatewayStatusError(e.toString()),
    );
    return Material(
      color: voice.elevated,
      elevation: 0,
      child: SizedBox(
        height: 40,
        width: double.infinity,
        child: Align(
          alignment: Alignment.centerLeft,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Text(
              text,
              key: const Key('gateway_status_text'),
              style: TextStyle(color: voice.textSecondary, fontSize: 13),
            ),
          ),
        ),
      ),
    );
  }
}
