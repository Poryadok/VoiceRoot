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
  String get authPasswordHelper => 'Не менее 8 символов';

  @override
  String get authErrorEmptyFields => 'Введите email и пароль.';

  @override
  String get authErrorPasswordTooShort =>
      'Пароль должен быть не короче 8 символов.';

  @override
  String get authErrorValidationFailed =>
      'Укажите корректный email и пароль не короче 8 символов.';

  @override
  String get authErrorRateLimited =>
      'Слишком много попыток. Подождите и попробуйте снова.';

  @override
  String get authErrorInvalidCredentials => 'Неверный email или пароль.';

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
  String authSessionHandle(String handle) {
    return '$handle';
  }

  @override
  String get socialDiscoverHint => 'Найти людей — иконка слева';

  @override
  String get backendUnavailable =>
      'Социальные функции и чаты недоступны. Запустите полный API-стек (docker compose --profile app).';

  @override
  String get socialTabSearch => 'Поиск';

  @override
  String get socialTabFriends => 'Друзья';

  @override
  String get socialTabRequests => 'Заявки';

  @override
  String get socialSearchHint => 'Поиск по имени или @нику';

  @override
  String get socialSearchStart => 'Поиск людей';

  @override
  String get socialSearchStartHint =>
      'Введите имя или @ник, чтобы начать диалог.';

  @override
  String get socialSearchEmpty => 'Профили не найдены';

  @override
  String get socialSearchEmptyHint =>
      'Проверьте написание или попробуйте другой ник.';

  @override
  String get socialSearchLoading => 'Идёт поиск…';

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
  String get socialFriendsBackendUnavailable =>
      'Друзья недоступны. Запустите полный API-стек (docker compose --profile app).';

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
  String socialPresenceLastSeen(String dateTime) {
    return 'Был(а) $dateTime';
  }

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
  String get chatListEmptyHint =>
      'Найдите людей в поиске, чтобы начать личный диалог.';

  @override
  String get chatListLoadMore => 'Загрузить ещё чаты';

  @override
  String chatListUnreadCount(int count) {
    return 'Непрочитанных: $count';
  }

  @override
  String get chatListLoadError => 'Не удалось загрузить чаты';

  @override
  String get chatListBackendUnavailable =>
      'Чаты недоступны. Запустите полный API-стек (docker compose --profile app).';

  @override
  String chatListDmFallback(String id) {
    return 'Чат $id';
  }

  @override
  String get chatRoomSelectPrompt => 'Выберите диалог';

  @override
  String get chatRoomBack => 'Назад к чатам';

  @override
  String chatRoomTitle(String id) {
    return 'Чат $id';
  }

  @override
  String get chatRoomEmpty => 'Сообщений пока нет';

  @override
  String get chatRoomEmptyHint =>
      'Отправьте первое сообщение, когда будете готовы.';

  @override
  String get chatRoomLoadOlder => 'Загрузить старые сообщения';

  @override
  String get chatRoomInputHint => 'Сообщение';

  @override
  String get chatSendMessage => 'Отправить сообщение';

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
  String get callStartAudio => 'Начать аудиозвонок';

  @override
  String get callStartVideo => 'Начать видеозвонок';

  @override
  String callIncomingTitle(String name) {
    return '$name звонит';
  }

  @override
  String get callIncomingAudio => 'Аудиозвонок';

  @override
  String get callIncomingVideo => 'Видеозвонок';

  @override
  String get callAccept => 'Принять';

  @override
  String get callDecline => 'Отклонить';

  @override
  String get callConnecting => 'Подключение к звонку…';

  @override
  String get callActive => 'Звонок активен';

  @override
  String get callTapToEnableAudio => 'Нажмите, чтобы включить входящий звук';

  @override
  String get callMute => 'Выключить микрофон';

  @override
  String get callUnmute => 'Включить микрофон';

  @override
  String get callSpeakerOff => 'Выключить звук';

  @override
  String get callSpeakerOn => 'Включить звук';

  @override
  String get callVideoOn => 'Включить камеру';

  @override
  String get callVideoOff => 'Выключить камеру';

  @override
  String get callHangup => 'Завершить';

  @override
  String callOutgoingTitle(String name) {
    return 'Звоним $name…';
  }

  @override
  String callFailed(String message) {
    return 'Не удалось начать звонок: $message';
  }

  @override
  String get callLivekitConnectFailed => 'Не удалось подключиться к LiveKit';

  @override
  String get callActiveCallExists => 'У вас уже есть активный звонок';

  @override
  String get profileMessage => 'Написать';

  @override
  String get profileEditTitle => 'Редактировать профиль';

  @override
  String get profileEditTooltip => 'Редактировать профиль';

  @override
  String get profileDisplayNameLabel => 'Имя профиля';

  @override
  String get profileBioLabel => 'О себе';

  @override
  String get profileBioHelper => 'До 500 символов';

  @override
  String get profileAvatarChange => 'Сменить аватар';

  @override
  String profileAvatarSelected(String fileName) {
    return 'Выбрано: $fileName';
  }

  @override
  String get profileSave => 'Сохранить';

  @override
  String get profileErrorDisplayNameRequired => 'Введите имя профиля.';

  @override
  String get profileErrorDisplayNameTooLong =>
      'Имя профиля должно быть не длиннее 32 символов.';

  @override
  String get profileErrorBioTooLong =>
      'Описание должно быть не длиннее 500 символов.';

  @override
  String get profileErrorAvatarType =>
      'Используйте статичное изображение JPEG, PNG или WebP.';

  @override
  String get profileErrorAvatarTooLarge =>
      'Аватар должен быть непустым изображением до 5 МБ.';

  @override
  String profileEditSaveError(String message) {
    return 'Не удалось сохранить профиль: $message';
  }

  @override
  String get commonCancel => 'Отмена';

  @override
  String get commonSave => 'Сохранить';

  @override
  String get commonRetry => 'Попробовать снова';

  @override
  String get commonLoading => '…';

  @override
  String get chatInboxDm => 'Личные';

  @override
  String get chatInboxRequests => 'Запросы';

  @override
  String get chatTyping => 'Печатает…';

  @override
  String get chatAttachFile => 'Прикрепить файл';

  @override
  String get chatMessageEdit => 'Изменить';

  @override
  String get chatMessageDeleteForMe => 'Удалить у меня';

  @override
  String get chatMessageDeleteForEveryone => 'Удалить у всех';

  @override
  String get chatEditMessageTitle => 'Изменить сообщение';

  @override
  String get chatMessageEdited => '(ред.)';

  @override
  String get chatDeliverySent => 'Отправлено';

  @override
  String get chatDeliveryDelivered => 'Доставлено';

  @override
  String get chatDeliveryRead => 'Прочитано';

  @override
  String get chatImageAttachment => 'Вложение-изображение';

  @override
  String get chatNewMessages => 'Новые сообщения';

  @override
  String get chatUnreadSeparator => 'Непрочитанные сообщения';

  @override
  String get chatListStrangerBadge => 'Незнакомец';

  @override
  String get chatCreateGroupTooltip => 'Создать группу';

  @override
  String get chatCreateGroupTitle => 'Новая группа';

  @override
  String get chatCreateGroupNameLabel => 'Название группы';

  @override
  String get chatCreateGroupNameHint => 'Пятничная тусовка';

  @override
  String get chatCreateGroupMembers => 'Добавить участников';

  @override
  String get chatCreateGroupMembersHint =>
      'Выберите минимум 2 друзей (всего 3 человека, включая вас).';

  @override
  String get chatCreateGroupSubmit => 'Создать группу';

  @override
  String get chatCreateGroupMinMembers =>
      'Выберите минимум 2 друзей для создания группы.';

  @override
  String get chatCreateGroupFriendsEmptyHint =>
      'Сначала добавьте друзей, затем пригласите их в группу.';

  @override
  String chatCreateGroupError(String message) {
    return 'Не удалось создать группу: $message';
  }

  @override
  String get settingsTitle => 'Настройки';

  @override
  String get settingsTooltip => 'Настройки';

  @override
  String get settingsTheme => 'Тема';

  @override
  String get settingsThemeSystem => 'Системная';

  @override
  String get settingsThemeLight => 'Светлая';

  @override
  String get settingsThemeDark => 'Тёмная';

  @override
  String get settingsThemeHighContrast => 'Высокий контраст';

  @override
  String get settingsLanguage => 'Язык';

  @override
  String get settingsLanguageSystem => 'Как в системе';

  @override
  String get settingsLanguageEn => 'English';

  @override
  String get settingsLanguageRu => 'Русский';

  @override
  String get settingsAccent => 'Акцент профиля';

  @override
  String get authTagline => 'Голос и сообщения для геймеров';

  @override
  String get versionUpdateRequired => 'Требуется обновление';

  @override
  String versionUpdateAvailable(String version) {
    return 'Доступно обновление $version';
  }

  @override
  String get versionUpdateAvailableGeneric => 'Доступно обновление';

  @override
  String get versionUpdateLater => 'Позже';

  @override
  String get profileBlock => 'Заблокировать';

  @override
  String get profileBlockConfirmTitle => 'Заблокировать пользователя?';

  @override
  String get profileBlockConfirmMessage => 'Он не сможет писать вам сообщения.';

  @override
  String get callVideoPlaceholder => 'Видео';

  @override
  String get themeLoadError => 'Не удалось загрузить тему';

  @override
  String get bootstrapRestoring => 'Восстановление сессии…';
}
