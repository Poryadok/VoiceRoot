import 'package:web/web.dart' as web;

import '../state/linked_oauth_return.dart';

/// Tab persistence only. Does not store the JWT or the returned result handle.
class BrowserLinkedOAuthTabStore implements LinkedOAuthTabStore {
  static const _key = 'voice.linked-oauth-return.v1';
  @override
  String? read() => web.window.sessionStorage.getItem(_key);
  @override
  void write(String value) => web.window.sessionStorage.setItem(_key, value);
  @override
  void clear() => web.window.sessionStorage.removeItem(_key);
}

/// Invoke at the browser entry boundary before routing or analytics. Retain the
/// returned URI privately for controller.handleReturn; never forward/log it.
Uri captureLinkedOAuthBrowserReturn() {
  final raw = Uri.parse(web.window.location.href);
  scrubLinkedOAuthBrowserLocation(stripLinkedOAuthReturnSecrets(raw));
  return raw;
}

void scrubLinkedOAuthBrowserLocation(Uri cleanUri) {
  web.window.history.replaceState(null, '', cleanUri.toString());
}

Future<bool> navigateLinkedOAuthSameTab(Uri authorizationUri) async {
  web.window.location.assign(authorizationUri.toString());
  return true;
}
