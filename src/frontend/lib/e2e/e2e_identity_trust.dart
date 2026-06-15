import 'dart:typed_data';

import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Tracks peer identity keys and user trust decisions for E2E DMs.
class E2eIdentityTrustState {
  const E2eIdentityTrustState({
    this.knownIdentityKeys = const {},
    this.pendingKeyChangePeers = const {},
    this.distrustedPeers = const {},
  });

  final Map<String, Uint8List> knownIdentityKeys;
  final Set<String> pendingKeyChangePeers;
  final Set<String> distrustedPeers;

  E2eIdentityTrustState copyWith({
    Map<String, Uint8List>? knownIdentityKeys,
    Set<String>? pendingKeyChangePeers,
    Set<String>? distrustedPeers,
  }) {
    return E2eIdentityTrustState(
      knownIdentityKeys: knownIdentityKeys ?? this.knownIdentityKeys,
      pendingKeyChangePeers:
          pendingKeyChangePeers ?? this.pendingKeyChangePeers,
      distrustedPeers: distrustedPeers ?? this.distrustedPeers,
    );
  }
}

class E2eIdentityTrustNotifier extends StateNotifier<E2eIdentityTrustState> {
  E2eIdentityTrustNotifier() : super(const E2eIdentityTrustState());

  void notePeerIdentityKey({
    required String peerProfileId,
    required Uint8List identityKeyBytes,
  }) {
    final known = state.knownIdentityKeys[peerProfileId];
    if (known == null) {
      state = state.copyWith(
        knownIdentityKeys: {
          ...state.knownIdentityKeys,
          peerProfileId: Uint8List.fromList(identityKeyBytes),
        },
      );
      return;
    }
    if (!_bytesEqual(known, identityKeyBytes)) {
      state = state.copyWith(
        pendingKeyChangePeers: {...state.pendingKeyChangePeers, peerProfileId},
      );
    }
  }

  void acceptKeyChange(String peerProfileId, Uint8List identityKeyBytes) {
    final pending = Set<String>.from(state.pendingKeyChangePeers)
      ..remove(peerProfileId);
    final distrusted = Set<String>.from(state.distrustedPeers)
      ..remove(peerProfileId);
    state = state.copyWith(
      pendingKeyChangePeers: pending,
      distrustedPeers: distrusted,
      knownIdentityKeys: {
        ...state.knownIdentityKeys,
        peerProfileId: Uint8List.fromList(identityKeyBytes),
      },
    );
  }

  void distrustPeer(String peerProfileId) {
    final pending = Set<String>.from(state.pendingKeyChangePeers)
      ..remove(peerProfileId);
    final distrusted = Set<String>.from(state.distrustedPeers)
      ..add(peerProfileId);
    state = state.copyWith(
      pendingKeyChangePeers: pending,
      distrustedPeers: distrusted,
    );
  }

  bool isDistrusted(String peerProfileId) =>
      state.distrustedPeers.contains(peerProfileId);

  bool hasPendingKeyChange(String peerProfileId) =>
      state.pendingKeyChangePeers.contains(peerProfileId);

  static bool _bytesEqual(Uint8List a, Uint8List b) {
    if (a.length != b.length) return false;
    for (var i = 0; i < a.length; i++) {
      if (a[i] != b[i]) return false;
    }
    return true;
  }
}

final e2eIdentityTrustProvider =
    StateNotifierProvider<E2eIdentityTrustNotifier, E2eIdentityTrustState>(
  (ref) => E2eIdentityTrustNotifier(),
);
