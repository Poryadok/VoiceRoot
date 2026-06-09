import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'backend/gateway_client.dart';
import 'backend/users_client.dart';
import 'l10n/app_localizations.dart';
import 'shell/three_column_shell.dart';
import 'state/auth_providers.dart';
import 'state/gateway_providers.dart';
import 'state/chat_providers.dart';
import 'state/social_providers.dart';
import 'theme/voice_colors.dart';
import 'theme/voice_theme_providers.dart';
import 'ui/auth/auth_screen.dart';
import 'ui/call/active_call_panel.dart';
import 'ui/call/call_error_listener.dart';
import 'ui/call/incoming_call_overlay.dart';
import 'ui/call/outgoing_call_overlay.dart';
import 'ui/chat/chat_list_panel.dart';
import 'ui/chat/chat_room_panel.dart';
import 'ui/core/profile_accent_dot.dart';
import 'ui/core/voice_bottom_sheet.dart';
import 'ui/core/voice_state_panel.dart';
import 'ui/profile/profile_edit_sheet.dart';
import 'ui/settings/settings_sheet.dart';
import 'ui/social/social_panel.dart';
import 'ui/version/version_policy_overlay.dart';

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
    final localePref = ref.watch(appLocalePreferenceProvider);
    final effectiveLocale = locale ?? localePref;

    return themeAsync.when(
      data: (theme) {
        if (!auth.isAuthenticated) {
          return MaterialApp(
            locale: effectiveLocale,
            theme: theme,
            onGenerateTitle: (ctx) => AppLocalizations.of(ctx)!.appTitle,
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const AuthScreen(),
          );
        }
        return MaterialApp(
          locale: effectiveLocale,
          theme: theme,
          onGenerateTitle: (ctx) => AppLocalizations.of(ctx)!.appTitle,
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: _AuthenticatedShell(locale: effectiveLocale),
        );
      },
      loading: () => MaterialApp(
        locale: effectiveLocale,
        theme: _bootstrapTheme(),
        home: const Scaffold(body: Center(child: CircularProgressIndicator())),
      ),
      error: (e, _) => MaterialApp(
        locale: effectiveLocale,
        theme: _bootstrapTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Builder(
          builder: (ctx) {
            final l10n = AppLocalizations.of(ctx)!;
            return Scaffold(
              body: Center(
                child: VoiceStatePanel(
                  title: l10n.themeLoadError,
                  message: e.toString(),
                  icon: Icons.palette_outlined,
                ),
              ),
            );
          },
        ),
      ),
    );
  }
}

class _AuthenticatedShell extends ConsumerStatefulWidget {
  const _AuthenticatedShell({this.locale});

  final Locale? locale;

  @override
  ConsumerState<_AuthenticatedShell> createState() =>
      _AuthenticatedShellState();
}

class _AuthenticatedShellState extends ConsumerState<_AuthenticatedShell> {
  var _discoverHintScheduled = false;
  var _socialPanelOpen = false;

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
    WidgetsBinding.instance.addPostFrameCallback(
      (_) => _maybeShowDiscoverHint(),
    );
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

  void _openSocialPanel() {
    setState(() => _socialPanelOpen = true);
    showVoiceBottomSheet<void>(
      context: context,
      scrollable: false,
      child: const SocialPanel(),
    ).whenComplete(() {
      if (mounted) setState(() => _socialPanelOpen = false);
    });
  }

  void _openProfileEditSheet(VoiceProfile profile) {
    showVoiceBottomSheet<void>(
      context: context,
      child: ProfileEditSheet(profile: profile),
    );
  }

  void _openSettingsSheet() {
    showVoiceBottomSheet<void>(
      context: context,
      child: const SettingsSheet(),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final health = ref.watch(gatewayHealthProvider);
    final selectedChatId = ref.watch(selectedChatIdProvider);
    final profileAsync = ref.watch(activeProfileProvider);
    final voice = VoiceColors.of(context);
    final socialBadge =
        ref.watch(friendRequestsProvider).valueOrNull?.incoming.length ?? 0;

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

    return VersionPolicyOverlay(
      child: CallErrorListener(
        child: Scaffold(
          backgroundColor: voice.canvas,
          body: Stack(
            children: [
              SafeArea(
            child: LayoutBuilder(
              builder: (context, constraints) {
                final narrow = constraints.maxWidth < 600;
                final onBackToChats = selectedChatId == null
                    ? null
                    : () => ref.read(selectedChatIdProvider.notifier).state =
                          null;
                return ThreeColumnShell(
                  railChild: _SocialRail(
                    active: _socialPanelOpen,
                    badgeCount: socialBadge,
                    onOpenSocial: _openSocialPanel,
                  ),
                  mobileRailChild: _MobileRailStrip(
                    active: _socialPanelOpen,
                    badgeCount: socialBadge,
                    onOpenSocial: _openSocialPanel,
                  ),
                  listChild: const ChatListPanel(),
                  mainChild: selectedChatId == null
                      ? Center(child: Text(l10n.chatRoomSelectPrompt))
                      : ChatRoomPanel(
                          chatId: selectedChatId,
                          onBack: narrow ? onBackToChats : null,
                        ),
                  showMainOnlyOnNarrow: selectedChatId != null,
                  header: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      _SessionBar(
                        onLogout: () =>
                            ref.read(authControllerProvider.notifier).logout(),
                        onEditProfile: profileAsync.valueOrNull == null
                            ? null
                            : () => _openProfileEditSheet(
                                profileAsync.valueOrNull!,
                              ),
                        onOpenSettings: _openSettingsSheet,
                        sessionLabel: sessionLabel,
                        logoutLabel: l10n.authLogout,
                        editProfileTooltip: l10n.profileEditTooltip,
                        settingsTooltip: l10n.settingsTooltip,
                      ),
                      if (_GatewayStatusBar.shouldShow(health))
                        _GatewayStatusBar(asyncHealth: health),
                    ],
                  ),
                );
              },
            ),
          ),
            const IncomingCallOverlay(),
            const OutgoingCallOverlay(),
            const SafeArea(child: ActiveCallPanel()),
          ],
        ),
      ),
      ),
    );
  }
}

