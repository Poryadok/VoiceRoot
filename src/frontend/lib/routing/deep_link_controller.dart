import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/auth_providers.dart';
import 'deep_link_parser.dart';

class DeepLinkState {
  const DeepLinkState({this.pending, this.resolved});

  final DeepLinkTarget? pending;
  final DeepLinkTarget? resolved;

  DeepLinkState copyWith({
    DeepLinkTarget? pending,
    DeepLinkTarget? resolved,
    bool clearPending = false,
    bool clearResolved = false,
  }) {
    return DeepLinkState(
      pending: clearPending ? null : (pending ?? this.pending),
      resolved: clearResolved ? null : (resolved ?? this.resolved),
    );
  }
}

class DeepLinkController extends Notifier<DeepLinkState> {
  @override
  DeepLinkState build() => const DeepLinkState();

  Future<void> onIncomingLink(DeepLinkTarget target) async {
    final authed = ref.read(authControllerProvider).isAuthenticated;
    if (authed) {
      state = state.copyWith(resolved: target, clearPending: true);
      return;
    }
    state = state.copyWith(pending: target, clearResolved: true);
  }

  Future<void> flushPendingAfterAuth() async {
    final pending = state.pending;
    if (pending == null) return;
    state = state.copyWith(resolved: pending, clearPending: true);
  }

  void clearResolved() {
    state = state.copyWith(clearResolved: true);
  }
}

final deepLinkControllerProvider =
    NotifierProvider<DeepLinkController, DeepLinkState>(DeepLinkController.new);
