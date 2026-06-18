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
  String get authContinueGuest => 'Продолжить как гость';

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
  String get globalSearchHint => 'Поиск контактов, спейсов и сообщений';

  @override
  String get globalSearchContacts => 'Контакты';

  @override
  String get globalSearchSpaces => 'Спейсы';

  @override
  String get globalSearchMessages => 'Сообщения';

  @override
  String get globalSearchStartHint =>
      'Введите запрос для поиска по чатам и спейсам.';

  @override
  String get globalSearchEmptyContacts => 'Контакты не найдены';

  @override
  String get globalSearchEmptySpaces => 'Спейсы не найдены';

  @override
  String get globalSearchEmptyMessages => 'Сообщения не найдены';

  @override
  String get inChatSearchHint => 'Поиск в этом чате';

  @override
  String get inChatSearchPrevious => 'Предыдущее совпадение';

  @override
  String get inChatSearchNext => 'Следующее совпадение';

  @override
  String inChatSearchResultScore(String score) {
    return 'Релевантность $score';
  }

  @override
  String get inChatSearchOpen => 'Поиск по сообщениям';

  @override
  String get socialAddFriend => 'Добавить в друзья';

  @override
  String get socialRemoveFriend => 'Удалить из друзей';

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
  String get socialProfileUnavailable => 'Пользователь недоступен';

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
  String get botTimeoutError => 'Бот не ответил вовремя. Попробуйте позже.';

  @override
  String get botDeferredProcessing => 'Обрабатываю запрос…';

  @override
  String get slashCommandsTitle => 'Команды';

  @override
  String get slashCommandsLoadError => 'Не удалось загрузить команды ботов.';

  @override
  String get slashCommandsEmpty => 'В этом чате нет команд ботов.';

  @override
  String get botUnavailableTooltip => 'Бот недоступен';

  @override
  String get botOnlineStatus => 'Бот в сети';

  @override
  String get botOfflineStatus => 'Бот офлайн';

  @override
  String get botInstallTitle => 'Установка бота';

  @override
  String get botInstallDescriptionHeading => 'Описание';

  @override
  String get botInstallScopesHeading => 'Разрешения';

  @override
  String get botInstallCommandsHeading => 'Команды';

  @override
  String get botInstallCommandsEmpty =>
      'Slash-команды появятся после регистрации.';

  @override
  String get botInstallWhitelistHeading => 'Установить в спейс';

  @override
  String get botInstallSelectSpace => 'Выберите спейс';

  @override
  String get botInstallNoSpaces =>
      'Вступите или создайте спейс, чтобы установить бота.';

  @override
  String get botInstallConfirm => 'Установить бота';

  @override
  String slashOptionPickUser(String name) {
    return 'Пользователь: $name';
  }

  @override
  String slashOptionPickChannel(String name) {
    return 'Канал: $name';
  }

  @override
  String slashOptionPickRole(String name) {
    return 'Роль: $name';
  }

  @override
  String slashOptionPickAttachment(String name) {
    return 'Вложение: $name';
  }

  @override
  String slashOptionAttachmentSelected(String fileName) {
    return 'Выбрано: $fileName';
  }

  @override
  String get slashOptionPickerUnavailable => 'Выбор недоступен в этом чате.';

  @override
  String get slashCommandRun => 'Выполнить команду';

  @override
  String get chatBotsSectionTitle => 'Боты';

  @override
  String get chatBotsLoadError => 'Не удалось загрузить ботов для чата.';

  @override
  String get chatBotsEmpty => 'В спейсе нет установленных ботов.';

  @override
  String get spaceBotsTitle => 'Боты спейса';

  @override
  String get spaceBotsInstall => 'Установить бота';

  @override
  String get spaceBotsUninstall => 'Удалить из спейса';

  @override
  String get spaceBotsInstallConfirm => 'Установить';

  @override
  String get spaceBotsScopeWarning =>
      'Бот запрашивает привилегированный доступ к истории чата.';

  @override
  String get spaceBotsPrivilegedAck =>
      'Я понимаю, что бот может читать историю чата';

  @override
  String get spaceBotsSelectChats => 'Разрешённые текстовые чаты';

  @override
  String get spaceBotsInstallSuccess => 'Бот установлен.';

  @override
  String get spaceBotsUninstallSuccess => 'Бот удалён из спейса.';

  @override
  String get ephemeralMessageLabel => 'Только для вас';

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
  String get chatOfflineReadOnly => 'Нет сети. Показаны сохранённые сообщения.';

  @override
  String get chatOfflineSendBlocked => 'Нельзя отправить сообщение без сети.';

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
  String get commonEdit => 'Изменить';

  @override
  String get commonDelete => 'Удалить';

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
  String get chatMessageReply => 'Ответить';

  @override
  String chatReplyingTo(String preview) {
    return 'Ответ на $preview';
  }

  @override
  String get chatThreadTitle => 'Тред';

  @override
  String get chatThreadEmpty => 'Пока нет ответов';

  @override
  String get chatChannelMainFeedBlocked => 'Пишите в тред или от имени канала';

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
  String get spaceRolesTooltip => 'Управление ролями';

  @override
  String get spaceRolesTitle => 'Роли';

  @override
  String get spaceRolesLoadError => 'Не удалось загрузить роли';

  @override
  String get spaceRoleCreateTitle => 'Создать роль';

  @override
  String get spaceRoleEditTitle => 'Редактировать роль';

  @override
  String get spaceRoleNameLabel => 'Название роли';

  @override
  String get spaceRoleManaged => 'Системная роль';

  @override
  String get spaceRoleCustom => 'Кастомная роль';

  @override
  String get spaceChatOverrideTitle => 'Оверрайды доступа к чату';

  @override
  String get spaceChatOverrideHint =>
      'Запретить просмотр или отправку для роли только в этом чате.';

  @override
  String get spaceChatOverrideDenyView => 'Запретить просмотр';

  @override
  String get spaceChatOverrideDenySend => 'Запретить отправку';

  @override
  String get spaceVoiceOverrideTitle => 'Оверрайды голосовой комнаты';

  @override
  String get spaceVoiceOverrideHint =>
      'Запретить вход для роли только в этой комнате.';

  @override
  String get spaceVoiceOverrideDenyJoin => 'Запретить вход в голос';

  @override
  String get spaceSetDefaultJoinRole => 'Роль при вступлении';

  @override
  String spaceDefaultJoinRole(String name) {
    return 'Роль при вступлении: $name';
  }

  @override
  String get spaceRevokeRole => 'Снять';

  @override
  String spaceRevokeRoleError(String message) {
    return 'Не удалось снять роль: $message';
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
  String get chatInfoTitle => 'Информация о чате';

  @override
  String get chatInfoOpen => 'Информация о чате';

  @override
  String get chatSharedMediaTabMedia => 'Медиа';

  @override
  String get chatSharedMediaTabFiles => 'Файлы';

  @override
  String get chatSharedMediaTabLinks => 'Ссылки';

  @override
  String get chatSharedMediaTabVoice => 'Голосовые';

  @override
  String get chatSharedMediaEmpty => 'Пока ничего нет';

  @override
  String get chatSharedMediaLoadError => 'Не удалось загрузить медиа';

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
      'Передайте владение другому участнику перед выходом.';

  @override
  String chatGroupTransferOwnershipTo(String name) {
    return 'Передать владение: $name';
  }

  @override
  String get chatGroupTransferOwnershipTitle => 'Передать владение группой';

  @override
  String chatGroupTransferOwnershipMessage(String name) {
    return 'Сделать $name новым владельцем?';
  }

  @override
  String get chatGroupTransferOwnershipConfirm => 'Передать';

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
  String get settingsReducedMotion => 'Уменьшить анимацию';

  @override
  String get settingsHelp => 'Помощь';

  @override
  String get settingsHelpTitle => 'Помощь';

  @override
  String get settingsHelpChatsTitle => 'Чаты';

  @override
  String get settingsHelpChatsBody =>
      'ЛС, группы и каналы — в списке чатов. Используйте папки для организации.';

  @override
  String get settingsHelpSpacesTitle => 'Спейсы';

  @override
  String get settingsHelpSpacesBody =>
      'Вступайте в спейсы или создавайте сообщества с каналами и войс-комнатами.';

  @override
  String get settingsHelpMatchmakingTitle => 'Матчмейкинг';

  @override
  String get settingsHelpMatchmakingBody =>
      'Ищите команду по игре и критериям во вкладке матчмейкинга.';

  @override
  String get settingsHelpVoiceTitle => 'Голос';

  @override
  String get settingsHelpVoiceBody =>
      'Подключайтесь к войс-комнатам в спейсах или звоните из ЛС.';

  @override
  String get settingsHelpFooter =>
      'Нужна помощь? Напишите в поддержку через настройки аккаунта.';

  @override
  String get settingsSubscription => 'Подписка';

  @override
  String get subscriptionSettingsTitle => 'Подписка';

  @override
  String get subscriptionCurrentPlan => 'Текущий план';

  @override
  String get subscriptionStatusFree => 'Бесплатный';

  @override
  String get subscriptionStatusPremium => 'Премиум';

  @override
  String subscriptionBillingPeriod(String period) {
    return 'Оплата: $period';
  }

  @override
  String get subscriptionUpgradeTitle => 'Перейти на Премиум';

  @override
  String get subscriptionUpgradeMonthly => 'Премиум — помесячно';

  @override
  String get subscriptionUpgradeYearly => 'Премиум — годовой (−20%)';

  @override
  String get subscriptionManageBilling => 'Управление подпиской';

  @override
  String get subscriptionCancel => 'Отменить подписку';

  @override
  String get subscriptionProfilesLoadError => 'Не удалось загрузить профили';

  @override
  String get downgradeProfilePickerTitle =>
      'Выберите 2 профиля, которые останутся активными';

  @override
  String get downgradeProfilePickerHint =>
      'Остальные профили будут заморожены до продления Премиума.';

  @override
  String get downgradeProfilePickerConfirm => 'Сохранить выбранные профили';

  @override
  String get downgradeProfilePrimary => 'Основной профиль';

  @override
  String get premiumBadgeLabel => 'Премиум';

  @override
  String get settingsSecurity => 'Безопасность и доверие';

  @override
  String get securitySettingsTitle => 'Безопасность';

  @override
  String get verificationSettingsTitle => 'Верификация';

  @override
  String get verificationSettingsHint =>
      'Привяжите платформы для значка верификации или подтвердите домен организации.';

  @override
  String get verificationLinkedAccountsTitle => 'Связанные аккаунты';

  @override
  String get verificationLinkedAccountsEmpty => 'Связанных аккаунтов пока нет.';

  @override
  String get verificationLinkTwitch => 'Привязать Twitch';

  @override
  String get verifiedBadgePersonal => 'Верифицирован';

  @override
  String get verifiedBadgeOrganization => 'Верифицированная организация';

  @override
  String get security2faEnableTitle => 'Двухфакторная аутентификация';

  @override
  String get security2faEnableHint =>
      'Подтвердите пароль, чтобы начать настройку 2FA.';

  @override
  String get security2faContinue => 'Продолжить';

  @override
  String get security2faScanQr =>
      'Отсканируйте QR-код в приложении-аутентификаторе';

  @override
  String get security2faBackupCodesTitle => 'Резервные коды (сохраните сейчас)';

  @override
  String get security2faVerifyTitle => 'Подтвердите аутентификатор';

  @override
  String get security2faVerifyHint =>
      'Введите 6-значный код или резервный код.';

  @override
  String get security2faVerify => 'Включить 2FA';

  @override
  String get security2faBackToQr => 'Назад к QR';

  @override
  String get security2faEnabled => 'Двухфакторная аутентификация включена.';

  @override
  String get privacySettingsTitle => 'Приватность';

  @override
  String get privacyLoadError => 'Не удалось загрузить настройки приватности';

  @override
  String get privacySaved => 'Настройки приватности сохранены';

  @override
  String get privacyPresetTitle => 'Пресет';

  @override
  String get privacyPresetPersonal => 'Личный';

  @override
  String get privacyPresetGaming => 'Игровой';

  @override
  String get privacyPresetWork => 'Рабочий';

  @override
  String get privacyAllowDm => 'Кто может писать в ЛС';

  @override
  String get privacyAllowGuestDm => 'Разрешить гостевые аккаунты в ЛС';

  @override
  String get privacyVisibilityTitle => 'Видимость';

  @override
  String get privacyShowOnline => 'Онлайн-статус';

  @override
  String get privacyShowGameStatus => 'Статус «в игре»';

  @override
  String get privacyShowMmRating => 'Рейтинг ММ';

  @override
  String get privacyShowPhone => 'Телефон';

  @override
  String get privacyShowStories => 'Стори';

  @override
  String get privacyAllowFriendRequests => 'Заявки в друзья';

  @override
  String get privacyAudienceEveryone => 'Все';

  @override
  String get privacyAudienceFriends => 'Друзья';

  @override
  String get privacyAudienceFriendsOfFriends => 'Друзья друзей';

  @override
  String get privacyAudienceNobody => 'Никто';

  @override
  String get privacyAudienceSpaceMembers => 'Участники спейса';

  @override
  String get privacyAudienceIncludeGuests => 'Гостевые аккаунты';

  @override
  String get privacyAudienceSpacesTitle => 'Спейсы';

  @override
  String get privacyAudienceSpacesEmpty => 'Вы не состоите ни в одном спейсе';

  @override
  String get privacyActionsTitle => 'Действия';

  @override
  String get privacyAllowPhoneSearch => 'Поиск по номеру телефона';

  @override
  String get privacyAllowCalls => 'Звонки';

  @override
  String get privacyAllowChatSpaceInvites => 'Приглашения в чаты и спейсы';

  @override
  String get privacyAllowFiles => 'Отправка файлов';

  @override
  String get privacyAllowVoiceMessages => 'Голосовые сообщения';

  @override
  String get reportAction => 'Пожаловаться';

  @override
  String get reportTitle => 'Жалоба';

  @override
  String get reportSubtitle => 'Выберите категорию. Мы рассмотрим обращение.';

  @override
  String get reportCategorySpam => 'Спам';

  @override
  String get reportCategoryHarassment => 'Домогательство';

  @override
  String get reportCategoryOffensive => 'Оскорбительный контент';

  @override
  String get reportCategoryFake => 'Фейк / выдача себя за другого';

  @override
  String get reportCategoryMmToxic => 'Читерство / токсик в ММ';

  @override
  String get reportCategoryOther => 'Другое';

  @override
  String get reportCommentLabel => 'Комментарий';

  @override
  String get reportCommentRequired =>
      'Обязательно для «Другое» (до 500 символов)';

  @override
  String get reportSubmit => 'Отправить жалобу';

  @override
  String get reportAcceptedTitle => 'Жалоба принята';

  @override
  String get reportAcceptedMessage =>
      'Мы рассмотрим её в ближайшее время. Статус жалобы не сообщается.';

  @override
  String get authTotpStepTitle => 'Код двухфакторной аутентификации';

  @override
  String get authTotpLabel => 'Код аутентификатора или резервный';

  @override
  String get authTotpHelper =>
      'Введите код из приложения-аутентификатора или резервный код.';

  @override
  String get authErrorTotpRequired =>
      'Введите код двухфакторной аутентификации.';

  @override
  String get authErrorInvalidTotp =>
      'Неверный код аутентификатора или резервный код.';

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
  String get versionUpdateNow => 'Обновить';

  @override
  String get versionRestartToUpdate => 'Перезапустить и обновить';

  @override
  String get profileBlock => 'Заблокировать';

  @override
  String get profileBlockConfirmTitle => 'Заблокировать пользователя?';

  @override
  String get profileBlockConfirmMessage => 'Он не сможет писать вам сообщения.';

  @override
  String get callVideoPlaceholder => 'Видео';

  @override
  String get screenShareStart => 'Демонстрация экрана';

  @override
  String get screenShareStop => 'Остановить демонстрацию';

  @override
  String get screenSharePause => 'Пауза';

  @override
  String get screenShareResume => 'Продолжить';

  @override
  String get screenShareQualityTitle => 'Качество демонстрации';

  @override
  String get screenShareQuality720p15 => '720p · 15 FPS';

  @override
  String get screenShareQuality720p30 => '720p · 30 FPS';

  @override
  String get screenShareLimitReached =>
      'В этом войсе уже 3 демонстрации экрана';

  @override
  String get screenShareWaitingForVideo => 'Ожидание видео…';

  @override
  String get themeLoadError => 'Не удалось загрузить тему';

  @override
  String get bootstrapRestoring => 'Восстановление сессии…';

  @override
  String get guestNicknameTitle => 'Выберите никнейм';

  @override
  String get guestNicknameSubtitle => 'Гостям нужно имя перед началом общения.';

  @override
  String get guestNicknameLabel => 'Никнейм';

  @override
  String get guestNicknameHint => 'Как вас увидят другие';

  @override
  String get guestNicknameContinue => 'Продолжить';

  @override
  String get guestConvertTitle => 'Создайте аккаунт';

  @override
  String get guestConvertSubtitle =>
      'Укажите email и пароль, чтобы сохранить чаты и профиль.';

  @override
  String get guestConvertSubmit => 'Создать аккаунт';

  @override
  String get guestSaveAccountReminder =>
      'Зарегистрируйте аккаунт, чтобы не потерять доступ.';

  @override
  String get guestSaveAccountReminderCta => 'Зарегистрироваться';

  @override
  String get privacyShowOnlineIncludeGuests =>
      'Гостевые аккаунты видят мой онлайн-статус';

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
  String get queueSearchNudgeTitle => 'Долго ищем';

  @override
  String get queueSearchNudgeBody => 'Попробуйте изменить параметры поиска.';

  @override
  String get queueSearchTimeoutTitle => 'Не удалось найти';

  @override
  String get queueSearchTimeoutBody => 'Найти не удалось, попробуйте снова.';

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
  String get matchRatingSubmitError => 'Не удалось отправить оценку';

  @override
  String get matchRatingBanError => 'Не удалось забанить в матчмейкинге';

  @override
  String profileMmRating(String rating) {
    return 'Рейтинг ММ: $rating ★';
  }

  @override
  String get matchHistoryTitle => 'Match history';

  @override
  String get matchHistoryEntry => 'Match history';

  @override
  String get matchHistoryLoadError => 'Could not load match history';

  @override
  String get matchHistoryEmpty => 'No match squads yet';

  @override
  String get matchHistoryParticipants => 'Teammates';

  @override
  String get matchHistoryStatusActive => 'Active';

  @override
  String get matchHistoryStatusCompleted => 'Completed';

  @override
  String get matchHistoryLoadMore => 'Load more';

  @override
  String get matchHistoryAddFriend => 'Add friend';

  @override
  String get e2eEnableTitle => 'Включить сквозное шифрование';

  @override
  String get e2eChatInfoSwitchLabel => 'Сквозное шифрование';

  @override
  String get e2eChatInfoKeyBackup => 'Резервная копия ключей';

  @override
  String get e2eEncryptFailed =>
      'Не удалось зашифровать сообщение. Откройте приложение на обоих устройствах и повторите попытку.';

  @override
  String get e2ePeerMissingPreKeys =>
      'У собеседника ещё не настроены ключи шифрования.';

  @override
  String get e2eEnableBody =>
      'Включён режим сквозного шифрования для этого чата.\n\nСообщения зашифрованы и недоступны серверу.\n— Глобальный поиск не найдёт тексты сообщений этого чата.\n— Локальный поиск работает только по истории на этом устройстве.\n— Вложения зашифрованы и автоматически удаляются через 90 дней.';

  @override
  String get e2eEnableConfirm => 'Включить E2E';

  @override
  String get e2eEnableCancel => 'Отмена';

  @override
  String get e2eDisableTitle => 'Отключить сквозное шифрование?';

  @override
  String get e2eDisableBody =>
      'При отключении E2E новые сообщения сохраняются на сервере в открытом виде, и серверный поиск снова будет работать.';

  @override
  String get e2eDisableConfirm => 'Отключить E2E';

  @override
  String get e2eDisableCancel => 'Отмена';

  @override
  String get e2eKeyBackupTitle => 'Резервная копия ключей E2E';

  @override
  String get e2eKeyBackupHint =>
      'Задайте пароль резервной копии, чтобы восстановить ключи на новом устройстве.';

  @override
  String get e2eKeyBackupPasswordLabel => 'Пароль резервной копии';

  @override
  String get e2eKeyBackupPasswordHintLabel =>
      'Подсказка к паролю (необязательно)';

  @override
  String get e2eKeyBackupSave => 'Сохранить копию';

  @override
  String get e2eKeyBackupRestore => 'Восстановить из копии';

  @override
  String get e2eUndecryptableGeneric =>
      'Сообщение зашифровано и недоступно на этом устройстве. Настройте резервную копию ключей.';

  @override
  String e2eUndecryptableBefore(String date) {
    return 'Сообщения до $date зашифрованы и недоступны на этом устройстве. Настройте резервную копию ключей.';
  }

  @override
  String get e2eInChatSearchLocalOnly =>
      'Шифрованный чат: поиск только по загруженной истории на устройстве';

  @override
  String get e2eChatSettingsEnable => 'Включить E2E';

  @override
  String get e2eChatSettingsDisable => 'Отключить E2E';

  @override
  String get e2eChatSettingsKeyBackup => 'Резервная копия ключей';

  @override
  String get e2eEncryptionCodeTitle => 'Код шифрования';

  @override
  String get e2eEncryptionCodeBody =>
      'Сверьте с собеседником в войсе или лично. Коды должны совпадать. Если коды не совпадают, переписка может быть не защищена от прослушивания.';

  @override
  String e2eIdentityKeyChangedTitle(String nick) {
    return 'У $nick сменился ключ шифрования';
  }

  @override
  String get e2eIdentityKeyChangedBody =>
      'Так бывает после переустановки приложения или смены устройства без резервной копии. Продолжайте только если вы это ожидали. При сомнении сверьте код в настройках чата.';

  @override
  String get e2eIdentityKeyChangedContinue => 'Продолжить';

  @override
  String get e2eIdentityKeyChangedDistrust => 'Не доверять';

  @override
  String get e2eFileRetentionNotice =>
      'Зашифрованные вложения в этом чате автоматически удаляются через 90 дней.';

  @override
  String get e2eAttachmentTapToDownload =>
      'Нажмите, чтобы скачать зашифрованный файл';

  @override
  String get e2eAttachmentDownloadFailed => 'Не удалось сохранить вложение';

  @override
  String get e2eAttachmentDecryptFailed => 'Не удалось расшифровать вложение';

  @override
  String get storyRingActiveLabel => 'Активная сторис';

  @override
  String get storyCreateTitle => 'Новая сторис';

  @override
  String get storyCreateTypeText => 'Текст';

  @override
  String get storyCreateTypePhoto => 'Фото';

  @override
  String get storyCreateTypeVideo => 'Видео';

  @override
  String get storyCreateTextLabel => 'Текст сторис';

  @override
  String get storyCreateCaptionLabel => 'Подпись';

  @override
  String get storyCreatePickMedia => 'Выбрать медиа';

  @override
  String get storyCreateSubmit => 'Опубликовать';

  @override
  String get storyCreateTextRequired => 'Введите текст сторис';

  @override
  String get storyCreateMediaRequired => 'Сначала выберите фото или видео';

  @override
  String get storyViewerEmpty => 'Нет сторис для просмотра';

  @override
  String get storyViewerLoadError => 'Не удалось загрузить сторис';

  @override
  String get storyViewerNoMedia => 'Медиа недоступно';

  @override
  String get storyViewerVideoPlaceholder =>
      'Воспроизведение видео недоступно в этой сборке';

  @override
  String get storyReactTooltip => 'Реакция';

  @override
  String get storyReactSent => 'Реакция отправлена';

  @override
  String get storyHighlightsTitle => 'Избранное';

  @override
  String get storyLfpTitle => 'Ищу пати';

  @override
  String get storyLfpGame => 'Игра';

  @override
  String get storyCreateVisibilityLabel => 'Кто видит сторис';

  @override
  String get storyCreateMentionLabel => 'Упомянуть друзей';

  @override
  String get storyCreateMentionHint => '@username';

  @override
  String get storyCreateGameTagLabel => 'Игра';

  @override
  String get storyCreateGameTagPick => 'Выбрать игру';

  @override
  String get storyCreateGameTagClear => 'Сбросить';

  @override
  String get storyCreateTextStyleLabel => 'Фон';

  @override
  String get storyVisibilityEveryone => 'Все';

  @override
  String get storyVisibilityFriends => 'Друзья';

  @override
  String get storyVisibilityCloseFriends => 'Близкие друзья';

  @override
  String get storyViewerReply => 'Ответить';

  @override
  String get storyViewerReplyHint => 'Приватный ответ';

  @override
  String get storyViewerReplySent => 'Ответ отправлен';

  @override
  String storyViewerViewCount(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count просмотров',
      many: '$count просмотров',
      few: '$count просмотра',
      one: '$count просмотр',
    );
    return '$_temp0';
  }

  @override
  String storyHighlightVisibility(String value) {
    return 'Видимость: $value';
  }

  @override
  String get storyLfpJoin => 'Присоединиться';

  @override
  String get storyLfpWrite => 'Написать';

  @override
  String get socialStoryCreate => 'Новая сторис';
}
