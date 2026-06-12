import 'package:auto_updater/auto_updater.dart' as auto_updater;
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../backend/client_version.dart';

enum DesktopUpdateStatus {
  idle,
  downloading,
  readyToRestart,
  failed,
}

/// Windows desktop auto-update via platform channel (WinSparkle under the hood).
abstract class DesktopUpdaterService {
  Future<DesktopUpdateStatus> checkForUpdate(String manifestUrl);

  Future<void> restartAndApply();
}

class MethodChannelDesktopUpdaterService implements DesktopUpdaterService {
  MethodChannelDesktopUpdaterService({
    MethodChannel? channel,
  }) : _channel = channel ?? const MethodChannel(ClientVersion.desktopUpdaterChannel);

  final MethodChannel _channel;

  @override
  Future<DesktopUpdateStatus> checkForUpdate(String manifestUrl) async {
    if (manifestUrl.isEmpty) {
      return DesktopUpdateStatus.idle;
    }
    try {
      final result = await _channel.invokeMethod<Object?>(
        'checkForUpdate',
        manifestUrl,
      );
      return _statusFromPlatform(result);
    } on PlatformException {
      return DesktopUpdateStatus.failed;
    }
  }

  @override
  Future<void> restartAndApply() async {
    await _channel.invokeMethod<void>('restartAndApply');
  }

  DesktopUpdateStatus _statusFromPlatform(Object? result) {
    if (result is Map) {
      final status = result['status']?.toString();
      return switch (status) {
        'downloading' => DesktopUpdateStatus.downloading,
        'ready' => DesktopUpdateStatus.readyToRestart,
        'failed' => DesktopUpdateStatus.failed,
        _ => DesktopUpdateStatus.idle,
      };
    }
    return DesktopUpdateStatus.idle;
  }
}

class RecordingDesktopUpdaterService implements DesktopUpdaterService {
  int checkForUpdateCalls = 0;
  String? lastManifestUrl;
  DesktopUpdateStatus nextStatus = DesktopUpdateStatus.readyToRestart;
  int restartCalls = 0;

  @override
  Future<DesktopUpdateStatus> checkForUpdate(String manifestUrl) async {
    checkForUpdateCalls++;
    lastManifestUrl = manifestUrl;
    return nextStatus;
  }

  @override
  Future<void> restartAndApply() async {
    restartCalls++;
  }
}

class AutoUpdaterDesktopService implements DesktopUpdaterService {
  const AutoUpdaterDesktopService();

  @override
  Future<DesktopUpdateStatus> checkForUpdate(String manifestUrl) async {
    if (manifestUrl.isEmpty) return DesktopUpdateStatus.idle;
    await auto_updater.autoUpdater.setFeedURL(manifestUrl);
    await auto_updater.autoUpdater.checkForUpdates(inBackground: true);
    return DesktopUpdateStatus.downloading;
  }

  @override
  Future<void> restartAndApply() async {
    await auto_updater.autoUpdater.checkForUpdates(inBackground: false);
  }
}

class NoopDesktopUpdaterService implements DesktopUpdaterService {
  const NoopDesktopUpdaterService();

  @override
  Future<DesktopUpdateStatus> checkForUpdate(String manifestUrl) async {
    return DesktopUpdateStatus.idle;
  }

  @override
  Future<void> restartAndApply() async {}
}

final desktopUpdaterServiceProvider = Provider<DesktopUpdaterService>((ref) {
  if (ClientVersion.usesDesktopAutoUpdater) {
    return const AutoUpdaterDesktopService();
  }
  return const NoopDesktopUpdaterService();
});
