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

**Доступ к API из GitHub Actions:** секрет `STAGING_KUBECONFIG` в репозитории должен указывать на endpoint, **доступный бегунку** GitHub. Если API Kubernetes доступен только из LAN, облачный CI до него не дойдёт — нужны self-hosted runner, VPN/Tailscale до раннера, либо контролируемый публичный endpoint (TLS, ограничение по IP и т.д.).

---

## Traefik (ingress)

| Параметр | Значение |
|----------|----------|
| Namespace | `traefik` |
| Helm | релиз: `helm list -n traefik` |
| NodePort HTTP | `30080` |
| NodePort HTTPS | `30443` |

---

## Другой сайт на том же кластере (справочно, не Voice)

| Параметр | Значение |
|----------|----------|
| Домен | `tastytest.online` |
| DNS | Cloudflare → `95.31.10.177`, proxy (оранжевая туча) включён |
| TLS Cloudflare ↔ origin | **Flexible SSL** |
| Ingress | `tastytest-http`, `tastytest-https` |
| TLS в кластере (self-signed) | secret `tastytest-tls` (namespace приложения — уточнять: `kubectl get ingress -A`) |
| Deployment | 2 реплики; имя: `kubectl get deploy -A` и фильтр по `tastytest` |

Zone ID и API token Cloudflare — только в менеджере паролей.

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
