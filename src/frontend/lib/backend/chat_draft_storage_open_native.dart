import 'package:hive/hive.dart';
import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';

Future<void> initializeChatDraftStorage() async {
  final directory = await getApplicationSupportDirectory();
  Hive.init(p.join(directory.path, 'voice_chat_drafts'));
}
