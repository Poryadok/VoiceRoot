# Moderation Service

## Обзор

Система модерации: жалобы, автоматическая модерация, санкции, апелляции.

**Язык**: Go
**БД**: PostgreSQL `moderation_db`

## Ответственность

- Жалобы на пользователей, сообщения, пространства
- Категории: spam, harassment, offensive, fake, cheating, other
- Авто-блокировка: 1% или 10+ жалоб за 24 часа → shadow ban
- Pattern spam detection → auto-mute
- Shadow ban (пользователь не знает о бане)
- Очередь ручной модерации
- Санкции: warning, temp ban, permanent ban
- Апелляции (1 на санкцию, 7 дней на подачу)
- ММ-специфичные баны (через Matchmaking Service)
- Федеративная модерация (нода отвечает за свой контент, нарушения → дефедерация)

## API (gRPC)

```protobuf
service ModerationService {
  // Жалобы
  rpc CreateReport(CreateReportRequest) returns (Report);
  rpc GetReport(GetReportRequest) returns (Report);
  rpc ListReports(ListReportsRequest) returns (ReportList); // admin
  rpc ResolveReport(ResolveReportRequest) returns (Report); // admin

  // Санкции
  rpc ApplySanction(ApplySanctionRequest) returns (Sanction);
  rpc RevokeSanction(RevokeSanctionRequest) returns (Empty);
  rpc GetAccountSanctions(GetAccountSanctionsRequest) returns (SanctionList);
  rpc GetActiveSanction(GetActiveSanctionRequest) returns (Sanction);

  // Апелляции
  rpc SubmitAppeal(SubmitAppealRequest) returns (Appeal);
  rpc ReviewAppeal(ReviewAppealRequest) returns (Appeal); // admin
  rpc GetAppeal(GetAppealRequest) returns (Appeal);

  // Auto-moderation
  rpc CheckMessage(CheckMessageRequest) returns (CheckResult); // internal, sync
  rpc GetAutoModStats(GetAutoModStatsRequest) returns (AutoModStats);

  // Shadow ban
  rpc IsShadowBanned(IsShadowBannedRequest) returns (IsShadowBannedResponse); // internal
}
```

## Модель данных

```
reports
├── id (UUID)
├── reporter_profile_id
├── target_type (user | message | space)
├── target_id (UUID)
├── category (spam | harassment | offensive | fake | cheating | other)
├── description (text, nullable)
├── evidence (jsonb — screenshots, message_ids)
├── status (pending | reviewing | resolved | dismissed)
├── assigned_to (admin profile_id, nullable)
├── resolved_at (nullable)
├── resolution (jsonb, nullable)
├── created_at
└── updated_at

sanctions
├── id (UUID)
├── target_account_id
├── type (warning | temp_ban | perm_ban | shadow_ban | mm_ban)
├── reason (text)
├── report_id (FK, nullable)
├── issued_by (profile_id)
├── expires_at (nullable)
├── revoked_at (nullable)
├── revoked_by (profile_id, nullable)
├── created_at
└── updated_at

appeals
├── id (UUID)
├── sanction_id (FK)
├── appellant_account_id
├── reason (text)
├── status (pending | approved | denied)
├── reviewed_by (profile_id, nullable)
├── reviewed_at (nullable)
├── review_notes (text, nullable)
├── created_at
└── UNIQUE(sanction_id) -- 1 appeal per sanction

auto_mod_log
├── id (UUID)
├── target_profile_id
├── trigger (spam_pattern | report_threshold)
├── action (mute | shadow_ban)
├── details (jsonb)
├── created_at
└── reverted_at (nullable)
```

## Авто-модерация

```
message.sent event ──► CheckMessage():
  1. Spam pattern detection (repeated messages, link flooding)
  2. Если trigger → auto-mute (5 мин первый раз, экспоненциально)

report.created event ──► Check thresholds:
  1. Count reports on target in 24h
  2. Если ≥10 или ≥1% от активных пользователей → shadow_ban
  3. Добавить в очередь ручной модерации
```

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`moderation.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие                       | Данные                               |
|-------------------------------|--------------------------------------|
| `moderation.report_created`   | report_id, target_type, target_id    |
| `moderation.report_resolved`  | report_id, resolution                |
| `moderation.sanction_applied` | sanction_id, target_account_id, type |
| `moderation.sanction_revoked` | sanction_id, target_account_id       |
| `moderation.appeal_submitted` | appeal_id, sanction_id               |
| `moderation.appeal_reviewed`  | appeal_id, status                    |
| `moderation.auto_action`      | target_id, trigger, action           |

## Зависимости

- **User Service** — получение данных о пользователе
- **Messaging Service** — получение содержимого сообщения при жалобе
- **Matchmaking Service** — ММ-баны
- **Notification Service** — (через NATS) уведомление о санкции
- **Federation Service** — (через NATS) дефедерация при нарушениях


