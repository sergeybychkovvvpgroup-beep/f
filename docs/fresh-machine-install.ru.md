# Установка aoo/f на свежей машине

Эта инструкция описывает быстрый сценарий: поставить `f`/`aoo`, подключить репозиторий SSH-хостов и сразу получить рабочий picker.

## Что получится

- бинарник `f` в `~/.local/bin/f`;
- совместимый symlink `~/.local/bin/aoo`;
- отдельный каталог хостов только для aoo:

```text
~/.ssh/config.d/aoo_hosts/
```

- include в OpenSSH config:

```ssh-config
Include ~/.ssh/config.d/aoo_hosts/*.conf
```

Так aoo не перетирает и не забирает под git весь существующий `~/.ssh/config.d`.

## Установка на новой машине

```bash
curl -fsSL https://raw.githubusercontent.com/sergeybychkovvvpgroup-beep/f/main/install.sh | sh
```

Если `~/.local/bin` ещё не в `PATH`, добавь его в shell config:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Проверь бинарник:

```bash
f version
```

## Подключение хостов из repo

```bash
f setup <ssh-config-repo-url>
```

Например:

```bash
f setup https://git.dawq.me/sergeyb/aoo-hosts.git
```

Команда:

1. создаст или обновит `~/.ssh/config`;
2. добавит `Include ~/.ssh/config.d/aoo_hosts/*.conf`;
3. склонирует repo в `~/.ssh/config.d/aoo_hosts`;
4. оставит остальные файлы `~/.ssh/config.d` нетронутыми.

После этого можно запускать:

```bash
f
f list
f prod
```

## Первичная публикация хостов с текущей машины

На машине, где уже есть текущий список хостов, один раз выполни:

```bash
f setup --adopt <ssh-config-repo-url>
```

Если старый файл уже лежит в `~/.ssh/config.d/aoo.conf`, команда автоматически скопирует его в:

```text
~/.ssh/config.d/aoo_hosts/aoo.conf
```

и запушит каталог `aoo_hosts` в указанный repo.

Если хосты лежат в других `.conf` файлах, сначала скопируй нужные файлы вручную:

```bash
mkdir -p ~/.ssh/config.d/aoo_hosts
cp ~/.ssh/config.d/my-hosts.conf ~/.ssh/config.d/aoo_hosts/
f setup --adopt <ssh-config-repo-url>
```

## Как работает синхронизация

Если `~/.ssh/config.d/aoo_hosts` является git repo, aoo синхронизирует его автоматически:

- при запуске picker делает `git pull --rebase --autostash`;
- перед `f add` тоже подтягивает изменения;
- после `f add` делает commit и push;
- после редактирования хоста через `Ctrl+E` / `Alt+E` делает commit и push.

Ручная проверка:

```bash
f config sync
cd ~/.ssh/config.d/aoo_hosts && git status --short --branch
```

## Обновление aoo

```bash
f upgrade
```

или повторно:

```bash
curl -fsSL https://raw.githubusercontent.com/sergeybychkovvvpgroup-beep/f/main/install.sh | sh
```

## Быстрая диагностика

Пути:

```bash
f config show
```

Проверка OpenSSH include:

```bash
grep -n 'aoo_hosts' ~/.ssh/config
```

Проверка видимости alias:

```bash
ssh -G <alias> >/dev/null
f list
```
