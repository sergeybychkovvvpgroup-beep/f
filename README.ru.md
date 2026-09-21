# f / aoo

`f` / `aoo` — персональный OpenSSH picker и launcher.

Утилита читает обычный SSH config, показывает fuzzy TUI и запускает настоящий `ssh`. Для импортированных SSH-записей нет отдельной скрытой базы хостов.

Текущий UX ориентирован на `sshelf`: сверху строка поиска, слева компактный список, справа подробности выбранной записи.

## Установка на новую машину

Одна команда ставит `f` и legacy-symlink `aoo`:

```bash
curl -fsSL https://raw.githubusercontent.com/sergeybychkovvvpgroup-beep/f/main/install.sh | sh
```

Затем подключи репозиторий с хостами:

```bash
f setup <ssh-config-repo-url>
```

`f setup` клонирует repo в `~/.ssh/config.d/aoo_hosts`, добавляет в `~/.ssh/config` строку `Include ~/.ssh/config.d/aoo_hosts/*.conf`, и машина готова к работе. Чтобы один раз опубликовать текущий список хостов этой машины как начальное содержимое repo, используй `f setup --adopt <ssh-config-repo-url>`.

Подробная статья: [Установка aoo/f на свежей машине](docs/fresh-machine-install.ru.md).

## Быстрый старт

```bash
f             # открыть picker
f prod db     # открыть picker с начальным запросом
f list        # вывести известные записи
f config show # показать пути конфигов
f config sync # подтянуть изменения хостов
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
~/.ssh/config.d/aoo_hosts/aoo.conf
```

В `~/.ssh/config` обычно должен быть include:

```ssh-config
Include ~/.ssh/config.d/aoo_hosts/*.conf
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
~/.ssh/config.d/aoo_hosts/aoo.conf
```

Block оборачивается маркерами:

```ssh-config
# aoo-edit begin alias-name
Host alias-name
  HostName ...
  User ...
# aoo-edit end alias-name
```

Generated/imported source files не переписываются. Пользовательские правки должны жить в обычном активном `aoo.conf`. Если `~/.ssh/config.d/aoo_hosts` является git-репозиторием, aoo делает pull при запуске и commit/push после редактирования через `e` или `f add`, то есть синк хостов работает в обе стороны по модели nb notes.

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

Новые записи сохраняются как marked OpenSSH blocks в `~/.ssh/config.d/aoo_hosts/aoo.conf`. Legacy YAML hosts из `~/.config/aoo/` пока читаются для совместимости, но новые хосты должны жить в SSH config repo.

## Сборка и установка из исходников

```bash
go test ./...
go build -o ~/.local/bin/f ./cmd/f
ln -sf f ~/.local/bin/aoo
```

Cross-build для Linux amd64:

```bash
GOOS=linux GOARCH=amd64 go build -o /tmp/aoo-f-linux-amd64 ./cmd/f
```
