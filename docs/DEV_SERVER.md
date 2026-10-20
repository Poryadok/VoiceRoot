# Общий dev-стенд

Фактически развёрнут и проверен 16 октября 2026 года. Стенд предназначен для
WIP и интеграционной проверки изменений. Он позволяет обновлять приложение,
сохраняя на staging отдельно выбранную сборку для регрессии и демонстрации.
Наличие staging не является гарантией стабильности установленной версии.

## Адреса и доступ

| Компонент | Адрес |
|---|---|
| Flutter web | https://dev.comrade.click |
| Gateway API | https://api-dev.comrade.click |
| API health | https://api-dev.comrade.click/health |
| Moderation Admin | https://admin-dev.comrade.click |
| Developer Portal | https://developers-dev.comrade.click |
| LiveKit signaling | `wss://livekit-dev.comrade.click` |

Пользователи dev независимы от staging и production. Учётные записи и права
между окружениями не копировались. Для Admin необходимы предусмотренные
приложением роли; доступность страницы сама по себе их не выдаёт.

Операторский SSH: `ssh voice-staging`, пользователь `pmd`, LAN `192.168.0.109`,
хост `pmdebook`, Debian 13, один k3s node. Этот SSH alias сохранил старое имя,
поскольку dev и staging находятся на одном компьютере. Доступ к kubectl
на хосте уже настроен для `pmd`; проверять namespace перед любой командой.

## Топология и границы окружений

```text
DNS-only dev hostname -> 95.31.10.177 -> home router
  -> pmdebook / Traefik HTTPS NodePort30443
  -> Ingress in voice-dev -> corresponding dev Service

LiveKit media -> 95.31.10.177 TCP31081 / UDP31082 -> dev LiveKit
```

Dev использует namespace `voice-dev`, staging — `voice-staging`. Dev не проходит
через production VPS. Production имеет отдельный домашний хост и VPS edge:
см. [PRODUCTION_HOME_SERVER.md](PRODUCTION_HOME_SERVER.md).

У dev собственные PostgreSQL, Redis, MinIO, ClickHouse, NATS JetStream и LiveKit.
Новые DB/S3/JWT/NATS/principal/media credentials выпущены независимо; данные и
приватные ключи staging не копировались. Изоляция namespace не даёт аппаратной
отказоустойчивости: CPU, RAM, диск, сеть, k3s и питание общие со staging.

NetworkPolicy допускает dev peers, cluster DNS, Traefik и необходимый публичный
трафик, ограничивая доступ к другим namespace и частным сетям. Проверено:
dev подключается к своей PostgreSQL, соединение к staging PostgreSQL запрещено.
Отдельное исключение только для LiveKit разрешает UDP1024–65535 к LAN
`192.168.0.0/24`, чтобы работали реальные локальные ICE candidates.

## Ресурсы и диски

На хосте 16 CPU и 31 GiB RAM. ResourceQuota `dev-budget` ограничивает namespace:

| Ресурс | Квота |
|---|---:|
| Pods | 60 |
| CPU requests / limits | 6 / 40 |
| Memory requests / limits | 8 / 20 GiB |
| PVC storage requests | 20 GiB |

При приёмке готовы 27 Deployment и 3 StatefulSet; requests около 960m CPU и
3040 MiB RAM. Это наблюдение, не постоянный резерв свободных ресурсов.

При восстановлении ресурсов 19 октября ClickHouse на dev имел 22 перезапуска,
последний — `OOMKilled` при лимите 1 GiB. Его бюджет увеличен до request
256 MiB / limit 2 GiB в живом StatefulSet и в генераторе dev-манифестов.
После замены Pod проверены готовность, неизменный UID PVC и UUID всех шести
прикладных таблиц. Квота namespace остаётся 20 GiB для memory limits.

Четыре независимых static PV имеют storage class `voice-dev-local` и reclaim
policy `Retain`:

| Данные | Каталог | Заявленный размер |
|---|---|---:|
| PostgreSQL | `/home/voice-dev/data/postgres` | 5 GiB |
| MinIO | `/home/voice-dev/data/minio` | 5 GiB |
| ClickHouse | `/home/voice-dev/data/clickhouse` | 2 GiB |
| NATS JetStream | `/home/voice-dev/data/nats` | 1 GiB |

Размеры local PV служат декларациями для Kubernetes, **не являются файловыми
квотами**. Контролировать фактическое место нужно через `df -h /home`.

