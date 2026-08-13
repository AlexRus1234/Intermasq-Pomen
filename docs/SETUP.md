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

# Установка и настройка Pomen

## 1. Требования

- Работоспособный **Intermasq** с поддержкой плагинов (`/etc/intermasq/plugins/`).
- **Caddy** на каждом узле с настроенным Admin API (`:2019`) и Step-CA
  (см. документацию Povez).
- **Wildcard-запись DNS** в dnsmasq для каждого узла:
  `address=/.yadr00.internal/172.20.5.3` (IP-адрес Caddy узла).
- **Вебхук adnanh/webhook** на каждой виртуальной машине с контейнерами Podman
  (порт 9000 или иной).
- Podman запущен от того же пользователя, что и вебхук.

## 2. Установка плагина на сервер Intermasq

### 2.1 Перенос файлов

Начиная с версии после рефакторинга интерфейс встраивается в исполняемый файл
директивой `//go:embed web` (см. [INTERNALS.md](INTERNALS.md)); отдельное
копирование `index.html` и `app.js` не требуется, поскольку они уже входят в
состав `pomen`:

```bash
sudo mkdir -p /etc/intermasq/plugins/pomen
sudo cp pomen-linux                /etc/intermasq/plugins/pomen/pomen
sudo chmod +x                      /etc/intermasq/plugins/pomen/pomen
sudo cp manifest.json              /etc/intermasq/plugins/pomen/
sudo cp config.example.json        /etc/intermasq/plugins/pomen/config.json
sudo chown -R intermasq:intermasq  /etc/intermasq/plugins/pomen
```

> `config.example.json` — шаблон с подстановочными значениями. Его копируют в
> `config.json` и редактируют применительно к целевому окружению (см. 2.2).
> Сам `config.json` с рабочими данными в репозиторий не помещается и включён в
> `.gitignore`.

### 2.2 config.json (статические параметры — узлы, Caddy, TLS, таймауты)

`/etc/intermasq/plugins/pomen/config.json`:

```json
{
    "base_domain": ".internal",
    "nodes": {
        "yadr00": { "caddy_url": "http://172.20.5.3:2019" },
        "yadr01": { "caddy_url": "http://172.20.6.3:2019" }
    },
    "tls":     { "acme_ca": "https://172.20.0.1:9000/acme/acme/directory",
                 "root_ca_path": "/etc/caddy/root_ca.crt" },
    "webhook": { "path": "/hooks/podman", "secret_header": "X-VM-Secret" },
    "timeouts":{ "caddy": "10s", "webhook": "15s", "restart_delay": "2s" }
}
```

- `base_domain` — суффикс всех доменов;
  FQDN = `<container>.<vm>.<node><base_domain>`.
- `nodes` — соответствие узлов URL их Caddy Admin API. **Те же, что и в Povez.**
- `tls` — параметры издателя ACME для Caddy (по умолчанию внутренний Step-CA).
- `webhook` — путь на вебхуке виртуальной машины (`/hooks/podman` для
  adnanh/webhook) и имя HTTP-заголовка для передачи секрета.
- `timeouts` — HTTP-таймауты Caddy и вебхука, а также пауза перед запросом
  `/stop` в ReplayCaddy.

Любую секцию, кроме `base_domain` и `nodes`, можно опустить — будут применены
значения по умолчанию из `core/constants.go`. Секреты виртуальных машин в этом
файле **не хранятся**; они размещаются в `vms.json` и добавляются через
интерфейс.

Путь к конфигурации можно переопределить переменной окружения `CONFIG_FILE` (по
умолчанию `config.json` рядом с исполняемым файлом).

### 2.3 manifest.json

Уже содержится в комплекте:

```json
{
    "id": "pomen",
    "name": "Pomen",
    "version": "1.0.0",
    "bin": "pomen"
}
```

Поле `version` обновляется автоматически при сборке в CI (через `jq`) и
совпадает со значением, возвращаемым `GET /api/version` и `--version`.

При запуске Intermasq автоматически:

- обнаруживает `/etc/intermasq/plugins/pomen/manifest.json`;
- запускает `pomen` как дочерний процесс;
- передаёт `PLUGIN_SOCKET=/run/intermasq/sockets/pomen.sock`;
- проксирует `/plugins/pomen/*` на этот сокет.

### 2.4 Перезапуск Intermasq

```bash
sudo systemctl restart intermasq
sudo journalctl -u intermasq --since "1 min ago" | grep PLUGINS
```

В журнале должна появиться запись:
`[PLUGINS] Started Pomen on socket /run/intermasq/sockets/pomen.sock`.

### 2.5 Доступ к интерфейсу

```
http://<IP_INTERMASQ>:8080/plugins/pomen/
```

В заголовке интерфейса отображается версия сборки — это удобная проверка того,
что запущен ожидаемый исполняемый файл.

### 2.6 Альтернатива: режим разработки без Intermasq

Для локальной разработки или smoke-тестов Pomen можно запустить вне хоста:

```bash
cp config.example.json config.json       # отредактируйте применительно к окружению
POMEN_DEV_PORT=18992 ./pomen             # прослушивает TCP :18992, без unix-сокета
curl http://127.0.0.1:18992/api/version
```

`POMEN_DEV_PORT` — единственный способ запустить Pomen в обход контракта
Intermasq. В этом режиме **прокси аутентификации отсутствует**; не используйте
его в рабочем окружении.

## 3. Настройка вебхука на виртуальной машине

На каждой виртуальной машине с Podman должен быть запущен `adnanh/webhook` с
хуком `podman`.

### 3.1 Файл хуков для Pomen

Создайте отдельный файл, не затрагивая рабочие хуки git-sync:

