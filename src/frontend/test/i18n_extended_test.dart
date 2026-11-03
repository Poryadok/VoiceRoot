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
    expect(ru.settingsUnsavedChangesTitle, 'Сбросить несохранённые изменения?');
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

  test('remaining UI v3 labels are localized in EN and RU', () {
    final en = AppLocalizationsEn();
    final ru = AppLocalizationsRu();

    expect(
      en.backendUnavailable,
      'The service is temporarily unavailable. Please try again.',
    );
    expect(
      ru.backendUnavailable,
      'Сервис временно недоступен. Попробуйте ещё раз.',
    );
    expect(en.contactProfileAccountDeleted, 'Account deleted');
    expect(ru.contactProfileAccountDeleted, 'Аккаунт удалён');
    expect(en.subscriptionPlusAnnual, 'Voice Plus · annual');
    expect(ru.subscriptionPlusAnnual, 'Voice Plus · годовой');
    expect(en.subscriptionPlusMonthly, 'Voice Plus · monthly');
    expect(ru.subscriptionPlusMonthly, 'Voice Plus · месячный');
    expect(en.subscriptionPlusBadge, 'PLUS');
    expect(ru.subscriptionPlusBadge, 'PLUS');
    expect(
      en.verificationProviderLinkUnavailable,
      'Provider linking is unavailable until OAuth return and provider configuration are enabled.',
    );
    expect(
      ru.verificationProviderLinkUnavailable,
      'Привязка платформ недоступна, пока не настроены возврат после OAuth и конфигурация провайдера.',
    );
    expect(
      en.verificationDomainEligibilityUnavailable,
      'Domain verification is unavailable until organization eligibility and pending DNS details can be read back.',
    );
    expect(
      ru.verificationDomainEligibilityUnavailable,
      'Проверка домена недоступна, пока нельзя подтвердить право организации на верификацию и восстановить текущие DNS-инструкции.',
    );

    expect(en.screenShareWatchStream, 'Watch stream');
    expect(ru.screenShareWatchStream, 'Смотреть трансляцию');
    expect(en.screenShareWaitingForVideo, 'Waiting for screen video…');
    expect(ru.screenShareWaitingForVideo, 'Ожидание видео…');
    expect(en.screenShareCloseViewer, 'Close viewer');
    expect(ru.screenShareCloseViewer, 'Закрыть просмотр');
    expect(en.screenShareStreamPaused, 'Stream paused');
    expect(ru.screenShareStreamPaused, 'Трансляция приостановлена');
    expect(en.screenSharePausedLabel, 'Paused');
    expect(ru.screenSharePausedLabel, 'Пауза');
    expect(en.screenSharePanelTitle, 'Screen shares');
    expect(ru.screenSharePanelTitle, 'Демонстрации экрана');
    expect(en.screenShareChooseStream, 'Choose a stream');
    expect(ru.screenShareChooseStream, 'Выбрать трансляцию');
    expect(en.screenShareWatching, 'Watching');
    expect(ru.screenShareWatching, 'Просмотр');
    expect(en.screenShareStreamCaption('Alice'), 'Alice · Screen share');
    expect(ru.screenShareStreamCaption('Алиса'), 'Алиса · Демонстрация экрана');
    expect(en.screenSharePausedParticipant('Alice'), 'Alice, paused');
    expect(ru.screenSharePausedParticipant('Алиса'), 'Алиса, пауза');

    expect(en.botWebhookTitle, 'Interaction webhook');
    expect(ru.botWebhookTitle, 'Webhook для взаимодействий');
    expect(en.botWebhookLoadError, 'Could not load the endpoint.');
    expect(ru.botWebhookLoadError, 'Не удалось загрузить endpoint.');
    expect(en.botWebhookSaveError, 'Could not save the endpoint. Try again.');
    expect(
      ru.botWebhookSaveError,
      'Не удалось сохранить endpoint. Повторите попытку.',
    );
    expect(en.botWebhookSaved, 'Endpoint saved');
    expect(ru.botWebhookSaved, 'Endpoint сохранён');
    expect(en.botWebhookPollingConfirmTitle, 'Switch to polling?');
    expect(ru.botWebhookPollingConfirmTitle, 'Переключиться на polling?');
    expect(
      en.botWebhookPollingConfirmMessage,
      'Outgoing webhook delivery will stop. Your bot can receive interactions by polling.',
    );
    expect(
      ru.botWebhookPollingConfirmMessage,
      'Доставка через исходящий webhook прекратится. Бот сможет получать взаимодействия через polling.',
    );
    expect(en.botWebhookKeep, 'Keep webhook');
    expect(ru.botWebhookKeep, 'Оставить webhook');
    expect(
      en.botWebhookOwnerOnly,
      'Only the bot owner can configure this endpoint.',
    );
    expect(
      ru.botWebhookOwnerOnly,
      'Только владелец бота может настроить этот endpoint.',
    );
    expect(en.botWebhookPollingEnabled, 'Polling enabled');
    expect(ru.botWebhookPollingEnabled, 'Polling включён');
    expect(en.botWebhookEnabled, 'Webhook enabled');
    expect(ru.botWebhookEnabled, 'Webhook включён');
    expect(
      en.botWebhookDescription,
      'Voice sends interactions to your bot at this endpoint. Use polling for local development.',
    );
    expect(
      ru.botWebhookDescription,
      'Voice отправляет взаимодействия вашему боту на этот endpoint. Для локальной разработки используйте polling.',
    );
    expect(en.botWebhookEndpoint, 'Endpoint');
    expect(ru.botWebhookEndpoint, 'Endpoint');
    expect(en.botWebhookEndpointHint, 'https://your-bot.example/interactions');
    expect(ru.botWebhookEndpointHint, 'https://your-bot.example/interactions');
    expect(en.botWebhookInvalidEndpoint, 'Enter a valid HTTPS endpoint.');
    expect(ru.botWebhookInvalidEndpoint, 'Введите корректный HTTPS endpoint.');
    expect(en.botWebhookSavedEndpoint, 'SAVED ENDPOINT');
    expect(ru.botWebhookSavedEndpoint, 'СОХРАНЁННЫЙ ENDPOINT');
    expect(en.botWebhookCopied, 'Endpoint copied');
    expect(ru.botWebhookCopied, 'Endpoint скопирован');
    expect(en.botWebhookCopy, 'Copy endpoint');
    expect(ru.botWebhookCopy, 'Копировать endpoint');
    expect(en.botWebhookSwitchToPolling, 'Switch to polling');
    expect(ru.botWebhookSwitchToPolling, 'Переключиться на polling');
    expect(en.botWebhookSaving, 'Saving…');
    expect(ru.botWebhookSaving, 'Сохранение…');
    expect(en.botWebhookSave, 'Save endpoint');
    expect(ru.botWebhookSave, 'Сохранить endpoint');
    expect(en.botWebhookClose, 'Close');
    expect(ru.botWebhookClose, 'Закрыть');
    expect(en.botWebhookConfirm, 'Confirm');
    expect(ru.botWebhookConfirm, 'Подтвердить');

    expect(en.storySuggestionTitle, 'Share your match');
    expect(ru.storySuggestionTitle, 'Поделиться матчем');
    expect(en.storySuggestionCompleted, 'Match completed');
    expect(ru.storySuggestionCompleted, 'Матч завершён');
    expect(en.storySuggestionEditPublish, 'Edit & publish');
    expect(ru.storySuggestionEditPublish, 'Редактировать и опубликовать');
    expect(en.storySuggestionDismiss, 'Dismiss');
    expect(ru.storySuggestionDismiss, 'Не сейчас');
    expect(
      en.storySuggestionDraft('Apex Legends'),
      'Match completed · Apex Legends',
    );
    expect(
      ru.storySuggestionDraft('Apex Legends'),
      'Матч завершён · Apex Legends',
    );
  });
}
