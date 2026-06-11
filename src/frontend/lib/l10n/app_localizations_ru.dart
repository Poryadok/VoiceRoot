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
  String get chatMentionInsert => 'Вставить упоминание';

  @override
  String get chatMentionEveryone => '@everyone';

  @override
  String get chatMentionHere => '@here';

  @override
  String chatMentionMember(String profileId) {
    return 'Участник $profileId';
  }

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
  String get callGroupVoiceStart => 'Начать голосовой чат';

  @override
  String get callGroupVoiceJoin => 'Присоединиться';

  @override
  String get callGroupVoiceActive => 'Групповой голос активен';

  @override
  String get callGroupVoiceInProgress => 'В группе идёт голосовой звонок';

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
  String get chatMessageForward => 'Переслать';

  @override
  String get chatMessageAddReaction => 'Добавить реакцию';

  @override
  String get chatMessagePin => 'Закрепить сообщение';

  @override
  String get chatMessageUnpin => 'Открепить сообщение';

  @override
  String chatPinnedBar(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count закреплённых сообщения',
      many: '$count закреплённых сообщений',
      few: '$count закреплённых сообщения',
      one: '$count закреплённое сообщение',
    );
    return '$_temp0';
  }

  @override
  String get chatForwardTitle => 'Переслать в';

  @override
  String get chatForwardSearchHint => 'Поиск чатов';

  @override
  String chatForwardFrom(String sender) {
    return 'Переслано от $sender';
  }

  @override
  String get chatForwardCommentaryTitle => 'Добавить комментарий';

  @override
  String get chatForwardCommentaryHint =>
      'Необязательное сообщение перед пересылкой';

  @override
  String get chatForwardEmpty => 'Нет чатов для пересылки';

  @override
  String get chatForwardSuccess => 'Сообщение переслано';

  @override
  String chatForwardError(String message) {
    return 'Не удалось переслать сообщение: $message';
  }

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
  String get spaceCreateTooltip => 'Создать спейс';

  @override
  String get spaceCreateTitle => 'Новый спейс';

  @override
  String get spaceCreateNameLabel => 'Название спейса';

  @override
  String get spaceCreateNameHint => 'Пятничная тусовка';

  @override
  String get spaceCreateDescriptionLabel => 'Описание';

  @override
  String get spaceCreateDescriptionHint => 'О чём этот спейс?';

  @override
  String get spaceCreateIconLabel => 'URL иконки';

  @override
  String get spaceCreateIconHint => 'https://cdn.example/icon.webp';

  @override
  String get spaceCreateSubmit => 'Создать спейс';

  @override
  String spaceCreateError(String message) {
    return 'Не удалось создать спейс: $message';
  }

  @override
  String get spaceTreeTitle => 'Каналы';

  @override
  String get spaceTreeEmpty => 'Пока нет каналов';

  @override
  String get spaceTreeLoadError => 'Не удалось загрузить дерево спейса';

  @override
  String get spaceTreeTextChat => 'Текстовый чат';

  @override
  String get spaceTreeVoiceRoom => 'Голосовая комната';

  @override
  String get spaceTreeUncategorized => 'Каналы';

  @override
  String get spaceSelectPrompt => 'Выберите спейс';

  @override
  String get spaceListTitle => 'Мои спейсы';

  @override
  String get spaceOpenAction => 'Открыть спейс';

  @override
  String get spaceInvitesTooltip => 'Пригласить людей';

  @override
  String get spaceInvitesTitle => 'Инвайт-ссылки';

  @override
  String get spaceInvitesSubtitle =>
      'Создайте ссылку, чтобы пригласить людей в спейс.';

  @override
  String get spaceInvitesEmpty => 'Нет активных инвайт-ссылок';

  @override
  String get spaceInvitesLoadError => 'Не удалось загрузить инвайты';

  @override
  String get spaceInvitesRetry => 'Повторить';

  @override
  String get spaceInviteCreate => 'Создать ссылку';

  @override
  String get spaceInviteAdvancedToggle => 'Дополнительно';

  @override
  String get spaceInviteMaxUsesLabel => 'Лимит использований';

  @override
  String get spaceInviteMaxUsesHint => 'Пусто — без лимита';

  @override
  String get spaceInviteMaxUsesInvalid =>
      'Лимит должен быть положительным числом';

  @override
  String spaceInviteCreateError(String message) {
    return 'Не удалось создать инвайт: $message';
  }

  @override
  String spaceInviteRevokeError(String message) {
    return 'Не удалось отозвать инвайт: $message';
  }

  @override
  String get spaceInviteCopy => 'Копировать ссылку';

  @override
  String get spaceInviteCopied => 'Ссылка скопирована';

  @override
  String get spaceInviteRevoke => 'Отозвать';

  @override
  String spaceInviteUses(int used, String maxSuffix) {
    return '$used использований$maxSuffix';
  }

  @override
  String get spaceInviteJoinTooltip => 'Вступить по инвайту';

  @override
  String get spaceInviteJoinTitle => 'Вступить в спейс';

  @override
  String get spaceInviteJoinSubtitle => 'Вставьте код или ссылку от друга.';

  @override
  String get spaceInviteJoinCodeLabel => 'Код инвайта';

  @override
  String get spaceInviteJoinCodeHint => 'abc123xyz';

  @override
  String get spaceInviteJoinSubmit => 'Вступить';

  @override
  String spaceInviteJoinError(String message) {
    return 'Не удалось вступить: $message';
  }

  @override
  String get spaceMembersTooltip => 'Участники спейса';

  @override
  String get spaceMembersTitle => 'Участники';

  @override
  String get spaceMembersSubtitle =>
      'Владелец и администраторы могут назначать роли и удалять участников.';

  @override
  String get spaceMembersLoadError => 'Не удалось загрузить участников';

  @override
  String spaceMemberYou(String name) {
    return '$name (вы)';
  }

  @override
  String get spaceKick => 'Удалить';

  @override
  String get spaceKickConfirmTitle => 'Удалить участника?';

  @override
  String spaceKickConfirmMessage(String name) {
    return 'Удалить $name из спейса?';
  }

  @override
  String spaceKickError(String message) {
    return 'Не удалось удалить участника: $message';
  }

  @override
  String get spaceBan => 'Забанить';

  @override
  String get spaceBanConfirmTitle => 'Забанить участника?';

  @override
  String spaceBanConfirmMessage(String name) {
    return 'Забанить $name в этом спейсе? Он не сможет вернуться.';
  }

  @override
  String spaceBanError(String message) {
    return 'Не удалось забанить участника: $message';
  }

  @override
  String get spaceTimeout => 'Таймаут';

  @override
  String get spaceTimeoutConfirmTitle => 'Выдать таймаут?';

  @override
  String spaceTimeoutConfirmMessage(String name) {
    return 'Запретить $name писать сообщения на 10 минут?';
  }

  @override
  String spaceTimeoutError(String message) {
    return 'Не удалось выдать таймаут: $message';
  }

  @override
  String get spaceSlowMode => 'Медленный режим';

  @override
  String get spaceSlowModeSubtitle =>
      'Минимальная пауза между сообщениями в этом канале';

  @override
  String get spaceSlowModeOff => 'Выкл.';

  @override
  String spaceSlowModeSeconds(int seconds) {
    return '$seconds с';
  }

  @override
  String get spaceAssignRole => 'Назначить роль';

  @override
  String get spaceAssignRoleTitle => 'Назначить роль';

  @override
  String get spaceAssignRoleEmpty => 'Нет доступных ролей';

  @override
  String spaceAssignRoleError(String message) {
    return 'Не удалось назначить роль: $message';
  }

  @override
  String get chatGroupMembersTooltip => 'Участники группы';

  @override
  String get chatGroupMembersTitle => 'Участники';

  @override
  String get chatGroupMembersSubtitle =>
      'Владелец может удалять участников. Участники могут выйти из группы.';

  @override
  String get chatGroupMembersLoadError => 'Не удалось загрузить участников';

  @override
  String get chatGroupRoleOwner => 'Владелец';

  @override
  String chatGroupMemberYou(String name) {
    return '$name (вы)';
  }

  @override
  String get chatGroupKick => 'Удалить';

  @override
  String get chatGroupKickConfirmTitle => 'Удалить участника?';

  @override
  String chatGroupKickConfirmMessage(String name) {
    return 'Удалить $name из группы?';
  }

  @override
  String get chatGroupLeave => 'Выйти из группы';

  @override
  String get chatGroupLeaveConfirmTitle => 'Выйти из группы?';

  @override
  String get chatGroupLeaveConfirmMessage =>
      'Вы больше не будете получать сообщения из этой группы.';

  @override
  String get chatGroupOwnerLeaveHint =>
      'Владелец не может выйти, пока не передаст права (скоро).';

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

  @override
  String get gameCatalogTitle => 'Каталог игр';

  @override
  String get gameCatalogEntry => 'Каталог игр';

  @override
  String get gameCatalogSearchHint => 'Поиск игр';

  @override
  String get gameCatalogLoadError => 'Не удалось загрузить каталог игр';

  @override
  String get gameCatalogEmpty => 'Игры не найдены';

  @override
  String get gameCatalogInGameRoles => 'Игровые роли';

  @override
  String get gameCatalogRankLadder => 'Ранги';

  @override
  String gameCatalogRegions(String regions) {
    return 'Регионы: $regions';
  }

  @override
  String gameCatalogModeSlots(int slots, int min, int max) {
    return '$slots игроков · пати $min–$max';
  }

  @override
  String get playerProfileTitle => 'Профиль игрока';

  @override
  String get playerProfileEntry => 'Профиль игрока';

  @override
  String get playerProfileLoadError => 'Не удалось загрузить профиль игрока';

  @override
  String get playerProfileEmpty => 'Игры ещё не настроены';

  @override
  String get playerProfileAddGame => 'Добавить игру';

  @override
  String get playerProfileSave => 'Сохранить';

  @override
  String get playerProfileSection => 'Матчмейкинг';

  @override
  String get playerProfileForGame => 'Мой профиль для этой игры';

  @override
  String get queueSearchStart => 'Встать в очередь';

  @override
  String get queueSearchTitle => 'Поиск тиммейтов';

  @override
  String get queueSearchSearching => 'Ищем тиммейтов…';

  @override
  String get queueSearchCancel => 'Отменить поиск';

  @override
  String get queueSearchRegion => 'Регион';

  @override
  String get queueSearchRole => 'Ваша роль';

  @override
  String get queueSearchRank => 'Ваш ранг';

  @override
  String get queueSearchSoughtRankMin => 'Мин. ранг в поиске';

  @override
  String get queueSearchSoughtRankMax => 'Макс. ранг в поиске';

  @override
  String get queueSearchStartError => 'Не удалось начать поиск';

  @override
  String get queueSearchCancelError => 'Не удалось отменить поиск';

  @override
  String get matchFoundTitle => 'Матч найден';

  @override
  String matchFoundSubtitle(String gameName, String mode) {
    return '$gameName · $mode';
  }

  @override
  String get matchFoundAccept => 'Принять';

  @override
  String get matchFoundDecline => 'Отклонить';

  @override
  String get matchFoundRespondError => 'Не удалось ответить на матч';

  @override
  String get matchSquadLeave => 'Выйти из отряда';

  @override
  String get matchSquadLeaveError => 'Не удалось выйти из матч-отряда';

  @override
  String get matchRatingTitle => 'Оцените тиммейтов';

  @override
  String get matchRatingSubtitle =>
      'Звёзды можно пропустить для каждого игрока.';

  @override
  String get matchRatingSkipTeammate => 'Пропустить';

  @override
  String get matchRatingSkipAll => 'Пропустить всех';

  @override
  String get matchRatingSkipped => 'Пропущено';

  @override
  String get matchRatingSubmit => 'Отправить оценки';

  @override
  String get matchRatingBanTitle => 'Забанить в матчмейкинге?';

  @override
  String matchRatingBanMessage(String name) {
    return 'Больше не матчиться с $name?';
  }

  @override
  String get matchRatingBanCancel => 'Отмена';

  @override
  String get matchRatingBanConfirm => 'Забанить';

  @override
  String get matchRatingBanAction => 'Бан в ММ';

  @override
  String profileMmRating(String rating) {
    return 'Рейтинг ММ: $rating ★';
  }
}
