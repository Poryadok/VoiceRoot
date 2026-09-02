import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/auth_client.dart';
import '../../l10n/app_localizations.dart';
import '../../state/auth_providers.dart';
import '../../theme/voice_colors.dart';
import '../core/voice_state_panel.dart';

/// Lists active refresh-token sessions and allows revoking other devices.
class ActiveSessionsScreen extends ConsumerStatefulWidget {
  const ActiveSessionsScreen({super.key});

  static const Key screenKey = Key('active_sessions_screen');
  static const Key listKey = Key('active_sessions_list');
  static Key sessionRowKey(String sessionId) =>
      Key('active_session_row_$sessionId');
  static Key revokeButtonKey(String sessionId) =>
      Key('active_session_revoke_$sessionId');

  @override
  ConsumerState<ActiveSessionsScreen> createState() =>
      _ActiveSessionsScreenState();
}

class _ActiveSessionsScreenState extends ConsumerState<ActiveSessionsScreen> {
  List<AuthDeviceSession>? _sessions;
  var _loading = true;
  String? _error;
  String? _revokingId;

  @override
  void initState() {
    super.initState();
    _loadSessions();
  }

  Future<void> _loadSessions() async {
    final session = ref.read(authControllerProvider).session;
    if (session == null) return;

    setState(() {
      _loading = true;
      _error = null;
    });

    final result = await ref
        .read(voiceAuthClientProvider)
        .listSessions(session: session);

    if (!mounted) return;
    switch (result) {
      case AuthApiOk(:final data):
        setState(() {
          _sessions = data;
          _loading = false;
        });
      case AuthApiFailure(:final message):
        setState(() {
          _loading = false;
          _error = message;
        });
    }
  }

  Future<void> _revokeSession(AuthDeviceSession deviceSession) async {
    if (deviceSession.current) return;
    final authSession = ref.read(authControllerProvider).session;
    if (authSession == null) return;

    setState(() => _revokingId = deviceSession.id);

    final result = await ref.read(voiceAuthClientProvider).revokeSession(
      session: authSession,
      sessionId: deviceSession.id,
    );

    if (!mounted) return;
    setState(() => _revokingId = null);

    final l10n = AppLocalizations.of(context)!;
    switch (result) {
      case AuthApiOk<void>():
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.securitySessionsRevoked)),
        );
        await _loadSessions();
      case AuthApiFailure(:final message):
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(message)),
        );
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final sessions = _sessions;

    return Scaffold(
      key: ActiveSessionsScreen.screenKey,
      backgroundColor: voice.canvas,
      appBar: AppBar(
        title: Text(l10n.securitySessionsTitle),
        backgroundColor: voice.surface,
      ),
      body: SafeArea(
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _error != null
            ? VoiceStatePanel(
                title: l10n.securitySessionsLoadError,
                message: _error,
                icon: Icons.devices_other_outlined,
                actionLabel: l10n.commonRetry,
                onAction: _loadSessions,
              )
            : sessions == null || sessions.isEmpty
            ? VoiceStatePanel(
                title: l10n.securitySessionsEmpty,
                icon: Icons.devices_other_outlined,
              )
            : ListView.separated(
                key: ActiveSessionsScreen.listKey,
                padding: const EdgeInsets.all(16),
                itemCount: sessions.length,
                separatorBuilder: (_, _) => const SizedBox(height: 8),
                itemBuilder: (context, index) {
                  final deviceSession = sessions[index];
                  final revoking = _revokingId == deviceSession.id;
                  return Material(
                    key: ActiveSessionsScreen.sessionRowKey(deviceSession.id),
                    color: voice.surface,
                    borderRadius: BorderRadius.circular(8),
                    child: ListTile(
                      title: Text(deviceSession.deviceLabel),
                      subtitle: Text(
                        deviceSession.current
                            ? l10n.securitySessionsCurrentDevice
                            : l10n.securitySessionsOtherDevice,
                      ),
                      trailing: deviceSession.current
                          ? Chip(
                              label: Text(l10n.securitySessionsCurrentBadge),
                            )
                          : TextButton(
                              key: ActiveSessionsScreen.revokeButtonKey(
                                deviceSession.id,
                              ),
                              onPressed: revoking
                                  ? null
                                  : () => _revokeSession(deviceSession),
                              child: Text(l10n.securitySessionsRevoke),
                            ),
                    ),
                  );
                },
              ),
      ),
    );
  }
}
