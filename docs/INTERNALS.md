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

Документ описывает внутреннее устройство Pomen: послойную организацию пакетов,
потоки данных, известные обходные пути (с обоснованием причин их применения) и
тестовую инфраструктуру. Адресат — разработчики, сопровождающие исходный код;
для пользователей и администраторов предназначены [FEATURES.md](FEATURES.md) и
[SETUP.md](SETUP.md).

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

- **main** — точка входа. Выполняет разбор переменных окружения (`PLUGIN_SOCKET`,
  `POMEN_DEV_PORT`, `CONFIG_FILE`, `STATE_FILE`, `VMS_FILE`), загрузку
  `config.json`, сборку Engine и подключение обработчиков через `api.Register`.
  Платформенно-зависимый запуск сокета вынесен в `socket_{unix,windows}.go`.
- **api** — HTTP-слой. Структура `Server` оборачивает Engine;
  `Register(mux, engine, version, ui)` — единая точка подключения обработчиков
  (по образцу `webapi.Register` в Intermasq). Структура `VMView` (DTO без
  секрета) определена в этом же пакете.
- **core** — доменный слой. Engine координирует операции
  Provision/Deprovision/Replay через интерфейсы `CaddyAPI`/`WebhookAPI`;
  конкретные реализации — `CaddyClient` и `WebhookClient`. Хранилища
  `StateStore` и `VMStore` построены на основе обобщённого `JSONStore[T]`.
- **internal/version** — единственное расположение переменной `var Version`.
  CI внедряет значение версии посредством
  `-ldflags="-X pomen/internal/version.Version=…"`.

## Жизненный цикл Provision

`POST /api/provision {vm, container_name}` → `Engine.Provision`:

1. **Валидация.** При `c.Port == ""` возвращается `ErrBadRequest`; при
   неизвестном узле — `ErrBadRequest`. Эти ошибки преобразуются обработчиком в
   HTTP 400.
2. **Вычисление идентификаторов и домена** по шаблонам из
   `core/constants.go`:
   `domain = fmt.Sprintf(FQDNFormat, name, vm, node, baseDomain)`,
   `routeID = pod-<vm>-<name>-<node>`,
   `tlsID = podtls-<vm>-<name>-<node>`.
