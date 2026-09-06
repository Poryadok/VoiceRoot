import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// Persists the auto-generated password for guest accounts (convert-guest later)
/// and whether the guest has chosen a display nickname.
abstract class GuestCredentialsStorage {
  Future<String?> readPassword();
  Future<void> writePassword(String password);
  Future<String?> readPendingConversionEmail();
  Future<void> writePendingConversionEmail(String email);
  Future<void> clearPendingConversionEmail();
  Future<bool> isGuestConversionPromotionPending();
  Future<void> setGuestConversionPromotionPending(bool pending);
  Future<bool> isNicknameCompleted(String accountId);
  Future<void> markNicknameCompleted(String accountId);
  Future<GuestCredentialsSnapshot> snapshot();
  Future<bool> clearIfUnchanged(GuestCredentialsSnapshot snapshot);
  Future<void> clear();
}

/// The mutable credentials that identify the guest conversion currently in
/// progress. A promotion only clears this snapshot, so a newer guest flow is
/// never erased by a delayed earlier promotion.
class GuestCredentialsSnapshot {
  const GuestCredentialsSnapshot({
    required this.password,
    required this.pendingConversionEmail,
    required this.promotionPending,
  });

  final String? password;
  final String? pendingConversionEmail;
  final bool promotionPending;

  @override
  bool operator ==(Object other) =>
      other is GuestCredentialsSnapshot &&
      password == other.password &&
      pendingConversionEmail == other.pendingConversionEmail &&
      promotionPending == other.promotionPending;

  @override
  int get hashCode =>
      Object.hash(password, pendingConversionEmail, promotionPending);
}

/// True when the profile still has the server-assigned placeholder display name.
bool isPlaceholderGuestDisplayName({
  required String accountId,
  required String displayName,
}) {
  String norm(String s) => s.replaceAll('-', '').toLowerCase();
  return norm(displayName) == norm(accountId);
}

class FlutterGuestCredentialsStorage implements GuestCredentialsStorage {
  FlutterGuestCredentialsStorage({
    FlutterSecureStorage? storage,
    SharedPreferences? prefs,
  }) : _storage = storage ?? const FlutterSecureStorage(),
       _prefs = prefs;

  static const _passwordKey = 'voice.auth.guest_password';
  static const _pendingConversionEmailKey =
      'voice.auth.guest_pending_conversion_email';
  static const _pendingConversionPromotionKey =
      'voice.auth.guest_pending_conversion_promotion';
  static const _nicknameKeyPrefix = 'voice.auth.guest_nickname_done.';

  final FlutterSecureStorage _storage;
  final SharedPreferences? _prefs;
  Future<void> _operations = Future<void>.value();

  Future<T> _serialize<T>(Future<T> Function() operation) {
    final result = _operations.then((_) => operation());
    _operations = result.then<void>((_) {}, onError: (error, stackTrace) {});
    return result;
  }

  String _nicknameKey(String accountId) => '$_nicknameKeyPrefix$accountId';

  @override
  Future<void> clear() async {
    await _serialize(() async {
      await _storage.delete(key: _passwordKey);
      await _storage.delete(key: _pendingConversionEmailKey);
      await _storage.delete(key: _pendingConversionPromotionKey);
      final prefs = _prefs;
      if (prefs == null) return;
      final keys = prefs
          .getKeys()
          .where((key) => key.startsWith(_nicknameKeyPrefix))
          .toList();
      for (final key in keys) {
        await prefs.remove(key);
      }
    });
  }

  @override
  Future<bool> clearIfUnchanged(GuestCredentialsSnapshot snapshot) =>
      _serialize(() async {
        final current = GuestCredentialsSnapshot(
          password: await _storage.read(key: _passwordKey),
          pendingConversionEmail: await _storage.read(
            key: _pendingConversionEmailKey,
          ),
          promotionPending:
              await _storage.read(key: _pendingConversionPromotionKey) ==
              'true',
        );
        if (current != snapshot) return false;
        await _storage.delete(key: _passwordKey);
        await _storage.delete(key: _pendingConversionEmailKey);
        await _storage.delete(key: _pendingConversionPromotionKey);
        final prefs = _prefs;
        if (prefs != null) {
          final keys = prefs
              .getKeys()
              .where((key) => key.startsWith(_nicknameKeyPrefix))
              .toList();
          for (final key in keys) {
            await prefs.remove(key);
          }
        }
        return true;
      });

