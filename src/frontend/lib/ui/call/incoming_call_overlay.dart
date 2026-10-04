import 'package:flutter/foundation.dart'
    show TargetPlatform, defaultTargetPlatform, kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../backend/voice_client.dart' show VoiceCallMediaKind;
import '../../l10n/app_localizations.dart';
import '../../state/call_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../a11y/focus_trap.dart';
import '../a11y/voice_focus_return.dart';
import '../core/voice_avatar.dart';

class IncomingCallOverlay extends ConsumerStatefulWidget {
  const IncomingCallOverlay({super.key});

  static const Key overlayKey = Key('incoming_call_overlay');
  static const Key acceptKey = Key('incoming_call_accept');
  static const Key declineKey = Key('incoming_call_decline');

  @override
  ConsumerState<IncomingCallOverlay> createState() =>
      _IncomingCallOverlayState();
}

class _IncomingCallOverlayState extends ConsumerState<IncomingCallOverlay> {
  VoiceFocusReturn? _focusReturn;
  bool _wasVisible = false;

  void _updateFocusReturn(bool visible) {
    if (_wasVisible == visible) return;
    _wasVisible = visible;

    if (visible) {
      _focusReturn = VoiceFocusReturn.capture();
      return;
    }

    final focusReturn = _focusReturn;
    _focusReturn = null;
    focusReturn?.restore();
  }

  Widget _hidden() {
    _updateFocusReturn(false);
    return const SizedBox.shrink();
  }

  @override
  void dispose() {
    _focusReturn?.restore();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // Native iOS already presents the incoming call through CallKit. Keep the
    // Flutter overlay on iOS web, where the native call UI is unavailable.
    if (!kIsWeb && defaultTargetPlatform == TargetPlatform.iOS) {
      return _hidden();
    }
    if (!ref.watch(gatewayConfigProvider).canPlaceVoiceCalls) {
      return _hidden();
    }
    final call = ref.watch(callControllerProvider);
    final session = call.session;
    if (!call.isIncoming || session == null) {
      return _hidden();
    }
    _updateFocusReturn(true);

    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final caller = ref
        .watch(profileProvider(session.initiatorProfileId))
        .valueOrNull;
    final title = caller?.displayName ?? session.initiatorProfileId;
    final isVideo = session.mediaKind == VoiceCallMediaKind.video;

    return Positioned.fill(
      key: IncomingCallOverlay.overlayKey,
      child: VoiceFocusTrap(
        child: Material(
          color: voice.canvas,
          child: SafeArea(
            child: Center(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Container(
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        boxShadow: [
                          BoxShadow(
                            color: voice.focusRing.withValues(alpha: 0.25),
                            blurRadius: 0,
                            spreadRadius: 7,
                          ),
                        ],
                      ),
                      child: VoiceAvatar(
                        imageUrl: caller?.avatarUrl,
                        label: title,
                        radius: 38,
                      ),
                    ),
                    const SizedBox(height: 14),
                    Text(
                      title,
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.titleLarge,
                    ),
                    const SizedBox(height: 14),
                    Text(
                      isVideo ? l10n.callIncomingVideo : l10n.callIncomingAudio,
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        color: voice.textPrimary,
                      ),
                    ),
                    const SizedBox(height: 14),
                    Text(
                      l10n.callIncomingTitle(title),
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        color: voice.textSecondary,
                      ),
                    ),
                    const SizedBox(height: 14),
                    Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        IconButton.filled(
                          key: IncomingCallOverlay.declineKey,
                          tooltip: l10n.callDecline,
                          onPressed: () => ref
                              .read(callControllerProvider.notifier)
                              .declineCall(),
                          style: IconButton.styleFrom(
                            backgroundColor: voice.error,
                            foregroundColor: voice.textPrimary,
                          ),
                          icon: const Icon(Icons.call_end),
                        ),
                        const SizedBox(width: 14),
                        IconButton.filled(
                          key: IncomingCallOverlay.acceptKey,
                          tooltip: l10n.callAccept,
                          onPressed: () => ref
                              .read(callControllerProvider.notifier)
                              .acceptCall(),
                          style: IconButton.styleFrom(
                            backgroundColor: voice.success,
                            foregroundColor: voice.textPrimary,
                          ),
                          icon: Icon(isVideo ? Icons.videocam : Icons.call),
                        ),
                      ],
                    ),
                    const SizedBox(height: 14),
                    Text(
                      '${l10n.callDecline} · ${l10n.callAccept}',
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        color: voice.textSecondary,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
