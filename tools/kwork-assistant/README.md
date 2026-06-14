# Kwork Assistant

Autopilot for a Kwork seller account.

Normal use is one command from the repo root:

```powershell
D:\claude\vk_bot\start-kwork-assistant.bat
```

The batch file:

- enters `tools\kwork-assistant`;
- creates `.env` from `.env.example` if missing;
- installs `playwright-core` if `node_modules` is missing;
- starts Chromium through Playwright;
- monitors Kwork inbox;
- sends replies automatically when a new request is detected.

On the first run, autopilot marks currently visible threads as old by default. This prevents it from replying to every old dialog already on the page. New threads after that are processed and answered.

## What To Fill

The first run creates these private files:

- `data/profile.md` - services, prices, deadlines, tone, stop topics.
- `data/examples.md` - examples of good incoming briefs and your successful replies.

They are ignored by Git.

## Important Settings

Edit `.env` only when needed:

```env
CHROMIUM_EXECUTABLE_PATH=C:\Users\GigaChat\AppData\Local\ms-playwright\chromium-1208\chrome-win64\chrome.exe
KWORK_USER_DATA_DIR=.\profile
KWORK_INBOX_URL=https://kwork.ru/inbox
KWORK_AUTO_SEND=true
KWORK_AUTOPRIME_ON_FIRST_RUN=true
PI_ENV_FILE=
PI_MODEL=
PI_THINKING_LEVEL=medium
PI_TIMEOUT_MS=240000
```

The assistant uses `@earendil-works/pi-coding-agent` directly, like the existing Pi bot in `D:\claude\stady`. It reads the same Pi auth/model storage as the installed Pi agent. If no model is available, open Pi once in a terminal and run `/login`.

If the working Pi bot keeps provider keys in its own env file, set:

```env
PI_ENV_FILE=D:\claude\stady\.env
```

## Service Commands

These are only for debugging:

```powershell
npm.cmd run login
npm.cmd run prime
npm.cmd run once
npm.cmd run monitor
npm.cmd run reply -- data\drafts\file.md --send
```
