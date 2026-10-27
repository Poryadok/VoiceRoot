import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/chat/link_preview.dart';

void main() {
  Widget wrap(String url, http.Client client) => MaterialApp(
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    home: Scaffold(
      body: LinkPreviewCard(url: url, client: client),
    ),
  );

  Future<void> pumpAsync(WidgetTester tester) async {
    // Subscribe to a replacement future after Retry before flushing streams.
    await tester.pump();
    // MockClient finalizes request/response streams asynchronously. Let their
    // real event-loop work complete before painting the resulting FutureBuilder.
    for (
      var turn = 0;
      turn < 20 &&
          find.byKey(const Key('link_preview_loading')).evaluate().isNotEmpty;
      turn++
    ) {
      await tester.runAsync(() => Future<void>.delayed(Duration.zero));
      await tester.pump();
    }
    expect(find.byKey(const Key('link_preview_loading')), findsNothing);
  }

  test('extracts the first eligible HTTPS URL from text', () {
    expect(
      linkPreviewUrlFromContent(
        'http://plain.test https://user:secret@example.test '
        'https://example.test/post, https://second.test',
      ),
      'https://example.test/post',
    );
    expect(linkPreviewUrlFromContent('http://example.test'), isNull);
    expect(linkPreviewUrlFromContent('https://localhost/page'), isNull);
    expect(linkPreviewUrlFromContent('https://service.internal/page'), isNull);
    expect(linkPreviewUrlFromContent('ordinary text'), isNull);
  });

  testWidgets('shows loading then sanitized Open Graph card', (tester) async {
    final response = Completer<http.Response>();
    final client = MockClient(
      (request) async => request.url.path == '/cover.jpg'
          ? MockClient.pngResponse(request: request)
          : response.future,
    );
    await tester.pumpWidget(wrap('https://example.test/post', client));
    expect(find.byKey(const Key('link_preview_loading')), findsOneWidget);

    response.complete(
      http.Response(
        '<meta property="og:title" content="&lt;script&gt;A useful title&lt;/script&gt;">'
        '<meta property="og:description" content="A &amp; B">'
        '<meta property="og:image" content="https://example.test/cover.jpg">',
        200,
        headers: {'content-type': 'text/html; charset=utf-8'},
      ),
    );
    await pumpAsync(tester);

    expect(find.text('A useful title'), findsOneWidget);
    expect(find.text('A & B'), findsOneWidget);
    expect(find.text('example.test'), findsOneWidget);
    expect(find.byKey(const Key('link_preview_image')), findsOneWidget);
    expect(
      (tester.widget<Image>(find.byKey(const Key('link_preview_image'))).image
              as ResizeImage)
          .imageProvider,
      isA<MemoryImage>(),
    );
    expect(find.textContaining('<script>'), findsNothing);
  });

  testWidgets('uses title and hostname fallback when OG title is absent', (
    tester,
  ) async {
    final client = MockClient(
      (_) async => http.Response(
        '<title>Page title</title>',
        200,
        headers: {'content-type': 'text/html'},
      ),
    );
    await tester.pumpWidget(wrap('https://example.test/page', client));
    await pumpAsync(tester);
    expect(find.text('Page title'), findsOneWidget);
    expect(find.text('example.test'), findsOneWidget);
  });

  testWidgets('shows failure and retries after unsuccessful response', (
    tester,
  ) async {
    var attempts = 0;
    final client = MockClient((_) async {
      attempts++;
      return attempts == 1
          ? http.Response('unavailable', 503)
          : http.Response(
              '<title>Recovered</title>',
              200,
              headers: {'content-type': 'text/html'},
            );
    });
    await tester.pumpWidget(wrap('https://example.test/page', client));
    await pumpAsync(tester);
    expect(find.byKey(const Key('link_preview_unavailable')), findsOneWidget);
    expect(find.text('example.test'), findsOneWidget);

    await tester.tap(find.byKey(const Key('link_preview_retry')));
    await pumpAsync(tester);
    expect(find.text('Recovered'), findsOneWidget);
    expect(attempts, 2);
  });

  testWidgets('uses hostname when neither OG nor page title is available', (
    tester,
  ) async {
    final client = MockClient(
      (_) async => http.Response(
        '<html><body>No metadata</body></html>',
        200,
        headers: {'content-type': 'text/html'},
      ),
    );
    await tester.pumpWidget(wrap('https://example.test/page', client));
    await pumpAsync(tester);
    final title = tester.widget<Text>(
      find.byKey(const Key('link_preview_title')),
    );
    expect(title.data, 'example.test');
    expect(find.byKey(const Key('link_preview_ready')), findsOneWidget);
  });

  for (final body in ['not HTML', 'x' * (256 * 1024 + 1)]) {
    testWidgets('fails closed for invalid or oversized HTML (${body.length})', (
      tester,
    ) async {
      final client = MockClient(
        (_) async => http.Response(
          body,
          200,
          headers: {
            'content-type': body == 'not HTML'
                ? 'application/json'
                : 'text/html',
          },
        ),
      );
      await tester.pumpWidget(wrap('https://example.test/page', client));
      await pumpAsync(tester);
      expect(find.byKey(const Key('link_preview_unavailable')), findsOneWidget);
      expect(find.byKey(const Key('link_preview_ready')), findsNothing);
    });
  }

  testWidgets('does not render a non-HTTPS metadata image', (tester) async {
    final client = MockClient(
      (_) async => http.Response(
        '<title>Safe title</title>'
        '<meta property="og:image" content="http://example.test/cover.jpg">',
        200,
        headers: {'content-type': 'text/html'},
      ),
    );
    await tester.pumpWidget(wrap('https://example.test/page', client));
    await pumpAsync(tester);
    expect(find.text('Safe title'), findsOneWidget);
    expect(find.byKey(const Key('link_preview_image')), findsNothing);
  });

  testWidgets('rejects thumbnail redirects while retaining the text card', (
    tester,
  ) async {
    final requests = <http.Request>[];
    final client = MockClient((request) async {
      requests.add(request);
      if (request.url.path == '/cover.jpg') {
        return http.Response(
          '',
          302,
          headers: {'location': 'http://insecure.test/cover.jpg'},
        );
      }
      return http.Response(
        '<title>Safe title</title>'
        '<meta property="og:image" content="https://example.test/cover.jpg">',
        200,
        headers: {'content-type': 'text/html'},
      );
    });
    await tester.pumpWidget(wrap('https://example.test/page', client));
    await pumpAsync(tester);
    expect(find.text('Safe title'), findsOneWidget);
    expect(find.byKey(const Key('link_preview_image')), findsNothing);
    expect(requests.map((request) => request.url.path), [
      '/page',
      '/cover.jpg',
    ]);
    expect(requests.last.followRedirects, isFalse);
    expect(requests.last.maxRedirects, 0);
  });

  testWidgets('does not fetch non-HTTPS URLs', (tester) async {
    var attempts = 0;
    final client = MockClient((_) async {
      attempts++;
      return http.Response('<title>must not load</title>', 200);
    });
    await tester.pumpWidget(wrap('http://example.test/page', client));
    await pumpAsync(tester);
    expect(attempts, 0);
    expect(find.byKey(const Key('link_preview_unavailable')), findsOneWidget);
    expect(find.text('example.test'), findsOneWidget);
  });

  testWidgets('does not follow redirects', (tester) async {
    var attempts = 0;
    final client = MockClient((request) async {
      attempts++;
      expect(request.url.scheme, 'https');
      expect(request.followRedirects, isFalse);
      expect(request.maxRedirects, 0);
      return http.Response('', 302, headers: {'location': 'http://evil.test'});
    });
    await tester.pumpWidget(wrap('https://example.test/page', client));
    await pumpAsync(tester);
    expect(attempts, 1);
    expect(find.byKey(const Key('link_preview_unavailable')), findsOneWidget);
    expect(find.text('example.test'), findsOneWidget);
  });
}
