@TestOn('browser')
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:web/web.dart' as web;
import 'package:voice_frontend/services/linked_oauth_browser_web.dart';

void main() {
  test(
    'real browser history removes result query and hostile fragment synchronously',
    () {
      final original = web.window.location.href;
      addTearDown(() => web.window.history.replaceState(null, '', original));
      final route = Uri.parse(original).replace(query: '', fragment: '');
      web.window.history.replaceState(
        null,
        '',
        '${route.toString().split('?').first.split('#').first}?attempt_id=private&result_handle=private#access_token=secret',
      );
      final captured = captureLinkedOAuthBrowserReturn();
      expect(captured.fragment, 'access_token=secret');
      final visible = web.window.location.href;
      expect(visible, isNot(contains('private')));
      expect(visible, isNot(contains('secret')));
      expect(visible, endsWith('#'));
      expect(Uri.parse(visible).hasQuery, isFalse);
      expect(Uri.parse(visible).fragment, isEmpty);
    },
  );

  test(
    'real browser preserves explicit empty fragment and uses tab-only storage',
    () {
      final original = web.window.location.href;
      addTearDown(() => web.window.history.replaceState(null, '', original));
      web.window.history.replaceState(
        null,
        '',
        '${Uri.parse(original).toString().split(RegExp(r'[?#]')).first}?attempt_id=private&result_handle=private#',
      );
      final captured = captureLinkedOAuthBrowserReturn();
      expect(captured.hasFragment, isTrue);
      expect(captured.fragment, isEmpty);
      expect(web.window.location.href, endsWith('#'));
      final store = BrowserLinkedOAuthTabStore();
      addTearDown(store.clear);
      store.write('tab-local-proof');
      expect(BrowserLinkedOAuthTabStore().read(), 'tab-local-proof');
      expect(
        web.window.localStorage.getItem('voice.linked-oauth-return.v1'),
        isNull,
      );
      store.clear();
      expect(store.read(), isNull);
    },
  );
}