3. **`Caddy.ReplayRoute`** — помещает политику TLS и маршрут в Caddy методом
   `upsertByID` (см. [Upsert вместо прямого добавления](#upsert-вместо-прямого-добавления)).
4. **`Caddy.RestartCaddy`** — запрос `/stop` и полагание на параметр systemd
   `Restart=always`
   (см. [Принудительный перезапуск Caddy](#принудительный-перезапуск-caddy)).
   При ошибке выполняется попытка компенсации через `DeleteRouteAndTLS`.
5. **`State.Upsert`** — запись в `routes.json`. При ошибке конфигурация Caddy
   уже применена: домен фактически работоспособен, но не отображается в
   интерфейсе; ошибка фиксируется в журнале через `slog.Error`.

При успешном выполнении возвращается
`200 {"message":"Успех! <domain> -> <ip>:<port>"}`.

## Хранилище: JSONStore\[T\]

До рефакторинга `StateStore` и `VMStore` дублировали один и тот же код
загрузки/сохранения/атомарного переименования/мьютекса (около 60 строк). После
рефакторинга оба хранилища встраивают `*JSONStore[T]`:

```go
type StateStore struct { *JSONStore[RouteRecord] }
type VMStore     struct { *JSONStore[VMConfig]   }
```

`JSONStore[T]` предоставляет методы `Load()`, `Save()` (с мьютексом) и
приватные `load()/save()` для использования внутри критических секций хранилища
(методы `Upsert`/`Remove` самостоятельно получают блокировку и выполняют чтение
— изменение — запись). Атомарность сохранения обеспечивается временным файлом
и `os.Rename`, поэтому в случае сбоя посредине записи остаётся либо прежнее,
либо новое содержимое, но никогда не частичное.

## Изоляция для тестов: интерфейсы

Engine обращается к Caddy и вебхуку через интерфейсы, а не через конкретные
типы:

```go
type CaddyAPI interface {
    ReplayRoute(...) error
    RestartCaddy(node string) error
    DeleteRouteAndTLS(node, routeID, tlsID string) error
}
type WebhookAPI interface { ListContainers(...) ([]ContainerInfo, error) }
```

Благодаря этому в `core/engine_test.go` Caddy заменяется заглушкой, что
позволяет проверять бизнес-логику Provision/Deprovision без работающих
экземпляров Caddy и вебхука. Для интеграционных smoke-тестов предусмотрены
отдельные компоненты — `tests/fixtures/caddy-mock/` и
`tests/fixtures/webhook-mock/`, каждый с собственным `go.mod`, что исключает
зависимостные циклы с основным модулем.

## Известные обходные пути

### Upsert вместо прямого добавления

Изначально в `CaddyClient` было два метода: `AddRoute` (непосредственный POST)
и `ReplayRoute` (upsert через `GET /id/<id>` → PUT при наличии, иначе POST).
`AddRoute` оказался неиспользуемым и при повторном вызове Provision для того же
домена приводил к сбою Caddy с сообщением
`cannot apply more than one automation policy to host`. Метод удалён;
`ReplayRoute`, являющийся идемпотентным, применяется во всех сценариях.

Дополнительно `upsertByID` различает HTTP 500: если POST завершается с 500,
предполагается отсутствие родительского контейнера
(`/config/apps/tls/automation` или `/config/apps/http/servers/srv0`); в этом
случае вызывается `initIfMissing` (PUT на родительский путь), после чего POST
повторяется. При повторной ошибке POST возвращается ошибка с обоими телами
ответа для последующего анализа.

### Принудительный перезапуск Caddy

После выпуска сертификата Caddy не всегда применяет его немедленно при
перезагрузке: кэш политик автоматизации может сохранять устаревшее состояние.
Надёжное решение — направить `POST /stop`: процесс завершается, systemd с
параметром `Restart=always` перезапускает его, после чего Caddy перечитывает
`autosave.json` с чистого состояния.

Цена — кратковременный (около 1–2 секунд) перерыв в обслуживании узла. Для
Pomen это допустимо: Provision представляет собой ручную операцию, выполняемую
обычно 1–2 раза в день; автоматическая фоновая перезагрузка на текущем этапе
охватывает не все сценарии.

### Требование adnanh/webhook к телу запроса

`adnanh/webhook` возвращает `unsupported content type` **до** проверки
`trigger-rule`, если у запроса отсутствует тело. Поэтому `WebhookClient`
отправляет не `nil`, а `bytes.NewBufferString("{}")` с заголовком
`Content-Type: application/json`. При переходе на иной вебхук-демон тело
можно удалить. Подробности — в комментарии к функции `fetchContainers` в
`core/webhook.go`.

### Удаление с приоритетом состояния (state-first Deprovision)

`DeprovisionByID` сначала удаляет запись из `state.json`, затем выполняет
удаление в Caddy по принципу наилучшего усилия. Обратный порядок (Caddy →
state) при сбое `state.Remove` оставлял бы в состоянии запись без
соответствующего домена в Caddy, которую впоследствии невозможно удалить
повторно (Caddy уже пуст).

Caddy не поддерживает транзакции, поэтому при сбое удаления в Caddy в нём может
остаться фантомная запись; однако при следующем `ReplayCaddy` она не будет
возвращена в состояние (поскольку там отсутствует) — интерфейс и состояние
остаются согласованнанными.

### Откат при ошибке RestartCaddy

Если `ReplayRoute` завершается успешно, но `RestartCaddy` возвращает ошибку,
маршрут и политика TLS уже находятся в Caddy, однако сертификат не применён.
Engine выполняет откат: вызывает `DeleteRouteAndTLS` (по принципу наилучшего
усилия, ошибки фиксируются внутри) и возвращает ошибку клиенту. Полная очистка
не гарантируется (Caddy нетранзакционен), однако наиболее частый случай
обрабатывается.

При ошибке `State.Upsert` (после успешного применения Caddy и перезапуска)
домен фактически работоспособен, но не отображается в интерфейсе; ошибка
фиксируется как `slog.Error`. Откат Caddy в этом случае не требуется
(сертификат уже действует), а запись может быть восстановлена повторным
назначением домена (Upsert идемпотентен по `route_id`).

### Сентинельные ошибки (ErrBadRequest / ErrNotFound)

Слою API необходимо различать «некорректный запрос клиента» (4xx) и «внутренний
сбой» (5xx). В `core/errors.go` определены:

```go
var (
    ErrBadRequest = errors.New("bad request")
    ErrNotFound   = errors.New("not found")
)
```

Методы `VMStore.Get/Upsert/Delete`, `Engine.AddVM/Provision/DeprovisionByID` и
др. оборачивают их посредством `fmt.Errorf("%w: …", ErrBadRequest, …)`. В
`api/routes.go` функция `statusForError(err)` выполняет проверку `errors.Is` и
возвращает 400/404/500. До введения этого механизма любая ошибка приводила к
500, и smoke-тесты фиксировали 500 там, где семантически требовался 400
(отсутствует метка порта) или 404 (несуществующий `route_id`).

### Межисточниковый iframe (getPluginBase / getAuthToken)

Интерфейс Pomen располагается в `<iframe>` внутри панели Intermasq. До
рефакторинга фронтенд на верхнем уровне напрямую обращался к
`window.parent.location.origin`, что в межисточниковом iframe вызывает
`SecurityError` и нарушает работу bootstrap в режиме разработки.

В настоящее время `web/app.js` использует вспомогательные функции
`getPluginBase()` и `getAuthToken()` с конструкцией `try/catch` и явным
разделением сценариев: рабочий iframe, разработка (тот же источник) и
межисточниковый родитель (возврат к тому же источнику). Это закрывает как
сценарии разработки, так и граничные случаи blob-/popup-iframe.

### Права сокета через umask

Unix-сокет создаётся вызовом `net.Listen("unix", path)`. Если выполнить `Chmod`
**после** `Listen`, между вызовами остаётся окно состязания, в течение которого
сокет доступен всем (согласно umask). В `socket_unix.go` umask устанавливается
**до** `Listen`:

```go
oldUmask := syscall.Umask(0o007)   // → 0777 & ~0o007 = 0770
listener, err := net.Listen("unix", path)
syscall.Umask(oldUmask)
```

`socket_windows.go` — заглушка без umask (Windows применяется только для
разработки, не для рабочего развёртывания). `Chmod` после `Listen` сохранён как
дополнительная мера.

## Версионирование

`internal/version.Version` — единственный источник значения версии. По
умолчанию `"1.0.0-pre"`; CI внедряет значение через `-ldflags`. CI также
обновляет `manifest.json` командой
`jq --arg v "${APP_VERSION}" '.version = $v'`, обеспечивая совпадение версий
исполняемого файла, API (`/api/version`) и `manifest.json`.
