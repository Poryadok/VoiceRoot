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

  test('new UI copy and pinned position are localized in EN and RU', () {
    final en = AppLocalizationsEn();
    final ru = AppLocalizationsRu();

    expect(en.chatPinnedBarPosition(2, 5), 'Pinned message 2 of 5');
    expect(ru.chatPinnedBarPosition(2, 5), 'Закреплённое сообщение 2 из 5');
    expect(en.chatThemesIncomingSample, 'A message');
    expect(ru.chatThemesIncomingSample, 'Сообщение');
    expect(en.chatThemesOutgoingSample, 'Your message');
    expect(ru.chatThemesOutgoingSample, 'Ваше сообщение');
    expect(en.contactProfileCall, 'Call');
    expect(ru.contactProfileCall, 'Позвонить');
    expect(en.settingsTextScale, 'Text scale');
    expect(ru.settingsTextScale, 'Масштаб текста');
    expect(en.settingsUnsavedChangesTitle, 'Discard unsaved changes?');
    expect(
      ru.settingsUnsavedChangesTitle,
      'Сбросить несохранённые изменения?',
    );
    expect(
      en.settingsUnsavedChangesMessage,
      'Your changes have not been saved.',
    );
    expect(ru.settingsUnsavedChangesMessage, 'Изменения ещё не сохранены.');
    expect(en.commonDiscard, 'Discard');
    expect(ru.commonDiscard, 'Сбросить');
    expect(en.storyHighlightsTitle, 'Highlights');
    expect(ru.storyHighlightsTitle, 'Актуальное');
  });
}
