# Brand and UX (Voice client)

Канон для Penpot, design tokens и Flutter. Продуктовое поведение и фазность остаются в `docs/features/` и [PLAN.md](../PLAN.md); этот документ описывает, **как** показывать уже описанные фичи в интерфейсе.

## Product UX North Star

Voice должен ощущаться как быстрый мессенджер с сильным голосовым и игровым слоем:

- **Telegram first:** для базового общения, DM, списков, поиска, пересылки, медиа, настроек и быстрых действий берём простоту Telegram: минимум экранов, понятные списки, быстрый доступ к чату, ненавязчивые empty/error states.
- **Discord second:** когда у Telegram нет подходящего паттерна, особенно для спейсов, дерева каналов, голосовых комнат, ролей, presence и community-flow, берём UX-логику Discord, но визуальный стиль остаётся messaging-soft (не Discord chrome).
- **Voice native:** для новых фич вроде матчмейкинга выбираем самый короткий путь к результату: пользователь должен понимать состояние системы, следующий шаг и способ отмены без чтения справки.
- **No surprise UX:** destructive-действия требуют подтверждения, long-running-сценарии имеют visible progress, ошибки объясняют действие для восстановления.

## Визуальный стиль

- **Flat:** без декоративных теней, градиентов и стеклянных эффектов. Elevation только у модалок, меню, bottom sheet и transient overlays.
- **Спокойная геометрия:** messaging surface мягче (bubbles `radius.bubble`, search/badge `radius.pill`); chrome и space tree — спокойные `radius.sm`/`md`/`lg`. Аватары — круг. Канон размеров — [tokens.md](tokens.md) / `voice.tokens.json`.
- **Слои вместо декора:** читаемость строится на поверхности, бордерах, отступах и типографике, а не на цветных панелях.
- **Акцент точечно:** primary CTA, send button, выбранный профиль, focus ring, активный элемент навигации. Не красить каждую иконку и каждый list item.
- **Светлая тема:** белый и светло-серые поверхности, тёмно-серый текст, мягкие разделители.
- **Тёмная тема:** тёмно-серые слои, не чистый чёрный canvas; избегать сине-фиолетового Discord-like залития.
- **High contrast:** отдельный набор токенов с усиленными границами и текстом, не "яркая dark theme".
- **Typography:** короткие заголовки, плотные списки, не больше 2 уровней визуального веса в одном блоке. Secondary text не должен становиться нечитаемым.

**Не делать:** Material `fromSeed`, indigo-by-default, цветные сайдбары "как Discord", neon accents, heavy shadows, glassmorphism, декоративные иллюстрации на каждом empty state, большие круглые карточки без причины.

## UX References

| Сценарий | Базовый референс | Что брать |
|----------|------------------|-----------|
| DM, список чатов, поиск, медиа, forwarding | **Telegram** | Скорость, компактность, inline-действия, bottom sheets, минимум модалок. |
| Desktop shell, спейсы, голосовые комнаты, роли | **Discord** IA + soft chrome | Дерево каналов, members/voice, rail; визуал — [tokens.md](tokens.md) (`layout.*`, messaging radii), не Discord skin. |
| Матчмейкинг, игровые сценарии | **Voice native** | Пошаговую ясность, прозрачное состояние очереди, простой rollback/cancel. |

Если Telegram и Discord расходятся, приоритет такой:

1. Быстрее и понятнее для нового пользователя.
2. Меньше permanent chrome на экране.
3. Лучше работает на Web/Windows в первой версии.
4. Не ломает будущий mobile layout.

app stack: inline / side panel / bottom sheet; full-screen только где необходимо (Auth, mobile chat, critical flows).

## Layout Principles

