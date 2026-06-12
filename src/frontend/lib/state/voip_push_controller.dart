import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'auth_providers.dart';
import 'call_providers.dart';
import 'callkit_incoming_handler.dart';
import 'push_notifications_bootstrap.dart';
import 'push_notifications_controller.dart';
import 'push_platform.dart';
import 'voip_push_platform.dart';

const _voipChannel = MethodChannel('voice/voip');

final voipPushControllerProvider = Provider<VoIPPushController>((ref) {
  final controller = VoIPPushController(ref);
  ref.onDispose(controller.dispose);
  return controller;
});

/// Registers PushKit VoIP tokens and bridges CallKit actions to [CallController].
class VoIPPushController {
  VoIPPushController(this._ref) {
    if (!isVoIPPushSupported) return;
    _authSub = _ref.listen<AuthState>(authControllerProvider, (prev, next) {
      if (next.isAuthenticated &&
          (prev?.session?.accessToken != next.session?.accessToken)) {
        unawaited(_startListening());
      }
      if (!next.isAuthenticated && prev?.isAuthenticated == true) {
        unawaited(_stopListening());
      }
    }, fireImmediately: true);
  }

  final Ref _ref;
  ProviderSubscription<AuthState>? _authSub;

  void dispose() {
    _authSub?.close();
    unawaited(_stopListening());
  }

  Future<void> _startListening() async {
    _voipChannel.setMethodCallHandler(_onNativeCall);
  }

  Future<void> _stopListening() async {
    await _voipChannel.invokeMethod<void>('stop');
    _voipChannel.setMethodCallHandler(null);
  }

  Future<void> _onNativeCall(MethodCall call) async {
    switch (call.method) {
      case 'onVoIPToken':
        await _registerVoIPToken('${call.arguments}');
      case 'onCallAccepted':
        await _handleCallAccepted(call.arguments);
      case 'onCallDeclined':
        await _handleCallDeclined(call.arguments);
      case 'onIncomingCall':
        _handleIncomingPayload(call.arguments);
    }
  }

  Future<void> _registerVoIPToken(String token) async {
    if (token.isEmpty) return;
    final header = _ref.read(authControllerProvider).session?.authorizationHeader;
    if (header == null) return;
    const bootstrap = PushNotificationsBootstrap();
    await bootstrap.registerToken(
      client: _ref.read(voiceNotificationsClientProvider),
      authorization: header,
      platform: pushPlatformForTarget(),
      token: token,
      pushService: voipPushServiceForTarget()!,
    );
  }

  void _handleIncomingPayload(dynamic arguments) {
    if (arguments is! Map) return;
    final parsed = parseIncomingCallPayload(arguments);
    if (parsed == null) return;
    final session = sessionFromVoIPPayload(parsed);
    if (session == null) return;

    final callState = _ref.read(callControllerProvider);
    if (shouldIgnoreDuplicateIncoming(
      roomId: session.roomId,
      currentRoomId: callState.session?.roomId,
      isIncomingPhase: callState.phase == CallPhase.incoming,
    )) {
      return;
    }
    _ref.read(callControllerProvider.notifier).applyIncomingFromVoIP(session);
  }

  Future<void> _handleCallAccepted(dynamic arguments) async {
    if (arguments is Map) {
      _handleIncomingPayload(arguments);
    }
    await _ref.read(callControllerProvider.notifier).acceptCall();
  }

  Future<void> _handleCallDeclined(dynamic arguments) async {
    if (arguments is Map) {
      _handleIncomingPayload(arguments);
    }
    await _ref.read(callControllerProvider.notifier).declineCall();
  }

  @visibleForTesting
  Future<void> handleMethodCall(MethodCall call) => _onNativeCall(call);
}
