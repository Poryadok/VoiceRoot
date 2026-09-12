# Домашний production-сервер

Этот документ фиксирует фактически развёрнутый 12 сентября 2026 года тестовый
production-контур Voice. Значения секретов, приватные ключи и домашний WAN IP в
репозиторий не записываются.

Контур пригоден для первых проверок и ограниченного запуска. Он не означает
выполнение всех production-gates из [PLAN.md](PLAN.md): домен, полноценный
мониторинг, продуктовые секреты внешних провайдеров и release evidence ещё
нужно подготовить отдельно.

## Топология

```text
HTTPS client
  -> VPS 5.252.225.0:443
  -> home WAN:22443/TCP
  -> 192.168.0.234:30443
  -> k3s Traefik -> Voice Ingress

LiveKit RTC/TCP client
  -> VPS 5.252.225.0:30981/TCP
  -> home WAN:22981/TCP
  -> 192.168.0.234:30981/TCP

LiveKit RTC/UDP client
  -> VPS 5.252.225.0:30982/UDP
  -> home WAN:22982/UDP
  -> 192.168.0.234:30982/UDP
```

Staging остаётся на `192.168.0.109` и использует домашний внешний `443`. Его
маршруты не проходят через production-хост.

Правила Archer C80 для production:

| Назначение | Устройство | Внешний порт | Внутренний порт | Протокол |
|---|---|---:|---:|---|
| HTTPS origin | `192.168.0.234` | `22443` | `30443` | TCP |
| LiveKit RTC fallback | `192.168.0.234` | `22981` | `30981` | TCP |
| LiveKit RTC media | `192.168.0.234` | `22982` | `30982` | UDP |

## Production-хост

| Параметр | Значение |
|---|---|
| LAN | `192.168.0.234` |
| ОС | Debian 13 (Trixie), после обновления с Debian 12.9 |
| Kernel при приёмке | `6.12.107+deb13-amd64` |
| Kubernetes | k3s `v1.35.8+k3s1`, один node |
| Runtime | встроенный containerd k3s |
| Namespace | `voice-prod` |
| Проверенный image tag | `533964408c3c085f56827ed65dde8a8a2f2d132d` |

Системный NVMe остаётся корневым диском. Отдельный NVMe объёмом 1 ТБ
отформатирован в ext4 с label `voice-prod-data` и смонтирован в
`/var/lib/rancher`. Запись в `/etc/fstab` использует UUID, поэтому порядок имён
`nvme0n1`/`nvme1n1` после перезагрузки не важен.

Traefik работает как NodePort без ServiceLB:

- HTTP: `30080/TCP`, только для локальной диагностики;
- HTTPS: `30443/TCP`, origin для VPS;
- LiveKit: `30981/TCP` и `30982/UDP`.

В `/etc/nftables.d/voice-origin-firewall.nft` origin и LiveKit NodePort разрешены
только от VPS `5.252.225.0` и из `192.168.0.0/24`. Остальной входящий трафик на
эти порты отбрасывается до правил k3s. `nftables.service` запускается до k3s через
`/etc/systemd/system/k3s.service.d/10-nftables.conf`.

## VPS edge

VPS сохраняет существующий Docker-контейнер `3x-ui` и Xray listener на
`8443/TCP`. Панель x-ui слушает `2053`; её публикация firewall не менялась.
Старый сайт `tastytest.online` отключён. Пакеты kubeadm/kubelet/kubectl и CNI
удалены; старые runtime-правила окончательно исчезнут после следующей плановой
перезагрузки VPS.

Основные файлы:

| Путь | Назначение |
|---|---|
| `/etc/nginx/sites-available/voice-prod.conf` | TLS termination и HTTPS proxy на домашний origin |
| `/etc/nginx/stream-conf.d/voice-livekit.conf` | TCP/UDP relay LiveKit |
| `/etc/letsencrypt/live/5.252.225.0/` | доверенный сертификат на IP VPS |
| `/opt/voice-certbot/` | отдельное окружение современного Certbot |
| `voice-certbot-renew.timer` | проверка продления сертификата каждые четыре часа |

Перед каждым reload:

```bash
sudo nginx -t
sudo systemctl reload nginx
sudo systemctl is-active nginx docker
sudo docker inspect -f '{{.State.Status}}' 3x-ui
sudo ss -lntup | grep -E ':(443|8443|30981|30982)\b'
```

Если домашний белый IP изменился, обновить upstream во всех `proxy_pass`/`server`
в двух nginx-файлах выше, выполнить `nginx -t` и только затем reload. Автоматически
определять WAN с production-хоста нельзя: его исходящий трафик может идти через
VPN и возвращать другой адрес.

## TLS и LiveKit

До покупки домена внешний smoke доступен по `https://5.252.225.0/`. IP-сертификат
Let's Encrypt короткоживущий, поэтому таймер renewal является обязательной
частью контура.

Origin использует отдельный self-signed сертификат из `/etc/voice/tls/`; ключ
хранится только на production-хосте и в зашифрованном локальном backup. VPS
проверяет внешний сертификат для клиентов и устанавливает отдельное TLS-соединение
до origin.