- **Desktop/Web:** основа — three-column shell из [navigation.md](../features/navigation.md): активные/папки, список чатов, открытый чат. Левая колонка не должна соревноваться с контентом: нейтральный фон, компактные строки, badge только где есть сигнал.
- **Mobile:** один главный фокус на экран. Навигация сворачивается, но пользователь не должен терять unread/status в других чатах.
- **Density:** ближе к Telegram Desktop: много информации без ощущения тесноты. Вертикальные отступы списков держать компактными, но touch targets на mobile не меньше доступного минимума.
- **No zero inset:** текст, заголовки и контролы не лижут край экрана или рамку колонки — минимум `space.16` (16 px). Исключения и AccentBar — [penpot-workflow.md](penpot-workflow.md) §1.5.
- **Panels before pages:** для профиля, информации о чате, списка участников, настроек уведомлений и быстрых действий предпочтительны панели/sheets. Отдельная страница нужна, если сценарий длинный или требует своего состояния.
- **Stable layout:** новые события в чате не должны сдвигать фокус пользователя. Если пользователь читает историю выше, показывать "новые сообщения" вместо автоскролла.

## Components And States

- **Buttons:** один primary CTA на область. Secondary actions — text/outlined/icon button по важности. Destructive actions отделять цветом и подтверждением.
- **Inputs:** ошибки рядом с полем, не только toast. Для поиска — debounce и состояние "ничего не найдено" с понятным следующим действием.
- **Lists:** avatar/title/subtitle/meta/badge как базовый паттерн. Строки кликабельны целиком; вторичные действия через hover/menu/long press.
- **Menus:** короткие, контекстные, с одинаковым порядком действий. Опасные пункты внизу.
- **Toasts/snackbars:** только для transient feedback. Не использовать toast как единственный способ показать ошибку формы или блокер.
- **Empty states:** короткая причина + один следующий шаг. Не превращать empty state в маркетинговый баннер.
- **Loading:** skeleton для списков, inline spinner для кнопки, progress для долгих операций. Не блокировать весь экран, если можно заблокировать только область.
- **Offline/reconnect:** явная плашка или compact banner; пользователь должен понимать, что ввод сохранён локально или отправка не выполнена.

## Messaging UX

- **Chat first:** открыть, прочитать, написать — основной happy path. В app stack не добавлять UI-шум будущих фич.
- **Composer:** поле ввода всегда рядом с историей; send CTA видимый, Enter-to-send на desktop, настройки поведения позже через preferences.
- **Message grouping:** группировать соседние сообщения одного отправителя по времени; повторять avatar/name только при смене отправителя или значительной паузе.
- **Unread:** unread badge в списках, separator в истории, быстрый переход к первому непрочитанному.
- **Context actions:** reply/edit/delete/reaction/forward появляются по фазам из [PLAN.md](../PLAN.md); место под них проектировать через контекстное меню, не через постоянные кнопки на каждом сообщении.
- **DM from strangers:** до отдельной папки запросов ([text-chat.md](../features/text-chat.md)) не обещать отдельный inbox в UI; когда появится, паттерн ближе к Telegram requests: заметно, но не мешает основному списку.

## Voice UX

- **1:1 calls:** входящий звонок должен быть мгновенно понятен: кто звонит, принять, отклонить, mute before join где применимо. Не открывать тяжёлый экран, если компактный overlay решает задачу.
- **Voice rooms:** для спейсов брать Discord-like mental model: видимая комната, участники внутри, join/leave рядом с контекстом комнаты.
- **Call controls:** mute, deafen/speaker, screen share, leave — стабильные позиции. Красный цвет только для hangup/leave/destructive.
- **Connection state:** connecting/reconnecting/degraded показывать рядом с голосовым контролом, не только в dev logs.
- **Push-to-talk:** в desktop UX должен быть discoverable shortcut и явная индикация активного PTT.

## Spaces And Community UX

