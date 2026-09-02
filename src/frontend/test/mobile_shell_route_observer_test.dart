import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/ui/shell/mobile_shell_route_observer.dart';

void main() {
  testWidgets('MobileShellOverlayObserver tracks page route depth', (
    tester,
  ) async {
    var depth = 0;
    final observer = MobileShellOverlayObserver((delta) => depth += delta);
    late NavigatorState navigator;

    await tester.pumpWidget(
      MaterialApp(
        navigatorObservers: [observer],
        home: Builder(
          builder: (context) {
            navigator = Navigator.of(context);
            return Scaffold(
              body: ElevatedButton(
                onPressed: () {
                  navigator.push(
                    MaterialPageRoute<void>(
                      builder: (_) => const Scaffold(body: Text('Overlay')),
                    ),
                  );
                },
                child: const Text('Push'),
              ),
            );
          },
        ),
      ),
    );

    expect(depth, 0);
    await tester.tap(find.text('Push'));
    await tester.pumpAndSettle();
    expect(depth, 1);

    navigator.pop();
    await tester.pumpAndSettle();
    expect(depth, 0);
  });
}
