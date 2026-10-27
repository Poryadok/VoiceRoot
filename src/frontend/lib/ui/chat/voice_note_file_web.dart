import 'package:web/web.dart' as web;

Future<String> newVoiceNotePath() async => '';
Future<void> discardVoiceNote(String path) async {
  web.URL.revokeObjectURL(path);
}
