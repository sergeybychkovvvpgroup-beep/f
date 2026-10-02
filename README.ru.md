# f / aoo

`f` / `aoo` — персональный OpenSSH picker в стиле современного `fzf`.

Утилита читает обычный SSH config, выполняет fuzzy-поиск по всем типам записей в одном списке и запускает настоящий `ssh`. Отдельной скрытой базы импортированных SSH-хостов нет.

## Интерфейс

Рамки, вкладки, preview-панель и отдельный большой UI удалены. В обоих режимах используется один и тот же минимальный интерфейс:

- строка запроса;
- счётчик совпадений;
- одноcтрочный список;
- цветное выделение совпавших символов;
- контрастная выбранная строка.

Доступны два размера:

```bash
f config ui compact      # компактный picker в текущем терминале
f config ui full-screen  # тот же интерфейс на весь экран
```

Положение compact-режима:

```bash
f config layout bottom   # внизу экрана
f config layout top      # вверху экрана
```

Старые значения конфигурации мигрируют автоматически: `light` → `compact`, `full` → `full-screen`.

### Compact, bottom

![Compact picker at the bottom](docs/screenshots/compact-bottom.png)

### Compact, top

![Compact picker at the top](docs/screenshots/compact-top.png)

### Full-screen

![Full-screen picker](docs/screenshots/full-screen.png)

## Установка на новую машину

Одна команда ставит `f` и legacy-symlink `aoo`:

```bash
curl -fsSL https://raw.githubusercontent.com/sergeybychkovvvpgroup-beep/f/main/install.sh | sh
```

Встроенная проверка обновлений по умолчанию использует приватный Gitea-репозиторий `git@git.dawq.me:sergeyb/aoo.git`. При необходимости источник можно временно переопределить через `AOO_UPGRADE_REPO`.

Затем подключи репозиторий с хостами:

```bash
f setup <ssh-config-repo-url>
```

`f setup` клонирует repo в `~/.ssh/config.d/aoo_hosts`, добавляет в `~/.ssh/config` строку `Include ~/.ssh/config.d/aoo_hosts/*.conf`, и машина готова к работе. Чтобы один раз опубликовать текущий список хостов этой машины как начальное содержимое repo, используй:

```bash
f setup --adopt <ssh-config-repo-url>
```

Подробная статья: [Установка aoo/f на свежей машине](docs/fresh-machine-install.ru.md).

## Быстрый старт

```bash
f                       # открыть picker
f prod db               # открыть с начальным запросом
f list                  # вывести известные записи
f config show           # показать пути конфигов
f config sync           # подтянуть изменения хостов
f config ui compact     # compact UI
f config ui full-screen # полноэкранный UI
f config layout bottom  # compact снизу
f config layout top     # compact сверху
```

Клавиши:

- ввод — фильтр;
- `↑` / `↓`, `Ctrl+K` / `Ctrl+J` — выбор по видимому списку сверху вниз во всех layout;
- `Enter` — запустить выбранный SSH;
- `Ctrl+Y` / `Alt+Enter` — вывести команду без запуска;
- `Ctrl+E` / `Alt+E` — редактировать SSH config block;
- `Ctrl+N` — добавить хост;
- `Esc` / `Ctrl+C` — выйти.

## Единый поиск

Обычные SSH-входы, jump-маршруты, port forwards и записи с `RemoteCommand` находятся в одном fuzzy-списке. Вкладок и переключателей `F1`–`F4` нет.

## Модель SSH config

Ожидаемый активный файл:

```text
~/.ssh/config.d/aoo_hosts/aoo.conf
```

В `~/.ssh/config` должен быть include:

```ssh-config
Include ~/.ssh/config.d/aoo_hosts/*.conf
```

Picker читает `~/.ssh/config` и следует `Include` директивам. Для импортированных записей запуск идёт через точный alias:

```bash
ssh alias-name
```

Так сохраняется поведение OpenSSH для `ProxyJump`, forwards, `RemoteCommand`, identity files, legacy algorithms и остальных опций.

## Редактирование и синхронизация

`Ctrl+E` выходит из TUI и открывает `$EDITOR`, fallback — `nano`. Сохранённый block upsert-ится в `~/.ssh/config.d/aoo_hosts/aoo.conf` между маркерами `# aoo-edit begin/end`.

Если `~/.ssh/config.d/aoo_hosts` является git-репозиторием, `aoo` делает pull при запуске и commit/push после редактирования или `f add`.

## Правила отображения

- шумные префиксы `ssh-` и `aoo-` скрываются только в UI;
- route suffixes становятся читаемыми: `[netbird emergency]`, `[jump]`, `[tunnel]`;
- у forwards показывается remote endpoint;
- реальные alias и команды не изменяются.

Пример:

```text
ssh-pve-beria-03-netrack-netbird-emergency
```

отображается как:

```text
pve-beria-03-netrack [netbird emergency]
```

## Добавление записей

```bash
f add NAME HOST
f add db 10.20.30.40 -user admin -p 2222 --args "-A -J jump"
```

`Ctrl+N` в picker запускает интерактивное добавление. Новые записи сохраняются как OpenSSH blocks в `~/.ssh/config.d/aoo_hosts/aoo.conf`.

## Сборка и установка из исходников

```bash
go test ./...
go build -o ~/.local/bin/f ./cmd/f
ln -sf f ~/.local/bin/aoo
```

Production-сборка должна содержать точный commit:

```bash
commit=$(git rev-parse HEAD)
go build -buildvcs=false -ldflags "-X aoo/internal/app.buildCommit=$commit" -o ~/.local/bin/f ./cmd/f
```
