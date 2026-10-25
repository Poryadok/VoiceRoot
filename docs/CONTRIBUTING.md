# Участие в разработке (Git и процесс)

For older branches and release integration, see [Release evidence and recovery
lessons](RELEASE_EVIDENCE.md). Review actual scope and landed equivalents; retain
source-bound acceptance when integrating and never overwrite shipped migrations.

Репозиторий на **GitHub**. Хостинг кода и **CI/CD** — GitHub Actions ([MICROSERVICES.md](MICROSERVICES.md)). Сколько репозиториев заводить и как называть — [REPOSITORIES.md](REPOSITORIES.md).

---

## Ветки

| Ветка              | Назначение                                                                                                            |
|--------------------|-----------------------------------------------------------------------------------------------------------------------|
| `master`           | Всегда деплойable-кандидат: проходит CI, без известных блокеров. Защищённая ветка (после настройки): прямых push нет. |
| `feature/<кратко>` | Новая функциональность.                                                                                               |
| `fix/<кратко>`     | Исправления багов.                                                                                                    |

Допустимы `chore/`, `docs/` — по тому же принципу короткого суффикса. Релизные ветки `release/x.y` вводятся при необходимости отдельного стабилизационного окна; до появления такого процесса релизы идут с `master` + теги (см. [DEPLOYMENT.md](DEPLOYMENT.md)).

---

## Pull Request

1. Один PR — по возможности одна смысловая задача; крупные изменения разбиваются на обозримые части.
2. Описание PR: **что** сделано и **зачем**; ссылка на issue/задачу, если есть.
3. Перед запросом review: локальные проверки из [TESTING.md](TESTING.md) для затронутых пакетов; для новой нетривиальной логики ожидается цикл из раздела «Порядок разработки (TDD)» там же.
   Быстрые CI-script checks: `make -j4 ci-script-tests-pr` и отдельный
   `scripts/ci/current-diff-contract.py --base <comparison-commit>` на committed
   checkout. `make ci-script-tests` сохраняет полный исторический replay.
   Required `ci-gate` требует каждый выбранный shard; исторический аудит
   обязателен на master перед release/staging и на PR исторического валидатора
   или release-ветки. Методика замеров и исключения —
   [CI gate policy](../.github/ci/gate-policy.md).
4. **Изменения в `protos/`** — перед review: `make buf-ci` (lint + проверка формата); при правках публичного API относительно `master` — `make buf-breaking` (см. [TESTING.md](TESTING.md)). После регенерации Go stubs: `make buf-generate-all` (или `make buf-generate` + `make sync-pb-from-gen`) и `make buf-generate-dart` / `make buf-dart-check` — см. [REPOSITORIES.md](REPOSITORIES.md). Новые поля и RPC — по buf STANDARD и [DATA_MODEL.md](DATA_MODEL.md) (`account_id` / `profile_id`, время, пары `*Request` / `*Response`).
5. Merge в `master` после approve (для соло-разработки допускается self-merge после зелёного CI; при команде из двух и более человек — **минимум один чужой approve** на PR в `master`).
6. В `master` — **merge commit** (`gh pr merge --merge`): все коммиты ветки остаются в истории. **Squash merge не использовать** — схлопывание коммитов скрывает промежуточные шаги (TDD, ревью-итерации).

---

## Коммиты

- Сообщения — на английском, **единый язык внутри PR**; первая строка — краткая суть (до ~72 символов).
- **Conventional Commits** не обязательны. Введение формата — правка этого файла и CI.
- После создания коммита агент пушит его сразу, если пользователь явно не попросил оставить коммит локальным.

---

## История Git и хуки

**Не переписывать историю** — ни локально, ни на remote (любая ветка):

- запрещены: force-push (`--force`, `-f`, `--force-with-lease`, `+refspec`), rebase (в т.ч. `git pull --rebase`), `commit --amend`, `reset --hard`, `filter-branch` / `filter-repo`;
- запрещён обход хуков: `git commit --no-verify` / `-n`, `git push --no-verify` / `-n`.

Если pre-commit / pre-push / Cursor hook упал — **исправить файлы и сделать новый коммит**, не обходить хук.

