# T-CHAT: multi-client acceptance contract

Этот контракт замыкает один пользовательский путь `A1` и локальный gate `G0`.
Он не меняет продуктовую семантику: ожидания ниже собраны из
[`PLAN.md`](../PLAN.md), [`text-chat.md`](../features/text-chat.md),
[`ARCHITECTURE_REQUIREMENTS.md`](../ARCHITECTURE_REQUIREMENTS.md) и документов
Auth, Social, Chat, Messaging, Realtime и API Gateway.

## Граница доказательства

Два новых regular account проходят регистрацию/логин через публичный Gateway,
находят друг друга, обрабатывают friend/message request и переписываются в DM,
standalone group и Space channel. Доказательство использует реальные
PostgreSQL, Redis, NATS, Gateway REST и Realtime WebSocket из изолированного
Compose project. Нельзя подменять ответы сервисов фиктивными transport-ответами
или считать наличие маршрута доказательством поведения.

До block два WebSocket tab одного profile и socket peer открываются при работающем
стеке. Runner затем останавливает NATS: публичный `BlockAccount` успешно сохраняет
block, а проигнорированный `PublishUserBlocked` error гарантирует отсутствие
revoke event. Те же открытые sockets должны получить generic deny на `typing`,
`mark_read`, `delivery_ack`, без peer/same-profile fan-out и durable ack publish.

После реального restart Chat, Messaging и Realtime новый WebSocket сначала
принимает `hello`; `resume.last_s` относится только к новой connection-local
последовательности. Отдельно после reconnect клиент обязан выполнить REST
`ListChats` для `main`, `requests`, `archive` до пустого `next_cursor`. Сбой
любой страницы сохраняет ранее подтверждённые строки и metadata этого scope до
явного retry. История запрашивается через Messaging только для выбранного
`chat_id`, своим message cursor; она не восстанавливается через WS resume.

## Acceptance matrix

| ID | Обязательное наблюдение | Канонический источник | Исполняемое доказательство |
|---|---|---|---|
| TC-01 | Два новых пользователя получают разные account/profile/session, находят друг друга и завершают friend request/contact | `PLAN.md` A1; `auth-service.md`; `social-service.md` | `TestComposeA1TwoAccountsFoundation_live`; prepare в `TestComposeTChatBlockedDMTransportClosure_live` |
| TC-02 | Первое сообщение незнакомцу находится только в `requests`; Accept переводит DM в `main` | `text-chat.md` «Запросы сообщений»; `chat-service.md` | `TestComposeA1DailyMessagingREST_live`; новый prepare использует тех же двух пользователей |
| TC-03 | DM, standalone group и Space channel доступны участникам через Gateway, сообщения сохраняются | `PLAN.md` A1; `chat-service.md`; `messaging-service.md` | `TestComposeA1DailyMessagingREST_live` |
| TC-04 | DM/group/channel имеют независимый per-member `unread → read`; только REST `MarkRead` меняет durable cursor | `text-chat.md` «Статусы доставки»; `messaging-service.md` `MarkRead` | `TestComposeA1TwoAccountsFoundation_live`, `TestComposeA1GroupReadIsolation_live`, `TestComposeA1ChannelReadIsolation_live` |
| TC-05 | После restart клиент получает новый WS `hello`; `resume` не заменяет durable recovery | `ARCHITECTURE_REQUIREMENTS.md` «Reconnect»; `realtime-service.md` checklist | трёхфазный `TestComposeTChatBlockedDMTransportClosure_live`; `TestComposeWSResume_live` |
| TC-06 | После reconnect REST snapshot и WS resume утверждаются раздельно | `PLAN.md` A1; `chat-service.md` «Reconnect» | `t055_profile_switch_reconnect_inbox_e2e_live_test.dart`; новый verify сначала выполняет REST recovery, затем WS `resume` |
| TC-07 | `main`, `requests`, `archive` проходят минимум две реальные страницы; failed page сохраняет cache и retry cursor | `ARCHITECTURE_REQUIREMENTS.md`; `chat-service.md` «Reconnect» | `t055_profile_switch_reconnect_inbox_e2e_live_test.dart` |
| TC-08 | Полная история догружается cursor-пагинацией только для выбранного `chat_id` | `messaging-service.md` `GetMessages`; `realtime-service.md` checklist | `t055_profile_switch_reconnect_inbox_e2e_live_test.dart`; `requireTChatCursorHistory` до и после restart |
| TC-09 | Archive per-profile; block запрещает DM send в обе стороны; account soft-delete отзывает sessions, скрывает fresh DM snapshot и даёт один local terminal marker для уже загруженной истории | `text-chat.md` «Архивирование»; `privacy.md`; `PLAN.md` A1 | `TestComposeA1DailyMessagingREST_live`, `TestComposeA1BlockDMDenyBothDirections_live`, `t106_account_soft_delete_e2e_live_test.dart` |
| TC-10 | После block/delete альтернативный transport не возвращает доступ, который основной REST path уже запретил; успешный block закрывает уже открытый WS и без Social event | `privacy.md`: DM недоступен с обеих сторон; `PLAN.md`: deny не fail-open; `api-gateway.md`/`ARCHITECTURE_REQUIREMENTS.md`: REST и WS — клиентские ingress | GREEN: NATS-off active phase и restart verify в `TestComposeTChatBlockedDMTransportClosure_live`; Realtime policy/event/barrier regressions |