  @override
  Future<GuestCredentialsSnapshot> snapshot() => _serialize(() async {
    return GuestCredentialsSnapshot(
      password: await _storage.read(key: _passwordKey),
      pendingConversionEmail: await _storage.read(
        key: _pendingConversionEmailKey,
      ),
      promotionPending:
          await _storage.read(key: _pendingConversionPromotionKey) == 'true',
    );
  });

  @override
  Future<bool> isNicknameCompleted(String accountId) async {
    return _prefs?.getBool(_nicknameKey(accountId)) ?? false;
  }

  @override
  Future<void> markNicknameCompleted(String accountId) async {
    await _prefs?.setBool(_nicknameKey(accountId), true);
  }

  @override
  Future<String?> readPassword() =>
      _serialize(() => _storage.read(key: _passwordKey));

  @override
  Future<String?> readPendingConversionEmail() =>
      _serialize(() => _storage.read(key: _pendingConversionEmailKey));

  @override
  Future<void> writePendingConversionEmail(String email) => _serialize(
    () => _storage.write(key: _pendingConversionEmailKey, value: email),
  );

  @override
  Future<void> clearPendingConversionEmail() =>
      _serialize(() => _storage.delete(key: _pendingConversionEmailKey));

  @override
  Future<bool> isGuestConversionPromotionPending() async =>
      await _serialize(
        () => _storage.read(key: _pendingConversionPromotionKey),
      ) ==
      'true';

  @override
  Future<void> setGuestConversionPromotionPending(bool pending) => pending
      ? _serialize(
          () => _storage.write(
            key: _pendingConversionPromotionKey,
            value: 'true',
          ),
        )
      : _serialize(() => _storage.delete(key: _pendingConversionPromotionKey));

  @override
  Future<void> writePassword(String password) =>
      _serialize(() => _storage.write(key: _passwordKey, value: password));
}

class InMemoryGuestCredentialsStorage implements GuestCredentialsStorage {
  String? _password;
  String? _pendingConversionEmail;
  var _pendingConversionPromotion = false;
  final Map<String, bool> _nicknameCompleted = {};
  Future<void> _operations = Future<void>.value();

  Future<T> _serialize<T>(Future<T> Function() operation) {
    final result = _operations.then((_) => operation());
    _operations = result.then<void>((_) {}, onError: (error, stackTrace) {});
    return result;
  }

  @override
  Future<void> clear() => _serialize(() async {
    _password = null;
    _pendingConversionEmail = null;
    _pendingConversionPromotion = false;
    _nicknameCompleted.clear();
  });

  @override
  Future<bool> clearIfUnchanged(GuestCredentialsSnapshot snapshot) =>
      _serialize(() async {
        final current = GuestCredentialsSnapshot(
          password: _password,
          pendingConversionEmail: _pendingConversionEmail,
          promotionPending: _pendingConversionPromotion,
        );
        if (current != snapshot) return false;
        _password = null;
        _pendingConversionEmail = null;
        _pendingConversionPromotion = false;
        _nicknameCompleted.clear();
        return true;
      });

  @override
  Future<bool> isNicknameCompleted(String accountId) => _serialize(() async {
    return _nicknameCompleted[accountId] ?? false;
  });

  @override
  Future<void> markNicknameCompleted(String accountId) => _serialize(() async {
    _nicknameCompleted[accountId] = true;
  });

  @override
  Future<String?> readPassword() => _serialize(() async => _password);

  @override
  Future<String?> readPendingConversionEmail() =>
      _serialize(() async => _pendingConversionEmail);

  @override
  Future<void> writePendingConversionEmail(String email) =>
      _serialize(() async {
        _pendingConversionEmail = email;
      });

  @override
  Future<void> clearPendingConversionEmail() => _serialize(() async {
    _pendingConversionEmail = null;
  });

  @override
  Future<bool> isGuestConversionPromotionPending() =>
      _serialize(() async => _pendingConversionPromotion);

  @override
  Future<void> setGuestConversionPromotionPending(bool pending) =>
      _serialize(() async {
        _pendingConversionPromotion = pending;
      });

  @override
  Future<void> writePassword(String password) => _serialize(() async {
    _password = password;
  });

  @override
  Future<GuestCredentialsSnapshot> snapshot() => _serialize(() async {
    return GuestCredentialsSnapshot(
      password: _password,
      pendingConversionEmail: _pendingConversionEmail,
      promotionPending: _pendingConversionPromotion,
    );
  });
}
