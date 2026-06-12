import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';

import 'push_notification_handler.dart';

/// Background FCM handler (Android/iOS); must be top-level.
@pragma('vm:entry-point')
Future<void> firebaseMessagingBackgroundHandler(RemoteMessage message) async {
  await Firebase.initializeApp();
  final data = message.data.map((k, v) => MapEntry(k, v.toString()));
  handlePushPayloadMap(data, (_) {});
}