`TC-01..TC-10` образуют один acceptance gate. Отдельно зелёный тест на каждый
компонент не заменяет последовательный multi-client run: identities, chat IDs,
cursor и deny state должны происходить из одной реальной fixture chain там, где
это отмечено трёхфазным тестом.

## RED → GREEN evidence

Команда:

```bash
VOICE_TCHAT_CLOSURE_CLEANUP=true bash scripts/ci/compose-tchat-multiclient-red.sh
```

Runner создаёт уникальный Compose project и диапазон host ports, запускает
prepare, затем останавливает NATS и запускает active phase с теми же durable IDs.
После authoritative active-socket deny он возвращает NATS, перезапускает Chat,
Messaging и Realtime и запускает restart verify. State
file создаётся с mode `0600` во временном каталоге и удаляется runner'ом.

На исходном SHA `9757f7c25a8b6ed736006624a5392a5849f28614` сохранённый RED относится к
`TC-10`: REST send после Social block возвращает `403` до и после restart, но
Realtime bootstrap/lazy `subscribe` опирается на Chat membership и принимает тот
же blocked DM. Это открывает WS side effects для deny-состояния. RED нельзя
«исправлять» ослаблением ожидания, ручным unsubscribe или client-only фильтром.

GREEN сохраняет одну трёхфазную prepare → active → verify fixture chain и публичные
REST/WS assertions. Перед verify runner перезапускает сервисы. Active phase открывает sockets до block и намеренно лишает
Social возможности доставить event. Realtime теперь исключает blocked DM из свежего
`subscription_sync`, отвечает generic `permission_denied` на lazy subscribe и
через per-instance consumer `social.user_blocked` отзывает уже открытые локальные
DM subscriptions. Независимый authoritative Social read перед client side effects
закрывает event-loss случай. Regression tests отдельно доказывают отсутствие
`typing`, `mark_read` и `delivery_ack` side effects, bounded pair index, barrier
`check/event/add` и неизменные group/channel membership semantics.

## Реализованные GREEN slices

1. Realtime authorization получает fail-closed account-pair block decision для
   DM bootstrap и lazy `subscribe`; dependency failure не превращается в allow.
2. Social block event отзывает уже активные DM subscriptions на всех Realtime
   instances; synchronous Social read на каждой DM client operation сохраняет
   correctness при publish/delivery loss.
3. Те же negative checks применяются к `typing`, `mark_read` и `delivery_ack`,
   включая соединение, открытое до block/delete.
4. Трёхфазный RED переведён в GREEN; затем одним изолированным прогоном
   выполняются существующие A1 Go и Flutter proofs из матрицы без изменения их
   assertions.

Account soft-delete имеет отдельный `T106` contract и не должен быть смешан с
полной 30-дневной erasure из `A4`.
