# f / aoo

`f` / `aoo` — персональный OpenSSH picker и launcher.

Утилита читает обычный SSH config, показывает fuzzy TUI и запускает настоящий `ssh`. Для импортированных SSH-записей нет отдельной скрытой базы хостов.

Текущий UX ориентирован на `sshelf`: сверху строка поиска, слева компактный список, справа подробности выбранной записи.

## Быстрый старт

```bash
f             # открыть picker
f prod db     # открыть picker с начальным запросом
f list        # вывести известные записи
f config show # показать пути конфигов
```

В picker:

- ввод — фильтр;
- `Enter` — запустить выбранный SSH;
- `Ctrl+Y` / `Alt+Enter` — вывести команду без запуска;
- `e` — редактировать SSH config block выбранной записи;
- `Esc` / `Ctrl+C` — выйти.

## Режимы

Записи группируются автоматически по синтаксису SSH config:

| Клавиша | Режим | Как определяется |
| --- | --- | --- |
| `F1` | general SSH logins | обычные SSH-записи |
| `F2` | jumps | `ProxyJump` / `ProxyCommand` |
| `F3` | port forwards | `LocalForward` / `RemoteForward` / `DynamicForward` |
| `F4` | commands | `RemoteCommand` или custom command entries |

Текущий режим показывается в верхней status line.

## Модель SSH config

`aoo` работает с обычными файлами OpenSSH config.

Ожидаемый активный файл:

```text
~/.ssh/config.d/aoo.conf
```

В `~/.ssh/config` обычно должен быть include:

```ssh-config
Include ~/.ssh/config.d/*.conf
```

Picker читает `~/.ssh/config` и следует `Include` директивам. Для импортированных SSH config entries запуск идёт через точный alias:

```bash
ssh alias-name
```

Так сохраняется поведение OpenSSH для `ProxyJump`, `LocalForward`, `RemoteCommand`, legacy algorithms, forwards и прочих опций.

Справа показывается best-effort expanded multiline command для читаемости, но выполнение всё равно идёт через alias.

## Редактирование

Hotkey:

```text
e
```

Поведение:

1. `aoo` выходит из TUI и открывает `$EDITOR`, fallback — `nano`.
2. В редактор попадает SSH config block, собранный из `ssh -G <alias>`.
3. После сохранения block upsert-ится в:

```text
~/.ssh/config.d/aoo.conf
```

Block оборачивается маркерами:

```ssh-config
# aoo-edit begin alias-name
Host alias-name
  HostName ...
  User ...
# aoo-edit end alias-name
```

Generated/imported source files не переписываются. Пользовательские правки должны жить в обычном активном `aoo.conf`.

## Правила отображения

Левая панель специально короткая:

- показывается только читаемое имя записи;
- шумные префиксы `ssh-` и `aoo-` скрываются только в UI;
- route suffixes человеко-читаемые, например:

```text
ssh-pve-beria-03-netrack-netbird-emergency
```

показывается как:

```text
pve-beria-03-netrack [netbird emergency]
```

Для forwards в коротком имени сразу виден удалённый endpoint:

```text
omada-chashnikovo [tunnel] [remote 10.117.100.10:443]
```

Правая панель содержит подробности выбранной строки:

- name;
- description;
- short command;
- full command, многострочно если полезно.

Теги сейчас в UI не показываются.

## Добавление записей

```bash
f add NAME HOST
f add db 10.20.30.40 -user admin -p 2222 --args "-A -J jump"
```

`Ctrl+N` в picker запускает интерактивное добавление.

Custom hosts хранятся тут:

```text
~/.config/aoo/config.d/*.yaml
```

Legacy `~/.config/aoo/hosts.yaml` пока читается для совместимости.

## Темы

Default theme для новых конфигов — `sshelf`.

```bash
f themes
f set-theme sshelf
```

## Сборка и установка

```bash
go test ./...
go build -o ~/.local/bin/f ./cmd/f
ln -sf f ~/.local/bin/aoo
```

Cross-build для Linux amd64:

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/aoo-f-linux-amd64 ./cmd/f
```