```bash
cat > /opt/appdata/config/webhook/pomen-hooks.json <<'EOF'
[
  {
    "id": "podman",
    "execute-command": "/usr/bin/podman",
    "pass-arguments-to-command": [
      { "source": "string", "name": "ps" },
      { "source": "string", "name": "--format" },
      { "source": "string", "name": "json" }
    ],
    "command-working-directory": "/home/app-runner",
    "capture-command-output": true,
    "include-command-output-in-response": true,
    "include-command-output-in-response-on-error": true,
    "trigger-rule": {
      "match": {
        "type": "value",
        "value": "ТВОЙ_СЕКРЕТ",
        "parameter": { "source": "header", "name": "X-VM-Secret" }
      }
    }
  }
]
EOF
```

### 3.2 Подключение к запуску вебхука

Если вебхук уже запущен с одним файлом хуков, добавьте второй через
переопределение:

```bash
mkdir -p ~/.config/systemd/user/webhook.service.d
cat > ~/.config/systemd/user/webhook.service.d/override.conf <<'EOF'
[Service]
ExecStart=
ExecStart=%h/.local/bin/webhook -hooks /opt/appdata/config/webhook/hooks.json -hooks /opt/appdata/config/webhook/pomen-hooks.json -verbose -port 9000
EOF

systemctl --user daemon-reload
systemctl --user restart webhook
systemctl --user status webhook
```

### 3.3 Проверка

С сервера Intermasq:

```bash
curl -X POST -H 'X-VM-Secret: ТВОЙ_СЕКРЕТ' -H 'Content-Type: application/json' -d '{}' http://IP_VM:9000/hooks/podman
```

В ответ должен вернуться JSON-массив контейнеров, формируемый командой
`podman ps --format json`.

> **Важно:** в curl используйте **одинарные кавычки** для заголовка с секретом;
  при наличии в секрете символа `$` двойные кавычки привели бы к подстановке
  переменной оболочкой.

## 4. Регистрация виртуальных машин в интерфейсе Pomen

1. Откройте интерфейс: `http://<IP_INTERMASQ>:8080/plugins/pomen/`.
2. На вкладке **«Виртуальные машины»** заполните форму «Добавить ВМ»:

| Поле | Пример | Описание |
|---|---|---|
| Имя | `obshaga` | Уникальное имя виртуальной машины (без учёта регистра) |
| Узел | `yadr00` | Выбор из config.json |
| IP | `172.20.5.17` | IP-адрес виртуальной машины в сети |
| Webhook URL | `http://172.20.5.17:9000` | URL вебхука виртуальной машины |
| Секрет | `nP$JANUTx^#39dKzs3LQ` | Значение `value` из pomen-hooks.json |

3. Нажмите «Добавить». Запись будет помещена в
   `/etc/intermasq/plugins/pomen/vms.json`.

## 5. Метки в .container (Quadlet)

Параметры проксирования определяются метками контейнера. В файле Quadlet
(`*.container`):

```ini
[Container]
Image=...
PublishPort=8080:8080
Label=name-app
Label=port-8080
Label=proto-http
```

| Метка | Обязательность | Описание |
|---|---|---|
| `name-<имя>` | Нет | Переопределяет имя домена. По умолчанию берётся из имени контейнера с удалением префикса `systemd-` и нумерации (`systemd-01-athens` → `athens`). |
| `port-<порт>` | Нет | Переопределяет порт восходящего потока. По умолчанию берётся первый `host_port` из `Ports` вывода `podman ps`. |
| `proto-<http/https>` | Нет | Протокол восходящего потока. По умолчанию `http`; при `https` добавляется `insecure_skip_verify`. |

Если метка не указана, плагин использует значения по умолчанию из `podman ps`.

## 6. Wildcard-запись DNS в dnsmasq

Для каждого узла с установленным Caddy добавьте в `/etc/dnsmasq.d/`:

```ini
address=/.yadr00.internal/172.20.5.3
address=/.yadr01.internal/172.20.6.3
```

Затем перезапустите dnsmasq через Intermasq (кнопка Reload).

Проверка:

```bash
dig athens.obshaga.yadr00.internal
```

Должен быть возвращён IP-адрес Caddy узла (`172.20.5.3`).

## 7. Caddy (на каждом узле)

Требования к Caddy **те же, что и в Povez**:

- Admin API на `:2019`;
- глобальная настройка Step-CA в качестве ACME CA;
- `Restart=always` в системном юните (для принудительного перезапуска —
  запрос `/stop` с автоматическим восстановлением процесса);
- корневой сертификат Step-CA в `/etc/caddy/root_ca.crt`.

Подробности — в документации Povez, раздел «Подготовка Caddy».

## 8. Первый запуск

1. В интерфейсе Pomen: «ВМ» → добавьте виртуальную машину (раздел 4).
2. «Контейнеры» → выберите виртуальную машину → «Обновить» — отобразится список
   контейнеров.
3. Нажмите «Выдать домен» для требуемого контейнера.
4. «Маршруты» → убедитесь, что домен появился в таблице.
5. Откройте в браузере `https://<container>.<vm>.<node>.internal` — должен
   открыться сервис через Caddy с действительным сертификатом Step-CA.

## 9. Обновление плагина

```bash
sudo systemctl stop intermasq
sudo cp pomen-linux /etc/intermasq/plugins/pomen/pomen
sudo chmod +x /etc/intermasq/plugins/pomen/pomen
sudo chown intermasq:intermasq /etc/intermasq/plugins/pomen/pomen
sudo systemctl start intermasq
```

Состояние (`vms.json`, `routes.json`) сохраняется; данные не теряются.

После обновления проверьте версию через `GET /api/version` или в заголовке
интерфейса — она должна совпадать с тегом релиза.