class _MobileRailStrip extends StatelessWidget {
  const _MobileRailStrip({
    required this.onOpenSocial,
    this.active = false,
    this.badgeCount = 0,
  });

  final VoidCallback onOpenSocial;
  final bool active;
  final int badgeCount;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    return ColoredBox(
      color: voice.muted,
      child: Align(
        alignment: Alignment.centerLeft,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8),
          child: _SocialNavButton(
            buttonKey: const Key('nav_open_social_mobile'),
            tooltip: l10n.socialRailTooltip,
            active: active,
            badgeCount: badgeCount,
            onPressed: onOpenSocial,
          ),
        ),
      ),
    );
  }
}

class _SocialRail extends StatelessWidget {
  const _SocialRail({
    required this.onOpenSocial,
    this.active = false,
    this.badgeCount = 0,
  });

  final VoidCallback onOpenSocial;
  final bool active;
  final int badgeCount;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    return ColoredBox(
      color: voice.muted,
      child: Column(
        children: [
          const SizedBox(height: 8),
          _SocialNavButton(
            buttonKey: const Key('nav_open_social'),
            tooltip: l10n.socialRailTooltip,
            active: active,
            badgeCount: badgeCount,
            onPressed: onOpenSocial,
          ),
          const SizedBox(height: 8),
          Tooltip(
            message: l10n.chatListTitle,
            child: Icon(
              Icons.chat_bubble_outline,
              size: 20,
              color: voice.textDisabled,
            ),
          ),
        ],
      ),
    );
  }
}

class _SocialNavButton extends StatelessWidget {
  const _SocialNavButton({
    required this.buttonKey,
    required this.tooltip,
    required this.onPressed,
    this.active = false,
    this.badgeCount = 0,
  });

  final Key buttonKey;
  final String tooltip;
  final VoidCallback onPressed;
  final bool active;
  final int badgeCount;

  @override
  Widget build(BuildContext context) {
    final voice = VoiceColors.of(context);
    final iconColor = active ? voice.profileAccent : voice.textSecondary;
    return Stack(
      clipBehavior: Clip.none,
      children: [
        IconButton(
          key: buttonKey,
          tooltip: tooltip,
          onPressed: onPressed,
          style: active
              ? IconButton.styleFrom(backgroundColor: voice.elevated)
              : null,
          icon: Icon(Icons.people_outline, color: iconColor),
        ),
        if (badgeCount > 0)
          Positioned(
            right: 4,
            top: 4,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 1),
              decoration: BoxDecoration(
                color: voice.profileAccent,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Text(
                badgeCount > 9 ? '9+' : '$badgeCount',
                style: TextStyle(
                  fontSize: 10,
                  color: Theme.of(context).colorScheme.onPrimary,
                ),
              ),
            ),
          ),
      ],
    );
  }
}

class _SessionBar extends StatelessWidget {
  const _SessionBar({
    required this.onLogout,
    required this.onEditProfile,
    required this.onOpenSettings,
    required this.sessionLabel,
    required this.logoutLabel,
    required this.editProfileTooltip,
    required this.settingsTooltip,
  });

  final VoidCallback onLogout;
  final VoidCallback? onEditProfile;
  final VoidCallback onOpenSettings;
  final String sessionLabel;
  final String logoutLabel;
  final String editProfileTooltip;
  final String settingsTooltip;

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
              IconButton(
                key: const Key('settings_open'),
                tooltip: settingsTooltip,
                onPressed: onOpenSettings,
                icon: Icon(Icons.settings_outlined, color: voice.textSecondary),
              ),
              IconButton(
                key: const Key('profile_edit_open'),
                tooltip: editProfileTooltip,
                onPressed: onEditProfile,
                icon: Icon(Icons.edit_outlined, color: voice.textSecondary),
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

  static bool shouldShow(AsyncValue<GatewayHealthResult> asyncHealth) {
    return asyncHealth.when(
      data: (r) => r is! GatewayHealthOk,
      loading: () => false,
      error: (_, _) => true,
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final text = asyncHealth.when(
      data: (r) => switch (r) {
        GatewayHealthOk() => l10n.gatewayStatusOk,
        GatewayHealthFailure(:final message) =>
          message == kGatewayMissingBaseUrlDetail
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