LiveKit config создаётся как Kubernetes Secret `voice-livekit-config`, а не как
ConfigMap. `rtc.use_external_ip=false`, `rtc.node_ip=5.252.225.0`; поэтому SFU
рекламирует адрес VPS, даже если исходящий трафик домашнего сервера идёт через
VPN. API key/secret LiveKit были ротированы после установки Secret.

После покупки домена нужны отдельные DNS-записи и HTTPS virtual hosts для web,
API, LiveKit signaling, admin и developer portal. До этого IP endpoint проверяет
web build и edge, но не является окончательной публичной конфигурацией клиента.

## Деплой и холодный запуск

Production использует immutable image tag. Полный ручной apply требует как
минимум:

```bash
export VOICE_IMAGE_TAG=<green-git-sha>
export VOICE_LIVEKIT_NODE_IP=5.252.225.0
export VOICE_K8S_NAMESPACE=voice-prod
bash scripts/prod/render-and-apply-prod.sh
```

Production secrets создаются независимо от staging. Если `voice-app-secrets`
уже существует, apply сохраняет его. Сгенерированный operator token хранится
root-only в `/etc/voice/operator.env`.

Некоторые сервисы имеют зависимый порядок холодного старта. После включения
питания `voice-prod-ordered-start.service` ждёт Postgres, Redis, NATS и
ClickHouse, затем выполняет `scripts/staging/rollout-app-tier.sh` с namespace
`voice-prod` и проверяет Available у всех Deployment. Этот сценарий проверен
полной перезагрузкой хоста.

Диагностика:

```bash
sudo systemctl status k3s voice-prod-ordered-start.service
sudo kubectl get nodes
sudo kubectl get deploy,statefulset,pods,pvc -n voice-prod
curl -k --resolve app.voice.example.com:30443:192.168.0.234 \
  https://app.voice.example.com:30443/health
curl https://5.252.225.0/health
```

## Резервные копии

`voice-prod-backup.timer` запускается ежедневно около `04:15`; применяется
случайная задержка до 20 минут.

Схема разделена на два уровня:

1. Полный зашифрованный restic snapshot хранится на системном SSD в
   `/var/backups/voice-prod/restic`. В него входят PVC data, состояние k3s,
   `/etc/voice`, releases и portable dumps.
2. Компактный зашифрованный snapshot с PostgreSQL dump, Kubernetes resources и
   operator config отправляется на VPS в изолированный SFTP-only аккаунт
   `voicebackup`. Репозиторий доступен production-хосту как
   `sftp:voice-backup-vps:/data/restic`.

Пароли restic лежат в root-only файлах `/etc/voice/restic-local-password` и
`/etc/voice/restic-vps-password`. Backup transport использует отдельный ключ;
учётная запись VPS не имеет shell, port forwarding или TTY.

Retention:

- local: 7 daily, 4 weekly, 6 monthly;
- VPS critical: 7 daily, 4 weekly, 3 monthly;
- один файл compact dump больше 8 ГБ блокирует off-host upload, чтобы не заполнить
  маленький VPS.

Для согласованного полного снимка backup кратковременно останавливает writers,
снимает данные и поднимает приложения в проверенном порядке. Это означает
короткое ночное окно недоступности. При появлении платящих пользователей схему
следует заменить на service-native online backups и вместительное off-host
хранилище.

Проверка архивов:

```bash
sudo systemctl status voice-prod-backup.timer
sudo env RESTIC_PASSWORD_FILE=/etc/voice/restic-local-password \
  restic snapshots --repo /var/backups/voice-prod/restic
sudo env RESTIC_PASSWORD_FILE=/etc/voice/restic-vps-password \
  restic snapshots --repo sftp:voice-backup-vps:/data/restic
```

Restore всегда начинать в изолированный каталог или namespace, никогда поверх
живого production. При приёмке оба репозитория прошли `restic check`, snapshots
были восстановлены в `/var/tmp`, PostgreSQL dumps проверены `zstd -t` и совпали
по SHA-256.

## Доступ и откат

Обычный оператор входит как `pm` по LAN и использует `sudo`. Временные ключи с
comments `voice-agents-prod` и `voice-agents-vps` удалены из серверных
`authorized_keys`; локальные private/public key files владельца не удалялись.
Чтобы временно вернуть агенту доступ, добавить нужный `.pub` только на время
работ и снова удалить строку после приёмки. Для production предпочтительно
авторизовать `pm`, а root-доступ выдавать только для ограниченного bootstrap.

Перед rollback сначала определить предыдущий каталог в `/opt/voice/releases`,
затем атомарно переключить `/opt/voice/current` и применить соответствующий
immutable image tag. Не смешивать manifests одного commit с images другого.
Миграции откатывать только по правилам [OPERATIONS.md](OPERATIONS.md).

## Известные ограничения

- один домашний хост, один провайдер, один роутер и один VPS не дают HA;
- power loss защищён автозапуском, но не резервным питанием;
- полный backup на системном SSD не спасает от потери всего компьютера;
- VPS хранит только компактную disaster-recovery часть;
- production observability profile ещё привязан к staging namespace и не
  включён в этом контуре;
- реальные домены и production credentials R2/FCM/APNs/Paddle отсутствуют;
- 100 Мбит/с домашнего uplink достаточно для теста, но не является целевой
  инфраструктурой после появления устойчивой нагрузки.
