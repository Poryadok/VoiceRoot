import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/services/desktop_updater_service.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  const channel = MethodChannel('voice/desktop_updater');

  test('checkForUpdate invokes platform channel with manifest url', () async {
  final service = MethodChannelDesktopUpdaterService();
  String? invokedMethod;
  String? manifestUrl;

  TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
      .setMockMethodCallHandler(channel, (call) async {
    invokedMethod = call.method;
    manifestUrl = call.arguments as String?;
    return {'status': 'downloading'};
  });
  addTearDown(() {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, null);
  });

  final result = await service.checkForUpdate(
    'https://updates.voice.example/windows/appcast.xml',
  );

  expect(invokedMethod, 'checkForUpdate');
  expect(manifestUrl, 'https://updates.voice.example/windows/appcast.xml');
  expect(result, DesktopUpdateStatus.downloading);
  });

  test('restartAndApply invokes platform channel', () async {
    final service = MethodChannelDesktopUpdaterService();
    var restartCalled = false;

    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
      if (call.method == 'restartAndApply') {
        restartCalled = true;
      }
      return null;
    });
    addTearDown(() {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    });

    await service.restartAndApply();

    expect(restartCalled, isTrue);
  });
}
