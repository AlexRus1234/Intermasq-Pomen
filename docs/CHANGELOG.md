<!--
Pomen - plugin for Intermasq
Copyright (C) 2026 AlexRus1234

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
-->

# Changelog

Формат: каждый этап рефакторинга выделен в отдельный раздел. Ссылки на коммиты
приведены в журнале git.

## [Unreleased]

### Этап 7 — Документация

- Расширенный `README.md` по образцу Intermasq: бейджи, оглавление, разделы
  (Краткое описание, Быстрый старт, Конфигурация, Связь с Intermasq,
  Сравнение с Povez, API, Структура проекта, Разработка, Стек, Лицензия).
- `README.en.md` — английская версия.
- `docs/INTERNALS.md` — внутренняя архитектура и обходные пути.
- `docs/SETUP.md` — обновлён под `CONFIG_FILE`, режим разработки, embed web/, `/api/version`.
- `docs/CHANGELOG.md` — этот файл.

### Этап 6 — Тесты

- Go unit-тесты для всех пакетов: `core/state_test.go` (включая `-race`),
  `core/vmstore_test.go`, `core/webhook_test.go` (табличные кейсы normalize),
  `core/config_test.go`, `core/caddy_test.go` (JSON round-trip),
  `core/engine_test.go` (mock CaddyAPI), `api/routes_test.go` (httptest +
  regression на утечку Secret).
- `tests/fixtures/caddy-mock/` + `tests/fixtures/webhook-mock/` — отдельные
  go.mod, эмулируют Caddy Admin API и вебхук ВМ для smoke.
- `tests/lib/{state,common,http}.sh` — bash-хелперы по образцу Intermasq
  (адаптированные: без auth, т.к. Pomen в режиме разработки не имеет auth proxy).
- `tests/suites/NN-*.sh` — 8 bash-сьют: preflight, version, nodes, vms-crud,
  containers, provision, deprovision, replay.
- `tests/smoke.sh` — оркестратор.
- `tests/known-bugs.txt` — пустой, по образцу Intermasq.
- `tests/e2e/` — Playwright config + один UI-mount spec.
- CI: активирован L3 smoke (запускает mocks и pomen-ci в фоне и выполняет
  smoke.sh); активирован L4 Playwright (opt-in через `run_e2e_tests=true`).
- `core/errors.go` — sentinel errors `ErrBadRequest`/`ErrNotFound`;
  handlers через `statusForError` переводят их в 400/404, прочее — в 500.
- `main.go` — env `CONFIG_FILE` для override пути config.json.

### Этап 5 — Вынос жёстко заданных значений в конфигурацию

- `core/constants.go` — все значения по умолчанию и шаблоны (DefaultCaddyTimeout,
  DefaultWebhookTimeout, DefaultRestartDelay, DefaultWebhookPath,
  DefaultVMSecretHeader, FQDNFormat, RouteIDFormat, TLSIDFormat,
  ContainerSystemdPrefix).
- `core/config.go` — `Config` с секциями `tls`/`webhook`/`timeouts`;
  `WithDefaults()` применяет значения по умолчанию и разбирает Duration.
- `NewCaddyClient(urls, tlsCfg, timeout)`,
  `NewWebhookClient(timeout, secretHeader)`,
  `NewEngine(EngineOptions{...})` — все параметры через конфиг.
- `GenerateTLSPolicy(tlsCfg, …)` — ACME URL и CA-путь берутся из конфига,
  а не зашиты в код.
