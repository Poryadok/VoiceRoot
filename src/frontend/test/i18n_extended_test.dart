import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations_en.dart';
import 'package:voice_frontend/l10n/app_localizations_ru.dart';

void main() {
  test('voice call strings exist in EN and RU', () {
    final en = AppLocalizationsEn();
    final ru = AppLocalizationsRu();

    expect(en.callStartAudio, 'Start audio call');
    expect(en.callIncomingTitle('Alice'), 'Alice is calling');
    expect(en.callHangup, 'Hang up');

    expect(ru.callStartAudio, 'Начать аудиозвонок');
    expect(ru.callIncomingTitle('Алиса'), contains('Алиса'));
    expect(ru.callHangup, 'Завершить');
  });
}
