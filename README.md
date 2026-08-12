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

**Русский** | [English](README.en.md) |

<div align="center">

<h1>Pomen</h1>

**Плагин для [Intermasq](../Intermasq) — выдача доменов контейнерам Podman**

Pomen запускается как дочерний процесс панели Intermasq и по кнопке в UI
выдаёт контейнерам Podman (запущенным через Quadlet внутри ВМ) домены вида
`<container>.<vm>.<node>.internal` с HTTPS через Caddy. Источник данных —
вебхук на ВМ с `podman ps`; DNS-зону Intermasq плагин не трогает, маршрутизация
по Host-header делает Caddy. Один бинарь, ноль внешних зависимостей.

[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue.svg?style=flat-square)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8.svg?style=flat-square)](https://go.dev/)
[![Vue](https://img.shields.io/badge/Vue-3-4FC08D.svg?style=flat-square)](https://vuejs.org/)
[![Bootstrap](https://img.shields.io/badge/Bootstrap-5-7952B3.svg?style=flat-square)](https://getbootstrap.com/)
[![Platform](https://img.shields.io/badge/Linux-any-1793D1.svg?style=flat-square)](#быстрый-старт)

</div>

---

## Содержание

- [Краткое описание](#краткое-описание)
- [Быстрый старт](#быстрый-старт)
- [Конфигурация](#конфигурация)
- [Связь с материнским приложением](#связь-с-материнским-приложением)
- [Сравнение с Povez](#сравнение-с-povez)
- [API](#api)
- [Структура проекта](#структура-проекта)
- [Разработка и тесты](#разработка-и-тесты)
- [Технологический стек](#технологический-стек)
- [Документация](#документация)
- [Лицензия](#лицензия)

> Расширенная документация по возможностям, установке и внутренней архитектуре
> приведена в каталоге [`docs/`](docs/FEATURES.md). Настоящий README содержит
> обзор плагина и инструкции по запуску.

## Краткое описание

Pomen — это плагин для панели [Intermasq](../Intermasq). Он не работает в
отрыве от хоста: материнское приложение запускает его через unix-сокет и
проксирует на него запросы через `/plugins/pomen/*` после своей аутентификации.

Цель — по клику в UI выдавать домены контейнерам Podman, запущенным через
Quadlet внутри короткоживущих ВМ. Под капотом:

- **On-demand pull:** сети нет фонового шума. Между кликами плагин молчит;
  по клику «Обновить» он стучит на вебхук ВМ (adnanh/webhook), тот исполняет
  `podman ps --format json` и возвращает список контейнеров.
- **Caddy reverse_proxy:** маршрут пишется в Admin API Caddy ноды; TLS
  выпускается через внутренний Step-CA. DNS-зону Intermasq плагин не трогает —
  достаточно одного wildcard `address=/.<node>.internal/<caddy-ip>` в dnsmasq.
- **Idempotent upsert:** повторный Provision того же контейнера не плодит
  дубль TLS-политик в Caddy (см. [docs/INTERNALS.md](docs/INTERNALS.md)).
- **Один бинарь:** Go + Vue 3 + Bootstrap 5 в одном файле ~10 МБ. UI embed'ится
  через `//go:embed web`, поэтому никаких внешних файлов в production не нужно.

## Быстрый старт

Pomen — плагин. Подробная инструкция по установке в окружение Intermasq — в
[docs/SETUP.md](docs/SETUP.md). Здесь — короткая версия для разработки.

### Требования

- Go 1.25+
- Linux (для production); Windows/macOS собираются, но unix-сокет работает
  только на POSIX (см. `socket_unix.go` / `socket_windows.go`)

### Сборка

```bash
make build          # локальный бинарь ./pomen
make build-linux    # кросс-компиляция под linux/amd64 → ./pomen-linux
```

### Production-режим (под Intermasq)

Бинарь копируется в `/etc/intermasq/plugins/pomen/`, `manifest.json`
объявляет плагин хосту. Хост передаёт `PLUGIN_SOCKET` в env и
проксирует `/plugins/pomen/*` на сокет. См. [docs/SETUP.md](docs/SETUP.md).

### Dev-режим (локальная разработка без Intermasq)

```bash
cp config.example.json config.json     # отредактируй под своё окружение
POMEN_DEV_PORT=18992 ./pomen           # поднимется на TCP :18992
curl http://127.0.0.1:18992/api/version
```

`POMEN_DEV_PORT` заменяет `:5001` fallback, который был раньше. Теперь это
явный, отдельный режим без unix-сокета — он нужен только для локальной
разработки и CI smoke/E2E тестов.

## Конфигурация

### `config.json`

Положите рядом с бинарем (или укажите через `CONFIG_FILE`). Шаблон — в
[`config.example.json`](config.example.json). Все секции кроме `base_domain`
и `nodes` можно опустить, будут применены дефолты из `core/constants.go`.

```json
{
  "base_domain": ".internal",
  "nodes": {
    "node0": { "caddy_url": "http://caddy-node0.internal:2019" }
  },
  "tls":     { "acme_ca": "https://step-ca.internal:9000/acme/acme/directory",
               "root_ca_path": "/etc/caddy/root_ca.crt" },
  "webhook": { "path": "/hooks/podman", "secret_header": "X-VM-Secret" },
  "timeouts":{ "caddy": "10s", "webhook": "15s", "restart_delay": "2s" }
}
```

### Переменные окружения

| Переменная | Назначение | По умолчанию |
|---|---|---|
| `PLUGIN_SOCKET` | Путь unix-сокета (от хоста Intermasq) | — (обязателен в production) |
| `POMEN_DEV_PORT` | TCP-порт для dev-режима (без сокета) | — |
| `CONFIG_FILE` | Путь к config.json | `config.json` |
| `STATE_FILE` | Путь к routes.json | `/etc/intermasq/plugins/pomen/routes.json` |
| `VMS_FILE` | Путь к vms.json | `/etc/intermasq/plugins/pomen/vms.json` |

Шаблон с описанием — в [`.env.example`](.env.example).

## Связь с материнским приложением

Pomen следует контракту плагинов Intermasq (см. `docs/func/ru/plugins.md`
в материнском репозитории):

1. Каталог `/etc/intermasq/plugins/pomen/` содержит `manifest.json` + бинарь.
2. Хост запускает плагин как дочерний процесс и передаёт `PLUGIN_SOCKET` в env.
3. Плагин слушает этот unix-сокет с правами `0770`.
4. Хост монтирует reverse-proxy на `/plugins/pomen/*` **после** своей auth.
5. UI монтирует `<iframe src="/plugins/pomen/">` — фронтенд читает JWT из
   `window.parent.localStorage`.

Контракт не требует от плагина собственной аутентификации — это задача хоста.
В dev-режиме (`POMEN_DEV_PORT`) плагин работает без auth и без сокета.

## Сравнение с Povez

Pomen и [Povez](../Intermasq-Povez) — два плагина выдачи доменов, различаются
источником данных:

| | Povez | Pomen |
|---|---|---|
| Источник | Proxmox API | Вебхук ВМ + `podman ps` |
| Объект | ВМ/LXC PVE | Контейнеры Podman внутри ВМ |
| Параметры | Теги PVE `port-`/`proto-`/`name-` | Labels Podman `port-`/`proto-`/`name-` |
| IP upstream | Вычисление `[subnet].[VMID-98]` | `IP_VM` (из реестра) |
| DNS в Intermasq | `dhcp-host` (DHCP-резервация) | **не пишется** (wildcard) |
| Caddy ID | `proxy-<VMID>-<node>` | `pod-<vm>-<name>-<node>` |
| Trigger | По MAC из leases | По кнопке в UI |

Оба плагина пишут в один и тот же Caddy per-ноды; префиксы `@id` (`proxy-` vs
`pod-`) гарантируют отсутствие конфликтов.

## API

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/version` | Версия сборки (JSON `{"version":"…"}`) |
| GET | `/api/nodes` | Список ключей нод из config.json |
| GET | `/api/vms` | Реестр ВМ (без секретов) |
| POST | `/api/vms` | Добавить/обновить ВМ |
| DELETE | `/api/vms/<name>` | Удалить ВМ из реестра |
| GET | `/api/containers?vm=<name>` | Список контейнеров через вебхук ВМ |
| POST | `/api/provision` | Выдать домен контейнеру |
| POST | `/api/deprovision` | Удалить маршрут по `route_id` |
| DELETE | `/api/deprovision/<route_id>` | Удалить маршрут |
| GET | `/api/state` | Содержимое routes.json |
| POST | `/api/replay` | Перезаписать конфиг Caddy из state |
| GET | `/` | UI (index.html, embed) |
| GET | `/app.js` | UI (app.js, embed) |

Ошибки `core.ErrBadRequest` → HTTP 400, `core.ErrNotFound` → HTTP 404, прочее
→ 500 (см. [docs/INTERNALS.md](docs/INTERNALS.md), раздел «Sentinel errors»).

## Структура проекта

```
Pomen/
├── main.go                     # точка входа: loadConfig + запуск сервера
├── socket_unix.go              # unix-сокет с umask(0o007) → права 0770
├── socket_windows.go           # заглушка для Windows dev-сборок
├── manifest.json               # контракт Intermasq: {id,name,version,bin}
├── config.example.json         # шаблон config.json (без prod-данных)
├── .env.example                # шаблон env-переменных
├── go.mod                      # module pomen, go 1.25 (0 внешних зависимостей)
├── Makefile                    # build / test / lint / run цели
│
├── api/                        # транспортный слой (HTTP handlers)
│   ├── routes.go               # Server + handlers + VMView (DTO без Secret)
│   └── register.go             # Register(mux, engine, version, ui) — единая точка
│
├── core/                       # доменный слой
│   ├── models.go               # NodeConfig / VMConfig / ContainerInfo
│   ├── config.go               # Config + WithDefaults()
│   ├── constants.go            # дефолты (timeouts, header, path) + ID-форматы
│   ├── errors.go               # ErrBadRequest / ErrNotFound
│   ├── jsonstore.go            # JSONStore[T] — generic load/save/mutex
│   ├── state.go                # StateStore = JSONStore[RouteRecord] + Upsert/Remove
│   ├── vmstore.go              # VMStore   = JSONStore[VMConfig]  + Get/Upsert/Delete
│   ├── interfaces.go           # CaddyAPI / WebhookAPI — для mock'ов в тестах
│   ├── caddy.go                # CaddyClient (upsertByID, Restart, Delete)
│   ├── caddy_types.go          # CaddyRoute / CaddyTLSPolicy / CaddyIssuer ...
│   ├── caddy_paths.go          # константы путей Caddy Admin API
│   ├── webhook.go              # WebhookClient + normalizeContainer
│   └── engine.go               # Engine — оркестрация Provision / Deprovision / Replay
│
├── internal/version/           # ldflags-инжектируемая версия сборки
├── web/                        # embed-нутые в бинарь UI-файлы
│   ├── index.html              # Vue 3 + Bootstrap 5 (CDN)
│   └── app.js                  # фронтенд-логика (вынесена из index.html)
│
├── tests/                      # все уровни тестирования
│   ├── smoke.sh                # оркестратор bash-smoke
│   ├── known-bugs.txt          # список известных багов (для KNOWN-fail'ов)
│   ├── lib/                    # state.sh / common.sh / http.sh (bash-хелперы)
│   ├── suites/                 # NN-*.sh сьюты (preflight, version, vms, ...)
│   ├── fixtures/
│   │   ├── caddy-mock/         # эмулятор Caddy Admin API (отдельный go.mod)
│   │   └── webhook-mock/       # эмулятор вебхука ВМ с podman ps
│   └── e2e/                    # Playwright (UI-mount spec)
│
├── docs/                       # FEATURES / SETUP / INTERNALS / CHANGELOG
├── .forgejo/workflows/         # CI (build.yml + mirror.yaml)
├── LICENSE                     # AGPL-3.0
└── SECURITY.md                 # политика безопасности (RU+EN)
```

## Разработка и тесты

```bash
make test           # go test ./...
make test-race      # + -race (конкурентность StateStore и т.п.)
make cover          # coverage.out + summary
make lint           # go vet + gofmt gate
make run            # собрать и запустить в dev-режиме
```

Полный CI прогон (см. [`.forgejo/workflows/build.yml`](.forgejo/workflows/build.yml)):

- **L1+L2 — Go unit** (`go test -race`)
- **L3 — bash smoke** (поднимает `caddy-mock` + `webhook-mock` + `pomen-ci` в
  dev-режиме, гоняет `tests/smoke.sh`)
- **L4 — Playwright E2E** (opt-in через `run_e2e_tests=true`; один UI-mount spec)

Fixtures (`tests/fixtures/`) — отдельные go.mod, чтобы не засорять основной
модуль зависимостями тестов.

## Технологический стек

- **Backend:** Go 1.25 (stdlib only — `net/http`, `encoding/json`, `log/slog`,
  `slices`, `sync`, `embed`)
- **Frontend:** Vue 3 (Composition API) + Bootstrap 5 + Axios (через CDN)
- **External services:** Caddy (Admin API + Step-CA), adnanh/webhook на ВМ
- **CI/CD:** Forgejo Actions, fedora:44 runner (тот же, что у Intermasq)
- **Лицензия:** AGPL-3.0

## Документация

- [`docs/FEATURES.md`](docs/FEATURES.md) — полный список возможностей
- [`docs/SETUP.md`](docs/SETUP.md) — пошаговая установка в Intermasq + настройка ВМ
- [`docs/INTERNALS.md`](docs/INTERNALS.md) — внутренняя архитектура и обходные пути
- [`docs/CHANGELOG.md`](docs/CHANGELOG.md) — история изменений
- [`SECURITY.md`](SECURITY.md) — политика безопасности (RU+EN)

## Лицензия

[AGPL-3.0](LICENSE) © 2026 AlexRus1234

---

> Проект разработан в соответствии с заранее определённой архитектурой; при
> подготовке исходного кода использовался ИИ-ассистент.
