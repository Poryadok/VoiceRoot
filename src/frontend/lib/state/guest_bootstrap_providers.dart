import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// When true, unauthenticated bootstrap auto-calls registerGuest (web product default).
final webGuestAutoRegisterEnabledProvider = Provider<bool>(
  (ref) => kIsWeb,
);
