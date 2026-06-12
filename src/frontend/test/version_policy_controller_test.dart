import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_api_error.dart';
import 'package:voice_frontend/services/desktop_updater_service.dart';
import 'package:voice_frontend/state/version_policy_providers.dart';

import 'support/auth_test_overrides.dart';

void main() {
  group('VersionPolicyController refresh parsing', () {
    test('ok when no update flags', () async {
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response(
              jsonEncode({
                'force_update': false,
                'update_available': false,
                'latest_version': '1.0.0',
              }),
              200,
            )),
          ),
          versionPolicyProvider.overrideWith(
            (ref) => VersionPolicyController(ref, enablePolling: false),
          ),
        ],
      );
      addTearDown(container.dispose);

      await container.read(versionPolicyProvider.notifier).refresh();

      expect(container.read(versionPolicyProvider).phase, VersionPolicyPhase.ok);
    });

    test('soft update when update_available', () async {
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response(
              jsonEncode({
                'force_update': false,
                'update_available': true,
                'latest_version': '1.8.0',
                'update_url': 'https://updates.voice.example/windows/appcast.xml',
              }),
              200,
            )),
          ),
          versionPolicyProvider.overrideWith(
            (ref) => VersionPolicyController(ref, enablePolling: false),
          ),
        ],
      );
      addTearDown(container.dispose);

      await container.read(versionPolicyProvider.notifier).refresh();

      final policy = container.read(versionPolicyProvider);
      expect(policy.phase, VersionPolicyPhase.softUpdate);
      expect(policy.latestVersion, '1.8.0');
      expect(policy.updateUrl, 'https://updates.voice.example/windows/appcast.xml');
    });

    test('force update when force_update true', () async {
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response(
              jsonEncode({
                'force_update': true,
                'update_available': true,
                'latest_version': '1.8.0',
                'update_url': 'https://updates.voice.example/windows/appcast.xml',
                'release_notes': 'Required',
              }),
              200,
            )),
          ),
          versionPolicyProvider.overrideWith(
            (ref) => VersionPolicyController(ref, enablePolling: false),
          ),
        ],
      );
      addTearDown(container.dispose);

      await container.read(versionPolicyProvider.notifier).refresh();

      final policy = container.read(versionPolicyProvider);
      expect(policy.phase, VersionPolicyPhase.forceUpdate);
      expect(policy.releaseNotes, 'Required');
    });
  });

  group('VersionPolicyController 426 handler', () {
    test('client_outdated sets force update with update_url', () {
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(client: MockClient((_) async => http.Response('{}', 404))),
          versionPolicyProvider.overrideWith(
            (ref) => VersionPolicyController(ref, enablePolling: false),
          ),
        ],
      );
      addTearDown(container.dispose);

      container.read(versionPolicyProvider.notifier).onGatewayUpgradeRequired(
        const GatewayApiError(
          errorCode: 'client_outdated',
          message: 'Update required',
          statusCode: 426,
          updateUrl: 'https://updates.voice.example/windows/appcast.xml',
        ),
      );

      final policy = container.read(versionPolicyProvider);
      expect(policy.phase, VersionPolicyPhase.forceUpdate);
      expect(policy.updateUrl, 'https://updates.voice.example/windows/appcast.xml');
    });
  });

  group('Windows desktop auto-updater', () {
    test('soft update schedules background download via DesktopUpdaterService', () async {
      debugDefaultTargetPlatformOverride = TargetPlatform.windows;
      addTearDown(() => debugDefaultTargetPlatformOverride = null);

      final updater = RecordingDesktopUpdaterService();
      final container = ProviderContainer(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((_) async => http.Response(
              jsonEncode({
                'force_update': false,
                'update_available': true,
                'latest_version': '1.8.0',
                'update_url': 'https://updates.voice.example/windows/appcast.xml',
              }),
              200,
            )),
          ),
          desktopUpdaterServiceProvider.overrideWithValue(updater),
          versionPolicyProvider.overrideWith(
            (ref) => VersionPolicyController(ref, enablePolling: false),
          ),
        ],
      );
      addTearDown(container.dispose);

      await container.read(versionPolicyProvider.notifier).refresh();

      expect(updater.checkForUpdateCalls, 1);
      expect(updater.lastManifestUrl, 'https://updates.voice.example/windows/appcast.xml');
      expect(
        container.read(versionPolicyProvider).phase,
        VersionPolicyPhase.desktopReadyToRestart,
      );
    });
  });
}
