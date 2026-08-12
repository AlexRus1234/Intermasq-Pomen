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

# Внутренняя архитектура Pomen

Этот документ описывает, как Pomen устроен внутри: слой пакетов, потоки
данных, известные обходные пути (с пояснением, почему они нужны) и
тестовую инфраструктуру. Аудитория — разработчики, сопровождающие код; для
пользователей и админов есть [FEATURES.md](FEATURES.md) и [SETUP.md](SETUP.md).

## Содержание

- [Слои](#слои)
- [Жизненный цикл Provision](#жизненный-цикл-provision)
- [Хранилище: JSONStore\[T\]](#хранилище-jsonstoret)
- [Изоляция для тестов: интерфейсы](#изоляция-для-тестов-интерфейсы)
- [Известные обходные пути](#известные-обходные-пути)
- [Версионирование](#версионирование)

## Слои

```
                 ┌────────────────────────────────────┐
                 │                main                │     ← env, flag.Parse, lifecycle
                 │  (socket_unix.go, socket_win.go)   │
                 └───┬──────────────────────────┬─────┘
                     │                          │
                     ▼                          ▼
              ┌────────────┐            ┌──────────────┐
              │  api/http  │            │  internal /  │
              │  (Server,  │            │   version    │
              │  Register) │            │  (ldflags)   │
              └─────┬──────┘            └──────────────┘
                    │ вызывает
                    ▼
        ┌───────────────────────────────────────┐
        │                core                   │
        │  ┌───────────┐  ┌───────────┐         │
        │  │  Engine   │──│ Interfaces│         │
        │  └─────┬─────┘  └───────────┘         │
        │        │ depends on (via interface)   │
        │  ┌─────┴────────────┬──────────────┐  │
        │  ▼                  ▼              ▼  │
        │ CaddyClient    WebhookClient   Stores │
        │ (Admin API)    (HTTP+podman)   (JSON) │
        └───────────────────────────────────────┘
```

- **main** — точка входа. Парсит env (`PLUGIN_SOCKET`, `POMEN_DEV_PORT`,
  `CONFIG_FILE`, `STATE_FILE`, `VMS_FILE`), грузит `config.json`, собирает
  Engine и навешивает handlers через `api.Register`. Платформо-зависимый
  запуск сокета вынесен в `socket_{unix,windows}.go`.
- **api** — HTTP-слой. `Server` оборачивает Engine; `Register(mux, engine,
  version, ui)` — единая точка навешивания handlers (по образцу
  `webapi.Register` в Intermasq). VMView (DTO без Secret) живет здесь же.
- **core** — домен. Engine оркестрирует Provision/Deprovision/Replay через
  интерфейсы `CaddyAPI`/`WebhookAPI`; конкретные реализации — `CaddyClient`
  и `WebhookClient`. Хранилища — `StateStore` и `VMStore` поверх generic
  `JSONStore[T]`.
- **internal/version** — единственное место, где лежит `var Version`. CI
  инжектит туда версию через `-ldflags="-X pomen/internal/version.Version=…"`.

## Жизненный цикл Provision

`POST /api/provision {vm, container_name}` → `Engine.Provision`:

1. **Валидация.** `c.Port == ""` → `ErrBadRequest`; unknown node → `ErrBadRequest`.
   Эти ошибки handler переводит в HTTP 400.
2. **Вычисление ID/домена** по шаблонам из `core/constants.go`:
   `domain = fmt.Sprintf(FQDNFormat, name, vm, node, baseDomain)`,
   `routeID = pod-<vm>-<name>-<node>`,
   `tlsID = podtls-<vm>-<name>-<node>`.
3. **`Caddy.ReplayRoute`** — пишет TLS-политику и маршрут в Caddy через
   `upsertByID` (см. [Replay vs Add](#replay-vs-add-почему-upsert)).
4. **`Caddy.RestartCaddy`** — `/stop` + расчёт на `systemd Restart=always`
   (см. [The Nuke](#the-nuke-caddy-stop--systemd-restartalways)).
   При ошибке — попытка компенсации через `DeleteRouteAndTLS`.
5. **`State.Upsert`** — запись в routes.json. При ошибке Caddy уже настроен:
   домен фактически жив, но невидим в UI. Логируется `slog.Error`.

При успехе возвращается `200 {"message":"Успех! <domain> -> <ip>:<port>"}`.

## Хранилище: JSONStore\[T\]

`StateStore` и `VMStore` до рефакторинга копировали один и тот же
load/save/atomic-rename/mutex код (~60 строк). После — оба встраивают
`*JSONStore[T]`:

```go
type StateStore struct { *JSONStore[RouteRecord] }
type VMStore     struct { *JSONStore[VMConfig]   }
```

`JSONStore[T]` предоставляет `Load()`, `Save()` (с мьютексом) и приватные
`load()/save()` для использования внутри критических секций стор'а
(`Upsert`/`Remove` сами берут `locker()` и читают-модифицируют-пишут).
Атомарность записи — tmp-файл + `os.Rename`, поэтому при сбое посреди записи
остаётся либо старое содержимое, либо новое, но никогда половинчатое.

## Изоляция для тестов: интерфейсы

Engine держит Caddy и Webhook через интерфейсы, не через конкретные типы:

```go
type CaddyAPI interface {
    ReplayRoute(...) error
    RestartCaddy(node string) error
    DeleteRouteAndTLS(node, routeID, tlsID string) error
}
type WebhookAPI interface { ListContainers(...) ([]ContainerInfo, error) }
```

Поэтому `core/engine_test.go` подменяет Caddy моком и проверяет бизнес-логику
Provision/Deprovision без живого Caddy/вебхука. Для интеграционных smoke-тестов
есть отдельные fixture'ы — `tests/fixtures/caddy-mock/` и
`tests/fixtures/webhook-mock/`, у каждого свой `go.mod`, чтобы не плодить
dep-циклы с основным модулем.

## Известные обходные пути

### Replay vs Add (почему upsert)

Изначально у CaddyClient было два метода: `AddRoute` (чистый POST) и
`ReplayRoute` (upsert через `GET /id/<id>` → PUT если есть, иначе POST).
`AddRoute` оказался мёртвым кодом и сломанным при повторном Provision того же
домена: Caddy падал с `cannot apply more than one automation policy to host`.
Сейчас удалён; `ReplayRoute` используется везде — он идемпотентен.

`upsertByID` дополнительно различает HTTP 500: если POST падает с 500, мы
предполагаем «родительский путь не создан» (`/config/apps/tls/automation` или
`/config/apps/http/servers/srv0`) и вызываем `initIfMissing` — PUT на
родительский путь, затем повторяем POST. Если повторный POST тоже падает —
возвращаем ошибку с обоими телами ответа (чтобы было что анализировать).

### The Nuke (Caddy /stop + systemd Restart=always)

После выпуска сертификата Caddy не всегда подхватывает его сразу через
`reload` — кэш automation policy иногда залипает. Надёжный workaround: послать
`POST /stop` — процесс падает, systemd с `Restart=always` поднимает его через
секунду, Caddy перечитывает `autosave.json` с чистого листа.

Цена — даунтайм ~1-2 секунды на ноде. В Pomen это приемлемо: Provision —
ручная операция, обычно 1-2 раза в день; автоматический фоновый reload сейчас
не покрывает все кейсы.

### adnanh/webhook требует пустой JSON-body

`adnanh/webhook` падает с `unsupported content type` **до** проверки
`trigger-rule`, если у запроса нет body. Поэтому `WebhookClient` шлёт не `nil`,
а `bytes.NewBufferString("{}")` с заголовком `Content-Type: application/json`.
Когда/если перейдём на другой вебхук-демон, body можно убрать. Подробнее — в
комментарии в `core/webhook.go`, `fetchContainers`.

### State-first Deprovision

`DeprovisionByID` сначала удаляет запись из `state.json`, потом best-effort
`Caddy.DeleteRouteAndTLS`. Обратный порядок (Caddy → state) при сбое
`state.Remove` оставлял бы «висящую» в state запись без домена в Caddy,
которую потом нельзя повторно удалить (Caddy уже пуст).

Caddy не транзакционен, поэтому при сбое Caddy-удаления в Caddy остаётся
фантом — но при следующем `ReplayCaddy` он не вернётся в state (его там нет),
и UI/state остаются консистентны.

### Rollback при ошибке RestartCaddy

Если `ReplayRoute` успешен, но `RestartCaddy` падает — маршрут и TLS-политика
уже в Caddy, но сертификат не применён. Engine пытается откатить: вызывает
`DeleteRouteAndTLS` (best-effort, ошибки логируются внутри) и возвращает
ошибку клиенту. Это не гарантирует полную чистоту (Caddy не транзакционен),
но спасает от самого частого случая.

При ошибке `State.Upsert` (после успешного Caddy+restart) домен фактически
жив, но невидим в UI — это логируется как `slog.Error`. Откатывать Caddy в
этом случае не нужно (сертификат уже работает), а запись можно восстановить,
пере-выдав домен (Upsert идемпотентен по `route_id`).

### Sentinel errors (ErrBadRequest / ErrNotFound)

Слою API важно различать «клиент дал плохой запрос» (4xx) от «что-то сломалось
внутри» (5xx). В `core/errors.go` определены:

```go
var (
    ErrBadRequest = errors.New("bad request")
    ErrNotFound   = errors.New("not found")
)
```

`VMStore.Get/Upsert/Delete`, `Engine.AddVM/Provision/DeprovisionByID` и т.п.
оборачивают их через `fmt.Errorf("%w: …", ErrBadRequest, …)`. В `api/routes.go`
`statusForError(err)` проверяет `errors.Is` и возвращает 400/404/500. До этого
любая ошибка давала 500, и smoke ловил 500 там, где semantically нужен был
400 (нет port label) или 404 (несуществующий route_id).

### Cross-origin iframe (getPluginBase / getAuthToken)

UI Pomen живёт в `<iframe>` внутри панели Intermasq. До рефакторинга фронт
буквально делал `window.parent.location.origin` на верхнем уровне — это
бросает `SecurityError` в cross-origin iframe и ломает bootstrap в dev-режиме.

Теперь `web/app.js` использует хелперы `getPluginBase()` / `getAuthToken()` с
`try/catch` и явными сценариями: production iframe, dev (тот же origin),
cross-origin parent (fallback на тот же origin). Это закрывает и的开发-кейсы,
и edge-кейсы blob/popup iframe.

### umask-based socket permissions

Unix-сокет создаётся через `net.Listen("unix", path)`. Если сделать `Chmod`
**после** Listen — между вызовами есть race window, когда сокет доступен world
(по umask). В `socket_unix.go` umask выставляется **до** Listen:

```go
oldUmask := syscall.Umask(0o007)   // → 0777 & ~0o007 = 0770
listener, err := net.Listen("unix", path)
syscall.Umask(oldUmask)
```

`socket_windows.go` — заглушка без umask (Windows не в production, только dev).
`Chmod` после Listen оставлен как подстраховка.

## Версионирование

`internal/version.Version` — единственный источник истины. По умолчанию
`"1.0.0-pre"`; CI инжектит через `-ldflags`. CI также обновляет `manifest.json`
через `jq --arg v "${APP_VERSION}" '.version = $v'`, чтобы версии бинаря, API
(`/api/version`) и `manifest.json` совпадали.
