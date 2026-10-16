# Staging-сервер (операторские заметки)

Инвентарь **текущего** staging-хоста с k3s. Общая политика выката и CI — [DEPLOYMENT.md](DEPLOYMENT.md).

**Не хранить в этом файле и в git:** пароли, приватные SSH-ключи, содержимое kubeconfig, Cloudflare API token, любые секреты.

---

## Назначение

Debian + **k3s** на одной ноде: интеграционный стенд. Манифесты Voice для staging — [`deploy/staging/`](../deploy/staging/); деплой из GitHub Actions — см. [DEPLOYMENT.md](DEPLOYMENT.md) (`Staging deploy`, `STAGING_KUBECONFIG`, `STAGING_DEPLOY_ENABLED`).

---

## Хост и SSH

| Параметр | Значение |
|----------|----------|
| ОС | Debian |
| Имя ноды (hostname) | `pmdebook` |
| Пользователь SSH | `pmd` |
| Внутренний IP | `192.168.0.109` |
| Внешний IP | `95.31.10.177` |

**Доступ с Windows:** в `~/.ssh/config` заведён алиас **`voice-staging`** (→ `HostName`, `User pmd`, **`IdentityFile`** на ключ). Подключение: `ssh voice-staging`.

---

## k3s

| Параметр | Значение |
|----------|----------|
| Статус | Установлен, рабочий |
| Kubeconfig на сервере | `/etc/rancher/k3s/k3s.yaml` |
| kubectl | Обычно `sudo kubectl` |
| Прослушивание kube-apiserver | `*:6443` (все интерфейсы; без sudo: `ss -lntp`, затем отбор `6443`) |
| Типичное значение `server:` в k3s.yaml | `https://127.0.0.1:6443` — **в секрет `STAGING_KUBECONFIG` всегда** `https://127.0.0.1:6443` (self-hosted runner или SSH-туннель). Публичный `https://95.31.10.177:6443` — только для kubectl с вашей машины в LAN, **не** для github-hosted CI. |
| Ответ API по публичному IP с ноды | `curl -sk https://95.31.10.177:6443/` → **401** без учётных данных (ожидаемо) |

**Доступ к API из GitHub Actions:** для домашнего стенда **рекомендуется self-hosted runner** на этой ноде — исходящий polling GitHub, `kubectl` к `127.0.0.1:6443`, **без** открытия SSH/6443 в интернет. Установка: [`scripts/staging/setup-github-runner.sh`](../scripts/staging/setup-github-runner.sh); в Environment **staging** Variable **`STAGING_RUNNER_LABELS`** = `["self-hosted","Linux","X64","voice-staging"]`, **`STAGING_DEPLOY_VIA_SSH`** = `false`, **`STAGING_KUBECONFIG`** с `server: https://127.0.0.1:6443`.

Альтернатива (хуже для home LAN): **SSH-туннель** с github-hosted runner ([`configure-kubectl-ci.sh`](../scripts/staging/configure-kubectl-ci.sh)) — нужен **входящий SSH** с интернета (или allowlist IP GitHub Actions, не `0.0.0.0/0`). Секрет **`STAGING_SSH_PRIVATE_KEY`**. UFW сейчас: `22/tcp ALLOW 192.168.0.0/24` — с облачного runner SSH **не пройдёт** (это нормально).

**Не открывайте** kube-apiserver (6443) и SSH (22) на весь интернет без жёсткого allowlist — для CI достаточно self-hosted runner.

---

## Traefik (ingress)

| Параметр | Значение |
|----------|----------|
| Namespace | `traefik` |
| Helm | релиз: `helm list -n traefik` |
| NodePort HTTP | `30080` |
| NodePort HTTPS | `30443` |

---

## Несколько проектов на одной ноде / кластере

На одном k3s могут жить **несколько независимых приложений**. Они не «мешают» друг другу, если:

- у каждого свой **namespace** (у Voice по умолчанию `voice-staging` / в проде — свой);
- у каждого внешнего сервиса свой **FQDN** в DNS и свои **Ingress** с разным `spec.rules[].host` (один и тот же Traefik маршрутизирует по имени хоста);
- TLS: свой **Secret** в namespace приложения или терминация TLS на внешнем прокси — см. [DEPLOYMENT.md](DEPLOYMENT.md) (раздел про маршрутизацию Gateway и сценарии «чистый кластер / общий / снятие»).

**Чистый кластер только под Voice:** достаточно манифестов репозитория и DNS на IP ноды / балансера перед Traefik.

**Уже есть чужие сервисы:** перед выкатом посмотреть занятые хосты и порты: `kubectl get ingress -A -o wide`, `kubectl get svc -n traefik` (NodePort ingress-контроллера). Voice добавляет **отдельные** объекты `Ingress` с метками `app.kubernetes.io/name: voice-gateway` — не пересекаются с чужими, пока FQDN уникален.

