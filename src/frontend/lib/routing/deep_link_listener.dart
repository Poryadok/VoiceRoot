import 'package:app_links/app_links.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'deep_link_controller.dart';
import 'deep_link_parser.dart';

/// Subscribes to platform / web deep links and forwards to [DeepLinkController].
class DeepLinkListener {
  DeepLinkListener(this._ref);

  final Ref _ref;
  final _appLinks = AppLinks();

  Future<void> start() async {
    try {
      final initial = await _appLinks.getInitialLink();
      if (initial != null) {
        _handleUri(initial);
      }
    } catch (e) {
      if (kDebugMode) {
        debugPrint('deep link initial: $e');
      }
    }

    _appLinks.uriLinkStream.listen(
      _handleUri,
      onError: (Object e) {
        if (kDebugMode) debugPrint('deep link stream: $e');
      },
    );
  }

  void _handleUri(Uri uri) {
    try {
      final target = parseDeepLinkUrl(uri.toString());
      _ref.read(deepLinkControllerProvider.notifier).onIncomingLink(target);
    } catch (_) {
      // Ignore malformed links.
    }
  }
}

final deepLinkListenerProvider = Provider<DeepLinkListener>((ref) {
  return DeepLinkListener(ref);
});
