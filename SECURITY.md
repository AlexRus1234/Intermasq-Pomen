# Security Policy

## Reporting a vulnerability

Please do not report security vulnerabilities through public issues, pull
requests, or other public discussions.

Use the private vulnerability-reporting feature of the Forgejo instance that
hosts this repository, if it is available. You can also contact the maintainer
privately through:

- Matrix: `@alexrus1234:matrix.alexrus1234.ru`
- Telegram: `AlexRus1234`
- Email: `alexrus1234alex@gmail.com`

Do not include secrets, production credentials, or personal data in the initial
report.

Please include, when possible:

- a short description of the vulnerability and its impact;
- the affected version, commit, or deployment configuration;
- clear reproduction steps or a minimal proof of concept;
- relevant logs, HTTP requests, or screenshots with credentials and personal
  data removed;
- any suggested mitigation or fix.

Maintainers will acknowledge a report as soon as practical, investigate it,
and coordinate disclosure with the reporter. Please allow time for a fix and
an announcement before disclosing the issue publicly.

## Scope

Pomen is a plugin for Intermasq. Security reports are especially relevant to:

- webhook secret handling (`X-VM-Secret`) and per-VM authentication;
- unauthorized access to the plugin API, its web UI, or its Unix socket;
- command injection, path traversal, or unsafe interaction with the Caddy
  Admin API, the VM registry (`vms.json`), or the route table (`routes.json`);
- exposure of secrets stored in `vms.json`, route records, or configuration
  files.

Because Pomen runs as a child process of Intermasq and relies on its
JWT-based authentication, serious vulnerabilities in Intermasq itself also
affect Pomen. Please report Intermasq-core issues through the Intermasq
security policy.

Reports about unsupported or intentionally exposed services, weak webhook
secrets, or insecure deployment configuration should clearly distinguish the
deployment issue from a vulnerability in Pomen itself.

## Supported versions

There is currently no separately published supported-version matrix. Unless a
release specifies otherwise, security fixes are made against the current
development branch. Users should keep their checkout up to date and run Pomen
only behind a properly protected Intermasq instance, with a valid
`INTERMASQ_SECRET`, restricted network access, and appropriate filesystem
permissions on the plugin directory and `vms.json`.

## Disclosure

Please keep vulnerability details private until the maintainers and reporter
agree that coordinated disclosure is appropriate. Once fixed, the project may
publish a short advisory or changelog entry describing the impact and affected
versions without exposing sensitive report details.

---

# Политика безопасности

## Сообщение об уязвимости

Не публикуйте сведения об уязвимостях в открытых issues, pull request или
других публичных обсуждениях.

Если это возможно, используйте приватный механизм сообщений об уязвимостях
на Forgejo-инстансе, где размещён репозиторий. Также можно связаться с
мейнтейнером напрямую через приватный канал:

- Matrix: `@alexrus1234:matrix.alexrus1234.ru`
- Telegram: `AlexRus1234`
- Email: `alexrus1234alex@gmail.com`

Не прикладывайте к первому сообщению секреты, рабочие учётные данные или
персональные данные.

По возможности укажите:

- краткое описание уязвимости и её последствий;
- затронутую версию, commit или конфигурацию развёртывания;
- точные шаги воспроизведения или минимальный proof of concept;
- относящиеся к проблеме логи, HTTP-запросы или скриншоты без секретов и
  персональных данных;
- предлагаемое исправление или способ временного снижения риска.

Мейнтейнеры подтвердят получение сообщения, изучат проблему и согласуют с
автором порядок раскрытия. Дождитесь исправления и объявления до публичного
раскрытия деталей.

## Область действия

Pomen — плагин Intermasq. Особенно важны сообщения об:

- обращении с секретом вебхука (`X-VM-Secret`) и аутентификации per-VM;
- несанкционированном доступе к API плагина, веб-интерфейсу или его
  Unix-сокету;
- command injection, path traversal или небезопасном взаимодействии с
  Admin API Caddy, реестром ВМ (`vms.json`) или таблицей маршрутов
  (`routes.json`);
- утечках секретов из `vms.json`, записей маршрутов или конфигурационных
  файлов.

Поскольку Pomen работает как дочерний процесс Intermasq и полагается на его
JWT-аутентификацию, серьёзные уязвимости в самом Intermasq затрагивают и Pomen.
Сообщения о проблемах ядра Intermasq направляйте через security-политику
Intermasq.

Сообщения о неподдерживаемых или намеренно открытых сервисах, слабых секретах
вебхуков или небезопасной конфигурации развёртывания должны чётко отличать
проблему развёртывания от уязвимости самого Pomen.

## Поддерживаемые версии

Отдельная матрица поддерживаемых версий пока не опубликована. Если в описании
релиза не указано иное, исправления безопасности вносятся в текущую ветку
разработки. Используйте Pomen только за правильно защищённым экземпляром
Intermasq: с корректным `INTERMASQ_SECRET`, ограниченным сетевым доступом и
корректными правами на каталог плагина и `vms.json`.

## Раскрытие информации

До согласования с мейнтейнерами и автором сохраняйте детали уязвимости в
тайне. После исправления проект может опубликовать краткое security advisory
или запись в истории версий без раскрытия чувствительных деталей сообщения.
