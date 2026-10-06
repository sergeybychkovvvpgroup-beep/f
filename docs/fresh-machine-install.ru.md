# Установка f на свежей машине

## Что получится

- бинарник `f` в `~/.local/bin/f`;
- конфигурация в `~/.config/f/config.yaml`;
- отдельный каталог SSH-хостов `~/.ssh/config.d/f_hosts/`;
- include `Include ~/.ssh/config.d/f_hosts/*.conf` в `~/.ssh/config`.

## Установка

```bash
curl -fsSL https://raw.githubusercontent.com/sergeybychkovvvpgroup-beep/f/main/install.sh | sh
```

Если `~/.local/bin` ещё не входит в `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Проверка:

```bash
f version
```

## Подключение репозитория хостов

```bash
f setup
```

Команда предложит репозиторий `git@git.dawq.me:sergeyb/sshconfig.git` по умолчанию, проверит clone и dry-run push, затем добавит OpenSSH include и клонирует репозиторий в `~/.ssh/config.d/f_hosts`.

При ошибке SSH-аутентификации `f setup` напечатает только проверенный публичный SSH-ключ. Добавь его в права репозитория как deploy key с доступом на запись и повтори. Команда не отключает проверку host key: при ошибке host-key verification сначала проверь fingerprint сервера и обнови `known_hosts`. Для HTTPS URL настрой HTTPS credentials с правом записи.

```bash
f setup
```

Чтобы опубликовать уже подготовленные `.conf`-файлы:

```bash
mkdir -p ~/.ssh/config.d/f_hosts
cp ~/.ssh/config.d/my-hosts.conf ~/.ssh/config.d/f_hosts/
f setup --adopt <ssh-config-repo-url>
```

## Синхронизация

Если `~/.ssh/config.d/f_hosts` является Git-репозиторием, `f` подтягивает изменения перед чтением и отправляет изменения после `f add` или редактирования через `Ctrl+E`.

```bash
f config sync
cd ~/.ssh/config.d/f_hosts && git status --short --branch
```

## Обновление

```bash
f upgrade
```

Источник можно временно переопределить через `F_UPGRADE_REPO`.

## Диагностика

```bash
f config show
grep -n 'f_hosts' ~/.ssh/config
ssh -G <alias> >/dev/null
f list
```