Основной checkout держать на актуальном `master`: перед работой и после merge PR выполнять `git fetch origin` и `git merge origin/master`. **Rebase не использовать.** Не оставлять основной `master` в состоянии `ahead/behind`, если нет явного решения держать локальную работу незапушенной.

Установка локальных git-хуков и Cursor `beforeShellExecution` (блок history rewrite):

```powershell
.\scripts\git\install-hooks.ps1
```

Полный чеклист подготовки машины: [`docs/DEV_SETUP.md`](DEV_SETUP.md). Подробности хуков: [`scripts/git/README.md`](../scripts/git/README.md). Для агентов: `.cursor/rules/git-safety.mdc`.

---

## Секреты и конфигурация

- Секреты и ключи **не** коммитить. Локально — `.env` / секреты вне репозитория; в CI — GitHub **Secrets**; в кластере — Kubernetes Secrets (или внешний секрет-стор, когда появится).

---

## Временные файлы (`tmp/`)

Корневая папка `tmp/` — для локальных и агентских артефактов, которые не входят в продукт и не коммитятся (см. `.gitignore`).

Правила для агентов (и людей, если нужны те же временные файлы):

1. Создавать файлы только под `tmp/<session>/`, где `<session>` — краткое имя на английском по теме текущей сессии/задачи (например `tmp/penpot-audit/`, `tmp/buf-smoke/`).
2. Не класть временные файлы в корень репозитория, в `docs/`, `src/` и другие рабочие деревья.
3. После полного выполнения задачи, если содержимое сессии больше не нужно — удалить `tmp/<session>/` целиком.

Оставлять папку сессии можно только если артефакты ещё нужны для продолжения работы (явный follow-up) или пользователь попросил сохранить.

---

## Документация

- Изменения в контрактах API, SLO или поведении фич сопровождаются правками в `docs/` в том же PR, если иначе документация становится ложной.

## Managed worktrees and process custody

Preserve a dirty main checkout and inspect `git status --short` before choosing
write scope. Product writers use the repository's managed worktrees: acquire an
actual lease, record its absolute physical checkout path, lease identity,
branch/base and writer, then run edits and checks in that path. Before integrating,
inspect the exact diff and base; a completion message is not the change itself.

Before returning a lease or cleaning a checkout, account for its live processes,
persistent test containers and needed ignored artifacts. A lease ID identifies
the reservation; it does not establish which checkout a process uses. A shared
SDK path, generic basename, timestamp or dead parent is insufficient attribution.
Use the full physical checkout or a known owned command handle. If the manager
would terminate an unattributed process, retain the lease and report custody.

Tool session handles are not OS PIDs. A handle unavailable to the next agent does
not establish that execution stopped or that its container was removed. Preserve
the original owner, handle and container/cache identities; obtain the terminal
result from the owning session when possible before restarting anything. These
rules do not authorize killing unknown processes or reclaiming another lease.

Repository-owned Codex skill sources and the normal local installation route are
documented in [the migration map](../.agent/codex/README.md#portable-codex-assets).
Do not independently edit installed copies: review the source change first and
preserve local differences before synchronization. The full installer replaces
all matching local skill directories, so do not run it to update one reference
without checking its actual scope.

---

## Связанные документы

- [DEV_SETUP.md](DEV_SETUP.md) — подготовка машины (toolchain, git hooks, compose)
- [REPOSITORIES.md](REPOSITORIES.md) — монорепо, protos, имена репозиториев
- [TESTING.md](TESTING.md) — что гонять перед PR и в CI
- [DEPLOYMENT.md](DEPLOYMENT.md) — окружения и выкат
- [PLAN.md](PLAN.md) — соответствие кода плану
- [TODO.md](TODO.md) — известные пробелы в спецификациях
- [design/penpot-setup.md](design/penpot-setup.md) — Penpot + MCP для UI-макетов (ключи не коммитить)
- [design/penpot-workflow.md](design/penpot-workflow.md) — раскладка фреймов (канон ↓, варианты →), clip/контейнеры, placeholder content