Общее контейнерное хранилище хоста также перенесено на `/home`. Постоянные bind
mount сохраняют исходные пути `/var/lib/rancher/k3s`, `/var/lib/docker`,
`/var/lib/containerd`, `/var/lib/kubelet`, `/var/log/pods`,
`/var/lib/voice-nats-preservation`; фактические данные находятся под
`/home/container-data/active/`. Mount dependencies запрещают запуск runtimes
без этих mounts. После переноса и сборок: `/var` 82 GiB свободно, `/home`
195 GiB свободно; значения относятся к 16 октября 2026 года.

Холодные исходные копии сохранены в `/home/container-data/rollback/`, протокол —
в root-only `/home/container-data/migration/`. Это копии на том же диске, не
внешний backup. Полную перезагрузку хоста в рамках переноса не выполняли.
Подробности и проверка TTenrec — [STAGING_SERVER.md](STAGING_SERVER.md).

## Регистрация и тестовая почта

Dev Auth отправляет письма в собственную Resend-совместимую заглушку
`voice-dev-mail:4180`. **Реальные письма на почтовые ящики не доставляются.**
Коды подтверждения доступны оператору через loopback port-forward.

С Windows запустить и оставить открытым отдельный терминал:

```powershell
ssh -L 127.0.0.1:14180:127.0.0.1:14180 voice-staging "kubectl -n voice-dev port-forward svc/voice-dev-mail 14180:4180 --address=127.0.0.1"
```

После регистрации или запроса кода в приложении, во втором терминале:

```powershell
curl.exe --get --data-urlencode "to=your-test-email@example.org" http://127.0.0.1:14180/emails/latest
```

Подставить адрес зарегистрированного dev-пользователя и взять код из письма.
Если обращение выполняется непосредственно на сервере, достаточно отдельного
`kubectl -n voice-dev port-forward svc/voice-dev-mail 14180:4180 --address=127.0.0.1`.
Завершить port-forward через Ctrl+C после проверки. Заглушка не опубликована
через публичный Ingress; её нельзя использовать как надёжный архив писем.

## Обновление WIP и откат образов

Автоматического выката WIP в dev сейчас нет. Использовать актуальный Linux
checkout репозитория, Python3 и kubeconfig нужного кластера. На приёмке
`STAGING_DEPLOY_ENABLED=false`; существующий staging/prod pipeline не был
переназначен на dev. Общая политика — [DEPLOYMENT.md](DEPLOYMENT.md).

1. Собрать нужные образы и опубликовать их в registry либо импортировать в k3s
   containerd этого node. Зафиксировать registry manifest digest каждого образа.
2. Frontend собрать с dev API и WS адресами. Staging frontend не подходит:
   адреса API/RTC запекаются в сборку. Схемные изменения требуют отдельно
   подготовленных dev-миграций из той же версии исходников.
   Для CI-сборки используйте `dev-web:<git SHA>` и artifact
   `dev-web-image-<sha>`: receipt связывает source SHA с immutable
   `dev-web@sha256:...` ref и build args. CI публикует DEV-вариант на master
   для frontend-изменений и изменений publisher workflow. Staging `web` image
   имеет другие адреса API/LiveKit и для DEV не подходит.
3. Предварительно сохранить текущие image pins и выполнить preview:

```sh
kubectl -n voice-dev get deployments -o custom-columns='NAME:.metadata.name,IMAGES:.spec.template.spec.containers[*].image'
python3 scripts/dev/update_images.py --image user=ghcr.io/poryadok/voiceroot/user@sha256:<64hex>
```

4. После проверки результата применить тот же набор (для GIS используйте
`--image gameintegration=...`, после проверки что соответствующий Deployment
создан из актуальных staging workload inputs и dev credentials):

```sh
python3 scripts/dev/update_images.py --image user=ghcr.io/poryadok/voiceroot/user@sha256:<64hex> --apply
kubectl -n voice-dev get deploy,sts,pvc
curl --fail https://api-dev.comrade.click/health
```

Заменить `<64hex>` настоящим digest; для нескольких сервисов повторять `--image`.
Теги, включая полный git SHA, updater не принимает. Preview выполняет server
dry-run. Apply меняет только образы основных app containers и ждёт rollout;
не запускает миграции и не изменяет stores, конфигурацию или sidecars.
Namespace ownership, ConfigMap lock и UID/resourceVersion CAS защищают target
и конкурентные обновления.

При неудаче lock `voice-dev-image-update-lock` сохраняется. Сначала проверить
rollout, Deployment и UID lock, затем решать причину; автоматического force/unlock
и отката нет. Для отката image-only изменения повторно применить прежние
digest через updater. Это не откатывает схему БД или уже записанные данные.

Первые web/admin/portal images собраны из
`e0d875ba612517cd7aac3b184a2deff41cb0fec3` и импортированы локально, без публикации
в GHCR. Для импорта необходим также repository@sha256 alias в containerd;
процедура — [scripts/dev/README.md](../scripts/dev/README.md). До публикации
этих образов нельзя рассчитывать восстановить frontend из registry после
потери локального image cache.