- **Space tree:** Discord-like дерево категорий, текстовых чатов и голосовых комнат, но нейтральный visual treatment. Каналы не должны выглядеть как рекламные карточки.
- **Roles and permissions:** когда действие недоступно, лучше объяснить permission reason в disabled state/tooltip, чем скрывать всё без следа.
- **Invites:** invite flow короткий: создать/скопировать/настроить срок и лимит. Defaults безопасные, advanced settings спрятаны.
- **Moderation:** действия модерации не смешивать с обычными действиями участника; destructive flow требует подтверждения и объясняет последствия.

## Matchmaking UX

Матчмейкинг — ключевое отличие продукта, но UX должен быть проще формы в админке:

- **Start queue:** один явный entry point из игры/спейса/глобального поиска. Пользователь видит игру, режим, роль, регион, размер команды и может быстро изменить параметры.
- **Queue state:** показывать статус "ищем", примерное ожидание если есть данные, текущий состав/слоты, cancel без наказания если правила не говорят иначе.
- **Match found:** высокий приоритет, но без паники: принять/отклонить, таймер только если он реально нужен продуктовой логикой.
- **Party created:** после успешного подбора пользователь попадает в готовый текст+voice контекст, а не в промежуточную страницу.
- **Recovery:** если кто-то отказался или отвалился, объяснить, что произошло, и предложить вернуться в очередь.

Не вводить ранги, проверки, penalties или игровые API в UI, если они не описаны в feature specs.

## Motion And Feedback

- **Motion restrained:** быстрые 120-180ms transitions для panel/menu/sheet; длинные анимации не нужны.
- **Reduced motion:** все анимации должны иметь статичный вариант, см. [accessibility.md](../features/accessibility.md).
- **Feedback near action:** результат операции показывать там, где пользователь действовал. Глобальный toast — только дополнительный сигнал.
- **Optimistic UI:** допустим для сообщений и lightweight actions, если есть понятный failed state и retry.

## Copy And Localization

- Тон: коротко, спокойно, без мемов и "геймерского" сленга в системных сообщениях.
- RU/EN строки должны быть равноправными; не проектировать layout только под короткий английский текст.
- CTA должен описывать действие: "Send", "Join voice", "Start queue", "Cancel search", а не абстрактное "OK".
- Ошибки: причина + что сделать дальше. Не показывать backend code пользователю.

## Design Review Checklist

Перед реализацией или Penpot-фреймом проверить:

- Сценарий есть в `docs/features/` или [PLAN.md](../PLAN.md), новая продуктовая логика не придумана в дизайне.
- Используются tokens из [tokens.md](tokens.md), нет raw hex в feature UI.
- Telegram-паттерн применён для базового messaging UX; Discord-паттерн — только там, где он лучше подходит для voice/community.
- На экране один primary CTA и понятный next step для empty/loading/error/offline.
- Desktop и mobile имеют реалистичный layout, без скрытых critical actions.
- Keyboard/focus/contrast не противоречат [accessibility.md](../features/accessibility.md).

## Accent по профилю

См. [multi-profile.md](../features/multi-profile.md): у каждого профиля своя цветовая индикация.

- Нейтраль (фон, текст) — общий для приложения, modes `light` / `dark` / `high_contrast` в tokens.
- **Accent** — на профиль: пользователь может задать свой hex (позже User Service); иначе дефолт по индексу профиля на аккаунте (0-based).
- При переключении профиля accent обновляется без перезапуска приложения.

Дефолты (`profileAccent.defaults` в [voice.tokens.json](../../design/tokens/voice.tokens.json)):

| Индекс | Цвет | Hex |
|--------|------|-----|
| 0 | Небесно-голубой | `#7EC8E3` |
| 1 | Пастельно-зелёный | `#9ED9A6` |
| 2 | Пастельно-красный | `#F0A8A8` |
| 3 | Жёлтый | `#F5E6A3` |
| 4 | Оранжевый | `#FFCC99` |
| 5 | Фиолетовый | `#C9B8FF` |
| 6 | Розовый | `#FFB3E6` |

Далее: `index % defaults.length`.

Follow-up (не app stack tokens PR): поле `accent_color` в `profiles` (User Service) + API.
