import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations_en.dart';
import 'package:voice_frontend/l10n/app_localizations_ru.dart';

void main() {
  test('Phase 2 call strings exist in EN and RU', () {
    final en = AppLocalizationsEn();
    final ru = AppLocalizationsRu();

    expect(en.callStartAudio, isNotEmpty);
    expect(en.callIncomingTitle('Alice'), contains('Alice'));
    expect(en.callHangup, isNotEmpty);

    expect(ru.callStartAudio, isNotEmpty);
    expect(ru.callIncomingTitle('Алиса'), contains('Алиса'));
    expect(ru.callHangup, isNotEmpty);
  });
}