## DNS, HTTPS и сертификаты

Для `dev`, `api-dev`, `admin-dev`, `developers-dev`, `livekit-dev` в зоне
`comrade.click` созданы явные A-записи `95.31.10.177`, **DNS only**, TTL Auto.
Wildcard и staging-записи не менялись. Ранее proxied-маршрут Cloudflare приводил
к зависанию API bodies и сбою signaling; прямая схема прошла приёмку.

Все пять имён покрывает отдельный Let's Encrypt сертификат в Secret
`voice-dev/voice-dev-signaling-tls`. HTTP-01 challenges идут в dev web через
отдельные Ingress; HTTPS — в соответствующие dev Services. LiveKit использует
TCP31081 / UDP31082 для media; доступность страницы не доказывает работу UDP.

Root-only ACME state: `/home/voice-dev/tls/` (0700). Установленный операторский
скрипт: `/home/voice-dev/operator/signal_tls.py`. Daily systemd timer
`voice-dev-signaling-tls-renew.timer` вызывает `renew`; реальный renewal/no-op
проверен. На приёмке сертификат действовал до 14 января 2027 года.

```sh
systemctl status voice-dev-signaling-tls-renew.timer
systemctl show voice-dev-signaling-tls-renew.service -p Result -p ExecMainStatus
```

Issuer проверяет фиксированные dev names, ownership, сертификат, key match и
trust до публикации; существующие ресурсы меняются с CAS. После изменения
исходника TLS operator нужно обновить защищённую root-копию. Initial issue,
expand и renew — разные операции; подробности в [dev tooling](../scripts/dev/README.md).
Private keys, issuance state и сырые диагностические логи не публиковать.

## Диагностика и результаты приёмки

На сервере:

```sh
kubectl -n voice-dev get deploy,sts,pods,pvc
kubectl -n voice-dev get resourcequota,limitrange,networkpolicy,ingress
kubectl -n voice-dev rollout status deployment/voice-gateway --timeout=180s
kubectl -n voice-dev logs deployment/voice-gateway --tail=100
df -h /var /home
```

Логи просматривать локально и проверять на секреты перед передачей. Не выводить
Secret payloads и не прикладывать private issuance/migration logs к PR.

16 октября проверены browser login, отправка/прочтение DM, authenticated DM/WS,
refresh/reconnect/history/read state. Исходный
`TestComposeVoiceCallBidirectionalAudio_live` прошёл за 7.760s через публичные
HTTPS API/WSS с нормальным DNS, без host mapping, TLS bypass и отключения HTTP/2
у тестового клиента. Сервер LiveKit остался версии1.8.4; экспериментальное
обновление откатили. Dev Voice сохраняет HTTP/1.1 compatibility setting для
HTTPS Gateway JWKS; TLS verification включена.

Проверки tooling:

```sh
python3 -m unittest discover -s scripts/dev -p '*_test.py'
python3 -m unittest discover -s scripts/ops -p 'container_storage_home_test.py'
```

37 dev и 4 storage tests прошли; проводились независимые проверки tooling.
Это приёмка стенда, не подтверждение готовности всех продуктовых функций.
История измерений: [ExecPlan](../.agent/execplans/dev-environment.md).

## Execution and shared probes

An executable-launch/firewall restriction is an environment limitation, not a
product assertion failure. Preserve the exact blocked command and use an
authorized executable environment or CI for missing runtime evidence; repeated
unchanged local attempts add no proof. On Windows, resolve which `bash` would run:
an unqualified command may select the WSL shim, including for `bash -n`. An active
WSL/runtime prohibition applies to that invocation too; route it to the authorized
Linux lane instead. These conditions do not prohibit unrelated static work.

Reserve shared DEV probes during actual mutation/restart or a stable-version
proof window. Release the reservation during source fixes only after checking
that the relevant workloads remain ready and no owned operation still needs it.
Keep source and fast tests with their author in an owned pinned environment;
retain independent risk review rather than relaying every fixture between agents.
Normal Kubernetes access does not grant host sudo, registry write or containerd
socket authority. Current ownership and next actions belong in TT.

## Восстановление и ограничения

Fresh bootstrap не является командой обновления существующего стенда. Он
отказывается принимать существующий namespace и непустые каталоги данных.
Не запускать `start`, storage `capture`/`migrate` или повторную выдачу ключей
как способ ремонта работающего окружения. Operator recovery требует отдельного
разбора текущего состояния и подходящей операции из runbook.

