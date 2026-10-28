import 'dart:io';
import 'package:path_provider/path_provider.dart';
import 'package:uuid/uuid.dart';

Future<String> newVoiceNotePath() async =>
    '${(await getTemporaryDirectory()).path}/voice-${const Uuid().v4()}.wav';

Future<void> discardVoiceNote(String path) async {
  final file = File(path);
  if (await file.exists()) await file.delete();
}
