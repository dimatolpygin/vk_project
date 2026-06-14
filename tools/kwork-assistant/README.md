# Kwork Assistant

MVP ассистента для продавца на Kwork: Playwright открывает авторизованный Chromium, ищет новые диалоги/заявки, Pi agent анализирует ТЗ и готовит человечный черновик ответа.

По умолчанию ассистент **не отправляет ответы автоматически**. Он сохраняет черновики в `data/drafts/` и, если настроен Telegram, присылает уведомление.

## Быстрый старт

```powershell
cd D:\claude\vk_bot\tools\kwork-assistant
npm.cmd install
Copy-Item .env.example .env
npm.cmd run login
npm.cmd run prime
npm.cmd run once
```

`login` откроет Chromium с профилем из `KWORK_USER_DATA_DIR`. Войдите в аккаунт продавца Kwork и нажмите Enter в терминале. После этого `once` сможет использовать тот же профиль.

## Настройка

Основной файл настроек: `.env`.

Важные параметры:

- `CHROMIUM_EXECUTABLE_PATH` - путь к Chromium.
- `KWORK_USER_DATA_DIR` - отдельный профиль браузера с авторизацией продавца.
- `KWORK_INBOX_URL` - страница, где видны новые запросы.
- `AI_ENGINE` - `pi`, `openai` или `auto`.
- `PI_COMMAND` / `PI_ARGS` - как запускать Pi agent в RPC-режиме.
- `KWORK_*_SELECTOR` - CSS-селекторы для страницы Kwork, если стандартные не подойдут.

Перед боевым запуском заполните:

- `data/profile.md` - услуги, цены, сроки, стиль общения, стоп-темы.
- `data/examples.md` - примеры ваших удачных ответов и кейсов.

## Команды

```powershell
npm.cmd run login
```

Открыть Chromium и авторизовать профиль.

```powershell
npm.cmd run prime
```

Пометить текущие видимые диалоги как уже просмотренные, чтобы мониторинг реагировал только на новые ссылки после запуска.

```powershell
npm.cmd run once
```

Один раз проверить новые диалоги и сохранить черновики.

```powershell
npm.cmd run monitor
```

Постоянно проверять входящие с интервалом `KWORK_POLL_MS`.

```powershell
npm.cmd run reply -- data\drafts\2026-06-14T120000-abc123.md
```

Открыть диалог из черновика и вставить ответ в поле ввода. Без `--send` ответ не отправляется.

```powershell
npm.cmd run reply -- data\drafts\2026-06-14T120000-abc123.md --send
```

Вставить и нажать кнопку отправки. Используйте только после проверки селекторов.

## Важное

Kwork может менять HTML, поэтому первый запуск лучше делать в режиме `once` и смотреть, корректно ли находятся диалоги. Если ассистент не видит заявки или не вставляет ответ, настройте селекторы в `.env`.