Private bootstrap state расположен в `/home/voice-dev/bootstrap`, issuance —
в `/home/voice-dev/issuance/generation1`; они доступны только root. Схема
создания окружения и порядок migrations/NATS bootstrap описаны в
[scripts/dev/README.md](../scripts/dev/README.md).

Recovery issuer для DEV media JWKS — отдельный операторский Docker named volume
`voice-dev-media-pki`, работающий с выделенным UID 10001. Он не использует
root-only issuance archive выше и не хранит CA key в Kubernetes. Не удалять
volume при обычной ротации; потеря volume означает отдельную reviewed выдачу
нового DEV trust root с additive overlap. Текущая tooling версия не экспортирует
plaintext backup; encrypted offline backup должен быть организован до
долгосрочной эксплуатации. Инструкции по выпуску и DEV-only cutover находятся
в [scripts/dev/README.md](../scripts/dev/README.md).

Issuer ограничен тремя DEV server identities: `voice-voice`,
`voice-gameintegration` и `voice-matchmaking.voice-dev.svc`. Для Matchmaking он
создаёт отдельный публичный CA Secret `voice-matchmaking-principal-jwks-ca` и
TLS Secret `voice-matchmaking-principal-jwks-tls`; Chat/Voice читают только CA
Secret, а Matchmaking монтирует TLS Secret read-only. Эти имена и сертификат
не используются для Compose alias или namespace `voice-staging`.

MatchSquad mTLS использует отдельный DEV trust domain, не `voice-media-ca`:
`voice-matchmaking-client-tls` выпускается с `clientAuth` и SAN
`voice-matchmaking.voice-dev.svc`, `voice-chat-match-squad-server-tls` — с
`serverAuth` и SAN `voice-chat`, а `voice-voice-match-squad-server-tls` — с
`serverAuth` и SAN `voice-voice` для портов 9094 и 9095. Публичный корень
хранится в ConfigMap `voice-match-squad-ca`; закрытый ключ CA остаётся только в
отдельном приватном томе `voice-dev-match-squad-pki`. Secrets подписи principal
и replay credentials создаются отдельно и неизменяемо тем же DEV-оператором.

DEV overlay переключает четыре CA-файла: исходящую проверку Matchmaking для
Chat/Voice, входящую проверку клиента Matchmaking в Chat 9092 и Voice 9094, и
исходящую проверку Voice 9095 в Gateway. Проверка клиентского сертификата
Voice 9095 остаётся на `voice-media-ca`; JWKS-корни и существующие media
идентичности не меняются. Порядок выпуска и точные overlay patches описаны в
`scripts/dev/README.md` и
`deploy/dev/match-squad-trust-overlay/README.md`. Этот отдельный оператор не
продолжает и не изменяет journal/volume уже завершённого media PKI recovery.

Для DEV CompleteMatch предусмотрен ещё один отдельный issuer: Matchmaking
server leaf подписывается retained MatchSquad CA, а Gateway client leaf
получает собственный новый CA. Он создаёт только три фиксированных объекта
`voice-matchmaking-complete-server-tls`,
`voice-matchmaking-complete-client-ca` и
`voice-gateway-matchmaking-complete-client-tls`; principal/JWKS credentials,
существующий PKI journal и workload trust mounts остаются вне этой операции.
Provisioning и переключение consumer overlay — разные reviewed шаги. До
отдельного grant на фактический выпуск не запускать `prepare`/`apply`/`resume`
и не создавать эти объекты вручную. Детали guarded source/operator flow:
[scripts/dev/README.md](../scripts/dev/README.md#dev-completematch-transport-identities).
Оператор вызывает `kubectl` только с явно выбранными `/home/pmd/.kube/config` и
контекстом `default`;
он проверяет владельца и файловые права без чтения содержимого, не наследует
ambient `KUBECONFIG` и не исправляет права kubeconfig автоматически.

Пересоздание pod не равно удалению PVC: stateful данные dev находятся на PV;
`Retain` сохраняет backing data при удалении claim, но не восстанавливает
namespace, привязки и конфигурацию автоматически. Redis/mail fixture и любые
неприкреплённые к PV файлы не следует считать долговременным хранилищем.

TTenrec остаётся отдельным namespace `chinchilla`. Его принудительный CRI restart
прошёл с совпавшими хешами SQLite и integrity_check; исходный PVC сохранён.
У его PV политика **Delete**, поэтому удалять PVC/namespace нельзя как способ
перезапуска. Позднейшая работа трекера может менять хеши относительно холодного
baseline; текущая целостность проверяется независимо от исторического равенства.

Внешний backup для dev, полный host reboot test, HA и activation реальных
billing/push/provider integrations в эту установку не входили. Federation
остаётся вне разрешённой области. Общая инфраструктура и заполнение `/home`
могут повлиять сразу на dev, staging и соседние проекты.
