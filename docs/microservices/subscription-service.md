# Subscription Service

## Обзор

Управление подписками, биллингом и лимитами. Два плана: Premium (пользовательский) и Space Pro (для пространств).

**Язык**: Go
**БД**: PostgreSQL `subscription_db`

## Ответственность

- Планы: Premium ($5/мес), Space Pro ($5/мес), скидка -20% за год
- Платёжные провайдеры: Paddle (международный), CloudPayments (СНГ)
- Webhook обработка от провайдеров (создание, продление, отмена)
- Grace period 7 дней после неоплаты
- Downgrade: заморозка excess профилей, снижение лимитов
- Лимиты по подписке (source of truth для других сервисов)
- Федеративные ноды устанавливают свои лимиты

## Лимиты по плану

| Параметр              | Free       | Premium      | Space Pro    |
|-----------------------|------------|-------------|--------------|
| Размер файла          | 50 MB      | 200 MB      | —            |
| Retention файлов      | 90 дней    | Бессрочно   | —            |
| Профили               | 2          | 5           | —            |
| Пространства (join)   | 50         | 1000        | —            |
| Voice quality         | 480p       | 720p        | —            |
| Кастомный статус      | Нет        | Да          | —            |
| Premium username      | Нет        | Да          | —            |
| Анонимный просмотр    | Нет        | Да          | —            |
| Участники пространства| —          | —           | 5000         |
| Voice slots           | —          | —           | 128          |
| Каналы                | —          | —           | 500          |
| Custom emoji          | —          | —           | Да           |

## API (gRPC)

```protobuf
service SubscriptionService {
  // Подписки
  rpc GetSubscription(GetSubscriptionRequest) returns (Subscription);
  rpc CreateCheckoutSession(CreateCheckoutRequest) returns (CheckoutResponse);
  rpc CancelSubscription(CancelSubscriptionRequest) returns (Subscription);
  rpc ResumeSubscription(ResumeSubscriptionRequest) returns (Subscription);

  // Space Pro
  rpc GetSpaceSubscription(GetSpaceSubRequest) returns (SpaceSubscription);
  rpc CreateSpaceCheckout(CreateSpaceCheckoutRequest) returns (CheckoutResponse);

  // Лимиты (internal — вызывается другими сервисами)
  rpc GetLimits(GetLimitsRequest) returns (Limits);
  rpc CheckLimit(CheckLimitRequest) returns (CheckLimitResponse);

  // Webhooks (от Paddle/CloudPayments)
  rpc HandlePaddleWebhook(WebhookRequest) returns (Empty);
  rpc HandleCloudPaymentsWebhook(WebhookRequest) returns (Empty);

  // Billing history
  rpc GetBillingHistory(GetBillingHistoryRequest) returns (BillingHistoryList);
}
```

## Модель данных

```
subscriptions
├── id (UUID)
├── account_id (FK)
├── plan (premium)
├── billing_period (monthly | yearly)
├── status (active | past_due | cancelled | expired)
├── provider (paddle | cloudpayments)
├── provider_subscription_id (string)
├── current_period_start
├── current_period_end
├── grace_period_end (nullable)
├── cancelled_at (nullable)
├── created_at
└── updated_at

space_subscriptions
├── id (UUID)
├── space_id (FK)
├── purchaser_account_id (FK)
├── plan (space_pro)
├── billing_period (monthly | yearly)
├── status (active | past_due | cancelled | expired)
├── provider (paddle | cloudpayments)
├── provider_subscription_id (string)
├── current_period_start
├── current_period_end
├── grace_period_end (nullable)
├── created_at
└── updated_at

billing_events
├── id (UUID)
├── subscription_id (FK, nullable)
├── space_subscription_id (FK, nullable)
├── type (payment_success | payment_failed | subscription_created | subscription_cancelled | ...)
├── amount (decimal)
├── currency (string)
├── provider_event_id (string)
├── details (jsonb)
├── created_at
└── INDEX(subscription_id, created_at)
```

## Публикуемые события (→ NATS)

| Событие                        | Данные                                |
|--------------------------------|---------------------------------------|
| `subscription.plan_started`    | account_id, plan, provider            |
| `subscription.plan_renewed`    | account_id, plan                      |
| `subscription.plan_cancelled`  | account_id, plan                      |
| `subscription.plan_expired`    | account_id, plan                      |
| `subscription.payment_success` | account_id, amount, currency          |
| `subscription.payment_failed`  | account_id, reason                    |
| `subscription.space_pro_started`| space_id, purchaser_id               |
| `subscription.space_pro_expired`| space_id                             |
| `subscription.downgrade`       | account_id, frozen_profiles           |

## Зависимости

- **Paddle** — международные платежи (webhook → subscription events)
- **CloudPayments** — СНГ платежи (webhook)
- **User Service** — (через NATS) заморозка excess профилей при downgrade
- **Space Service** — (через NATS) снижение лимитов пространства при expiry
- **File Service** — (через NATS) изменение retention при downgrade