**Переезд Voice на другой сервер:** новый kubeconfig в секрет `STAGING_KUBECONFIG`, переменные с FQDN/namespace, деплой по [DEPLOYMENT.md](DEPLOYMENT.md). **Снятие Voice с ноды:** удалить релиз приложения (Ingress, Deployment, Service, при необходимости namespace `voice-staging`), образы в registry при желании почистить отдельно; чужие namespace и Ingress не трогать.

---

## Команды на сервере (диагностика)

```bash
sudo kubectl get ingress -A -o wide
sudo kubectl get pods -A
sudo kubectl get svc -A
sudo kubectl get svc -n traefik traefik -o yaml
sudo kubectl get secret -A | grep tls
helm get values traefik -n traefik
sudo kubectl logs -n traefik deployment/traefik --tail=50
```

**Kubeconfig для пользователя без sudo на каждый вызов:**

```bash
mkdir -p ~/.kube
sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
sudo chown "$(id -u):$(id -g)" ~/.kube/config
chmod 600 ~/.kube/config
```

При необходимости поправить в копии `server:` URL (часто с `127.0.0.1` на IP/имя ноды), если kubectl с другой машины.

---

## Voice на этом стенде

### Контейнерное хранилище и dev (2026-10-16)

Раздел `/var` имеет 92 GiB, `/home` — 342 GiB; остальные разделы физического
NVMe заняты в том числе Windows. Контейнерное хранилище перенесено холодным
копированием на `/home/container-data/active/`. Старые пути сохранены bind mount:
`/var/lib/rancher/k3s`, `/var/lib/docker`, `/var/lib/containerd`,
`/var/lib/kubelet`, `/var/log/pods`, `/var/lib/voice-nats-preservation`.
Таким образом существующие PV и bind mounts продолжают указывать на те же пути.
Данные dev дополнительно находятся в `/home/voice-dev/data/`.

Постоянные systemd mount units требуют `/home` и `/var`; drop-in
`container-storage-home.conf` требует все шесть mounts перед запуском runtimes.
При недоступном `/home` runtime должен остановиться, а не создать пустой каталог.
Проверка после переноса: `/var` 5.1 GiB занято / 82 GiB свободно,
`/home` около 127 GiB занято / 199 GiB свободно, включая rollback-копии.
Полная перезагрузка хоста в рамках переноса не выполнялась.

Одноразовый host-specific executor: [container-storage-home.py](../scripts/ops/container-storage-home.py).
Протокол, согласованные SQLite backups и инвентарь — root-only
`/home/container-data/migration/`; холодные оригиналы сохранены в
`/home/container-data/rollback/`. Это копии на том же диске, не внешний backup.
Не запускать `capture`/`migrate` повторно на работающем перенесённом стенде.

**TTenrec**: namespace `chinchilla`, Deployment `tracker`, PVC `tracker-data`,
mount `/data`; SQLite `tracker.db` и WAL лежат в PVC. Все исходные 17 PV/PVC
сохранили UID. После принудительной остановки CRI-контейнера tracker восстановился,
SQLite integrity_check и хеши содержимого обеих таблиц совпали с холодным backup.
Удаление pod/контейнера сохраняет данные; удаление PVC/namespace опасно:
у существующего PV reclaim policy `Delete`. Защита от потери диска требует
отдельного внешнего резервного копирования.

Dev: [dev.comrade.click](https://dev.comrade.click),
[API](https://api-dev.comrade.click/health),
[admin](https://admin-dev.comrade.click),
[developer portal](https://developers-dev.comrade.click).
Порядок обновления и ограничения — [dev runbook](../scripts/dev/README.md).

- Kubernetes namespace: **`voice-staging`** (как в репозитории).
- Образ **gateway** из GHCR и теги — [DEPLOYMENT.md](DEPLOYMENT.md).
- Публичные **FQDN** на этом стенде — [`deploy/staging/domains.defaults`](../deploy/staging/domains.defaults) (`voice`, `developers`, `app`, `admin`, `livekit` на `comrade.click`). В GitHub: Variables **`VOICE_*_INGRESS_HOST`**. DNS: в Cloudflare для зоны **comrade.click** — записи **A** на внешний IP ноды; **`livekit`** — предпочтительно **DNS only** (grey cloud) для WebRTC UDP.
- Внешняя маршрутизация: **Traefik** (`ingressClassName: traefik`), манифест [`deploy/gateway/ingress.yaml`](../deploy/gateway/ingress.yaml), бэкенд — `Service` `voice-gateway`. Имя TLS Secret на origin — переменная **`VOICE_GATEWAY_TLS_SECRET`** (если используете HTTPS-Ingress); при только Flexible часто достаточно HTTP-маршрута. На проде — тот же шаблон, другой FQDN и переменные.
