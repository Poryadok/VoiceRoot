// ignore: unused_import
import 'package:intl/intl.dart' as intl;
import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for Russian (`ru`).
class AppLocalizationsRu extends AppLocalizations {
  AppLocalizationsRu([String locale = 'ru']) : super(locale);

  @override
  String get appTitle => 'Voice';

  @override
  String get gatewayStatusOk => 'Шлюз: ок';

  @override
  String get gatewayStatusChecking => 'Шлюз: проверка…';

  @override
  String get gatewayMissingBaseUrl => 'Шлюз: не указан базовый URL';

  @override
  String gatewayStatusError(String error) {
    return 'Шлюз: ошибка ($error)';
  }

  @override
  String gatewayStatusFailure(String detail) {
    return 'Шлюз: $detail';
  }

  @override
  String get authTitle => 'Вход в Voice';

  @override
  String get authEmailLabel => 'Email';

  @override
  String get authPasswordLabel => 'Пароль';

  @override
  String get authLogin => 'Войти';

  @override
  String get authRegister => 'Регистрация';

  @override
  String get authLogout => 'Выйти';

  @override
  String authError(String message) {
    return 'Ошибка входа: $message';
  }

  @override
  String authSessionProfile(String profileId) {
    return 'Профиль: $profileId';
  }

  @override
  String get socialTabSearch => 'Поиск';

  @override
  String get socialTabFriends => 'Друзья';

  @override
  String get socialTabRequests => 'Заявки';

  @override
  String get socialSearchHint => 'Поиск по имени или @нику';

  @override
  String get socialAddFriend => 'Добавить в друзья';

  @override
  String get socialAcceptRequest => 'Принять';

  @override
  String get socialDeclineRequest => 'Отклонить';

  @override
  String get socialRequestPending => 'Заявка отправлена';

  @override
  String get socialFriendsEmpty => 'Пока нет друзей';

  @override
  String get socialRequestsEmpty => 'Нет заявок в друзья';

  @override
  String get socialIncomingRequests => 'Входящие';

  @override
  String get socialOutgoingRequests => 'Исходящие';

  @override
  String get socialFriendsLoadError => 'Не удалось загрузить друзей';

  @override
  String get socialRequestsLoadError => 'Не удалось загрузить заявки';

  @override
  String get socialProfileLoadError => 'Не удалось загрузить профиль';

  @override
  String get socialPresenceOnline => 'В сети';

  @override
  String get socialPresenceIdle => 'Отошёл';

  @override
  String get socialPresenceDnd => 'Не беспокоить';

  @override
  String get socialPresenceOffline => 'Не в сети';

  @override
  String get socialPresenceUnknown => 'Неизвестно';

  @override
  String socialActionError(String message) {
    return 'Ошибка: $message';
  }

  @override
  String get socialRailTooltip => 'Друзья и поиск';

  @override
  String get chatListTitle => 'Личные сообщения';

  @override
  String get chatListEmpty => 'Пока нет диалогов';

  @override
  String get chatListLoadError => 'Не удалось загрузить чаты';

  @override
  String chatListDmFallback(String id) {
    return 'Чат $id';
  }

  @override
  String get chatRoomSelectPrompt => 'Выберите диалог';

  @override
  String chatRoomTitle(String id) {
    return 'Чат $id';
  }

  @override
  String get chatRoomEmpty => 'Сообщений пока нет';

  @override
  String get chatRoomInputHint => 'Сообщение';

  @override
  String chatRoomError(String message) {
    return 'Ошибка: $message';
  }

  @override
  String get chatRealtimeConnected => 'Онлайн';

  @override
  String get chatRealtimeConnecting => 'Подключение…';

  @override
  String get chatRealtimeReconnecting => 'Переподключение…';

  @override
  String get chatRealtimeOffline => 'Офлайн';

  @override
  String get chatOpenDm => 'Написать';

  @override
  String get profileMessage => 'Написать';

  @override
  String get commonLoading => '…';
}
