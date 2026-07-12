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
| Типичное значение `server:` в k3s.yaml | `https://0.0.0.0:6443` или `127.0.0.1` — **в секрет `STAGING_KUBECONFIG` подставлять** публичный endpoint, напр. **`https://95.31.10.177:6443`** |
| Ответ API по публичному IP с ноды | `curl -sk https://95.31.10.177:6443/` → **401** без учётных данных (ожидаемо) |

**Доступ к API из GitHub Actions:** облачный runner **не достигает** публичного `6443` на этом хосте (таймаут). Workflow **Staging deploy** поднимает **SSH-туннель** (`scripts/staging/configure-kubectl-ci.sh`): в Environment **staging** нужен секрет **`STAGING_SSH_PRIVATE_KEY`** (ключ для `pmd@95.31.10.177`). `STAGING_KUBECONFIG` — с `server: https://127.0.0.1:6443` (как в `k3s.yaml`). Опционально Variables: `STAGING_SSH_HOST`, `STAGING_SSH_USER`, `STAGING_DEPLOY_VIA_SSH=true`. Альтернатива: self-hosted runner на ноде или открытый API с ограничением по IP.

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

- Kubernetes namespace: **`voice-staging`** (как в репозитории).
- Образ **gateway** из GHCR и теги — [DEPLOYMENT.md](DEPLOYMENT.md).
- Публичные **FQDN** на этом стенде — [`deploy/staging/domains.defaults`](../deploy/staging/domains.defaults) (`voice.comrade.click`, `developers.comrade.click`). В GitHub: переменные **`VOICE_GATEWAY_INGRESS_HOST`** / **`VOICE_DEVELOPER_PORTAL_INGRESS_HOST`**. DNS: в Cloudflare для зоны **comrade.click** — записи **A** для поддоменов **`voice`** и **`developers`** на внешний IP этой ноды (см. таблицу «Хост и SSH» выше). TLS: **Cloudflare Flexible SSL** — HTTPS до пользователя, HTTP до origin; детали — [DEPLOYMENT.md](DEPLOYMENT.md).
- Внешняя маршрутизация: **Traefik** (`ingressClassName: traefik`), манифест [`deploy/gateway/ingress.yaml`](../deploy/gateway/ingress.yaml), бэкенд — `Service` `voice-gateway`. Имя TLS Secret на origin — переменная **`VOICE_GATEWAY_TLS_SECRET`** (если используете HTTPS-Ingress); при только Flexible часто достаточно HTTP-маршрута. На проде — тот же шаблон, другой FQDN и переменные.
