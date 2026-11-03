import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../l10n/app_localizations.dart';
import '../../state/call_providers.dart';
import '../../state/gateway_providers.dart';
import '../../state/social_providers.dart';
import '../../theme/voice_colors.dart';
import '../a11y/focus_trap.dart';
import '../a11y/voice_focus_return.dart';
import '../core/voice_avatar.dart';

class OutgoingCallOverlay extends ConsumerStatefulWidget {
  const OutgoingCallOverlay({super.key});

  static const Key overlayKey = Key('outgoing_call_overlay');
  static const Key cancelKey = Key('outgoing_call_cancel');

  @override
  ConsumerState<OutgoingCallOverlay> createState() =>
      _OutgoingCallOverlayState();
}

class _OutgoingCallOverlayState extends ConsumerState<OutgoingCallOverlay> {
  VoiceFocusReturn? _focusReturn;
  Timer? _timer;
  int _elapsed = 0;

  Widget _hidden() {
    _timer?.cancel();
    _timer = null;
    _focusReturn?.restore();
    _focusReturn = null;
    return const SizedBox.shrink();
  }

  @override
  void dispose() {
    _timer?.cancel();
    _focusReturn?.restore();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (!ref.watch(gatewayConfigProvider).canPlaceVoiceCalls) {
      return _hidden();
    }
    final call = ref.watch(callControllerProvider);
    final session = call.session;
    if (session?.isGroupVoice == true) {
      return _hidden();
    }
    final calleeProfileId =
        session?.calleeProfileId ?? call.outgoingCalleeProfileId;
    if (!call.isOutgoing ||
        calleeProfileId == null ||
        calleeProfileId.isEmpty) {
      return _hidden();
    }
    _focusReturn ??= VoiceFocusReturn.capture();
    if (_timer == null) {
      _elapsed = 0;
      _timer = Timer.periodic(const Duration(seconds: 1), (_) {
        if (mounted) setState(() => _elapsed++);
      });
    }

    final l10n = AppLocalizations.of(context)!;
    final voice = VoiceColors.of(context);
    final callee = ref.watch(profileProvider(calleeProfileId)).valueOrNull;
    final title = callee?.displayName ?? calleeProfileId;

    final video =
        session?.mediaKind.name == 'video' ||
        (session == null && call.outgoingMediaKind?.name == 'video');
    final controller = ref.read(callControllerProvider.notifier);
    final muteLabel = call.isMuted ? l10n.callUnmute : l10n.callMute;
    final cameraLabel = call.isVideoEnabled
        ? l10n.callVideoOff
        : l10n.callVideoOn;
    final elapsed =
        '${(_elapsed ~/ 60).toString().padLeft(2, '0')}:${(_elapsed % 60).toString().padLeft(2, '0')}';
    Widget control(
      String label,
      IconData icon,
      VoidCallback onPressed, {
      bool selected = false,
      bool destructive = false,
      Key? key,
    }) => IconButton.filled(
      key: key,
      tooltip: label,
      isSelected: selected,
      onPressed: onPressed,
      style: IconButton.styleFrom(
        minimumSize: const Size(36, 36),
        fixedSize: const Size(36, 36),
        tapTargetSize: MaterialTapTargetSize.padded,
        backgroundColor: destructive ? Colors.transparent : voice.callControl,
        foregroundColor: voice.callForeground,
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      ),
      icon: Icon(icon, size: 20),
    );
    return Positioned.fill(
      key: OutgoingCallOverlay.overlayKey,
      child: VoiceFocusTrap(
        child: Material(
          color: voice.callBackground,
          child: DecoratedBox(
            decoration: BoxDecoration(color: voice.callBackground),
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
                              spreadRadius: 7,
                            ),
                          ],
                        ),
                        child: VoiceAvatar(
                          imageUrl: callee?.avatarUrl,
                          label: title,
                          radius: 40,
                        ),
                      ),
                      const SizedBox(height: 22),
                      Text(
                        title,
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          color: voice.callForeground,
                          fontSize: 20,
                          height: 1.4,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      const SizedBox(height: 18),
                      Text(
                        video ? l10n.callIncomingVideo : l10n.callIncomingAudio,
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          color: voice.callForeground,
                          fontSize: 15,
                        ),
                      ),
                      const SizedBox(height: 14),
                      Text(
                        '${session == null ? l10n.callConnecting : l10n.callOutgoingTitle(title)} $elapsed',
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          color: voice.callSecondary,
                          fontSize: 12,
                        ),
                      ),
                      const SizedBox(height: 28),
                      Wrap(
                        alignment: WrapAlignment.center,
                        spacing: 8,
                        runSpacing: 8,
                        children: [
                          control(
                            muteLabel,
                            call.isMuted ? Icons.mic_off : Icons.mic,
                            () => controller.setMuted(!call.isMuted),
                            selected: call.isMuted,
                          ),
                          if (video)
                            control(
                              cameraLabel,
                              call.isVideoEnabled
                                  ? Icons.videocam
                                  : Icons.videocam_off,
                              () => controller.setVideoEnabled(
                                !call.isVideoEnabled,
                              ),
                              selected: call.isVideoEnabled,
                            ),
                          control(
                            l10n.callHangup,
                            Icons.call_end,
                            controller.hangUp,
                            destructive: true,
                            key: OutgoingCallOverlay.cancelKey,
                          ),
                        ],
                      ),
                      const SizedBox(height: 14),
                      Text(
                        [
                          muteLabel,
                          if (video) cameraLabel,
                          l10n.callHangup,
                        ].join(' · '),
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          color: voice.callSecondary,
                          fontSize: 12,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