- `config.example.json` (placeholder'ы вместо продакшен-IP) + `.env.example`.
- `config.json` убран из git (`git rm --cached`) и добавлен в `.gitignore`.
- `GET /api/version` — отдаёт `internal/version.Version`.
- UI показывает версию в заголовке и footer.
- CI синхронизирует `manifest.json` через `jq --arg v "${APP_VERSION}"`.

### Этап 4 — Рефакторинг архитектуры

- `core/caddy_types.go` — структуры `CaddyRoute`/`CaddyTLSPolicy`/`CaddyIssuer`/…
  с json-тегами заменили `map[string]interface{}`.
- `core/caddy_paths.go` — константы путей Caddy Admin API.
- `core/jsonstore.go` — обобщённый `JSONStore[T any]` (sync.Mutex + atomic
  rename). `StateStore` и `VMStore` встраивают его.
- `core/webhook.go` — `ListContainers` разбит на `fetchContainers` (HTTP) +
  `normalizeContainer` (чистая функция).
- `core/interfaces.go` — интерфейсы `CaddyAPI`/`WebhookAPI`. Engine держит
  интерфейсы → можно мокать в тестах.
- `log/slog` вместо `fmt.Printf` для структурированного логирования.
- `api/register.go` — единая точка регистрации роутов `Register(mux, engine,
  version, ui)` по образцу хост-панели. `ApiServer` → `Server` (Go-конвенция).
- `//go:embed web` — интерфейс встраивается в исполняемый файл, зависимость от
  CWD отсутствует. JS вынесен в `web/app.js`, HTML в `web/index.html`.
- Engine: компенсация при частичных отказах. Provision при ошибке
  RestartCaddy откатывает через DeleteRouteAndTLS; Deprovision теперь
  state-first (Caddy-удаление best-effort).

### Этап 3 — Устранение обходных путей и неиспользуемого кода

- Удалён неиспользуемый `CaddyClient.AddRoute` (77 строк, не вызывался).
- `NewCaddyClient` более не изменяет входной map (создаёт копию).
- `slices.DeleteFunc` вместо приёма `records[:0]` в StateStore/VMStore.
- `upsertByID` различает первичный ответ 500 (инициализация родителя и повтор)
  и вторичный (фактическая ошибка Caddy): в сообщении об ошибке приводятся оба
  тела ответа.
- `webhook.go` — добавлен подробный комментарий об обходе требования
  adnanh/webhook к наличию тела запроса.
- `web/app.js` — функции `getPluginBase()`/`getAuthToken()` с явной обработкой
  сценариев production/dev/cross-origin.

### Этап 2 — Критические баги

- `StateStore` — добавлен `sync.Mutex` (раньше был без мьютекса, data race
  при параллельных Provision/Replay).
- `GET /api/vms` — DTO `VMView` без `Secret`. Ранее секрет был доступен через API.
- `main.go` — убран TCP-fallback `:5001`, добавлен явный режим разработки
  `POMEN_DEV_PORT`. Graceful shutdown через `http.Server.Shutdown(ctx)`
  с кодом возврата 0 (ранее `os.Exit(1)` на SIGTERM).
- `socket_unix.go` — umask до Listen, чтобы права сокета были 0770 atomically.
- `caddy.go` — закрытие `resp.Body` + проверка err во всех вызовах Caddy API.
- UI — `domainExample` вычисляет node из выбранной ВМ (`<name>.<vm>.<node>.internal`).

### Этап 1 — CI

- `.forgejo/workflows/build.yml` — копия пайплайна Intermasq (Fedora 44,
  Nora RPM, Step-CA, Go 1.26), адаптированная под Pomen.
- `.forgejo/workflows/mirror.yaml` — зеркалирование на GitHub/Codeberg
  (отключено до создания зеркал).
- `Makefile` — `build` / `build-linux` / `run` / `test` / `test-race` /
  `cover` / `vet` / `fmt` / `lint` / `clean`.
- `internal/version/version.go` + `version_test.go` — пакет версии.

### Этап 0 — Лицензирование

- AGPL-3.0 вместо заявленного Apache-2.0.
- `LICENSE` (полный текст AGPL-3.0).
- `SECURITY.md` — политика безопасности (RU+EN) по образцу Intermasq.
- AGPL-header на все 12 исходников (`.go` → `//`, `.md`/`.html` → `<!-- -->`).
- `manifest.json` — добавлено поле `"version"`.
- `.gitignore` переписан по образцу Intermasq.
